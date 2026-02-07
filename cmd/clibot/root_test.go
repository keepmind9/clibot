package main

import (
	"os"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestRootCommand_Properties(t *testing.T) {
	// Test root command properties
	assert.NotNil(t, rootCmd)
	assert.Equal(t, "clibot", rootCmd.Use)
	assert.NotEmpty(t, rootCmd.Short)
	assert.Contains(t, rootCmd.Short, "middleware")
}

func TestRootCommand_HasSubcommands(t *testing.T) {
	// Init commands to register subcommands
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	// Execute with help to init commands
	os.Args = []string{"clibot", "--help"}
	rootCmd.Execute()

	// Check that expected subcommands are registered
	expectedCommands := []string{
		"serve",
		"hook",
		"status",
		"validate",
		"version",
	}

	subcommands := rootCmd.Commands()
	subcommandNames := make(map[string]bool)
	for _, cmd := range subcommands {
		subcommandNames[cmd.Name()] = true
	}

	for _, expected := range expectedCommands {
		assert.True(t, subcommandNames[expected], "missing subcommand: %s", expected)
	}
}

func TestVersionCommand_Exists(t *testing.T) {
	// Init commands to register subcommands
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"clibot", "--help"}
	rootCmd.Execute()

	// Find version command
	subcommands := rootCmd.Commands()
	var found bool
	for _, cmd := range subcommands {
		if cmd.Name() == "version" {
			found = true
			assert.NotEmpty(t, cmd.Short)
			break
		}
	}
	assert.True(t, found, "version command not found")
}

func TestValidateCommand_Exists(t *testing.T) {
	// Init commands to register subcommands
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"clibot", "--help"}
	rootCmd.Execute()

	// Find validate command
	subcommands := rootCmd.Commands()
	var found bool
	for _, cmd := range subcommands {
		if cmd.Name() == "validate" {
			found = true
			assert.NotEmpty(t, cmd.Short)
			break
		}
	}
	assert.True(t, found, "validate command not found")
}

func TestStatusCommand_Exists(t *testing.T) {
	// Init commands to register subcommands
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"clibot", "--help"}
	rootCmd.Execute()

	// Find status command
	subcommands := rootCmd.Commands()
	var found bool
	for _, cmd := range subcommands {
		if cmd.Name() == "status" {
			found = true
			assert.NotEmpty(t, cmd.Short)
			break
		}
	}
	assert.True(t, found, "status command not found")
}

func TestHookCommand_HasCliTypeFlag(t *testing.T) {
	// Init commands to register subcommands
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"clibot", "--help"}
	rootCmd.Execute()

	// Find hook command
	subcommands := rootCmd.Commands()
	var hookCmd *cobra.Command
	for _, cmd := range subcommands {
		if cmd.Name() == "hook" {
			hookCmd = cmd
			break
		}
	}

	if hookCmd != nil {
		// Test that hook command has cli-type flag
		flag := hookCmd.Flags().Lookup("cli-type")
		assert.NotNil(t, flag, "hook command should have cli-type flag")
	}
}
