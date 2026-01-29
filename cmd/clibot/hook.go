package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/spf13/cobra"
)

var (
	cliType string

	hookCmd = &cobra.Command{
		Use:   "hook --cli-type <type>",
		Short: "Called by CLI hook to notify main process of events",
		Long: `Receives hook data from stdin and forwards it to the main process.

The CLI should pass event data as JSON via stdin. Different CLI types may
have different JSON structures - this command just forwards the data.

Examples:
  echo '{"session":"my-session","event":"completed"}' | clibot hook --cli-type claude
  cat hook-data.json | clibot hook --cli-type gemini`,
		Run: func(cmd *cobra.Command, args []string) {
			// Read JSON data from stdin
			stdinData, err := io.ReadAll(os.Stdin)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error reading stdin: %v\n", err)
				os.Exit(1)
			}

			if len(stdinData) == 0 {
				fmt.Fprintf(os.Stderr, "Error: no data received from stdin\n")
				os.Exit(1)
			}

			// Validate that stdin contains valid JSON
			var jsonData interface{}
			if err := json.Unmarshal(stdinData, &jsonData); err != nil {
				fmt.Fprintf(os.Stderr, "Error: stdin data is not valid JSON: %v\n", err)
				os.Exit(1)
			}

			// Send HTTP POST request to main process
			// Pass cli_type as query parameter, forward stdin data as-is
			url := fmt.Sprintf("http://localhost:8080/hook?cli_type=%s", cliType)
			resp, err := http.Post(url, "application/json", bytes.NewBuffer(stdinData))
			if err != nil {
				fmt.Fprintf(os.Stderr, "Hook request failed: %v\n", err)
				os.Exit(1)
			}
			defer resp.Body.Close()

			if resp.StatusCode == 200 {
				fmt.Println("Hook notification succeeded")
			} else {
				body, _ := io.ReadAll(resp.Body)
				fmt.Fprintf(os.Stderr, "Hook notification failed, status code: %d, response: %s\n", resp.StatusCode, string(body))
				os.Exit(1)
			}
		},
	}
)

func init() {
	hookCmd.Flags().StringVar(&cliType, "cli-type", "", "CLI type (claude/gemini/opencode)")
	hookCmd.MarkFlagRequired("cli-type")
}
