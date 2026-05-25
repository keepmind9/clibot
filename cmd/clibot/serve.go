package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/keepmind9/clibot/internal/bot"
	"github.com/keepmind9/clibot/internal/bot/feishu"
	"github.com/keepmind9/clibot/internal/cli"
	"github.com/keepmind9/clibot/internal/cli/stdio"
	"github.com/keepmind9/clibot/internal/core"
	"github.com/keepmind9/clibot/internal/logger"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	configFile string

	serveCmd = &cobra.Command{
		Use:   "serve",
		Short: "Run clibot as a service",
		Long:  "Run clibot as a service daemon, listening to bot messages and dispatching to AI CLI tools",
		Run: func(cmd *cobra.Command, args []string) {
			// Load configuration
			config, err := core.LoadConfig(configFile)
			if err != nil {
				log.Fatalf("Failed to load config: %v", err)
			}

			fmt.Printf("Running clibot service with config: %s\n", configFile)
			fmt.Printf("Hook server port: %d\n", config.HookServer.Port)
			fmt.Printf("Whitelist enabled: %v\n", config.Security.WhitelistEnabled)

			// Initialize logger
			logConfig := logger.Config{
				Level:        config.Logging.Level,
				File:         config.Logging.File,
				MaxSize:      config.Logging.MaxSize,
				MaxBackups:   config.Logging.MaxBackups,
				MaxAge:       config.Logging.MaxAge,
				Compress:     config.Logging.Compress,
				EnableStdout: config.Logging.EnableStdout,
			}
			if err := logger.InitLogger(logConfig); err != nil {
				log.Fatalf("Failed to initialize logger: %v", err)
			}

			logger.WithFields(logrus.Fields{
				"config_file": configFile,
				"log_level":   config.Logging.Level,
				"log_file":    config.Logging.File,
			}).Info("logger-initialized")

			// Create engine
			engine := core.NewEngine(config)

			// Register CLI adapters using factory pattern
			if err := registerCLIAdapters(engine, config); err != nil {
				log.Fatalf("Failed to register CLI adapters: %v", err)
			}

			// Register bot adapters using factory pattern
			if err := registerBotAdapters(engine, config); err != nil {
				log.Fatalf("Failed to register bot adapters: %v", err)
			}

			// Setup signal handling for graceful shutdown and config reload
			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

			// Create context for cancellation
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			engine.SetConfigPath(configFile)

			// Start engine in a goroutine
			engineErrChan := make(chan error, 1)
			go func() {
				fmt.Println("clibot engine starting...")
				fmt.Println("Press Ctrl+C to stop")
				engineErrChan <- engine.Run(ctx)
			}()

			// Main signal loop: SIGHUP reloads config, SIGINT/SIGTERM shuts down
		signalLoop:
			for {
				select {
				case sig := <-sigChan:
					if sig == syscall.SIGHUP {
						log.Printf("Received SIGHUP, reloading config from %s...", configFile)
						if err := engine.ReloadConfig(configFile); err != nil {
							log.Printf("Config reload failed: %v", err)
						} else {
							log.Println("Config reloaded successfully")
						}
						continue
					}
					// SIGINT or SIGTERM
					log.Printf("\nReceived signal: %v, shutting down gracefully...", sig)
					cancel()
					if err := engine.Stop(); err != nil {
						log.Printf("Error during shutdown: %v", err)
					}
					break signalLoop
				case err := <-engineErrChan:
					if err != nil {
						log.Fatalf("Engine error: %v", err)
					}
					break signalLoop
				}
			}

			// Wait for engine to actually stop (with timeout via second Ctrl+C)
			select {
			case sig := <-sigChan:
				if sig == syscall.SIGHUP {
					log.Println("Ignoring SIGHUP during shutdown")
				} else {
					log.Printf("\nReceived second signal: %v, forcing shutdown...", sig)
					if err := engine.Stop(); err != nil {
						log.Printf("Error during forced shutdown: %v", err)
					}
				}
			case <-engineErrChan:
			}

			log.Println("Clibot stopped")
		},
	}
)

func init() {
	serveCmd.Flags().StringVarP(&configFile, "config", "c", "config.yaml", "Configuration file path")
}

