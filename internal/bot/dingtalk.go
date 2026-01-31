package bot

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/keepmind9/clibot/internal/logger"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/chatbot"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/client"
	"github.com/sirupsen/logrus"
)

// DingTalkBot implements BotAdapter interface for DingTalk using WebSocket long connection
type DingTalkBot struct {
	mu               sync.RWMutex
	clientID         string
	clientSecret     string
	streamClient     *client.StreamClient
	messageHandler   func(BotMessage)
	ctx              context.Context
	cancel           context.CancelFunc
}

// NewDingTalkBot creates a new DingTalk bot instance
func NewDingTalkBot(clientID, clientSecret string) *DingTalkBot {
	return &DingTalkBot{
		clientID:     clientID,
		clientSecret: clientSecret,
	}
}

// Start establishes WebSocket long connection to DingTalk and begins listening for messages
func (d *DingTalkBot) Start(messageHandler func(BotMessage)) error {
	d.SetMessageHandler(messageHandler)
	d.ctx, d.cancel = context.WithCancel(context.Background())

	logger.WithFields(logrus.Fields{
		"client_id": maskClientID(d.clientID),
	}).Info("starting-dingtalk-bot-with-websocket-long-connection")

	// Create stream client with credentials
	credential := client.NewAppCredentialConfig(d.clientID, d.clientSecret)

	d.mu.Lock()
	d.streamClient = client.NewStreamClient(client.WithAppCredential(credential))
	streamClient := d.streamClient
	d.mu.Unlock()

	// Register chatbot message callback
	streamClient.RegisterChatBotCallbackRouter(d.handleMessageReceive)

	// Start long connection
	go func() {
		if err := streamClient.Start(d.ctx); err != nil {
			logger.WithFields(logrus.Fields{
				"client_id": d.clientID,
				"error":     err,
			}).Error("dingtalk-websocket-connection-failed")
		}
	}()

	// Give connection time to establish
	time.Sleep(2 * time.Second)

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

	// Extract message content
	content := ""
	if data.Msgtype == "text" {
		content = data.Text.Content
	}

	// Call the handler with BotMessage
	handler := d.GetMessageHandler()
	if handler != nil {
		handler(BotMessage{
			Platform:  "dingtalk",
			UserID:    data.SenderStaffId, // Use staffId as user identifier
			Channel:   data.ConversationId,
			Content:   content,
			Timestamp: time.Now(),
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

	// DingTalk message limit
	const maxDingTalkLength = 20000
	if len(message) > maxDingTalkLength {
		logger.WithFields(logrus.Fields{
			"original_length": len(message),
			"max_length":      maxDingTalkLength,
		}).Info("truncating-message-for-dingtalk-limit")
		message = message[:maxDingTalkLength]
	}

	// TODO: Need SessionWebhook from message context to reply
	// The replier needs a session webhook URL to send messages
	// This is a limitation - we need to store the webhook URL per conversation
	logger.WithFields(logrus.Fields{
		"conversation_id": conversationID,
		"message_length":  len(message),
	}).Warn("dingtalk-send-message-not-fully-implemented-needs-session-webhook")

	return fmt.Errorf("sending messages to DingTalk requires session webhook URL (not yet implemented)")
}

// Stop closes the DingTalk WebSocket connection and cleans up resources
func (d *DingTalkBot) Stop() error {
	if d.cancel != nil {
		d.cancel()
	}

	d.mu.Lock()
	streamClient := d.streamClient
	d.streamClient = nil
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

// maskClientID masks sensitive client ID information for logging
func maskClientID(clientID string) string {
	if len(clientID) <= 8 {
		return "***"
	}
	return clientID[:4] + "***" + clientID[len(clientID)-4:]
}
