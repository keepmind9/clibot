package feishu

import "github.com/keepmind9/clibot/internal/bot"

// ThreadScope returns the session scope key for a message, considering thread/topic context.
// For topic messages with a ThreadID, it returns "channelID:threadID" for thread isolation.
// Otherwise, it returns the channelID unchanged.
func (b *Bot) ThreadScope(channelID string, msg bot.BotMessage) string {
	if msg.ChatType == "topic" && msg.ThreadID != "" {
		return channelID + ":" + msg.ThreadID
	}
	return channelID
}
