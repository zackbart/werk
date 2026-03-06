package commands

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

func newDiffCmd() *cobra.Command {
	diffCmd := &cobra.Command{
		Use:   "diff",
		Short: "Show changes since last session",
		RunE: func(cmd *cobra.Command, args []string) error {
			sinceRef, _ := cmd.Flags().GetString("since")

			var since time.Time

			if sinceRef != "" {
				s, err := database.GetSession(sinceRef)
				if err != nil {
					outputError(fmt.Sprintf("session not found: %s", sinceRef))
					return nil
				}
				since = s.StartedAt
			} else {
				sessions, err := database.ListSessions()
				if err != nil {
					outputError(err.Error())
					return nil
				}
				// Find most recent ended session
				for _, s := range sessions {
					if s.EndedAt != nil {
						since = s.StartedAt
						break
					}
				}
				if since.IsZero() {
					outputError("no ended sessions found")
					return nil
				}
			}

			entries, err := database.GetAuditSince(since)
			if err != nil {
				outputError(err.Error())
				return nil
			}

			// Group by task
			grouped := map[string][]interface{}{}
			for _, e := range entries {
				tid := e.TaskID
				grouped[tid] = append(grouped[tid], map[string]interface{}{
					"field":      e.Field,
					"old_value":  e.OldValue,
					"new_value":  e.NewValue,
					"changed_at": e.ChangedAt,
					"changed_by": e.ChangedBy,
				})
			}

			if len(grouped) == 0 {
				outputJSON(map[string]interface{}{"changes": map[string]interface{}{}, "since": since})
			} else {
				outputJSON(map[string]interface{}{"changes": grouped, "since": since})
			}
			return nil
		},
	}
	diffCmd.Flags().String("since", "", "session ID to diff from")
	return diffCmd
}
