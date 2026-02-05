package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
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
//   {"cwd": "/path/to/workdir", "session_id": "...", "hook_event_name": "..."}
//
// Returns: (cwd, lastUserPrompt, response, error)
func (o *OpenCodeAdapter) HandleHookData(data []byte) (string, string, string, error) {
	// Parse JSON
	var hookData struct {
		CWD       string `json:"cwd"`
		SessionID string `json:"session_id"`
		EventName string `json:"hook_event_name"`
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
		"session_id":      hookData.SessionID,
		"hook_event_name": hookData.EventName,
	}).Debug("hook-data-parsed")

	var lastUserPrompt, response string
	var err error

	// Extract interaction
	lastUserPrompt, response, err = o.extractLatestInteractionFromStorage(hookData.CWD, hookData.SessionID)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"cwd":   hookData.CWD,
			"error": err,
		}).Warn("failed-to-extract-interaction-from-storage")
	}

	// Clear response for notification events
	if strings.EqualFold(hookData.EventName, "Notification") {
		response = ""
	}

	logger.WithFields(logrus.Fields{
		"cwd":          hookData.CWD,
		"prompt_len":   len(lastUserPrompt),
		"response_len": len(response),
	}).Info("response-extracted-from-storage")

	return hookData.CWD, lastUserPrompt, response, nil
}

// ========== OpenCode Storage Parsing ==========

// OpenCodeMessageInfo represents the structure of an OpenCode message file
type OpenCodeMessageInfo struct {
	ID       string            `json:"id"`
	Role     string            `json:"role"` // "user", "assistant"
	Parts    []OpenCodePart    `json:"parts"`
	Metadata OpenCodeMetadata  `json:"metadata"`
}

// OpenCodePart represents a part of a message
type OpenCodePart struct {
	Type string `json:"type"` // "text", "reasoning", "tool-invocation", etc.
	Text string `json:"text,omitempty"`
}

// OpenCodeMetadata represents message metadata
type OpenCodeMetadata struct {
	SessionID string `json:"sessionID"`
	Time      struct {
		Created int64 `json:"created"`
	} `json:"time"`
}

// OpenCodeSessionInfo represents the structure of an OpenCode session info file
type OpenCodeSessionInfo struct {
	ID        string `json:"id"`
	ProjectID string `json:"projectID"`
	Time      struct {
		Updated int64 `json:"updated"`
	} `json:"time"`
}

// extractLatestInteractionFromStorage extracts the latest interaction from OpenCode's local storage
func (o *OpenCodeAdapter) extractLatestInteractionFromStorage(cwd string, sessionID string) (string, string, error) {
	storageDir, err := getStorageDir()
	if err != nil {
		return "", "", err
	}

	// 1. Determine session ID if not provided
	if sessionID == "" {
		projectID, err := getProjectID(cwd)
		if err != nil {
			return "", "", fmt.Errorf("failed to get project ID: %w", err)
		}

		sessionID, err = getLatestSessionID(storageDir, projectID)
		if err != nil {
			return "", "", fmt.Errorf("failed to get latest session ID: %w", err)
		}
	}

	// 2. Read all messages for the session
	messageDir := filepath.Join(storageDir, "message", sessionID)
	files, err := filepath.Glob(filepath.Join(messageDir, "*.json"))
	if err != nil || len(files) == 0 {
		return "", "", fmt.Errorf("no messages found for session %s", sessionID)
	}

	// Files are named msg_... and are sortable by name (ascending)
	sort.Strings(files)

	var messages []OpenCodeMessageInfo
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		var msg OpenCodeMessageInfo
		if err := json.Unmarshal(data, &msg); err == nil {
			messages = append(messages, msg)
		}
	}

	if len(messages) == 0 {
		return "", "", fmt.Errorf("no valid messages parsed for session %s", sessionID)
	}

	// 3. Find last interaction
	var prompt, response string
	var lastUserIndex = -1

	// Find last user message
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			lastUserIndex = i
			prompt = getOpencodeMessageText(messages[i])
			break
		}
	}

	if lastUserIndex == -1 {
		return "", "", fmt.Errorf("no user message found in session %s", sessionID)
	}

	// Collect all subsequent assistant messages
	var responseParts []string
	for i := lastUserIndex + 1; i < len(messages); i++ {
		if messages[i].Role == "assistant" {
			if text := getOpencodeMessageText(messages[i]); text != "" {
				responseParts = append(responseParts, text)
			}
		}
	}
	response = strings.Join(responseParts, "\n\n")

	return prompt, response, nil
}

// ExtractLatestInteraction legacy support
func (o *OpenCodeAdapter) ExtractLatestInteraction(transcriptPath string) (string, string, error) {
	// If transcriptPath is actually a directory (cwd), try storage extraction
	if info, err := os.Stat(transcriptPath); err == nil && info.IsDir() {
		return o.extractLatestInteractionFromStorage(transcriptPath, "")
	}
	// Fallback to simple file parsing if it looks like a single JSON file
	return extractLatestInteractionFromFile(transcriptPath)
}

// Helper functions

func getStorageDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	// Default XDG_DATA_HOME/opencode/storage
	return filepath.Join(home, ".local", "share", "opencode", "storage"), nil
}

func getProjectID(cwd string) (string, error) {
	// Get first commit hash
	cmd := exec.Command("git", "rev-list", "--max-parents=0", "--all")
	cmd.Dir = cwd
	out, err := cmd.Output()
	if err != nil {
		// Fallback to "global" if not a git repo
		return "global", nil
	}
	commits := strings.Fields(string(out))
	if len(commits) == 0 {
		return "global", nil
	}
	// Sort to be consistent with opencode logic
	sort.Strings(commits)
	return commits[0], nil
}

func getLatestSessionID(storageDir, projectID string) (string, error) {
	sessionDir := filepath.Join(storageDir, "session", projectID)
	files, err := filepath.Glob(filepath.Join(sessionDir, "*.json"))
	if err != nil || len(files) == 0 {
		return "", fmt.Errorf("no sessions found for project %s", projectID)
	}

	var latestID string
	var latestTime int64

	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		var info OpenCodeSessionInfo
		if err := json.Unmarshal(data, &info); err == nil {
			if info.Time.Updated > latestTime {
				latestTime = info.Time.Updated
				latestID = info.ID
			}
		}
	}

	if latestID == "" {
		return "", fmt.Errorf("could not find latest session in %s", sessionDir)
	}
	return latestID, nil
}

func getOpencodeMessageText(msg OpenCodeMessageInfo) string {
	var texts []string
	for _, part := range msg.Parts {
		if part.Type == "text" && part.Text != "" {
			texts = append(texts, part.Text)
		}
	}
	return strings.Join(texts, "\n\n")
}

func extractLatestInteractionFromFile(path string) (string, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", "", err
	}

	// Try parsing as a single message first
	var msg OpenCodeMessageInfo
	if err := json.Unmarshal(data, &msg); err == nil {
		return "", getOpencodeMessageText(msg), nil
	}

	return "", "", fmt.Errorf("unsupported opencode file format: %s", path)
}