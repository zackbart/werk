package db

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func TestMigrationBackfillsRefsAndCounters(t *testing.T) {
	path := t.TempDir() + "/legacy.db"

	conn, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open legacy db: %v", err)
	}
	_, err = conn.Exec(`
CREATE TABLE tasks (
  id TEXT PRIMARY KEY,
  parent_id TEXT REFERENCES tasks(id),
  type TEXT NOT NULL,
  title TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'open',
  priority INTEGER NOT NULL DEFAULT 2,
  notes TEXT,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME,
  closed_at DATETIME
);
CREATE TABLE dependencies (
  upstream_id TEXT NOT NULL,
  downstream_id TEXT NOT NULL,
  PRIMARY KEY (upstream_id, downstream_id)
);
CREATE TABLE audit (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  task_id TEXT NOT NULL,
  field TEXT NOT NULL,
  old_value TEXT,
  new_value TEXT,
  changed_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  changed_by TEXT NOT NULL
);
CREATE TABLE decisions (
  id TEXT PRIMARY KEY,
  summary TEXT NOT NULL,
  rationale TEXT,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  created_by TEXT NOT NULL
);
CREATE TABLE sessions (
  id TEXT PRIMARY KEY,
  started_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  ended_at DATETIME,
  summary TEXT,
  tasks_touched TEXT DEFAULT '[]'
);
INSERT INTO tasks (id, parent_id, type, title, status, priority, created_at) VALUES
  ('ep-111111', NULL, 'epic', 'Legacy Epic', 'open', 2, '2026-01-01T00:00:00Z'),
  ('tk-111111', 'ep-111111', 'task', 'Legacy Task 1', 'open', 2, '2026-01-01T01:00:00Z'),
  ('tk-222222', 'ep-111111', 'task', 'Legacy Task 2', 'open', 2, '2026-01-01T02:00:00Z');
`)
	if err != nil {
		t.Fatalf("seed legacy db: %v", err)
	}
	_ = conn.Close()

	d, err := Open(path)
	if err != nil {
		t.Fatalf("migrate open: %v", err)
	}
	defer d.Close()

	epic, _ := d.GetTask("ep-111111")
	task1, _ := d.GetTask("tk-111111")
	task2, _ := d.GetTask("tk-222222")

	if epic.Ref != "1" {
		t.Fatalf("expected epic ref 1, got %s", epic.Ref)
	}
	if task1.Ref != "1.1" || task2.Ref != "1.2" {
		t.Fatalf("unexpected task refs after backfill: %s, %s", task1.Ref, task2.Ref)
	}

	task3, err := d.CreateTask("task", "Post-migration task", &epic.ID, 2, nil, "human")
	if err != nil {
		t.Fatalf("create post-migration task: %v", err)
	}
	if task3.Ref != "1.3" {
		t.Fatalf("expected next ref 1.3, got %s", task3.Ref)
	}
}
