package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Project summary",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := database.GetStatus()
			if err != nil {
				outputErrorCode("STATUS_FAILED", fmt.Sprintf("failed to get status: %v", err))
				return nil
			}
			if pretty {
				fmt.Printf("Open:        %d\n", s.Open)
				fmt.Printf("In Progress: %d\n", s.InProgress)
				fmt.Printf("Blocked:     %d\n", s.Blocked)
				fmt.Printf("Done:        %d\n", s.Done)
				fmt.Printf("Decisions:   %d\n", s.Decisions)
				fmt.Printf("Sessions:    %d\n", s.Sessions)
			} else {
				outputJSON(s)
			}
			return nil
		},
	}
}
