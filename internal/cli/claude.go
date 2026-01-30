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

	"github.com/keepmind9/clibot/internal/logger"
	"github.com/keepmind9/clibot/internal/watchdog"
	"github.com/sirupsen/logrus"
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
	logger.WithFields(logrus.Fields{
		"session": sessionName,
		"input":   input,
		"length":  len(input),
	}).Debug("Sending input to tmux session")

	if err := watchdog.SendKeys(sessionName, input); err != nil {
		logger.WithFields(logrus.Fields{
			"session": sessionName,
			"error":   err,
		}).Error("Failed to send input to tmux")
		return err
	}

	return nil
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

// HandleHookData handles raw hook data from Claude Code
// Expected data format (JSON):
//   {"cwd": "/path/to/workdir", "session_id": "...", "transcript_path": "...", ...}
//
// This returns the cwd as the session identifier, which will be matched against
// the configured session's work_dir in the engine.
//
// Parameter data: raw hook data (JSON bytes)
// Returns: (cwd, lastUserPrompt, response, error)
func (c *ClaudeAdapter) HandleHookData(data []byte) (string, string, string, error) {
	// Parse JSON
	var hookData map[string]interface{}
	if err := json.Unmarshal(data, &hookData); err != nil {
		logger.WithField("error", err).Error("Failed to parse hook JSON data")
		return "", "", "", fmt.Errorf("failed to parse JSON data: %w", err)
	}

	// Extract cwd (current working directory) - used to match the tmux session
	cwd, ok := hookData["cwd"].(string)
	if !ok {
		logger.Warn("Missing cwd in hook data")
		return "", "", "", fmt.Errorf("missing cwd in hook data")
	}

	// Extract transcript_path (contains the conversation history)
	transcriptPath, ok := hookData["transcript_path"].(string)
	if !ok {
		logger.Warn("Missing transcript_path in hook data")
		return "", "", "", fmt.Errorf("missing transcript_path in hook data")
	}

	logger.WithFields(logrus.Fields{
		"cwd":             cwd,
		"transcript_path": transcriptPath,
	}).Debug("Hook data parsed")

	// Extract last user prompt for tmux filtering
	lastUserPrompt, err := extractLastUserPrompt(transcriptPath)
	if err != nil {
		logger.WithField("error", err).Debug("Failed to extract last user prompt")
	} else {
		logger.WithField("last_user_prompt", lastUserPrompt).Debug("Extracted last user prompt")
	}

	// Try to extract response from transcript
	response, err := extractFromTranscriptFile(transcriptPath)
	if err != nil {
		// Don't fail the hook - transcript parsing errors are not critical
		logger.WithFields(logrus.Fields{
			"transcript": transcriptPath,
			"error":      err,
		}).Warn("Failed to extract response from transcript")
		return cwd, lastUserPrompt, "", nil
	}

	logger.WithFields(logrus.Fields{
		"cwd":          cwd,
		"response_len": len(response),
	}).Info("Response extracted from transcript")

	return cwd, lastUserPrompt, response, nil
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
	Type    string         `json:"type"` // "user", "assistant", "progress", etc.
	Message MessageContent `json:"message"`
}

// MessageContent represents the message content structure
// Note: content can be either a string (user messages) or an array (assistant messages)
type MessageContent struct {
	ID          string         `json:"id,omitempty"`
	Type        string         `json:"type,omitempty"`         // "message" for assistant
	Role        string         `json:"role,omitempty"`         // "user" or "assistant"
	Model       string         `json:"model,omitempty"`        // Model name
	Content     []ContentBlock `json:"content,omitempty"`
	ContentText string         `json:"-"`                      // Extracted when content is a string
	StopReason  string         `json:"stop_reason,omitempty"`  // null if incomplete, "end_turn"/"max_tokens" if complete
}

// UnmarshalJSON implements custom JSON unmarshaling for MessageContent
func (mc *MessageContent) UnmarshalJSON(data []byte) error {
	// Try to unmarshal as full message object first
	var full struct {
		ID          string `json:"id"`
		Type        string `json:"type"`
		Role        string `json:"role"`
		Model       string `json:"model"`
		Content     interface{} `json:"content"`
		StopReason  string `json:"stop_reason"`
		StopSequence string `json:"stop_sequence"`
		Usage       map[string]interface{} `json:"usage"`
	}
	if err := json.Unmarshal(data, &full); err == nil {
		mc.ID = full.ID
		mc.Type = full.Type
		mc.Role = full.Role
		mc.Model = full.Model
		mc.StopReason = full.StopReason

		// Handle content field (can be string or array)
		switch v := full.Content.(type) {
		case string:
			mc.ContentText = v
		case []interface{}:
			// Convert []interface{} to []ContentBlock
			contentJSON, _ := json.Marshal(v)
			json.Unmarshal(contentJSON, &mc.Content)
		}
		return nil
	}

	// Fallback: try to unmarshal as string (for simple user messages)
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		mc.ContentText = str
		return nil
	}

	// Fallback: try to unmarshal as array (shouldn't happen but just in case)
	var arr []ContentBlock
	if err := json.Unmarshal(data, &arr); err == nil {
		mc.Content = arr
		return nil
	}

	return fmt.Errorf("content is neither a message object, string, nor array")
}

