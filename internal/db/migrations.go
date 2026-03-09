package db

import (
	"database/sql"
	"fmt"
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
	if err := d.ensureArchivedColumn(); err != nil {
		return err
	}
	if err := d.ensureStartedAtColumn(); err != nil {
		return err
	}
	if err := d.ensureLinksColumn(); err != nil {
		return err
	}
	return nil
}

func (d *DB) ensureStartedAtColumn() error {
	return d.ensureColumn("tasks", "started_at", "DATETIME")
}

func (d *DB) ensureLinksColumn() error {
	return d.ensureColumn("tasks", "links", "TEXT DEFAULT '[]'")
}

func (d *DB) ensureColumn(table, column, colDef string) error {
	rows, err := d.conn.Query(fmt.Sprintf(`PRAGMA table_info(%s)`, table))
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, colType string
		var notNull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &colType, &notNull, &dflt, &pk); err != nil {
			return err
		}
		if name == column {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	_, err = d.conn.Exec(fmt.Sprintf(`ALTER TABLE %s ADD COLUMN %s %s`, table, column, colDef))
	return err
}

func (d *DB) ensureArchivedColumn() error {
	rows, err := d.conn.Query(`PRAGMA table_info(tasks)`)
	if err != nil {
		return err
	}
	defer rows.Close()

	hasArchived := false
	for rows.Next() {
		var cid int
		var name, colType string
		var notNull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &colType, &notNull, &dflt, &pk); err != nil {
			return err
		}
		if name == "archived" {
			hasArchived = true
			break
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	if !hasArchived {
		if _, err := d.conn.Exec(`ALTER TABLE tasks ADD COLUMN archived INTEGER NOT NULL DEFAULT 0`); err != nil {
			return err
		}
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

