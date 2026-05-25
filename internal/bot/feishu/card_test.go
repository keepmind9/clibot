package feishu

import (
	"context"
	"testing"

	"github.com/keepmind9/clibot/internal/bot"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkcardkit "github.com/larksuite/oapi-sdk-go/v3/service/cardkit/v1"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	"github.com/stretchr/testify/assert"
)

// mockCardAPI implements cardAPI for testing.
type mockCardAPI struct {
	createResp       *larkcardkit.CreateCardResp
	createErr        error
	createCalls      int
	batchUpdateResp  *larkcardkit.BatchUpdateCardResp
	batchUpdateErr   error
	batchUpdateCalls int
	settingsResp     *larkcardkit.SettingsCardResp
	settingsErr      error
	settingsCalls    int
}

func (m *mockCardAPI) Create(_ context.Context, _ *larkcardkit.CreateCardReq, _ ...larkcore.RequestOptionFunc) (*larkcardkit.CreateCardResp, error) {
	m.createCalls++
	return m.createResp, m.createErr
}

func (m *mockCardAPI) BatchUpdate(_ context.Context, _ *larkcardkit.BatchUpdateCardReq, _ ...larkcore.RequestOptionFunc) (*larkcardkit.BatchUpdateCardResp, error) {
	m.batchUpdateCalls++
	return m.batchUpdateResp, m.batchUpdateErr
}

func (m *mockCardAPI) Settings(_ context.Context, _ *larkcardkit.SettingsCardReq, _ ...larkcore.RequestOptionFunc) (*larkcardkit.SettingsCardResp, error) {
	m.settingsCalls++
	return m.settingsResp, m.settingsErr
}

// mockMsgCreateReply implements message create and reply for testing.
type mockMsgCreateReply struct {
	createResp  *larkim.CreateMessageResp
	replyResp   *larkim.ReplyMessageResp
	createErr   error
	replyErr    error
	createCalls int
	replyCalls  int
}

func (m *mockMsgCreateReply) Create(_ context.Context, _ *larkim.CreateMessageReq, _ ...larkcore.RequestOptionFunc) (*larkim.CreateMessageResp, error) {
	m.createCalls++
	return m.createResp, m.createErr
}

func (m *mockMsgCreateReply) Reply(_ context.Context, _ *larkim.ReplyMessageReq, _ ...larkcore.RequestOptionFunc) (*larkim.ReplyMessageResp, error) {
	m.replyCalls++
	return m.replyResp, m.replyErr
}

func makeCreateCardResp(cardID string) *larkcardkit.CreateCardResp {
	r := &larkcardkit.CreateCardResp{}
	id := cardID
	r.Data = &larkcardkit.CreateCardRespData{CardId: &id}
	return r
}

func makeBatchUpdateResp() *larkcardkit.BatchUpdateCardResp {
	return &larkcardkit.BatchUpdateCardResp{}
}

func makeSettingsResp() *larkcardkit.SettingsCardResp {
	return &larkcardkit.SettingsCardResp{}
}

func TestCreateRichMessage_Success(t *testing.T) {
	api := &mockCardAPI{createResp: makeCreateCardResp("card_123")}
	sender := &mockMsgCreateReply{createResp: &larkim.CreateMessageResp{}}

	handle, err := createRichMessage(context.Background(), api, sender, "chat_001", bot.RichMessageOptions{
		Title: "Test Card",
	})
	assert.NoError(t, err)
	assert.NotNil(t, handle)
	assert.Equal(t, "chat_001", handle.Channel())
}

func TestCreateRichMessage_WithReply(t *testing.T) {
	api := &mockCardAPI{createResp: makeCreateCardResp("card_456")}
	sender := &mockMsgCreateReply{
		replyResp:  &larkim.ReplyMessageResp{},
		createResp: &larkim.CreateMessageResp{},
	}

	handle, err := createRichMessage(context.Background(), api, sender, "chat_002", bot.RichMessageOptions{
		ReplyToID: "msg_reply_target",
	})
	assert.NoError(t, err)
	assert.NotNil(t, handle)
	assert.Equal(t, 1, sender.replyCalls)
	assert.Equal(t, 0, sender.createCalls)
}

