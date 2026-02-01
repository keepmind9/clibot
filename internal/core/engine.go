package core

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/keepmind9/clibot/internal/bot"
	"github.com/keepmind9/clibot/internal/cli"
	"github.com/keepmind9/clibot/internal/logger"
	"github.com/keepmind9/clibot/pkg/constants"
	"github.com/keepmind9/clibot/internal/watchdog"
	"github.com/sirupsen/logrus"
)

const (
	capturePaneLine    = constants.DefaultCaptureLines
	tmuxCapturePaneLine = constants.DefaultManualCaptureLines

	// maxSpecialCommandInputLength is the maximum allowed input length for special commands.
	// This prevents DoS attacks from extremely long inputs.
	maxSpecialCommandInputLength = 10000 // 10KB

	// maxViewLines is the maximum allowed line count for the view command.
	// This prevents integer overflow and excessive resource usage.
	maxViewLines = 10000
)

// specialCommands defines commands that can be used without a prefix.
// These are matched exactly (case-sensitive) for optimal performance.
//
// Performance: O(1) map lookup for exact match commands.
// Only "view" command supports arguments (special case).
var specialCommands = map[string]struct{}{
	"help":     {},
	"status":   {},
	"sessions": {},
	"whoami":   {},
	"view":     {},
}

// isSpecialCommand checks if input is a special command.
//
// Matching strategy (exact match for maximum performance):
//   - Exact match: "help", "status", "sessions", "whoami", "view"
//   - View with args: "view 100", "view 50" (only "view" supports args)
//
// Returns: (commandName, isCommand, remainingArgs)
//
// Performance characteristics:
//   - Common case (exact match): 1 map lookup, O(1)
//   - View with args: HasPrefix + Fields, O(n) where n = input length
//   - Not a command: 1 map lookup, O(1)
func isSpecialCommand(input string) (string, bool, []string) {
	// Security: Reject extremely long inputs early (DoS protection)
	if len(input) > maxSpecialCommandInputLength {
		return "", false, nil
	}

	// Fast path: exact match for commands without arguments.
	// This covers 95% of cases with a single O(1) map lookup.
	if _, exists := specialCommands[input]; exists {
		return input, true, nil
	}

	// Special case: view command with numeric arguments (e.g., "view 100", "view 50")
	// This is the only command that supports arguments.
	// Arguments must be numeric to avoid false positives (e.g., "view help" → normal input)
	if len(input) >= 5 && input[:4] == "view" {
		// Check if 5th character is whitespace (space or tab)
		if input[4] == ' ' || input[4] == '\t' {
			// Split into fields and use only the first argument
			fields := strings.Fields(input[5:])
			if len(fields) > 0 {
				arg := fields[0]
				// Validate argument is numeric and within safe range
				num, err := strconv.Atoi(arg)
				if err == nil {
					// Security: Validate range to prevent integer overflow
					if num >= -maxViewLines && num <= maxViewLines {
						return "view", true, []string{arg}
					}
				}
				// Not a valid number or out of range → treat as normal input (e.g., "view help", "view abc")
			}
		}
	}

	return "", false, nil
}

// Engine is the core scheduling engine that manages CLI sessions and bot connections
type Engine struct {
	config          *Config
	cliAdapters     map[string]cli.CLIAdapter  // CLI type -> adapter
	activeBots      map[string]bot.BotAdapter  // Bot type -> adapter
	sessions        map[string]*Session        // Session name -> Session
	sessionMu       sync.RWMutex               // Mutex for session access
	messageChan     chan bot.BotMessage        // Bot message channel
	hookServer      *http.Server               // HTTP server for hooks
	sessionChannels map[string]BotChannel      // Session name -> active bot channel (for routing responses)
	ctx             context.Context            // Context for cancellation
	cancel          context.CancelFunc         // Cancel function for graceful shutdown
}

// BotChannel represents a bot channel for sending responses
type BotChannel struct {
	Platform string  // "discord", "telegram", "feishu", etc.
	Channel  string  // Channel ID (platform-specific)
}

// NewEngine creates a new Engine instance
func NewEngine(config *Config) *Engine {
	ctx, cancel := context.WithCancel(context.Background())
	return &Engine{
		config:          config,
		cliAdapters:     make(map[string]cli.CLIAdapter),
		activeBots:      make(map[string]bot.BotAdapter),
		sessions:        make(map[string]*Session),
		messageChan:     make(chan bot.BotMessage, constants.MessageChannelBufferSize),
		sessionChannels: make(map[string]BotChannel),
		ctx:             ctx,
		cancel:          cancel,
	}
}

// RegisterCLIAdapter registers a CLI adapter
func (e *Engine) RegisterCLIAdapter(cliType string, adapter cli.CLIAdapter) {
	e.cliAdapters[cliType] = adapter
}

