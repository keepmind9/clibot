package watchdog

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Bug reproduction tests for non-hook mode tmux content extraction issues

// TestBug_PartialMatchInAIResponse tests that partial matching doesn't match AI's response text
// Bug: Priority 3 partial match can match lines like "Let me test this for you" when userPrompt is "test"
func TestBug_PartialMatchInAIResponse(t *testing.T) {
	tmuxOutput := `❯ test
Thinking...
Let me test this function for you
Here is the result: success`

	inputs := []InputRecord{
		{Timestamp: 1001, Content: "test"},
	}

	result := ExtractContentAfterAnyInput(tmuxOutput, inputs)

	// Should extract content after "❯ test", which includes "Let me test this..."
	// But should NOT match "Let me test this..." as the user prompt line
	assert.Contains(t, result, "Let me test this function for you", "Should contain AI's response")
	assert.Contains(t, result, "Here is the result", "Should contain full response")

	// The critical check: result should start with AI's response, not include the prompt line
	assert.NotContains(t, result, "❯ test", "Should not include the prompt line itself")
}

// TestBug_GreaterThanPrefix tests that "> test" prefix is properly handled
// Bug: isLikelyUserPromptLine only checks for "❯ " prefix, ignoring "> " and ">>>"
func TestBug_GreaterThanPrefix(t *testing.T) {
	tmuxOutput := `> test
Processing...
Result: done`

	inputs := []InputRecord{
		{Timestamp: 1001, Content: "test"},
	}

	result := ExtractContentAfterAnyInput(tmuxOutput, inputs)

	// Should match "> test" as it has cursor prefix
	assert.Contains(t, result, "Processing", "Should extract content after > test")
	assert.Contains(t, result, "Result: done", "Should contain full response")
}

// TestBug_UserInputOnLastLine tests extraction when user input is the last line
// Bug: extractContent starts from promptIndex + 1, which returns empty if AI hasn't responded yet
func TestBug_UserInputOnLastLine(t *testing.T) {
	// Edge case: User just sent input, AI hasn't responded yet
	tmuxOutput := `❯ test`

	inputs := []InputRecord{
		{Timestamp: 1001, Content: "test"},
	}

	result := ExtractContentAfterAnyInput(tmuxOutput, inputs)

	// Should return empty or fallback, not crash
	// This is expected behavior - no content yet
	assert.NotContains(t, result, "❯ test", "Should not include prompt in result")
}

// TestBug_MultipleShortInputsFallback tests fallback behavior for multiple short inputs
// Bug: When all inputs are short and don't appear in tmux, should use extractLastAssistantContent
func TestBug_MultipleShortInputsFallback(t *testing.T) {
	tmuxOutput := `❯ help me
Thinking...
Here is your help

❯ 1. Continue
❯ 2. Stop
More AI content here`

	// User history shows they selected menu options, but those short inputs may not be visible
	inputs := []InputRecord{
		{Timestamp: 1003, Content: "1"},       // Most recent - menu selection
		{Timestamp: 1002, Content: "help me"}, // Original input
	}

	result := ExtractContentAfterAnyInput(tmuxOutput, inputs)

	// Should match "help me" and extract content after it
	assert.Contains(t, result, "Here is your help", "Should find and match the original prompt")

	// SHOULD include menu options (they are part of AI's response!)
	assert.Contains(t, result, "❯ 1. Continue", "Should include menu options as part of AI response")
	assert.Contains(t, result, "More AI content here", "Should include content after menu")
}

// TestBug_DuplicateInputInHistory tests when same input appears multiple times in history
// Bug: findPromptIndex searches backwards and might match the wrong occurrence
func TestBug_DuplicateInputInHistory(t *testing.T) {
	tmuxOutput := `help me
Response 1: This is help

help me
Response 2: This is newer help

help me
Response 3: This is the latest`

	inputs := []InputRecord{
		{Timestamp: 1003, Content: "help me"}, // Most recent input
	}

	result := ExtractContentAfterAnyInput(tmuxOutput, inputs)

	// Should match the LAST "help me" (most recent) and extract "Response 3"
	assert.Contains(t, result, "Response 3", "Should extract content after the last occurrence")

	// Should NOT include older responses
	assert.NotContains(t, result, "Response 1", "Should not include first response")
}

// TestBug_SpecialCharactersInInput tests inputs with special regex characters
// Bug: User input like "test.c" or "help me?" might not match correctly
func TestBug_SpecialCharactersInInput(t *testing.T) {
	tmuxOutput := `❯ test.c
Compiling test.c...
Success`

	inputs := []InputRecord{
		{Timestamp: 1001, Content: "test.c"},
	}

	result := ExtractContentAfterAnyInput(tmuxOutput, inputs)

	assert.Contains(t, result, "Compiling test.c", "Should handle dots in input")
	assert.Contains(t, result, "Success", "Should extract full response")
}

// TestBug_LongInputTruncation tests prefix matching for long inputs
// Bug: Long inputs (> MaxPromptPrefixLength) use prefix matching which might be too aggressive
func TestBug_LongInputTruncation(t *testing.T) {
	longInput := "this is a very long input that exceeds the max prefix length and gets truncated"
	tmuxOutput := `❯ ` + longInput + `
Processing your long input...`

	inputs := []InputRecord{
		{Timestamp: 1001, Content: longInput},
	}

	result := ExtractContentAfterAnyInput(tmuxOutput, inputs)

	assert.Contains(t, result, "Processing your long input", "Should handle truncated long inputs")
}

// TestBug_MultilineInput tests inputs that span multiple lines
// Bug: Input with newlines is stored as JSON but matching might fail
func TestBug_MultilineInput(t *testing.T) {
	tmuxOutput := `❯ help me
write code
Thinking...`

	inputs := []InputRecord{
		{Timestamp: 1001, Content: "help me\nwrite code"},
	}

	result := ExtractContentAfterAnyInput(tmuxOutput, inputs)

	// The multiline input is stored with \n in JSON, but tmux shows it as separate lines
	// This is a known limitation - should fall back to basic extraction
	assert.NotEmpty(t, result, "Should return some content even with multiline input mismatch")
}
