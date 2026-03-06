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
		archived := r.URL.Query().Get("archived") == "1"
		writeJSON(w, queryTasks(db, "epic", "", archived))
	})

	mux.HandleFunc("/api/tasks", func(w http.ResponseWriter, r *http.Request) {
		status := r.URL.Query().Get("status")
		epicIDOrRef := r.URL.Query().Get("epic")
		archived := r.URL.Query().Get("archived") == "1"
		query := "SELECT id, ref, parent_id, (SELECT ref FROM tasks p WHERE p.id = t.parent_id), type, title, status, priority, notes, archived, created_at, updated_at, closed_at FROM tasks t WHERE type = 'task'"
		args := []interface{}{}
		if !archived {
			query += " AND archived = 0"
		}
		if epicIDOrRef != "" {
			epicID := resolveTaskIDFromDB(db, epicIDOrRef)
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

	mux.HandleFunc("/api/sessions", func(w http.ResponseWriter, r *http.Request) {
		rows, err := db.Query("SELECT id, started_at, ended_at, summary, tasks_touched FROM sessions ORDER BY started_at DESC")
		if err != nil {
			writeJSON(w, []interface{}{})
			return
		}
		defer rows.Close()
		var sessions []map[string]interface{}
		for rows.Next() {
			var id, startedAt, tasksTouched string
			var endedAt, summary sql.NullString
			rows.Scan(&id, &startedAt, &endedAt, &summary, &tasksTouched)
			s := map[string]interface{}{
				"id":         id,
				"started_at": startedAt,
			}
			if endedAt.Valid {
				s["ended_at"] = endedAt.String
			}
			if summary.Valid {
				s["summary"] = summary.String
			}
			s["tasks_touched"] = tasksTouched
			sessions = append(sessions, s)
		}
		writeJSON(w, emptyIfNil(sessions))
	})

	mux.HandleFunc("/api/subtasks", func(w http.ResponseWriter, r *http.Request) {
		taskID := r.URL.Query().Get("task")
		if taskID == "" {
			writeJSON(w, queryTasks(db, "subtask", "", false))
			return
		}
		query := "SELECT id, ref, parent_id, (SELECT ref FROM tasks p WHERE p.id = t.parent_id), type, title, status, priority, notes, archived, created_at, updated_at, closed_at FROM tasks t WHERE type = 'subtask' AND parent_id = ? ORDER BY created_at ASC"
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
		writeJSON(w, emptyIfNil(decs))
	})

	mux.HandleFunc("/api/audit/", func(w http.ResponseWriter, r *http.Request) {
		idOrRef := strings.TrimPrefix(r.URL.Path, "/api/audit/")
		if idOrRef == "" {
			http.Error(w, `{"error":"task ID required"}`, 400)
			return
		}
		taskID := resolveTaskIDFromDB(db, idOrRef)
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
		writeJSON(w, emptyIfNil(entries))
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
		writeJSON(w, emptyIfNil(deps))
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

func emptyIfNil[T any](s []T) interface{} {
	if s == nil {
		return []interface{}{}
	}
	return s
}

// resolveTaskIDFromDB resolves an id-or-ref to an internal ID using the DB directly.
func resolveTaskIDFromDB(db *sql.DB, idOrRef string) string {
	// Try as ID first
	var exists int
	if err := db.QueryRow(`SELECT 1 FROM tasks WHERE id = ?`, idOrRef).Scan(&exists); err == nil {
		return idOrRef
	}
	// Try as ref
	var id string
	if err := db.QueryRow(`SELECT id FROM tasks WHERE ref = ?`, idOrRef).Scan(&id); err == nil {
		return id
	}
	return idOrRef
}

func queryTasks(db *sql.DB, taskType, status string, includeArchived bool) []map[string]interface{} {
	query := "SELECT id, ref, parent_id, (SELECT ref FROM tasks p WHERE p.id = t.parent_id), type, title, status, priority, notes, archived, created_at, updated_at, closed_at FROM tasks t WHERE type = ?"
	args := []interface{}{taskType}
	if !includeArchived {
		query += " AND archived = 0"
	}
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
		var id, ref, tType, title, status string
		var parentID, parentRef, notes sql.NullString
		var priority, archived int
		var createdAt string
		var updatedAt, closedAt sql.NullString
		rows.Scan(&id, &ref, &parentID, &parentRef, &tType, &title, &status, &priority, &notes, &archived, &createdAt, &updatedAt, &closedAt)

		t := map[string]interface{}{
			"id":         id,
			"ref":        ref,
			"parent_id":  nil,
			"parent_ref": nil,
			"type":       tType,
			"title":      title,
			"status":     status,
			"priority":   priority,
			"archived":   archived != 0,
			"created_at": createdAt,
			"notes":      nil,
			"updated_at": nil,
			"closed_at":  nil,
		}
		if parentID.Valid {
			t["parent_id"] = parentID.String
		}
		if parentRef.Valid {
			t["parent_ref"] = parentRef.String
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
