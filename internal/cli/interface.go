package cli

// CLIAdapter defines the interface for CLI adapters
type CLIAdapter interface {
	// SendInput sends input to the CLI (via tmux send-keys)
	SendInput(sessionName, input string) error

	// GetLastResponse retrieves the latest complete response (reads CLI history files)
	GetLastResponse(sessionName string) (string, error)

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

	// CreateSession creates a new session (optional)
	CreateSession(sessionName, cliType, workDir string) error

	// CheckInteractive checks if the CLI is waiting for user input
	// Returns: (isWaiting, promptText, error)
	// Used for handling intermediate interactions, such as confirming command execution, clarifying ambiguities, etc.
	CheckInteractive(sessionName string) (bool, string, error)
}
