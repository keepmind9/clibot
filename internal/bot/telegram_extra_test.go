package bot

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestTelegramBot_SetMessageHandler tests the SetMessageHandler method
func TestTelegramBot_SetMessageHandler(t *testing.T) {
	bot := &TelegramBot{}

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

// TestTelegramBot_GetMessageHandler tests the GetMessageHandler method
func TestTelegramBot_GetMessageHandler(t *testing.T) {
	bot := &TelegramBot{}

	// Test getting handler when none is set
	assert.Nil(t, bot.GetMessageHandler())

	// Test getting handler after setting one
	handler := func(msg BotMessage) {
		// Test handler
	}
	bot.SetMessageHandler(handler)
	assert.NotNil(t, bot.GetMessageHandler())
}

// TestTelegramBot_Stop tests the Stop method
func TestTelegramBot_Stop(t *testing.T) {
	t.Run("stop with nil cancel", func(t *testing.T) {
		bot := &TelegramBot{}
		err := bot.Stop()
		assert.NoError(t, err)
	})

	t.Run("stop with nil bot", func(t *testing.T) {
		bot := &TelegramBot{bot: nil}
		err := bot.Stop()
		assert.NoError(t, err)
	})
}

// TestTelegramBot_NewTelegramBot tests the NewTelegramBot constructor
func TestTelegramBot_NewTelegramBot(t *testing.T) {
	t.Run("creates bot with token", func(t *testing.T) {
		token := "test-token-123"
		bot := NewTelegramBot(token)

		assert.NotNil(t, bot)
		assert.Equal(t, token, bot.token)
	})

	t.Run("creates bot with empty token", func(t *testing.T) {
		bot := NewTelegramBot("")
		assert.NotNil(t, bot)
		assert.Equal(t, "", bot.token)
	})
}
