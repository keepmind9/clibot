package feishu

import (
	"fmt"

	"github.com/keepmind9/clibot/internal/bot"
	larkcardkit "github.com/larksuite/oapi-sdk-go/v3/service/cardkit/v1"
)

const (
	maxFieldChars         = 600
	maxThinkingChars      = 1500
	toolCollapseThreshold = 5
)

// buildBatchUpdateActions converts ContentBlocks into CardKit batch update action list.
// Returns the actions list and updated tool count.
func buildBatchUpdateActions(blocks []bot.ContentBlock, existingToolCount int) ([]larkcardkit.Action, int) {
	var actions []larkcardkit.Action
	toolCount := existingToolCount

	for _, block := range blocks {
		switch block.Type {
		case bot.ContentBlockText:
			actions = append(actions, buildTextAction(block))

		case bot.ContentBlockToolCall:
			toolCount++
			actions = append(actions, buildToolCallAction(block, toolCount > toolCollapseThreshold))

		case bot.ContentBlockToolResult:
			actions = append(actions, buildToolResultAction(block))

		case bot.ContentBlockThinking:
			actions = append(actions, buildThinkingAction(block))

		case bot.ContentBlockStatus:
			actions = append(actions, buildStatusAction(block))
		}
	}

	return actions, toolCount
}

func buildTextAction(block bot.ContentBlock) larkcardkit.Action {
	content := truncateString(block.Content, maxFieldChars)
	partial := fmt.Sprintf(`{"content":%q}`, content)

	return larkcardkit.Action{
		Action: strPtr("partial_update_element"),
		Params: &larkcardkit.Params{
			ElementId:      strPtr("main_content"),
			PartialElement: strPtr(partial),
		},
	}
}

func buildToolCallAction(block bot.ContentBlock, collapsed bool) larkcardkit.Action {
	summary := toolSummary(block.Title, block.Meta)
	safeName := sanitizeID(block.Title)
	if safeName == "" {
		safeName = "tool"
	}

	collapsedAttr := ""
	if collapsed {
		collapsedAttr = `,"collapsed":true`
	}

	element := fmt.Sprintf(
		`{"tag":"collapsible_panel","element_id":%q,"title":{"content":%q,"tag":"plain_text"}%s}`,
		safeName, truncateString(summary, maxFieldChars), collapsedAttr,
	)

	return larkcardkit.Action{
		Action: strPtr("add_elements"),
		Params: &larkcardkit.Params{
			Type:     strPtr("append"),
			Elements: []string{element},
		},
	}
}

func buildToolResultAction(block bot.ContentBlock) larkcardkit.Action {
	content := truncateString(block.Content, maxFieldChars)
	partial := fmt.Sprintf(`{"content":%q}`, content)

	return larkcardkit.Action{
		Action: strPtr("partial_update_element"),
		Params: &larkcardkit.Params{
			ElementId:      strPtr("main_content"),
			PartialElement: strPtr(partial),
		},
	}
}

func buildThinkingAction(block bot.ContentBlock) larkcardkit.Action {
	content := truncateString(block.Content, maxThinkingChars)
	element := fmt.Sprintf(
		`{"tag":"collapsible_panel","element_id":"thinking","title":{"content":"Thinking","tag":"plain_text"},"collapsed":true,"content":{"tag":"markdown","content":%q}}`,
		content,
	)

	return larkcardkit.Action{
		Action: strPtr("add_elements"),
		Params: &larkcardkit.Params{
			Type:     strPtr("append"),
			Elements: []string{element},
		},
	}
}

func buildStatusAction(block bot.ContentBlock) larkcardkit.Action {
	partial := fmt.Sprintf(`{"content":%q}`, truncateString(block.Content, maxFieldChars))

	return larkcardkit.Action{
		Action: strPtr("partial_update_element"),
		Params: &larkcardkit.Params{
			ElementId:      strPtr("main_content"),
			PartialElement: strPtr(partial),
		},
	}
}

// toolSummary generates a short summary for a tool call based on tool name and metadata.
func toolSummary(name string, meta map[string]string) string {
	switch name {
	case "Bash":
		if cmd, ok := meta["command"]; ok {
			return truncateString(cmd, 80)
		}
	case "Read", "Edit", "Write":
		if path, ok := meta["file_path"]; ok {
			return fmt.Sprintf("%s: %s", name, truncateString(path, 60))
		}
	case "Grep", "Glob":
		if pattern, ok := meta["pattern"]; ok {
			return fmt.Sprintf("%s: %s", name, truncateString(pattern, 60))
		}
	}
	if name != "" {
		return name
	}
	return "Tool"
}

func sanitizeID(s string) string {
	// Keep only alphanumeric, underscore, dash
	var result []byte
	for _, c := range s {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '-' {
			result = append(result, byte(c))
		}
	}
	if len(result) > 32 {
		result = result[:32]
	}
	return string(result)
}

func strPtr(s string) *string { return &s }
