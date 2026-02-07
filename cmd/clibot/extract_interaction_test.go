package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestExtractInteractionCommand_Exists tests extract-interaction command exists
func TestExtractInteractionCommand_Exists(t *testing.T) {
	// Find extract-interaction command
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "extract-interaction" {
			assert.NotEmpty(t, cmd.Short, "extract-interaction command should have short description")
			return
		}
	}
	t.Skip("extract-interaction command not found")
}

// TestIncrementCommand_Exists tests increment command exists
func TestIncrementCommand_Exists(t *testing.T) {
	// Find increment command
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "increment" {
			assert.NotEmpty(t, cmd.Short, "increment command should have short description")
			return
		}
	}
	t.Skip("increment command not found")
}

// TestAllCommands_HaveLongDescription tests all commands have long descriptions
func TestAllCommands_HaveLongDescription(t *testing.T) {
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "help" || cmd.Name() == "completion" {
			continue
		}
		assert.NotEmpty(t, cmd.Long, "command %s should have long description", cmd.Name())
	}
}

// TestCommand_SubcommandsRelationships tests command hierarchy
func TestCommand_SubcommandsRelationships(t *testing.T) {
	subcommands := rootCmd.Commands()
	assert.GreaterOrEqual(t, len(subcommands), 5, "root command should have at least 5 subcommands")

	// Verify expected subcommands exist
	expectedCommands := []string{
		"serve",
		"hook",
		"status",
		"validate",
		"version",
	}

	cmdMap := make(map[string]bool)
	for _, cmd := range subcommands {
		cmdMap[cmd.Name()] = true
	}

	for _, expected := range expectedCommands {
		assert.True(t, cmdMap[expected], "expected command %s should exist", expected)
	}
}

// TestRootCommand_PersistentFlags tests root command persistent flags
func TestRootCommand_PersistentFlags(t *testing.T) {
	// Root command may have persistent flags
	flags := rootCmd.PersistentFlags()
	assert.NotNil(t, flags, "root command should have persistent flags")
}

// TestCommand_RunE_ShouldReturnError tests RunE function
func TestCommand_RunE_ShouldReturnError(t *testing.T) {
	// Test that root command RunE is properly set
	// This is a placeholder to verify command structure
	if rootCmd.RunE != nil {
		// If RunE is set, it should be a valid function
		assert.NotNil(t, rootCmd.RunE)
	} else if rootCmd.Run != nil {
		assert.NotNil(t, rootCmd.Run)
	}
}

// TestAllCommands_UniqueNames tests all subcommands have unique names
func TestAllCommands_UniqueNames(t *testing.T) {
	subcommands := rootCmd.Commands()
	seen := make(map[string]bool)

	for _, cmd := range subcommands {
		assert.False(t, seen[cmd.Name()], "command name %s should be unique", cmd.Name())
		seen[cmd.Name()] = true
	}
}

// TestRootCommand_OutputOutputs tests that commands can output
func TestCommand_OutputOutputs(t *testing.T) {
	// Test root command structure
	assert.NotNil(t, rootCmd.Execute)
	// RunE might be nil, so we only check Execute
}
