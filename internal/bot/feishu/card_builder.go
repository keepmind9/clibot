package feishu

import (
	"fmt"
	"strings"

	"github.com/keepmind9/clibot/internal/bot"
	larkcardkit "github.com/larksuite/oapi-sdk-go/v3/service/cardkit/v1"
)

const (
	maxFieldChars    = 600
	maxThinkingChars = 1500
)

// buildBatchUpdateActions converts ContentBlocks into a single update_element action.
// All blocks are rendered into one markdown string to replace the main_content element.
func buildBatchUpdateActions(blocks []bot.ContentBlock, existingToolCount int) ([]larkcardkit.Action, int) {
	if len(blocks) == 0 {
		return nil, existingToolCount
	}

	content := renderBlocksToMarkdown(blocks)

	toolCount := existingToolCount
	for _, block := range blocks {
		if block.Type == bot.ContentBlockToolCall {
			toolCount++
		}
	}

	element := fmt.Sprintf(`{"tag":"markdown","element_id":"main_content","content":%q}`, content)

	return []larkcardkit.Action{{
		Action: strPtr("update_element"),
		Params: &larkcardkit.Params{
			ElementId: strPtr("main_content"),
			Element:   strPtr(element),
		},
	}}, toolCount
}

// renderBlocksToMarkdown concatenates all blocks into a single markdown string.
func renderBlocksToMarkdown(blocks []bot.ContentBlock) string {
	var sb strings.Builder
	for i, block := range blocks {
		if i > 0 {
			sb.WriteString("\n")
		}
		switch block.Type {
		case bot.ContentBlockText:
			sb.WriteString(truncateString(block.Content, maxFieldChars))
		case bot.ContentBlockToolCall:
			summary := toolSummary(block.Title, block.Meta)
			sb.WriteString(fmt.Sprintf("**[%s]**", truncateString(summary, maxFieldChars)))
		case bot.ContentBlockToolResult:
			sb.WriteString(truncateString(block.Content, maxFieldChars))
		case bot.ContentBlockThinking:
			sb.WriteString(truncateString(block.Content, maxThinkingChars))
		case bot.ContentBlockStatus:
			sb.WriteString(truncateString(block.Content, maxFieldChars))
			sb.WriteString("\n")
		}
	}
	return strings.TrimSpace(sb.String())
}

// toolSummary generates a short summary for a tool call based on tool name and metadata.
func toolSummary(name string, meta map[string]string) string {
	switch name {
	case "Bash":
		if cmd, ok := meta["command"]; ok {
			return truncateString(cmd, 80)
		}
	case "Read", "Edit", "Write", "NotebookEdit":
		if path, ok := meta["file_path"]; ok {
			return fmt.Sprintf("%s: %s", name, truncateString(path, 60))
		}
	case "Grep":
		pat := meta["pattern"]
		p := meta["path"]
		if p != "" {
			return fmt.Sprintf("Grep: %s in %s", truncateString(pat, 40), truncateString(p, 30))
		}
		if pat != "" {
			return "Grep: " + truncateString(pat, 60)
		}
	case "Glob":
		if pattern, ok := meta["pattern"]; ok {
			return "Glob: " + truncateString(pattern, 60)
		}
	case "WebFetch":
		if url, ok := meta["url"]; ok {
			return "WebFetch: " + truncateString(url, 60)
		}
	case "WebSearch":
		if q, ok := meta["query"]; ok {
			return "WebSearch: " + truncateString(q, 60)
		}
	case "Agent", "Task":
		if desc, ok := meta["description"]; ok {
			return fmt.Sprintf("%s: %s", name, truncateString(desc, 60))
		}
		if sub, ok := meta["subagent_type"]; ok {
			return fmt.Sprintf("%s: %s", name, truncateString(sub, 60))
		}
	default:
		for _, key := range []string{"command", "file_path", "path", "query"} {
			if v, ok := meta[key]; ok {
				return fmt.Sprintf("%s: %s", name, truncateString(v, 60))
			}
		}
	}
	if name != "" {
		return name
	}
	return "Tool"
}

func sanitizeID(s string) string {
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
