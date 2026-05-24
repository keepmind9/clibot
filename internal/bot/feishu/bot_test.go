package feishu

import (
	"context"
	"testing"

	"github.com/keepmind9/clibot/internal/bot"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	"github.com/stretchr/testify/assert"
)

func TestExtractTextContent(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name:     "normal text message",
			content:  `{"text":"hello world"}`,
			expected: "hello world",
		},
		{
			name:     "text with special chars",
			content:  `{"text":"hello\nworld"}`,
			expected: "hello\nworld",
		},
		{
			name:     "plain text without JSON",
			content:  "plain text",
			expected: "plain text",
		},
		{
			name:     "empty JSON",
			content:  `{}`,
			expected: "",
		},
		{
			name:     "invalid JSON",
			content:  `{invalid}`,
			expected: "{invalid}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractTextContent(tt.content)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEscapeJSONString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "normal text",
			input:    "hello world",
			expected: "hello world",
		},
		{
			name:     "with quote",
			input:    `say "hello"`,
			expected: `say \"hello\"`,
		},
		{
			name:     "with backslash",
			input:    `path\to\file`,
			expected: `path\\to\\file`,
		},
		{
			name:     "with newline",
			input:    "line1\nline2",
			expected: "line1\\nline2",
		},
		{
			name:     "with carriage return",
			input:    "line1\rline2",
			expected: "line1\\rline2",
		},
		{
			name:     "with tab",
			input:    "col1\tcol2",
			expected: "col1\\tcol2",
		},
		{
			name:     "mixed special chars",
			input:    "quote: \"\nbackslash: \\\r",
			expected: "quote: \\\"\\nbackslash: \\\\\\r",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "unicode characters",
			input:    "hello 世界",
			expected: "hello 世界",
		},
		{
			name:     "all escape sequences",
			input:    "\"\\\n\r\t",
			expected: "\\\"\\\\\\n\\r\\t",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := escapeJSONString(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBot_HandleMessageReceive(t *testing.T) {
	b := NewBot("test_app_id", "test_app_secret")

	userID := "test_user_id"
	messageID := "test_message_id"
	chatID := "test_chat_id"
	messageType := "text"
	chatType := "p2p"
	content := `{"text":"hello world"}`

	event := &larkim.P2MessageReceiveV1{
		Event: &larkim.P2MessageReceiveV1Data{
			Sender: &larkim.EventSender{
				SenderId: &larkim.UserId{UserId: &userID},
			},
			Message: &larkim.EventMessage{
				MessageId:   &messageID,
				ChatId:      &chatID,
				MessageType: &messageType,
				ChatType:    &chatType,
				Content:     &content,
			},
		},
	}

	messagesReceived := []bot.BotMessage{}
	b.SetMessageHandler(func(msg bot.BotMessage) {
		messagesReceived = append(messagesReceived, msg)
	})

	err := b.handleMessageReceive(context.Background(), event)
	assert.NoError(t, err)

	assert.Len(t, messagesReceived, 1)
	assert.Equal(t, "feishu", messagesReceived[0].Platform)
	assert.Equal(t, "test_user_id", messagesReceived[0].UserID)
	assert.Equal(t, "test_chat_id", messagesReceived[0].Channel)
	assert.Equal(t, "hello world", messagesReceived[0].Content)
	assert.Equal(t, "p2p", messagesReceived[0].ChatType)
}

func TestBot_HandleMessageReceive_NilEvent(t *testing.T) {
	b := NewBot("test_app_id", "test_app_secret")

	err := b.handleMessageReceive(context.Background(), nil)
	assert.NoError(t, err)

	err = b.handleMessageReceive(context.Background(), &larkim.P2MessageReceiveV1{})
	assert.NoError(t, err)
}
