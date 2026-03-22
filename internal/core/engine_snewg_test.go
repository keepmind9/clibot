package core

import (
	"os"
	"testing"

	"github.com/keepmind9/clibot/internal/bot"
	"github.com/keepmind9/clibot/internal/cli"
	"github.com/stretchr/testify/assert"
)

// mockCLIAdapter is a mock implementation of CLIAdapter for testing
type mockCLIAdapter struct {
	cli.CLIAdapter
	createdSessions map[string]bool
}

func (m *mockCLIAdapter) CreateSession(name, workDir, startCmd, transportURL string) error {
	if m.createdSessions == nil {
		m.createdSessions = make(map[string]bool)
	}
	m.createdSessions[name] = true
	return nil
}

func (m *mockCLIAdapter) IsSessionAlive(name string) bool { return true }
func (m *mockCLIAdapter) ResetSession(name string) error { return nil }

func TestEngine_HandleNewGeminiACPSession(t *testing.T) {
	// Create a temp directory for workDir
	tempDir, err := os.MkdirTemp("", "clibot-test-*")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	config := &Config{
		Security: SecurityConfig{
			Admins: map[string][]string{
				"testbot": {"admin123"},
			},
		},
		Session: SessionGlobalConfig{
			MaxDynamicSessions: 5,
		},
	}
	engine := NewEngine(config)

	// Register mock bot and CLI adapter
	mockBot := &mockBotAdapter{}
	engine.RegisterBotAdapter("testbot", mockBot)
	
	mockCLI := &mockCLIAdapter{}
	engine.cliAdapters["acp"] = mockCLI

	msg := bot.BotMessage{
		Platform: "testbot",
		Channel:  "test-channel",
		UserID:   "admin123",
	}

	// Test case 1: Successful creation
	t.Run("Success", func(t *testing.T) {
		args := []string{"mysess", tempDir}
		engine.handleNewGeminiACPSession(args, msg)

		assert.Equal(t, 1, mockBot.messageCount)
		assert.Contains(t, mockBot.lastMessage, "✅ Gemini ACP session 'mysess' created")
		assert.True(t, mockCLI.createdSessions["mysess"])
		
		// Verify session exists in engine
		engine.sessionMu.RLock()
		sess, exists := engine.sessions["mysess"]
		engine.sessionMu.RUnlock()
		assert.True(t, exists)
		assert.Equal(t, "acp", sess.CLIType)
		assert.Equal(t, "gemini", sess.StartCmd)
		
		// Verify it was selected for the user
		userKey := getUserKey(msg.Platform, msg.UserID)
		assert.Equal(t, "mysess", engine.userSessions[userKey])
	})

	// Test case 2: Not admin
	t.Run("NotAdmin", func(t *testing.T) {
		mockBot.messageCount = 0
		badMsg := bot.BotMessage{Platform: "testbot", UserID: "regular-user", Channel: "ch1"}
		engine.handleNewGeminiACPSession([]string{"fail", tempDir}, badMsg)
		assert.Contains(t, mockBot.lastMessage, "Permission denied")
	})

	// Test case 3: Invalid args
	t.Run("InvalidArgs", func(t *testing.T) {
		mockBot.messageCount = 0
		engine.handleNewGeminiACPSession([]string{"only-one-arg"}, msg)
		assert.Contains(t, mockBot.lastMessage, "Invalid arguments")
	})

	// Test case 4: Directory does not exist
	t.Run("DirNotFound", func(t *testing.T) {
		mockBot.messageCount = 0
		engine.handleNewGeminiACPSession([]string{"bad-dir", "/non/existent/path/clibot"}, msg)
		assert.Contains(t, mockBot.lastMessage, "does not exist")
	})
}

func TestIsSpecialCommand_Snewg(t *testing.T) {
	t.Run("Normal", func(t *testing.T) {
		cmd, isCmd, args := isSpecialCommand("snewg my-sess /tmp")
		assert.True(t, isCmd)
		assert.Equal(t, "snewg", cmd)
		assert.Equal(t, []string{"my-sess", "/tmp"}, args)
	})

	t.Run("WithSlash", func(t *testing.T) {
		cmd, isCmd, args := isSpecialCommand("/snewg my-sess /tmp")
		assert.True(t, isCmd)
		assert.Equal(t, "snewg", cmd)
		assert.Equal(t, []string{"my-sess", "/tmp"}, args)
	})

	t.Run("HelpWithSlash", func(t *testing.T) {
		cmd, isCmd, args := isSpecialCommand("/help")
		assert.True(t, isCmd)
		assert.Equal(t, "help", cmd)
		assert.Nil(t, args)
	})
}
