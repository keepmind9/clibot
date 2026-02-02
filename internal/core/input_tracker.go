package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	// maxInputLength is the maximum allowed input length (1MB)
	// This prevents DoS attacks through extremely large inputs
	maxInputLength = 1024 * 1024

	// maxSessionNameLength is the maximum allowed session name length
	maxSessionNameLength = 100

	// minSessionNameLength is the minimum allowed session name length
	minSessionNameLength = 1

	// DefaultInputHistorySize is the default maximum number of inputs to keep
	DefaultInputHistorySize = 10
)

// sessionNamePattern defines valid session name format
// Only alphanumeric characters, hyphens, and underscores are allowed
var sessionNamePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// InputRecord represents a single input entry in the history
type InputRecord struct {
	Timestamp int64  `json:"timestamp"`
	Content   string `json:"input"`
}

// InputTracker tracks user input for each session
// Used to extract the correct response from tmux output in polling mode
//
// The tracker records user input before sending to CLI, and uses it as an anchor
// to extract the relevant response from tmux capture (which may contain historical data).
type InputTracker struct {
	baseDir string
	maxSize int
	mu      sync.RWMutex
}

// NewInputTracker creates a new input tracker
// baseDir is the root directory for storing session input files
func NewInputTracker(baseDir string) (*InputTracker, error) {
	return NewInputTrackerWithSize(baseDir, DefaultInputHistorySize)
}

// NewInputTrackerWithSize creates a new input tracker with custom max size
func NewInputTrackerWithSize(baseDir string, maxSize int) (*InputTracker, error) {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create base dir: %w", err)
	}

	return &InputTracker{
		baseDir: baseDir,
		maxSize: maxSize,
	}, nil
}

// validateSessionName validates session name to prevent path traversal attacks
// Returns error if session name is invalid
func validateSessionName(session string) error {
	// Check length
	if len(session) < minSessionNameLength || len(session) > maxSessionNameLength {
		return fmt.Errorf("invalid session name length: must be between %d and %d characters",
			minSessionNameLength, maxSessionNameLength)
	}

	// Check for path traversal attempts
	if strings.Contains(session, "..") || strings.Contains(session, "/") || strings.Contains(session, "\\") {
		return fmt.Errorf("path traversal detected in session name")
	}

	// Check format (only alphanumeric, hyphen, underscore)
	if !sessionNamePattern.MatchString(session) {
		return fmt.Errorf("invalid session name format: only alphanumeric, hyphen, and underscore allowed")
	}

	return nil
}

// RecordInput records user input with millisecond timestamp in JSONL format
// The input can contain newlines and special characters - they are preserved in JSON
//
// File format (JSONL - one JSON object per line):
//   {"timestamp":1706878200123,"input":"help me\nwrite code"}
//   {"timestamp":1706878201000,"input":"1"}
//
// After recording, the file is trimmed to keep only the most recent maxSize entries
//
// Returns error if:
//   - Session name is invalid (path traversal check)
//   - Input is empty
//   - Input exceeds maxInputLength (1MB)
//   - File operations fail
func (t *InputTracker) RecordInput(session, input string) error {
	// Validate session name (security check)
	if err := validateSessionName(session); err != nil {
		return fmt.Errorf("invalid session name: %w", err)
	}

	// Validate input length
	if len(input) == 0 {
		return fmt.Errorf("input cannot be empty")
	}
	if len(input) > maxInputLength {
		return fmt.Errorf("input too large: %d bytes (max: %d bytes)", len(input), maxInputLength)
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	// Create session directory
	sessionDir := filepath.Join(t.baseDir, session)
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		return fmt.Errorf("failed to create session dir: %w", err)
	}

	// Construct JSON record
	record := InputRecord{
		Timestamp: time.Now().UnixMilli(),
		Content:   input,
	}
	jsonData, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("failed to marshal input: %w", err)
	}

	// Append to history file
	historyPath := filepath.Join(sessionDir, "input_history.jsonl")
	f, err := os.OpenFile(historyPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open history file: %w", err)
	}
	defer f.Close()

	if _, err := f.Write(append(jsonData, '\n')); err != nil {
		return fmt.Errorf("failed to write input: %w", err)
	}

	// Trim to size to prevent file from growing too large
	if err := t.trimToSize(historyPath); err != nil {
		// Trim failure is not critical, just log it
		// (in real implementation, would log here)
		return fmt.Errorf("failed to trim history: %w", err)
	}

	return nil
}

