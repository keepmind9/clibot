package cli

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewOpenCodeAdapter tests the NewOpenCodeAdapter function
func TestNewOpenCodeAdapter(t *testing.T) {
	t.Run("creates adapter with hook mode", func(t *testing.T) {
		config := OpenCodeAdapterConfig{
			UseHook: true,
		}

		adapter, err := NewOpenCodeAdapter(config)

		assert.NoError(t, err)
		assert.NotNil(t, adapter)
		assert.True(t, adapter.UseHook())
	})

	t.Run("creates adapter with polling mode", func(t *testing.T) {
		config := OpenCodeAdapterConfig{
			UseHook:      false,
			PollInterval: 2 * time.Second,
			StableCount:  5,
			PollTimeout:  60 * time.Second,
		}

		adapter, err := NewOpenCodeAdapter(config)

		assert.NoError(t, err)
		assert.NotNil(t, adapter)
		assert.False(t, adapter.UseHook())
		assert.Equal(t, 2*time.Second, adapter.GetPollInterval())
		assert.Equal(t, 5, adapter.GetStableCount())
		assert.Equal(t, 60*time.Second, adapter.GetPollTimeout())
	})

	t.Run("defaults to hook mode when polling not configured", func(t *testing.T) {
		config := OpenCodeAdapterConfig{
			UseHook: false,
		}

		adapter, err := NewOpenCodeAdapter(config)

		assert.NoError(t, err)
		assert.NotNil(t, adapter)
		assert.True(t, adapter.UseHook())
	})
}

// TestOpenCodeAdapter_HandleHookData tests the HandleHookData function
func TestOpenCodeAdapter_HandleHookData(t *testing.T) {
	t.Run("valid hook data with cwd", func(t *testing.T) {
		config := OpenCodeAdapterConfig{UseHook: true}
		adapter, err := NewOpenCodeAdapter(config)
		require.NoError(t, err)

		hookData := map[string]interface{}{
			"cwd":        "/home/user/project",
			"session_id": "test-session",
		}
		data, _ := json.Marshal(hookData)

		cwd, prompt, response, err := adapter.HandleHookData(data)

		assert.NoError(t, err)
		assert.Equal(t, "/home/user/project", cwd)
		assert.Equal(t, "", prompt)
		assert.Equal(t, "", response)
	})

	t.Run("invalid JSON", func(t *testing.T) {
		config := OpenCodeAdapterConfig{UseHook: true}
		adapter, err := NewOpenCodeAdapter(config)
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
		config := OpenCodeAdapterConfig{UseHook: true}
		adapter, err := NewOpenCodeAdapter(config)
		require.NoError(t, err)

		hookData := map[string]interface{}{
			"session_id": "test-session",
		}
		data, _ := json.Marshal(hookData)

		cwd, prompt, response, err := adapter.HandleHookData(data)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing cwd in hook data")
		assert.Equal(t, "", cwd)
		assert.Equal(t, "", prompt)
		assert.Equal(t, "", response)
	})

	t.Run("notification event clears response", func(t *testing.T) {
		config := OpenCodeAdapterConfig{UseHook: true}
		adapter, err := NewOpenCodeAdapter(config)
		require.NoError(t, err)

		hookData := map[string]interface{}{
			"cwd":           "/home/user/project",
			"session_id":    "test-session",
			"hook_event_name": "Notification",
		}
		data, _ := json.Marshal(hookData)

		cwd, prompt, response, err := adapter.HandleHookData(data)

		assert.NoError(t, err)
		assert.Equal(t, "/home/user/project", cwd)
		assert.Equal(t, "", prompt)
		assert.Equal(t, "", response)
	})

	t.Run("empty hook data", func(t *testing.T) {
		config := OpenCodeAdapterConfig{UseHook: true}
		adapter, err := NewOpenCodeAdapter(config)
		require.NoError(t, err)

		hookData := map[string]interface{}{}
		data, _ := json.Marshal(hookData)

		_, _, _, err = adapter.HandleHookData(data)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing cwd in hook data")
	})
}

// TestOpenCodeAdapter_PollingMethods tests polling-related methods
func TestOpenCodeAdapter_PollingMethods(t *testing.T) {
	config := OpenCodeAdapterConfig{
		UseHook:      false,
		PollInterval: 3 * time.Second,
		StableCount:  7,
		PollTimeout:  180 * time.Second,
	}

	adapter, err := NewOpenCodeAdapter(config)
	require.NoError(t, err)

	assert.Equal(t, 3*time.Second, adapter.GetPollInterval())
	assert.Equal(t, 7, adapter.GetStableCount())
	assert.Equal(t, 180*time.Second, adapter.GetPollTimeout())
}

// TestOpenCodeAdapter_ExtractLatestInteraction tests ExtractLatestInteraction method
func TestOpenCodeAdapter_ExtractLatestInteraction(t *testing.T) {
	config := OpenCodeAdapterConfig{
		UseHook: true,
	}
	adapter, err := NewOpenCodeAdapter(config)
	require.NoError(t, err)

	t.Run("nonexistent file", func(t *testing.T) {
		prompt, response, err := adapter.ExtractLatestInteraction("/nonexistent/file.json")
		assert.Error(t, err)
		assert.Empty(t, prompt)
		assert.Empty(t, response)
	})

	t.Run("empty path", func(t *testing.T) {
		prompt, response, err := adapter.ExtractLatestInteraction("")
		assert.Error(t, err)
		assert.Empty(t, prompt)
		assert.Empty(t, response)
	})
}
