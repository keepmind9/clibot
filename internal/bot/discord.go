package bot

import (
	"fmt"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/keepmind9/clibot/internal/logger"
	"github.com/keepmind9/clibot/pkg/constants"
	"github.com/sirupsen/logrus"
)

// DiscordMessage represents a Discord message for our interface
type DiscordMessage interface {
	ID() string
}

// DiscordSessionInterface defines the interface we need from discordgo.Session
// This allows us to mock it in tests without depending on concrete types
type DiscordSessionInterface interface {
	AddHandler(handler interface{}) func()
	Open() error
	Close() error
	ChannelMessageSend(channelID string, content string, options ...discordgo.RequestOption) (*discordgo.Message, error)
}

// DiscordBot implements BotAdapter interface for Discord
type DiscordBot struct {
	mu             sync.RWMutex
	token          string
	channelID      string
	session        DiscordSessionInterface
	messageHandler func(BotMessage)
}

// NewDiscordBot creates a new Discord bot instance
func NewDiscordBot(token, channelID string) *DiscordBot {
	return &DiscordBot{
		token:     token,
		channelID: channelID,
		session:   nil, // Will be created in Start()
	}
}

// Start establishes connection to Discord and begins listening for messages
func (d *DiscordBot) Start(messageHandler func(BotMessage)) error {
	d.SetMessageHandler(messageHandler)

	// Log bot startup
	logger.WithFields(logrus.Fields{
		"token":   maskSecret(d.token),
		"channel": d.channelID,
	}).Info("starting-discord-bot")

	// Create Discord session
	session, err := discordgo.New("Bot " + d.token)
	if err != nil {
		return fmt.Errorf("failed to create discord session: %w", err)
	}

	d.session = session

	// Register message handler
	session.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
		// Ignore messages from bots
		if m.Author.Bot {
			return
		}

		// Log received message
		logger.WithFields(logrus.Fields{
			"platform":  "discord",
			"user_id":   m.Author.ID,
			"username":  m.Author.Username,
			"channel":   m.ChannelID,
			"content":   m.Content,
		}).Debug("received-discord-message")

		// Call the handler with BotMessage
		handler := d.GetMessageHandler()
		if handler != nil {
			handler(BotMessage{
				Platform:  "discord",
				UserID:    m.Author.ID,
				Channel:   m.ChannelID,
				Content:   m.Content,
				Timestamp: time.Now(),
			})

			logger.WithFields(logrus.Fields{
				"platform": "discord",
				"user":     m.Author.ID,
				"channel":  m.ChannelID,
			}).Info("message-received-from-discord")
		}
	})

	// Open connection
	if err := session.Open(); err != nil {
		return fmt.Errorf("failed to open discord connection: %w", err)
	}

	return nil
}

// SendMessage sends a message to a Discord channel
func (d *DiscordBot) SendMessage(channel, message string) error {
	d.mu.RLock()
	session := d.session
	channelID := d.channelID
	d.mu.RUnlock()

	if session == nil {
		return fmt.Errorf("discord session not initialized")
	}

	// Use configured channel if not specified
	targetChannel := channel
	if targetChannel == "" {
		targetChannel = channelID
	}

	// Discord limit: message length
	const maxDiscordLength = constants.MaxDiscordMessageLength
	if len(message) > maxDiscordLength {
		logger.WithFields(logrus.Fields{
			"original_length": len(message),
			"max_length":      maxDiscordLength,
		}).Info("truncating-message-for-discord-limit")
		// Keep the last (max-3) characters to show the newest content
		message = "..." + message[len(message)-maxDiscordLength+3:]
	}

	_, err := session.ChannelMessageSend(targetChannel, message)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"channel": targetChannel,
			"error":   err,
		}).Error("failed-to-send-message-to-discord")
		return fmt.Errorf("failed to send message to channel %s: %w", targetChannel, err)
	}

	logger.WithField("channel", targetChannel).Info("message-sent-to-discord")
	return nil
}

// Stop closes the Discord connection and cleans up resources
func (d *DiscordBot) Stop() error {
	d.mu.Lock()
	session := d.session
	d.session = nil
	d.mu.Unlock()

	if session == nil {
		return nil
	}

	if err := session.Close(); err != nil {
		return fmt.Errorf("failed to close discord session: %w", err)
	}

	return nil
}

// SetMessageHandler sets the message handler in a thread-safe manner
func (d *DiscordBot) SetMessageHandler(handler func(BotMessage)) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.messageHandler = handler
}

// GetMessageHandler gets the message handler in a thread-safe manner
func (d *DiscordBot) GetMessageHandler() func(BotMessage) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.messageHandler
}