func TestCreateRichMessage_ReplyFallsBackToCreate(t *testing.T) {
	api := &mockCardAPI{createResp: makeCreateCardResp("card_789")}
	sender := &mockMsgCreateReply{
		replyErr:   assert.AnError,
		createResp: &larkim.CreateMessageResp{},
	}

	handle, err := createRichMessage(context.Background(), api, sender, "chat_003", bot.RichMessageOptions{
		ReplyToID: "msg_will_fail",
	})
	assert.NoError(t, err)
	assert.NotNil(t, handle)
	assert.Equal(t, 1, sender.replyCalls)
	assert.Equal(t, 1, sender.createCalls)
}

func TestCreateRichMessage_CardCreateError(t *testing.T) {
	api := &mockCardAPI{createErr: assert.AnError}
	sender := &mockMsgCreateReply{}

	_, err := createRichMessage(context.Background(), api, sender, "chat_004", bot.RichMessageOptions{})
	assert.Error(t, err)
}

func TestCreateRichMessage_CardCreateApiError(t *testing.T) {
	r := &larkcardkit.CreateCardResp{}
	r.Code = 999
	r.Msg = "forbidden"
	api := &mockCardAPI{createResp: r}
	sender := &mockMsgCreateReply{}

	_, err := createRichMessage(context.Background(), api, sender, "chat_005", bot.RichMessageOptions{})
	assert.Error(t, err)
}

func TestCardHandle_UpdateAndFinish(t *testing.T) {
	api := &mockCardAPI{
		batchUpdateResp: makeBatchUpdateResp(),
		settingsResp:    makeSettingsResp(),
	}

	handle := &cardHandle{
		api:     api,
		ctx:     context.Background(),
		cardID:  "card_test",
		channel: "chat_test",
	}

	assert.Equal(t, "chat_test", handle.Channel())

	err := handle.Update([]bot.ContentBlock{
		{Type: bot.ContentBlockText, Content: "Hello"},
	})
	assert.NoError(t, err)
	assert.Equal(t, 1, api.batchUpdateCalls)

	// Double finish should be ok
	assert.NoError(t, handle.Finish(nil))
	assert.Equal(t, 1, api.settingsCalls)
	assert.NoError(t, handle.Finish(nil))
	assert.Equal(t, 1, api.settingsCalls) // second call is no-op
}

func TestBuildSkeletonCard(t *testing.T) {
	card := buildSkeletonCard("Hello", "Cancel")
	assert.Contains(t, card, `"content": "Hello"`)
	assert.Contains(t, card, `"content": "Cancel"`)
	assert.Contains(t, card, `"streaming_mode": true`)
	assert.Contains(t, card, `"element_id": "main_content"`)
}

func TestToolSummary(t *testing.T) {
	tests := []struct {
		name string
		meta map[string]string
		want string
	}{
		{"Bash", map[string]string{"command": "ls -la"}, "ls -la"},
		{"Read", map[string]string{"file_path": "/tmp/test.go"}, "Read: /tmp/test.go"},
		{"Grep", map[string]string{"pattern": "TODO"}, "Grep: TODO"},
		{"Unknown", nil, "Unknown"},
		{"", nil, "Tool"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toolSummary(tt.name, tt.meta)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTruncateString(t *testing.T) {
	assert.Equal(t, "hello", truncateString("hello", 10))
	assert.Equal(t, "hel...", truncateString("hello world", 6))
}

func TestSanitizeID(t *testing.T) {
	assert.Equal(t, "abc123", sanitizeID("abc123"))
	assert.Equal(t, "a-b_c", sanitizeID("a-b_c"))
	assert.Equal(t, "", sanitizeID("!@#$%"))
}
