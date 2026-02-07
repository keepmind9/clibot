package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestIsValidSessionName_ValidNames tests valid session names
func TestIsValidSessionName_ValidNames(t *testing.T) {
	validNames := []string{
		"session",
		"my-session",
		"my_session",
		"session123",
		"123session",
		"a",
		"test-session-123",
		"UPPERCASE",
		"MixedCase",
	}

	for _, name := range validNames {
		t.Run(name, func(t *testing.T) {
			assert.True(t, isValidSessionName(name), "%s should be valid", name)
		})
	}
}

// TestIsValidSessionName_InvalidNames tests invalid session names
func TestIsValidSessionName_InvalidNames(t *testing.T) {
	invalidNames := []string{
		"",
		"session with spaces",
		"session/with/slash",
		"session\\with\\backslash",
		"session.with.dots",
		"session@with@special",
		"session:with:colon",
		"session;with;semicolon",
	}

	for _, name := range invalidNames {
		t.Run(name, func(t *testing.T) {
			assert.False(t, isValidSessionName(name), "%s should be invalid", name)
		})
	}
}

// TestIsValidSessionName_LengthLimits tests length restrictions
func TestIsValidSessionName_LengthLimits(t *testing.T) {
	t.Run("exactly 100 characters is valid", func(t *testing.T) {
		// Create a 100-character string
		name := ""
		for i := 0; i < 100; i++ {
			name += "a"
		}
		assert.True(t, isValidSessionName(name))
	})

	t.Run("101 characters is invalid", func(t *testing.T) {
		// Create a 101-character string
		name := ""
		for i := 0; i < 101; i++ {
			name += "a"
		}
		assert.False(t, isValidSessionName(name))
	})
}
