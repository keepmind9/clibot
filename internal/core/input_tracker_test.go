package core

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestInputTracker_RecordAndRetrieve(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "input-tracker-test-*")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create tracker
	tracker, err := NewInputTracker(tmpDir)
	assert.NoError(t, err)
	assert.NotNil(t, tracker)

	// Test recording simple input
	session := "test-session"
	input := "Hello, world!"

	err = tracker.RecordInput(session, input)
	assert.NoError(t, err)

	// Verify file was created
	filePath := filepath.Join(tmpDir, session, "last_input.txt")
	_, err = os.Stat(filePath)
	assert.NoError(t, err)

	// Retrieve the input
	retrievedInput, timestamp, err := tracker.GetLastInput(session)
	assert.NoError(t, err)
	assert.Equal(t, input, retrievedInput)
	assert.Greater(t, timestamp, int64(0))
}

func TestInputTracker_MultiLineInput(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "input-tracker-test-*")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create tracker
	tracker, err := NewInputTracker(tmpDir)
	assert.NoError(t, err)

	// Test recording multi-line input
	session := "test-session"
	input := "Help me write a function\nThat handles multiple lines\nOf input"

	err = tracker.RecordInput(session, input)
	assert.NoError(t, err)

	// Retrieve the input
	retrievedInput, _, err := tracker.GetLastInput(session)
	assert.NoError(t, err)
	assert.Equal(t, input, retrievedInput)
}

func TestInputTracker_TimestampPrecision(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "input-tracker-test-*")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create tracker
	tracker, err := NewInputTracker(tmpDir)
	assert.NoError(t, err)

	// Record input and check timestamp is in milliseconds
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

	// Verify timestamp is in milliseconds (13 digits for current time)
 timestampStr := fmt.Sprintf("%d", timestamp)
	assert.Len(t, timestampStr, 13)
}

func TestInputTracker_HasInput(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "input-tracker-test-*")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create tracker
	tracker, err := NewInputTracker(tmpDir)
	assert.NoError(t, err)

	session := "test-session"

	// Initially should not have input
	assert.False(t, tracker.HasInput(session))

	// Record input
	err = tracker.RecordInput(session, "test")
	assert.NoError(t, err)

	// Now should have input
	assert.True(t, tracker.HasInput(session))
}

func TestInputTracker_Clear(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "input-tracker-test-*")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create tracker
	tracker, err := NewInputTracker(tmpDir)
	assert.NoError(t, err)

	session := "test-session"

	// Record input
	err = tracker.RecordInput(session, "test")
	assert.NoError(t, err)
	assert.True(t, tracker.HasInput(session))

	// Clear input
	err = tracker.Clear(session)
	assert.NoError(t, err)
	assert.False(t, tracker.HasInput(session))

	// Clear non-existent input should not error
	err = tracker.Clear("non-existent")
	assert.NoError(t, err)
}

func TestInputTracker_GetTimestamp(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "input-tracker-test-*")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create tracker
	tracker, err := NewInputTracker(tmpDir)
	assert.NoError(t, err)

	session := "test-session"

	// No input initially
	timestamp := tracker.GetTimestamp(session)
	assert.Equal(t, int64(0), timestamp)

	// Record input
	err = tracker.RecordInput(session, "test")
	assert.NoError(t, err)

	// Should get timestamp now
	timestamp = tracker.GetTimestamp(session)
	assert.Greater(t, timestamp, int64(0))
}

func TestInputTracker_SessionIsolation(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "input-tracker-test-*")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create tracker
	tracker, err := NewInputTracker(tmpDir)
	assert.NoError(t, err)

	// Record inputs for different sessions
	session1 := "session-1"
	session2 := "session-2"
	input1 := "Input for session 1"
	input2 := "Input for session 2"

	err = tracker.RecordInput(session1, input1)
	assert.NoError(t, err)

	err = tracker.RecordInput(session2, input2)
	assert.NoError(t, err)

	// Verify isolation
	retrieved1, _, _ := tracker.GetLastInput(session1)
	assert.Equal(t, input1, retrieved1)

	retrieved2, _, _ := tracker.GetLastInput(session2)
	assert.Equal(t, input2, retrieved2)

	// Clear one should not affect the other
	tracker.Clear(session1)
	assert.False(t, tracker.HasInput(session1))
	assert.True(t, tracker.HasInput(session2))
}
