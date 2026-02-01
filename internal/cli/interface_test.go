package cli

import (
	"testing"
	"time"
)

// TestClaudeAdapter_CLIAdapterInterface verifies that ClaudeAdapter implements CLIAdapter
func TestClaudeAdapter_CLIAdapterInterface(t *testing.T) {
	var _ CLIAdapter = (*ClaudeAdapter)(nil)

	adapter, err := NewClaudeAdapter(ClaudeAdapterConfig{
		UseHook:      true,
		PollInterval: 1 * time.Second,
		StableCount:  3,
		PollTimeout:  120 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewClaudeAdapter failed: %v", err)
	}

	// Verify all methods exist and have correct signatures
	_ = adapter.SendInput
	_ = adapter.HandleHookData
	_ = adapter.IsSessionAlive
	_ = adapter.CreateSession
	_ = adapter.UseHook
	_ = adapter.GetPollInterval
	_ = adapter.GetStableCount
	_ = adapter.GetPollTimeout
}
