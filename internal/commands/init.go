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
				outputError("werk is already initialized (database exists)")
				return nil
			}

			d, err := db.Open(path)
			if err != nil {
				outputError(fmt.Sprintf("failed to initialize: %v", err))
				return nil
			}
			d.Close()

			// Create .gitignore in .werk/
			gitignore := ".werk/*.db-wal\n.werk/*.db-shm\n.werk/session.lock\n"
			os.WriteFile(".werk/.gitignore", []byte(gitignore), 0644)

			outputJSON(map[string]string{"status": "initialized", "path": path})
			return nil
		},
	}
}
