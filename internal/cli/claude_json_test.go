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
		assert.Equal(t, "", mc.ContentText)
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
		assert.Equal(t, "", mc.ContentText)
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

	t.Run("new format with isSidechain", func(t *testing.T) {
		data := []byte(`{
				"type": "user",
				"isSidechain": true,
				"sessionId": "abc-123",
				"message": {
					"role": "user",
					"content": "internal message"
				}
			}`)

		var tm TranscriptMessage
		err := json.Unmarshal(data, &tm)
		require.NoError(t, err)
		assert.Equal(t, "user", tm.Type)
		assert.False(t, tm.IsMeta)
		assert.True(t, tm.IsSidechain)
		assert.Equal(t, "abc-123", tm.SessionID)
		assert.Equal(t, "internal message", tm.Message.ContentText)
	})

	t.Run("new format with isCompactSummary", func(t *testing.T) {
		data := []byte(`{
				"type": "user",
				"isSidechain": false,
				"isCompactSummary": true,
				"sessionId": "abc-123",
				"message": {
					"role": "user",
					"content": "This session is being continued from a previous conversation"
				}
			}`)

		var tm TranscriptMessage
		err := json.Unmarshal(data, &tm)
		require.NoError(t, err)
		assert.Equal(t, "user", tm.Type)
		assert.False(t, tm.IsMeta)
		assert.False(t, tm.IsSidechain)
		assert.True(t, tm.IsCompactSummary)
	})

	t.Run("new format user message without isMeta", func(t *testing.T) {
		data := []byte(`{
				"type": "user",
				"isSidechain": false,
				"sessionId": "abc-123",
				"message": {
					"role": "user",
					"content": "Hello from new format"
				}
			}`)

		var tm TranscriptMessage
		err := json.Unmarshal(data, &tm)
		require.NoError(t, err)
		assert.Equal(t, "user", tm.Type)
		assert.False(t, tm.IsMeta)
		assert.False(t, tm.IsSidechain)
		assert.False(t, tm.IsCompactSummary)
		assert.Equal(t, "Hello from new format", tm.Message.ContentText)
		assert.True(t, isRealUserMessage(tm))
	})

	t.Run("new format with tool_result content blocks", func(t *testing.T) {
		data := []byte(`{
				"type": "user",
				"isSidechain": false,
				"sessionId": "abc-123",
				"message": {
					"role": "user",
					"content": [
						{"type": "tool_result", "tool_use_id": "call_abc123", "content": "file content here"}
					]
				}
			}`)

		var tm TranscriptMessage
		err := json.Unmarshal(data, &tm)
		require.NoError(t, err)
		assert.Equal(t, "user", tm.Type)
		assert.Empty(t, tm.Message.ContentText)
		assert.Len(t, tm.Message.Content, 1)
		assert.Equal(t, "tool_result", tm.Message.Content[0].Type)
		assert.Empty(t, getMessageText(tm))
		assert.False(t, isRealUserMessage(tm))
	})

	t.Run("new format assistant with thinking and tool_use blocks", func(t *testing.T) {
		data := []byte(`{
				"type": "assistant",
				"isSidechain": false,
				"sessionId": "abc-123",
				"message": {
					"id": "msg_456",
					"type": "message",
					"role": "assistant",
					"model": "claude-opus-4-7",
					"content": [
						{"type": "thinking", "thinking": "let me think...", "signature": "sig123"},
						{"type": "tool_use", "id": "tool123", "name": "Read", "input": {"file_path": "/test.go"}},
						{"type": "text", "text": "Here is the result."}
					],
					"stop_reason": "end_turn"
				}
			}`)

		var tm TranscriptMessage
		err := json.Unmarshal(data, &tm)
		require.NoError(t, err)
		assert.Equal(t, "assistant", tm.Type)
		assert.Equal(t, "msg_456", tm.Message.ID)
		assert.Equal(t, "claude-opus-4-7", tm.Message.Model)
		assert.Len(t, tm.Message.Content, 3)
		assert.Equal(t, "thinking", tm.Message.Content[0].Type)
		assert.Equal(t, "tool_use", tm.Message.Content[1].Type)
		assert.Equal(t, "text", tm.Message.Content[2].Type)
		assert.Equal(t, "Here is the result.", getMessageText(tm))
	})

	t.Run("new format with both isMeta and isSidechain", func(t *testing.T) {
		data := []byte(`{
				"type": "user",
				"isMeta": false,
				"isSidechain": false,
				"sessionId": "abc-123",
				"message": {
					"role": "user",
					"content": "Hello transition format"
				}
			}`)

		var tm TranscriptMessage
		err := json.Unmarshal(data, &tm)
		require.NoError(t, err)
		assert.False(t, tm.IsMeta)
		assert.False(t, tm.IsSidechain)
		assert.True(t, isRealUserMessage(tm))
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
		tmpFile := t.TempDir() + "/invalid.json"
		err := os.WriteFile(tmpFile, []byte("not valid json"), 0644)
		require.NoError(t, err)

		messages, err := parseTranscript(tmpFile)
		assert.NoError(t, err)
		assert.Empty(t, messages)
	})

	t.Run("valid JSONL with single message", func(t *testing.T) {
		tmpFile := t.TempDir() + "/transcript.jsonl"
		content := `{"type": "user", "isMeta": false, "message": {"id": "msg1", "content": "Hello"}}`
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
		content := `{"type": "user", "isMeta": false, "message": {"content": "Hello"}}
{"type": "assistant", "isMeta": false, "message": {"content": "Hi there!"}}
{"type": "system", "isMeta": true, "message": {"content": "System note"}}`
		err := os.WriteFile(tmpFile, []byte(content), 0644)
		require.NoError(t, err)

		messages, err := parseTranscript(tmpFile)
		require.NoError(t, err)
		assert.Len(t, messages, 2)
		assert.Equal(t, "user", messages[0].Type)
		assert.Equal(t, "assistant", messages[1].Type)
	})

	t.Run("JSONL with empty lines", func(t *testing.T) {
		tmpFile := t.TempDir() + "/transcript.jsonl"
		content := `{"type": "user", "isMeta": false, "message": {"content": "Hello"}}

{"type": "assistant", "isMeta": false, "message": {"content": "Hi!"}}`
		err := os.WriteFile(tmpFile, []byte(content), 0644)
		require.NoError(t, err)

		messages, err := parseTranscript(tmpFile)
		require.NoError(t, err)
		assert.Len(t, messages, 2)
	})

	t.Run("new format JSONL with isSidechain filtering", func(t *testing.T) {
		tmpFile := t.TempDir() + "/transcript.jsonl"
		content := `{"type":"user","isSidechain":false,"sessionId":"s1","message":{"role":"user","content":"Real question"}}
{"type":"user","isSidechain":true,"sessionId":"s1","message":{"role":"user","content":"Sidechain message"}}
{"type":"user","isSidechain":false,"isCompactSummary":true,"sessionId":"s1","message":{"role":"user","content":"Compact summary"}}
{"type":"user","isSidechain":false,"sessionId":"s1","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"t1","content":"result"}]}}
{"type":"assistant","isSidechain":false,"sessionId":"s1","message":{"id":"m1","type":"message","role":"assistant","content":[{"type":"text","text":"The answer."}],"stop_reason":"end_turn"}}`
		err := os.WriteFile(tmpFile, []byte(content), 0644)
		require.NoError(t, err)

		messages, err := parseTranscript(tmpFile)
		require.NoError(t, err)
		assert.Len(t, messages, 5)

		prompt, response, err := ExtractLatestInteraction(tmpFile)
		require.NoError(t, err)
		assert.Equal(t, "Real question", prompt)
		assert.Equal(t, "The answer.", response)
	})
}

