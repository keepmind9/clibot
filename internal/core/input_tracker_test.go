package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestInputTracker_RecordAndRetrieve(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "input-tracker-test-*")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	tracker, err := NewInputTracker(tmpDir)
	assert.NoError(t, err)
	assert.NotNil(t, tracker)

	session := "test-session"
	input := "Hello, world!"

	err = tracker.RecordInput(session, input)
	assert.NoError(t, err)

	// Check new JSONL file exists
	filePath := filepath.Join(tmpDir, session, "input_history.jsonl")
	_, err = os.Stat(filePath)
	assert.NoError(t, err)

	retrievedInput, timestamp, err := tracker.GetLastInput(session)
	assert.NoError(t, err)
	assert.Equal(t, input, retrievedInput)
	assert.Greater(t, timestamp, int64(0))
}

func TestInputTracker_MultiLineInput(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "input-tracker-test-*")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	tracker, err := NewInputTracker(tmpDir)
	assert.NoError(t, err)

	session := "test-session"
	input := "Help me write a function\nThat handles multiple lines\nOf input"

	err = tracker.RecordInput(session, input)
	assert.NoError(t, err)

	retrievedInput, _, err := tracker.GetLastInput(session)
	assert.NoError(t, err)
	assert.Equal(t, input, retrievedInput)
}

func TestInputTracker_TimestampPrecision(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "input-tracker-test-*")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	tracker, err := NewInputTracker(tmpDir)
	assert.NoError(t, err)

	session := "test-session"
	input := "Test input"

	before := time.Now().UnixMilli()
	err = tracker.RecordInput(session, input)
	assert.NoError(t, err)
	after := time.Now().UnixMilli()

	_, timestamp, err := tracker.GetLastInput(session)
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, timestamp, before)
	assert.LessOrEqual(t, timestamp, after)

	timestampStr := fmt.Sprintf("%d", timestamp)
	assert.Len(t, timestampStr, 13)
}

func TestInputTracker_HasInput(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "input-tracker-test-*")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	tracker, err := NewInputTracker(tmpDir)
	assert.NoError(t, err)

	session := "test-session"

	assert.False(t, tracker.HasInput(session))

	err = tracker.RecordInput(session, "test")
	assert.NoError(t, err)

	assert.True(t, tracker.HasInput(session))
}

func TestInputTracker_Clear(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "input-tracker-test-*")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	tracker, err := NewInputTracker(tmpDir)
	assert.NoError(t, err)

	session := "test-session"

	err = tracker.RecordInput(session, "test")
	assert.NoError(t, err)
	assert.True(t, tracker.HasInput(session))

	// Clear is now a no-op - it preserves history for multi-input matching
	err = tracker.Clear(session)
	assert.NoError(t, err)
	// Input should still exist after Clear
	assert.True(t, tracker.HasInput(session))

	err = tracker.Clear("non-existent")
	assert.NoError(t, err)
}

func TestInputTracker_GetTimestamp(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "input-tracker-test-*")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	tracker, err := NewInputTracker(tmpDir)
	assert.NoError(t, err)

	session := "test-session"

	timestamp := tracker.GetTimestamp(session)
	assert.Equal(t, int64(0), timestamp)

	err = tracker.RecordInput(session, "test")
	assert.NoError(t, err)

	timestamp = tracker.GetTimestamp(session)
	assert.Greater(t, timestamp, int64(0))
}

func TestInputTracker_SessionIsolation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "input-tracker-test-*")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	tracker, err := NewInputTracker(tmpDir)
	assert.NoError(t, err)

	session1 := "session-1"
	session2 := "session-2"
	input1 := "Input for session 1"
	input2 := "Input for session 2"

	err = tracker.RecordInput(session1, input1)
	assert.NoError(t, err)

	err = tracker.RecordInput(session2, input2)
	assert.NoError(t, err)

	retrieved1, _, _ := tracker.GetLastInput(session1)
	assert.Equal(t, input1, retrieved1)

	retrieved2, _, _ := tracker.GetLastInput(session2)
	assert.Equal(t, input2, retrieved2)

	// Clear is now a no-op - sessions should still have their inputs
	tracker.Clear(session1)
	assert.True(t, tracker.HasInput(session1))
	assert.True(t, tracker.HasInput(session2))
}

func TestInputTracker_EmptyInput(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "input-tracker-test-*")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	tracker, err := NewInputTracker(tmpDir)
	assert.NoError(t, err)

	err = tracker.RecordInput("test-session", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be empty")
}

func TestInputTracker_TooLongInput(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "input-tracker-test-*")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	tracker, err := NewInputTracker(tmpDir)
	assert.NoError(t, err)

	longInput := strings.Repeat("a", maxInputLength+1)

	err = tracker.RecordInput("test-session", longInput)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "too large")
}

func TestInputTracker_MaxValidInput(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "input-tracker-test-*")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	tracker, err := NewInputTracker(tmpDir)
	assert.NoError(t, err)

	maxInput := strings.Repeat("a", maxInputLength)

	err = tracker.RecordInput("test-session", maxInput)
	assert.NoError(t, err)

	retrieved, _, err := tracker.GetLastInput("test-session")
	assert.NoError(t, err)
	assert.Len(t, retrieved, maxInputLength)
}

