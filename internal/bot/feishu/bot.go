package feishu

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/keepmind9/clibot/internal/bot"
	"github.com/keepmind9/clibot/internal/logger"
	"github.com/keepmind9/clibot/internal/proxy"
	"github.com/keepmind9/clibot/pkg/constants"
	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	"github.com/larksuite/oapi-sdk-go/v3/ws"
	"github.com/sirupsen/logrus"
)

// Bot implements BotAdapter interface for Feishu (Lark) using WebSocket long connection
type Bot struct {
	mu                sync.RWMutex
	appID             string
	appSecret         string
	encryptKey        string
	verificationToken string
	wsClient          *ws.Client
	larkClient        *lark.Client
	dispatcher        *dispatcher.EventDispatcher
	messageHandler    func(bot.BotMessage)
	ctx               context.Context
	cancel            context.CancelFunc
	typingReactions   map[string]string
	proxyMgr          proxy.Manager
	mentionInGroup    bool
	debounceMs        int
}

// NewBot creates a new Feishu bot instance
func NewBot(appID, appSecret string) *Bot {
	return &Bot{
		appID:           appID,
		appSecret:       appSecret,
		mentionInGroup:  true,
		typingReactions: make(map[string]string),
	}
}

// SetProxyManager sets the proxy manager for the Feishu bot
func (b *Bot) SetProxyManager(mgr proxy.Manager) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.proxyMgr = mgr
}

// Start establishes WebSocket long connection to Feishu and begins listening for messages
func (b *Bot) Start(messageHandler func(bot.BotMessage)) error {
	b.SetMessageHandler(messageHandler)
	b.ctx, b.cancel = context.WithCancel(context.Background())

	logger.WithFields(logrus.Fields{
		"app_id": bot.MaskSecret(b.appID),
	}).Info("starting-feishu-bot-with-websocket-long-connection")

	// Create event dispatcher
	b.mu.Lock()
	b.dispatcher = dispatcher.NewEventDispatcher(b.verificationToken, b.encryptKey)

	if b.proxyMgr != nil {
		proxyURL := b.proxyMgr.GetProxyURL("feishu")
		if proxyURL != "" && proxyURL != "env://HTTP_PROXY" {
			logger.WithField("proxy", proxyURL).Info("feishu-proxy-configured-but-sdk-requires-env-vars")
		}
		b.larkClient = lark.NewClient(b.appID, b.appSecret)
	} else {
		b.larkClient = lark.NewClient(b.appID, b.appSecret)
	}
	b.mu.Unlock()

	// Register message received event handler
	b.mu.RLock()
	dispatcher := b.dispatcher
	b.mu.RUnlock()
	dispatcher.OnP2MessageReceiveV1(func(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
		return b.handleMessageReceive(ctx, event)
	})

	// Create WebSocket client
	b.mu.Lock()
	opts := []ws.ClientOption{
		ws.WithEventHandler(dispatcher),
		ws.WithLogLevel(larkcore.LogLevelInfo),
		ws.WithAutoReconnect(true),
	}

	b.wsClient = ws.NewClient(b.appID, b.appSecret, opts...)
	wsClient := b.wsClient
	b.mu.Unlock()

	go func() {
		if err := wsClient.Start(b.ctx); err != nil {
			logger.WithFields(logrus.Fields{
				"app_id": b.appID,
				"error":  err,
			}).Error("feishu-websocket-connection-failed")
		}
	}()

	time.Sleep(constants.DefaultConnectionTimeout)

	logger.Info("feishu-websocket-long-connection-started")
	return nil
}

