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
	"github.com/keepmind9/clibot/pkg/constants"
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
		Timeout:     1 * time.Hour, // Safety fallback - actual completion determined by stable_count
	}
}

// WaitForCompletion waits for CLI output to become stable.
//
// Parameters:
//   - sessionName: tmux session name
//   - inputs: List of historical user inputs (newest first)
//   - beforeContent: 发送指令前的屏幕快照 (用于定位锚点，防止回显缺失)
//   - config: polling configuration
//   - ctx: context for cancellation
//
// Returns:
//   - response: Extracted NEW content from the AI
//   - rawContent: Full raw output from tmux (used for snapshots)
//   - error: Error if any
func WaitForCompletion(sessionName string, inputs []InputRecord, beforeContent string, config PollingConfig, ctx context.Context) (string, string, error) {
	// ... (Apply defaults logic same as before)
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
		return "", "", fmt.Errorf("StableCount must be between 1 and 100, got %d", config.StableCount)
	}

	// Set default for CaptureLines
	if config.CaptureLines == 0 {
		config.CaptureLines = constants.StatusCheckLines
	}

	logger.WithFields(logrus.Fields{
		"session":      sessionName,
		"interval":     config.Interval,
		"stableCount":  config.StableCount,
		"timeout":      config.Timeout,
		"captureLines": config.CaptureLines,
	}).Info("polling-wait-for-completion-started")

	ticker := time.NewTicker(config.Interval)
	defer ticker.Stop()

	timeout := time.NewTimer(config.Timeout)
	defer timeout.Stop()

	var lastContent string
	var stableTimes int
	var consecutiveErrors int
	const maxConsecutiveErrors = 10

	emptyThreshold := config.StableCount + 2
	if emptyThreshold < 5 {
		emptyThreshold = 5
	}

	for {
		select {
		case <-ctx.Done():
			logger.Info("polling-cancelled-by-context")
			return "", "", ErrCancelled

		case <-timeout.C:
			logger.WithField("session", sessionName).Warn("polling-timeout")
			if lastContent != "" {
				return lastContent, "", nil
			}
			return "", "", ErrTimeout

		case <-ticker.C:
			select {
			case <-ctx.Done():
				return "", "", ErrCancelled
			default:
			}

			output, err := CapturePane(sessionName, config.CaptureLines)
			if err != nil {
				consecutiveErrors++
				if consecutiveErrors > maxConsecutiveErrors {
					return "", "", fmt.Errorf("tmux capture failed repeatedly after %d attempts: %w", consecutiveErrors, err)
				}
				continue
			}
			consecutiveErrors = 0

			// UNIFIED EXTRACTION: Identify new content using Prompt and Snapshot
			// For stability check, we use the small capture (10 lines)
			currentContent := ExtractNewContentWithHistory(output, inputs, beforeContent)

			if isPollingCompleted(currentContent, lastContent) {
				if currentContent != "" {
					stableTimes++
					if stableTimes >= config.StableCount {
						logger.WithFields(logrus.Fields{
							"session":        sessionName,
							"content_length": len(currentContent),
						}).Info("polling-completed-detecting-stability")

						// CRITICAL FIX: Once stable, capture a LARGER window (200 lines)
						// to ensure we can find the prompt and extract the FULL response.
						fullOutput, err := CapturePane(sessionName, constants.SnapshotCaptureLines)
						if err != nil {
							logger.Warn("failed-to-capture-full-output-using-small-capture")
							return currentContent, output, nil
						}

						// Perform final extraction on the large window
						finalResponse := ExtractNewContentWithHistory(fullOutput, inputs, beforeContent)
						// Return both the extracted response AND the full raw output for snapshot saving
						return finalResponse, fullOutput, nil
					}
				} else {
					stableTimes++
					if stableTimes > emptyThreshold {
						return "", "", ErrEmpty
					}
				}
			} else {
				stableTimes = 0
				lastContent = currentContent
			}
		}
	}
}

// ExtractStableContent extracts content suitable for stability comparison.
func ExtractStableContent(output string) string {
	clean := StripANSI(output)
	clean = strings.TrimSpace(clean)
	return clean
}

// isPollingCompleted determines if polling should be considered complete.
func isPollingCompleted(current, last string) bool {
	if current == last {
		return true
	}
	if IsThinking(current) {
		return false
	}
	if isMenuMode(current) {
		sim := calculateSimilarity(current, last)
		if sim >= 0.90 {
			return true
		}
	}
	return false
}

// isMenuMode checks if the content shows a menu with numbered options.
func isMenuMode(content string) bool {
	isMenu := hasNumberedOptions(content)
	if isMenu {
		logger.WithField("content", content).Debug("detected-as-menu-mode")
	}
	return isMenu
}

// hasNumberedOptions checks if content contains numbered menu options.
func hasNumberedOptions(content string) bool {
	lines := strings.Split(content, "\n")
	numberedLines := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if len(trimmed) < 2 {
			continue
		}

		firstChar := trimmed[0]
		if firstChar >= '0' && firstChar <= '9' {
			if isUIStatusLine(trimmed) {
				continue
			}

			rest := trimmed[1:]
			if strings.HasPrefix(rest, ".") ||
				strings.HasPrefix(rest, ")") ||
				strings.HasPrefix(rest, " ") ||
				strings.HasPrefix(rest, "、") {
				numberedLines++
			}
		}
	}
	return numberedLines >= 2
}

// calculateSimilarity calculates the line-based similarity between two strings.
func calculateSimilarity(a, b string) float64 {
	linesA := strings.Split(a, "\n")
	linesB := strings.Split(b, "\n")
	maxLines := len(linesA)
	if len(linesB) > maxLines {
		maxLines = len(linesB)
	}
	if maxLines == 0 {
		return 1.0
	}
	matchedLines := 0
	for i := 0; i < maxLines; i++ {
		if i < len(linesA) && i < len(linesB) {
			if linesA[i] == linesB[i] {
				matchedLines++
			}
		}
	}
	return float64(matchedLines) / float64(maxLines)
}
