package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"werk/internal/db"
)

var (
	dbPath    string
	rootPath  string
	wsName    string
	pretty    bool
	agentMode bool
	database  *db.DB
	version   = "0.1.0"
)

func changedBy() string {
	if agentMode {
		return "agent"
	}
	return "human"
}

func outputJSON(v interface{}) {
	if pretty {
		data, _ := json.MarshalIndent(v, "", "  ")
		fmt.Println(string(data))
	} else {
		data, _ := json.Marshal(v)
		fmt.Println(string(data))
	}
}

func outputError(msg string) {
	outputErrorCode(errorCodeFromMessage(msg), msg, nil)
}

func outputErrorCode(code, message string, details interface{}) {
	payload := map[string]interface{}{
		"code":    code,
		"message": message,
	}
	if details != nil {
		payload["details"] = details
	}
	outputJSON(payload)
	os.Exit(1)
}

func errorCodeFromMessage(message string) string {
	m := strings.ToLower(message)
	switch {
	case strings.Contains(m, "not found"), strings.Contains(m, "no active session"):
		return "ERR_NOT_FOUND"
	case strings.Contains(m, "already"), strings.Contains(m, "cannot"), strings.Contains(m, "cycle"), strings.Contains(m, "exists"):
		return "ERR_CONFLICT"
	case strings.Contains(m, "required"), strings.Contains(m, "invalid"), strings.Contains(m, "must"):
		return "ERR_INVALID_INPUT"
	case strings.Contains(m, "database"), strings.Contains(m, "failed to open"), strings.Contains(m, "failed to initialize"):
		return "ERR_DB"
	case strings.Contains(m, "lock"):
		return "ERR_SESSION_LOCK"
	default:
		return "ERR_INTERNAL"
	}
}

func prettyTable(headers []string, rows [][]string) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	header := ""
	for i, h := range headers {
		if i > 0 {
			header += "\t"
		}
		header += h
	}
	fmt.Fprintln(w, header)
	for _, row := range rows {
		line := ""
		for i, col := range row {
			if i > 0 {
				line += "\t"
			}
			line += col
		}
		fmt.Fprintln(w, line)
	}
	w.Flush()
}

func getDBPath() string {
	// 1. --db flag
	if dbPath != "" {
		return dbPath
	}
	// 2. --ws <name> → resolve from workspace registry
	if wsName != "" {
		ws := loadWorkspaces()
		if p, ok := ws[wsName]; ok {
			return filepath.Join(p, ".werk", "tasks.db")
		}
	}
	// 3. --root <path>
	if rootPath != "" {
		return filepath.Join(rootPath, ".werk", "tasks.db")
	}
	// 4. WERK_DB env
	if env := os.Getenv("WERK_DB"); env != "" {
		return env
	}
	// 5. WERK_ROOT env
	if env := os.Getenv("WERK_ROOT"); env != "" {
		return filepath.Join(env, ".werk", "tasks.db")
	}
	// 6. Walk up directory tree to find .werk/tasks.db
	dir, err := os.Getwd()
	if err != nil {
		return ".werk/tasks.db"
	}
	for {
		candidate := filepath.Join(dir, ".werk", "tasks.db")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break // reached filesystem root
		}
		dir = parent
	}
	// Not found — default to cwd (for init)
	return ".werk/tasks.db"
}

func openDB() {
	var err error
	database, err = db.Open(getDBPath())
	if err != nil {
		outputError(fmt.Sprintf("failed to open database: %v", err))
	}
}

func NewRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "werk",
		Short: "Local-first task and decision tracker",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			// Don't open DB for commands that don't need it
			if cmd.Name() == "init" || cmd.Name() == "version" || cmd.Name() == "help" {
				return
			}
			if cmd.Parent() != nil && cmd.Parent().Name() == "workspace" {
				return
			}
			// Allow WERK_PRETTY=1 env var to enable pretty output,
			// but only if the --pretty flag was not explicitly set.
			if !cmd.Flags().Changed("pretty") && os.Getenv("WERK_PRETTY") == "1" {
				pretty = true
			}
			openDB()
		},
		PersistentPostRun: func(cmd *cobra.Command, args []string) {
			if database != nil {
				database.Close()
			}
		},
	}

	rootCmd.PersistentFlags().StringVar(&dbPath, "db", "", "database path (default: .werk/tasks.db)")
	rootCmd.PersistentFlags().StringVarP(&rootPath, "root", "r", "", "project root directory")
	rootCmd.PersistentFlags().StringVar(&wsName, "ws", "", "named workspace")
	rootCmd.PersistentFlags().BoolVar(&pretty, "pretty", false, "human-readable output")
	rootCmd.PersistentFlags().BoolVar(&agentMode, "agent", false, "set changed_by to agent")

	rootCmd.Version = version

	rootCmd.AddCommand(
		newInitCmd(),
		newStatusCmd(),
		newEpicCmd(),
		newTaskCmd(),
		newSubtaskCmd(),
		newDepCmd(),
		newDecisionCmd(),
		newSessionCmd(),
		newAuditCmd(),
		newHandoffCmd(),
		newLogCmd(),
		newServeCmd(),
		newWorkspaceCmd(),
		newExportCmd(),
		newImportCmd(),
		newDiffCmd(),
		newNextCmd(),
		newBatchCmd(),
	)

	return rootCmd
}
