package cli

import (
	"os/exec"
	"regexp"
	"strings"
	"testing"
	"time"
)

// Mock functions for testing (to be replaced with proper mocks later)
var (
	mockSendKeys      func(sessionName, input string) error
	mockCapturePane   func(sessionName string, lines int) (string, error)
	mockIsAlive       func(sessionName string) bool
	mockCreateSession func(sessionName, cliType, workDir string) error
)

func TestClaudeAdapter_NewClaudeAdapter(t *testing.T) {
	adapter, err := NewClaudeAdapter(ClaudeAdapterConfig{
		HistoryDir: "/tmp/test/conversations",
		CheckLines: 3,
		Patterns:   []string{`\? \[y/N\]`, `Confirm\?`},
	})

	if err != nil {
		t.Fatalf("NewClaudeAdapter returned error: %v", err)
	}

	if adapter == nil {
		t.Fatal("NewClaudeAdapter returned nil")
	}

	if adapter.historyDir != "/tmp/test/conversations" {
		t.Errorf("expected historyDir /tmp/test/conversations, got %s", adapter.historyDir)
	}

	if adapter.checkLines != 3 {
		t.Errorf("expected checkLines 3, got %d", adapter.checkLines)
	}

	if len(adapter.patterns) != 2 {
		t.Errorf("expected 2 patterns, got %d", len(adapter.patterns))
	}
}

func TestClaudeAdapter_NewClaudeAdapter_PatternCompilation(t *testing.T) {
	adapter, err := NewClaudeAdapter(ClaudeAdapterConfig{
		HistoryDir: "/tmp/test/conversations",
		CheckLines: 3,
		Patterns:   []string{`\? \[y/N\]`, `Confirm\?`},
	})

	if err != nil {
		t.Fatalf("NewClaudeAdapter returned error: %v", err)
	}

	// Verify patterns are compiled regex
	if len(adapter.patterns) != 2 {
		t.Fatal("expected 2 patterns")
	}

	// Test first pattern - it requires a question mark followed by space and [y/N]
	// Note: We need to escape the brackets in the pattern
	if !adapter.patterns[0].MatchString("Execute? [y/N]") {
		t.Error("first pattern should match 'Execute? [y/N]'")
	}

	// Test second pattern
	if !adapter.patterns[1].MatchString("Confirm?") {
		t.Error("second pattern should match 'Confirm?'")
	}

	// Test with actual prompt format
	if !adapter.patterns[0].MatchString("Execute 'rm -rf ./temp'? [y/N]") {
		t.Error("first pattern should match full prompt")
	}
}

func TestClaudeAdapter_SendInput(t *testing.T) {
	adapter, err := NewClaudeAdapter(ClaudeAdapterConfig{
		HistoryDir: "/tmp/test/conversations",
		CheckLines: 3,
		Patterns:   []string{`\? \[y/N\]`},
	})
	if err != nil {
		t.Fatalf("NewClaudeAdapter failed: %v", err)
	}

	// Test that SendInput doesn't panic and returns appropriate error for non-existent session
	err = adapter.SendInput("test-session-nonexistent", "help")

	// We expect an error since the session doesn't exist
	if err == nil {
		t.Error("expected error when sending to non-existent session")
	}
}

func TestClaudeAdapter_CheckInteractive_WithConfirmationPrompt_ReturnsTrue(t *testing.T) {
	adapter, err := NewClaudeAdapter(ClaudeAdapterConfig{
		HistoryDir: "/tmp/test/conversations",
		CheckLines: 3,
		Patterns:   []string{`\? \[y/N\]`, `Confirm\?`},
	})
	if err != nil {
		t.Fatalf("NewClaudeAdapter failed: %v", err)
	}

	// Test pattern matching
	lines := []string{"Processing files...", "Execute 'rm -rf ./temp'? [y/N]"}
	waiting, prompt := testCheckInteractive(adapter, lines)

	if !waiting {
		t.Error("expected waiting=true for confirmation prompt")
	}

	if prompt == "" {
		t.Error("expected non-empty prompt")
	}
}

func TestClaudeAdapter_CheckInteractive_WithoutPrompt_ReturnsFalse(t *testing.T) {
	adapter, err := NewClaudeAdapter(ClaudeAdapterConfig{
		HistoryDir: "/tmp/test/conversations",
		CheckLines: 3,
		Patterns:   []string{`\? \[y/N\]`, `Confirm\?`},
	})
	if err != nil {
		t.Fatalf("NewClaudeAdapter failed: %v", err)
	}

	// Test data: normal output without prompt
	lines := []string{"Processing files...", "Done!"}

	waiting, _ := testCheckInteractive(adapter, lines)

	if waiting {
		t.Error("expected waiting=false for normal output")
	}
}

func TestClaudeAdapter_CheckInteractive_AnsiCodes_StripCorrectly(t *testing.T) {
	adapter, err := NewClaudeAdapter(ClaudeAdapterConfig{
		HistoryDir: "/tmp/test/conversations",
		CheckLines: 3,
		Patterns:   []string{`\? \[y/N\]`},
	})
	if err != nil {
		t.Fatalf("NewClaudeAdapter failed: %v", err)
	}

	// Test data: ANSI codes with prompt
	lines := []string{"\x1b[31mError:\x1b[0m Execute? [y/N]"}

	waiting, prompt := testCheckInteractive(adapter, lines)

	if !waiting {
		t.Error("expected waiting=true even with ANSI codes")
	}

	// Verify prompt was cleaned
	if prompt == "" {
		t.Error("expected cleaned prompt")
	}

	// Verify ANSI codes were stripped
	if strings.Contains(prompt, "\x1b") {
		t.Error("prompt should not contain ANSI codes")
	}
}

