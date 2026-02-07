package core

import (
	"testing"

	"github.com/keepmind9/clibot/internal/bot"
	"github.com/stretchr/testify/assert"
)

// TestEngine_SendToBot_WithRegisteredBot tests SendToBot with registered bot
func TestEngine_SendToBot_WithRegisteredBot(t *testing.T) {
	config := &Config{
		Sessions: []SessionConfig{
			{Name: "test", CLIType: "claude", WorkDir: "/tmp"},
		},
	}
	engine := NewEngine(config)

	// Register a mock bot
	mockBot := &mockBotAdapter{}
	engine.RegisterBotAdapter("testbot", mockBot)

	// Send message to the bot
	engine.SendToBot("testbot", "test-channel", "test message")

	// Verify the message was sent
	assert.Equal(t, 1, mockBot.messageCount)
	assert.Equal(t, "test message", mockBot.lastMessage)
}

// TestEngine_SendToAllBots_WithRegisteredBots tests SendToAllBots with multiple bots
func TestEngine_SendToAllBots_WithRegisteredBots(t *testing.T) {
	config := &Config{
		Sessions: []SessionConfig{
			{Name: "test", CLIType: "claude", WorkDir: "/tmp"},
		},
	}
	engine := NewEngine(config)

	// Register multiple mock bots
	mockBot1 := &mockBotAdapter{}
	mockBot2 := &mockBotAdapter{}
	engine.RegisterBotAdapter("bot1", mockBot1)
	engine.RegisterBotAdapter("bot2", mockBot2)

	// Send message to all bots
	engine.SendToAllBots("broadcast message")

	// Verify both bots received the message
	assert.Equal(t, 1, mockBot1.messageCount)
	assert.Equal(t, 1, mockBot2.messageCount)
	assert.Equal(t, "broadcast message", mockBot1.lastMessage)
	assert.Equal(t, "broadcast message", mockBot2.lastMessage)
}

// TestEngine_SendToAllBots_NoBots tests SendToAllBots with no registered bots
func TestEngine_SendToAllBots_NoBots(t *testing.T) {
	config := &Config{
		Sessions: []SessionConfig{
			{Name: "test", CLIType: "claude", WorkDir: "/tmp"},
		},
	}
	engine := NewEngine(config)

	// Should not panic
	engine.SendToAllBots("test message")
}

// TestEngine_SendToBot_NonExistentBot tests SendToBot with non-existent bot
func TestEngine_SendToBot_NonExistentBot(t *testing.T) {
	config := &Config{
		Sessions: []SessionConfig{
			{Name: "test", CLIType: "claude", WorkDir: "/tmp"},
		},
	}
	engine := NewEngine(config)

	// Should not panic
	engine.SendToBot("nonexistent", "channel", "message")
}

// mockBotAdapter is a mock implementation of BotAdapter for testing
type mockBotAdapter struct {
	messageCount int
	lastMessage  string
	lastChannel  string
}

func (m *mockBotAdapter) Start(messageHandler func(bot.BotMessage)) error {
	return nil
}

func (m *mockBotAdapter) Stop() error {
	return nil
}

func (m *mockBotAdapter) SendMessage(channel, message string) error {
	m.messageCount++
	m.lastMessage = message
	m.lastChannel = channel
	return nil
}

func (m *mockBotAdapter) SetMessageHandler(handler func(bot.BotMessage)) {
	// Nothing to set
}

func (m *mockBotAdapter) GetMessageHandler() func(bot.BotMessage) {
	return func(msg bot.BotMessage) {}
}