func TestInputTracker_InvalidSessionName_PathTraversal(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "input-tracker-test-*")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	tracker, err := NewInputTracker(tmpDir)
	assert.NoError(t, err)

	invalidSessions := []string{
		"../../../etc/passwd",
		"../test",
		"test/../../etc",
		"test\\..\\windows",
	}

	for _, session := range invalidSessions {
		err = tracker.RecordInput(session, "test")
		assert.Error(t, err, "should reject path traversal: %s", session)
		assert.Contains(t, err.Error(), "path traversal")
	}
}

func TestInputTracker_InvalidSessionName_SpecialChars(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "input-tracker-test-*")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	tracker, err := NewInputTracker(tmpDir)
	assert.NoError(t, err)

	invalidSessions := []string{
		"test session",
		"test.session",
		"test@session",
		"test#session",
		"",
	}

	for _, session := range invalidSessions {
		err = tracker.RecordInput(session, "test")
		assert.Error(t, err, "should reject invalid session name: %s", session)
	}
}

func TestInputTracker_ValidSessionName(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "input-tracker-test-*")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	tracker, err := NewInputTracker(tmpDir)
	assert.NoError(t, err)

	validSessions := []string{
		"test",
		"test-session",
		"test_session",
		"Test123",
		"123",
		"a",
		strings.Repeat("a", maxSessionNameLength),
	}

	for _, session := range validSessions {
		err = tracker.RecordInput(session, "test input")
		assert.NoError(t, err, "should accept valid session name: %s", session)
	}
}

func TestInputTracker_SessionNameTooLong(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "input-tracker-test-*")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	tracker, err := NewInputTracker(tmpDir)
	assert.NoError(t, err)

	longSession := strings.Repeat("a", maxSessionNameLength+1)

	err = tracker.RecordInput(longSession, "test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid session name length")
}

func TestInputTracker_SpecialCharacters(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "input-tracker-test-*")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	tracker, err := NewInputTracker(tmpDir)
	assert.NoError(t, err)

	specialInputs := []string{
		"Tab\tcharacter",
		"New\nline",
		"Carriage\rReturn",
		"Quote\"character",
		"Apostrophe'character",
		"Backslash\\character",
		"Null\x00byte",
		"Emoji 😀",
		"Unicode 中文",
	}

	for _, input := range specialInputs {
		session := fmt.Sprintf("test-%d", time.Now().UnixNano())
		err = tracker.RecordInput(session, input)
		assert.NoError(t, err, "should accept special characters in input")

		retrieved, _, err := tracker.GetLastInput(session)
		assert.NoError(t, err)
		assert.Equal(t, input, retrieved, "special characters should be preserved")
	}
}

func TestInputTracker_ConcurrentAccess(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "input-tracker-test-*")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	tracker, err := NewInputTracker(tmpDir)
	assert.NoError(t, err)

	session := "test-session"
	const numGoroutines = 100
	const numOperations = 100

	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines*numOperations)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				switch j % 4 {
				case 0:
					err := tracker.RecordInput(session, fmt.Sprintf("input-%d-%d", id, j))
					if err != nil && !strings.Contains(err.Error(), "empty") {
						errors <- err
					}
				case 1:
					tracker.GetLastInput(session)
				case 2:
					tracker.HasInput(session)
				case 3:
					tracker.Clear(session)
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("unexpected error during concurrent access: %v", err)
	}
}

func TestInputTracker_GetSessionDir(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "input-tracker-test-*")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	tracker, err := NewInputTracker(tmpDir)
	assert.NoError(t, err)

	dir := tracker.GetSessionDir("test-session")
	assert.Contains(t, dir, "test-session")
	assert.Contains(t, dir, tmpDir)

	dir = tracker.GetSessionDir("../invalid")
	assert.Empty(t, dir)
}

func TestInputTracker_CorruptedFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "input-tracker-test-*")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	tracker, err := NewInputTracker(tmpDir)
	assert.NoError(t, err)

	session := "test-session"
	sessionDir := filepath.Join(tmpDir, session)
	sessionFile := filepath.Join(sessionDir, "input_history.jsonl")

	os.MkdirAll(sessionDir, 0755)

	testCases := []struct {
		name          string
		content       string
		shouldHaveData bool
	}{
		{"empty file", "", false},
		{"invalid JSON", "{invalid json}", false},
		{"mixed valid and invalid", "{\"timestamp\":123,\"input\":\"valid\"}\n{invalid}\n{\"timestamp\":456,\"input\":\"also valid\"}", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := os.WriteFile(sessionFile, []byte(tc.content), 0644)
			assert.NoError(t, err)

			records, retrieveErr := tracker.GetAllInputs(session)
			if tc.shouldHaveData {
				assert.NoError(t, retrieveErr)
				assert.Greater(t, len(records), 0, "Should have parsed some valid entries")
			} else {
				// Either returns empty or error (depending on implementation)
				if retrieveErr == nil {
					assert.Equal(t, 0, len(records))
				}
			}
		})
	}
}

func TestInputTracker_RetrieveNonExistent(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "input-tracker-test-*")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	tracker, err := NewInputTracker(tmpDir)
	assert.NoError(t, err)

	_, _, err = tracker.GetLastInput("non-existent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no inputs found")
}
