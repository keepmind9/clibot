package core

import (
	"testing"

	"github.com/keepmind9/clibot/internal/bot"
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

	// GetActiveSession should return the first session
	session := engine.GetActiveSession("test-channel")
	assert.NotNil(t, session)
	assert.Equal(t, "session1", session.Name)
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

// TestEngine_ShowStatus_EmptySessionsConfig tests showStatus with no sessions
func TestEngine_ShowStatus_EmptySessionsConfig(t *testing.T) {
	config := &Config{
		Sessions: []SessionConfig{},
	}
	engine := NewEngine(config)

	msg := bot.BotMessage{
		Platform: "discord",
		UserID:   "user123",
		Channel:  "channel456",
	}

	// Should not panic even with no sessions
	engine.showStatus(msg)
}

// TestEngine_HandleSpecialCommand_AllCommands tests HandleSpecialCommand with all special commands
func TestEngine_HandleSpecialCommand_AllCommands(t *testing.T) {
	config := &Config{
		Sessions: []SessionConfig{
			{Name: "test", CLIType: "claude", WorkDir: "/tmp"},
		},
	}
	engine := NewEngine(config)

	msg := bot.BotMessage{
		Platform: "discord",
		UserID:   "user123",
		Channel:  "channel456",
	}

	// Test all special commands
	commands := []string{"slist", "status", "whoami", "help", "echo"}
	for _, cmd := range commands {
		t.Run(cmd, func(t *testing.T) {
			// Should not panic
			engine.HandleSpecialCommand(cmd, msg)
		})
	}
}
