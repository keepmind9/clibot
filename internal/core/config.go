package core

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	DefaultHookPort       = 8080
	DefaultCommandPrefix  = "!!"
	DefaultLogLevel       = "info"
	DefaultLogMaxSize     = 100 // MB
	DefaultLogMaxBackups  = 5
	DefaultLogMaxAge      = 30  // days
	DefaultLogCompress    = true
	DefaultLogEnableStdout = true
)

// LoadConfig loads configuration from file and expands environment variables
func LoadConfig(configPath string) (*Config, error) {
	// Read configuration file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Expand environment variables
	expandedData := expandEnv(string(data))

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
func expandEnv(input string) string {
	return os.Expand(input, func(key string) string {
		// Support both ${VAR} and $VAR formats
		if val := os.Getenv(key); val != "" {
			return val
		}
		return "${" + key + "}"
	})
}

// validateConfig performs basic validation on the configuration
func validateConfig(config *Config) error {
	// Validate hook server port
	if config.HookServer.Port == 0 {
		config.HookServer.Port = DefaultHookPort
	}

	// Validate command prefix
	if config.CommandPrefix == "" {
		config.CommandPrefix = DefaultCommandPrefix
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

	// Expand home directory in paths
	adapter.HistoryDir = expandHome(adapter.HistoryDir)
	adapter.HistoryDB = expandHome(adapter.HistoryDB)
	adapter.HistoryFile = expandHome(adapter.HistoryFile)

	return adapter, nil
}

// expandHome expands ~ to user's home directory
func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			// If we can't get home directory, return original path
			// This allows the system to continue with potentially invalid paths
			// that will be caught by actual usage
			return path
		}
		return home + path[1:]
	}
	return path
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
