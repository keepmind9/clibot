package bot

import (
	"errors"
	"testing"

	"github.com/bwmarrin/discordgo"
)

// MockDiscordSession is a mock implementation of DiscordSessionInterface for testing
type MockDiscordSession struct {
	shouldFailOnOpen bool
	shouldFailOnSend bool
	openCalled       bool
	closed           bool
	sentMessages     []SentMessage
	handler          interface{}
}

type SentMessage struct {
	Channel string
	Message string
}

func (m *MockDiscordSession) AddHandler(handler interface{}) func() {
	m.handler = handler
	return func() {} // Return a remove handler function
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

func (m *MockDiscordSession) ChannelMessageSend(channel, message string, options ...discordgo.RequestOption) (*discordgo.Message, error) {
	if m.shouldFailOnSend {
		return nil, errors.New("failed to send message")
	}
	m.sentMessages = append(m.sentMessages, SentMessage{
		Channel: channel,
		Message: message,
	})
	return &discordgo.Message{ID: "msg-id"}, nil
}

// Helper to simulate receiving a message through the mock session
func (m *MockDiscordSession) SimulateMessage(s *discordgo.Session, msg *discordgo.MessageCreate) {
	if m.handler == nil {
		return
	}
	handlerFunc, ok := m.handler.(func(*discordgo.Session, *discordgo.MessageCreate))
	if !ok {
		return
	}
	handlerFunc(s, msg)
}

func TestNewDiscordBot_WithValidToken_CreatesBot(t *testing.T) {
	bot := NewDiscordBot("test-token", "123456789")

	if bot == nil {
		t.Fatal("Expected bot to be created, got nil")
	}

	if bot.Token != "test-token" {
		t.Errorf("Expected token 'test-token', got '%s'", bot.Token)
	}

	if bot.ChannelID != "123456789" {
		t.Errorf("Expected channelID '123456789', got '%s'", bot.ChannelID)
	}

	if bot.Session != nil {
		t.Error("Expected session to be nil initially")
	}
}

func TestNewDiscordBot_WithEmptyToken_CreatesBot(t *testing.T) {
	bot := NewDiscordBot("", "123456789")

	if bot == nil {
		t.Fatal("Expected bot to be created, got nil")
	}

	if bot.Token != "" {
		t.Errorf("Expected empty token, got '%s'", bot.Token)
	}
}

func TestDiscordBot_Start_WithValidSession_ConnectsSuccessfully(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test that requires Discord session initialization")
	}
	mockSession := &MockDiscordSession{
		shouldFailOnOpen: false,
	}

	bot := NewDiscordBot("test-token", "123456789")
	bot.Session = mockSession

	handlerCalled := false
	var receivedMsg BotMessage
	messageHandler := func(msg BotMessage) {
		handlerCalled = true
		receivedMsg = msg
	}

	err := bot.Start(messageHandler)
	if err != nil {
		t.Fatalf("Expected no error on start, got %v", err)
	}

	if !mockSession.openCalled {
		t.Error("Expected session.Open() to be called")
	}

	if mockSession.handler == nil {
		t.Error("Expected message handler to be registered")
	}

	// Create a discordgo session for simulation
	dgSession := &discordgo.Session{}

	// Simulate a message event
	mockSession.SimulateMessage(dgSession, &discordgo.MessageCreate{
		Message: &discordgo.Message{
			Content:   "Hello, bot!",
			ChannelID: "123456789",
			Author: &discordgo.User{
				ID:            "user-123",
				Bot:           false,
				Username:      "testuser",
				Discriminator: "1234",
			},
		},
	})

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
	if testing.Short() {
		t.Skip("skipping test that requires Discord session initialization")
	}
	mockSession := &MockDiscordSession{
		shouldFailOnOpen: true,
	}

	bot := NewDiscordBot("test-token", "123456789")
	bot.Session = mockSession

	err := bot.Start(func(msg BotMessage) {})
	if err == nil {
		t.Error("Expected error on session open failure, got nil")
	}

	if !mockSession.openCalled {
		t.Error("Expected session.Open() to be called even on failure")
	}
}

func TestDiscordBot_Start_IgnoresBotMessages(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test that requires Discord session initialization")
	}
	mockSession := &MockDiscordSession{
		shouldFailOnOpen: false,
	}

	bot := NewDiscordBot("test-token", "123456789")
	bot.Session = mockSession

	handlerCalled := false
	messageHandler := func(msg BotMessage) {
		handlerCalled = true
	}

	err := bot.Start(messageHandler)
	if err != nil {
		t.Fatalf("Expected no error on start, got %v", err)
	}

	// Create a discordgo session for simulation
	dgSession := &discordgo.Session{}

	// Simulate a bot message
	mockSession.SimulateMessage(dgSession, &discordgo.MessageCreate{
		Message: &discordgo.Message{
			Content:   "Bot message",
			ChannelID: "123456789",
			Author: &discordgo.User{
				ID:            "bot-123",
				Bot:           true, // This is a bot message
				Username:      "testbot",
				Discriminator: "5678",
			},
		},
	})

	if handlerCalled {
		t.Error("Expected bot messages to be ignored, but handler was called")
	}

	// Cleanup
	bot.Stop()
}

func TestDiscordBot_SendMessage_WithValidChannel_SendsSuccessfully(t *testing.T) {
	mockSession := &MockDiscordSession{
		shouldFailOnSend: false,
	}

	bot := NewDiscordBot("test-token", "123456789")
	bot.Session = mockSession

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

	bot := NewDiscordBot("test-token", "123456789")
	bot.Session = mockSession

	err := bot.SendMessage("test-channel", "Hello, world!")
	if err == nil {
		t.Error("Expected error on send failure, got nil")
	}
}

func TestDiscordBot_Stop_WithActiveSession_ClosesSuccessfully(t *testing.T) {
	mockSession := &MockDiscordSession{}

	bot := NewDiscordBot("test-token", "123456789")
	bot.Session = mockSession

	err := bot.Stop()
	if err != nil {
		t.Fatalf("Expected no error on stop, got %v", err)
	}

	if !mockSession.closed {
		t.Error("Expected session.Close() to be called")
	}
}

func TestDiscordBot_Stop_WithNilSession_NoError(t *testing.T) {
	bot := NewDiscordBot("test-token", "123456789")
	bot.Session = nil

	err := bot.Stop()
	if err != nil {
		t.Fatalf("Expected no error on stop with nil session, got %v", err)
	}
}
