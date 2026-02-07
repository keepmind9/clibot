package core

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestValidateSessionName_ValidNames tests valid session names
func TestValidateSessionName_ValidNames(t *testing.T) {
	validNames := []string{
		"session",
		"my-session",
		"my_session",
		"session123",
		"test",
		"abc",
	}

	for _, name := range validNames {
		t.Run(name, func(t *testing.T) {
			err := validateSessionName(name)
			assert.NoError(t, err, "%s should be valid", name)
		})
	}
}

// TestValidateSessionName_LengthValidation tests session name length restrictions
func TestValidateSessionName_LengthValidation(t *testing.T) {
	t.Run("empty string is invalid", func(t *testing.T) {
		err := validateSessionName("")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid session name length")
	})

	t.Run("exactly min length (1)", func(t *testing.T) {
		// minSessionNameLength is 1
		err := validateSessionName("a")
		// Note: "a" may fail format check, but should pass length check
		if err != nil {
			assert.NotContains(t, err.Error(), "invalid session name length")
		}
	})

	t.Run("exactly max length (100)", func(t *testing.T) {
		// maxSessionNameLength is 100
		name := ""
		for i := 0; i < 100; i++ {
			name += "a"
		}
		err := validateSessionName(name)
		assert.NoError(t, err)
	})

	t.Run("too long - more than max length", func(t *testing.T) {
		// maxSessionNameLength is 100
		name := ""
		for i := 0; i < 101; i++ {
			name += "a"
		}
		err := validateSessionName(name)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid session name length")
	})
}

// TestValidateSessionName_PathTraversalTests tests path traversal detection
func TestValidateSessionName_PathTraversalTests(t *testing.T) {
	t.Run("double dot", func(t *testing.T) {
		err := validateSessionName("..")
		assert.Error(t, err)
		// Format check comes before path traversal, but ".." should fail either way
		assert.Error(t, err)
	})

	t.Run("double dot with characters", func(t *testing.T) {
		err := validateSessionName("../session")
		assert.Error(t, err)
		// Should fail on path traversal
		assert.Contains(t, err.Error(), "path traversal detected")
	})

	t.Run("forward slash", func(t *testing.T) {
		err := validateSessionName("session/with/slash")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "path traversal detected")
	})

	t.Run("backslash", func(t *testing.T) {
		err := validateSessionName("session\\with\\backslash")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "path traversal detected")
	})

	t.Run("double dot in middle", func(t *testing.T) {
		err := validateSessionName("session../test")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "path traversal detected")
	})
}

// TestValidateCLIType_ValidTypes tests valid CLI types
func TestValidateCLIType_ValidTypes(t *testing.T) {
	validTypes := []string{
		"claude",
		"gemini",
		"opencode",
		"test-cli",
		"my_cli",
		"a",
	}

	for _, cliType := range validTypes {
		t.Run(cliType, func(t *testing.T) {
			err := validateCLIType(cliType)
			assert.NoError(t, err, "%s should be valid", cliType)
		})
	}
}

// TestValidateCLIType_EmptyType tests empty CLI type
func TestValidateCLIType_EmptyType(t *testing.T) {
	err := validateCLIType("")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid CLI type length")
}

// TestValidateCLIType_LengthValidation tests CLI type length restrictions
func TestValidateCLIType_LengthValidation(t *testing.T) {
	t.Run("exactly 50 characters is valid", func(t *testing.T) {
		cliType := ""
		for i := 0; i < 50; i++ {
			cliType += "a"
		}
		err := validateCLIType(cliType)
		assert.NoError(t, err)
	})

	t.Run("51 characters is invalid", func(t *testing.T) {
		cliType := ""
		for i := 0; i < 51; i++ {
			cliType += "a"
		}
		err := validateCLIType(cliType)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid CLI type length")
	})
}

// TestValidateCLIType_PathTraversalTests tests path traversal detection in CLI type
func TestValidateCLIType_PathTraversalTests(t *testing.T) {
	invalidTypes := []string{
		"../cli",
		"cli/../test",
		"cli/with/slash",
		"cli\\with\\backslash",
		"..",
		".../cli",
	}

	for _, cliType := range invalidTypes {
		t.Run(cliType, func(t *testing.T) {
			err := validateCLIType(cliType)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "path traversal detected")
		})
	}
}

