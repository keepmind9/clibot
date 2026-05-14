package stdio

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGeminiSpec_Properties(t *testing.T) {
	spec := GeminiSpec{}
	assert.Equal(t, "gemini", spec.Name())
	assert.Equal(t, PerTurnMode, spec.Mode())
	assert.Equal(t, "gemini", spec.Binary())
}

func TestGeminiSpec_BuildArgs_FirstTurn(t *testing.T) {
	spec := GeminiSpec{}
	args := spec.BuildArgs(StartOptions{Prompt: "explain this code"})
	assert.Equal(t, []string{
		"-p", "explain this code",
		"--output-format", "stream-json",
	}, args)
}

func TestGeminiSpec_BuildArgs_WithModel(t *testing.T) {
	spec := GeminiSpec{}
	args := spec.BuildArgs(StartOptions{Prompt: "hello", Model: "flash"})
	assert.Contains(t, args, "--model")
	assert.Contains(t, args, "flash")
}

func TestGeminiSpec_BuildArgs_Resume(t *testing.T) {
	spec := GeminiSpec{}
	args := spec.BuildArgs(StartOptions{Prompt: "continue", Resume: true})
	assert.Contains(t, args, "--resume")
	assert.Contains(t, args, "latest")
}

func TestGeminiSpec_BuildArgs_AllOptions(t *testing.T) {
	spec := GeminiSpec{}
	args := spec.BuildArgs(StartOptions{
		Prompt: "test",
		Resume: true,
		Model:  "pro",
	})
	assert.Contains(t, args, "-p")
	assert.Contains(t, args, "test")
	assert.Contains(t, args, "--output-format")
	assert.Contains(t, args, "stream-json")
	assert.Contains(t, args, "--resume")
	assert.Contains(t, args, "--model")
}

func TestGeminiSpec_BuildArgs_EmptyPrompt(t *testing.T) {
	spec := GeminiSpec{}
	args := spec.BuildArgs(StartOptions{})
	assert.Equal(t, []string{
		"-p", " ",
		"--output-format", "stream-json",
	}, args)
}

func TestGeminiSpec_FormatInput(t *testing.T) {
	spec := GeminiSpec{}
	data, err := spec.FormatInput("hello")
	assert.NoError(t, err)
	assert.Nil(t, data)
}

func TestGeminiSpec_FormatPermissionResponse(t *testing.T) {
	spec := GeminiSpec{}
	data, err := spec.FormatPermissionResponse("req-1", "allow")
	assert.NoError(t, err)
	assert.Nil(t, data)
}

func TestGeminiSpec_ParseLine(t *testing.T) {
	spec := GeminiSpec{}

	tests := []struct {
		name          string
		line          string
		expectErr     bool
		expectEvents  int
		expectType    EventType
		expectText    string
		expectToolUse *ToolUseInfo
		expectSession string
		expectDone    bool
	}{
		{
			name:          "init event with session",
			line:          `{"type":"init","session":{"id":"sess-abc-123"},"model":"gemini-2.5-flash"}`,
			expectEvents:  1,
			expectSession: "sess-abc-123",
		},
		{
			name:         "init event without session",
			line:         `{"type":"init","model":"gemini-2.5-flash"}`,
			expectEvents: 0,
		},
		{
			name:         "init event with empty session id",
			line:         `{"type":"init","session":{"id":""}}`,
			expectEvents: 0,
		},
		{
			name:         "assistant message",
			line:         `{"type":"message","role":"assistant","text":"Here is the analysis."}`,
			expectEvents: 1,
			expectType:   EventText,
			expectText:   "Here is the analysis.",
		},
		{
			name:         "user message ignored",
			line:         `{"type":"message","role":"user","text":"hello"}`,
			expectEvents: 0,
		},
		{
			name:         "message empty text ignored",
			line:         `{"type":"message","role":"assistant","text":""}`,
			expectEvents: 0,
		},
		{
			name:         "tool_use event",
			line:         `{"type":"tool_use","name":"read_file","args":{"path":"/tmp/file.go"}}`,
			expectEvents: 1,
			expectType:   EventToolUse,
			expectToolUse: &ToolUseInfo{
				Name:  "read_file",
				Input: `{"path":"/tmp/file.go"}`,
			},
		},
		{
			name:         "tool_use with nil args",
			line:         `{"type":"tool_use","name":"list_files"}`,
			expectEvents: 1,
			expectType:   EventToolUse,
			expectToolUse: &ToolUseInfo{
				Name:  "list_files",
				Input: "",
			},
		},
		{
			name:         "tool_use with empty name ignored",
			line:         `{"type":"tool_use","name":""}`,
			expectEvents: 0,
		},
		{
			name:         "result event",
			line:         `{"type":"result","response":"The code is clean."}`,
			expectEvents: 1,
			expectType:   EventResult,
			expectText:   "The code is clean.",
			expectDone:   true,
		},
		{
			name:          "result event with session",
			line:          `{"type":"result","response":"Done.","session":{"id":"sess-xyz"}}`,
			expectEvents:  1,
			expectType:    EventResult,
			expectText:    "Done.",
			expectSession: "sess-xyz",
			expectDone:    true,
		},
		{
			name:         "error event with message",
			line:         `{"type":"error","message":"API rate limit"}`,
			expectEvents: 1,
			expectType:   EventError,
		},
		{
			name:         "error event with error field",
			line:         `{"type":"error","error":"timeout"}`,
			expectEvents: 1,
			expectType:   EventError,
		},
		{
			name:         "error event without message",
			line:         `{"type":"error"}`,
			expectEvents: 1,
			expectType:   EventError,
		},
		{
			name:         "unknown type ignored",
			line:         `{"type":"tool_result","name":"read_file"}`,
			expectEvents: 0,
		},
		{
			name:      "invalid JSON",
			line:      `{not json}`,
			expectErr: true,
		},
		{
			name:      "empty string",
			line:      "",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			events, err := spec.ParseLine(tt.line)
			if tt.expectErr {
				assert.Error(t, err)
				return
			}
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
					// Input is JSON-encoded, check contains key
					assert.Contains(t, evt.ToolUse.Input, tt.expectToolUse.Input)
				}

				if tt.expectType == EventResult {
					assert.Equal(t, tt.expectDone, evt.Done)
				}
			}
		})
	}
}

