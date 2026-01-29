package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/keepmind9/clibot/internal/watchdog"
)

// ClaudeAdapterConfig configuration for Claude Code adapter
type ClaudeAdapterConfig struct {
	HistoryDir string   // Directory containing conversation JSON files
	CheckLines int      // Number of lines to check for interactive prompts
	Patterns   []string // Regex patterns for interactive prompts
}

// ClaudeAdapter implements CLIAdapter for Claude Code
type ClaudeAdapter struct {
	historyDir string           // Expanded path to conversation history directory
	checkLines int              // Number of lines to check for prompts
	patterns   []*regexp.Regexp // Compiled regex patterns
}

// NewClaudeAdapter creates a new Claude Code adapter
// Returns an error if any of the regex patterns fail to compile
func NewClaudeAdapter(config ClaudeAdapterConfig) (*ClaudeAdapter, error) {
	// Expand home directory in historyDir
	historyDir := expandHome(config.HistoryDir)

	// Compile regex patterns
	patterns := make([]*regexp.Regexp, len(config.Patterns))
	for i, pattern := range config.Patterns {
		compiled, err := regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("failed to compile pattern '%s': %w", pattern, err)
		}
		patterns[i] = compiled
	}

	return &ClaudeAdapter{
		historyDir: historyDir,
		checkLines: config.CheckLines,
		patterns:   patterns,
	}, nil
}

// SendInput sends input to Claude Code via tmux
func (c *ClaudeAdapter) SendInput(sessionName, input string) error {
	return watchdog.SendKeys(sessionName, input)
}

// GetLastResponse retrieves the last assistant response from conversation history
func (c *ClaudeAdapter) GetLastResponse(sessionName string) (string, error) {
	// Get last assistant message content from conversation files
	content, err := GetLastAssistantContent(c.historyDir)
	if err != nil {
		return "", fmt.Errorf("failed to get last response: %w", err)
	}

	return content, nil
}

// IsSessionAlive checks if the tmux session is still running
func (c *ClaudeAdapter) IsSessionAlive(sessionName string) bool {
	return watchdog.IsSessionAlive(sessionName)
}

// CreateSession creates a new tmux session and starts Claude Code
func (c *ClaudeAdapter) CreateSession(sessionName, cliType, workDir string) error {
	// Create tmux session
	args := []string{"new-session", "-d", "-s", sessionName}

	// Set working directory if specified
	if workDir != "" {
		workDir = expandHome(workDir)
		args = append(args, "-c", workDir)
	}

	cmd := exec.Command("tmux", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create tmux session %s: %w (output: %s)", sessionName, err, string(output))
	}

	// Start Claude Code in the session
	if err := c.startClaude(sessionName); err != nil {
		return fmt.Errorf("failed to start Claude Code: %w", err)
	}

	return nil
}

// CheckInteractive checks if Claude Code is waiting for user input
func (c *ClaudeAdapter) CheckInteractive(sessionName string) (bool, string, error) {
	// Capture last N lines from tmux session
	output, err := watchdog.CapturePane(sessionName, c.checkLines)
	if err != nil {
		return false, "", fmt.Errorf("failed to capture pane: %w", err)
	}

	// Split into lines
	lines := strings.Split(output, "\n")

	// Check last N lines for interactive prompts
	// Only check the last checkLines lines to avoid false positives
	startIdx := len(lines) - c.checkLines
	if startIdx < 0 {
		startIdx = 0
	}

	relevantLines := lines[startIdx:]

	// Check each line for patterns
	for _, line := range relevantLines {
		// Strip ANSI codes
		clean := watchdog.StripANSI(line)

		// Check against all patterns
		for _, pattern := range c.patterns {
			if pattern.MatchString(clean) {
				return true, clean, nil
			}
		}
	}

	return false, "", nil
}

// startClaude starts Claude Code in the specified tmux session
func (c *ClaudeAdapter) startClaude(sessionName string) error {
	// Send "claude" command to start Claude Code
	if err := watchdog.SendKeys(sessionName, "claude"); err != nil {
		return err
	}

	return nil
}

// expandHome expands ~ to the user's home directory
func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") || path == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}

		if path == "~" {
			return home
		}

		return filepath.Join(home, path[2:])
	}

	return path
}
