package feishu

import (
	"context"
	"fmt"

	"github.com/keepmind9/clibot/internal/logger"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	"github.com/sirupsen/logrus"
)

// messageSender abstracts SendMessage for testability (Bot already satisfies this).
type messageSender interface {
	SendMessage(channel, message string) error
}

// messageReplier abstracts the Feishu message reply API for testability.
type messageReplier interface {
	Reply(ctx context.Context, req *larkim.ReplyMessageReq, options ...larkcore.RequestOptionFunc) (*larkim.ReplyMessageResp, error)
}

// SendMessageWithReply sends a message that visually references a parent message.
// Uses the Feishu Reply API when a replyToMessageID is provided.
// Falls back to regular SendMessage on any error.
func (b *Bot) SendMessageWithReply(channel, message, replyToMessageID string) error {
	if replyToMessageID == "" {
		return b.SendMessage(channel, message)
	}

	b.mu.RLock()
	larkClient := b.larkClient
	ctx := b.ctx
	b.mu.RUnlock()

	if larkClient == nil {
		return fmt.Errorf("feishu client not initialized")
	}

	return sendMessageWithReply(ctx, larkClient.Im.Message, b, channel, message, replyToMessageID)
}

func sendMessageWithReply(ctx context.Context, replier messageReplier, sender messageSender, channel, message, replyToMessageID string) error {
	contentJSON := fmt.Sprintf(`{"text":"%s"}`, escapeJSONString(message))

	body := larkim.NewReplyMessageReqBodyBuilder().
		MsgType(larkim.MsgTypeText).
		Content(contentJSON).
		Build()

	req := larkim.NewReplyMessageReqBuilder().
		MessageId(replyToMessageID).
		Body(body).
		Build()

	resp, err := replier.Reply(ctx, req)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"message_id": replyToMessageID,
			"error":      err,
		}).Warn("feishu-reply-failed-falling-back-to-send")
		return sender.SendMessage(channel, message)
	}

	if resp == nil || !resp.Success() {
		fields := logrus.Fields{"message_id": replyToMessageID}
		if resp != nil {
			fields["code"] = resp.Code
			fields["msg"] = resp.Msg
		}
		logger.WithFields(fields).Warn("feishu-reply-api-error-falling-back-to-send")
		return sender.SendMessage(channel, message)
	}

	return nil
}
