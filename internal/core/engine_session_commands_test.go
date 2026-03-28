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

// TestEngine_HandleCloseSession_NonExistingSession tests handleCloseSession with non-existing session
func TestEngine_HandleCloseSession_NonExistingSession(t *testing.T) {
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

	engine.handleCloseSession([]string{"nonexistent"}, msg)

	// Should send error message
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
