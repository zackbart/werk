package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func werkspacesConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "werk", "werkspaces.json")
}

func loadWerkspaces() map[string]string {
	data, err := os.ReadFile(werkspacesConfigPath())
	if err != nil {
		return map[string]string{}
	}
	var ws map[string]string
	if err := json.Unmarshal(data, &ws); err != nil {
		return map[string]string{}
	}
	return ws
}

func saveWerkspaces(ws map[string]string) error {
	p := werkspacesConfigPath()
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(ws, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0644)
}

func newWerkspaceCmd() *cobra.Command {
	wsCmd := &cobra.Command{
		Use:   "werkspace",
		Short: "Manage named werkspaces",
	}

	addCmd := &cobra.Command{
		Use:   "add <name> [path]",
		Short: "Register a werkspace",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			var wsPath string
			if len(args) > 1 {
				wsPath = args[1]
			} else {
				var err error
				wsPath, err = os.Getwd()
				if err != nil {
					outputError(fmt.Sprintf("failed to get working directory: %v", err))
					return nil
				}
			}

			absPath, err := filepath.Abs(wsPath)
			if err != nil {
				outputError(fmt.Sprintf("invalid path: %v", err))
				return nil
			}

			dbFile := filepath.Join(absPath, ".werk", "tasks.db")
			if _, err := os.Stat(dbFile); os.IsNotExist(err) {
				outputError(fmt.Sprintf("no .werk/tasks.db found at %s", absPath))
				return nil
			}

			ws := loadWerkspaces()
			ws[name] = absPath
			if err := saveWerkspaces(ws); err != nil {
				outputError(fmt.Sprintf("failed to save werkspaces: %v", err))
				return nil
			}

			outputJSON(map[string]string{"status": "added", "name": name, "path": absPath})
			return nil
		},
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List registered werkspaces",
		RunE: func(cmd *cobra.Command, args []string) error {
			ws := loadWerkspaces()
			if pretty {
				var rows [][]string
				for name, path := range ws {
					rows = append(rows, []string{name, path})
				}
				prettyTable([]string{"NAME", "PATH"}, rows)
			} else {
				outputJSON(ws)
			}
			return nil
		},
	}

	removeCmd := &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a registered werkspace",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			ws := loadWerkspaces()
			if _, ok := ws[name]; !ok {
				outputError(fmt.Sprintf("werkspace not found: %s", name))
				return nil
			}
			delete(ws, name)
			if err := saveWerkspaces(ws); err != nil {
				outputError(fmt.Sprintf("failed to save werkspaces: %v", err))
				return nil
			}
			outputJSON(map[string]string{"status": "removed", "name": name})
			return nil
		},
	}

	wsCmd.AddCommand(addCmd, listCmd, removeCmd)
	return wsCmd
}
