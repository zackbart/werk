package server

import (
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

var webFS embed.FS

func SetWebFS(fs embed.FS) {
	webFS = fs
}

func Start(db *sql.DB, port int) error {
	mux := http.NewServeMux()

	// API endpoints
	mux.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
		status := map[string]int{}
		for _, s := range []string{"open", "in_progress", "blocked", "done"} {
			var count int
			db.QueryRow("SELECT COUNT(*) FROM tasks WHERE status = ?", s).Scan(&count)
			status[s] = count
		}
		var decs, sess int
		db.QueryRow("SELECT COUNT(*) FROM decisions").Scan(&decs)
		db.QueryRow("SELECT COUNT(*) FROM sessions").Scan(&sess)
		status["decisions"] = decs
		status["sessions"] = sess
		writeJSON(w, status)
	})

	mux.HandleFunc("/api/epics", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, queryTasks(db, "epic", ""))
	})

	mux.HandleFunc("/api/tasks", func(w http.ResponseWriter, r *http.Request) {
		status := r.URL.Query().Get("status")
		epicID := r.URL.Query().Get("epic")
		query := "SELECT id, parent_id, type, title, status, priority, notes, created_at, updated_at, closed_at FROM tasks WHERE type = 'task'"
		args := []interface{}{}
		if epicID != "" {
			query += " AND parent_id = ?"
			args = append(args, epicID)
		}
		if status != "" && status != "all" {
			query += " AND status = ?"
			args = append(args, status)
		}
		query += " ORDER BY priority ASC, created_at ASC"
		writeJSON(w, queryTasksRaw(db, query, args...))
	})

	mux.HandleFunc("/api/subtasks", func(w http.ResponseWriter, r *http.Request) {
		taskID := r.URL.Query().Get("task")
		if taskID == "" {
			writeJSON(w, queryTasks(db, "subtask", ""))
			return
		}
		query := "SELECT id, parent_id, type, title, status, priority, notes, created_at, updated_at, closed_at FROM tasks WHERE type = 'subtask' AND parent_id = ? ORDER BY created_at ASC"
		writeJSON(w, queryTasksRaw(db, query, taskID))
	})

	mux.HandleFunc("/api/decisions", func(w http.ResponseWriter, r *http.Request) {
		rows, err := db.Query("SELECT id, summary, rationale, created_at, created_by FROM decisions ORDER BY created_at ASC")
		if err != nil {
			writeJSON(w, []interface{}{})
			return
		}
		defer rows.Close()
		var decs []map[string]interface{}
		for rows.Next() {
			var id, summary, createdBy string
			var rationale sql.NullString
			var createdAt string
			rows.Scan(&id, &summary, &rationale, &createdAt, &createdBy)
			d := map[string]interface{}{
				"id":         id,
				"summary":    summary,
				"created_at": createdAt,
				"created_by": createdBy,
			}
			if rationale.Valid {
				d["rationale"] = rationale.String
			}
			decs = append(decs, d)
		}
		if decs == nil {
			writeJSON(w, []interface{}{})
		} else {
			writeJSON(w, decs)
		}
	})

	mux.HandleFunc("/api/audit/", func(w http.ResponseWriter, r *http.Request) {
		taskID := strings.TrimPrefix(r.URL.Path, "/api/audit/")
		if taskID == "" {
			http.Error(w, `{"error":"task ID required"}`, 400)
			return
		}
		rows, err := db.Query("SELECT id, task_id, field, old_value, new_value, changed_at, changed_by FROM audit WHERE task_id = ? ORDER BY changed_at ASC, id ASC", taskID)
		if err != nil {
			writeJSON(w, []interface{}{})
			return
		}
		defer rows.Close()
		var entries []map[string]interface{}
		for rows.Next() {
			var id int64
			var tid, field, changedBy string
			var oldVal, newVal sql.NullString
			var changedAt string
			rows.Scan(&id, &tid, &field, &oldVal, &newVal, &changedAt, &changedBy)
			e := map[string]interface{}{
				"id":         id,
				"task_id":    tid,
				"field":      field,
				"changed_at": changedAt,
				"changed_by": changedBy,
			}
			if oldVal.Valid {
				e["old_value"] = oldVal.String
			}
			if newVal.Valid {
				e["new_value"] = newVal.String
			}
			entries = append(entries, e)
		}
		if entries == nil {
			writeJSON(w, []interface{}{})
		} else {
			writeJSON(w, entries)
		}
	})

	mux.HandleFunc("/api/dependencies", func(w http.ResponseWriter, r *http.Request) {
		rows, err := db.Query("SELECT upstream_id, downstream_id FROM dependencies")
		if err != nil {
			writeJSON(w, []interface{}{})
			return
		}
		defer rows.Close()
		var deps []map[string]string
		for rows.Next() {
			var up, down string
			rows.Scan(&up, &down)
			deps = append(deps, map[string]string{"upstream_id": up, "downstream_id": down})
		}
		if deps == nil {
			writeJSON(w, []interface{}{})
		} else {
			writeJSON(w, deps)
		}
	})

	// Serve web UI
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		data, err := webFS.ReadFile("index.html")
		if err != nil {
			http.Error(w, "web UI not found", 500)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(data)
	})

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	return srv.ListenAndServe()
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func queryTasks(db *sql.DB, taskType, status string) []map[string]interface{} {
	query := "SELECT id, parent_id, type, title, status, priority, notes, created_at, updated_at, closed_at FROM tasks WHERE type = ?"
	args := []interface{}{taskType}
	if status != "" && status != "all" {
		query += " AND status = ?"
		args = append(args, status)
	}
	query += " ORDER BY priority ASC, created_at ASC"
	return queryTasksRaw(db, query, args...)
}

func queryTasksRaw(db *sql.DB, query string, args ...interface{}) []map[string]interface{} {
	rows, err := db.Query(query, args...)
	if err != nil {
		return []map[string]interface{}{}
	}
	defer rows.Close()

	var tasks []map[string]interface{}
	for rows.Next() {
		var id, tType, title, status string
		var parentID, notes sql.NullString
		var priority int
		var createdAt string
		var updatedAt, closedAt sql.NullString
		rows.Scan(&id, &parentID, &tType, &title, &status, &priority, &notes, &createdAt, &updatedAt, &closedAt)

		t := map[string]interface{}{
			"id":         id,
			"type":       tType,
			"title":      title,
			"status":     status,
			"priority":   priority,
			"created_at": createdAt,
		}
		if parentID.Valid {
			switch tType {
			case "task":
				t["epic_id"] = parentID.String
			case "subtask":
				t["task_id"] = parentID.String
			}
		}
		if notes.Valid {
			t["notes"] = notes.String
		}
		if updatedAt.Valid {
			t["updated_at"] = updatedAt.String
		}
		if closedAt.Valid {
			t["closed_at"] = closedAt.String
		}
		tasks = append(tasks, t)
	}
	if tasks == nil {
		return []map[string]interface{}{}
	}
	return tasks
}
