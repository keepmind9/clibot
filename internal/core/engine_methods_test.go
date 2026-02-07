package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestEngine_GetActiveSession_NoSessions tests GetActiveSession with no sessions
func TestEngine_GetActiveSession_NoSessions(t *testing.T) {
	config := &Config{
		Sessions: []SessionConfig{},
	}
	engine := NewEngine(config)

	session := engine.GetActiveSession("test-channel")
	assert.Nil(t, session)
}

// TestEngine_GetActiveSession_WithSessions tests GetActiveSession with configured sessions
func TestEngine_GetActiveSession_WithSessions(t *testing.T) {
	config := &Config{
		Sessions: []SessionConfig{
			{Name: "session1", CLIType: "claude", WorkDir: "/tmp"},
		},
	}
	engine := NewEngine(config)

	// GetActiveSession should not panic
	_ = engine.GetActiveSession("test-channel")
	// Verify the sessions map exists
	assert.NotNil(t, engine.sessions)
}

// TestEngine_UpdateSessionState_NonExistentSession tests updateSessionState with non-existent session
func TestEngine_UpdateSessionState_NonExistentSession(t *testing.T) {
	config := &Config{
		Sessions: []SessionConfig{
			{Name: "test", CLIType: "claude", WorkDir: "/tmp"},
		},
	}
	engine := NewEngine(config)

	// Should not panic when updating non-existent session
	engine.updateSessionState("nonexistent", StateIdle)

	// Verify sessions map still exists
	assert.NotNil(t, engine.sessions)
}

// TestEngine_UpdateSessionState_ValidSession tests updateSessionState with valid session
func TestEngine_UpdateSessionState_ValidSession(t *testing.T) {
	config := &Config{
		Sessions: []SessionConfig{
			{Name: "test", CLIType: "claude", WorkDir: "/tmp"},
		},
	}
	engine := NewEngine(config)

	// Create a session manually for testing
	testSession := &Session{
		Name:  "test",
		State: StateIdle,
	}
	engine.sessions["test"] = testSession

	// Update session state
	engine.updateSessionState("test", StateProcessing)

	// Verify state was updated
	assert.Equal(t, StateProcessing, testSession.State)
}

// TestEngine_SendToBot_NoActiveBots tests SendToBot with no active bots
func TestEngine_SendToBot_NoActiveBots(t *testing.T) {
	config := &Config{
		Sessions: []SessionConfig{
			{Name: "test", CLIType: "claude", WorkDir: "/tmp"},
		},
	}
	engine := NewEngine(config)

	// Should not panic when there are no active bots
	engine.SendToBot("discord", "channel", "test message")
}

// TestEngine_SendToAllBots_NoActiveBots tests SendToAllBots with no active bots
func TestEngine_SendToAllBots_NoActiveBots(t *testing.T) {
	config := &Config{
		Sessions: []SessionConfig{
			{Name: "test", CLIType: "claude", WorkDir: "/tmp"},
		},
	}
	engine := NewEngine(config)

	// Should not panic when there are no active bots
	engine.SendToAllBots("test message")
}
