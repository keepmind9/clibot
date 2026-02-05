package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/keepmind9/clibot/internal/logger"
	"github.com/keepmind9/clibot/internal/watchdog"
	"github.com/sirupsen/logrus"
)

// ClaudeAdapterConfig configuration for Claude Code adapter
type ClaudeAdapterConfig struct {
	// Polling mode configuration (when UseHook = false)
	UseHook      bool          // Use hook mode (true) or polling mode (false). Default: true
	PollInterval time.Duration // Polling interval. Default: 1s
	StableCount  int           // Consecutive stable checks required. Default: 3
	PollTimeout  time.Duration // Maximum time to wait. Default: 120s
}

// ClaudeAdapter implements CLIAdapter for Claude Code
type ClaudeAdapter struct {
	// Polling mode configuration
	useHook      bool
	pollInterval time.Duration
	stableCount  int
	pollTimeout  time.Duration
}

// NewClaudeAdapter creates a new Claude Code adapter
func NewClaudeAdapter(config ClaudeAdapterConfig) (*ClaudeAdapter, error) {
	// Default to hook mode (true) if not explicitly configured
	// If UseHook is false and polling config is at defaults, assume user wants hook mode
	useHook := config.UseHook
	if !useHook && config.PollInterval == 0 && config.PollTimeout == 0 {
		useHook = true
	}

	pollInterval, stableCount, pollTimeout := normalizePollingConfig(
		config.PollInterval, config.StableCount, config.PollTimeout)

	return &ClaudeAdapter{
		useHook:      useHook,
		pollInterval: pollInterval,
		stableCount:  stableCount,
		pollTimeout:  pollTimeout,
	}, nil
}

// SendInput sends input to Claude Code via tmux
func (c *ClaudeAdapter) SendInput(sessionName, input string) error {
	logger.WithFields(logrus.Fields{
		"session": sessionName,
		"input":   input,
		"length":  len(input),
	}).Debug("sending-input-to-tmux-session")

	if err := watchdog.SendKeys(sessionName, input); err != nil {
		logger.WithFields(logrus.Fields{
			"session": sessionName,
			"error":   err,
		}).Error("failed-to-send-input-to-tmux")
		return err
	}

	return nil
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
	var hookData struct {
		CWD            string `json:"cwd"`
		TranscriptPath string `json:"transcript_path"`
		EventName      string `json:"hook_event_name"`
	}
	if err := json.Unmarshal(data, &hookData); err != nil {
		logger.WithField("error", err).Error("failed-to-parse-hook-json-data")
		return "", "", "", fmt.Errorf("failed to parse JSON data: %w", err)
	}

	if hookData.CWD == "" {
		return "", "", "", fmt.Errorf("missing cwd in hook data")
	}

	logger.WithFields(logrus.Fields{
		"cwd":             hookData.CWD,
		"transcript_path": hookData.TranscriptPath,
		"hook_event_name": hookData.EventName,
	}).Debug("hook-data-parsed")

	var lastUserPrompt, response string
	var err error

	// Extract both prompt and response in one pass if possible
	if hookData.TranscriptPath != "" {
		lastUserPrompt, response, err = extractLatestInteraction(hookData.TranscriptPath)
		if err != nil {
			logger.WithFields(logrus.Fields{
				"transcript": hookData.TranscriptPath,
				"error":      err,
			}).Warn("failed-to-extract-interaction-from-transcript")
		}

		// Clear response for notification events as per original logic
		if strings.EqualFold(hookData.EventName, "Notification") {
			logger.Debug("clearing-response-for-notification-event")
			response = ""
		}
	}

	logger.WithFields(logrus.Fields{
		"cwd":          hookData.CWD,
		"prompt_len":   len(lastUserPrompt),
		"response_len": len(response),
	}).Info("interaction-extracted-from-transcript")

	return hookData.CWD, lastUserPrompt, response, nil
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
		var err error
		workDir, err = expandHome(workDir)
		if err != nil {
			return fmt.Errorf("invalid work_dir: %w", err)
		}
		args = append(args, "-c", workDir)
	}

	cmd := exec.Command("tmux", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create tmux session %s: %w (output: %s)", sessionName, err, string(output))
	}

	// Start Claude Code in the session
	if err := c.start(sessionName); err != nil {
		return fmt.Errorf("failed to start Claude Code: %w", err)
	}

	return nil
}

// UseHook returns whether this adapter uses hook mode (true) or polling mode (false)
func (c *ClaudeAdapter) UseHook() bool {
	return c.useHook
}

// GetPollInterval returns the polling interval for polling mode
func (c *ClaudeAdapter) GetPollInterval() time.Duration {
	return c.pollInterval
}

// GetStableCount returns the number of consecutive stable checks required
func (c *ClaudeAdapter) GetStableCount() int {
	return c.stableCount
}

// GetPollTimeout returns the maximum time to wait for completion
func (c *ClaudeAdapter) GetPollTimeout() time.Duration {
	return c.pollTimeout
}

