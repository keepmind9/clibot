package feishu

import (
	"context"
	"fmt"
	"sync"

	"github.com/keepmind9/clibot/internal/bot"
	"github.com/keepmind9/clibot/internal/logger"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkcardkit "github.com/larksuite/oapi-sdk-go/v3/service/cardkit/v1"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	"github.com/sirupsen/logrus"
)

// cardCreator abstracts Cardkit.V1.Card.Create for testability.
type cardCreator interface {
	Create(ctx context.Context, req *larkcardkit.CreateCardReq, opts ...larkcore.RequestOptionFunc) (*larkcardkit.CreateCardResp, error)
}

// cardElementContentUpdater abstracts Cardkit.V1.CardElement.Content for testability.
type cardElementContentUpdater interface {
	Content(ctx context.Context, req *larkcardkit.ContentCardElementReq, opts ...larkcore.RequestOptionFunc) (*larkcardkit.ContentCardElementResp, error)
}

// cardUpdater abstracts Cardkit.V1.Card.Update for testability.
type cardUpdater interface {
	Update(ctx context.Context, req *larkcardkit.UpdateCardReq, opts ...larkcore.RequestOptionFunc) (*larkcardkit.UpdateCardResp, error)
}

// cardAPI combines card creation, update and settings for testability.
type cardAPI interface {
	cardCreator
	cardUpdater
}

// cardHandle implements bot.RichMessageHandle for a Feishu CardKit card.
type cardHandle struct {
	api      cardAPI
	elemAPI  cardElementContentUpdater
	cardID   string
	channel  string
	ctx      context.Context
	seq      int
	mu       sync.Mutex
	finished bool
}

func (h *cardHandle) Channel() string { return h.channel }

func (h *cardHandle) Update(blocks []bot.ContentBlock) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.finished {
		return fmt.Errorf("card already finished")
	}

	if len(blocks) == 0 {
		return nil
	}

	content := renderBlocksToMarkdown(blocks)

	h.seq++
	seq := h.seq

	req := larkcardkit.NewContentCardElementReqBuilder().
		CardId(h.cardID).
		ElementId("main_content").
		Body(larkcardkit.NewContentCardElementReqBodyBuilder().
			Content(content).
			Sequence(seq).
			Build()).
		Build()

	logger.WithFields(logrus.Fields{
		"card_id":     h.cardID,
		"seq":         seq,
		"content_len": len(content),
	}).Debug("feishu-card-element-content-request")

	resp, err := h.elemAPI.Content(h.ctx, req)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"card_id": h.cardID,
			"seq":     seq,
			"error":   err,
		}).Warn("feishu-card-element-content-error")
		return err
	}
	if !resp.Success() {
		logger.WithFields(logrus.Fields{
			"card_id": h.cardID,
			"seq":     seq,
			"code":    resp.Code,
			"msg":     resp.Msg,
		}).Warn("feishu-card-element-content-api-error")
		return fmt.Errorf("card element content API error: code=%d", resp.Code)
	}

	logger.WithFields(logrus.Fields{
		"card_id": h.cardID,
		"seq":     seq,
		"code":    resp.Code,
	}).Debug("feishu-card-element-content-success")

	return nil
}

func (h *cardHandle) Finish(blocks []bot.ContentBlock) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.finished {
		return nil
	}
	h.finished = true

	// Final content update via CardElement.Content
	if len(blocks) > 0 {
		content := renderBlocksToMarkdown(blocks)
		h.seq++

		req := larkcardkit.NewContentCardElementReqBuilder().
			CardId(h.cardID).
			ElementId("main_content").
			Body(larkcardkit.NewContentCardElementReqBodyBuilder().
				Content(content).
				Sequence(h.seq).
				Build()).
			Build()

		logger.WithFields(logrus.Fields{
			"card_id":     h.cardID,
			"seq":         h.seq,
			"content_len": len(content),
		}).Debug("feishu-card-finish-content-request")

		resp, err := h.elemAPI.Content(h.ctx, req)
		if err != nil {
			logger.WithFields(logrus.Fields{
				"card_id": h.cardID,
				"error":   err,
			}).Warn("feishu-card-finish-content-error")
		} else if !resp.Success() {
			logger.WithFields(logrus.Fields{
				"card_id": h.cardID,
				"code":    resp.Code,
				"msg":     resp.Msg,
			}).Warn("feishu-card-finish-content-api-error")
		} else {
			logger.WithFields(logrus.Fields{
				"card_id": h.cardID,
				"seq":     h.seq,
				"code":    resp.Code,
			}).Debug("feishu-card-finish-content-success")
		}
	} else {
		logger.WithField("card_id", h.cardID).Debug("feishu-card-finish-no-blocks")
	}

	// End streaming and remove header via Card.Update
	h.seq++

	// Include final content in the update to avoid overwriting CardElement.Content
	content := ""
	if len(blocks) > 0 {
		content = escapeJSONString(renderBlocksToMarkdown(blocks))
	}

	// No header in final card — just body with content and streaming off
	finalCardData := fmt.Sprintf(`{"schema":"2.0","body":{"elements":[{"tag":"markdown","element_id":"main_content","content":"%s"}]},"config":{"streaming_mode":false}}`, content)

	updateReq := larkcardkit.NewUpdateCardReqBuilder().
		CardId(h.cardID).
		Body(larkcardkit.NewUpdateCardReqBodyBuilder().
			Card(&larkcardkit.Card{Type: ptrString("card_json"), Data: ptrString(finalCardData)}).
			Sequence(h.seq).
			Build()).
		Build()

	if resp, err := h.api.Update(h.ctx, updateReq); err != nil {
		logger.WithFields(logrus.Fields{
			"card_id": h.cardID,
			"error":   err,
		}).Warn("feishu-card-update-error")
	} else if !resp.Success() {
		logger.WithFields(logrus.Fields{
			"card_id": h.cardID,
			"code":    resp.Code,
			"msg":     resp.Msg,
		}).Warn("feishu-card-update-api-error")
	}

	logger.WithField("card_id", h.cardID).Info("feishu-card-finished")
	return nil
}

