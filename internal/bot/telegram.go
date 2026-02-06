package bot

import (
	"context"
	"fmt"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/keepmind9/clibot/internal/logger"
	"github.com/keepmind9/clibot/pkg/constants"
	"github.com/sirupsen/logrus"
)

// TelegramBot implements BotAdapter interface for Telegram using long polling
type TelegramBot struct {
	mu             sync.RWMutex
	token          string
	bot            *tgbotapi.BotAPI
	messageHandler func(BotMessage)
	ctx            context.Context
	cancel         context.CancelFunc
}

// NewTelegramBot creates a new Telegram bot instance
func NewTelegramBot(token string) *TelegramBot {
	return &TelegramBot{
		token: token,
	}
}

// Start establishes long polling connection to Telegram and begins listening for messages
func (t *TelegramBot) Start(messageHandler func(BotMessage)) error {
	t.SetMessageHandler(messageHandler)
	t.ctx, t.cancel = context.WithCancel(context.Background())

	logger.WithFields(logrus.Fields{
		"token": maskSecret(t.token),
	}).Info("starting-telegram-bot-with-long-polling")

	// Initialize Telegram bot
	bot, err := tgbotapi.NewBotAPI(t.token)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"error": err,
		}).Error("failed-to-initialize-telegram-bot")
		return fmt.Errorf("failed to initialize Telegram bot: %w", err)
	}

	t.mu.Lock()
	t.bot = bot
	t.mu.Unlock()

	logger.WithFields(logrus.Fields{
		"bot_username": bot.Self.UserName,
		"bot_id":       bot.Self.ID,
	}).Info("telegram-bot-initialized-successfully")

	// Set up long polling configuration
	u := tgbotapi.NewUpdate(0)
	u.Timeout = int(constants.DefaultPollTimeout.Seconds()) // Long poll timeout in seconds

	// Start receiving updates via long polling
	updates := bot.GetUpdatesChan(u)

	// Process updates in background
	go func() {
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

				if update.Message != nil {
					t.handleMessage(update.Message)
				}
			}
		}
	}()

	logger.Info("telegram-long-polling-connection-started")
	return nil
}

// handleMessage handles incoming message events from Telegram
func (t *TelegramBot) handleMessage(message *tgbotapi.Message) {
	if message == nil {
		return
	}

	// Extract message information
	var userID, chatID, content string
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

	if message.Text != "" {
		content = message.Text
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

	// Only process text messages
	if message.Text != "" {
		// Call the handler with BotMessage
		handler := t.GetMessageHandler()
		if handler != nil {
			handler(BotMessage{
				Platform:  "telegram",
				UserID:    userID,
				Channel:   chatID,
				Content:   content,
				Timestamp: time.Now(),
			})
		}
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

	// Create message
	msg := tgbotapi.NewMessage(chatIDInt, message)
	msg.ParseMode = "Markdown" // Support markdown formatting

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