// RegisterBotAdapter registers a bot adapter
func (e *Engine) RegisterBotAdapter(botType string, adapter bot.BotAdapter) {
	e.activeBots[botType] = adapter
}

// initializeSessions initializes all configured sessions
func (e *Engine) initializeSessions() error {
	e.sessionMu.Lock()
	defer e.sessionMu.Unlock()

	for _, sessionConfig := range e.config.Sessions {
		// Check if session already exists
		if _, exists := e.sessions[sessionConfig.Name]; exists {
			continue
		}

		// Create new session
		session := &Session{
			Name:      sessionConfig.Name,
			CLIType:   sessionConfig.CLIType,
			WorkDir:   sessionConfig.WorkDir,
			State:     StateIdle,
			CreatedAt: time.Now().Format(time.RFC3339),
		}

		// Check if CLI adapter exists
		adapter, exists := e.cliAdapters[session.CLIType]
		if !exists {
			log.Printf("Warning: CLI adapter %s not found for session %s", session.CLIType, session.Name)
			continue
		}

		// Check if session is alive or create if auto_start is enabled
		if adapter.IsSessionAlive(session.Name) {
			log.Printf("Session %s is already running", session.Name)
		} else if sessionConfig.AutoStart {
			log.Printf("Auto-starting session %s", session.Name)
			if err := adapter.CreateSession(session.Name, session.CLIType, session.WorkDir); err != nil {
				log.Printf("Failed to create session %s: %v", session.Name, err)
				continue
			}
		} else {
			log.Printf("Session %s is not running and auto_start is disabled", session.Name)
		}

		e.sessions[session.Name] = session
	}

	return nil
}

// Run starts the engine and begins processing messages
func (e *Engine) Run(ctx context.Context) error {
	logger.Info("starting-clibot-engine")

	// Initialize sessions
	if err := e.initializeSessions(); err != nil {
		return fmt.Errorf("failed to initialize sessions: %w", err)
	}

	// Start HTTP hook server
	go e.startHookServer()

	// Start all enabled bots
	for botType, botConfig := range e.config.Bots {
		if !botConfig.Enabled {
			continue
		}

		botAdapter, exists := e.activeBots[botType]
		if !exists {
			log.Printf("Warning: Bot adapter %s not found", botType)
			continue
		}

		log.Printf("Starting %s bot...", botType)
		go func(bt string, ba bot.BotAdapter) {
			defer func() {
				if r := recover(); r != nil {
					logger.WithFields(logrus.Fields{
						"bot_type": bt,
						"panic":    r,
					}).Error("bot-start-panic-recovered")
				}
			}()
			if err := ba.Start(e.HandleBotMessage); err != nil {
				logger.WithFields(logrus.Fields{
					"bot_type": bt,
					"error":    err,
				}).Error("failed-to-start-bot")
			}
		}(botType, botAdapter)
	}

	// Start main event loop
	e.runEventLoop(ctx)

	return nil
}

// runEventLoop runs the main event loop for processing messages
func (e *Engine) runEventLoop(ctx context.Context) {
	logger.Info("engine-event-loop-started")

	for {
		select {
		case <-ctx.Done():
			logger.Info("event-loop-shutting-down")
			return
		case msg := <-e.messageChan:
			e.HandleUserMessage(msg)
		}
	}
}

// HandleBotMessage is the callback function for bots to deliver messages
func (e *Engine) HandleBotMessage(msg bot.BotMessage) {
	e.messageChan <- msg
}

