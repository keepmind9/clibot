package feishu

import (
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
	assert.NotNil(t, actions[0].Action)
	assert.Equal(t, "partial_update_element", *actions[0].Action)
}

func TestBuildBatchUpdateActions_ToolCallBlock(t *testing.T) {
	blocks := []bot.ContentBlock{
		{Type: bot.ContentBlockToolCall, Title: "Bash", Meta: map[string]string{"command": "ls"}},
	}

	actions, toolCount := buildBatchUpdateActions(blocks, 0)
	assert.Len(t, actions, 1)
	assert.Equal(t, 1, toolCount)
	assert.Equal(t, "add_elements", *actions[0].Action)
}

func TestBuildBatchUpdateActions_AutoCollapse(t *testing.T) {
	var blocks []bot.ContentBlock
	for i := 0; i < 6; i++ {
		blocks = append(blocks, bot.ContentBlock{
			Type:  bot.ContentBlockToolCall,
			Title: "Bash",
			Meta:  map[string]string{"command": "cmd"},
		})
	}

	actions, toolCount := buildBatchUpdateActions(blocks, 0)
	assert.Equal(t, 6, toolCount)
	assert.Len(t, actions, 6)
}

func TestBuildBatchUpdateActions_ThinkingBlock(t *testing.T) {
	blocks := []bot.ContentBlock{
		{Type: bot.ContentBlockThinking, Content: "Let me think about this..."},
	}

	actions, _ := buildBatchUpdateActions(blocks, 0)
	assert.Len(t, actions, 1)
	assert.Equal(t, "add_elements", *actions[0].Action)
}

func TestBuildBatchUpdateActions_StatusBlock(t *testing.T) {
	blocks := []bot.ContentBlock{
		{Type: bot.ContentBlockStatus, Content: "Token usage: 100"},
	}

	actions, _ := buildBatchUpdateActions(blocks, 0)
	assert.Len(t, actions, 1)
	assert.Equal(t, "partial_update_element", *actions[0].Action)
}

func TestBuildBatchUpdateActions_ToolResultBlock(t *testing.T) {
	blocks := []bot.ContentBlock{
		{Type: bot.ContentBlockToolResult, Content: "file contents here"},
	}

	actions, _ := buildBatchUpdateActions(blocks, 0)
	assert.Len(t, actions, 1)
	assert.Equal(t, "partial_update_element", *actions[0].Action)
}

func TestBuildBatchUpdateActions_Empty(t *testing.T) {
	actions, toolCount := buildBatchUpdateActions(nil, 5)
	assert.Len(t, actions, 0)
	assert.Equal(t, 5, toolCount)
}
