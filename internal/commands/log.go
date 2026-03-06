package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newLogCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "log",
		Short: "Show recent project activity",
		RunE: func(cmd *cobra.Command, args []string) error {
			limit, _ := cmd.Flags().GetInt("limit")
			verbose, _ := cmd.Flags().GetBool("verbose")

			entries, err := database.GetLog(limit, verbose)
			if err != nil {
				outputErr(err)
				return nil
			}

			if pretty {
				for _, e := range entries {
					ts := e.Timestamp.Format("2006-01-02 15:04")
					fmt.Printf("%-16s  %-14s  %-12s  %s\n", ts, e.Event, e.ID, e.Title)
					if verbose && e.Detail != nil && *e.Detail != "" {
						fmt.Printf("                  %s\n", *e.Detail)
					}
				}
				if len(entries) == 0 {
					fmt.Println("No activity yet.")
				}
			} else {
				if entries == nil {
					outputJSON([]interface{}{})
				} else {
					outputJSON(entries)
				}
			}
			return nil
		},
	}

	cmd.Flags().IntP("limit", "n", 20, "number of entries to show")
	cmd.Flags().BoolP("verbose", "v", false, "include notes, rationale, and session details")

	return cmd
}
