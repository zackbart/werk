package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"werk/internal/db"
)

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize .werk/tasks.db in current directory",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := getDBPath()
			if _, err := os.Stat(path); err == nil {
				outputErrorCode("INVALID_STATE", "werk is already initialized (database exists)")
				return nil
			}

			d, err := db.Open(path)
			if err != nil {
				outputErrorCode("INIT_FAILED", fmt.Sprintf("failed to initialize: %v", err))
				return nil
			}
			d.Close()

			// Create .gitignore in .werk/
			gitignore := "*.db-wal\n*.db-shm\nsession.lock\nserve.pid\n"
			os.WriteFile(".werk/.gitignore", []byte(gitignore), 0644)

			outputJSON(map[string]string{"status": "initialized", "path": path})
			return nil
		},
	}
}
