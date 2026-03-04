package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func werkDir() string {
	return filepath.Dir(getDBPath())
}

func lockfilePath() string {
	return filepath.Join(werkDir(), "session.lock")
}

func newSessionCmd() *cobra.Command {
	sessionCmd := &cobra.Command{
		Use:   "session",
		Short: "Manage work sessions",
	}

	startCmd := &cobra.Command{
		Use:   "start",
		Short: "Start a new session",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Check lockfile
			if _, err := os.Stat(lockfilePath()); err == nil {
				data, _ := os.ReadFile(lockfilePath())
				outputError(fmt.Sprintf("session already active: %s", strings.TrimSpace(string(data))))
				return nil
			}

			s, err := database.CreateSession()
			if err != nil {
				outputError(err.Error())
				return nil
			}

			// Write lockfile
			if err := os.WriteFile(lockfilePath(), []byte(s.ID), 0644); err != nil {
				outputError(fmt.Sprintf("failed to write lockfile: %v", err))
				return nil
			}

			outputJSON(s)
			return nil
		},
	}

	endCmd := &cobra.Command{
		Use:   "end",
		Short: "End the active session",
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := os.ReadFile(lockfilePath())
			if err != nil {
				outputError("no active session")
				return nil
			}

			sessionID := strings.TrimSpace(string(data))
			summary, _ := cmd.Flags().GetString("summary")
			var summaryPtr *string
			if cmd.Flags().Changed("summary") {
				summaryPtr = &summary
			}

			s, err := database.EndSession(sessionID, summaryPtr)
			if err != nil {
				outputError(err.Error())
				return nil
			}

			os.Remove(lockfilePath())
			outputJSON(s)
			return nil
		},
	}
	endCmd.Flags().String("summary", "", "session summary")

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List all sessions",
		RunE: func(cmd *cobra.Command, args []string) error {
			sessions, err := database.ListSessions()
			if err != nil {
				outputError(err.Error())
				return nil
			}
			if sessions == nil {
				outputJSON([]interface{}{})
			} else {
				outputJSON(sessions)
			}
			return nil
		},
	}

	showCmd := &cobra.Command{
		Use:   "show <id>",
		Short: "Show session details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := database.GetSession(args[0])
			if err != nil {
				outputError(err.Error())
				return nil
			}
			outputJSON(s)
			return nil
		},
	}

	sessionCmd.AddCommand(startCmd, endCmd, listCmd, showCmd)
	return sessionCmd
}
