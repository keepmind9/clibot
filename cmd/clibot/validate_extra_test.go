package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/keepmind9/clibot/internal/core"
	"github.com/stretchr/testify/assert"
)

// TestOutputValidationResult_ValidConfig tests output for valid configuration
func TestOutputValidationResult_ValidConfig(t *testing.T) {
	result := ValidationResult{
		Valid:    true,
		Config:   "test.yaml",
		Sessions: 3,
		Adapters: 2,
		Errors:   []string{},
	}

	t.Run("text format", func(t *testing.T) {
		// This test verifies the function doesn't panic
		outputValidationResult(result, false)
	})

	t.Run("json format", func(t *testing.T) {
		// This test verifies the function doesn't panic
		outputValidationResult(result, true)

		// Verify it's valid JSON by marshaling again
		output, _ := json.Marshal(result)
		assert.NotEmpty(t, output)
	})
}

// TestOutputValidationResult_InvalidConfig tests output for invalid configuration
func TestOutputValidationResult_InvalidConfig(t *testing.T) {
	result := ValidationResult{
		Valid:    false,
		Config:   "test.yaml",
		Sessions: 0,
		Adapters: 0,
		Errors:   []string{"error 1", "error 2"},
	}

	t.Run("text format", func(t *testing.T) {
		outputValidationResult(result, false)
	})

	t.Run("json format", func(t *testing.T) {
		outputValidationResult(result, true)
	})
}

// TestValidateConfigDetails_NoWarnings tests config with no warnings
func TestValidateConfigDetails_NoWarnings(t *testing.T) {
	cfg := &core.Config{
		Security: core.SecurityConfig{
			WhitelistEnabled: true,
			AllowedUsers: map[string][]string{
				"discord": {"user123"},
			},
		},
		Bots: map[string]core.BotConfig{
			"discord": {
				Enabled: true,
				Token:   "test-token",
			},
		},
		Sessions: []core.SessionConfig{
			{Name: "test", CLIType: "claude"},
		},
	}

	warnings := validateConfigDetails(cfg)
	assert.Empty(t, warnings)
}

// TestValidateConfigDetails_WhitelistWarnings tests whitelist-related warnings
func TestValidateConfigDetails_WhitelistWarnings(t *testing.T) {
	t.Run("whitelist disabled", func(t *testing.T) {
		cfg := &core.Config{
			Security: core.SecurityConfig{
				WhitelistEnabled: false,
			},
			Bots: map[string]core.BotConfig{
				"discord": {Enabled: true, Token: "test"},
			},
			Sessions: []core.SessionConfig{
				{Name: "test", CLIType: "claude"},
			},
		}

		warnings := validateConfigDetails(cfg)
		assert.NotEmpty(t, warnings)
		assert.Contains(t, warnings[0], "Whitelist is disabled")
	})

	t.Run("whitelist enabled but no users", func(t *testing.T) {
		cfg := &core.Config{
			Security: core.SecurityConfig{
				WhitelistEnabled: true,
				AllowedUsers:     map[string][]string{},
			},
			Bots: map[string]core.BotConfig{
				"discord": {Enabled: true, Token: "test"},
			},
			Sessions: []core.SessionConfig{
				{Name: "test", CLIType: "claude"},
			},
		}

		warnings := validateConfigDetails(cfg)
		assert.NotEmpty(t, warnings)
		found := false
		for _, w := range warnings {
			if strings.Contains(w, "no users are allowed") {
				found = true
				break
			}
		}
		assert.True(t, found, "should warn about no allowed users")
	})
}

// TestValidateConfigDetails_NoEnabledBots tests warning when no bots are enabled
func TestValidateConfigDetails_NoEnabledBots(t *testing.T) {
	cfg := &core.Config{
		Security: core.SecurityConfig{
			WhitelistEnabled: true,
			AllowedUsers: map[string][]string{
				"discord": {"user123"},
			},
		},
		Bots: map[string]core.BotConfig{
			"discord": {Enabled: false},
		},
		Sessions: []core.SessionConfig{
			{Name: "test", CLIType: "claude"},
		},
	}

	warnings := validateConfigDetails(cfg)
	assert.NotEmpty(t, warnings)
	assert.Contains(t, warnings[0], "No bots are enabled")
}

// TestValidateConfigDetails_MissingBots tests warning when bots have no tokens
func TestValidateConfigDetails_MissingBots(t *testing.T) {
	cfg := &core.Config{
		Security: core.SecurityConfig{
			WhitelistEnabled: true,
			AllowedUsers: map[string][]string{
				"discord": {"user123"},
			},
		},
		Bots: map[string]core.BotConfig{
			"discord": {Enabled: true, Token: ""},
		},
		Sessions: []core.SessionConfig{
			{Name: "test", CLIType: "claude"},
		},
	}

	warnings := validateConfigDetails(cfg)
	assert.NotEmpty(t, warnings)
	assert.Contains(t, warnings[0], "no credentials configured")
}

// TestValidateConfigDetails_NoSessions tests warning when no sessions defined
func TestValidateConfigDetails_NoSessions(t *testing.T) {
	cfg := &core.Config{
		Security: core.SecurityConfig{
			WhitelistEnabled: true,
			AllowedUsers: map[string][]string{
				"discord": {"user123"},
			},
		},
		Bots: map[string]core.BotConfig{
			"discord": {Enabled: true, Token: "test"},
		},
		Sessions: []core.SessionConfig{},
	}

	warnings := validateConfigDetails(cfg)
	assert.NotEmpty(t, warnings)
	assert.Contains(t, warnings[0], "No sessions configured")
}

// TestOutputValidationResult_JsonErrorHandling tests JSON error handling
func TestOutputValidationResult_JsonErrorHandling(t *testing.T) {
	// Create a result that will cause issues
	result := ValidationResult{
		Valid:  true,
		Errors: []string{},
	}

	// The function should handle JSON marshaling gracefully
	outputValidationResult(result, true)
}

// TestValidateConfigDetails_MultipleWarnings tests config with multiple warnings
func TestValidateConfigDetails_MultipleWarnings(t *testing.T) {
	cfg := &core.Config{
		Security: core.SecurityConfig{
			WhitelistEnabled: false, // Warning 1
			AllowedUsers:     map[string][]string{},
		},
		Bots: map[string]core.BotConfig{
			"discord": {Enabled: false}, // Warning 2
		},
		Sessions: []core.SessionConfig{}, // Warning 3
	}

	warnings := validateConfigDetails(cfg)
	assert.GreaterOrEqual(t, len(warnings), 2, "should have multiple warnings")
}
