package watchdog

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestIsLikelyUserPromptLine_MenuOptions tests that menu options are not mistaken for user input
func TestIsLikelyUserPromptLine_MenuOptions(t *testing.T) {
	tests := []struct {
		name       string
		line       string
		userPrompt string
		want       bool
	}{
		{
			name:       "menu option with cursor should be rejected",
			line:       "❯ 1. Yes",
			userPrompt: "1",
			want:       false, // This is a menu option, not user input
		},
		{
			name:       "menu option with cursor should be rejected (option 2)",
			line:       "❯ 2. Yes, allow all edits",
			userPrompt: "2",
			want:       false, // This is a menu option, not user input
		},
		{
			name:       "actual user input with cursor should be accepted",
			line:       "❯ 1",
			userPrompt: "1",
			want:       true, // This IS user input (just the number)
		},
		{
			name:       "menu option without cursor should be rejected",
			line:       "1. Yes",
			userPrompt: "1",
			want:       false, // Menu option without cursor
		},
		{
			name:       "plain user input should be accepted",
			line:       "1",
			userPrompt: "1",
			want:       true, // Exact match
		},
		{
			name:       "user input with text should be accepted",
			line:       "❯ help me",
			userPrompt: "help me",
			want:       true, // Actual user input
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isLikelyUserPromptLine(tt.line, tt.userPrompt)
			assert.Equal(t, tt.want, got, "isLikelyUserPromptLine(%q, %q)", tt.line, tt.userPrompt)
		})
	}
}

// TestExtractContentAfterPrompt_MenuScenario tests the full menu scenario
func TestExtractContentAfterPrompt_MenuScenario(t *testing.T) {
	// Simulate a menu scenario where user selects option 1
	tmuxOutput := `Do you want to make this edit to config.yaml?
❯ 1. Yes
2. Yes, allow all edits during this session (shift+Tab)
3. No
Esc to cancel · Tab to amend
1
[Response from AI after user selects option 1]`

	userPrompt := "1"

	result := ExtractContentAfterPrompt(tmuxOutput, userPrompt)

	// Should extract the AI response, not the menu
	// The result should NOT contain "2. Yes, allow all edits..."
	assert.NotContains(t, result, "2. Yes, allow all edits", "Should not include menu option 2")
	assert.NotContains(t, result, "3. No", "Should not include menu option 3")
	// Should contain the actual AI response
	assert.Contains(t, result, "[Response from AI after user selects option 1]", "Should contain AI response")
}

// TestExtractContentAfterPrompt_MenuScenarioWithCursor tests when user input is in menu context
func TestExtractContentAfterPrompt_MenuScenarioWithCursor(t *testing.T) {
	// User sent "1", hook triggered after user input
	// tmux shows the menu with user's "1" on a separate line
	tmuxOutput := `Do you want to make this edit to config.yaml?
❯ 1. Yes
2. Yes, allow all edits during this session (shift+Tab)
3. No
Esc to cancel · Tab to amend
1
Thinking...
[AI starts processing the selection]`

	userPrompt := "1"

	result := ExtractContentAfterPrompt(tmuxOutput, userPrompt)

	// Should extract content after the user's "1" input line
	// Should NOT include menu options 2 and 3
	assert.NotContains(t, result, "2. Yes, allow all edits", "Should not include menu option 2")
	assert.NotContains(t, result, "3. No", "Should not include menu option 3")
	assert.NotContains(t, result, "❯ 1. Yes", "Should not include the menu option 1 line")
	// Should contain the processing indicator
	assert.Contains(t, result, "[AI starts processing the selection]", "Should contain AI response")
}