// HandleUserMessage processes a message from a user
func (e *Engine) HandleUserMessage(msg bot.BotMessage) {
	logger.WithFields(logrus.Fields{
		"platform": msg.Platform,
		"user":     msg.UserID,
		"channel":  msg.Channel,
	}).Info("processing-user-message")

	// Step 0: Security check - verify user is in whitelist
	if !e.config.IsUserAuthorized(msg.Platform, msg.UserID) {
		logger.WithFields(logrus.Fields{
			"platform": msg.Platform,
			"user":     msg.UserID,
		}).Warn("unauthorized-access-attempt")
		e.SendToBot(msg.Platform, msg.Channel, "❌ Unauthorized: Please contact the administrator to add your user ID")
		return
	}

	logger.WithField("user", msg.UserID).Debug("user-authorized")

	// Step 1: Check if it's a special command (no prefix required)
	// Commands are matched exactly for optimal performance
	input := strings.TrimSpace(msg.Content)
	if cmd, isCmd, args := isSpecialCommand(input); isCmd {
		logger.WithFields(logrus.Fields{
			"command": cmd,
			"args":    args,
			"user":    msg.UserID,
		}).Info("special-command-received")
		e.HandleSpecialCommandWithArgs(cmd, args, msg)
		return
	}

	// Step 2: Get the active session for this channel
	session := e.GetActiveSession(msg.Channel)
	if session == nil {
		logger.WithFields(logrus.Fields{
			"channel": msg.Channel,
		}).Warn("no-active-session-found-for-channel")
		e.SendToBot(msg.Platform, msg.Channel, "❌ No active session. Use 'sessions' to list available sessions")
		return
	}

	logger.WithFields(logrus.Fields{
		"session": session.Name,
		"state":   session.State,
		"cli":     session.CLIType,
	}).Debug("session-found")

	// Record the session → channel mapping for routing responses
	e.sessionMu.Lock()
	e.sessionChannels[session.Name] = BotChannel{
		Platform: msg.Platform,
		Channel:  msg.Channel,
	}
	e.sessionMu.Unlock()

	// Step 3: If session is waiting for input (interactive state), pass directly
	// Use read lock to prevent race condition with session state updates
	e.sessionMu.RLock()
	isWaitingInput := session.State == StateWaitingInput
	e.sessionMu.RUnlock()

	if isWaitingInput {
		// Process key words before sending
		processedContent := watchdog.ProcessKeyWords(msg.Content)
		if processedContent != msg.Content {
			logger.WithFields(logrus.Fields{
				"original": msg.Content,
				"processed": fmt.Sprintf("%q", processedContent),
			}).Debug("keyword-converted-to-key-sequence")
		}

		adapter := e.cliAdapters[session.CLIType]
		if err := adapter.SendInput(session.Name, processedContent); err != nil {
			// Don't update state on error - keep it in waiting input state
			e.SendToBot(msg.Platform, msg.Channel, fmt.Sprintf("❌ Failed to send input: %v", err))
			return
		}

		// Update session state
		e.updateSessionState(session.Name, StateProcessing)

		// Start new watchdog for this session
		ctx, cleanup := e.startNewWatchdogForSession(session.Name)

		// Resume watchdog monitoring
		go func(sessionName string, watchdogCtx context.Context) {
			defer func() {
				if r := recover(); r != nil {
					logger.WithFields(logrus.Fields{
						"session": sessionName,
						"panic":   r,
					}).Error("watchdog-panic-recovered")
				}
				// Clear the cancel function when done
				cleanup()
			}()

			// Re-fetch session under lock to avoid race condition
			e.sessionMu.RLock()
			session, exists := e.sessions[sessionName]
			e.sessionMu.RUnlock()

			if !exists {
				logger.WithField("session", sessionName).Warn("session-no-longer-exists")
				return
			}

			if err := e.startWatchdogWithContext(watchdogCtx, session); err != nil {
				logger.WithFields(logrus.Fields{
					"session": sessionName,
					"error":   err,
				}).Error("watchdog-failed")
			}
		}(session.Name, ctx)

		return
	}

	// Step 4: Process key words (tab, esc, stab, enter)
	// Converts entire input matching keywords to actual key sequences
	processedContent := watchdog.ProcessKeyWords(msg.Content)
	if processedContent != msg.Content {
		logger.WithFields(logrus.Fields{
			"original": msg.Content,
			"processed": fmt.Sprintf("%q", processedContent),
		}).Debug("keyword-converted-to-key-sequence")
	}

	// Step 5: Normal flow - send to CLI
	adapter := e.cliAdapters[session.CLIType]
	if err := adapter.SendInput(session.Name, processedContent); err != nil {
		logger.WithFields(logrus.Fields{
			"session": session.Name,
			"error":   err,
		}).Error("failed-to-send-input-to-cli")
		// Restore session state to idle on error
		e.updateSessionState(session.Name, StateIdle)
		e.SendToBot(msg.Platform, msg.Channel, fmt.Sprintf("❌ Failed to send input: %v", err))
		return
	}

	logger.WithFields(logrus.Fields{
		"session": session.Name,
		"cli":     session.CLIType,
	}).Info("input-sent-to-cli")

	// Update session state
	e.updateSessionState(session.Name, StateProcessing)

	// Start new watchdog for this session
	ctx, cleanup := e.startNewWatchdogForSession(session.Name)

	// Start watchdog monitoring (for detecting interactive prompts)
	go func(sessionName string, watchdogCtx context.Context) {
		defer func() {
			if r := recover(); r != nil {
				logger.WithFields(logrus.Fields{
					"session": sessionName,
					"panic":   r,
				}).Error("watchdog-panic-recovered")
			}
			// Clear the cancel function when done
			cleanup()
		}()

		// Re-fetch session under lock to avoid race condition
		e.sessionMu.RLock()
		session, exists := e.sessions[sessionName]
		e.sessionMu.RUnlock()

		if !exists {
			logger.WithField("session", sessionName).Warn("session-no-longer-exists")
			return
		}

		if err := e.startWatchdogWithContext(watchdogCtx, session); err != nil {
			logger.WithFields(logrus.Fields{
				"session": sessionName,
				"error":   err,
			}).Error("watchdog-failed")
		}
	}(session.Name, ctx)
}

