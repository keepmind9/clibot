package feishu

import (
	"context"
	"testing"

	"github.com/keepmind9/clibot/internal/bot"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	"github.com/stretchr/testify/assert"
)

// mockMessageGetter implements messageGetter for testing
type mockMessageGetter struct {
	getResp   *larkim.GetMessageResp
	getErr    error
	createErr error
}

func (m *mockMessageGetter) Get(_ context.Context, _ *larkim.GetMessageReq, _ ...larkcore.RequestOptionFunc) (*larkim.GetMessageResp, error) {
	return m.getResp, m.getErr
}

func (m *mockMessageGetter) Create(_ context.Context, _ *larkim.CreateMessageReq, _ ...larkcore.RequestOptionFunc) (*larkim.CreateMessageResp, error) {
	return nil, m.createErr
}

func TestBot_FetchQuotedMessage_EmptyMessageID(t *testing.T) {
	b := NewBot("test-app", "test-secret")
	result, err := b.FetchQuotedMessage(context.Background(), "chat_123", "")
	assert.NoError(t, err)
	assert.Nil(t, result)
}

func TestBot_FetchQuotedMessage_NoClient(t *testing.T) {
	b := NewBot("test-app", "test-secret")
	result, err := b.FetchQuotedMessage(context.Background(), "chat_123", "msg_123")
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestFetchQuotedMessage(t *testing.T) {
	tests := []struct {
		name        string
		messageID   string
		getter      *mockMessageGetter
		expectError bool
		expectQuote *bot.QuotedMessage
	}{
		{
			name:      "successful fetch",
			messageID: "msg_123",
			getter: func() *mockMessageGetter {
				senderID := "user_001"
				content := `{"text":"quoted text"}`
				createTime := "1700000000000"
				return &mockMessageGetter{
					getResp: &larkim.GetMessageResp{
						Data: &larkim.GetMessageRespData{
							Items: []*larkim.Message{
								{
									Sender: &larkim.Sender{
										Id: &senderID,
									},
									Body: &larkim.MessageBody{
										Content: &content,
									},
									CreateTime: &createTime,
								},
							},
						},
					},
				}
			}(),
			expectError: false,
			expectQuote: &bot.QuotedMessage{
				SenderID: "user_001",
				Content:  "quoted text",
			},
		},
		{
			name:      "API error response",
			messageID: "msg_456",
			getter: &mockMessageGetter{
				getResp: func() *larkim.GetMessageResp {
					resp := &larkim.GetMessageResp{}
					resp.Code = 999
					resp.Msg = "internal error"
					return resp
				}(),
			},
			expectError: true,
		},
		{
			name: "network error",
			getter: &mockMessageGetter{
				getErr: context.DeadlineExceeded,
			},
			messageID:   "msg_789",
			expectError: true,
		},
		{
			name:      "empty items in response",
			messageID: "msg_empty",
			getter: &mockMessageGetter{
				getResp: &larkim.GetMessageResp{
					Data: &larkim.GetMessageRespData{
						Items: []*larkim.Message{},
					},
				},
			},
			expectError: true,
		},
		{
			name:      "nil data in response",
			messageID: "msg_nil_data",
			getter: &mockMessageGetter{
				getResp: &larkim.GetMessageResp{},
			},
			expectError: true,
		},
		{
			name:      "message without create time uses now",
			messageID: "msg_no_time",
			getter: func() *mockMessageGetter {
				senderID := "user_002"
				content := `{"text":"no timestamp"}`
				return &mockMessageGetter{
					getResp: &larkim.GetMessageResp{
						Data: &larkim.GetMessageRespData{
							Items: []*larkim.Message{
								{
									Sender: &larkim.Sender{Id: &senderID},
									Body:   &larkim.MessageBody{Content: &content},
								},
							},
						},
					},
				}
			}(),
			expectError: false,
			expectQuote: &bot.QuotedMessage{
				SenderID: "user_002",
				Content:  "no timestamp",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := fetchQuotedMessage(context.Background(), tt.getter, tt.messageID)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				if tt.expectQuote != nil {
					assert.Equal(t, tt.expectQuote.SenderID, result.SenderID)
					assert.Equal(t, tt.expectQuote.Content, result.Content)
					assert.False(t, result.Timestamp.IsZero())
				}
			}
		})
	}
}
