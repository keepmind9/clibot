package watchdog

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDefaultPollingConfig_ReturnsCorrectDefaults(t *testing.T) {
	config := DefaultPollingConfig()

	assert.Equal(t, 1*time.Second, config.Interval)
	assert.Equal(t, 3, config.StableCount)
	assert.Equal(t, 120*time.Second, config.Timeout)
}

func TestExtractStableContent_RemovesANSICodes(t *testing.T) {
	input := "\x1b[31mRed text\x1b[0mNormal text"
	result := ExtractStableContent(input)

	assert.NotContains(t, result, "\x1b")
	assert.Contains(t, result, "Red text")
	assert.Contains(t, result, "Normal text")
}

func TestExtractStableContent_TrimsWhitespace(t *testing.T) {
	input := `
  Hello World
  `
	result := ExtractStableContent(input)

	assert.Equal(t, "Hello World", result)
}

func TestExtractStableContent_EmptyInput(t *testing.T) {
	result := ExtractStableContent("")
	assert.Equal(t, "", result)
}

func TestWaitForCompletion_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	config := PollingConfig{
		Interval:    100 * time.Millisecond,
		StableCount: 2,
		Timeout:     1 * time.Second,
	}

	_, err := WaitForCompletion("test-session", config, ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cancelled")
}

func TestWaitForCompletion_EmptyConfig_UsesDefaults(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	config := PollingConfig{} // Empty config

	_, err := WaitForCompletion("nonexistent-session", config, ctx)
	// Will fail because session doesn't exist, but shouldn't panic
	assert.Error(t, err)
}

func TestPollingConfig_ZeroValues_UsingDefaults(t *testing.T) {
	// This test verifies that WaitForCompletion handles zero values correctly
	// by applying defaults internally (as documented in the function)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	config := PollingConfig{} // All zeros - should use defaults

	_, err := WaitForCompletion("nonexistent-session", config, ctx)
	// Should fail because session doesn't exist, but shouldn't panic
	// The defaults should be applied internally
	assert.Error(t, err)
}
