package core

import (
	"testing"

	"github.com/keepmind9/clibot/internal/bot"
	"github.com/stretchr/testify/assert"
)

// TestTruncateString tests the truncateString function
func TestTruncateString(t *testing.T) {
	t.Run("string shorter than max", func(t *testing.T) {
		result := truncateString("hello", 10)
		assert.Equal(t, "hello", result)
	})

	t.Run("string equal to max", func(t *testing.T) {
		result := truncateString("hello", 5)
		assert.Equal(t, "hello", result)
	})

	t.Run("string longer than max", func(t *testing.T) {
		result := truncateString("hello world", 5)
		assert.Equal(t, "hello", result)
	})

	t.Run("empty string", func(t *testing.T) {
		result := truncateString("", 10)
		assert.Equal(t, "", result)
	})

	t.Run("max length zero", func(t *testing.T) {
		result := truncateString("hello", 0)
		assert.Equal(t, "", result)
	})

	t.Run("unicode characters", func(t *testing.T) {
		result := truncateString("你好世界", 6)
		// Note: this truncates by bytes, not runes
		// "你好" = 6 bytes (3 bytes per character)
		assert.Equal(t, "你好", result)
	})

	t.Run("unicode characters truncated mid-character", func(t *testing.T) {
		result := truncateString("你好", 2)
		// Truncates to 2 bytes, which is incomplete for "你" (3 bytes)
		// Result is invalid UTF-8 but that's expected behavior
		assert.Equal(t, 2, len(result))
	})
}

// TestEngine_RegisterCLIAdapter tests RegisterCLIAdapter method
func TestEngine_RegisterCLIAdapter(t *testing.T) {
	config := &Config{
		Sessions: []SessionConfig{
			{Name: "test", CLIType: "claude", WorkDir: "/tmp"},
		},
	}
	engine := NewEngine(config)

	// Register a mock CLI adapter - using nil since we just need to test the registration logic
	engine.RegisterCLIAdapter("test-cli", nil)

	// Verify the adapter was registered (value is nil but key exists)
	_, exists := engine.cliAdapters["test-cli"]
	assert.True(t, exists)
}

// TestEngine_RegisterCLIAdapter_MultipleAdapters tests registering multiple CLI adapters
func TestEngine_RegisterCLIAdapter_MultipleAdapters(t *testing.T) {
	config := &Config{
		Sessions: []SessionConfig{
			{Name: "test", CLIType: "claude", WorkDir: "/tmp"},
		},
	}
	engine := NewEngine(config)

	// Register multiple adapters
	adapters := []string{"cli1", "cli2", "cli3"}
	for _, adapterType := range adapters {
		engine.RegisterCLIAdapter(adapterType, nil)
	}

	// Verify all adapters were registered
	for _, adapterType := range adapters {
		_, exists := engine.cliAdapters[adapterType]
		assert.True(t, exists)
	}
}

// TestEngine_RegisterCLIAdapter_Overwrite tests overwriting an existing adapter
func TestEngine_RegisterCLIAdapter_Overwrite(t *testing.T) {
	config := &Config{
		Sessions: []SessionConfig{
			{Name: "test", CLIType: "claude", WorkDir: "/tmp"},
		},
	}
	engine := NewEngine(config)

	// Register first adapter
	engine.RegisterCLIAdapter("test-cli", nil)

	// Overwrite with new adapter
	engine.RegisterCLIAdapter("test-cli", nil)

	// Verify the key still exists
	_, exists := engine.cliAdapters["test-cli"]
	assert.True(t, exists)
}

// TestEngine_HandleBotMessage tests HandleBotMessage method
func TestEngine_HandleBotMessage(t *testing.T) {
	config := &Config{
		Sessions: []SessionConfig{
			{Name: "test", CLIType: "claude", WorkDir: "/tmp"},
		},
	}
	engine := NewEngine(config)

	msg := bot.BotMessage{
		Platform: "discord",
		UserID:   "user123",
		Channel:  "channel456",
		Content:  "test message",
	}

	// Should not panic
	engine.HandleBotMessage(msg)
}

