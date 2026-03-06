package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"werk/internal/db"
)

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize or upgrade .werk/tasks.db in current directory",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := getDBPath()
			existing := false
			if _, err := os.Stat(path); err == nil {
				existing = true
			}

			d, err := db.Open(path)
			if err != nil {
				outputError(fmt.Sprintf("failed to initialize: %v", err))
				return nil
			}
			d.Close()

			// Write/fix .gitignore in .werk/ (fixes broken patterns from <= 0.1.1)
			werkDir := filepath.Dir(path)
			gitignorePath := filepath.Join(werkDir, ".gitignore")
			gitignore := "*.db-wal\n*.db-shm\nsession.lock\nserve.pid\n"
			os.WriteFile(gitignorePath, []byte(gitignore), 0644)

			// Auto-register workspace
			if cwd, err := os.Getwd(); err == nil {
				name := filepath.Base(cwd)
				ws := loadWorkspaces()
				if _, exists := ws[name]; !exists {
					ws[name] = cwd
					saveWorkspaces(ws)
				}
			}

			if existing {
				outputJSON(map[string]string{"status": "upgraded", "path": path})
			} else {
				outputJSON(map[string]string{"status": "initialized", "path": path})
			}
			return nil
		},
	}
}
