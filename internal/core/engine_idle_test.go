package core

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCleanIdleSessions_RemovesExpired(t *testing.T) {
	engine := newTestEngine()
	engine.config.Security.Admins = map[string][]string{"testbot": {"admin1"}}
	engine.RegisterCLIAdapter("codex-stdio", &mockCLIAdapter{})

	// Create a dynamic session with old LastActiveAt
	engine.sessionMu.Lock()
	engine.sessions["old-session"] = &Session{
		Name:         "old-session",
		CLIType:      "codex-stdio",
		IsDynamic:    true,
		LastActiveAt: time.Now().Add(-2 * time.Hour),
		CreatedBy:    "testbot:admin1",
	}
	engine.sessions["active-session"] = &Session{
		Name:         "active-session",
		CLIType:      "codex-stdio",
		IsDynamic:    true,
		LastActiveAt: time.Now(),
		CreatedBy:    "testbot:admin1",
	}
	engine.sessionMu.Unlock()

	engine.cleanIdleSessions(1 * time.Hour)

	engine.sessionMu.RLock()
	defer engine.sessionMu.RUnlock()
	_, oldExists := engine.sessions["old-session"]
	_, activeExists := engine.sessions["active-session"]
	assert.False(t, oldExists, "expired session should be removed")
	assert.True(t, activeExists, "active session should remain")
}

func TestCleanIdleSessions_SkipsNonDynamic(t *testing.T) {
	engine := newTestEngine()
	engine.RegisterCLIAdapter("test-stdio", &mockCLIAdapter{})

	engine.sessionMu.Lock()
	engine.sessions["static-session"] = &Session{
		Name:         "static-session",
		CLIType:      "test-stdio",
		IsDynamic:    false,
		LastActiveAt: time.Now().Add(-24 * time.Hour),
	}
	engine.sessionMu.Unlock()

	engine.cleanIdleSessions(1 * time.Hour)

	engine.sessionMu.RLock()
	defer engine.sessionMu.RUnlock()
	_, exists := engine.sessions["static-session"]
	assert.True(t, exists, "static session should not be cleaned")
}

func TestCleanIdleSessions_SkipsZeroLastActiveAt(t *testing.T) {
	engine := newTestEngine()
	engine.RegisterCLIAdapter("codex-stdio", &mockCLIAdapter{})

	engine.sessionMu.Lock()
	engine.sessions["zero-session"] = &Session{
		Name:      "zero-session",
		CLIType:   "codex-stdio",
		IsDynamic: true,
	}
	engine.sessionMu.Unlock()

	engine.cleanIdleSessions(1 * time.Hour)

	engine.sessionMu.RLock()
	defer engine.sessionMu.RUnlock()
	_, exists := engine.sessions["zero-session"]
	assert.True(t, exists, "session with zero LastActiveAt should not be cleaned")
}

func TestCleanIdleSessions_CleansUserSessions(t *testing.T) {
	engine := newTestEngine()
	engine.RegisterCLIAdapter("codex-stdio", &mockCLIAdapter{})

	engine.sessionMu.Lock()
	engine.sessions["idle-s"] = &Session{
		Name:         "idle-s",
		CLIType:      "codex-stdio",
		IsDynamic:    true,
		LastActiveAt: time.Now().Add(-3 * time.Hour),
		CreatedBy:    "testbot:user1",
	}
	engine.userSessions["testbot:user1"] = "idle-s"
	engine.sessionChannels["idle-s"] = map[string]BotChannel{"testbot:user1": {Platform: "testbot", Channel: "ch1"}}
	engine.sessionMu.Unlock()

	engine.cleanIdleSessions(1 * time.Hour)

	engine.sessionMu.RLock()
	defer engine.sessionMu.RUnlock()
	_, sessionExists := engine.sessions["idle-s"]
	_, userExists := engine.userSessions["testbot:user1"]
	_, channelExists := engine.sessionChannels["idle-s"]
	assert.False(t, sessionExists)
	assert.False(t, userExists, "user session mapping should be cleaned")
	assert.False(t, channelExists, "session channel mapping should be cleaned")
}

func TestCleanIdleSessions_CleansSessionChannels(t *testing.T) {
	engine := newTestEngine()
	engine.RegisterCLIAdapter("codex-stdio", &mockCLIAdapter{})

	engine.sessionMu.Lock()
	engine.sessions["idle-ch"] = &Session{
		Name:         "idle-ch",
		CLIType:      "codex-stdio",
		IsDynamic:    true,
		LastActiveAt: time.Now().Add(-2 * time.Hour),
		CreatedBy:    "testbot:user1",
	}
	engine.sessionChannels["idle-ch"] = map[string]BotChannel{"testbot:user1": {Platform: "testbot", Channel: "ch1"}}
	engine.sessionMu.Unlock()

	engine.cleanIdleSessions(1 * time.Hour)

	engine.sessionMu.RLock()
	defer engine.sessionMu.RUnlock()
	_, channelExists := engine.sessionChannels["idle-ch"]
	assert.False(t, channelExists, "session channel should be cleaned up")
}

func TestTouchSession(t *testing.T) {
	engine := newTestEngine()
	before := time.Now()

	engine.sessionMu.Lock()
	engine.sessions["s1"] = &Session{Name: "s1", LastActiveAt: time.Time{}}
	engine.sessionMu.Unlock()

	engine.touchSession("s1")

	engine.sessionMu.RLock()
	defer engine.sessionMu.RUnlock()
	assert.True(t, engine.sessions["s1"].LastActiveAt.After(before))
}

func TestTouchSession_NoPanicOnMissing(t *testing.T) {
	engine := newTestEngine()
	assert.NotPanics(t, func() {
		engine.touchSession("nonexistent")
	})
}

func TestIdleSessionCleaner_DisabledOnZero(t *testing.T) {
	config := &Config{
		Sessions: []SessionConfig{{Name: "default", CLIType: "test-stdio", WorkDir: "/tmp"}},
		Session:  SessionGlobalConfig{IdleTimeout: "0"},
	}
	engine := NewEngine(config)

	// Should return immediately without blocking
	done := make(chan struct{})
	go func() {
		engine.startIdleSessionCleaner()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("startIdleSessionCleaner should return immediately when idle_timeout is 0")
	}
}

func TestIdleSessionCleaner_InvalidDuration(t *testing.T) {
	config := &Config{
		Sessions: []SessionConfig{{Name: "default", CLIType: "test-stdio", WorkDir: "/tmp"}},
		Session:  SessionGlobalConfig{IdleTimeout: "invalid"},
	}
	engine := NewEngine(config)

	done := make(chan struct{})
	go func() {
		engine.startIdleSessionCleaner()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("startIdleSessionCleaner should return immediately on invalid duration")
	}
}

func TestHandleNewFromTemplate_SetsLastActiveAt(t *testing.T) {
	engine := newTestEngine()
	engine.config.Security.Admins = map[string][]string{"testbot": {"admin1"}}
	engine.RegisterCLIAdapter("codex-stdio", &mockCLIAdapter{})
	before := time.Now()

	engine.handleNewFromTemplate([]string{"codex", "/tmp", "last-active-test"}, adminMsg())

	engine.sessionMu.RLock()
	s := engine.sessions["last-active-test"]
	engine.sessionMu.RUnlock()
	require.NotNil(t, s)
	assert.True(t, s.LastActiveAt.After(before))
}
