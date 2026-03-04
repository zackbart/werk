package commands

import (
	"github.com/spf13/cobra"
)

func newAuditCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "audit <task-id>",
		Short: "Show full change history for a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			entries, err := database.GetAudit(args[0])
			if err != nil {
				outputError(err.Error())
				return nil
			}
			if entries == nil {
				outputJSON([]interface{}{})
			} else {
				outputJSON(entries)
			}
			return nil
		},
	}
}
