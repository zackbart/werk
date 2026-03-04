//go:build windows

package commands

import (
	"os"
	"os/exec"
	"strconv"
)

func setSysProcAttr(cmd *exec.Cmd) {
	// No Setsid equivalent needed on Windows
}

func processRunning(pid int) bool {
	// Use tasklist to check if process exists
	cmd := exec.Command("tasklist", "/FI", "PID eq "+strconv.Itoa(pid), "/NH")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return len(out) > 0 && string(out) != ""
}

func stopProcess(pid int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return proc.Kill()
}