// TestEngine_HandleBotMessage_NoSessions tests HandleBotMessage with no sessions
func TestEngine_HandleBotMessage_NoSessions(t *testing.T) {
	config := &Config{
		Sessions: []SessionConfig{},
	}
	engine := NewEngine(config)

	msg := bot.BotMessage{
		Platform: "discord",
		UserID:   "user123",
		Channel:  "channel456",
		Content:  "test message",
	}

	// Should not panic
	engine.HandleBotMessage(msg)
}

// TestEngine_HandleSpecialCommandWithArgs_UnknownCommand tests HandleSpecialCommandWithArgs with unknown command
func TestEngine_HandleSpecialCommandWithArgs_UnknownCommand(t *testing.T) {
	config := &Config{
		Sessions: []SessionConfig{
			{Name: "test", CLIType: "claude", WorkDir: "/tmp"},
		},
	}
	engine := NewEngine(config)

	msg := bot.BotMessage{
		Platform: "discord",
		UserID:   "user123",
		Channel:  "channel456",
		Content:  "unknown command",
	}

	// Should not panic - unknown commands should be handled gracefully
	engine.HandleSpecialCommandWithArgs("unknown-command", []string{"arg1"}, msg)
}

// TestEngine_HandleSpecialCommandWithArgs_EmptyArgs tests HandleSpecialCommandWithArgs with empty args
func TestEngine_HandleSpecialCommandWithArgs_EmptyArgs(t *testing.T) {
	config := &Config{
		Sessions: []SessionConfig{
			{Name: "test", CLIType: "claude", WorkDir: "/tmp"},
		},
	}
	engine := NewEngine(config)

	msg := bot.BotMessage{
		Platform: "discord",
		UserID:   "user123",
		Channel:  "channel456",
		Content:  "status",
	}

	// Should not panic
	engine.HandleSpecialCommandWithArgs("status", []string{}, msg)
}

// TestEngine_ShowWhoami_DifferentPlatforms tests showWhoami with different platforms
func TestEngine_ShowWhoami_DifferentPlatforms(t *testing.T) {
	config := &Config{
		Sessions: []SessionConfig{
			{Name: "test", CLIType: "claude", WorkDir: "/tmp"},
		},
	}
	engine := NewEngine(config)

	platforms := []struct {
		name     string
		platform string
		userID   string
	}{
		{"discord", "discord", "discord-user-123"},
		{"telegram", "telegram", "telegram-user-456"},
		{"feishu", "feishu", "feishu-user-789"},
	}

	for _, tt := range platforms {
		t.Run(tt.name, func(t *testing.T) {
			msg := bot.BotMessage{
				Platform: tt.platform,
				UserID:   tt.userID,
				Channel:  "channel456",
			}
			// Should not panic
			engine.showWhoami(msg)
		})
	}
}

// TestEngine_ListSessions_EmptySessions tests listSessions with empty sessions
func TestEngine_ListSessions_EmptySessions(t *testing.T) {
	config := &Config{
		Sessions: []SessionConfig{},
	}
	engine := NewEngine(config)

	msg := bot.BotMessage{
		Platform: "discord",
		UserID:   "user123",
		Channel:  "channel456",
	}

	// Should not panic
	engine.listSessions(msg)
}

// TestEngine_ListSessions_WithSessions tests listSessions with configured sessions
func TestEngine_ListSessions_WithSessions(t *testing.T) {
	config := &Config{
		Sessions: []SessionConfig{
			{Name: "session1", CLIType: "claude", WorkDir: "/tmp1"},
			{Name: "session2", CLIType: "gemini", WorkDir: "/tmp2"},
		},
	}
	engine := NewEngine(config)

	// Create sessions
	for _, cfg := range config.Sessions {
		engine.sessions[cfg.Name] = &Session{
			Name:    cfg.Name,
			CLIType: cfg.CLIType,
			State:   StateIdle,
		}
	}

	msg := bot.BotMessage{
		Platform: "discord",
		UserID:   "user123",
		Channel:  "channel456",
	}

	// Should not panic
	engine.listSessions(msg)
}
