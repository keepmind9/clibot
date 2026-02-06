package watchdog

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestExtractIncrement_BasicNewContent tests basic new content extraction
func TestExtractIncrement_BasicNewContent(t *testing.T) {
	before := `❯ test
Thinking...`

	after := `❯ test
Thinking...
Here is the response
With multiple lines`

	result := ExtractIncrement(after, before)

	assert.Contains(t, result, "Here is the response")
	assert.Contains(t, result, "With multiple lines")
	assert.NotContains(t, result, "❯ test")
}

// TestExtractIncrement_BeforeContentScrolledOff tests when before content has scrolled off
func TestExtractIncrement_BeforeContentScrolledOff(t *testing.T) {
	// Before: Old command at top
	before := `old command 1
old response 1
❯ test
Thinking...`

	// After: Old command has scrolled off, new content appears
	after := `❯ test
Thinking...
Here is new response`

	result := ExtractIncrement(after, before)

	// Should extract the new content at the end
	assert.Contains(t, result, "Here is new response")
}

// TestExtractIncrement_ContentReplacement tests content replacement scenarios
func TestExtractIncrement_ContentReplacement(t *testing.T) {
	tests := []struct {
		name     string
		before   string
		after    string
		contains string
	}{
		{
			name: "progress bar replacement",
			before: `❯ download
Progress: [=====>     ] 50%`,
			after: `❯ download
Progress: [==========] 100%
Download complete`,
			contains: "Download complete",
		},
		{
			name: "dynamic menu",
			before: `❯ help
Options:
1. View
2. Edit`,
			after: `❯ help
Options:
1. View
2. Edit
3. Delete
Choose option:`,
			contains: "Choose option:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractIncrement(tt.after, tt.before)
			assert.Contains(t, result, tt.contains)
		})
	}
}

// TestExtractIncrement_WithMenuOptions ensures menu options are preserved
func TestExtractIncrement_WithMenuOptions(t *testing.T) {
	before := `❯ help
Thinking...`

	after := `❯ help
Choose option:
❯ 1. Yes
❯ 2. No
❯ 3. Cancel`

	result := ExtractIncrement(after, before)

	// Menu options should be preserved
	assert.Contains(t, result, "❯ 1. Yes")
	assert.Contains(t, result, "❯ 2. No")
	assert.Contains(t, result, "❯ 3. Cancel")
}

// TestExtractIncrement_EmptyBefore tests with empty before snapshot
func TestExtractIncrement_EmptyBefore(t *testing.T) {
	before := ""

	after := `❯ actual test input
Here is response`

	result := ExtractIncrement(after, before)

	// Should return non-empty content from after
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "Here is response")
}

// TestExtractIncrement_NoNewContent tests when there's no new content
func TestExtractIncrement_NoNewContent(t *testing.T) {
	before := `❯ test
Thinking...`

	after := `❯ test
Thinking...`

	result := ExtractIncrement(after, before)

	// May return empty or minimal content
	// The important thing is it doesn't crash
	assert.NotNil(t, result)
}

// TestExtractIncrement_MultilineInput tests with multiline user input
func TestExtractIncrement_MultilineInput(t *testing.T) {
	before := `❯ echo "hello
world"`

	after := `❯ echo "hello
world"
hello
world`

	result := ExtractIncrement(after, before)

	// Should extract the output
	assert.Contains(t, result, "hello")
	assert.Contains(t, result, "world")
}

// TestExtractIncrement_LongResponse tests with long response (more than 50 lines)
func TestExtractIncrement_LongResponse(t *testing.T) {
	// Build a long before
	beforeLines := []string{"❯ generate"}
	for i := 0; i < 30; i++ {
		beforeLines = append(beforeLines, "old line content")
	}
	before := joinLines(beforeLines)

	// Build a long after (100 lines total)
	afterLines := []string{"❯ generate"}
	// Old content (some scrolled off)
	for i := 0; i < 30; i++ {
		afterLines = append(afterLines, "old line content")
	}
	// New content
	for i := 0; i < 70; i++ {
		afterLines = append(afterLines, "new line content")
	}
	after := joinLines(afterLines)

	result := ExtractIncrement(after, before)

	// Should extract new content
	assert.Contains(t, result, "new line content")
	// Should not include old content
	assert.NotContains(t, result, "old line content")
}