// start starts Claude Code in the specified tmux session
func (c *ClaudeAdapter) start(sessionName string) error {
	// Send "claude" command to start Claude Code
	if err := watchdog.SendKeys(sessionName, "claude"); err != nil {
		return err
	}

	return nil
}

// ========== Transcript.jsonl Parsing ==========

// TranscriptMessage represents a single message in Claude Code's transcript.jsonl
// Each line in the file is a JSON object with this structure
type TranscriptMessage struct {
	Type      string         `json:"type"` // "user", "assistant", "progress", etc.
	SessionID string         `json:"sessionId"`
	IsMeta    bool           `json:"isMeta"`
	Message   MessageContent `json:"message"`
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

// parseTranscript parses a Claude Code transcript.jsonl file
// Returns all messages in order
func parseTranscript(filePath string) ([]TranscriptMessage, error) {
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

// getMessageText extracts plain text content from a TranscriptMessage
func getMessageText(msg TranscriptMessage) string {
	if msg.Message.ContentText != "" {
		return msg.Message.ContentText
	}
	var texts []string
	for _, block := range msg.Message.Content {
		if block.Type == "text" && block.Text != "" {
			texts = append(texts, block.Text)
		}
	}
	return strings.Join(texts, "\n\n")
}

// isRealUserMessage checks if a message is an actual user prompt
func isRealUserMessage(msg TranscriptMessage) bool {
	if msg.Type != "user" || msg.IsMeta {
		return false
	}

	content := getMessageText(msg)
	if content == "" {
		return false
	}

	// Skip internal command/tool messages
	return !strings.HasPrefix(content, "<local-command-") &&
		!strings.HasPrefix(content, "<command-name>")
}

// extractLatestInteraction extracts the latest user prompt and assistant response
func extractLatestInteraction(transcriptPath string) (string, string, error) {
	path, err := expandHome(transcriptPath)
	if err != nil {
		return "", "", err
	}

	messages, err := parseTranscript(path)
	if err != nil {
		return "", "", err
	}

	// Find the last real user message index
	lastUserIndex := -1
	var sessionId, prompt string
	for i := len(messages) - 1; i >= 0; i-- {
		if isRealUserMessage(messages[i]) {
			lastUserIndex = i
			sessionId = messages[i].SessionID
			prompt = getMessageText(messages[i])
			break
		}
	}

	if lastUserIndex == -1 {
		return "", "", fmt.Errorf("no user messages found")
	}

	// Try subagent first
	subFile, err := extractLatestSubagentFile(path)
	if err == nil {
		subMsgs, err := parseTranscript(subFile)
		if err == nil {
			var responseTexts []string
			for _, m := range subMsgs {
				if m.Type == "assistant" && (m.SessionID == "" || m.SessionID == sessionId) {
					if text := getMessageText(m); text != "" {
						responseTexts = append(responseTexts, text)
					}
				}
			}
			if len(responseTexts) > 0 {
				return prompt, strings.Join(responseTexts, "\n\n"), nil
			}
		}
	}

	// Fallback to main transcript
	var responseTexts []string
	for i := lastUserIndex + 1; i < len(messages); i++ {
		if messages[i].Type == "assistant" && (messages[i].SessionID == "" || messages[i].SessionID == sessionId) {
			if text := getMessageText(messages[i]); text != "" {
				responseTexts = append(responseTexts, text)
			}
		}
	}

	return prompt, strings.Join(responseTexts, "\n\n"), nil
}

// ExtractLatestInteraction exports the latest user prompt and assistant response extraction logic
func ExtractLatestInteraction(transcriptPath string) (string, string, error) {
	return extractLatestInteraction(transcriptPath)
}

// ExtractLastAssistantResponse extracts all assistant messages after the last user message
func ExtractLastAssistantResponse(transcriptPath string) (string, error) {
	_, response, err := extractLatestInteraction(transcriptPath)
	return response, err
}

// extractLatestSubagentFile finds the latest jsonl file in the subagents directory
func extractLatestSubagentFile(transcriptPath string) (string, error) {
	ext := filepath.Ext(transcriptPath)
	base := transcriptPath[:len(transcriptPath)-len(ext)]
	subagentsDir := filepath.Join(base, "subagents")

	if _, err := os.Stat(subagentsDir); err != nil {
		return "", err
	}

	files, err := os.ReadDir(subagentsDir)
	if err != nil {
		return "", err
	}

	var jsonlFiles []os.FileInfo
	for _, f := range files {
		if !f.IsDir() && filepath.Ext(f.Name()) == ".jsonl" {
			info, err := f.Info()
			if err == nil {
				jsonlFiles = append(jsonlFiles, info)
			}
		}
	}

	if len(jsonlFiles) == 0 {
		return "", fmt.Errorf("no jsonl files found in subagents dir")
	}

	// Sort by modification time (newest first)
	sort.Slice(jsonlFiles, func(i, j int) bool {
		return jsonlFiles[i].ModTime().After(jsonlFiles[j].ModTime())
	})

	return filepath.Join(subagentsDir, jsonlFiles[0].Name()), nil
}
