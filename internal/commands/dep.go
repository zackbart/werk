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
			err := database.AddDependency(args[0], args[1], changedBy())
			if err != nil {
				outputError(err.Error())
				return nil
			}
			outputJSON(map[string]string{
				"status":      "added",
				"upstream_id":  args[0],
				"downstream_id": args[1],
			})
			return nil
		},
	}

	removeCmd := &cobra.Command{
		Use:   "remove <upstream-id> <downstream-id>",
		Short: "Remove dependency",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			err := database.RemoveDependency(args[0], args[1], changedBy())
			if err != nil {
				outputError(err.Error())
				return nil
			}
			outputJSON(map[string]string{
				"status":      "removed",
				"upstream_id":  args[0],
				"downstream_id": args[1],
			})
			return nil
		},
	}

	listCmd := &cobra.Command{
		Use:   "list <id>",
		Short: "List dependencies for a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			info, err := database.GetDependencies(args[0])
			if err != nil {
				outputError(err.Error())
				return nil
			}
			outputJSON(info)
			return nil
		},
	}

	depCmd.AddCommand(addCmd, removeCmd, listCmd)
	return depCmd
}
