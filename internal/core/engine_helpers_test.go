package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestSessionState_StringRepresentation tests SessionState string representation
func TestSessionState_StringRepresentation(t *testing.T) {
	states := []SessionState{
		StateIdle,
		StateProcessing,
		StateWaitingInput,
		StateError,
	}

	for _, state := range states {
		assert.NotEmpty(t, string(state), "state should have string representation")
	}
}

// TestSession_DefaultValues tests Session default values
func TestSession_DefaultValues(t *testing.T) {
	session := Session{}

	assert.Empty(t, session.Name)
	assert.Empty(t, session.CLIType)
	assert.Empty(t, session.WorkDir)
	assert.Empty(t, session.StartCmd)
	assert.Empty(t, session.State)
	assert.Empty(t, session.CreatedAt)
	assert.False(t, session.IsDynamic)
	assert.Empty(t, session.CreatedBy)
	assert.Nil(t, session.cancelCtx)
}

// TestResponseEvent_DefaultValues tests ResponseEvent default values
func TestResponseEvent_DefaultValues(t *testing.T) {
	event := ResponseEvent{}

	assert.Empty(t, event.SessionName)
	assert.Empty(t, event.Response)
	assert.Empty(t, event.Timestamp)
}

// TestConfig_EmptyConfig tests empty Config
func TestConfig_EmptyConfig(t *testing.T) {
	config := Config{}

	assert.Equal(t, 0, config.HookServer.Port)
	assert.False(t, config.Security.WhitelistEnabled)
	assert.Equal(t, 0, config.Session.InputHistorySize)
	assert.Empty(t, config.Logging.Level)
}

// TestSecurityConfig_EmptyConfig tests SecurityConfig with empty maps
func TestSecurityConfig_EmptyConfig(t *testing.T) {
	config := SecurityConfig{
		AllowedUsers: map[string][]string{},
		Admins:       map[string][]string{},
	}

	assert.False(t, config.WhitelistEnabled)
	assert.NotNil(t, config.AllowedUsers)
	assert.NotNil(t, config.Admins)
	assert.Len(t, config.AllowedUsers, 0)
	assert.Len(t, config.Admins, 0)
}

// TestSessionConfig_MinimalConfig tests minimal SessionConfig
func TestSessionConfig_MinimalConfig(t *testing.T) {
	config := SessionConfig{
		Name:    "test",
		CLIType: "claude",
	}

	assert.Equal(t, "test", config.Name)
	assert.Equal(t, "claude", config.CLIType)
	assert.Empty(t, config.WorkDir)
	assert.False(t, config.AutoStart)
}

// TestBotConfig_MinimalConfig tests minimal BotConfig
func TestBotConfig_MinimalConfig(t *testing.T) {
	config := BotConfig{
		Enabled: true,
		Token:   "test-token",
	}

	assert.True(t, config.Enabled)
	assert.Equal(t, "test-token", config.Token)
	assert.Empty(t, config.ChannelID)
	assert.Empty(t, config.AppID)
	assert.Empty(t, config.AppSecret)
}

// TestCLIAdapterConfig_PollingOnlyConfig tests polling-only configuration
func TestCLIAdapterConfig_PollingOnlyConfig(t *testing.T) {
	config := CLIAdapterConfig{
		UseHook:      false,
		PollInterval: "2s",
		StableCount:  5,
		PollTimeout:  "180s",
	}

	assert.False(t, config.UseHook)
	assert.Equal(t, "2s", config.PollInterval)
	assert.Equal(t, 5, config.StableCount)
	assert.Equal(t, "180s", config.PollTimeout)
}

// TestWatchdogConfig_EnabledOnly tests watchdog enabled configuration
func TestWatchdogConfig_EnabledOnly(t *testing.T) {
	config := WatchdogConfig{
		Enabled: true,
	}

	assert.True(t, config.Enabled)
	assert.Empty(t, config.CheckIntervals)
	assert.Empty(t, config.Timeout)
	assert.Equal(t, 0, config.MaxRetries)
}

