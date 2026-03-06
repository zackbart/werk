package commands

import (
	"github.com/spf13/cobra"
)

func newAuditCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "audit <task-id-or-ref>",
		Short: "Show full change history for a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := mustResolveID(args[0])
			ref := mustResolveRef(id)
			entries, err := database.GetAudit(id)
			if err != nil {
				outputError(err.Error())
				return nil
			}
			if entries == nil {
				outputJSON([]interface{}{})
			} else {
				out := map[string]interface{}{
					"id":      id,
					"ref":     ref,
					"entries": entries,
				}
				outputJSON(out)
			}
			return nil
		},
	}
}
