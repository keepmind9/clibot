package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIncrementCommandFlags tests increment command flags
func TestIncrementCommandFlags(t *testing.T) {
	// Find increment command
	var found bool
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "increment" {
			found = true

			// Test that before flag exists
			flag := cmd.Flags().Lookup("before")
			assert.NotNil(t, flag, "increment command should have before flag")

			// Test that after flag exists
			flag = cmd.Flags().Lookup("after")
			assert.NotNil(t, flag, "increment command should have after flag")

			// Verify short description
			assert.NotEmpty(t, cmd.Short, "increment command should have short description")
			break
		}
	}
	assert.True(t, found, "increment command should exist")
}

// TestIncrementCommandExecution tests increment command execution
func TestIncrementCommandExecution(t *testing.T) {
	// Find increment command
	var incrementCmd *cobra.Command
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "increment" {
			incrementCmd = cmd
			break
		}
	}

	if incrementCmd == nil {
		t.Skip("increment command not found")
	}

	t.Run("missing flags returns error", func(t *testing.T) {
		// Execute command without flags
		err := incrementCmd.RunE(incrementCmd, []string{})
		assert.Error(t, err)
	})

	t.Run("non-existent files return error", func(t *testing.T) {
		// Set flags with non-existent files
		incrementCmd.Flags().Set("before", "/nonexistent/before.txt")
		incrementCmd.Flags().Set("after", "/nonexistent/after.txt")

		// Execute command
		err := incrementCmd.RunE(incrementCmd, []string{})
		assert.Error(t, err)
	})

	t.Run("valid files execute successfully", func(t *testing.T) {
		tmpDir := t.TempDir()
		beforeFile := filepath.Join(tmpDir, "before.txt")
		afterFile := filepath.Join(tmpDir, "after.txt")

		err := os.WriteFile(beforeFile, []byte("before content"), 0644)
		require.NoError(t, err)
		err = os.WriteFile(afterFile, []byte("after content"), 0644)
		require.NoError(t, err)

		// Set flags
		incrementCmd.Flags().Set("before", beforeFile)
		incrementCmd.Flags().Set("after", afterFile)

		// Execute command
		err = incrementCmd.RunE(incrementCmd, []string{})
		assert.NoError(t, err)
	})
}

// TestExtractInteractionCommandFlags tests extract-interaction command flags
func TestExtractInteractionCommandFlags(t *testing.T) {
	// Find extract-interaction command
	var found bool
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "extract-interaction" {
			found = true

			// Verify short description
			assert.NotEmpty(t, cmd.Short, "extract-interaction command should have short description")
			break
		}
	}
	assert.True(t, found, "extract-interaction command should exist")
}

// TestCompletionCommandExists tests completion command exists
func TestCompletionCommandExists(t *testing.T) {
	// Find completion command (may not exist in all versions)
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "completion" {
			assert.NotEmpty(t, cmd.Short, "completion command should have short description")
			return
		}
	}
	// completion command is provided by cobra, so it's ok if not found
	t.Skip("completion command not found (may be provided by cobra)")
}
