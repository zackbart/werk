package commands

import (
	"werk/internal/models"

	"github.com/spf13/cobra"
)

type handoffIdentity struct {
	ID  string `json:"id"`
	Ref string `json:"ref"`
}

type handoffDependencies struct {
	BlockedBy []handoffIdentity `json:"blocked_by"`
	Blocks    []handoffIdentity `json:"blocks"`
}

type handoffStatusSummary struct {
	Open       int `json:"open"`
	InProgress int `json:"in_progress"`
	Blocked    int `json:"blocked"`
	Done       int `json:"done"`
}

type handoffPacket struct {
	Item           models.TaskJSON       `json:"item"`
	Dependencies   handoffDependencies   `json:"dependencies"`
	Children       []models.TaskJSON     `json:"children"`
	SubtaskSummary *handoffStatusSummary `json:"subtask_summary,omitempty"`
	Decisions      []models.Decision     `json:"decisions"`
	RecentAudit    []models.AuditEntry   `json:"recent_audit"`
}

func newHandoffCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "handoff <id-or-ref>",
		Short: "Emit compact handoff context for an epic/task/subtask",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := resolveTaskIDOrExit(args[0])
			item, err := database.GetTask(id)
			if err != nil {
				outputErr(err)
				return nil
			}

			deps, err := database.GetDependencies(id)
			if err != nil {
				outputErr(err)
				return nil
			}

			blockedBy := make([]handoffIdentity, 0, len(deps.BlockedBy))
			for _, depID := range deps.BlockedBy {
				t, err := database.GetTask(depID)
				if err == nil {
					blockedBy = append(blockedBy, handoffIdentity{ID: t.ID, Ref: t.Ref})
				}
			}
			blocks := make([]handoffIdentity, 0, len(deps.Blocks))
			for _, depID := range deps.Blocks {
				t, err := database.GetTask(depID)
				if err == nil {
					blocks = append(blocks, handoffIdentity{ID: t.ID, Ref: t.Ref})
				}
			}
			packet := handoffPacket{
				Item: item.ToJSON(),
				Dependencies: handoffDependencies{
					BlockedBy: blockedBy,
					Blocks:    blocks,
				},
				Children: []models.TaskJSON{},
			}

			subtaskSummary := &handoffStatusSummary{}
			switch item.Type {
			case "task":
				subtasks, _ := database.ListTasks("subtask", &item.ID, "all")
				for _, child := range subtasks {
					packet.Children = append(packet.Children, child.ToJSON())
				}
			case "epic":
				tasks, _ := database.ListTasks("task", &item.ID, "all")
				for _, task := range tasks {
					packet.Children = append(packet.Children, task.ToJSON())
					subtasks, _ := database.ListTasks("subtask", &task.ID, "all")
					for _, st := range subtasks {
						switch st.Status {
						case "open":
							subtaskSummary.Open++
						case "in_progress":
							subtaskSummary.InProgress++
						case "blocked":
							subtaskSummary.Blocked++
						case "done":
							subtaskSummary.Done++
						}
					}
				}
				packet.SubtaskSummary = subtaskSummary
			}

			decisions, err := database.ListDecisions()
			if err != nil {
				outputErr(err)
				return nil
			}
			if len(decisions) > 5 {
				decisions = decisions[len(decisions)-5:]
			}
			if decisions == nil {
				decisions = []models.Decision{}
			}
			packet.Decisions = decisions

			audit, err := database.GetAudit(item.ID)
			if err != nil {
				outputErr(err)
				return nil
			}
			if len(audit) > 12 {
				audit = audit[len(audit)-12:]
			}
			if audit == nil {
				audit = []models.AuditEntry{}
			}
			packet.RecentAudit = audit

			outputJSON(packet)
			return nil
		},
	}
	cmd.Flags().Bool("compact", false, "emit compact deterministic handoff payload")
	return cmd
}
