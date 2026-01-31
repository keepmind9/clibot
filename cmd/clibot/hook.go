package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/keepmind9/clibot/internal/logger"
	"github.com/spf13/cobra"
	"github.com/sirupsen/logrus"
)

var (
	cliType  string
	hookPort int

	hookCmd = &cobra.Command{
		Use:   "hook --cli-type <type>",
		Short: "Called by CLI hook to notify main process of events",
		Long: `Receives hook data from stdin and forwards it to the main process.

The CLI should pass event data as JSON via stdin. Different CLI types may
have different JSON structures - this command just forwards the data.

This command uses an asynchronous notification strategy:
- Sends HTTP request in background (non-blocking)
- Returns quickly after a short delay (300ms)
- Allows Claude Code to continue execution without UI freeze

Examples:
  echo '{"session":"my-session","event":"completed"}' | clibot hook --cli-type claude
  cat hook-data.json | clibot hook --cli-type gemini
  cat hook-data.json | clibot hook --cli-type claude --port 9000`,
		Run: func(cmd *cobra.Command, args []string) {
			// Read raw data from stdin (forward as-is, no parsing)
			stdinData, err := io.ReadAll(os.Stdin)
			if err != nil {
				logger.WithField("error", err).Error("failed-to-read-stdin")
				// Exit gracefully to avoid affecting CLI behavior
				return
			}

			if len(stdinData) == 0 {
				logger.Warn("no-data-received-from-stdin")
				// Exit gracefully
				return
			}

			logger.WithFields(logrus.Fields{
				"cli_type": cliType,
				"size":     len(stdinData),
			}).Debug("hook-command-received-data")

			// DEBUG: Print raw data (can be removed later)
			fmt.Fprintf(os.Stderr, "=== Hook Debug ===\n")
			fmt.Fprintf(os.Stderr, "cli_type: %s\n", cliType)
			fmt.Fprintf(os.Stderr, "stdin (%d bytes):\n%s\n", len(stdinData), string(stdinData))
			fmt.Fprintf(os.Stderr, "==================\n")

			// Forward raw data to Engine asynchronously (non-blocking)
			// This allows Claude Code to continue without waiting for engine response
			go func() {
				url := fmt.Sprintf("http://localhost:%d/hook?cli_type=%s", hookPort, cliType)

				logger.WithFields(logrus.Fields{
					"cli_type": cliType,
					"url":      url,
					"size":     len(stdinData),
				}).Debug("forwarding-hook-data-to-engine-async")

				resp, err := http.Post(url, "application/octet-stream", bytes.NewBuffer(stdinData))
				if err != nil {
					logger.WithFields(logrus.Fields{
						"cli_type": cliType,
						"error":    err,
					}).Error("hook-request-failed-async")
					// Don't print to stderr - we've already returned
					return
				}
				defer resp.Body.Close()

				if resp.StatusCode == 200 {
					logger.WithFields(logrus.Fields{
						"cli_type":    cliType,
						"status_code": resp.StatusCode,
					}).Info("hook-notification-succeeded-async")
				} else {
					body, _ := io.ReadAll(resp.Body)
					logger.WithFields(logrus.Fields{
						"cli_type":     cliType,
						"status_code":  resp.StatusCode,
						"response":     string(body),
					}).Warn("hook-notification-failed-non-200-status-async")
				}
			}()

			// Light-weight delay to allow HTTP request to be sent
			// This keeps the hook "alive" briefly but doesn't block Claude Code's UI
			// 300ms is enough for HTTP request to initiate, but short enough to not affect UX
			time.Sleep(300 * time.Millisecond)

			// Return immediately - let Claude Code continue execution
			// The background goroutine will handle the HTTP request independently
			logger.WithFields(logrus.Fields{
				"cli_type": cliType,
			}).Debug("hook-command-returning-async-notification-sent")

			fmt.Println("Hook notification sent asynchronously")
		},
	}
)

func init() {
	hookCmd.Flags().StringVar(&cliType, "cli-type", "", "CLI type (claude/gemini/opencode)")
	hookCmd.MarkFlagRequired("cli-type")
	hookCmd.Flags().IntVarP(&hookPort, "port", "p", 8080, "Hook server port")
}
