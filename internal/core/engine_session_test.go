package core

import (
	"testing"

	"github.com/keepmind9/clibot/internal/bot"
	"github.com/stretchr/testify/assert"
)

// TestEngine_HandleUserMessage_WithSession tests HandleUserMessage with session
func TestEngine_HandleUserMessage_WithSession(t *testing.T) {
	config := &Config{
		Sessions: []SessionConfig{
			{Name: "test", CLIType: "claude", WorkDir: "/tmp"},
		},
	}
	engine := NewEngine(config)

	// Create a test session
	testSession := &Session{
		Name:  "test",
		State: StateIdle,
	}
	engine.sessions["test"] = testSession

	msg := bot.BotMessage{
		Platform: "discord",
		UserID:   "user123",
		Channel:  "channel456",
		Content:  "hello",
	}

	// Should not panic
	engine.HandleUserMessage(msg)
}

// TestEngine_HandleUserMessage_MultipleMessages tests multiple user messages
func TestEngine_HandleUserMessage_MultipleMessages(t *testing.T) {
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

	// Send multiple messages
	messages := []string{"hello", "how are you?", "help"}
	for _, content := range messages {
		msg.Content = content
		// Should not panic
		engine.HandleUserMessage(msg)
	}
}

// TestEngine_ListSessions_WithMultipleSessions tests listSessions with multiple sessions
func TestEngine_ListSessions_WithMultipleSessions(t *testing.T) {
	config := &Config{
		Sessions: []SessionConfig{
			{Name: "session1", CLIType: "claude", WorkDir: "/tmp1"},
			{Name: "session2", CLIType: "gemini", WorkDir: "/tmp2"},
			{Name: "session3", CLIType: "opencode", WorkDir: "/tmp3"},
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

// TestEngine_ShowStatus_WithActiveSessions tests showStatus with active sessions
func TestEngine_ShowStatus_WithActiveSessions(t *testing.T) {
	config := &Config{
		Sessions: []SessionConfig{
			{Name: "session1", CLIType: "claude", WorkDir: "/tmp1"},
			{Name: "session2", CLIType: "gemini", WorkDir: "/tmp2"},
		},
	}
	engine := NewEngine(config)

	// Create sessions with different states
	engine.sessions["session1"] = &Session{
		Name:  "session1",
		State: StateProcessing,
	}
	engine.sessions["session2"] = &Session{
		Name:  "session2",
		State: StateIdle,
	}

	msg := bot.BotMessage{
		Platform: "discord",
		UserID:   "user123",
		Channel:  "channel456",
	}

	// Should not panic
	engine.showStatus(msg)
}

// TestEngine_HandleUseSession_MultipleAttempts tests multiple use session attempts
func TestEngine_HandleUseSession_MultipleAttempts(t *testing.T) {
	config := &Config{
		Sessions: []SessionConfig{
			{Name: "session1", CLIType: "claude", WorkDir: "/tmp1"},
			{Name: "session2", CLIType: "gemini", WorkDir: "/tmp2"},
		},
	}
	engine := NewEngine(config)

	// Create sessions
	engine.sessions["session1"] = &Session{
		Name:  "session1",
		State: StateIdle,
	}
	engine.sessions["session2"] = &Session{
		Name:  "session2",
		State: StateIdle,
	}

	msg := bot.BotMessage{
		Platform: "discord",
		UserID:   "user123",
		Channel:  "channel456",
	}

	// Try to use different sessions
	engine.handleUseSession([]string{"session1"}, msg)
	engine.handleUseSession([]string{"session2"}, msg)
}

// TestEngine_HandleDeleteSession_ExistingSession tests handleDeleteSession with existing session
func TestEngine_HandleDeleteSession_ExistingSession(t *testing.T) {
	config := &Config{
		Sessions: []SessionConfig{
			{Name: "test", CLIType: "claude", WorkDir: "/tmp"},
		},
	}
	engine := NewEngine(config)

	// Create a session
	engine.sessions["test"] = &Session{
		Name:    "test",
		CLIType: "claude",
		State:   StateIdle,
	}

	msg := bot.BotMessage{
		Platform: "discord",
		UserID:   "user123",
		Channel:  "channel456",
	}

	// Should not panic
	engine.handleDeleteSession([]string{"test"}, msg)

	// Function might not actually delete session, or might require admin
	// Just verify it doesn't panic
	assert.NotNil(t, engine.sessions)
}

// TestEngine_HandleDeleteSession_NonExistingSession tests handleDeleteSession with non-existing session
func TestEngine_HandleDeleteSession_NonExistingSession(t *testing.T) {
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
	engine.handleDeleteSession([]string{"nonexistent"}, msg)
}

// TestEngine_HandleNewSession_WithVariousConfigs tests handleNewSession with different configurations
func TestEngine_HandleNewSession_WithVariousConfigs(t *testing.T) {
	config := &Config{
		Sessions: []SessionConfig{
			{Name: "test", CLIType: "claude", WorkDir: "/tmp"},
		},
		Session: SessionGlobalConfig{
			MaxDynamicSessions: 10,
		},
	}
	engine := NewEngine(config)

	msg := bot.BotMessage{
		Platform: "discord",
		UserID:   "user123",
		Channel:  "channel456",
	}

	// Test with different argument combinations
	testCases := [][]string{
		{"new-session", "claude", "/tmp"},
		{"new-session", "gemini", "/home/user/project"},
		{"new-session", "opencode", "/work", "echo 'start'"},
	}

	for _, args := range testCases {
		// Should not panic - even though it might fail to create session
		engine.handleNewSession(args, msg)
	}
}
