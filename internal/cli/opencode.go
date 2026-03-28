package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/keepmind9/clibot/internal/logger"
	"github.com/sirupsen/logrus"
	_ "modernc.org/sqlite"
)

// OpenCodeAdapterConfig configuration for OpenCode adapter
type OpenCodeAdapterConfig struct {
	Env map[string]string // Environment variables to set for the CLI process
}

// OpenCodeAdapter implements CLIAdapter for OpenCode
type OpenCodeAdapter struct {
	BaseAdapter
}

// NewOpenCodeAdapter creates a new OpenCode adapter
func NewOpenCodeAdapter(config OpenCodeAdapterConfig) (*OpenCodeAdapter, error) {
	return &OpenCodeAdapter{
		BaseAdapter: NewBaseAdapter("opencode", "opencode", 0),
	}, nil
}

// HandleHookData handles raw hook data from OpenCode
// Expected data format (JSON):
//
//	{"cwd": "/path/to/workdir", "session_id": "...", "hook_event_name": "..."}
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

// dbMessage represents a parsed message row from OpenCode storage
type dbMessage struct {
	ID   string
	Role string
}

// OpenCodeSessionInfo represents the structure of an OpenCode session info file
type OpenCodeSessionInfo struct {
	ID        string `json:"id"`
	ProjectID string `json:"projectID"`
	Time      struct {
		Updated int64 `json:"updated"`
	} `json:"time"`
}

// extractLatestInteractionFromStorage extracts the latest interaction from OpenCode's storage.
// It tries SQLite first, then falls back to file-based storage.
func (o *OpenCodeAdapter) extractLatestInteractionFromStorage(cwd string, sessionID string) (string, string, error) {
	// Try SQLite first (covers newer sessions)
	prompt, response, err := o.extractLatestInteractionFromDB(cwd, sessionID)
	if err == nil {
		logger.WithField("source", "sqlite").Debug("opencode-interaction-extracted")
		return prompt, response, nil
	}
	logger.WithField("error", err).Debug("opencode-sqlite-failed-trying-file-storage")

	// Fallback to file-based storage
	return o.extractLatestInteractionFromFiles(cwd, sessionID)
}

// ========== SQLite Extraction ==========

func (o *OpenCodeAdapter) extractLatestInteractionFromDB(cwd string, sessionID string) (string, string, error) {
	dbPath, err := getDBPath()
	if err != nil {
		return "", "", err
	}

	if _, err := os.Stat(dbPath); err != nil {
		return "", "", fmt.Errorf("sqlite db not found: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath+"?mode=ro")
	if err != nil {
		return "", "", fmt.Errorf("failed to open sqlite db: %w", err)
	}
	defer db.Close()

	// Resolve session ID if not provided
	if sessionID == "" {
		projectID, err := getProjectID(cwd)
		if err != nil {
			return "", "", fmt.Errorf("failed to get project ID: %w", err)
		}
		sessionID, err = getLatestSessionIDFromDB(db, projectID)
		if err != nil {
			return "", "", err
		}
	}

	// Query messages ordered by creation time
	messages, err := getMessagesFromDB(db, sessionID)
	if err != nil {
		return "", "", err
	}

	if len(messages) == 0 {
		return "", "", fmt.Errorf("no messages found for session %s", sessionID)
	}

	// Find last user message
	lastUserIndex := -1
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			lastUserIndex = i
			break
		}
	}
	if lastUserIndex == -1 {
		return "", "", fmt.Errorf("no user message found in session %s", sessionID)
	}

	// Get text for the user message
	prompt, err := getMessageTextFromDB(db, messages[lastUserIndex].ID)
	if err != nil {
		return "", "", fmt.Errorf("failed to get user message text: %w", err)
	}

	// Collect all subsequent assistant messages
	var responseParts []string
	for i := lastUserIndex + 1; i < len(messages); i++ {
		if messages[i].Role == "assistant" {
			text, err := getMessageTextFromDB(db, messages[i].ID)
			if err != nil {
				continue
			}
			if text != "" {
				responseParts = append(responseParts, text)
			}
		}
	}

	return prompt, strings.Join(responseParts, "\n\n"), nil
}

func getDBPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", "opencode", "opencode.db"), nil
}

func getLatestSessionIDFromDB(db *sql.DB, projectID string) (string, error) {
	var id string
	err := db.QueryRow(
		"SELECT id FROM session WHERE project_id = ? ORDER BY time_updated DESC LIMIT 1",
		projectID,
	).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("failed to find session for project %s: %w", projectID, err)
	}
	return id, nil
}

