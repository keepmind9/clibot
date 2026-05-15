package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestEngine_UpdateSessionState_ExistingSession tests updating state of existing session
func TestEngine_UpdateSessionState_ExistingSession(t *testing.T) {
	config := &Config{
		Sessions: []SessionConfig{
			{Name: "test", CLIType: "claude", WorkDir: "/tmp"},
		},
	}
	engine := NewEngine(config)

	// Create a session
	engine.sessions["test"] = &Session{
		Name:  "test",
		State: StateIdle,
	}

	// Update state
	engine.updateSessionState("test", StateProcessing)

	// Verify state was updated
	assert.Equal(t, StateProcessing, engine.sessions["test"].State)
}

// TestEngine_UpdateSessionState_NonExistingSession tests updating state of non-existing session
func TestEngine_UpdateSessionState_NonExistingSession(t *testing.T) {
	config := &Config{
		Sessions: []SessionConfig{
			{Name: "test", CLIType: "claude", WorkDir: "/tmp"},
		},
	}
	engine := NewEngine(config)

	// Try to update non-existing session (should not panic)
	engine.updateSessionState("nonexistent", StateProcessing)

	// No session should be created
	_, exists := engine.sessions["nonexistent"]
	assert.False(t, exists)
}
