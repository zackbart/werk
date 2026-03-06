package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func workspacesConfigPath() string {
	if dir, err := os.UserConfigDir(); err == nil {
		return filepath.Join(dir, "werk", "workspaces.json")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "werk", "workspaces.json")
}

func loadWorkspaces() map[string]string {
	data, err := os.ReadFile(workspacesConfigPath())
	if err != nil {
		return map[string]string{}
	}
	var ws map[string]string
	if err := json.Unmarshal(data, &ws); err != nil {
		return map[string]string{}
	}
	return ws
}

func saveWorkspaces(ws map[string]string) error {
	p := workspacesConfigPath()
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(ws, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0644)
}

func newWorkspaceCmd() *cobra.Command {
	wsCmd := &cobra.Command{
		Use:   "workspace",
		Short: "Manage named workspaces",
	}

	addCmd := &cobra.Command{
		Use:   "add <name> [path]",
		Short: "Register a workspace",
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

			ws := loadWorkspaces()
			ws[name] = absPath
			if err := saveWorkspaces(ws); err != nil {
				outputError(fmt.Sprintf("failed to save workspaces: %v", err))
				return nil
			}

			outputJSON(map[string]string{"status": "added", "name": name, "path": absPath})
			return nil
		},
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List registered workspaces",
		RunE: func(cmd *cobra.Command, args []string) error {
			ws := loadWorkspaces()
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
		Short: "Remove a registered workspace",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			ws := loadWorkspaces()
			if _, ok := ws[name]; !ok {
				outputError(fmt.Sprintf("workspace not found: %s", name))
				return nil
			}
			delete(ws, name)
			if err := saveWorkspaces(ws); err != nil {
				outputError(fmt.Sprintf("failed to save workspaces: %v", err))
				return nil
			}
			outputJSON(map[string]string{"status": "removed", "name": name})
			return nil
		},
	}

	wsCmd.AddCommand(addCmd, listCmd, removeCmd)
	return wsCmd
}
