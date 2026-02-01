package core

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/keepmind9/clibot/internal/bot"
)

// MockCLIAdapter is a mock implementation of CLIAdapter for testing
type MockCLIAdapter struct {
	sendInputCalls     []SendInputCall
	getLastResponse    string
	getLastResponseErr error
	isAlive            bool
	createSessionErr   error
	checkInteractive   CheckInteractiveResult
	mu                 sync.Mutex
}

type SendInputCall struct {
	SessionName string
	Input       string
}

type CheckInteractiveResult struct {
	Waiting bool
	Prompt  string
	Err     error
}

func (m *MockCLIAdapter) SendInput(sessionName, input string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sendInputCalls = append(m.sendInputCalls, SendInputCall{
		SessionName: sessionName,
		Input:       input,
	})
	return nil
}

func (m *MockCLIAdapter) GetLastResponse(sessionName string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.getLastResponseErr != nil {
		return "", m.getLastResponseErr
	}
	return m.getLastResponse, nil
}

func (m *MockCLIAdapter) IsSessionAlive(sessionName string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.isAlive
}

func (m *MockCLIAdapter) CreateSession(sessionName, cliType, workDir string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.createSessionErr
}

func (m *MockCLIAdapter) CheckInteractive(sessionName string) (bool, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.checkInteractive.Waiting, m.checkInteractive.Prompt, m.checkInteractive.Err
}

func (m *MockCLIAdapter) HandleHookData(data []byte) (string, string, string, error) {
	// Mock implementation for testing
	return "mock-cwd", "mock-prompt", "mock-response", nil
}

func (m *MockCLIAdapter) GetSendInputCalls() []SendInputCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	calls := make([]SendInputCall, len(m.sendInputCalls))
	copy(calls, m.sendInputCalls)
	return calls
}

func (m *MockCLIAdapter) SetIsAlive(alive bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.isAlive = alive
}

func (m *MockCLIAdapter) SetLastResponse(response string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.getLastResponse = response
	m.getLastResponseErr = err
}

func (m *MockCLIAdapter) SetCreateSessionError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.createSessionErr = err
}

func (m *MockCLIAdapter) SetCheckInteractiveResult(result CheckInteractiveResult) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.checkInteractive = result
}

// MockBotAdapter is a mock implementation of BotAdapter for testing
type MockBotAdapter struct {
	startCalled       bool
	stopCalled        bool
	messageHandler    func(message bot.BotMessage)
	sendMessageCalls  []SendMessageCall
	startError        error
	stopError         error
	sendMessageError  error
	mu                sync.Mutex
	receivedMessages  []bot.BotMessage
}

type SendMessageCall struct {
	Channel string
	Message string
}

func (m *MockBotAdapter) Start(messageHandler func(bot.BotMessage)) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.startCalled = true
	m.messageHandler = messageHandler
	if m.startError != nil {
		return m.startError
	}
	return nil
}

func (m *MockBotAdapter) SendMessage(channel, message string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sendMessageCalls = append(m.sendMessageCalls, SendMessageCall{
		Channel: channel,
		Message: message,
	})
	if m.sendMessageError != nil {
		return m.sendMessageError
	}
	return nil
}

func (m *MockBotAdapter) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopCalled = true
	if m.stopError != nil {
		return m.stopError
	}
	return nil
}

// SimulateMessage simulates receiving a message from this bot
func (m *MockBotAdapter) SimulateMessage(platform, userID, channel, content string) {
	m.mu.Lock()
	handler := m.messageHandler
	m.mu.Unlock()

	if handler != nil {
		m.mu.Lock()
		m.receivedMessages = append(m.receivedMessages, bot.BotMessage{
			Platform:  platform,
			UserID:    userID,
			Channel:   channel,
			Content:   content,
			Timestamp: time.Now(),
		})
		m.mu.Unlock()

		handler(bot.BotMessage{
			Platform:  platform,
			UserID:    userID,
			Channel:   channel,
			Content:   content,
			Timestamp: time.Now(),
		})
	}
}

