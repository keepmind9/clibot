package core

import (
	"testing"

	_ "github.com/keepmind9/clibot/internal/proxy"
	"github.com/stretchr/testify/assert"
)

// TestEngine_NewEngine_MinimalConfig tests NewEngine with minimal config
func TestEngine_NewEngine_MinimalConfig(t *testing.T) {
	config := &Config{}
	engine := NewEngine(config)

	assert.NotNil(t, engine)
	assert.NotNil(t, engine.sessions)
	assert.NotNil(t, engine.cliAdapters)
	assert.NotNil(t, engine.activeBots)
	assert.NotNil(t, engine.config)
}

// TestEngine_UpdateSessionState_MultipleUpdates tests updateSessionState with multiple updates
func TestEngine_UpdateSessionState_MultipleUpdates(t *testing.T) {
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

	// Update state multiple times
	states := []SessionState{StateProcessing, StateWaitingInput, StateIdle, StateProcessing}
	for _, state := range states {
		engine.updateSessionState("test", state)
		assert.Equal(t, state, engine.sessions["test"].State)
	}
}

// TestEngine_GetActiveSession_MultipleSessions tests GetActiveSession with multiple sessions
func TestEngine_GetActiveSession_MultipleSessions(t *testing.T) {
	config := &Config{
		Sessions: []SessionConfig{
			{Name: "session1", CLIType: "claude", WorkDir: "/tmp"},
			{Name: "session2", CLIType: "gemini", WorkDir: "/tmp"},
			{Name: "session3", CLIType: "opencode", WorkDir: "/tmp"},
		},
	}
	engine := NewEngine(config)

	// Create sessions
	for _, cfg := range config.Sessions {
		engine.sessions[cfg.Name] = &Session{
			Name:    cfg.Name,
			CLIType: cfg.CLIType,
			State:   StateIdle,
		}
	}

	// GetActiveSession should return one of the sessions
	// Note: map iteration order is non-deterministic in Go
	session := engine.GetActiveSession("test-channel")
	assert.NotNil(t, session)
	assert.Contains(t, []string{"session1", "session2", "session3"}, session.Name)
}

// TestEngine_SendToBot_MultipleMessages tests sending multiple messages
func TestEngine_SendToBot_MultipleMessages(t *testing.T) {
	config := &Config{
		Sessions: []SessionConfig{
			{Name: "test", CLIType: "claude", WorkDir: "/tmp"},
		},
	}
	engine := NewEngine(config)

	// Register a mock bot
	mockBot := &mockBotAdapter{}
	engine.RegisterBotAdapter("testbot", mockBot)

	// Send multiple messages
	messages := []string{"message 1", "message 2", "message 3"}
	for _, msg := range messages {
		engine.SendToBot("testbot", "test-channel", msg)
	}

	// Verify all messages were sent
	assert.Equal(t, 3, mockBot.messageCount)
}

// TestEngine_SendToAllBots_WithRealBots tests SendToAllBots with multiple registered bots
func TestEngine_SendToAllBots_WithRealBots(t *testing.T) {
	config := &Config{
		Sessions: []SessionConfig{
			{Name: "test", CLIType: "claude", WorkDir: "/tmp"},
		},
	}
	engine := NewEngine(config)

	// Register multiple mock bots
	mockBot1 := &mockBotAdapter{}
	mockBot2 := &mockBotAdapter{}
	mockBot3 := &mockBotAdapter{}
	engine.RegisterBotAdapter("bot1", mockBot1)
	engine.RegisterBotAdapter("bot2", mockBot2)
	engine.RegisterBotAdapter("bot3", mockBot3)

	// Send multiple messages
	engine.SendToAllBots("message 1")
	engine.SendToAllBots("message 2")

	// Verify all bots received both messages
	assert.Equal(t, 2, mockBot1.messageCount)
	assert.Equal(t, 2, mockBot2.messageCount)
	assert.Equal(t, 2, mockBot3.messageCount)
}

// TestEngine_NeedsHookServer_AllACPSessions_ReturnsFalse tests needsHookServer with all ACP sessions
func TestEngine_NeedsHookServer_AllACPSessions_ReturnsFalse(t *testing.T) {
	config := &Config{
		Sessions: []SessionConfig{
			{Name: "acp-session-1", CLIType: "acp", WorkDir: "/tmp/test1"},
			{Name: "acp-session-2", CLIType: "acp", WorkDir: "/tmp/test2"},
		},
	}
	engine := NewEngine(config)

	// All sessions are ACP, should not need hook server
	assert.False(t, engine.needsHookServer())
}

// TestEngine_NeedsHookServer_MixedSessionTypes_ReturnsTrue tests needsHookServer with mixed session types
func TestEngine_NeedsHookServer_MixedSessionTypes_ReturnsTrue(t *testing.T) {
	config := &Config{
		Sessions: []SessionConfig{
			{Name: "acp-session", CLIType: "acp", WorkDir: "/tmp/test1"},
			{Name: "claude-session", CLIType: "claude", WorkDir: "/tmp/test2"},
		},
	}
	engine := NewEngine(config)

	// Has non-ACP session, should need hook server
	assert.True(t, engine.needsHookServer())
}

// TestEngine_NeedsHookServer_NonACPSessions_ReturnsTrue tests needsHookServer with non-ACP sessions
func TestEngine_NeedsHookServer_NonACPSessions_ReturnsTrue(t *testing.T) {
	config := &Config{
		Sessions: []SessionConfig{
			{Name: "claude-session", CLIType: "claude", WorkDir: "/tmp/test1"},
			{Name: "gemini-session", CLIType: "gemini", WorkDir: "/tmp/test2"},
		},
	}
	engine := NewEngine(config)

	// All non-ACP sessions, should need hook server
	assert.True(t, engine.needsHookServer())
}

// TestEngine_NeedsHookServer_NoSessions_ReturnsFalse tests needsHookServer with no sessions
func TestEngine_NeedsHookServer_NoSessions_ReturnsFalse(t *testing.T) {
	config := &Config{
		Sessions: []SessionConfig{},
	}
	engine := NewEngine(config)

	// No sessions, should not need hook server
	assert.False(t, engine.needsHookServer())
}
