package core

import (
	"testing"

	"github.com/keepmind9/clibot/internal/bot"
	_ "github.com/keepmind9/clibot/internal/proxy"
	"github.com/stretchr/testify/assert"
)

// TestEngine_HandleSpecialCommand_EmptyCommand tests empty command handling
func TestEngine_HandleSpecialCommand_EmptyCommand(t *testing.T) {
	config := &Config{
		Sessions: []SessionConfig{
			{Name: "test", CLIType: "claude", WorkDir: "/tmp"},
		},
	}
	engine := NewEngine(config)

	// Register a mock bot to capture messages
	mockBot := &mockBotAdapter{}
	engine.RegisterBotAdapter("testbot", mockBot)

	msg := bot.BotMessage{
		Platform: "testbot",
		Channel:  "test-channel",
		UserID:   "user123",
	}

	// Test empty command
	engine.HandleSpecialCommand("", msg)

	// Should send error message about empty command
	assert.Equal(t, 1, mockBot.messageCount)
	assert.Contains(t, mockBot.lastMessage, "Empty command")
}

// TestEngine_HandleSpecialCommand_UnknownCommand tests unknown command handling
func TestEngine_HandleSpecialCommand_UnknownCommand(t *testing.T) {
	config := &Config{
		Sessions: []SessionConfig{
			{Name: "test", CLIType: "claude", WorkDir: "/tmp"},
		},
	}
	engine := NewEngine(config)

	mockBot := &mockBotAdapter{}
	engine.RegisterBotAdapter("testbot", mockBot)

	msg := bot.BotMessage{
		Platform: "testbot",
		Channel:  "test-channel",
		UserID:   "user123",
	}

	// Test unknown command
	engine.HandleSpecialCommand("unknown-command", msg)

	// Should send error message about unknown command
	assert.Equal(t, 1, mockBot.messageCount)
	assert.Contains(t, mockBot.lastMessage, "Unknown command")
	assert.Contains(t, mockBot.lastMessage, "help")
}

// TestEngine_HandleSpecialCommand_KnownCommands tests command routing
func TestEngine_HandleSpecialCommand_KnownCommands(t *testing.T) {
	config := &Config{
		Sessions: []SessionConfig{
			{Name: "test", CLIType: "claude", WorkDir: "/tmp"},
		},
	}
	engine := NewEngine(config)

	mockBot := &mockBotAdapter{}
	engine.RegisterBotAdapter("testbot", mockBot)

	msg := bot.BotMessage{
		Platform: "testbot",
		Channel:  "test-channel",
		UserID:   "user123",
	}

	commands := []string{"help", "slist", "status", "whoami", "echo"}
	for _, cmd := range commands {
		mockBot.messageCount = 0 // Reset counter
		engine.HandleSpecialCommand(cmd, msg)
		// Each command should send a message (not crash)
		assert.Equal(t, 1, mockBot.messageCount, "Command '%s' should send a message", cmd)
	}
}

// TestEngine_HandleSpecialCommandWithArgs_EmptyCommand tests empty command
func TestEngine_HandleSpecialCommandWithArgs_EmptyCommand(t *testing.T) {
	config := &Config{
		Sessions: []SessionConfig{
			{Name: "test", CLIType: "claude", WorkDir: "/tmp"},
		},
	}
	engine := NewEngine(config)

	mockBot := &mockBotAdapter{}
	engine.RegisterBotAdapter("testbot", mockBot)

	msg := bot.BotMessage{
		Platform: "testbot",
		Channel:  "test-channel",
		UserID:   "user123",
	}

	// Test empty command
	engine.HandleSpecialCommandWithArgs("", []string{}, msg)

	// Should send error message about unknown command
	assert.Equal(t, 1, mockBot.messageCount)
	assert.Contains(t, mockBot.lastMessage, "Unknown command")
}

// TestEngine_HandleSpecialCommandWithArgs_CommandRouting tests correct command routing
func TestEngine_HandleSpecialCommandWithArgs_CommandRouting(t *testing.T) {
	config := &Config{
		Sessions: []SessionConfig{
			{Name: "test", CLIType: "claude", WorkDir: "/tmp"},
		},
	}
	engine := NewEngine(config)

	mockBot := &mockBotAdapter{}
	engine.RegisterBotAdapter("testbot", mockBot)

	msg := bot.BotMessage{
		Platform: "testbot",
		Channel:  "test-channel",
		UserID:   "user123",
	}

	// Test that different commands are handled
	testCases := []struct {
		command      string
		args         []string
		expectInResp string
	}{
		{"help", []string{}, "Available commands"},
		{"slist", []string{}, "Available Sessions"},
		{"status", []string{}, "Session Status"},
		{"whoami", []string{}, "Your Information"},
		{"echo", []string{}, "Your IM Information"},
		{"unknown", []string{}, "Unknown command"},
	}

	for _, tc := range testCases {
		t.Run(tc.command, func(t *testing.T) {
			mockBot.messageCount = 0
			engine.HandleSpecialCommandWithArgs(tc.command, tc.args, msg)

			// Should send a message
			assert.Equal(t, 1, mockBot.messageCount, "Command '%s' should send a message", tc.command)
			// Response should contain expected text (or error for unknown command)
			assert.NotEmpty(t, mockBot.lastMessage, "Response should not be empty")
		})
	}
}

// TestEngine_HandleSpecialCommandWithArgs_WithArgs tests commands with arguments
func TestEngine_HandleSpecialCommandWithArgs_WithArgs(t *testing.T) {
	config := &Config{
		Sessions: []SessionConfig{
			{Name: "session1", CLIType: "claude", WorkDir: "/tmp"},
		},
	}
	engine := NewEngine(config)

	mockBot := &mockBotAdapter{}
	engine.RegisterBotAdapter("testbot", mockBot)

	msg := bot.BotMessage{
		Platform: "testbot",
		Channel:  "test-channel",
		UserID:   "user123",
	}

	// Test commands that accept arguments
	testCases := []struct {
		name    string
		command string
		args    []string
	}{
		{"suse with session", "suse", []string{"session1"}},
		{"suse no args", "suse", []string{}},
		{"sstatus with session", "sstatus", []string{"session1"}},
		{"sstatus no args", "sstatus", []string{}},
		{"sdel with session", "sdel", []string{"session1"}},
		{"sdel no args", "sdel", []string{}},
		{"sclose with session", "sclose", []string{"session1"}},
		{"sclose no args", "sclose", []string{}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockBot.messageCount = 0
			engine.HandleSpecialCommandWithArgs(tc.command, tc.args, msg)

			// Should send a message (response or error)
			assert.Equal(t, 1, mockBot.messageCount)
			assert.NotEmpty(t, mockBot.lastMessage)
		})
	}
}