func (m *MockBotAdapter) GetSendMessageCalls() []SendMessageCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	calls := make([]SendMessageCall, len(m.sendMessageCalls))
	copy(calls, m.sendMessageCalls)
	return calls
}

func (m *MockBotAdapter) WasStartCalled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.startCalled
}

func (m *MockBotAdapter) WasStopCalled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.stopCalled
}

func (m *MockBotAdapter) SetStartError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.startError = err
}

func (m *MockBotAdapter) SetStopError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopError = err
}

func (m *MockBotAdapter) SetSendMessageError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sendMessageError = err
}

func (m *MockBotAdapter) GetReceivedMessages() []bot.BotMessage {
	m.mu.Lock()
	defer m.mu.Unlock()
	messages := make([]bot.BotMessage, len(m.receivedMessages))
	copy(messages, m.receivedMessages)
	return messages
}

// Test NewEngine creates a new Engine instance
func TestNewEngine_CreatesEngine(t *testing.T) {
	config := &Config{
		HookServer: HookServerConfig{
			Port: 8080,
		},
		CommandPrefix: "!!",
		Security: SecurityConfig{
			WhitelistEnabled: false,
		},
		Sessions: []SessionConfig{
			{
				Name:      "test-session",
				CLIType:   "claude",
				WorkDir:   "/tmp/test",
				AutoStart: false,
			},
		},
		DefaultSession: "test-session",
		Bots: map[string]BotConfig{
			"discord": {
				Enabled:   true,
				Token:     "test-token",
				ChannelID: "test-channel",
			},
		},
		CLIAdapters: map[string]CLIAdapterConfig{
			"claude": {
				HistoryDir: "/tmp/history",
				Interactive: InteractiveConfig{
					Enabled:    true,
					CheckLines: 50,
					Patterns:   []string{`.*\?`},
				},
			},
		},
	}

	engine := NewEngine(config)

	if engine == nil {
		t.Fatal("Expected engine to be created, got nil")
	}

	if engine.config != config {
		t.Error("Expected config to be set")
	}

	if len(engine.cliAdapters) != 0 {
		t.Errorf("Expected no CLI adapters initially, got %d", len(engine.cliAdapters))
	}

	if len(engine.activeBots) != 0 {
		t.Errorf("Expected no bots initially, got %d", len(engine.activeBots))
	}

	if len(engine.sessions) != 0 {
		t.Errorf("Expected no sessions initially, got %d", len(engine.sessions))
	}

	if engine.messageChan == nil {
		t.Error("Expected messageChan to be initialized")
	}

	if engine.sessionChannels == nil {
		t.Error("Expected sessionChannels to be initialized")
	}
}

// Test RegisterCLIAdapter registers a CLI adapter
func TestRegisterCLIAdapter_RegistersAdapter(t *testing.T) {
	engine := NewEngine(&Config{})
	mockAdapter := &MockCLIAdapter{}

	engine.RegisterCLIAdapter("claude", mockAdapter)

	adapter, exists := engine.cliAdapters["claude"]
	if !exists {
		t.Fatal("Expected CLI adapter to be registered")
	}

	// Check that the adapter is not nil
	if adapter == nil {
		t.Error("Expected adapter to be non-nil")
	}

	// Verify it's our mock by calling a method
	err := adapter.SendInput("test", "input")
	if err != nil {
		t.Error("Expected mock adapter to work without error")
	}
}

// Test RegisterBotAdapter registers a bot adapter
func TestRegisterBotAdapter_RegistersAdapter(t *testing.T) {
	engine := NewEngine(&Config{})
	mockAdapter := &MockBotAdapter{}

	engine.RegisterBotAdapter("discord", mockAdapter)

	adapter, exists := engine.activeBots["discord"]
	if !exists {
		t.Fatal("Expected bot adapter to be registered")
	}

	if adapter != mockAdapter {
		t.Error("Expected registered adapter to be the same instance")
	}
}