func TestGeminiSpec_ParseLine_MultipleAssistantMessages(t *testing.T) {
	spec := GeminiSpec{}

	line1 := `{"type":"message","role":"assistant","text":"Step 1: "}`
	line2 := `{"type":"message","role":"assistant","text":"Step 2."}`
	line3 := `{"type":"result","response":"All steps done."}`

	events1, err := spec.ParseLine(line1)
	assert.NoError(t, err)
	assert.Equal(t, EventText, events1[0].Type)
	assert.Equal(t, "Step 1: ", events1[0].Text)

	events2, err := spec.ParseLine(line2)
	assert.NoError(t, err)
	assert.Equal(t, EventText, events2[0].Type)

	events3, err := spec.ParseLine(line3)
	assert.NoError(t, err)
	assert.Equal(t, EventResult, events3[0].Type)
	assert.Equal(t, "All steps done.", events3[0].Text)
	assert.True(t, events3[0].Done)
}

func TestGeminiSpec_ParseLine_RealWorldSequence(t *testing.T) {
	spec := GeminiSpec{}
	var allEvents []Event

	lines := []string{
		`{"type":"init","session":{"id":"sess-001"},"model":"gemini-2.5-flash"}`,
		`{"type":"message","role":"assistant","text":"I'll check the files."}`,
		`{"type":"tool_use","name":"read_file","args":{"path":"main.go"}}`,
		`{"type":"tool_result","name":"read_file","output":"package main\n..."}`,
		`{"type":"message","role":"assistant","text":"Found the issue."}`,
		`{"type":"result","response":"The issue is on line 42."}`,
	}

	for _, line := range lines {
		events, err := spec.ParseLine(line)
		assert.NoError(t, err)
		allEvents = append(allEvents, events...)
	}

	// init(session) + 2 messages + 1 tool_use + 1 result
	assert.Len(t, allEvents, 5)

	// Session ID from init
	assert.Equal(t, "sess-001", allEvents[0].SessionID)

	// First assistant message
	assert.Equal(t, EventText, allEvents[1].Type)

	// Tool use
	assert.Equal(t, EventToolUse, allEvents[2].Type)

	// Second assistant message
	assert.Equal(t, EventText, allEvents[3].Type)

	// Final result
	assert.Equal(t, EventResult, allEvents[4].Type)
	assert.Equal(t, "The issue is on line 42.", allEvents[4].Text)
	assert.True(t, allEvents[4].Done)
}

func TestGeminiSpec_ParseLine_LargeResponse(t *testing.T) {
	spec := GeminiSpec{}
	longText := strings.Repeat("b", 10000)
	line := `{"type":"result","response":"` + jsonEscape(longText) + `"}`
	events, err := spec.ParseLine(line)
	assert.NoError(t, err)
	assert.Len(t, events, 1)
	assert.Equal(t, longText, events[0].Text)
}

func TestGeminiSpec_InterfaceCompliance(t *testing.T) {
	var _ CLISpec = GeminiSpec{}
}

func TestGeminiSpec_ParseLine_ToolUseWithComplexArgs(t *testing.T) {
	spec := GeminiSpec{}
	line := `{"type":"tool_use","name":"write_file","args":{"path":"/tmp/out.go","content":"package main\nfunc main() {}"}}`
	events, err := spec.ParseLine(line)
	assert.NoError(t, err)
	assert.Len(t, events, 1)
	assert.Equal(t, EventToolUse, events[0].Type)
	assert.Equal(t, "write_file", events[0].ToolUse.Name)
	assert.Contains(t, events[0].ToolUse.Input, "/tmp/out.go")
}
