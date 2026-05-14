package stdio

import (
	"encoding/json"
	"fmt"
)

// GeminiSpec implements CLISpec for Google Gemini CLI.
// Gemini uses per-turn mode: each invocation is a separate process.
// Session continuity is achieved via --resume latest.
//
// Launch pattern:
//
//	First turn:  gemini --output-format stream-json -p PROMPT
//	Resume:      gemini --resume latest --output-format stream-json -p PROMPT
type GeminiSpec struct{}

const defaultPromptPlaceholder = " " // Non-empty placeholder to satisfy CLI arg requirements

func (GeminiSpec) Name() string    { return "gemini" }
func (GeminiSpec) Mode() StdioMode { return PerTurnMode }
func (GeminiSpec) Binary() string  { return "gemini" }

func (GeminiSpec) BuildArgs(opts StartOptions) []string {
	prompt := opts.Prompt
	if prompt == "" {
		prompt = defaultPromptPlaceholder
	}
	args := []string{"-p", prompt, "--output-format", "stream-json"}
	if opts.Yolo {
		args = append(args, "--yolo")
	}
	if opts.Resume {
		args = append(args, "--resume", "latest")
	}
	if opts.Model != "" {
		args = append(args, "--model", opts.Model)
	}
	return args
}

// FormatInput returns nil — Gemini takes prompt via -p flag, not stdin.
func (GeminiSpec) FormatInput(message string) ([]byte, error) {
	return nil, nil
}

// FormatPermissionResponse is not applicable for per-turn mode.
func (GeminiSpec) FormatPermissionResponse(requestID, optionID string) ([]byte, error) {
	return nil, nil
}

// ParseLine parses Gemini stream-json NDJSON events.
// Event types: init, message, tool_use, tool_result, result, error.
func (GeminiSpec) ParseLine(line string) ([]Event, error) {
	var raw map[string]any
	if err := json.Unmarshal([]byte(line), &raw); err != nil {
		return nil, fmt.Errorf("parse JSON: %w", err)
	}

	eventType, _ := raw["type"].(string)
	switch eventType {
	case "init":
		return parseGeminiInit(raw), nil
	case "message":
		return parseGeminiMessage(raw), nil
	case "tool_use":
		return parseGeminiToolUse(raw), nil
	case "result":
		return parseGeminiResult(raw), nil
	case "error":
		msg, _ := raw["message"].(string)
		if msg == "" {
			msg, _ = raw["error"].(string)
		}
		if msg == "" {
			msg = "unknown error"
		}
		return []Event{{Type: EventError, Error: fmt.Errorf("gemini: %s", msg)}}, nil
	default:
		return nil, nil
	}
}

func parseGeminiInit(raw map[string]any) []Event {
	sess, _ := raw["session"].(map[string]any)
	if sess == nil {
		return nil
	}
	sessionID, _ := sess["id"].(string)
	if sessionID == "" {
		return nil
	}
	return []Event{{SessionID: sessionID}}
}

func parseGeminiMessage(raw map[string]any) []Event {
	role, _ := raw["role"].(string)
	if role != "assistant" {
		return nil
	}
	text, _ := raw["text"].(string)
	if text == "" {
		return nil
	}
	return []Event{{Type: EventText, Text: text}}
}

func parseGeminiToolUse(raw map[string]any) []Event {
	name, _ := raw["name"].(string)
	if name == "" {
		return nil
	}
	input := summarizeGeminiArgs(name, raw["args"])
	return []Event{
		{
			Type:    EventToolUse,
			ToolUse: &ToolUseInfo{Name: name, Input: input},
		},
	}
}

func parseGeminiResult(raw map[string]any) []Event {
	response, _ := raw["response"].(string)
	var sessionID string
	if sess, ok := raw["session"].(map[string]any); ok {
		sessionID, _ = sess["id"].(string)
	}
	return []Event{{
		Type:      EventResult,
		Text:      response,
		Done:      true,
		SessionID: sessionID,
	}}
}

// summarizeGeminiArgs creates a short summary of tool arguments for display.
func summarizeGeminiArgs(toolName string, args any) string {
	if args == nil {
		return ""
	}
	return summarizeInput(toolName, args)
}

var _ CLISpec = GeminiSpec{}
