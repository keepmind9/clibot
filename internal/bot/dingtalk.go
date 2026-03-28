package bot

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/keepmind9/clibot/internal/logger"
	"github.com/keepmind9/clibot/internal/proxy"
	"github.com/keepmind9/clibot/pkg/constants"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/chatbot"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/client"
	"github.com/sirupsen/logrus"
)

// DingTalkBot implements BotAdapter interface for DingTalk using WebSocket long connection
type DingTalkBot struct {
	DefaultTypingIndicator
	mu              sync.RWMutex
	clientID        string
	clientSecret    string
	streamClient    *client.StreamClient
	replier         *chatbot.ChatbotReplier
	messageHandler  func(BotMessage)
	sessionWebhooks map[string]string // conversationID -> sessionWebhook
	ctx             context.Context
	cancel          context.CancelFunc
	proxyMgr        proxy.Manager
}

// NewDingTalkBot creates a new DingTalk bot instance
func NewDingTalkBot(clientID, clientSecret string) *DingTalkBot {
	return &DingTalkBot{
		clientID:        clientID,
		clientSecret:    clientSecret,
		sessionWebhooks: make(map[string]string),
		replier:         chatbot.NewChatbotReplier(),
	}
}

// SetProxyManager sets the proxy manager for the DingTalk bot
func (d *DingTalkBot) SetProxyManager(mgr proxy.Manager) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.proxyMgr = mgr
}

// Start establishes WebSocket long connection to DingTalk and begins listening for messages
func (d *DingTalkBot) Start(messageHandler func(BotMessage)) error {
	d.SetMessageHandler(messageHandler)
	d.ctx, d.cancel = context.WithCancel(context.Background())

	logger.WithFields(logrus.Fields{
		"client_id": maskSecret(d.clientID),
	}).Info("starting-dingtalk-bot-with-websocket-long-connection")

	// Create stream client with credentials
	credential := client.NewAppCredentialConfig(d.clientID, d.clientSecret)

	d.mu.Lock()
	defer d.mu.Unlock()

	// Create stream client with proxy support
	if d.proxyMgr != nil {
		proxyURL := d.proxyMgr.GetProxyURL("dingtalk")
		if proxyURL != "" && proxyURL != "env://HTTP_PROXY" {
			logger.WithField("proxy", proxyURL).Info("dingtalk-proxy-configured-but-sdk-requires-env-vars")
		}
		d.streamClient = client.NewStreamClient(client.WithAppCredential(credential))
	} else {
		d.streamClient = client.NewStreamClient(client.WithAppCredential(credential))
	}
	streamClient := d.streamClient

	// Register chatbot message callback
	streamClient.RegisterChatBotCallbackRouter(d.handleMessageReceive)

	// Start long connection
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.WithField("panic", r).Error("dingtalk-stream-client-panic")
			}
		}()
		if err := streamClient.Start(d.ctx); err != nil {
			logger.WithFields(logrus.Fields{
				"client_id": d.clientID,
				"error":     err,
			}).Error("dingtalk-websocket-connection-failed")
		}
	}()

	// Give connection time to establish
	time.Sleep(constants.DefaultConnectionTimeout)

	logger.Info("dingtalk-websocket-long-connection-started")
	return nil
}

