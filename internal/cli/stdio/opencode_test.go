package stdio

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOpenCodeSpec_Properties(t *testing.T) {
	spec := OpenCodeSpec{}
	assert.Equal(t, "opencode", spec.Name())
	assert.Equal(t, PerTurnMode, spec.Mode())
	assert.Equal(t, "opencode", spec.Binary())
}

func TestOpenCodeSpec_BuildArgs_FirstTurn(t *testing.T) {
	spec := OpenCodeSpec{}
	args := spec.BuildArgs(StartOptions{Prompt: "fix the bug"})
	assert.Equal(t, []string{
		"run", "--format", "json", "fix the bug",
	}, args)
}

func TestOpenCodeSpec_BuildArgs_WithModel(t *testing.T) {
	spec := OpenCodeSpec{}
	args := spec.BuildArgs(StartOptions{Prompt: "hello", Model: "claude-4"})
	assert.Contains(t, args, "--model")
	assert.Contains(t, args, "claude-4")
}

func TestOpenCodeSpec_BuildArgs_WithWorkDir(t *testing.T) {
	spec := OpenCodeSpec{}
	args := spec.BuildArgs(StartOptions{Prompt: "hello", WorkDir: "/project"})
	assert.Contains(t, args, "--dir")
	assert.Contains(t, args, "/project")
}

func TestOpenCodeSpec_BuildArgs_Resume(t *testing.T) {
	spec := OpenCodeSpec{}
	args := spec.BuildArgs(StartOptions{Prompt: "continue", Resume: true})
	assert.Contains(t, args, "--continue")
}

func TestOpenCodeSpec_BuildArgs_AllOptions(t *testing.T) {
	spec := OpenCodeSpec{}
	args := spec.BuildArgs(StartOptions{
		Prompt:  "test",
		Resume:  true,
		Model:   "gpt-4",
		WorkDir: "/project",
	})
	assert.Contains(t, args, "run")
	assert.Contains(t, args, "--format")
	assert.Contains(t, args, "json")
	assert.Contains(t, args, "--continue")
	assert.Contains(t, args, "--dir")
	assert.Contains(t, args, "--model")
	assert.Contains(t, args, "test")
}

func TestOpenCodeSpec_BuildArgs_EmptyPrompt(t *testing.T) {
	spec := OpenCodeSpec{}
	args := spec.BuildArgs(StartOptions{})
	assert.Equal(t, []string{
		"run", "--format", "json", "",
	}, args)
}

func TestOpenCodeSpec_FormatInput(t *testing.T) {
	spec := OpenCodeSpec{}
	data, err := spec.FormatInput("hello")
	assert.NoError(t, err)
	assert.Nil(t, data)
}

func TestOpenCodeSpec_FormatPermissionResponse(t *testing.T) {
	spec := OpenCodeSpec{}
	data, err := spec.FormatPermissionResponse("req-1", "allow")
	assert.NoError(t, err)
	assert.Nil(t, data)
}

func TestOpenCodeSpec_ParseLine_PlainText(t *testing.T) {
	spec := OpenCodeSpec{}

	tests := []struct {
		name     string
		line     string
		expected string
	}{
		{"normal text", "Hello, world!", "Hello, world!"},
		{"code block", "```go\npackage main\n```", "```go\npackage main\n```"},
		{"unicode text", "你好世界", "你好世界"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			events, err := spec.ParseLine(tt.line)
			assert.NoError(t, err)
			assert.Len(t, events, 1)
			assert.Equal(t, EventResult, events[0].Type)
			assert.Equal(t, tt.expected, events[0].Text)
		})
	}
}

func TestOpenCodeSpec_ParseLine_EmptyString(t *testing.T) {
	spec := OpenCodeSpec{}
	events, err := spec.ParseLine("")
	assert.NoError(t, err)
	assert.Nil(t, events)
}

