package main

import (
	"fmt"
	"os"

	"github.com/keepmind9/clibot/internal/cli"
	"github.com/spf13/cobra"
)

var extractInteractionCmd = &cobra.Command{
	Use:   "extract-interaction",
	Short: "Extract the latest user input and assistant response from transcript",
	Long:  `Identify the latest conversation exchange in the specified transcript file for debugging purposes.`,
	Run: func(cmd *cobra.Command, args []string) {
		cliType, _ := cmd.Flags().GetString("type")
		path, _ := cmd.Flags().GetString("path")

		if cliType == "" || path == "" {
			fmt.Println("Error: both --type and --path are required")
			cmd.Help()
			os.Exit(1)
		}

		var prompt, response string
		var err error

		switch cliType {
		case "claude":
			prompt, response, err = cli.ExtractLatestInteraction(path)
		case "gemini":
			// Initialize a temporary adapter for extraction
			adapter, _ := cli.NewGeminiAdapter(cli.GeminiAdapterConfig{})
			prompt, response, err = adapter.ExtractLatestInteraction(path, "")
		case "opencode":
			// Initialize a temporary adapter for extraction
			adapter, _ := cli.NewOpenCodeAdapter(cli.OpenCodeAdapterConfig{})
			prompt, response, err = adapter.ExtractLatestInteraction(path)
		default:
			fmt.Printf("Unsupported CLI type: %s\n", cliType)
			os.Exit(1)
		}

		if err != nil {
			fmt.Printf("Error during extraction: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf(">>> USER INPUT:\n%s\n\n", prompt)
		fmt.Printf(">>> ASSISTANT RESPONSE:\n%s\n", response)
	},
}

func init() {
	rootCmd.AddCommand(extractInteractionCmd)
	extractInteractionCmd.Flags().StringP("type", "t", "", "CLI type (claude, gemini)")
	extractInteractionCmd.Flags().StringP("path", "p", "", "Path to the transcript or session file")
}