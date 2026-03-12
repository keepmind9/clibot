package cli

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/keepmind9/clibot/internal/logger"
	"github.com/keepmind9/clibot/internal/watchdog"
	"github.com/sirupsen/logrus"
)

// GeminiAdapterConfig configuration for Gemini CLI adapter
type GeminiAdapterConfig struct {
	Env map[string]string // Environment variables to set for the CLI process
}

// GeminiAdapter implements CLIAdapter for Gemini CLI
type GeminiAdapter struct {
	BaseAdapter
}

// NewGeminiAdapter creates a new Gemini CLI adapter
func NewGeminiAdapter(config GeminiAdapterConfig) (*GeminiAdapter, error) {
	return &GeminiAdapter{
		BaseAdapter: NewBaseAdapter("gemini", "gemini", 200),
	}, nil
}

// ResetSession starts a new session for Gemini CLI
func (g *GeminiAdapter) ResetSession(sessionName string) error {
	logger.WithField("session", sessionName).Info("resetting-gemini-session")
	// Send "gemini --new" command followed by enter
	// Note: We use SendKeys with "enter" keyword which is handled by watchdog
	return watchdog.SendKeys(sessionName, "gemini --new\n", g.inputDelayMs)
}

// ListSessions returns a list of available Gemini session files for the current project.
// It scans the ~/.gemini/tmp/{project_hash}/chats directory.
func (g *GeminiAdapter) ListSessions(sessionName string) ([]string, error) {
	// Get current working directory from tmux session
	cwd, err := watchdog.GetCWD(sessionName)
	if err != nil {
		logger.WithField("error", err).Warn("failed-to-get-cwd-for-gemini-session-listing")
		return nil, fmt.Errorf("could not determine current work dir: %w", err)
	}
	return listGeminiSessionsByWorkDir(cwd)
}

// SwitchSession switches to a specific Gemini session using the /resume command.
func (g *GeminiAdapter) SwitchSession(sessionName, cliSessionID string) error {
	logger.WithFields(logrus.Fields{
		"session":     sessionName,
		"cli_session": cliSessionID,
	}).Info("switching-gemini-session-natively")
	return g.SendInput(sessionName, fmt.Sprintf("/resume %s\n", cliSessionID))
}

// listGeminiSessionsByWorkDir is a shared package-level helper that scans
// ~/.gemini/tmp/{hash}/chats for session-*.json files and returns formatted
// "#<id>: <first-user-message>" strings, sorted newest-first.
func listGeminiSessionsByWorkDir(workDir string) ([]string, error) {
	chatsDir, err := findGeminiChatsDir(workDir)
	if err != nil {
		return nil, err
	}

	matches, err := filepath.Glob(filepath.Join(chatsDir, "session-*.json"))
	if err != nil {
		return nil, fmt.Errorf("failed to find session files: %w", err)
	}
	if len(matches) == 0 {
		return []string{}, nil
	}

	sort.Slice(matches, func(i, j int) bool {
		infoI, _ := os.Stat(matches[i])
		infoJ, _ := os.Stat(matches[j])
		return infoI.ModTime().After(infoJ.ModTime())
	})

	var summaries []string
	for _, file := range matches {
		id := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(file), "session-"), ".json")
		summary := geminiSessionSummary(file)
		summaries = append(summaries, fmt.Sprintf("#%s: %s", id, summary))
	}
	return summaries, nil
}

// geminiSessionSummary extracts the first user prompt (≤50 chars) from a session JSON file.
func geminiSessionSummary(sessionFile string) string {
	data, err := os.ReadFile(sessionFile)
	if err != nil {
		return "(unreadable)"
	}
	var sd struct {
		Messages []struct {
			Type    string `json:"type"`
			Content string `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(data, &sd); err != nil {
		return "(parse error)"
	}
	for _, msg := range sd.Messages {
		if msg.Type == "user" {
			content := strings.TrimSpace(msg.Content)
			if len(content) > 50 {
				return content[:47] + "..."
			}
			return content
		}
	}
	return "(No messages)"
}

// HandleHookData handles raw hook data from Gemini CLI
// Expected data format (JSON):
//
//	{"session_id": "...", "cwd": "...", ...}
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
		"transcript_path": transcriptPath,
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
func (g *GeminiAdapter) lastSessionFile(cwd string) (string, error) {
	// Build path to chats directory
	chatsDir, err := findGeminiChatsDir(cwd)
	if err != nil {
		return "", err
	}

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
		"chats_dir":   chatsDir,
	}).Debug("found-latest-gemini-session-file")
	return latestFile, nil
}

// ExtractLatestInteraction exports the latest Gemini response extraction logic
func (g *GeminiAdapter) ExtractLatestInteraction(transcriptPath string, cwd string) (string, string, error) {
	return g.extractGeminiResponse(transcriptPath, cwd)
}

// extractGeminiResponse extracts the latest Gemini response from history
// JSON structure: {"messages": [{"type": "user", ...}, {"type": "gemini", "content": "...", "thoughts": [...]}, ...]}
func (g *GeminiAdapter) extractGeminiResponse(transcriptPath string, cwd string) (string, string, error) {
	var latestFile string
	if transcriptPath == "" {
		var err error
		latestFile, err = g.lastSessionFile(cwd)
		if err != nil {
			return "", "", err
		}
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

// findGeminiChatsDir finds the Gemini chats directory by searching for the .project_root file
// if the hash-based approach fails.
func findGeminiChatsDir(workDir string) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	absWorkDir, err := filepath.Abs(workDir)
	if err != nil {
		absWorkDir = workDir
	}

	// 1. Try name-based directory (e.g. .gemini/tmp/clibot)
	projectName := filepath.Base(absWorkDir)
	nameDir := filepath.Join(homeDir, ".gemini", "tmp", projectName)
	if _, err := os.Stat(filepath.Join(nameDir, ".project_root")); err == nil {
		// Verify project root matches
		rootContent, err := os.ReadFile(filepath.Join(nameDir, ".project_root"))
		if err == nil && strings.TrimSpace(strings.ToLower(string(rootContent))) == strings.TrimSpace(strings.ToLower(absWorkDir)) {
			return filepath.Join(nameDir, "chats"), nil
		}
	}

	// 2. Try hash-based directory
	projectHash := computeProjectHash(absWorkDir)
	hashDir := filepath.Join(homeDir, ".gemini", "tmp", projectHash)
	if _, err := os.Stat(filepath.Join(hashDir, "chats")); err == nil {
		return filepath.Join(hashDir, "chats"), nil
	}

	// 3. Scan all directories in .gemini/tmp for .project_root matching absWorkDir
	tmpDir := filepath.Join(homeDir, ".gemini", "tmp")
	entries, err := os.ReadDir(tmpDir)
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			rootFile := filepath.Join(tmpDir, entry.Name(), ".project_root")
			if _, err := os.Stat(rootFile); err == nil {
				rootContent, err := os.ReadFile(rootFile)
				if err == nil && strings.TrimSpace(strings.ToLower(string(rootContent))) == strings.TrimSpace(strings.ToLower(absWorkDir)) {
					return filepath.Join(tmpDir, entry.Name(), "chats"), nil
				}
			}
		}
	}

	// Fallback to the hash-based path
	return filepath.Join(hashDir, "chats"), nil
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
