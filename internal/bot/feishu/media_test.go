package feishu

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/keepmind9/clibot/internal/bot"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	"github.com/stretchr/testify/assert"
)

// mockMessageResourceGetter implements messageResourceGetter for testing.
type mockMessageResourceGetter struct {
	resp *larkim.GetMessageResourceResp
	err  error
}

func (m *mockMessageResourceGetter) Get(ctx context.Context, req *larkim.GetMessageResourceReq, options ...larkcore.RequestOptionFunc) (*larkim.GetMessageResourceResp, error) {
	return m.resp, m.err
}

func makeResourceResp(data []byte) *larkim.GetMessageResourceResp {
	r := &larkim.GetMessageResourceResp{}
	r.File = bytes.NewReader(data)
	return r
}

func TestParseMediaContent(t *testing.T) {
	tests := []struct {
		name        string
		messageType string
		content     string
		wantKey     string
		wantName    string
	}{
		{
			name:        "image message",
			messageType: "image",
			content:     `{"image_key":"img_abc123"}`,
			wantKey:     "img_abc123",
			wantName:    "",
		},
		{
			name:        "file message",
			messageType: "file",
			content:     `{"file_key":"file_xyz","file_name":"report.pdf","file_size":"1024"}`,
			wantKey:     "file_xyz",
			wantName:    "report.pdf",
		},
		{
			name:        "audio message",
			messageType: "audio",
			content:     `{"file_key":"aud_123","file_name":"voice.opus","file_size":"2048"}`,
			wantKey:     "aud_123",
			wantName:    "voice.opus",
		},
		{
			name:        "video message",
			messageType: "video",
			content:     `{"file_key":"vid_456","file_name":"clip.mp4","file_size":"4096"}`,
			wantKey:     "vid_456",
			wantName:    "clip.mp4",
		},
		{
			name:        "text message returns empty",
			messageType: "text",
			content:     `{"text":"hello"}`,
			wantKey:     "",
			wantName:    "",
		},
		{
			name:        "invalid JSON returns empty",
			messageType: "image",
			content:     "not json",
			wantKey:     "",
			wantName:    "",
		},
		{
			name:        "empty content returns empty",
			messageType: "image",
			content:     "",
			wantKey:     "",
			wantName:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fileKey, fileName := parseMediaContent(tt.messageType, tt.content)
			assert.Equal(t, tt.wantKey, fileKey)
			assert.Equal(t, tt.wantName, fileName)
		})
	}
}

func TestDownloadMedia_ImageSuccess(t *testing.T) {
	tmpDir := t.TempDir()
	getter := &mockMessageResourceGetter{
		resp: makeResourceResp([]byte("fake-image-data")),
	}

	imgContent, _ := json.Marshal(ImageContent{ImageKey: "img_test123"})
	msg := &bot.BotMessage{
		MessageType: "image",
		MessageID:   "msg_001",
		Channel:     "chat_001",
		Content:     string(imgContent),
	}

	err := downloadMedia(context.Background(), getter, msg, tmpDir, 0)
	assert.NoError(t, err)
	assert.Len(t, msg.Attachments, 1)
	assert.Equal(t, "image", msg.Attachments[0].Type)
	assert.Equal(t, "img_test123", msg.Attachments[0].FileKey)
	assert.Contains(t, msg.Attachments[0].FilePath, "img_test123.png")
	assert.Equal(t, "[image]", msg.Content)

	_, statErr := os.Stat(msg.Attachments[0].FilePath)
	assert.NoError(t, statErr)
}

func TestDownloadMedia_FileSuccess(t *testing.T) {
	tmpDir := t.TempDir()
	getter := &mockMessageResourceGetter{
		resp: makeResourceResp([]byte("file-content")),
	}

	msg := &bot.BotMessage{
		MessageType: "file",
		MessageID:   "msg_002",
		Channel:     "chat_002",
		Content:     `{"file_key":"file_abc","file_name":"report.pdf","file_size":"100"}`,
	}

	err := downloadMedia(context.Background(), getter, msg, tmpDir, 0)
	assert.NoError(t, err)
	assert.Len(t, msg.Attachments, 1)
	assert.Equal(t, "file", msg.Attachments[0].Type)
	assert.Equal(t, "report.pdf", msg.Attachments[0].FileName)
	assert.Contains(t, msg.Content, "report.pdf")
}