// TestExtractLatestInteraction_FromTranscript tests extractLatestInteraction with transcript data
func TestExtractLatestInteraction_FromTranscript(t *testing.T) {
	t.Run("transcript with user and assistant", func(t *testing.T) {
		tmpDir := t.TempDir()

		transcriptContent := `{"type": "user", "isMeta": false, "message": {"content": "What is 2+2?"}}
{"type": "assistant", "isMeta": false, "message": {"content": "2+2 equals 4."}}`
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

		transcriptContent := `{"type": "user", "isMeta": false, "message": {"content": "Hello?"}}`
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

		transcriptFile := tmpDir + "/transcript.jsonl"
		err := os.WriteFile(transcriptFile, []byte(""), 0644)
		require.NoError(t, err)

		prompt, response, err := ExtractLatestInteraction(transcriptFile)
		assert.Error(t, err)
		assert.Empty(t, prompt)
		assert.Empty(t, response)
	})

	t.Run("compact summary should not be returned as prompt", func(t *testing.T) {
		tmpDir := t.TempDir()

		transcriptContent := `{"type":"user","isSidechain":false,"isCompactSummary":true,"sessionId":"s1","message":{"role":"user","content":"This session is being continued from a previous conversation"}}
{"type":"assistant","isSidechain":false,"sessionId":"s1","message":{"id":"m1","type":"message","role":"assistant","content":[{"type":"text","text":"I understand the context."}],"stop_reason":"end_turn"}}`
		transcriptFile := tmpDir + "/transcript.jsonl"
		err := os.WriteFile(transcriptFile, []byte(transcriptContent), 0644)
		require.NoError(t, err)

		prompt, _, err := ExtractLatestInteraction(transcriptFile)
		// Only a compact summary, no real user message
		assert.Error(t, err)
		assert.Empty(t, prompt)
	})

	t.Run("new format extract skips compact summary", func(t *testing.T) {
		tmpDir := t.TempDir()

		transcriptContent := `{"type":"user","isSidechain":false,"sessionId":"s1","message":{"role":"user","content":"First question"}}
{"type":"assistant","isSidechain":false,"sessionId":"s1","message":{"id":"m1","type":"message","role":"assistant","content":[{"type":"text","text":"First answer"}],"stop_reason":"end_turn"}}
{"type":"user","isSidechain":false,"isCompactSummary":true,"sessionId":"s1","message":{"role":"user","content":"This session is being continued"}}
{"type":"user","isSidechain":false,"sessionId":"s1","message":{"role":"user","content":"Second question"}}
{"type":"assistant","isSidechain":false,"sessionId":"s1","message":{"id":"m2","type":"message","role":"assistant","content":[{"type":"text","text":"Second answer"}],"stop_reason":"end_turn"}}`
		transcriptFile := tmpDir + "/transcript.jsonl"
		err := os.WriteFile(transcriptFile, []byte(transcriptContent), 0644)
		require.NoError(t, err)

		prompt, response, err := ExtractLatestInteraction(transcriptFile)
		require.NoError(t, err)
		assert.Equal(t, "Second question", prompt)
		assert.Equal(t, "Second answer", response)
	})
}

