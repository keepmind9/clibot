package core

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/keepmind9/clibot/internal/bot"
	"github.com/keepmind9/clibot/internal/cli"
	"github.com/keepmind9/clibot/internal/logger"
	"github.com/keepmind9/clibot/internal/watchdog"
	"github.com/keepmind9/clibot/pkg/constants"
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
	inputTracker    *InputTracker              // Tracks user input for response extraction
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

	// Initialize input tracker for response extraction
	// Get history size from config, default to 10
	historySize := config.Session.InputHistorySize
	if historySize == 0 {
		historySize = DefaultInputHistorySize
	}

	tracker, err := NewInputTrackerWithSize(filepath.Join(os.Getenv("HOME"), ".clibot", "sessions"), historySize)
	if err != nil {
		logger.WithField("error", err).Warn("failed-to-create-input-tracker-response-extraction-may-be-affected")
		tracker = nil // Continue without tracker
	}

	return &Engine{
		config:          config,
		cliAdapters:     make(map[string]cli.CLIAdapter),
		activeBots:      make(map[string]bot.BotAdapter),
		sessions:        make(map[string]*Session),
		messageChan:     make(chan bot.BotMessage, constants.MessageChannelBufferSize),
		sessionChannels: make(map[string]BotChannel),
		inputTracker:    tracker,
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

	// Step 3: Process key words (tab, esc, stab, enter, ctrlc, etc.)
	// Converts entire input matching keywords to actual key sequences
	processedContent := watchdog.ProcessKeyWords(msg.Content)
	if processedContent != msg.Content {
		logger.WithFields(logrus.Fields{
			"original": msg.Content,
			"processed": fmt.Sprintf("%q", processedContent),
		}).Debug("keyword-converted-to-key-sequence")
	}

	// Step 3.5: Capture before snapshot for incremental extraction (polling mode only)
	// IMPORTANT: Must capture BEFORE recording input to ensure before snapshot
	// only contains state BEFORE user input, not after
	// This snapshot is used to calculate the increment (after - before)
	// In hook mode, snapshots are not needed as hook provides the response directly
	adapter := e.cliAdapters[session.CLIType]
	if e.inputTracker != nil && !adapter.UseHook() {
		beforeCapture, err := watchdog.CapturePane(session.Name, capturePaneLine)
		if err != nil {
			logger.WithFields(logrus.Fields{
				"session": session.Name,
				"cliType": session.CLIType,
				"error":   err,
			}).Warn("failed-to-capture-before-snapshot-falling-back-to-full-extraction")
		} else {
			// IMPORTANT: Strip ANSI codes to match after snapshot format
			// after snapshot comes from WaitForCompletion which uses ExtractStableContent (StripANSI)
			// This ensures before and after snapshots are comparable
			beforeCapture = watchdog.StripANSI(beforeCapture)

			if err := e.inputTracker.RecordBeforeSnapshot(session.Name, session.CLIType, beforeCapture); err != nil {
				logger.WithFields(logrus.Fields{
					"session": session.Name,
					"cliType": session.CLIType,
					"error":   err,
				}).Warn("failed-to-save-before-snapshot-will-use-full-extraction")
			} else {
				logger.WithFields(logrus.Fields{
					"session":     session.Name,
					"cliType":     session.CLIType,
					"capture_len": len(beforeCapture),
				}).Debug("before-snapshot-captured")
			}
		}
	}

	// Step 3.6: Record user input for response extraction (polling mode)
	// This recorded input will be used as an anchor to extract the correct response
	// from tmux output, which may contain historical conversation data
	// IMPORTANT: Recorded AFTER before snapshot to ensure clean state
	if e.inputTracker != nil {
		if err := e.inputTracker.RecordInput(session.Name, msg.Content); err != nil {
			logger.WithFields(logrus.Fields{
				"session": session.Name,
				"error":   err,
			}).Warn("failed-to-record-input-response-extraction-may-be-affected")
		} else {
			logger.WithFields(logrus.Fields{
				"session":      session.Name,
				"input_length": len(msg.Content),
			}).Debug("input-recorded-for-response-extraction")
		}
	}

	// Step 4: Send to CLI
	if err := adapter.SendInput(session.Name, processedContent); err != nil {
		logger.WithFields(logrus.Fields{
			"session": session.Name,
			"error":   err,
		}).Error("failed-to-send-input-to-cli")
		// On error, keep session in its current state (don't change to idle)
		// This preserves the waiting input state if it was interactive
		e.SendToBot(msg.Platform, msg.Channel, fmt.Sprintf("❌ Failed to send input: %v", err))
		return
	}

	logger.WithFields(logrus.Fields{
		"session": session.Name,
		"cli":     session.CLIType,
	}).Info("input-sent-to-cli")

	// Step 5: Update session state to processing
	e.updateSessionState(session.Name, StateProcessing)

	// Step 6: Check if CLI adapter uses hook mode
	// Hook mode: CLI sends notification when complete, no polling needed
	// Polling mode: Need to monitor tmux output to detect completion
	if adapter.UseHook() {
		// Hook mode - CLI will notify via HTTP hook when complete
		// No need to start watchdog polling
		logger.WithFields(logrus.Fields{
			"session": session.Name,
			"mode":    "hook",
		}).Debug("skipping-watchdog-startup-in-hook-mode")
		return
	}

	// Step 7: Polling mode - start watchdog monitoring
	// Watchdog polls tmux output to detect when CLI completes response
	ctx, cleanup := e.startNewWatchdogForSession(session.Name)

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
	// This returns all stable content from tmux, which may include historical data
	rawContent, err := watchdog.WaitForCompletion(session.Name, pollingConfig, ctx)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"session": session.Name,
			"error":   err,
		}).Error("polling-failed")

		// Update session state
		e.updateSessionState(session.Name, StateIdle)

		return err
	}

	// Capture after snapshot for incremental extraction
	var response string
	if e.inputTracker != nil {
		// Save after snapshot
		if err := e.inputTracker.RecordAfterSnapshot(session.Name, session.CLIType, rawContent); err != nil {
			logger.WithFields(logrus.Fields{
				"session": session.Name,
				"cliType": session.CLIType,
				"error":   err,
			}).Warn("failed-to-save-after-snapshot-will-use-full-extraction")
			// Fall back to full content
			response = rawContent
		} else {
			logger.WithFields(logrus.Fields{
				"session":     session.Name,
				"cliType":     session.CLIType,
				"capture_len": len(rawContent),
			}).Debug("after-snapshot-captured")

			// Try incremental extraction using snapshots
			beforeSnapshot, _, snapshotErr := e.inputTracker.GetSnapshotPair(session.Name, session.CLIType)
			if snapshotErr != nil {
				logger.WithFields(logrus.Fields{
					"session":  session.Name,
					"cliType":  session.CLIType,
					"error":    snapshotErr,
					"event":    "parser_failed_to_get_snapshot_fallback",
					"fallback": "raw_content",
				}).Warn("parser_failed_to_get_snapshot_falling_back_to_raw")
				response = rawContent
			} else if beforeSnapshot == "" {
				// No before snapshot available, fall back to legacy method
				logger.WithFields(logrus.Fields{
					"session":   session.Name,
					"cliType":   session.CLIType,
					"event":     "parser_no_before_snapshot_fallback",
					"algorithm": "prompt_matching",
				}).Info("parser_no_before_snapshot_fallback_to_prompt_matching")
				response = e.extractResponseUsingInputs(session.Name, rawContent)
			} else {
				// Use incremental extraction with snapshots
				response = watchdog.ExtractIncrement(rawContent, beforeSnapshot)

				rawLines := len(strings.Split(rawContent, "\n"))
				extractedLines := len(strings.Split(response, "\n"))

				logger.WithFields(logrus.Fields{
					"session":          session.Name,
					"cliType":          session.CLIType,
					"raw_length":       len(rawContent),
					"raw_lines":        rawLines,
					"extracted_length": len(response),
					"extracted_lines":  extractedLines,
					"algorithm":        "incremental_snapshot",
					"event":            "parser_using_incremental_snapshot",
				}).Info("parser_using_incremental_snapshot")
			}
		}
	} else {
		// No input tracker available, use full content
		logger.WithFields(logrus.Fields{
			"session": session.Name,
		}).Debug("no-input-tracker-using-full-tmux-content")
		response = rawContent
	}

	// Send response to user
	logger.WithFields(logrus.Fields{
		"session":        session.Name,
		"response_length": len(response),
		"mode":           "polling",
		"event":          "parser_response_completed",
	}).Info("parser_response_completed_sending_to_user")

	e.sendResponseToUser(session.Name, response)

	// Update session state
	e.updateSessionState(session.Name, StateIdle)

	return nil
}

