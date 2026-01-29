package bot

import (
	"errors"
	"testing"
)

// MockDiscordSession is a mock implementation of DiscordSession for testing
type MockDiscordSession struct {
	shouldFailOnOpen bool
	shouldFailOnSend bool
	openCalled       bool
	closed           bool
	sentMessages     []SentMessage
	onAddHandler     func(handler interface{})
}

type SentMessage struct {
	Channel string
	Message string
}

func (m *MockDiscordSession) AddHandler(handler interface{}) {
	if m.onAddHandler != nil {
		m.onAddHandler(handler)
	}
}

func (m *MockDiscordSession) Open() error {
	m.openCalled = true
	if m.shouldFailOnOpen {
		return errors.New("failed to open discord connection")
	}
	return nil
}

func (m *MockDiscordSession) Close() error {
	m.closed = true
	return nil
}

func (m *MockDiscordSession) ChannelMessageSend(channel, message string) (string, error) {
	if m.shouldFailOnSend {
		return "", errors.New("failed to send message")
	}
	m.sentMessages = append(m.sentMessages, SentMessage{
		Channel: channel,
		Message: message,
	})
	return "msg-id", nil
}

// MockMessageAuthor represents a message author for testing
type MockMessageAuthor struct {
	Bot    bool
	UserID string
}

// ToDiscordMessageAuthor converts MockMessageAuthor to DiscordMessageAuthor
func (m MockMessageAuthor) ToDiscordMessageAuthor() DiscordMessageAuthor {
	return DiscordMessageAuthor{
		Bot:    m.Bot,
		UserID: m.UserID,
	}
}

func TestNewDiscordBot_WithValidToken_CreatesBot(t *testing.T) {
	bot := NewDiscordBot(DiscordConfig{
		Token:     "test-token",
		ChannelID: "123456789",
	})

	if bot == nil {
		t.Fatal("Expected bot to be created, got nil")
	}

	if bot.token != "test-token" {
		t.Errorf("Expected token 'test-token', got '%s'", bot.token)
	}

	if bot.channelID != "123456789" {
		t.Errorf("Expected channelID '123456789', got '%s'", bot.channelID)
	}
}

func TestNewDiscordBot_WithEmptyToken_ReturnsNil(t *testing.T) {
	bot := NewDiscordBot(DiscordConfig{
		Token:     "",
		ChannelID: "123456789",
	})

	if bot != nil {
		t.Error("Expected nil bot with empty token, got non-nil")
	}
}

func TestDiscordBot_Start_WithValidSession_ConnectsSuccessfully(t *testing.T) {
	mockSession := &MockDiscordSession{
		shouldFailOnOpen: false,
	}

	bot := &DiscordBot{
		token:     "test-token",
		channelID: "123456789",
		session:   mockSession,
	}

	handlerCalled := false
	var receivedMsg BotMessage
	messageHandler := func(msg BotMessage) {
		handlerCalled = true
		receivedMsg = msg
	}

	// Track the handler registration
	var registeredHandler interface{}
	mockSession.onAddHandler = func(handler interface{}) {
		registeredHandler = handler
		bot.messageHandler = messageHandler
	}

	err := bot.Start(messageHandler)
	if err != nil {
		t.Fatalf("Expected no error on start, got %v", err)
	}

	if !mockSession.openCalled {
		t.Error("Expected session.Open() to be called")
	}

	if registeredHandler == nil {
		t.Error("Expected message handler to be registered")
	}

	// Simulate a message event
	if registeredHandler != nil {
		// Simulate message through the bot's handleMessage
		bot.handleMessage(DiscordMessage{
			Content:   "Hello, bot!",
			ChannelID: "123456789",
			Author: DiscordMessageAuthor{
				Bot:    false,
				UserID: "user-123",
			},
		})
	}

	if !handlerCalled {
		t.Error("Expected message handler to be called")
	}

	if receivedMsg.Content != "Hello, bot!" {
		t.Errorf("Expected message content 'Hello, bot!', got '%s'", receivedMsg.Content)
	}

	if receivedMsg.Platform != "discord" {
		t.Errorf("Expected platform 'discord', got '%s'", receivedMsg.Platform)
	}

	if receivedMsg.Channel != "123456789" {
		t.Errorf("Expected channel '123456789', got '%s'", receivedMsg.Channel)
	}

	if receivedMsg.UserID != "user-123" {
		t.Errorf("Expected UserID 'user-123', got '%s'", receivedMsg.UserID)
	}

	// Cleanup
	bot.Stop()
}

