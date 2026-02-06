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

// tmuxSemaphore limits concurrent tmux operations to prevent system overload
// Max 50 concurrent tmux operations allowed
var tmuxSemaphore = make(chan struct{}, 50)

// CapturePane captures the last N lines from a tmux session
func CapturePane(sessionName string, lines int) (string, error) {
	// Acquire semaphore slot (blocks if 50 operations are already running)
	tmuxSemaphore <- struct{}{}
	defer func() { <-tmuxSemaphore }()

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
	// Comprehensive ANSI escape code regex pattern
	// Handles colors, cursor movements, and other CSI/OSC sequences
	const ansi = "[\u001B\u009B][[()#;?]*(?:[0-9]{1,4}(?:;[0-9]{0,4})*)?[0-9A-ORZcf-nqrtuy=><]"
	re := regexp.MustCompile(ansi)
	return re.ReplaceAllString(input, "")
}

// IsSessionAlive checks if a tmux session exists and is running
func IsSessionAlive(sessionName string) bool {
	cmd := exec.Command("tmux", "has-session", "-t", sessionName)
	err := cmd.Run()
	// tmux has-session returns 0 if session exists, non-zero otherwise
	return err == nil
}

// isTmuxKeyName checks if the input is a tmux key name (e.g., "C-[", "C-i", "C-c")
// Key names should not be sent with the -l (literal) flag.
func isTmuxKeyName(input string) bool {
	// Check for tmux key prefix patterns
	keyPrefixes := []string{
		"C-",   // Control keys (C-a, C-c, C-m, etc.)
		"M-",   // Meta/Alt keys
		"C-S-", // Control+Shift combinations
	}

	for _, prefix := range keyPrefixes {
		if strings.HasPrefix(input, prefix) {
			return true
		}
	}

	return false
}

// isLiteralKeySequence checks if the input is a literal key sequence (e.g., "\x1b[Z" for Shift+Tab)
// These should be sent with the -l flag but without the trailing Enter key.
func isLiteralKeySequence(input string) bool {
	// Check for ANSI escape sequences (starts with \x1b)
	return strings.Contains(input, "\x1b")
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

	// Determine input type:
	// 1. Tmux key name (e.g., "C-[", "C-i") → send without -l flag, no Enter
	// 2. Literal key sequence (e.g., "\x1b[Z" for Shift+Tab) → send with -l flag, no Enter
	// 3. Regular text → send with -l flag, with Enter

	isKeyName := isTmuxKeyName(input)
	isLiteralSeq := isLiteralKeySequence(input)

	// Step 1: Send the input text
	var args1 []string
	if isKeyName {
		// Key names should NOT use -l flag
		args1 = []string{"send-keys", "-t", sessionName, input}
	} else {
		// Regular text and literal sequences use -l flag
		args1 = []string{"send-keys", "-t", sessionName, "-l", input}
	}

	cmd1 := exec.Command("tmux", args1...)
	if output, err := cmd1.CombinedOutput(); err != nil {
		logger.WithFields(logrus.Fields{
			"session": sessionName,
			"error":   err,
			"output":  string(output),
		}).Error("failed-to-send-input-to-tmux-session")
		return fmt.Errorf("failed to send input to session %s: %w (output: %s)", sessionName, err, string(output))
	}

	// Step 2: Send Enter key (C-m) - only for regular text input
	// Key names and literal key sequences are already complete
	if !isKeyName && !isLiteralSeq {
		// Delay before Enter key if specified
		if delay > 0 {
			time.Sleep(time.Duration(delay) * time.Millisecond)
		}

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
