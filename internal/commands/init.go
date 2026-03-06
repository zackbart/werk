package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"werk/internal/db"
	"werk/internal/models"
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
			defer d.Close()

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

			// Restore from snapshot if this is a fresh init
			snapshotRestored := false
			if !existing {
				snapshotPath := filepath.Join(werkDir, "snapshot.json")
				if data, err := os.ReadFile(snapshotPath); err == nil {
					var payload models.ExportPayload
					if err := json.Unmarshal(data, &payload); err == nil && payload.SchemaVersion == 1 {
						if err := d.ImportAll(&payload); err == nil {
							snapshotRestored = true
						}
					}
				}
			}

			if existing {
				outputJSON(map[string]string{"status": "upgraded", "path": path})
			} else if snapshotRestored {
				outputJSON(map[string]string{"status": "initialized", "snapshot": "restored", "path": path})
			} else {
				outputJSON(map[string]string{"status": "initialized", "path": path})
			}
			return nil
		},
	}
}
