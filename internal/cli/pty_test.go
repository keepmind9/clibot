//go:build !windows

package cli

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewPTYAdapter tests the creation of a new PTY adapter.
func TestNewPTYAdapter(t *testing.T) {
	config := PTYAdapterConfig{
		Env: map[string]string{"TEST_VAR": "true"},
	}
	adapter, err := NewPTYAdapter(config)
	require.NoError(t, err)
	require.NotNil(t, adapter)
	assert.Equal(t, config, adapter.config)
	assert.NotNil(t, adapter.sessions)
}

// TestPTYSessionLifecycle tests the full lifecycle of a PTY session.
func TestPTYSessionLifecycle(t *testing.T) {
	config := PTYAdapterConfig{}
	adapter, err := NewPTYAdapter(config)
	require.NoError(t, err)

	mockEngine := &mockPtyEngine{}
	adapter.SetEngine(mockEngine)

	sessionName := "test-pty-session"
	workDir := "."
	// Use a simple command that prints to stdout and then exits.
	startCmd := "echo 'hello from pty'"

	// Create session
	err = adapter.CreateSession(sessionName, workDir, startCmd, "")
	require.NoError(t, err)
	assert.True(t, adapter.IsSessionAlive(sessionName), "Session should be alive after creation")

	// Wait for the command to execute and the response to be captured.
	// The goroutine in CreateSession should read the output.
	time.Sleep(500 * time.Millisecond)

	// Check if the response was received by the mock engine.
	response := mockEngine.getResponse()
	assert.Contains(t, response, "hello from pty", "Expected response was not received")

	// The process should exit by itself. Wait a bit and check.
	time.Sleep(500 * time.Millisecond)
	assert.False(t, adapter.IsSessionAlive(sessionName), "Session should be dead after command exits")
}

// TestPTYSendInput tests sending input to a PTY session.
func TestPTYSendInput(t *testing.T) {
	config := PTYAdapterConfig{}
	adapter, err := NewPTYAdapter(config)
	require.NoError(t, err)

	mockEngine := &mockPtyEngine{}
	adapter.SetEngine(mockEngine)

	sessionName := "test-input-session"
	workDir := "."
	// Use 'cat' to wait for input and echo it back.
	startCmd := "cat"

	err = adapter.CreateSession(sessionName, workDir, startCmd, "")
	require.NoError(t, err)
	assert.True(t, adapter.IsSessionAlive(sessionName))

	// Send input
	input := "hello cat"
	err = adapter.SendInput(sessionName, input)
	require.NoError(t, err)

	// Wait for cat to process and echo the input.
	time.Sleep(500 * time.Millisecond)

	response := mockEngine.getResponse()
	assert.Contains(t, response, "hello cat", "Expected echoed response was not received")

	// Clean up
	err = adapter.CloseAllSessions()
	require.NoError(t, err)
	assert.False(t, adapter.IsSessionAlive(sessionName))
}

// mockPtyEngine is a mock implementation of the Engine interface for testing.
type mockPtyEngine struct {
	lastResponse string
}

func (m *mockPtyEngine) SendResponseToSession(sessionName, message string) {
	m.lastResponse += message
}

// SendToBot is a mock implementation.
func (m *mockPtyEngine) SendToBot(platform, channel, message string) {
	// Not used in these tests
}

func (m *mockPtyEngine) getResponse() string {
	return m.lastResponse
}
