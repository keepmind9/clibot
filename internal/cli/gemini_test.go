package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestComputeProjectHash tests the computeProjectHash function
func TestComputeProjectHash(t *testing.T) {
	t.Run("returns a valid SHA256 hash", func(t *testing.T) {
		result := computeProjectHash("/home/user/project")
		assert.NotEmpty(t, result)
		assert.Len(t, result, 64) // SHA256 hex string
	})

	t.Run("returns consistent hash for same path", func(t *testing.T) {
		path := "/home/user/project"
		hash1 := computeProjectHash(path)
		hash2 := computeProjectHash(path)
		assert.Equal(t, hash1, hash2)
	})

	t.Run("returns different hashes for different paths", func(t *testing.T) {
		hash1 := computeProjectHash("/home/user/project1")
		hash2 := computeProjectHash("/home/user/project2")
		assert.NotEqual(t, hash1, hash2)
	})

	t.Run("handles relative paths", func(t *testing.T) {
		result := computeProjectHash("./project")
		assert.NotEmpty(t, result)
		assert.Len(t, result, 64) // SHA256 hex string
	})
}

// TestNewGeminiAdapter tests the NewGeminiAdapter function
func TestNewGeminiAdapter(t *testing.T) {
	t.Run("creates adapter with default config", func(t *testing.T) {
		config := GeminiAdapterConfig{
			UseHook:      true,
			PollInterval: time.Second,
			StableCount:  3,
			PollTimeout:  120 * time.Second,
		}

		adapter, err := NewGeminiAdapter(config)

		assert.NoError(t, err)
		assert.NotNil(t, adapter)
		assert.True(t, adapter.UseHook())
	})

	t.Run("creates adapter with polling mode", func(t *testing.T) {
		config := GeminiAdapterConfig{
			UseHook:      false,
			PollInterval: 2 * time.Second,
			StableCount:  5,
			PollTimeout:  60 * time.Second,
		}

		adapter, err := NewGeminiAdapter(config)

		assert.NoError(t, err)
		assert.NotNil(t, adapter)
		assert.False(t, adapter.UseHook())
		assert.Equal(t, 2*time.Second, adapter.GetPollInterval())
		assert.Equal(t, 5, adapter.GetStableCount())
		assert.Equal(t, 60*time.Second, adapter.GetPollTimeout())
	})
}

// TestGeminiAdapter_HandleHookData tests the HandleHookData function
func TestGeminiAdapter_HandleHookData(t *testing.T) {
	t.Run("valid hook data with cwd", func(t *testing.T) {
		config := GeminiAdapterConfig{UseHook: true}
		adapter, err := NewGeminiAdapter(config)
		require.NoError(t, err)

		hookData := map[string]interface{}{
			"cwd":             "/home/user/project",
			"transcript_path": "/path/to/session.json",
		}
		data, _ := json.Marshal(hookData)

		cwd, prompt, response, err := adapter.HandleHookData(data)

		assert.NoError(t, err)
		assert.Equal(t, "/home/user/project", cwd)
		assert.Equal(t, "", prompt)
		assert.Equal(t, "", response)
	})

	t.Run("hook data with notification event", func(t *testing.T) {
		config := GeminiAdapterConfig{UseHook: true}
		adapter, err := NewGeminiAdapter(config)
		require.NoError(t, err)

		hookData := map[string]interface{}{
			"cwd":              "/home/user/project",
			"hook_event_name":  "NotificationEvent",
			"transcript_path":  "/path/to/session.json",
		}
		data, _ := json.Marshal(hookData)

		cwd, prompt, response, err := adapter.HandleHookData(data)

		assert.NoError(t, err)
		assert.Equal(t, "/home/user/project", cwd)
		assert.Equal(t, "", prompt)
		assert.Equal(t, "", response)
	})

	t.Run("invalid JSON", func(t *testing.T) {
		config := GeminiAdapterConfig{UseHook: true}
		adapter, err := NewGeminiAdapter(config)
		require.NoError(t, err)

		data := []byte("invalid json")

		cwd, prompt, response, err := adapter.HandleHookData(data)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse JSON data")
		assert.Equal(t, "", cwd)
		assert.Equal(t, "", prompt)
		assert.Equal(t, "", response)
	})

	t.Run("missing cwd in hook data", func(t *testing.T) {
		config := GeminiAdapterConfig{UseHook: true}
		adapter, err := NewGeminiAdapter(config)
		require.NoError(t, err)

		hookData := map[string]interface{}{
			"transcript_path": "/path/to/session.json",
		}
		data, _ := json.Marshal(hookData)

		cwd, prompt, response, err := adapter.HandleHookData(data)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing cwd in hook data")
		assert.Equal(t, "", cwd)
		assert.Equal(t, "", prompt)
		assert.Equal(t, "", response)
	})

	t.Run("empty hook data", func(t *testing.T) {
		config := GeminiAdapterConfig{UseHook: true}
		adapter, err := NewGeminiAdapter(config)
		require.NoError(t, err)

		hookData := map[string]interface{}{}
		data, _ := json.Marshal(hookData)

		_, _, _, err = adapter.HandleHookData(data)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing cwd in hook data")
	})
}