// HandleSpecialCommand handles special clibot commands
func (e *Engine) HandleSpecialCommand(cmd string, msg bot.BotMessage) {
	// Parse command and arguments for backward compatibility
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		e.SendToBot(msg.Platform, msg.Channel, "❌ Empty command")
		return
	}
	e.HandleSpecialCommandWithArgs(parts[0], parts[1:], msg)
}

// HandleSpecialCommandWithArgs handles special commands with pre-parsed arguments
// This is more efficient as it avoids re-parsing the command string
func (e *Engine) HandleSpecialCommandWithArgs(command string, args []string, msg bot.BotMessage) {
	logger.WithField("command", command).Info("handling-special-command")

	switch command {
	case "help":
		e.showHelp(msg)
	case "sessions":
		e.listSessions(msg)
	case "status":
		e.showStatus(msg)
	case "whoami":
		e.showWhoami(msg)
	case "view":
		// Reconstruct parts for captureView (expects full parts array)
		parts := append([]string{command}, args...)
		e.captureView(msg, parts)
	default:
		e.SendToBot(msg.Platform, msg.Channel,
			fmt.Sprintf("❌ Unknown command: %s\nUse 'help' to see available commands", command))
	}
}

// listSessions lists all available sessions
func (e *Engine) listSessions(msg bot.BotMessage) {
	e.sessionMu.RLock()
	defer e.sessionMu.RUnlock()

	response := "📋 Available Sessions:\n"
	for _, session := range e.sessions {
		response += fmt.Sprintf("  • %s (%s) - %s\n", session.Name, session.CLIType, session.State)
	}

	e.SendToBot(msg.Platform, msg.Channel, response)
}

// showStatus shows the status of all sessions
func (e *Engine) showStatus(msg bot.BotMessage) {
	e.sessionMu.RLock()
	defer e.sessionMu.RUnlock()

	response := "📊 clibot Status:\n\n"
	response += "Sessions:\n"
	for _, session := range e.sessions {
		alive := false
		if adapter, exists := e.cliAdapters[session.CLIType]; exists {
			alive = adapter.IsSessionAlive(session.Name)
		}
		status := "❌"
		if alive {
			status = "✅"
		}
		response += fmt.Sprintf("  %s %s (%s) - %s\n", status, session.Name, session.CLIType, session.State)
	}

	e.SendToBot(msg.Platform, msg.Channel, response)
}

// showWhoami shows current session information
func (e *Engine) showWhoami(msg bot.BotMessage) {
	session := e.GetActiveSession(msg.Channel)
	if session == nil {
		e.SendToBot(msg.Platform, msg.Channel, "No active session")
		return
	}

	response := fmt.Sprintf("Current Session:\n  Name: %s\n  CLI: %s\n  State: %s\n  WorkDir: %s",
		session.Name, session.CLIType, session.State, session.WorkDir)
	e.SendToBot(msg.Platform, msg.Channel, response)
}

// showHelp displays help information about available commands and keywords
func (e *Engine) showHelp(msg bot.BotMessage) {
	help := `📖 **clibot Help**

**Special Commands** (no prefix required):
  help         - Show this help message
  sessions     - List all available sessions
  status       - Show status of all sessions
  whoami       - Show current session info
  view [n]     - View CLI output (default: 20 lines)

**Special Keywords** (exact match, case-insensitive):
  tab          - Send Tab key
  esc          - Send Escape key
  stab/s-tab   - Send Shift+Tab
  enter        - Send Enter key
  ctrlc/ctrl-c - Send Ctrl+C (interrupt)

**Usage Examples:**
  help              → Show help
  status            → Show status
  tab               → Send Tab key to CLI
  ctrl-c            → Interrupt current process
  view 100          → View last 100 lines of output

**Tips:**
  - Special commands are exact match (case-sensitive)
  - Special keywords are case-insensitive
  - Any other input will be sent to the CLI
  - Use "help" anytime to see this message`

	e.SendToBot(msg.Platform, msg.Channel, help)
}

