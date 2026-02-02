package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestInputTracker_JSONL_Record tests recording input in JSONL format
func TestInputTracker_JSONL_Record(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "input-tracker-jsonl-test-*")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	tracker, err := NewInputTrackerWithSize(tmpDir, 10)
	assert.NoError(t, err)

	session := "test-session"
	input := "help me write code"

	// Record input
	err = tracker.RecordInput(session, input)
	assert.NoError(t, err)

	// Verify file exists
	historyPath := filepath.Join(tmpDir, session, "input_history.jsonl")
	_, err = os.Stat(historyPath)
	assert.NoError(t, err)

	// Read and verify JSONL format
	data, err := os.ReadFile(historyPath)
	assert.NoError(t, err)

	lines := strings.Split(string(data), "\n")
	assert.Equal(t, 1, len(lines)-1) // Last line is empty

	// Parse JSON
	var record InputRecord
	err = json.Unmarshal([]byte(lines[0]), &record)
	assert.NoError(t, err)
	assert.Equal(t, input, record.Content)
	assert.Greater(t, record.Timestamp, int64(0))
}

// TestInputTracker_JSONL_MultiLineInput tests recording multi-line input
func TestInputTracker_JSONL_MultiLineInput(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "input-tracker-jsonl-test-*")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	tracker, err := NewInputTrackerWithSize(tmpDir, 10)
	assert.NoError(t, err)

	session := "test-session"
	input := "help me\nwrite code\nwith multiple lines"

	err = tracker.RecordInput(session, input)
	assert.NoError(t, err)

	// Retrieve
	records, err := tracker.GetAllInputs(session)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(records))
	assert.Equal(t, input, records[0].Content)
}

// TestInputTracker_JSONL_SpecialCharacters tests recording input with special characters
func TestInputTracker_JSONL_SpecialCharacters(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "input-tracker-jsonl-test-*")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	tracker, err := NewInputTrackerWithSize(tmpDir, 10)
	assert.NoError(t, err)

	session := "test-session"

	testCases := []string{
		"input with | pipe",
		"input with \n newline",
		"input with \t tab",
		"input with \\ backslash",
		"input with \" quote",
		"path/to/file|1",
	}

	for i, input := range testCases {
		err = tracker.RecordInput(session, input)
		assert.NoError(t, err, "Case %d failed", i)

		records, err := tracker.GetAllInputs(session)
		assert.NoError(t, err, "Case %d failed", i)
		assert.Equal(t, i+1, len(records), "Case %d failed", i)
		assert.Equal(t, input, records[0].Content, "Case %d failed", i)
	}
}

// TestInputTracker_JSONL_MultipleInputs tests recording multiple inputs
func TestInputTracker_JSONL_MultipleInputs(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "input-tracker-jsonl-test-*")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	maxSize := 5
	tracker, err := NewInputTrackerWithSize(tmpDir, maxSize)
	assert.NoError(t, err)

	session := "test-session"

	// Record 7 inputs (more than maxSize)
	for i := 1; i <= 7; i++ {
		input := string(rune('0' + i))
		err = tracker.RecordInput(session, input)
		assert.NoError(t, err)
	}

	// Should only keep last 5
	records, err := tracker.GetAllInputs(session)
	assert.NoError(t, err)
	assert.Equal(t, maxSize, len(records))

	// Verify order (newest first)
	assert.Equal(t, "7", records[0].Content)
	assert.Equal(t, "6", records[1].Content)
	assert.Equal(t, "5", records[2].Content)
	assert.Equal(t, "4", records[3].Content)
	assert.Equal(t, "3", records[4].Content)
}

// TestInputTracker_JSONL_NewestFirst verifies GetAllInputs returns newest first
func TestInputTracker_JSONL_NewestFirst(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "input-tracker-jsonl-test-*")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	tracker, err := NewInputTrackerWithSize(tmpDir, 10)
	assert.NoError(t, err)

	session := "test-session"

	inputs := []string{"first", "second", "third"}
	for _, input := range inputs {
		err = tracker.RecordInput(session, input)
		assert.NoError(t, err)
	}

	records, err := tracker.GetAllInputs(session)
	assert.NoError(t, err)
	assert.Equal(t, len(inputs), len(records))

	// Should be in reverse order (newest first)
	assert.Equal(t, "third", records[0].Content)
	assert.Equal(t, "second", records[1].Content)
	assert.Equal(t, "first", records[2].Content)
}

// TestInputTracker_JSONL_HasInput tests HasInput method
func TestInputTracker_JSONL_HasInput(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "input-tracker-jsonl-test-*")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	tracker, err := NewInputTrackerWithSize(tmpDir, 10)
	assert.NoError(t, err)

	session := "test-session"

	// Initially no input
	assert.False(t, tracker.HasInput(session))

	// After recording
	err = tracker.RecordInput(session, "test")
	assert.NoError(t, err)
	assert.True(t, tracker.HasInput(session))
}

// TestInputTracker_JSONL_GetLastInput tests GetLastInput method
func TestInputTracker_JSONL_GetLastInput(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "input-tracker-jsonl-test-*")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	tracker, err := NewInputTrackerWithSize(tmpDir, 10)
	assert.NoError(t, err)

	session := "test-session"

	// Record multiple inputs
	inputs := []string{"first", "second", "third"}
	for _, input := range inputs {
		err = tracker.RecordInput(session, input)
		assert.NoError(t, err)
	}

	// GetLastInput should return the most recent
	lastInput, timestamp, err := tracker.GetLastInput(session)
	assert.NoError(t, err)
	assert.Equal(t, "third", lastInput)
	assert.Greater(t, timestamp, int64(0))
}

// TestInputTracker_JSONL_ClearIsNoOp tests that Clear is now a no-op
func TestInputTracker_JSONL_ClearIsNoOp(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "input-tracker-jsonl-test-*")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	tracker, err := NewInputTrackerWithSize(tmpDir, 10)
	assert.NoError(t, err)

	session := "test-session"

	// Record input
	err = tracker.RecordInput(session, "test")
	assert.NoError(t, err)

	// Clear should be no-op
	err = tracker.Clear(session)
	assert.NoError(t, err)

	// Input should still exist
	assert.True(t, tracker.HasInput(session))

	records, err := tracker.GetAllInputs(session)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(records))
}

// TestInputTracker_JSONL_EmptySession tests session with no inputs
func TestInputTracker_JSONL_EmptySession(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "input-tracker-jsonl-test-*")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	tracker, err := NewInputTrackerWithSize(tmpDir, 10)
	assert.NoError(t, err)

	session := "test-session"

	// GetAllInputs should return empty slice
	records, err := tracker.GetAllInputs(session)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(records))

	// HasInput should return false
	assert.False(t, tracker.HasInput(session))

	// GetLastInput should return error
	_, _, err = tracker.GetLastInput(session)
	assert.Error(t, err)
}
