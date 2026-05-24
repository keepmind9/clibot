package bot

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestDiscordBot_NewDiscordBot_TokenOnly tests creating bot with just token
func TestDiscordBot_NewDiscordBot_TokenOnly(t *testing.T) {
	bot := NewDiscordBot("test-token", "")

	assert.NotNil(t, bot)
	assert.Equal(t, "test-token", bot.token)
	assert.Equal(t, "", bot.channelID)
}

// TestTelegramBot_NewTelegramBot_EmptyToken tests creating bot with empty token
func TestTelegramBot_NewTelegramBot_EmptyToken(t *testing.T) {
	bot := NewTelegramBot("")

	assert.NotNil(t, bot)
	assert.Equal(t, "", bot.token)
}

// TestDingTalkBot_NewDingTalkBot_EmptyCredentials tests creating bot with empty credentials
func TestDingTalkBot_NewDingTalkBot_EmptyCredentials(t *testing.T) {
	bot := NewDingTalkBot("", "")

	assert.NotNil(t, bot)
	assert.Equal(t, "", bot.clientID)
	assert.Equal(t, "", bot.clientSecret)
}
