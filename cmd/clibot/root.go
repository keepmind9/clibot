package main

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "clibot",
	Short: "clibot is a middleware connecting IM platforms with AI CLI tools",
	Long: `clibot is a lightweight middleware that connects various IM platforms
(Feishu, Discord, Telegram, etc.) with AI CLI tools (Claude Code, Gemini CLI,
OpenCode, etc.), enabling users to remotely use AI programming assistants
through chat interfaces.`,
}

// Execute executes the root command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	// Add subcommands here
	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(hookCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(versionCmd)
}
