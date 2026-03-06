package db

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
)

func (d *DB) ResolveTaskID(idOrRef string) (string, error) {
	if _, err := d.GetTask(idOrRef); err == nil {
		return idOrRef, nil
	} else if ce := AsCodedError(err); ce != nil && ce.Code != ErrCodeNotFound {
		return "", err
	}

	var id string
	err := d.conn.QueryRow(`SELECT id FROM tasks WHERE ref = ?`, idOrRef).Scan(&id)
	if err == sql.ErrNoRows {
		return "", codedError(ErrCodeNotFound, fmt.Sprintf("task not found: %s", idOrRef))
	}
	if err != nil {
		return "", err
	}
	return id, nil
}

func (d *DB) nextRef(taskType string, parentID *string) (string, error) {
	scope, err := d.refScope(taskType, parentID)
	if err != nil {
		return "", err
	}
	nextIndex, err := d.nextCounter(scope)
	if err != nil {
		return "", err
	}

	if taskType == "epic" {
		return strconv.Itoa(nextIndex), nil
	}

	if parentID == nil {
		return "", codedError(ErrCodeInvalidParent, "missing parent for ref assignment")
	}

	parent, err := d.GetTask(*parentID)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s.%d", parent.Ref, nextIndex), nil
}

func (d *DB) refScope(taskType string, parentID *string) (string, error) {
	switch taskType {
	case "epic":
		return "epic", nil
	case "task", "subtask":
		if parentID == nil {
			return "", codedError(ErrCodeInvalidParent, "missing parent for ref assignment")
		}
		return fmt.Sprintf("%s:%s", taskType, *parentID), nil
	default:
		return "", codedError(ErrCodeInvalidRef, fmt.Sprintf("unsupported task type for refs: %s", taskType))
	}
}

func (d *DB) nextCounter(scope string) (int, error) {
	tx, err := d.conn.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var next int
	err = tx.QueryRow(`SELECT next_value FROM ref_counters WHERE scope = ?`, scope).Scan(&next)
	if err == sql.ErrNoRows {
		if _, err := tx.Exec(`INSERT INTO ref_counters (scope, next_value) VALUES (?, ?)`, scope, 2); err != nil {
			return 0, err
		}
		if err := tx.Commit(); err != nil {
			return 0, err
		}
		return 1, nil
	}
	if err != nil {
		return 0, err
	}

	if _, err := tx.Exec(`UPDATE ref_counters SET next_value = ? WHERE scope = ?`, next+1, scope); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return next, nil
}

func (d *DB) backfillMissingRefs() error {
	if err := d.backfillMissingEpicRefs(); err != nil {
		return err
	}
	if err := d.backfillMissingChildRefs("task", "epic"); err != nil {
		return err
	}
	if err := d.backfillMissingChildRefs("subtask", "task"); err != nil {
		return err
	}
	return nil
}

func (d *DB) backfillMissingEpicRefs() error {
	maxIndex := 0
	rows, err := d.conn.Query(`SELECT ref FROM tasks WHERE type = 'epic' AND ref IS NOT NULL AND ref != ''`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var ref string
		if err := rows.Scan(&ref); err != nil {
			rows.Close()
			return err
		}
		if idx, err := strconv.Atoi(ref); err == nil && idx > maxIndex {
			maxIndex = idx
		}
	}
	rows.Close()

	missingRows, err := d.conn.Query(`SELECT id FROM tasks WHERE type = 'epic' AND (ref IS NULL OR ref = '') ORDER BY created_at ASC, id ASC`)
	if err != nil {
		return err
	}
	defer missingRows.Close()

	for missingRows.Next() {
		var id string
		if err := missingRows.Scan(&id); err != nil {
			return err
		}
		maxIndex++
		if _, err := d.conn.Exec(`UPDATE tasks SET ref = ? WHERE id = ?`, strconv.Itoa(maxIndex), id); err != nil {
			return err
		}
	}
	return nil
}