// ContentBlock represents a block of content (text, thinking, image, etc.)
type ContentBlock struct {
	Type     string `json:"type"` // "text", "thinking", "image", etc.
	Text     string `json:"text,omitempty"`
	Thinking string `json:"thinking,omitempty"`
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

		// Parse type first to filter out non-message lines
		var typeCheck struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal([]byte(line), &typeCheck); err != nil {
			continue
		}

		// Only process user and assistant messages
		if typeCheck.Type != "user" && typeCheck.Type != "assistant" {
			continue
		}

		var msg TranscriptMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			// Skip invalid lines but log for debugging
			fmt.Printf("Warning: failed to parse line (type=%s): %v\n", typeCheck.Type, err)
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
//
// Currently only attempts to parse from transcript file.
// TODO: Add tmux fallback with session name
//
// If no text response is found (e.g., assistant is still thinking), returns empty string.
func ExtractLastAssistantResponse(transcriptPath string) (string, error) {
	logger.WithField("transcript", transcriptPath).Debug("Starting response extraction from transcript")

	return extractFromTranscriptFile(transcriptPath)
}

// extractFromTranscriptFile tries to extract response from transcript file
func extractFromTranscriptFile(transcriptPath string) (string, error) {
	logger.WithField("transcript", transcriptPath).Debug("Parsing transcript file")

	messages, err := ParseTranscript(transcriptPath)
	if err != nil {
		logger.WithField("error", err).Debug("Failed to parse transcript")
		return "", fmt.Errorf("failed to parse transcript: %w", err)
	}

	logger.WithField("message_count", len(messages)).Debug("Parsed transcript messages")

	// Find the last user message index
	lastUserIndex := -1
	for i, msg := range messages {
		if msg.Type == "user" {
			lastUserIndex = i
		}
	}

	logger.WithField("last_user_index", lastUserIndex).Debug("Found last user message")

	if lastUserIndex == -1 {
		return "", fmt.Errorf("no user messages found in transcript")
	}

	// Extract all assistant text responses after the last user message
	var responseTexts []string
	assistantMessageCount := 0

	for i := lastUserIndex + 1; i < len(messages); i++ {
		if messages[i].Type == "assistant" {
			assistantMessageCount++
			for _, block := range messages[i].Message.Content {
				if block.Type == "text" && block.Text != "" {
					responseTexts = append(responseTexts, block.Text)
					logger.WithFields(logrus.Fields{
						"block_type": block.Type,
						"text_length": len(block.Text),
					}).Debug("Found text block in assistant message")
				}
			}
		}
	}

	logger.WithFields(logrus.Fields{
		"assistant_messages": assistantMessageCount,
		"text_blocks":        len(responseTexts),
	}).Debug("Extracted text blocks from assistant messages")

	if len(responseTexts) == 0 {
		logger.Debug("No text responses found in transcript")
		return "", fmt.Errorf("no text responses found in transcript")
	}

	result := strings.Join(responseTexts, "\n\n")
	logger.WithField("joined_length", len(result)).Debug("Joined response texts")

	return result, nil
}

// extractFromTmux captures response from tmux session (fallback method)
func extractFromTmux(sessionName string) (string, error) {
	logger.WithField("session", sessionName).Debug("Capturing tmux pane")

	// Capture the last 200 lines from tmux session
	output, err := watchdog.CapturePane(sessionName, 200)
	if err != nil {
		logger.WithField("error", err).Error("Failed to capture tmux pane")
		return "", fmt.Errorf("failed to capture tmux pane: %w", err)
	}

	logger.WithFields(logrus.Fields{
		"raw_length":    len(output),
		"raw_preview":   output[:min(500, len(output))],
	}).Debug("Captured raw tmux output")

	// Clean ANSI codes
	cleanOutput := watchdog.StripANSI(output)

	logger.WithField("cleaned_length", len(cleanOutput)).Debug("Cleaned ANSI codes")

	// Simple heuristic: extract the last assistant response
	lines := strings.Split(cleanOutput, "\n")

	// Filter out empty lines and prompts
	var contentLines []string
	filteredCount := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Skip empty lines
		if trimmed == "" {
			continue
		}

		// Skip user prompts and common CLI patterns
		if isPromptOrCommand(trimmed) {
			filteredCount++
			logger.WithField("filtered_line", trimmed).Debug("Filtered prompt/command")
			continue
		}

		contentLines = append(contentLines, trimmed)
	}

	logger.WithFields(logrus.Fields{
		"total_lines":      len(lines),
		"content_lines":    len(contentLines),
		"filtered_count":   filteredCount,
	}).Debug("Processed tmux lines")

	if len(contentLines) == 0 {
		logger.Warn("No content found in tmux capture after filtering")
		return "", fmt.Errorf("no content found in tmux capture")
	}

	// Join and return
	response := strings.Join(contentLines, "\n")
	logger.WithField("final_length", len(response)).Debug("Constructed final response from tmux")

	return response, nil
}


// isPromptOrCommand checks if a line is a prompt/command rather than assistant output
func isPromptOrCommand(line string) bool {
	// Common CLI patterns that are not assistant output
	promptPatterns := []string{
		"user@",
		"$ ",
		">>>",
		"...",
		"\\[?\\]",
		"Press Enter",
		"Confirm?",
	}

	for _, pattern := range promptPatterns {
		if strings.Contains(line, pattern) {
			return true
		}
	}

	return false
}
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

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}


// extractLastUserPrompt extracts the last user's prompt from transcript
// This is used to filter tmux output to only show the latest response
func extractLastUserPrompt(transcriptPath string) (string, error) {
	transcriptPath = expandHome(transcriptPath)

	messages, err := ParseTranscript(transcriptPath)
	if err != nil {
		return "", fmt.Errorf("failed to parse transcript: %w", err)
	}

	// Find the last user message
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Type == "user" {
			// Get the user's content
			if messages[i].Message.ContentText != "" {
				return messages[i].Message.ContentText, nil
			}
			// Try extracting from content array
			for _, block := range messages[i].Message.Content {
				if block.Type == "text" && block.Text != "" {
					return block.Text, nil
				}
			}
		}
	}

	return "", fmt.Errorf("no user message found in transcript")
}
