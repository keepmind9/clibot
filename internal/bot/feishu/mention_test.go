package feishu

import (
	"testing"

	"github.com/keepmind9/clibot/internal/bot"
	"github.com/stretchr/testify/assert"
)

func TestBot_ShouldRespond(t *testing.T) {
	tests := []struct {
		name           string
		mentionInGroup bool
		msg            bot.BotMessage
		expected       bool
	}{
		{
			name:           "p2p always responds with mention disabled",
			mentionInGroup: false,
			msg: bot.BotMessage{
				ChatType: "p2p",
				Content:  "hello",
			},
			expected: true,
		},
		{
			name:           "p2p always responds with mention enabled",
			mentionInGroup: true,
			msg: bot.BotMessage{
				ChatType: "p2p",
				Content:  "hello",
			},
			expected: true,
		},
		{
			name:           "empty chatType always responds",
			mentionInGroup: true,
			msg: bot.BotMessage{
				ChatType: "",
				Content:  "hello",
			},
			expected: true,
		},
		{
			name:           "group without mention required responds to any message",
			mentionInGroup: false,
			msg: bot.BotMessage{
				ChatType: "group",
				Content:  "hello everyone",
			},
			expected: true,
		},
		{
			name:           "group with mention required and bot is mentioned",
			mentionInGroup: true,
			msg: bot.BotMessage{
				ChatType: "group",
				Content:  "@_user_1 what do you think?",
			},
			expected: true,
		},
		{
			name:           "group with mention required and bot is not mentioned",
			mentionInGroup: true,
			msg: bot.BotMessage{
				ChatType: "group",
				Content:  "just chatting here",
			},
			expected: false,
		},
		{
			name:           "topic without mention required responds to any message",
			mentionInGroup: false,
			msg: bot.BotMessage{
				ChatType: "topic",
				Content:  "topic message",
			},
			expected: true,
		},
		{
			name:           "topic with mention required and bot is mentioned",
			mentionInGroup: true,
			msg: bot.BotMessage{
				ChatType: "topic",
				Content:  "@_user_1 help please",
			},
			expected: true,
		},
		{
			name:           "topic with mention required and bot is not mentioned",
			mentionInGroup: true,
			msg: bot.BotMessage{
				ChatType: "topic",
				Content:  "general discussion",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := NewBot("test-app", "test-secret")
			b.SetMentionInGroup(tt.mentionInGroup)
			result := b.ShouldRespond(tt.msg)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBot_SetMentionInGroup(t *testing.T) {
	b := NewBot("test-app", "test-secret")

	// Default should be true (mention required in groups)
	assert.False(t, b.ShouldRespond(bot.BotMessage{
		ChatType: "group",
		Content:  "no mention",
	}))
	assert.True(t, b.ShouldRespond(bot.BotMessage{
		ChatType: "p2p",
		Content:  "no mention",
	}))

	b.SetMentionInGroup(true)
	assert.False(t, b.ShouldRespond(bot.BotMessage{
		ChatType: "group",
		Content:  "no mention",
	}))
	assert.True(t, b.ShouldRespond(bot.BotMessage{
		ChatType: "group",
		Content:  "@_user_1 mentioned",
	}))

	b.SetMentionInGroup(false)
	assert.True(t, b.ShouldRespond(bot.BotMessage{
		ChatType: "group",
		Content:  "no mention",
	}))
}