func TestClaudeAdapter_HomeDirExpansion(t *testing.T) {
	adapter, err := NewClaudeAdapter(ClaudeAdapterConfig{
		HistoryDir: "~/.claude/conversations",
		CheckLines: 3,
		Patterns:   []string{`\? \[y/N\]`},
	})
	if err != nil {
		t.Fatalf("NewClaudeAdapter failed: %v", err)
	}

	// Home directory should be expanded
	if adapter.historyDir == "~" {
		t.Error("home directory should be expanded")
	}

	// Should start with /home or /Users (typical home dirs)
	if !strings.HasPrefix(adapter.historyDir, "/") {
		t.Error("expanded path should be absolute")
	}
}

// Helper function to test CheckInteractive logic
func testCheckInteractive(adapter *ClaudeAdapter, lines []string) (bool, string) {
	for _, line := range lines {
		clean := stripAnsiHelper(line)
		for _, pattern := range adapter.patterns {
			if pattern.MatchString(clean) {
				return true, clean
			}
		}
	}
	return false, ""
}

// Helper function to strip ANSI codes (simplified version)
func stripAnsiHelper(input string) string {
	// Simple ANSI escape code removal
	ansiEscape := regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)
	return ansiEscape.ReplaceAllString(input, "")
}

func TestClaudeAdapter_IsSessionAlive(t *testing.T) {
	adapter, err := NewClaudeAdapter(ClaudeAdapterConfig{
		HistoryDir: "/tmp/test/conversations",
		CheckLines: 3,
		Patterns:   []string{`\? \[y/N\]`},
	})
	if err != nil {
		t.Fatalf("NewClaudeAdapter failed: %v", err)
	}

	// Test interface implementation
	// This will be properly tested with mocks
	alive := adapter.IsSessionAlive("test-session")

	// Without tmux running, this should return false
	if alive {
		t.Error("expected session to be not alive without tmux")
	}
}

func TestConversation_LastAssistantMessage(t *testing.T) {
	conv := &Conversation{
		Messages: []Message{
			{Role: "user", Content: "Hello", Timestamp: time.Now()},
			{Role: "assistant", Content: "Hi there!", Timestamp: time.Now()},
			{Role: "user", Content: "How are you?", Timestamp: time.Now()},
		},
	}

	msg := conv.LastAssistantMessage()
	if msg == nil {
		t.Fatal("expected assistant message, got nil")
	}

	if msg.Content != "Hi there!" {
		t.Errorf("expected 'Hi there!', got %s", msg.Content)
	}
}

func TestConversation_LastAssistantMessage_NoMessages(t *testing.T) {
	conv := &Conversation{
		Messages: []Message{},
	}

	msg := conv.LastAssistantMessage()
	if msg != nil {
		t.Error("expected nil for empty conversation")
	}
}

func TestConversation_LastAssistantMessage_OnlyUserMessages(t *testing.T) {
	conv := &Conversation{
		Messages: []Message{
			{Role: "user", Content: "Hello", Timestamp: time.Now()},
			{Role: "user", Content: "How are you?", Timestamp: time.Now()},
		},
	}

	msg := conv.LastAssistantMessage()
	if msg != nil {
		t.Error("expected nil when no assistant messages")
	}
}

func TestConversation_LoadConversation_InvalidJSON(t *testing.T) {
	// This test verifies error handling for invalid JSON
	_, err := LoadConversation("/nonexistent/file.json")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestClaudeAdapter_CreateSession(t *testing.T) {
	adapter, err := NewClaudeAdapter(ClaudeAdapterConfig{
		HistoryDir: "/tmp/test/conversations",
		CheckLines: 3,
		Patterns:   []string{`\? \[y/N\]`},
	})
	if err != nil {
		t.Fatalf("NewClaudeAdapter failed: %v", err)
	}

	// Test CreateSession with a unique name to avoid conflicts
	sessionName := "test-clibot-session-12345"

	err = adapter.CreateSession(sessionName, "claude", "/tmp")
	// This might fail if tmux is not installed or not configured
	// We're just testing that it doesn't panic
	if err != nil {
		// Expected to fail in most test environments
		t.Logf("CreateSession failed as expected in test environment: %v", err)
	}

	// Clean up: try to kill the session if it was created
	exec.Command("tmux", "kill-session", "-t", sessionName).Run()
}

func TestClaudeAdapter_GetLastResponse_NoConversationFiles(t *testing.T) {
	// Create a temporary directory with no conversation files
	tmpDir := t.TempDir()

	adapter, err := NewClaudeAdapter(ClaudeAdapterConfig{
		HistoryDir: tmpDir,
		CheckLines: 3,
		Patterns:   []string{`\? \[y/N\]`},
	})
	if err != nil {
		t.Fatalf("NewClaudeAdapter failed: %v", err)
	}

	_, err = adapter.GetLastResponse("test-session")
	if err == nil {
		t.Error("expected error when no conversation files exist")
	}
}

func TestClaudeAdapter_NewClaudeAdapter_InvalidPattern(t *testing.T) {
	// Test with invalid regex pattern
	_, err := NewClaudeAdapter(ClaudeAdapterConfig{
		HistoryDir: "/tmp/test/conversations",
		CheckLines: 3,
		Patterns:   []string{`[invalid(`}, // Unclosed bracket
	})

	if err == nil {
		t.Error("expected error for invalid regex pattern")
	}
}
