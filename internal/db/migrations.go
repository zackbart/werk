package db

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
)

const schema = `
CREATE TABLE IF NOT EXISTS tasks (
  id          TEXT PRIMARY KEY,
  ref         TEXT UNIQUE,
  parent_id   TEXT REFERENCES tasks(id),
  type        TEXT NOT NULL CHECK(type IN ('epic','task','subtask')),
  title       TEXT NOT NULL,
  status      TEXT NOT NULL DEFAULT 'open'
                CHECK(status IN ('open','in_progress','blocked','done')),
  priority    INTEGER NOT NULL DEFAULT 2
                CHECK(priority BETWEEN 0 AND 4),
  notes       TEXT,
  created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at  DATETIME,
  closed_at   DATETIME
);

CREATE TABLE IF NOT EXISTS dependencies (
  upstream_id   TEXT NOT NULL REFERENCES tasks(id),
  downstream_id TEXT NOT NULL REFERENCES tasks(id),
  PRIMARY KEY (upstream_id, downstream_id),
  CHECK (upstream_id != downstream_id)
);

CREATE TABLE IF NOT EXISTS audit (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  task_id     TEXT NOT NULL REFERENCES tasks(id),
  field       TEXT NOT NULL,
  old_value   TEXT,
  new_value   TEXT,
  changed_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  changed_by  TEXT NOT NULL CHECK(changed_by IN ('agent','human'))
);

CREATE TABLE IF NOT EXISTS decisions (
  id          TEXT PRIMARY KEY,
  summary     TEXT NOT NULL,
  rationale   TEXT,
  created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  created_by  TEXT NOT NULL CHECK(created_by IN ('agent','human'))
);

CREATE TABLE IF NOT EXISTS sessions (
  id            TEXT PRIMARY KEY,
  started_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  ended_at      DATETIME,
  summary       TEXT,
  tasks_touched TEXT DEFAULT '[]'
);
`

func (d *DB) Migrate() error {
	_, err := d.conn.Exec(schema)
	if err != nil {
		return err
	}

	if err := d.ensureTaskRefColumn(); err != nil {
		return err
	}
	if err := d.backfillMissingRefs(); err != nil {
		return err
	}
	return nil
}

