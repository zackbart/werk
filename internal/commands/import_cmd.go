package commands

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"werk/internal/models"
)

func newImportCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "import <file>",
		Short: "Import data from a JSON export",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := os.ReadFile(args[0])
			if err != nil {
				outputError(fmt.Sprintf("failed to read file: %v", err))
				return nil
			}

			var payload models.ExportPayload
			if err := json.Unmarshal(data, &payload); err != nil {
				outputError(fmt.Sprintf("invalid JSON: %v", err))
				return nil
			}

			if payload.SchemaVersion != 1 {
				outputError(fmt.Sprintf("unsupported schema version: %d", payload.SchemaVersion))
				return nil
			}

			if err := database.ImportAll(&payload); err != nil {
				outputError(fmt.Sprintf("import failed: %v", err))
				return nil
			}

			outputJSON(map[string]interface{}{
				"status":    "imported",
				"tasks":     len(payload.Tasks),
				"decisions": len(payload.Decisions),
				"sessions":  len(payload.Sessions),
			})
			return nil
		},
	}
}
