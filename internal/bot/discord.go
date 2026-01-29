package bot

import (
	"fmt"
	"time"
)

// NOTE: This implementation uses a mock DiscordSession interface for testing.
// In production, you would integrate the actual discordgo library:
//
// import (
//     "github.com/bwmarrin/discordgo"
// )
//
// The DiscordSession interface mimics the discordgo.Session methods we use.
// To use the real Discord bot, replace the mock session with discordgo.Session:
//
// session, err := discordgo.New("Bot " + d.token)
// if err != nil {
//     return fmt.Errorf("failed to create discord session: %w", err)
// }
//
// session.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
//     // Handle message
// })
//
// err = session.Open()
// return err

// DiscordConfig contains configuration for Discord bot
type DiscordConfig struct {
	Token     string
	ChannelID string
}

// DiscordSession interface represents Discord session methods we use
// This allows us to mock the session in tests
type DiscordSession interface {
	AddHandler(handler interface{})
	Open() error
	Close() error
	ChannelMessageSend(channel, message string) (string, error)
}

// DiscordBot implements BotAdapter interface for Discord
type DiscordBot struct {
	token           string
	channelID       string
	session         DiscordSession
	messageHandler  func(BotMessage)
}

// DiscordMessageAuthor represents the author of a Discord message
type DiscordMessageAuthor struct {
	Bot    bool
	UserID string
}

// DiscordMessage represents a Discord message
type DiscordMessage struct {
	Content   string
	ChannelID string
	Author    DiscordMessageAuthor
}

// NewDiscordBot creates a new Discord bot instance
// Returns nil if token is empty
func NewDiscordBot(config DiscordConfig) *DiscordBot {
	if config.Token == "" {
		return nil
	}

	return &DiscordBot{
		token:     config.Token,
		channelID: config.ChannelID,
		session:   nil, // Will be created in Start()
	}
}

// Start establishes connection to Discord and begins listening for messages
func (d *DiscordBot) Start(messageHandler func(BotMessage)) error {
	// Store the message handler
	d.messageHandler = messageHandler

	// Create Discord session
	// In production, this would be: discordgo.New("Bot " + d.token)
	// For now, we expect session to be injected via SetSession for testing
	if d.session == nil {
		return fmt.Errorf("discord session not initialized")
	}

	// Register message handler
	d.session.AddHandler(func(s *DiscordBot, m DiscordMessage) {
		d.handleMessage(m)
	})

	// Open connection
	if err := d.session.Open(); err != nil {
		return fmt.Errorf("failed to open discord connection: %w", err)
	}

	return nil
}

// handleMessage processes an incoming Discord message
func (d *DiscordBot) handleMessage(msg DiscordMessage) {
	// Ignore messages from bots
	if msg.Author.Bot {
		return
	}

	// Call the handler with BotMessage
	if d.messageHandler != nil {
		d.messageHandler(BotMessage{
			Platform:  "discord",
			UserID:    msg.Author.UserID,
			Channel:   msg.ChannelID,
			Content:   msg.Content,
			Timestamp: time.Now(),
		})
	}
}

// SendMessage sends a message to a Discord channel
func (d *DiscordBot) SendMessage(channel, message string) error {
	if d.session == nil {
		return fmt.Errorf("discord session not initialized")
	}

	_, err := d.session.ChannelMessageSend(channel, message)
	if err != nil {
		return fmt.Errorf("failed to send message to channel %s: %w", channel, err)
	}

	return nil
}

// Stop closes the Discord connection and cleans up resources
func (d *DiscordBot) Stop() error {
	if d.session == nil {
		return nil
	}

	if err := d.session.Close(); err != nil {
		return fmt.Errorf("failed to close discord session: %w", err)
	}

	return nil
}

// SetSession allows injecting a mock session for testing
func (d *DiscordBot) SetSession(session DiscordSession) {
	d.session = session
}
