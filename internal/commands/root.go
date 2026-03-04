package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"werk/internal/db"
)

var (
	dbPath    string
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
	outputJSON(map[string]string{"error": msg})
	os.Exit(1)
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
	if dbPath != "" {
		return dbPath
	}
	if env := os.Getenv("WERK_DB"); env != "" {
		return env
	}
	// Walk up directory tree to find .werk/tasks.db
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
			// Don't open DB for init or version
			if cmd.Name() == "init" || cmd.Name() == "version" || cmd.Name() == "help" {
				return
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
		newServeCmd(),
	)

	return rootCmd
}
