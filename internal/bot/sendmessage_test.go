package bot

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestDiscordBot_SendMessage_NoSession tests SendMessage when session is not initialized
func TestDiscordBot_SendMessage_NoSession(t *testing.T) {
	bot := NewDiscordBot("test-token", "test-channel")
	// Don't call Start, so session remains nil

	err := bot.SendMessage("", "test message")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

// TestDiscordBot_SendMessage_WithChannel tests SendMessage with specified channel
func TestDiscordBot_SendMessage_WithChannel(t *testing.T) {
	bot := NewDiscordBot("test-token", "default-channel")

	// Since session is nil, should still error
	err := bot.SendMessage("custom-channel", "test message")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

// TestTelegramBot_SendMessage_NoSession tests SendMessage when session is not initialized
func TestTelegramBot_SendMessage_NoSession(t *testing.T) {
	bot := NewTelegramBot("test-token")
	// Don't call Start, so session remains nil

	err := bot.SendMessage("", "test message")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

// TestFeishuBot_SendMessage_NoSession tests SendMessage when session is not initialized
func TestFeishuBot_SendMessage_NoSession(t *testing.T) {
	bot := NewFeishuBot("test-app-id", "test-app-secret")
	// Don't call Start, so session remains nil

	err := bot.SendMessage("", "test message")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "chat ID is required")
}

// TestDingTalkBot_SendMessage_NoSession tests SendMessage when session is not initialized
func TestDingTalkBot_SendMessage_NoSession(t *testing.T) {
	bot := NewDingTalkBot("test-client-id", "test-client-secret")
	// Don't call Start, so client remains nil

	err := bot.SendMessage("", "test message")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "conversation ID is required")
}

// TestDiscordBot_Stop_NilSession tests Stop when session is nil
func TestDiscordBot_Stop_NilSession(t *testing.T) {
	bot := NewDiscordBot("test-token", "test-channel")
	// Don't call Start, session is nil

	// Should not panic
	bot.Stop()
}

// TestTelegramBot_Stop_NilSession tests Stop when session is nil
func TestTelegramBot_Stop_NilSession(t *testing.T) {
	bot := NewTelegramBot("test-token")
	// Don't call Start, session is nil

	// Should not panic
	bot.Stop()
}

// TestFeishuBot_Stop_NilSession tests Stop when session is nil
func TestFeishuBot_Stop_NilSession(t *testing.T) {
	bot := NewFeishuBot("test-app-id", "test-app-secret")
	// Don't call Start, session is nil

	// Should not panic
	bot.Stop()
}

// TestDingTalkBot_Stop_NilClient tests Stop when client is nil
func TestDingTalkBot_Stop_NilClient(t *testing.T) {
	bot := NewDingTalkBot("test-client-id", "test-client-secret")
	// Don't call Start, client is nil

	// Should not panic
	bot.Stop()
}
