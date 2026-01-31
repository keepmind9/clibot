package bot

import (
	"context"
	"fmt"
	"time"

	"github.com/keepmind9/clibot/internal/logger"
	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	"github.com/larksuite/oapi-sdk-go/v3/ws"
	"github.com/sirupsen/logrus"
)

// FeishuBot implements BotAdapter interface for Feishu (Lark) using WebSocket long connection
type FeishuBot struct {
	AppID           string
	AppSecret       string
	EncryptKey      string // Optional, for encrypted events
	VerificationToken string // Optional, for event verification
	WSClient        *ws.Client
	LarkClient      *lark.Client
	Dispatcher      *dispatcher.EventDispatcher
	messageHandler  func(BotMessage)
	ctx             context.Context
	cancel          context.CancelFunc
}

// NewFeishuBot creates a new Feishu bot instance
func NewFeishuBot(appID, appSecret string) *FeishuBot {
	return &FeishuBot{
		AppID:      appID,
		AppSecret:  appSecret,
		LarkClient: lark.NewClient(appID, appSecret),
	}
}

// Start establishes WebSocket long connection to Feishu and begins listening for messages
func (f *FeishuBot) Start(messageHandler func(BotMessage)) error {
	f.messageHandler = messageHandler
	f.ctx, f.cancel = context.WithCancel(context.Background())

	// Log bot startup
	logger.WithFields(logrus.Fields{
		"app_id": maskAppID(f.AppID),
	}).Info("starting-feishu-bot-with-websocket-long-connection")

	// Create event dispatcher
	f.Dispatcher = dispatcher.NewEventDispatcher(f.VerificationToken, f.EncryptKey)

	// Register message received event handler
	f.Dispatcher.OnP2MessageReceiveV1(func(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
		return f.handleMessageReceive(ctx, event)
	})

	// Create WebSocket client
	f.WSClient = ws.NewClient(f.AppID, f.AppSecret,
		ws.WithEventHandler(f.Dispatcher),
		ws.WithLogLevel(larkcore.LogLevelInfo),
		ws.WithAutoReconnect(true),
	)

	// Start long connection (this blocks)
	go func() {
		if err := f.WSClient.Start(f.ctx); err != nil {
			logger.WithFields(logrus.Fields{
				"app_id": f.AppID,
				"error":  err,
			}).Error("feishu-websocket-connection-failed")
		}
	}()

	// Give connection time to establish
	time.Sleep(2 * time.Second)

	logger.Info("feishu-websocket-long-connection-started")
	return nil
}

// handleMessageReceive handles incoming message events from Feishu
func (f *FeishuBot) handleMessageReceive(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
	if event == nil || event.Event == nil {
		return nil
	}

	// // Log the complete event object as JSON for debugging
	// eventJSON, err := json.Marshal(event)
	// if err == nil {
	// 	logger.WithField("event", string(eventJSON)).Info("Received Feishu event (raw JSON)")
	// } else {
	// 	logger.WithField("error", err).Warn("Failed to marshal event to JSON")
	// }

	// Extract message information
	ev := event.Event

	var messageID, chatID, senderID, content string
	var messageType, chatType string

	// Get message details from Message field
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
		// Extract message content (JSON string format)
		if ev.Message.Content != nil {
			content = *ev.Message.Content
			// For text messages, content is like: {"text":"actual message"}
			// Parse to extract actual text
			content = extractTextContent(content)
		}
	}

	// Get sender ID
	if ev.Sender != nil && ev.Sender.SenderId != nil {
		if ev.Sender.SenderId.UserId != nil {
			senderID = *ev.Sender.SenderId.UserId
		}
	}

	// Log parsed event data
	logger.WithFields(logrus.Fields{
		"platform":     "feishu",
		"user_id":      senderID,
		"chat_id":      chatID,
		"chat_type":    chatType,
		"message_id":   messageID,
		"message_type": messageType,
		"content":      content,
		"content_len":  len(content),
	}).Info("received-feishu-message-event-parsed")

	// Call the handler with BotMessage
	if f.messageHandler != nil {
		f.messageHandler(BotMessage{
			Platform:  "feishu",
			UserID:    senderID,
			Channel:   chatID,
			Content:   content,
			Timestamp: time.Now(),
		})
	}

	return nil
}

// SendMessage sends a message to a Feishu chat
func (f *FeishuBot) SendMessage(chatID, message string) error {
	if f.LarkClient == nil {
		return fmt.Errorf("feishu client not initialized")
	}

	if chatID == "" {
		return fmt.Errorf("chat ID is required for Feishu")
	}

	// Feishu limit: text message content length
	const maxFeishuLength = 20000
	if len(message) > maxFeishuLength {
		logger.WithFields(logrus.Fields{
			"original_length": len(message),
			"max_length":      maxFeishuLength,
		}).Info("truncating-message-for-feishu-limit")
		message = message[:maxFeishuLength]
	}

	// Create message request body
	// For text messages, content format: {"text":"actual content"}
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

	// Send message
	resp, err := f.LarkClient.Im.Message.Create(f.ctx, req)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"chat_id": chatID,
			"error":   err,
		}).Error("failed-to-send-message-to-feishu")
		return fmt.Errorf("failed to send message to chat %s: %w", chatID, err)
	}

	if !resp.Success() {
		logger.WithFields(logrus.Fields{
			"chat_id":    chatID,
			"code":       resp.Code,
			"msg":        resp.Msg,
			"request_id": resp.RequestId,
		}).Error("failed-to-send-message-to-feishu-api-error")
		return fmt.Errorf("API error: code=%d, msg=%s", resp.Code, resp.Msg)
	}

	logger.WithField("chat_id", chatID).Info("message-sent-to-feishu")
	return nil
}

// Stop closes the Feishu WebSocket connection and cleans up resources
func (f *FeishuBot) Stop() error {
	if f.cancel != nil {
		f.cancel()
	}

	if f.WSClient != nil {
		// Note: ws.Client doesn't have a Stop method in v3.5.3
		// The connection is managed by the context
		logger.Info("feishu-websocket-connection-stopped")
	}

	logger.Info("feishu-bot-stopped")
	return nil
}

// maskAppID masks sensitive app ID information for logging
func maskAppID(appID string) string {
	if len(appID) <= 8 {
		return "***"
	}
	return appID[:4] + "***" + appID[len(appID)-4:]
}

// extractTextContent extracts actual text from Feishu message content
// Feishu text message format: {"text":"actual message"}
func extractTextContent(content string) string {
	// Simple JSON parsing to extract text field
	// In production, should use proper JSON unmarshaling
	if len(content) > 10 && content[:1] == "{" {
		// Try to find "text":"..." pattern
		textStart := findInString(content, `"text":"`, 0)
		if textStart > 0 {
			textStart += 8 // len(`"text":"`)
			textEnd := findInString(content, `"`, textStart)
			if textEnd > textStart {
				return content[textStart:textEnd]
			}
		}
	}
	return content
}

// findInString finds substring in string starting from index
func findInString(s, substr string, start int) int {
	if start >= len(s) {
		return -1
	}
	for i := start; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// escapeJSONString escapes special characters for JSON string content
func escapeJSONString(s string) string {
	// Simple JSON escaping for text content
	result := ""
	for _, c := range s {
		switch c {
		case '"':
			result += "\\\""
		case '\\':
			result += "\\\\"
		case '\n':
			result += "\\n"
		case '\r':
			result += "\\r"
		case '\t':
			result += "\\t"
		default:
			result += string(c)
		}
	}
	return result
}