// handleMessageReceive handles incoming message events from Feishu
func (b *Bot) handleMessageReceive(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
	if event == nil || event.Event == nil {
		return nil
	}

	eventJSON, err := json.Marshal(event)
	if err == nil {
		logger.WithField("event", string(eventJSON)).Debug("Received Feishu event (raw JSON)")
	} else {
		logger.WithField("error", err).Warn("Failed to marshal event to JSON")
	}

	ev := event.Event

	var messageID, chatID, senderID, content string
	var messageType, chatType, parentID string

	if ev.Message != nil {
		if ev.Message.MessageId != nil {
			messageID = *ev.Message.MessageId
		}
		if ev.Message.ChatId != nil {
			chatID = *ev.Message.ChatId
		}
		if ev.Message.MessageType != nil {
			messageType = *ev.Message.MessageType
		}
		if ev.Message.ChatType != nil {
			chatType = *ev.Message.ChatType
		}
		if ev.Message.ParentId != nil {
			parentID = *ev.Message.ParentId
		}
		if ev.Message.Content != nil {
			content = *ev.Message.Content
			content = extractTextContent(content)
		}
	}

	if ev.Sender != nil && ev.Sender.SenderId != nil {
		if ev.Sender.SenderId.OpenId != nil {
			senderID = *ev.Sender.SenderId.OpenId
		} else if ev.Sender.SenderId.UserId != nil {
			senderID = *ev.Sender.SenderId.UserId
		}
	}

	logger.WithFields(logrus.Fields{
		"platform":     "feishu",
		"user_id":      senderID,
		"chat_id":      chatID,
		"chat_type":    chatType,
		"message_id":   messageID,
		"message_type": messageType,
		"parent_id":    parentID,
		"content_len":  len(content),
	}).Info("received-feishu-message-event-parsed")

	handler := b.GetMessageHandler()
	if handler != nil {
		handler(bot.BotMessage{
			Platform:  "feishu",
			UserID:    senderID,
			Channel:   chatID,
			MessageID: messageID,
			Content:   content,
			Timestamp: time.Now(),
			ChatType:  chatType,
			ThreadID:  parentID,
			QuoteID:   parentID,
		})
	}

	return nil
}

// SendMessage sends a message to a Feishu chat
func (b *Bot) SendMessage(chatID, message string) error {
	b.mu.RLock()
	larkClient := b.larkClient
	ctx := b.ctx
	b.mu.RUnlock()

	if larkClient == nil {
		return fmt.Errorf("feishu client not initialized")
	}

	if chatID == "" {
		return fmt.Errorf("chat ID is required for Feishu")
	}

	const maxFeishuLength = constants.MaxFeishuMessageLength
	if len(message) > maxFeishuLength {
		logger.WithFields(logrus.Fields{
			"original_length": len(message),
			"max_length":      maxFeishuLength,
		}).Info("truncating-message-for-feishu-limit")
		message = message[:maxFeishuLength]
	}

	contentJSON := fmt.Sprintf(`{"text":"%s"}`, escapeJSONString(message))

	body := larkim.NewCreateMessageReqBodyBuilder().
		ReceiveId(chatID).
		MsgType(larkim.MsgTypeText).
		Content(contentJSON).
		Build()

	req := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType(larkim.ReceiveIdTypeChatId).
		Body(body).
		Build()

	resp, err := larkClient.Im.Message.Create(ctx, req)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"chat_id": chatID,
			"error":   err,
		}).Error("failed-to-send-message-to-feishu")
		return fmt.Errorf("failed to send message to chat %s: %w", chatID, err)
	}

	if !resp.Success() {
		logger.WithFields(logrus.Fields{
			"chat_id":      chatID,
			"code":         resp.Code,
			"msg":          resp.Msg,
			"request_id":   resp.RequestId,
			"message":      message,
			"message_len":  len(message),
			"content_json": contentJSON,
		}).Error("failed-to-send-message-to-feishu-api-error")
		return fmt.Errorf("API error: code=%d, msg=%s", resp.Code, resp.Msg)
	}

	logger.WithField("chat_id", chatID).Info("message-sent-to-feishu")
	return nil
}

// Stop closes the Feishu WebSocket connection and cleans up resources
func (b *Bot) Stop() error {
	if b.cancel != nil {
		b.cancel()
	}

	b.mu.Lock()
	b.wsClient = nil
	b.mu.Unlock()

	logger.Info("feishu-bot-stopped")
	return nil
}

