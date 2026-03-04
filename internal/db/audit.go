package db

func (d *DB) WriteAudit(taskID, field string, oldValue, newValue *string, changedBy string) error {
	_, err := d.conn.Exec(
		`INSERT INTO audit (task_id, field, old_value, new_value, changed_by) VALUES (?, ?, ?, ?, ?)`,
		taskID, field, oldValue, newValue, changedBy,
	)
	return err
}
