package stdio

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCodexSpec_Properties(t *testing.T) {
	spec := CodexSpec{}
	assert.Equal(t, "codex", spec.Name())
	assert.Equal(t, PerTurnMode, spec.Mode())
	assert.Equal(t, "codex", spec.Binary())
}

func TestCodexSpec_BuildArgs_FirstTurn(t *testing.T) {
	spec := CodexSpec{}
	args := spec.BuildArgs(StartOptions{})
	assert.Equal(t, []string{
		"exec", "--json", "--skip-git-repo-check", "-",
	}, args)
}

func TestCodexSpec_BuildArgs_WithModel(t *testing.T) {
	spec := CodexSpec{}
	args := spec.BuildArgs(StartOptions{Model: "gpt-5.4"})
	assert.Contains(t, args, "--model")
	assert.Contains(t, args, "gpt-5.4")
}

func TestCodexSpec_BuildArgs_WithWorkDir(t *testing.T) {
	spec := CodexSpec{}
	args := spec.BuildArgs(StartOptions{WorkDir: "/project"})
	assert.Contains(t, args, "--cd")
	assert.Contains(t, args, "/project")
}

func TestCodexSpec_BuildArgs_Resume(t *testing.T) {
	spec := CodexSpec{}
	args := spec.BuildArgs(StartOptions{Resume: true})
	assert.Equal(t, []string{
		"exec", "--json", "--skip-git-repo-check", "resume", "--last", "-",
	}, args)
}

func TestCodexSpec_BuildArgs_ResumeWithSessionID(t *testing.T) {
	spec := CodexSpec{}
	args := spec.BuildArgs(StartOptions{Resume: true, SessionID: "uuid-123"})
	assert.Contains(t, args, "resume")
	assert.Contains(t, args, "uuid-123")
	assert.NotContains(t, args, "--last")
}

func TestCodexSpec_BuildArgs_AllOptions(t *testing.T) {
	spec := CodexSpec{}
	args := spec.BuildArgs(StartOptions{
		Resume:  true,
		Model:   "gpt-5.4",
		WorkDir: "/project",
	})
	assert.Contains(t, args, "resume")
	assert.Contains(t, args, "--model")
	assert.Contains(t, args, "--cd")
}

