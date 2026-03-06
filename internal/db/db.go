package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"werk/internal/models"
)

type DB struct {
	conn *sql.DB
	path string
}

func Open(path string) (*DB, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("creating db directory: %w", err)
	}

	conn, err := sql.Open("sqlite", path+"?_pragma=foreign_keys(1)&_pragma=journal_mode(wal)")
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	d := &DB{conn: conn, path: path}
	if err := d.Migrate(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("running migrations: %w", err)
	}

	return d, nil
}

func (d *DB) Close() error {
	return d.conn.Close()
}

func (d *DB) GetConn() *sql.DB {
	return d.conn
}

// --- Tasks ---

func (d *DB) CreateTask(taskType, title string, parentID *string, priority int, notes *string, changedBy string) (*models.Task, error) {
	// Validate hierarchy
	if err := d.validateHierarchy(taskType, parentID); err != nil {
		return nil, err
	}

	prefix := map[string]string{"epic": "ep-", "task": "tk-", "subtask": "st-"}[taskType]
	id, err := d.GenerateID(prefix, title)
	if err != nil {
		return nil, err
	}

	ref, err := d.nextTaskRef(taskType, parentID)
	if err != nil {
		return nil, err
	}

	_, err = d.conn.Exec(
		`INSERT INTO tasks (id, ref, parent_id, type, title, priority, notes) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, ref, parentID, taskType, title, priority, notes,
	)
	if err != nil {
		return nil, err
	}

	// Audit
	titleStr := title
	statusStr := "open"
	priStr := fmt.Sprintf("%d", priority)
	d.WriteAudit(id, "title", nil, &titleStr, changedBy)
	d.WriteAudit(id, "status", nil, &statusStr, changedBy)
	d.WriteAudit(id, "priority", nil, &priStr, changedBy)
	if notes != nil {
		d.WriteAudit(id, "notes", nil, notes, changedBy)
	}
	if parentID != nil {
		d.WriteAudit(id, "parent_id", nil, parentID, changedBy)
	}
	d.WriteAudit(id, "ref", nil, &ref, changedBy)

	d.touchSession(id)

	return d.GetTask(id)
}

func (d *DB) validateHierarchy(taskType string, parentID *string) error {
	switch taskType {
	case "epic":
		if parentID != nil {
			return fmt.Errorf("epics cannot have a parent")
		}
	case "task":
		if parentID == nil {
			return fmt.Errorf("tasks must have an epic parent (use --epic)")
		}
		parent, err := d.GetTask(*parentID)
		if err != nil {
			return fmt.Errorf("parent not found: %s", *parentID)
		}
		if parent.Type != "epic" {
			return fmt.Errorf("task parent must be an epic, got %s", parent.Type)
		}
	case "subtask":
		if parentID == nil {
			return fmt.Errorf("subtasks must have a task parent (use --task)")
		}
		parent, err := d.GetTask(*parentID)
		if err != nil {
			return fmt.Errorf("parent not found: %s", *parentID)
		}
		if parent.Type != "task" {
			return fmt.Errorf("subtask parent must be a task, got %s", parent.Type)
		}
	}
	return nil
}

func (d *DB) GetTask(id string) (*models.Task, error) {
	t := &models.Task{}
	var ref, createdAt, updatedAt, closedAt, startedAt sql.NullString
	var archived int
	var linksJSON sql.NullString
	err := d.conn.QueryRow(
		`SELECT id, ref, parent_id, type, title, status, priority, notes, archived, created_at, updated_at, closed_at, started_at, links FROM tasks WHERE id = ?`, id,
	).Scan(&t.ID, &ref, &t.ParentID, &t.Type, &t.Title, &t.Status, &t.Priority, &t.Notes, &archived, &createdAt, &updatedAt, &closedAt, &startedAt, &linksJSON)
	t.Archived = archived != 0
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("task not found: %s", id)
		}
		return nil, err
	}
	if ref.Valid {
		t.Ref = ref.String
	}

	t.CreatedAt = parseTime(createdAt.String)
	if updatedAt.Valid {
		ut := parseTime(updatedAt.String)
		t.UpdatedAt = &ut
	}
	if closedAt.Valid {
		ct := parseTime(closedAt.String)
		t.ClosedAt = &ct
	}
	if startedAt.Valid {
		st := parseTime(startedAt.String)
		t.StartedAt = &st
	}
	if linksJSON.Valid && linksJSON.String != "" {
		json.Unmarshal([]byte(linksJSON.String), &t.Links)
	}
	if t.ParentID != nil {
		var parentRef sql.NullString
		if err := d.conn.QueryRow(`SELECT ref FROM tasks WHERE id = ?`, *t.ParentID).Scan(&parentRef); err == nil && parentRef.Valid {
			t.ParentRef = &parentRef.String
		}
	}

	// Load blockers
	rows, err := d.conn.Query(`SELECT upstream_id FROM dependencies WHERE downstream_id = ?`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var bid string
		rows.Scan(&bid)
		// Only include if upstream is not done
		var status string
		d.conn.QueryRow(`SELECT status FROM tasks WHERE id = ?`, bid).Scan(&status)
		if status != "done" {
			t.Blockers = append(t.Blockers, bid)
		}
	}

	return t, nil
}

func (d *DB) GetTaskByRef(ref string) (*models.Task, error) {
	var id string
	err := d.conn.QueryRow(`SELECT id FROM tasks WHERE ref = ?`, ref).Scan(&id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("task not found: %s", ref)
		}
		return nil, err
	}
	return d.GetTask(id)
}

func (d *DB) ResolveTaskID(idOrRef string) (string, error) {
	var exists int
	if err := d.conn.QueryRow(`SELECT 1 FROM tasks WHERE id = ? LIMIT 1`, idOrRef).Scan(&exists); err == nil {
		return idOrRef, nil
	}
	var id string
	err := d.conn.QueryRow(`SELECT id FROM tasks WHERE ref = ?`, idOrRef).Scan(&id)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("task not found: %s", idOrRef)
		}
		return "", err
	}
	return id, nil
}

func (d *DB) ResolveTaskRef(idOrRef string) (string, error) {
	t, err := d.GetTaskByIDOrRef(idOrRef)
	if err != nil {
		return "", err
	}
	return t.Ref, nil
}

func (d *DB) GetTaskByIDOrRef(idOrRef string) (*models.Task, error) {
	id, err := d.ResolveTaskID(idOrRef)
	if err != nil {
		return nil, err
	}
	return d.GetTask(id)
}

type listOpts struct {
	includeArchived bool
}

type ListOption func(*listOpts)

func WithArchived() ListOption {
	return func(o *listOpts) { o.includeArchived = true }
}

func (d *DB) ListTasks(taskType string, parentID *string, status string, opts ...ListOption) ([]models.Task, error) {
	o := &listOpts{}
	for _, fn := range opts {
		fn(o)
	}

	query := `SELECT id FROM tasks WHERE type = ?`
	args := []interface{}{taskType}

	if !o.includeArchived {
		query += ` AND archived = 0`
	}

	if parentID != nil {
		query += ` AND parent_id = ?`
		args = append(args, *parentID)
	}

	if status != "" && status != "all" {
		query += ` AND status = ?`
		args = append(args, status)
	}

	query += ` ORDER BY priority ASC, created_at ASC`

	rows, err := d.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []models.Task
	for rows.Next() {
		var id string
		rows.Scan(&id)
		t, err := d.GetTask(id)
		if err != nil {
			continue
		}
		tasks = append(tasks, *t)
	}
	return tasks, nil
}

func (d *DB) UpdateTask(id string, title, notes *string, priority *int, changedBy string) (*models.Task, error) {
	existing, err := d.GetTask(id)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC().Format(time.RFC3339)

	if title != nil && *title != existing.Title {
		oldTitle := existing.Title
		d.conn.Exec(`UPDATE tasks SET title = ?, updated_at = ? WHERE id = ?`, *title, now, id)
		d.WriteAudit(id, "title", &oldTitle, title, changedBy)
	}

	if priority != nil && *priority != existing.Priority {
		oldPri := fmt.Sprintf("%d", existing.Priority)
		newPri := fmt.Sprintf("%d", *priority)
		d.conn.Exec(`UPDATE tasks SET priority = ?, updated_at = ? WHERE id = ?`, *priority, now, id)
		d.WriteAudit(id, "priority", &oldPri, &newPri, changedBy)
	}

	if notes != nil {
		var oldNotes *string
		if existing.Notes != nil {
			oldNotes = existing.Notes
		}
		d.conn.Exec(`UPDATE tasks SET notes = ?, updated_at = ? WHERE id = ?`, *notes, now, id)
		d.WriteAudit(id, "notes", oldNotes, notes, changedBy)
	}

	d.touchSession(id)
	return d.GetTask(id)
}

func (d *DB) AppendNote(id, text, changedBy string) (*models.Task, error) {
	existing, err := d.GetTask(id)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	var oldNotes *string
	var newNotes string
	if existing.Notes != nil && *existing.Notes != "" {
		oldNotes = existing.Notes
		newNotes = *existing.Notes + "\n" + text
	} else {
		newNotes = text
	}
	d.conn.Exec(`UPDATE tasks SET notes = ?, updated_at = ? WHERE id = ?`, newNotes, now, id)
	d.WriteAudit(id, "notes", oldNotes, &newNotes, changedBy)
	d.touchSession(id)
	return d.GetTask(id)
}

func (d *DB) AddLink(id, link, changedBy string) (*models.Task, error) {
	existing, err := d.GetTask(id)
	if err != nil {
		return nil, err
	}
	for _, l := range existing.Links {
		if l == link {
			return existing, nil // already linked
		}
	}
	now := time.Now().UTC().Format(time.RFC3339)
	newLinks := append(existing.Links, link)
	linksJSON, _ := json.Marshal(newLinks)
	d.conn.Exec(`UPDATE tasks SET links = ?, updated_at = ? WHERE id = ?`, string(linksJSON), now, id)
	old := "[]"
	if len(existing.Links) > 0 {
		ob, _ := json.Marshal(existing.Links)
		old = string(ob)
	}
	new := string(linksJSON)
	d.WriteAudit(id, "links", &old, &new, changedBy)
	d.touchSession(id)
	return d.GetTask(id)
}

func (d *DB) RemoveLink(id, link, changedBy string) (*models.Task, error) {
	existing, err := d.GetTask(id)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	var newLinks []string
	for _, l := range existing.Links {
		if l != link {
			newLinks = append(newLinks, l)
		}
	}
	if len(newLinks) == len(existing.Links) {
		return existing, nil // link not found
	}
	linksJSON, _ := json.Marshal(newLinks)
	d.conn.Exec(`UPDATE tasks SET links = ?, updated_at = ? WHERE id = ?`, string(linksJSON), now, id)
	ob, _ := json.Marshal(existing.Links)
	old := string(ob)
	new := string(linksJSON)
	d.WriteAudit(id, "links", &old, &new, changedBy)
	d.touchSession(id)
	return d.GetTask(id)
}

func (d *DB) GetSubtaskProgress(parentID string) (*models.SubtaskProgress, error) {
	p := &models.SubtaskProgress{}
	d.conn.QueryRow(`SELECT COUNT(*) FROM tasks WHERE parent_id = ? AND type = 'subtask'`, parentID).Scan(&p.Total)
	if p.Total == 0 {
		return nil, nil
	}
	d.conn.QueryRow(`SELECT COUNT(*) FROM tasks WHERE parent_id = ? AND type = 'subtask' AND status = 'done'`, parentID).Scan(&p.Done)
	d.conn.QueryRow(`SELECT COUNT(*) FROM tasks WHERE parent_id = ? AND type = 'subtask' AND status = 'open'`, parentID).Scan(&p.Open)
	return p, nil
}

func (d *DB) GetLogForTask(taskID string, limit int, verbose bool) ([]models.LogEntry, error) {
	query := `
		SELECT timestamp, event, id, title, detail FROM (
			SELECT a.changed_at AS timestamp,
				CASE a.new_value
					WHEN 'in_progress' THEN 'started'
					WHEN 'done' THEN 'closed'
					WHEN 'blocked' THEN 'blocked'
				END AS event,
				t.id AS id,
				t.type || ': ' || t.title AS title,
				CASE WHEN ? THEN t.notes ELSE NULL END AS detail
			FROM audit a
			JOIN tasks t ON a.task_id = t.id
			WHERE a.field = 'status' AND a.new_value IN ('in_progress', 'done', 'blocked')
			AND a.task_id = ?

			UNION ALL

			SELECT a.changed_at AS timestamp,
				'created' AS event,
				t.id AS id,
				t.type || ': ' || t.title AS title,
				CASE WHEN ? THEN t.notes ELSE NULL END AS detail
			FROM audit a
			JOIN tasks t ON a.task_id = t.id
			WHERE a.field = 'status' AND a.old_value IS NULL AND a.new_value = 'open'
			AND a.task_id = ?
		)
		ORDER BY timestamp DESC
		LIMIT ?
	`
	rows, err := d.conn.Query(query, verbose, taskID, verbose, taskID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []models.LogEntry
	for rows.Next() {
		e := models.LogEntry{}
		var ts string
		rows.Scan(&ts, &e.Event, &e.ID, &e.Title, &e.Detail)
		e.Timestamp = parseTime(ts)
		entries = append(entries, e)
	}
	return entries, nil
}

func (d *DB) GenerateAutoSummary(sessionID string) (string, error) {
	s, err := d.GetSession(sessionID)
	if err != nil {
		return "", err
	}
	// Use SQLite-native timestamp format for comparison (CURRENT_TIMESTAMP format)
	startTS := s.StartedAt.Format("2006-01-02 15:04:05")
	rows, err := d.conn.Query(`
		SELECT a.task_id, t.type, t.title, a.field, a.old_value, a.new_value
		FROM audit a
		JOIN tasks t ON a.task_id = t.id
		WHERE a.changed_at >= ?
		ORDER BY a.changed_at ASC
	`, startTS)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	created := []string{}
	started := []string{}
	closed := []string{}
	seen := map[string]bool{}
	for rows.Next() {
		var taskID, taskType, title, field string
		var oldVal, newVal sql.NullString
		rows.Scan(&taskID, &taskType, &title, &field, &oldVal, &newVal)
		key := taskID + field + newVal.String
		if seen[key] {
			continue
		}
		seen[key] = true
		label := taskType + " '" + title + "'"
		if field == "status" {
			if !oldVal.Valid && newVal.String == "open" {
				created = append(created, label)
			} else if newVal.String == "in_progress" {
				started = append(started, label)
			} else if newVal.String == "done" {
				closed = append(closed, label)
			}
		}
	}
	var parts []string
	if len(created) > 0 {
		parts = append(parts, fmt.Sprintf("Created %d items", len(created)))
	}
	if len(started) > 0 {
		parts = append(parts, fmt.Sprintf("Started %d items", len(started)))
	}
	if len(closed) > 0 {
		parts = append(parts, fmt.Sprintf("Closed %d items", len(closed)))
	}
	if len(parts) == 0 {
		return "No activity recorded", nil
	}
	return strings.Join(parts, ". "), nil
}

func (d *DB) SetTaskStatus(id, newStatus, changedBy string) (*models.Task, error) {
	existing, err := d.GetTask(id)
	if err != nil {
		return nil, err
	}

	if existing.Status == newStatus {
		return existing, nil
	}

	if existing.Status == "done" && newStatus != "open" {
		return nil, fmt.Errorf("cannot change status of a closed task")
	}

	now := time.Now().UTC().Format(time.RFC3339)

	// Closing checks
	if newStatus == "done" {
		if err := d.validateClose(id, existing.Type); err != nil {
			return nil, err
		}
		d.conn.Exec(`UPDATE tasks SET status = ?, updated_at = ?, closed_at = ? WHERE id = ?`, newStatus, now, now, id)
	} else if newStatus == "open" && existing.Status == "done" {
		d.conn.Exec(`UPDATE tasks SET status = ?, updated_at = ?, closed_at = NULL WHERE id = ?`, newStatus, now, id)
	} else if newStatus == "in_progress" && existing.StartedAt == nil {
		d.conn.Exec(`UPDATE tasks SET status = ?, updated_at = ?, started_at = ? WHERE id = ?`, newStatus, now, now, id)
	} else {
		d.conn.Exec(`UPDATE tasks SET status = ?, updated_at = ? WHERE id = ?`, newStatus, now, id)
	}

	oldStatus := existing.Status
	d.WriteAudit(id, "status", &oldStatus, &newStatus, changedBy)
	d.touchSession(id)

	return d.GetTask(id)
}

func (d *DB) validateClose(id, taskType string) error {
	// Check all children are done
	var openChildren int
	err := d.conn.QueryRow(
		`SELECT COUNT(*) FROM tasks WHERE parent_id = ? AND status != 'done'`, id,
	).Scan(&openChildren)
	if err != nil {
		return err
	}
	if openChildren > 0 {
		childType := "tasks"
		if taskType == "task" {
			childType = "subtasks"
		}
		return fmt.Errorf("cannot close: %d %s are still open", openChildren, childType)
	}
	return nil
}

func (d *DB) DeleteTask(id string, force bool, changedBy string) error {
	existing, err := d.GetTask(id)
	if err != nil {
		return fmt.Errorf("task not found: %s", id)
	}

	// Without --force, only allow deleting open items
	if !force && existing.Status != "open" {
		return fmt.Errorf("cannot delete %s in status '%s' (use --force to override)", id, existing.Status)
	}

	// Check for children regardless of force
	var childCount int
	err = d.conn.QueryRow(
		`SELECT COUNT(*) FROM tasks WHERE parent_id = ?`, id,
	).Scan(&childCount)
	if err != nil {
		return err
	}
	if childCount > 0 {
		childType := "tasks"
		if existing.Type == "task" {
			childType = "subtasks"
		}
		return fmt.Errorf("cannot delete: %d %s still exist (delete children first)", childCount, childType)
	}

	// Transaction: clean up all references then delete
	tx, err := d.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	tx.Exec(`DELETE FROM dependencies WHERE upstream_id = ? OR downstream_id = ?`, id, id)
	tx.Exec(`DELETE FROM audit WHERE task_id = ?`, id)
	tx.Exec(`DELETE FROM tasks WHERE id = ?`, id)

	if err := tx.Commit(); err != nil {
		return err
	}

	d.touchSession(id)
	return nil
}

func (d *DB) ReparentTask(id, newParentID, changedBy string) (*models.Task, error) {
	existing, err := d.GetTask(id)
	if err != nil {
		return nil, err
	}
	newParent, err := d.GetTask(newParentID)
	if err != nil {
		return nil, fmt.Errorf("new parent not found: %s", newParentID)
	}

	// Validate hierarchy
	switch existing.Type {
	case "task":
		if newParent.Type != "epic" {
			return nil, fmt.Errorf("tasks can only be moved to epics, got %s", newParent.Type)
		}
	case "subtask":
		if newParent.Type != "task" {
			return nil, fmt.Errorf("subtasks can only be moved to tasks, got %s", newParent.Type)
		}
	default:
		return nil, fmt.Errorf("cannot reparent %s items", existing.Type)
	}

	tx, err := d.conn.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Compute new ref
	newRef, err := nextTaskRef(tx, existing.Type, &newParentID)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	oldParentID := ""
	if existing.ParentID != nil {
		oldParentID = *existing.ParentID
	}
	oldRef := existing.Ref

	tx.Exec(`UPDATE tasks SET parent_id = ?, ref = ?, updated_at = ? WHERE id = ?`, newParentID, newRef, now, id)

	// Update children refs recursively
	d.updateChildRefsTx(tx, id, newRef)

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	d.WriteAudit(id, "parent_id", &oldParentID, &newParentID, changedBy)
	d.WriteAudit(id, "ref", &oldRef, &newRef, changedBy)
	d.touchSession(id)

	return d.GetTask(id)
}

func (d *DB) updateChildRefsTx(tx *sql.Tx, parentID, newParentRef string) {
	rows, err := tx.Query(`SELECT id, ref FROM tasks WHERE parent_id = ?`, parentID)
	if err != nil {
		return
	}
	var children []struct{ id, ref string }
	for rows.Next() {
		var c struct{ id, ref string }
		rows.Scan(&c.id, &c.ref)
		children = append(children, c)
	}
	rows.Close()

	for i, child := range children {
		newRef := fmt.Sprintf("%s.%d", newParentRef, i+1)
		tx.Exec(`UPDATE tasks SET ref = ? WHERE id = ?`, newRef, child.id)
		d.updateChildRefsTx(tx, child.id, newRef)
	}
}

func (d *DB) ReadyTasks() ([]models.Task, error) {
	// Tasks (not epics, not subtasks) that are open and have no open blockers
	rows, err := d.conn.Query(`
		SELECT t.id FROM tasks t
		WHERE t.type = 'task'
		AND t.status IN ('open', 'in_progress')
		AND t.archived = 0
		AND NOT EXISTS (
			SELECT 1 FROM dependencies d
			JOIN tasks blocker ON blocker.id = d.upstream_id
			WHERE d.downstream_id = t.id AND blocker.status != 'done'
		)
		ORDER BY t.priority ASC, t.created_at ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []models.Task
	for rows.Next() {
		var id string
		rows.Scan(&id)
		t, _ := d.GetTask(id)
		if t != nil {
			tasks = append(tasks, *t)
		}
	}
	return tasks, nil
}

// --- Archive ---

func (d *DB) ArchiveTask(id, changedBy string) (*models.Task, error) {
	existing, err := d.GetTask(id)
	if err != nil {
		return nil, err
	}
	if existing.Archived {
		return existing, nil
	}

	now := time.Now().UTC().Format(time.RFC3339)
	_, err = d.conn.Exec(`UPDATE tasks SET archived = 1, updated_at = ? WHERE id = ?`, now, id)
	if err != nil {
		return nil, err
	}

	oldVal := "0"
	newVal := "1"
	d.WriteAudit(id, "archived", &oldVal, &newVal, changedBy)
	d.touchSession(id)
	return d.GetTask(id)
}

func (d *DB) UnarchiveTask(id, changedBy string) (*models.Task, error) {
	existing, err := d.GetTask(id)
	if err != nil {
		return nil, err
	}
	if !existing.Archived {
		return existing, nil
	}

	now := time.Now().UTC().Format(time.RFC3339)
	_, err = d.conn.Exec(`UPDATE tasks SET archived = 0, updated_at = ? WHERE id = ?`, now, id)
	if err != nil {
		return nil, err
	}

	oldVal := "1"
	newVal := "0"
	d.WriteAudit(id, "archived", &oldVal, &newVal, changedBy)
	d.touchSession(id)
	return d.GetTask(id)
}

func (d *DB) SearchTasks(query string) ([]models.Task, error) {
	pattern := "%" + query + "%"
	rows, err := d.conn.Query(`
		SELECT id FROM tasks
		WHERE title LIKE ? OR notes LIKE ?
		ORDER BY priority ASC, created_at ASC
	`, pattern, pattern)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []models.Task
	for rows.Next() {
		var id string
		rows.Scan(&id)
		t, err := d.GetTask(id)
		if err != nil {
			continue
		}
		tasks = append(tasks, *t)
	}
	return tasks, nil
}

// --- Dependencies ---

func (d *DB) AddDependency(upstreamID, downstreamID string, changedBy string) error {
	// Verify both exist
	if _, err := d.GetTask(upstreamID); err != nil {
		return err
	}
	if _, err := d.GetTask(downstreamID); err != nil {
		return err
	}

	// Cycle detection
	if d.wouldCreateCycle(upstreamID, downstreamID) {
		return fmt.Errorf("adding this dependency would create a cycle")
	}

	_, err := d.conn.Exec(
		`INSERT INTO dependencies (upstream_id, downstream_id) VALUES (?, ?)`,
		upstreamID, downstreamID,
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") || strings.Contains(err.Error(), "PRIMARY") {
			return fmt.Errorf("dependency already exists")
		}
		return err
	}

	upStr := upstreamID
	d.WriteAudit(downstreamID, "dependency_added", nil, &upStr, changedBy)
	d.touchSession(downstreamID)
	return nil
}

