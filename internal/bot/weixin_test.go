package bot

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildAuthHeaders(t *testing.T) {
	headers := buildAuthHeaders("test-token")
	assert.Equal(t, "application/json", headers["Content-Type"])
	assert.Equal(t, "ilink_bot_token", headers["AuthorizationType"])
	assert.Equal(t, "Bearer test-token", headers["Authorization"])
	assert.NotEmpty(t, headers["X-WECHAT-UIN"])
}

func TestExtractInboundText(t *testing.T) {
	textPtr := func(s string) *inboundTextItem { return &inboundTextItem{Text: s} }
	urlPtr := func(s string) *inboundMediaItem { return &inboundMediaItem{ImageURL: s} }

	tests := []struct {
		name     string
		items    []inboundMessageItem
		expected string
	}{
		{
			name:     "text message",
			items:    []inboundMessageItem{{Type: MessageItemTypeText, TextItem: textPtr("hello world")}},
			expected: "hello world",
		},
		{
			name:     "image message",
			items:    []inboundMessageItem{{Type: MessageItemTypeImage, ImageItem: urlPtr("https://example.com/img.png")}},
			expected: "[image]",
		},
		{
			name:     "voice message",
			items:    []inboundMessageItem{{Type: MessageItemTypeVoice, VoiceItem: urlPtr("https://example.com/voice.ogg")}},
			expected: "[voice]",
		},
		{
			name:     "file message",
			items:    []inboundMessageItem{{Type: MessageItemTypeFile, FileItem: urlPtr("https://example.com/doc.pdf")}},
			expected: "[file]",
		},
		{
			name:     "video message",
			items:    []inboundMessageItem{{Type: MessageItemTypeVideo, VideoItem: urlPtr("https://example.com/vid.mp4")}},
			expected: "[video]",
		},
		{
			name:     "mixed message",
			items:    []inboundMessageItem{{Type: MessageItemTypeText, TextItem: textPtr("hello")}, {Type: MessageItemTypeImage, ImageItem: urlPtr("https://example.com/img.png")}, {Type: MessageItemTypeText, TextItem: textPtr("world")}},
			expected: "hello[image]world",
		},
		{
			name:     "empty text",
			items:    []inboundMessageItem{{Type: MessageItemTypeText, TextItem: textPtr("")}},
			expected: "",
		},
		{
			name:     "nil text pointer",
			items:    []inboundMessageItem{{Type: MessageItemTypeText, TextItem: nil}},
			expected: "",
		},
		{
			name:     "unknown type",
			items:    []inboundMessageItem{{Type: 99}},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractInboundText(tt.items)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFilterInboundMessages(t *testing.T) {
	tests := []struct {
		name         string
		messageType  int
		messageState int
		shouldPass   bool
	}{
		{name: "user message new", messageType: MessageTypeUser, messageState: MessageStateNew, shouldPass: true},
		{name: "user message finished", messageType: MessageTypeUser, messageState: MessageStateFinish, shouldPass: true},
		{name: "user message generating", messageType: MessageTypeUser, messageState: MessageStateGenerating, shouldPass: false},
		{name: "bot message", messageType: MessageTypeBot, messageState: MessageStateFinish, shouldPass: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := inboundMessage{
				MessageType:  tt.messageType,
				MessageState: tt.messageState,
				FromUserID:   "user123",
				ItemList:     []inboundMessageItem{{Type: MessageItemTypeText, TextItem: &inboundTextItem{Text: "hello"}}},
			}
			pass := msg.MessageType == MessageTypeUser && msg.MessageState != MessageStateGenerating
			assert.Equal(t, tt.shouldPass, pass)
		})
	}
}

func TestChunkMessage(t *testing.T) {
	tests := []struct {
		name     string
		msg      string
		maxLen   int
		expected []string
	}{
		{name: "short message", msg: "hello", maxLen: 2000, expected: []string{"hello"}},
		{name: "exact boundary", msg: strings.Repeat("a", 2000), maxLen: 2000, expected: []string{strings.Repeat("a", 2000)}},
		{name: "two chunks", msg: strings.Repeat("b", 4000), maxLen: 2000, expected: []string{strings.Repeat("b", 2000), strings.Repeat("b", 2000)}},
		{name: "three chunks remainder", msg: strings.Repeat("c", 4500), maxLen: 2000, expected: []string{strings.Repeat("c", 2000), strings.Repeat("c", 2000), strings.Repeat("c", 500)}},
		{name: "empty message", msg: "", maxLen: 2000, expected: []string{""}},
		{name: "single char", msg: "x", maxLen: 2000, expected: []string{"x"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := chunkMessage(tt.msg, tt.maxLen)
			assert.Equal(t, tt.expected, result)
			assert.Equal(t, tt.msg, strings.Join(result, ""))
		})
	}
}

func TestCredentialsRoundtrip(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "credentials.json")

	creds := &Credentials{
		Token:     "test-token-123",
		BaseURL:   "https://ilinkai.weixin.qq.com",
		AccountID: "bot_account_id",
		UserID:    "user_abc",
	}

	err := saveCredentials(path, creds)
	require.NoError(t, err)

	loaded, err := loadCredentials(path)
	require.NoError(t, err)
	assert.Equal(t, creds.Token, loaded.Token)
	assert.Equal(t, creds.BaseURL, loaded.BaseURL)
	assert.Equal(t, creds.AccountID, loaded.AccountID)
	assert.Equal(t, creds.UserID, loaded.UserID)

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm())
}

