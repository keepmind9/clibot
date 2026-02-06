package watchdog

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestCanSkipUIBorders tests that canSkip correctly identifies UI border lines
func TestCanSkipUIBorders(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		expected bool
	}{
		{"empty line", "", true},
		{"single dash border", "────", true},
		{"single dash border long", "──────────────────────────────────────────────────────────", true},
		{"double dash border", "═════", true},
		{"double dash border long", "═════════════════════════════════════════════════════════", true},
		{"mixed borders", "─┌┐", true},
		{"text with border prefix", "─── text", false}, // contains text, should not skip
		{"normal text", "Here is the response", false},
		{"thinking with dashes", "Thinking────────", false}, // contains text, should not skip
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := canSkip(tt.line)
			assert.Equal(t, tt.expected, result, "canSkip(%q) = %v, want %v", tt.line, result, tt.expected)
		})
	}
}
