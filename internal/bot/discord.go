package bot

import (
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/keepmind9/clibot/internal/logger"
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
	Token          string
	ChannelID      string
	Session        DiscordSessionInterface
	messageHandler func(BotMessage)
}

// NewDiscordBot creates a new Discord bot instance
func NewDiscordBot(token, channelID string) *DiscordBot {
	return &DiscordBot{
		Token:     token,
		ChannelID: channelID,
		Session:   nil, // Will be created in Start()
	}
}

// Start establishes connection to Discord and begins listening for messages
func (d *DiscordBot) Start(messageHandler func(BotMessage)) error {
	d.messageHandler = messageHandler

	// Log bot startup
	logger.WithFields(logrus.Fields{
		"token":   maskToken(d.Token),
		"channel": d.ChannelID,
	}).Info("Starting Discord bot")

	// Create Discord session
	session, err := discordgo.New("Bot " + d.Token)
	if err != nil {
		return fmt.Errorf("failed to create discord session: %w", err)
	}

	d.Session = session

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
		}).Debug("Received Discord message")

		// Call the handler with BotMessage
		if d.messageHandler != nil {
			d.messageHandler(BotMessage{
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
			}).Info("Message received from Discord")
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
	if d.Session == nil {
		return fmt.Errorf("discord session not initialized")
	}

	// Use configured channel if not specified
	targetChannel := channel
	if targetChannel == "" {
		targetChannel = d.ChannelID
	}

	// Discord limit: 4000 characters
	const maxDiscordLength = 4000
	if len(message) > maxDiscordLength {
		logger.WithFields(logrus.Fields{
			"original_length": len(message),
			"max_length":      maxDiscordLength,
		}).Info("Truncating message for Discord limit (keeping latest content)")
		// Keep the last (max-3) characters to show the newest content
		message = "..." + message[len(message)-maxDiscordLength+3:]
	}

	_, err := d.Session.ChannelMessageSend(targetChannel, message)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"channel": targetChannel,
			"error":   err,
		}).Error("Failed to send message to Discord")
		return fmt.Errorf("failed to send message to channel %s: %w", targetChannel, err)
	}

	logger.WithField("channel", targetChannel).Info("Message sent to Discord")
	return nil
}

// Stop closes the Discord connection and cleans up resources
func (d *DiscordBot) Stop() error {
	if d.Session == nil {
		return nil
	}

	if err := d.Session.Close(); err != nil {
		return fmt.Errorf("failed to close discord session: %w", err)
	}

	return nil
}

// maskToken masks sensitive token information for logging
func maskToken(token string) string {
	if len(token) <= 10 {
		return "***"
	}
	return token[:7] + "***" + token[len(token)-4:]
}
