// Package core provides the central engine and configuration management for clibot.
//
// The core package implements the main orchestration logic that connects IM platforms
// with AI CLI tools. It handles:
//
//   - Configuration loading and validation (from YAML files)
//   - Session management for CLI tools
//   - Message routing between bots and CLI adapters
//   - HTTP hook server for receiving CLI notifications
//   - Graceful shutdown and cleanup
//
// # Main Components
//
//   - Engine: Central orchestration engine
//   - Config: Configuration structure and loading
//   - Session: CLI session state management
//
// # Configuration
//
// Configuration is loaded from a YAML file with the following main sections:
//
//   - hook_server: HTTP server settings
//   - sessions: CLI tool sessions to manage
//   - bots: IM platform bot configurations
//   - cli_adapters: CLI tool adapter configurations
//   - security: Access control and whitelisting
//   - logging: Log configuration
//
// # Example Configuration
//
//   hook_server:
//     port: 8080
//   sessions:
//     - name: "my-session"
//       cli_type: "claude"
//       work_dir: "/path/to/project"
//   bots:
//     discord:
//       enabled: true
//       token: "your-bot-token"
//   cli_adapters:
//     claude:
//       history_dir: "~/.config/claude"
//
package core

import (
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	DefaultHookPort       = 8080
	DefaultLogLevel       = "info"
	DefaultLogMaxSize     = 100 // MB
	DefaultLogMaxBackups  = 5
	DefaultLogMaxAge      = 30  // days
	DefaultLogCompress    = true
	DefaultLogEnableStdout = true

	// Default timeout values
	DefaultWatchdogMaxRetries   = 10
	DefaultWatchdogInitialDelay = "500ms"
	DefaultWatchdogRetryDelay   = "800ms"
	DefaultTimeout             = "2s"
	DefaultPollTimeout         = "60s"

	// Default polling mode values
	DefaultPollInterval = "1s"  // Poll every 1 second
	DefaultStableCount   = 3    // Require 3 consecutive stable checks
)

