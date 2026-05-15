package core

import (
	"testing"

	"github.com/keepmind9/clibot/internal/bot"
	"github.com/keepmind9/clibot/internal/cli"
	"github.com/stretchr/testify/assert"
)

func TestHandleSwitchSession_Success(t *testing.T) {
	engine := newTestEngine()
	mockOld := &mockCLIAdapter{}
	mockNew := &mockCLIAdapter{}
	engine.RegisterCLIAdapter("claude", mockOld)
	engine.RegisterCLIAdapter("claude-stdio", mockNew)

	engine.sessionMu.Lock()
	engine.sessions["mysession"] = &Session{
		Name:      "mysession",
		CLIType:   "claude",
		WorkDir:   "/tmp/project",
		IsDynamic: true,
		Env:       map[string]string{"FOO": "bar"},
	}
	engine.sessionMu.Unlock()

	mockBot := &mockBotAdapter{}
	engine.activeBots["testbot"] = mockBot

	msg := bot.BotMessage{Platform: "testbot", Channel: "ch1", UserID: "admin"}
	engine.config.Security.Admins = map[string][]string{"testbot": {"admin"}}

	engine.handleSwitchSession([]string{"mysession", "claude-stdio"}, msg)

	engine.sessionMu.RLock()
	defer engine.sessionMu.RUnlock()

	s := engine.sessions["mysession"]
	assert.NotNil(t, s)
	assert.Equal(t, "claude-stdio", s.CLIType)
	assert.Equal(t, "/tmp/project", s.WorkDir)
	assert.Equal(t, "bar", s.Env["FOO"])
	assert.True(t, mockNew.createSessionCalled)
	assert.Contains(t, mockBot.lastMessage, "switched")
}

func TestHandleSwitchSession_NotFound(t *testing.T) {
	engine := newTestEngine()
	engine.RegisterCLIAdapter("claude-stdio", &mockCLIAdapter{})

	mockBot := &mockBotAdapter{}
	engine.activeBots["testbot"] = mockBot

	msg := bot.BotMessage{Platform: "testbot", Channel: "ch1", UserID: "admin"}
	engine.config.Security.Admins = map[string][]string{"testbot": {"admin"}}

	engine.handleSwitchSession([]string{"nonexistent", "claude-stdio"}, msg)
	assert.Contains(t, mockBot.lastMessage, "not found")
}

func TestHandleSwitchSession_StaticSession(t *testing.T) {
	engine := newTestEngine()
	engine.RegisterCLIAdapter("claude-stdio", &mockCLIAdapter{})

	engine.sessionMu.Lock()
	engine.sessions["static"] = &Session{
		Name:      "static",
		CLIType:   "claude",
		IsDynamic: false,
	}
	engine.sessionMu.Unlock()

	mockBot := &mockBotAdapter{}
	engine.activeBots["testbot"] = mockBot

	msg := bot.BotMessage{Platform: "testbot", Channel: "ch1", UserID: "admin"}
	engine.config.Security.Admins = map[string][]string{"testbot": {"admin"}}

	engine.handleSwitchSession([]string{"static", "claude-stdio"}, msg)
	assert.Contains(t, mockBot.lastMessage, "Cannot switch configured session")
}

func TestHandleSwitchSession_InvalidArgs(t *testing.T) {
	engine := newTestEngine()
	mockBot := &mockBotAdapter{}
	engine.activeBots["testbot"] = mockBot

	msg := bot.BotMessage{Platform: "testbot", Channel: "ch1", UserID: "admin"}
	engine.config.Security.Admins = map[string][]string{"testbot": {"admin"}}

	engine.handleSwitchSession([]string{"onlyone"}, msg)
	assert.Contains(t, mockBot.lastMessage, "Invalid arguments")
}

func TestHandleSwitchSession_AdapterNotFound(t *testing.T) {
	engine := newTestEngine()
	mockBot := &mockBotAdapter{}
	engine.activeBots["testbot"] = mockBot

	msg := bot.BotMessage{Platform: "testbot", Channel: "ch1", UserID: "admin"}
	engine.config.Security.Admins = map[string][]string{"testbot": {"admin"}}

	engine.handleSwitchSession([]string{"mysession", "unknown-type"}, msg)
	assert.Contains(t, mockBot.lastMessage, "not found")
}

func TestHandleSwitchSession_NotAdmin(t *testing.T) {
	engine := newTestEngine()
	mockBot := &mockBotAdapter{}
	engine.activeBots["testbot"] = mockBot

	msg := bot.BotMessage{Platform: "testbot", Channel: "ch1", UserID: "user"}

	engine.handleSwitchSession([]string{"mysession", "claude-stdio"}, msg)
	assert.Contains(t, mockBot.lastMessage, "Permission denied")
}

func TestHandleSwitchSession_ResumeOptionPassed(t *testing.T) {
	engine := newTestEngine()
	mockNew := &mockCLIAdapter{}
	engine.RegisterCLIAdapter("claude", &mockCLIAdapter{})
	engine.RegisterCLIAdapter("claude-stdio", mockNew)

	engine.sessionMu.Lock()
	engine.sessions["mysession"] = &Session{
		Name:      "mysession",
		CLIType:   "claude",
		WorkDir:   "/tmp/project",
		IsDynamic: true,
	}
	engine.sessionMu.Unlock()

	mockBot := &mockBotAdapter{}
	engine.activeBots["testbot"] = mockBot

	msg := bot.BotMessage{Platform: "testbot", Channel: "ch1", UserID: "admin"}
	engine.config.Security.Admins = map[string][]string{"testbot": {"admin"}}

	engine.handleSwitchSession([]string{"mysession", "claude-stdio"}, msg)

	assert.True(t, mockNew.createSessionCalled)
	o := cli.ApplySessionOptions(mockNew.lastOpts)
	assert.True(t, o.Resume)
	assert.Equal(t, "/tmp/project", o.WorkDir)
}

func TestHandleSwitchSession_SameType(t *testing.T) {
	engine := newTestEngine()
	mockAdapter := &mockCLIAdapter{}
	engine.RegisterCLIAdapter("claude", mockAdapter)

	engine.sessionMu.Lock()
	engine.sessions["mysession"] = &Session{
		Name:      "mysession",
		CLIType:   "claude",
		WorkDir:   "/tmp/project",
		IsDynamic: true,
	}
	engine.sessionMu.Unlock()

	mockBot := &mockBotAdapter{}
	engine.activeBots["testbot"] = mockBot

	msg := bot.BotMessage{Platform: "testbot", Channel: "ch1", UserID: "admin"}
	engine.config.Security.Admins = map[string][]string{"testbot": {"admin"}}

	engine.handleSwitchSession([]string{"mysession", "claude"}, msg)

	engine.sessionMu.RLock()
	defer engine.sessionMu.RUnlock()
	s := engine.sessions["mysession"]
	assert.NotNil(t, s)
	assert.Equal(t, "claude", s.CLIType)
}
