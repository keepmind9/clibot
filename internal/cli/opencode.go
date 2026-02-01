package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/keepmind9/clibot/internal/logger"
	"github.com/keepmind9/clibot/internal/watchdog"
	"github.com/sirupsen/logrus"
)

// OpenCodeAdapterConfig configuration for OpenCode adapter
type OpenCodeAdapterConfig struct {
	// Polling mode configuration (when UseHook = false)
	UseHook      bool          // Use hook mode (true) or polling mode (false). Default: true
	PollInterval time.Duration // Polling interval. Default: 1s
	StableCount  int           // Consecutive stable checks required. Default: 3
	PollTimeout  time.Duration // Maximum time to wait. Default: 120s
}

// OpenCodeAdapter implements CLIAdapter for OpenCode
type OpenCodeAdapter struct {
	// Polling mode configuration
	useHook      bool
	pollInterval time.Duration
	stableCount  int
	pollTimeout  time.Duration
}

// NewOpenCodeAdapter creates a new OpenCode adapter
func NewOpenCodeAdapter(config OpenCodeAdapterConfig) (*OpenCodeAdapter, error) {
	// Default to hook mode (true) if not explicitly configured
	useHook := config.UseHook
	if !useHook && config.PollInterval == 0 && config.PollTimeout == 0 {
		useHook = true
	}

	pollInterval, stableCount, pollTimeout := normalizePollingConfig(
		config.PollInterval, config.StableCount, config.PollTimeout)

	return &OpenCodeAdapter{
		useHook:      useHook,
		pollInterval: pollInterval,
		stableCount:  stableCount,
		pollTimeout:  pollTimeout,
	}, nil
}

// SendInput sends input to OpenCode via tmux
func (o *OpenCodeAdapter) SendInput(sessionName, input string) error {
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

// HandleHookData handles raw hook data from OpenCode
// Expected data format (JSON):
//   {"cwd": "/path/to/workdir", "session_id": "...", "transcript_path": "...", ...}
//
// This returns the cwd as the session identifier, which will be matched against
// the configured session's work_dir in the engine.
//
// Parameter data: raw hook data (JSON bytes)
// Returns: (cwd, lastUserPrompt, response, error)
func (o *OpenCodeAdapter) HandleHookData(data []byte) (string, string, string, error) {
	// Parse JSON
	var hookData map[string]interface{}
	if err := json.Unmarshal(data, &hookData); err != nil {
		logger.WithField("error", err).Error("failed-to-parse-hook-json-data")
		return "", "", "", fmt.Errorf("failed to parse JSON data: %w", err)
	}

	// Extract cwd (current working directory) - used to match the tmux session
	cwd, ok := hookData["cwd"].(string)
	if !ok {
		logger.Warn("missing-cwd-in-hook-data")
		return "", "", "", fmt.Errorf("missing cwd in hook data")
	}

	// Extract transcript_path (contains the conversation history)
	transcriptPath, ok := hookData["transcript_path"].(string)
	if !ok {
		logger.Warn("missing-transcript-path-in-hook-data")
		return "", "", "", fmt.Errorf("missing transcript_path in hook data")
	}

	// Extract hook_event_name to check if this is a notification event
	hookEventName := ""
	if v, ok := hookData["hook_event_name"].(string); ok {
		hookEventName = v
	}

	logger.WithFields(logrus.Fields{
		"cwd":             cwd,
		"transcript_path": transcriptPath,
		"hook_event_name": hookEventName,
	}).Debug("hook-data-parsed")

	var lastUserPrompt string
	var response string
	var err error

	// Only extract response for non-notification events
	// Notification events don't have assistant responses to extract
	if !strings.EqualFold(hookEventName, "Notification") {
		response, err = extractFromOpenCodeTranscript(transcriptPath)
		if err != nil {
			// Don't fail the hook - transcript parsing errors are not critical
			logger.WithFields(logrus.Fields{
				"transcript": transcriptPath,
				"error":      err,
			}).Warn("failed-to-extract-response-from-transcript")
		}
	} else {
		logger.WithField("hook_event_name", hookEventName).Debug("skipping-response-extraction-for-notification-event")
	}

	// Extract user prompt for tmux filtering when response is empty or extraction failed
	if response == "" || err != nil {
		lastUserPrompt, err = extractLastUserPromptFromTranscript(transcriptPath)
		if err != nil {
			logger.WithField("error", err).Debug("failed-to-extract-last-user-prompt")
		} else {
			logger.WithField("last_user_prompt", lastUserPrompt).Debug("extracted-last-user-prompt")
		}
	}

	logger.WithFields(logrus.Fields{
		"cwd":          cwd,
		"response_len": len(response),
	}).Info("response-extracted-from-transcript")

	return cwd, lastUserPrompt, response, nil
}

// IsSessionAlive checks if the tmux session is still running
func (o *OpenCodeAdapter) IsSessionAlive(sessionName string) bool {
	return watchdog.IsSessionAlive(sessionName)
}

// CreateSession creates a new tmux session and starts OpenCode
func (o *OpenCodeAdapter) CreateSession(sessionName, cliType, workDir string) error {
	// Create tmux session
	args := []string{"new-session", "-d", "-s", sessionName}

	// Set working directory if specified
	if workDir != "" {
		workDir, err := expandHome(workDir)
		if err != nil {
			return fmt.Errorf("invalid work_dir: %w", err)
		}
		args = append(args, "-c", workDir)
	}

	cmd := exec.Command("tmux", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create tmux session %s: %w (output: %s)", sessionName, err, string(output))
	}

	// Start OpenCode in the session
	if err := o.startOpenCode(sessionName); err != nil {
		return fmt.Errorf("failed to start OpenCode: %w", err)
	}

	return nil
}

// UseHook returns whether this adapter uses hook mode (true) or polling mode (false)
func (o *OpenCodeAdapter) UseHook() bool {
	return o.useHook
}

// GetPollInterval returns the polling interval for polling mode
func (o *OpenCodeAdapter) GetPollInterval() time.Duration {
	return o.pollInterval
}

// GetStableCount returns the number of consecutive stable checks required
func (o *OpenCodeAdapter) GetStableCount() int {
	return o.stableCount
}

// GetPollTimeout returns the maximum time to wait for completion
func (o *OpenCodeAdapter) GetPollTimeout() time.Duration {
	return o.pollTimeout
}

// startOpenCode starts OpenCode in the specified tmux session
func (o *OpenCodeAdapter) startOpenCode(sessionName string) error {
	logger.WithField("session", sessionName).Info("starting-opencode-cli-in-tmux-session")

	// Start OpenCode using "opencode" command
	// Note: The exact command may vary depending on OpenCode installation.
	// Common variants are "opencode", "opencode-cli", or "cursor".
	// Update this if the default command doesn't work with your setup.
	cmd := exec.Command("tmux", "send-keys", "-t", sessionName, "opencode", "C-m")

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to start OpenCode CLI: %w (output: %s)", err, string(output))
	}

	logger.WithField("session", sessionName).Info("opencode-cli-started-successfully")
	return nil
}

