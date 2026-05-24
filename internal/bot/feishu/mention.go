package feishu

import (
	"strings"

	"github.com/keepmind9/clibot/internal/bot"
)

// ShouldRespond determines whether the bot should respond to a message
// based on mention rules and chat type.
//
// Rules:
//   - p2p or empty chatType: always respond
//   - group/topic with mentionInGroup=true: only respond if content contains @_user_1
//   - group/topic with mentionInGroup=false: always respond
func (b *Bot) ShouldRespond(msg bot.BotMessage) bool {
	if msg.ChatType == "" || msg.ChatType == "p2p" {
		return true
	}

	b.mu.RLock()
	mention := b.mentionInGroup
	b.mu.RUnlock()

	if !mention {
		return true
	}

	return strings.Contains(msg.Content, "@_user_1")
}

// SetMentionInGroup configures whether the bot requires an explicit @mention
// in group/topic chats to respond.
func (b *Bot) SetMentionInGroup(mention bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.mentionInGroup = mention
}
