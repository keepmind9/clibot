package cli

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/keepmind9/clibot/internal/logger"
	"github.com/keepmind9/clibot/internal/watchdog"
	"github.com/sirupsen/logrus"
)

// GeminiAdapterConfig configuration for Gemini CLI adapter
type GeminiAdapterConfig struct {
	HistoryDir string   // Base directory for Gemini data
	CheckLines int      // Number of lines to check for interactive prompts
	Patterns   []string // Regex patterns for interactive prompts
}

// GeminiAdapter implements CLIAdapter for Gemini CLI
type GeminiAdapter struct {
	historyDir string           // Base directory for Gemini data
	checkLines int              // Number of lines to check for prompts
	patterns   []*regexp.Regexp // Compiled regex patterns
}

// NewGeminiAdapter creates a new Gemini CLI adapter
// Returns an error if any of the regex patterns fail to compile
func NewGeminiAdapter(config GeminiAdapterConfig) (*GeminiAdapter, error) {
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

	return &GeminiAdapter{
		historyDir: historyDir,
		checkLines: config.CheckLines,
		patterns:   patterns,
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

// GetLastResponse retrieves the last assistant response from conversation history
// This is the fallback method when hook doesn't provide transcript_path
func (g *GeminiAdapter) GetLastResponse(sessionName string) (string, error) {
	// Try to get from conversation history files (fallback)
	content, err := GetLastAssistantContent(g.historyDir)
	if err != nil {
		return "", fmt.Errorf("failed to get last response: %w", err)
	}

	return content, nil
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

	logger.WithFields(logrus.Fields{
		"cwd": cwd,
	}).Debug("hook-data-parsed")

	// For Gemini, we need to compute project hash to find history files
	projectHash := computeProjectHash(cwd)

	// Find and parse Gemini's session JSON file
	response, err := g.extractGeminiResponse(projectHash)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"project_hash": projectHash,
			"error":        err,
		}).Warn("failed-to-extract-gemini-response")
		return cwd, "", "", nil // Return cwd but empty response (will trigger tmux fallback)
	}

	logger.WithFields(logrus.Fields{
		"cwd":          cwd,
		"response_len": len(response),
	}).Info("response-extracted-from-gemini-history")

	return cwd, "", response, nil
}

// extractGeminiResponse extracts the latest Gemini response from history
// Gemini stores history in: ~/.gemini/tmp/{project_hash}/chats/session-*.json
// JSON structure: {"messages": [{"type": "user", ...}, {"type": "gemini", "content": "...", "thoughts": [...]}, ...]}
func (g *GeminiAdapter) extractGeminiResponse(projectHash string) (string, error) {
	// Build path to chats directory
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

	// Parse JSON
	data, err := os.ReadFile(latestFile)
	if err != nil {
		return "", fmt.Errorf("failed to read session file: %w", err)
	}

	var sessionData struct {
		Messages []struct {
			Type     string                 `json:"type"`
			Content  string                 `json:"content"`
			Thoughts []map[string]interface{} `json:"thoughts,omitempty"`
		} `json:"messages"`
	}

	if err := json.Unmarshal(data, &sessionData); err != nil {
		return "", fmt.Errorf("failed to parse session JSON: %w", err)
	}

	messages := sessionData.Messages
	if len(messages) == 0 {
		return "", fmt.Errorf("no messages in session file")
	}

	// Find last user message index
	lastUserIndex := -1
	for i, msg := range messages {
		if msg.Type == "user" {
			lastUserIndex = i
		}
	}

	if lastUserIndex == -1 {
		return "", fmt.Errorf("no user message found in session")
	}

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
		return "", fmt.Errorf("no Gemini messages found after last user message")
	}

	// Join all content parts with double newline
	response := strings.Join(contentParts, "\n\n")

	logger.WithFields(logrus.Fields{
		"total_messages":    len(messages),
		"last_user_index":   lastUserIndex,
		"gemini_messages":   len(contentParts),
		"response_length":   len(response),
	}).Info("extracted-gemini-response-from-session-file")

	return response, nil
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
		workDir = expandHome(workDir)
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

// CheckInteractive checks if Gemini CLI is waiting for user input
func (g *GeminiAdapter) CheckInteractive(sessionName string) (bool, string, error) {
	// Capture last N lines from tmux session
	output, err := watchdog.CapturePane(sessionName, g.checkLines)
	if err != nil {
		return false, "", fmt.Errorf("failed to capture pane: %w", err)
	}

	// Split into lines
	lines := strings.Split(output, "\n")

	// Check last N lines for interactive prompts
	startIdx := len(lines) - g.checkLines
	if startIdx < 0 {
		startIdx = 0
	}

	relevantLines := lines[startIdx:]

	// Check each line for patterns
	for _, line := range relevantLines {
		trimmed := strings.TrimSpace(line)

		// Skip empty lines
		if trimmed == "" {
			continue
		}

		// Check against regex patterns
		for _, pattern := range g.patterns {
			if pattern.MatchString(trimmed) {
				logger.WithFields(logrus.Fields{
					"session":  sessionName,
					"pattern":  pattern.String(),
					"line":     trimmed,
				}).Info("interactive-prompt-detected")
				return true, trimmed, nil
			}
		}
	}

	return false, "", nil
}

// startGemini starts Gemini CLI in a tmux session
func (g *GeminiAdapter) startGemini(sessionName string) error {
	logger.WithField("session", sessionName).Info("starting-gemini-cli-in-tmux-session")

	// Start Gemini CLI
	// TODO: Verify the exact command to start Gemini CLI
	// This might be "gemini chat", "gemini", or something else
	cmd := exec.Command("tmux", "send-keys", "-t", sessionName, "gemini", "C-m")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to start Gemini CLI: %w (output: %s)", err, string(output))
	}

	logger.WithField("session", sessionName).Info("gemini-cli-started")
	return nil
}
