package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/keepmind9/clibot/internal/logger"
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
	BaseAdapter
}

// NewOpenCodeAdapter creates a new OpenCode adapter
func NewOpenCodeAdapter(config OpenCodeAdapterConfig) (*OpenCodeAdapter, error) {
	return &OpenCodeAdapter{
		BaseAdapter: NewBaseAdapter("opencode", "opencode", 0, config.UseHook, config.PollInterval, config.StableCount, config.PollTimeout),
	}, nil
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

	var lastUserPrompt, response string
	var err error

	// Extract interaction in one pass
	if transcriptPath != "" {
		lastUserPrompt, response, err = extractLatestOpenCodeInteraction(transcriptPath)
		if err != nil {
			logger.WithFields(logrus.Fields{
				"transcript": transcriptPath,
				"error":      err,
			}).Warn("failed-to-extract-interaction-from-transcript")
		}

		// Clear response for notification events
		if strings.EqualFold(hookEventName, "Notification") {
			response = ""
		}
	}

	logger.WithFields(logrus.Fields{
		"cwd":          cwd,
		"prompt_len":   len(lastUserPrompt),
		"response_len": len(response),
	}).Info("response-extracted-from-transcript")

	return cwd, lastUserPrompt, response, nil
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

// ExtractLatestInteraction exports the latest OpenCode response extraction logic
func (o *OpenCodeAdapter) ExtractLatestInteraction(transcriptPath string) (string, string, error) {
	return extractLatestOpenCodeInteraction(transcriptPath)
}

// extractLatestOpenCodeInteraction extracts the last interaction from OpenCode's transcript file
func extractLatestOpenCodeInteraction(transcriptPath string) (string, string, error) {
	logger.WithField("transcript", transcriptPath).Debug("starting-opencode-transcript-extraction")

	// Read transcript file
	file, err := os.Open(transcriptPath)
	if err != nil {
		return "", "", fmt.Errorf("failed to open transcript file: %w", err)
	}
	defer file.Close()

	// Parse JSON
	var transcript OpenCodeTranscript
	if err := json.NewDecoder(file).Decode(&transcript); err != nil {
		return "", "", fmt.Errorf("failed to parse transcript JSON: %w", err)
	}

	var prompt, response string

	// Find last user message
	for i := len(transcript.Messages) - 1; i >= 0; i-- {
		msg := transcript.Messages[i]
		if msg.Type == "user" && msg.Content != "" {
			prompt = msg.Content
			// Match logic from original extractLastUserPromptFromTranscript:
			// Extract first line or first 50 chars for matching
			if strings.Contains(prompt, "\n") {
				prompt = strings.Split(prompt, "\n")[0]
			}
			if len(prompt) > 50 {
				prompt = prompt[:50]
			}
			break
		}
	}

	// Find last assistant message
	for i := len(transcript.Messages) - 1; i >= 0; i-- {
		msg := transcript.Messages[i]
		if msg.Type == "assistant" && msg.Content != "" {
			response = msg.Content
			break
		}
	}

	if prompt == "" && response == "" {
		return "", "", fmt.Errorf("no interaction found in transcript")
	}

	logger.WithFields(logrus.Fields{
		"transcript":   transcriptPath,
		"prompt_len":   len(prompt),
		"response_len": len(response),
	}).Info("opencode-interaction-extracted")

	return prompt, response, nil
}

// GetLastOpenCodeResponse is removed - no longer needed as OpenCode uses hook mode
// which provides transcript_path directly in the hook data.
