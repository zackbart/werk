package commands

import (
	"github.com/spf13/cobra"
)

func toIdentityList(ids []string) []map[string]string {
	out := make([]map[string]string, 0, len(ids))
	for _, id := range ids {
		t, err := database.GetTask(id)
		if err != nil {
			continue
		}
		out = append(out, map[string]string{
			"id":  t.ID,
			"ref": t.Ref,
		})
	}
	return out
}

func newDepCmd() *cobra.Command {
	depCmd := &cobra.Command{
		Use:   "dep",
		Short: "Manage dependencies",
	}

	addCmd := &cobra.Command{
		Use:   "add <upstream-id-or-ref> <downstream-id-or-ref>",
		Short: "Add dependency (upstream blocks downstream)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			upstreamID := resolveTaskIDOrExit(args[0])
			downstreamID := resolveTaskIDOrExit(args[1])
			upstreamTask, _ := database.GetTask(upstreamID)
			downstreamTask, _ := database.GetTask(downstreamID)
			err := database.AddDependency(upstreamID, downstreamID, changedBy())
			if err != nil {
				outputErr(err)
				return nil
			}
			outputJSON(map[string]string{
				"status":         "added",
				"upstream_id":    upstreamID,
				"downstream_id":  downstreamID,
				"upstream_ref":   upstreamTask.Ref,
				"downstream_ref": downstreamTask.Ref,
			})
			return nil
		},
	}

	removeCmd := &cobra.Command{
		Use:   "remove <upstream-id-or-ref> <downstream-id-or-ref>",
		Short: "Remove dependency",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			upstreamID := resolveTaskIDOrExit(args[0])
			downstreamID := resolveTaskIDOrExit(args[1])
			upstreamTask, _ := database.GetTask(upstreamID)
			downstreamTask, _ := database.GetTask(downstreamID)
			err := database.RemoveDependency(upstreamID, downstreamID, changedBy())
			if err != nil {
				outputErr(err)
				return nil
			}
			outputJSON(map[string]string{
				"status":         "removed",
				"upstream_id":    upstreamID,
				"downstream_id":  downstreamID,
				"upstream_ref":   upstreamTask.Ref,
				"downstream_ref": downstreamTask.Ref,
			})
			return nil
		},
	}

	listCmd := &cobra.Command{
		Use:   "list <id-or-ref>",
		Short: "List dependencies for a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := resolveTaskIDOrExit(args[0])
			task, _ := database.GetTask(id)
			info, err := database.GetDependencies(id)
			if err != nil {
				outputErr(err)
				return nil
			}
			outputJSON(map[string]interface{}{
				"id":             id,
				"ref":            task.Ref,
				"blocked_by":     toIdentityList(info.BlockedBy),
				"blocks":         toIdentityList(info.Blocks),
				"blocked_by_ids": info.BlockedBy,
				"blocks_ids":     info.Blocks,
			})
			return nil
		},
	}

	depCmd.AddCommand(addCmd, removeCmd, listCmd)
	return depCmd
}