// trimToSize reads the history file and keeps only the most recent maxSize entries
// Must be called while holding the lock
func (t *InputTracker) trimToSize(historyPath string) error {
	// Read all lines
	data, err := os.ReadFile(historyPath)
	if err != nil {
		return err
	}

	lines := strings.Split(string(data), "\n")
	var validLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			validLines = append(validLines, trimmed)
		}
	}

	// Keep only the most recent maxSize entries
	if len(validLines) > t.maxSize {
		validLines = validLines[len(validLines)-t.maxSize:]
	}

	// Rewrite file
	output := strings.Join(validLines, "\n") + "\n"
	return os.WriteFile(historyPath, []byte(output), 0644)
}

// GetAllInputs retrieves all recorded inputs for a session, ordered from newest to oldest
// Returns empty slice if session doesn't exist or has no inputs
//
// Returns error if:
//   - Session name is invalid
//   - File exists but is corrupted
func (t *InputTracker) GetAllInputs(session string) ([]InputRecord, error) {
	// Validate session name
	if err := validateSessionName(session); err != nil {
		return nil, fmt.Errorf("invalid session name: %w", err)
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	historyPath := filepath.Join(t.baseDir, session, "input_history.jsonl")

	// Read file
	data, err := os.ReadFile(historyPath)
	if err != nil {
		if os.IsNotExist(err) {
			// No history file exists, return empty
			return []InputRecord{}, nil
		}
		return nil, fmt.Errorf("failed to read history: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	var records []InputRecord

	// Parse each line as JSON
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		var record InputRecord
		if err := json.Unmarshal([]byte(trimmed), &record); err != nil {
			// Skip corrupted lines but continue parsing
			continue
		}
		records = append(records, record)
	}

	// Reverse to get newest first
	for i, j := 0, len(records)-1; i < j; i, j = i+1, j-1 {
		records[i], records[j] = records[j], records[i]
	}

	return records, nil
}

// GetLastInput retrieves the most recent recorded input
// Returns: (input, timestampInMillis, error)
//
// The timestamp can be used to calculate response time:
//   responseTime = currentTime - inputTimestamp
//
// Returns error if:
//   - Session name is invalid
//   - No inputs found
func (t *InputTracker) GetLastInput(session string) (string, int64, error) {
	records, err := t.GetAllInputs(session)
	if err != nil {
		return "", 0, err
	}

	if len(records) == 0 {
		return "", 0, fmt.Errorf("no inputs found for session")
	}

	// GetAllInputs returns newest first, so index 0 is the most recent
	return records[0].Content, records[0].Timestamp, nil
}

// HasInput checks if there's any recorded input for the session
// This is used to determine whether to send response to bot
// (only send response if input came from bot, not from manual CLI interaction)
//
// Returns false if session name is invalid or no inputs exist
func (t *InputTracker) HasInput(session string) bool {
	// Validate session name (security check)
	if err := validateSessionName(session); err != nil {
		return false
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	historyPath := filepath.Join(t.baseDir, session, "input_history.jsonl")
	data, err := os.ReadFile(historyPath)
	if err != nil {
		return false
	}

	// Check if file has any content
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			return true
		}
	}

	return false
}

// Clear is a no-op for the new JSONL-based implementation
// We now preserve input history for multi-input matching
// This method is kept for backward compatibility but does nothing
//
// Deprecated: Input history is now preserved and automatically trimmed
func (t *InputTracker) Clear(session string) error {
	// No-op: we now preserve history
	// The trimToSize logic in RecordInput keeps the file size manageable
	return nil
}

// GetTimestamp retrieves just the timestamp (milliseconds)
// Returns 0 if no input exists or session is invalid
//
// Note: This method acquires its own lock internally and is safe to call
// from any context. It does NOT require external locking.
func (t *InputTracker) GetTimestamp(session string) int64 {
	// Validate session name first (without lock for performance)
	if err := validateSessionName(session); err != nil {
		return 0
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	path := filepath.Join(t.baseDir, session, "input_history.jsonl")
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}

	// Parse JSONL format - get the most recent (last) entry
	lines := strings.Split(string(data), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" {
			continue
		}

		// Parse JSON to get timestamp
		var record InputRecord
		if err := json.Unmarshal([]byte(trimmed), &record); err != nil {
			continue
		}
		return record.Timestamp
	}

	return 0
}

// GetSessionDir returns the directory path for a specific session
// Useful for debugging or cleanup
//
// Returns empty string if session name is invalid
func (t *InputTracker) GetSessionDir(session string) string {
	if err := validateSessionName(session); err != nil {
		return ""
	}
	return filepath.Join(t.baseDir, session)
}
