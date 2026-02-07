package bot

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestDingTalkBot_SetMessageHandler tests the SetMessageHandler method
func TestDingTalkBot_SetMessageHandler(t *testing.T) {
	bot := &DingTalkBot{}

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

// TestDingTalkBot_GetMessageHandler tests the GetMessageHandler method
func TestDingTalkBot_GetMessageHandler(t *testing.T) {
	bot := &DingTalkBot{}

	// Test getting handler when none is set
	assert.Nil(t, bot.GetMessageHandler())

	// Test getting handler after setting one
	handler := func(msg BotMessage) {
		// Test handler
	}
	bot.SetMessageHandler(handler)
	assert.NotNil(t, bot.GetMessageHandler())
}

// TestDingTalkBot_Stop tests the Stop method
func TestDingTalkBot_Stop(t *testing.T) {
	t.Run("stop with nil cancel", func(t *testing.T) {
		bot := &DingTalkBot{}
		err := bot.Stop()
		assert.NoError(t, err)
	})

	t.Run("stop with nil streamClient", func(t *testing.T) {
		bot := &DingTalkBot{streamClient: nil}
		err := bot.Stop()
		assert.NoError(t, err)
	})
}

// TestDingTalkBot_NewDingTalkBot tests the NewDingTalkBot constructor
func TestDingTalkBot_NewDingTalkBot(t *testing.T) {
	t.Run("creates bot with credentials", func(t *testing.T) {
		clientID := "test-client-id"
		clientSecret := "test-client-secret"
		bot := NewDingTalkBot(clientID, clientSecret)

		assert.NotNil(t, bot)
		assert.Equal(t, clientID, bot.clientID)
		assert.Equal(t, clientSecret, bot.clientSecret)
	})

	t.Run("creates bot with empty credentials", func(t *testing.T) {
		bot := NewDingTalkBot("", "")
		assert.NotNil(t, bot)
		assert.Equal(t, "", bot.clientID)
		assert.Equal(t, "", bot.clientSecret)
	})
}
