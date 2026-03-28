package bot

import (
	"context"
	"fmt"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/keepmind9/clibot/internal/logger"
	"github.com/keepmind9/clibot/internal/proxy"
	"github.com/keepmind9/clibot/pkg/constants"
	"github.com/sirupsen/logrus"
)

// TelegramBot implements BotAdapter interface for Telegram using long polling
type TelegramBot struct {
	DefaultTypingIndicator
	mu             sync.RWMutex
	token          string
	bot            *tgbotapi.BotAPI
	messageHandler func(BotMessage)
	ctx            context.Context
	cancel         context.CancelFunc
	proxyMgr       proxy.Manager
}

// NewTelegramBot creates a new Telegram bot instance
func NewTelegramBot(token string) *TelegramBot {
	return &TelegramBot{
		token: token,
	}
}

// SetProxyManager sets the proxy manager for the Telegram bot
func (t *TelegramBot) SetProxyManager(mgr proxy.Manager) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.proxyMgr = mgr
}

// Start establishes long polling connection to Telegram and begins listening for messages
func (t *TelegramBot) Start(messageHandler func(BotMessage)) error {
	t.SetMessageHandler(messageHandler)
	t.ctx, t.cancel = context.WithCancel(context.Background())

	logger.WithFields(logrus.Fields{
		"token": maskSecret(t.token),
	}).Info("starting-telegram-bot-with-long-polling")

	var err error
	t.mu.Lock()
	defer t.mu.Unlock()

	// Use proxy manager if available
	if t.proxyMgr != nil {
		client, clientErr := t.proxyMgr.GetHTTPClient("telegram")
		if clientErr != nil {
			logger.WithField("error", clientErr).Error("failed-to-create-proxy-client")
			return fmt.Errorf("failed to create proxy client: %w", clientErr)
		}
		t.bot, err = tgbotapi.NewBotAPIWithClient(t.token, tgbotapi.APIEndpoint, client)
	} else {
		t.bot, err = tgbotapi.NewBotAPI(t.token)
	}

	if err != nil {
		logger.WithFields(logrus.Fields{
			"error": err,
		}).Error("failed-to-initialize-telegram-bot")
		return fmt.Errorf("failed to initialize Telegram bot: %w", err)
	}

	bot := t.bot

	logger.WithFields(logrus.Fields{
		"bot_username": bot.Self.UserName,
		"bot_id":       bot.Self.ID,
	}).Info("telegram-bot-initialized-successfully")

	// Set up long polling configuration
	u := tgbotapi.NewUpdate(0)
	u.Timeout = int(constants.TelegramLongPollTimeout.Seconds())

	// Start receiving updates via long polling
	updates := bot.GetUpdatesChan(u)

	// Process updates in background
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.WithField("panic", r).Error("telegram-message-handler-panic")
			}
		}()
		for {
			select {
			case <-t.ctx.Done():
				logger.Info("telegram-long-polling-stopped")
				return
			case update, ok := <-updates:
				if !ok {
					logger.Info("telegram-updates-channel-closed")
					return
				}

				t.handleUpdate(update)
			}
		}
	}()

	logger.Info("telegram-long-polling-connection-started")
	return nil
}

// handleUpdate handles incoming update events from Telegram
func (t *TelegramBot) handleUpdate(update tgbotapi.Update) {
	// Handle regular messages
	if update.Message != nil {
		t.handleMessage(update.Message)
		return
	}

	// Handle edited messages
	if update.EditedMessage != nil {
		t.handleMessage(update.EditedMessage)
		return
	}

	// Handle channel posts
	if update.ChannelPost != nil {
		t.handleMessage(update.ChannelPost)
		return
	}

	// Handle edited channel posts
	if update.EditedChannelPost != nil {
		t.handleMessage(update.EditedChannelPost)
		return
	}

	// Handle callback queries (inline keyboard button clicks)
	if update.CallbackQuery != nil {
		t.handleCallbackQuery(update.CallbackQuery)
		return
	}
}

