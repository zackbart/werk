package commands

import (
	"github.com/spf13/cobra"
)

func newDepCmd() *cobra.Command {
	depCmd := &cobra.Command{
		Use:   "dep",
		Short: "Manage dependencies",
	}

	addCmd := &cobra.Command{
		Use:   "add <upstream-id> <downstream-id>",
		Short: "Add dependency (upstream blocks downstream)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			upstreamID := mustResolveID(args[0])
			downstreamID := mustResolveID(args[1])
			err := database.AddDependency(upstreamID, downstreamID, changedBy())
			if err != nil {
				outputError(err.Error())
				return nil
			}
			upstreamRef, _ := resolveRef(upstreamID)
			downstreamRef, _ := resolveRef(downstreamID)
			outputJSON(map[string]string{
				"status":         "added",
				"upstream_id":    upstreamID,
				"upstream_ref":   upstreamRef,
				"downstream_id":  downstreamID,
				"downstream_ref": downstreamRef,
			})
			return nil
		},
	}

	removeCmd := &cobra.Command{
		Use:   "remove <upstream-id> <downstream-id>",
		Short: "Remove dependency",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			upstreamID := mustResolveID(args[0])
			downstreamID := mustResolveID(args[1])
			upstreamRef, _ := resolveRef(upstreamID)
			downstreamRef, _ := resolveRef(downstreamID)
			err := database.RemoveDependency(upstreamID, downstreamID, changedBy())
			if err != nil {
				outputError(err.Error())
				return nil
			}
			outputJSON(map[string]string{
				"status":         "removed",
				"upstream_id":    upstreamID,
				"upstream_ref":   upstreamRef,
				"downstream_id":  downstreamID,
				"downstream_ref": downstreamRef,
			})
			return nil
		},
	}

	listCmd := &cobra.Command{
		Use:   "list <id>",
		Short: "List dependencies for a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := mustResolveID(args[0])
			info, err := database.GetDependencies(id)
			if err != nil {
				outputError(err.Error())
				return nil
			}

			blockedBy := make([]map[string]string, 0, len(info.BlockedBy))
			for _, depID := range info.BlockedBy {
				ref, _ := resolveRef(depID)
				blockedBy = append(blockedBy, map[string]string{"id": depID, "ref": ref})
			}
			blocks := make([]map[string]string, 0, len(info.Blocks))
			for _, depID := range info.Blocks {
				ref, _ := resolveRef(depID)
				blocks = append(blocks, map[string]string{"id": depID, "ref": ref})
			}
			outputJSON(map[string]interface{}{
				"id":         id,
				"ref":        mustResolveRef(id),
				"blocked_by": blockedBy,
				"blocks":     blocks,
			})
			return nil
		},
	}

	depCmd.AddCommand(addCmd, removeCmd, listCmd)
	return depCmd
}
