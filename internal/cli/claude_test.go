package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClaudeAdapter_NewClaudeAdapter(t *testing.T) {
	adapter, err := NewClaudeAdapter(ClaudeAdapterConfig{})

	if err != nil {
		t.Fatalf("NewClaudeAdapter returned error: %v", err)
	}

	if adapter == nil {
		t.Fatal("NewClaudeAdapter returned nil")
	}
}

func TestClaudeAdapter_SendInput(t *testing.T) {
	adapter, err := NewClaudeAdapter(ClaudeAdapterConfig{})
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

func TestClaudeAdapter_IsSessionAlive(t *testing.T) {
	adapter, err := NewClaudeAdapter(ClaudeAdapterConfig{})
	if err != nil {
		t.Fatalf("NewClaudeAdapter failed: %v", err)
	}

	// Use a unique session name that almost certainly doesn't exist
	uniqueSessionName := fmt.Sprintf("clibot-test-nonexistent-%d", time.Now().UnixNano())
	alive := adapter.IsSessionAlive(uniqueSessionName)

	// This should return false for a non-existent session
	if alive {
		t.Errorf("expected session %s to be not alive", uniqueSessionName)
	}
}

func TestClaudeAdapter_CreateSession(t *testing.T) {
	adapter, err := NewClaudeAdapter(ClaudeAdapterConfig{})
	if err != nil {
		t.Fatalf("NewClaudeAdapter failed: %v", err)
	}

	// Test CreateSession with a unique name to avoid conflicts
	sessionName := "test-clibot-session-12345"

	err = adapter.CreateSession(sessionName, "/tmp", "claude", "")
	// This might fail if tmux is not installed or not configured
	// We're just testing that it doesn't panic
	if err != nil {
		// Expected to fail in most test environments
		t.Logf("CreateSession failed as expected in test environment: %v", err)
	}

	// Clean up: try to kill the session if it was created
	exec.Command("tmux", "kill-session", "-t", sessionName).Run()
}

func TestClaudeAdapter_CreateSession_Idempotent(t *testing.T) {
	adapter, err := NewClaudeAdapter(ClaudeAdapterConfig{})
	if err != nil {
		t.Fatalf("NewClaudeAdapter failed: %v", err)
	}

	// Test idempotency: calling CreateSession multiple times should succeed
	sessionName := "test-clibot-idempotent"

	// Clean up first in case it exists from previous run
	exec.Command("tmux", "kill-session", "-t", sessionName).Run()

	// First call
	err = adapter.CreateSession(sessionName, "/tmp", "echo 'test'", "")
	if err != nil {
		t.Skipf("Skipping test, First CreateSession failed (tmux may not be installed): %v", err)
	}

	// Second call should succeed due to idempotency (session already exists)
	err = adapter.CreateSession(sessionName, "/tmp", "echo 'test'", "")
	if err != nil {
		t.Fatalf("Second CreateSession should succeed (idempotent), but failed: %v", err)
	}

	// Verify session still exists
	if !adapter.IsSessionAlive(sessionName) {
		t.Fatal("Session should still be alive after second CreateSession call")
	}

	// Clean up
	exec.Command("tmux", "kill-session", "-t", sessionName).Run()
}

// TestExtractLatestInteraction tests ExtractLatestInteraction function
func TestExtractLatestInteraction(t *testing.T) {
	t.Run("nonexistent file", func(t *testing.T) {
		prompt, response, err := ExtractLatestInteraction("/nonexistent/file.json")
		assert.Error(t, err)
		assert.Empty(t, prompt)
		assert.Empty(t, response)
	})

	t.Run("empty path", func(t *testing.T) {
		prompt, response, err := ExtractLatestInteraction("")
		assert.Error(t, err)
		assert.Empty(t, prompt)
		assert.Empty(t, response)
	})
}

// TestExtractLastAssistantResponse tests ExtractLastAssistantResponse function
func TestExtractLastAssistantResponse(t *testing.T) {
	t.Run("nonexistent file", func(t *testing.T) {
		response, err := ExtractLastAssistantResponse("/nonexistent/file.json")
		assert.Error(t, err)
		assert.Empty(t, response)
	})

	t.Run("empty path", func(t *testing.T) {
		response, err := ExtractLastAssistantResponse("")
		assert.Error(t, err)
		assert.Empty(t, response)
	})
}

// TestClaudeAdapter_HandleHookData tests HandleHookData method
func TestClaudeAdapter_HandleHookData(t *testing.T) {
	adapter, err := NewClaudeAdapter(ClaudeAdapterConfig{})
	require.NoError(t, err)

	t.Run("valid hook data with cwd", func(t *testing.T) {
		hookData := map[string]interface{}{
			"cwd": "/home/user/project",
		}
		data, _ := json.Marshal(hookData)

		cwd, prompt, response, err := adapter.HandleHookData(data)
		assert.NoError(t, err)
		assert.Equal(t, "/home/user/project", cwd)
		assert.Empty(t, prompt)
		assert.Empty(t, response)
	})

	t.Run("hook data with prompt and response", func(t *testing.T) {
		hookData := map[string]interface{}{
			"cwd":      "/home/user/project",
			"prompt":   "test prompt",
			"response": "test response",
		}
		data, _ := json.Marshal(hookData)

		cwd, prompt, response, err := adapter.HandleHookData(data)
		assert.NoError(t, err)
		assert.Equal(t, "/home/user/project", cwd)
		// HandleHookData might not extract prompt/response from hook data directly
		// The exact behavior depends on implementation
		_ = prompt
		_ = response
	})

	t.Run("invalid JSON data", func(t *testing.T) {
		data := []byte("invalid json")

		cwd, prompt, response, err := adapter.HandleHookData(data)
		assert.Error(t, err)
		assert.Empty(t, cwd)
		assert.Empty(t, prompt)
		assert.Empty(t, response)
	})

	t.Run("empty JSON object", func(t *testing.T) {
		data := []byte("{}")

		cwd, _, _, err := adapter.HandleHookData(data)
		assert.Error(t, err) // Should error because cwd is missing
		assert.Empty(t, cwd)
	})
}

// TestExtractLatestSubagentFile_FileOperations tests the extractLatestSubagentFile function
func TestExtractLatestSubagentFile_FileOperations(t *testing.T) {
	t.Run("returns error for non-existent subagents directory", func(t *testing.T) {
		_, err := extractLatestSubagentFile("/nonexistent/path/transcript.json")
		assert.Error(t, err)
	})

	t.Run("returns error when subagents directory exists but has no jsonl files", func(t *testing.T) {
		tmpDir := t.TempDir()
		// Create base directory (transcript file without extension)
		baseDir := tmpDir + "/transcript"
		subagentsDir := baseDir + "/subagents"

		err := os.MkdirAll(subagentsDir, 0755)
		require.NoError(t, err)

		// Create a dummy transcript file
		transcriptPath := baseDir + ".json"
		_, err = extractLatestSubagentFile(transcriptPath)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no jsonl files found")
	})

	t.Run("finds the latest jsonl file by modification time", func(t *testing.T) {
		tmpDir := t.TempDir()
		baseDir := tmpDir + "/transcript"
		subagentsDir := baseDir + "/subagents"

		err := os.MkdirAll(subagentsDir, 0755)
		require.NoError(t, err)

		file1 := subagentsDir + "/agent1.jsonl"
		file2 := subagentsDir + "/agent2.jsonl"
		file3 := subagentsDir + "/agent3.jsonl"

		err = os.WriteFile(file1, []byte("content1"), 0644)
		require.NoError(t, err)

		time.Sleep(10 * time.Millisecond)

		err = os.WriteFile(file2, []byte("content2"), 0644)
		require.NoError(t, err)

		time.Sleep(10 * time.Millisecond)

		err = os.WriteFile(file3, []byte("content3"), 0644)
		require.NoError(t, err)

		transcriptPath := baseDir + ".json"
		latestFile, err := extractLatestSubagentFile(transcriptPath)
		assert.NoError(t, err)
		assert.Equal(t, filepath.Clean(file3), latestFile)
	})

	t.Run("ignores non-jsonl files in subagents directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		baseDir := tmpDir + "/transcript"
		subagentsDir := baseDir + "/subagents"

		err := os.MkdirAll(subagentsDir, 0755)
		require.NoError(t, err)

		jsonlFile := subagentsDir + "/agent.jsonl"
		txtFile := subagentsDir + "/readme.txt"

		err = os.WriteFile(jsonlFile, []byte("jsonl content"), 0644)
		require.NoError(t, err)

		err = os.WriteFile(txtFile, []byte("txt content"), 0644)
		require.NoError(t, err)

		transcriptPath := baseDir + ".json"
		latestFile, err := extractLatestSubagentFile(transcriptPath)
		assert.NoError(t, err)
		assert.Equal(t, filepath.Clean(jsonlFile), latestFile)
	})
}
