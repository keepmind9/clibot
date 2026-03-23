package cli

import (
	"fmt"
	"os/exec"

	"github.com/keepmind9/clibot/internal/logger"
	"github.com/keepmind9/clibot/internal/watchdog"
	"github.com/sirupsen/logrus"
)

// BaseAdapter provides common fields and methods for CLI adapters
type BaseAdapter struct {
	cliName      string
	startCmd     string
	inputDelayMs int
}

// NewBaseAdapter creates a new BaseAdapter
func NewBaseAdapter(name, startCmd string, delay int) BaseAdapter {
	return BaseAdapter{
		cliName:      name,
		startCmd:     startCmd,
		inputDelayMs: delay,
	}
}

func (b *BaseAdapter) IsSessionAlive(sessionName string) bool {
	return watchdog.IsSessionAlive(sessionName)
}

// CreateSession creates a new tmux session and starts the CLI
// This method is idempotent: if the session already exists, it returns successfully
// The transportURL parameter is ignored by base adapters (only used by ACP)
func (b *BaseAdapter) CreateSession(sessionName, workDir, startCmd, transportURL string, env map[string]string) error {
	// Idempotency check: if session already exists, return success
	if b.IsSessionAlive(sessionName) {
		logger.WithField("session", sessionName).Info("session already exists, skipping creation")
		return nil
	}

	// Create tmux session
	args := []string{"new-session", "-d", "-s", sessionName}

	// Set working directory if specified
	if workDir != "" {
		expandedDir, err := expandHome(workDir)
		if err != nil {
			return fmt.Errorf("session %s: invalid work_dir: %w", sessionName, err)
		}

		// Check if directory exists
		if _, err := exec.Command("test", "-d", expandedDir).CombinedOutput(); err != nil {
			return fmt.Errorf("session %s: work_dir does not exist: %s", sessionName, expandedDir)
		}

		args = append(args, "-c", expandedDir)
	}

	cmd := exec.Command("tmux", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("session %s: failed to create tmux session: %w (output: %s)", sessionName, err, string(output))
	}

	// Set session-level environment variables (inherited by CLI process)
	for k, v := range env {
		setEnvCmd := exec.Command("tmux", "set-environment", "-t", sessionName, k, v)
		if output, err := setEnvCmd.CombinedOutput(); err != nil {
			return fmt.Errorf("session %s: failed to set env %s: %w (output: %s)", sessionName, k, err, string(output))
		}
	}

	// Start the CLI in the session with the specified command
	if err := b.Start(sessionName, startCmd); err != nil {
		return fmt.Errorf("session %s: failed to start %s: %w", sessionName, b.cliName, err)
	}

	return nil
}

// Start starts the CLI in the specified tmux session
func (b *BaseAdapter) Start(sessionName, startCmd string) error {
	if startCmd == "" {
		startCmd = b.startCmd
	}
	logger.WithFields(logrus.Fields{
		"session":   sessionName,
		"cli":       b.cliName,
		"start_cmd": startCmd,
	}).Info("starting-cli-in-tmux-session")

	if err := watchdog.SendKeys(sessionName, startCmd); err != nil {
		return err
	}

	return nil
}

// SendInput sends input to the CLI via tmux
func (b *BaseAdapter) SendInput(sessionName, input string) error {
	logger.WithFields(logrus.Fields{
		"session": sessionName,
		"cli":     b.cliName,
		"input":   input,
		"delay":   b.inputDelayMs,
	}).Debug("sending-input-to-tmux-session")

	if err := watchdog.SendKeys(sessionName, input, b.inputDelayMs); err != nil {
		logger.Errorf("failed to send input to tmux: %v", err)
		return err
	}
	return nil
}
