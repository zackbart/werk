package db

import (
	"testing"
)

func openTestDB(t *testing.T) *DB {
	t.Helper()
	path := t.TempDir() + "/tasks.db"
	d, err := Open(path)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	return d
}

func TestCreateTaskAssignsDottedRefs(t *testing.T) {
	d := openTestDB(t)

	epic1, err := d.CreateTask("epic", "Epic 1", nil, 2, nil, "human")
	if err != nil {
		t.Fatalf("create epic1: %v", err)
	}
	epic2, err := d.CreateTask("epic", "Epic 2", nil, 2, nil, "human")
	if err != nil {
		t.Fatalf("create epic2: %v", err)
	}
	task1, err := d.CreateTask("task", "Task 1", &epic1.ID, 2, nil, "human")
	if err != nil {
		t.Fatalf("create task1: %v", err)
	}
	task2, err := d.CreateTask("task", "Task 2", &epic1.ID, 2, nil, "human")
	if err != nil {
		t.Fatalf("create task2: %v", err)
	}
	subtask, err := d.CreateTask("subtask", "Subtask 1", &task2.ID, 2, nil, "human")
	if err != nil {
		t.Fatalf("create subtask: %v", err)
	}

	if epic1.Ref != "1" || epic2.Ref != "2" {
		t.Fatalf("unexpected epic refs: %q, %q", epic1.Ref, epic2.Ref)
	}
	if task1.Ref != "1.1" || task2.Ref != "1.2" {
		t.Fatalf("unexpected task refs: %q, %q", task1.Ref, task2.Ref)
	}
	if subtask.Ref != "1.2.1" {
		t.Fatalf("unexpected subtask ref: %q", subtask.Ref)
	}
}

func TestResolveTaskIDSupportsIDsAndRefs(t *testing.T) {
	d := openTestDB(t)

	epic, err := d.CreateTask("epic", "Epic", nil, 2, nil, "human")
	if err != nil {
		t.Fatalf("create epic: %v", err)
	}
	task, err := d.CreateTask("task", "Task", &epic.ID, 2, nil, "human")
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	id, err := d.ResolveTaskID(task.ID)
	if err != nil {
		t.Fatalf("resolve by id: %v", err)
	}
	if id != task.ID {
		t.Fatalf("unexpected id by id: %q", id)
	}

	id, err = d.ResolveTaskID(task.Ref)
	if err != nil {
		t.Fatalf("resolve by ref: %v", err)
	}
	if id != task.ID {
		t.Fatalf("unexpected id by ref: %q", id)
	}
}

func TestTaskJSONIncludesParentRef(t *testing.T) {
	d := openTestDB(t)
	epic, _ := d.CreateTask("epic", "Epic", nil, 2, nil, "human")
	task, _ := d.CreateTask("task", "Task", &epic.ID, 2, nil, "human")

	j := task.ToJSON()
	if j.Ref != "1.1" {
		t.Fatalf("unexpected ref: %q", j.Ref)
	}
	if j.ParentID == nil || *j.ParentID != epic.ID {
		t.Fatalf("unexpected parent id: %+v", j.ParentID)
	}
	if j.ParentRef == nil || *j.ParentRef != "1" {
		t.Fatalf("unexpected parent ref: %+v", j.ParentRef)
	}
}

func TestRefsDoNotReuseDeletedNumbers(t *testing.T) {
	d := openTestDB(t)
	epic, _ := d.CreateTask("epic", "Epic", nil, 2, nil, "human")
	task1, _ := d.CreateTask("task", "Task 1", &epic.ID, 2, nil, "human")
	task2, _ := d.CreateTask("task", "Task 2", &epic.ID, 2, nil, "human")

	if err := d.DeleteTask(task2.ID, true, "human"); err != nil {
		t.Fatalf("delete task2: %v", err)
	}
	task3, _ := d.CreateTask("task", "Task 3", &epic.ID, 2, nil, "human")

	if task1.Ref != "1.1" {
		t.Fatalf("unexpected ref for task1: %s", task1.Ref)
	}
	if task3.Ref != "1.3" {
		t.Fatalf("expected deleted ref gap to remain, got %s", task3.Ref)
	}
}
