package cli

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMessageContent_UnmarshalJSON tests UnmarshalJSON method
func TestMessageContent_UnmarshalJSON(t *testing.T) {
	t.Run("full message object with string content", func(t *testing.T) {
		data := []byte(`{
			"id": "msg123",
			"type": "message",
			"role": "assistant",
			"model": "claude-3",
			"content": "Hello, world!",
			"stop_reason": "end_turn"
		}`)

		var mc MessageContent
		err := mc.UnmarshalJSON(data)
		require.NoError(t, err)
		assert.Equal(t, "msg123", mc.ID)
		assert.Equal(t, "message", mc.Type)
		assert.Equal(t, "assistant", mc.Role)
		assert.Equal(t, "claude-3", mc.Model)
		assert.Equal(t, "Hello, world!", mc.ContentText)
		assert.Equal(t, "end_turn", mc.StopReason)
	})

	t.Run("full message object with array content", func(t *testing.T) {
		data := []byte(`{
			"id": "msg124",
			"type": "message",
			"role": "assistant",
			"content": [
				{"type": "text", "text": "Hello"},
				{"type": "text", "text": " world!"}
			],
			"stop_reason": "end_turn"
		}`)

		var mc MessageContent
		err := mc.UnmarshalJSON(data)
		require.NoError(t, err)
		assert.Equal(t, "msg124", mc.ID)
		assert.Equal(t, "message", mc.Type)
		assert.Equal(t, "assistant", mc.Role)
		// ContentText is not extracted from array content
		assert.Equal(t, "", mc.ContentText)
		// Check that Content array is properly populated
		assert.Len(t, mc.Content, 2)
		assert.Equal(t, "text", mc.Content[0].Type)
		assert.Equal(t, "Hello", mc.Content[0].Text)
		assert.Equal(t, " world!", mc.Content[1].Text)
	})

	t.Run("simple string content", func(t *testing.T) {
		data := []byte(`"Just a simple string"`)

		var mc MessageContent
		err := mc.UnmarshalJSON(data)
		require.NoError(t, err)
		assert.Equal(t, "Just a simple string", mc.ContentText)
	})

	t.Run("empty string", func(t *testing.T) {
		data := []byte(`""`)

		var mc MessageContent
		err := mc.UnmarshalJSON(data)
		require.NoError(t, err)
		assert.Equal(t, "", mc.ContentText)
	})

	t.Run("null value", func(t *testing.T) {
		data := []byte(`null`)

		var mc MessageContent
		err := mc.UnmarshalJSON(data)
		require.NoError(t, err)
		assert.Equal(t, "", mc.ContentText)
	})

	t.Run("invalid JSON", func(t *testing.T) {
		data := []byte(`not valid json`)

		var mc MessageContent
		err := mc.UnmarshalJSON(data)
		assert.Error(t, err)
	})

	t.Run("full message with usage", func(t *testing.T) {
		data := []byte(`{
			"id": "msg125",
			"type": "message",
			"role": "assistant",
			"content": "Test",
			"usage": {"input_tokens": 10, "output_tokens": 20}
		}`)

		var mc MessageContent
		err := mc.UnmarshalJSON(data)
		require.NoError(t, err)
		assert.Equal(t, "msg125", mc.ID)
		assert.Equal(t, "Test", mc.ContentText)
	})

	t.Run("message with stop_reason", func(t *testing.T) {
		data := []byte(`{
			"id": "msg126",
			"content": "Test",
			"stop_reason": "max_tokens"
		}`)

		var mc MessageContent
		err := mc.UnmarshalJSON(data)
		require.NoError(t, err)
		assert.Equal(t, "max_tokens", mc.StopReason)
	})

	t.Run("message with nested content blocks", func(t *testing.T) {
		data := []byte(`{
			"content": [
				{
					"type": "tool_use",
					"id": "tool123",
					"name": "calculator",
					"input": {"expression": "2+2"}
				},
				{
					"type": "text",
					"text": "Result: 4"
				}
			]
		}`)

		var mc MessageContent
		err := mc.UnmarshalJSON(data)
		require.NoError(t, err)
		// ContentText is not extracted from array content
		assert.Equal(t, "", mc.ContentText)
		// Check that Content array is properly populated
		assert.Len(t, mc.Content, 2)
		assert.Equal(t, "tool_use", mc.Content[0].Type)
		assert.Equal(t, "text", mc.Content[1].Type)
		assert.Equal(t, "Result: 4", mc.Content[1].Text)
	})
}

// TestTranscriptMessage_UnmarshalJSON tests TranscriptMessage JSON unmarshaling
func TestTranscriptMessage_UnmarshalJSON(t *testing.T) {
	t.Run("valid transcript message", func(t *testing.T) {
		data := []byte(`{
			"type": "user",
			"isMeta": false,
			"message": {
				"id": "msg1",
				"content": "Hello"
			}
		}`)

		var tm TranscriptMessage
		err := json.Unmarshal(data, &tm)
		require.NoError(t, err)
		assert.Equal(t, "user", tm.Type)
		assert.False(t, tm.IsMeta)
		assert.Equal(t, "msg1", tm.Message.ID)
		assert.Equal(t, "Hello", tm.Message.ContentText)
	})

	t.Run("transcript message with metadata", func(t *testing.T) {
		data := []byte(`{
			"type": "system",
			"isMeta": true,
			"message": {
				"content": "System started"
			}
		}`)

		var tm TranscriptMessage
		err := json.Unmarshal(data, &tm)
		require.NoError(t, err)
		assert.Equal(t, "system", tm.Type)
		assert.True(t, tm.IsMeta)
	})
}

