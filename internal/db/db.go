package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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

	_, err = d.conn.Exec(
		`INSERT INTO tasks (id, parent_id, type, title, priority, notes) VALUES (?, ?, ?, ?, ?, ?)`,
		id, parentID, taskType, title, priority, notes,
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
	var createdAt, updatedAt, closedAt sql.NullString
	err := d.conn.QueryRow(
		`SELECT id, parent_id, type, title, status, priority, notes, created_at, updated_at, closed_at FROM tasks WHERE id = ?`, id,
	).Scan(&t.ID, &t.ParentID, &t.Type, &t.Title, &t.Status, &t.Priority, &t.Notes, &createdAt, &updatedAt, &closedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("task not found: %s", id)
		}
		return nil, err
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

func (d *DB) ListTasks(taskType string, parentID *string, status string) ([]models.Task, error) {
	query := `SELECT id FROM tasks WHERE type = ?`
	args := []interface{}{taskType}

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

func (d *DB) SetTaskStatus(id, newStatus, changedBy string) (*models.Task, error) {
	existing, err := d.GetTask(id)
	if err != nil {
		return nil, err
	}

	if existing.Status == newStatus {
		return existing, nil
	}

	if existing.Status == "done" {
		return nil, fmt.Errorf("cannot change status of a closed task")
	}

	now := time.Now().UTC().Format(time.RFC3339)

	// Closing checks
	if newStatus == "done" {
		if err := d.validateClose(id, existing.Type); err != nil {
			return nil, err
		}
		d.conn.Exec(`UPDATE tasks SET status = ?, updated_at = ?, closed_at = ? WHERE id = ?`, newStatus, now, now, id)
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

func (d *DB) ReadyTasks() ([]models.Task, error) {
	// Tasks (not epics, not subtasks) that are open and have no open blockers
	rows, err := d.conn.Query(`
		SELECT t.id FROM tasks t
		WHERE t.type = 'task'
		AND t.status IN ('open', 'in_progress')
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

// --- Status ---

func (d *DB) GetStatus() (*models.StatusSummary, error) {
	s := &models.StatusSummary{}
	d.conn.QueryRow(`SELECT COUNT(*) FROM tasks WHERE status = 'open'`).Scan(&s.Open)
	d.conn.QueryRow(`SELECT COUNT(*) FROM tasks WHERE status = 'in_progress'`).Scan(&s.InProgress)
	d.conn.QueryRow(`SELECT COUNT(*) FROM tasks WHERE status = 'blocked'`).Scan(&s.Blocked)
	d.conn.QueryRow(`SELECT COUNT(*) FROM tasks WHERE status = 'done'`).Scan(&s.Done)
	d.conn.QueryRow(`SELECT COUNT(*) FROM decisions`).Scan(&s.Decisions)
	d.conn.QueryRow(`SELECT COUNT(*) FROM sessions`).Scan(&s.Sessions)
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

// --- Helpers ---

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
