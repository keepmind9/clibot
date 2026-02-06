package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/keepmind9/clibot/internal/core"
	"github.com/spf13/cobra"
)

var (
	validateConfig string
	validateShow   bool
	validateJSON   bool
)

// ValidationResult represents the validation result
type ValidationResult struct {
	Valid    bool     `json:"valid"`
	Config   string   `json:"config"`
	Sessions int      `json:"sessions"`
	Adapters int      `json:"adapters"`
	Bots     int      `json:"bots"`
	Errors   []string `json:"errors,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate clibot configuration file",
	Long: `Validate the clibot configuration file without starting the service.

This command checks:
  - YAML syntax
  - Required fields
  - Session configuration
  - Bot credentials
  - CLI adapter settings

Exit codes:
  0 - Configuration is valid
  1 - Configuration has errors`,
	Run: func(cmd *cobra.Command, args []string) {
		// Get config file path
		configFile := validateConfig
		if configFile == "" {
			// Try default locations
			for _, loc := range []string{
				"config.yaml",
				filepath.Join(os.Getenv("HOME"), ".config/clibot/config.yaml"),
				"/etc/clibot/config.yaml",
			} {
				if _, err := os.Stat(loc); err == nil {
					configFile = loc
					break
				}
			}
		}

		if configFile == "" {
			fmt.Println("❌ No configuration file found")
			fmt.Println("\nSpecify a config file with --config or ensure one exists at:")
			fmt.Println("  - ./config.yaml")
			fmt.Println("  - ~/.config/clibot/config.yaml")
			fmt.Println("  - /etc/clibot/config.yaml")
			os.Exit(1)
		}

		// Load configuration
		cfg, err := core.LoadConfig(configFile)
		if err != nil {
			result := ValidationResult{
				Valid:  false,
				Config: configFile,
				Errors: []string{err.Error()},
			}
			outputValidationResult(result, validateJSON)
			os.Exit(1)
		}

		// Collect validation results
		result := ValidationResult{
			Valid:    true,
			Config:   configFile,
			Sessions: len(cfg.Sessions),
			Adapters: len(cfg.CLIAdapters),
			Bots:     len(cfg.Bots),
			Warnings: []string{},
		}

		// Perform additional validation checks
		warnings := validateConfigDetails(cfg)
		result.Warnings = warnings

		if len(warnings) > 0 {
			result.Valid = false
		}

		// Show full config if requested
		if validateShow {
			fmt.Printf("✓ Configuration loaded: %s\n\n", configFile)
			fmt.Printf("Sessions (%d):\n", len(cfg.Sessions))
			for _, session := range cfg.Sessions {
				autoStart := "no"
				if session.AutoStart {
					autoStart = "yes"
				}
				fmt.Printf("  - %s: %s @ %s (auto_start: %s)\n",
					session.Name, session.CLIType, session.WorkDir, autoStart)
			}
			fmt.Printf("\nCLI Adapters (%d):\n", len(cfg.CLIAdapters))
			for name := range cfg.CLIAdapters {
				fmt.Printf("  - %s\n", name)
			}
			fmt.Printf("\nBots (%d):\n", len(cfg.Bots))
			for name, bot := range cfg.Bots {
				status := "disabled"
				if bot.Enabled {
					status = "enabled"
				}
				fmt.Printf("  - %s: %s\n", name, status)
			}
			fmt.Println()
		}

		outputValidationResult(result, validateJSON)

		// Exit with appropriate code
		if !result.Valid {
			os.Exit(1)
		}
	},
}

func outputValidationResult(result ValidationResult, jsonFormat bool) {
	if jsonFormat {
		output, err := json.Marshal(result)
		if err != nil {
			fmt.Printf("{\"error\": \"failed to marshal json: %v\"}\n", err)
			return
		}
		fmt.Println(string(output))
		return
	}

	if result.Valid {
		fmt.Println("✓ Configuration is valid")
		fmt.Printf("  - Config: %s\n", result.Config)
		fmt.Printf("  - Sessions: %d\n", result.Sessions)
		fmt.Printf("  - CLI adapters: %d\n", result.Adapters)
		fmt.Printf("  - Bots configured: %d\n", result.Bots)
		if len(result.Warnings) > 0 {
			fmt.Println("\n⚠️  Warnings:")
			for _, warning := range result.Warnings {
				fmt.Printf("  - %s\n", warning)
			}
		}
	} else {
		fmt.Println("❌ Configuration validation failed:")
		if len(result.Errors) > 0 {
			fmt.Println("\nErrors:")
			for _, errMsg := range result.Errors {
				fmt.Printf("  - %s\n", errMsg)
			}
		}
		if len(result.Warnings) > 0 {
			fmt.Println("\nWarnings:")
			for _, warning := range result.Warnings {
				fmt.Printf("  - %s\n", warning)
			}
		}
	}
}

func validateConfigDetails(cfg *core.Config) []string {
	var warnings []string

	// Check if security whitelist is enabled
	if !cfg.Security.WhitelistEnabled {
		warnings = append(warnings, "Whitelist is disabled - this is a security risk")
	}

	// Check if there are allowed users
	if cfg.Security.WhitelistEnabled && len(cfg.Security.AllowedUsers) == 0 {
		warnings = append(warnings, "Whitelist is enabled but no users are allowed")
	}

	// Check if any bots are enabled
	enabledBots := 0
	for _, bot := range cfg.Bots {
		if bot.Enabled {
			enabledBots++
			// Check if bot has token configured (simple check)
			if bot.Token == "" && bot.AppID == "" {
				warnings = append(warnings, fmt.Sprintf("Bot '%s' is enabled but has no credentials configured", getBotName(&bot)))
			}
		}
	}

	if enabledBots == 0 {
		warnings = append(warnings, "No bots are enabled - at least one bot must be enabled")
	}

	// Check if there are any sessions configured
	if len(cfg.Sessions) == 0 {
		warnings = append(warnings, "No sessions configured - add at least one session")
	}

	return warnings
}

// getBotName attempts to identify the bot type from the config
// This is a helper function for warning messages
func getBotName(bot *core.BotConfig) string {
	// Since we don't have the bot name in the context, return a generic message
	return "bot"
}

func init() {
	validateCmd.Flags().StringVarP(&validateConfig, "config", "c", "", "Configuration file path")
	validateCmd.Flags().BoolVar(&validateShow, "show", false, "Show full configuration details")
	validateCmd.Flags().BoolVar(&validateJSON, "json", false, "Output in JSON format")

	// Add validation flag to serve command as well
	serveCmd.Flags().Bool("validate", false, "Validate configuration and exit")
}
