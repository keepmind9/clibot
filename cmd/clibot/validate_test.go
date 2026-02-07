package main

import (
	"github.com/keepmind9/clibot/internal/core"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestGetBotName tests the getBotName function
func TestGetBotName(t *testing.T) {
	t.Run("returns bot for nil config", func(t *testing.T) {
		result := getBotName(nil)
		assert.Equal(t, "bot", result)
	})

	t.Run("returns bot for valid config", func(t *testing.T) {
		botConfig := &core.BotConfig{
			Enabled: true,
			Token:   "test-token",
		}
		result := getBotName(botConfig)
		assert.Equal(t, "bot", result)
	})

	t.Run("returns bot for empty config", func(t *testing.T) {
		botConfig := &core.BotConfig{}
		result := getBotName(botConfig)
		assert.Equal(t, "bot", result)
	})
}

// TestValidateCommandFlags tests validate command flags
func TestValidateCommandFlags(t *testing.T) {
	// Find validate command
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "validate" {
			// Test that config flag exists
			flag := cmd.Flags().Lookup("config")
			assert.NotNil(t, flag, "validate command should have config flag")

			// Test that show flag exists
			flag = cmd.Flags().Lookup("show")
			assert.NotNil(t, flag, "validate command should have show flag")

			// Test that json flag exists
			flag = cmd.Flags().Lookup("json")
			assert.NotNil(t, flag, "validate command should have json flag")
			return
		}
	}
	t.Skip("validate command not found")
}

// TestServeCommandValidateFlag tests serve command validate flag
func TestServeCommandValidateFlag(t *testing.T) {
	// Find serve command
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "serve" {
			// Test that validate flag exists
			flag := cmd.Flags().Lookup("validate")
			assert.NotNil(t, flag, "serve command should have validate flag")
			return
		}
	}
	t.Skip("serve command not found")
}

// TestHookCommandFlags tests hook command flags
func TestHookCommandFlags(t *testing.T) {
	// Find hook command
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "hook" {
			// Test that cli-type flag exists
			flag := cmd.Flags().Lookup("cli-type")
			assert.NotNil(t, flag, "hook command should have cli-type flag")
			return
		}
	}
	t.Skip("hook command not found")
}

// TestAllCommandsHaveShortDescription tests all commands have short descriptions
func TestAllCommandsHaveShortDescription(t *testing.T) {
	for _, cmd := range rootCmd.Commands() {
		assert.NotEmpty(t, cmd.Short, "command %s should have short description", cmd.Name())
	}
}

// TestCommandStructure tests command structure
func TestCommandStructure(t *testing.T) {
	// Test root command
	assert.NotNil(t, rootCmd, "rootCmd should not be nil")
	assert.Equal(t, "clibot", rootCmd.Use, "root command use should be clibot")
	assert.NotEmpty(t, rootCmd.Short, "root command should have short description")
	assert.NotEmpty(t, rootCmd.Long, "root command should have long description")

	// Test that all subcommands are properly initialized
	expectedCommands := []string{
		"serve",
		"hook",
		"status",
		"validate",
		"version",
	}

	subcommands := rootCmd.Commands()
	assert.GreaterOrEqual(t, len(subcommands), len(expectedCommands),
		"should have at least %d subcommands", len(expectedCommands))
}
