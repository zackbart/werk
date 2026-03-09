package commands

import (
	"bufio"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func newBatchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "batch",
		Short: "Execute commands from stdin, one per line",
		RunE: func(cmd *cobra.Command, args []string) error {
			root := cmd.Root()
			scanner := bufio.NewScanner(os.Stdin)
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if line == "" || strings.HasPrefix(line, "#") {
					continue
				}
				cmdArgs := strings.Fields(line)
				subCmd, subArgs, err := root.Find(cmdArgs)
				if err != nil || subCmd == root {
					outputJSON(map[string]interface{}{
						"command": line,
						"error":   "unknown command",
					})
					continue
				}
				subCmd.SetArgs(subArgs)
				if err := subCmd.Execute(); err != nil {
					outputJSON(map[string]interface{}{
						"command": line,
						"error":   err.Error(),
					})
				}
			}
			return nil
		},
	}
}