// handleMessageReceive handles incoming message events from DingTalk
func (d *DingTalkBot) handleMessageReceive(ctx context.Context, data *chatbot.BotCallbackDataModel) ([]byte, error) {
	if data == nil {
		return []byte(""), nil
	}

	// Log the complete event object for debugging
	logger.WithFields(logrus.Fields{
		"platform":          "dingtalk",
		"conversation_id":   data.ConversationId,
		"conversation_type": data.ConversationType,
		"sender_id":         data.SenderId,
		"sender_nick":       data.SenderNick,
		"sender_staff_id":   data.SenderStaffId,
		"msg_id":            data.MsgId,
		"msg_type":          data.Msgtype,
		"text_content":      data.Text.Content,
		"is_admin":          data.IsAdmin,
		"is_in_at_list":     data.IsInAtList,
		"create_at":         data.CreateAt,
	}).Info("received-dingtalk-message-event-parsed")

	// Extract message content based on message type
	content := ""
	switch data.Msgtype {
	case "text":
		content = data.Text.Content
	case "image":
		content = "[image]"
	case "voice":
		content = "[voice]"
	case "file":
		content = "[file]"
	case "video":
		content = "[video]"
	case "richText":
		content = "[rich text]"
	default:
		// For unknown types, try to use text content if available
		if data.Msgtype == "" && data.Text.Content != "" {
			content = data.Text.Content
		}
	}

	// Skip empty content messages
	if content == "" {
		return []byte(""), nil
	}

	// Store session webhook for sending replies later
	if data.SessionWebhook != "" {
		d.mu.Lock()
		d.sessionWebhooks[data.ConversationId] = data.SessionWebhook
		d.mu.Unlock()
	}

	// Convert CreateAt (Unix milliseconds) to time.Time
	var msgTimestamp time.Time
	if data.CreateAt > 0 {
		msgTimestamp = time.Unix(data.CreateAt/1000, data.CreateAt%1000*int64(time.Millisecond))
	} else {
		msgTimestamp = time.Now()
	}

	// Call the handler with BotMessage
	handler := d.GetMessageHandler()
	if handler != nil {
		handler(BotMessage{
			Platform:  "dingtalk",
			UserID:    data.SenderStaffId,
			Channel:   data.ConversationId,
			MessageID: data.MsgId,
			Content:   content,
			Timestamp: msgTimestamp,
		})
	}

	// Return success (empty response means no error)
	return []byte(""), nil
}

// SendMessage sends a message to a DingTalk conversation
func (d *DingTalkBot) SendMessage(conversationID, message string) error {
	if conversationID == "" {
		return fmt.Errorf("conversation ID is required for DingTalk")
	}

	// Get session webhook for this conversation
	d.mu.RLock()
	sessionWebhook, ok := d.sessionWebhooks[conversationID]
	d.mu.RUnlock()

	if sessionWebhook == "" || !ok {
		return fmt.Errorf("no session webhook found for conversation %s, please send a message first", conversationID)
	}

	// DingTalk message limit
	const maxDingTalkLength = constants.MaxDingTalkMessageLength
	if len(message) > maxDingTalkLength {
		logger.WithFields(logrus.Fields{
			"original_length": len(message),
			"max_length":      maxDingTalkLength,
		}).Info("truncating-message-for-dingtalk-limit")
		message = message[:maxDingTalkLength]
	}

	// Send message using ChatbotReplier
	ctx, cancel := context.WithTimeout(context.Background(), constants.DingTalkMessageSendTimeout)
	defer cancel()

	err := d.replier.SimpleReplyText(ctx, sessionWebhook, []byte(message))
	if err != nil {
		logger.WithFields(logrus.Fields{
			"conversation_id": conversationID,
			"error":           err,
		}).Error("failed-to-send-message-to-dingtalk")
		return fmt.Errorf("failed to send message to DingTalk: %w", err)
	}

	logger.WithField("conversation_id", conversationID).Info("message-sent-to-dingtalk")
	return nil
}

// Stop closes the DingTalk WebSocket connection and cleans up resources
func (d *DingTalkBot) Stop() error {
	if d.cancel != nil {
		d.cancel()
	}

	d.mu.Lock()
	streamClient := d.streamClient
	d.streamClient = nil
	d.sessionWebhooks = make(map[string]string)
	d.mu.Unlock()

	if streamClient != nil {
		streamClient.Close()
		logger.Info("dingtalk-websocket-connection-stopped")
	}

	logger.Info("dingtalk-bot-stopped")
	return nil
}

// SetMessageHandler sets the message handler in a thread-safe manner
func (d *DingTalkBot) SetMessageHandler(handler func(BotMessage)) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.messageHandler = handler
}

// GetMessageHandler gets the message handler in a thread-safe manner
func (d *DingTalkBot) GetMessageHandler() func(BotMessage) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.messageHandler
}
