package cli

import (
	"bufio"
	"encoding/json"
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
// This is the fallback method when hook doesn't provide transcript_path
func (c *ClaudeAdapter) GetLastResponse(sessionName string) (string, error) {
	// Try to get from conversation history files (fallback)
	content, err := GetLastAssistantContent(c.historyDir)
	if err != nil {
		return "", fmt.Errorf("failed to get last response: %w", err)
	}

	return content, nil
}

// HandleHookData processes hook data from Claude Code Stop hook
// Expected data format: {"transcript_path": "/path/to/transcript.jsonl", ...}
func (c *ClaudeAdapter) HandleHookData(data map[string]interface{}) (string, error) {
	// Extract transcript_path from hook data
	transcriptPath, ok := data["transcript_path"].(string)
	if !ok {
		return "", fmt.Errorf("missing transcript_path in hook data")
	}

	// Use transcript_path to get response
	return c.GetLastResponseFromTranscript(transcriptPath)
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

// ========== Transcript.jsonl Parsing ==========

// TranscriptMessage represents a single message in Claude Code's transcript.jsonl
// Each line in the file is a JSON object with this structure
type TranscriptMessage struct {
	Type    string         `json:"type"` // "user" or "assistant"
	Message MessageContent `json:"message"`
}

// MessageContent represents the message content structure
type MessageContent struct {
	Content []ContentBlock `json:"content"`
}

// ContentBlock represents a block of content (text, image, etc.)
type ContentBlock struct {
	Type string `json:"type"` // "text", "image", etc.
	Text string `json:"text,omitempty"`
}

// ParseTranscript parses a Claude Code transcript.jsonl file
// Returns all messages in order
func ParseTranscript(filePath string) ([]TranscriptMessage, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open transcript file: %w", err)
	}
	defer file.Close()

	var messages []TranscriptMessage
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var msg TranscriptMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			// Skip invalid lines
			continue
		}

		messages = append(messages, msg)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading transcript file: %w", err)
	}

	return messages, nil
}

// ExtractLastAssistantResponse extracts all assistant messages after the last user message
// This matches the logic from claudecode-telegram's send-to-telegram.sh hook
func ExtractLastAssistantResponse(transcriptPath string) (string, error) {
	// Parse transcript file
	messages, err := ParseTranscript(transcriptPath)
	if err != nil {
		return "", err
	}

	// Find the last user message index
	lastUserIndex := -1
	for i, msg := range messages {
		if msg.Type == "user" {
			lastUserIndex = i
		}
	}

	if lastUserIndex == -1 {
		return "", fmt.Errorf("no user messages found in transcript")
	}

	// Extract all assistant messages after the last user message
	var responseTexts []string
	for i := lastUserIndex + 1; i < len(messages); i++ {
		if messages[i].Type == "assistant" {
			// Extract all text content blocks
			for _, block := range messages[i].Message.Content {
				if block.Type == "text" && block.Text != "" {
					responseTexts = append(responseTexts, block.Text)
				}
			}
		}
	}

	if len(responseTexts) == 0 {
		return "", fmt.Errorf("no assistant responses found after last user message")
	}

	// Join all response texts with double newlines (matching Claude's output format)
	return strings.Join(responseTexts, "\n\n"), nil
}

// GetLastResponseFromTranscript retrieves response from transcript.jsonl
// This is called when hook provides transcript_path
func (c *ClaudeAdapter) GetLastResponseFromTranscript(transcriptPath string) (string, error) {
	// Expand home directory in path
	transcriptPath = expandHome(transcriptPath)

	// Extract response from transcript
	response, err := ExtractLastAssistantResponse(transcriptPath)
	if err != nil {
		return "", fmt.Errorf("failed to extract response from transcript: %w", err)
	}

	return response, nil
}
