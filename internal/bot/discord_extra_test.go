package bot

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestDiscordBot_SetMessageHandler tests the SetMessageHandler method
func TestDiscordBot_SetMessageHandler(t *testing.T) {
	bot := &DiscordBot{}

	// Verify initial handler is nil
	assert.Nil(t, bot.GetMessageHandler())

	// Test setting message handler
	called := false
	handler := func(msg BotMessage) {
		called = true
	}
	bot.SetMessageHandler(handler)

	// Verify handler was set
	retrievedHandler := bot.GetMessageHandler()
	assert.NotNil(t, retrievedHandler)

	// Test that the handler works
	retrievedHandler(BotMessage{})
	assert.True(t, called, "handler should be called")

	// Test updating handler
	newCalled := false
	newHandler := func(msg BotMessage) {
		newCalled = true
	}
	bot.SetMessageHandler(newHandler)
	bot.GetMessageHandler()(BotMessage{})
	assert.True(t, newCalled, "new handler should be called")
}

// TestDiscordBot_GetMessageHandler tests the GetMessageHandler method
func TestDiscordBot_GetMessageHandler(t *testing.T) {
	bot := &DiscordBot{}

	// Test getting handler when none is set
	assert.Nil(t, bot.GetMessageHandler())

	// Test getting handler after setting one
	handler := func(msg BotMessage) {
		// Test handler
	}
	bot.SetMessageHandler(handler)
	assert.NotNil(t, bot.GetMessageHandler())
}

// TestDiscordBot_Stop tests the Stop method
func TestDiscordBot_Stop(t *testing.T) {
	t.Run("stop with nil session", func(t *testing.T) {
		bot := &DiscordBot{}
		err := bot.Stop()
		assert.NoError(t, err)
	})

	t.Run("stop with session set to nil", func(t *testing.T) {
		bot := &DiscordBot{session: nil}
		err := bot.Stop()
		assert.NoError(t, err)
		assert.Nil(t, bot.session)
	})
}

// TestDiscordBot_NewDiscordBot tests the NewDiscordBot constructor
func TestDiscordBot_NewDiscordBot(t *testing.T) {
	t.Run("creates bot with token and channel", func(t *testing.T) {
		token := "test-token-123"
		channelID := "test-channel-456"
		bot := NewDiscordBot(token, channelID)

		assert.NotNil(t, bot)
		assert.Equal(t, token, bot.token)
		assert.Equal(t, channelID, bot.channelID)
	})

	t.Run("creates bot with empty token and channel", func(t *testing.T) {
		bot := NewDiscordBot("", "")
		assert.NotNil(t, bot)
		assert.Equal(t, "", bot.token)
		assert.Equal(t, "", bot.channelID)
	})
}
