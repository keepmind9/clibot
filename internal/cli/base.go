package cli

import (
	"fmt"
	"os/exec"
	"time"

	"github.com/keepmind9/clibot/internal/logger"
	"github.com/keepmind9/clibot/internal/watchdog"
	"github.com/sirupsen/logrus"
)

// BaseAdapter provides common fields and methods for CLI adapters
type BaseAdapter struct {
	cliName      string
	startCmd     string
	inputDelayMs int
	useHook      bool
	pollInterval time.Duration
	stableCount  int
	pollTimeout  time.Duration
}

// NewBaseAdapter creates a new BaseAdapter with normalized polling config
func NewBaseAdapter(name, startCmd string, delay int, useHook bool, interval time.Duration, count int, timeout time.Duration) BaseAdapter {
	// Default to hook mode (true) if not explicitly configured
	if !useHook && interval == 0 && timeout == 0 {
		useHook = true
	}

	pollInterval, stableCount, pollTimeout := normalizePollingConfig(interval, count, timeout)

	return BaseAdapter{
		cliName:      name,
		startCmd:     startCmd,
		inputDelayMs: delay,
		useHook:      useHook,
		pollInterval: pollInterval,
		stableCount:  stableCount,
		pollTimeout:  pollTimeout,
	}
}

func (b *BaseAdapter) UseHook() bool {
	return b.useHook
}

func (b *BaseAdapter) GetPollInterval() time.Duration {
	return b.pollInterval
}

func (b *BaseAdapter) GetStableCount() int {
	return b.stableCount
}

func (b *BaseAdapter) GetPollTimeout() time.Duration {
	return b.pollTimeout
}

func (b *BaseAdapter) IsSessionAlive(sessionName string) bool {
	return watchdog.IsSessionAlive(sessionName)
}

// CreateSession creates a new tmux session and starts the CLI
func (b *BaseAdapter) CreateSession(sessionName, workDir, startCmd string) error {
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
