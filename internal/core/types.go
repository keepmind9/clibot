package core

import (
	"context"
	"time"

	"github.com/keepmind9/clibot/internal/cli/stdio"
)

// SessionState represents the current state of a session
type SessionState string

const (
	StateIdle         SessionState = "idle"          // Idle and ready for new tasks
	StateProcessing   SessionState = "processing"    // Currently processing a command
	StateWaitingInput SessionState = "waiting_input" // Waiting for user input (mid-interaction)
	StateError        SessionState = "error"         // Error state
)

// Session represents a tmux session with its metadata
type Session struct {
	ID           int                `json:"id"`                     // Numeric ID for quick reference (stable, never reassigned)
	Name         string             `json:"name"`                   // tmux session name
	CLIType      string             `json:"cli_type"`               // claude/gemini/opencode
	WorkDir      string             `json:"work_dir"`               // Working directory
	StartCmd     string             `json:"start_cmd"`              // Command to start the CLI (default: same as CLIType)
	State        SessionState       `json:"-"`                      // Current state (not persisted)
	CreatedAt    string             `json:"created_at"`             // Creation timestamp
	LastActiveAt time.Time          `json:"last_active_at"`         // Last activity timestamp (input sent or response received)
	IsDynamic    bool               `json:"is_dynamic"`             // true if session was created dynamically via IM
	CreatedBy    string             `json:"created_by"`             // creator identity (format: "platform:userID")
	Env          map[string]string  `json:"env,omitempty"`          // Environment variables for the session
	LastChannel  *BotChannelRef     `json:"last_channel,omitempty"` // Last active bot channel for response routing
	cancelCtx    context.CancelFunc `json:"-"`                      // Cancel function for active watchdog goroutine
}

// BotChannelRef is the persistable subset of BotChannel.
type BotChannelRef struct {
	Platform string `json:"platform"`
	Channel  string `json:"channel"`
}

// NeedsWatchdog returns true if session requires watchdog monitoring
// ACP and stdio sessions handle responses asynchronously via callbacks,
// so they don't need watchdog (tmux polling or hook waiting)
func (s *Session) NeedsWatchdog() bool {
	return s.CLIType != "acp" && !stdio.IsStdioCLIType(s.CLIType)
}

// ResponseEvent represents a CLI response event
type ResponseEvent struct {
	SessionName string
	Response    string
	Timestamp   string
}

// Config represents the complete clibot configuration structure
type Config struct {
	HookServer       HookServerConfig            `yaml:"hook_server"`
	Security         SecurityConfig              `yaml:"security"`
	Watchdog         WatchdogConfig              `yaml:"watchdog"`
	Session          SessionGlobalConfig         `yaml:"session"`
	Sessions         []SessionConfig             `yaml:"sessions"`
	SessionTemplates map[string]SessionTemplate  `yaml:"session_templates"`
	Bots             map[string]BotConfig        `yaml:"bots"`
	CLIAdapters      map[string]CLIAdapterConfig `yaml:"cli_adapters"`
	Logging          LoggingConfig               `yaml:"logging"`
	Proxy            ProxyConfig                 `yaml:"proxy"`
	DataDir          string                      `yaml:"data_dir"` // Directory for persistent state (default: ~/.clibot)
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
	MaxDynamicSessions int    `yaml:"max_dynamic_sessions"` // Maximum number of dynamic sessions allowed (default: 50)
	IdleTimeout        string `yaml:"idle_timeout"`         // Auto-cleanup idle dynamic sessions (default: 1h, "0" to disable)
}

// SessionConfig represents a session configuration
type SessionConfig struct {
	Name      string            `yaml:"name"`
	CLIType   string            `yaml:"cli_type"`
	WorkDir   string            `yaml:"work_dir"`
	AutoStart bool              `yaml:"auto_start"`
	StartCmd  string            `yaml:"start_cmd"` // Command to start the CLI (default: same as CLIType)
	Transport string            `yaml:"transport"` // Connection URL for ACP: stdio://, tcp://host:port, unix:///path (for acp cli_type only)
	Env       map[string]string `yaml:"env"`       // Session-level environment variables (merged with adapter-level env)
}

// BotConfig represents bot configuration
type BotConfig struct {
	Enabled           bool         `yaml:"enabled"`
	AppID             string       `yaml:"app_id"`
	AppSecret         string       `yaml:"app_secret"`
	Token             string       `yaml:"token"`
	ChannelID         string       `yaml:"channel_id"`         // For Discord: server channel ID
	EncryptKey        string       `yaml:"encrypt_key"`        // Feishu: event encryption key (optional)
	VerificationToken string       `yaml:"verification_token"` // Feishu: verification token (optional)
	BaseURL           string       `yaml:"base_url"`           // WeChat iLink: API base URL (optional)
	CredentialsPath   string       `yaml:"credentials_path"`   // WeChat iLink: credentials file path (optional)
	Proxy             *ProxyConfig `yaml:"proxy"`              // Optional bot-level proxy override
	// Feishu-specific rich channel configuration
	MentionInGroup   *bool  `yaml:"mention_in_group"`   // Require @bot in groups to respond (default: true when nil)
	DebounceMs       int    `yaml:"debounce_ms"`        // Message debounce window in ms (0 = disabled)
	MaxMessageLength int    `yaml:"max_message_length"` // Max chars per message before splitting by lines (0 = no limit)
}

// CLIAdapterConfig represents CLI adapter configuration
type CLIAdapterConfig struct {
	// Timeout configuration
	// - Hook mode: maximum time to wait for response after hook triggers
	// - ACP mode: idle timeout (max time without activity before cancelling)
	// Default: 5 minutes for ACP, 1 hour for hook mode
	Timeout string `yaml:"timeout"`

	// Environment variables to set for the CLI process
	Env map[string]string `yaml:"env"`
}

// SessionTemplate defines a reusable blueprint for creating sessions via IM.
// Built-in defaults: codex, claude, gemini, opencode (zero config needed).
type SessionTemplate struct {
	CLIType  string            `yaml:"cli_type"`  // Required: e.g., codex-stdio, claude-stdio, acp
	StartCmd string            `yaml:"start_cmd"` // Optional: override start command (e.g., "claude-agent-acp")
	Yolo     bool              `yaml:"yolo"`      // Auto-approve all permission prompts
	Env      map[string]string `yaml:"env"`       // Template-level env vars
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

// ProxyConfig represents network proxy configuration
type ProxyConfig struct {
	Enabled  bool   `yaml:"enabled"`  // Whether proxy is enabled
	URL      string `yaml:"url"`      // Proxy URL (e.g., http://127.0.0.1:7890, socks5://127.0.0.1:1080)
	Username string `yaml:"username"` // Optional username for authentication
	Password string `yaml:"password"` // Optional password for authentication
}
