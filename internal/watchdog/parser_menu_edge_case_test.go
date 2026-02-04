package watchdog

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestEdgeCase_MenuOptionsInResponse tests that menu options in AI responses
// are preserved in the extracted content (they are part of AI's response)
func TestEdgeCase_MenuOptionsInResponse(t *testing.T) {
	// Scenario: AI asks user to choose from a menu
	tmuxOutput := `❯ help me
Thinking...
Do you want to proceed?
❯ 1. Yes
❯ 2. No
❯ 3. Cancel
I'll wait for your choice.`

	inputs := []InputRecord{
		{Timestamp: 1001, Content: "help me"},
	}

	result := ExtractContentAfterAnyInput(tmuxOutput, inputs)

	// Should extract the AI response
	assert.Contains(t, result, "Thinking", "Should contain 'Thinking'")
	assert.Contains(t, result, "Do you want to proceed?", "Should contain the question")

	// SHOULD include the menu options (they are part of AI's response!)
	assert.Contains(t, result, "❯ 1. Yes", "Should include menu option 1")
	assert.Contains(t, result, "❯ 2. No", "Should include menu option 2")
	assert.Contains(t, result, "❯ 3. Cancel", "Should include menu option 3")

	// Should include content after the menu
	assert.Contains(t, result, "I'll wait for your choice", "Should include text after menu")
}

// TestEdgeCase_UserSelectsMenuOption tests when user selects "1" from menu
func TestEdgeCase_UserSelectsMenuOption(t *testing.T) {
	// Scenario: User selected option 1
	tmuxOutput := `Do you want to proceed?
❯ 1. Yes
❯ 2. No
❯ 3. Cancel
1
Great! Proceeding with the operation...`

	inputs := []InputRecord{
		{Timestamp: 1001, Content: "1"},
	}

	result := ExtractContentAfterAnyInput(tmuxOutput, inputs)

	// Should find the user's "1" input (not "❯ 1. Yes")
	// and extract content after it
	assert.Contains(t, result, "Great! Proceeding with the operation", "Should contain response after user selection")

	// Should NOT include the menu options that appeared before user input
	assert.NotContains(t, result, "❯ 1. Yes", "Should not include menu option 1")
	assert.NotContains(t, result, "❯ 2. No", "Should not include menu option 2")
}

// TestEdgeCase_MixedMenuAndContent tests complex scenario with multiple menus
func TestEdgeCase_MixedMenuAndContent(t *testing.T) {
	tmuxOutput := `❯ run test
Executing tests...
Test failed with 3 errors:
1. Syntax error on line 10
2. Missing semicolon on line 25
3. Undefined variable 'x'

Fix errors?
❯ 1. Yes, fix them
❯ 2. No, show me details
❯ 3. Skip

1
Fixed 3 errors.
Tests passed!`

	inputs := []InputRecord{
		{Timestamp: 1001, Content: "run test"},
	}

	result := ExtractContentAfterAnyInput(tmuxOutput, inputs)

	// Should extract the error list
	assert.Contains(t, result, "Test failed with 3 errors", "Should contain error summary")
	assert.Contains(t, result, "1. Syntax error on line 10", "Should contain error 1")
	assert.Contains(t, result, "2. Missing semicolon on line 25", "Should contain error 2")
	assert.Contains(t, result, "3. Undefined variable", "Should contain error 3")

	// SHOULD include the menu options (they are part of AI's response!)
	assert.Contains(t, result, "❯ 1. Yes, fix them", "Should include menu options as part of AI response")
	assert.Contains(t, result, "Fixed 3 errors", "Should include content after user selection")
}

// TestEdgeCase_MenuWithoutCursor tests menu options without cursor prefix
func TestEdgeCase_MenuWithoutCursor(t *testing.T) {
	// Some CLIs show menus without cursor prefix
	tmuxOutput := `Choose an option:
1. Yes
2. No
3. Cancel

1
Option 1 selected!`

	inputs := []InputRecord{
		{Timestamp: 1001, Content: "1"},
	}

	result := ExtractContentAfterAnyInput(tmuxOutput, inputs)

	// Should extract content after user's "1"
	assert.Contains(t, result, "Option 1 selected!", "Should contain response")

	// This is tricky - the menu "1. Yes" might be included since it doesn't have ❯ prefix
	// But that's acceptable behavior for menus without cursor
}
