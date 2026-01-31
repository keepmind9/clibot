// Package watchdog provides tmux session monitoring and output parsing capabilities.
//
// The watchdog package handles two main concerns:
//
// 1. Low-level tmux operations: CapturePane, SendKeys, StripANSI, IsSessionAlive
// 2. Content parsing and filtering: ExtractContentAfterPrompt, IsThinking, RemoveUIStatusLines
//
// # Tmux Operations
//
// This package wraps tmux commands for session management:
//
//   - CapturePane: Capture output from a tmux session
//   - SendKeys: Send keystrokes to a tmux session
//   - IsSessionAlive: Check if a session exists
//   - ListSessions: List all active sessions
//
// # Content Parsing
//
// The parser provides utilities for extracting relevant content from tmux output:
//
//   - ExtractContentAfterPrompt: Filters tmux output to show only the latest AI response
//   - IsThinking: Detects if the AI is still processing (shows "thinking" indicators)
//   - RemoveUIStatusLines: Removes UI artifacts like "ESC to interrupt"
//   - StripANSI: Removes ANSI escape codes for clean text
//
// # Example Usage
//
//   // Capture and parse tmux output
//   output, err := watchdog.CapturePane("my-session", 100)
//   if err != nil {
//       log.Fatal(err)
//   }
//   clean := watchdog.StripANSI(output)
//   filtered := watchdog.ExtractContentAfterPrompt(clean, "my prompt")
//
package watchdog

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/keepmind9/clibot/internal/logger"
	"github.com/sirupsen/logrus"
)

// CapturePane captures the last N lines from a tmux session
func CapturePane(sessionName string, lines int) (string, error) {
	// Build tmux command to capture pane
	// Use -S flag for start line (negative means from end)
	// Format: -S -N captures N lines from the end
	var cmd *exec.Cmd
	if lines > 0 {
		cmd = exec.Command("tmux", "capture-pane", "-t", sessionName, "-p", "-e", "-S", fmt.Sprintf("-%d", lines))
	} else {
		// Capture all lines if lines is 0 or negative
		cmd = exec.Command("tmux", "capture-pane", "-t", sessionName, "-p", "-e", "-S", "-")
	}

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to capture pane from session %s: %w", sessionName, err)
	}

	return string(output), nil
}

// StripANSI removes ANSI escape codes from a string
func StripANSI(input string) string {
	// ANSI escape code regex pattern
	ansiEscape := regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)
	return ansiEscape.ReplaceAllString(input, "")
}

// IsSessionAlive checks if a tmux session exists and is running
func IsSessionAlive(sessionName string) bool {
	cmd := exec.Command("tmux", "has-session", "-t", sessionName)
	err := cmd.Run()
	// tmux has-session returns 0 if session exists, non-zero otherwise
	return err == nil
}

// SendKeys sends keystrokes to a tmux session
// Parameters:
//   - sessionName: tmux session name
//   - input: text to send
//   - delayMs: delay in milliseconds before sending Enter key (default 0)
func SendKeys(sessionName, input string, delayMs ...int) error {
	delay := 0
	if len(delayMs) > 0 {
		delay = delayMs[0]
	}

	logger.WithFields(logrus.Fields{
		"session":  sessionName,
		"input":    input,
		"delay_ms": delay,
	}).Debug("sending-keys-to-tmux-session")

	// Step 1: Send the input text
	args1 := []string{"send-keys", "-t", sessionName, "-l", input}
	cmd1 := exec.Command("tmux", args1...)
	if output, err := cmd1.CombinedOutput(); err != nil {
		logger.WithFields(logrus.Fields{
			"session": sessionName,
			"error":   err,
			"output":  string(output),
		}).Error("failed-to-send-text-to-tmux-session")
		return fmt.Errorf("failed to send text to session %s: %w (output: %s)", sessionName, err, string(output))
	}

	// Delay before Enter key if specified
	if delay > 0 {
		time.Sleep(time.Duration(delay) * time.Millisecond)
	}

	// Step 2: Send Enter key (C-m)
	args2 := []string{"send-keys", "-t", sessionName, "C-m"}
	cmd2 := exec.Command("tmux", args2...)
	if output, err := cmd2.CombinedOutput(); err != nil {
		logger.WithFields(logrus.Fields{
			"session": sessionName,
			"error":   err,
			"output":  string(output),
		}).Error("failed-to-send-enter-key-to-tmux-session")
		return fmt.Errorf("failed to send Enter to session %s: %w (output: %s)", sessionName, err, string(output))
	}

	return nil
}

// CapturePaneClean captures and strips ANSI codes from tmux output
func CapturePaneClean(sessionName string, lines int) (string, error) {
	output, err := CapturePane(sessionName, lines)
	if err != nil {
		return "", err
	}
	return StripANSI(output), nil
}

// ListSessions returns a list of all tmux session names
func ListSessions() ([]string, error) {
	cmd := exec.Command("tmux", "list-sessions", "-F", "#{session_name}")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list tmux sessions: %w", err)
	}

	sessions := strings.Split(strings.TrimSpace(string(output)), "\n")
	// Handle case with no sessions
	if len(sessions) == 1 && sessions[0] == "" {
		return []string{}, nil
	}

	return sessions, nil
}