// registerCLIAdapters registers all CLI adapters. Every known type is registered
// with defaults so dynamic sessions (sn/snew) always work. Config overrides are
// applied when the corresponding entry exists in cli_adapters.
func registerCLIAdapters(engine *core.Engine, config *core.Config) error {
	// --- ACP adapter ---
	acpAdapter, err := cli.NewACPAdapter(cli.ACPAdapterConfig{})
	if err != nil {
		return fmt.Errorf("failed to create ACP adapter: %w", err)
	}
	acpAdapter.SetEngine(engine)
	engine.RegisterCLIAdapter("acp", acpAdapter)

	// --- Stdio adapters ---
	for _, cliType := range stdio.KnownStdioTypes() {
		spec := newStdioSpec(cliType)
		if spec == nil {
			continue
		}
		adapter := stdio.NewStdioAdapter(spec, stdio.StdioAdapterConfig{})
		adapter.SetEngine(engine)
		engine.RegisterCLIAdapter(cliType, adapter)
	}

	// --- Hook-mode adapters ---
	for _, name := range []string{"claude", "gemini", "opencode"} {
		var adapter cli.CLIAdapter
		var err error
		switch name {
		case "claude":
			adapter, err = cli.NewClaudeAdapter(cli.ClaudeAdapterConfig{})
		case "gemini":
			adapter, err = cli.NewGeminiAdapter(cli.GeminiAdapterConfig{})
		case "opencode":
			adapter, err = cli.NewOpenCodeAdapter(cli.OpenCodeAdapterConfig{})
		}
		if err != nil {
			return fmt.Errorf("failed to create %s adapter: %w", name, err)
		}
		engine.RegisterCLIAdapter(name, adapter)
	}

	// --- Apply config overrides ---
	// Config entries override the default adapters registered above.
	for cliType, cliConfig := range config.CLIAdapters {
		switch {
		case cliType == "acp":
			var idleTimeout time.Duration
			if cliConfig.Timeout != "" {
				if d, err := time.ParseDuration(cliConfig.Timeout); err == nil {
					idleTimeout = d
				}
			}
			acpAdapter, err := cli.NewACPAdapter(cli.ACPAdapterConfig{
				IdleTimeout:     idleTimeout,
				MaxTotalTimeout: 0,
				Env:             cliConfig.Env,
			})
			if err != nil {
				return fmt.Errorf("failed to create ACP adapter: %w", err)
			}
			acpAdapter.SetEngine(engine)
			engine.RegisterCLIAdapter("acp", acpAdapter)

		case stdio.IsStdioCLIType(cliType):
			spec := newStdioSpec(cliType)
			if spec == nil {
				continue
			}
			var permTimeout time.Duration
			if cliConfig.Timeout != "" {
				if d, err := time.ParseDuration(cliConfig.Timeout); err == nil {
					permTimeout = d
				}
			}
			adapter := stdio.NewStdioAdapter(spec, stdio.StdioAdapterConfig{
				PermissionTimeout: permTimeout,
				Env:               cliConfig.Env,
			})
			adapter.SetEngine(engine)
			engine.RegisterCLIAdapter(cliType, adapter)

		default:
			// Hook-mode override
			var adapter cli.CLIAdapter
			var err error
			switch cliType {
			case "claude":
				adapter, err = cli.NewClaudeAdapter(cli.ClaudeAdapterConfig{Env: cliConfig.Env})
			case "gemini":
				adapter, err = cli.NewGeminiAdapter(cli.GeminiAdapterConfig{Env: cliConfig.Env})
			case "opencode":
				adapter, err = cli.NewOpenCodeAdapter(cli.OpenCodeAdapterConfig{Env: cliConfig.Env})
			default:
				log.Printf("Warning: CLI adapter type '%s' not implemented yet", cliType)
				continue
			}
			if err != nil {
				return fmt.Errorf("failed to create %s adapter: %w", cliType, err)
			}
			engine.RegisterCLIAdapter(cliType, adapter)
		}
	}

	return nil
}

