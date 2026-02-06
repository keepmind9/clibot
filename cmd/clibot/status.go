package main

import (
	"encoding/json"
	"fmt"
	"net"

	"github.com/spf13/cobra"
)

var (
	statusPort int
	statusJSON bool
)

// StatusOutput represents the status output structure
type StatusOutput struct {
	Version string `json:"version"`
	Engine  string `json:"engine,omitempty"`
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show clibot status",
	Long: "Display current status of all sessions and connection information. " +
		"If port is specified, check if the port is being listened on.",
	Run: func(cmd *cobra.Command, args []string) {
		status := StatusOutput{
			Version: Version,
		}

		if statusPort > 0 {
			// Check if port is being listened on
			address := fmt.Sprintf(":%d", statusPort)
			conn, err := net.Dial("tcp", address)
			if err != nil {
				status.Engine = fmt.Sprintf("Not running (port %d not listening)", statusPort)
			} else {
				conn.Close()
				status.Engine = fmt.Sprintf("Running (port %d is listening)", statusPort)
			}
		}

		if statusJSON {
			output, err := json.MarshalIndent(status, "", "  ")
			if err != nil {
				fmt.Printf("{\"error\": \"failed to marshal json: %v\"}\n", err)
				return
			}
			fmt.Println(string(output))
		} else {
			fmt.Println("clibot status:")
			fmt.Printf("  - Version: %s\n", status.Version)
			if statusPort > 0 {
				fmt.Printf("  - Engine: %s\n", status.Engine)
			} else {
				fmt.Println("  - Status: Use --port to check if engine is running")
			}
		}
	},
}

func init() {
	statusCmd.Flags().IntVarP(&statusPort, "port", "p", 0, "Port number to check if engine is running")
	statusCmd.Flags().BoolVar(&statusJSON, "json", false, "Output in JSON format")
}
