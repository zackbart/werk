package models

import "time"

type Task struct {
	ID        string     `json:"id"`
	Ref       string     `json:"ref"`
	ParentID  *string    `json:"parent_id,omitempty"`
	ParentRef *string    `json:"parent_ref,omitempty"`
	Type      string     `json:"type"`
	Title     string     `json:"title"`
	Status    string     `json:"status"`
	Priority  int        `json:"priority"`
	Notes     *string    `json:"notes,omitempty"`
	Blockers  []string   `json:"blockers,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
	ClosedAt  *time.Time `json:"closed_at,omitempty"`
}

type TaskJSON struct {
	ID        string     `json:"id"`
	Ref       string     `json:"ref"`
	Type      string     `json:"type"`
	Title     string     `json:"title"`
	Status    string     `json:"status"`
	Priority  int        `json:"priority"`
	ParentID  *string    `json:"parent_id"`
	ParentRef *string    `json:"parent_ref"`
	Blockers  []string   `json:"blockers"`
	Notes     *string    `json:"notes"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt *time.Time `json:"updated_at"`
	ClosedAt  *time.Time `json:"closed_at"`
}

func (t Task) ToJSON() TaskJSON {
	j := TaskJSON{
		ID:        t.ID,
		Ref:       t.Ref,
		Type:      t.Type,
		Title:     t.Title,
		Status:    t.Status,
		Priority:  t.Priority,
		ParentID:  t.ParentID,
		ParentRef: t.ParentRef,
		Notes:     t.Notes,
		Blockers:  t.Blockers,
		CreatedAt: t.CreatedAt,
		UpdatedAt: t.UpdatedAt,
		ClosedAt:  t.ClosedAt,
	}
	if j.Blockers == nil {
		j.Blockers = []string{}
	}
	return j
}

type Decision struct {
	ID        string    `json:"id"`
	Summary   string    `json:"summary"`
	Rationale *string   `json:"rationale,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	CreatedBy string    `json:"created_by"`
}

type Session struct {
	ID           string     `json:"id"`
	StartedAt    time.Time  `json:"started_at"`
	EndedAt      *time.Time `json:"ended_at,omitempty"`
	Summary      *string    `json:"summary,omitempty"`
	TasksTouched []string   `json:"tasks_touched"`
}

type AuditEntry struct {
	ID        int64     `json:"id"`
	TaskID    string    `json:"task_id"`
	Field     string    `json:"field"`
	OldValue  *string   `json:"old_value"`
	NewValue  *string   `json:"new_value"`
	ChangedAt time.Time `json:"changed_at"`
	ChangedBy string    `json:"changed_by"`
}

type Dependency struct {
	UpstreamID   string `json:"upstream_id"`
	DownstreamID string `json:"downstream_id"`
}

type DepInfo struct {
	BlockedBy []string `json:"blocked_by"`
	Blocks    []string `json:"blocks"`
}

type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Event     string    `json:"event"`
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Detail    *string   `json:"detail,omitempty"`
}

type StatusSummary struct {
	Open       int `json:"open"`
	InProgress int `json:"in_progress"`
	Blocked    int `json:"blocked"`
	Done       int `json:"done"`
	Decisions  int `json:"decisions"`
	Sessions   int `json:"sessions"`
}