// newStdioSpec creates a CLISpec for the given stdio cli_type.
func newStdioSpec(cliType string) stdio.CLISpec {
	switch cliType {
	case "claude-stdio":
		return stdio.ClaudeSpec{}
	case "codex-stdio":
		return stdio.CodexSpec{}
	case "gemini-stdio":
		return stdio.GeminiSpec{}
	case "opencode-stdio":
		return stdio.OpenCodeSpec{}
	default:
		return nil
	}
}

// registerBotAdapters registers all configured bot adapters using factory pattern
func registerBotAdapters(engine *core.Engine, config *core.Config) error {
	for botType, botConfig := range config.Bots {
		if !botConfig.Enabled {
			log.Printf("Bot %s is disabled, skipping", botType)
			continue
		}

		var botAdapter bot.BotAdapter

		switch botType {
		case "discord":
			discordBot := bot.NewDiscordBot(botConfig.Token, botConfig.ChannelID)
			discordBot.SetProxyManager(engine.GetProxyManager())
			botAdapter = discordBot
			log.Printf("Registered %s bot adapter", botType)

		case "feishu":
			feishuBot := feishu.NewBot(botConfig.AppID, botConfig.AppSecret)
			if botConfig.EncryptKey != "" {
				feishuBot.SetEncryptKey(botConfig.EncryptKey)
			}
			if botConfig.VerificationToken != "" {
				feishuBot.SetVerificationToken(botConfig.VerificationToken)
			}
			feishuBot.SetProxyManager(engine.GetProxyManager())
			if botConfig.MentionInGroup != nil {
				feishuBot.SetMentionInGroup(*botConfig.MentionInGroup)
			} else {
				feishuBot.SetMentionInGroup(true)
			}
			if botConfig.DebounceMs > 0 {
				feishuBot.SetDebounceMs(botConfig.DebounceMs)
			}
			if botConfig.ReplyMode != "" && botConfig.ReplyMode != "text" && botConfig.ReplyMode != "card" {
				log.Fatalf("Invalid reply_mode %q for feishu bot, must be 'text' or 'card'", botConfig.ReplyMode)
			}
			if botConfig.MediaDir != "" {
				ttl := 24 * time.Hour
				if botConfig.MediaTTL != "" {
					if d, err := time.ParseDuration(botConfig.MediaTTL); err == nil {
						ttl = d
					}
				}
				var maxSize int64
				if botConfig.MaxMediaSize > 0 {
					maxSize = int64(botConfig.MaxMediaSize)
				}
				feishuBot.SetMediaConfig(botConfig.MediaDir, ttl, maxSize)
			}
			botAdapter = feishuBot
			log.Printf("Registered %s bot adapter (WebSocket long connection)", botType)

		case "dingtalk":
			dingtalkBot := bot.NewDingTalkBot(botConfig.AppID, botConfig.AppSecret)
			dingtalkBot.SetProxyManager(engine.GetProxyManager())
			botAdapter = dingtalkBot
			log.Printf("Registered %s bot adapter (WebSocket long connection)", botType)

		case "telegram":
			telegramBot := bot.NewTelegramBot(botConfig.Token)
			telegramBot.SetProxyManager(engine.GetProxyManager())
			botAdapter = telegramBot
			log.Printf("Registered %s bot adapter (long polling)", botType)

		case "qq":
			qqBot := bot.NewQQBot(botConfig.AppID, botConfig.AppSecret)
			qqBot.SetProxyManager(engine.GetProxyManager())
			botAdapter = qqBot
			log.Printf("Registered %s bot adapter (WebSocket long connection)", botType)

		case "weixin":
			baseURL := botConfig.BaseURL
			if baseURL == "" {
				baseURL = bot.DefaultBaseURL
			}
			credPath := botConfig.CredentialsPath
			if credPath == "" {
				credPath = bot.DefaultCredentialsPath()
			}
			weixinBot := bot.NewWeixinBot(baseURL, credPath)
			weixinBot.SetProxyManager(engine.GetProxyManager())
			botAdapter = weixinBot
			log.Printf("Registered %s bot adapter (QR login + long polling)", botType)

		default:
			log.Printf("Warning: Bot type '%s' not implemented yet", botType)
			continue
		}

		engine.RegisterBotAdapter(botType, botAdapter)
	}

	return nil
}