// TestGeminiAdapter_LastSessionFile tests the lastSessionFile function
func TestGeminiAdapter_LastSessionFile(t *testing.T) {
	t.Run("directory does not exist", func(t *testing.T) {
		config := GeminiAdapterConfig{UseHook: true}
		adapter, err := NewGeminiAdapter(config)
		require.NoError(t, err)

		// Use a non-existent path
		nonExistentPath := "/non/existent/path/that/does/not/exist"
		file, err := adapter.lastSessionFile(nonExistentPath)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "chats directory not found")
		assert.Equal(t, "", file)
	})

	t.Run("directory exists but no session files", func(t *testing.T) {
		config := GeminiAdapterConfig{UseHook: true}
		adapter, err := NewGeminiAdapter(config)
		require.NoError(t, err)

		// Create a temporary directory
		tmpDir := t.TempDir()
		homeDir, _ := os.UserHomeDir()
		projectHash := computeProjectHash(tmpDir)
		chatsDir := filepath.Join(homeDir, ".gemini", "tmp", projectHash, "chats")

		// Create directory structure
		err = os.MkdirAll(chatsDir, 0755)
		require.NoError(t, err)
		defer os.RemoveAll(filepath.Join(homeDir, ".gemini"))

		file, err := adapter.lastSessionFile(tmpDir)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no session files found")
		assert.Equal(t, "", file)
	})
}

// TestGeminiAdapter_ExtractLatestInteraction tests the ExtractLatestInteraction method
func TestGeminiAdapter_ExtractLatestInteraction(t *testing.T) {
	t.Run("transcript path provided", func(t *testing.T) {
		config := GeminiAdapterConfig{UseHook: true}
		adapter, err := NewGeminiAdapter(config)
		require.NoError(t, err)

		// Create a temporary session file
		tmpDir := t.TempDir()
		sessionFile := filepath.Join(tmpDir, "session.json")

		sessionData := struct {
			Messages []struct {
				Type    string `json:"type"`
				Content string `json:"content"`
			} `json:"messages"`
		}{
			Messages: []struct {
				Type    string `json:"type"`
				Content string `json:"content"`
			}{
				{Type: "user", Content: "Hello"},
				{Type: "gemini", Content: "Hi there!"},
			},
		}

		data, _ := json.Marshal(sessionData)
		err = os.WriteFile(sessionFile, data, 0644)
		require.NoError(t, err)

		prompt, response, err := adapter.ExtractLatestInteraction(sessionFile, "")

		assert.NoError(t, err)
		assert.Equal(t, "Hello", prompt)
		assert.Equal(t, "Hi there!", response)
	})

	t.Run("no messages in session", func(t *testing.T) {
		config := GeminiAdapterConfig{UseHook: true}
		adapter, err := NewGeminiAdapter(config)
		require.NoError(t, err)

		// Create a temporary session file with no messages
		tmpDir := t.TempDir()
		sessionFile := filepath.Join(tmpDir, "session.json")

		sessionData := struct {
			Messages []struct {
				Type    string `json:"type"`
				Content string `json:"content"`
			} `json:"messages"`
		}{
			Messages: []struct {
				Type    string `json:"type"`
				Content string `json:"content"`
			}{},
		}

		data, _ := json.Marshal(sessionData)
		err = os.WriteFile(sessionFile, data, 0644)
		require.NoError(t, err)

		prompt, response, err := adapter.ExtractLatestInteraction(sessionFile, "/some/path")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no messages in session file")
		assert.Equal(t, "", prompt)
		assert.Equal(t, "", response)
	})

	t.Run("no user message in session", func(t *testing.T) {
		config := GeminiAdapterConfig{UseHook: true}
		adapter, err := NewGeminiAdapter(config)
		require.NoError(t, err)

		// Create a temporary session file with only gemini messages
		tmpDir := t.TempDir()
		sessionFile := filepath.Join(tmpDir, "session.json")

		sessionData := struct {
			Messages []struct {
				Type    string `json:"type"`
				Content string `json:"content"`
			} `json:"messages"`
		}{
			Messages: []struct {
				Type    string `json:"type"`
				Content string `json:"content"`
			}{
				{Type: "gemini", Content: "Hello!"},
			},
		}

		data, _ := json.Marshal(sessionData)
		err = os.WriteFile(sessionFile, data, 0644)
		require.NoError(t, err)

		prompt, response, err := adapter.ExtractLatestInteraction(sessionFile, "/some/path")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no user message found")
		assert.Equal(t, "", prompt)
		assert.Equal(t, "", response)
	})

	t.Run("multiple gemini responses", func(t *testing.T) {
		config := GeminiAdapterConfig{UseHook: true}
		adapter, err := NewGeminiAdapter(config)
		require.NoError(t, err)

		// Create a temporary session file with multiple responses
		tmpDir := t.TempDir()
		sessionFile := filepath.Join(tmpDir, "session.json")

		sessionData := struct {
			Messages []struct {
				Type    string `json:"type"`
				Content string `json:"content"`
			} `json:"messages"`
		}{
			Messages: []struct {
				Type    string `json:"type"`
				Content string `json:"content"`
			}{
				{Type: "user", Content: "Explain this"},
				{Type: "gemini", Content: "Part 1"},
				{Type: "gemini", Content: "Part 2"},
			},
		}

		data, _ := json.Marshal(sessionData)
		err = os.WriteFile(sessionFile, data, 0644)
		require.NoError(t, err)

		prompt, response, err := adapter.ExtractLatestInteraction(sessionFile, "/some/path")

		assert.NoError(t, err)
		assert.Equal(t, "Explain this", prompt)
		assert.Equal(t, "Part 1\n\nPart 2", response)
	})

	t.Run("file does not exist", func(t *testing.T) {
		config := GeminiAdapterConfig{UseHook: true}
		adapter, err := NewGeminiAdapter(config)
		require.NoError(t, err)

		prompt, response, err := adapter.ExtractLatestInteraction("/nonexistent/file.json", "/some/path")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to read session file")
		assert.Equal(t, "", prompt)
		assert.Equal(t, "", response)
	})
}

