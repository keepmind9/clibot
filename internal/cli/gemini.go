package cli

import (
	"crypto/sha256"
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

// GeminiAdapterConfig configuration for Gemini CLI adapter
type GeminiAdapterConfig struct {
	// Polling mode configuration (when UseHook = false)
	UseHook      bool          // Use hook mode (true) or polling mode (false). Default: true
	PollInterval time.Duration // Polling interval. Default: 1s
	StableCount  int           // Consecutive stable checks required. Default: 3
	PollTimeout  time.Duration // Maximum time to wait. Default: 120s
}

// GeminiAdapter implements CLIAdapter for Gemini CLI
type GeminiAdapter struct {
	// Polling mode configuration
	useHook      bool
	pollInterval time.Duration
	stableCount  int
	pollTimeout  time.Duration
}

// NewGeminiAdapter creates a new Gemini CLI adapter
func NewGeminiAdapter(config GeminiAdapterConfig) (*GeminiAdapter, error) {
	// Default to hook mode (true) if not explicitly configured
	useHook := config.UseHook
	if !useHook && config.PollInterval == 0 && config.PollTimeout == 0 {
		useHook = true
	}

	pollInterval, stableCount, pollTimeout := normalizePollingConfig(
		config.PollInterval, config.StableCount, config.PollTimeout)

	return &GeminiAdapter{
		useHook:      useHook,
		pollInterval: pollInterval,
		stableCount:  stableCount,
		pollTimeout:  pollTimeout,
	}, nil
}

// SendInput sends input to Gemini CLI via tmux
func (g *GeminiAdapter) SendInput(sessionName, input string) error {
	logger.WithFields(logrus.Fields{
		"session": sessionName,
		"input":   input,
		"length":  len(input),
	}).Debug("sending-input-to-tmux-session")

	// Gemini CLI needs a delay before Enter key to properly process the input
	if err := watchdog.SendKeys(sessionName, input, 200); err != nil {
		logger.WithFields(logrus.Fields{
			"session": sessionName,
			"error":   err,
		}).Error("failed-to-send-input-to-tmux")
		return err
	}

	return nil
}

// HandleHookData handles raw hook data from Gemini CLI
// Expected data format (JSON):
//   {"session_id": "...", "cwd": "...", ...}
//
// Gemini stores history in: ~/.gemini/tmp/{project_hash}/chats/session-*.json
// where project_hash = SHA256(project_path)
//
// This returns the cwd as the session identifier, which will be matched against
// the configured session's work_dir in the engine.
//
// Parameter data: raw hook data (JSON bytes)
// Returns: (cwd, lastUserPrompt, response, error)
func (g *GeminiAdapter) HandleHookData(data []byte) (string, string, string, error) {
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

	// Extract transcript_path if available
	transcriptPath := ""
	if v, ok := hookData["transcript_path"].(string); ok {
		transcriptPath = v
	}

	// Extract hook_event_name to check if this is a notification event
	hookEventName := ""
	if v, ok := hookData["hook_event_name"].(string); ok {
		hookEventName = v
	}

	logger.WithFields(logrus.Fields{
		"cwd":             cwd,
		"transcript_path":  transcriptPath,
		"hook_event_name": hookEventName,
	}).Debug("hook-data-parsed")

	var response string
	var lastUserPrompt string
	var err error

	// Only extract response for non-notification events
	// Notification events don't have assistant responses to extract
	if !strings.EqualFold(hookEventName, "Notification") {
		lastUserPrompt, response, err = g.extractGeminiResponse(transcriptPath, cwd)
		if err != nil {
			logger.WithFields(logrus.Fields{
				"transcript_path": transcriptPath,
				"cwd":             cwd,
				"error":           err,
			}).Warn("failed-to-extract-gemini-response")
		}
	} else {
		logger.WithField("hook_event_name", hookEventName).Debug("skipping-response-extraction-for-notification-event")
	}

	logger.WithFields(logrus.Fields{
		"cwd":          cwd,
		"prompt_len":   len(lastUserPrompt),
		"response_len": len(response),
	}).Info("response-extracted-from-gemini-history")

	return cwd, lastUserPrompt, response, nil
}

// Gemini stores history in: ~/.gemini/tmp/{project_hash}/chats/session-*.json
func (g *GeminiAdapter) lastSessionFile(cwd string)(string, error){
	// Build path to chats directory
	projectHash := computeProjectHash(cwd)
	homeDir, _ := os.UserHomeDir()
	chatsDir := filepath.Join(homeDir, ".gemini", "tmp", projectHash, "chats")

	// Check if directory exists
	if _, err := os.Stat(chatsDir); os.IsNotExist(err) {
		return "", fmt.Errorf("chats directory not found: %s", chatsDir)
	}

	// Find all session-*.json files
	pattern := filepath.Join(chatsDir, "session-*.json")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", fmt.Errorf("failed to find session files: %w", err)
	}

	if len(matches) == 0 {
		return "", fmt.Errorf("no session files found in %s", chatsDir)
	}

	// Sort by modification time, get the latest
	sort.Slice(matches, func(i, j int) bool {
		infoI, _ := os.Stat(matches[i])
		infoJ, _ := os.Stat(matches[j])
		return infoI.ModTime().After(infoJ.ModTime())
	})

	latestFile := matches[0]

	logger.WithFields(logrus.Fields{
		"latest_file": latestFile,
		"chats_dir":    chatsDir,
	}).Debug("found-latest-gemini-session-file")
	return latestFile, nil
}

