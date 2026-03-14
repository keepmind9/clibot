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
// The CLI adapter pattern separates transport layer (HTTP, file I/O, tmux)
// from protocol logic. Each adapter:
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

// Engine defines the interface for sending responses to users.
// It's implemented by the core Engine and passed to adapters.
type Engine interface {
	SendToBot(platform, channel, message string)
	SendResponseToSession(sessionName, message string)
}

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
	// The transportURL parameter is for ACP adapter (e.g., "stdio://", "tcp://host:port", "unix:///path")
	// Other adapters should ignore this parameter
	CreateSession(sessionName, workDir, startCmd, transportURL string) error

	// ResetSession resets the session (e.g., starts a new conversation)
	ResetSession(sessionName string) error

	// SwitchWorkDir changes the working directory for a session
	// This may require restarting the CLI process in the new directory
	SwitchWorkDir(sessionName, newWorkDir string) error

	// ListSessions returns a list of available CLI-native sessions/conversations
	// botUsername is passed to allow generating platform-specific links
	ListSessions(sessionName string, botUsername string) ([]string, error)

	// SwitchSession switches to a specific CLI-native session/conversation
	// Returns a preview context string of the loaded session on success
	SwitchSession(sessionName, cliSessionID string) (string, error)

	// GetSessionStats returns diagnostic stats for the session (e.g., current session ID and title)
	GetSessionStats(sessionName string, botUsername string) (map[string]interface{}, error)
}
