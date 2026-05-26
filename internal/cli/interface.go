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

// SessionOptions holds parameters for session creation.
type SessionOptions struct {
	WorkDir      string
	StartCmd     string
	TransportURL string
	Env          map[string]string
	Yolo         bool
	Resume       bool // Resume previous conversation on first turn
}

// SessionOption configures a SessionOptions value.
type SessionOption func(*SessionOptions)

// ApplySessionOptions applies all options to a new SessionOptions value.
func ApplySessionOptions(opts []SessionOption) SessionOptions {
	var o SessionOptions
	for _, opt := range opts {
		opt(&o)
	}
	return o
}

// WithWorkDir sets the working directory for the session.
func WithWorkDir(dir string) SessionOption {
	return func(o *SessionOptions) { o.WorkDir = dir }
}

// WithStartCmd sets the command to start the CLI.
func WithStartCmd(cmd string) SessionOption {
	return func(o *SessionOptions) { o.StartCmd = cmd }
}

// WithTransportURL sets the transport URL (for ACP adapter).
func WithTransportURL(transportURL string) SessionOption {
	return func(o *SessionOptions) { o.TransportURL = transportURL }
}

// WithEnv sets session-level environment variables.
func WithEnv(env map[string]string) SessionOption {
	return func(o *SessionOptions) { o.Env = env }
}

// WithYolo enables auto-approve mode (each CLI appends its own flag).
func WithYolo(yolo bool) SessionOption {
	return func(o *SessionOptions) { o.Yolo = yolo }
}

// WithResume marks the session as having prior history so the first turn uses resume mode.
func WithResume(resume bool) SessionOption {
	return func(o *SessionOptions) { o.Resume = resume }
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

	// CreateSession creates a new session and starts the CLI.
	// Use WithWorkDir, WithStartCmd, WithTransportURL, WithEnv, WithYolo, WithResume options.
	CreateSession(sessionName string, opts ...SessionOption) error

	// StopSession stops and cleans up a running session.
	StopSession(sessionName string) error
}

// StreamingCLI is an optional interface for CLI adapters that support
// receiving intermediate events during processing.
// The engine detects this via type assertion and uses it for rich streaming replies.
type StreamingCLI interface {
	// SendInputStreaming sends input and returns a channel of intermediate events.
	// The channel is closed when the turn completes (normally or on error).
	// Implementations must also satisfy CLIAdapter for session management.
	SendInputStreaming(sessionName, input string) (<-chan CLIEvent, error)
}

// CLIEventType enumerates the kinds of events emitted during streaming.
type CLIEventType string

const (
	CLIEventText       CLIEventType = "text"        // Text output chunk
	CLIEventToolUse    CLIEventType = "tool_use"    // Tool invocation started
	CLIEventToolResult CLIEventType = "tool_result" // Tool invocation completed
	CLIEventThinking   CLIEventType = "thinking"    // Thinking/reasoning output
	CLIEventPermission CLIEventType = "permission"  // Permission request pending user action
	CLIEventDone       CLIEventType = "done"        // Turn completed
	CLIEventUsage      CLIEventType = "usage"       // Token usage information
)

// CLIEvent represents an intermediate event during CLI processing.
type CLIEvent struct {
	Type     CLIEventType
	Content  string            // Text content or tool output
	ToolID   string            // For tool_use/tool_result: tool call ID
	ToolName string            // For tool_use/tool_result: tool name
	ToolMeta map[string]string // For tool_use: command, file_path, etc.
}
