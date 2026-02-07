package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestSessionState_Values tests SessionState constants
func TestSessionState_Values(t *testing.T) {
	t.Run("StateIdle", func(t *testing.T) {
		assert.Equal(t, "idle", string(StateIdle))
	})

	t.Run("StateProcessing", func(t *testing.T) {
		assert.Equal(t, "processing", string(StateProcessing))
	})

	t.Run("StateWaitingInput", func(t *testing.T) {
		assert.Equal(t, "waiting_input", string(StateWaitingInput))
	})

	t.Run("StateError", func(t *testing.T) {
		assert.Equal(t, "error", string(StateError))
	})
}

// TestSession_Struct tests Session struct
func TestSession_Struct(t *testing.T) {
	session := Session{
		Name:      "test-session",
		CLIType:   "claude",
		WorkDir:   "/path/to/work",
		StartCmd:  "claude",
		State:     StateIdle,
		CreatedAt: "2024-01-01T00:00:00Z",
		IsDynamic: true,
		CreatedBy: "discord:user123",
	}

	assert.Equal(t, "test-session", session.Name)
	assert.Equal(t, "claude", session.CLIType)
	assert.Equal(t, "/path/to/work", session.WorkDir)
	assert.Equal(t, StateIdle, session.State)
	assert.True(t, session.IsDynamic)
	assert.Equal(t, "discord:user123", session.CreatedBy)
}

// TestResponseEvent_Struct tests ResponseEvent struct
func TestResponseEvent_Struct(t *testing.T) {
	event := ResponseEvent{
		SessionName: "test-session",
		Response:    "Test response",
		Timestamp:   "2024-01-01T00:00:00Z",
	}

	assert.Equal(t, "test-session", event.SessionName)
	assert.Equal(t, "Test response", event.Response)
	assert.Equal(t, "2024-01-01T00:00:00Z", event.Timestamp)
}

// TestSessionConfig_Struct tests SessionConfig struct
func TestSessionConfig_Struct(t *testing.T) {
	config := SessionConfig{
		Name:      "test-session",
		CLIType:   "claude",
		WorkDir:   "/path/to/work",
		AutoStart: true,
	}

	assert.Equal(t, "test-session", config.Name)
	assert.Equal(t, "claude", config.CLIType)
	assert.Equal(t, "/path/to/work", config.WorkDir)
	assert.True(t, config.AutoStart)
}

// TestBotConfig_Struct tests BotConfig struct
func TestBotConfig_Struct(t *testing.T) {
	config := BotConfig{
		Enabled:  true,
		Token:    "test-token",
		ChannelID: "123456",
	}

	assert.True(t, config.Enabled)
	assert.Equal(t, "test-token", config.Token)
	assert.Equal(t, "123456", config.ChannelID)
}

// TestCLIAdapterConfig_Struct tests CLIAdapterConfig struct
func TestCLIAdapterConfig_Struct(t *testing.T) {
	config := CLIAdapterConfig{
		UseHook:      true,
		PollInterval: "1s",
		StableCount:  3,
		PollTimeout:  "120s",
	}

	assert.True(t, config.UseHook)
	assert.Equal(t, "1s", config.PollInterval)
	assert.Equal(t, 3, config.StableCount)
	assert.Equal(t, "120s", config.PollTimeout)
}

// TestSecurityConfig_Struct tests SecurityConfig struct
func TestSecurityConfig_Struct(t *testing.T) {
	config := SecurityConfig{
		WhitelistEnabled: true,
		AllowedUsers: map[string][]string{
			"discord": {"user123", "user456"},
		},
		Admins: map[string][]string{
			"discord": {"admin123"},
		},
	}

	assert.True(t, config.WhitelistEnabled)
	assert.NotNil(t, config.AllowedUsers)
	assert.NotNil(t, config.Admins)
	assert.Equal(t, []string{"user123", "user456"}, config.AllowedUsers["discord"])
	assert.Equal(t, []string{"admin123"}, config.Admins["discord"])
}

// TestHookServerConfig_Struct tests HookServerConfig struct
func TestHookServerConfig_Struct(t *testing.T) {
	config := HookServerConfig{
		Port: 8080,
	}

	assert.Equal(t, 8080, config.Port)
}

// TestWatchdogConfig_Struct tests WatchdogConfig struct
func TestWatchdogConfig_Struct(t *testing.T) {
	config := WatchdogConfig{
		Enabled:        true,
		CheckIntervals: []string{"1s", "5s"},
		Timeout:        "120s",
		MaxRetries:     3,
		InitialDelay:   "1s",
		RetryDelay:     "2s",
	}

	assert.True(t, config.Enabled)
	assert.Equal(t, []string{"1s", "5s"}, config.CheckIntervals)
	assert.Equal(t, "120s", config.Timeout)
	assert.Equal(t, 3, config.MaxRetries)
	assert.Equal(t, "1s", config.InitialDelay)
	assert.Equal(t, "2s", config.RetryDelay)
}

// TestSessionGlobalConfig_Struct tests SessionGlobalConfig struct
func TestSessionGlobalConfig_Struct(t *testing.T) {
	config := SessionGlobalConfig{
		InputHistorySize:   10,
		MaxDynamicSessions: 50,
	}

	assert.Equal(t, 10, config.InputHistorySize)
	assert.Equal(t, 50, config.MaxDynamicSessions)
}

// TestLoggingConfig_Struct tests LoggingConfig struct
func TestLoggingConfig_Struct(t *testing.T) {
	config := LoggingConfig{
		Level:        "info",
		File:         "/var/log/clibot.log",
		MaxSize:      100,
		MaxBackups:   3,
		MaxAge:       7,
		Compress:     true,
		EnableStdout: true,
	}

	assert.Equal(t, "info", config.Level)
	assert.Equal(t, "/var/log/clibot.log", config.File)
	assert.Equal(t, 100, config.MaxSize)
	assert.Equal(t, 3, config.MaxBackups)
	assert.Equal(t, 7, config.MaxAge)
	assert.True(t, config.Compress)
	assert.True(t, config.EnableStdout)
}
