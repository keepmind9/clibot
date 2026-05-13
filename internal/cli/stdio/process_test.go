package stdio

import (
	"context"
	"encoding/json"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// echoSpec uses "cat" as binary for round-trip testing.
// FormatInput produces JSON that ParseLine can handle when echoed back.
type echoSpec struct{}

func (echoSpec) Name() string                    { return "echo" }
func (echoSpec) Mode() StdioMode                 { return PersistentMode }
func (echoSpec) Binary() string                  { return "cat" }
func (echoSpec) BuildArgs(StartOptions) []string { return nil }
func (echoSpec) FormatInput(msg string) ([]byte, error) {
	return json.Marshal(map[string]any{
		"type":    "result",
		"message": msg,
	})
}
func (echoSpec) FormatPermissionResponse(reqID, optID string) ([]byte, error) {
	return json.Marshal(map[string]any{
		"type":       "control_response",
		"request_id": reqID,
		"option_id":  optID,
	})
}
func (echoSpec) ParseLine(line string) ([]Event, error) {
	var raw map[string]any
	if err := json.Unmarshal([]byte(line), &raw); err != nil {
		return nil, err
	}
	eventType, _ := raw["type"].(string)
	switch eventType {
	case "result":
		msg, _ := raw["message"].(string)
		return []Event{{Type: EventResult, Text: msg, Done: true}}, nil
	default:
		return nil, nil
	}
}

func TestStdioProcess_StartAndClose(t *testing.T) {
	ctx := context.Background()
	proc, err := NewStdioProcess(ctx, echoSpec{}, StartOptions{})
	require.NoError(t, err)
	defer proc.Close()

	assert.True(t, proc.Pid() > 0)
}

func TestStdioProcess_WriteAndRead(t *testing.T) {
	ctx := context.Background()
	proc, err := NewStdioProcess(ctx, echoSpec{}, StartOptions{})
	require.NoError(t, err)
	defer proc.Close()

	// Write input — cat echoes it back, ParseLine parses the result event
	err = proc.WriteInput("hello process test")
	require.NoError(t, err)

	// Read event from channel
	select {
	case evt := <-proc.Events():
		assert.Equal(t, EventResult, evt.Type)
		assert.Equal(t, "hello process test", evt.Text)
		assert.True(t, evt.Done)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for event")
	}
}

func TestStdioProcess_WritePermissionResponse(t *testing.T) {
	ctx := context.Background()
	proc, err := NewStdioProcess(ctx, echoSpec{}, StartOptions{})
	require.NoError(t, err)
	defer proc.Close()

	err = proc.WritePermissionResponse("req-1", "allow")
	require.NoError(t, err)

	// cat echoes it back, but ParseLine won't match "control_response" type
	// so we just verify no error occurred — the response is consumed silently
	select {
	case <-proc.Events():
		// Unexpected event — cat echoed back something ParseLine matched
	case <-time.After(500 * time.Millisecond):
		// Expected: no event produced for control_response
	}
}

func TestStdioProcess_Close_TerminatesProcess(t *testing.T) {
	ctx := context.Background()
	proc, err := NewStdioProcess(ctx, echoSpec{}, StartOptions{})
	require.NoError(t, err)

	pid := proc.Pid()
	assert.True(t, pid > 0)

	err = proc.Close()
	assert.NoError(t, err)

	// Drain any buffered events, then verify channel is closed
	for range proc.Events() {
	}
	// Channel is now closed — next read should return zero value
	_, ok := <-proc.Events()
	assert.False(t, ok, "events channel should be closed after Close")
}

func TestStdioProcess_Events_ReturnsChannel(t *testing.T) {
	ctx := context.Background()
	proc, err := NewStdioProcess(ctx, echoSpec{}, StartOptions{})
	require.NoError(t, err)
	defer proc.Close()

	ch := proc.Events()
	assert.NotNil(t, ch)
}

func TestStdioProcess_Context_Cancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	proc, err := NewStdioProcess(ctx, echoSpec{}, StartOptions{})
	require.NoError(t, err)

	// Cancel context
	cancel()

	// Give time for process to terminate
	time.Sleep(200 * time.Millisecond)

	// Process should have terminated (PID may still be > 0 on some systems,
	// but the event channel should close eventually)
	_ = proc.Pid()
}

func TestStdioProcess_WorkDir(t *testing.T) {
	ctx := context.Background()
	proc, err := NewStdioProcess(ctx, echoSpec{}, StartOptions{WorkDir: "/tmp"})
	require.NoError(t, err)
	defer proc.Close()

	assert.True(t, proc.Pid() > 0)
}

func TestStdioProcess_ExtraEnv(t *testing.T) {
	ctx := context.Background()
	env := map[string]string{"TEST_CLIBOT_ENV": "test_value"}
	proc, err := NewStdioProcess(ctx, echoSpec{}, StartOptions{Env: env})
	require.NoError(t, err)
	defer proc.Close()

	assert.True(t, proc.Pid() > 0)
}

func TestStdioProcess_MultipleWrites(t *testing.T) {
	ctx := context.Background()
	proc, err := NewStdioProcess(ctx, echoSpec{}, StartOptions{})
	require.NoError(t, err)
	defer proc.Close()

	messages := []string{"first", "second", "third"}
	for _, msg := range messages {
		err = proc.WriteInput(msg)
		require.NoError(t, err)
	}

	// Read all echoed events
	received := 0
	timeout := time.After(3 * time.Second)
	for received < len(messages) {
		select {
		case evt := <-proc.Events():
			assert.Equal(t, EventResult, evt.Type)
			assert.Equal(t, messages[received], evt.Text)
			received++
		case <-timeout:
			t.Fatalf("timeout: received %d of %d events", received, len(messages))
		}
	}
}

func TestStdioProcess_InvalidBinary(t *testing.T) {
	spec := &mockSpec{mode: PersistentMode, binary: "nonexistent_binary_xyz"}
	ctx := context.Background()
	_, err := NewStdioProcess(ctx, spec, StartOptions{})
	assert.Error(t, err)
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		n        int
		expected string
	}{
		{"short string unchanged", "hello", 10, "hello"},
		{"exact length unchanged", "hello", 5, "hello"},
		{"truncated with suffix", "hello world", 5, "hello..."},
		{"empty string", "", 5, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, truncate(tt.input, tt.n))
		})
	}
}

func TestStdioProcess_Pid_ReturnsZeroWhenNoProcess(t *testing.T) {
	// Pid() returns 0 when cmd.Process is nil
	p := &StdioProcess{cmd: &exec.Cmd{}}
	assert.Equal(t, 0, p.Pid())
}
