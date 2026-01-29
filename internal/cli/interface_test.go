package cli

import (
	"testing"
)

// TestClaudeAdapter_CLIAdapterInterface verifies that ClaudeAdapter implements CLIAdapter
func TestClaudeAdapter_CLIAdapterInterface(t *testing.T) {
	var _ CLIAdapter = (*ClaudeAdapter)(nil)

	adapter := NewClaudeAdapter(ClaudeAdapterConfig{
		HistoryDir: "/tmp/test/conversations",
		CheckLines: 3,
		Patterns:   []string{`\? \[y/N\]`},
	})

	// Verify all methods exist and have correct signatures
	_ = adapter.SendInput
	_ = adapter.GetLastResponse
	_ = adapter.IsSessionAlive
	_ = adapter.CreateSession
	_ = adapter.CheckInteractive
}
