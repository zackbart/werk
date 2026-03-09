package db

import (
	"path/filepath"
	"testing"
)

func openTestDB(t *testing.T) *DB {
	t.Helper()
	d, err := Open(filepath.Join(t.TempDir(), "tasks.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	return d
}

func TestResolveTaskIDByID(t *testing.T) {
	d := openTestDB(t)

	epic, err := d.CreateTask("epic", "Epic", nil, 2, nil, "agent")
	if err != nil {
		t.Fatalf("create epic: %v", err)
	}
	task, err := d.CreateTask("task", "Task", &epic.ID, 2, nil, "agent")
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	gotByID, err := d.ResolveTaskID(task.ID)
	if err != nil {
		t.Fatalf("resolve by id: %v", err)
	}
	if gotByID != task.ID {
		t.Fatalf("resolve by id mismatch: got %q want %q", gotByID, task.ID)
	}
}

func TestResolveNonexistent(t *testing.T) {
	d := openTestDB(t)

	_, err := d.ResolveTaskID("tk-nonexistent")
	if err == nil {
		t.Fatal("expected error resolving nonexistent id")
	}
}

func TestCompactHandoffIncludesCoreSections(t *testing.T) {
	d := openTestDB(t)

	epic, err := d.CreateTask("epic", "Epic", nil, 2, nil, "agent")
	if err != nil {
		t.Fatalf("create epic: %v", err)
	}
	taskA, err := d.CreateTask("task", "Task A", &epic.ID, 2, nil, "agent")
	if err != nil {
		t.Fatalf("create taskA: %v", err)
	}
	taskB, err := d.CreateTask("task", "Task B", &epic.ID, 2, nil, "agent")
	if err != nil {
		t.Fatalf("create taskB: %v", err)
	}
	_, err = d.CreateTask("subtask", "Subtask", &taskA.ID, 2, nil, "agent")
	if err != nil {
		t.Fatalf("create subtask: %v", err)
	}
	if err := d.AddDependency(taskB.ID, taskA.ID, "agent"); err != nil {
		t.Fatalf("add dep: %v", err)
	}
	if _, err := d.CreateDecision("Use compact handoff", nil, "agent"); err != nil {
		t.Fatalf("create decision: %v", err)
	}

	packet, err := d.BuildCompactHandoff(taskA.ID)
	if err != nil {
		t.Fatalf("build handoff: %v", err)
	}
	if packet.Item.ID != taskA.ID {
		t.Fatalf("unexpected item identity: %#v", packet.Item)
	}
	if len(packet.Dependencies.BlockedBy) == 0 {
		t.Fatalf("expected blocked_by entries")
	}
	if len(packet.Children) == 0 {
		t.Fatalf("expected child entries")
	}
	if len(packet.Decisions) == 0 {
		t.Fatalf("expected decision entries")
	}
	if len(packet.RecentAudit) == 0 {
		t.Fatalf("expected recent audit entries")
	}
}
