package core

import (
	"testing"
	"time"

	"github.com/keepmind9/clibot/internal/bot"
	_ "github.com/keepmind9/clibot/internal/proxy"
	"github.com/stretchr/testify/assert"
)

// TestEngine_HandleEcho tests the handleEcho function
func TestEngine_HandleEcho(t *testing.T) {
	config := &Config{
		Sessions: []SessionConfig{
			{Name: "test", CLIType: "claude", WorkDir: "/tmp"},
		},
	}
	engine := NewEngine(config)

	mockBot := &mockBotAdapter{}
	engine.RegisterBotAdapter("testbot", mockBot)

	msg := bot.BotMessage{
		Platform:  "testbot",
		Channel:   "test-channel",
		UserID:    "user123",
		Timestamp: time.Now(),
	}

	engine.handleEcho(msg)

	// Should send a message with user info
	assert.Equal(t, 1, mockBot.messageCount)
	assert.Contains(t, mockBot.lastMessage, "Your IM Information")
	assert.Contains(t, mockBot.lastMessage, "testbot")
	assert.Contains(t, mockBot.lastMessage, "user123")
	assert.Contains(t, mockBot.lastMessage, "test-channel")
}

// TestEngine_HandleShowHelp tests the showHelp function
func TestEngine_HandleShowHelp(t *testing.T) {
	config := &Config{
		Sessions: []SessionConfig{
			{Name: "test", CLIType: "claude", WorkDir: "/tmp"},
		},
	}
	engine := NewEngine(config)

	mockBot := &mockBotAdapter{}
	engine.RegisterBotAdapter("testbot", mockBot)

	msg := bot.BotMessage{
		Platform: "testbot",
		Channel:  "test-channel",
		UserID:   "user123",
	}

	engine.showHelp(msg)

	// Should send help message
	assert.Equal(t, 1, mockBot.messageCount)
	assert.Contains(t, mockBot.lastMessage, "clibot Help")
	assert.Contains(t, mockBot.lastMessage, "Special Commands")
}

// TestEngine_HandleUseSession_NoArgs tests handleUseSession with no arguments
func TestEngine_HandleUseSession_NoArgs(t *testing.T) {
	config := &Config{
		Sessions: []SessionConfig{
			{Name: "test", CLIType: "claude", WorkDir: "/tmp"},
		},
	}
	engine := NewEngine(config)

	mockBot := &mockBotAdapter{}
	engine.RegisterBotAdapter("testbot", mockBot)

	msg := bot.BotMessage{
		Platform: "testbot",
		Channel:  "test-channel",
		UserID:   "user123",
	}

	engine.handleUseSession([]string{}, msg)

	// Should send error message about invalid arguments
	assert.Equal(t, 1, mockBot.messageCount)
	assert.Contains(t, mockBot.lastMessage, "Invalid arguments")
}

// TestEngine_HandleUseSession_WithSession tests handleUseSession with valid session
func TestEngine_HandleUseSession_WithSession(t *testing.T) {
	config := &Config{
		Sessions: []SessionConfig{
			{Name: "session1", CLIType: "claude", WorkDir: "/tmp"},
		},
	}
	engine := NewEngine(config)

	// Create a session
	engine.sessions["session1"] = &Session{
		Name:    "session1",
		CLIType: "claude",
		State:   StateIdle,
	}

	mockBot := &mockBotAdapter{}
	engine.RegisterBotAdapter("testbot", mockBot)

	msg := bot.BotMessage{
		Platform: "testbot",
		Channel:  "test-channel",
		UserID:   "user123",
	}

	engine.handleUseSession([]string{"session1"}, msg)

	// Should send a message (success or error)
	assert.Equal(t, 1, mockBot.messageCount)
}

// TestEngine_HandleUseSession_NonExistingSession tests handleUseSession with non-existing session
func TestEngine_HandleUseSession_NonExistingSession(t *testing.T) {
	config := &Config{
		Sessions: []SessionConfig{
			{Name: "session1", CLIType: "claude", WorkDir: "/tmp"},
		},
	}
	engine := NewEngine(config)

	mockBot := &mockBotAdapter{}
	engine.RegisterBotAdapter("testbot", mockBot)

	msg := bot.BotMessage{
		Platform: "testbot",
		Channel:  "test-channel",
		UserID:   "user123",
	}

	engine.handleUseSession([]string{"nonexistent"}, msg)

	// Should send error message
	assert.Equal(t, 1, mockBot.messageCount)
	assert.Contains(t, mockBot.lastMessage, "does not exist")
}

