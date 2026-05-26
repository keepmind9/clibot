package stdio

import (
	"testing"

	"github.com/keepmind9/clibot/internal/cli"
	"github.com/stretchr/testify/assert"
)

func TestStdioEventToCLIEvent(t *testing.T) {
	tests := []struct {
		name string
		evt  Event
		want cli.CLIEvent
	}{
		{
			name: "text event",
			evt:  Event{Type: EventText, Text: "hello"},
			want: cli.CLIEvent{Type: cli.CLIEventText, Content: "hello"},
		},
		{
			name: "tool use event",
			evt: Event{
				Type:    EventToolUse,
				ToolUse: &ToolUseInfo{Name: "Bash", Input: "ls -la"},
			},
			want: cli.CLIEvent{
				Type:     cli.CLIEventToolUse,
				ToolName: "Bash",
				ToolMeta: map[string]string{"input": "ls -la"},
			},
		},
		{
			name: "tool use nil ToolUse",
			evt:  Event{Type: EventToolUse},
			want: cli.CLIEvent{
				Type:     cli.CLIEventToolUse,
				ToolMeta: map[string]string{},
			},
		},
		{
			name: "result done event",
			evt:  Event{Type: EventResult, Text: "final answer", Done: true},
			want: cli.CLIEvent{Type: cli.CLIEventDone, Content: "final answer"},
		},
		{
			name: "error event",
			evt:  Event{Type: EventError, Error: assert.AnError},
			want: cli.CLIEvent{Type: cli.CLIEventDone, Content: "error: " + assert.AnError.Error()},
		},
		{
			name: "permission event maps to permission type",
			evt: Event{
				Type: EventPermission,
				Permission: &PermissionRequest{
					RequestID: "req_1",
					ToolName:  "Edit",
					Options:   []PermissionOption{{ID: "allow", Text: "Allow"}},
				},
			},
			want: cli.CLIEvent{Type: cli.CLIEventPermission, ToolName: "Edit", Content: "Permission requested: Edit\nReply 1-1:\n1. Allow"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stdioEventToCLIEvent(tt.evt)
			assert.Equal(t, tt.want.Type, got.Type)
			assert.Equal(t, tt.want.Content, got.Content)
			assert.Equal(t, tt.want.ToolName, got.ToolName)
			if tt.want.ToolMeta != nil {
				assert.Equal(t, tt.want.ToolMeta, got.ToolMeta)
			}
		})
	}
}

func TestSendInputStreaming_SessionNotFound(t *testing.T) {
	spec := &mockSpec{mode: PersistentMode}
	adapter := NewStdioAdapter(spec, StdioAdapterConfig{})

	_, err := adapter.SendInputStreaming("nonexistent", "hello")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestSendInputStreaming_PersistentAlreadyStreaming(t *testing.T) {
	spec := &mockSpec{mode: PersistentMode}
	adapter := NewStdioAdapter(spec, StdioAdapterConfig{})

	sess := &stdioSession{
		name:     "test",
		process:  &StdioProcess{},
		streamCh: make(chan<- Event),
	}
	adapter.mu.Lock()
	adapter.sessions["test"] = sess
	adapter.mu.Unlock()

	_, err := adapter.SendInputStreaming("test", "hello")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already in progress")
}
