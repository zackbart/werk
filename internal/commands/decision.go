package commands

import (
	"github.com/spf13/cobra"
)

func newDecisionCmd() *cobra.Command {
	decCmd := &cobra.Command{
		Use:   "decision",
		Short: "Manage decisions",
	}

	createCmd := &cobra.Command{
		Use:   "create <summary>",
		Short: "Record a decision",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			rationale, _ := cmd.Flags().GetString("rationale")
			var ratPtr *string
			if cmd.Flags().Changed("rationale") {
				ratPtr = &rationale
			}

			dec, err := database.CreateDecision(args[0], ratPtr, changedBy())
			if err != nil {
				outputErr(err)
				return nil
			}
			outputJSON(dec)
			return nil
		},
	}
	createCmd.Flags().String("rationale", "", "rationale for the decision")

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List all decisions",
		RunE: func(cmd *cobra.Command, args []string) error {
			decs, err := database.ListDecisions()
			if err != nil {
				outputErr(err)
				return nil
			}
			if decs == nil {
				outputJSON([]interface{}{})
			} else {
				outputJSON(decs)
			}
			return nil
		},
	}

	showCmd := &cobra.Command{
		Use:   "show <id>",
		Short: "Show a decision",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dec, err := database.GetDecision(args[0])
			if err != nil {
				outputErr(err)
				return nil
			}
			outputJSON(dec)
			return nil
		},
	}

	decCmd.AddCommand(createCmd, listCmd, showCmd)
	return decCmd
}