// TestExtractLatestSubagentFile tests extractLatestSubagentFile function
func TestExtractLatestSubagentFile(t *testing.T) {
	t.Run("nonexistent directory", func(t *testing.T) {
		result, err := extractLatestSubagentFile("/nonexistent/directory")
		assert.Error(t, err)
		assert.Empty(t, result)
	})

	t.Run("empty directory path", func(t *testing.T) {
		result, err := extractLatestSubagentFile("")
		assert.Error(t, err)
		assert.Empty(t, result)
	})
}

// TestParseTranscript tests parseTranscript function
func TestParseTranscript(t *testing.T) {
	t.Run("nonexistent file", func(t *testing.T) {
		messages, err := parseTranscript("/nonexistent/file.json")
		assert.Error(t, err)
		assert.Nil(t, messages)
	})

	t.Run("empty file path", func(t *testing.T) {
		messages, err := parseTranscript("")
		assert.Error(t, err)
		assert.Nil(t, messages)
	})

	t.Run("invalid JSON file", func(t *testing.T) {
		// Create a temporary file with invalid JSON
		// parseTranscript skips invalid lines, so it won't error
		tmpFile := t.TempDir() + "/invalid.json"
		err := os.WriteFile(tmpFile, []byte("not valid json"), 0644)
		require.NoError(t, err)

		messages, err := parseTranscript(tmpFile)
		// Function skips invalid lines, returns empty slice
		assert.NoError(t, err)
		assert.Empty(t, messages)
	})

	t.Run("valid JSONL with single message", func(t *testing.T) {
		tmpFile := t.TempDir() + "/transcript.jsonl"
		// JSONL format: one JSON object per line
		content := `{"type": "user", "is_meta": false, "message": {"id": "msg1", "content": "Hello"}}`
		err := os.WriteFile(tmpFile, []byte(content), 0644)
		require.NoError(t, err)

		messages, err := parseTranscript(tmpFile)
		require.NoError(t, err)
		assert.Len(t, messages, 1)
		assert.Equal(t, "user", messages[0].Type)
		assert.Equal(t, "Hello", messages[0].Message.ContentText)
	})

	t.Run("valid JSONL with multiple messages", func(t *testing.T) {
		tmpFile := t.TempDir() + "/transcript.jsonl"
		// JSONL format: one JSON object per line
		content := `{"type": "user", "is_meta": false, "message": {"content": "Hello"}}
{"type": "assistant", "is_meta": false, "message": {"content": "Hi there!"}}
{"type": "system", "is_meta": true, "message": {"content": "System note"}}`
		err := os.WriteFile(tmpFile, []byte(content), 0644)
		require.NoError(t, err)

		messages, err := parseTranscript(tmpFile)
		require.NoError(t, err)
		// Only user and assistant messages are processed, system is skipped
		assert.Len(t, messages, 2)
		assert.Equal(t, "user", messages[0].Type)
		assert.Equal(t, "assistant", messages[1].Type)
	})

	t.Run("JSONL with empty lines", func(t *testing.T) {
		tmpFile := t.TempDir() + "/transcript.jsonl"
		content := `{"type": "user", "is_meta": false, "message": {"content": "Hello"}}

{"type": "assistant", "is_meta": false, "message": {"content": "Hi!"}}`
		err := os.WriteFile(tmpFile, []byte(content), 0644)
		require.NoError(t, err)

		messages, err := parseTranscript(tmpFile)
		require.NoError(t, err)
		assert.Len(t, messages, 2)
	})
}

// TestExtractLatestInteraction_FromTranscript tests extractLatestInteraction with transcript data
func TestExtractLatestInteraction_FromTranscript(t *testing.T) {
	t.Run("transcript with user and assistant", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create transcript file in JSONL format
		transcriptContent := `{"type": "user", "is_meta": false, "message": {"content": "What is 2+2?"}}
{"type": "assistant", "is_meta": false, "message": {"content": "2+2 equals 4."}}`
		transcriptFile := tmpDir + "/transcript.jsonl"
		err := os.WriteFile(transcriptFile, []byte(transcriptContent), 0644)
		require.NoError(t, err)

		prompt, response, err := ExtractLatestInteraction(transcriptFile)
		require.NoError(t, err)
		assert.Equal(t, "What is 2+2?", prompt)
		assert.Equal(t, "2+2 equals 4.", response)
	})

	t.Run("transcript with only user message", func(t *testing.T) {
		tmpDir := t.TempDir()

		transcriptContent := `{"type": "user", "is_meta": false, "message": {"content": "Hello?"}}`
		transcriptFile := tmpDir + "/transcript.jsonl"
		err := os.WriteFile(transcriptFile, []byte(transcriptContent), 0644)
		require.NoError(t, err)

		prompt, response, err := ExtractLatestInteraction(transcriptFile)
		require.NoError(t, err)
		assert.Equal(t, "Hello?", prompt)
		assert.Empty(t, response)
	})

	t.Run("empty transcript", func(t *testing.T) {
		tmpDir := t.TempDir()

		transcriptContent := ``
		transcriptFile := tmpDir + "/transcript.jsonl"
		err := os.WriteFile(transcriptFile, []byte(transcriptContent), 0644)
		require.NoError(t, err)

		prompt, response, err := ExtractLatestInteraction(transcriptFile)
		// Empty file should error because no user messages found
		assert.Error(t, err)
		assert.Empty(t, prompt)
		assert.Empty(t, response)
	})
}
