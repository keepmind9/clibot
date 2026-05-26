package feishu

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/keepmind9/clibot/internal/bot"
	"github.com/stretchr/testify/assert"
)

func TestBuildBatchUpdateActions_TextBlock(t *testing.T) {
	blocks := []bot.ContentBlock{
		{Type: bot.ContentBlockText, Content: "Hello world"},
	}

	actions, toolCount := buildBatchUpdateActions(blocks, 0)
	assert.Len(t, actions, 1)
	assert.Equal(t, 0, toolCount)
	assert.Equal(t, "update_element", *actions[0].Action)
	assert.NotNil(t, actions[0].Params.Element)
	assert.Contains(t, *actions[0].Params.Element, "Hello world")
}

func TestBuildBatchUpdateActions_ToolCallBlock(t *testing.T) {
	blocks := []bot.ContentBlock{
		{Type: bot.ContentBlockToolCall, Title: "Bash", Meta: map[string]string{"command": "ls"}},
	}

	actions, toolCount := buildBatchUpdateActions(blocks, 0)
	assert.Len(t, actions, 1)
	assert.Equal(t, 1, toolCount)
	assert.Equal(t, "update_element", *actions[0].Action)
	assert.Contains(t, *actions[0].Params.Element, "ls")
}

func TestBuildBatchUpdateActions_MultipleBlocks(t *testing.T) {
	blocks := []bot.ContentBlock{
		{Type: bot.ContentBlockText, Content: "Hello"},
		{Type: bot.ContentBlockToolCall, Title: "Bash", Meta: map[string]string{"command": "ls"}},
		{Type: bot.ContentBlockToolResult, Content: "file1.txt"},
		{Type: bot.ContentBlockText, Content: "Done"},
	}

	actions, toolCount := buildBatchUpdateActions(blocks, 0)
	assert.Len(t, actions, 1)
	assert.Equal(t, 1, toolCount)
	assert.Equal(t, "update_element", *actions[0].Action)

	// All content should be in the single element
	element := *actions[0].Params.Element
	assert.Contains(t, element, "Hello")
	assert.Contains(t, element, "ls")
	assert.Contains(t, element, "file1.txt")
	assert.Contains(t, element, "Done")
}

func TestBuildBatchUpdateActions_ThinkingBlock(t *testing.T) {
	blocks := []bot.ContentBlock{
		{Type: bot.ContentBlockThinking, Content: "Let me think about this..."},
	}

	actions, _ := buildBatchUpdateActions(blocks, 0)
	assert.Len(t, actions, 1)
	assert.Equal(t, "update_element", *actions[0].Action)
	assert.Contains(t, *actions[0].Params.Element, "Let me think")
}

func TestBuildBatchUpdateActions_StatusBlock(t *testing.T) {
	blocks := []bot.ContentBlock{
		{Type: bot.ContentBlockStatus, Content: "Token usage: 100"},
	}

	actions, _ := buildBatchUpdateActions(blocks, 0)
	assert.Len(t, actions, 1)
	assert.Equal(t, "update_element", *actions[0].Action)
	assert.Contains(t, *actions[0].Params.Element, "Token usage: 100")
}

func TestBuildBatchUpdateActions_ToolResultBlock(t *testing.T) {
	blocks := []bot.ContentBlock{
		{Type: bot.ContentBlockToolResult, Content: "file contents here"},
	}

	actions, _ := buildBatchUpdateActions(blocks, 0)
	assert.Len(t, actions, 1)
	assert.Equal(t, "update_element", *actions[0].Action)
	assert.Contains(t, *actions[0].Params.Element, "file contents here")
}

func TestBuildBatchUpdateActions_Empty(t *testing.T) {
	actions, toolCount := buildBatchUpdateActions(nil, 5)
	assert.Len(t, actions, 0)
	assert.Equal(t, 5, toolCount)
}

func TestBuildBatchUpdateActions_ElementIsValidJSON(t *testing.T) {
	blocks := []bot.ContentBlock{
		{Type: bot.ContentBlockText, Content: "Test with \"quotes\" and \nnewlines"},
	}

	actions, _ := buildBatchUpdateActions(blocks, 0)
	assert.Len(t, actions, 1)

	// The element field should be valid JSON
	var element map[string]interface{}
	err := json.Unmarshal([]byte(*actions[0].Params.Element), &element)
	assert.NoError(t, err)
	assert.Equal(t, "markdown", element["tag"])
	assert.Equal(t, "main_content", element["element_id"])
	assert.Contains(t, element["content"], "Test with")
}

func TestRenderBlocksToMarkdown(t *testing.T) {
	blocks := []bot.ContentBlock{
		{Type: bot.ContentBlockText, Content: "Hello"},
		{Type: bot.ContentBlockToolCall, Title: "Bash", Meta: map[string]string{"command": "ls -la"}},
		{Type: bot.ContentBlockText, Content: "Done"},
	}

	result := renderBlocksToMarkdown(blocks)
	assert.Contains(t, result, "Hello")
	assert.Contains(t, result, "ls -la")
	assert.Contains(t, result, "Done")
	assert.True(t, strings.HasPrefix(result, "Hello"))
}

func TestRenderBlocksToMarkdown_Truncation(t *testing.T) {
	longText := strings.Repeat("a", 1000)
	blocks := []bot.ContentBlock{
		{Type: bot.ContentBlockText, Content: longText},
	}

	result := renderBlocksToMarkdown(blocks)
	assert.LessOrEqual(t, len(result), maxFieldChars+10) // some slack for newlines
}
