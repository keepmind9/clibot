package core

import (
	"fmt"
	"strings"
	"testing"

	"github.com/keepmind9/clibot/internal/bot"
	"github.com/keepmind9/clibot/internal/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockCLIAdapter implements cli.CLIAdapter for testing.
type mockCLIAdapter struct {
	createSessionCalled bool
	lastSessionName     string
	lastOpts            []cli.SessionOption
	createSessionErr    error
}

func (m *mockCLIAdapter) SendInput(sessionName, input string) error { return nil }
func (m *mockCLIAdapter) HandleHookData(data []byte) (string, string, string, error) {
	return "", "", "", nil
}
func (m *mockCLIAdapter) IsSessionAlive(sessionName string) bool { return true }
func (m *mockCLIAdapter) CreateSession(sessionName string, opts ...cli.SessionOption) error {
	m.createSessionCalled = true
	m.lastSessionName = sessionName
	m.lastOpts = opts
	return m.createSessionErr
}

// newTestEngine creates a minimal Engine for sn command tests.
func newTestEngine() *Engine {
	config := &Config{
		Sessions: []SessionConfig{{Name: "default", CLIType: "test-stdio", WorkDir: "/tmp"}},
	}
	engine := NewEngine(config)
	mockBot := &mockBotAdapter{}
	engine.RegisterBotAdapter("testbot", mockBot)
	return engine
}

func adminMsg() bot.BotMessage {
	return bot.BotMessage{
		Platform: "testbot",
		Channel:  "ch1",
		UserID:   "admin1",
	}
}

func getMockBot(e *Engine) *mockBotAdapter {
	return e.activeBots["testbot"].(*mockBotAdapter)
}

// --- generateSessionName ---

func TestGenerateSessionName(t *testing.T) {
	tests := []struct {
		name     string
		template string
		workDir  string
		expected string
	}{
		{"normal path", "codex", "/home/user/my-project", "codex-my-project"},
		{"dot base", "claude", "/tmp/.", "claude-session"},
		{"slash only", "gemini", "/", "gemini-session"},
		{"empty base after clean", "codex", "/tmp/...", "codex-session"},
		{"special chars sanitized", "codex", "/tmp/hello world!", "codex-hello-world"},
		{"multiple hyphens collapsed", "claude", "/tmp/a---b", "claude-a-b"},
		{"dot segment", "codex", "/tmp/..", "codex-session"},
		{"unicode replaced", "gemini", "/tmp/项目目录", "gemini-session"},
		{"simple name", "opencode", "/tmp/app", "opencode-app"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generateSessionName(tt.template, tt.workDir)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// --- resolveNameConflict ---

func TestResolveNameConflict_NoConflict(t *testing.T) {
	engine := newTestEngine()
	result := engine.resolveNameConflict("new-session")
	assert.Equal(t, "new-session", result)
}

func TestResolveNameConflict_FirstAvailable(t *testing.T) {
	engine := newTestEngine()
	engine.sessions["my-session"] = &Session{Name: "my-session"}
	result := engine.resolveNameConflict("my-session")
	assert.Equal(t, "my-session-2", result)
}

func TestResolveNameConflict_SkipsExisting(t *testing.T) {
	engine := newTestEngine()
	engine.sessions["s"] = &Session{Name: "s"}
	engine.sessions["s-2"] = &Session{Name: "s-2"}
	engine.sessions["s-3"] = &Session{Name: "s-3"}
	result := engine.resolveNameConflict("s")
	assert.Equal(t, "s-4", result)
}

func TestResolveNameConflict_Exhausted(t *testing.T) {
	engine := newTestEngine()
	engine.sessions["x"] = &Session{}
	for i := 2; i <= maxNameConflictRetries; i++ {
		engine.sessions[fmt.Sprintf("x-%d", i)] = &Session{}
	}
	result := engine.resolveNameConflict("x")
	assert.Equal(t, "", result)
}

// --- resolveTemplate ---

func TestResolveTemplate_Builtin(t *testing.T) {
	engine := newTestEngine()
	for _, name := range []string{"codex", "claude", "gemini", "opencode"} {
		tpl := engine.resolveTemplate(name)
		require.NotNil(t, tpl, "builtin template '%s' should resolve", name)
		assert.NotEmpty(t, tpl.CLIType, "builtin '%s' should have CLIType", name)
	}
}

func TestResolveTemplate_Unknown(t *testing.T) {
	engine := newTestEngine()
	tpl := engine.resolveTemplate("nonexistent")
	assert.Nil(t, tpl)
}

func TestResolveTemplate_CustomOverridesBuiltin(t *testing.T) {
	engine := newTestEngine()
	engine.config.SessionTemplates = map[string]SessionTemplate{
		"codex": {CLIType: "custom-stdio", Yolo: true},
	}
	tpl := engine.resolveTemplate("codex")
	require.NotNil(t, tpl)
	assert.Equal(t, "custom-stdio", tpl.CLIType)
	assert.True(t, tpl.Yolo)
}

func TestResolveTemplate_CustomOnly(t *testing.T) {
	engine := newTestEngine()
	engine.config.SessionTemplates = map[string]SessionTemplate{
		"mytool": {CLIType: "mytool-stdio", Env: map[string]string{"KEY": "val"}},
	}
	tpl := engine.resolveTemplate("mytool")
	require.NotNil(t, tpl)
	assert.Equal(t, "mytool-stdio", tpl.CLIType)
	assert.Equal(t, "val", tpl.Env["KEY"])
}

// --- defaultBuiltinTemplates ---

func TestDefaultBuiltinTemplates_ReturnsNewMap(t *testing.T) {
	m1 := defaultBuiltinTemplates()
	m2 := defaultBuiltinTemplates()
	m1["codex"] = SessionTemplate{CLIType: "mutated"}
	assert.NotEqual(t, m1["codex"], m2["codex"], "mutation of returned map should not affect future calls")
}

func TestDefaultBuiltinTemplates_ContainsAll(t *testing.T) {
	templates := defaultBuiltinTemplates()
	expected := map[string]string{
		"codex":    "codex-stdio",
		"claude":   "claude-stdio",
		"gemini":   "gemini-stdio",
		"opencode": "opencode-stdio",
	}
	for name, cliType := range expected {
		tpl, ok := templates[name]
		assert.True(t, ok, "should contain '%s'", name)
		assert.Equal(t, cliType, tpl.CLIType)
	}
	assert.Equal(t, len(expected), len(templates))
}

// --- handleNewFromTemplate ---

func TestHandleNewFromTemplate_NonAdmin(t *testing.T) {
	engine := newTestEngine()
	msg := bot.BotMessage{Platform: "testbot", Channel: "ch1", UserID: "stranger"}
	engine.handleNewFromTemplate([]string{"codex", "/tmp"}, msg)
	assert.Contains(t, getMockBot(engine).lastMessage, "Permission denied")
}

func TestHandleNewFromTemplate_TooFewArgs(t *testing.T) {
	engine := newTestEngine()
	engine.config.Security.Admins = map[string][]string{"testbot": {"admin1"}}
	engine.handleNewFromTemplate([]string{"codex"}, adminMsg())
	assert.Contains(t, getMockBot(engine).lastMessage, "Invalid arguments")
}

func TestHandleNewFromTemplate_EmptyArgs(t *testing.T) {
	engine := newTestEngine()
	engine.config.Security.Admins = map[string][]string{"testbot": {"admin1"}}
	engine.handleNewFromTemplate([]string{}, adminMsg())
	assert.Contains(t, getMockBot(engine).lastMessage, "Invalid arguments")
}

func TestHandleNewFromTemplate_UnknownTemplate(t *testing.T) {
	engine := newTestEngine()
	engine.config.Security.Admins = map[string][]string{"testbot": {"admin1"}}
	engine.handleNewFromTemplate([]string{"unknown", "/tmp"}, adminMsg())
	assert.Contains(t, getMockBot(engine).lastMessage, "Unknown template")
}

func TestHandleNewFromTemplate_EmptyCLIType(t *testing.T) {
	engine := newTestEngine()
	engine.config.Security.Admins = map[string][]string{"testbot": {"admin1"}}
	engine.config.SessionTemplates = map[string]SessionTemplate{
		"bad": {CLIType: ""},
	}
	engine.handleNewFromTemplate([]string{"bad", "/tmp"}, adminMsg())
	assert.Contains(t, getMockBot(engine).lastMessage, "no cli_type")
}

func TestHandleNewFromTemplate_AdapterNotRegistered(t *testing.T) {
	engine := newTestEngine()
	engine.config.Security.Admins = map[string][]string{"testbot": {"admin1"}}
	engine.handleNewFromTemplate([]string{"codex", "/tmp", "test-session"}, adminMsg())
	assert.Contains(t, getMockBot(engine).lastMessage, "not registered")
}

func TestHandleNewFromTemplate_WorkDirNotExist(t *testing.T) {
	engine := newTestEngine()
	engine.config.Security.Admins = map[string][]string{"testbot": {"admin1"}}
	engine.RegisterCLIAdapter("codex-stdio", &mockCLIAdapter{})
	engine.handleNewFromTemplate([]string{"codex", "/nonexistent/path/xyz", "test-session"}, adminMsg())
	assert.Contains(t, getMockBot(engine).lastMessage, "does not exist")
}

func TestHandleNewFromTemplate_CustomName(t *testing.T) {
	engine := newTestEngine()
	engine.config.Security.Admins = map[string][]string{"testbot": {"admin1"}}
	engine.RegisterCLIAdapter("codex-stdio", &mockCLIAdapter{})

	engine.handleNewFromTemplate([]string{"codex", "/tmp", "my-custom-name"}, adminMsg())
	assert.Contains(t, getMockBot(engine).lastMessage, "my-custom-name")
	assert.Contains(t, getMockBot(engine).lastMessage, "created")
}

func TestHandleNewFromTemplate_AutoName(t *testing.T) {
	engine := newTestEngine()
	engine.config.Security.Admins = map[string][]string{"testbot": {"admin1"}}
	engine.RegisterCLIAdapter("codex-stdio", &mockCLIAdapter{})

	engine.handleNewFromTemplate([]string{"codex", "/tmp"}, adminMsg())
	botMsg := getMockBot(engine).lastMessage
	assert.Contains(t, botMsg, "codex-")
	assert.Contains(t, botMsg, "created")
}

func TestHandleNewFromTemplate_WithYolo(t *testing.T) {
	engine := newTestEngine()
	engine.config.Security.Admins = map[string][]string{"testbot": {"admin1"}}
	engine.config.SessionTemplates = map[string]SessionTemplate{
		"codex": {CLIType: "codex-stdio", Yolo: true},
	}
	mockCli := &mockCLIAdapter{}
	engine.RegisterCLIAdapter("codex-stdio", mockCli)

	engine.handleNewFromTemplate([]string{"codex", "/tmp", "yolo-session"}, adminMsg())
	assert.Contains(t, getMockBot(engine).lastMessage, "[yolo]")
	assert.True(t, mockCli.createSessionCalled)
}

func TestHandleNewFromTemplate_NameConflictAutoResolved(t *testing.T) {
	engine := newTestEngine()
	engine.config.Security.Admins = map[string][]string{"testbot": {"admin1"}}
	engine.RegisterCLIAdapter("codex-stdio", &mockCLIAdapter{})

	autoName := generateSessionName("codex", "/tmp")
	engine.sessionMu.Lock()
	engine.sessions[autoName] = &Session{Name: autoName}
	engine.sessionMu.Unlock()

	engine.handleNewFromTemplate([]string{"codex", "/tmp"}, adminMsg())
	assert.Contains(t, getMockBot(engine).lastMessage, "created")
}

func TestHandleNewFromTemplate_InvalidSessionName(t *testing.T) {
	engine := newTestEngine()
	engine.config.Security.Admins = map[string][]string{"testbot": {"admin1"}}
	engine.RegisterCLIAdapter("codex-stdio", &mockCLIAdapter{})

	engine.handleNewFromTemplate([]string{"codex", "/tmp", "bad name!"}, adminMsg())
	assert.Contains(t, getMockBot(engine).lastMessage, "Invalid session name")
}

func TestHandleNewFromTemplate_WithEnv(t *testing.T) {
	engine := newTestEngine()
	engine.config.Security.Admins = map[string][]string{"testbot": {"admin1"}}
	engine.config.SessionTemplates = map[string]SessionTemplate{
		"codex": {CLIType: "codex-stdio", Env: map[string]string{"FOO": "bar"}},
	}
	mockCli := &mockCLIAdapter{}
	engine.RegisterCLIAdapter("codex-stdio", mockCli)

	engine.handleNewFromTemplate([]string{"codex", "/tmp", "env-test"}, adminMsg())
	assert.True(t, mockCli.createSessionCalled)

	// Verify env was passed through SessionOptions
	var opts cli.SessionOptions
	for _, opt := range mockCli.lastOpts {
		opt(&opts)
	}
	assert.Equal(t, "bar", opts.Env["FOO"])
}

// --- listTemplates / snlist ---

func TestListTemplates_BuiltInOnly(t *testing.T) {
	engine := newTestEngine()
	mockBot := &mockBotAdapter{}
	engine.RegisterBotAdapter("testbot", mockBot)

	engine.listTemplates(bot.BotMessage{Platform: "testbot", Channel: "ch1"})

	assert.Equal(t, 1, mockBot.messageCount)
	assert.Contains(t, mockBot.lastMessage, "codex")
	assert.Contains(t, mockBot.lastMessage, "claude")
	assert.Contains(t, mockBot.lastMessage, "gemini")
	assert.Contains(t, mockBot.lastMessage, "opencode")
	assert.Contains(t, mockBot.lastMessage, "built-in")
}

func TestListTemplates_CustomOverridesBuiltIn(t *testing.T) {
	engine := newTestEngine()
	engine.config.SessionTemplates = map[string]SessionTemplate{
		"codex": {CLIType: "codex-stdio", Yolo: true},
	}
	mockBot := &mockBotAdapter{}
	engine.RegisterBotAdapter("testbot", mockBot)

	engine.listTemplates(bot.BotMessage{Platform: "testbot", Channel: "ch1"})

	assert.Equal(t, 1, mockBot.messageCount)
	assert.Contains(t, mockBot.lastMessage, "[yolo]")
	assert.Contains(t, mockBot.lastMessage, "custom")
	assert.Equal(t, 1, strings.Count(mockBot.lastMessage, "**codex**"))
}

func TestListTemplates_SortedOutput(t *testing.T) {
	engine := newTestEngine()
	engine.config.SessionTemplates = map[string]SessionTemplate{
		"zebra":  {CLIType: "test-stdio"},
		"alpha":  {CLIType: "test-stdio"},
		"middle": {CLIType: "test-stdio"},
	}
	mockBot := &mockBotAdapter{}
	engine.RegisterBotAdapter("testbot", mockBot)

	engine.listTemplates(bot.BotMessage{Platform: "testbot", Channel: "ch1"})

	msg := mockBot.lastMessage
	assert.True(t, strings.Index(msg, "alpha") < strings.Index(msg, "middle"))
	assert.True(t, strings.Index(msg, "middle") < strings.Index(msg, "zebra"))
}
