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
	createResp  *larkcardkit.CreateCardResp
	createErr   error
	createCalls int
	updateResp  *larkcardkit.UpdateCardResp
	updateErr   error
	updateCalls int
}

func (m *mockCardAPI) Create(_ context.Context, _ *larkcardkit.CreateCardReq, _ ...larkcore.RequestOptionFunc) (*larkcardkit.CreateCardResp, error) {
	m.createCalls++
	return m.createResp, m.createErr
}

func (m *mockCardAPI) Update(_ context.Context, _ *larkcardkit.UpdateCardReq, _ ...larkcore.RequestOptionFunc) (*larkcardkit.UpdateCardResp, error) {
	m.updateCalls++
	return m.updateResp, m.updateErr
}

// mockCardElementAPI implements cardElementContentUpdater for testing.
type mockCardElementAPI struct {
	contentResp  *larkcardkit.ContentCardElementResp
	contentErr   error
	contentCalls int
}

func (m *mockCardElementAPI) Content(_ context.Context, _ *larkcardkit.ContentCardElementReq, _ ...larkcore.RequestOptionFunc) (*larkcardkit.ContentCardElementResp, error) {
	m.contentCalls++
	return m.contentResp, m.contentErr
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

func makeContentResp() *larkcardkit.ContentCardElementResp {
	return &larkcardkit.ContentCardElementResp{}
}

func TestCreateRichMessage_Success(t *testing.T) {
	api := &mockCardAPI{createResp: makeCreateCardResp("card_123")}
	elemAPI := &mockCardElementAPI{contentResp: makeContentResp()}
	sender := &mockMsgCreateReply{createResp: &larkim.CreateMessageResp{}}

	handle, err := createRichMessage(context.Background(), api, elemAPI, sender, "chat_001", bot.RichMessageOptions{
		Title: "Test Card",
	})
	assert.NoError(t, err)
	assert.NotNil(t, handle)
	assert.Equal(t, "chat_001", handle.Channel())
}

func TestCreateRichMessage_WithReply(t *testing.T) {
	api := &mockCardAPI{createResp: makeCreateCardResp("card_456")}
	elemAPI := &mockCardElementAPI{contentResp: makeContentResp()}
	sender := &mockMsgCreateReply{
		replyResp:  &larkim.ReplyMessageResp{},
		createResp: &larkim.CreateMessageResp{},
	}

	handle, err := createRichMessage(context.Background(), api, elemAPI, sender, "chat_002", bot.RichMessageOptions{
		ReplyToID: "msg_reply_target",
	})
	assert.NoError(t, err)
	assert.NotNil(t, handle)
	assert.Equal(t, 1, sender.replyCalls)
	assert.Equal(t, 0, sender.createCalls)
}

func TestCreateRichMessage_ReplyFallsBackToCreate(t *testing.T) {
	api := &mockCardAPI{createResp: makeCreateCardResp("card_789")}
	elemAPI := &mockCardElementAPI{contentResp: makeContentResp()}
	sender := &mockMsgCreateReply{
		replyErr:   assert.AnError,
		createResp: &larkim.CreateMessageResp{},
	}

	handle, err := createRichMessage(context.Background(), api, elemAPI, sender, "chat_003", bot.RichMessageOptions{
		ReplyToID: "msg_will_fail",
	})
	assert.NoError(t, err)
	assert.NotNil(t, handle)
	assert.Equal(t, 1, sender.replyCalls)
	assert.Equal(t, 1, sender.createCalls)
}

func TestCreateRichMessage_CardCreateError(t *testing.T) {
	api := &mockCardAPI{createErr: assert.AnError}
	elemAPI := &mockCardElementAPI{contentResp: makeContentResp()}
	sender := &mockMsgCreateReply{}

	_, err := createRichMessage(context.Background(), api, elemAPI, sender, "chat_004", bot.RichMessageOptions{})
	assert.Error(t, err)
}

func TestCreateRichMessage_CardCreateApiError(t *testing.T) {
	r := &larkcardkit.CreateCardResp{}
	r.Code = 999
	r.Msg = "forbidden"
	api := &mockCardAPI{createResp: r}
	elemAPI := &mockCardElementAPI{contentResp: makeContentResp()}
	sender := &mockMsgCreateReply{}

	_, err := createRichMessage(context.Background(), api, elemAPI, sender, "chat_005", bot.RichMessageOptions{})
	assert.Error(t, err)
}

func TestCardHandle_UpdateAndFinish(t *testing.T) {
	api := &mockCardAPI{updateResp: &larkcardkit.UpdateCardResp{}}
	elemAPI := &mockCardElementAPI{contentResp: makeContentResp()}

	handle := &cardHandle{
		api:     api,
		elemAPI: elemAPI,
		ctx:     context.Background(),
		cardID:  "card_test",
		channel: "chat_test",
	}

	assert.Equal(t, "chat_test", handle.Channel())

	err := handle.Update([]bot.ContentBlock{
		{Type: bot.ContentBlockText, Content: "Hello"},
	})
	assert.NoError(t, err)
	assert.Equal(t, 1, elemAPI.contentCalls)

	// Double finish should be ok
	assert.NoError(t, handle.Finish(nil))
	assert.Equal(t, 1, api.updateCalls)
	assert.NoError(t, handle.Finish(nil))
	assert.Equal(t, 1, api.updateCalls) // second call is no-op
}

func TestBuildSkeletonCard(t *testing.T) {
	card := buildSkeletonCard("Cancel")
	assert.Contains(t, card, `"Processing..."`)
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
