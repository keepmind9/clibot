package stdio

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ClaudeSpec implements CLISpec for Claude Code.
// Claude Code uses a persistent process with bidirectional JSON over stdio.
//
// Launch flags:
//
//	--output-format stream-json      → stdout emits NDJSON events
//	--input-format stream-json       → stdin accepts NDJSON messages
//	--permission-prompt-tool stdio   → permission requests as JSON on stdout
type ClaudeSpec struct{}

func (ClaudeSpec) Name() string    { return "claude" }
func (ClaudeSpec) Mode() StdioMode { return PersistentMode }
func (ClaudeSpec) Binary() string  { return "claude" }

func (ClaudeSpec) BuildArgs(opts StartOptions) []string {
	return []string{
		"--output-format", "stream-json",
		"--input-format", "stream-json",
		"--permission-prompt-tool", "stdio",
		"--verbose",
	}
}

func (ClaudeSpec) FormatInput(message string) ([]byte, error) {
	msg := map[string]any{
		"type": "user",
		"message": map[string]any{
			"role":    "user",
			"content": message,
		},
	}
	return json.Marshal(msg)
}

// FormatPermissionResponse writes a control_response for allow or deny.
// optionID "allow" → allow, anything else → deny.
func (ClaudeSpec) FormatPermissionResponse(requestID string, optionID string) ([]byte, error) {
	var response map[string]any
	if optionID == "allow" {
		response = map[string]any{
			"behavior":     "allow",
			"updatedInput": map[string]any{},
		}
	} else {
		response = map[string]any{
			"behavior": "deny",
			"message":  "Denied by user.",
		}
	}

	controlResp := map[string]any{
		"type": "control_response",
		"response": map[string]any{
			"subtype":    "success",
			"request_id": requestID,
			"response":   response,
		},
	}
	return json.Marshal(controlResp)
}

// ParseLine parses a single JSON line from Claude Code's stdout.
func (ClaudeSpec) ParseLine(line string) ([]Event, error) {
	var raw map[string]any
	if err := json.Unmarshal([]byte(line), &raw); err != nil {
		return nil, fmt.Errorf("parse JSON: %w", err)
	}

	eventType, _ := raw["type"].(string)
	switch eventType {
	case "system":
		return nil, nil // session_id capture not needed at this layer
	case "assistant":
		return parseAssistant(raw), nil
	case "result":
		return parseResult(raw), nil
	case "control_request":
		return parseControlRequest(raw), nil
	case "control_cancel_request":
		return nil, nil // permission cancelled, nothing to do
	default:
		return nil, nil
	}
}

func parseAssistant(raw map[string]any) []Event {
	msg, ok := raw["message"].(map[string]any)
	if !ok {
		return nil
	}
	contentArr, ok := msg["content"].([]any)
	if !ok {
		return nil
	}

	var events []Event
	for _, item := range contentArr {
		c, ok := item.(map[string]any)
		if !ok {
			continue
		}
		contentType, _ := c["type"].(string)
		switch contentType {
		case "text":
			if text, _ := c["text"].(string); text != "" {
				events = append(events, Event{Type: EventText, Text: text})
			}
		case "tool_use":
			name, _ := c["name"].(string)
			input := summarizeInput(name, c["input"])
			events = append(events, Event{
				Type:    EventToolUse,
				ToolUse: &ToolUseInfo{Name: name, Input: input},
			})
		}
	}
	return events
}

func parseResult(raw map[string]any) []Event {
	result, _ := raw["result"].(string)
	return []Event{{
		Type: EventResult,
		Text: result,
		Done: true,
	}}
}

func parseControlRequest(raw map[string]any) []Event {
	requestID, _ := raw["request_id"].(string)
	request, _ := raw["request"].(map[string]any)
	if request == nil {
		return nil
	}

	subtype, _ := request["subtype"].(string)
	if subtype != "can_use_tool" {
		return nil
	}

	toolName, _ := request["tool_name"].(string)
	input := summarizeInput(toolName, request["input"])

	return []Event{{
		Type: EventPermission,
		Permission: &PermissionRequest{
			RequestID: requestID,
			ToolName:  toolName,
			Input:     input,
			Options: []PermissionOption{
				{ID: "allow", Text: "Allow"},
				{ID: "deny", Text: "Deny"},
			},
		},
	}}
}

// summarizeInput creates a short summary of a tool input for display.
func summarizeInput(toolName string, input any) string {
	if input == nil {
		return ""
	}

	switch v := input.(type) {
	case string:
		if len(v) > summarizeMaxRunes {
			return truncateRunes(v, summarizeMaxRunes) + "..."
		}
		return v
	case map[string]any:
		// For common tools, extract the most useful field
		switch toolName {
		case "Bash":
			if cmd, ok := v["command"].(string); ok {
				return cmd
			}
		case "Edit", "Write":
			if path, ok := v["file_path"].(string); ok {
				return path
			}
		}
		// Generic: JSON-encode truncated
		b, _ := json.Marshal(v)
		s := string(b)
		if len(s) > summarizeMaxRunes {
			return truncateRunes(s, summarizeMaxRunes) + "..."
		}
		return s
	default:
		s := fmt.Sprintf("%v", input)
		if len(s) > summarizeMaxRunes {
			return truncateRunes(s, summarizeMaxRunes) + "..."
		}
		return s
	}
}

const summarizeMaxRunes = 300

// Ensure ClaudeSpec implements CLISpec.
var _ CLISpec = ClaudeSpec{}

// NeedsStdioAdapter is a marker interface that adapters can implement
// to indicate they need the stdio permission flow.
// The engine checks for this interface to route permission responses.
type NeedsStdioAdapter interface {
	GetPendingPermission(sessionName string) *PendingPermission
	RespondPermission(sessionName, requestID, optionID string) error
}

// IsStdioCLIType checks if a cli_type string refers to a stdio-mode adapter.
func IsStdioCLIType(cliType string) bool {
	return strings.HasSuffix(cliType, "-stdio")
}
