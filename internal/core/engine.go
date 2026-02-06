package core

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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
	capturePaneLine     = constants.SnapshotCaptureLines
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
	"help":   {},
	"status": {},
	"slist":  {},
	"whoami": {},
	"view":   {},
	"echo":   {},
	"snew":   {},
	"sdel":   {},
	"suse":   {},
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

	// Handle commands with string arguments (suse, snew, sdel)
	// These commands accept arbitrary string arguments (session names, paths, etc.)
	fields := strings.Fields(input)
	if len(fields) > 1 {
		cmd := fields[0]
		// Only check known commands that accept string arguments
		if cmd == "suse" || cmd == "snew" || cmd == "sdel" {
			if _, exists := specialCommands[cmd]; exists {
				return cmd, true, fields[1:]
			}
		}
	}

	return "", false, nil
}

// Engine is the core scheduling engine that manages CLI sessions and bot connections
type Engine struct {
	config          *Config
	cliAdapters     map[string]cli.CLIAdapter // CLI type -> adapter
	activeBots      map[string]bot.BotAdapter // Bot type -> adapter
	sessions        map[string]*Session       // Session name -> Session
	sessionMu       sync.RWMutex              // Mutex for session access
	messageChan     chan bot.BotMessage       // Bot message channel
	hookServer      *http.Server              // HTTP server for hooks
	sessionChannels map[string]BotChannel     // Session name -> active bot channel (for routing responses)
	userSessions    map[string]string         // User key (platform:userID) -> current session name
	inputTracker    *InputTracker             // Tracks user input for response extraction
	ctx             context.Context           // Context for cancellation
	cancel          context.CancelFunc        // Cancel function for graceful shutdown
}