// LoadConfig loads configuration from file and expands environment variables
func LoadConfig(configPath string) (*Config, error) {
	// Read configuration file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Expand environment variables
	expandedData, err := expandEnv(string(data))
	if err != nil {
		return nil, fmt.Errorf("failed to expand environment variables: %w", err)
	}

	// Parse YAML
	var config Config
	if err := yaml.Unmarshal([]byte(expandedData), &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Validate configuration
	if err := validateConfig(&config); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &config, nil
}

// expandEnv replaces ${VAR_NAME} patterns with environment variable values
func expandEnv(input string) (string, error) {
	var missingVars []string

	result := os.Expand(input, func(key string) string {
		if val := os.Getenv(key); val != "" {
			return val
		}
		missingVars = append(missingVars, key)
		return "" // Return empty string to let config parsing fail
	})

	if len(missingVars) > 0 {
		return "", fmt.Errorf("missing required environment variables: %s",
			strings.Join(missingVars, ", "))
	}

	return result, nil
}

// validateConfig performs basic validation on the configuration
func validateConfig(config *Config) error {
	// Validate hook server port
	if config.HookServer.Port == 0 {
		config.HookServer.Port = DefaultHookPort
	}

	// Set default logging configuration
	if config.Logging.Level == "" {
		config.Logging.Level = DefaultLogLevel
	}
	if config.Logging.MaxSize == 0 {
		config.Logging.MaxSize = DefaultLogMaxSize
	}
	if config.Logging.MaxBackups == 0 {
		config.Logging.MaxBackups = DefaultLogMaxBackups
	}
	if config.Logging.MaxAge == 0 {
		config.Logging.MaxAge = DefaultLogMaxAge
	}
	if !config.Logging.Compress {
		config.Logging.Compress = DefaultLogCompress
	}
	if !config.Logging.EnableStdout {
		config.Logging.EnableStdout = DefaultLogEnableStdout
	}

	// Set default watchdog configuration
	if config.Watchdog.MaxRetries == 0 {
		config.Watchdog.MaxRetries = DefaultWatchdogMaxRetries
	}
	if config.Watchdog.InitialDelay == "" {
		config.Watchdog.InitialDelay = DefaultWatchdogInitialDelay
	}
	if config.Watchdog.RetryDelay == "" {
		config.Watchdog.RetryDelay = DefaultWatchdogRetryDelay
	}

	// Set default timeout values for CLI adapters
	for cliType, adapter := range config.CLIAdapters {
		if adapter.Timeout == "" {
			adapter.Timeout = DefaultTimeout
		}
		if adapter.PollTimeout == "" {
			adapter.PollTimeout = DefaultPollTimeout
		}
		if adapter.PollInterval == "" {
			adapter.PollInterval = DefaultPollInterval
		}
		if adapter.StableCount == 0 {
			adapter.StableCount = DefaultStableCount
		}
		// UseHook defaults to true if not explicitly set in YAML
		// Note: bool zero value is false, so we need to check if it was explicitly set
		// For now, the adapter constructors will use the config value as-is
		// This will be validated below

		config.CLIAdapters[cliType] = adapter
	}

	// Validate polling configuration and set UseHook default
	for cliType, adapter := range config.CLIAdapters {
		if !adapter.UseHook {
			// Polling mode - validate parameters
			interval, err := time.ParseDuration(adapter.PollInterval)
			if err != nil {
				return fmt.Errorf("invalid poll_interval for %s: %w", cliType, err)
			}
			if interval < 100*time.Millisecond {
				return fmt.Errorf("poll_interval for %s must be at least 100ms (got %v)", cliType, interval)
			}
			if interval > 60*time.Second {
				return fmt.Errorf("poll_interval for %s is too large (max 60s, got %v)", cliType, interval)
			}

			timeout, err := time.ParseDuration(adapter.PollTimeout)
			if err != nil {
				return fmt.Errorf("invalid poll_timeout for %s: %w", cliType, err)
			}
			if timeout < interval {
				return fmt.Errorf("poll_timeout for %s must be greater than poll_interval", cliType)
			}
			if timeout > 10*time.Minute {
				return fmt.Errorf("poll_timeout for %s is too large (max 10m, got %v)", cliType, timeout)
			}

			if adapter.StableCount < 1 || adapter.StableCount > 20 {
				return fmt.Errorf("stable_count for %s must be between 1 and 20 (got %d)", cliType, adapter.StableCount)
			}

			// Validate that timeout is sufficient for stable_count
			// Minimum timeout should be at least (stable_count + 1) * interval
			minimumTimeout := time.Duration(adapter.StableCount+2) * interval
			if timeout < minimumTimeout {
				return fmt.Errorf("poll_timeout for %s must be at least %v (interval * (stable_count + 2)), got %v",
					cliType, minimumTimeout, timeout)
			}
		}
	}

	// Validate security settings
	if config.Security.WhitelistEnabled {
		if len(config.Security.AllowedUsers) == 0 {
			return fmt.Errorf("security.allowed_users cannot be empty when whitelist is enabled")
		}
	}

	// Validate at least one bot is configured
	if len(config.Bots) == 0 {
		return fmt.Errorf("at least one bot must be configured")
	}

	// Validate at least one session is configured
	if len(config.Sessions) == 0 {
		return fmt.Errorf("at least one session must be configured")
	}

	// Set default session if not specified
	if config.DefaultSession == "" && len(config.Sessions) > 0 {
		config.DefaultSession = config.Sessions[0].Name
	}

	// Validate that default_session references an existing session
	if config.DefaultSession != "" {
		found := false
		for _, session := range config.Sessions {
			if session.Name == config.DefaultSession {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("default_session '%s' does not exist in sessions configuration", config.DefaultSession)
		}
	}

	return nil
}

// GetBotConfig retrieves configuration for a specific bot
func (c *Config) GetBotConfig(botType string) (BotConfig, error) {
	bot, exists := c.Bots[botType]
	if !exists {
		return BotConfig{}, fmt.Errorf("bot type %s not found in configuration", botType)
	}

	if !bot.Enabled {
		return BotConfig{}, fmt.Errorf("bot type %s is disabled", botType)
	}

	return bot, nil
}

// GetCLIAdapterConfig retrieves configuration for a specific CLI adapter
func (c *Config) GetCLIAdapterConfig(cliType string) (CLIAdapterConfig, error) {
	adapter, exists := c.CLIAdapters[cliType]
	if !exists {
		return CLIAdapterConfig{}, fmt.Errorf("CLI adapter %s not found in configuration", cliType)
	}

	// Note: HistoryDir, HistoryDB, HistoryFile are deprecated and no longer processed
	// They are kept in the struct for backward compatibility with existing config files

	return adapter, nil
}

// expandHome expands ~ to user's home directory
func expandHome(path string) (string, error) {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get home directory: %w", err)
		}
		return home + path[1:], nil
	}
	return path, nil
}

// GetSessionConfig retrieves configuration for a specific session
func (c *Config) GetSessionConfig(sessionName string) (SessionConfig, error) {
	for _, session := range c.Sessions {
		if session.Name == sessionName {
			return session, nil
		}
	}
	return SessionConfig{}, fmt.Errorf("session %s not found in configuration", sessionName)
}

// IsUserAuthorized checks if a user is in the whitelist
func (c *Config) IsUserAuthorized(platform, userID string) bool {
	// If whitelist is disabled, allow all users (warning: not recommended for production)
	if !c.Security.WhitelistEnabled {
		return true
	}

	// Get allowed users for this platform
	userIDs, exists := c.Security.AllowedUsers[platform]
	if !exists {
		return false
	}

	// Check if user is in the whitelist
	for _, uid := range userIDs {
		if uid == userID {
			return true
		}
	}

	return false
}

// IsAdmin checks if a user is an admin
func (c *Config) IsAdmin(platform, userID string) bool {
	admins, exists := c.Security.Admins[platform]
	if !exists {
		return false
	}

	for _, adminID := range admins {
		if adminID == userID {
			return true
		}
	}

	return false
}
