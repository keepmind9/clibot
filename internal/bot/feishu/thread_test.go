package feishu

import (
	"testing"

	"github.com/keepmind9/clibot/internal/bot"
	"github.com/stretchr/testify/assert"
)

func TestBot_ThreadScope(t *testing.T) {
	b := NewBot("test-app", "test-secret")

	tests := []struct {
		name      string
		channelID string
		msg       bot.BotMessage
		expected  string
	}{
		{
			name:      "p2p chat returns channel ID unchanged",
			channelID: "chat_001",
			msg: bot.BotMessage{
				ChatType: "p2p",
			},
			expected: "chat_001",
		},
		{
			name:      "group chat returns channel ID unchanged",
			channelID: "chat_002",
			msg: bot.BotMessage{
				ChatType: "group",
			},
			expected: "chat_002",
		},
		{
			name:      "topic with thread ID returns scoped key",
			channelID: "chat_003",
			msg: bot.BotMessage{
				ChatType: "topic",
				ThreadID: "thread_001",
			},
			expected: "chat_003:thread_001",
		},
		{
			name:      "topic without thread ID returns channel ID unchanged",
			channelID: "chat_004",
			msg: bot.BotMessage{
				ChatType: "topic",
				ThreadID: "",
			},
			expected: "chat_004",
		},
		{
			name:      "empty chat type returns channel ID unchanged",
			channelID: "chat_005",
			msg: bot.BotMessage{
				ChatType: "",
			},
			expected: "chat_005",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := b.ThreadScope(tt.channelID, tt.msg)
			assert.Equal(t, tt.expected, result)
		})
	}
}
