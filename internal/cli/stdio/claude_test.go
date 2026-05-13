package stdio

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClaudeSpec_Properties(t *testing.T) {
	spec := ClaudeSpec{}
	assert.Equal(t, "claude", spec.Name())
	assert.Equal(t, PersistentMode, spec.Mode())
	assert.Equal(t, "claude", spec.Binary())
}

func TestClaudeSpec_BuildArgs(t *testing.T) {
	spec := ClaudeSpec{}
	args := spec.BuildArgs(StartOptions{})
	assert.Equal(t, []string{
		"--output-format", "stream-json",
		"--input-format", "stream-json",
		"--permission-prompt-tool", "stdio",
		"--verbose",
	}, args)
}

func TestClaudeSpec_FormatInput(t *testing.T) {
	tests := []struct {
		name    string
		message string
	}{
		{"normal message", "hello world"},
		{"empty string", ""},
		{"unicode", "你好世界 🌍"},
		{"special chars", "line1\nline2\ttab"},
		{"json string", `{"key": "value"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := ClaudeSpec{}
			data, err := spec.FormatInput(tt.message)
			assert.NoError(t, err)

			var parsed map[string]any
			assert.NoError(t, json.Unmarshal(data, &parsed))

			assert.Equal(t, "user", parsed["type"])

			msg, ok := parsed["message"].(map[string]any)
			assert.True(t, ok)
			assert.Equal(t, "user", msg["role"])
			assert.Equal(t, tt.message, msg["content"])
		})
	}
}

func TestClaudeSpec_FormatPermissionResponse_Allow(t *testing.T) {
	spec := ClaudeSpec{}
	data, err := spec.FormatPermissionResponse("req-123", "allow")
	assert.NoError(t, err)

	var parsed map[string]any
	assert.NoError(t, json.Unmarshal(data, &parsed))

	assert.Equal(t, "control_response", parsed["type"])

	resp, ok := parsed["response"].(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, "success", resp["subtype"])
	assert.Equal(t, "req-123", resp["request_id"])

	inner, ok := resp["response"].(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, "allow", inner["behavior"])
	assert.Empty(t, inner["message"])
}

func TestClaudeSpec_FormatPermissionResponse_Deny(t *testing.T) {
	tests := []struct {
		name     string
		optionID string
	}{
		{"deny option", "deny"},
		{"unknown option", "something_else"},
		{"empty option", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := ClaudeSpec{}
			data, err := spec.FormatPermissionResponse("req-456", tt.optionID)
			assert.NoError(t, err)

			var parsed map[string]any
			assert.NoError(t, json.Unmarshal(data, &parsed))

			resp := parsed["response"].(map[string]any)["response"].(map[string]any)
			assert.Equal(t, "deny", resp["behavior"])
			assert.Equal(t, "Denied by user.", resp["message"])
		})
	}
}

func TestClaudeSpec_ParseLine(t *testing.T) {
	spec := ClaudeSpec{}

	tests := []struct {
		name          string
		line          string
		expectErr     bool
		expectEvents  int
		expectType    EventType
		expectText    string
		expectToolUse *ToolUseInfo
		expectPerm    *PermissionRequest
		expectDone    bool
	}{
		{
			name:         "system event ignored",
			line:         `{"type":"system","session_id":"sess-1"}`,
			expectEvents: 0,
		},
		{
			name:         "assistant text content",
			line:         `{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"Hello!"}]}}`,
			expectEvents: 1,
			expectType:   EventText,
			expectText:   "Hello!",
		},
		{
			name:         "assistant tool_use content",
			line:         `{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","name":"Bash","input":{"command":"ls -la"}}]}}`,
			expectEvents: 1,
			expectType:   EventToolUse,
			expectToolUse: &ToolUseInfo{
				Name:  "Bash",
				Input: "ls -la",
			},
		},
		{
			name:         "assistant mixed content",
			line:         `{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"Running "},{"type":"tool_use","name":"Bash","input":{"command":"pwd"}}]}}`,
			expectEvents: 2,
		},
		{
			name:         "assistant empty text skipped",
			line:         `{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":""}]}}`,
			expectEvents: 0,
		},
		{
			name:         "assistant missing message field",
			line:         `{"type":"assistant"}`,
			expectEvents: 0,
		},
		{
			name:         "assistant missing content field",
			line:         `{"type":"assistant","message":{"role":"assistant"}}`,
			expectEvents: 0,
		},
		{
			name:         "assistant content not array",
			line:         `{"type":"assistant","message":{"role":"assistant","content":"text"}}`,
			expectEvents: 0,
		},
		{
			name:         "result event",
			line:         `{"type":"result","result":"Task completed."}`,
			expectEvents: 1,
			expectType:   EventResult,
			expectText:   "Task completed.",
			expectDone:   true,
		},
		{
			name:         "result event empty string",
			line:         `{"type":"result","result":""}`,
			expectEvents: 1,
			expectType:   EventResult,
			expectText:   "",
			expectDone:   true,
		},
		{
			name:         "control_request can_use_tool",
			line:         `{"type":"control_request","request_id":"req-1","request":{"subtype":"can_use_tool","tool_name":"Bash","input":{"command":"rm -rf /"}}}`,
			expectEvents: 1,
			expectType:   EventPermission,
			expectPerm: &PermissionRequest{
				RequestID: "req-1",
				ToolName:  "Bash",
				Input:     "rm -rf /",
				Options: []PermissionOption{
					{ID: "allow", Text: "Allow"},
					{ID: "deny", Text: "Deny"},
				},
			},
		},
		{
			name:         "control_request other subtype ignored",
			line:         `{"type":"control_request","request_id":"req-2","request":{"subtype":"other","tool_name":"Bash"}}`,
			expectEvents: 0,
		},
		{
			name:         "control_request missing request field",
			line:         `{"type":"control_request","request_id":"req-3"}`,
			expectEvents: 0,
		},
		{
			name:         "control_cancel_request ignored",
			line:         `{"type":"control_cancel_request","request_id":"req-4"}`,
			expectEvents: 0,
		},
		{
			name:         "unknown type ignored",
			line:         `{"type":"something_else","data":"foo"}`,
			expectEvents: 0,
		},
		{
			name:      "invalid JSON",
			line:      `{invalid json}`,
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
				assert.Equal(t, tt.expectType, evt.Type)

				if tt.expectText != "" || tt.expectType == EventResult {
					assert.Equal(t, tt.expectText, evt.Text)
				}
				if tt.expectToolUse != nil {
					assert.NotNil(t, evt.ToolUse)
					assert.Equal(t, tt.expectToolUse.Name, evt.ToolUse.Name)
					assert.Equal(t, tt.expectToolUse.Input, evt.ToolUse.Input)
				}
				if tt.expectPerm != nil {
					assert.NotNil(t, evt.Permission)
					assert.Equal(t, tt.expectPerm.RequestID, evt.Permission.RequestID)
					assert.Equal(t, tt.expectPerm.ToolName, evt.Permission.ToolName)
					assert.Equal(t, tt.expectPerm.Input, evt.Permission.Input)
					assert.Equal(t, tt.expectPerm.Options, evt.Permission.Options)
				}
				if tt.expectType == EventResult {
					assert.Equal(t, tt.expectDone, evt.Done)
				}
			}
		})
	}
}

