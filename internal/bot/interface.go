// Package bot provides bot adapters for various IM platforms.
//
// This package implements a unified interface for connecting to multiple chat platforms,
// including Discord, Telegram, Feishu (Lark), and DingTalk. Each adapter handles
// platform-specific connection logic, message formatting, and communication patterns.
//
// # Supported Platforms
//
//   - Discord: WebSocket connection with real-time message handling
//   - Telegram: Long polling for message updates
//   - Feishu/Lark: WebSocket long connection for enterprise messaging
//   - DingTalk: WebSocket long connection for enterprise messaging
//
// # Usage
//
// To use a bot adapter:
//
//   1. Create a bot instance using the New* function for your platform
//   2. Call Start() with a message handler callback
//   3. Send messages using SendMessage()
//   4. Call Stop() when shutting down
//
// Example:
//
//   discordBot := bot.NewDiscordBot(token, channelID)
//   err := discordBot.Start(func(msg bot.BotMessage) {
//       fmt.Printf("Received: %s\n", msg.Content)
//   })
//   if err != nil {
//       log.Fatal(err)
//   }
//   discordBot.SendMessage(channelID, "Hello, world!")
//   discordBot.Stop()
//
// # Thread Safety
//
// All bot adapters are thread-safe and use internal mutexes to protect
// shared state. The message handler callback may be called concurrently
// from multiple goroutines.
//
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
