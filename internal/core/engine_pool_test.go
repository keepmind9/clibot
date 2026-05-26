package core

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/keepmind9/clibot/internal/bot"
	"github.com/keepmind9/clibot/internal/cli"
	"github.com/stretchr/testify/assert"
)

// mockPoolCLI implements cli.CLIAdapter for pool testing.
// SendInput blocks until releaseCh is closed, simulating a long-running CLI run.
type mockPoolCLI struct {
	running   atomic.Int32
	releaseCh chan struct{}
}

func (m *mockPoolCLI) CreateSession(sessionName string, opts ...cli.SessionOption) error {
	return nil
}
func (m *mockPoolCLI) IsSessionAlive(sessionName string) bool { return true }
func (m *mockPoolCLI) StopSession(sessionName string) error   { return nil }
func (m *mockPoolCLI) HandleHookData(data []byte) (string, string, string, error) {
	return "", "", "", nil
}

func (m *mockPoolCLI) SendInput(sessionName, input string) error {
	m.running.Add(1)
	<-m.releaseCh
	m.running.Add(-1)
	return nil
}

func newPoolTestEngine(maxConcurrent int) (*Engine, *mockPoolCLI) {
	config := &Config{
		Session: SessionGlobalConfig{
			MaxConcurrentRuns: maxConcurrent,
		},
		Sessions: []SessionConfig{{Name: "s1", CLIType: "pool-cli", WorkDir: "/tmp"}},
		Bots: map[string]BotConfig{
			"testbot": {Enabled: true},
		},
		Security: SecurityConfig{WhitelistEnabled: false},
	}
	engine := NewEngine(config)
	poolCLI := &mockPoolCLI{releaseCh: make(chan struct{})}
	engine.RegisterCLIAdapter("pool-cli", poolCLI)
	engine.RegisterBotAdapter("testbot", &mockBotAdapter{})
	engine.initializeSessions()

	// Pre-select session for test user
	engine.userSessions["testbot:u1"] = "s1"

	return engine, poolCLI
}

func TestPool_DisabledWhenZero(t *testing.T) {
	engine, poolCLI := newPoolTestEngine(0)
	assert.Nil(t, engine.pool)

	msg := bot.BotMessage{
		Platform: "testbot",
		Channel:  "ch1",
		UserID:   "u1",
		Content:  "hello",
	}

	go engine.HandleUserMessage(msg)

	assert.Eventually(t, func() bool {
		return poolCLI.running.Load() == 1
	}, time.Second, 50*time.Millisecond)

	close(poolCLI.releaseCh)
}

func TestPool_ReleaseOnSendInputError(t *testing.T) {
	engine, _ := newPoolTestEngine(1)
	assert.Equal(t, 1, engine.pool.Cap())

	// Register a CLI adapter that errors
	errCLI := &errorCLI{}
	engine.RegisterCLIAdapter("pool-cli", errCLI)

	msg := bot.BotMessage{
		Platform: "testbot",
		Channel:  "ch1",
		UserID:   "u1",
		Content:  "hello",
	}

	engine.HandleUserMessage(msg)

	// Pool should have a free slot (permit was released)
	done := make(chan struct{})
	go func() {
		engine.acquirePool("s1")
		engine.releasePool("s1")
		close(done)
	}()
	select {
	case <-done:
		// success
	case <-time.After(500 * time.Millisecond):
		t.Fatal("pool permit was not released after SendInput error")
	}
}

type errorCLI struct{}

func (e *errorCLI) CreateSession(sessionName string, opts ...cli.SessionOption) error {
	return nil
}
func (e *errorCLI) IsSessionAlive(sessionName string) bool { return true }
func (e *errorCLI) StopSession(sessionName string) error   { return nil }
func (e *errorCLI) HandleHookData(data []byte) (string, string, string, error) {
	return "", "", "", nil
}
func (e *errorCLI) SendInput(sessionName, input string) error {
	return assert.AnError
}

