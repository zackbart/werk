package commands

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newExportCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "export",
		Short: "Export all data as JSON",
		RunE: func(cmd *cobra.Command, args []string) error {
			payload, err := database.ExportAll()
			if err != nil {
				outputError(fmt.Sprintf("export failed: %v", err))
				return nil
			}
			data, err := json.MarshalIndent(payload, "", "  ")
			if err != nil {
				outputError(fmt.Sprintf("marshal failed: %v", err))
				return nil
			}
			os.Stdout.Write(data)
			fmt.Println()
			return nil
		},
	}
}
