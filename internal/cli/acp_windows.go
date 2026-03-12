//go:build windows

package cli

import (
	"os/exec"
	"strconv"

	"github.com/keepmind9/clibot/internal/logger"
)

// killProcess terminates the ACP server process on Windows
func (a *ACPAdapter) killProcess(sess *acpSession) error {
	if !sess.isRemote && sess.cmd != nil && sess.cmd.Process != nil {
		logger.WithField("session", sess.workDir).Info("killing-acp-process")

		// Windows: Kill entire process tree using taskkill
		// This is necessary because 'cmd /c' starts a child process
		// and Kill() only kills the shell, not the child.
		pidStr := strconv.Itoa(sess.cmd.Process.Pid)
		killCmd := exec.Command("taskkill", "/F", "/T", "/PID", pidStr)
		killErr := killCmd.Run()

		if killErr != nil {
			logger.WithField("error", killErr).Warn("failed-to-kill-acp-process-tree")
			// Fallback to basic kill
			sess.cmd.Process.Kill()
		}

		// Wait for process to exit
		sess.cmd.Wait()
		sess.cmd = nil
	}

	return nil
}
