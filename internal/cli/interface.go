// Package cli provides adapters for AI-powered CLI tools.
//
// This package implements a unified interface for interacting with various AI CLI
// tools such as Claude Code, Gemini CLI, and others. Each adapter handles:
//
//   - Session management via tmux
//   - Input delivery to the CLI
//   - Response extraction from history files or hooks
//   - Interactive state detection
//
// # Supported CLIs
//
//   - Claude Code (claude): Anthropic's AI programming assistant
//   - Gemini (gemini): Google's AI assistant
//   - OpenCode (opencode): AI programming assistant
//
// # Architecture
//
// The CLI adapter pattern separates the transport layer (HTTP, file I/O, tmux)
// from the protocol logic. Each adapter:
//
//  1. Creates/manages tmux sessions for the CLI
//  2. Sends user input via tmux send-keys
//  3. Receives responses via two mechanisms:
//     - Hook mode: Real-time notifications when CLI completes a task (use_hook: true)
//     - Polling mode: Periodic tmux capture when output becomes stable (use_hook: false)
//  4. Detects interactive states (prompts, confirmations)
//
// # Thread Safety
//
// CLI adapters are not thread-safe and should not be accessed concurrently.
// The engine ensures serialized access to each adapter.
package cli

import "time"

// CLIAdapter defines the interface for CLI adapters
type CLIAdapter interface {
	// SendInput sends input to the CLI (via tmux send-keys)
	SendInput(sessionName, input string) error

	// HandleHookData handles raw hook data from the CLI
	// The adapter is responsible for:
	//   - Parsing the data (in any format: JSON, text, etc.)
	//   - Extracting the last user prompt for tmux filtering
	//   - Extracting the session name from the data
	//   - Processing the hook data and generating the response
	//
	// This interface is protocol-agnostic - it works with HTTP, gRPC, message queues, etc.
	// The engine is responsible for extracting the raw data from the transport layer.
	//
	// Parameter data: raw hook data (bytes)
	// Returns: (sessionName, lastUserPrompt, responseText, error)
	//   - sessionName: which session this hook is for (cwd)
	//   - lastUserPrompt: the last user's input (for filtering tmux output)
	//   - responseText: the CLI's response to send back to the user
	//   - error: any error that occurred
	HandleHookData(data []byte) (sessionName string, lastUserPrompt string, response string, err error)

	// IsSessionAlive checks if the session is still alive
	IsSessionAlive(sessionName string) bool

	// CreateSession creates a new session and starts the CLI with the specified command
	// The startCmd parameter allows sessions to use different commands than the adapter default
	CreateSession(sessionName, workDir, startCmd string) error

	// UseHook returns whether this adapter uses hook mode (true) or polling mode (false)
	// Hook mode: Real-time notifications via CLI hooks (requires CLI configuration)
	// Polling mode: Periodic tmux capture when output becomes stable (no configuration needed)
	UseHook() bool

	// GetPollInterval returns the polling interval for polling mode
	// Only used when UseHook() returns false
	GetPollInterval() time.Duration

	// GetStableCount returns the number of consecutive stable checks required
	// Only used when UseHook() returns false
	// Default: 3 (output must be stable for 3 consecutive checks)
	GetStableCount() int

	// GetPollTimeout returns the maximum time to wait for completion
	// Only used when UseHook() returns false
	// Default: 120 seconds
	GetPollTimeout() time.Duration
}

// normalizePollingConfig applies default values for polling configuration.
// This helper function reduces code duplication across CLI adapters.
//
// Parameters:
//   - interval: polling interval (0 → 1 second)
//   - stableCount: consecutive stable checks required (0 → 3)
//   - timeout: maximum wait time (0 → 1 hour)
//
// Note: timeout acts as a safety fallback; actual completion is determined
// by the stable_count mechanism (output stability detection).
//
// Returns:
//   - Normalized interval, stableCount, timeout
func normalizePollingConfig(interval time.Duration, stableCount int, timeout time.Duration) (time.Duration, int, time.Duration) {
	if interval == 0 {
		interval = 1 * time.Second
	}
	if stableCount == 0 {
		stableCount = 3
	}
	if timeout == 0 {
		timeout = 1 * time.Hour // Safety fallback - actual completion determined by stable_count
	}
	return interval, stableCount, timeout
}
