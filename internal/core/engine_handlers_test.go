package core

import (
	"testing"
	"time"

	"github.com/keepmind9/clibot/internal/bot"
)

// TestEngine_HandleEcho tests handleEcho method
func TestEngine_HandleEcho(t *testing.T) {
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
	engine.handleEcho(msg)
}

// TestEngine_HandleEcho_AllFields tests handleEcho with all message fields
func TestEngine_HandleEcho_AllFields(t *testing.T) {
	config := &Config{
		Sessions: []SessionConfig{
			{Name: "test", CLIType: "claude", WorkDir: "/tmp"},
		},
	}
	engine := NewEngine(config)

	msg := bot.BotMessage{
		Platform:  "telegram",
		UserID:    "user789",
		Channel:   "channel123",
		Content:   "test content",
		Timestamp: time.Now(),
	}

	// Should not panic
	engine.handleEcho(msg)
}

// TestEngine_HandleEcho_DifferentPlatforms tests handleEcho with different platforms
func TestEngine_HandleEcho_DifferentPlatforms(t *testing.T) {
	config := &Config{
		Sessions: []SessionConfig{
			{Name: "test", CLIType: "claude", WorkDir: "/tmp"},
		},
	}
	engine := NewEngine(config)

	platforms := []string{"discord", "telegram", "feishu", "dingtalk"}
	for _, platform := range platforms {
		t.Run(platform, func(t *testing.T) {
			msg := bot.BotMessage{
				Platform:  platform,
				UserID:    "test-user",
				Channel:   "test-channel",
				Timestamp: time.Now(),
			}
			// Should not panic
			engine.handleEcho(msg)
		})
	}
}

// TestEngine_HandleNewSession_NoArgs tests handleNewSession with no arguments
func TestEngine_HandleNewSession_NoArgs(t *testing.T) {
	config := &Config{
		Sessions: []SessionConfig{
			{Name: "test", CLIType: "claude", WorkDir: "/tmp"},
		},
		Session: SessionGlobalConfig{
			MaxDynamicSessions: 5,
		},
	}
	engine := NewEngine(config)

	msg := bot.BotMessage{
		Platform: "discord",
		UserID:   "user123",
		Channel:  "channel456",
	}

	// Should not panic with no args
	engine.handleNewSession([]string{}, msg)
}

// TestEngine_HandleNewSession_WithArgs tests handleNewSession with arguments
func TestEngine_HandleNewSession_WithArgs(t *testing.T) {
	config := &Config{
		Sessions: []SessionConfig{
			{Name: "test", CLIType: "claude", WorkDir: "/tmp"},
		},
		Session: SessionGlobalConfig{
			MaxDynamicSessions: 5,
		},
	}
	engine := NewEngine(config)

	msg := bot.BotMessage{
		Platform: "discord",
		UserID:   "user123",
		Channel:  "channel456",
	}

	// Should not panic with args
	engine.handleNewSession([]string{"test-session", "claude", "/tmp"}, msg)
}

// TestEngine_HandleUseSession_NoArgs tests handleUseSession with no arguments
func TestEngine_HandleUseSession_NoArgs(t *testing.T) {
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

	// Should not panic with no args
	engine.handleUseSession([]string{}, msg)
}

// TestEngine_HandleUseSession_WithArg tests handleUseSession with session name
func TestEngine_HandleUseSession_WithArg(t *testing.T) {
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

	// Should not panic with session name arg
	engine.handleUseSession([]string{"test"}, msg)
}

// TestEngine_HandleDeleteSession_NoArgs tests handleDeleteSession with no arguments
func TestEngine_HandleDeleteSession_NoArgs(t *testing.T) {
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

	// Should not panic with no args
	engine.handleDeleteSession([]string{}, msg)
}

// TestEngine_HandleDeleteSession_WithArg tests handleDeleteSession with session name
func TestEngine_HandleDeleteSession_WithArg(t *testing.T) {
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

	// Should not panic with session name arg
	engine.handleDeleteSession([]string{"test-session"}, msg)
}

// TestEngine_ListSessions tests listSessions method
func TestEngine_ListSessions(t *testing.T) {
	config := &Config{
		Sessions: []SessionConfig{
			{Name: "session1", CLIType: "claude", WorkDir: "/tmp"},
			{Name: "session2", CLIType: "gemini", WorkDir: "/tmp"},
		},
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

// TestEngine_ListSessions_EmptyConfig tests listSessions with no sessions
func TestEngine_ListSessions_EmptyConfig(t *testing.T) {
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