// TestGeminiAdapter_UseHook tests the UseHook method
func TestGeminiAdapter_UseHook(t *testing.T) {
	t.Run("hook mode enabled", func(t *testing.T) {
		config := GeminiAdapterConfig{UseHook: true}
		adapter, err := NewGeminiAdapter(config)
		require.NoError(t, err)

		assert.True(t, adapter.UseHook())
	})

	t.Run("polling mode with interval configured", func(t *testing.T) {
		// Must configure interval to disable hook mode
		config := GeminiAdapterConfig{
			UseHook:      false,
			PollInterval: time.Second,
		}
		adapter, err := NewGeminiAdapter(config)
		require.NoError(t, err)

		assert.False(t, adapter.UseHook())
	})

	t.Run("defaults to hook mode when polling not configured", func(t *testing.T) {
		// When UseHook=false but no polling config, it defaults to hook mode
		config := GeminiAdapterConfig{UseHook: false}
		adapter, err := NewGeminiAdapter(config)
		require.NoError(t, err)

		assert.True(t, adapter.UseHook())
	})
}

// TestGeminiAdapter_GetPollInterval tests the GetPollInterval method
func TestGeminiAdapter_GetPollInterval(t *testing.T) {
	config := GeminiAdapterConfig{
		UseHook:      false,
		PollInterval: 5 * time.Second,
	}
	adapter, err := NewGeminiAdapter(config)
	require.NoError(t, err)

	assert.Equal(t, 5*time.Second, adapter.GetPollInterval())
}

// TestGeminiAdapter_GetStableCount tests the GetStableCount method
func TestGeminiAdapter_GetStableCount(t *testing.T) {
	config := GeminiAdapterConfig{
		UseHook:     false,
		StableCount: 7,
	}
	adapter, err := NewGeminiAdapter(config)
	require.NoError(t, err)

	assert.Equal(t, 7, adapter.GetStableCount())
}

// TestGeminiAdapter_GetPollTimeout tests the GetPollTimeout method
func TestGeminiAdapter_GetPollTimeout(t *testing.T) {
	config := GeminiAdapterConfig{
		UseHook:     false,
		PollTimeout: 180 * time.Second,
	}
	adapter, err := NewGeminiAdapter(config)
	require.NoError(t, err)

	assert.Equal(t, 180*time.Second, adapter.GetPollTimeout())
}
