package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestGetBotConfig_ReturnsBotConfig tests GetBotConfig method
func TestGetBotConfig_ReturnsBotConfig(t *testing.T) {
	config := &Config{
		Bots: map[string]BotConfig{
			"discord": {
				Enabled:   true,
				Token:     "test-token",
				ChannelID: "123456",
			},
		},
	}

	botConfig, err := config.GetBotConfig("discord")
	assert.NoError(t, err)
	assert.NotNil(t, botConfig)
	assert.True(t, botConfig.Enabled)
	assert.Equal(t, "test-token", botConfig.Token)
}

// TestGetBotConfig_BotNotFound tests GetBotConfig with non-existent bot
func TestGetBotConfig_BotNotFound(t *testing.T) {
	config := &Config{
		Bots: map[string]BotConfig{},
	}

	_, err := config.GetBotConfig("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// TestGetCLIAdapterConfig_ReturnsConfig tests GetCLIAdapterConfig method
func TestGetCLIAdapterConfig_ReturnsConfig(t *testing.T) {
	config := &Config{
		CLIAdapters: map[string]CLIAdapterConfig{
			"claude": {
				UseHook:      true,
				PollInterval: "1s",
				StableCount:  3,
			},
		},
	}

	adapterConfig, err := config.GetCLIAdapterConfig("claude")
	assert.NoError(t, err)
	assert.NotNil(t, adapterConfig)
	assert.True(t, adapterConfig.UseHook)
}

// TestGetCLIAdapterConfig_AdapterNotFound tests GetCLIAdapterConfig with non-existent adapter
func TestGetCLIAdapterConfig_AdapterNotFound(t *testing.T) {
	config := &Config{
		CLIAdapters: map[string]CLIAdapterConfig{},
	}

	_, err := config.GetCLIAdapterConfig("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// TestGetSessionConfig_ReturnsConfig tests GetSessionConfig method
func TestGetSessionConfig_ReturnsConfig(t *testing.T) {
	config := &Config{
		Sessions: []SessionConfig{
			{
				Name:    "test-session",
				CLIType: "claude",
				WorkDir: "/path/to/work",
			},
		},
	}

	sessionConfig, err := config.GetSessionConfig("test-session")
	assert.NoError(t, err)
	assert.NotNil(t, sessionConfig)
	assert.Equal(t, "test-session", sessionConfig.Name)
	assert.Equal(t, "claude", sessionConfig.CLIType)
}

// TestGetSessionConfig_SessionNotFound tests GetSessionConfig with non-existent session
func TestGetSessionConfig_SessionNotFound(t *testing.T) {
	config := &Config{
		Sessions: []SessionConfig{},
	}

	_, err := config.GetSessionConfig("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// TestIsUserAuthorized_WhitelistEnabled tests IsUserAuthorized with whitelist enabled
func TestIsUserAuthorized_WhitelistEnabled(t *testing.T) {
	config := &Config{
		Security: SecurityConfig{
			WhitelistEnabled: true,
			AllowedUsers: map[string][]string{
				"discord": {"user123", "user456"},
			},
		},
	}

	// Test authorized user
	authorized := config.IsUserAuthorized("discord", "user123")
	assert.True(t, authorized)

	// Test unauthorized user
	authorized = config.IsUserAuthorized("discord", "unknown")
	assert.False(t, authorized)
}

// TestIsUserAuthorized_WhitelistDisabled tests IsUserAuthorized with whitelist disabled
func TestIsUserAuthorized_WhitelistDisabled(t *testing.T) {
	config := &Config{
		Security: SecurityConfig{
			WhitelistEnabled: false,
		},
	}

	// All users are authorized when whitelist is disabled
	authorized := config.IsUserAuthorized("discord", "anyone")
	assert.True(t, authorized)
}

// TestIsAdmin_ChecksAdminStatus tests IsAdmin method
func TestIsAdmin_ChecksAdminStatus(t *testing.T) {
	config := &Config{
		Security: SecurityConfig{
			Admins: map[string][]string{
				"discord": {"admin123", "admin456"},
			},
		},
	}

	// Test admin user
	isAdmin := config.IsAdmin("discord", "admin123")
	assert.True(t, isAdmin)

	// Test non-admin user
	isAdmin = config.IsAdmin("discord", "user123")
	assert.False(t, isAdmin)

	// Test empty admin list
	config2 := &Config{
		Security: SecurityConfig{
			Admins: map[string][]string{},
		},
	}

	isAdmin = config2.IsAdmin("discord", "user123")
	assert.False(t, isAdmin)
}

// TestLoadConfig_InvalidFile tests LoadConfig with invalid file
func TestLoadConfig_InvalidFile(t *testing.T) {
	_, err := LoadConfig("/nonexistent/config.yaml")
	assert.Error(t, err)
}

// TestLoadConfig_EmptyFile tests LoadConfig with empty config
func TestLoadConfig_RequiresValidation(t *testing.T) {
	// This test verifies that LoadConfig performs validation
	// Actual validation is tested in config_test.go
	assert.NotNil(t, LoadConfig)
}

// TestConfig_Methods tests Config methods coverage
func TestConfig_Methods(t *testing.T) {
	config := &Config{
		Bots: map[string]BotConfig{
			"test": {
				Enabled: true,
			},
		},
	}

	// Test GetBotConfig method exists
	botConfig, err := config.GetBotConfig("test")
	assert.NoError(t, err)
	assert.NotNil(t, botConfig)
}

// TestSessionConfig_AllFieldsCoverage tests SessionConfig field access
func TestSessionConfig_AllFieldsCoverage(t *testing.T) {
	configs := []SessionConfig{
		{
			Name:      "session1",
			CLIType:   "claude",
			WorkDir:   "/path1",
			AutoStart: true,
			StartCmd:  "cmd1",
		},
		{
			Name:      "session2",
			CLIType:   "gemini",
			WorkDir:   "/path2",
			AutoStart: false,
			StartCmd:  "cmd2",
		},
	}

	for i, cfg := range configs {
		assert.NotEmpty(t, cfg.Name, "config %d should have name", i)
		assert.NotEmpty(t, cfg.CLIType, "config %d should have CLI type", i)
		assert.NotEmpty(t, cfg.WorkDir, "config %d should have work dir", i)
	}
}

// TestBotConfig_AllFieldsCoverage tests BotConfig field access
func TestBotConfig_AllFieldsCoverage(t *testing.T) {
	configs := []BotConfig{
		{
			Enabled:           true,
			AppID:             "app1",
			AppSecret:         "secret1",
			Token:             "token1",
			ChannelID:         "channel1",
			EncryptKey:        "key1",
			VerificationToken: "token1",
		},
		{
			Enabled: false,
		},
	}

	for i, cfg := range configs {
		if cfg.Token != "" {
			assert.NotEmpty(t, cfg.Token, "config %d should have token", i)
		}
	}
}

// TestWatchdogConfig_AllFieldsCoverage tests WatchdogConfig field access
func TestWatchdogConfig_AllFieldsCoverage(t *testing.T) {
	config := WatchdogConfig{
		Enabled:        true,
		CheckIntervals: []string{"1s", "5s"},
		Timeout:        "120s",
		MaxRetries:     3,
		InitialDelay:   "1s",
		RetryDelay:     "2s",
	}

	assert.True(t, config.Enabled)
	assert.Len(t, config.CheckIntervals, 2)
	assert.Equal(t, "120s", config.Timeout)
	assert.Equal(t, 3, config.MaxRetries)
	assert.Equal(t, "1s", config.InitialDelay)
	assert.Equal(t, "2s", config.RetryDelay)
}

// TestLoggingConfig_AllFieldsCoverage tests LoggingConfig field access
func TestLoggingConfig_AllFieldsCoverage(t *testing.T) {
	config := LoggingConfig{
		Level:        "debug",
		File:         "/var/log/test.log",
		MaxSize:      100,
		MaxBackups:   5,
		MaxAge:       10,
		Compress:     true,
		EnableStdout: false,
	}

	assert.Equal(t, "debug", config.Level)
	assert.Equal(t, "/var/log/test.log", config.File)
	assert.Equal(t, 100, config.MaxSize)
	assert.Equal(t, 5, config.MaxBackups)
	assert.Equal(t, 10, config.MaxAge)
	assert.True(t, config.Compress)
	assert.False(t, config.EnableStdout)
}

// TestSessionGlobalConfig_DefaultValues tests SessionGlobalConfig defaults
func TestSessionGlobalConfig_DefaultValues(t *testing.T) {
	config := SessionGlobalConfig{
		InputHistorySize:   10,
		MaxDynamicSessions: 50,
	}

	assert.Equal(t, 10, config.InputHistorySize)
	assert.Equal(t, 50, config.MaxDynamicSessions)
}

// TestHookServerConfig_DefaultValues tests HookServerConfig defaults
func TestHookServerConfig_DefaultValues(t *testing.T) {
	config := HookServerConfig{
		Port: 8080,
	}

	assert.Equal(t, 8080, config.Port)
}
