package watchdog

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestExtractContentAfterAnyInput_MenuScenario tests the multi-input matching for menu selections
func TestExtractContentAfterAnyInput_MenuScenario(t *testing.T) {
	// Simulate a scenario where user inputs multiple menu selections
	tmuxOutput := `Historical response to "help me"
---
Do you want to continue?
❯ 1. Yes
2. No
Esc to cancel
[AI Processing option 1]`

	// Input history (from newest to oldest)
	inputs := []InputRecord{
		{Timestamp: 1003, Content: "1"},       // Current input (won't match)
		{Timestamp: 1002, Content: "yes"},     // Previous input (won't match)
		{Timestamp: 1001, Content: "help me"}, // Oldest input (will match)
	}

	result := ExtractContentAfterAnyInput(tmuxOutput, inputs)

	// Should match "help me" and extract content after it
	assert.Contains(t, result, "Historical response")
	assert.Contains(t, result, "[AI Processing option 1]")
}

// TestExtractContentAfterAnyInput_AllShortInputs tests when all inputs are short
func TestExtractContentAfterAnyInput_AllShortInputs(t *testing.T) {
	tmuxOutput := `Some historical content
More history
[Latest AI response]`

	inputs := []InputRecord{
		{Timestamp: 1003, Content: "1"},
		{Timestamp: 1002, Content: "2"},
		{Timestamp: 1001, Content: "y"},
	}

	result := ExtractContentAfterAnyInput(tmuxOutput, inputs)

	// No inputs match, should return fallback (full content)
	assert.Contains(t, result, "[Latest AI response]")
}

// TestExtractContentAfterAnyInput_FirstMatchWins tests that first match wins
func TestExtractContentAfterAnyInput_FirstMatchWins(t *testing.T) {
	tmuxOutput := `First response
---
Second response
---
Third response`

	inputs := []InputRecord{
		{Timestamp: 1003, Content: "Second"}, // This should match first
		{Timestamp: 1002, Content: "First"},  // This also matches but shouldn't be used
	}

	result := ExtractContentAfterAnyInput(tmuxOutput, inputs)

	// Should extract after "Second", which includes "Third response"
	assert.Contains(t, result, "Third response")
	// Note: extractContent starts from the line AFTER the match
	// So after "Second" comes "Third response"
}

// TestExtractContentAfterAnyInput_EmptyInputs tests with no inputs
func TestExtractContentAfterAnyInput_EmptyInputs(t *testing.T) {
	tmuxOutput := `Some content`

	inputs := []InputRecord{}

	result := ExtractContentAfterAnyInput(tmuxOutput, inputs)

	// Should return fallback
	assert.Contains(t, result, "Some content")
}

// TestExtractContentAfterAnyInput_WithNewlinesAndSpecialChars tests inputs with special characters
func TestExtractContentAfterAnyInput_WithNewlinesAndSpecialChars(t *testing.T) {
	tmuxOutput := `User asked: help me|write code
AI responded: Here's your code
function test() {
  return "hello";
}`

	inputs := []InputRecord{
		{Timestamp: 1001, Content: "help me|write code"},
	}

	result := ExtractContentAfterAnyInput(tmuxOutput, inputs)

	assert.Contains(t, result, "AI responded:")
	assert.Contains(t, result, "function test()")
}
