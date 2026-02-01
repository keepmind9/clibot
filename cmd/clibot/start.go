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

	startCmd = &cobra.Command{
		Use:   "start",
		Short: "Start clibot main process",
		Long:  "Start clibot main process, listen to bot messages and dispatch to AI CLI tools",
		Run: func(cmd *cobra.Command, args []string) {
			// Load configuration
			config, err := core.LoadConfig(configFile)
			if err != nil {
				log.Fatalf("Failed to load config: %v", err)
			}

			fmt.Printf("Starting clibot with config: %s\n", configFile)
			fmt.Printf("Hook server port: %d\n", config.HookServer.Port)
			fmt.Printf("Command prefix: %s\n", config.CommandPrefix)
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

			// Wait for signal or engine error
			select {
			case sig := <-sigChan:
				log.Printf("\nReceived signal: %v, shutting down gracefully...", sig)
				if err := engine.Stop(); err != nil {
					log.Printf("Error during shutdown: %v", err)
				}
			case err := <-engineErrChan:
				if err != nil {
					log.Fatalf("Engine error: %v", err)
				}
			}

			log.Println("Clbot stopped")
		},
	}
)

func init() {
	startCmd.Flags().StringVarP(&configFile, "config", "c", "config.yaml", "Configuration file path")
}

// registerCLIAdapters registers all configured CLI adapters using factory pattern
func registerCLIAdapters(engine *core.Engine, config *core.Config) error {
	for cliType, cliConfig := range config.CLIAdapters {
		var adapter cli.CLIAdapter
		var err error

		// Parse polling configuration
		pollInterval, err := time.ParseDuration(cliConfig.PollInterval)
		if err != nil {
			return fmt.Errorf("failed to parse poll_interval for %s: %w", cliType, err)
		}

		pollTimeout, err := time.ParseDuration(cliConfig.PollTimeout)
		if err != nil {
			return fmt.Errorf("failed to parse poll_timeout for %s: %w", cliType, err)
		}

		switch cliType {
		case "claude":
			adapter, err = cli.NewClaudeAdapter(cli.ClaudeAdapterConfig{
				HistoryDir:   cliConfig.HistoryDir,
				CheckLines:   cliConfig.Interactive.CheckLines,
				Patterns:     cliConfig.Interactive.Patterns,
				UseHook:      cliConfig.UseHook,
				PollInterval: pollInterval,
				StableCount:  cliConfig.StableCount,
				PollTimeout:  pollTimeout,
			})
		case "gemini":
			adapter, err = cli.NewGeminiAdapter(cli.GeminiAdapterConfig{
				HistoryDir:   cliConfig.HistoryDir,
				CheckLines:   cliConfig.Interactive.CheckLines,
				Patterns:     cliConfig.Interactive.Patterns,
				UseHook:      cliConfig.UseHook,
				PollInterval: pollInterval,
				StableCount:  cliConfig.StableCount,
				PollTimeout:  pollTimeout,
			})
		case "opencode":
			adapter, err = cli.NewOpenCodeAdapter(cli.OpenCodeAdapterConfig{
				HistoryDir:   cliConfig.HistoryDir,
				CheckLines:   cliConfig.Interactive.CheckLines,
				Patterns:     cliConfig.Interactive.Patterns,
				UseHook:      cliConfig.UseHook,
				PollInterval: pollInterval,
				StableCount:  cliConfig.StableCount,
				PollTimeout:  pollTimeout,
			})
		default:
			log.Printf("Warning: CLI adapter type '%s' not implemented yet", cliType)
			continue
		}

		if err != nil {
			return fmt.Errorf("failed to create %s CLI adapter: %w", cliType, err)
		}

		engine.RegisterCLIAdapter(cliType, adapter)

		// Log mode
		mode := "hook"
		if !cliConfig.UseHook {
			mode = "polling"
		}
		log.Printf("Registered %s CLI adapter (mode: %s)", cliType, mode)
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
			botAdapter = bot.NewDiscordBot(botConfig.Token, botConfig.ChannelID)
			log.Printf("Registered %s bot adapter", botType)

		case "feishu":
			feishuBot := bot.NewFeishuBot(botConfig.AppID, botConfig.AppSecret)
			if botConfig.EncryptKey != "" {
				feishuBot.SetEncryptKey(botConfig.EncryptKey)
			}
			if botConfig.VerificationToken != "" {
				feishuBot.SetVerificationToken(botConfig.VerificationToken)
			}
			botAdapter = feishuBot
			log.Printf("Registered %s bot adapter (WebSocket long connection)", botType)

		case "dingtalk":
			botAdapter = bot.NewDingTalkBot(botConfig.AppID, botConfig.AppSecret)
			log.Printf("Registered %s bot adapter (WebSocket long connection)", botType)

		case "telegram":
			botAdapter = bot.NewTelegramBot(botConfig.Token)
			log.Printf("Registered %s bot adapter (long polling)", botType)

		default:
			log.Printf("Warning: Bot type '%s' not implemented yet", botType)
			continue
		}

		engine.RegisterBotAdapter(botType, botAdapter)
	}

	return nil
}
