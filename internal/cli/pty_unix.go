//go:build linux || darwin

package cli

import (
	"os/exec"
	"syscall"
)

// killProcessGroup terminates the entire process group on Unix-like systems.
func killProcessGroup(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	// Send a SIGKILL signal to the entire process group.
	// The negative PID targets the process group.
	return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
}