// TestExpandEnv_Expansion tests environment variable expansion
func TestExpandEnv_Expansion(t *testing.T) {
	// Set test environment variables
	require.NoError(t, os.Setenv("TEST_VAR", "test_value"))
	require.NoError(t, os.Setenv("ANOTHER_VAR", "another_value"))
	defer os.Unsetenv("TEST_VAR")
	defer os.Unsetenv("ANOTHER_VAR")

	t.Run("expand single variable", func(t *testing.T) {
		result, err := expandEnv("prefix_${TEST_VAR}_suffix")
		require.NoError(t, err)
		assert.Equal(t, "prefix_test_value_suffix", result)
	})

	t.Run("expand multiple variables", func(t *testing.T) {
		result, err := expandEnv("${TEST_VAR}_${ANOTHER_VAR}")
		require.NoError(t, err)
		assert.Equal(t, "test_value_another_value", result)
	})

	t.Run("no variables to expand", func(t *testing.T) {
		result, err := expandEnv("plain string")
		require.NoError(t, err)
		assert.Equal(t, "plain string", result)
	})

	t.Run("empty string", func(t *testing.T) {
		result, err := expandEnv("")
		require.NoError(t, err)
		assert.Equal(t, "", result)
	})

	t.Run("variable at start", func(t *testing.T) {
		result, err := expandEnv("${TEST_VAR}/path")
		require.NoError(t, err)
		assert.Equal(t, "test_value/path", result)
	})

	t.Run("variable at end", func(t *testing.T) {
		result, err := expandEnv("/path/${TEST_VAR}")
		require.NoError(t, err)
		assert.Equal(t, "/path/test_value", result)
	})
}

// TestExpandEnv_MissingVariable tests missing environment variable
func TestExpandEnv_MissingVariable(t *testing.T) {
	// Make sure this variable is not set
	os.Unsetenv("NONEXISTENT_VAR")

	result, err := expandEnv("prefix_${NONEXISTENT_VAR}_suffix")
	assert.Error(t, err)
	// The error message format is "missing required environment variables: NONEXISTENT_VAR"
	assert.Contains(t, err.Error(), "missing")
	assert.Contains(t, err.Error(), "NONEXISTENT_VAR")
	assert.Equal(t, "", result)
}

// TestValidateSecuritySettings_WhitelistValidation tests security settings validation
func TestValidateSecuritySettings_WhitelistValidation(t *testing.T) {
	t.Run("whitelist enabled with users", func(t *testing.T) {
		config := &Config{
			Security: SecurityConfig{
				WhitelistEnabled: true,
				AllowedUsers: map[string][]string{
					"discord": {"user123", "user456"},
				},
			},
		}
		err := validateSecuritySettings(config)
		assert.NoError(t, err)
	})

	t.Run("whitelist enabled without users", func(t *testing.T) {
		config := &Config{
			Security: SecurityConfig{
				WhitelistEnabled: true,
				AllowedUsers:     map[string][]string{},
			},
		}
		err := validateSecuritySettings(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "allowed_users cannot be empty")
	})

	t.Run("whitelist disabled", func(t *testing.T) {
		config := &Config{
			Security: SecurityConfig{
				WhitelistEnabled: false,
				AllowedUsers:     map[string][]string{},
			},
		}
		err := validateSecuritySettings(config)
		assert.NoError(t, err)
	})

	t.Run("whitelist disabled with users configured", func(t *testing.T) {
		config := &Config{
			Security: SecurityConfig{
				WhitelistEnabled: false,
				AllowedUsers: map[string][]string{
					"discord": {"user123"},
				},
			},
		}
		err := validateSecuritySettings(config)
		assert.NoError(t, err)
	})
}

// TestValidateBotAndSessionConfig_MissingComponents tests validation of required config
func TestValidateBotAndSessionConfig(t *testing.T) {
	t.Run("both bots and sessions present", func(t *testing.T) {
		config := &Config{
			Bots: map[string]BotConfig{
				"discord": {Enabled: true, Token: "test"},
			},
			Sessions: []SessionConfig{
				{Name: "test", CLIType: "claude"},
			},
		}
		err := validateBotAndSessionConfig(config)
		assert.NoError(t, err)
	})

	t.Run("no bots configured", func(t *testing.T) {
		config := &Config{
			Bots:     map[string]BotConfig{},
			Sessions: []SessionConfig{{Name: "test", CLIType: "claude"}},
		}
		err := validateBotAndSessionConfig(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "at least one bot")
	})

	t.Run("no sessions configured", func(t *testing.T) {
		config := &Config{
			Bots: map[string]BotConfig{
				"discord": {Enabled: true, Token: "test"},
			},
			Sessions: []SessionConfig{},
		}
		err := validateBotAndSessionConfig(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "at least one session")
	})

	t.Run("both missing", func(t *testing.T) {
		config := &Config{
			Bots:     map[string]BotConfig{},
			Sessions: []SessionConfig{},
		}
		err := validateBotAndSessionConfig(config)
		assert.Error(t, err)
	})
}

// TestValidateSessionName_EmptyString tests empty session name
func TestValidateSessionName_EmptyString(t *testing.T) {
	err := validateSessionName("")
	assert.Error(t, err)
}

// TestValidateCLIType_SpecialCharacters tests special characters in CLI type
func TestValidateCLIType_SpecialCharacters(t *testing.T) {
	// These should be valid - hyphens and underscores are allowed
	validTypes := []string{
		"my-cli",
		"my_cli",
		"my-cli_test",
		"a-b-c",
	}

	for _, cliType := range validTypes {
		t.Run(cliType, func(t *testing.T) {
			err := validateCLIType(cliType)
			assert.NoError(t, err, "%s should be valid", cliType)
		})
	}
}
