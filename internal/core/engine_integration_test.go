//go:build integration

package core

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/keepmind9/clibot/internal/bot"
	"github.com/keepmind9/clibot/internal/cli/stdio"
	"github.com/keepmind9/clibot/internal/logger"
	"github.com/keepmind9/clibot/internal/proxy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// syncBotAdapter captures bot messages with synchronization for integration tests.
type syncBotAdapter struct {
	bot.DefaultTypingIndicator
	mu       sync.Mutex
	messages []string
	notify   chan struct{}
}

func newSyncBotAdapter() *syncBotAdapter {
	return &syncBotAdapter{
		notify: make(chan struct{}, 10),
	}
}

func (s *syncBotAdapter) Start(func(bot.BotMessage)) error       { return nil }
func (s *syncBotAdapter) Stop() error                            { return nil }
func (s *syncBotAdapter) SetMessageHandler(func(bot.BotMessage)) {}
func (s *syncBotAdapter) GetMessageHandler() func(bot.BotMessage) {
	return func(bot.BotMessage) {}
}
func (s *syncBotAdapter) SetProxyManager(proxy.Manager) {}

func (s *syncBotAdapter) SendMessage(channel, message string) error {
	s.mu.Lock()
	s.messages = append(s.messages, message)
	s.mu.Unlock()
	s.notify <- struct{}{}
	return nil
}

func (s *syncBotAdapter) waitForMessage(timeout time.Duration) (string, error) {
	select {
	case <-s.notify:
		s.mu.Lock()
		defer s.mu.Unlock()
		if len(s.messages) == 0 {
			return "", fmt.Errorf("no messages")
		}
		return s.messages[len(s.messages)-1], nil
	case <-time.After(timeout):
		return "", fmt.Errorf("timeout waiting for message after %v", timeout)
	}
}

func (s *syncBotAdapter) reset() {
	s.mu.Lock()
	s.messages = nil
	s.mu.Unlock()
	for {
		select {
		case <-s.notify:
		default:
			return
		}
	}
}

// newIntegrationEngine creates an Engine with real stdio adapters for integration testing.
func newIntegrationEngine(t *testing.T) (*Engine, *syncBotAdapter) {
	_ = logger.InitLogger(logger.Config{Level: "info"})

	config := &Config{
		Session: SessionGlobalConfig{
			MaxDynamicSessions: 10,
			IdleTimeout:        "30m",
		},
		SessionTemplates: defaultBuiltinTemplates(),
	}

	engine := NewEngine(config)

	testBot := newSyncBotAdapter()
	engine.RegisterBotAdapter("itest", testBot)
	engine.config.Security.Admins = map[string][]string{"itest": {"test-admin"}}

	claudeAdapter := stdio.NewStdioAdapter(stdio.ClaudeSpec{}, stdio.StdioAdapterConfig{
		IdleTimeout: 5 * time.Minute,
	})
	claudeAdapter.SetEngine(engine)
	engine.RegisterCLIAdapter("claude-stdio", claudeAdapter)

	codexAdapter := stdio.NewStdioAdapter(stdio.CodexSpec{}, stdio.StdioAdapterConfig{
		IdleTimeout: 5 * time.Minute,
	})
	codexAdapter.SetEngine(engine)
	engine.RegisterCLIAdapter("codex-stdio", codexAdapter)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case msg := <-engine.messageChan:
				engine.HandleUserMessage(msg)
			}
		}
	}()

	t.Cleanup(func() {
		cancel()
		for name, s := range engine.sessions {
			if adapter, ok := engine.cliAdapters[s.CLIType]; ok {
				adapter.StopSession(name)
			}
		}
	})

	return engine, testBot
}

func itestMsg(content string) bot.BotMessage {
	return bot.BotMessage{
		Platform: "itest",
		Channel:  "test-channel",
		UserID:   "test-admin",
		Content:  content,
	}
}