func TestOpenCodeSpec_ParseLine_JSONEvents(t *testing.T) {
	spec := OpenCodeSpec{}

	tests := []struct {
		name          string
		line          string
		expectEvents  int
		expectType    EventType
		expectText    string
		expectToolUse *ToolUseInfo
		expectSession string
		expectDone    bool
	}{
		{
			name:         "message type with text field",
			line:         `{"type":"message","text":"Hello from OpenCode"}`,
			expectEvents: 1,
			expectType:   EventResult,
			expectText:   "Hello from OpenCode",
		},
		{
			name:         "message type with content field",
			line:         `{"type":"message","content":"Alternative field"}`,
			expectEvents: 1,
			expectType:   EventResult,
			expectText:   "Alternative field",
		},
		{
			name:         "assistant type",
			line:         `{"type":"assistant","text":"Response text"}`,
			expectEvents: 1,
			expectType:   EventResult,
			expectText:   "Response text",
		},
		{
			name:         "text type",
			line:         `{"type":"text","text":"Direct text"}`,
			expectEvents: 1,
			expectType:   EventResult,
			expectText:   "Direct text",
		},
		{
			name:         "message type empty text ignored",
			line:         `{"type":"message","text":""}`,
			expectEvents: 0,
		},
		{
			name:         "tool_use type",
			line:         `{"type":"tool_use","name":"read_file","input":{"path":"main.go"}}`,
			expectEvents: 1,
			expectType:   EventToolUse,
			expectToolUse: &ToolUseInfo{
				Name:  "read_file",
				Input: `{"path":"main.go"}`,
			},
		},
		{
			name:         "tool type (alias)",
			line:         `{"type":"tool","name":"bash","input":{"cmd":"ls"}}`,
			expectEvents: 1,
			expectType:   EventToolUse,
			expectToolUse: &ToolUseInfo{
				Name:  "bash",
				Input: `{"cmd":"ls"}`,
			},
		},
		{
			name:         "result type",
			line:         `{"type":"result","text":"Final answer."}`,
			expectEvents: 1,
			expectType:   EventResult,
			expectText:   "Final answer.",
			expectDone:   true,
		},
		{
			name:         "done type",
			line:         `{"type":"done","text":"Completed."}`,
			expectEvents: 1,
			expectType:   EventResult,
			expectText:   "Completed.",
			expectDone:   true,
		},
		{
			name:         "complete type",
			line:         `{"type":"complete","content":"All done."}`,
			expectEvents: 1,
			expectType:   EventResult,
			expectText:   "All done.",
			expectDone:   true,
		},
		{
			name:         "error type with message",
			line:         `{"type":"error","message":"file not found"}`,
			expectEvents: 1,
			expectType:   EventError,
		},
		{
			name:         "error type with error field",
			line:         `{"type":"error","error":"connection lost"}`,
			expectEvents: 1,
			expectType:   EventError,
		},
		{
			name:         "error type with text field",
			line:         `{"type":"error","text":"some text error"}`,
			expectEvents: 1,
			expectType:   EventError,
		},
		{
			name:         "error type fallback",
			line:         `{"type":"error"}`,
			expectEvents: 1,
			expectType:   EventError,
		},
		{
			name:          "session type",
			line:          `{"type":"session","session_id":"sess-oc-123"}`,
			expectEvents:  1,
			expectSession: "sess-oc-123",
		},
		{
			name:          "init type",
			line:          `{"type":"init","session_id":"sess-init-456"}`,
			expectEvents:  1,
			expectSession: "sess-init-456",
		},
		{
			name:         "session type without id",
			line:         `{"type":"session"}`,
			expectEvents: 0,
		},
		{
			name:         "unknown type ignored",
			line:         `{"type":"progress","percent":50}`,
			expectEvents: 0,
		},
		{
			name:          "result with session_id",
			line:          `{"type":"result","text":"Done","session_id":"sess-res-789"}`,
			expectEvents:  1,
			expectType:    EventResult,
			expectSession: "sess-res-789",
			expectDone:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			events, err := spec.ParseLine(tt.line)
			assert.NoError(t, err)
			assert.Len(t, events, tt.expectEvents)

			if tt.expectEvents > 0 {
				evt := events[0]

				if tt.expectSession != "" {
					assert.Equal(t, tt.expectSession, evt.SessionID)
				}

				if tt.expectType != EventType(0) {
					assert.Equal(t, tt.expectType, evt.Type)
				}

				if tt.expectText != "" {
					assert.Equal(t, tt.expectText, evt.Text)
				}

				if tt.expectToolUse != nil {
					assert.NotNil(t, evt.ToolUse)
					assert.Equal(t, tt.expectToolUse.Name, evt.ToolUse.Name)
					assert.Contains(t, evt.ToolUse.Input, tt.expectToolUse.Input)
				}

				if tt.expectType == EventResult {
					assert.Equal(t, tt.expectDone, evt.Done)
				}
			}
		})
	}
}

func TestOpenCodeSpec_ParseLine_MixedJSONAndText(t *testing.T) {
	spec := OpenCodeSpec{}
	var results []string

	lines := []string{
		`{"type":"message","text":"Starting analysis..."}`,
		`Some plain text output`,
		`{"type":"tool_use","name":"read","input":{"path":"main.go"}}`,
		`More plain text`,
		`{"type":"result","text":"Done.","session_id":"sess-mix"}`,
	}

	var allEvents []Event
	for _, line := range lines {
		events, err := spec.ParseLine(line)
		assert.NoError(t, err)
		allEvents = append(allEvents, events...)
	}

	// message + plain text + tool_use + plain text + result
	assert.Len(t, allEvents, 5)

	// Collect results
	for _, evt := range allEvents {
		if evt.Type == EventResult && evt.Text != "" {
			results = append(results, evt.Text)
		}
	}
	assert.Contains(t, results, "Starting analysis...")
	assert.Contains(t, results, "Some plain text output")
	assert.Contains(t, results, "More plain text")
	assert.Contains(t, results, "Done.")

	// Session ID captured
	assert.Equal(t, "sess-mix", allEvents[4].SessionID)
}

func TestOpenCodeSpec_ParseLine_ResponseFieldPriority(t *testing.T) {
	spec := OpenCodeSpec{}

	// "text" field is tried first
	events, err := spec.ParseLine(`{"type":"message","text":"from text","content":"from content","message":"from message","response":"from response"}`)
	assert.NoError(t, err)
	assert.Equal(t, "from text", events[0].Text)

	// "content" when no "text"
	events, err = spec.ParseLine(`{"type":"message","content":"from content","message":"from message","response":"from response"}`)
	assert.NoError(t, err)
	assert.Equal(t, "from content", events[0].Text)

	// "message" when no "text" or "content"
	events, err = spec.ParseLine(`{"type":"message","message":"from message","response":"from response"}`)
	assert.NoError(t, err)
	assert.Equal(t, "from message", events[0].Text)

	// "response" when nothing else
	events, err = spec.ParseLine(`{"type":"message","response":"from response"}`)
	assert.NoError(t, err)
	assert.Equal(t, "from response", events[0].Text)
}

func TestOpenCodeSpec_ParseLine_LargePlainText(t *testing.T) {
	spec := OpenCodeSpec{}
	longText := strings.Repeat("c", 10000)
	events, err := spec.ParseLine(longText)
	assert.NoError(t, err)
	assert.Len(t, events, 1)
	assert.Equal(t, longText, events[0].Text)
}

func TestOpenCodeSpec_InterfaceCompliance(t *testing.T) {
	var _ CLISpec = OpenCodeSpec{}
}