func (d *DB) wouldCreateCycle(upstream, downstream string) bool {
	// DFS from upstream following existing upstream edges to see if we can reach downstream
	visited := map[string]bool{}
	var dfs func(string) bool
	dfs = func(node string) bool {
		if node == downstream {
			return true
		}
		if visited[node] {
			return false
		}
		visited[node] = true
		rows, _ := d.conn.Query(`SELECT upstream_id FROM dependencies WHERE downstream_id = ?`, node)
		if rows == nil {
			return false
		}
		defer rows.Close()
		for rows.Next() {
			var next string
			rows.Scan(&next)
			if dfs(next) {
				return true
			}
		}
		return false
	}
	return dfs(upstream)
}

func (d *DB) RemoveDependency(upstreamID, downstreamID string, changedBy string) error {
	res, err := d.conn.Exec(
		`DELETE FROM dependencies WHERE upstream_id = ? AND downstream_id = ?`,
		upstreamID, downstreamID,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("dependency not found")
	}
	upStr := upstreamID
	d.WriteAudit(downstreamID, "dependency_removed", &upStr, nil, changedBy)
	d.touchSession(downstreamID)
	return nil
}

func (d *DB) GetDependencies(id string) (*models.DepInfo, error) {
	if _, err := d.GetTask(id); err != nil {
		return nil, err
	}

	info := &models.DepInfo{
		BlockedBy: []string{},
		Blocks:    []string{},
	}

	rows, _ := d.conn.Query(`SELECT upstream_id FROM dependencies WHERE downstream_id = ?`, id)
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var uid string
			rows.Scan(&uid)
			info.BlockedBy = append(info.BlockedBy, uid)
		}
	}

	rows2, _ := d.conn.Query(`SELECT downstream_id FROM dependencies WHERE upstream_id = ?`, id)
	if rows2 != nil {
		defer rows2.Close()
		for rows2.Next() {
			var did string
			rows2.Scan(&did)
			info.Blocks = append(info.Blocks, did)
		}
	}

	return info, nil
}

