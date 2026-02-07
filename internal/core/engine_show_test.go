package core

import (
	"testing"

	"github.com/keepmind9/clibot/internal/bot"
)

// TestEngine_ShowStatus tests showStatus method
func TestEngine_ShowStatus(t *testing.T) {
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
	}

	// Should not panic
	engine.showStatus(msg)
}

// TestEngine_ShowWhoami tests showWhoami method
func TestEngine_ShowWhoami(t *testing.T) {
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
	}

	// Should not panic
	engine.showWhoami(msg)
}

// TestEngine_ShowHelp tests showHelp method
func TestEngine_ShowHelp(t *testing.T) {
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
	}

	// Should not panic
	engine.showHelp(msg)
}

// TestEngine_HandleSpecialCommand tests HandleSpecialCommand method
func TestEngine_HandleSpecialCommand(t *testing.T) {
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
		Content:  "help",
	}

	// Should not panic
	engine.HandleSpecialCommand("help", msg)
}

// TestEngine_HandleSpecialCommand_UnknownCommand tests HandleSpecialCommand with unknown command
func TestEngine_HandleSpecialCommand_UnknownCommand(t *testing.T) {
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
		Content:  "unknown-command",
	}

	// Should not panic - unknown commands should be handled gracefully
	engine.HandleSpecialCommand("unknown-command", msg)
}

// TestEngine_ShowStatus_DifferentPlatforms tests showStatus with different platforms
func TestEngine_ShowStatus_DifferentPlatforms(t *testing.T) {
	config := &Config{
		Sessions: []SessionConfig{
			{Name: "test", CLIType: "claude", WorkDir: "/tmp"},
		},
	}
	engine := NewEngine(config)

	platforms := []string{"discord", "telegram", "feishu"}
	for _, platform := range platforms {
		t.Run(platform, func(t *testing.T) {
			msg := bot.BotMessage{
				Platform: platform,
				UserID:   "user123",
				Channel:  "channel456",
			}
			// Should not panic
			engine.showStatus(msg)
		})
	}
}