// TestEngine_HandleNewSession_NoArgs tests handleNewSession with no arguments
func TestEngine_HandleNewSession_NoArgs(t *testing.T) {
	config := &Config{
		Sessions: []SessionConfig{
			{Name: "test", CLIType: "claude", WorkDir: "/tmp"},
		},
		Security: SecurityConfig{
			Admins: map[string][]string{
				"testbot": {"user123"},
			},
		},
	}
	engine := NewEngine(config)

	mockBot := &mockBotAdapter{}
	engine.RegisterBotAdapter("testbot", mockBot)

	msg := bot.BotMessage{
		Platform: "testbot",
		Channel:  "test-channel",
		UserID:   "user123",
	}

	engine.handleNewSession([]string{}, msg)

	// Should send error about invalid arguments
	assert.Equal(t, 1, mockBot.messageCount)
	assert.Contains(t, mockBot.lastMessage, "Invalid arguments")
}

// TestEngine_HandleNewSession_NotAdmin tests handleNewSession by non-admin
func TestEngine_HandleNewSession_NotAdmin(t *testing.T) {
	config := &Config{
		Sessions: []SessionConfig{
			{Name: "test", CLIType: "claude", WorkDir: "/tmp"},
		},
		Security: SecurityConfig{
			Admins: map[string][]string{
				"testbot": {"admin456"}, // Different user
			},
		},
	}
	engine := NewEngine(config)

	mockBot := &mockBotAdapter{}
	engine.RegisterBotAdapter("testbot", mockBot)

	msg := bot.BotMessage{
		Platform: "testbot",
		Channel:  "test-channel",
		UserID:   "user123", // Not admin
	}

	engine.handleNewSession([]string{"newsession", "claude", "/tmp"}, msg)

	// Should send permission denied error
	assert.Equal(t, 1, mockBot.messageCount)
	assert.Contains(t, mockBot.lastMessage, "Permission denied")
}

// TestEngine_HandleDeleteSession_NoArgs tests handleDeleteSession with no arguments
func TestEngine_HandleDeleteSession_NoArgs(t *testing.T) {
	config := &Config{
		Sessions: []SessionConfig{
			{Name: "test", CLIType: "claude", WorkDir: "/tmp"},
		},
	}
	engine := NewEngine(config)

	mockBot := &mockBotAdapter{}
	engine.RegisterBotAdapter("testbot", mockBot)

	msg := bot.BotMessage{
		Platform: "testbot",
		Channel:  "test-channel",
		UserID:   "user123",
	}

	engine.handleDeleteSession([]string{}, msg)

	// Should send error message
	assert.Equal(t, 1, mockBot.messageCount)
}

// TestEngine_HandleNewSession_NonACPWithoutHookServer tests creating non-ACP session when hook server is not running
func TestEngine_HandleNewSession_NonACPWithoutHookServer(t *testing.T) {
	config := &Config{
		Sessions: []SessionConfig{
			{Name: "test", CLIType: "acp", WorkDir: "/tmp"}, // Only ACP sessions
		},
		Security: SecurityConfig{
			Admins: map[string][]string{
				"testbot": {"user123"},
			},
		},
	}
	engine := NewEngine(config)

	// Register CLI adapter (using nil is sufficient for this test)
	engine.RegisterCLIAdapter("claude", nil)

	mockBot := &mockBotAdapter{}
	engine.RegisterBotAdapter("testbot", mockBot)

	msg := bot.BotMessage{
		Platform: "testbot",
		Channel:  "test-channel",
		UserID:   "user123", // Admin
	}

	// Try to create a non-ACP session when hook server is not running
	engine.handleNewSession([]string{"newsession", "claude", "/tmp"}, msg)

	// Should send error about hook server not running
	assert.Equal(t, 1, mockBot.messageCount)
	assert.Contains(t, mockBot.lastMessage, "HTTP hook server is not running")
	assert.Contains(t, mockBot.lastMessage, "All configured sessions are ACP type")
}

// TestEngine_HandleNewSession_ACPWithoutHookServer tests creating ACP session when hook server is not running
func TestEngine_HandleNewSession_ACPWithoutHookServer(t *testing.T) {
	config := &Config{
		Sessions: []SessionConfig{
			{Name: "test", CLIType: "acp", WorkDir: "/tmp"}, // Only ACP sessions
		},
		Security: SecurityConfig{
			Admins: map[string][]string{
				"testbot": {"user123"},
			},
		},
	}
	engine := NewEngine(config)

	// Register CLI adapter (using nil is sufficient for this test)
	engine.RegisterCLIAdapter("acp", nil)

	mockBot := &mockBotAdapter{}
	engine.RegisterBotAdapter("testbot", mockBot)

	msg := bot.BotMessage{
		Platform: "testbot",
		Channel:  "test-channel",
		UserID:   "user123", // Admin
	}

	// Try to create an ACP session (should work even without hook server)
	// This test verifies the check only applies to non-ACP sessions
	// Note: This will fail at directory validation since we don't create /tmp/test-acp,
	// but we should NOT get the "hook server not running" error
	engine.handleNewSession([]string{"new-acp", "acp", "/tmp/test-acp"}, msg)

	// Should NOT send error about hook server (may fail at directory check instead)
	assert.Equal(t, 1, mockBot.messageCount)
	assert.NotContains(t, mockBot.lastMessage, "HTTP hook server is not running")
}
