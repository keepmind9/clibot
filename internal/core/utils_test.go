package core

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestExpandPath tests the expandPath function
func TestExpandPath(t *testing.T) {
	t.Run("expand tilde with path", func(t *testing.T) {
		home, err := os.UserHomeDir()
		require.NoError(t, err)

		result, err := expandPath("~/test/path")
		assert.NoError(t, err)
		expected := filepath.Join(home, "test/path")
		assert.Equal(t, expected, result)
	})

	t.Run("expand tilde only", func(t *testing.T) {
		home, err := os.UserHomeDir()
		require.NoError(t, err)

		result, err := expandPath("~")
		assert.NoError(t, err)
		assert.Equal(t, home, result)
	})

	t.Run("absolute path unchanged", func(t *testing.T) {
		result, err := expandPath("/absolute/path")
		assert.NoError(t, err)
		assert.Equal(t, "/absolute/path", result)
	})

	t.Run("relative path unchanged", func(t *testing.T) {
		result, err := expandPath("relative/path")
		assert.NoError(t, err)
		assert.Equal(t, "relative/path", result)
	})

	t.Run("empty path", func(t *testing.T) {
		result, err := expandPath("")
		assert.NoError(t, err)
		assert.Equal(t, "", result)
	})
}

// TestNormalizePath tests the normalizePath function
func TestNormalizePath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "path with trailing slash",
			input:    "/path/to/dir/",
			expected: "/path/to/dir",
		},
		{
			name:     "path without trailing slash",
			input:    "/path/to/dir",
			expected: "/path/to/dir",
		},
		{
			name:     "path with multiple trailing slashes",
			input:    "/path/to/dir///",
			expected: "/path/to/dir//",
		},
		{
			name:     "root path",
			input:    "/",
			expected: "",
		},
		{
			name:     "empty path",
			input:    "",
			expected: "",
		},
		{
			name:     "relative path with trailing slash",
			input:    "relative/path/",
			expected: "relative/path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizePath(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestGetUserKey tests the getUserKey function
func TestGetUserKey(t *testing.T) {
	tests := []struct {
		name     string
		platform string
		userID   string
		expected string
	}{
		{
			name:     "discord user",
			platform: "discord",
			userID:   "123456789",
			expected: "discord:123456789",
		},
		{
			name:     "telegram user",
			platform: "telegram",
			userID:   "987654321",
			expected: "telegram:987654321",
		},
		{
			name:     "feishu user",
			platform: "feishu",
			userID:   "abc123",
			expected: "feishu:abc123",
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


// TestSetServerDefaults tests the setServerDefaults function
func TestSetServerDefaults(t *testing.T) {
	t.Run("sets default port when zero", func(t *testing.T) {
		config := &Config{
			HookServer: HookServerConfig{
				Port: 0,
			},
		}

		setServerDefaults(config)

		assert.Equal(t, DefaultHookPort, config.HookServer.Port)
	})

	t.Run("preserves existing port", func(t *testing.T) {
		config := &Config{
			HookServer: HookServerConfig{
				Port: 9090,
			},
		}

		setServerDefaults(config)

		assert.Equal(t, 9090, config.HookServer.Port)
	})
}

// TestSetSessionDefaults tests the setSessionDefaults function
func TestSetSessionDefaults(t *testing.T) {
	t.Run("sets default input history size when zero", func(t *testing.T) {
		config := &Config{
			Session: SessionGlobalConfig{
				InputHistorySize: 0,
			},
		}

		err := setSessionDefaults(config)

		assert.NoError(t, err)
		assert.Equal(t, DefaultInputHistorySize, config.Session.InputHistorySize)
	})

	t.Run("validates minimum input history size", func(t *testing.T) {
		config := &Config{
			Session: SessionGlobalConfig{
				InputHistorySize: 0,
			},
		}

		err := setSessionDefaults(config)
		assert.NoError(t, err)
		assert.Equal(t, DefaultInputHistorySize, config.Session.InputHistorySize)
	})

	t.Run("rejects input history size too small", func(t *testing.T) {
		config := &Config{
			Session: SessionGlobalConfig{
				InputHistorySize: 0,
			},
		}

		// First set to valid value
		err := setSessionDefaults(config)
		require.NoError(t, err)

		// Try to set to invalid value
		config.Session.InputHistorySize = 0
		err = setSessionDefaults(config)

		// Should set to default, not error
		assert.NoError(t, err)
		assert.Equal(t, DefaultInputHistorySize, config.Session.InputHistorySize)
	})

	t.Run("rejects input history size too large", func(t *testing.T) {
		config := &Config{
			Session: SessionGlobalConfig{
				InputHistorySize: 101,
			},
		}

		err := setSessionDefaults(config)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must be between 1 and 100")
	})
}

// TestValidatePollingConfig tests the validatePollingConfig function
func TestValidatePollingConfig(t *testing.T) {
	tests := []struct {
		name        string
		cliType     string
		config      CLIAdapterConfig
		expectError bool
		errorMsg    string
	}{
		{
			name:    "valid polling config",
			cliType: "claude",
			config: CLIAdapterConfig{
				PollInterval: "1s",
				PollTimeout:  "1h",
				StableCount:  3,
			},
			expectError: false,
		},
		{
			name:    "valid polling config with minimum interval",
			cliType: "claude",
			config: CLIAdapterConfig{
				PollInterval: "100ms",
				PollTimeout:  "1s",
				StableCount:  1,
			},
			expectError: false,
		},
		{
			name:    "invalid poll interval too small",
			cliType: "claude",
			config: CLIAdapterConfig{
				PollInterval: "50ms",
				PollTimeout:  "1h",
				StableCount:  3,
			},
			expectError: true,
			errorMsg:    "must be at least 100ms",
		},
		{
			name:    "invalid poll interval too large",
			cliType: "claude",
			config: CLIAdapterConfig{
				PollInterval: "70s",
				PollTimeout:  "2h",
				StableCount:  3,
			},
			expectError: true,
			errorMsg:    "is too large (max 60s",
		},
		{
			name:    "invalid poll timeout less than interval",
			cliType: "claude",
			config: CLIAdapterConfig{
				PollInterval: "10s",
				PollTimeout:  "5s",
				StableCount:  3,
			},
			expectError: true,
			errorMsg:    "must be greater than poll_interval",
		},
		{
			name:    "invalid poll timeout too large",
			cliType: "claude",
			config: CLIAdapterConfig{
				PollInterval: "1s",
				PollTimeout:  "3h",
				StableCount:  3,
			},
			expectError: true,
			errorMsg:    "is too large (max 2h",
		},
		{
			name:    "invalid stable count too small",
			cliType: "claude",
			config: CLIAdapterConfig{
				PollInterval: "1s",
				PollTimeout:  "1h",
				StableCount:  0,
			},
			expectError: true,
			errorMsg:    "must be between 1 and 20",
		},
		{
			name:    "invalid stable count too large",
			cliType: "claude",
			config: CLIAdapterConfig{
				PollInterval: "1s",
				PollTimeout:  "1h",
				StableCount:  25,
			},
			expectError: true,
			errorMsg:    "must be between 1 and 20",
		},
		{
			name:    "timeout less than minimum required",
			cliType: "claude",
			config: CLIAdapterConfig{
				PollInterval: "10s",
				PollTimeout:  "11s",
				StableCount:  10,
			},
			expectError: true,
			errorMsg:    "must be at least",
		},
		{
			name:    "invalid poll interval format",
			cliType: "claude",
			config: CLIAdapterConfig{
				PollInterval: "invalid",
				PollTimeout:  "1h",
				StableCount:  3,
			},
			expectError: true,
			errorMsg:    "invalid poll_interval",
		},
		{
			name:    "invalid poll timeout format",
			cliType: "claude",
			config: CLIAdapterConfig{
				PollInterval: "1s",
				PollTimeout:  "invalid",
				StableCount:  3,
			},
			expectError: true,
			errorMsg:    "invalid poll_timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePollingConfig(tt.cliType, tt.config)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestValidateSecuritySettings tests the validateSecuritySettings function
func TestValidateSecuritySettings(t *testing.T) {
	t.Run("whitelist enabled with allowed users", func(t *testing.T) {
		config := &Config{
			Security: SecurityConfig{
				WhitelistEnabled: true,
				AllowedUsers: map[string][]string{
					"discord": {"123456789"},
				},
			},
		}

		err := validateSecuritySettings(config)
		assert.NoError(t, err)
	})

	t.Run("whitelist enabled without allowed users", func(t *testing.T) {
		config := &Config{
			Security: SecurityConfig{
				WhitelistEnabled: true,
				AllowedUsers:     map[string][]string{},
			},
		}

		err := validateSecuritySettings(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be empty when whitelist is enabled")
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

	t.Run("whitelist disabled with nil allowed users", func(t *testing.T) {
		config := &Config{
			Security: SecurityConfig{
				WhitelistEnabled: false,
			},
		}

		err := validateSecuritySettings(config)
		assert.NoError(t, err)
	})
}

// TestExpandHome tests the expandHome function
func TestExpandHome(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	t.Run("expand tilde with path", func(t *testing.T) {
		home, err := os.UserHomeDir()
		require.NoError(t, err)

		result, err := expandHome("~/test/path")
		assert.NoError(t, err)
		expected := home + "/test/path"
		assert.Equal(t, expected, result)
	})

	t.Run("path without tilde unchanged", func(t *testing.T) {
		result, err := expandHome("/absolute/path")
		assert.NoError(t, err)
		assert.Equal(t, "/absolute/path", result)
	})

	t.Run("relative path unchanged", func(t *testing.T) {
		result, err := expandHome("relative/path")
		assert.NoError(t, err)
		assert.Equal(t, "relative/path", result)
	})

	t.Run("empty path", func(t *testing.T) {
		result, err := expandHome("")
		assert.NoError(t, err)
		assert.Equal(t, "", result)
	})

	t.Run("path starting with tilde only returns unchanged", func(t *testing.T) {
		// expandHome only expands paths starting with "~/", not "~" alone
		result, err := expandHome("~")
		assert.NoError(t, err)
		assert.Equal(t, "~", result)
	})
}
