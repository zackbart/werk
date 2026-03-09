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
			taskIDOrRef, _ := cmd.Flags().GetString("task")

			var entries interface{}
			var err error
			if taskIDOrRef != "" {
				id := mustResolveID(taskIDOrRef)
				e, er := database.GetLogForTask(id, limit, verbose)
				entries = e
				err = er
				if err != nil {
					outputError(err.Error())
					return nil
				}
				if pretty {
					for _, entry := range e {
						ts := entry.Timestamp.Format("2006-01-02 15:04")
						fmt.Printf("%-16s  %-14s  %-12s  %s\n", ts, entry.Event, entry.ID, entry.Title)
						if verbose && entry.Detail != nil && *entry.Detail != "" {
							fmt.Printf("                  %s\n", *entry.Detail)
						}
					}
					if len(e) == 0 {
						fmt.Println("No activity yet.")
					}
					return nil
				}
				if e == nil {
					outputJSON([]interface{}{})
				} else {
					outputJSON(e)
				}
				return nil
			}

			e, er := database.GetLog(limit, verbose)
			entries = e
			err = er
			_ = entries
			if err != nil {
				outputError(err.Error())
				return nil
			}

			if pretty {
				for _, entry := range e {
					ts := entry.Timestamp.Format("2006-01-02 15:04")
					fmt.Printf("%-16s  %-14s  %-12s  %s\n", ts, entry.Event, entry.ID, entry.Title)
					if verbose && entry.Detail != nil && *entry.Detail != "" {
						fmt.Printf("                  %s\n", *entry.Detail)
					}
				}
				if len(e) == 0 {
					fmt.Println("No activity yet.")
				}
			} else {
				if e == nil {
					outputJSON([]interface{}{})
				} else {
					outputJSON(e)
				}
			}
			return nil
		},
	}

	cmd.Flags().IntP("limit", "n", 20, "number of entries to show")
	cmd.Flags().BoolP("verbose", "v", false, "include notes, rationale, and session details")
	cmd.Flags().String("task", "", "filter log to a specific task <id>")

	return cmd
}
