package watchdog

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestTruncateString tests the truncateString function
func TestTruncateString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected string
	}{
		{
			name:     "string shorter than max",
			input:    "short",
			maxLen:   100,
			expected: "short",
		},
		{
			name:     "string exactly max length",
			input:    "exact",
			maxLen:   5,
			expected: "exact",
		},
		{
			name:     "string longer than max",
			input:    "this is a very long string",
			maxLen:   10,
			expected: "this is a ...",
		},
		{
			name:     "empty string",
			input:    "",
			maxLen:   10,
			expected: "",
		},
		{
			name:     "zero max length",
			input:    "test",
			maxLen:   0,
			expected: "...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateString(tt.input, tt.maxLen)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestIsThinking tests the IsThinking function
func TestIsThinking(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "thinking indicator lowercase",
			input:    "thinking...",
			expected: true,
		},
		{
			name:     "thinking indicator uppercase",
			input:    "THINKING...",
			expected: true,
		},
		{
			name:     "loading indicator",
			input:    "loading content",
			expected: true,
		},
		{
			name:     "working indicator",
			input:    "working on it",
			expected: true,
		},
		{
			name:     "generating indicator",
			input:    "generating response",
			expected: true,
		},
		{
			name:     "computing indicator",
			input:    "computing answer",
			expected: true,
		},
		{
			name:     "summoning indicator",
			input:    "summoning magic",
			expected: true,
		},
		{
			name:     "regular content",
			input:    "This is a normal response",
			expected: false,
		},
		{
			name:     "empty string",
			input:    "",
			expected: false,
		},
		{
			name:     "code block",
			input:    "```python\ndef hello():\n    pass\n```",
			expected: false,
		},
		{
			name:     "list item",
			input:    "- First item",
			expected: false,
		},
		{
			name:     "thinking in multiline output",
			input:    strings.Repeat("line\n", 100) + "thinking...",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsThinking(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestMin tests Min function
func TestMin(t *testing.T) {
	t.Run("first smaller", func(t *testing.T) {
		assert.Equal(t, 1, Min(1, 5))
		assert.Equal(t, -10, Min(-10, 5))
	})

	t.Run("second smaller", func(t *testing.T) {
		assert.Equal(t, 2, Min(5, 2))
		assert.Equal(t, -5, Min(5, -5))
	})

	t.Run("equal values", func(t *testing.T) {
		assert.Equal(t, 5, Min(5, 5))
		assert.Equal(t, 0, Min(0, 0))
	})

	t.Run("negative values", func(t *testing.T) {
		assert.Equal(t, -10, Min(-5, -10))
		assert.Equal(t, -100, Min(-50, -100))
	})

	t.Run("with zero", func(t *testing.T) {
		assert.Equal(t, 0, Min(0, 5))
		assert.Equal(t, -5, Min(0, -5))
	})
}

// TestExtractLastAssistantContent tests ExtractLastAssistantContent function
func TestExtractLastAssistantContent(t *testing.T) {
	t.Run("empty output", func(t *testing.T) {
		result := ExtractLastAssistantContent("")
		assert.Equal(t, "", result)
	})

	t.Run("output with assistant content", func(t *testing.T) {
		output := `User: Hello
Assistant: World!`
		result := ExtractLastAssistantContent(output)
		assert.NotEmpty(t, result)
	})

	t.Run("output without assistant", func(t *testing.T) {
		output := `User: Hello
User: How are you?`
		result := ExtractLastAssistantContent(output)
		assert.NotEmpty(t, result)
	})
}

// TestExtractNewContent_EmptyCases tests ExtractNewContent with empty inputs
func TestExtractNewContent_EmptyCases(t *testing.T) {
	t.Run("empty output", func(t *testing.T) {
		result := ExtractNewContent("", "", "")
		assert.Equal(t, "", result)
	})

	t.Run("output with no prompt", func(t *testing.T) {
		output := `Some content here
More content`
		result := ExtractNewContent(output, "", "")
		assert.NotEmpty(t, result)
	})

	t.Run("empty before snapshot", func(t *testing.T) {
		output := `User: Hello
Assistant: Hi there!`
		result := ExtractNewContent(output, "Hello", "")
		assert.NotEmpty(t, result)
	})
}

// TestExtractNewContentWithHistory_EmptyCases tests ExtractNewContentWithHistory with empty inputs
func TestExtractNewContentWithHistory_EmptyCases(t *testing.T) {
	t.Run("empty output", func(t *testing.T) {
		result := ExtractNewContentWithHistory("", []InputRecord{}, "")
		assert.Equal(t, "", result)
	})

	t.Run("output with empty history", func(t *testing.T) {
		output := `Some content
More content`
		result := ExtractNewContentWithHistory(output, []InputRecord{}, "")
		assert.NotEmpty(t, result)
	})

	t.Run("output with nil history", func(t *testing.T) {
		output := `Content here`
		result := ExtractNewContentWithHistory(output, nil, "")
		assert.NotEmpty(t, result)
	})

	t.Run("output with history", func(t *testing.T) {
		output := `User: Test
Assistant: Response here`
		history := []InputRecord{
			{Content: "Test"},
		}
		result := ExtractNewContentWithHistory(output, history, "")
		assert.NotEmpty(t, result)
	})
}

// TestInputRecord tests InputRecord structure
func TestInputRecord(t *testing.T) {
	t.Run("create input record", func(t *testing.T) {
		record := InputRecord{
			Content:   "test content",
			Timestamp: 1234567890,
		}
		assert.Equal(t, "test content", record.Content)
		assert.Equal(t, int64(1234567890), record.Timestamp)
	})

	t.Run("empty input record", func(t *testing.T) {
		record := InputRecord{}
		assert.Empty(t, record.Content)
		assert.Zero(t, record.Timestamp)
	})
}
