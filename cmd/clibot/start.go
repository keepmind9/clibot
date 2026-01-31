package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

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
			}).Info("Logger initialized")

			// Create engine
			engine := core.NewEngine(config)

			// Register CLI adapters
			for cliType, cliConfig := range config.CLIAdapters {
				switch cliType {
				case "claude":
					claudeAdapter, err := cli.NewClaudeAdapter(cli.ClaudeAdapterConfig{
						HistoryDir: cliConfig.HistoryDir,
						CheckLines: cliConfig.Interactive.CheckLines,
						Patterns:   cliConfig.Interactive.Patterns,
					})
					if err != nil {
						log.Fatalf("Failed to create Claude CLI adapter: %v", err)
					}
					engine.RegisterCLIAdapter(cliType, claudeAdapter)
					log.Printf("Registered %s CLI adapter", cliType)

				case "gemini":
					geminiAdapter, err := cli.NewGeminiAdapter(cli.GeminiAdapterConfig{
						HistoryDir: cliConfig.HistoryDir,
						CheckLines: cliConfig.Interactive.CheckLines,
						Patterns:   cliConfig.Interactive.Patterns,
					})
					if err != nil {
						log.Fatalf("Failed to create Gemini CLI adapter: %v", err)
					}
					engine.RegisterCLIAdapter(cliType, geminiAdapter)
					log.Printf("Registered %s CLI adapter", cliType)

				// Add other CLI adapters (opencode) when implemented
				default:
					log.Printf("Warning: CLI adapter type '%s' not implemented yet", cliType)
				}
			}

			// Register bot adapters
			for botType, botConfig := range config.Bots {
				if !botConfig.Enabled {
					log.Printf("Bot %s is disabled, skipping", botType)
					continue
				}

				switch botType {
				case "discord":
					discordBot := bot.NewDiscordBot(botConfig.Token, botConfig.ChannelID)
					engine.RegisterBotAdapter(botType, discordBot)
					log.Printf("Registered %s bot adapter", botType)

				case "feishu":
					feishuBot := bot.NewFeishuBot(botConfig.AppID, botConfig.AppSecret)
					// Set optional encryption fields if provided
					if botConfig.EncryptKey != "" {
						feishuBot.EncryptKey = botConfig.EncryptKey
					}
					if botConfig.VerificationToken != "" {
						feishuBot.VerificationToken = botConfig.VerificationToken
					}
					engine.RegisterBotAdapter(botType, feishuBot)
					log.Printf("Registered %s bot adapter (WebSocket long connection)", botType)

				// TODO: Add other bot adapters (telegram) when implemented
				default:
					log.Printf("Warning: Bot type '%s' not implemented yet", botType)
				}
			}

			// Setup signal handling for graceful shutdown
			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

			// Start engine in a goroutine
			engineErrChan := make(chan error, 1)
			go func() {
				fmt.Println("\nclibot engine starting...")
				fmt.Println("Press Ctrl+C to stop\n")
				engineErrChan <- engine.Run()
			}()

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