// BotChannel represents a bot channel for sending responses
type BotChannel struct {
	Platform string // "discord", "telegram", "feishu", etc.
	Channel  string // Channel ID (platform-specific)
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

	// Set default for max dynamic sessions if not configured
	if config.Session.MaxDynamicSessions == 0 {
		config.Session.MaxDynamicSessions = 50
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
		userSessions:    make(map[string]string), // user key -> current session name
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

		// Determine start command: use configured value or default to CLI type
		startCmd := sessionConfig.StartCmd
		if startCmd == "" {
			startCmd = sessionConfig.CLIType
		}

		// Create new session
		session := &Session{
			Name:      sessionConfig.Name,
			CLIType:   sessionConfig.CLIType,
			WorkDir:   sessionConfig.WorkDir,
			StartCmd:  startCmd,
			State:     StateIdle,
			CreatedAt: time.Now().Format(time.RFC3339),
			IsDynamic: false, // Configured sessions are not dynamic
			CreatedBy: "",
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
			if err := adapter.CreateSession(session.Name, session.WorkDir, session.StartCmd); err != nil {
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

	// Step 2: Get the active session for this user
	userKey := getUserKey(msg.Platform, msg.UserID)

	e.sessionMu.Lock()
	sessionName, userHasSession := e.userSessions[userKey]
	var session *Session
	sessionInvalid := false

	if userHasSession {
		session = e.sessions[sessionName]
		if session == nil {
			// User's selected session no longer exists, clean up the stale reference
			delete(e.userSessions, userKey)
			sessionInvalid = true
			logger.WithFields(logrus.Fields{
				"user":          userKey,
				"stale_session": sessionName,
			}).Warn("cleaned-stale-user-session-reference")
		}
	}
	e.sessionMu.Unlock()

	if session == nil {
		// Build list of available sessions for the user
		e.sessionMu.RLock()
		availableSessions := make([]string, 0, len(e.sessions))
		for _, s := range e.sessions {
			availableSessions = append(availableSessions,
				fmt.Sprintf("  • %s (%s)", s.Name, s.CLIType))
		}
		e.sessionMu.RUnlock()

		if sessionInvalid {
			logger.WithFields(logrus.Fields{
				"user":          userKey,
				"stale_session": sessionName,
			}).Warn("user-selected-session-no-longer-exists")
		} else {
			logger.WithFields(logrus.Fields{
				"user": userKey,
			}).Warn("user-has-no-session-selected")
		}

		// Build error message with available sessions
		errorMsg := "❌ Please select a session first\n\n"
		if sessionInvalid {
			errorMsg += fmt.Sprintf("⚠️  Your previous session '%s' no longer exists\n\n", sessionName)
		}
		errorMsg += "Available sessions:\n"
		for _, s := range availableSessions {
			errorMsg += s + "\n"
		}
		errorMsg += "\n💡 Use: suse <session_name> to select a session"

		e.SendToBot(msg.Platform, msg.Channel, errorMsg)
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
			"original":  msg.Content,
			"processed": fmt.Sprintf("%q", processedContent),
		}).Debug("keyword-converted-to-key-sequence")
	}

	// Step 3.5: Capture before snapshot for incremental extraction (polling mode only)
	adapter := e.cliAdapters[session.CLIType]
	var beforeCapture string
	if e.inputTracker != nil && !adapter.UseHook() {
		var err error
		beforeCapture, err = watchdog.CapturePane(session.Name, capturePaneLine)
		if err != nil {
			logger.WithFields(logrus.Fields{
				"session": session.Name,
				"cliType": session.CLIType,
				"error":   err,
			}).Warn("failed-to-capture-before-snapshot-will-use-prompt-matching-fallback")
		} else {
			// IMPORTANT: Strip ANSI codes to match after snapshot format
			beforeCapture = watchdog.StripANSI(beforeCapture)

			if err := e.inputTracker.RecordBeforeSnapshot(session.Name, session.CLIType, beforeCapture); err != nil {
				logger.WithFields(logrus.Fields{
					"session": session.Name,
					"cliType": session.CLIType,
					"error":   err,
				}).Warn("failed-to-save-before-snapshot-will-use-prompt-matching-fallback")
			} else {
				logger.WithFields(logrus.Fields{
					"session":     session.Name,
					"cliType":     session.CLIType,
					"capture_len": len(beforeCapture),
				}).Debug("before-snapshot-captured")
			}
		}
	}
	// NOTE: If beforeCapture is empty due to capture failure, the system will use
	// prompt matching as fallback (userPrompt is always passed to watchdog)

	// Step 3.6: Record user input for response extraction (polling mode)
	if e.inputTracker != nil {
		if err := e.inputTracker.RecordInput(session.Name, msg.Content); err != nil {
			logger.WithFields(logrus.Fields{
				"session": session.Name,
				"error":   err,
			}).Warn("failed-to-record-input-response-extraction-may-be-affected")
		}
	}

	// Step 4: Send to CLI
	if err := adapter.SendInput(session.Name, processedContent); err != nil {
		logger.WithFields(logrus.Fields{
			"session": session.Name,
			"error":   err,
		}).Error("failed-to-send-input-to-cli")
		e.SendToBot(msg.Platform, msg.Channel, fmt.Sprintf("❌ Failed to send input: %v", err))
		return
	}

	// Step 5: Update session state to processing
	e.updateSessionState(session.Name, StateProcessing)

	// Step 6: Hook mode check
	if adapter.UseHook() {
		return
	}

	// Step 7: Polling mode - start watchdog monitoring
	ctx, cleanup := e.startNewWatchdogForSession(session.Name)

	go func(sessionName string, watchdogCtx context.Context, pContent string, bCapture string) {
		defer func() {
			if r := recover(); r != nil {
				logger.WithFields(logrus.Fields{
					"session": sessionName,
					"panic":   r,
				}).Error("watchdog-panic-recovered")
			}
			cleanup()
		}()

		e.sessionMu.RLock()
		session, exists := e.sessions[sessionName]
		e.sessionMu.RUnlock()

		if !exists {
			return
		}

		if err := e.startWatchdogWithContext(watchdogCtx, session, pContent, bCapture); err != nil {
			logger.WithFields(logrus.Fields{
				"session": sessionName,
				"error":   err,
			}).Error("watchdog-failed")
		}
	}(session.Name, ctx, msg.Content, beforeCapture)
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
	case "slist":
		e.listSessions(msg)
	case "suse":
		e.handleUseSession(args, msg)
	case "status":
		e.showStatus(msg)
	case "whoami":
		e.showWhoami(msg)
	case "view":
		// Reconstruct parts for captureView (expects full parts array)
		parts := append([]string{command}, args...)
		e.captureView(msg, parts)
	case "echo":
		e.handleEcho(msg)
	case "snew":
		e.handleNewSession(args, msg)
	case "sdel":
		e.handleDeleteSession(args, msg)
	default:
		e.SendToBot(msg.Platform, msg.Channel,
			fmt.Sprintf("❌ Unknown command: %s\nUse 'help' to see available commands", command))
	}
}