func (d *DB) backfillMissingChildRefs(childType, parentType string) error {
	parentRows, err := d.conn.Query(`SELECT id, ref FROM tasks WHERE type = ? ORDER BY created_at ASC, id ASC`, parentType)
	if err != nil {
		return err
	}
	defer parentRows.Close()

	for parentRows.Next() {
		var parentID, parentRef string
		if err := parentRows.Scan(&parentID, &parentRef); err != nil {
			return err
		}

		maxIndex := 0
		childRows, err := d.conn.Query(`SELECT ref FROM tasks WHERE type = ? AND parent_id = ? AND ref IS NOT NULL AND ref != ''`, childType, parentID)
		if err != nil {
			return err
		}
		for childRows.Next() {
			var childRef string
			if err := childRows.Scan(&childRef); err != nil {
				childRows.Close()
				return err
			}
			parts := strings.Split(childRef, ".")
			idx, err := strconv.Atoi(parts[len(parts)-1])
			if err == nil && idx > maxIndex {
				maxIndex = idx
			}
		}
		childRows.Close()

		missingRows, err := d.conn.Query(`SELECT id FROM tasks WHERE type = ? AND parent_id = ? AND (ref IS NULL OR ref = '') ORDER BY created_at ASC, id ASC`, childType, parentID)
		if err != nil {
			return err
		}
		for missingRows.Next() {
			var id string
			if err := missingRows.Scan(&id); err != nil {
				missingRows.Close()
				return err
			}
			maxIndex++
			ref := fmt.Sprintf("%s.%d", parentRef, maxIndex)
			if _, err := d.conn.Exec(`UPDATE tasks SET ref = ? WHERE id = ?`, ref, id); err != nil {
				missingRows.Close()
				return err
			}
		}
		missingRows.Close()
	}
	return nil
}

func (d *DB) rebuildRefCounters() error {
	if _, err := d.conn.Exec(`DELETE FROM ref_counters`); err != nil {
		return err
	}

	epicRows, err := d.conn.Query(`SELECT ref FROM tasks WHERE type = 'epic' AND ref IS NOT NULL AND ref != ''`)
	if err != nil {
		return err
	}
	maxEpic := 0
	for epicRows.Next() {
		var ref string
		if err := epicRows.Scan(&ref); err != nil {
			epicRows.Close()
			return err
		}
		if idx, err := strconv.Atoi(ref); err == nil && idx > maxEpic {
			maxEpic = idx
		}
	}
	epicRows.Close()
	if _, err := d.conn.Exec(`INSERT INTO ref_counters (scope, next_value) VALUES ('epic', ?)`, maxEpic+1); err != nil {
		return err
	}

	if err := d.rebuildChildCounters("task"); err != nil {
		return err
	}
	if err := d.rebuildChildCounters("subtask"); err != nil {
		return err
	}
	return nil
}

func (d *DB) rebuildChildCounters(childType string) error {
	rows, err := d.conn.Query(`SELECT parent_id, ref FROM tasks WHERE type = ? AND parent_id IS NOT NULL AND ref IS NOT NULL AND ref != ''`, childType)
	if err != nil {
		return err
	}
	defer rows.Close()

	maxByParent := map[string]int{}
	for rows.Next() {
		var parentID, ref string
		if err := rows.Scan(&parentID, &ref); err != nil {
			return err
		}
		parts := strings.Split(ref, ".")
		idx, err := strconv.Atoi(parts[len(parts)-1])
		if err != nil {
			continue
		}
		if idx > maxByParent[parentID] {
			maxByParent[parentID] = idx
		}
	}

	for parentID, max := range maxByParent {
		scope := fmt.Sprintf("%s:%s", childType, parentID)
		if _, err := d.conn.Exec(`INSERT INTO ref_counters (scope, next_value) VALUES (?, ?)`, scope, max+1); err != nil {
			return err
		}
	}
	return nil
}
