package core

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadConfig_ValidConfig_ReturnsConfigStruct(t *testing.T) {
	// Create temporary config file
	configContent := `
hook_server:
  port: 8080

security:
  whitelist_enabled: true
  allowed_users:
    discord:
      - "123456789012345678"
sessions:
  - name: "test-session"
    cli_type: "claude"
    work_dir: "/tmp/test"
    auto_start: false
bots:
  discord:
    enabled: true
    token: "${TEST_TOKEN}"
cli_adapters:
  claude:
    history_dir: "~/.claude/conversations"
    interactive:
      enabled: true
      check_lines: 3
      patterns:
        - "\\? [y/N]"
`
	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(configContent); err != nil {
		t.Fatalf("Failed to write config content: %v", err)
	}
	tmpFile.Close()

	// Set environment variable for testing
	os.Setenv("TEST_TOKEN", "test-token-12345")
	defer os.Unsetenv("TEST_TOKEN")

	// Load config
	config, err := LoadConfig(tmpFile.Name())

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, 8080, config.HookServer.Port)
	assert.True(t, config.Security.WhitelistEnabled)
	assert.Len(t, config.Sessions, 1)
	assert.Equal(t, "test-session", config.Sessions[0].Name)
}

func TestLoadConfig_EnvExpansion_ExpandsVariables(t *testing.T) {
	// Create temporary config file with environment variables
	configContent := `
hook_server:
  port: 8080

security:
  whitelist_enabled: false
sessions:
  - name: "test-session"
    cli_type: "claude"
    work_dir: "/tmp/test"
    auto_start: false
bots:
  discord:
    enabled: true
    token: "${DISCORD_TOKEN}"
cli_adapters:
  claude:
    history_dir: "~/.claude/conversations"
    interactive:
      enabled: true
      check_lines: 3
      patterns:
        - "\\? [y/N]"
`
	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(configContent); err != nil {
		t.Fatalf("Failed to write config content: %v", err)
	}
	tmpFile.Close()

	// Set environment variable
	os.Setenv("DISCORD_TOKEN", "my-secret-token")
	defer os.Unsetenv("DISCORD_TOKEN")

	// Load config
	config, err := LoadConfig(tmpFile.Name())

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, "my-secret-token", config.Bots["discord"].Token)
}

func TestLoadConfig_HomeDirectoryExpansion_DeprecatedFieldNoLongerProcessed(t *testing.T) {
	// Create temporary config file with ~ in paths
	configContent := `
hook_server:
  port: 8080

security:
  whitelist_enabled: false
sessions:
  - name: "test-session"
    cli_type: "claude"
    work_dir: "/tmp/test"
    auto_start: false
bots:
  discord:
    enabled: true
    token: "test-token"
cli_adapters:
  claude:
    history_dir: "~/.claude/conversations"
    use_hook: true
    poll_interval: "1s"
    stable_count: 3
`
	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(configContent); err != nil {
		t.Fatalf("Failed to write config content: %v", err)
	}
	tmpFile.Close()

	// Load config
	config, err := LoadConfig(tmpFile.Name())

	// Assert
	assert.NoError(t, err)

	// GetCLIAdapterConfig should succeed but history_dir is no longer expanded
	adapterConfig, err := config.GetCLIAdapterConfig("claude")
	assert.NoError(t, err)
	// history_dir is kept as-is for backward compatibility, but no longer expanded
	assert.Equal(t, "~/.claude/conversations", adapterConfig.HistoryDir)
}

func TestLoadConfig_InvalidFile_ReturnsError(t *testing.T) {
	// Try to load non-existent file
	_, err := LoadConfig("/nonexistent/path/config.yaml")

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read config file")
}

func TestGetBotConfig_ValidBot_ReturnsConfig(t *testing.T) {
	config := &Config{
		Bots: map[string]BotConfig{
			"discord": {
				Enabled: true,
				Token:   "test-token",
			},
		},
	}

	botConfig, err := config.GetBotConfig("discord")

	// Assert
	assert.NoError(t, err)
	assert.True(t, botConfig.Enabled)
	assert.Equal(t, "test-token", botConfig.Token)
}

func TestGetBotConfig_DisabledBot_ReturnsError(t *testing.T) {
	config := &Config{
		Bots: map[string]BotConfig{
			"discord": {
				Enabled: false,
				Token:   "test-token",
			},
		},
	}

	_, err := config.GetBotConfig("discord")

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "is disabled")
}

func TestGetBotConfig_NonExistentBot_ReturnsError(t *testing.T) {
	config := &Config{
		Bots: map[string]BotConfig{},
	}

	_, err := config.GetBotConfig("telegram")

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found in configuration")
}

func TestGetCLIAdapterConfig_ValidAdapter_ReturnsConfig(t *testing.T) {
	config := &Config{
		CLIAdapters: map[string]CLIAdapterConfig{
			"claude": {
				HistoryDir: "~/.claude/conversations",
				UseHook:    true,
			},
		},
	}

	adapterConfig, err := config.GetCLIAdapterConfig("claude")

	// Assert
	assert.NoError(t, err)
	// HistoryDir is kept for backward compatibility but no longer expanded
	assert.Equal(t, "~/.claude/conversations", adapterConfig.HistoryDir)
	assert.True(t, adapterConfig.UseHook)
}

func TestGetCLIAdapterConfig_NonExistentAdapter_ReturnsError(t *testing.T) {
	config := &Config{
		CLIAdapters: map[string]CLIAdapterConfig{},
	}

	_, err := config.GetCLIAdapterConfig("gemini")

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found in configuration")
}

