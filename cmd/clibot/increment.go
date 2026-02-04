package main

import (
	"fmt"
	"os"

	"github.com/keepmind9/clibot/internal/watchdog"
	"github.com/spf13/cobra"
)

var incrementCmd = &cobra.Command{
	Use:   "increment",
	Short: "Extract incremental content from two snapshots",
	Long: `Extract incremental content by comparing after and before snapshots.

This tool reads two snapshot files and uses the ExtractIncrement method
to identify and return the new content that was added.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		beforeFile, _ := cmd.Flags().GetString("before")
		afterFile, _ := cmd.Flags().GetString("after")

		if beforeFile == "" || afterFile == "" {
			return fmt.Errorf("both --before and --after flags are required")
		}

		// Read before snapshot
		beforeContent, err := os.ReadFile(beforeFile)
		if err != nil {
			return fmt.Errorf("failed to read before file: %w", err)
		}

		// Read after snapshot
		afterContent, err := os.ReadFile(afterFile)
		if err != nil {
			return fmt.Errorf("failed to read after file: %w", err)
		}

		// Extract increment
		result := watchdog.ExtractIncrement(string(afterContent), string(beforeContent))

		// Output result
		fmt.Println(result)

		return nil
	},
}

func init() {
	incrementCmd.Flags().String("before", "", "Path to the before snapshot file")
	incrementCmd.Flags().String("after", "", "Path to the after snapshot file")
	rootCmd.AddCommand(incrementCmd)
}