func (d *DB) ListChildren(parentID string) ([]models.Task, error) {
	rows, err := d.conn.Query(`SELECT id FROM tasks WHERE parent_id = ? ORDER BY created_at ASC, id ASC`, parentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var children []models.Task
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		child, err := d.GetTask(id)
		if err != nil {
			continue
		}
		children = append(children, *child)
	}
	return children, nil
}

// --- Decisions ---

func (d *DB) CreateDecision(summary string, rationale *string, createdBy string) (*models.Decision, error) {
	id, err := d.GenerateID("dc-", summary)
	if err != nil {
		return nil, err
	}

	_, err = d.conn.Exec(
		`INSERT INTO decisions (id, summary, rationale, created_by) VALUES (?, ?, ?, ?)`,
		id, summary, rationale, createdBy,
	)
	if err != nil {
		return nil, err
	}

	return d.GetDecision(id)
}

func (d *DB) GetDecision(id string) (*models.Decision, error) {
	dec := &models.Decision{}
	var createdAt string
	err := d.conn.QueryRow(
		`SELECT id, summary, rationale, created_at, created_by FROM decisions WHERE id = ?`, id,
	).Scan(&dec.ID, &dec.Summary, &dec.Rationale, &createdAt, &dec.CreatedBy)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("decision not found: %s", id)
		}
		return nil, err
	}
	dec.CreatedAt = parseTime(createdAt)
	return dec, nil
}

