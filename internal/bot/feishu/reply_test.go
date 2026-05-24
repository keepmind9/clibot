package feishu

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBot_SendMessageWithReply(t *testing.T) {
	tests := []struct {
		name          string
		channel       string
		message       string
		replyToID     string
		expectError   bool
		errorContains string
	}{
		{
			name:          "empty replyToID falls back to SendMessage (no client)",
			channel:       "chat_001",
			message:       "hello",
			replyToID:     "",
			expectError:   true,
			errorContains: "not initialized",
		},
		{
			name:          "non-empty replyToID falls back to SendMessage (no client)",
			channel:       "chat_002",
			message:       "reply message",
			replyToID:     "msg_original",
			expectError:   true,
			errorContains: "not initialized",
		},
		{
			name:          "empty channel still goes through SendMessage",
			channel:       "",
			message:       "hello",
			replyToID:     "msg_123",
			expectError:   true,
			errorContains: "not initialized",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := NewBot("test-app", "test-secret")
			err := b.SendMessageWithReply(tt.channel, tt.message, tt.replyToID)
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestBot_SendMessageWithReply_DelegatesToExistingFields(t *testing.T) {
	b := NewBot("test-app", "test-secret")

	// Both code paths (empty and non-empty replyToID) delegate to SendMessage.
	// Without a real lark client, both should fail with "not initialized".
	err := b.SendMessageWithReply("chat_001", "hello", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")

	err = b.SendMessageWithReply("chat_001", "hello", "msg_reply_target")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}