// Test initializeSessions initializes sessions from config
func TestInitializeSessions_InitializesSessions(t *testing.T) {
	mockCLI := &MockCLIAdapter{}
	mockCLI.SetIsAlive(false)

	config := &Config{
		Sessions: []SessionConfig{
			{
				Name:      "session1",
				CLIType:   "claude",
				WorkDir:   "/tmp/session1",
				AutoStart: true,
			},
			{
				Name:      "session2",
				CLIType:   "claude",
				WorkDir:   "/tmp/session2",
				AutoStart: false,
			},
		},
	}

	engine := NewEngine(config)
	engine.RegisterCLIAdapter("claude", mockCLI)

	err := engine.initializeSessions()
	if err != nil {
		t.Fatalf("Expected no error initializing sessions, got %v", err)
	}

	engine.sessionMu.RLock()
	defer engine.sessionMu.RUnlock()

	if len(engine.sessions) != 2 {
		t.Errorf("Expected 2 sessions, got %d", len(engine.sessions))
	}

	session1, exists := engine.sessions["session1"]
	if !exists {
		t.Fatal("Expected session1 to exist")
	}

	if session1.Name != "session1" {
		t.Errorf("Expected session name 'session1', got '%s'", session1.Name)
	}

	if session1.CLIType != "claude" {
		t.Errorf("Expected CLI type 'claude', got '%s'", session1.CLIType)
	}

	if session1.State != StateIdle {
		t.Errorf("Expected state '%s', got '%s'", StateIdle, session1.State)
	}
}

// Test initializeSessions skips already existing sessions
func TestInitializeSessions_SkipsExistingSessions(t *testing.T) {
	mockCLI := &MockCLIAdapter{}
	mockCLI.SetIsAlive(true)

	config := &Config{
		Sessions: []SessionConfig{
			{
				Name:      "session1",
				CLIType:   "claude",
				WorkDir:   "/tmp/session1",
				AutoStart: false,
			},
		},
	}

	engine := NewEngine(config)
	engine.RegisterCLIAdapter("claude", mockCLI)

	// Initialize once
	err := engine.initializeSessions()
	if err != nil {
		t.Fatalf("Expected no error on first init, got %v", err)
	}

	// Initialize again - should skip existing session
	err = engine.initializeSessions()
	if err != nil {
		t.Fatalf("Expected no error on second init, got %v", err)
	}

	engine.sessionMu.RLock()
	defer engine.sessionMu.RUnlock()

	if len(engine.sessions) != 1 {
		t.Errorf("Expected 1 session, got %d", len(engine.sessions))
	}
}

// Test IsUserAuthorized checks user authorization
func TestIsUserAuthorized_WithWhitelistEnabled_ChecksWhitelist(t *testing.T) {
	config := &Config{
		Security: SecurityConfig{
			WhitelistEnabled: true,
			AllowedUsers: map[string][]string{
				"discord": {"user1", "user2"},
				"feishu":  {"user3"},
			},
		},
	}

	// Test authorized user
	authorized := config.IsUserAuthorized("discord", "user1")
	if !authorized {
		t.Error("Expected user1 to be authorized on discord")
	}

	// Test unauthorized user
	authorized = config.IsUserAuthorized("discord", "user3")
	if authorized {
		t.Error("Expected user3 to be unauthorized on discord")
	}

	// Test non-existent platform
	authorized = config.IsUserAuthorized("telegram", "user1")
	if authorized {
		t.Error("Expected user to be unauthorized on non-existent platform")
	}
}

