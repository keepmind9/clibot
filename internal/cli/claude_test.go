package cli

import (
	"fmt"
	"os/exec"
	"testing"
	"time"
)

func TestClaudeAdapter_NewClaudeAdapter(t *testing.T) {
	adapter, err := NewClaudeAdapter(ClaudeAdapterConfig{
		UseHook:      true,
		PollInterval: 1 * time.Second,
		StableCount:  3,
		PollTimeout:  120 * time.Second,
	})

	if err != nil {
		t.Fatalf("NewClaudeAdapter returned error: %v", err)
	}

	if adapter == nil {
		t.Fatal("NewClaudeAdapter returned nil")
	}

	// Verify polling config is set correctly
	if adapter.useHook != true {
		t.Errorf("expected useHook true, got %v", adapter.useHook)
	}

	if adapter.pollInterval != 1*time.Second {
		t.Errorf("expected pollInterval 1s, got %v", adapter.pollInterval)
	}

	if adapter.stableCount != 3 {
		t.Errorf("expected stableCount 3, got %d", adapter.stableCount)
	}

	if adapter.pollTimeout != 120*time.Second {
		t.Errorf("expected pollTimeout 120s, got %v", adapter.pollTimeout)
	}
}

func TestClaudeAdapter_NewClaudeAdapter_Defaults(t *testing.T) {
	// Test with zero values - should use defaults
	adapter, err := NewClaudeAdapter(ClaudeAdapterConfig{})

	if err != nil {
		t.Fatalf("NewClaudeAdapter returned error: %v", err)
	}

	// Verify defaults are applied
	if adapter.pollInterval != 1*time.Second {
		t.Errorf("expected default pollInterval 1s, got %v", adapter.pollInterval)
	}

	if adapter.stableCount != 3 {
		t.Errorf("expected default stableCount 3, got %d", adapter.stableCount)
	}

	if adapter.pollTimeout != 1*time.Hour {
		t.Errorf("expected default pollTimeout 1h, got %v", adapter.pollTimeout)
	}
}

func TestClaudeAdapter_SendInput(t *testing.T) {
	adapter, err := NewClaudeAdapter(ClaudeAdapterConfig{
		UseHook:      true,
		PollInterval: 1 * time.Second,
		StableCount:  3,
		PollTimeout:  120 * time.Second,
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

func TestClaudeAdapter_UseHook(t *testing.T) {
	tests := []struct {
		name     string
		config   ClaudeAdapterConfig
		expected bool
	}{
		{
			name: "hook mode enabled",
			config: ClaudeAdapterConfig{
				UseHook: true,
			},
			expected: true,
		},
		{
			name: "polling mode (explicitly configured)",
			config: ClaudeAdapterConfig{
				UseHook:      false,
				PollInterval: 1 * time.Second,
			},
			expected: false,
		},
		{
			name:     "default (hook mode)",
			config:   ClaudeAdapterConfig{},
			expected: true, // Default is true when not configured
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter, err := NewClaudeAdapter(tt.config)
			if err != nil {
				t.Fatalf("NewClaudeAdapter failed: %v", err)
			}

			if adapter.UseHook() != tt.expected {
				t.Errorf("expected UseHook=%v, got %v", tt.expected, adapter.UseHook())
			}
		})
	}
}

func TestClaudeAdapter_PollingConfig(t *testing.T) {
	adapter, err := NewClaudeAdapter(ClaudeAdapterConfig{
		UseHook:      false,
		PollInterval: 2 * time.Second,
		StableCount:  5,
		PollTimeout:  60 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewClaudeAdapter failed: %v", err)
	}

	if adapter.GetPollInterval() != 2*time.Second {
		t.Errorf("expected pollInterval 2s, got %v", adapter.GetPollInterval())
	}

	if adapter.GetStableCount() != 5 {
		t.Errorf("expected stableCount 5, got %d", adapter.GetStableCount())
	}

	if adapter.GetPollTimeout() != 60*time.Second {
		t.Errorf("expected pollTimeout 60s, got %v", adapter.GetPollTimeout())
	}
}

func TestClaudeAdapter_IsSessionAlive(t *testing.T) {
	adapter, err := NewClaudeAdapter(ClaudeAdapterConfig{
		UseHook:      true,
		PollInterval: 1 * time.Second,
		StableCount:  3,
		PollTimeout:  120 * time.Second,
	})
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
	adapter, err := NewClaudeAdapter(ClaudeAdapterConfig{
		UseHook:      true,
		PollInterval: 1 * time.Second,
		StableCount:  3,
		PollTimeout:  120 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewClaudeAdapter failed: %v", err)
	}

	// Test CreateSession with a unique name to avoid conflicts
	sessionName := "test-clibot-session-12345"

	err = adapter.CreateSession(sessionName, "/tmp", "claude")
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
	adapter, err := NewClaudeAdapter(ClaudeAdapterConfig{
		UseHook:      true,
		PollInterval: 1 * time.Second,
		StableCount:  3,
		PollTimeout:  120 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewClaudeAdapter failed: %v", err)
	}

	// Test idempotency: calling CreateSession multiple times should succeed
	sessionName := "test-clibot-idempotent"

	// Clean up first in case it exists from previous run
	exec.Command("tmux", "kill-session", "-t", sessionName).Run()

	// First call
	err = adapter.CreateSession(sessionName, "/tmp", "echo 'test'")
	if err != nil {
		t.Fatalf("First CreateSession failed: %v", err)
	}

	// Second call should succeed due to idempotency (session already exists)
	err = adapter.CreateSession(sessionName, "/tmp", "echo 'test'")
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