// listSessions lists all available sessions
func (e *Engine) listSessions(msg bot.BotMessage) {
	e.sessionMu.RLock()
	defer e.sessionMu.RUnlock()

	// Get user's current session
	userKey := getUserKey(msg.Platform, msg.UserID)
	currentSessionName, hasCurrent := e.userSessions[userKey]

	response := "📋 Available Sessions:\n\n"

	if hasCurrent {
		response += fmt.Sprintf("✅ Your current session: **%s**\n\n", currentSessionName)
	} else {
		response += "⚠️  You haven't selected a session yet\n\n"
	}

	// Categorize sessions
	var staticSessions, dynamicSessions []*Session
	for _, session := range e.sessions {
		if session.IsDynamic {
			dynamicSessions = append(dynamicSessions, session)
		} else {
			staticSessions = append(staticSessions, session)
		}
	}

	// Display static sessions
	if len(staticSessions) > 0 {
		response += "Static Sessions (configured):\n"
		for _, session := range staticSessions {
			marker := ""
			if hasCurrent && session.Name == currentSessionName {
				marker = " ⬅️ **CURRENT**"
			}
			response += fmt.Sprintf("  • %s (%s) - %s [static]%s\n",
				session.Name, session.CLIType, session.State, marker)
		}
		response += "\n"
	}

	// Display dynamic sessions
	if len(dynamicSessions) > 0 {
		response += "Dynamic Sessions (created via IM):\n"
		for _, session := range dynamicSessions {
			marker := ""
			if hasCurrent && session.Name == currentSessionName {
				marker = " ⬅️ **CURRENT**"
			}
			response += fmt.Sprintf("  • %s (%s) - %s [dynamic, created by %s]%s\n",
				session.Name, session.CLIType, session.State, session.CreatedBy, marker)
		}
	}

	if !hasCurrent && len(e.sessions) > 0 {
		response += "\n💡 Use: suse <session_name> to select a session\n"
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

		// Add origin tag
		origin := "[static]"
		if session.IsDynamic {
			origin = fmt.Sprintf("[dynamic, created by %s]", session.CreatedBy)
		}

		response += fmt.Sprintf("  %s %s (%s) - %s %s\n", status, session.Name, session.CLIType, session.State, origin)
	}

	e.SendToBot(msg.Platform, msg.Channel, response)
}

// showWhoami shows current session information
func (e *Engine) showWhoami(msg bot.BotMessage) {
	userKey := getUserKey(msg.Platform, msg.UserID)

	e.sessionMu.RLock()
	sessionName, hasSession := e.userSessions[userKey]
	var session *Session
	if hasSession {
		session = e.sessions[sessionName]
	}
	e.sessionMu.RUnlock()

	if session == nil {
		response := fmt.Sprintf("🔍 **Your Information**\n\n"+
			"**Platform:** %s\n"+
			"**User ID:** `%s`\n"+
			"**Channel ID:** `%s`\n"+
			"**Current Session:** ⚠️  Not selected\n\n"+
			"💡 Use 'slist' to see available sessions\n"+
			"   Use 'suse <name>' to select a session",
			msg.Platform, msg.UserID, msg.Channel)
		e.SendToBot(msg.Platform, msg.Channel, response)
		return
	}

	sessionType := "Static (configured)"
	if session.IsDynamic {
		sessionType = fmt.Sprintf("Dynamic (created by %s)", session.CreatedBy)
	}

	response := fmt.Sprintf("🔍 **Your Information**\n\n"+
		"**Platform:** %s\n"+
		"**User ID:** `%s`\n"+
		"**Channel ID:** `%s`\n\n"+
		"**✅ Current Session:** %s\n"+
		"**CLI Type:** %s\n"+
		"**State:** %s\n"+
		"**WorkDir:** %s\n"+
		"**Type:** %s",
		msg.Platform, msg.UserID, msg.Channel,
		session.Name, session.CLIType, session.State, session.WorkDir, sessionType)
	e.SendToBot(msg.Platform, msg.Channel, response)
}

