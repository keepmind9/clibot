package stdio

import (
	"encoding/json"
	"fmt"
)

// OpenCodeSpec implements CLISpec for OpenCode CLI.
// OpenCode uses per-turn mode: each invocation is a separate process.
// Session continuity is achieved via --continue flag.
//
// Launch pattern:
//
//	First turn:  opencode run --format json PROMPT
//	Resume:      opencode run --format json --continue PROMPT
type OpenCodeSpec struct{}

func (OpenCodeSpec) Name() string    { return "opencode" }
func (OpenCodeSpec) Mode() StdioMode { return PerTurnMode }
func (OpenCodeSpec) Binary() string  { return "opencode" }

func (OpenCodeSpec) BuildArgs(opts StartOptions) []string {
	args := []string{"run", "--format", "json"}
	// TODO: add yolo flag when OpenCode officially supports it (e.g., --yolo)
	if opts.Resume {
		args = append(args, "--continue")
	}
	if opts.WorkDir != "" {
		args = append(args, "--dir", opts.WorkDir)
	}
	if opts.Model != "" {
		args = append(args, "--model", opts.Model)
	}
	prompt := opts.Prompt
	if prompt == "" {
		prompt = defaultPromptPlaceholder
	}
	args = append(args, prompt)
	return args
}

// FormatInput returns nil — OpenCode takes prompt as positional arg, not stdin.
func (OpenCodeSpec) FormatInput(message string) ([]byte, error) {
	return nil, nil
}

// FormatPermissionResponse is not applicable for per-turn mode.
func (OpenCodeSpec) FormatPermissionResponse(requestID, optionID string) ([]byte, error) {
	return nil, nil
}

// ParseLine parses OpenCode JSON output events.
// Falls back to plain text if the line is not valid JSON.
func (OpenCodeSpec) ParseLine(line string) ([]Event, error) {
	if line == "" {
		return nil, nil
	}

	var raw map[string]any
	if err := json.Unmarshal([]byte(line), &raw); err != nil {
		// Not JSON — treat as plain text output
		return []Event{{Type: EventResult, Text: line}}, nil
	}

	eventType, _ := raw["type"].(string)
	switch eventType {
	case "message", "assistant", "text":
		return parseOpenCodeMessage(raw), nil
	case "tool_use", "tool":
		return parseOpenCodeToolUse(raw), nil
	case "result", "done", "complete":
		return parseOpenCodeResult(raw), nil
	case "error":
		msg := extractOpenCodeError(raw)
		return []Event{{Type: EventError, Error: fmt.Errorf("opencode: %s", msg)}}, nil
	case "session", "init":
		return parseOpenCodeSession(raw), nil
	default:
		return nil, nil
	}
}

func parseOpenCodeMessage(raw map[string]any) []Event {
	text := extractText(raw)
	if text == "" {
		return nil
	}
	return []Event{{Type: EventResult, Text: text}}
}

func parseOpenCodeToolUse(raw map[string]any) []Event {
	name, _ := raw["name"].(string)
	if name == "" {
		return nil
	}
	input := summarizeInput(name, raw["input"])
	return []Event{
		{
			Type:    EventToolUse,
			ToolUse: &ToolUseInfo{Name: name, Input: input},
		},
	}
}

func parseOpenCodeResult(raw map[string]any) []Event {
	text := extractText(raw)
	var sessionID string
	if id, ok := raw["session_id"].(string); ok {
		sessionID = id
	}
	return []Event{{
		Type:      EventResult,
		Text:      text,
		Done:      true,
		SessionID: sessionID,
	}}
}

func parseOpenCodeSession(raw map[string]any) []Event {
	sessionID, _ := raw["session_id"].(string)
	if sessionID == "" {
		return nil
	}
	return []Event{{SessionID: sessionID}}
}

// extractText tries common field names for text content.
func extractText(raw map[string]any) string {
	for _, key := range []string{"text", "content", "message", "response"} {
		if v, ok := raw[key].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

// extractOpenCodeError tries common field names for error messages.
func extractOpenCodeError(raw map[string]any) string {
	for _, key := range []string{"message", "error", "text"} {
		if v, ok := raw[key].(string); ok && v != "" {
			return v
		}
	}
	return "unknown error"
}

var _ CLISpec = OpenCodeSpec{}
