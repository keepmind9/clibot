package feishu

import (
	"context"
	"errors"
	"testing"

	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	"github.com/stretchr/testify/assert"
)

type mockMessageReplier struct {
	resp *larkim.ReplyMessageResp
	err  error
}

func (m *mockMessageReplier) Reply(_ context.Context, _ *larkim.ReplyMessageReq, _ ...larkcore.RequestOptionFunc) (*larkim.ReplyMessageResp, error) {
	return m.resp, m.err
}

type mockMessageSender struct {
	err error
}

func (m *mockMessageSender) SendMessage(_, _ string) error {
	return m.err
}

func TestSendMessageWithReply(t *testing.T) {
	tests := []struct {
		name         string
		replier      *mockMessageReplier
		sender       *mockMessageSender
		replyToID    string
		wantErr      bool
		wantSendCall bool
	}{
		{
			name:         "empty replyToID delegates to sender",
			replier:      &mockMessageReplier{},
			sender:       &mockMessageSender{},
			replyToID:    "",
			wantSendCall: true,
		},
		{
			name: "successful reply",
			replier: &mockMessageReplier{
				resp: &larkim.ReplyMessageResp{},
			},
			sender:    &mockMessageSender{},
			replyToID: "msg_001",
		},
		{
			name: "reply API error falls back to send",
			replier: &mockMessageReplier{
				err: errors.New("network timeout"),
			},
			sender:       &mockMessageSender{},
			replyToID:    "msg_002",
			wantSendCall: true,
		},
		{
			name: "reply non-success falls back to send",
			replier: &mockMessageReplier{
				resp: func() *larkim.ReplyMessageResp {
					r := &larkim.ReplyMessageResp{}
					r.Code = 230001
					r.Msg = "permission denied"
					return r
				}(),
			},
			sender:       &mockMessageSender{},
			replyToID:    "msg_003",
			wantSendCall: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := sendMessageWithReply(context.Background(), tt.replier, tt.sender, "chat_001", "hello", tt.replyToID)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestBot_SendMessageWithReply_NoClient(t *testing.T) {
	b := NewBot("test-app", "test-secret")

	err := b.SendMessageWithReply("chat_001", "hello", "msg_reply_target")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")

	// Empty replyToID should fall through to SendMessage (also fails without client)
	err = b.SendMessageWithReply("chat_001", "hello", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}
