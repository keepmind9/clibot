package core

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
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
)

// sessionNamePattern defines valid session name format
// Only alphanumeric characters, hyphens, and underscores are allowed
var sessionNamePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// InputTracker tracks user input for each session
// Used to extract the correct response from tmux output in polling mode
//
// The tracker records user input before sending to CLI, and uses it as an anchor
// to extract the relevant response from tmux capture (which may contain historical data).
type InputTracker struct {
	baseDir string
	mu      sync.RWMutex
}

// NewInputTracker creates a new input tracker
// baseDir is the root directory for storing session input files
func NewInputTracker(baseDir string) (*InputTracker, error) {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create base dir: %w", err)
	}

	return &InputTracker{
		baseDir: baseDir,
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

// RecordInput records user input with millisecond timestamp
// The input can contain newlines - they will be preserved
//
// File format:
//   Line 1: timestamp in milliseconds (13 digits)
//   Line 2+: complete user input (may contain newlines)
//
// Example:
//   1706878200123
//   Help me write a function
//   That handles multiple lines
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

	// Generate timestamp in milliseconds
	timestamp := time.Now().UnixMilli()

	// Format: first line is timestamp, rest is input (including newlines)
	data := fmt.Sprintf("%d\n%s", timestamp, input)

	path := filepath.Join(sessionDir, "last_input.txt")
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		return fmt.Errorf("failed to write input: %w", err)
	}

	return nil
}

// GetLastInput retrieves the last recorded input
// Returns: (input, timestampInMillis, error)
//
// The timestamp can be used to calculate response time:
//   responseTime = currentTime - inputTimestamp
//
// Returns error if:
//   - Session name is invalid
//   - File doesn't exist or can't be read
//   - File format is invalid
//   - Timestamp is malformed
func (t *InputTracker) GetLastInput(session string) (string, int64, error) {
	// Validate session name (security check)
	if err := validateSessionName(session); err != nil {
		return "", 0, fmt.Errorf("invalid session name: %w", err)
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	path := filepath.Join(t.baseDir, session, "last_input.txt")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", 0, fmt.Errorf("failed to read input: %w", err)
	}

	// Validate file is not empty
	if len(data) == 0 {
		return "", 0, fmt.Errorf("invalid format: empty file")
	}

	// Split first newline only
	strData := string(data)
	idx := strings.Index(strData, "\n")
	if idx == -1 {
		return "", 0, fmt.Errorf("invalid format: no newline found")
	}

	// Validate timestamp is not empty
	if idx == 0 {
		return "", 0, fmt.Errorf("invalid format: empty timestamp")
	}

	// Parse timestamp from first line
	timestampStr := strData[:idx]
	timestamp, err := strconv.ParseInt(timestampStr, 10, 64)
	if err != nil {
		return "", 0, fmt.Errorf("invalid timestamp: %w", err)
	}

	// Validate timestamp is reasonable (not too old or too far in future)
	now := time.Now().UnixMilli()
	maxAge := int64(365 * 24 * 3600 * 1000) // 1 year in milliseconds
	if timestamp < now-maxAge || timestamp > now+maxAge {
		return "", 0, fmt.Errorf("invalid timestamp: out of valid range")
	}

	// Everything after first newline is the input (may contain newlines)
	input := strData[idx+1:]

	return input, timestamp, nil
}

// HasInput checks if there's a recorded input for the session
// This is used to determine whether to send response to bot
// (only send response if input came from bot, not from manual CLI interaction)
//
// Returns false if session name is invalid
func (t *InputTracker) HasInput(session string) bool {
	// Validate session name (security check)
	if err := validateSessionName(session); err != nil {
		return false
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	path := filepath.Join(t.baseDir, session, "last_input.txt")
	_, err := os.ReadFile(path)
	return err == nil
}

// Clear removes the recorded input
// Should be called after response is extracted and sent to bot
//
// If session doesn't exist or file doesn't exist, returns no error (idempotent)
func (t *InputTracker) Clear(session string) error {
	// Validate session name (security check)
	if err := validateSessionName(session); err != nil {
		return fmt.Errorf("invalid session name: %w", err)
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	path := filepath.Join(t.baseDir, session, "last_input.txt")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to clear input: %w", err)
	}

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

	path := filepath.Join(t.baseDir, session, "last_input.txt")
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}

	// Parse just the timestamp
	strData := string(data)
	idx := strings.Index(strData, "\n")
	if idx == -1 || idx == 0 {
		return 0
	}

	timestampStr := strData[:idx]
	timestamp, err := strconv.ParseInt(timestampStr, 10, 64)
	if err != nil {
		return 0
	}

	return timestamp
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
