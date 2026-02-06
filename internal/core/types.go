package core

import "context"

// SessionState represents the current state of a session
type SessionState string

const (
	StateIdle         SessionState = "idle"            // Idle and ready for new tasks
	StateProcessing   SessionState = "processing"      // Currently processing a command
	StateWaitingInput SessionState = "waiting_input"   // Waiting for user input (mid-interaction)
	StateError        SessionState = "error"           // Error state
)

// Session represents a tmux session with its metadata
type Session struct {
	Name      string                // tmux session name
	CLIType   string                // claude/gemini/opencode
	WorkDir   string                // Working directory
	StartCmd  string                // Command to start the CLI (default: same as CLIType)
	State     SessionState         // Current state
	CreatedAt string                // Creation timestamp
	cancelCtx context.CancelFunc // Cancel function for active watchdog goroutine
}

// ResponseEvent represents a CLI response event
type ResponseEvent struct {
	SessionName string
	Response    string
	Timestamp   string
}

// Config represents the complete clibot configuration structure
type Config struct {
	HookServer    HookServerConfig        `yaml:"hook_server"`
	Security      SecurityConfig          `yaml:"security"`
	Watchdog      WatchdogConfig          `yaml:"watchdog"`
	Session       SessionGlobalConfig     `yaml:"session"`
	Sessions      []SessionConfig         `yaml:"sessions"`
	DefaultSession string                 `yaml:"default_session"`
	Bots          map[string]BotConfig    `yaml:"bots"`
	CLIAdapters   map[string]CLIAdapterConfig `yaml:"cli_adapters"`
	Logging       LoggingConfig           `yaml:"logging"`
}

// HookServerConfig represents HTTP Hook server configuration
type HookServerConfig struct {
	Port int `yaml:"port"`
}

// SecurityConfig represents security and access control configuration
type SecurityConfig struct {
	WhitelistEnabled bool                `yaml:"whitelist_enabled"`
	AllowedUsers     map[string][]string `yaml:"allowed_users"`
	Admins           map[string][]string `yaml:"admins"`
}

// WatchdogConfig represents watchdog monitoring configuration
type WatchdogConfig struct {
	Enabled        bool     `yaml:"enabled"`
	CheckIntervals []string `yaml:"check_intervals"`
	Timeout        string   `yaml:"timeout"`
	MaxRetries     int      `yaml:"max_retries"`
	InitialDelay   string   `yaml:"initial_delay"`
	RetryDelay     string   `yaml:"retry_delay"`
}

// SessionGlobalConfig represents global session configuration
type SessionGlobalConfig struct {
	InputHistorySize int `yaml:"input_history_size"` // Maximum number of input history entries to keep (default: 10)
}

// SessionConfig represents a session configuration
type SessionConfig struct {
	Name      string `yaml:"name"`
	CLIType   string `yaml:"cli_type"`
	WorkDir   string `yaml:"work_dir"`
	AutoStart bool   `yaml:"auto_start"`
	StartCmd  string `yaml:"start_cmd"` // Command to start the CLI (default: same as CLIType)
}

// BotConfig represents bot configuration
type BotConfig struct {
	Enabled           bool   `yaml:"enabled"`
	AppID             string `yaml:"app_id"`
	AppSecret         string `yaml:"app_secret"`
	Token             string `yaml:"token"`
	ChannelID         string `yaml:"channel_id"`       // For Discord: server channel ID
	EncryptKey        string `yaml:"encrypt_key"`      // Feishu: event encryption key (optional)
	VerificationToken string `yaml:"verification_token"` // Feishu: verification token (optional)
}

// CLIAdapterConfig represents CLI adapter configuration
type CLIAdapterConfig struct {
	// Deprecated fields - kept for backward compatibility but not used
	HistoryDir  string `yaml:"history_dir"`  // Deprecated: Not used anymore
	HistoryDB   string `yaml:"history_db"`   // Deprecated: Not used anymore
	HistoryFile string `yaml:"history_file"` // Deprecated: Not used anymore

	PollTimeout string `yaml:"poll_timeout"` // Polling mode timeout (e.g., "60s")

	// Polling mode configuration (alternative to hook mode)
	UseHook      bool   `yaml:"use_hook"`      // Use hook mode (true) or polling mode (false). Default: true
	PollInterval string `yaml:"poll_interval"` // Polling interval (e.g., "1s"). Default: "1s"
	StableCount   int    `yaml:"stable_count"`  // Consecutive stable checks required. Default: 3
}

// LoggingConfig represents logging configuration
type LoggingConfig struct {
	Level        string `yaml:"level"`         // debug, info, warn, error
	File         string `yaml:"file"`          // Log file path
	MaxSize      int    `yaml:"max_size"`      // Single file max size in MB (default: 100)
	MaxBackups   int    `yaml:"max_backups"`   // Number of backups to keep (default: 5)
	MaxAge       int    `yaml:"max_age"`       // Maximum days to retain (default: 30)
	Compress     bool   `yaml:"compress"`      // Whether to compress old logs (default: true)
	EnableStdout bool   `yaml:"enable_stdout"` // Also output to stdout (default: true)
}
