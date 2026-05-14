package stdio

import (
	"context"
	"encoding/json"
	"os/exec"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockEngine records calls for test assertions.
type mockEngine struct {
	sync.Mutex
	sentToBot      []call3
	sentResponse   []call2
	sentPermission []call2
}

type call3 struct{ a, b, c string }
type call2 struct{ a, b string }

func (m *mockEngine) SendToBot(platform, channel, message string) {
	m.Lock()
	defer m.Unlock()
	m.sentToBot = append(m.sentToBot, call3{platform, channel, message})
}

func (m *mockEngine) SendResponseToSession(sessionName, message string) {
	m.Lock()
	defer m.Unlock()
	m.sentResponse = append(m.sentResponse, call2{sessionName, message})
}

func (m *mockEngine) SendPermissionPrompt(sessionName, message string) {
	m.Lock()
	defer m.Unlock()
	m.sentPermission = append(m.sentPermission, call2{sessionName, message})
}

func (m *mockEngine) getPermissions() []call2 {
	m.Lock()
	defer m.Unlock()
	result := make([]call2, len(m.sentPermission))
	copy(result, m.sentPermission)
	return result
}

func (m *mockEngine) getResponses() []call2 {
	m.Lock()
	defer m.Unlock()
	result := make([]call2, len(m.sentResponse))
	copy(result, m.sentResponse)
	return result
}

// mockSpec implements CLISpec using "cat" as binary for process tests.
// ParseLine wraps echoed JSON into events.
type mockSpec struct {
	mode   StdioMode
	binary string
}

func (s *mockSpec) Name() string    { return "mock" }
func (s *mockSpec) Mode() StdioMode { return s.mode }
func (s *mockSpec) Binary() string {
	if s.binary != "" {
		return s.binary
	}
	return "cat"
}
func (s *mockSpec) BuildArgs(StartOptions) []string {
	if s.binary == "head" {
		return []string{"-n", "1"}
	}
	return nil
}
func (s *mockSpec) FormatInput(message string) ([]byte, error) {
	return json.Marshal(map[string]any{
		"type":    "result",
		"message": message,
	})
}
func (s *mockSpec) FormatPermissionResponse(requestID, optionID string) ([]byte, error) {
	return json.Marshal(map[string]any{
		"type":       "permission_response",
		"request_id": requestID,
		"option_id":  optionID,
	})
}
func (s *mockSpec) ParseLine(line string) ([]Event, error) {
	var raw map[string]any
	if err := json.Unmarshal([]byte(line), &raw); err != nil {
		return nil, err
	}

	eventType, _ := raw["type"].(string)
	switch eventType {
	case "result":
		msg, _ := raw["message"].(string)
		return []Event{{Type: EventResult, Text: msg, Done: true}}, nil
	case "text":
		msg, _ := raw["content"].(string)
		return []Event{{Type: EventText, Text: msg}}, nil
	case "control_request":
		reqID, _ := raw["request_id"].(string)
		toolName, _ := raw["tool_name"].(string)
		return []Event{
			{
				Type: EventPermission,
				Permission: &PermissionRequest{
					RequestID: reqID,
					ToolName:  toolName,
					Options: []PermissionOption{
						{ID: "allow", Text: "Allow"},
						{ID: "deny", Text: "Deny"},
					},
				},
			},
		}, nil
	default:
		return nil, nil
	}
}

func TestNewStdioAdapter_Defaults(t *testing.T) {
	adapter := NewStdioAdapter(ClaudeSpec{}, StdioAdapterConfig{})
	assert.Equal(t, defaultPermissionTimeout, adapter.config.PermissionTimeout)
	assert.Equal(t, defaultIdleTimeout, adapter.config.IdleTimeout)
}

func TestNewStdioAdapter_CustomConfig(t *testing.T) {
	cfg := StdioAdapterConfig{
		PermissionTimeout: 1 * time.Minute,
		IdleTimeout:       2 * time.Minute,
		Env:               map[string]string{"FOO": "bar"},
	}
	adapter := NewStdioAdapter(ClaudeSpec{}, cfg)
	assert.Equal(t, 1*time.Minute, adapter.config.PermissionTimeout)
	assert.Equal(t, 2*time.Minute, adapter.config.IdleTimeout)
	assert.Equal(t, "bar", adapter.config.Env["FOO"])
}

func TestStdioAdapter_SetEngine(t *testing.T) {
	adapter := NewStdioAdapter(ClaudeSpec{}, StdioAdapterConfig{})
	eng := &mockEngine{}
	adapter.SetEngine(eng)
	assert.Equal(t, eng, adapter.engine)
}

func TestStdioAdapter_HandleHookData(t *testing.T) {
	adapter := NewStdioAdapter(ClaudeSpec{}, StdioAdapterConfig{})
	_, _, _, err := adapter.HandleHookData([]byte("data"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not support hooks")
}

func TestStdioAdapter_IsSessionAlive_NoSession(t *testing.T) {
	adapter := NewStdioAdapter(ClaudeSpec{}, StdioAdapterConfig{})
	assert.False(t, adapter.IsSessionAlive("nonexistent"))
}

func TestStdioAdapter_SendInput_NoSession(t *testing.T) {
	adapter := NewStdioAdapter(ClaudeSpec{}, StdioAdapterConfig{})
	err := adapter.SendInput("nonexistent", "hello")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestStdioAdapter_GetPendingPermission_NoSession(t *testing.T) {
	adapter := NewStdioAdapter(ClaudeSpec{}, StdioAdapterConfig{})
	assert.Nil(t, adapter.GetPendingPermission("nonexistent"))
}

func TestStdioAdapter_RespondPermission_NoSession(t *testing.T) {
	adapter := NewStdioAdapter(ClaudeSpec{}, StdioAdapterConfig{})
	err := adapter.RespondPermission("nonexistent", "req-1", "allow")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestStdioAdapter_CreateSession_WithCatProcess(t *testing.T) {
	adapter := NewStdioAdapter(&mockSpec{mode: PersistentMode}, StdioAdapterConfig{})
	eng := &mockEngine{}
	adapter.SetEngine(eng)

	err := adapter.CreateSession("test-session", "", "", "", nil)
	require.NoError(t, err)

	assert.True(t, adapter.IsSessionAlive("test-session"))

	// Cleanup
	require.NoError(t, adapter.Close())
	assert.False(t, adapter.IsSessionAlive("test-session"))
}

func TestStdioAdapter_CreateSession_Idempotent(t *testing.T) {
	adapter := NewStdioAdapter(&mockSpec{mode: PersistentMode}, StdioAdapterConfig{})
	defer adapter.Close()

	err := adapter.CreateSession("sess1", "", "", "", nil)
	require.NoError(t, err)

	// Second call should be idempotent
	err = adapter.CreateSession("sess1", "", "", "", nil)
	assert.NoError(t, err)
}

func TestStdioAdapter_CreateSession_MergesEnv(t *testing.T) {
	cfg := StdioAdapterConfig{
		Env: map[string]string{"ADAPTER_KEY": "adapter_val"},
	}
	adapter := NewStdioAdapter(&mockSpec{mode: PersistentMode}, cfg)
	defer adapter.Close()

	sessionEnv := map[string]string{"SESSION_KEY": "session_val"}
	err := adapter.CreateSession("env-test", "", "", "", sessionEnv)
	require.NoError(t, err)

	sess := adapter.sessions["env-test"]
	assert.Equal(t, "adapter_val", sess.env["ADAPTER_KEY"])
	assert.Equal(t, "session_val", sess.env["SESSION_KEY"])
}

func TestStdioAdapter_RespondPermission_NoPendingPerm(t *testing.T) {
	adapter := NewStdioAdapter(&mockSpec{mode: PersistentMode}, StdioAdapterConfig{})
	defer adapter.Close()

	err := adapter.CreateSession("perm-test", "", "", "", nil)
	require.NoError(t, err)

	// No pending permission set
	err = adapter.RespondPermission("perm-test", "req-1", "allow")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no pending permission")
}

func TestStdioAdapter_Close_NoSessions(t *testing.T) {
	adapter := NewStdioAdapter(ClaudeSpec{}, StdioAdapterConfig{})
	assert.NoError(t, adapter.Close())
}

func TestStdioAdapter_Close_MultipleSessions(t *testing.T) {
	adapter := NewStdioAdapter(&mockSpec{mode: PersistentMode}, StdioAdapterConfig{})
	defer adapter.Close()

	err := adapter.CreateSession("s1", "", "", "", nil)
	require.NoError(t, err)
	err = adapter.CreateSession("s2", "", "", "", nil)
	require.NoError(t, err)

	assert.True(t, adapter.IsSessionAlive("s1"))
	assert.True(t, adapter.IsSessionAlive("s2"))

	require.NoError(t, adapter.Close())
	assert.False(t, adapter.IsSessionAlive("s1"))
	assert.False(t, adapter.IsSessionAlive("s2"))
}

// createMockSession creates a session with a mock process (no real subprocess).
// The events channel is not managed by readLoop, so tests can safely inject events.
func createMockSession(adapter *StdioAdapter, name string) *stdioSession {
	proc := &StdioProcess{
		cmd:    &exec.Cmd{},
		events: make(chan Event, eventBufSize),
		ctx:    context.Background(),
		spec:   adapter.spec,
	}
	sess := &stdioSession{
		name:    name,
		process: proc,
	}
	adapter.mu.Lock()
	adapter.sessions[name] = sess
	adapter.mu.Unlock()

	go adapter.eventLoop(sess)

	return sess
}

func TestStdioAdapter_PermissionTimeout_AutoDeny(t *testing.T) {
	cfg := StdioAdapterConfig{
		PermissionTimeout: 50 * time.Millisecond,
	}
	spec := &mockSpec{mode: PersistentMode}
	adapter := NewStdioAdapter(spec, cfg)
	eng := &mockEngine{}
	adapter.SetEngine(eng)
	defer adapter.Close()

	sess := createMockSession(adapter, "timeout-test")

	// Simulate a permission event directly
	perm := &PermissionRequest{
		RequestID: "req-timeout",
		ToolName:  "Bash",
		Input:     "rm -rf /",
		Options: []PermissionOption{
			{ID: "allow", Text: "Allow"},
			{ID: "deny", Text: "Deny"},
		},
	}
	adapter.handlePermissionEvent(sess, perm)

	// Verify pending permission was set
	pending := adapter.GetPendingPermission("timeout-test")
	require.NotNil(t, pending)
	assert.Equal(t, "req-timeout", pending.Request.RequestID)

	// Verify engine was notified
	perms := eng.getPermissions()
	assert.Len(t, perms, 1)
	assert.Equal(t, "timeout-test", perms[0].a)
	assert.Contains(t, perms[0].b, "Bash")

	// Wait for timeout to fire
	time.Sleep(150 * time.Millisecond)

	// After timeout, pending should be cleared and auto-denied
	pending = adapter.GetPendingPermission("timeout-test")
	assert.Nil(t, pending)
}

func TestStdioAdapter_Permission_RespondClearsPending(t *testing.T) {
	cfg := StdioAdapterConfig{
		PermissionTimeout: 5 * time.Minute, // long enough to not auto-deny
	}
	spec := &mockSpec{mode: PersistentMode}
	adapter := NewStdioAdapter(spec, cfg)
	eng := &mockEngine{}
	adapter.SetEngine(eng)
	defer adapter.Close()

	sess := createMockSession(adapter, "respond-test")

	perm := &PermissionRequest{
		RequestID: "req-respond",
		ToolName:  "Read",
		Input:     "/etc/passwd",
		Options: []PermissionOption{
			{ID: "allow", Text: "Allow"},
			{ID: "deny", Text: "Deny"},
		},
	}
	adapter.handlePermissionEvent(sess, perm)
	require.NotNil(t, adapter.GetPendingPermission("respond-test"))

	// Respond with allow
	assert.NoError(t, adapter.RespondPermission("respond-test", "req-respond", "allow"))

	// Pending should be cleared
	assert.Nil(t, adapter.GetPendingPermission("respond-test"))
}

func TestStdioAdapter_Permission_NewRequestCancelsPrevious(t *testing.T) {
	cfg := StdioAdapterConfig{
		PermissionTimeout: 5 * time.Minute,
	}
	spec := &mockSpec{mode: PersistentMode}
	adapter := NewStdioAdapter(spec, cfg)
	eng := &mockEngine{}
	adapter.SetEngine(eng)
	defer adapter.Close()

	sess := createMockSession(adapter, "cancel-test")

	// First permission request
	perm1 := &PermissionRequest{
		RequestID: "req-1",
		ToolName:  "Bash",
		Options:   []PermissionOption{{ID: "allow", Text: "Allow"}, {ID: "deny", Text: "Deny"}},
	}
	adapter.handlePermissionEvent(sess, perm1)
	assert.Equal(t, "req-1", adapter.GetPendingPermission("cancel-test").Request.RequestID)

	// Second permission request replaces first
	perm2 := &PermissionRequest{
		RequestID: "req-2",
		ToolName:  "Edit",
		Options:   []PermissionOption{{ID: "allow", Text: "Allow"}, {ID: "deny", Text: "Deny"}},
	}
	adapter.handlePermissionEvent(sess, perm2)
	assert.Equal(t, "req-2", adapter.GetPendingPermission("cancel-test").Request.RequestID)
}

func TestStdioAdapter_EventLoop_ResultEvent(t *testing.T) {
	adapter := NewStdioAdapter(&mockSpec{mode: PersistentMode}, StdioAdapterConfig{})
	eng := &mockEngine{}
	adapter.SetEngine(eng)

	sess := createMockSession(adapter, "result-test")

	// Only EventResult triggers delivery; EventText is ignored
	sess.process.events <- Event{Type: EventText, Text: "ignored "}
	sess.process.events <- Event{Type: EventText, Text: "fragment"}
	sess.process.events <- Event{Type: EventResult, Text: "final response", Done: true}

	time.Sleep(100 * time.Millisecond)

	responses := eng.getResponses()
	require.Len(t, responses, 1)
	assert.Equal(t, "final response", responses[0].b)

	adapter.Close()
}

func TestStdioAdapter_EventLoop_ErrorEvent(t *testing.T) {
	adapter := NewStdioAdapter(&mockSpec{mode: PersistentMode}, StdioAdapterConfig{})
	eng := &mockEngine{}
	adapter.SetEngine(eng)

	sess := createMockSession(adapter, "error-test")

	// Send error event — should not crash the event loop
	sess.process.events <- Event{Type: EventError, Error: assert.AnError}

	// Verify event loop still works after error
	sess.process.events <- Event{Type: EventResult, Text: "after error", Done: true}

	time.Sleep(100 * time.Millisecond)

	responses := eng.getResponses()
	require.Len(t, responses, 1)
	assert.Equal(t, "after error", responses[0].b)

	adapter.Close()
}

func TestStdioAdapter_EventLoop_PermissionEvent(t *testing.T) {
	cfg := StdioAdapterConfig{
		PermissionTimeout: 5 * time.Minute,
	}
	adapter := NewStdioAdapter(&mockSpec{mode: PersistentMode}, cfg)
	eng := &mockEngine{}
	adapter.SetEngine(eng)

	sess := createMockSession(adapter, "perm-event-test")

	perm := &PermissionRequest{
		RequestID: "req-ev",
		ToolName:  "Bash",
		Input:     "ls",
		Options:   []PermissionOption{{ID: "allow", Text: "Allow"}, {ID: "deny", Text: "Deny"}},
	}
	sess.process.events <- Event{Type: EventPermission, Permission: perm}

	time.Sleep(100 * time.Millisecond)

	perms := eng.getPermissions()
	require.Len(t, perms, 1)
	assert.Contains(t, perms[0].b, "Bash")

	pending := adapter.GetPendingPermission("perm-event-test")
	require.NotNil(t, pending)
	assert.Equal(t, "req-ev", pending.Request.RequestID)

	adapter.Close()
}

func TestStdioAdapter_SendInput_Persistent(t *testing.T) {
	spec := &mockSpec{mode: PersistentMode}
	adapter := NewStdioAdapter(spec, StdioAdapterConfig{})
	eng := &mockEngine{}
	adapter.SetEngine(eng)

	err := adapter.CreateSession("send-test", "", "", "", nil)
	require.NoError(t, err)

	err = adapter.SendInput("send-test", "hello")
	assert.NoError(t, err)

	adapter.Close()
}

func TestStdioAdapter_InterfaceCompliance(t *testing.T) {
	adapter := NewStdioAdapter(ClaudeSpec{}, StdioAdapterConfig{})
	var _ NeedsStdioAdapter = adapter
}

func TestStdioAdapter_SendInput_PerTurnMode(t *testing.T) {
	// PerTurnMode: each SendInput spawns a new process, collects output, and delivers.
	// Use "head -n 1" (reads one line then exits) since cat would hang.
	spec := &mockSpec{mode: PerTurnMode, binary: "head"}
	adapter := NewStdioAdapter(spec, StdioAdapterConfig{})
	eng := &mockEngine{}
	adapter.SetEngine(eng)

	err := adapter.CreateSession("perturn-test", "", "", "", nil)
	require.NoError(t, err)

	err = adapter.SendInput("perturn-test", "hello per-turn")
	assert.NoError(t, err)

	// Wait for response delivery
	time.Sleep(500 * time.Millisecond)

	responses := eng.getResponses()
	require.Len(t, responses, 1)
	assert.Equal(t, "perturn-test", responses[0].a)
	assert.Equal(t, "hello per-turn", responses[0].b)

	adapter.Close()
}

func TestStdioAdapter_SendInput_PerTurnMode_NoEngine(t *testing.T) {
	spec := &mockSpec{mode: PerTurnMode, binary: "head"}
	adapter := NewStdioAdapter(spec, StdioAdapterConfig{})
	// No engine set

	err := adapter.CreateSession("perturn-noeng", "", "", "", nil)
	require.NoError(t, err)

	// Should not panic even without engine
	err = adapter.SendInput("perturn-noeng", "test")
	assert.NoError(t, err)

	adapter.Close()
}

func TestStdioAdapter_SendInput_UnknownMode(t *testing.T) {
	// Create adapter with a spec that returns an invalid mode
	spec := &mockSpec{mode: StdioMode(99)}
	adapter := NewStdioAdapter(spec, StdioAdapterConfig{})
	eng := &mockEngine{}
	adapter.SetEngine(eng)

	err := adapter.CreateSession("badmode", "", "", "", nil)
	require.NoError(t, err)

	err = adapter.SendInput("badmode", "test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown stdio mode")

	adapter.Close()
}

func TestStdioAdapter_Close_WithError(t *testing.T) {
	adapter := NewStdioAdapter(&mockSpec{mode: PersistentMode}, StdioAdapterConfig{})
	eng := &mockEngine{}
	adapter.SetEngine(eng)

	err := adapter.CreateSession("close-err", "", "", "", nil)
	require.NoError(t, err)

	// Close stdin early to cause potential error in process.Close()
	sess := adapter.sessions["close-err"]
	sess.process.stdin.Close()

	err = adapter.Close()
	// Should complete without panic even with close errors
	_ = err
}

func TestStdioAdapter_CreateSession_StartFailure(t *testing.T) {
	// Use a spec with nonexistent binary to trigger start failure
	spec := &mockSpec{mode: PersistentMode, binary: "nonexistent_binary_xyz"}
	adapter := NewStdioAdapter(spec, StdioAdapterConfig{})

	err := adapter.CreateSession("fail-session", "", "", "", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to start")
}
