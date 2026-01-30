package bot

import "time"

// BotAdapter defines the interface for bot adapters
type BotAdapter interface {
	// Start starts the bot, establishes connection and begins listening for messages
	Start(messageHandler func(BotMessage)) error

	// SendMessage sends a message to the IM platform
	// Adapter is responsible for:
	//   - Truncating to platform limits
	//   - Splitting long messages when necessary
	//   - Platform-specific formatting
	SendMessage(channel, message string) error

	// Stop stops the bot and cleans up resources
	Stop() error
}

// BotMessage represents a bot message structure
type BotMessage struct {
	Platform  string    // feishu/discord/telegram
	UserID    string    // Unique user identifier (for permission control)
	Channel   string    // Channel/session ID
	Content   string    // Message content
	Timestamp time.Time
}