func (d *DB) ensureTaskRefColumn() error {
	rows, err := d.conn.Query(`PRAGMA table_info(tasks)`)
	if err != nil {
		return err
	}
	defer rows.Close()

	hasRef := false
	for rows.Next() {
		var cid int
		var name, colType string
		var notNull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &colType, &notNull, &dflt, &pk); err != nil {
			return err
		}
		if name == "ref" {
			hasRef = true
			break
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	if !hasRef {
		if _, err := d.conn.Exec(`ALTER TABLE tasks ADD COLUMN ref TEXT`); err != nil {
			return err
		}
	}
	_, err = d.conn.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_tasks_ref ON tasks(ref) WHERE ref IS NOT NULL`)
	return err
}

func (d *DB) backfillMissingRefs() error {
	tx, err := d.conn.Begin()
	if err != nil {
		return fmt.Errorf("begin backfill transaction: %w", err)
	}
	defer tx.Rollback()

	// Fill top-level epics first.
	epics, err := d.selectTaskIDsByTypeTx(tx, "epic")
	if err != nil {
		return err
	}
	for _, epicID := range epics {
		if err := d.ensureTaskRefTx(tx, epicID); err != nil {
			return err
		}
	}

	// Then tasks, then subtasks.
	tasks, err := d.selectTaskIDsByTypeTx(tx, "task")
	if err != nil {
		return err
	}
	for _, taskID := range tasks {
		if err := d.ensureTaskRefTx(tx, taskID); err != nil {
			return err
		}
	}

	subtasks, err := d.selectTaskIDsByTypeTx(tx, "subtask")
	if err != nil {
		return err
	}
	for _, subtaskID := range subtasks {
		if err := d.ensureTaskRefTx(tx, subtaskID); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// querier abstracts *sql.DB and *sql.Tx for shared query logic.
type querier interface {
	Query(query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(query string, args ...interface{}) *sql.Row
	Exec(stmt string, args ...interface{}) (sql.Result, error)
}

func (d *DB) selectTaskIDsByType(taskType string) ([]string, error) {
	return selectTaskIDsByType(d.conn, taskType)
}

func (d *DB) selectTaskIDsByTypeTx(tx *sql.Tx, taskType string) ([]string, error) {
	return selectTaskIDsByType(tx, taskType)
}

func selectTaskIDsByType(q querier, taskType string) ([]string, error) {
	rows, err := q.Query(`SELECT id FROM tasks WHERE type = ? AND (ref IS NULL OR ref = '') ORDER BY created_at ASC, id ASC`, taskType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (d *DB) ensureTaskRef(taskID string) error {
	return ensureTaskRef(d.conn, taskID)
}

func (d *DB) ensureTaskRefTx(tx *sql.Tx, taskID string) error {
	return ensureTaskRef(tx, taskID)
}

func ensureTaskRef(q querier, taskID string) error {
	var currentRef sql.NullString
	var taskType string
	var parentID sql.NullString
	err := q.QueryRow(`SELECT ref, type, parent_id FROM tasks WHERE id = ?`, taskID).Scan(&currentRef, &taskType, &parentID)
	if err != nil {
		return err
	}
	if currentRef.Valid && strings.TrimSpace(currentRef.String) != "" {
		return nil
	}

	var parentPtr *string
	if parentID.Valid {
		parentPtr = &parentID.String
	}

	nextRef, err := nextTaskRef(q, taskType, parentPtr)
	if err != nil {
		return err
	}
	_, err = q.Exec(`UPDATE tasks SET ref = ? WHERE id = ?`, nextRef, taskID)
	return err
}

func (d *DB) nextTaskRef(taskType string, parentID *string) (string, error) {
	return nextTaskRef(d.conn, taskType, parentID)
}

func nextTaskRef(q querier, taskType string, parentID *string) (string, error) {
	if taskType == "epic" {
		next, err := nextTopLevelRef(q)
		if err != nil {
			return "", err
		}
		return strconv.Itoa(next), nil
	}

	if parentID == nil {
		return "", fmt.Errorf("parent is required for %s refs", taskType)
	}

	var parentRef sql.NullString
	err := q.QueryRow(`SELECT ref FROM tasks WHERE id = ?`, *parentID).Scan(&parentRef)
	if err != nil {
		return "", err
	}
	if !parentRef.Valid || strings.TrimSpace(parentRef.String) == "" {
		if err := ensureTaskRef(q, *parentID); err != nil {
			return "", err
		}
		if err := q.QueryRow(`SELECT ref FROM tasks WHERE id = ?`, *parentID).Scan(&parentRef); err != nil {
			return "", err
		}
	}

	next, err := nextChildSuffix(q, *parentID)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s.%d", parentRef.String, next), nil
}

func nextTopLevelRef(q querier) (int, error) {
	var maxN sql.NullInt64
	err := q.QueryRow(`SELECT MAX(CAST(ref AS INTEGER)) FROM tasks WHERE type = 'epic' AND ref IS NOT NULL`).Scan(&maxN)
	if err != nil {
		return 0, err
	}
	if !maxN.Valid {
		return 1, nil
	}
	return int(maxN.Int64) + 1, nil
}

func nextChildSuffix(q querier, parentID string) (int, error) {
	rows, err := q.Query(`SELECT ref FROM tasks WHERE parent_id = ? AND ref IS NOT NULL`, parentID)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	maxN := 0
	for rows.Next() {
		var ref string
		if err := rows.Scan(&ref); err != nil {
			return 0, err
		}
		parts := strings.Split(strings.TrimSpace(ref), ".")
		if len(parts) == 0 {
			continue
		}
		n, err := strconv.Atoi(parts[len(parts)-1])
		if err != nil {
			continue
		}
		if n > maxN {
			maxN = n
		}
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	return maxN + 1, nil
}
