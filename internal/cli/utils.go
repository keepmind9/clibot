package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
)

// expandHome expands ~ to user's home directory
// This is a shared utility function used across multiple CLI adapters
// Returns an error if the home directory cannot be determined
func expandHome(path string) (string, error) {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get home directory: %w", err)
		}
		return filepath.Join(home, path[2:]), nil
	}
	return path, nil
}

// buildShellCommand creates a cross-platform shell command
// For Linux/macOS (including WSL2): sh -c "command"
// For Windows (native, though not officially supported): cmd /c "command"
//
// Important: Sets process group ID on Unix-like systems to allow
// killing the entire process tree (shell + children) when needed.
func buildShellCommand(command string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		// Windows native: use cmd /c (not officially supported)
		return exec.Command("cmd", "/c", command)
	}
	// Linux/macOS (including WSL2): use sh -c with process group
	cmd := exec.Command("/bin/sh", "-c", command)

	// Set process group ID to allow killing entire process tree
	// This ensures that when we kill the shell process, all its
	// child processes (like claude-agent-acp) are also killed.
	attrs := &syscall.SysProcAttr{}
	setSetpgid(attrs)
	setPdeathsig(attrs)

	cmd.SysProcAttr = attrs
	return cmd
}