// Test IsUserAuthorized with whitelist disabled
func TestIsUserAuthorized_WithWhitelistDisabled_AllowsAll(t *testing.T) {
	config := &Config{
		Security: SecurityConfig{
			WhitelistEnabled: false,
			AllowedUsers: map[string][]string{
				"discord": {"user1"},
			},
		},
	}

	// Any user should be authorized when whitelist is disabled
	authorized := config.IsUserAuthorized("discord", "any-user")
	if !authorized {
		t.Error("Expected any user to be authorized when whitelist is disabled")
	}

	authorized = config.IsUserAuthorized("any-platform", "any-user")
	if !authorized {
		t.Error("Expected any user on any platform to be authorized when whitelist is disabled")
	}
}

// Test GetActiveSession returns the default session
func TestGetActiveSession_ReturnsDefaultSession(t *testing.T) {
	config := &Config{
		DefaultSession: "session1",
		Sessions: []SessionConfig{
			{
				Name:    "session1",
				CLIType: "claude",
				WorkDir: "/tmp/session1",
			},
		},
	}

	engine := NewEngine(config)
	engine.sessions["session1"] = &Session{
		Name:    "session1",
		CLIType: "claude",
		WorkDir: "/tmp/session1",
		State:   StateIdle,
	}

	session := engine.GetActiveSession("any-channel")

	if session == nil {
		t.Fatal("Expected session to be returned, got nil")
	}

	if session.Name != "session1" {
		t.Errorf("Expected session name 'session1', got '%s'", session.Name)
	}
}

// Test GetActiveSession returns first available when default doesn't exist
func TestGetActiveSession_WithNoDefault_ReturnsFirstAvailable(t *testing.T) {
	config := &Config{
		Sessions: []SessionConfig{
			{
				Name:    "session1",
				CLIType: "claude",
				WorkDir: "/tmp/session1",
			},
		},
	}

	engine := NewEngine(config)
	engine.sessions["session1"] = &Session{
		Name:    "session1",
		CLIType: "claude",
		WorkDir: "/tmp/session1",
		State:   StateIdle,
	}

	session := engine.GetActiveSession("any-channel")

	if session == nil {
		t.Fatal("Expected session to be returned, got nil")
	}

	if session.Name != "session1" {
		t.Errorf("Expected session name 'session1', got '%s'", session.Name)
	}
}

// Test GetActiveSession returns nil when no sessions exist
func TestGetActiveSession_WithNoSessions_ReturnsNil(t *testing.T) {
	config := &Config{
		Sessions: []SessionConfig{},
	}

	engine := NewEngine(config)

	session := engine.GetActiveSession("any-channel")

	if session != nil {
		t.Error("Expected nil when no sessions exist")
	}
}

// Test updateSessionState updates session state
func TestUpdateSessionState_UpdatesState(t *testing.T) {
	engine := NewEngine(&Config{})
	engine.sessions["test-session"] = &Session{
		Name:  "test-session",
		State: StateIdle,
	}

	engine.updateSessionState("test-session", StateProcessing)

	if engine.sessions["test-session"].State != StateProcessing {
		t.Errorf("Expected state '%s', got '%s'", StateProcessing, engine.sessions["test-session"].State)
	}
}

// Test SendToBot sends message to specific bot
func TestSendToBot_SendsMessage(t *testing.T) {
	mockBot := &MockBotAdapter{}

	engine := NewEngine(&Config{})
	engine.RegisterBotAdapter("discord", mockBot)

	engine.SendToBot("discord", "test-channel", "Hello, bot!")

	calls := mockBot.GetSendMessageCalls()
	if len(calls) != 1 {
		t.Fatalf("Expected 1 message sent, got %d", len(calls))
	}

	if calls[0].Channel != "test-channel" {
		t.Errorf("Expected channel 'test-channel', got '%s'", calls[0].Channel)
	}

	if calls[0].Message != "Hello, bot!" {
		t.Errorf("Expected message 'Hello, bot!', got '%s'", calls[0].Message)
	}
}