// extractResponseUsingInputs extracts response using recorded inputs (fallback method)
// This is used when incremental extraction is not available
func (e *Engine) extractResponseUsingInputs(sessionName string, rawContent string) string {
	// Get all recorded inputs (from newest to oldest)
	inputs, err := e.inputTracker.GetAllInputs(sessionName)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"session": sessionName,
			"error":   err,
		}).Warn("failed-to-get-recorded-inputs-using-full-content")
		return rawContent
	}

	if len(inputs) == 0 {
		// No recorded input, use full content
		logger.WithFields(logrus.Fields{
			"session": sessionName,
		}).Debug("no-recorded-inputs-using-full-tmux-content")
		return rawContent
	}

	// Try to extract content using any of the recorded inputs (from newest to oldest)
	inputRecords := make([]watchdog.InputRecord, len(inputs))
	for i, input := range inputs {
		inputRecords[i] = watchdog.InputRecord{
			Timestamp: input.Timestamp,
			Content:   input.Content,
		}
	}

	response := watchdog.ExtractContentAfterAnyInput(rawContent, inputRecords)

	// Calculate response time using the most recent input
	responseTime := time.Now().UnixMilli() - inputs[0].Timestamp
	rawLines := len(strings.Split(rawContent, "\n"))
	extractedLines := len(strings.Split(response, "\n"))

	logger.WithFields(logrus.Fields{
		"session":         sessionName,
		"response_time":   fmt.Sprintf("%dms", responseTime),
		"raw_length":      len(rawContent),
		"raw_lines":       rawLines,
		"extracted_length": len(response),
		"extracted_lines": extractedLines,
		"tried_inputs":    len(inputs),
		"algorithm":       "prompt_matching",
		"event":           "parser_using_prompt_matching",
	}).Info("parser_using_prompt_matching_fallback")

	return response
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
