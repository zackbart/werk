package commands

import (
	"embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"werk/internal/server"
)

var WebFS embed.FS

func pidfilePath() string {
	dir := werkDir()
	abs, err := filepath.Abs(dir)
	if err != nil {
		return filepath.Join(dir, "serve.pid")
	}
	return filepath.Join(abs, "serve.pid")
}

func newServeCmd() *cobra.Command {
	serveCmd := &cobra.Command{
		Use:   "serve",
		Short: "Manage the local web UI server",
	}

	upCmd := &cobra.Command{
		Use:   "up",
		Short: "Start the web UI server",
		RunE: func(cmd *cobra.Command, args []string) error {
			port, _ := cmd.Flags().GetInt("port")

			// Check if already running
			if pid, err := readPid(); err == nil {
				if processRunning(pid) {
					outputError(fmt.Sprintf("server already running (pid %d)", pid))
					return nil
				}
				// Stale pidfile, clean up
				os.Remove(pidfilePath())
			}

			background, _ := cmd.Flags().GetBool("foreground")
			if !background {
				os.Remove(pidfilePath()) // Clean before fork so we can detect child's write
				// Fork into background
				exe, _ := os.Executable()
				dbAbs, _ := filepath.Abs(getDBPath())
				child := exec.Command(exe, "serve", "up", "--port", strconv.Itoa(port), "--foreground", "--db", dbAbs)
				devnull, _ := os.Open(os.DevNull)
				child.Stdin = devnull
				child.Stdout = devnull
				child.Stderr = devnull
				child.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
				if err := child.Start(); err != nil {
					outputError(fmt.Sprintf("failed to start server: %v", err))
					return nil
				}
				child.Process.Release()

				// Wait briefly for child to write its PID
				for i := 0; i < 20; i++ {
					if _, err := os.Stat(pidfilePath()); err == nil {
						break
					}
					// sleep 50ms
					sleepMs(50)
				}

				pid, _ := readPid()
				outputJSON(map[string]interface{}{
					"status": "started",
					"pid":    pid,
					"port":   port,
					"url":    fmt.Sprintf("http://localhost:%d", port),
				})
				return nil
			}

			// Foreground mode (called by the forked process)
			os.MkdirAll(filepath.Dir(pidfilePath()), 0755)
			os.WriteFile(pidfilePath(), []byte(strconv.Itoa(os.Getpid())), 0644)

			server.SetWebFS(WebFS)
			return server.Start(database.GetConn(), port)
		},
	}
	upCmd.Flags().Int("port", 8080, "port to listen on")
	upCmd.Flags().Bool("foreground", false, "run in foreground (used internally)")
	upCmd.Flags().MarkHidden("foreground")

	downCmd := &cobra.Command{
		Use:   "down",
		Short: "Stop the web UI server",
		RunE: func(cmd *cobra.Command, args []string) error {
			pid, err := readPid()
			if err != nil {
				outputError("server is not running")
				return nil
			}

			if !processRunning(pid) {
				os.Remove(pidfilePath())
				outputError("server is not running (stale pidfile cleaned up)")
				return nil
			}

			proc, err := os.FindProcess(pid)
			if err != nil {
				outputError(fmt.Sprintf("failed to find process: %v", err))
				return nil
			}

			if err := proc.Signal(syscall.SIGTERM); err != nil {
				outputError(fmt.Sprintf("failed to stop server: %v", err))
				return nil
			}

			os.Remove(pidfilePath())
			outputJSON(map[string]interface{}{
				"status": "stopped",
				"pid":    pid,
			})
			return nil
		},
	}

	serveCmd.AddCommand(upCmd, downCmd)
	return serveCmd
}

func readPid() (int, error) {
	data, err := os.ReadFile(pidfilePath())
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(data)))
}

func sleepMs(ms int) {
	time.Sleep(time.Duration(ms) * time.Millisecond)
}

func processRunning(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = proc.Signal(syscall.Signal(0))
	return err == nil
}