// TestExtractIncrement_ScrollingScenario tests realistic scrolling scenario
func TestExtractIncrement_ScrollingScenario(t *testing.T) {
	// Before: Full pane with old command
	beforeLines := []string{}
	for i := 1; i <= 50; i++ {
		beforeLines = append(beforeLines, "old content line %d")
	}
	beforeLines = append(beforeLines, "❯ new command")
	before := joinLines(beforeLines)

	// After: Old content scrolled off, only new command and response visible
	after := `❯ new command
Thinking...
Response content here
More response`

	result := ExtractIncrement(after, before)

	// Should extract the new response
	assert.Contains(t, result, "Response content")
	assert.Contains(t, result, "More response")
}

// TestExtractIncrement_MenuDisappears tests when menu disappears after selection
func TestExtractIncrement_MenuDisappears(t *testing.T) {
	before := `❯ help
Choose option:
❯ 1. Yes
❯ 2. No`

	after := `1
Great! You chose 1`

	result := ExtractIncrement(after, before)

	// Should extract the response after menu selection
	assert.Contains(t, result, "Great! You chose 1")
}

// TestExtractIncrement_OnlyStatusChange tests when only status indicators change
func TestExtractIncrement_OnlyStatusChange(t *testing.T) {
	before := `❯ test
Thinking...
ESC to interrupt`

	after := `❯ test
ESC to interrupt
Done`

	result := ExtractIncrement(after, before)

	// Should extract "Done" as new content
	assert.Contains(t, result, "Done")
}

// Helper function to join lines with newlines
func joinLines(lines []string) string {
	result := ""
	for i, line := range lines {
		if i > 0 {
			result += "\n"
		}
		result += line
	}
	return result
}

// BenchmarkExtractIncrement benchmarks the extraction performance
func BenchmarkExtractIncrement(b *testing.B) {
	// Build realistic before/after snapshots
	beforeLines := []string{"❯ test"}
	for i := 0; i < 100; i++ {
		beforeLines = append(beforeLines, "content line %d")
	}
	before := joinLines(beforeLines)

	afterLines := []string{"❯ test"}
	for i := 0; i < 100; i++ {
		afterLines = append(afterLines, "content line %d")
	}
	for i := 0; i < 20; i++ {
		afterLines = append(afterLines, "new content line %d")
	}
	after := joinLines(afterLines)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ExtractIncrement(after, before)
	}
}

// TestCleanIncrementalContent_UIBorders tests cleanIncrementalContent directly
func TestCleanIncrementalContent_UIBorders(t *testing.T) {
	input := `Here is the response
──────────────────────────────────────────────────────────
═══════════════════════════════════════════════════════════
Another response`

	result := cleanIncrementalContent(input)

	// Should NOT contain border lines
	assert.NotContains(t, result, "─")
	assert.NotContains(t, result, "═")
	assert.NotContains(t, result, "│")

	// Should contain actual text
	assert.Contains(t, result, "Here is the response")
	assert.Contains(t, result, "Another response")
}

// TestExtractIncrement_UIBorderLines tests that UI border lines are filtered out in final output
func TestExtractIncrement_UIBorderLines(t *testing.T) {
	before := `❯ test
Thinking...`

	after := `❯ test
Thinking...
Here is the response
──────────────────────────────────────────────────────────`

	result := ExtractIncrement(after, before)

	// Should extract the response
	assert.Contains(t, result, "Here is the response")

	// Should NOT contain border lines (filtered in cleanIncrementalContent)
	assert.NotContains(t, result, "──")
}

// TestExtractIncrement_MultipleUILines tests multiple consecutive UI border lines are filtered
func TestExtractIncrement_MultipleUILines(t *testing.T) {
	before := `❯ generate`

	after := `❯ generate
──────────────────────────────────────────────────────────
Generating response...
──────────────────────────────────────────────────────────
Done!
═══════════════════════════════════════════════════════════`

	result := ExtractIncrement(after, before)

	// Should contain the actual response
	assert.Contains(t, result, "Generating response")
	assert.Contains(t, result, "Done!")

	// Should NOT contain border lines (filtered in cleanIncrementalContent)
	assert.NotContains(t, result, "──")
	assert.NotContains(t, result, "══")
}
