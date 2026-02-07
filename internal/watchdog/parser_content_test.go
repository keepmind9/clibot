package watchdog

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestExtractContentAfterPrompt tests ExtractContentAfterPrompt function
func TestExtractContentAfterPrompt(t *testing.T) {
	t.Run("output with user prompt", func(t *testing.T) {
		output := `Some text before
> user prompt here
Content after prompt`
		result := ExtractContentAfterPrompt(output, "user prompt here")
		assert.NotEmpty(t, result)
		assert.Contains(t, result, "Content after")
	})

	t.Run("output without prompt", func(t *testing.T) {
		output := `Just some content
No prompt here`
		result := ExtractContentAfterPrompt(output, "nonexistent prompt")
		// Should return original or empty depending on implementation
		assert.NotEmpty(t, result)
	})

	t.Run("empty output", func(t *testing.T) {
		result := ExtractContentAfterPrompt("", "prompt")
		assert.Equal(t, "", result)
	})

	t.Run("empty prompt", func(t *testing.T) {
		output := `Content here`
		result := ExtractContentAfterPrompt(output, "")
		assert.NotEmpty(t, result)
	})
}

// TestCleanContent tests cleanContent function behavior
func TestCleanContent_Behavior(t *testing.T) {
	// Test through ExtractContentAfterPrompt which uses cleanContent internally
	t.Run("clean content in output", func(t *testing.T) {
		output := "> prompt\nResponse here\nMore lines"
		result := ExtractContentAfterPrompt(output, "prompt")
		// Result should be cleaned
		assert.NotEmpty(t, result)
	})

	t.Run("multiline response", func(t *testing.T) {
		output := "> prompt\nLine 1\nLine 2\nLine 3"
		result := ExtractContentAfterPrompt(output, "prompt")
		assert.NotEmpty(t, result)
	})
}

// TestIsLikelyUserPromptLine tests isLikelyUserPromptLine function
func TestIsLikelyUserPromptLine(t *testing.T) {
	tests := []struct {
		name       string
		line       string
		userPrompt string
		expected   bool
	}{
		{
			name:       "user prompt line with > cursor",
			line:       "> user prompt",
			userPrompt: "user prompt",
			expected:   true,
		},
		{
			name:       "user prompt line with ❯ cursor",
			line:       "❯ user prompt",
			userPrompt: "user prompt",
			expected:   true,
		},
		{
			name:       "user prompt with extra text",
			line:       "> user prompt and more",
			userPrompt: "user prompt",
			expected:   true,
		},
		{
			name:       "normal content line",
			line:       "This is a response",
			userPrompt: "",
			expected:   false,
		},
		{
			name:       "cursor without user prompt",
			line:       "> some other text",
			userPrompt: "user prompt",
			expected:   false,
		},
		{
			name:       "empty line with empty prompt",
			line:       "",
			userPrompt: "",
			// When both are empty, strings.Contains and strings.HasPrefix return true
			expected: true,
		},
		{
			name:       "code line",
			line:       "    def hello():",
			userPrompt: "",
			expected:   false,
		},
		{
			name:       "assistant marker",
			line:       "Assistant: response",
			userPrompt: "",
			expected:   false,
		},
		{
			name:       "menu option",
			line:       "> 1. First option",
			userPrompt: "",
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isLikelyUserPromptLine(tt.line, tt.userPrompt)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestExtractNewContentWithHistory_MoreCases tests ExtractNewContentWithHistory with more cases
func TestExtractNewContentWithHistory_MoreCases(t *testing.T) {
	t.Run("simple user prompt", func(t *testing.T) {
		output := `> Hello world
This is the response`
		history := []InputRecord{
			{Content: "Hello world"},
		}
		result := ExtractNewContentWithHistory(output, history, "")
		assert.NotEmpty(t, result)
	})

	t.Run("multi-line prompt", func(t *testing.T) {
		output := `> First line
> Second line
Response content here`
		history := []InputRecord{
			{Content: "First line\nSecond line"},
		}
		result := ExtractNewContentWithHistory(output, history, "")
		assert.NotEmpty(t, result)
	})

	t.Run("no match found", func(t *testing.T) {
		output := `Some content without prompt
More content`
		history := []InputRecord{
			{Content: "unmatched prompt"},
		}
		result := ExtractNewContentWithHistory(output, history, "")
		// Should still return something
		assert.NotEmpty(t, result)
	})

	t.Run("with before snapshot", func(t *testing.T) {
		output := `> Test prompt
New response here`
		history := []InputRecord{
			{Content: "Test prompt"},
		}
		beforeSnapshot := "Old content that should be excluded"
		result := ExtractNewContentWithHistory(output, history, beforeSnapshot)
		assert.NotEmpty(t, result)
		assert.NotContains(t, result, beforeSnapshot)
	})
}
