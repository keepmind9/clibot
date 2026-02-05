package bot

import (
	"context"
	"testing"

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
			expected: "hello\nworld", // JSON unescape \n to actual newline
		},
		{
			name:     "plain text without JSON",
			content:  "plain text",
			expected: "plain text",
		},
		{
			name:     "empty JSON",
			content:  `{}`,
			expected: "", // Empty text field returns empty string
		},
		{
			name:     "invalid JSON",
			content:  `{invalid}`,
			expected: "{invalid}", // Returns original content on parse failure
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := escapeJSONString(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMaskSecret(t *testing.T) {
	tests := []struct {
		name     string
		secret   string
		expected string
	}{
		{
			name:     "normal secret",
			secret:   "cli_1234567890abcdef",
			expected: "cli_***cdef",
		},
		{
			name:     "short secret",
			secret:   "1234567890",
			expected: "***",
		},
		{
			name:     "very short secret",
			secret:   "1234",
			expected: "***",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := maskSecret(tt.secret)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFeishuBot_HandleMessageReceive(t *testing.T) {
	bot := NewFeishuBot("test_app_id", "test_app_secret")

	// Create a test message event with proper structure
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

	messagesReceived := []BotMessage{}
	bot.SetMessageHandler(func(msg BotMessage) {
		messagesReceived = append(messagesReceived, msg)
	})

	// Handle the message
	err := bot.handleMessageReceive(context.Background(), event)
	assert.NoError(t, err)

	// Verify message was processed
	assert.Len(t, messagesReceived, 1)
	assert.Equal(t, "feishu", messagesReceived[0].Platform)
	assert.Equal(t, "test_user_id", messagesReceived[0].UserID)
	assert.Equal(t, "test_chat_id", messagesReceived[0].Channel)
	assert.Equal(t, "hello world", messagesReceived[0].Content)
}

func TestFeishuBot_HandleMessageReceive_NilEvent(t *testing.T) {
	bot := NewFeishuBot("test_app_id", "test_app_secret")

	// Test nil event
	err := bot.handleMessageReceive(context.Background(), nil)
	assert.NoError(t, err)

	// Test event with nil Event field
	err = bot.handleMessageReceive(context.Background(), &larkim.P2MessageReceiveV1{})
	assert.NoError(t, err)
}

// Helper functions for creating pointers
func stringPtr(s string) *string {
	return &s
}

func int64Ptr(i int64) *int64 {
	return &i
}
