package core

import (
	"sync"
	"testing"

	"github.com/keepmind9/clibot/internal/bot"
	"github.com/keepmind9/clibot/internal/proxy"
	"github.com/stretchr/testify/assert"
)

// trackingMockBot records all sent messages for fan-out verification.
type trackingMockBot struct {
	bot.DefaultTypingIndicator
	mu       sync.Mutex
	messages []struct {
		channel string
		message string
	}
}

func (m *trackingMockBot) Start(func(bot.BotMessage)) error       { return nil }
func (m *trackingMockBot) Stop() error                            { return nil }
func (m *trackingMockBot) SetMessageHandler(func(bot.BotMessage)) {}
func (m *trackingMockBot) GetMessageHandler() func(bot.BotMessage) {
	return func(bot.BotMessage) {}
}
func (m *trackingMockBot) SetProxyManager(proxy.Manager) {}

func (m *trackingMockBot) SendMessage(channel, message string) error {
	m.mu.Lock()
	m.messages = append(m.messages, struct {
		channel string
		message string
	}{channel: channel, message: message})
	m.mu.Unlock()
	return nil
}

func (m *trackingMockBot) getMessages() []struct {
	channel string
	message string
} {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]struct {
		channel string
		message string
	}, len(m.messages))
	copy(result, m.messages)
	return result
}

// --- Fan-out tests ---

func TestSendResponseToSession_FanOutToMultipleUsers(t *testing.T) {
	config := &Config{
		Sessions: []SessionConfig{
			{Name: "shared", CLIType: "claude", WorkDir: "/tmp"},
		},
	}
	engine := NewEngine(config)

	mockBot := &trackingMockBot{}
	engine.RegisterBotAdapter("testbot", mockBot)

	// Manually set up session and sessionChannels
	engine.sessionMu.Lock()
	engine.sessions["shared"] = &Session{
		Name:    "shared",
		CLIType: "claude",
		State:   StateIdle,
	}
	engine.sessionChannels["shared"] = map[string]BotChannel{
		"testbot:user1": {Platform: "testbot", Channel: "ch-user1"},
		"testbot:user2": {Platform: "testbot", Channel: "ch-user2"},
	}
	engine.sessionMu.Unlock()

	engine.SendResponseToSession("shared", "hello from CLI")

	msgs := mockBot.getMessages()
	assert.Equal(t, 2, len(msgs), "should send to both user channels")

	channels := map[string]string{}
	for _, m := range msgs {
		channels[m.channel] = m.message
	}
	assert.Equal(t, "hello from CLI", channels["ch-user1"])
	assert.Equal(t, "hello from CLI", channels["ch-user2"])
}

func TestSendResponseToSession_SingleUser_BackwardCompatible(t *testing.T) {
	config := &Config{
		Sessions: []SessionConfig{
			{Name: "solo", CLIType: "claude", WorkDir: "/tmp"},
		},
	}
	engine := NewEngine(config)

	mockBot := &trackingMockBot{}
	engine.RegisterBotAdapter("testbot", mockBot)

	engine.sessionMu.Lock()
	engine.sessions["solo"] = &Session{
		Name:    "solo",
		CLIType: "claude",
		State:   StateIdle,
	}
	engine.sessionChannels["solo"] = map[string]BotChannel{
		"testbot:user1": {Platform: "testbot", Channel: "ch-user1"},
	}
	engine.sessionMu.Unlock()

	engine.SendResponseToSession("solo", "single response")

	msgs := mockBot.getMessages()
	assert.Equal(t, 1, len(msgs))
	assert.Equal(t, "ch-user1", msgs[0].channel)
	assert.Equal(t, "single response", msgs[0].message)
}

func TestSendResponseToSession_EmptyResponse_NotSent(t *testing.T) {
	config := &Config{
		Sessions: []SessionConfig{
			{Name: "empty-test", CLIType: "claude", WorkDir: "/tmp"},
		},
	}
	engine := NewEngine(config)

	mockBot := &trackingMockBot{}
	engine.RegisterBotAdapter("testbot", mockBot)

	engine.sessionMu.Lock()
	engine.sessions["empty-test"] = &Session{
		Name:    "empty-test",
		CLIType: "claude",
		State:   StateIdle,
	}
	engine.sessionChannels["empty-test"] = map[string]BotChannel{
		"testbot:user1": {Platform: "testbot", Channel: "ch-user1"},
	}
	engine.sessionMu.Unlock()

	engine.SendResponseToSession("empty-test", "")
	engine.SendResponseToSession("empty-test", "   ")
	engine.SendResponseToSession("empty-test", "\n\t")

	msgs := mockBot.getMessages()
	assert.Equal(t, 0, len(msgs), "empty/whitespace messages should not be sent")
}

// --- sexit tests ---

