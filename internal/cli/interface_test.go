package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestClaudeAdapter_CLIAdapterInterface verifies that ClaudeAdapter implements CLIAdapter
func TestClaudeAdapter_CLIAdapterInterface(t *testing.T) {
	var _ CLIAdapter = (*ClaudeAdapter)(nil)

	adapter, err := NewClaudeAdapter(ClaudeAdapterConfig{})
	if err != nil {
		t.Fatalf("NewClaudeAdapter failed: %v", err)
	}

	// Verify all methods exist and have correct signatures
	_ = adapter.SendInput
	_ = adapter.HandleHookData
	_ = adapter.IsSessionAlive
	_ = adapter.CreateSession
}

func TestSessionOptions(t *testing.T) {
	env := map[string]string{"KEY": "val"}

	opts := ApplySessionOptions([]SessionOption{
		WithWorkDir("/tmp"),
		WithStartCmd("codex"),
		WithTransportURL("stdio://"),
		WithEnv(env),
		WithYolo(true),
	})

	assert.Equal(t, "/tmp", opts.WorkDir)
	assert.Equal(t, "codex", opts.StartCmd)
	assert.Equal(t, "stdio://", opts.TransportURL)
	assert.Equal(t, env, opts.Env)
	assert.True(t, opts.Yolo)
}

func TestSessionOptions_Defaults(t *testing.T) {
	opts := ApplySessionOptions(nil)
	assert.Equal(t, "", opts.WorkDir)
	assert.Equal(t, "", opts.StartCmd)
	assert.Equal(t, "", opts.TransportURL)
	assert.Nil(t, opts.Env)
	assert.False(t, opts.Yolo)
}

func TestSessionOptions_Override(t *testing.T) {
	opts := ApplySessionOptions([]SessionOption{
		WithWorkDir("/a"),
		WithWorkDir("/b"),
	})
	assert.Equal(t, "/b", opts.WorkDir)
}
