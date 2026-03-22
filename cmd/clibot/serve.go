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
	"github.com/keepmind9/clibot/internal/cli"
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

			// Setup signal handling for graceful shutdown
			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

			// Create context for cancellation
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			// Start engine in a goroutine
			engineErrChan := make(chan error, 1)
			go func() {
				fmt.Println("clibot engine starting...")
				fmt.Println("Press Ctrl+C to stop")
				engineErrChan <- engine.Run(ctx)
			}()

			// Wait for signal or engine error
			select {
			case sig := <-sigChan:
				log.Printf("\nReceived signal: %v, shutting down gracefully...", sig)
				cancel() // Cancel context to stop event loop
				if err := engine.Stop(); err != nil {
					log.Printf("Error during shutdown: %v", err)
				}
			case err := <-engineErrChan:
				if err != nil {
					log.Fatalf("Engine error: %v", err)
				}
			}

			// Wait for engine to actually stop (with timeout via second Ctrl+C)
			select {
			case sig := <-sigChan:
				log.Printf("\nReceived second signal: %v, forcing shutdown...", sig)
				if err := engine.Stop(); err != nil {
					log.Printf("Error during forced shutdown: %v", err)
				}
			case err := <-engineErrChan:
				if err != nil {
					log.Fatalf("Engine error: %v", err)
				}
			}

			log.Println("Clibot stopped")
		},
	}
)

func init() {
	serveCmd.Flags().StringVarP(&configFile, "config", "c", "config.yaml", "Configuration file path")
}

// registerCLIAdapters registers all configured CLI adapters using factory pattern
func registerCLIAdapters(engine *core.Engine, config *core.Config) error {
	// Register ACP adapter (uses session-level config)
	// Check if any session uses ACP transport
	for _, session := range config.Sessions {
		if session.CLIType == "acp" && session.Transport != "" {
			// Get ACP configuration from global config
			acpConfig, ok := config.CLIAdapters["acp"]
			var idleTimeout time.Duration
			var err error

			// Get environment variables (nil if not configured)
			var env map[string]string
			if ok {
				env = acpConfig.Env
			}

			if ok && acpConfig.Timeout != "" {
				// Parse timeout if specified
				idleTimeout, err = time.ParseDuration(acpConfig.Timeout)
				if err != nil {
					return fmt.Errorf("failed to parse acp timeout: %w", err)
				}
			}
			// Use default max total timeout (1 hour)

			// Create ACP adapter with parsed configuration
			acpAdapter, err := cli.NewACPAdapter(cli.ACPAdapterConfig{
				IdleTimeout:     idleTimeout, // 0 = use default (5 min)
				MaxTotalTimeout: 0,           // 0 = use default (1 hour)
				Env:             env,         // Environment variables
			})
			if err != nil {
				return fmt.Errorf("failed to create ACP adapter: %w", err)
			}
			// Set engine reference for sending responses
			acpAdapter.SetEngine(engine)
			engine.RegisterCLIAdapter("acp", acpAdapter)

			// Only register once for all ACP sessions
			break
		}
	}

	// Register hook adapters
	for cliType, cliConfig := range config.CLIAdapters {
		var adapter cli.CLIAdapter
		var err error

		// Skip ACP adapter (already registered above)
		if cliType == "acp" {
			continue
		}

		switch cliType {
		case "claude":
			adapter, err = cli.NewClaudeAdapter(cli.ClaudeAdapterConfig{
				Env: cliConfig.Env,
			})
		case "gemini":
			adapter, err = cli.NewGeminiAdapter(cli.GeminiAdapterConfig{
				Env: cliConfig.Env,
			})
		case "opencode":
			adapter, err = cli.NewOpenCodeAdapter(cli.OpenCodeAdapterConfig{
				Env: cliConfig.Env,
			})
		default:
			log.Printf("Warning: CLI adapter type '%s' not implemented yet", cliType)
			continue
		}

		if err != nil {
			return fmt.Errorf("failed to create %s CLI adapter: %w", cliType, err)
		}

		engine.RegisterCLIAdapter(cliType, adapter)
		log.Printf("Registered %s CLI adapter (mode: hook)", cliType)
	}

	return nil
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
			feishuBot := bot.NewFeishuBot(botConfig.AppID, botConfig.AppSecret)
			if botConfig.EncryptKey != "" {
				feishuBot.SetEncryptKey(botConfig.EncryptKey)
			}
			if botConfig.VerificationToken != "" {
				feishuBot.SetVerificationToken(botConfig.VerificationToken)
			}
			feishuBot.SetProxyManager(engine.GetProxyManager())
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
