package feishu

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/keepmind9/clibot/internal/bot"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

// messageGetter abstracts the Feishu message API for testability.
// The lark SDK's Im.Message satisfies this interface.
type messageGetter interface {
	Get(ctx context.Context, req *larkim.GetMessageReq, options ...larkcore.RequestOptionFunc) (*larkim.GetMessageResp, error)
	Create(ctx context.Context, req *larkim.CreateMessageReq, options ...larkcore.RequestOptionFunc) (*larkim.CreateMessageResp, error)
}

// FetchQuotedMessage fetches a referenced message by its ID using the Feishu API.
// Returns nil if messageID is empty.
func (b *Bot) FetchQuotedMessage(ctx context.Context, channelID, messageID string) (*bot.QuotedMessage, error) {
	if messageID == "" {
		return nil, nil
	}

	b.mu.RLock()
	larkClient := b.larkClient
	b.mu.RUnlock()

	if larkClient == nil {
		return nil, fmt.Errorf("feishu client not initialized")
	}

	return fetchQuotedMessage(ctx, larkClient.Im.Message, messageID)
}

// fetchQuotedMessage is extracted for testability with a mockable messageGetter.
func fetchQuotedMessage(ctx context.Context, getter messageGetter, messageID string) (*bot.QuotedMessage, error) {
	req := larkim.NewGetMessageReqBuilder().
		MessageId(messageID).
		Build()

	resp, err := getter.Get(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch message %s: %w", messageID, err)
	}

	if !resp.Success() {
		return nil, fmt.Errorf("API error fetching message %s: code=%d, msg=%s", messageID, resp.Code, resp.Msg)
	}

	if resp.Data == nil || len(resp.Data.Items) == 0 || resp.Data.Items[0] == nil {
		return nil, fmt.Errorf("no message data returned for %s", messageID)
	}

	msg := resp.Data.Items[0]

	var senderID, content string
	var timestamp time.Time

	if msg.Sender != nil && msg.Sender.Id != nil {
		senderID = *msg.Sender.Id
	}

	if msg.Body != nil && msg.Body.Content != nil {
		content = extractTextContent(*msg.Body.Content)
	}

	if msg.CreateTime != nil {
		if ms, err := strconv.ParseInt(*msg.CreateTime, 10, 64); err == nil {
			timestamp = time.UnixMilli(ms)
		}
	}

	if timestamp.IsZero() {
		timestamp = time.Now()
	}

	return &bot.QuotedMessage{
		SenderID:  senderID,
		Content:   content,
		Timestamp: timestamp,
	}, nil
}