// Test SendToAllBots sends message to all bots
func TestSendToAllBots_SendsToAllBots(t *testing.T) {
	mockBot1 := &MockBotAdapter{}
	mockBot2 := &MockBotAdapter{}

	engine := NewEngine(&Config{})
	engine.RegisterBotAdapter("discord", mockBot1)
	engine.RegisterBotAdapter("telegram", mockBot2)

	engine.SendToAllBots("Broadcast message")

	calls1 := mockBot1.GetSendMessageCalls()
	calls2 := mockBot2.GetSendMessageCalls()

	if len(calls1) != 1 {
		t.Errorf("Expected discord bot to receive 1 message, got %d", len(calls1))
	}

	if len(calls2) != 1 {
		t.Errorf("Expected telegram bot to receive 1 message, got %d", len(calls2))
	}

	if len(calls1) > 0 && calls1[0].Message != "Broadcast message" {
		t.Errorf("Expected message 'Broadcast message', got '%s'", calls1[0].Message)
	}
}

// Test HandleSpecialCommand handles sessions command
func TestHandleSpecialCommand_SessionsCommand(t *testing.T) {
	mockBot := &MockBotAdapter{}

	engine := NewEngine(&Config{})
	engine.RegisterBotAdapter("discord", mockBot)
	engine.sessions["session1"] = &Session{
		Name:    "session1",
		CLIType: "claude",
		State:   StateIdle,
	}

	msg := bot.BotMessage{
		Platform: "discord",
		Channel:  "test-channel",
		UserID:   "user1",
		Content:  "!!sessions",
	}

	engine.HandleSpecialCommand("sessions", msg)

	calls := mockBot.GetSendMessageCalls()
	if len(calls) != 1 {
		t.Fatalf("Expected 1 message sent, got %d", len(calls))
	}

	// Check that response contains session info
	response := calls[0].Message
	if !contains(response, "session1") {
		t.Errorf("Expected response to contain 'session1', got: %s", response)
	}

	if !contains(response, "Available Sessions") {
		t.Errorf("Expected response to contain 'Available Sessions', got: %s", response)
	}
}

// Test HandleSpecialCommand handles status command
func TestHandleSpecialCommand_StatusCommand(t *testing.T) {
	mockCLI := &MockCLIAdapter{}
	mockCLI.SetIsAlive(true)
	mockBot := &MockBotAdapter{}

	engine := NewEngine(&Config{})
	engine.RegisterCLIAdapter("claude", mockCLI)
	engine.RegisterBotAdapter("discord", mockBot)
	engine.sessions["session1"] = &Session{
		Name:    "session1",
		CLIType: "claude",
		State:   StateProcessing,
	}

	msg := bot.BotMessage{
		Platform: "discord",
		Channel:  "test-channel",
		UserID:   "user1",
		Content:  "!!status",
	}

	engine.HandleSpecialCommand("status", msg)

	calls := mockBot.GetSendMessageCalls()
	if len(calls) != 1 {
		t.Fatalf("Expected 1 message sent, got %d", len(calls))
	}

	// Check that response contains status info
	response := calls[0].Message
	if !contains(response, "clibot Status") {
		t.Errorf("Expected response to contain 'clibot Status', got: %s", response)
	}

	if !contains(response, "session1") {
		t.Errorf("Expected response to contain 'session1', got: %s", response)
	}
}

// Test HandleSpecialCommand handles whoami command
func TestHandleSpecialCommand_WhoamiCommand(t *testing.T) {
	mockBot := &MockBotAdapter{}

	engine := NewEngine(&Config{
		DefaultSession: "session1",
	})
	engine.RegisterBotAdapter("discord", mockBot)
	engine.sessions["session1"] = &Session{
		Name:    "session1",
		CLIType: "claude",
		State:   StateIdle,
		WorkDir: "/tmp/test",
	}

	msg := bot.BotMessage{
		Platform: "discord",
		Channel:  "test-channel",
		UserID:   "user1",
		Content:  "!!whoami",
	}

	engine.HandleSpecialCommand("whoami", msg)

	calls := mockBot.GetSendMessageCalls()
	if len(calls) != 1 {
		t.Fatalf("Expected 1 message sent, got %d", len(calls))
	}

	// Check that response contains session info
	response := calls[0].Message
	if !contains(response, "session1") {
		t.Errorf("Expected response to contain 'session1', got: %s", response)
	}

	if !contains(response, "claude") {
		t.Errorf("Expected response to contain 'claude', got: %s", response)
	}
}

