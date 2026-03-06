package commands

import (
	"os"
	"path/filepath"
	"testing"

	"werk/internal/db"
)

func openTestDB(t *testing.T) *db.DB {
	t.Helper()
	d, err := db.Open(filepath.Join(t.TempDir(), "tasks.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	return d
}

func TestRecoverSessionLock(t *testing.T) {
	d := openTestDB(t)
	lockPath := filepath.Join(t.TempDir(), "session.lock")

	result, err := recoverSessionLock(d, lockPath)
	if err != nil {
		t.Fatalf("recover no lock: %v", err)
	}
	if result["status"] != "no_lock" {
		t.Fatalf("expected no_lock, got %#v", result["status"])
	}

	if err := os.WriteFile(lockPath, []byte("missing-session"), 0644); err != nil {
		t.Fatalf("write lock: %v", err)
	}
	result, err = recoverSessionLock(d, lockPath)
	if err != nil {
		t.Fatalf("recover stale lock: %v", err)
	}
	if result["status"] != "recovered" {
		t.Fatalf("expected recovered, got %#v", result["status"])
	}
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Fatalf("expected lock to be removed")
	}

	s, err := d.CreateSession()
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if err := os.WriteFile(lockPath, []byte(s.ID), 0644); err != nil {
		t.Fatalf("write lock 2: %v", err)
	}
	result, err = recoverSessionLock(d, lockPath)
	if err != nil {
		t.Fatalf("recover active lock: %v", err)
	}
	if result["status"] != "active_session" {
		t.Fatalf("expected active_session, got %#v", result["status"])
	}
}