// showHelp displays help information about available commands and keywords
func (e *Engine) showHelp(msg bot.BotMessage) {
	help := `📖 **clibot Help**

**Special Commands** (no prefix required):
  help         - Show this help message
  slist        - List all available sessions
  suse <name>  - Switch current session
  status       - Show status of all sessions
  whoami       - Show your current session info
  view [n]     - View CLI output (default: 20 lines)
  echo         - Echo your IM user info (for whitelist config)
  snew <name> <cli_type> <work_dir> [cmd] - Create new session (admin only)
  sdel <name>  - Delete dynamic session (admin only)

**Special Keywords** (exact match, case-insensitive):
  tab            - Send Tab key
  esc            - Send Escape key
  stab/s-tab     - Send Shift+Tab
  enter          - Send Enter key
  ctrlc/ctrl-c    - Send Ctrl+C (interrupt)
  ctrlt/ctrl-t    - Send Ctrl+T

**Usage Examples:**
  help              → Show help
  slist             → List all sessions
  suse myproject    → Switch to session 'myproject'
  status            → Show status
  tab               → Send Tab key to CLI
  ctrl-c            → Interrupt current process
  ctrl-t            → Trigger Ctrl+T action
  view 100          → View last 100 lines of output
  snew myproject claude ~/work  → Create new session

**Tips:**
  - Special commands are exact match (case-sensitive)
  - Special keywords are case-insensitive
  - Any other input will be sent to the CLI
  - Use "suse" to switch between sessions
  - Use "help" anytime to see this message`

	e.SendToBot(msg.Platform, msg.Channel, help)
}

// handleEcho returns the user's IM information to help with whitelist configuration
func (e *Engine) handleEcho(msg bot.BotMessage) {
	response := fmt.Sprintf("🔍 **Your IM Information**\n\n"+
		"**Platform:** %s\n"+
		"**User ID:** `%s` (Use this for whitelist)\n"+
		"**Channel ID:** `%s`",
		msg.Platform, msg.UserID, msg.Channel)

	e.SendToBot(msg.Platform, msg.Channel, response)
}

