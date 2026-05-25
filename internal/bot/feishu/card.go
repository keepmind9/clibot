package feishu

import (
	"context"
	"encoding/json"
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

// cardBatchUpdater abstracts Cardkit.V1.Card.BatchUpdate for testability.
type cardBatchUpdater interface {
	BatchUpdate(ctx context.Context, req *larkcardkit.BatchUpdateCardReq, opts ...larkcore.RequestOptionFunc) (*larkcardkit.BatchUpdateCardResp, error)
}

// cardSettingsUpdater abstracts Cardkit.V1.Card.Settings for testability.
type cardSettingsUpdater interface {
	Settings(ctx context.Context, req *larkcardkit.SettingsCardReq, opts ...larkcore.RequestOptionFunc) (*larkcardkit.SettingsCardResp, error)
}

// cardAPI combines card creation, batch update, and settings for testability.
type cardAPI interface {
	cardCreator
	cardBatchUpdater
	cardSettingsUpdater
}

// cardHandle implements bot.RichMessageHandle for a Feishu CardKit card.
type cardHandle struct {
	api       cardAPI
	cardID    string
	channel   string
	ctx       context.Context
	sequence  int
	mu        sync.Mutex
	finished  bool
	toolCount int
}

func (h *cardHandle) Channel() string { return h.channel }

func (h *cardHandle) Update(blocks []bot.ContentBlock) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.finished {
		return fmt.Errorf("card already finished")
	}

	actions, newToolCount := buildBatchUpdateActions(blocks, h.toolCount)
	h.toolCount = newToolCount

	actionsJSON, err := json.Marshal(actions)
	if err != nil {
		return fmt.Errorf("marshal actions: %w", err)
	}

	h.sequence++
	seq := h.sequence

	req := larkcardkit.NewBatchUpdateCardReqBuilder().
		CardId(h.cardID).
		Body(larkcardkit.NewBatchUpdateCardReqBodyBuilder().
			Sequence(seq).
			Actions(string(actionsJSON)).
			Build()).
		Build()

	resp, err := h.api.BatchUpdate(h.ctx, req)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"card_id": h.cardID,
			"error":   err,
		}).Warn("feishu-card-batch-update-error")
		return err
	}
	if !resp.Success() {
		logger.WithFields(logrus.Fields{
			"card_id": h.cardID,
			"code":    resp.Code,
			"msg":     resp.Msg,
		}).Warn("feishu-card-batch-update-api-error")
		return fmt.Errorf("card update API error: code=%d", resp.Code)
	}

	return nil
}

func (h *cardHandle) Finish(blocks []bot.ContentBlock) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.finished {
		return nil
	}
	h.finished = true

	// Final content update
	if len(blocks) > 0 {
		actions, _ := buildBatchUpdateActions(blocks, h.toolCount)
		actionsJSON, _ := json.Marshal(actions)
		h.sequence++

		req := larkcardkit.NewBatchUpdateCardReqBuilder().
			CardId(h.cardID).
			Body(larkcardkit.NewBatchUpdateCardReqBodyBuilder().
				Sequence(h.sequence).
				Actions(string(actionsJSON)).
				Build()).
			Build()

		if _, err := h.api.BatchUpdate(h.ctx, req); err != nil {
			logger.WithFields(logrus.Fields{
				"card_id": h.cardID,
				"error":   err,
			}).Warn("feishu-card-finish-batch-update-error")
		}
	}

	// End streaming mode
	streamingMode := false
	settingsJSON := fmt.Sprintf(
		`{"config":{"streaming_mode":%t},"summary":{"content":%q}}`,
		streamingMode,
		truncateString(extractSummary(blocks), 100),
	)

	h.sequence++
	settingsReq := larkcardkit.NewSettingsCardReqBuilder().
		CardId(h.cardID).
		Body(larkcardkit.NewSettingsCardReqBodyBuilder().
			Sequence(h.sequence).
			Settings(settingsJSON).
			Build()).
		Build()

	if resp, err := h.api.Settings(h.ctx, settingsReq); err != nil || !resp.Success() {
		logger.WithFields(logrus.Fields{
			"card_id": h.cardID,
			"error":   err,
		}).Warn("feishu-card-settings-error")
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

	return createRichMessage(botCtx, larkClient.Cardkit.V1.Card, larkClient.Im.Message, channel, opts)
}

func createRichMessage(
	ctx context.Context,
	api cardAPI,
	msgSender interface {
		Create(ctx context.Context, req *larkim.CreateMessageReq, opts ...larkcore.RequestOptionFunc) (*larkim.CreateMessageResp, error)
		Reply(ctx context.Context, req *larkim.ReplyMessageReq, opts ...larkcore.RequestOptionFunc) (*larkim.ReplyMessageResp, error)
	},
	channel string,
	opts bot.RichMessageOptions,
) (bot.RichMessageHandle, error) {
	title := "Processing..."
	if opts.Title != "" {
		title = opts.Title
	}

	cardData := buildSkeletonCard(title, opts.StopText)

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

	// Send card reference to chat
	contentJSON := fmt.Sprintf(`{"card_id":"%s"}`, cardID)

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
func buildSkeletonCard(title, stopText string) string {
	if stopText == "" {
		stopText = "Stop"
	}

	escapedTitle := escapeJSONString(title)
	escapedStop := escapeJSONString(stopText)

	return fmt.Sprintf(`{
  "schema": "2.0",
  "header": {"title": {"content": "%s", "tag": "plain_text"}},
  "body": {"elements": [
    {"tag": "markdown", "element_id": "main_content", "content": ""}
  ]},
  "action": {"elements": [
    {"tag": "button", "element_id": "stop_btn", "text": {"content": "%s", "tag": "plain_text"}, "type": "danger", "value": {"action": "stop"}}
  ]},
  "config": {"streaming_mode": true}
}`, escapedTitle, escapedStop)
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

func truncateString(s string, maxRunes int) string {
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes-3]) + "..."
}