// handleMessage handles incoming message events from Telegram
func (t *TelegramBot) handleMessage(message *tgbotapi.Message) {
	if message == nil {
		return
	}

	// Extract message information
	var userID, chatID string
	var userName, firstName, lastName string

	if message.From != nil {
		userID = fmt.Sprintf("%d", message.From.ID)
		userName = message.From.UserName
		firstName = message.From.FirstName
		lastName = message.From.LastName
	}

	if message.Chat != nil {
		chatID = fmt.Sprintf("%d", message.Chat.ID)
	}

	// Get text content - prefer Text, fallback to Caption
	content := message.Text
	if content == "" {
		content = message.Caption
	}

	// Log parsed message data
	logger.WithFields(logrus.Fields{
		"platform":    "telegram",
		"user_id":     userID,
		"username":    userName,
		"first_name":  firstName,
		"last_name":   lastName,
		"chat_id":     chatID,
		"chat_type":   message.Chat.Type,
		"message_id":  message.MessageID,
		"content":     content,
		"content_len": len(content),
	}).Info("received-telegram-message-parsed")

	// Only process messages with text content
	if content == "" {
		return
	}

	// Call the handler with BotMessage
	handler := t.GetMessageHandler()
	if handler != nil {
		handler(BotMessage{
			Platform:  "telegram",
			UserID:    userID,
			Channel:   chatID,
			MessageID: fmt.Sprintf("%d", message.MessageID),
			Content:   content,
			Timestamp: time.Unix(int64(message.Date), 0),
		})
	}
}

// handleCallbackQuery handles inline keyboard callback queries
func (t *TelegramBot) handleCallbackQuery(callback *tgbotapi.CallbackQuery) {
	if callback == nil || callback.Message == nil {
		return
	}

	var userID, chatID string
	if callback.From != nil {
		userID = fmt.Sprintf("%d", callback.From.ID)
	}
	if callback.Message.Chat != nil {
		chatID = fmt.Sprintf("%d", callback.Message.Chat.ID)
	}

	logger.WithFields(logrus.Fields{
		"platform":    "telegram",
		"user_id":     userID,
		"callback_id": callback.ID,
		"chat_id":     chatID,
		"message_id":  callback.Message.MessageID,
		"data":        callback.Data,
	}).Info("received-telegram-callback-query")

	handler := t.GetMessageHandler()
	if handler != nil {
		// Use callback data as content, prefixed to identify it as a callback
		content := callback.Data
		if content == "" {
			content = "[callback]"
		}
		handler(BotMessage{
			Platform:  "telegram",
			UserID:    userID,
			Channel:   chatID,
			MessageID: fmt.Sprintf("%d", callback.Message.MessageID),
			Content:   content,
			Timestamp: time.Unix(int64(callback.Message.Date), 0),
		})
	}
}

// SendMessage sends a message to a Telegram chat
func (t *TelegramBot) SendMessage(chatID, message string) error {
	t.mu.RLock()
	bot := t.bot
	t.mu.RUnlock()

	if bot == nil {
		return fmt.Errorf("telegram bot not initialized")
	}

	if chatID == "" {
		return fmt.Errorf("chat ID is required for Telegram")
	}

	// Telegram message limit
	const maxTelegramLength = constants.MaxTelegramMessageLength
	if len(message) > maxTelegramLength {
		logger.WithFields(logrus.Fields{
			"original_length": len(message),
			"max_length":      maxTelegramLength,
		}).Info("truncating-message-for-telegram-limit")
		message = message[:maxTelegramLength]
	}

	// Parse chat ID (convert string to int64)
	var chatIDInt int64
	if _, err := fmt.Sscanf(chatID, "%d", &chatIDInt); err != nil {
		return fmt.Errorf("invalid chat ID format: %w", err)
	}

	// Create message - use plain text to avoid markdown parsing issues
	msg := tgbotapi.NewMessage(chatIDInt, message)

	// Send message
	_, err := bot.Send(msg)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"chat_id": chatID,
			"error":   err,
		}).Error("failed-to-send-message-to-telegram")
		return fmt.Errorf("failed to send message to chat %s: %w", chatID, err)
	}

	logger.WithField("chat_id", chatID).Info("message-sent-to-telegram")
	return nil
}

// Stop closes the Telegram long polling connection and cleans up resources
func (t *TelegramBot) Stop() error {
	if t.cancel != nil {
		t.cancel()
	}

	t.mu.Lock()
	bot := t.bot
	t.bot = nil
	t.mu.Unlock()

	if bot != nil {
		bot.StopReceivingUpdates()
		logger.Info("telegram-long-polling-stopped")
	}

	logger.Info("telegram-bot-stopped")
	return nil
}

// SetMessageHandler sets the message handler in a thread-safe manner
func (t *TelegramBot) SetMessageHandler(handler func(BotMessage)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.messageHandler = handler
}

// GetMessageHandler gets the message handler in a thread-safe manner
func (t *TelegramBot) GetMessageHandler() func(BotMessage) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.messageHandler
}