// handleNewSession creates a new dynamic session (admin only)
// Usage: new <name> <cli_type> <work_dir> [start_cmd]
func (e *Engine) handleNewSession(args []string, msg bot.BotMessage) {
	logger.WithFields(logrus.Fields{
		"platform": msg.Platform,
		"user_id":  msg.UserID,
		"args":     args,
	}).Info("handle-new-session-command")

	// 1. Permission check
	if !e.config.IsAdmin(msg.Platform, msg.UserID) {
		e.SendToBot(msg.Platform, msg.Channel, "❌ Permission denied: admin only")
		return
	}

	// 2. Parameter validation
	if len(args) < 3 {
		e.SendToBot(msg.Platform, msg.Channel,
			"❌ Invalid arguments\nUsage: snew <name> <cli_type> <work_dir> [start_cmd]")
		return
	}

	name := args[0]
	cliType := args[1]
	workDir := args[2]
	startCmd := cliType
	if len(args) >= 4 {
		startCmd = args[3]
	}

	// 3. Validate session name format
	if !isValidSessionName(name) {
		e.SendToBot(msg.Platform, msg.Channel,
			fmt.Sprintf("❌ Invalid session name: '%s'\nUse letters, numbers, hyphen, underscore only", name))
		return
	}

	// 4. Validate CLI type
	adapter, exists := e.cliAdapters[cliType]
	if !exists {
		e.SendToBot(msg.Platform, msg.Channel,
			fmt.Sprintf("❌ Invalid CLI type: '%s'\nSupported: claude, gemini, opencode", cliType))
		return
	}

	// 5. Validate and expand work directory
	expandedDir, err := expandPath(workDir)
	if err != nil {
		e.SendToBot(msg.Platform, msg.Channel,
			fmt.Sprintf("❌ Invalid work_dir: %v", err))
		return
	}

	// Check if directory exists
	if _, err := exec.Command("test", "-d", expandedDir).CombinedOutput(); err != nil {
		e.SendToBot(msg.Platform, msg.Channel,
			fmt.Sprintf("❌ Work directory does not exist: %s", expandedDir))
		return
	}

	e.sessionMu.Lock()
	defer e.sessionMu.Unlock()

	// 6. Check for duplicate session name
	if _, exists := e.sessions[name]; exists {
		e.SendToBot(msg.Platform, msg.Channel,
			fmt.Sprintf("❌ Session '%s' already exists", name))
		return
	}

	// 7. Check dynamic session limit
	dynamicCount := 0
	for _, s := range e.sessions {
		if s.IsDynamic {
			dynamicCount++
		}
	}
	if dynamicCount >= e.config.Session.MaxDynamicSessions {
		e.SendToBot(msg.Platform, msg.Channel,
			fmt.Sprintf("❌ Maximum dynamic session limit reached (%d)", e.config.Session.MaxDynamicSessions))
		return
	}

	// 8. Create session object
	session := &Session{
		Name:      name,
		CLIType:   cliType,
		WorkDir:   expandedDir,
		StartCmd:  startCmd,
		State:     StateIdle,
		CreatedAt: time.Now().Format(time.RFC3339),
		IsDynamic: true,
		CreatedBy: fmt.Sprintf("%s:%s", msg.Platform, msg.UserID),
	}

	// 9. Create tmux session and start CLI
	if err := adapter.CreateSession(name, expandedDir, startCmd); err != nil {
		logger.WithField("error", err).Error("failed-to-create-dynamic-session")
		e.SendToBot(msg.Platform, msg.Channel,
			fmt.Sprintf("❌ Failed to create session: %v", err))
		return
	}

	// 10. Add to sessions map
	e.sessions[name] = session

	logger.WithFields(logrus.Fields{
		"action":     "create_session",
		"session":    name,
		"platform":   msg.Platform,
		"user_id":    msg.UserID,
		"cli_type":   cliType,
		"work_dir":   expandedDir,
		"start_cmd":  startCmd,
		"is_dynamic": true,
	}).Info("admin-created-dynamic-session")

	// 11. Success response
	e.SendToBot(msg.Platform, msg.Channel,
		fmt.Sprintf("✅ Session '%s' created successfully\nCLI: %s\nWorkDir: %s\nStartCmd: %s",
			name, cliType, expandedDir, startCmd))
}

// isValidSessionName checks if session name is valid
// Valid characters: letters, numbers, hyphen, underscore
func isValidSessionName(name string) bool {
	if name == "" || len(name) > 100 {
		return false
	}
	matched, _ := regexp.MatchString(`^[a-zA-Z0-9_-]+$`, name)
	return matched
}

// expandPath expands ~ and environment variables in path
func expandPath(path string) (string, error) {
	if strings.HasPrefix(path, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(homeDir, path[2:]), nil
	}
	if path == "~" {
		return os.UserHomeDir()
	}
	return path, nil
}

// handleUseSession switches the user's current active session
// Usage: suse <session_name>
func (e *Engine) handleUseSession(args []string, msg bot.BotMessage) {
	logger.WithFields(logrus.Fields{
		"platform": msg.Platform,
		"user_id":  msg.UserID,
		"args":     args,
	}).Info("handle-use-session-command")

	// 1. Parameter validation
	if len(args) < 1 {
		e.SendToBot(msg.Platform, msg.Channel,
			"❌ Invalid arguments\nUsage: suse <session_name>")
		return
	}

	sessionName := args[0]
	userKey := getUserKey(msg.Platform, msg.UserID)

	e.sessionMu.Lock()
	defer e.sessionMu.Unlock()

	// 2. Check if session exists
	session, exists := e.sessions[sessionName]
	if !exists {
		e.SendToBot(msg.Platform, msg.Channel,
			fmt.Sprintf("❌ Session '%s' does not exist\nUse 'slist' to see available sessions", sessionName))
		return
	}

	// 3. Update user's current session
	wasSwitched := e.userSessions[userKey] != sessionName
	e.userSessions[userKey] = sessionName

	logger.WithFields(logrus.Fields{
		"user":    userKey,
		"session": sessionName,
		"cli":     session.CLIType,
	}).Info("user-switched-session")

	// 4. Success response
	response := fmt.Sprintf("✅ Your current session is now: **%s**\n\n", sessionName)
	response += fmt.Sprintf("📊 Session Info:\n")
	response += fmt.Sprintf("  • CLI: %s\n", session.CLIType)
	response += fmt.Sprintf("  • State: %s\n", session.State)
	response += fmt.Sprintf("  • WorkDir: %s\n", session.WorkDir)
	if session.IsDynamic {
		response += fmt.Sprintf("  • Type: Dynamic (created by %s)\n", session.CreatedBy)
	} else {
		response += "  • Type: Static (configured)\n"
	}

	if !wasSwitched {
		response += "\nℹ️  You were already using this session"
	}

	e.SendToBot(msg.Platform, msg.Channel, response)
}