// SetMessageHandler sets the message handler in a thread-safe manner
func (b *Bot) SetMessageHandler(handler func(bot.BotMessage)) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.messageHandler = handler
}

// GetMessageHandler gets the message handler in a thread-safe manner
func (b *Bot) GetMessageHandler() func(bot.BotMessage) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.messageHandler
}

// SetEncryptKey sets the encryption key for event verification
func (b *Bot) SetEncryptKey(key string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.encryptKey = key
}

// SetVerificationToken sets the verification token for event verification
func (b *Bot) SetVerificationToken(token string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.verificationToken = token
}

// SupportsTypingIndicator returns true as Feishu supports message reactions
func (b *Bot) SupportsTypingIndicator() bool {
	return true
}

// AddTypingIndicator adds a typing reaction to a message
func (b *Bot) AddTypingIndicator(messageID string) bool {
	b.mu.RLock()
	_, exists := b.typingReactions[messageID]
	b.mu.RUnlock()

	if exists {
		return true
	}

	b.mu.RLock()
	larkClient := b.larkClient
	botCtx := b.ctx
	b.mu.RUnlock()

	if larkClient == nil {
		logger.WithField("error", "lark client not initialized").Error("failed-to-add-typing-indicator")
		return false
	}

	ctx, cancel := context.WithTimeout(botCtx, constants.TypingIndicatorTimeout)
	defer cancel()

	reactionType := larkim.NewEmojiBuilder().
		EmojiType("Typing").
		Build()

	body := larkim.NewCreateMessageReactionReqBodyBuilder().
		ReactionType(reactionType).
		Build()

	req := larkim.NewCreateMessageReactionReqBuilder().
		MessageId(messageID).
		Body(body).
		Build()

	resp, err := larkClient.Im.MessageReaction.Create(ctx, req)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"error": err,
		}).Error("failed-to-add-typing-indicator")
		return false
	}

	if !resp.Success() {
		logger.WithFields(logrus.Fields{
			"code":       resp.Code,
			"msg":        resp.Msg,
			"message_id": messageID,
		}).Error("feishu-add-typing-api-error")
		return false
	}

	if resp.Data == nil || resp.Data.ReactionId == nil {
		logger.WithFields(logrus.Fields{
			"message_id": messageID,
		}).Error("feishu-add-typing-no-reaction-id")
		return false
	}

	b.mu.Lock()
	b.typingReactions[messageID] = *resp.Data.ReactionId
	b.mu.Unlock()

	logger.WithFields(logrus.Fields{
		"message_id":  messageID,
		"reaction_id": *resp.Data.ReactionId,
	}).Info("feishu-typing-indicator-added")

	return true
}

// RemoveTypingIndicator removes the typing reaction from a message
func (b *Bot) RemoveTypingIndicator(messageID string) error {
	b.mu.Lock()
	reactionID, exists := b.typingReactions[messageID]
	if !exists {
		b.mu.Unlock()
		return nil
	}
	delete(b.typingReactions, messageID)
	b.mu.Unlock()

	b.mu.RLock()
	larkClient := b.larkClient
	botCtx := b.ctx
	b.mu.RUnlock()

	if larkClient == nil {
		return fmt.Errorf("lark client not initialized")
	}

	ctx, cancel := context.WithTimeout(botCtx, constants.TypingIndicatorTimeout)
	defer cancel()

	req := larkim.NewDeleteMessageReactionReqBuilder().
		MessageId(messageID).
		ReactionId(reactionID).
		Build()

	resp, err := larkClient.Im.MessageReaction.Delete(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to delete typing indicator: %w", err)
	}

	if !resp.Success() {
		return fmt.Errorf("failed to delete typing indicator: code=%d, msg=%s", resp.Code, resp.Msg)
	}

	logger.WithFields(logrus.Fields{
		"message_id":  messageID,
		"reaction_id": reactionID,
	}).Info("feishu-typing-indicator-removed")

	return nil
}