func TestPool_ReleaseIdempotent(t *testing.T) {
	engine, _ := newPoolTestEngine(1)

	engine.acquirePool("s1")
	assert.True(t, engine.poolPermits["s1"])

	// First release
	engine.releasePool("s1")
	assert.False(t, engine.poolPermits["s1"])

	// Double release should not panic or cause underflow
	assert.NotPanics(t, func() {
		engine.releasePool("s1")
		engine.releasePool("s1")
	})

	// Pool should have capacity available
	done := make(chan struct{})
	go func() {
		engine.acquirePool("s1")
		engine.releasePool("s1")
		close(done)
	}()
	select {
	case <-done:
		// success
	case <-time.After(500 * time.Millisecond):
		t.Fatal("pool should have capacity after idempotent releases")
	}
}
func TestPool_StreamingPathRelease(t *testing.T) {
	config := &Config{
		Session:  SessionGlobalConfig{MaxConcurrentRuns: 1},
		Sessions: []SessionConfig{{Name: "default", CLIType: "test-stream", WorkDir: "/tmp"}},
		Bots: map[string]BotConfig{
			"testbot": {Enabled: true, ReplyMode: "card"},
		},
		Security: SecurityConfig{WhitelistEnabled: false},
	}
	engine := NewEngine(config)

	streamCLI := &mockStreamingCLI{}
	richBot := &mockRichBot{rich: &mockRichMessenger{}}
	engine.RegisterCLIAdapter("test-stream", streamCLI)
	engine.RegisterBotAdapter("testbot", richBot)
	engine.initializeSessions()

	// Pre-select session for test user
	engine.userSessions["testbot:u1"] = "default"

	streamCLI.events = []cli.CLIEvent{
		{Type: cli.CLIEventDone, Content: "Done!"},
	}

	msg := bot.BotMessage{
		Platform: "testbot",
		Channel:  "ch1",
		UserID:   "u1",
		Content:  "hello",
	}

	engine.HandleUserMessage(msg)

	// Wait for streaming to complete
	assert.Eventually(t, func() bool {
		h := richBot.rich.getHandle()
			return h != nil && h.finished.Load()
	}, 2*time.Second, 50*time.Millisecond)

	// Pool permit should be released — can acquire again
	done := make(chan struct{})
	go func() {
		engine.acquirePool("default")
		engine.releasePool("default")
		close(done)
	}()
	select {
	case <-done:
		// success
	case <-time.After(500 * time.Millisecond):
		t.Fatal("pool permit was not released after streaming completed")
	}
}

func TestPool_SendResponseToSessionRelease(t *testing.T) {
	engine, _ := newPoolTestEngine(1)

	// Use a non-blocking CLI that responds immediately via the engine
	respondCLI := &respondCLI{engine: engine}
	engine.RegisterCLIAdapter("pool-cli", respondCLI)

	msg := bot.BotMessage{
		Platform: "testbot",
		Channel:  "ch1",
		UserID:   "u1",
		Content:  "hello",
	}

	// Register a channel for the session so SendResponseToSession can deliver
	engine.sessionMu.Lock()
	engine.sessionChannels["s1"] = map[string]BotChannel{
		"testbot:ch1": {Platform: "testbot", Channel: "ch1"},
	}
	engine.sessionMu.Unlock()

	engine.HandleUserMessage(msg)

	// After response delivery, pool permit should be released
	done := make(chan struct{})
	go func() {
		engine.acquirePool("s1")
		engine.releasePool("s1")
		close(done)
	}()
	select {
	case <-done:
		// success
	case <-time.After(500 * time.Millisecond):
		t.Fatal("pool permit was not released after SendResponseToSession")
	}
}

// respondCLI sends a response immediately via the engine.
type respondCLI struct {
	engine *Engine
}

func (r *respondCLI) CreateSession(sessionName string, opts ...cli.SessionOption) error {
	return nil
}
func (r *respondCLI) IsSessionAlive(sessionName string) bool { return true }
func (r *respondCLI) StopSession(sessionName string) error   { return nil }
func (r *respondCLI) HandleHookData(data []byte) (string, string, string, error) {
	return "", "", "", nil
}
func (r *respondCLI) SendInput(sessionName, input string) error {
	go r.engine.SendResponseToSession(sessionName, "response text")
	return nil
}
