package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show clibot status",
	Long:  "Display current status of all sessions and connection information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("clibot status:")
		// TODO: Implement status query logic
		fmt.Println("  - Version: v0.3")
		fmt.Println("  - Status: Initialized")
	},
}