func TestCodexSpec_FormatInput(t *testing.T) {
	tests := []struct {
		name    string
		message string
	}{
		{"normal message", "fix the bug"},
		{"empty string", ""},
		{"unicode", "修复bug 🐛"},
		{"multiline", "line1\nline2\nline3"},
		{"special chars", `{"key": "value"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := CodexSpec{}
			data, err := spec.FormatInput(tt.message)
			assert.NoError(t, err)
			assert.Equal(t, []byte(tt.message), data)
		})
	}
}

func TestCodexSpec_FormatPermissionResponse(t *testing.T) {
	spec := CodexSpec{}
	data, err := spec.FormatPermissionResponse("req-1", "allow")
	assert.NoError(t, err)
	assert.Nil(t, data)
}

func TestCodexSpec_ParseLine(t *testing.T) {
	spec := CodexSpec{}

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
			name:          "thread.started with session ID",
			line:          `{"type":"thread.started","thread_id":"0199a213-81c0-7800-8aa1-bbab2a035a53"}`,
			expectEvents:  1,
			expectSession: "0199a213-81c0-7800-8aa1-bbab2a035a53",
		},
		{
			name:         "thread.started without thread_id",
			line:         `{"type":"thread.started"}`,
			expectEvents: 0,
		},
		{
			name:         "item.completed agent_message",
			line:         `{"type":"item.completed","item":{"id":"item_1","type":"agent_message","text":"The repo contains 3 files."}}`,
			expectEvents: 1,
			expectType:   EventResult,
			expectText:   "The repo contains 3 files.",
		},
		{
			name:         "item.completed agent_message empty text",
			line:         `{"type":"item.completed","item":{"id":"item_2","type":"agent_message","text":""}}`,
			expectEvents: 0,
		},
		{
			name:         "item.completed command_execution",
			line:         `{"type":"item.completed","item":{"id":"item_3","type":"command_execution","command":"ls -la"}}`,
			expectEvents: 1,
			expectType:   EventToolUse,
			expectToolUse: &ToolUseInfo{
				Name:  "shell",
				Input: "ls -la",
			},
		},
		{
			name:         "item.completed command_execution no command",
			line:         `{"type":"item.completed","item":{"id":"item_4","type":"command_execution"}}`,
			expectEvents: 1,
			expectType:   EventToolUse,
			expectToolUse: &ToolUseInfo{
				Name:  "shell",
				Input: "command",
			},
		},
		{
			name:         "item.completed unknown item type",
			line:         `{"type":"item.completed","item":{"id":"item_5","type":"file_change"}}`,
			expectEvents: 0,
		},
		{
			name:         "item.completed missing item field",
			line:         `{"type":"item.completed"}`,
			expectEvents: 0,
		},
		{
			name:         "turn.completed",
			line:         `{"type":"turn.completed","usage":{"input_tokens":100,"output_tokens":50}}`,
			expectEvents: 1,
			expectType:   EventResult,
			expectDone:   true,
		},
		{
			name:         "turn.failed",
			line:         `{"type":"turn.failed"}`,
			expectEvents: 1,
			expectType:   EventError,
		},
		{
			name:         "error event with message",
			line:         `{"type":"error","message":"Rate limit exceeded"}`,
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
			line:         `{"type":"thread.completed"}`,
			expectEvents: 0,
		},
		{
			name:         "turn.started ignored",
			line:         `{"type":"turn.started"}`,
			expectEvents: 0,
		},
		{
			name:      "invalid JSON",
			line:      `{broken`,
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
					assert.Equal(t, tt.expectToolUse.Input, evt.ToolUse.Input)
				}

				if tt.expectType == EventResult {
					assert.Equal(t, tt.expectDone, evt.Done)
				}
			}
		})
	}
}

func TestCodexSpec_ParseLine_MultipleAgentMessages(t *testing.T) {
	spec := CodexSpec{}

	line1 := `{"type":"item.completed","item":{"id":"i1","type":"agent_message","text":"First part. "}}`
	line2 := `{"type":"item.completed","item":{"id":"i2","type":"agent_message","text":"Second part."}}`
	line3 := `{"type":"turn.completed"}`

	events1, err := spec.ParseLine(line1)
	assert.NoError(t, err)
	assert.Len(t, events1, 1)
	assert.Equal(t, "First part. ", events1[0].Text)

	events2, err := spec.ParseLine(line2)
	assert.NoError(t, err)
	assert.Len(t, events2, 1)
	assert.Equal(t, "Second part.", events2[0].Text)

	events3, err := spec.ParseLine(line3)
	assert.NoError(t, err)
	assert.Len(t, events3, 1)
	assert.True(t, events3[0].Done)
}

func TestCodexSpec_ParseLine_SessionIDCapture(t *testing.T) {
	spec := CodexSpec{}
	line := `{"type":"thread.started","thread_id":"abc-123"}`
	events, err := spec.ParseLine(line)
	assert.NoError(t, err)
	assert.Len(t, events, 1)
	assert.Equal(t, "abc-123", events[0].SessionID)
	// The event has no meaningful Type (0 = EventText) but empty text
	assert.Empty(t, events[0].Text)
}

func TestCodexSpec_InterfaceCompliance(t *testing.T) {
	var _ CLISpec = CodexSpec{}
}

func TestCodexSpec_ParseLine_RealWorldSequence(t *testing.T) {
	spec := CodexSpec{}
	var allEvents []Event

	lines := []string{
		`{"type":"thread.started","thread_id":"0199a213-81c0"}`,
		`{"type":"turn.started"}`,
		`{"type":"item.completed","item":{"id":"i1","type":"agent_message","text":"Let me check."}}`,
		`{"type":"item.completed","item":{"id":"i2","type":"command_execution","command":"ls"}}`,
		`{"type":"item.completed","item":{"id":"i3","type":"agent_message","text":"Found 5 files."}}`,
		`{"type":"turn.completed","usage":{"input_tokens":100,"output_tokens":50}}`,
	}

	for _, line := range lines {
		events, err := spec.ParseLine(line)
		assert.NoError(t, err)
		allEvents = append(allEvents, events...)
	}

	// Should have: session capture + 2 agent messages + 1 tool use + 1 done
	assert.Len(t, allEvents, 5)

	// Session ID captured
	assert.Equal(t, "0199a213-81c0", allEvents[0].SessionID)

	// Agent messages are EventResult
	assert.Equal(t, EventResult, allEvents[1].Type)
	assert.Equal(t, "Let me check.", allEvents[1].Text)

	// Tool use
	assert.Equal(t, EventToolUse, allEvents[2].Type)
	assert.Equal(t, "shell", allEvents[2].ToolUse.Name)

	// Second agent message
	assert.Equal(t, EventResult, allEvents[3].Type)
	assert.Equal(t, "Found 5 files.", allEvents[3].Text)

	// Turn completed
	assert.Equal(t, EventResult, allEvents[4].Type)
	assert.True(t, allEvents[4].Done)
}

func TestCodexSpec_FormatInput_LongMessage(t *testing.T) {
	spec := CodexSpec{}
	longMsg := strings.Repeat("x", 10000)
	data, err := spec.FormatInput(longMsg)
	assert.NoError(t, err)
	assert.Equal(t, longMsg, string(data))
}

func TestCodexSpec_ParseLine_LargeItemText(t *testing.T) {
	spec := CodexSpec{}
	longText := strings.Repeat("a", 5000)
	line := `{"type":"item.completed","item":{"id":"i1","type":"agent_message","text":"` + jsonEscape(longText) + `"}}`
	events, err := spec.ParseLine(line)
	assert.NoError(t, err)
	assert.Len(t, events, 1)
	assert.Equal(t, longText, events[0].Text)
}

// jsonEscape produces a JSON-safe string literal.
func jsonEscape(s string) string {
	b, _ := json.Marshal(s)
	return string(b[1 : len(b)-1])
}
