package commands

import (
	"github.com/spf13/cobra"
)

func newNextCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "next",
		Short: "Pick the highest-priority ready task and start it",
		RunE: func(cmd *cobra.Command, args []string) error {
			tasks, err := database.ReadyTasks()
			if err != nil {
				outputError(err.Error())
				return nil
			}
			if len(tasks) == 0 {
				outputError("no ready tasks")
				return nil
			}
			// ReadyTasks already ordered by priority ASC, created_at ASC
			pick := tasks[0]
			t, err := database.SetTaskStatus(pick.ID, "in_progress", changedBy())
			if err != nil {
				outputError(err.Error())
				return nil
			}
			subtasks, _ := database.ListChildren(t.ID)
			out := t.ToJSON()
			out["subtasks"] = toTaskJSONList(subtasks)
			outputJSON(out)
			return nil
		},
	}
}