// CreateRichMessage creates a CardKit card and sends it to the channel.
func (b *Bot) CreateRichMessage(channel string, opts bot.RichMessageOptions) (bot.RichMessageHandle, error) {
	b.mu.RLock()
	larkClient := b.larkClient
	botCtx := b.ctx
	b.mu.RUnlock()

	if larkClient == nil {
		return nil, fmt.Errorf("feishu client not initialized")
	}

	return createRichMessage(botCtx, larkClient.Cardkit.V1.Card, larkClient.Cardkit.V1.CardElement, larkClient.Im.Message, channel, opts)
}

func createRichMessage(
	ctx context.Context,
	api cardAPI,
	elemAPI cardElementContentUpdater,
	msgSender interface {
		Create(ctx context.Context, req *larkim.CreateMessageReq, opts ...larkcore.RequestOptionFunc) (*larkim.CreateMessageResp, error)
		Reply(ctx context.Context, req *larkim.ReplyMessageReq, opts ...larkcore.RequestOptionFunc) (*larkim.ReplyMessageResp, error)
	},
	channel string,
	opts bot.RichMessageOptions,
) (bot.RichMessageHandle, error) {
	cardData := buildSkeletonCard(opts.StopText)

	req := larkcardkit.NewCreateCardReqBuilder().
		Body(larkcardkit.NewCreateCardReqBodyBuilder().
			Type("card_json").
			Data(cardData).
			Build()).
		Build()

	resp, err := api.Create(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("create card: %w", err)
	}
	if !resp.Success() {
		return nil, fmt.Errorf("create card API error: code=%d, msg=%s", resp.Code, resp.Msg)
	}
	if resp.Data == nil || resp.Data.CardId == nil {
		return nil, fmt.Errorf("create card returned no card_id")
	}

	cardID := *resp.Data.CardId

	// Send card reference to chat — Feishu requires {"type":"card","data":{"card_id":"..."}} format
	contentJSON := fmt.Sprintf(`{"type":"card","data":{"card_id":"%s"}}`, cardID)

	if opts.ReplyToID != "" {
		body := larkim.NewReplyMessageReqBodyBuilder().
			MsgType("interactive").
			Content(contentJSON).
			Build()
		replyReq := larkim.NewReplyMessageReqBuilder().
			MessageId(opts.ReplyToID).
			Body(body).
			Build()
		if _, err := msgSender.Reply(ctx, replyReq); err != nil {
			logger.WithFields(logrus.Fields{
				"card_id": cardID,
				"error":   err,
			}).Warn("feishu-card-reply-failed-sending-as-new")
			// Fall back to creating a new message
			sendCardAsNewMessage(ctx, msgSender, channel, contentJSON)
		}
	} else {
		sendCardAsNewMessage(ctx, msgSender, channel, contentJSON)
	}

	logger.WithField("card_id", cardID).Info("feishu-card-created")

	return &cardHandle{
		api:     api,
		elemAPI: elemAPI,
		ctx:     ctx,
		cardID:  cardID,
		channel: channel,
	}, nil
}

func sendCardAsNewMessage(ctx context.Context, msgSender interface {
	Create(ctx context.Context, req *larkim.CreateMessageReq, opts ...larkcore.RequestOptionFunc) (*larkim.CreateMessageResp, error)
}, channel, contentJSON string) {
	body := larkim.NewCreateMessageReqBodyBuilder().
		ReceiveId(channel).
		MsgType("interactive").
		Content(contentJSON).
		Build()

	req := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType(larkim.ReceiveIdTypeChatId).
		Body(body).
		Build()

	if _, err := msgSender.Create(ctx, req); err != nil {
		logger.WithFields(logrus.Fields{
			"channel": channel,
			"error":   err,
		}).Warn("feishu-card-send-message-error")
	}
}

// buildSkeletonCard creates the initial card JSON with streaming mode enabled.
func buildSkeletonCard(stopText string) string {
	if stopText == "" {
		stopText = "Stop"
	}

	escapedStop := escapeJSONString(stopText)

	return fmt.Sprintf(`{
  "schema": "2.0",
  "header": {"title": {"content": "Processing...", "tag": "plain_text"}},
  "body": {"elements": [
    {"tag": "markdown", "element_id": "main_content", "content": ""}
  ]},
  "action": {"elements": [
    {"tag": "button", "element_id": "stop_btn", "text": {"content": "%s", "tag": "plain_text"}, "type": "danger", "value": {"action": "stop"}}
  ]},
  "config": {"streaming_mode": true}
}`, escapedStop)
}

// extractSummary builds a short summary from content blocks for the card summary field.
func extractSummary(blocks []bot.ContentBlock) string {
	for _, b := range blocks {
		if b.Type == bot.ContentBlockText && b.Content != "" {
			return truncateString(b.Content, 100)
		}
	}
	return "Done"
}

func ptrString(s string) *string { return &s }

func truncateString(s string, maxRunes int) string {
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes-3]) + "..."
}