// captureView captures and displays CLI tool output
// Usage: view [lines]
// If lines is not provided, defaults to 20 (DefaultManualCaptureLines)
func (e *Engine) captureView(msg bot.BotMessage, parts []string) {
	// Parse line count parameter (default: 20)
	lines := tmuxCapturePaneLine
	if len(parts) >= 2 {
		if _, err := fmt.Sscanf(parts[1], "%d", &lines); err != nil {
			e.SendToBot(msg.Platform, msg.Channel, fmt.Sprintf("❌ Invalid line count: %s\nUsage: view [lines]", parts[1]))
			return
		}
		// Limit to reasonable range
		if lines < 1 {
			lines = 1
		}
		if lines > constants.MaxTmuxCaptureLines {
			lines = constants.MaxTmuxCaptureLines
		}
	}

	// Get current active session
	session := e.GetActiveSession(msg.Channel)
	if session == nil {
		e.SendToBot(msg.Platform, msg.Channel, "❌ No active session")
		return
	}

	// Check if session is alive
	adapter, exists := e.cliAdapters[session.CLIType]
	if !exists {
		e.SendToBot(msg.Platform, msg.Channel, fmt.Sprintf("❌ CLI adapter not found: %s", session.CLIType))
		return
	}

	if !adapter.IsSessionAlive(session.Name) {
		e.SendToBot(msg.Platform, msg.Channel, fmt.Sprintf("❌ Session is not running: %s", session.Name))
		return
	}

	// Capture CLI output from tmux pane
	output, err := watchdog.CapturePane(session.Name, lines)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"session": session.Name,
			"lines":   lines,
			"error":   err,
		}).Error("failed-to-capture-cli-output")
		e.SendToBot(msg.Platform, msg.Channel, fmt.Sprintf("❌ Failed to capture CLI output: %v", err))
		return
	}

	// Strip ANSI codes for cleaner output
	cleanOutput := watchdog.StripANSI(output)
	// Send response with header
	response := fmt.Sprintf("📺 CLI Output (%s, last %d lines):\n```\n%s\n```", session.Name, lines, cleanOutput)
	e.SendToBot(msg.Platform, msg.Channel, response)

	logger.WithFields(logrus.Fields{
		"session":        session.Name,
		"lines_requested": lines,
		"output_length":  len(cleanOutput),
	}).Info("tmux-capture-command-executed")
}

// GetActiveSession gets the active session for a channel
// Currently returns the default session. Per-channel session mapping is not yet implemented.
//
// Future enhancement: Map each bot channel to a specific session for multi-tenancy support.
// See: https://github.com/keepmind9/clibot/issues/124
func (e *Engine) GetActiveSession(channel string) *Session {
	e.sessionMu.RLock()
	defer e.sessionMu.RUnlock()

	// Return the default session
	if session, exists := e.sessions[e.config.DefaultSession]; exists {
		return session
	}

	// Return first available session
	for _, session := range e.sessions {
		return session
	}

	return nil
}

// updateSessionState updates the state of a session
func (e *Engine) updateSessionState(sessionName string, newState SessionState) {
	e.sessionMu.Lock()
	defer e.sessionMu.Unlock()

	if session, exists := e.sessions[sessionName]; exists {
		oldState := session.State
		session.State = newState
		logger.WithFields(logrus.Fields{
			"session":   sessionName,
			"old_state": oldState,
			"new_state": newState,
		}).Debug("session-state-updated")
	}
}

// startNewWatchdogForSession cancels any existing watchdog and creates a new context.
// This prevents goroutine leaks when multiple messages are sent rapidly.
//
// Returns the new context and a cleanup function to be called when done.
// The cleanup function must be called to clear the session's cancelCtx.
func (e *Engine) startNewWatchdogForSession(sessionName string) (context.Context, func()) {
	e.sessionMu.Lock()
	defer e.sessionMu.Unlock()

	session, exists := e.sessions[sessionName]
	if !exists {
		// Session doesn't exist, return a cancelled context
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		return ctx, func() {}
	}

	// Cancel any existing watchdog
	if session.cancelCtx != nil {
		logger.WithField("session", session.Name).Debug("cancelling-previous-watchdog")
		session.cancelCtx()
		session.cancelCtx = nil
	}

	// Check if engine context is already cancelled
	select {
	case <-e.ctx.Done():
		// Engine is shutting down, return cancelled context
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		return ctx, func() {}
	default:
	}

	// Create new context for this watchdog
	ctx, cancel := context.WithCancel(e.ctx)
	session.cancelCtx = cancel

	// Return cleanup function
	cleanup := func() {
		e.sessionMu.Lock()
		defer e.sessionMu.Unlock()
		if session, exists := e.sessions[sessionName]; exists {
			session.cancelCtx = nil
		}
	}

	return ctx, cleanup
}

// SendToBot sends a message to a specific bot
func (e *Engine) SendToBot(platform, channel, message string) {
	if botAdapter, exists := e.activeBots[platform]; exists {
		if err := botAdapter.SendMessage(channel, message); err != nil {
			logger.WithFields(logrus.Fields{
				"platform": platform,
				"channel":  channel,
				"error":    err,
			}).Error("failed-to-send-message-to-bot")
		} else {
			logger.WithFields(logrus.Fields{
				"platform": platform,
				"channel":  channel,
				"length":   len(message),
			}).Info("message-sent-to-bot")
		}
	}
}