// Test HandleSpecialCommand handles help command
func TestHandleSpecialCommand_HelpCommand(t *testing.T) {
	mockBot := &MockBotAdapter{}

	engine := NewEngine(&Config{
		CommandPrefix: "!!",
	})
	engine.RegisterBotAdapter("discord", mockBot)

	msg := bot.BotMessage{
		Platform: "discord",
		Channel:  "test-channel",
		UserID:   "user1",
		Content:  "!!help",
	}

	engine.HandleSpecialCommand("help", msg)

	calls := mockBot.GetSendMessageCalls()
	if len(calls) != 1 {
		t.Fatalf("Expected 1 message sent, got %d", len(calls))
	}

	// Check that response contains help information
	response := calls[0].Message
	if !contains(response, "clibot Help") {
		t.Errorf("Expected response to contain 'clibot Help', got: %s", response)
	}
	if !contains(response, "Special Commands") {
		t.Errorf("Expected response to contain 'Special Commands', got: %s", response)
	}
	if !contains(response, "Special Keywords") {
		t.Errorf("Expected response to contain 'Special Keywords', got: %s", response)
	}
	if !contains(response, "tab") {
		t.Errorf("Expected response to contain 'tab', got: %s", response)
	}
}

// Test HandleSpecialCommand handles unknown command
func TestHandleSpecialCommand_UnknownCommand(t *testing.T) {
	mockBot := &MockBotAdapter{}

	engine := NewEngine(&Config{
		CommandPrefix: "!!",
	})
	engine.RegisterBotAdapter("discord", mockBot)

	msg := bot.BotMessage{
		Platform: "discord",
		Channel:  "test-channel",
		UserID:   "user1",
		Content:  "!!unknown",
	}

	engine.HandleSpecialCommand("unknown", msg)

	calls := mockBot.GetSendMessageCalls()
	if len(calls) != 1 {
		t.Fatalf("Expected 1 message sent, got %d", len(calls))
	}

	// Check that response contains error message and help suggestion
	response := calls[0].Message
	if !contains(response, "Unknown command") {
		t.Errorf("Expected response to contain 'Unknown command', got: %s", response)
	}
	if !contains(response, "help") {
		t.Errorf("Expected response to suggest using 'help', got: %s", response)
	}
}

// Test Stop stops all bots
func TestStop_StopsAllBots(t *testing.T) {
	mockBot1 := &MockBotAdapter{}
	mockBot2 := &MockBotAdapter{}

	engine := NewEngine(&Config{})
	engine.RegisterBotAdapter("discord", mockBot1)
	engine.RegisterBotAdapter("telegram", mockBot2)

	err := engine.Stop()
	if err != nil {
		t.Fatalf("Expected no error stopping engine, got %v", err)
	}

	if !mockBot1.WasStopCalled() {
		t.Error("Expected discord bot to be stopped")
	}

	if !mockBot2.WasStopCalled() {
		t.Error("Expected telegram bot to be stopped")
	}
}

// Test Stop handles bot stop errors gracefully
func TestStop_WithBotErrors_LogsError(t *testing.T) {
	mockBot := &MockBotAdapter{}
	mockBot.SetStopError(errors.New("stop failed"))

	engine := NewEngine(&Config{})
	engine.RegisterBotAdapter("discord", mockBot)

	// Should not return error even if bot fails to stop
	err := engine.Stop()
	if err != nil {
		t.Fatalf("Expected no error stopping engine, got %v", err)
	}

	if !mockBot.WasStopCalled() {
		t.Error("Expected bot stop to be attempted")
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && containsHelper(s, substr)))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