// ========== OpenCode Transcript Parsing ==========

// OpenCodeTranscript represents the structure of OpenCode's conversation history
// OpenCode stores conversations in JSON files similar to Claude's format
type OpenCodeTranscript struct {
	Messages []OpenCodeMessage `json:"messages"`
}

// OpenCodeMessage represents a single message in OpenCode's transcript
type OpenCodeMessage struct {
	Type    string `json:"type"`     // "user", "assistant", "progress", "error"
	Content string `json:"content"`   // Message content
	Metadata Metadata              `json:"metadata,omitempty"`
}

// Metadata represents additional message metadata
type Metadata struct {
	SessionID   string `json:"session_id"`
	Timestamp   string `json:"timestamp"`
	Model       string `json:"model,omitempty"`
}

// extractFromOpenCodeTranscript extracts the last assistant response from OpenCode's transcript file
func extractFromOpenCodeTranscript(transcriptPath string) (string, error) {
	logger.WithField("transcript", transcriptPath).Debug("starting-opencode-transcript-extraction")

	// Read transcript file
	file, err := os.Open(transcriptPath)
	if err != nil {
		return "", fmt.Errorf("failed to open transcript file: %w", err)
	}
	defer file.Close()

	// Parse JSON
	var transcript OpenCodeTranscript
	if err := json.NewDecoder(file).Decode(&transcript); err != nil {
		return "", fmt.Errorf("failed to parse transcript JSON: %w", err)
	}

	// Find last assistant message
	var lastAssistantContent string
	for i := len(transcript.Messages) - 1; i >= 0; i-- {
		msg := transcript.Messages[i]
		if msg.Type == "assistant" && msg.Content != "" {
			lastAssistantContent = msg.Content
			break
		}
	}

	if lastAssistantContent == "" {
		return "", fmt.Errorf("no assistant message found in transcript")
	}

	logger.WithFields(logrus.Fields{
		"transcript":    transcriptPath,
		"response_len":  len(lastAssistantContent),
	}).Info("opencode-response-extracted-from-transcript")

	return lastAssistantContent, nil
}

// extractLastUserPromptFromTranscript extracts the last user message from OpenCode's transcript
// This is used for filtering tmux output to show only the latest response
func extractLastUserPromptFromTranscript(transcriptPath string) (string, error) {
	file, err := os.Open(transcriptPath)
	if err != nil {
		return "", fmt.Errorf("failed to open transcript file: %w", err)
	}
	defer file.Close()

	var transcript OpenCodeTranscript
	if err := json.NewDecoder(file).Decode(&transcript); err != nil {
		return "", fmt.Errorf("failed to parse transcript JSON: %w", err)
	}

	// Find last user message
	for i := len(transcript.Messages) - 1; i >= 0; i-- {
		msg := transcript.Messages[i]
		if msg.Type == "user" && msg.Content != "" {
			// Extract first line or first 50 chars for matching
			prompt := msg.Content
			if strings.Contains(prompt, "\n") {
				prompt = strings.Split(prompt, "\n")[0]
			}
			if len(prompt) > 50 {
				prompt = prompt[:50]
			}
			return prompt, nil
		}
	}

	return "", fmt.Errorf("no user message found in transcript")
}

// GetLastOpenCodeResponse is removed - no longer needed as OpenCode uses hook mode
// which provides transcript_path directly in the hook data.
