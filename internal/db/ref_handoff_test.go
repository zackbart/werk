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

func TestCreateTaskAssignsDottedRefs(t *testing.T) {
	d := openTestDB(t)

	epic1, err := d.CreateTask("epic", "Epic 1", nil, 2, nil, "agent")
	if err != nil {
		t.Fatalf("create epic1: %v", err)
	}
	epic2, err := d.CreateTask("epic", "Epic 2", nil, 2, nil, "agent")
	if err != nil {
		t.Fatalf("create epic2: %v", err)
	}
	if epic1.Ref != "1" {
		t.Fatalf("expected epic1 ref 1, got %q", epic1.Ref)
	}
	if epic2.Ref != "2" {
		t.Fatalf("expected epic2 ref 2, got %q", epic2.Ref)
	}

	task1, err := d.CreateTask("task", "Task 1", &epic1.ID, 2, nil, "agent")
	if err != nil {
		t.Fatalf("create task1: %v", err)
	}
	task2, err := d.CreateTask("task", "Task 2", &epic1.ID, 2, nil, "agent")
	if err != nil {
		t.Fatalf("create task2: %v", err)
	}
	if task1.Ref != "1.1" {
		t.Fatalf("expected task1 ref 1.1, got %q", task1.Ref)
	}
	if task2.Ref != "1.2" {
		t.Fatalf("expected task2 ref 1.2, got %q", task2.Ref)
	}

	subtask, err := d.CreateTask("subtask", "Subtask 1", &task1.ID, 2, nil, "agent")
	if err != nil {
		t.Fatalf("create subtask: %v", err)
	}
	if subtask.Ref != "1.1.1" {
		t.Fatalf("expected subtask ref 1.1.1, got %q", subtask.Ref)
	}
}

func TestResolveTaskIDByIDOrRef(t *testing.T) {
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

	gotByRef, err := d.ResolveTaskID(task.Ref)
	if err != nil {
		t.Fatalf("resolve by ref: %v", err)
	}
	if gotByRef != task.ID {
		t.Fatalf("resolve by ref mismatch: got %q want %q", gotByRef, task.ID)
	}

	loaded, err := d.GetTask(task.ID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if loaded.ParentRef == nil || *loaded.ParentRef != epic.Ref {
		t.Fatalf("expected parent ref %q, got %#v", epic.Ref, loaded.ParentRef)
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

	packet, err := d.BuildCompactHandoff(taskA.Ref)
	if err != nil {
		t.Fatalf("build handoff: %v", err)
	}
	if packet.Item.ID != taskA.ID || packet.Item.Ref != taskA.Ref {
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