// TestIntegration_SwitchClaudeResume tests:
// 1. Create claude session, tell it a secret code
// 2. sswitch to codex-stdio
// 3. sswitch back to claude-stdio
// 4. Verify claude remembers the secret (resume works)
func TestIntegration_SwitchClaudeResume(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	engine, testBot := newIntegrationEngine(t)
	sessionName := "itest-switch-1"
	workDir := "/data/app/workspace/me/demo"

	// Step 1: Create claude session via "sn" command
	t.Log("Step 1: Creating claude session...")
	testBot.reset()
	engine.HandleBotMessage(itestMsg(fmt.Sprintf("sn claude %s %s", workDir, sessionName)))

	resp, err := testBot.waitForMessage(30 * time.Second)
	require.NoError(t, err, "timeout creating session")
	assert.Contains(t, resp, "created", "expected session creation, got: %s", resp)
	t.Logf("  -> %s", shortLog(resp))

	// Step 1b: Select the session
	t.Log("Step 1b: Selecting session via suse...")
	testBot.reset()
	engine.HandleBotMessage(itestMsg(fmt.Sprintf("suse %s", sessionName)))

	resp, err = testBot.waitForMessage(5 * time.Second)
	require.NoError(t, err, "timeout selecting session")
	t.Logf("  -> %s", shortLog(resp))

	// Step 2: Tell claude a secret to establish conversation context
	t.Log("Step 2: Telling claude a secret code (7423)...")
	testBot.reset()
	engine.HandleBotMessage(itestMsg("My secret code is exactly 7423. Please remember this number for later."))

	resp, err = testBot.waitForMessage(3 * time.Minute)
	require.NoError(t, err, "timeout waiting for claude response")
	t.Logf("  -> %s", shortLog(resp))
	require.NotEmpty(t, resp, "claude should respond")

	// Step 3: sswitch to codex-stdio
	t.Log("Step 3: sswitch -> codex-stdio...")
	testBot.reset()
	engine.HandleBotMessage(itestMsg(fmt.Sprintf("sswitch %s codex-stdio", sessionName)))

	resp, err = testBot.waitForMessage(30 * time.Second)
	require.NoError(t, err, "timeout switching to codex")
	assert.Contains(t, resp, "switched", "got: %s", resp)
	t.Logf("  -> %s", shortLog(resp))

	engine.sessionMu.RLock()
	sess := engine.sessions[sessionName]
	engine.sessionMu.RUnlock()
	assert.Equal(t, "codex-stdio", sess.CLIType)

	// Step 4: sswitch back to claude-stdio
	t.Log("Step 4: sswitch -> claude-stdio (resume)...")
	testBot.reset()
	engine.HandleBotMessage(itestMsg(fmt.Sprintf("sswitch %s claude-stdio", sessionName)))

	resp, err = testBot.waitForMessage(30 * time.Second)
	require.NoError(t, err, "timeout switching back to claude")
	assert.Contains(t, resp, "switched", "got: %s", resp)
	t.Logf("  -> %s", shortLog(resp))

	engine.sessionMu.RLock()
	sess = engine.sessions[sessionName]
	engine.sessionMu.RUnlock()
	assert.Equal(t, "claude-stdio", sess.CLIType)

	// Step 5: Verify resume - ask about the secret code
	t.Log("Step 5: Asking claude about the secret code (testing resume)...")
	testBot.reset()
	engine.HandleBotMessage(itestMsg("What is my secret code? Reply with ONLY the 4-digit number."))

	resp, err = testBot.waitForMessage(3 * time.Minute)
	require.NoError(t, err, "timeout waiting for claude resume response")
	t.Logf("  -> %s", shortLog(resp))
	assert.Contains(t, resp, "7423",
		"claude should remember secret code 7423 after resume")

	t.Log("PASSED: claude sswitch + resume works correctly")
}

// TestIntegration_SwitchCodexResume tests:
// 1. Create codex session, tell it a keyword
// 2. sswitch to claude-stdio
// 3. sswitch back to codex-stdio
// 4. Verify codex remembers the keyword (resume works)
func TestIntegration_SwitchCodexResume(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	engine, testBot := newIntegrationEngine(t)
	sessionName := "itest-switch-2"
	workDir := "/data/app/workspace/me/demo"

	// Step 1: Create codex session
	t.Log("Step 1: Creating codex session...")
	testBot.reset()
	engine.HandleBotMessage(itestMsg(fmt.Sprintf("sn codex %s %s", workDir, sessionName)))

	resp, err := testBot.waitForMessage(30 * time.Second)
	require.NoError(t, err, "timeout creating codex session")
	assert.Contains(t, resp, "created", "got: %s", resp)
	t.Logf("  -> %s", shortLog(resp))

	// Step 1b: Select the session
	t.Log("Step 1b: Selecting session via suse...")
	testBot.reset()
	engine.HandleBotMessage(itestMsg(fmt.Sprintf("suse %s", sessionName)))

	resp, err = testBot.waitForMessage(5 * time.Second)
	require.NoError(t, err, "timeout selecting session")
	t.Logf("  -> %s", shortLog(resp))

	// Step 2: Tell codex a keyword
	t.Log("Step 2: Telling codex a keyword (RAINBOW)...")
	testBot.reset()
	engine.HandleBotMessage(itestMsg("The magic keyword is RAINBOW. Remember it."))

	resp, err = testBot.waitForMessage(3 * time.Minute)
	require.NoError(t, err, "timeout waiting for codex response")
	t.Logf("  -> %s", shortLog(resp))
	require.NotEmpty(t, resp, "codex should respond")

	// Step 3: sswitch to claude-stdio
	t.Log("Step 3: sswitch -> claude-stdio...")
	testBot.reset()
	engine.HandleBotMessage(itestMsg(fmt.Sprintf("sswitch %s claude-stdio", sessionName)))

	resp, err = testBot.waitForMessage(30 * time.Second)
	require.NoError(t, err, "timeout switching to claude")
	assert.Contains(t, resp, "switched", "got: %s", resp)
	t.Logf("  -> %s", shortLog(resp))

	// Step 4: sswitch back to codex-stdio
	t.Log("Step 4: sswitch -> codex-stdio (resume)...")
	testBot.reset()
	engine.HandleBotMessage(itestMsg(fmt.Sprintf("sswitch %s codex-stdio", sessionName)))

	resp, err = testBot.waitForMessage(30 * time.Second)
	require.NoError(t, err, "timeout switching back to codex")
	assert.Contains(t, resp, "switched", "got: %s", resp)
	t.Logf("  -> %s", shortLog(resp))

	// Step 5: Verify codex resume
	t.Log("Step 5: Asking codex about the keyword (testing resume)...")
	testBot.reset()
	engine.HandleBotMessage(itestMsg("What is the magic keyword? Reply with ONLY the word."))

	resp, err = testBot.waitForMessage(3 * time.Minute)
	require.NoError(t, err, "timeout waiting for codex resume response")
	t.Logf("  -> %s", shortLog(resp))
	assert.Contains(t, resp, "RAINBOW",
		"codex should remember keyword RAINBOW after resume")

	t.Log("PASSED: codex sswitch + resume works correctly")
}

func shortLog(s string) string {
	if len(s) <= 200 {
		return s
	}
	return s[:200] + "..."
}
