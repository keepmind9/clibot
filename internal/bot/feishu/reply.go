package feishu

// SendMessageWithReply sends a message that visually references a parent message.
// The Feishu SDK v3 does not expose a native reply-to endpoint as a separate method,
// so this falls back to regular SendMessage.
func (b *Bot) SendMessageWithReply(channel, message, replyToMessageID string) error {
	if replyToMessageID == "" {
		return b.SendMessage(channel, message)
	}
	// The SDK's CreateMessage API does not have a dedicated reply field in v3.5.3.
	// Fall back to sending a regular message in the same channel.
	return b.SendMessage(channel, message)
}