func (d *DB) ListDecisions() ([]models.Decision, error) {
	rows, err := d.conn.Query(`SELECT id FROM decisions ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var decs []models.Decision
	for rows.Next() {
		var id string
		rows.Scan(&id)
		dec, _ := d.GetDecision(id)
		if dec != nil {
			decs = append(decs, *dec)
		}
	}
	return decs, nil
}

// --- Sessions ---

func (d *DB) CreateSession() (*models.Session, error) {
	id, err := d.GenerateID("ss-", "session")
	if err != nil {
		return nil, err
	}

	_, err = d.conn.Exec(`INSERT INTO sessions (id) VALUES (?)`, id)
	if err != nil {
		return nil, err
	}

	return d.GetSession(id)
}

func (d *DB) EndSession(id string, summary *string) (*models.Session, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := d.conn.Exec(
		`UPDATE sessions SET ended_at = ?, summary = ? WHERE id = ?`,
		now, summary, id,
	)
	if err != nil {
		return nil, err
	}
	return d.GetSession(id)
}

func (d *DB) GetSession(id string) (*models.Session, error) {
	s := &models.Session{}
	var startedAt string
	var endedAt sql.NullString
	var tasksTouched string
	err := d.conn.QueryRow(
		`SELECT id, started_at, ended_at, summary, tasks_touched FROM sessions WHERE id = ?`, id,
	).Scan(&s.ID, &startedAt, &endedAt, &s.Summary, &tasksTouched)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("session not found: %s", id)
		}
		return nil, err
	}
	s.StartedAt = parseTime(startedAt)
	if endedAt.Valid {
		et := parseTime(endedAt.String)
		s.EndedAt = &et
	}
	json.Unmarshal([]byte(tasksTouched), &s.TasksTouched)
	if s.TasksTouched == nil {
		s.TasksTouched = []string{}
	}
	return s, nil
}

func (d *DB) ListSessions() ([]models.Session, error) {
	rows, err := d.conn.Query(`SELECT id FROM sessions ORDER BY started_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []models.Session
	for rows.Next() {
		var id string
		rows.Scan(&id)
		s, _ := d.GetSession(id)
		if s != nil {
			sessions = append(sessions, *s)
		}
	}
	return sessions, nil
}

func (d *DB) touchSession(taskID string) {
	// Read active session from lockfile
	werkDir := filepath.Dir(d.path)
	lockPath := filepath.Join(werkDir, "session.lock")
	data, err := os.ReadFile(lockPath)
	if err != nil {
		return // No active session
	}
	sessionID := strings.TrimSpace(string(data))
	if sessionID == "" {
		return
	}

	// Read current tasks_touched
	var tasksTouched string
	err = d.conn.QueryRow(`SELECT tasks_touched FROM sessions WHERE id = ?`, sessionID).Scan(&tasksTouched)
	if err != nil {
		return
	}

	var touched []string
	json.Unmarshal([]byte(tasksTouched), &touched)

	// Dedupe
	for _, t := range touched {
		if t == taskID {
			return
		}
	}
	touched = append(touched, taskID)

	data2, _ := json.Marshal(touched)
	d.conn.Exec(`UPDATE sessions SET tasks_touched = ? WHERE id = ?`, string(data2), sessionID)
}

// --- Audit ---

func (d *DB) GetAudit(taskID string) ([]models.AuditEntry, error) {
	// Verify task exists
	if _, err := d.GetTask(taskID); err != nil {
		return nil, err
	}

	rows, err := d.conn.Query(
		`SELECT id, task_id, field, old_value, new_value, changed_at, changed_by FROM audit WHERE task_id = ? ORDER BY changed_at ASC, id ASC`,
		taskID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []models.AuditEntry
	for rows.Next() {
		e := models.AuditEntry{}
		var changedAt string
		rows.Scan(&e.ID, &e.TaskID, &e.Field, &e.OldValue, &e.NewValue, &changedAt, &e.ChangedBy)
		e.ChangedAt = parseTime(changedAt)
		entries = append(entries, e)
	}
	return entries, nil
}

func (d *DB) BuildCompactHandoff(idOrRef string) (*models.CompactHandoff, error) {
	task, err := d.GetTaskByIDOrRef(idOrRef)
	if err != nil {
		return nil, err
	}

	depInfo, err := d.GetDependencies(task.ID)
	if err != nil {
		return nil, err
	}

	children, err := d.ListChildren(task.ID)
	if err != nil {
		return nil, err
	}

	decisions, err := d.ListDecisions()
	if err != nil {
		return nil, err
	}
	if len(decisions) > 5 {
		decisions = decisions[len(decisions)-5:]
	}

	audit, err := d.GetAudit(task.ID)
	if err != nil {
		return nil, err
	}
	if len(audit) > 10 {
		audit = audit[len(audit)-10:]
	}

	packet := &models.CompactHandoff{
		Item:         task.ToJSON(),
		Dependencies: models.HandoffDependencies{BlockedBy: []models.HandoffRelation{}, Blocks: []models.HandoffRelation{}},
		Children:     []models.TaskJSON{},
		Decisions:    decisions,
		RecentAudit:  audit,
	}

	for _, blockedByID := range depInfo.BlockedBy {
		t, err := d.GetTask(blockedByID)
		if err != nil {
			continue
		}
		packet.Dependencies.BlockedBy = append(packet.Dependencies.BlockedBy, models.HandoffRelation{
			ID:     t.ID,
			Ref:    t.Ref,
			Type:   t.Type,
			Title:  t.Title,
			Status: t.Status,
		})
	}
	for _, blocksID := range depInfo.Blocks {
		t, err := d.GetTask(blocksID)
		if err != nil {
			continue
		}
		packet.Dependencies.Blocks = append(packet.Dependencies.Blocks, models.HandoffRelation{
			ID:     t.ID,
			Ref:    t.Ref,
			Type:   t.Type,
			Title:  t.Title,
			Status: t.Status,
		})
	}
	sort.Slice(packet.Dependencies.BlockedBy, func(i, j int) bool {
		return packet.Dependencies.BlockedBy[i].Ref < packet.Dependencies.BlockedBy[j].Ref
	})
	sort.Slice(packet.Dependencies.Blocks, func(i, j int) bool { return packet.Dependencies.Blocks[i].Ref < packet.Dependencies.Blocks[j].Ref })

	for _, child := range children {
		packet.Children = append(packet.Children, child.ToJSON())
	}

	return packet, nil
}

// --- Status ---

func (d *DB) GetStatus() (*models.StatusSummary, error) {
	s := &models.StatusSummary{}
	d.conn.QueryRow(`SELECT COUNT(*) FROM tasks WHERE status = 'open'`).Scan(&s.Open)
	d.conn.QueryRow(`SELECT COUNT(*) FROM tasks WHERE status = 'in_progress'`).Scan(&s.InProgress)
	d.conn.QueryRow(`SELECT COUNT(*) FROM tasks WHERE status = 'blocked'`).Scan(&s.Blocked)
	d.conn.QueryRow(`SELECT COUNT(*) FROM tasks WHERE status = 'done'`).Scan(&s.Done)
	d.conn.QueryRow(`SELECT COUNT(*) FROM decisions`).Scan(&s.Decisions)
	d.conn.QueryRow(`SELECT COUNT(*) FROM sessions`).Scan(&s.Sessions)
	// Active session: most recent session with no ended_at
	var activeID sql.NullString
	d.conn.QueryRow(`SELECT id FROM sessions WHERE ended_at IS NULL ORDER BY started_at DESC LIMIT 1`).Scan(&activeID)
	if activeID.Valid {
		s.ActiveSessionID = &activeID.String
	}
	return s, nil
}

// --- Log ---

func (d *DB) GetLog(limit int, verbose bool) ([]models.LogEntry, error) {
	// Union of high-signal events: status changes, decisions, session start/end
	query := `
		SELECT timestamp, event, id, title, detail FROM (
			-- Task/epic/subtask status changes (started, closed, blocked)
			SELECT a.changed_at AS timestamp,
				CASE a.new_value
					WHEN 'in_progress' THEN 'started'
					WHEN 'done' THEN 'closed'
					WHEN 'blocked' THEN 'blocked'
				END AS event,
				t.id AS id,
				t.type || ': ' || t.title AS title,
				CASE WHEN ? THEN t.notes ELSE NULL END AS detail
			FROM audit a
			JOIN tasks t ON a.task_id = t.id
			WHERE a.field = 'status' AND a.new_value IN ('in_progress', 'done', 'blocked')

			UNION ALL

			-- Task/epic/subtask creation
			SELECT a.changed_at AS timestamp,
				'created' AS event,
				t.id AS id,
				t.type || ': ' || t.title AS title,
				CASE WHEN ? THEN t.notes ELSE NULL END AS detail
			FROM audit a
			JOIN tasks t ON a.task_id = t.id
			WHERE a.field = 'status' AND a.old_value IS NULL AND a.new_value = 'open'

			UNION ALL

			-- Decisions
			SELECT d.created_at AS timestamp,
				'decision' AS event,
				d.id AS id,
				d.summary AS title,
				CASE WHEN ? THEN d.rationale ELSE NULL END AS detail
			FROM decisions d

			UNION ALL

			-- Session starts
			SELECT s.started_at AS timestamp,
				'session_start' AS event,
				s.id AS id,
				'Session started' AS title,
				NULL AS detail
			FROM sessions s

			UNION ALL

			-- Session ends (with summary)
			SELECT s.ended_at AS timestamp,
				'session_end' AS event,
				s.id AS id,
				COALESCE(s.summary, 'Session ended') AS title,
				CASE WHEN ? THEN s.tasks_touched ELSE NULL END AS detail
			FROM sessions s
			WHERE s.ended_at IS NOT NULL
		)
		ORDER BY timestamp DESC
		LIMIT ?
	`

	rows, err := d.conn.Query(query, verbose, verbose, verbose, verbose, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []models.LogEntry
	for rows.Next() {
		e := models.LogEntry{}
		var ts string
		rows.Scan(&ts, &e.Event, &e.ID, &e.Title, &e.Detail)
		e.Timestamp = parseTime(ts)
		entries = append(entries, e)
	}
	return entries, nil
}

// --- Export / Import ---

func (d *DB) ExportAll() (*models.ExportPayload, error) {
	payload := &models.ExportPayload{
		SchemaVersion: 1,
		ExportedAt:    time.Now().UTC(),
	}

	// Tasks
	rows, err := d.conn.Query(`SELECT id, ref, parent_id, type, title, status, priority, notes, created_at, updated_at, closed_at FROM tasks ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var t models.TaskExport
		var ref, createdAt sql.NullString
		var updatedAt, closedAt sql.NullString
		rows.Scan(&t.ID, &ref, &t.ParentID, &t.Type, &t.Title, &t.Status, &t.Priority, &t.Notes, &createdAt, &updatedAt, &closedAt)
		if ref.Valid {
			t.Ref = ref.String
		}
		t.CreatedAt = parseTime(createdAt.String)
		if updatedAt.Valid {
			ut := parseTime(updatedAt.String)
			t.UpdatedAt = &ut
		}
		if closedAt.Valid {
			ct := parseTime(closedAt.String)
			t.ClosedAt = &ct
		}
		payload.Tasks = append(payload.Tasks, t)
	}
	if payload.Tasks == nil {
		payload.Tasks = []models.TaskExport{}
	}

	// Dependencies
	depRows, err := d.conn.Query(`SELECT upstream_id, downstream_id FROM dependencies`)
	if err != nil {
		return nil, err
	}
	defer depRows.Close()
	for depRows.Next() {
		var dep models.Dependency
		depRows.Scan(&dep.UpstreamID, &dep.DownstreamID)
		payload.Dependencies = append(payload.Dependencies, dep)
	}
	if payload.Dependencies == nil {
		payload.Dependencies = []models.Dependency{}
	}

	// Decisions
	decRows, err := d.conn.Query(`SELECT id, summary, rationale, created_at, created_by FROM decisions ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer decRows.Close()
	for decRows.Next() {
		var dec models.Decision
		var createdAt string
		decRows.Scan(&dec.ID, &dec.Summary, &dec.Rationale, &createdAt, &dec.CreatedBy)
		dec.CreatedAt = parseTime(createdAt)
		payload.Decisions = append(payload.Decisions, dec)
	}
	if payload.Decisions == nil {
		payload.Decisions = []models.Decision{}
	}

	// Sessions
	sessRows, err := d.conn.Query(`SELECT id, started_at, ended_at, summary, tasks_touched FROM sessions ORDER BY started_at ASC`)
	if err != nil {
		return nil, err
	}
	defer sessRows.Close()
	for sessRows.Next() {
		var s models.Session
		var startedAt string
		var endedAt sql.NullString
		var tasksTouched string
		sessRows.Scan(&s.ID, &startedAt, &endedAt, &s.Summary, &tasksTouched)
		s.StartedAt = parseTime(startedAt)
		if endedAt.Valid {
			et := parseTime(endedAt.String)
			s.EndedAt = &et
		}
		json.Unmarshal([]byte(tasksTouched), &s.TasksTouched)
		if s.TasksTouched == nil {
			s.TasksTouched = []string{}
		}
		payload.Sessions = append(payload.Sessions, s)
	}
	if payload.Sessions == nil {
		payload.Sessions = []models.Session{}
	}

	// Audit
	auditRows, err := d.conn.Query(`SELECT id, task_id, field, old_value, new_value, changed_at, changed_by FROM audit ORDER BY changed_at ASC, id ASC`)
	if err != nil {
		return nil, err
	}
	defer auditRows.Close()
	for auditRows.Next() {
		var e models.AuditEntry
		var changedAt string
		auditRows.Scan(&e.ID, &e.TaskID, &e.Field, &e.OldValue, &e.NewValue, &changedAt, &e.ChangedBy)
		e.ChangedAt = parseTime(changedAt)
		payload.Audit = append(payload.Audit, e)
	}
	if payload.Audit == nil {
		payload.Audit = []models.AuditEntry{}
	}

	return payload, nil
}

func (d *DB) ImportAll(payload *models.ExportPayload) error {
	// Disable foreign keys during import to allow any insert order
	d.conn.Exec(`PRAGMA foreign_keys = OFF`)
	defer d.conn.Exec(`PRAGMA foreign_keys = ON`)

	tx, err := d.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, t := range payload.Tasks {
		_, err := tx.Exec(
			`INSERT OR IGNORE INTO tasks (id, ref, parent_id, type, title, status, priority, notes, created_at, updated_at, closed_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			t.ID, t.Ref, t.ParentID, t.Type, t.Title, t.Status, t.Priority, t.Notes,
			t.CreatedAt.UTC().Format(time.RFC3339), nilTimeStr(t.UpdatedAt), nilTimeStr(t.ClosedAt),
		)
		if err != nil {
			return fmt.Errorf("importing task %s: %w", t.ID, err)
		}
	}

	for _, dep := range payload.Dependencies {
		tx.Exec(`INSERT OR IGNORE INTO dependencies (upstream_id, downstream_id) VALUES (?, ?)`, dep.UpstreamID, dep.DownstreamID)
	}

	for _, dec := range payload.Decisions {
		tx.Exec(`INSERT OR IGNORE INTO decisions (id, summary, rationale, created_at, created_by) VALUES (?, ?, ?, ?, ?)`,
			dec.ID, dec.Summary, dec.Rationale, dec.CreatedAt.UTC().Format(time.RFC3339), dec.CreatedBy)
	}

	for _, s := range payload.Sessions {
		tt, _ := json.Marshal(s.TasksTouched)
		tx.Exec(`INSERT OR IGNORE INTO sessions (id, started_at, ended_at, summary, tasks_touched) VALUES (?, ?, ?, ?, ?)`,
			s.ID, s.StartedAt.UTC().Format(time.RFC3339), nilTimeStr(s.EndedAt), s.Summary, string(tt))
	}

	for _, e := range payload.Audit {
		tx.Exec(`INSERT OR IGNORE INTO audit (id, task_id, field, old_value, new_value, changed_at, changed_by) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			e.ID, e.TaskID, e.Field, e.OldValue, e.NewValue, e.ChangedAt.UTC().Format(time.RFC3339), e.ChangedBy)
	}

	return tx.Commit()
}

// --- Audit queries ---

func (d *DB) GetAuditSince(since time.Time) ([]models.AuditEntry, error) {
	rows, err := d.conn.Query(
		`SELECT id, task_id, field, old_value, new_value, changed_at, changed_by FROM audit WHERE changed_at >= ? ORDER BY changed_at ASC, id ASC`,
		since.UTC().Format("2006-01-02 15:04:05"),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []models.AuditEntry
	for rows.Next() {
		e := models.AuditEntry{}
		var changedAt string
		rows.Scan(&e.ID, &e.TaskID, &e.Field, &e.OldValue, &e.NewValue, &changedAt, &e.ChangedBy)
		e.ChangedAt = parseTime(changedAt)
		entries = append(entries, e)
	}
	return entries, nil
}

// --- Helpers ---

func nilTimeStr(t *time.Time) interface{} {
	if t == nil {
		return nil
	}
	return t.UTC().Format(time.RFC3339)
}

func parseTime(s string) time.Time {
	// Try multiple formats since SQLite can store in different formats
	for _, layout := range []string{
		time.RFC3339,
		"2006-01-02T15:04:05Z",
		"2006-01-02 15:04:05",
	} {
		t, err := time.Parse(layout, s)
		if err == nil {
			return t
		}
	}
	return time.Time{}
}
