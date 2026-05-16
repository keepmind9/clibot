package core

import (
	"testing"

	"github.com/keepmind9/clibot/internal/bot"
	_ "github.com/keepmind9/clibot/internal/proxy"
	"github.com/stretchr/testify/assert"
)

// TestEngine_HandleCloseSession_NoArgs tests handleCloseSession with no arguments
func TestEngine_HandleCloseSession_NoArgs(t *testing.T) {
	config := &Config{
		Sessions: []SessionConfig{
			{Name: "test", CLIType: "claude", WorkDir: "/tmp"},
		},
	}
	engine := NewEngine(config)

	mockBot := &mockBotAdapter{}
	engine.RegisterBotAdapter("testbot", mockBot)

	msg := bot.BotMessage{
		Platform: "testbot",
		Channel:  "test-channel",
		UserID:   "user123",
	}

	engine.handleCloseSession([]string{}, msg)

	// Should send a message
	assert.Equal(t, 1, mockBot.messageCount)
}

func TestEngine_HandleCloseSession_NonExistingSession(t *testing.T) {
	engine := newTestEngine()
	engine.RegisterCLIAdapter("test-stdio", &mockCLIAdapter{})
	engine.initializeSessions()

	msg := bot.BotMessage{Platform: "testbot", Channel: "ch1", UserID: "user1"}
	engine.handleCloseSession([]string{"nonexistent"}, msg)

	mockBot := getMockBot(engine)
	assert.Equal(t, 1, mockBot.messageCount)
	assert.Contains(t, mockBot.lastMessage, "not found")
}

// TestEngine_HandleSessionStatus_NoArgs tests handleSessionStatus with no arguments
func TestEngine_HandleSessionStatus_NoArgs(t *testing.T) {
	config := &Config{
		Sessions: []SessionConfig{
			{Name: "test", CLIType: "claude", WorkDir: "/tmp"},
		},
	}
	engine := NewEngine(config)

	mockBot := &mockBotAdapter{}
	engine.RegisterBotAdapter("testbot", mockBot)

	msg := bot.BotMessage{
		Platform: "testbot",
		Channel:  "test-channel",
		UserID:   "user123",
	}

	engine.handleSessionStatus([]string{}, msg)

	// Should send a message (could be status or error)
	assert.Equal(t, 1, mockBot.messageCount)
}

// TestEngine_HandleSessionStatus_WithSession tests handleSessionStatus with session name
func TestEngine_HandleSessionStatus_WithSession(t *testing.T) {
	config := &Config{
		Sessions: []SessionConfig{
			{Name: "test", CLIType: "claude", WorkDir: "/tmp"},
		},
	}
	engine := NewEngine(config)

	mockBot := &mockBotAdapter{}
	engine.RegisterBotAdapter("testbot", mockBot)

	msg := bot.BotMessage{
		Platform: "testbot",
		Channel:  "test-channel",
		UserID:   "user123",
	}

	engine.handleSessionStatus([]string{"test"}, msg)

	// Should send status message
	assert.Equal(t, 1, mockBot.messageCount)
}

// TestEngine_SendToBot_NoRegisteredBot tests sending message with no registered bot
func TestEngine_SendToBot_NoRegisteredBot(t *testing.T) {
	config := &Config{
		Sessions: []SessionConfig{
			{Name: "test", CLIType: "claude", WorkDir: "/tmp"},
		},
	}
	engine := NewEngine(config)

	// Should not panic when no bot is registered
	engine.SendToBot("nonexistent", "channel", "message")
}

// TestEngine_SendToAllBots_NoRegisteredBots tests sending to all bots with no registered bots
func TestEngine_SendToAllBots_NoRegisteredBots(t *testing.T) {
	config := &Config{
		Sessions: []SessionConfig{
			{Name: "test", CLIType: "claude", WorkDir: "/tmp"},
		},
	}
	engine := NewEngine(config)

	// Should not panic when no bots are registered
	engine.SendToAllBots("message")
}

// TestEngine_HandleBotMessage_MessageChannel tests HandleBotMessage with message channel
func TestEngine_HandleBotMessage_MessageChannel(t *testing.T) {
	config := &Config{
		Sessions: []SessionConfig{
			{Name: "test", CLIType: "claude", WorkDir: "/tmp"},
		},
	}
	engine := NewEngine(config)

	mockBot := &mockBotAdapter{}
	engine.RegisterBotAdapter("testbot", mockBot)

	msg := bot.BotMessage{
		Platform: "testbot",
		Channel:  "test-channel",
		UserID:   "user123",
		Content:  "test message",
	}

	// Send message to the channel
	engine.HandleBotMessage(msg)

	// Message should be queued (we can't verify this without starting the engine)
	// But we can verify it doesn't crash
	assert.NotNil(t, engine.messageChan)
}

// TestEngine_CloseSessionThenList verifies that after sclose:
// 1. The session is removed from the session list
// 2. The user's current session is cleared
func TestEngine_CloseSessionThenList(t *testing.T) {
	config := &Config{
		Sessions: []SessionConfig{
			{Name: "session1", CLIType: "test-stdio", WorkDir: "/tmp", AutoStart: false},
			{Name: "session2", CLIType: "test-stdio", WorkDir: "/tmp", AutoStart: false},
		},
	}
	engine := NewEngine(config)
	engine.RegisterCLIAdapter("test-stdio", &mockCLIAdapter{})

	mockBot := &mockBotAdapter{}
	engine.RegisterBotAdapter("testbot", mockBot)

	// Initialize sessions (normally done by Serve)
	engine.initializeSessions()

	msg := bot.BotMessage{
		Platform: "testbot",
		Channel:  "ch1",
		UserID:   "user1",
	}

	// Step 1: User selects session1
	engine.handleUseSession([]string{"session1"}, msg)
	assert.GreaterOrEqual(t, mockBot.messageCount, 1)

	// Step 2: Verify slist shows session1 as current
	mockBot.messageCount = 0
	mockBot.lastMessage = ""
	engine.listSessions(msg)
	assert.Contains(t, mockBot.lastMessage, "session1")
	assert.Contains(t, mockBot.lastMessage, "session2")
	assert.Contains(t, mockBot.lastMessage, "CURRENT")

	// Step 3: Close session1 (with arg)
	mockBot.messageCount = 0
	mockBot.lastMessage = ""
	engine.handleCloseSession([]string{"session1"}, msg)
	assert.Contains(t, mockBot.lastMessage, "closed")

	// Step 4: Verify session is gone from internal maps
	engine.sessionMu.RLock()
	_, exists := engine.sessions["session1"]
	userSession := engine.userSessions[getUserKey(msg.Platform, msg.UserID)]
	engine.sessionMu.RUnlock()
	assert.False(t, exists, "session1 should be removed from sessions map")
	assert.Empty(t, userSession, "user's current session should be cleared")

	// Step 5: Verify slist no longer shows session1 or current session
	mockBot.messageCount = 0
	mockBot.lastMessage = ""
	engine.listSessions(msg)
	assert.NotContains(t, mockBot.lastMessage, "session1", "slist should not show closed session1")
	assert.Contains(t, mockBot.lastMessage, "session2", "slist should still show session2")
	assert.NotContains(t, mockBot.lastMessage, "CURRENT", "no session should be marked as current")
}
