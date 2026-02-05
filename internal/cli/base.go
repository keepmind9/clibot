package cli

import (
	"fmt"
	"os/exec"
	"time"

	"github.com/keepmind9/clibot/internal/logger"
	"github.com/keepmind9/clibot/internal/watchdog"
)

// BaseAdapter provides common fields and methods for CLI adapters
type BaseAdapter struct {
	useHook      bool
	pollInterval time.Duration
	stableCount  int
	pollTimeout  time.Duration
}

// NewBaseAdapter creates a new BaseAdapter with normalized polling config
func NewBaseAdapter(useHook bool, interval time.Duration, count int, timeout time.Duration) BaseAdapter {
	// Default to hook mode (true) if not explicitly configured
	if !useHook && interval == 0 && timeout == 0 {
		useHook = true
	}

	pollInterval, stableCount, pollTimeout := normalizePollingConfig(interval, count, timeout)

	return BaseAdapter{
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

// CreateTmuxSession is a shared helper to create a tmux session and run a start command
func (b *BaseAdapter) CreateTmuxSession(sessionName, cliType, workDir string, starter func(string) error) error {
	// Create tmux session
	args := []string{"new-session", "-d", "-s", sessionName}

	// Set working directory if specified
	if workDir != "" {
		expandedDir, err := expandHome(workDir)
		if err != nil {
			return fmt.Errorf("invalid work_dir: %w", err)
		}
		args = append(args, "-c", expandedDir)
	}

	cmd := exec.Command("tmux", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create tmux session %s: %w (output: %s)", sessionName, err, string(output))
	}

	// Start the CLI in the session
	if err := starter(sessionName); err != nil {
		return fmt.Errorf("failed to start %s: %w", cliType, err)
	}

	return nil
}

// SendInputWithDelay is a shared helper for SendInput
func (b *BaseAdapter) SendInputWithDelay(sessionName, input string, delayMs ...int) error {
	if err := watchdog.SendKeys(sessionName, input, delayMs...); err != nil {
		logger.Errorf("failed to send input to tmux: %v", err)
		return err
	}
	return nil
}
