package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestConfig_StructFields tests Config struct field access
func TestConfig_StructFields(t *testing.T) {
	config := Config{
		HookServer: HookServerConfig{
			Port: 8080,
		},
		Security: SecurityConfig{
			WhitelistEnabled: true,
		},
		Watchdog: WatchdogConfig{
			Enabled: true,
		},
		Session: SessionGlobalConfig{
			InputHistorySize: 10,
		},
		Sessions: []SessionConfig{
			{
				Name:    "test-session",
				CLIType: "claude",
			},
		},
		Bots: map[string]BotConfig{
			"discord": {
				Enabled: true,
			},
		},
		CLIAdapters: map[string]CLIAdapterConfig{
			"claude": {
				UseHook: true,
			},
		},
		Logging: LoggingConfig{
			Level: "info",
		},
	}

	assert.Equal(t, 8080, config.HookServer.Port)
	assert.True(t, config.Security.WhitelistEnabled)
	assert.True(t, config.Watchdog.Enabled)
	assert.Equal(t, 10, config.Session.InputHistorySize)
	assert.Len(t, config.Sessions, 1)
	assert.Len(t, config.Bots, 1)
	assert.Len(t, config.CLIAdapters, 1)
	assert.Equal(t, "info", config.Logging.Level)
}

// TestConfig_EmptyMaps tests Config with empty maps
func TestConfig_EmptyMaps(t *testing.T) {
	config := Config{
		Bots:        map[string]BotConfig{},
		CLIAdapters: map[string]CLIAdapterConfig{},
		Sessions:    []SessionConfig{},
	}

	assert.NotNil(t, config.Bots)
	assert.NotNil(t, config.CLIAdapters)
	assert.NotNil(t, config.Sessions)
	assert.Len(t, config.Bots, 0)
	assert.Len(t, config.CLIAdapters, 0)
	assert.Len(t, config.Sessions, 0)
}

// TestConfig_NilMaps tests Config with nil maps
func TestConfig_NilMaps(t *testing.T) {
	config := Config{}

	assert.Nil(t, config.Bots)
	assert.Nil(t, config.CLIAdapters)
	assert.Nil(t, config.Sessions)
}

// TestSessionState_StringConversion tests SessionState string conversion
func TestSessionState_StringConversion(t *testing.T) {
	tests := []struct {
		state    SessionState
		expected string
	}{
		{StateIdle, "idle"},
		{StateProcessing, "processing"},
		{StateWaitingInput, "waiting_input"},
		{StateError, "error"},
		{SessionState("custom"), "custom"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, string(tt.state))
		})
	}
}

// TestSession_IsDynamic tests IsDynamic field
func TestSession_IsDynamic(t *testing.T) {
	staticSession := Session{
		Name:      "static-session",
		IsDynamic: false,
	}
	assert.False(t, staticSession.IsDynamic)

	dynamicSession := Session{
		Name:      "dynamic-session",
		IsDynamic: true,
		CreatedBy: "discord:user123",
	}
	assert.True(t, dynamicSession.IsDynamic)
}

// TestSession_CreatedBy tests CreatedBy field
func TestSession_CreatedBy(t *testing.T) {
	session := Session{
		CreatedBy: "discord:user123",
	}
	assert.Equal(t, "discord:user123", session.CreatedBy)
}

// TestSession_CancelContext tests cancel context field
func TestSession_CancelContext(t *testing.T) {
	session := Session{
		cancelCtx: nil,
	}
	assert.Nil(t, session.cancelCtx)
}

// TestResponseEvent_Fields tests ResponseEvent fields
func TestResponseEvent_Fields(t *testing.T) {
	event := ResponseEvent{
		SessionName: "test-session",
		Response:    "Test response",
		Timestamp:   "2024-01-01T00:00:00Z",
	}

	assert.Equal(t, "test-session", event.SessionName)
	assert.Equal(t, "Test response", event.Response)
	assert.Equal(t, "2024-01-01T00:00:00Z", event.Timestamp)
}

// TestResponseEvent_EmptyFields tests empty ResponseEvent
func TestResponseEvent_EmptyFields(t *testing.T) {
	event := ResponseEvent{}

	assert.Empty(t, event.SessionName)
	assert.Empty(t, event.Response)
	assert.Empty(t, event.Timestamp)
}

// TestSessionConfig_AutoStart tests AutoStart field
func TestSessionConfig_AutoStart(t *testing.T) {
	configAuto := SessionConfig{
		Name:      "auto-session",
		AutoStart: true,
	}
	assert.True(t, configAuto.AutoStart)

	configManual := SessionConfig{
		Name:      "manual-session",
		AutoStart: false,
	}
	assert.False(t, configManual.AutoStart)
}