// extractGeminiResponse extracts the latest Gemini response from history
// JSON structure: {"messages": [{"type": "user", ...}, {"type": "gemini", "content": "...", "thoughts": [...]}, ...]}
func (g *GeminiAdapter) extractGeminiResponse(transcriptPath string, cwd string) (string, string, error) {
	var latestFile = ""
	if transcriptPath == "" {
		_latestFile, _err := g.lastSessionFile(cwd)
		if _err != nil {
			return "", "", _err
		}
		latestFile = _latestFile
	} else {
		latestFile = transcriptPath
	}

	// Parse JSON
	data, err := os.ReadFile(latestFile)
	if err != nil {
		return "", "", fmt.Errorf("failed to read session file: %w", err)
	}

	var sessionData struct {
		Messages []struct {
			Type     string                   `json:"type"`
			Content  string                   `json:"content"`
			Thoughts []map[string]interface{} `json:"thoughts,omitempty"`
		} `json:"messages"`
	}

	if err := json.Unmarshal(data, &sessionData); err != nil {
		return "", "", fmt.Errorf("failed to parse session JSON: %w", err)
	}

	messages := sessionData.Messages
	if len(messages) == 0 {
		return "", "", fmt.Errorf("no messages in session file")
	}

	// Find last user message index
	lastUserIndex := -1
	for i, msg := range messages {
		if msg.Type == "user" {
			lastUserIndex = i
		}
	}

	if lastUserIndex == -1 {
		return "", "", fmt.Errorf("no user message found in session")
	}

	userPrompt := strings.TrimSpace(messages[lastUserIndex].Content)

	// Collect all Gemini messages after the last user message
	var contentParts []string
	for i := lastUserIndex + 1; i < len(messages); i++ {
		if messages[i].Type == "gemini" {
			content := strings.TrimSpace(messages[i].Content)
			if content != "" {
				contentParts = append(contentParts, content)
			}
		}
	}

	if len(contentParts) == 0 {
		return "", "", fmt.Errorf("no Gemini messages found after last user message")
	}

	// Join all content parts with double newline
	response := strings.Join(contentParts, "\n\n")

	logger.WithFields(logrus.Fields{
		"total_messages":  len(messages),
		"last_user_index": lastUserIndex,
		"gemini_messages": len(contentParts),
		"response_length": len(response),
	}).Info("extracted-gemini-response-from-session-file")

	return userPrompt, response, nil
}

// computeProjectHash computes SHA256 hash of project path
// This is used by Gemini to organize conversation history by project
func computeProjectHash(projectPath string) string {
	absPath, err := filepath.Abs(projectPath)
	if err != nil {
		logger.WithField("error", err).Warn("failed-to-get-absolute-path-using-raw-path")
		absPath = projectPath
	}

	hash := sha256.Sum256([]byte(absPath))
	return fmt.Sprintf("%x", hash)
}

// IsSessionAlive checks if the tmux session is still running
func (g *GeminiAdapter) IsSessionAlive(sessionName string) bool {
	return watchdog.IsSessionAlive(sessionName)
}

// CreateSession creates a new tmux session and starts Gemini CLI
func (g *GeminiAdapter) CreateSession(sessionName, cliType, workDir string) error {
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

	// Start Gemini CLI in the session
	if err := g.startGemini(sessionName); err != nil {
		return fmt.Errorf("failed to start Gemini CLI: %w", err)
	}

	return nil
}

// UseHook returns whether this adapter uses hook mode (true) or polling mode (false)
func (g *GeminiAdapter) UseHook() bool {
	return g.useHook
}

// GetPollInterval returns the polling interval for polling mode
func (g *GeminiAdapter) GetPollInterval() time.Duration {
	return g.pollInterval
}

// GetStableCount returns the number of consecutive stable checks required
func (g *GeminiAdapter) GetStableCount() int {
	return g.stableCount
}

// GetPollTimeout returns the maximum time to wait for completion
func (g *GeminiAdapter) GetPollTimeout() time.Duration {
	return g.pollTimeout
}

// startGemini starts Gemini CLI in a tmux session
func (g *GeminiAdapter) startGemini(sessionName string) error {
	logger.WithField("session", sessionName).Info("starting-gemini-cli-in-tmux-session")

	// Start Gemini CLI using "gemini" command
	// Note: The exact command may vary depending on Gemini CLI installation.
	// Common variants are "gemini", "gemini chat", or "gemini-cli".
	// Update this if the default command doesn't work with your setup.
	cmd := exec.Command("tmux", "send-keys", "-t", sessionName, "gemini", "C-m")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to start Gemini CLI: %w (output: %s)", err, string(output))
	}

	logger.WithField("session", sessionName).Info("gemini-cli-started")
	return nil
}