func TestCredentialsSnakeCase(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "snake.json")

	snakeJSON := `{"token":"snake-token","base_url":"https://snake.example.com","account_id":"snake_account","user_id":"snake_user"}`
	err := os.WriteFile(path, []byte(snakeJSON), 0600)
	require.NoError(t, err)

	loaded, err := loadCredentials(path)
	require.NoError(t, err)
	assert.Equal(t, "snake-token", loaded.Token)
	assert.Equal(t, "https://snake.example.com", loaded.BaseURL)
}

func TestCredentialsCamelCase(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "camel.json")

	camelJSON := `{"token":"camel-token","baseUrl":"https://camel.example.com","accountId":"camel_account","userId":"camel_user"}`
	err := os.WriteFile(path, []byte(camelJSON), 0600)
	require.NoError(t, err)

	loaded, err := loadCredentials(path)
	require.NoError(t, err)
	assert.Equal(t, "camel-token", loaded.Token)
	assert.Equal(t, "https://camel.example.com", loaded.BaseURL)
}

func TestCredentialsClear(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "credentials.json")

	err := saveCredentials(path, &Credentials{Token: "token"})
	require.NoError(t, err)
	err = clearCredentials(path)
	require.NoError(t, err)
	_, err = os.Stat(path)
	assert.True(t, os.IsNotExist(err))

	// clearCredentials should not error if file doesn't exist
	err = clearCredentials(path)
	require.NoError(t, err)
}

func TestLoadCredentialsNotFound(t *testing.T) {
	_, err := loadCredentials("/nonexistent/path/credentials.json")
	assert.Error(t, err)
}

func TestLoadCredentialsMissingToken(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "bad.json")

	err := os.WriteFile(path, []byte(`{"base_url":"https://example.com"}`), 0600)
	require.NoError(t, err)

	_, err = loadCredentials(path)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing token")
}

func TestApiErrorSessionExpired(t *testing.T) {
	tests := []struct {
		name     string
		code     int
		expected bool
	}{
		{name: "session expired", code: -14, expected: true},
		{name: "other error", code: -1, expected: false},
		{name: "success", code: 0, expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &ApiError{Code: tt.code, Message: "test"}
			assert.Equal(t, tt.expected, err.IsSessionExpired())
		})
	}
}

func TestApiErrorError(t *testing.T) {
	err := &ApiError{Status: 500, Code: -14, Message: "session expired"}
	msg := err.Error()
	assert.Contains(t, msg, "status=500")
	assert.Contains(t, msg, "code=-14")
}

func TestDefaultCredentialsPath(t *testing.T) {
	path := DefaultCredentialsPath()
	assert.Contains(t, path, ".clibot")
	assert.Contains(t, path, "weixin")
	assert.Contains(t, path, "credentials.json")
}

func TestNewWeixinBot(t *testing.T) {
	bot := NewWeixinBot("https://custom.example.com", "/custom/path.json")
	assert.NotNil(t, bot)
	assert.Equal(t, "https://custom.example.com", bot.baseURL)
	assert.Equal(t, "/custom/path.json", bot.credentialsPath)
}

func TestNewWeixinBotDefaults(t *testing.T) {
	bot := NewWeixinBot("", "")
	assert.NotNil(t, bot)
	assert.Equal(t, DefaultBaseURL, bot.baseURL)
	assert.NotEmpty(t, bot.credentialsPath)
}

func TestWeixinBotImplementsBotAdapter(t *testing.T) {
	var bot BotAdapter = NewWeixinBot("", "")
	assert.NotNil(t, bot)
}

func TestGetUpdatesRequestJSON(t *testing.T) {
	req := weixinGetUpdatesRequest{
		SyncBuf:  "cursor123",
		BaseInfo: weixinBaseInfo{ChannelVersion: "1.0.0"},
	}
	data, err := json.Marshal(req)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"sync_buf":"cursor123"`)
	assert.Contains(t, string(data), `"base_info"`)
	assert.Contains(t, string(data), `"channel_version":"1.0.0"`)
}

func TestSendMessageBodyJSON(t *testing.T) {
	body := weixinSendMessageBody{
		Msg: weixinOutboundMsg{
			FromUserID:   "",
			ToUserID:     "user_abc",
			ClientID:     "client_123",
			MessageType:  MessageTypeBot,
			MessageState: MessageStateFinish,
			ContextToken: "token_xyz",
			ItemList: []weixinOutboundItem{
				{Type: MessageItemTypeText, TextItem: &outboundTextItem{Text: "hello"}},
			},
		},
		BaseInfo: weixinBaseInfo{ChannelVersion: "1.0.0"},
	}
	data, err := json.Marshal(body)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"to_user_id":"user_abc"`)
	assert.Contains(t, string(data), `"message_type":2`)
	assert.Contains(t, string(data), `"message_state":2`)
	assert.Contains(t, string(data), `"context_token":"token_xyz"`)
	assert.Contains(t, string(data), `"type":1`)
	assert.Contains(t, string(data), `"text_item"`)
	assert.Contains(t, string(data), `"text":"hello"`)
	assert.Contains(t, string(data), `"base_info"`)
}