// TestExtractLatestInteraction_RealSessionLog tests against real Claude Code session logs
func TestExtractLatestInteraction_RealSessionLog(t *testing.T) {
	sessionDir := "/home/tiefeng/.claude/projects/-data-app-workspace-me-ai-switch"

	t.Run("latest session log", func(t *testing.T) {
		transcriptPath := sessionDir + "/b7a064d2-ed9e-4bdc-85f7-e8dd91b46ea7.jsonl"
		if _, err := os.Stat(transcriptPath); os.IsNotExist(err) {
			t.Skip("Skipping: real session log not found")
		}

		prompt, response, err := ExtractLatestInteraction(transcriptPath)
		require.NoError(t, err)
		assert.NotEmpty(t, prompt)
		assert.NotEmpty(t, response)
		// Verify the known last prompt from manual inspection
		assert.Contains(t, prompt, "docs")
		assert.Contains(t, prompt, "提交")
	})

	t.Run("second newest session log", func(t *testing.T) {
		transcriptPath := sessionDir + "/13bf0332-642a-4845-a4d5-8d547f377900.jsonl"
		if _, err := os.Stat(transcriptPath); os.IsNotExist(err) {
			t.Skip("Skipping: real session log not found")
		}

		prompt, response, err := ExtractLatestInteraction(transcriptPath)
		require.NoError(t, err)
		assert.NotEmpty(t, prompt)
		// Response may be empty if last message has no assistant reply yet
		t.Logf("Prompt: %q", prompt)
		t.Logf("Response length: %d", len(response))
	})

	t.Run("parse transcript correctly handles mixed formats", func(t *testing.T) {
		transcriptPath := sessionDir + "/b7a064d2-ed9e-4bdc-85f7-e8dd91b46ea7.jsonl"
		if _, err := os.Stat(transcriptPath); os.IsNotExist(err) {
			t.Skip("Skipping: real session log not found")
		}

		messages, err := parseTranscript(transcriptPath)
		require.NoError(t, err)
		assert.NotEmpty(t, messages)

		// Verify no compact summary passes isRealUserMessage
		for _, msg := range messages {
			if msg.IsCompactSummary {
				assert.False(t, isRealUserMessage(msg),
					"compact summary should not be a real user message")
			}
			if msg.IsSidechain {
				assert.False(t, isRealUserMessage(msg),
					"sidechain message should not be a real user message")
			}
		}

		// Count real user messages
		realCount := 0
		for _, msg := range messages {
			if isRealUserMessage(msg) {
				realCount++
			}
		}
		assert.Positive(t, realCount, "should have at least one real user message")
		t.Logf("Total messages: %d, Real user messages: %d", len(messages), realCount)
	})
}