// TestLoggingConfig_StdoutOnly tests stdout-only logging
func TestLoggingConfig_StdoutOnly(t *testing.T) {
	config := LoggingConfig{
		Level:        "debug",
		EnableStdout: true,
	}

	assert.Equal(t, "debug", config.Level)
	assert.True(t, config.EnableStdout)
	assert.Empty(t, config.File)
}

// TestHookServerConfig_DefaultPort tests default port
func TestHookServerConfig_DefaultPort(t *testing.T) {
	config := HookServerConfig{}

	assert.Equal(t, 0, config.Port)
}

// TestSessionGlobalConfig_ZeroValues tests zero values
func TestSessionGlobalConfig_ZeroValues(t *testing.T) {
	config := SessionGlobalConfig{}

	assert.Equal(t, 0, config.InputHistorySize)
	assert.Equal(t, 0, config.MaxDynamicSessions)
}

// TestMultipleConfigs_SliceAccess tests accessing multiple session configs
func TestMultipleConfigs_SliceAccess(t *testing.T) {
	configs := []SessionConfig{
		{Name: "session1", CLIType: "claude"},
		{Name: "session2", CLIType: "gemini"},
		{Name: "session3", CLIType: "opencode"},
	}

	assert.Len(t, configs, 3)
	assert.Equal(t, "session1", configs[0].Name)
	assert.Equal(t, "gemini", configs[1].CLIType)
	assert.Equal(t, "opencode", configs[2].CLIType)
}

// TestMultipleBots_MapAccess tests accessing multiple bot configs
func TestMultipleBots_MapAccess(t *testing.T) {
	bots := map[string]BotConfig{
		"discord": {
			Enabled: true,
			Token:   "discord-token",
		},
		"telegram": {
			Enabled: true,
			Token:   "telegram-token",
		},
		"feishu": {
			Enabled: true,
			Token:   "feishu-token",
		},
	}

	assert.Len(t, bots, 3)
	assert.True(t, bots["discord"].Enabled)
	assert.True(t, bots["telegram"].Enabled)
	assert.True(t, bots["feishu"].Enabled)
}

// TestMultipleAdapters_MapAccess tests accessing multiple adapter configs
func TestMultipleAdapters_MapAccess(t *testing.T) {
	adapters := map[string]CLIAdapterConfig{
		"claude": {
			UseHook: true,
		},
		"gemini": {
			UseHook: true,
		},
		"opencode": {
			UseHook: false,
		},
	}

	assert.Len(t, adapters, 3)
	assert.True(t, adapters["claude"].UseHook)
	assert.True(t, adapters["gemini"].UseHook)
	assert.False(t, adapters["opencode"].UseHook)
}

// TestConfig_FieldInitialization tests config field initialization order
func TestConfig_FieldInitialization(t *testing.T) {
	config := Config{
		HookServer: HookServerConfig{Port: 8080},
		Logging:    LoggingConfig{Level: "info"},
	}

	assert.Equal(t, 8080, config.HookServer.Port)
	assert.Equal(t, "info", config.Logging.Level)
}

// TestSession_CopyValue tests copying session values
func TestSession_CopyValue(t *testing.T) {
	session1 := Session{
		Name:      "session1",
		CLIType:   "claude",
			State:     StateIdle,
		IsDynamic: false,
	}

	session2 := session1
	session2.Name = "session2"

	assert.Equal(t, "session1", session1.Name)
	assert.Equal(t, "session2", session2.Name)
}

// TestResponseEvent_CopyValue tests copying event values
func TestResponseEvent_CopyValue(t *testing.T) {
	event1 := ResponseEvent{
		SessionName: "session1",
		Response:    "response1",
		Timestamp:   "2024-01-01",
	}

	event2 := event1
	event2.Response = "response2"

	assert.Equal(t, "response1", event1.Response)
	assert.Equal(t, "response2", event2.Response)
}
