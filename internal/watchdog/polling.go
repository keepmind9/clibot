// Package watchdog provides utilities for tmux session monitoring.
//
// This file implements polling-based completion detection for CLI tools.
// It waits for CLI output to become stable (no changes for N consecutive checks)
// which indicates the AI has finished responding.
package watchdog

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/keepmind9/clibot/internal/logger"
	"github.com/sirupsen/logrus"
)

// Sentinel errors for polling
var (
	ErrCancelled = errors.New("polling cancelled")
	ErrTimeout   = errors.New("polling timeout")
	ErrEmpty     = errors.New("empty output")
)

// PollingConfig defines the polling behavior
type PollingConfig struct {
	Interval     time.Duration // How often to check (default: 1s)
	StableCount  int           // How many consecutive checks must match (default: 3)
	Timeout      time.Duration // Maximum wait time (default: 120s)
	CaptureLines int           // Number of lines to capture from tmux (default: 100)
}

// DefaultPollingConfig returns the default polling configuration
func DefaultPollingConfig() PollingConfig {
	return PollingConfig{
		Interval:    1 * time.Second,
		StableCount: 3,
		Timeout:     120 * time.Second,
	}
}

// WaitForCompletion waits for CLI output to become stable.
//
// It captures tmux output at regular intervals and checks if the content
// remains unchanged for the specified number of consecutive checks.
//
// Parameters:
//   - sessionName: tmux session name
//   - config: polling configuration (use DefaultPollingConfig() for defaults)
//   - ctx: context for cancellation
//
// Returns:
//   - string: the final stable content
//   - error: timeout or cancellation error
//
// Example:
//   config := watchdog.DefaultPollingConfig()
//   content, err := watchdog.WaitForCompletion("my-session", config, ctx)
//   if err != nil {
//       log.Fatal(err)
//   }
func WaitForCompletion(sessionName string, config PollingConfig, ctx context.Context) (string, error) {
	// Apply defaults
	if config.Interval == 0 {
		config.Interval = DefaultPollingConfig().Interval
	}
	if config.StableCount == 0 {
		config.StableCount = DefaultPollingConfig().StableCount
	}
	if config.Timeout == 0 {
		config.Timeout = DefaultPollingConfig().Timeout
	}

	// Validate StableCount to prevent excessive polling
	if config.StableCount < 1 || config.StableCount > 100 {
		return "", fmt.Errorf("StableCount must be between 1 and 100, got %d", config.StableCount)
	}

	// Set default for CaptureLines
	if config.CaptureLines == 0 {
		config.CaptureLines = 100
	}

	logger.WithFields(logrus.Fields{
		"session":     sessionName,
		"interval":    config.Interval,
		"stableCount": config.StableCount,
		"timeout":     config.Timeout,
		"captureLines": config.CaptureLines,
	}).Info("polling-wait-for-completion-started")

	ticker := time.NewTicker(config.Interval)
	defer ticker.Stop()

	// Use NewTimer instead of time.After to avoid resource leak
	// time.After creates a goroutine that isn't cleaned up if we exit early
	timeout := time.NewTimer(config.Timeout)
	defer timeout.Stop()

	var lastContent string
	var stableTimes int
	var consecutiveErrors int
	const maxConsecutiveErrors = 10

	// Calculate empty output threshold based on config
	// Allow a bit more time for output to appear (config.StableCount + 2)
	emptyThreshold := config.StableCount + 2
	if emptyThreshold < 5 {
		emptyThreshold = 5 // Minimum threshold
	}

	for {
		select {
		case <-ctx.Done():
			logger.Info("polling-cancelled-by-context")
			return "", ErrCancelled

		case <-timeout.C:
			logger.WithField("session", sessionName).Warn("polling-timeout")
			if lastContent != "" {
				return lastContent, nil // Return what we have
			}
			return "", ErrTimeout

		case <-ticker.C:
			// Check context again before doing expensive work
			select {
			case <-ctx.Done():
				return "", ErrCancelled
			default:
			}

			// Capture current output
			output, err := CapturePane(sessionName, config.CaptureLines)
			if err != nil {
				consecutiveErrors++
				if consecutiveErrors > maxConsecutiveErrors {
					logger.WithFields(logrus.Fields{
						"session":  sessionName,
						"attempts": consecutiveErrors,
						"error":    err,
					}).Error("capture-pane-failed-too-many-times")
					return "", fmt.Errorf("tmux capture failed repeatedly after %d attempts: %w", consecutiveErrors, err)
				}
				logger.WithFields(logrus.Fields{
					"session":  sessionName,
					"error":    err,
					"attempt":  consecutiveErrors,
				}).Warn("capture-pane-failed-retrying")
				continue
			}

			// Reset error counter on successful capture
			consecutiveErrors = 0

			// Extract stable content for comparison
			currentContent := ExtractStableContent(output)

			// Check if content is stable
			if currentContent == lastContent {
				if currentContent != "" {
					// Normal case: non-empty stable content
					stableTimes++
					logger.WithFields(logrus.Fields{
						"session":     sessionName,
						"stableTimes": stableTimes,
						"threshold":   config.StableCount,
					}).Debug("content-stable")

					if stableTimes >= config.StableCount {
						logger.WithFields(logrus.Fields{
							"session":        sessionName,
							"content_length": len(currentContent),
						}).Info("polling-completed")
						return currentContent, nil
					}
				} else {
					// Empty output case - use calculated threshold
					stableTimes++
					if stableTimes > emptyThreshold {
						logger.WithFields(logrus.Fields{
							"session":  sessionName,
							"threshold": emptyThreshold,
						}).Warn("polling-empty-output-threshold-reached")
						return "", ErrEmpty
					}
					logger.WithFields(logrus.Fields{
						"session":     sessionName,
						"stableTimes": stableTimes,
					}).Debug("content-stable-empty")
				}
			} else {
				// Content changed, reset counter
				stableTimes = 0
				lastContent = currentContent
				logger.WithFields(logrus.Fields{
					"session":        sessionName,
					"content_length": len(currentContent),
				}).Debug("content-changed-reset-counter")
			}
		}
	}
}

// ExtractStableContent extracts content suitable for stability comparison.
//
// It removes:
// - ANSI escape codes
// - UI status lines (scrollbars, progress indicators)
// - Leading/trailing whitespace
//
// This ensures we're comparing meaningful content only.
func ExtractStableContent(output string) string {
	// Remove ANSI codes
	clean := StripANSI(output)

	// Remove UI status lines
	clean = RemoveUIStatusLines(clean)

	// Trim whitespace
	clean = strings.TrimSpace(clean)

	return clean
}
