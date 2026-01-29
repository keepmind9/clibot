package watchdog

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
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
func SendKeys(sessionName, input string) error {
	// Build tmux send-keys command
	// We use C-m to simulate Enter key
	args := []string{"send-keys", "-t", sessionName, input, "C-m"}
	cmd := exec.Command("tmux", args...)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to send keys to session %s: %w (output: %s)", sessionName, err, string(output))
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