// SendToAllBots sends a message to all active bots
func (e *Engine) SendToAllBots(message string) {
	for platform, botAdapter := range e.activeBots {
		if err := botAdapter.SendMessage("", message); err != nil {
			log.Printf("Failed to send message to %s: %v", platform, err)
		}
	}
}

// startWatchdog starts monitoring for CLI interactive prompts
//
// Note: This is a placeholder for future watchdog monitoring functionality.
// The current implementation uses hook-based retry mechanism in handleHookRequest.
// Full watchdog implementation is tracked at: https://github.com/keepmind9/clibot/issues/123
// startWatchdog starts monitoring the CLI session for completion
// It uses either hook mode (real-time notifications) or polling mode (periodic checks)
func (e *Engine) startWatchdog(session *Session) error {
	adapter := e.cliAdapters[session.CLIType]

	// Check which mode to use
	if adapter.UseHook() {
		logger.WithField("session", session.Name).Debug("using-hook-mode")
		return e.runWatchdogWithHook(session)
	} else {
		logger.WithField("session", session.Name).Debug("using-polling-mode")
		return e.runWatchdogPolling(session)
	}
}

// startWatchdogWithContext starts monitoring with a cancellable context
// This prevents goroutine leaks when multiple messages are sent rapidly
func (e *Engine) startWatchdogWithContext(ctx context.Context, session *Session) error {
	adapter := e.cliAdapters[session.CLIType]

	// Check which mode to use
	if adapter.UseHook() {
		logger.WithField("session", session.Name).Debug("using-hook-mode")
		return e.runWatchdogWithHook(session)
	} else {
		logger.WithField("session", session.Name).Debug("using-polling-mode")
		return e.runWatchdogPollingWithContext(ctx, session)
	}
}

// runWatchdogWithHook implements hook-based monitoring (real-time, requires CLI configuration)
func (e *Engine) runWatchdogWithHook(session *Session) error {
	logger.WithField("session", session.Name).Debug("hook-mode-watchdog-started")

	// Hook mode is event-driven
	// The engine waits for hook notifications via HTTP
	// This is a placeholder - actual hook handling is done in handleHookRequest
	logger.WithField("session", session.Name).Debug("hook-mode-watchdog-waiting")

	// In hook mode, we just wait for the hook to trigger
	// The actual processing happens when the hook is received
	return nil
}

// runWatchdogPolling implements polling-based monitoring (no CLI configuration required)
func (e *Engine) runWatchdogPolling(session *Session) error {
	return e.runWatchdogPollingWithContext(e.ctx, session)
}

// runWatchdogPollingWithContext implements polling-based monitoring with cancellable context
// This prevents goroutine leaks when multiple messages are sent rapidly
func (e *Engine) runWatchdogPollingWithContext(ctx context.Context, session *Session) error {
	adapter := e.cliAdapters[session.CLIType]

	// Get polling configuration
	pollingConfig := watchdog.PollingConfig{
		Interval:    adapter.GetPollInterval(),
		StableCount: adapter.GetStableCount(),
		Timeout:     adapter.GetPollTimeout(),
	}

	logger.WithFields(logrus.Fields{
		"session":     session.Name,
		"interval":    pollingConfig.Interval,
		"stableCount": pollingConfig.StableCount,
		"timeout":     pollingConfig.Timeout,
	}).Info("polling-mode-watchdog-started")

	// Wait for CLI to complete (polling mode)
	content, err := watchdog.WaitForCompletion(session.Name, pollingConfig, ctx)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"session": session.Name,
			"error":   err,
		}).Error("polling-failed")

		// Update session state
		e.updateSessionState(session.Name, StateIdle)

		return err
	}

	// Send response to user
	logger.WithFields(logrus.Fields{
		"session":        session.Name,
		"response_length": len(content),
	}).Info("response-completed-sending-to-user")

	e.sendResponseToUser(session.Name, content)

	// Update session state
	e.updateSessionState(session.Name, StateIdle)

	return nil
}

// sendResponseToUser sends the CLI response to the user via bot
func (e *Engine) sendResponseToUser(sessionName string, content string) {
	// Get the channel for this session
	e.sessionMu.RLock()
	botChannel, exists := e.sessionChannels[sessionName]
	e.sessionMu.RUnlock()

	if !exists {
		logger.WithField("session", sessionName).Warn("no-bot-channel-found-for-session")
		return
	}

	// Send response
	logger.WithFields(logrus.Fields{
		"session":        sessionName,
		"platform":       botChannel.Platform,
		"channel":        botChannel.Channel,
		"response_length": len(content),
	}).Info("sending-response-to-user")

	e.SendToBot(botChannel.Platform, botChannel.Channel, content)
}

