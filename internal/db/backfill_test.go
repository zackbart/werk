package db

import (
	"path/filepath"
	"testing"
)

func TestBackfillAssignsRefsToExistingTasks(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "tasks.db")

	// Open DB (runs migrations including backfill on empty DB).
	d, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	// Insert tasks WITHOUT refs to simulate pre-v0.2 data.
	d.conn.Exec(`INSERT INTO tasks (id, type, title, status, priority) VALUES ('ep-aaa', 'epic', 'Epic A', 'open', 2)`)
	d.conn.Exec(`INSERT INTO tasks (id, type, title, status, priority) VALUES ('ep-bbb', 'epic', 'Epic B', 'open', 2)`)
	d.conn.Exec(`INSERT INTO tasks (id, parent_id, type, title, status, priority) VALUES ('tk-aaa', 'ep-aaa', 'task', 'Task A1', 'open', 2)`)
	d.conn.Exec(`INSERT INTO tasks (id, parent_id, type, title, status, priority) VALUES ('tk-bbb', 'ep-aaa', 'task', 'Task A2', 'open', 2)`)
	d.conn.Exec(`INSERT INTO tasks (id, parent_id, type, title, status, priority) VALUES ('st-aaa', 'tk-aaa', 'subtask', 'Sub 1', 'open', 2)`)

	// Verify refs are NULL.
	var refCount int
	d.conn.QueryRow(`SELECT COUNT(*) FROM tasks WHERE ref IS NOT NULL AND ref != ''`).Scan(&refCount)
	if refCount != 0 {
		t.Fatalf("expected 0 refs before backfill, got %d", refCount)
	}

	// Run backfill.
	if err := d.backfillMissingRefs(); err != nil {
		t.Fatalf("backfill: %v", err)
	}

	// Verify all tasks now have refs.
	d.conn.QueryRow(`SELECT COUNT(*) FROM tasks WHERE ref IS NULL OR ref = ''`).Scan(&refCount)
	if refCount != 0 {
		t.Fatalf("expected 0 tasks without refs after backfill, got %d", refCount)
	}

	// Verify epic refs are sequential integers.
	assertRef(t, d, "ep-aaa", "1")
	assertRef(t, d, "ep-bbb", "2")

	// Verify task refs are dotted under their epic.
	assertRef(t, d, "tk-aaa", "1.1")
	assertRef(t, d, "tk-bbb", "1.2")

	// Verify subtask ref.
	assertRef(t, d, "st-aaa", "1.1.1")

	d.Close()
}

func TestBackfillIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "tasks.db")

	d, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer d.Close()

	// Insert task without ref and backfill.
	d.conn.Exec(`INSERT INTO tasks (id, type, title, status, priority) VALUES ('ep-xxx', 'epic', 'Epic X', 'open', 2)`)
	if err := d.backfillMissingRefs(); err != nil {
		t.Fatalf("first backfill: %v", err)
	}

	assertRef(t, d, "ep-xxx", "1")

	// Run backfill again — should not change anything or error.
	if err := d.backfillMissingRefs(); err != nil {
		t.Fatalf("second backfill: %v", err)
	}

	assertRef(t, d, "ep-xxx", "1")
}

func TestBackfillSkipsAlreadyAssignedRefs(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "tasks.db")

	d, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer d.Close()

	// Create a task with a ref via normal path.
	epic, err := d.CreateTask("epic", "Has Ref", nil, 2, nil, "agent")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	originalRef := epic.Ref

	// Insert one without a ref.
	d.conn.Exec(`INSERT INTO tasks (id, type, title, status, priority) VALUES ('ep-noref', 'epic', 'No Ref', 'open', 2)`)

	if err := d.backfillMissingRefs(); err != nil {
		t.Fatalf("backfill: %v", err)
	}

	// Original should keep its ref.
	assertRef(t, d, epic.ID, originalRef)
	// New one should get the next ref.
	assertRef(t, d, "ep-noref", "2")
}

func TestResolveTaskRef(t *testing.T) {
	d := openTestDB(t)

	epic, err := d.CreateTask("epic", "Epic", nil, 2, nil, "agent")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Resolve by ID.
	ref, err := d.ResolveTaskRef(epic.ID)
	if err != nil {
		t.Fatalf("resolve ref by id: %v", err)
	}
	if ref != epic.Ref {
		t.Fatalf("expected ref %q, got %q", epic.Ref, ref)
	}

	// Resolve by ref.
	ref2, err := d.ResolveTaskRef(epic.Ref)
	if err != nil {
		t.Fatalf("resolve ref by ref: %v", err)
	}
	if ref2 != epic.Ref {
		t.Fatalf("expected ref %q, got %q", epic.Ref, ref2)
	}
}

func TestResolveNonexistent(t *testing.T) {
	d := openTestDB(t)

	_, err := d.ResolveTaskID("nonexistent")
	if err == nil {
		t.Fatal("expected error resolving nonexistent id")
	}

	_, err = d.ResolveTaskRef("99.99")
	if err == nil {
		t.Fatal("expected error resolving nonexistent ref")
	}
}

func assertRef(t *testing.T, d *DB, taskID, expectedRef string) {
	t.Helper()
	var ref string
	err := d.conn.QueryRow(`SELECT ref FROM tasks WHERE id = ?`, taskID).Scan(&ref)
	if err != nil {
		t.Fatalf("query ref for %s: %v", taskID, err)
	}
	if ref != expectedRef {
		t.Fatalf("expected ref %q for %s, got %q", expectedRef, taskID, ref)
	}
}
