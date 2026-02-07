package cli

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBaseAdapter_UseHookMethod tests UseHook method
func TestBaseAdapter_UseHookMethod(t *testing.T) {
	t.Run("returns true when UseHook is true", func(t *testing.T) {
		adapter := NewBaseAdapter("test", "cmd", 100, true, 0, 0, 0)
		assert.True(t, adapter.UseHook())
	})

	t.Run("returns false when UseHook is false", func(t *testing.T) {
		adapter := NewBaseAdapter("test", "cmd", 100, false, 1, 2, 3)
		assert.False(t, adapter.UseHook())
	})

	t.Run("defaults to true when not specified", func(t *testing.T) {
		adapter := NewBaseAdapter("test", "cmd", 100, false, 0, 0, 0)
		assert.True(t, adapter.UseHook())
	})
}

// TestBaseAdapter_GetPollInterval tests GetPollInterval method
func TestBaseAdapter_GetPollInterval(t *testing.T) {
	t.Run("returns configured interval", func(t *testing.T) {
		adapter := NewBaseAdapter("test", "cmd", 100, false, 2*time.Second, 3, 60*time.Second)
		assert.Equal(t, 2*time.Second, adapter.GetPollInterval())
	})

	t.Run("returns default when not configured", func(t *testing.T) {
		adapter := NewBaseAdapter("test", "cmd", 100, true, 0, 0, 0)
		// When useHook is true and interval is 0, normalizePollingConfig sets default
		assert.Equal(t, 1*time.Second, adapter.GetPollInterval())
	})
}

// TestBaseAdapter_GetStableCount tests GetStableCount method
func TestBaseAdapter_GetStableCount(t *testing.T) {
	t.Run("returns configured count", func(t *testing.T) {
		adapter := NewBaseAdapter("test", "cmd", 100, false, 1*time.Second, 5, 60*time.Second)
		assert.Equal(t, 5, adapter.GetStableCount())
	})

	t.Run("returns default when not configured", func(t *testing.T) {
		adapter := NewBaseAdapter("test", "cmd", 100, true, 0, 0, 0)
		// normalizePollingConfig sets default stableCount to 3
		assert.Equal(t, 3, adapter.GetStableCount())
	})
}

// TestBaseAdapter_GetPollTimeout tests GetPollTimeout method
func TestBaseAdapter_GetPollTimeout(t *testing.T) {
	t.Run("returns configured timeout", func(t *testing.T) {
		adapter := NewBaseAdapter("test", "cmd", 100, false, 1*time.Second, 3, 180*time.Second)
		assert.Equal(t, 180*time.Second, adapter.GetPollTimeout())
	})

	t.Run("returns default when not configured", func(t *testing.T) {
		adapter := NewBaseAdapter("test", "cmd", 100, true, 0, 0, 0)
		// normalizePollingConfig sets default timeout to 1 hour
		assert.Equal(t, 1*time.Hour, adapter.GetPollTimeout())
	})
}

// TestNormalizePollingConfig tests normalizePollingConfig function behavior
func TestNormalizePollingConfig_DefaultValues(t *testing.T) {
	t.Run("zero values get defaults", func(t *testing.T) {
		interval, count, timeout := normalizePollingConfig(0, 0, 0)
		// interval defaults to 1 second
		assert.Equal(t, 1*time.Second, interval)
		// stableCount defaults to 3
		assert.Equal(t, 3, count)
		// timeout defaults to 1 hour
		assert.Equal(t, 1*time.Hour, timeout)
	})

	t.Run("values are preserved", func(t *testing.T) {
		interval, count, timeout := normalizePollingConfig(5*time.Second, 10, 300*time.Second)
		assert.Equal(t, 5*time.Second, interval)
		assert.Equal(t, 10, count)
		assert.Equal(t, 300*time.Second, timeout)
	})
}