func TestDownloadMedia_SkipsTextAndSticker(t *testing.T) {
	feishuBot := &Bot{}
	msg := &bot.BotMessage{MessageType: "text"}
	assert.NoError(t, feishuBot.DownloadMedia(context.Background(), msg))

	msg = &bot.BotMessage{MessageType: "sticker"}
	assert.NoError(t, feishuBot.DownloadMedia(context.Background(), msg))

	msg = &bot.BotMessage{MessageType: "post"}
	assert.NoError(t, feishuBot.DownloadMedia(context.Background(), msg))

	msg = &bot.BotMessage{MessageType: "interactive"}
	assert.NoError(t, feishuBot.DownloadMedia(context.Background(), msg))
}

func TestDownloadMedia_ApiError(t *testing.T) {
	tmpDir := t.TempDir()
	getter := &mockMessageResourceGetter{err: assert.AnError}

	msg := &bot.BotMessage{
		MessageType: "image",
		MessageID:   "msg_003",
		Channel:     "chat_003",
		Content:     `{"image_key":"img_err"}`,
	}

	err := downloadMedia(context.Background(), getter, msg, tmpDir, 0)
	assert.Error(t, err)
	assert.Empty(t, msg.Attachments)
}

func TestDownloadMedia_ApiUnsuccessful(t *testing.T) {
	tmpDir := t.TempDir()
	r := &larkim.GetMessageResourceResp{}
	r.Code = 999
	r.Msg = "forbidden"
	getter := &mockMessageResourceGetter{resp: r}

	msg := &bot.BotMessage{
		MessageType: "image",
		MessageID:   "msg_004",
		Channel:     "chat_004",
		Content:     `{"image_key":"img_fail"}`,
	}

	err := downloadMedia(context.Background(), getter, msg, tmpDir, 0)
	assert.Error(t, err)
}

func TestDownloadMedia_ExceedsSizeLimit(t *testing.T) {
	tmpDir := t.TempDir()
	getter := &mockMessageResourceGetter{
		resp: makeResourceResp([]byte("this is a large file content that exceeds limit")),
	}

	msg := &bot.BotMessage{
		MessageType: "file",
		MessageID:   "msg_005",
		Channel:     "chat_005",
		Content:     `{"file_key":"file_big","file_name":"big.zip","file_size":"100"}`,
	}

	err := downloadMedia(context.Background(), getter, msg, tmpDir, 10)
	assert.Error(t, err)
}

func TestDownloadMedia_NilResponse(t *testing.T) {
	tmpDir := t.TempDir()
	getter := &mockMessageResourceGetter{resp: nil}

	msg := &bot.BotMessage{
		MessageType: "image",
		MessageID:   "msg_006",
		Channel:     "chat_006",
		Content:     `{"image_key":"img_nil"}`,
	}

	err := downloadMedia(context.Background(), getter, msg, tmpDir, 0)
	assert.Error(t, err)
}

func TestCleanExpiredMedia(t *testing.T) {
	tmpDir := t.TempDir()

	oldFile := filepath.Join(tmpDir, "old.txt")
	assert.NoError(t, os.WriteFile(oldFile, []byte("old"), 0o644))

	oldTime := time.Now().Add(-2 * time.Hour)
	assert.NoError(t, os.Chtimes(oldFile, oldTime, oldTime))

	recentFile := filepath.Join(tmpDir, "recent.txt")
	assert.NoError(t, os.WriteFile(recentFile, []byte("recent"), 0o644))

	cleanExpiredMedia(tmpDir, time.Hour)

	_, err := os.Stat(oldFile)
	assert.True(t, os.IsNotExist(err))
	_, err = os.Stat(recentFile)
	assert.NoError(t, err)
}