func TestHandleExitSession_UserExits(t *testing.T) {
	config := &Config{
		Sessions: []SessionConfig{},
		Security: SecurityConfig{
			WhitelistEnabled: true,
			AllowedUsers:     map[string][]string{"testbot": {"user1", "user2"}},
		},
	}
	engine := NewEngine(config)

	mockBot := &trackingMockBot{}
	engine.RegisterBotAdapter("testbot", mockBot)

	engine.sessionMu.Lock()
	engine.sessions["shared"] = &Session{
		Name:    "shared",
		CLIType: "claude",
		State:   StateIdle,
	}
	engine.userSessions["testbot:user1"] = "shared"
	engine.userSessions["testbot:user2"] = "shared"
	engine.sessionChannels["shared"] = map[string]BotChannel{
		"testbot:user1": {Platform: "testbot", Channel: "ch-user1"},
		"testbot:user2": {Platform: "testbot", Channel: "ch-user2"},
	}
	engine.sessionMu.Unlock()

	msg := bot.BotMessage{
		Platform: "testbot",
		Channel:  "ch-user1",
		UserID:   "user1",
	}
	engine.HandleSpecialCommandWithArgs("sexit", nil, msg)

	// user1 should be removed
	engine.sessionMu.RLock()
	_, ok1 := engine.userSessions["testbot:user1"]
	_, ok2 := engine.userSessions["testbot:user2"]
	channels := engine.sessionChannels["shared"]
	engine.sessionMu.RUnlock()

	assert.False(t, ok1, "user1 should be removed from userSessions")
	assert.True(t, ok2, "user2 should remain in userSessions")
	assert.Equal(t, 1, len(channels), "sessionChannels should have 1 entry left")
	_, user2Exists := channels["testbot:user2"]
	assert.True(t, user2Exists, "user2 channel should remain")
}

func TestHandleExitSession_LastUserCleansUpSessionChannels(t *testing.T) {
	config := &Config{
		Sessions: []SessionConfig{},
		Security: SecurityConfig{
			WhitelistEnabled: true,
			AllowedUsers:     map[string][]string{"testbot": {"user1"}},
		},
	}
	engine := NewEngine(config)

	mockBot := &trackingMockBot{}
	engine.RegisterBotAdapter("testbot", mockBot)

	engine.sessionMu.Lock()
	engine.sessions["solo"] = &Session{
		Name:    "solo",
		CLIType: "claude",
		State:   StateIdle,
	}
	engine.userSessions["testbot:user1"] = "solo"
	engine.sessionChannels["solo"] = map[string]BotChannel{
		"testbot:user1": {Platform: "testbot", Channel: "ch-user1"},
	}
	engine.sessionMu.Unlock()

	msg := bot.BotMessage{
		Platform: "testbot",
		Channel:  "ch-user1",
		UserID:   "user1",
	}
	engine.HandleSpecialCommandWithArgs("sexit", nil, msg)

	engine.sessionMu.RLock()
	_, hasUser := engine.userSessions["testbot:user1"]
	_, hasChannels := engine.sessionChannels["solo"]
	engine.sessionMu.RUnlock()

	assert.False(t, hasUser, "user should be removed from userSessions")
	assert.False(t, hasChannels, "sessionChannels entry should be fully deleted when last user exits")
}

func TestHandleExitSession_NotInSession(t *testing.T) {
	config := &Config{
		Sessions: []SessionConfig{},
		Security: SecurityConfig{
			WhitelistEnabled: true,
			AllowedUsers:     map[string][]string{"testbot": {"user1"}},
		},
	}
	engine := NewEngine(config)

	mockBot := &trackingMockBot{}
	engine.RegisterBotAdapter("testbot", mockBot)

	msg := bot.BotMessage{
		Platform: "testbot",
		Channel:  "ch-user1",
		UserID:   "user1",
	}
	engine.HandleSpecialCommandWithArgs("sexit", nil, msg)

	msgs := mockBot.getMessages()
	assert.Equal(t, 1, len(msgs))
	assert.Contains(t, msgs[0].message, "not in any session")
}

func TestHandleExitSession_WithSessionNameArg(t *testing.T) {
	config := &Config{
		Sessions: []SessionConfig{},
		Security: SecurityConfig{
			WhitelistEnabled: true,
			AllowedUsers:     map[string][]string{"testbot": {"user1"}},
		},
	}
	engine := NewEngine(config)

	mockBot := &trackingMockBot{}
	engine.RegisterBotAdapter("testbot", mockBot)

	engine.sessionMu.Lock()
	engine.sessions["my-session"] = &Session{
		Name:    "my-session",
		CLIType: "claude",
		State:   StateIdle,
	}
	engine.userSessions["testbot:user1"] = "my-session"
	engine.sessionChannels["my-session"] = map[string]BotChannel{
		"testbot:user1": {Platform: "testbot", Channel: "ch-user1"},
	}
	engine.sessionMu.Unlock()

	msg := bot.BotMessage{
		Platform: "testbot",
		Channel:  "ch-user1",
		UserID:   "user1",
	}
	engine.HandleSpecialCommandWithArgs("sexit", []string{"my-session"}, msg)

	engine.sessionMu.RLock()
	_, hasUser := engine.userSessions["testbot:user1"]
	_, hasChannels := engine.sessionChannels["my-session"]
	engine.sessionMu.RUnlock()

	assert.False(t, hasUser, "user should be removed from userSessions")
	assert.False(t, hasChannels, "sessionChannels should be cleaned up")

	msgs := mockBot.getMessages()
	assert.Equal(t, 1, len(msgs))
	assert.Contains(t, msgs[0].message, "Exited session 'my-session'")
}