func getMessagesFromDB(db *sql.DB, sessionID string) ([]dbMessage, error) {
	rows, err := db.Query(
		"SELECT id, json_extract(data, '$.role') as role FROM message WHERE session_id = ? ORDER BY time_created",
		sessionID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query messages: %w", err)
	}
	defer rows.Close()

	var messages []dbMessage
	for rows.Next() {
		var msg dbMessage
		if err := rows.Scan(&msg.ID, &msg.Role); err != nil {
			continue
		}
		messages = append(messages, msg)
	}
	return messages, nil
}

func getMessageTextFromDB(db *sql.DB, messageID string) (string, error) {
	rows, err := db.Query(
		"SELECT json_extract(data, '$.text') FROM part WHERE message_id = ? AND json_extract(data, '$.type') = 'text' ORDER BY time_created",
		messageID,
	)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	var texts []string
	for rows.Next() {
		var text sql.NullString
		if err := rows.Scan(&text); err != nil {
			continue
		}
		if text.Valid && text.String != "" {
			texts = append(texts, text.String)
		}
	}
	return strings.Join(texts, "\n\n"), nil
}

// ========== File-Based Extraction (Legacy Fallback) ==========

// extractLatestInteractionFromFiles reads from the filesystem-based storage
func (o *OpenCodeAdapter) extractLatestInteractionFromFiles(cwd string, sessionID string) (string, string, error) {
	storageDir, err := getStorageDir()
	if err != nil {
		return "", "", err
	}

	// Resolve session ID if not provided
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

	// Read all message files for the session
	messageDir := filepath.Join(storageDir, "message", sessionID)
	files, err := filepath.Glob(filepath.Join(messageDir, "*.json"))
	if err != nil || len(files) == 0 {
		return "", "", fmt.Errorf("no messages found for session %s", sessionID)
	}

	sort.Strings(files)

	type fileMessage struct {
		ID   string
		Role string
	}

	var messages []fileMessage
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		var raw struct {
			ID   string `json:"id"`
			Role string `json:"role"`
		}
		if err := json.Unmarshal(data, &raw); err == nil && raw.Role != "" {
			messages = append(messages, fileMessage{ID: raw.ID, Role: raw.Role})
		}
	}

	if len(messages) == 0 {
		return "", "", fmt.Errorf("no valid messages parsed for session %s", sessionID)
	}

	// Find last user message
	lastUserIndex := -1
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			lastUserIndex = i
			break
		}
	}
	if lastUserIndex == -1 {
		return "", "", fmt.Errorf("no user message found in session %s", sessionID)
	}

	// Get text for user message
	prompt, err := loadPartsTextFromFiles(storageDir, messages[lastUserIndex].ID)
	if err != nil {
		logger.WithField("error", err).Debug("failed-to-load-user-parts")
	}

	// Collect all subsequent assistant messages
	var responseParts []string
	for i := lastUserIndex + 1; i < len(messages); i++ {
		if messages[i].Role == "assistant" {
			text, err := loadPartsTextFromFiles(storageDir, messages[i].ID)
			if err != nil {
				continue
			}
			if text != "" {
				responseParts = append(responseParts, text)
			}
		}
	}

	return prompt, strings.Join(responseParts, "\n\n"), nil
}

// loadPartsTextFromFiles reads text parts from storage/part/<messageID>/*.json
func loadPartsTextFromFiles(storageDir, messageID string) (string, error) {
	partDir := filepath.Join(storageDir, "part", messageID)
	files, err := filepath.Glob(filepath.Join(partDir, "*.json"))
	if err != nil || len(files) == 0 {
		return "", fmt.Errorf("no parts found for message %s", messageID)
	}

	var texts []string
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		var part struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}
		if err := json.Unmarshal(data, &part); err == nil {
			if part.Type == "text" && part.Text != "" {
				texts = append(texts, part.Text)
			}
		}
	}
	return strings.Join(texts, "\n\n"), nil
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

// ========== Shared Helper Functions ==========

func getStorageDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", "opencode", "storage"), nil
}

func getProjectID(cwd string) (string, error) {
	cmd := exec.Command("git", "rev-list", "--max-parents=0", "--all")
	cmd.Dir = cwd
	out, err := cmd.Output()
	if err != nil {
		return "global", nil
	}
	commits := strings.Fields(string(out))
	if len(commits) == 0 {
		return "global", nil
	}
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

func extractLatestInteractionFromFile(path string) (string, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", "", err
	}

	var raw struct {
		Role string `json:"role"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return "", "", fmt.Errorf("unsupported opencode file format: %s", path)
	}

	return "", "", nil
}