func TestClaudeSpec_ParseLine_AssistantMixedContent_BothEvents(t *testing.T) {
	spec := ClaudeSpec{}
	line := `{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"Step 1"},{"type":"tool_use","name":"Read","input":{"file_path":"/tmp/test.go"}}]}}`

	events, err := spec.ParseLine(line)
	assert.NoError(t, err)
	assert.Len(t, events, 2)

	assert.Equal(t, EventText, events[0].Type)
	assert.Equal(t, "Step 1", events[0].Text)

	assert.Equal(t, EventToolUse, events[1].Type)
	assert.Equal(t, "Read", events[1].ToolUse.Name)
	// Read is not a special-cased tool name, so input is JSON-encoded
	assert.Contains(t, events[1].ToolUse.Input, "/tmp/test.go")
}

func TestSummarizeInput(t *testing.T) {
	longStr := strings.Repeat("a", 400)
	shortStr := "short command"

	tests := []struct {
		name     string
		toolName string
		input    any
		expected string
	}{
		{"nil input", "Bash", nil, ""},
		{"short string", "Bash", shortStr, shortStr},
		{"long string truncated", "Bash", longStr, longStr[:300] + "..."},
		{"Bash extracts command", "Bash", map[string]any{"command": "go test ./..."}, "go test ./..."},
		{"Edit extracts file_path", "Edit", map[string]any{"file_path": "/tmp/file.go", "content": "x"}, "/tmp/file.go"},
		{"Write extracts file_path", "Write", map[string]any{"file_path": "/tmp/out.go", "content": "y"}, "/tmp/out.go"},
		{"unknown tool JSON-encodes map", "Custom", map[string]any{"key": "val"}, `{"key":"val"}`},
		{
			"large map truncated",
			"Custom",
			map[string]any{"data": longStr},
			func() string {
				s := `{"data":"` + longStr + `"}` //nolint:staticcheck
				if len(s) > 300 {
					return s[:300] + "..."
				}
				return s
			}(),
		},
		{"int type uses Sprintf", "Tool", 42, "42"},
		{"bool type uses Sprintf", "Tool", true, "true"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := summarizeInput(tt.toolName, tt.input)
			if len(tt.expected) > 303 {
				assert.True(t, strings.HasSuffix(result, "..."), "expected truncation suffix")
				assert.Equal(t, tt.expected[:300], result[:300])
			} else {
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestIsStdioCLIType(t *testing.T) {
	tests := []struct {
		cliType  string
		expected bool
	}{
		{"claude-stdio", true},
		{"codex-stdio", true},
		{"gemini-stdio", true},
		{"-stdio", true},
		{"claude", false},
		{"acp", false},
		{"claude-pty", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.cliType, func(t *testing.T) {
			assert.Equal(t, tt.expected, IsStdioCLIType(tt.cliType))
		})
	}
}
