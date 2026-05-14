package stdio

import (
	"encoding/json"
	"fmt"
)

// CodexSpec implements CLISpec for OpenAI Codex CLI.
// Codex uses per-turn mode: each invocation is a separate process.
// Session continuity is achieved via `codex exec resume --last`.
//
// Launch pattern:
//
//	First turn:  codex exec --json -          (reads prompt from stdin)
//	Resume:      codex exec resume --last --json -
type CodexSpec struct{}

func (CodexSpec) Name() string    { return "codex" }
func (CodexSpec) Mode() StdioMode { return PerTurnMode }
func (CodexSpec) Binary() string  { return "codex" }

func (CodexSpec) BuildArgs(opts StartOptions) []string {
	args := []string{"exec", "--json", "--skip-git-repo-check"}
	if opts.Yolo {
		args = append(args, "--full-auto")
	}
	if opts.WorkDir != "" {
		args = append(args, "--cd", opts.WorkDir)
	}
	if opts.Resume {
		args = append(args, "resume")
		if opts.SessionID != "" {
			args = append(args, opts.SessionID)
		} else {
			args = append(args, "--last")
		}
	}
	args = append(args, "-") // read prompt from stdin
	if opts.Model != "" {
		args = append(args, "--model", opts.Model)
	}
	return args
}

func (CodexSpec) FormatInput(message string) ([]byte, error) {
	return []byte(message), nil
}

// FormatPermissionResponse is not applicable for per-turn mode.
func (CodexSpec) FormatPermissionResponse(requestID, optionID string) ([]byte, error) {
	return nil, nil
}

// ParseLine parses Codex JSONL output events.
// Event types: thread.started, turn.started, item.completed, turn.completed, turn.failed, error.
func (CodexSpec) ParseLine(line string) ([]Event, error) {
	var raw map[string]any
	if err := json.Unmarshal([]byte(line), &raw); err != nil {
		return nil, fmt.Errorf("parse JSON: %w", err)
	}

	eventType, _ := raw["type"].(string)
	switch eventType {
	case "thread.started":
		return parseCodexThreadStarted(raw), nil
	case "item.completed":
		return parseCodexItem(raw), nil
	case "turn.completed":
		return []Event{{Type: EventResult, Done: true}}, nil
	case "turn.failed":
		return []Event{{Type: EventError, Error: fmt.Errorf("codex: turn failed")}}, nil
	case "error":
		msg, _ := raw["message"].(string)
		if msg == "" {
			msg = "unknown error"
		}
		return []Event{{Type: EventError, Error: fmt.Errorf("codex: %s", msg)}}, nil
	default:
		return nil, nil
	}
}

func parseCodexThreadStarted(raw map[string]any) []Event {
	threadID, _ := raw["thread_id"].(string)
	if threadID == "" {
		return nil
	}
	return []Event{{SessionID: threadID}}
}

func parseCodexItem(raw map[string]any) []Event {
	item, _ := raw["item"].(map[string]any)
	if item == nil {
		return nil
	}

	itemType, _ := item["type"].(string)
	switch itemType {
	case "agent_message":
		text, _ := item["text"].(string)
		if text == "" {
			return nil
		}
		return []Event{{Type: EventResult, Text: text}}
	case "command_execution":
		name, _ := item["command"].(string)
		if name == "" {
			name = "command"
		}
		return []Event{
			{
				Type:    EventToolUse,
				ToolUse: &ToolUseInfo{Name: "shell", Input: name},
			},
		}
	default:
		return nil
	}
}

var _ CLISpec = CodexSpec{}
