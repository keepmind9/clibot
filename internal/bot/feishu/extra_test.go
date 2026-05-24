package feishu

import (
	"testing"

	"github.com/keepmind9/clibot/internal/bot"
	"github.com/stretchr/testify/assert"
)

func TestBot_SupportsTypingIndicator(t *testing.T) {
	b := NewBot("test-app-id", "test-app-secret")

	assert.True(t, b.SupportsTypingIndicator(), "Feishu should support typing indicators")
}

func TestBot_AddTypingIndicator_NoClient(t *testing.T) {
	b := &Bot{}

	success := b.AddTypingIndicator("test-message-id")
	assert.False(t, success, "Should return false when client is nil")
}

func TestBot_AddTypingIndicator_AlreadyExists(t *testing.T) {
	b := NewBot("test-app-id", "test-app-secret")

	messageID := "test-message-id"

	b.mu.Lock()
	b.typingReactions[messageID] = "test-reaction-id"
	b.mu.Unlock()

	success := b.AddTypingIndicator(messageID)
	assert.True(t, success, "Should return true when reaction already exists")
}

func TestBot_RemoveTypingIndicator_NoReaction(t *testing.T) {
	b := NewBot("test-app-id", "test-app-secret")

	err := b.RemoveTypingIndicator("test-message-id")
	assert.NoError(t, err, "Should return nil when no reaction exists")
}

func TestBot_RemoveTypingIndicator_NoClient(t *testing.T) {
	b := &Bot{}

	b.typingReactions = map[string]string{
		"test-message-id": "test-reaction-id",
	}

	err := b.RemoveTypingIndicator("test-message-id")
	assert.Error(t, err, "Should return error when client is nil")
	assert.Contains(t, err.Error(), "not initialized", "Error should mention not initialized")
}

func TestBot_RemoveTypingIndicator_DeleteFromMap(t *testing.T) {
	b := NewBot("test-app-id", "test-app-secret")

	messageID := "test-message-id"

	b.mu.Lock()
	b.typingReactions[messageID] = "test-reaction-id"
	b.mu.Unlock()

	_ = b.RemoveTypingIndicator(messageID)

	b.mu.RLock()
	_, exists := b.typingReactions[messageID]
	b.mu.RUnlock()

	assert.False(t, exists, "Reaction should be deleted from map")
}

func TestBot_SetEncryptKey(t *testing.T) {
	b := &Bot{}

	b.SetEncryptKey("test-encrypt-key")
	assert.Equal(t, "test-encrypt-key", b.encryptKey)

	b.SetEncryptKey("new-encrypt-key")
	assert.Equal(t, "new-encrypt-key", b.encryptKey)

	b.SetEncryptKey("")
	assert.Equal(t, "", b.encryptKey)
}

func TestBot_SetVerificationToken(t *testing.T) {
	b := &Bot{}

	b.SetVerificationToken("test-token")
	assert.Equal(t, "test-token", b.verificationToken)

	b.SetVerificationToken("new-token")
	assert.Equal(t, "new-token", b.verificationToken)

	b.SetVerificationToken("")
	assert.Equal(t, "", b.verificationToken)
}

func TestBot_SetMessageHandler(t *testing.T) {
	b := &Bot{}

	assert.Nil(t, b.GetMessageHandler())

	called := false
	handler := func(msg bot.BotMessage) {
		called = true
	}
	b.SetMessageHandler(handler)

	retrievedHandler := b.GetMessageHandler()
	assert.NotNil(t, retrievedHandler)

	retrievedHandler(bot.BotMessage{})
	assert.True(t, called, "handler should be called")

	newCalled := false
	newHandler := func(msg bot.BotMessage) {
		newCalled = true
	}
	b.SetMessageHandler(newHandler)
	b.GetMessageHandler()(bot.BotMessage{})
	assert.True(t, newCalled, "new handler should be called")
}

func TestBot_GetMessageHandler(t *testing.T) {
	b := &Bot{}

	assert.Nil(t, b.GetMessageHandler())

	handler := func(msg bot.BotMessage) {}
	b.SetMessageHandler(handler)
	assert.NotNil(t, b.GetMessageHandler())
}

func TestBot_NewBot_EmptyCredentials(t *testing.T) {
	b := NewBot("", "")

	assert.NotNil(t, b)
	assert.Equal(t, "", b.appID)
	assert.Equal(t, "", b.appSecret)
}

func TestBot_SendMessage_NoClient(t *testing.T) {
	b := NewBot("test-app-id", "test-app-secret")

	err := b.SendMessage("", "test message")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestBot_Stop_NilClient(t *testing.T) {
	b := NewBot("test-app-id", "test-app-secret")

	b.Stop()
}
