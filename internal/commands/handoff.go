package commands

import "github.com/spf13/cobra"

func newHandoffCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "handoff <id>",
		Short: "Generate a compact handoff packet for a task-like item",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			packet, err := database.BuildCompactHandoff(args[0])
			if err != nil {
				outputError(err.Error())
				return nil
			}
			outputJSON(packet)
			return nil
		},
	}
}