// handleDeleteSession deletes a dynamic session (admin only)
// Usage: sdel <name>
func (e *Engine) handleDeleteSession(args []string, msg bot.BotMessage) {
	logger.WithFields(logrus.Fields{
		"platform": msg.Platform,
		"user_id":  msg.UserID,
		"args":     args,
	}).Info("handle-delete-session-command")

	// 1. Permission check
	if !e.config.IsAdmin(msg.Platform, msg.UserID) {
		e.SendToBot(msg.Platform, msg.Channel, "❌ Permission denied: admin only")
		return
	}

	// 2. Parameter validation
	if len(args) < 1 {
		e.SendToBot(msg.Platform, msg.Channel,
			"❌ Invalid arguments\nUsage: sdel <name>")
		return
	}

	name := args[0]

	e.sessionMu.Lock()
	defer e.sessionMu.Unlock()

	// 3. Check if session exists
	session, exists := e.sessions[name]
	if !exists {
		e.SendToBot(msg.Platform, msg.Channel,
			fmt.Sprintf("❌ Session '%s' not found", name))
		return
	}

	// 4. Only allow deleting dynamic sessions
	if !session.IsDynamic {
		e.SendToBot(msg.Platform, msg.Channel,
			fmt.Sprintf("❌ Cannot delete configured session '%s'\n"+
				"Please remove it from the config file manually", name))
		return
	}

	// 5. Kill tmux session
	cmd := exec.Command("tmux", "kill-session", "-t", name)
	if err := cmd.Run(); err != nil {
		logger.WithFields(logrus.Fields{
			"session": name,
			"error":   err,
		}).Warn("failed-to-kill-tmux-session")
	}

	// 6. Remove from sessions map
	delete(e.sessions, name)

	// 7. Clean up user sessions that reference this deleted session
	cleanedUsers := 0
	for userKey, sessionName := range e.userSessions {
		if sessionName == name {
			delete(e.userSessions, userKey)
			cleanedUsers++
		}
	}

	if cleanedUsers > 0 {
		logger.WithFields(logrus.Fields{
			"session":       name,
			"cleaned_users": cleanedUsers,
		}).Info("cleaned-user-sessions-after-deletion")
	}

	logger.WithFields(logrus.Fields{
		"action":   "delete_session",
		"session":  name,
		"platform": msg.Platform,
		"user_id":  msg.UserID,
	}).Info("admin-deleted-dynamic-session")

	// 8. Success response
	response := fmt.Sprintf("✅ Session '%s' deleted successfully", name)
	if cleanedUsers > 0 {
		response += fmt.Sprintf("\n🔄 %d user(s) switched to default session", cleanedUsers)
	}
	e.SendToBot(msg.Platform, msg.Channel, response)
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
		"session":         session.Name,
		"lines_requested": lines,
		"output_length":   len(cleanOutput),
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

	// Return first available session as fallback
	for _, session := range e.sessions {
		return session
	}

	return nil
}

