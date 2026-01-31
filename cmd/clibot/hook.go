package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/keepmind9/clibot/internal/logger"
	"github.com/keepmind9/clibot/pkg/constants"
	"github.com/spf13/cobra"
	"github.com/sirupsen/logrus"
)

// HookNotifier handles HTTP notifications with timeout and cancellation
type HookNotifier struct {
	timeout time.Duration
}

// Notify sends hook data to the engine with timeout control
func (h *HookNotifier) Notify(ctx context.Context, url string, data []byte) error {
	ctx, cancel := context.WithTimeout(ctx, h.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != constants.HTTPSuccessStatusCode {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	return nil
}

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
				// Create a context that outlives the main process
				ctx := context.Background()
				url := fmt.Sprintf("http://localhost:%d/hook?cli_type=%s", hookPort, cliType)

				logger.WithFields(logrus.Fields{
					"cli_type": cliType,
					"url":      url,
					"size":     len(stdinData),
				}).Debug("forwarding-hook-data-to-engine-async")

				notifier := &HookNotifier{timeout: constants.HookHTTPTimeout}
				if err := notifier.Notify(ctx, url, stdinData); err != nil {
					logger.WithFields(logrus.Fields{
						"cli_type": cliType,
						"error":    err,
					}).Error("hook-notification-failed")
					return
				}

				logger.WithFields(logrus.Fields{
					"cli_type": cliType,
				}).Info("hook-notification-succeeded")
			}()

			// Light-weight delay to allow HTTP request to be sent
			// This keeps the hook "alive" briefly but doesn't block Claude Code's UI
			time.Sleep(constants.HookNotificationDelay)

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