// startHookServer starts the HTTP hook server
func (e *Engine) startHookServer() {
	addr := fmt.Sprintf(":%d", e.config.HookServer.Port)

	// Create HTTP server instance
	mux := http.NewServeMux()
	mux.HandleFunc("/hook", e.handleHookRequest)

	e.hookServer = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	logger.WithField("address", addr).Info("hook-server-listening")

	// Start server (blocking)
	// When Shutdown() is called, ListenAndServe will return ErrServerClosed
	if err := e.hookServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Errorf("hook-server-error: %v", err)
	}

	logger.Info("hook-server-stopped")
}

// handleHookRequest handles HTTP hook requests
// This function extracts raw data from HTTP request and delegates to CLI adapter
// The CLI adapter is protocol-agnostic and only works with raw bytes
func (e *Engine) handleHookRequest(w http.ResponseWriter, r *http.Request) {
	// Only accept POST requests
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get cli_type from query parameter (used for routing to correct adapter)
	cliType := r.URL.Query().Get("cli_type")
	if cliType == "" {
		logger.Warn("missing-cli-type-query-parameter-in-hook-request")
		http.Error(w, "Missing cli_type parameter", http.StatusBadRequest)
		return
	}

	// Get CLI adapter
	adapter, exists := e.cliAdapters[cliType]
	if !exists {
		logger.WithField("cli_type", cliType).Warn("no-adapter-found-for-cli-type")
		http.Error(w, "CLI adapter not found", http.StatusInternalServerError)
		return
	}

	// Read raw data from request body
	// The adapter will parse this data in whatever format it expects (JSON, text, etc.)
	data, err := io.ReadAll(r.Body)
	if err != nil {
		logger.Errorf("failed-to-read-request-body: %v", err)
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if len(data) == 0 {
		logger.Warn("empty-request-body-in-hook-request")
		http.Error(w, "Empty request body", http.StatusBadRequest)
		return
	}

	logger.WithFields(logrus.Fields{
		"cli_type": cliType,
		"hook_data": string(data),
	}).Debug("hook-data-received")

	// Delegate to CLI adapter (protocol-agnostic)
	// The adapter parses the data and returns: (cwd, lastUserPrompt, response, error)
	// identifier is used to match the session (e.g., cwd, session name, etc.)
	identifier, lastUserPrompt, response, err := adapter.HandleHookData(data)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"cli_type": cliType,
			"error":    err,
		}).Error("failed-to-handle-hook-data")
		http.Error(w, "Failed to process hook", http.StatusInternalServerError)
		return
	}

	// Match identifier (cwd) to actual tmux session
	// The adapter returns an identifier (e.g., cwd), and we find the matching session
	e.sessionMu.RLock()
	var session *Session
	for _, s := range e.sessions {
		// Normalize paths for comparison
		normalizedWorkDir := normalizePath(s.WorkDir)
		normalizedIdentifier := normalizePath(identifier)

		if normalizedWorkDir == normalizedIdentifier {
			session = s
			break
		}
	}
	e.sessionMu.RUnlock()

	if session == nil {
		logger.WithFields(logrus.Fields{
			"identifier": identifier,
		}).Warn("no-session-found-matching-identifier")
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	logger.WithFields(logrus.Fields{
		"session":  session.Name,
		"work_dir": session.WorkDir,
	}).Debug("hook-matched-to-session")

	// If adapter returned empty response, try tmux capture as fallback
	if response == "" {
		logger.WithFields(logrus.Fields{
			"session":         session.Name,
			"last_user_prompt": lastUserPrompt,
			"reason":          "adapter returned empty response",
		}).Info("using-tmux-capture-as-fallback-with-user-prompt-filtering")

		// Retry mechanism: wait for Claude to finish thinking
		maxRetries := e.config.Watchdog.MaxRetries
		if maxRetries == 0 {
			maxRetries = constants.MaxHookRetries
		}

		initialDelay, err := time.ParseDuration(e.config.Watchdog.InitialDelay)
		if err != nil {
			initialDelay = constants.DefaultInitialDelay
		}

		retryDelay, err := time.ParseDuration(e.config.Watchdog.RetryDelay)
		if err != nil {
			retryDelay = constants.DefaultRetryDelay
		}

		var lastResponse string

	for attempt := 1; attempt <= maxRetries; attempt++ {
			if attempt == 1 {
				// Initial delay before first capture to let UI render
				logger.WithField("delay", initialDelay).Debug("initial-delay-before-first-tmux-capture")
				time.Sleep(initialDelay)
			} else {
				logger.WithField("attempt", attempt).Info("retrying-tmux-capture")
				time.Sleep(retryDelay)
			}

			tmuxOutput, err := watchdog.CapturePane(session.Name, capturePaneLine)
			if err != nil {
				logger.WithField("error", err).Warn("failed-to-capture-tmux-pane")
				break
			}

			cleanOutput := watchdog.StripANSI(tmuxOutput)

			// Extract content after the user's prompt
			filteredOutput := watchdog.ExtractContentAfterPrompt(cleanOutput, lastUserPrompt)

			logger.WithFields(logrus.Fields{
				"attempt":          attempt,
				"filtered_length":  len(filteredOutput),
				"filtered_preview": filteredOutput[:watchdog.Min(constants.MaxPromptPrefixLength*6, len(filteredOutput))], // ~200 chars for preview
				"is_thinking":      watchdog.IsThinking(filteredOutput),
			}).Debug("extracted-content-after-user-prompt")

			// Check if still thinking in the filtered content
			if watchdog.IsThinking(filteredOutput) {
				logger.WithFields(logrus.Fields{
					"attempt":     attempt,
					"max_retries": maxRetries,
					"reason":      "thinking detected in filtered output",
				}).Debug("claude-is-still-thinking-in-response-area-will-retry")
				lastResponse = filteredOutput // Save for final attempt
				continue
			}

			// Not thinking anymore, remove UI status lines and validate response
			response = watchdog.RemoveUIStatusLines(filteredOutput)

			// Validate response - just check if not empty
			// We already have thinking check and UI filtering, so any content is valid
			if response != "" {
				logger.WithFields(logrus.Fields{
					"source":           "tmux",
					"attempt":           attempt,
					"response_length":  len(response),
					"response_preview": response[:watchdog.Min(constants.MaxPromptPrefixLength*6, len(response))], // ~200 chars for preview
				}).Info("successfully-extracted-response-from-tmux")
				break // Got valid response, stop retrying
			}

			logger.WithFields(logrus.Fields{
				"attempt":          attempt,
				"response_length": len(response),
				"reason":           "empty response",
			}).Debug("response-validation-failed-will-retry")
		}

		// If still no valid response after all retries, use the last capture
		if response == "" && lastResponse != "" {
			logger.WithFields(logrus.Fields{
				"max_retries":       maxRetries,
				"using_last_attempt": true,
				"response_length":   len(lastResponse),
			}).Info("using-last-attempt-capture-still-may-be-thinking")
			response = watchdog.RemoveUIStatusLines(lastResponse)
		}

		if response == "" {
			logger.WithField("max_retries", maxRetries).Warn("all-retry-attempts-exhausted-and-no-valid-response")
		}
	}

	// Update session state to idle
	e.updateSessionState(session.Name, StateIdle)

	// Get the bot channel for this session
	e.sessionMu.RLock()
	botChannel, exists := e.sessionChannels[session.Name]
	e.sessionMu.RUnlock()

	if !exists {
		// No active channel - user might be operating CLI directly
		logger.WithFields(logrus.Fields{
			"session": session.Name,
		}).Debug("no-active-channel-found-skipping-bot-notification")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Hook received (no active bot channel)")
		return
	}

	// Send to the specific bot channel that initiated the request
	logger.WithFields(logrus.Fields{
		"platform": botChannel.Platform,
		"channel":  botChannel.Channel,
		"session":  session.Name,
	}).Info("sending-hook-response-to-bot")
	e.SendToBot(botChannel.Platform, botChannel.Channel, response)

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Hook received")
}

