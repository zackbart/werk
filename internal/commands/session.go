package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"werk/internal/db"
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
				outputErrorCode("INVALID_STATE", fmt.Sprintf("session already active: %s", strings.TrimSpace(string(data))))
				return nil
			}

			s, err := database.CreateSession()
			if err != nil {
				outputErr(err)
				return nil
			}

			// Write lockfile
			if err := os.WriteFile(lockfilePath(), []byte(s.ID), 0644); err != nil {
				outputErrorCode("SESSION_LOCK_FAILED", fmt.Sprintf("failed to write lockfile: %v", err))
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
				outputErrorCode("INVALID_STATE", "no active session")
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
				outputErr(err)
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
				outputErr(err)
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
				outputErr(err)
				return nil
			}
			outputJSON(s)
			return nil
		},
	}

	recoverCmd := &cobra.Command{
		Use:   "recover",
		Short: "Recover stale or invalid session lock state",
		RunE: func(cmd *cobra.Command, args []string) error {
			lockPath := lockfilePath()
			data, err := os.ReadFile(lockPath)
			if err != nil {
				outputJSON(map[string]interface{}{
					"status":    "ok",
					"recovered": false,
					"message":   "no session lock present",
				})
				return nil
			}

			sessionID := strings.TrimSpace(string(data))
			if sessionID == "" {
				if err := os.Remove(lockPath); err != nil && !os.IsNotExist(err) {
					outputErrorCode("SESSION_STALE", fmt.Sprintf("failed to remove stale lock: %v", err))
					return nil
				}
				outputJSON(map[string]interface{}{
					"status":    "ok",
					"recovered": true,
					"message":   "removed empty stale session lock",
				})
				return nil
			}

			s, err := database.GetSession(sessionID)
			if err != nil {
				if ce := db.AsCodedError(err); ce != nil && ce.Code == db.ErrCodeNotFound {
					_ = os.Remove(lockPath)
					outputJSON(map[string]interface{}{
						"status":     "ok",
						"recovered":  true,
						"message":    "removed stale lock for missing session",
						"session_id": sessionID,
					})
					return nil
				}
				outputErr(err)
				return nil
			}

			if s.EndedAt != nil {
				_ = os.Remove(lockPath)
				outputJSON(map[string]interface{}{
					"status":     "ok",
					"recovered":  true,
					"message":    "removed stale lock for ended session",
					"session_id": sessionID,
				})
				return nil
			}

			outputJSON(map[string]interface{}{
				"status":     "ok",
				"recovered":  false,
				"message":    "active session lock is valid",
				"session_id": sessionID,
			})
			return nil
		},
	}

	sessionCmd.AddCommand(startCmd, endCmd, listCmd, showCmd, recoverCmd)
	return sessionCmd
}