// getUserKey generates a unique key for a user across platforms
func getUserKey(platform, userID string) string {
	return fmt.Sprintf("%s:%s", platform, userID)
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
func (e *Engine) startWatchdog(session *Session, userPrompt string, beforeCapture string) error {
	adapter := e.cliAdapters[session.CLIType]

	// Check which mode to use
	if adapter.UseHook() {
		logger.WithField("session", session.Name).Debug("using-hook-mode")
		return e.runWatchdogWithHook(session)
	} else {
		logger.WithField("session", session.Name).Debug("using-polling-mode")
		return e.runWatchdogPolling(session, userPrompt, beforeCapture)
	}
}

// startWatchdogWithContext starts monitoring with a cancellable context
// This prevents goroutine leaks when multiple messages are sent rapidly
func (e *Engine) startWatchdogWithContext(ctx context.Context, session *Session, userPrompt string, beforeCapture string) error {
	adapter := e.cliAdapters[session.CLIType]

	// Check which mode to use
	if adapter.UseHook() {
		logger.WithField("session", session.Name).Debug("using-hook-mode")
		return e.runWatchdogWithHook(session)
	} else {
		logger.WithField("session", session.Name).Debug("using-polling-mode")
		return e.runWatchdogPollingWithContext(ctx, session, userPrompt, beforeCapture)
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
func (e *Engine) runWatchdogPolling(session *Session, userPrompt string, beforeCapture string) error {
	return e.runWatchdogPollingWithContext(e.ctx, session, userPrompt, beforeCapture)
}

// runWatchdogPollingWithContext implements polling-based monitoring with cancellable context
// This prevents goroutine leaks when multiple messages are sent rapidly
func (e *Engine) runWatchdogPollingWithContext(ctx context.Context, session *Session, userPrompt string, beforeCapture string) error {
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

	// Get input history for anchor matching
	var inputs []watchdog.InputRecord
	if e.inputTracker != nil {
		history, err := e.inputTracker.GetAllInputs(session.Name)
		if err == nil && len(history) > 0 {
			inputs = make([]watchdog.InputRecord, len(history))
			for i, h := range history {
				inputs[i] = watchdog.InputRecord{
					Timestamp: h.Timestamp,
					Content:   h.Content,
				}
			}
		}
	}

	// If no history found, at least use the current prompt
	if len(inputs) == 0 && userPrompt != "" {
		inputs = []watchdog.InputRecord{{Content: userPrompt}}
	}

	// Wait for completion (output stability)
	// WaitForCompletion now returns both the cleaned response and the full raw content for snapshots
	response, rawContent, err := watchdog.WaitForCompletion(session.Name, inputs, beforeCapture, pollingConfig, ctx)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"session": session.Name,
			"error":   err,
		}).Error("polling-failed")

		// Update session state
		e.updateSessionState(session.Name, StateIdle)

		return err
	}

	// Step 8: Persistence - Record the after snapshot for the next turn's alignment
	if e.inputTracker != nil && rawContent != "" {
		if err := e.inputTracker.RecordAfterSnapshot(session.Name, session.CLIType, rawContent); err != nil {
			logger.WithFields(logrus.Fields{
				"session": session.Name,
				"cliType": session.CLIType,
				"error":   err,
			}).Warn("failed-to-save-after-snapshot")
		} else {
			logger.WithFields(logrus.Fields{
				"session":     session.Name,
				"capture_len": len(rawContent),
			}).Debug("after-snapshot-persisted")
		}
	}

	// Send response to user
	if response != "" {
		logger.WithFields(logrus.Fields{
			"session":         session.Name,
			"response_length": len(response),
			"mode":            "polling",
			"event":           "parser_response_completed",
		}).Info("parser_response_completed_sending_to_user")

		e.sendResponseToUser(session.Name, response)
	} else {
		// response is empty but rawContent has content - this might indicate:
		// 1. Extraction logic correctly determined no new content
		// 2. Extraction bug - need to investigate
		logger.WithFields(logrus.Fields{
			"session":            session.Name,
			"raw_content_length": len(rawContent),
			"mode":               "polling",
			"event":              "parser_no_response_extracted",
		}).Warn("response-extraction-returned-empty-check-if-this-is-expected")
	}

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

	// Step 0: Check if content is empty (trimmed)
	// We don't send empty messages to avoid bot API errors
	if strings.TrimSpace(content) == "" {
		logger.WithFields(logrus.Fields{
			"session": sessionName,
			"event":   "skip_empty_response",
		}).Info("skipping-empty-response-delivery")
		return
	}

	// Send response
	logger.WithFields(logrus.Fields{
		"session":         sessionName,
		"platform":        botChannel.Platform,
		"channel":         botChannel.Channel,
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
