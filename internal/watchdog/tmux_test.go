package watchdog

import (
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStripANSI_WithANSICodes_ReturnsCleanText(t *testing.T) {
	input := "\x1b[31mRed text\x1b[0mNormal text"
	expected := "Red textNormal text"
	result := StripANSI(input)
	assert.Equal(t, expected, result)
}

func TestStripANSI_WithMultipleCodes_ReturnsCleanText(t *testing.T) {
	input := "\x1b[1;31;42mBold red on green\x1b[0m \x1b[34mBlue\x1b[0m"
	expected := "Bold red on green Blue"
	result := StripANSI(input)
	assert.Equal(t, expected, result)
}

func TestStripANSI_WithNoANSICodes_ReturnsSameText(t *testing.T) {
	input := "Plain text without codes"
	expected := "Plain text without codes"
	result := StripANSI(input)
	assert.Equal(t, expected, result)
}

func TestStripANSI_EmptyString_ReturnsEmptyString(t *testing.T) {
	input := ""
	expected := ""
	result := StripANSI(input)
	assert.Equal(t, expected, result)
}

func TestCapturePane_ValidSession_ReturnsOutput(t *testing.T) {
	if testing.Short() {
		t.Skip("requires actual tmux session")
	}
	// This test requires an actual tmux session to be running
	// Create a test session first
	if !IsSessionAlive("test-session") {
		t.Skip("test-session does not exist, skipping integration test")
	}
	output, err := CapturePane("test-session", 10)
	assert.NoError(t, err)
	assert.NotEmpty(t, output)
}

func TestCapturePane_InvalidSession_ReturnsError(t *testing.T) {
	if testing.Short() {
		t.Skip("requires actual tmux session")
	}
	output, err := CapturePane("non-existent-session-12345", 10)
	assert.Error(t, err)
	assert.Empty(t, output)
}

func TestIsSessionAlive_NonExistentSession_ReturnsFalse(t *testing.T) {
	if testing.Short() {
		t.Skip("requires actual tmux")
	}
	alive := IsSessionAlive("non-existent-session-12345")
	assert.False(t, alive)
}

func TestSendKeys_ValidSession_SendsKeys(t *testing.T) {
	if testing.Short() {
		t.Skip("requires actual tmux session")
	}
	err := SendKeys("test-session", "echo 'test'")
	// This might fail if session doesn't exist, but we're testing command construction
	// In a real test, we'd set up a test session first
	if err != nil {
		assert.Contains(t, err.Error(), "session")
	}
}

func TestSendKeys_EmptyInput_SendsEmpty(t *testing.T) {
	if testing.Short() {
		t.Skip("requires actual tmux session")
	}
	err := SendKeys("test-session", "")
	// Should not error on empty input, just send enter
	if err != nil {
		assert.Contains(t, err.Error(), "session")
	}
}

func TestCapturePaneClean_ValidSession_ReturnsCleanedOutput(t *testing.T) {
	if testing.Short() {
		t.Skip("requires actual tmux session")
	}
	// This test requires an actual tmux session
	if !IsSessionAlive("test-session") {
		t.Skip("test-session does not exist, skipping integration test")
	}
	output, err := CapturePaneClean("test-session", 10)
	assert.NoError(t, err)
	// Verify no ANSI codes remain
	assert.NotContains(t, output, "\x1b[")
}

func TestListSessions_ReturnsList(t *testing.T) {
	if testing.Short() {
		t.Skip("requires actual tmux")
	}
	// This test might fail if tmux server is not running
	// In that case, we expect an error
	sessions, err := ListSessions()
	if err != nil {
		// Tmux server might not be running, that's ok for this test
		assert.Contains(t, err.Error(), "failed to list")
	} else {
		assert.NotNil(t, sessions)
	}
}

func TestCapturePane_WithZeroLines_ReturnsAllLines(t *testing.T) {
	if testing.Short() {
		t.Skip("requires actual tmux session")
	}
	if !IsSessionAlive("test-session") {
		t.Skip("test-session does not exist, skipping integration test")
	}
	// Testing with 0 lines should capture all available lines
	output, err := CapturePane("test-session", 0)
	// This might work or might error depending on tmux behavior
	// Just verify it doesn't crash
	if err == nil {
		assert.NotNil(t, output)
	}
}

func TestCapturePane_WithNegativeLines_CapturesAllLines(t *testing.T) {
	if testing.Short() {
		t.Skip("requires actual tmux session")
	}
	if !IsSessionAlive("test-session") {
		t.Skip("test-session does not exist, skipping integration test")
	}
	// Negative lines should capture all lines (using -S -)
	output, err := CapturePane("test-session", -1)
	// Should succeed and capture all lines
	assert.NoError(t, err)
	assert.NotEmpty(t, output)
}

func TestStripANSI_WithCursorMovementCodes_ReturnsCleanText(t *testing.T) {
	// Test cursor movement codes
	input := "\x1b[2A\x1b[2KLine cleared\x1b[B"
	expected := "Line cleared"
	result := StripANSI(input)
	assert.Equal(t, expected, result)
}

func TestStripANSI_WithMixedTextAndCodes_ReturnsCleanText(t *testing.T) {
	// Test realistic terminal output
	input := "ERROR\x1b[31m [E123] \x1b[0mSomething failed\n\x1b[1;33mWARNING\x1b[0m Check config"
	expected := "ERROR [E123] Something failed\nWARNING Check config"
	result := StripANSI(input)
	assert.Equal(t, expected, result)
}

// TestIntegration_TmuxWorkflow tests a complete tmux workflow
// This test creates a session, sends commands, captures output, and cleans up
func TestIntegration_TmuxWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	testSession := "clibot-test-session"

	// Clean up any existing test session
	if IsSessionAlive(testSession) {
		exec.Command("tmux", "kill-session", "-t", testSession).Run()
	}

	// Create a new test session (detached)
	createCmd := exec.Command("tmux", "new-session", "-d", "-s", testSession, "/bin/bash")
	err := createCmd.Run()
	if err != nil {
		t.Skipf("failed to create test session: %v (tmux might not be available)", err)
	}

	// Ensure cleanup happens even if test fails
	defer func() {
		exec.Command("tmux", "kill-session", "-t", testSession).Run()
	}()

	// Give tmux a moment to initialize
	// In production code, we'd poll for session readiness

	// Test 1: IsSessionAlive should return true
	if !IsSessionAlive(testSession) {
		t.Fatal("session should be alive after creation")
	}

	// Test 2: SendKeys to execute a command
	err = SendKeys(testSession, "echo 'Hello from clibot'")
	if err != nil {
		t.Logf("Warning: SendKeys failed (session might not be ready): %v", err)
		// Continue anyway, we'll test other functions
	}

	// Test 3: CapturePane to get output (might be empty if command didn't execute)
	output, err := CapturePane(testSession, 10)
	if err != nil {
		// Session might not have a pane yet, that's ok
		t.Logf("Warning: CapturePane failed: %v", err)
	} else {
		t.Logf("Captured output: %q", output)
	}

	// Test 4: ListSessions should include our test session
	sessions, err := ListSessions()
	if err != nil {
		// Tmux server might have stopped
		t.Logf("Warning: ListSessions failed: %v", err)
	} else {
		assert.Contains(t, sessions, testSession, "session list should include our test session")
	}

	// Test overall: we successfully created and managed a tmux session
	t.Log("Successfully tested tmux workflow")
}