// Stop gracefully stops the engine
func (e *Engine) Stop() error {
	logger.Info("stopping-clibot-engine")

	// Cancel context to stop event loop
	if e.cancel != nil {
		e.cancel()
	}

	// Stop hook server with graceful shutdown
	if e.hookServer != nil {
		logger.Info("stopping-hook-server")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := e.hookServer.Shutdown(ctx); err != nil {
			logger.Errorf("failed-to-gracefully-stop-hook-server: %v", err)
			// Force close if graceful shutdown fails
			e.hookServer.Close()
		} else {
			logger.Info("hook-server-stopped-gracefully")
		}
	}

	// Stop all bots
	for botType, botAdapter := range e.activeBots {
		logger.WithField("bot_type", botType).Info("stopping-bot")
		if err := botAdapter.Stop(); err != nil {
			logger.WithFields(logrus.Fields{
				"bot_type": botType,
				"error":    err,
			}).Error("failed-to-stop-bot")
		}
	}

	logger.Info("engine-stopped")
	return nil
}

// normalizePath normalizes a path for comparison
// Removes trailing slashes from the path.
//
// Note: Relative path expansion is not yet implemented. Paths are compared
// as-is after removing trailing slashes. This works for most cases where both
// paths are either absolute or both relative to the same location.
func normalizePath(path string) string {
	return strings.TrimSuffix(path, "/")
}