func TestDiscordBot_Start_WithSessionOpenError_ReturnsError(t *testing.T) {
	mockSession := &MockDiscordSession{
		shouldFailOnOpen: true,
	}

	bot := &DiscordBot{
		token:     "test-token",
		channelID: "123456789",
		session:   mockSession,
	}

	err := bot.Start(func(msg BotMessage) {})
	if err == nil {
		t.Error("Expected error on session open failure, got nil")
	}

	if !mockSession.openCalled {
		t.Error("Expected session.Open() to be called even on failure")
	}
}

func TestDiscordBot_HandleMessage_IgnoresBotMessages(t *testing.T) {
	bot := &DiscordBot{
		token:     "test-token",
		channelID: "123456789",
	}

	handlerCalled := false
	messageHandler := func(msg BotMessage) {
		handlerCalled = true
	}
	bot.messageHandler = messageHandler

	// Simulate a bot message
	bot.handleMessage(DiscordMessage{
		Content:   "Bot message",
		ChannelID: "123456789",
		Author: DiscordMessageAuthor{
			Bot:    true, // This is a bot message
			UserID: "bot-123",
		},
	})

	if handlerCalled {
		t.Error("Expected bot messages to be ignored, but handler was called")
	}
}

func TestDiscordBot_SendMessage_WithValidChannel_SendsSuccessfully(t *testing.T) {
	mockSession := &MockDiscordSession{
		shouldFailOnSend: false,
	}

	bot := &DiscordBot{
		token:     "test-token",
		channelID: "123456789",
		session:   mockSession,
	}

	err := bot.SendMessage("test-channel", "Hello, world!")
	if err != nil {
		t.Fatalf("Expected no error sending message, got %v", err)
	}

	if len(mockSession.sentMessages) != 1 {
		t.Fatalf("Expected 1 message sent, got %d", len(mockSession.sentMessages))
	}

	if mockSession.sentMessages[0].Channel != "test-channel" {
		t.Errorf("Expected channel 'test-channel', got '%s'", mockSession.sentMessages[0].Channel)
	}

	if mockSession.sentMessages[0].Message != "Hello, world!" {
		t.Errorf("Expected message 'Hello, world!', got '%s'", mockSession.sentMessages[0].Message)
	}
}

func TestDiscordBot_SendMessage_WithSendError_ReturnsError(t *testing.T) {
	mockSession := &MockDiscordSession{
		shouldFailOnSend: true,
	}

	bot := &DiscordBot{
		token:     "test-token",
		channelID: "123456789",
		session:   mockSession,
	}

	err := bot.SendMessage("test-channel", "Hello, world!")
	if err == nil {
		t.Error("Expected error on send failure, got nil")
	}
}

func TestDiscordBot_Stop_WithActiveSession_ClosesSuccessfully(t *testing.T) {
	mockSession := &MockDiscordSession{}

	bot := &DiscordBot{
		token:     "test-token",
		channelID: "123456789",
		session:   mockSession,
	}

	err := bot.Stop()
	if err != nil {
		t.Fatalf("Expected no error on stop, got %v", err)
	}

	if !mockSession.closed {
		t.Error("Expected session.Close() to be called")
	}
}

func TestDiscordBot_Stop_WithNilSession_NoError(t *testing.T) {
	bot := &DiscordBot{
		token:     "test-token",
		channelID: "123456789",
		session:   nil,
	}

	err := bot.Stop()
	if err != nil {
		t.Fatalf("Expected no error on stop with nil session, got %v", err)
	}
}
