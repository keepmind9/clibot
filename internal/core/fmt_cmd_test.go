package core

import (
	"testing"

	"github.com/keepmind9/clibot/internal/bot"
	"github.com/stretchr/testify/assert"
)

func TestEngine_FmtCmd(t *testing.T) {
	engine := NewEngine(&Config{})

	tests := []struct {
		platform string
		cmd      string
		expected string
	}{
		{
			platform: "telegram",
			cmd:      "slist",
			expected: "[slist](tg://msg?text=slist)",
		},
		{
			platform: "telegram",
			cmd:      "suse [name]",
			expected: "[suse [name]](tg://msg?text=suse+)",
		},
		{
			platform: "discord",
			cmd:      "slist",
			expected: "`slist`",
		},
	}

	for _, tt := range tests {
		msg := bot.BotMessage{Platform: tt.platform}
		result := engine.fmtCmd(msg, tt.cmd)
		assert.Equal(t, tt.expected, result)
	}
}
