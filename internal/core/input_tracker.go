package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

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
func (t *InputTracker) RecordInput(session, input string) error {
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
func (t *InputTracker) GetLastInput(session string) (string, int64, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	path := filepath.Join(t.baseDir, session, "last_input.txt")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", 0, fmt.Errorf("failed to read input: %w", err)
	}

	// Split first newline only
	strData := string(data)
	idx := strings.Index(strData, "\n")
	if idx == -1 {
		return "", 0, fmt.Errorf("invalid format: no newline found")
	}

	// Parse timestamp from first line
	timestampStr := strData[:idx]
	timestamp, err := strconv.ParseInt(timestampStr, 10, 64)
	if err != nil {
		return "", 0, fmt.Errorf("invalid timestamp: %w", err)
	}

	// Everything after first newline is the input (may contain newlines)
	input := strData[idx+1:]

	return input, timestamp, nil
}

// HasInput checks if there's a recorded input for the session
// This is used to determine whether to send response to bot
// (only send response if input came from bot, not from manual CLI interaction)
func (t *InputTracker) HasInput(session string) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()

	path := filepath.Join(t.baseDir, session, "last_input.txt")
	_, err := os.ReadFile(path)
	return err == nil
}

// Clear removes the recorded input
// Should be called after response is extracted and sent to bot
func (t *InputTracker) Clear(session string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	path := filepath.Join(t.baseDir, session, "last_input.txt")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to clear input: %w", err)
	}

	return nil
}

// GetTimestamp retrieves just the timestamp (milliseconds)
// Returns 0 if no input exists
func (t *InputTracker) GetTimestamp(session string) int64 {
	_, timestamp, err := t.GetLastInput(session)
	if err != nil {
		return 0
	}
	return timestamp
}

// GetSessionDir returns the directory path for a specific session
// Useful for debugging or cleanup
func (t *InputTracker) GetSessionDir(session string) string {
	return filepath.Join(t.baseDir, session)
}
