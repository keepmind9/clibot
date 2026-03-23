package constants

import "time"

// Message length limits for different platforms
const (
	// MaxDiscordMessageLength is Discord's message character limit
	MaxDiscordMessageLength = 2000
	// MaxTelegramMessageLength is Telegram's message character limit
	MaxTelegramMessageLength = 4096
	// MaxFeishuMessageLength is Feishu's message character limit
	MaxFeishuMessageLength = 20000
	// MaxDingTalkMessageLength is DingTalk's message character limit
	MaxDingTalkMessageLength = 20000
	// MaxWeixinMessageLength is WeChat iLink's message character limit
	MaxWeixinMessageLength = 2000
)

// Timeouts and delays
const (
	// DefaultConnectionTimeout is the timeout for establishing connections
	DefaultConnectionTimeout = 2 * time.Second
	// TelegramLongPollTimeout is the long polling timeout for Telegram
	// Must be less than proxy DefaultHTTPClientTimeout (30s)
	TelegramLongPollTimeout = 20 * time.Second
	// WechatLongPollTimeout is the long polling timeout for WeChat iLink
	// Must be less than proxy DefaultHTTPClientTimeout (30s)
	WechatLongPollTimeout = 20 * time.Second
	// HookNotificationDelay is the delay for hook notification to send
	HookNotificationDelay = 300 * time.Millisecond
	// HookHTTPTimeout is the timeout for hook HTTP requests
	HookHTTPTimeout = 5 * time.Second
	// TypingIndicatorTimeout is the timeout for typing indicator HTTP requests
	TypingIndicatorTimeout = 5 * time.Second
	// TypingIndicatorRemoveDelay is the delay before removing typing indicator after sending response
	TypingIndicatorRemoveDelay = 500 * time.Millisecond
	// DingTalkMessageSendTimeout is the timeout for sending messages to DingTalk
	DingTalkMessageSendTimeout = 10 * time.Second
)

// Message buffer sizes
const (
	// MessageChannelBufferSize is the buffer size for the message channel
	MessageChannelBufferSize = 100
)

// Secret masking
const (
	// MinSecretLengthForMasking is the minimum secret length to apply masking
	MinSecretLengthForMasking = 10
	// SecretMaskPrefixLength is the length of prefix to show before masking
	SecretMaskPrefixLength = 4
	// SecretMaskSuffixLength is the length of suffix to show after masking
	SecretMaskSuffixLength = 4
)

// HTTP status codes
const (
	// HTTPSuccessStatusCode is the standard HTTP success status code
	HTTPSuccessStatusCode = 200
)
