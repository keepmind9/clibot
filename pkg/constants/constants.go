package constants

import "time"

// Tmux capture line limits for different scenarios
const (
	// DefaultCaptureLines is used for hook retry: capture more lines for long responses
	DefaultCaptureLines = 200
	// DefaultManualCaptureLines is used for manual command: smaller for readability
	DefaultManualCaptureLines = 20
	// MaxTmuxCaptureLines is the upper limit for tmux capture
	MaxTmuxCaptureLines = 1000
)

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
)

// Timeouts and delays
const (
	// DefaultConnectionTimeout is the timeout for establishing connections
	DefaultConnectionTimeout = 2 * time.Second
	// DefaultPollTimeout is the timeout for long polling operations
	DefaultPollTimeout = 60 * time.Second
	// DefaultRetryDelay is the delay between retry attempts
	DefaultRetryDelay = 800 * time.Millisecond
	// DefaultInitialDelay is the initial delay before first operation
	DefaultInitialDelay = 500 * time.Millisecond
	// HookNotificationDelay is the delay for hook notification to send
	HookNotificationDelay = 300 * time.Millisecond
	// HookHTTPTimeout is the timeout for hook HTTP requests
	HookHTTPTimeout = 5 * time.Second
)

// Retry configuration
const (
	// MaxHookRetries is the maximum number of retry attempts for hook operations
	MaxHookRetries = 10
)

// Message buffer sizes
const (
	// MessageChannelBufferSize is the buffer size for the message channel
	MessageChannelBufferSize = 100
)

// Prompt and parsing limits
const (
	// MaxPromptPrefixLength is the maximum length used for prompt prefix matching
	MaxPromptPrefixLength = 30
	// ThinkingCheckLines is the number of recent lines to check for thinking state
	ThinkingCheckLines = 20
)

// Token masking
const (
	// MinTokenLengthForMasking is the minimum token length to apply masking
	MinTokenLengthForMasking = 10
	// TokenMaskPrefixLength is the length of prefix to show before masking
	TokenMaskPrefixLength = 7
	// TokenMaskSuffixLength is the length of suffix to show after masking
	TokenMaskSuffixLength = 4
)

// Logging defaults
const (
	// DefaultLogMaxSize is the default maximum log file size in MB
	DefaultLogMaxSize = 100
	// DefaultLogMaxAge is the default maximum number of days to retain old logs
	DefaultLogMaxAge = 30
	// HTTPSuccessStatusCode is the standard HTTP success status code
	HTTPSuccessStatusCode = 200
)

// AppID masking
const (
	// MinAppIDLengthForMasking is the minimum app ID length to apply masking
	MinAppIDLengthForMasking = 8
	// AppIDMaskPrefixLength is the length of prefix to show before masking
	AppIDMaskPrefixLength = 4
	// AppIDMaskSuffixLength is the length of suffix to show after masking
	AppIDMaskSuffixLength = 4
)
