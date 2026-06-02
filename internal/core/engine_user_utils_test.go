package core

import (
	"testing"

	"github.com/keepmind9/clibot/internal/bot"
	_ "github.com/keepmind9/clibot/internal/proxy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEngine_InitializeSessions_UnknownCLIType tests error when adapters not registered
func TestEngine_InitializeSessions_UnknownCLIType(t *testing.T) {
	config := &Config{
		Sessions: []SessionConfig{
			{Name: "session1", CLIType: "claude", WorkDir: "/tmp1"},
			{Name: "session2", CLIType: "gemini", WorkDir: "/tmp2"},
		},
	}
	engine := NewEngine(config)

	err := engine.initializeSessions()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown cli_type")
	assert.Empty(t, engine.sessions)
}

// TestEngine_InitializeSessions_EmptyConfig tests with no sessions
func TestEngine_InitializeSessions_EmptyConfig(t *testing.T) {
	config := &Config{
		Sessions: []SessionConfig{},
	}
	engine := NewEngine(config)

	err := engine.initializeSessions()
	assert.NoError(t, err)

	// Should have no sessions
	assert.Empty(t, engine.sessions)
}

// TestEngine_RegisterBotAdapter_DuplicateRegistration tests registering the same bot type twice
func TestEngine_RegisterBotAdapter_DuplicateRegistration(t *testing.T) {
	config := &Config{
		Sessions: []SessionConfig{
			{Name: "test", CLIType: "claude", WorkDir: "/tmp"},
		},
	}
	engine := NewEngine(config)

	mockBot1 := &mockBotAdapter{}
	mockBot2 := &mockBotAdapter{}

	// Register first bot
	engine.RegisterBotAdapter("testbot", mockBot1)

	// Register second bot with same type (should overwrite)
	engine.RegisterBotAdapter("testbot", mockBot2)

	// Verify the bot was registered
	_, exists := engine.activeBots["testbot"]
	assert.True(t, exists)
}

// TestEngine_InitializeSessions_DuplicateNames tests duplicate session names
// First occurrence wins; subsequent duplicates are silently skipped
func TestEngine_InitializeSessions_DuplicateNames(t *testing.T) {
	config := &Config{
		Sessions: []SessionConfig{
			{Name: "dup", CLIType: "claude", WorkDir: "/tmp1"},
			{Name: "dup", CLIType: "claude", WorkDir: "/tmp2"},
		},
	}
	engine := NewEngine(config)
	engine.RegisterCLIAdapter("claude", &mockCLIAdapter{})

	err := engine.initializeSessions()
	require.NoError(t, err)

	// Only one session kept (first occurrence wins)
	assert.Len(t, engine.sessions, 1)
	assert.Equal(t, "/tmp1", engine.sessions["dup"].WorkDir)
}

// TestEngine_InitializeSessions_MissingCLIType tests that missing cli_type returns an error
func TestEngine_InitializeSessions_MissingCLIType(t *testing.T) {
	tests := []struct {
		name    string
		session SessionConfig
		errMsg  string
	}{
		{
			name:    "empty cli_type",
			session: SessionConfig{Name: "test", CLIType: "", WorkDir: "/tmp"},
			errMsg:  "missing required field 'cli_type'",
		},
		{
			name:    "unknown cli_type",
			session: SessionConfig{Name: "test", CLIType: "unknown-cli", WorkDir: "/tmp"},
			errMsg:  "unknown cli_type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				Sessions: []SessionConfig{tt.session},
			}
			engine := NewEngine(config)

			err := engine.initializeSessions()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errMsg)
			assert.Empty(t, engine.sessions)
		})
	}
}

// TestEngine_GetUserKey tests the getUserKey function
func TestEngine_GetUserKey(t *testing.T) {
	tests := []struct {
		name     string
		platform string
		userID   string
		expected string
	}{
		{
			name:     "discord user",
			platform: "discord",
			userID:   "user123",
			expected: "discord:user123",
		},
		{
			name:     "telegram user",
			platform: "telegram",
			userID:   "user456",
			expected: "telegram:user456",
		},
		{
			name:     "feishu user",
			platform: "feishu",
			userID:   "user789",
			expected: "feishu:user789",
		},
		{
			name:     "empty platform",
			platform: "",
			userID:   "user123",
			expected: ":user123",
		},
		{
			name:     "empty userID",
			platform: "discord",
			userID:   "",
			expected: "discord:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getUserKey(tt.platform, tt.userID)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestEngine_ListSessions_WithUserSession tests listSessions with user's current session
func TestEngine_ListSessions_WithUserSession(t *testing.T) {
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

	// Set user's current session
	userKey := getUserKey("testbot", "user123")
	engine.userSessions[userKey] = "session1"

	mockBot := &mockBotAdapter{}
	engine.RegisterBotAdapter("testbot", mockBot)

	msg := bot.BotMessage{
		Platform: "testbot",
		Channel:  "test-channel",
		UserID:   "user123",
	}

	engine.listSessions(msg)

	// Should send message with current session info
	assert.Equal(t, 1, mockBot.messageCount)
	assert.Contains(t, mockBot.lastMessage, "session1")
}

// TestEngine_EnsureSessionStarted_NonExistentSession tests ensureSessionStarted with non-existent session
func TestEngine_EnsureSessionStarted_NonExistentSession(t *testing.T) {
	config := &Config{
		Sessions: []SessionConfig{
			{
				Name:      "test-session",
				CLIType:   "claude",
				WorkDir:   "/tmp",
				AutoStart: false,
			},
		},
	}
	engine := NewEngine(config)

	session := &Session{
		Name:    "test-session",
		CLIType: "claude",
		WorkDir: "/tmp",
		State:   StateIdle,
	}

	wasRunning, err := engine.ensureSessionStarted(session, config.Sessions[0])

	// Session doesn't actually exist, so should return error
	require.Error(t, err)
	assert.False(t, wasRunning)
}
