package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestIsRealUserMessage tests isRealUserMessage function
func TestIsRealUserMessage(t *testing.T) {
	t.Run("real user message with content", func(t *testing.T) {
		msg := TranscriptMessage{
			Type:    "user",
			IsMeta:  false,
			Message: MessageContent{ContentText: "Hello world"},
		}
		assert.True(t, isRealUserMessage(msg))
	})

	t.Run("real user message with content blocks", func(t *testing.T) {
		msg := TranscriptMessage{
			Type:   "user",
			IsMeta: false,
			Message: MessageContent{
				Content: []ContentBlock{
					{Type: "text", Text: "Hello"},
				},
			},
		}
		assert.True(t, isRealUserMessage(msg))
	})

	t.Run("meta message", func(t *testing.T) {
		msg := TranscriptMessage{
			Type:    "user",
			IsMeta:  true,
			Message: MessageContent{ContentText: "test"},
		}
		assert.False(t, isRealUserMessage(msg))
	})

	t.Run("assistant message", func(t *testing.T) {
		msg := TranscriptMessage{
			Type:    "assistant",
			IsMeta:  false,
			Message: MessageContent{ContentText: "test"},
		}
		assert.False(t, isRealUserMessage(msg))
	})

	t.Run("user message with empty content", func(t *testing.T) {
		msg := TranscriptMessage{
			Type:    "user",
			IsMeta:  false,
			Message: MessageContent{},
		}
		assert.False(t, isRealUserMessage(msg))
	})

	t.Run("progress message", func(t *testing.T) {
		msg := TranscriptMessage{
			Type:    "progress",
			IsMeta:  false,
			Message: MessageContent{ContentText: "test"},
		}
		assert.False(t, isRealUserMessage(msg))
	})

	t.Run("user message with local command", func(t *testing.T) {
		msg := TranscriptMessage{
			Type:    "user",
			IsMeta:  false,
			Message: MessageContent{ContentText: "<local-command-test>"},
		}
		// Messages starting with <local-command- are internal commands
		assert.False(t, isRealUserMessage(msg))
	})

	t.Run("user message with command name", func(t *testing.T) {
		msg := TranscriptMessage{
			Type:    "user",
			IsMeta:  false,
			Message: MessageContent{ContentText: "<command-name>test"},
		}
		// Messages starting with <command-name> are internal commands
		assert.False(t, isRealUserMessage(msg))
	})
}

// TestGetMessageText tests getMessageText function
func TestGetMessageText(t *testing.T) {
	t.Run("text from ContentText", func(t *testing.T) {
		msg := TranscriptMessage{
			Message: MessageContent{ContentText: "Hello world"},
		}
		text := getMessageText(msg)
		assert.Equal(t, "Hello world", text)
	})

	t.Run("text from content blocks", func(t *testing.T) {
		msg := TranscriptMessage{
			Message: MessageContent{
				Content: []ContentBlock{
					{Type: "text", Text: "First"},
					{Type: "text", Text: "Second"},
				},
			},
		}
		text := getMessageText(msg)
		assert.Equal(t, "First\n\nSecond", text)
	})

	t.Run("mixed content blocks", func(t *testing.T) {
		msg := TranscriptMessage{
			Message: MessageContent{
				Content: []ContentBlock{
					{Type: "thinking", Thinking: "thinking..."},
					{Type: "text", Text: "Actual text"},
				},
			},
		}
		text := getMessageText(msg)
		assert.Equal(t, "Actual text", text)
	})

	t.Run("empty content", func(t *testing.T) {
		msg := TranscriptMessage{
			Message: MessageContent{},
		}
		text := getMessageText(msg)
		assert.Equal(t, "", text)
	})

	t.Run("non-text content blocks only", func(t *testing.T) {
		msg := TranscriptMessage{
			Message: MessageContent{
				Content: []ContentBlock{
					{Type: "thinking", Thinking: "thinking..."},
					{Type: "image", Text: ""},
				},
			},
		}
		text := getMessageText(msg)
		assert.Equal(t, "", text)
	})
}

// TestContentBlock_TextExtraction tests ContentBlock behavior
func TestContentBlock_TextExtraction(t *testing.T) {
	t.Run("text block", func(t *testing.T) {
		block := ContentBlock{Type: "text", Text: "Hello"}
		assert.Equal(t, "text", block.Type)
		assert.Equal(t, "Hello", block.Text)
	})

	t.Run("thinking block", func(t *testing.T) {
		block := ContentBlock{Type: "thinking", Thinking: "thinking..."}
		assert.Equal(t, "thinking", block.Type)
		assert.Equal(t, "thinking...", block.Thinking)
	})

	t.Run("image block", func(t *testing.T) {
		block := ContentBlock{Type: "image"}
		assert.Equal(t, "image", block.Type)
	})
}

// TestTranscriptMessage_Fields tests TranscriptMessage structure
func TestTranscriptMessage_Fields(t *testing.T) {
	msg := TranscriptMessage{
		Type:      "user",
		SessionID: "session-123",
		IsMeta:    false,
		Message: MessageContent{
			Role:        "user",
			Type:        "message",
			ContentText: "test",
		},
	}

	assert.Equal(t, "user", msg.Type)
	assert.Equal(t, "session-123", msg.SessionID)
	assert.False(t, msg.IsMeta)
	assert.Equal(t, "user", msg.Message.Role)
}

// TestMessageContent_Structure tests MessageContent structure
func TestMessageContent_Structure(t *testing.T) {
	content := MessageContent{
		ID:          "msg-123",
		Type:        "message",
		Role:        "user",
		Model:       "claude-3",
		ContentText: "Hello",
		Content: []ContentBlock{
			{Type: "text", Text: "Hello"},
		},
	}

	assert.Equal(t, "msg-123", content.ID)
	assert.Equal(t, "message", content.Type)
	assert.Equal(t, "user", content.Role)
	assert.Equal(t, "claude-3", content.Model)
	assert.Equal(t, "Hello", content.ContentText)
	assert.Len(t, content.Content, 1)
}
