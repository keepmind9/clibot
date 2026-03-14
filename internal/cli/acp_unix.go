//go:build !windows

package cli

import (
	"syscall"

	"github.com/keepmind9/clibot/internal/logger"
)

// killProcess terminates the ACP server process on Unix/Linux/macOS
func (a *ACPAdapter) killProcess(sess *acpSession) error {
	if !sess.isRemote && sess.cmd != nil && sess.cmd.Process != nil {
		logger.WithField("session", sess.workDir).Info("killing-acp-process")

		var killErr error
		// Unix/Linux/macOS: Kill entire process group using negative PID
		// The Setpgid: true in buildShellCommand ensures the process
		// is the process group leader, so -pid kills the entire group
		killErr = syscall.Kill(-sess.cmd.Process.Pid, syscall.SIGKILL)

		if killErr != nil {
			logger.WithField("error", killErr).Warn("failed-to-kill-acp-process")
		}

		// Wait for process to exit
		sess.cmd.Wait()
		sess.cmd = nil
	}

	return nil
}
