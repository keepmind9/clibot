package core

import (
	"testing"

	"github.com/keepmind9/clibot/internal/bot"
	"github.com/stretchr/testify/assert"
)

// TestEngine_RegisterBotAdapter tests registering bot adapters
func TestEngine_RegisterBotAdapter(t *testing.T) {
	config := &Config{
		Sessions: []SessionConfig{
			{Name: "test", CLIType: "claude", WorkDir: "/tmp"},
		},
	}
	engine := NewEngine(config)

	// Create a mock bot adapter
	mockBot := bot.NewDiscordBot("test-token", "test-channel")

	// Register the bot
	engine.RegisterBotAdapter("discord", mockBot)

	// Verify it was registered
	assert.NotNil(t, engine.activeBots["discord"])
}

// TestEngine_RegisterMultipleBots tests registering multiple bot adapters
func TestEngine_RegisterMultipleBots(t *testing.T) {
	config := &Config{
		Sessions: []SessionConfig{
			{Name: "test", CLIType: "claude", WorkDir: "/tmp"},
		},
	}
	engine := NewEngine(config)

	// Register multiple bot adapters
	engine.RegisterBotAdapter("discord", bot.NewDiscordBot("token1", "channel1"))
	engine.RegisterBotAdapter("telegram", bot.NewTelegramBot("token2"))
	engine.RegisterBotAdapter("feishu", bot.NewFeishuBot("app-id", "app-secret"))

	// Verify all were registered
	assert.NotNil(t, engine.activeBots["discord"])
	assert.NotNil(t, engine.activeBots["telegram"])
	assert.NotNil(t, engine.activeBots["feishu"])
}

// TestEngine_OverwriteBotAdapter tests overwriting existing bot adapter
func TestEngine_OverwriteBotAdapter(t *testing.T) {
	config := &Config{
		Sessions: []SessionConfig{
			{Name: "test", CLIType: "claude", WorkDir: "/tmp"},
		},
	}
	engine := NewEngine(config)

	// Register initial bot
	bot1 := bot.NewDiscordBot("token1", "channel1")
	engine.RegisterBotAdapter("discord", bot1)

	// Overwrite with new bot
	bot2 := bot.NewDiscordBot("token2", "channel2")
	engine.RegisterBotAdapter("discord", bot2)

	// Verify the bot was replaced
	assert.Equal(t, bot2, engine.activeBots["discord"])
}