func TestIsUserAuthorized_WhitelistEnabled_UserInList_ReturnsTrue(t *testing.T) {
	config := &Config{
		Security: SecurityConfig{
			WhitelistEnabled: true,
			AllowedUsers: map[string][]string{
				"discord": {"123456789012345678", "987654321098765432"},
			},
		},
	}

	result := config.IsUserAuthorized("discord", "123456789012345678")

	// Assert
	assert.True(t, result)
}

func TestIsUserAuthorized_WhitelistEnabled_UserNotInList_ReturnsFalse(t *testing.T) {
	config := &Config{
		Security: SecurityConfig{
			WhitelistEnabled: true,
			AllowedUsers: map[string][]string{
				"discord": {"123456789012345678"},
			},
		},
	}

	result := config.IsUserAuthorized("discord", "999999999999999999")

	// Assert
	assert.False(t, result)
}

func TestIsUserAuthorized_WhitelistDisabled_AllowsAllUsers(t *testing.T) {
	config := &Config{
		Security: SecurityConfig{
			WhitelistEnabled: false,
			AllowedUsers:     map[string][]string{},
		},
	}

	result := config.IsUserAuthorized("discord", "any-user-id")

	// Assert
	assert.True(t, result)
}

func TestIsAdmin_UserIsAdmin_ReturnsTrue(t *testing.T) {
	config := &Config{
		Security: SecurityConfig{
			Admins: map[string][]string{
				"discord": {"123456789012345678"},
			},
		},
	}

	result := config.IsAdmin("discord", "123456789012345678")

	// Assert
	assert.True(t, result)
}

func TestIsAdmin_UserIsNotAdmin_ReturnsFalse(t *testing.T) {
	config := &Config{
		Security: SecurityConfig{
			Admins: map[string][]string{
				"discord": {"123456789012345678"},
			},
		},
	}

	result := config.IsAdmin("discord", "999999999999999999")

	// Assert
	assert.False(t, result)
}

func TestGetSessionConfig_ValidSession_ReturnsConfig(t *testing.T) {
	config := &Config{
		Sessions: []SessionConfig{
			{
				Name:      "test-session",
				CLIType:   "claude",
				WorkDir:   "/tmp/test",
				AutoStart: false,
			},
		},
	}

	sessionConfig, err := config.GetSessionConfig("test-session")

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, "test-session", sessionConfig.Name)
	assert.Equal(t, "claude", sessionConfig.CLIType)
	assert.Equal(t, "/tmp/test", sessionConfig.WorkDir)
}

func TestGetSessionConfig_NonExistentSession_ReturnsError(t *testing.T) {
	config := &Config{
		Sessions: []SessionConfig{},
	}

	_, err := config.GetSessionConfig("non-existent")

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found in configuration")
}

func TestValidateConfig_MissingBot_ReturnsError(t *testing.T) {
	configContent := `
hook_server:
  port: 8080

security:
  whitelist_enabled: false
sessions:
  - name: "test-session"
    cli_type: "claude"
    work_dir: "/tmp/test"
    auto_start: false
bots: {}
cli_adapters:
  claude:
    history_dir: "~/.claude/conversations"
    interactive:
      enabled: true
      check_lines: 3
      patterns:
        - "\\? [y/N]"
`
	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(configContent); err != nil {
		t.Fatalf("Failed to write config content: %v", err)
	}
	tmpFile.Close()

	// Load config
	_, err = LoadConfig(tmpFile.Name())

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "at least one bot must be configured")
}

func TestValidateConfig_MissingSession_ReturnsError(t *testing.T) {
	configContent := `
hook_server:
  port: 8080

security:
  whitelist_enabled: false
sessions: []
bots:
  discord:
    enabled: true
    token: "test-token"
cli_adapters:
  claude:
    history_dir: "~/.claude/conversations"
    interactive:
      enabled: true
      check_lines: 3
      patterns:
        - "\\? [y/N]"
`
	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(configContent); err != nil {
		t.Fatalf("Failed to write config content: %v", err)
	}
	tmpFile.Close()

	// Load config
	_, err = LoadConfig(tmpFile.Name())

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "at least one session must be configured")
}

func TestExpandEnv_UndefinedVariable_ReturnsError(t *testing.T) {
	input := "token: ${UNDEFINED_VAR}"
	_, err := expandEnv(input)

	// Assert - undefined variables should return error
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing required environment variables")
}

func TestExpandEnv_MultipleVariables_ExpandsAll(t *testing.T) {
	// Set environment variables
	os.Setenv("VAR1", "value1")
	os.Setenv("VAR2", "value2")
	defer os.Unsetenv("VAR1")
	defer os.Unsetenv("VAR2")

	input := "${VAR1}/${VAR2}"
	result, err := expandEnv(input)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, "value1/value2", result)
}

func TestExpandHome_RelativePath_ReturnsUnchanged(t *testing.T) {
	// Test with a relative path that doesn't start with ~
	path := "./relative/path"
	result, err := expandHome(path)

	// Assert - should return unchanged
	assert.NoError(t, err)
	assert.Equal(t, "./relative/path", result)
}

func TestExpandHome_AbsolutePath_ReturnsUnchanged(t *testing.T) {
	// Test with an absolute path
	path := "/absolute/path/to/file"
	result, err := expandHome(path)

	// Assert - should return unchanged
	assert.NoError(t, err)
	assert.Equal(t, "/absolute/path/to/file", result)
}

