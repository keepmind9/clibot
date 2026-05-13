// Package stdio provides a unified adapter framework for CLI tools
// that communicate via stdin/stdout using structured JSON protocols.
//
// It supports two communication patterns:
//   - Persistent mode: Long-lived process with bidirectional JSON (e.g., Claude Code)
//   - Per-turn mode: New process per message (e.g., Codex exec, Gemini, OpenCode)
//
// Adding a new CLI tool requires only implementing the CLISpec interface.
package stdio

import (
	"context"
	"fmt"
)

// StdioMode determines how the CLI process is managed.
type StdioMode int

const (
	// PersistentMode keeps a single long-lived process.
	// Messages are sent via stdin and responses arrive via stdout.
	PersistentMode StdioMode = iota

	// PerTurnMode spawns a new process for each user message.
	// The process exits after producing a result.
	PerTurnMode
)

// EventType identifies the kind of event emitted by a CLI process.
type EventType int

const (
	EventText       EventType = iota // Text content from the assistant
	EventToolUse                     // Tool invocation notification
	EventPermission                  // Permission request requiring user action
	EventError                       // Error occurred
	EventResult                      // Turn completed
)

// Event represents a single event from a CLI process.
type Event struct {
	Type       EventType
	Text       string             // For EventText and EventResult
	ToolUse    *ToolUseInfo       // For EventToolUse
	Permission *PermissionRequest // For EventPermission
	Error      error              // For EventError
	Done       bool               // True when the turn is fully complete
	SessionID  string             // Populated when a new session is detected
}

// ToolUseInfo describes a tool invocation.
type ToolUseInfo struct {
	Name  string
	Input string
}

// PermissionRequest represents a permission prompt from the CLI.
// It contains multiple numbered options the user can choose from.
type PermissionRequest struct {
	RequestID string
	ToolName  string
	Input     string
	Options   []PermissionOption
}

// FormatOptions formats the permission request as a numbered list for display.
func (p *PermissionRequest) FormatOptions() string {
	s := fmt.Sprintf("Permission requested: %s", p.ToolName)
	if p.Input != "" {
		if len(p.Input) > 500 {
			s += fmt.Sprintf("\nInput: %s...", truncateRunes(p.Input, 500))
		} else {
			s += fmt.Sprintf("\nInput: %s", p.Input)
		}
	}
	s += fmt.Sprintf("\nReply 1-%d:", len(p.Options))
	for i, opt := range p.Options {
		s += fmt.Sprintf("\n%d. %s", i+1, opt.Text)
	}
	return s
}

// OptionByID returns the option with the given ID.
func (p *PermissionRequest) OptionByID(id string) *PermissionOption {
	for i := range p.Options {
		if p.Options[i].ID == id {
			return &p.Options[i]
		}
	}
	return nil
}

// OptionByIndex returns the option at the given 1-based index.
// Returns nil if the index is out of range.
func (p *PermissionRequest) OptionByIndex(idx int) *PermissionOption {
	if idx < 1 || idx > len(p.Options) {
		return nil
	}
	return &p.Options[idx-1]
}

// PermissionOption represents one choice in a permission prompt.
type PermissionOption struct {
	ID   string // Machine-readable identifier
	Text string // Human-readable label
}

// StartOptions contains parameters for starting a CLI process.
type StartOptions struct {
	WorkDir   string
	Model     string
	Env       map[string]string
	Context   context.Context
	Prompt    string // User's input message (for CLIs that take prompt as arg)
	Resume    bool   // Whether to resume a previous session
	SessionID string // Explicit session ID to resume (optional)
}

// CLISpec defines how to interact with a specific CLI tool.
// Implementing this interface is all that's needed to add a new CLI.
type CLISpec interface {
	// Name returns the CLI tool identifier (e.g., "claude", "codex").
	Name() string

	// Mode returns the communication pattern for this CLI.
	Mode() StdioMode

	// Binary returns the command name or path to execute.
	Binary() string

	// BuildArgs returns command-line arguments for starting the process.
	BuildArgs(opts StartOptions) []string

	// ParseLine parses a single JSON line from stdout into events.
	// May return zero or more events per line.
	ParseLine(line string) ([]Event, error)

	// FormatInput formats a user message for writing to stdin.
	FormatInput(message string) ([]byte, error)

	// FormatPermissionResponse formats a permission response for stdin.
	// optionID is the ID from the selected PermissionOption.
	FormatPermissionResponse(requestID string, optionID string) ([]byte, error)
}

// truncateRunes truncates a string to at most n runes, preserving valid UTF-8.
func truncateRunes(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n])
}
