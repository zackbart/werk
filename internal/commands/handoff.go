package commands

import "github.com/spf13/cobra"

func newHandoffCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "handoff <id-or-ref>",
		Short: "Generate a compact handoff packet for a task-like item",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			compact, _ := cmd.Flags().GetBool("compact")
			if !compact {
				outputErrorCode("ERR_INVALID_INPUT", "--compact is required for handoff v0.2", nil)
				return nil
			}

			packet, err := database.BuildCompactHandoff(args[0])
			if err != nil {
				outputError(err.Error())
				return nil
			}
			outputJSON(packet)
			return nil
		},
	}

	cmd.Flags().Bool("compact", true, "emit compact machine-friendly packet")
	return cmd
}