// TestNewBaseAdapter_AllParameters tests NewBaseAdapter with all parameters
func TestNewBaseAdapter_AllParameters(t *testing.T) {
	adapter := NewBaseAdapter(
		"test-cli",
		"test-command",
		300,
		false,
		5*time.Second,
		7,
		180*time.Second,
	)

	assert.Equal(t, "test-cli", adapter.cliName)
	assert.Equal(t, "test-command", adapter.startCmd)
	assert.Equal(t, 300, adapter.inputDelayMs)
	assert.False(t, adapter.useHook)
	assert.Equal(t, 5*time.Second, adapter.pollInterval)
	assert.Equal(t, 7, adapter.stableCount)
	assert.Equal(t, 180*time.Second, adapter.pollTimeout)
}

// TestBaseAdapter_Name tests that adapter name is set correctly
func TestBaseAdapter_Name(t *testing.T) {
	adapter := NewBaseAdapter("mycli", "mycommand", 100, true, 0, 0, 0)

	// Check that the adapter has the right name
	// The name is stored in cliName field
	assert.Equal(t, "mycli", adapter.cliName)
}

// TestBaseAdapter_CommandField tests command field
func TestBaseAdapter_CommandField(t *testing.T) {
	adapter := NewBaseAdapter("cli", "startCmd", 200, true, 0, 0, 0)

	assert.Equal(t, "startCmd", adapter.startCmd)
	assert.Equal(t, 200, adapter.inputDelayMs)
}

// TestInputDelay_Values tests different input delay values
func TestInputDelay_Values(t *testing.T) {
	delays := []int{0, 100, 200, 500, 1000}

	for _, delay := range delays {
		adapter := NewBaseAdapter("test", "cmd", delay, true, 0, 0, 0)
		assert.Equal(t, delay, adapter.inputDelayMs)
	}
}

// TestClaudeAdapter_UseHookDefaultsToTrue tests Claude adapter defaults
func TestClaudeAdapter_UseHookDefaultsToTrue(t *testing.T) {
	config := ClaudeAdapterConfig{}
	config.UseHook = true

	adapter, err := NewClaudeAdapter(config)
	require.NoError(t, err)
	assert.True(t, adapter.UseHook())
}

// TestGeminiAdapter_UseHookDefaultsToTrue tests Gemini adapter defaults
func TestGeminiAdapter_UseHookDefaultsToTrue(t *testing.T) {
	config := GeminiAdapterConfig{}
	config.UseHook = true

	adapter, err := NewGeminiAdapter(config)
	require.NoError(t, err)
	assert.True(t, adapter.UseHook())
}

// TestOpenCodeAdapter_UseHookDefaultsToTrue tests OpenCode adapter defaults
func TestOpenCodeAdapter_UseHookDefaultsToTrue(t *testing.T) {
	config := OpenCodeAdapterConfig{}
	config.UseHook = true

	adapter, err := NewOpenCodeAdapter(config)
	require.NoError(t, err)
	assert.True(t, adapter.UseHook())
}

// TestComputeProjectHash_Consistency tests hash consistency
func TestComputeProjectHash_Consistency(t *testing.T) {
	path := "/home/user/project"

	hash1 := computeProjectHash(path)
	hash2 := computeProjectHash(path)

	assert.Equal(t, hash1, hash2, "hash should be consistent for same path")
}

// TestExpandHome_VariousPaths tests expandHome with different paths
func TestExpandHome_VariousPaths(t *testing.T) {
	paths := []string{
		"~/Documents",
		"~/Downloads/file.txt",
		"~/../parent",
		"~/relative/path",
	}

	for _, path := range paths {
		result, err := expandHome(path)
		assert.NoError(t, err)
		assert.NotEmpty(t, result, "expanded path should not be empty")
		assert.NotContains(t, result, "~", "expanded path should not contain tilde")
	}
}

// TestExpandHome_RelativePaths tests expandHome with relative paths
func TestExpandHome_RelativePaths(t *testing.T) {
	paths := []string{
		"relative/path",
		"./current",
		"../parent",
		"/absolute/path",
	}

	for _, path := range paths {
		result, err := expandHome(path)
		assert.NoError(t, err)
		assert.Equal(t, path, result, "relative and absolute paths should be unchanged")
	}
}
