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
		{"text with border prefix", "─── text", false},  // 包含文本，不应该跳过
		{"normal text", "Here is the response", false},
		{"thinking with dashes", "Thinking────────", false}, // 包含文本，不应该跳过
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := canSkip(tt.line)
			assert.Equal(t, tt.expected, result, "canSkip(%q) = %v, want %v", tt.line, result, tt.expected)
		})
	}
}
