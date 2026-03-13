package core

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
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
	"github.com/keepmind9/clibot/internal/proxy"
	"github.com/keepmind9/clibot/internal/watchdog"
	"github.com/keepmind9/clibot/pkg/constants"
	"github.com/sirupsen/logrus"
)

const (
	// maxSpecialCommandInputLength is the maximum allowed input length for special commands.
	// This prevents DoS attacks from extremely long inputs.
	maxSpecialCommandInputLength = 10000 // 10KB
)

// specialCommands defines commands that can be used without a prefix.
// These are matched exactly (case-sensitive) for optimal performance.
//
// Performance: O(1) map lookup for exact match commands.
var specialCommands = map[string]struct{}{
	"help":    {},
	"帮助":      {},
	"status":  {},
	"slist":   {},
	"sstatus": {},
	"whoami":  {},
	"echo":    {},
	"snew":    {},
	"sdel":    {},
	"suse":    {},
	"sclose":  {},
	"ssnew":   {},
	"scd":     {},
	"ssls":    {},
	"sssw":    {},
	"snewg":   {},
}

// isSpecialCommand checks if input is a special command.
//
// Matching strategy (exact match for maximum performance):
//   - Exact match: "help", "status", "sessions", "whoami", "echo"
//   - With args: "suse", "snew", "sdel" (with string arguments)
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

	// Handle optional leading slash for mobile apps/link clicking compatibility
	input = strings.TrimPrefix(input, "/")

	// Fast path: exact match for commands without arguments.
	// This covers 95% of cases with a single O(1) map lookup.
	if _, exists := specialCommands[input]; exists {
		return input, true, nil
	}

	// Handle commands with string arguments (suse, snew, sdel, sclose, sstatus, etc.)
	// These commands accept arbitrary string arguments (session names, paths, etc.)
	fields := strings.Fields(input)
	if len(fields) > 1 {
		cmd := fields[0]
		// Only check known commands that accept string arguments
		if cmd == "suse" || cmd == "snew" || cmd == "sdel" || cmd == "sclose" || cmd == "sstatus" ||
			cmd == "sssw" || cmd == "scd" || cmd == "ssnew" || cmd == "ssls" || cmd == "snewg" {
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
	cmdLocksMu      sync.RWMutex              // Protects sessionCmdLocks map
	sessionCmdLocks map[string]*sync.Mutex    // Per-session command locks (prevents concurrent commands on same session)
	proxyMgr        *proxy.ProxyManager       // Proxy manager for HTTP clients
	ctx             context.Context           // Context for cancellation
	cancel          context.CancelFunc        // Cancel function for graceful shutdown
}

// BotChannel represents a bot channel for sending responses
type BotChannel struct {
	Platform  string // "discord", "telegram", "feishu", etc.
	Channel   string // Channel ID (platform-specific)
	MessageID string // Message ID (for typing indicator removal)
}

// NewEngine creates a new Engine instance
func NewEngine(config *Config) *Engine {
	ctx, cancel := context.WithCancel(context.Background())

	// Set default for max dynamic sessions if not configured
	if config.Session.MaxDynamicSessions == 0 {
		config.Session.MaxDynamicSessions = 50
	}

	engine := &Engine{
		config:          config,
		cliAdapters:     make(map[string]cli.CLIAdapter),
		activeBots:      make(map[string]bot.BotAdapter),
		sessions:        make(map[string]*Session),
		messageChan:     make(chan bot.BotMessage, constants.MessageChannelBufferSize),
		sessionChannels: make(map[string]BotChannel),
		userSessions:    make(map[string]string),
		sessionCmdLocks: make(map[string]*sync.Mutex),
		proxyMgr:        proxy.NewProxyManager(NewCoreConfigAdapter(config)),
		ctx:             ctx,
		cancel:          cancel,
	}
	return engine
}

// RegisterCLIAdapter registers a CLI adapter
func (e *Engine) RegisterCLIAdapter(cliType string, adapter cli.CLIAdapter) {
	e.cliAdapters[cliType] = adapter
}

// RegisterBotAdapter registers a bot adapter
func (e *Engine) RegisterBotAdapter(botType string, adapter bot.BotAdapter) {
	e.activeBots[botType] = adapter
}

// GetProxyManager returns the proxy manager
func (e *Engine) GetProxyManager() *proxy.ProxyManager {
	return e.proxyMgr
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
			if _, err := e.ensureSessionStarted(session, sessionConfig); err != nil {
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

// ensureSessionStarted ensures a session is running, starting it if necessary
// Returns true if the session was already running, false if it was started
func (e *Engine) ensureSessionStarted(session *Session, sessionConfig SessionConfig) (bool, error) {
	adapter, exists := e.cliAdapters[session.CLIType]
	if !exists {
		return false, fmt.Errorf("CLI adapter '%s' not found", session.CLIType)
	}

	// Check if already running
	if adapter.IsSessionAlive(session.Name) {
		return true, nil
	}

	// Determine start command
	startCmd := sessionConfig.StartCmd
	if startCmd == "" {
		startCmd = session.CLIType
	}

	// Start the session
	if err := adapter.CreateSession(session.Name, session.WorkDir, startCmd, sessionConfig.Transport); err != nil {
		return false, fmt.Errorf("failed to create session: %w", err)
	}

	// Update session state
	session.State = StateProcessing
	return false, nil
}

// needsHookServer checks if any session requires hook server
// Returns false if all sessions are ACP type (which use native protocol)
func (e *Engine) needsHookServer() bool {
	for _, sessionConfig := range e.config.Sessions {
		if sessionConfig.CLIType != "acp" {
			return true
		}
	}
	return false
}

// Run starts the engine and begins processing messages
func (e *Engine) Run(ctx context.Context) error {
	logger.Info("starting-clibot-engine")

	// Initialize sessions
	if err := e.initializeSessions(); err != nil {
		return fmt.Errorf("failed to initialize sessions: %w", err)
	}

	// Start HTTP hook server only if needed
	if e.needsHookServer() {
		go e.startHookServer()
	} else {
		logger.Info("all-sessions-are-acp-type-hook-server-not-required")
	}

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
	// Fast-track: special commands are processed immediately without queueing
	// This allows commands like slist, sstatus, whoami to respond instantly
	input := strings.TrimSpace(msg.Content)
	cmd, isSpecialCmd, args := isSpecialCommand(input)

	if isSpecialCmd {
		// Special commands are processed asynchronously for immediate response
		logger.WithFields(logrus.Fields{
			"command": cmd,
			"args":    args,
			"user":    msg.UserID,
		}).Info("special-command-fast-track")

		go e.handleSpecialCommandWithAuth(cmd, args, msg)
		return
	}

	// Regular AI requests enter the message queue for serial processing
	e.messageChan <- msg
}

// handleSpecialCommandWithAuth handles special commands with authorization check
// getSessionCmdLock gets or creates a mutex for the specified session
// This ensures commands for the same session are executed serially
func (e *Engine) getSessionCmdLock(sessionName string) *sync.Mutex {
	e.cmdLocksMu.Lock()
	defer e.cmdLocksMu.Unlock()

	if e.sessionCmdLocks[sessionName] == nil {
		e.sessionCmdLocks[sessionName] = &sync.Mutex{}
	}
	return e.sessionCmdLocks[sessionName]
}

// requiresSessionLock checks if a command requires per-session locking
// Commands that modify or query a specific session state require locking
// Global commands like slist, whoami, help do not require locking
func requiresSessionLock(command string) bool {
	switch command {
	case "suse", "sclose", "sdel", "sstatus":
		return true
	default:
		return false
	}
}

func (e *Engine) handleSpecialCommandWithAuth(command string, args []string, msg bot.BotMessage) {
	// help and echo bypass whitelist to allow users to get help and their user_id
	if command == "help" || command == "echo" {
		e.HandleSpecialCommandWithArgs(command, args, msg)
		return
	}

	// Other commands require whitelist authorization
	if !e.config.IsUserAuthorized(msg.Platform, msg.UserID) {
		logger.WithFields(logrus.Fields{
			"platform": msg.Platform,
			"user":     msg.UserID,
		}).Warn("unauthorized-special-command")
		e.SendToBot(msg.Platform, msg.Channel, "❌ Unauthorized: Please contact administrator")
		return
	}

	// Authorization passed, execute the command
	// For session-specific commands, use TryLock to prevent concurrent execution
	if requiresSessionLock(command) && len(args) > 0 {
		sessionName := args[0]

		lock := e.getSessionCmdLock(sessionName)
		if !lock.TryLock() {
			// Lock is held by another command for this session
			logger.WithFields(logrus.Fields{
				"session": sessionName,
				"command": command,
				"user":    msg.UserID,
			}).Warn("session-command-lock-held")

			e.SendToBot(msg.Platform, msg.Channel,
				"⚠️  This session is currently processing another command. Please try again later.")
			return
		}
		defer lock.Unlock()
	}

	e.HandleSpecialCommandWithArgs(command, args, msg)
}

// HandleUserMessage processes a message from a user
func (e *Engine) HandleUserMessage(msg bot.BotMessage) {
	logger.WithFields(logrus.Fields{
		"platform": msg.Platform,
		"user":     msg.UserID,
		"channel":  msg.Channel,
	}).Info("processing-user-message")

	// Step 0: Check if it's a special command (no prefix required)
	// Only "help" and "echo" bypass whitelist to allow users to get their user_id
	input := strings.TrimSpace(msg.Content)
	cmd, isSpecialCmd, args := isSpecialCommand(input)

	if isSpecialCmd {
		// Only "help" and "echo" bypass whitelist check
		if cmd == "help" || cmd == "echo" {
			logger.WithFields(logrus.Fields{
				"command": cmd,
				"args":    args,
				"user":    msg.UserID,
			}).Info("special-command-received")
			e.HandleSpecialCommandWithArgs(cmd, args, msg)
			return
		}
	}

	// Step 1: Security check - verify user is in whitelist
	// Applies to all commands except "help" and "echo", and all AI queries
	if !e.config.IsUserAuthorized(msg.Platform, msg.UserID) {
		logger.WithFields(logrus.Fields{
			"platform": msg.Platform,
			"user":     msg.UserID,
		}).Warn("unauthorized-access-attempt")
		e.SendToBot(msg.Platform, msg.Channel, "❌ Unauthorized: Please contact administrator to add your user ID")
		return
	}

	logger.WithField("user", msg.UserID).Debug("user-authorized")

	// Step 2: Handle remaining special commands (status, slist, etc.)
	if isSpecialCmd {
		logger.WithFields(logrus.Fields{
			"command": cmd,
			"args":    args,
			"user":    msg.UserID,
		}).Info("special-command-received")
		e.HandleSpecialCommandWithArgs(cmd, args, msg)
		return
	}

	// Step 3: Get active session for this user

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
		Platform:  msg.Platform,
		Channel:   msg.Channel,
		MessageID: msg.MessageID, // Save message ID for typing indicator removal
	}
	e.sessionMu.Unlock()

	// Step 3.5: Add typing indicator reaction IMMEDIATELY for supported platforms
	// This should be done ASAP to give user immediate visual feedback
	if msg.MessageID != "" {
		e.sessionMu.RLock()
		botAdapter, exists := e.activeBots[msg.Platform]
		e.sessionMu.RUnlock()

		if exists && botAdapter.SupportsTypingIndicator() {
			// Add typing indicator immediately (in goroutine to avoid blocking)
			// Capture local variables to avoid closure issues
			messageID := msg.MessageID
			platform := msg.Platform
			adapter := botAdapter

			go func() {
				defer func() {
					if r := recover(); r != nil {
						logger.WithFields(logrus.Fields{
							"platform":   platform,
							"message_id": messageID,
							"panic":      r,
						}).Error("panic-in-add-typing-indicator")
					}
				}()

				if adapter.AddTypingIndicator(messageID) {
					logger.WithFields(logrus.Fields{
						"platform":   platform,
						"message_id": messageID,
					}).Info("typing-indicator-added")
				}
			}()
		}
	}

	// Step 4: Process key words (tab, esc, stab, enter, ctrlc, etc.)
	// Converts entire input matching keywords to actual key sequences
	processedContent := watchdog.ProcessKeyWords(msg.Content)
	if processedContent != msg.Content {
		logger.WithFields(logrus.Fields{
			"original":  msg.Content,
			"processed": fmt.Sprintf("%q", processedContent),
		}).Debug("keyword-converted-to-key-sequence")
	}

	// NOTE: Before snapshot capture removed - only hook mode is supported
	adapter := e.cliAdapters[session.CLIType]

	// Step 5: Send to CLI
	if err := adapter.SendInput(session.Name, processedContent); err != nil {
		logger.WithFields(logrus.Fields{
			"session": session.Name,
			"error":   err,
		}).Error("failed-to-send-input-to-cli")
		e.SendToBot(msg.Platform, msg.Channel, fmt.Sprintf("❌ Failed to send input: %v", err))
		return
	}

	// Step 6: Update session state to processing
	e.updateSessionState(session.Name, StateProcessing)

	// Hook mode: No polling needed, responses will be received via hooks
	// Start watchdog for session monitoring
	if session.NeedsWatchdog() {
		ctx, cleanup := e.startNewWatchdogForSession(session.Name)
		go func(sessionName string, watchdogCtx context.Context) {
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

			if err := e.startWatchdogWithContext(watchdogCtx, session, "", ""); err != nil {
				logger.WithFields(logrus.Fields{
					"session": sessionName,
					"error":   err,
				}).Error("watchdog-failed")
			}
		}(session.Name, ctx)
	}
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
	case "帮助":
		e.showHelpChinese(msg)
	case "slist":
		e.listSessions(msg)
	case "suse":
		e.handleUseSession(args, msg)
	case "status":
		e.showStatus(msg)
	case "whoami":
		e.showWhoami(msg)
	case "echo":
		e.handleEcho(msg)
	case "snew":
		e.handleNewSession(args, msg)
	case "sdel":
		e.handleDeleteSession(args, msg)
	case "sclose":
		e.handleCloseSession(args, msg)
	case "sstatus":
		e.handleSessionStatus(args, msg)
	case "ssnew":
		e.handleNewGeminiSession(args, msg)
	case "scd":
		e.handleSwitchWorkDir(args, msg)
	case "ssls":
		e.handleListGeminiSessions(args, msg)
	case "sssw":
		e.handleSwitchGeminiSession(args, msg)
	case "snewg":
		e.handleNewGeminiACPSession(args, msg)
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

	// Get bot username for link formatting
	botUsername := ""
	if botAdapter, exists := e.activeBots[msg.Platform]; exists {
		botUsername = botAdapter.GetBotUsername()
	}

	formatSessionName := func(name string) string {
		escaped := url.QueryEscape(name)
		if botUsername != "" {
			return fmt.Sprintf("[**%s**](tg://resolve?domain=%s&text=suse%%20%s)", name, botUsername, escaped)
		}
		return fmt.Sprintf("[**%s**](tg://msg?text=suse%%20%s)", name, escaped)
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
				formatSessionName(session.Name), session.CLIType, session.State, marker)
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
				formatSessionName(session.Name), session.CLIType, session.State, session.CreatedBy, marker)
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

	// Get bot username for link formatting
	botUsername := ""
	if botAdapter, exists := e.activeBots[msg.Platform]; exists {
		botUsername = botAdapter.GetBotUsername()
	}

	formatSessionName := func(name string) string {
		escaped := url.QueryEscape(name)
		if botUsername != "" {
			return fmt.Sprintf("[**%s**](tg://resolve?domain=%s&text=suse%%20%s)", name, botUsername, escaped)
		}
		return fmt.Sprintf("[**%s**](tg://msg?text=suse%%20%s)", name, escaped)
	}

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

		response += fmt.Sprintf("  %s %s (%s) - %s %s\n", status, formatSessionName(session.Name), session.CLIType, session.State, origin)
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
// showHelp displays help information about available commands and keywords
func (e *Engine) showHelp(msg bot.BotMessage) {
	help := `📖 **clibot Help**

**Special Commands** (clickable):
# (Tap a command to copy/pre-fill)
` + e.fmtCmd(msg, "help") + `          - Show this help message
` + e.fmtCmd(msg, "slist") + `         - List all available sessions
` + e.fmtCmd(msg, "suse <name>") + `   - Switch current session
` + e.fmtCmd(msg, "sclose [name]") + ` - Close session (default: current)
` + e.fmtCmd(msg, "sstatus [name]") + ` - Show detailed session status
` + e.fmtCmd(msg, "status") + `        - Show status of all sessions
` + e.fmtCmd(msg, "whoami") + `        - Show your current session info
` + e.fmtCmd(msg, "echo") + `          - Echo your IM user info
` + e.fmtCmd(msg, "snew <name> <type> <dir> [cmd]") + ` - New session
` + e.fmtCmd(msg, "snewg <name> <work_dir>") + ` - New Gemini ACP session
` + e.fmtCmd(msg, "sdel <name>") + `   - Delete dynamic session
` + e.fmtCmd(msg, "ssnew") + `         - Start a NEW Gemini conversation
` + e.fmtCmd(msg, "scd <path>") + `    - Change working directory
` + e.fmtCmd(msg, "ssls") + `          - List native Gemini session IDs
` + e.fmtCmd(msg, "sssw <id>") + `     - Switch to a specific Gemini ID

**Special Keywords** (exact match, case-insensitive):
  ⚠️ These keywords only work in Hook mode with tmux input
  tab            - Send Tab key
  esc            - Send Escape key
  stab/s-tab     - Send Shift+Tab
  enter          - Send Enter key
  ctrlc/ctrl-c    - Send Ctrl+C (interrupt)
  ctrlt/ctrl-t    - Send Ctrl+T

**Usage Examples:**
  status            → Show status
  tab               → Send Tab key to CLI
  ctrl-c            → Interrupt current process
  ctrl-t            → Trigger Ctrl+T action
  snew myproject claude ~/work  → Create new session

**Tips:**
  - Special commands are exact match (case-sensitive)
  - Special keywords are case-insensitive
  - Any other input will be sent to the CLI
  - Use "suse" to switch between sessions
  - Use "sclose" to free up resources when not using a session
  - Use "sstatus" to monitor session health and resource usage
  - Use "help" anytime to see this message`

	e.SendToBot(msg.Platform, msg.Channel, help)
}

// showHelpChinese displays Chinese help information
func (e *Engine) showHelpChinese(msg bot.BotMessage) {
	help := `📖 **clibot 帮助手册**

**1. 机器人分身管理 (点击指令可复制):**

` + e.fmtCmd(msg, "slist") + `        - 查看所有已配置的机器人
` + e.fmtCmd(msg, "suse <名>") + `    - 切换到指定的机器人
` + e.fmtCmd(msg, "sstatus [名]") + ` - 查看 PID, 内存, 运行时间
` + e.fmtCmd(msg, "status") + `       - 查看所有机器人的简要状态
` + e.fmtCmd(msg, "whoami") + `       - 查看当前会话详情
` + e.fmtCmd(msg, "snew <名> <类型> <目录> [命令]") + ` - (管理员)
` + e.fmtCmd(msg, "snewg <名> <目录>") + ` - 快速创建 Gemini ACP (管理员)
` + e.fmtCmd(msg, "sdel <名>") + `    - 彻底删除会话 (管理员)
` + e.fmtCmd(msg, "sclose [名]") + `  - 暂时关闭后台进程以节省资源


**2. AI 记忆与存档管理 (Gemini 专用):**

` + e.fmtCmd(msg, "ssnew") + `        - 【重要】开启全新对话 (保留旧存档)
` + e.fmtCmd(msg, "scd <路径>") + `    - 更改 AI 关注的目录 (记忆环境切换)
` + e.fmtCmd(msg, "ssls") + `         - 列出当前项目的历史存档 ID
` + e.fmtCmd(msg, "sssw <ID>") + `    - 读档 (切换到特定的历史对话)


**3. 其他指令:**
- ` + "`帮助`" + ` / ` + "`help`" + ` - 显示此信息
- ` + "`echo`" + ` - 回显账号 ID (用于白名单配置)

**特殊关键词 (直接发送):**
` + "```" + `text
tab, enter, ctrlc, esc
` + "```" + `

**💡 提示:**
- 绝大多数情况下，你只需要用 ` + "`suse`" + ` 切换机器人。
- 聊太久导致 AI 变傻时，请务必使用 ` + "`ssnew`" + ` 刷新它。
- 任何非指令消息都会被直接发送给底层的 AI 工具。`

	e.SendToBot(msg.Platform, msg.Channel, help)
}

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

	// 4.5. Check if hook server is available for non-ACP sessions
	if cliType != "acp" && e.hookServer == nil {
		e.SendToBot(msg.Platform, msg.Channel,
			fmt.Sprintf("❌ Cannot create '%s' session: HTTP hook server is not running\n\n"+
				"Reason: All configured sessions are ACP type, so the HTTP hook server was not started.\n"+
				"Non-ACP sessions (like '%s') require the hook server to receive CLI responses.\n\n"+
				"Solutions:\n"+
				"  1. Add at least one non-ACP session to your config file and restart\n"+
				"  2. Or use 'acp' CLI type for this session", cliType, cliType))
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
	// For dynamic sessions, transport is typically empty (non-ACP adapters)
	if err := adapter.CreateSession(name, expandedDir, startCmd, ""); err != nil {
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

	// 3. Ensure session is running (start if necessary)
	// Get session config
	var sessionConfig SessionConfig
	for _, cfg := range e.config.Sessions {
		if cfg.Name == sessionName {
			sessionConfig = cfg
			break
		}
	}

	sessionWasRunning, err := e.ensureSessionStarted(session, sessionConfig)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"session": sessionName,
			"error":   err,
		}).Error("failed-to-ensure-session-started")
		e.SendToBot(msg.Platform, msg.Channel,
			fmt.Sprintf("❌ Failed to start session '%s': %v", sessionName, err))
		return
	}

	// 4. Update user's current session
	wasSwitched := e.userSessions[userKey] != sessionName
	e.userSessions[userKey] = sessionName

	logger.WithFields(logrus.Fields{
		"user":    userKey,
		"session": sessionName,
		"cli":     session.CLIType,
	}).Info("user-switched-session")

	// 5. Success response
	response := fmt.Sprintf("✅ Your current session is now: **%s**\n\n", sessionName)

	if !sessionWasRunning {
		response += "🚀 Session was not running, started automatically\n\n"
	}

	response += "📊 Session Info:\n"
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

	// 5. Stop the session (close + release resources)
	if err := e.stopSession(session); err != nil {
		logger.WithFields(logrus.Fields{
			"session": name,
			"error":   err,
		}).Warn("failed-to-stop-session-before-deletion")
		// Continue with deletion even if stop failed
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

// handleCloseSession handles the sclose command
// Stops the CLI process for a session without deleting the session configuration
func (e *Engine) handleCloseSession(args []string, msg bot.BotMessage) {
	logger.WithFields(logrus.Fields{
		"platform": msg.Platform,
		"user_id":  msg.UserID,
		"args":     args,
	}).Info("handle-close-session-command")

	e.sessionMu.Lock()
	defer e.sessionMu.Unlock()

	var sessionName string
	var session *Session

	// Determine target session
	if len(args) == 0 {
		// No argument: close current user's current session
		userKey := getUserKey(msg.Platform, msg.UserID)
		var exists bool
		sessionName, exists = e.userSessions[userKey]
		if !exists {
			e.SendToBot(msg.Platform, msg.Channel,
				"❌ You don't have an active session\nUsage: sclose <name>")
			return
		}
		session, exists = e.sessions[sessionName]
		if !exists {
			e.SendToBot(msg.Platform, msg.Channel,
				fmt.Sprintf("❌ Session '%s' not found", sessionName))
			return
		}
	} else {
		// With argument: close specified session
		sessionName = args[0]
		var exists bool
		session, exists = e.sessions[sessionName]
		if !exists {
			e.SendToBot(msg.Platform, msg.Channel,
				fmt.Sprintf("❌ Session '%s' not found", sessionName))
			return
		}

		// Permission check: admin or session creator
		isAdmin := e.config.IsAdmin(msg.Platform, msg.UserID)
		isCreator := session.CreatedBy == getUserKey(msg.Platform, msg.UserID)

		if !isAdmin && !isCreator {
			e.SendToBot(msg.Platform, msg.Channel,
				"❌ Permission denied: admin or session creator only")
			return
		}
	}

	// Check if session is alive
	if session.State != StateProcessing && session.State != StateIdle {
		e.SendToBot(msg.Platform, msg.Channel,
			fmt.Sprintf("⚠️  Session '%s' is not running (state: %s)", sessionName, session.State))
		return
	}

	// Stop the session using shared stopSession method
	if err := e.stopSession(session); err != nil {
		logger.WithFields(logrus.Fields{
			"session":  sessionName,
			"error":    err,
			"platform": msg.Platform,
			"user_id":  msg.UserID,
		}).Error("failed-to-stop-session")
		e.SendToBot(msg.Platform, msg.Channel,
			fmt.Sprintf("❌ Failed to stop session '%s': %v", sessionName, err))
		return
	}

	logger.WithFields(logrus.Fields{
		"action":   "close_session",
		"session":  sessionName,
		"platform": msg.Platform,
		"user_id":  msg.UserID,
	}).Info("session-closed-successfully")

	e.SendToBot(msg.Platform, msg.Channel,
		fmt.Sprintf("✅ Session '%s' closed successfully", sessionName))
}

// stopSession stops a running session and releases resources
// This is a helper method used by both sclose and sdel commands
func (e *Engine) stopSession(session *Session) error {
	logger.WithFields(logrus.Fields{
		"session": session.Name,
		"cliType": session.CLIType,
	}).Info("stopping-session")

	// Get CLI adapter
	adapter, exists := e.cliAdapters[session.CLIType]
	if !exists {
		return fmt.Errorf("CLI adapter '%s' not found", session.CLIType)
	}

	// Different cleanup strategies based on CLI type
	var err error
	if session.CLIType == "acp" {
		// ACP adapter: call DeleteSession to cleanup connection and process
		if acpAdapter, ok := adapter.(*cli.ACPAdapter); ok {
			err = acpAdapter.DeleteSession(session.Name)
		} else {
			err = fmt.Errorf("adapter is not ACPAdapter")
		}
	} else {
		// Tmux-based adapters: kill tmux session
		cmd := exec.Command("tmux", "kill-session", "-t", session.Name)
		err = cmd.Run()
	}

	if err != nil {
		return err
	}

	// Update session state
	session.State = StateIdle

	// Cancel any running watchdog goroutine
	if session.cancelCtx != nil {
		session.cancelCtx()
		session.cancelCtx = nil
	}

	return nil
}

// SessionStatus represents detailed status information about a session
type SessionStatus struct {
	Name         string
	State        SessionState
	CLIType      string
	WorkDir      string
	IsDynamic    bool
	CreatedBy    string
	IsAlive      bool
	ProcessInfo  *ProcessInfo
	LastActivity string
}

// ProcessInfo contains process-related information
type ProcessInfo struct {
	PID     int
	Memory  string // Human-readable memory usage
	Uptime  string // Human-readable uptime
	Command string // Process command
}

// handleSessionStatus handles the sstatus command
// Usage: sstatus [session_name]
func (e *Engine) handleSessionStatus(args []string, msg bot.BotMessage) {
	logger.WithFields(logrus.Fields{
		"platform": msg.Platform,
		"user_id":  msg.UserID,
		"args":     args,
	}).Info("handle-session-status-command")

	e.sessionMu.RLock()
	defer e.sessionMu.RUnlock()

	// No argument: show all sessions
	if len(args) == 0 {
		e.showAllSessionsStatus(msg)
		return
	}

	// With argument: show specific session
	sessionName := args[0]
	session, exists := e.sessions[sessionName]
	if !exists {
		e.SendToBot(msg.Platform, msg.Channel,
			fmt.Sprintf("❌ Session '%s' does not exist\nUse 'slist' to see available sessions", sessionName))
		return
	}

	status := e.getSessionStatus(session)
	e.sendSessionStatus(msg, status)
}

// handleNewGeminiSession starts a new conversation session within the current clibot session
func (e *Engine) handleNewGeminiSession(args []string, msg bot.BotMessage) {
	userKey := getUserKey(msg.Platform, msg.UserID)
	e.sessionMu.RLock()
	sessionName, hasSession := e.userSessions[userKey]
	var session *Session
	if hasSession {
		session = e.sessions[sessionName]
	}
	e.sessionMu.RUnlock()

	if session == nil {
		e.SendToBot(msg.Platform, msg.Channel, "❌ No active session. Select one with 'suse <name>'.")
		return
	}

	adapter, exists := e.cliAdapters[session.CLIType]
	if !exists {
		e.SendToBot(msg.Platform, msg.Channel, fmt.Sprintf("❌ CLI adapter '%s' not found", session.CLIType))
		return
	}

	if err := adapter.ResetSession(session.Name); err != nil {
		e.SendToBot(msg.Platform, msg.Channel, fmt.Sprintf("❌ Failed to start new Gemini session: %v", err))
		return
	}

	e.SendToBot(msg.Platform, msg.Channel, fmt.Sprintf("✅ Started a NEW Gemini session for: **%s**", session.Name))
}

// handleSwitchWorkDir switches the working directory of the current session
func (e *Engine) handleSwitchWorkDir(args []string, msg bot.BotMessage) {
	if len(args) < 1 {
		e.SendToBot(msg.Platform, msg.Channel, "❌ Usage: scd <path>")
		return
	}

	newPath := args[0]
	expandedPath, err := expandPath(newPath)
	if err != nil {
		e.SendToBot(msg.Platform, msg.Channel, fmt.Sprintf("❌ Invalid path: %v", err))
		return
	}

	userKey := getUserKey(msg.Platform, msg.UserID)
	e.sessionMu.RLock()
	sessionName, hasSession := e.userSessions[userKey]
	var session *Session
	if hasSession {
		session = e.sessions[sessionName]
	}
	e.sessionMu.RUnlock()

	if session == nil {
		e.SendToBot(msg.Platform, msg.Channel, "❌ No active session. Select one with 'suse <name>'.")
		return
	}

	adapter, exists := e.cliAdapters[session.CLIType]
	if !exists {
		e.SendToBot(msg.Platform, msg.Channel, fmt.Sprintf("❌ CLI adapter '%s' not found", session.CLIType))
		return
	}

	// Update session work dir in engine
	e.sessionMu.Lock()
	session.WorkDir = expandedPath
	e.sessionMu.Unlock()

	// Tell adapter to switch (this might restart the process)
	if err := adapter.SwitchWorkDir(session.Name, expandedPath); err != nil {
		e.SendToBot(msg.Platform, msg.Channel, fmt.Sprintf("❌ Failed to switch directory: %v", err))
		return
	}

	e.SendToBot(msg.Platform, msg.Channel, fmt.Sprintf("✅ Switched session '%s' to: %s", session.Name, expandedPath))
}

// handleListGeminiSessions natively lists all available Gemini session files for the current project
func (e *Engine) handleListGeminiSessions(args []string, msg bot.BotMessage) {
	userKey := getUserKey(msg.Platform, msg.UserID)
	e.sessionMu.RLock()
	sessionName, hasSession := e.userSessions[userKey]
	var session *Session
	if hasSession {
		session = e.sessions[sessionName]
	}
	e.sessionMu.RUnlock()

	if session == nil {
		e.SendToBot(msg.Platform, msg.Channel, "❌ No active session selected.")
		return
	}

	if session.CLIType != "gemini" && session.CLIType != "acp" {
		e.SendToBot(msg.Platform, msg.Channel, "❌ This command is only for Gemini or ACP sessions.")
		return
	}

	adapter, exists := e.cliAdapters[session.CLIType]
	if !exists {
		e.SendToBot(msg.Platform, msg.Channel, fmt.Sprintf("❌ CLI adapter '%s' not found.", session.CLIType))
		return
	}

	// Use adapter's ListSessions to get a machine-readable list of sessions.
	// Retrieve bot username for platform-specific linking (e.g., Telegram tg://resolve)
	var botUsername string
	if botAdapter, ok := e.activeBots[msg.Platform]; ok {
		botUsername = botAdapter.GetBotUsername()
	}

	sessions, err := adapter.ListSessions(session.Name, botUsername)
	if err != nil {
		e.SendToBot(msg.Platform, msg.Channel, fmt.Sprintf("❌ Failed to list sessions: %v", err))
		return
	}

	if len(sessions) == 0 {
		e.SendToBot(msg.Platform, msg.Channel, "ℹ️ No previous Gemini sessions found for this project.")
		return
	}

	// Format sessions into Telegram-friendly chunks (e.g., 15 sessions per message)
	const chunkSize = 15
	for i := 0; i < len(sessions); i += chunkSize {
		end := i + chunkSize
		if end > len(sessions) {
			end = len(sessions)
		}

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("📂 *Available Gemini Sessions* (%d/%d)\n\n", (i/chunkSize)+1, (len(sessions)+chunkSize-1)/chunkSize))
		for _, s := range sessions[i:end] {
			sb.WriteString(fmt.Sprintf("%s\n", s))
		}
		if end == len(sessions) {
			sb.WriteString("\n💡 Use `sssw <id>` to switch to a session.")
		}
		e.SendToBot(msg.Platform, msg.Channel, sb.String())
	}
}

// handleNewGeminiACPSession creates a new Gemini session in ACP mode with a specified WorkDir
// Usage: snewg <name> <work_dir>
func (e *Engine) handleNewGeminiACPSession(args []string, msg bot.BotMessage) {
	logger.WithFields(logrus.Fields{
		"platform": msg.Platform,
		"user_id":  msg.UserID,
		"args":     args,
	}).Info("handle-snewg-command")

	// 1. Permission check
	if !e.config.IsAdmin(msg.Platform, msg.UserID) {
		e.SendToBot(msg.Platform, msg.Channel, "❌ Permission denied: admin only")
		return
	}

	// 2. Parameter validation
	if len(args) < 2 {
		e.SendToBot(msg.Platform, msg.Channel,
			"❌ Invalid arguments\nUsage: sadf <name> <work_dir>")
		return
	}

	name := args[0]
	workDir := args[1]
	cliType := "acp"
	startCmd := "gemini"

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
			"❌ ACP CLI adapter not found. Please ensure it is registered.")
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
	if info, err := os.Stat(expandedDir); err != nil || !info.IsDir() {
		e.SendToBot(msg.Platform, msg.Channel,
			fmt.Sprintf("❌ Work directory does not exist or is not a directory: %s", expandedDir))
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

	// 9. Create ACP session
	if err := adapter.CreateSession(name, expandedDir, startCmd, "stdio://"); err != nil {
		logger.WithField("error", err).Error("failed-to-create-sadf-session")
		e.SendToBot(msg.Platform, msg.Channel,
			fmt.Sprintf("❌ Failed to create session: %v", err))
		return
	}

	// 10. Add to sessions map
	e.sessions[name] = session

	// 11. Automatically select for the current user
	userKey := getUserKey(msg.Platform, msg.UserID)
	e.userSessions[userKey] = name

	logger.WithFields(logrus.Fields{
		"action":     "create_snewg_session",
		"session":    name,
		"platform":   msg.Platform,
		"user_id":    msg.UserID,
		"work_dir":   expandedDir,
		"is_dynamic": true,
	}).Info("admin-created-snewg-session")

	// 12. Success response
	e.SendToBot(msg.Platform, msg.Channel,
		fmt.Sprintf("✅ Gemini ACP session '%s' created and selected\nWorkDir: %s",
			name, expandedDir))
}

// handleSwitchGeminiSession switches the current Gemini process to a different session file natively
func (e *Engine) handleSwitchGeminiSession(args []string, msg bot.BotMessage) {
	if len(args) < 1 {
		e.SendToBot(msg.Platform, msg.Channel, "❌ Usage: sssw <gemini_session_id>")
		return
	}

	id := args[0]
	userKey := getUserKey(msg.Platform, msg.UserID)
	e.sessionMu.RLock()
	sessionName, hasSession := e.userSessions[userKey]
	var session *Session
	if hasSession {
		session = e.sessions[sessionName]
	}
	e.sessionMu.RUnlock()

	if session == nil || (session.CLIType != "gemini" && session.CLIType != "acp") {
		e.SendToBot(msg.Platform, msg.Channel, "❌ No active Gemini or ACP session selected.")
		return
	}

	adapter, exists := e.cliAdapters[session.CLIType]
	if !exists {
		e.SendToBot(msg.Platform, msg.Channel, fmt.Sprintf("❌ CLI adapter '%s' not found.", session.CLIType))
		return
	}

	// Use adapter's SwitchSession to switch natively.
	// This will typically send a /resume <id> command to the CLI.
	contextStr, err := adapter.SwitchSession(session.Name, id)
	if err != nil {
		e.SendToBot(msg.Platform, msg.Channel, fmt.Sprintf("❌ Failed to switch session: %v", err))
		return
	}

	responseMsg := fmt.Sprintf("✅ Switched Gemini session to: **%s**", id)
	if contextStr != "" {
		responseMsg += fmt.Sprintf("\n\n%s", contextStr)
	}

	// Append status bar to the switch confirmation if enabled
	if e.config.Session.ShowSessionStats {
		// Retrieve bot username for platform-specific linking
		var botUsername string
		if botAdapter, ok := e.activeBots[msg.Platform]; ok {
			botUsername = botAdapter.GetBotUsername()
		}

		stats, err := adapter.GetSessionStats(session.Name, botUsername)
		if err == nil && len(stats) > 0 {
			workDir := ""
			if wd, ok := stats["work_dir"].(string); ok {
				workDir = wd
			}
			usagePerc := 0.0
			if up, ok := stats["usage_perc"].(float64); ok {
				usagePerc = up
			}
			sessionTitle := ""
			if st, ok := stats["session_title"].(string); ok {
				sessionTitle = st
			}

			// Markdown Format: 📂 `[dir]` | 💬 ID: Summary | 🧠 `[usage]%` used
			statsBar := fmt.Sprintf("\n\n---\n📂 `%s` | 💬 %s | 🧠 `%.0f%%` used",
				workDir, sessionTitle, usagePerc)
			responseMsg += statsBar
		}
	}

	e.SendToBot(msg.Platform, msg.Channel, responseMsg)
}

// showAllSessionsStatus shows status of all sessions
func (e *Engine) showAllSessionsStatus(msg bot.BotMessage) {
	if len(e.sessions) == 0 {
		e.SendToBot(msg.Platform, msg.Channel, "⚠️  No sessions configured")
		return
	}

	var statuses []*SessionStatus
	for _, session := range e.sessions {
		status := e.getSessionStatus(session)
		statuses = append(statuses, status)
	}

	// Build response
	response := "📊 **All Sessions Status**\n\n"

	for _, status := range statuses {
		// Status icon
		var icon string
		switch status.State {
		case StateProcessing:
			icon = "🟢"
		case StateIdle:
			icon = "⏸️"
		case StateWaitingInput:
			icon = "⏳"
		case StateError:
			icon = "❌"
		default:
			icon = "⚪"
		}

		if !status.IsAlive {
			icon = "⚫"
		}

		response += fmt.Sprintf("%s **%s** - %s\n", icon, status.Name, status.State)
		response += fmt.Sprintf("  CLI: %s", status.CLIType)

		if status.IsAlive && status.ProcessInfo != nil {
			response += fmt.Sprintf(" | PID: %d | Mem: %s", status.ProcessInfo.PID, status.ProcessInfo.Memory)
		}

		response += "\n"
	}

	e.SendToBot(msg.Platform, msg.Channel, response)
}

// getSessionStatus gathers detailed status information for a session
func (e *Engine) getSessionStatus(session *Session) *SessionStatus {
	status := &SessionStatus{
		Name:         session.Name,
		State:        session.State,
		CLIType:      session.CLIType,
		WorkDir:      session.WorkDir,
		IsDynamic:    session.IsDynamic,
		CreatedBy:    session.CreatedBy,
		IsAlive:      false,
		LastActivity: "Unknown",
	}

	// Check if session is alive
	adapter, exists := e.cliAdapters[session.CLIType]
	if exists {
		status.IsAlive = adapter.IsSessionAlive(session.Name)
	}

	// Get process info if alive
	if status.IsAlive {
		if procInfo := e.getProcessInfo(session); procInfo != nil {
			status.ProcessInfo = procInfo
		}
	}

	return status
}

// getProcessInfo retrieves process information for a session
func (e *Engine) getProcessInfo(session *Session) *ProcessInfo {
	// Try to get PID from tmux session
	var pid int
	var cmd string

	// Get tmux session PID
	if session.CLIType != "acp" {
		// For tmux-based sessions, get the pane PID
		out, err := exec.Command("tmux", "list-panes", "-t", session.Name, "-F", "#{pane_pid}").Output()
		if err == nil && len(out) > 0 {
			fmt.Sscanf(string(out), "%d", &pid)
		}

		// Get command name
		cmdOut, err := exec.Command("tmux", "list-panes", "-t", session.Name, "-F", "#{pane_current_command}").Output()
		if err == nil && len(cmdOut) > 0 {
			cmd = strings.TrimSpace(string(cmdOut))
		}
	} else {
		// For ACP sessions, try to find the process by work directory
		// This is more complex and may not always work
		// Skip for now
	}

	if pid == 0 {
		return nil
	}

	// Get memory usage and uptime from /proc
	memory := e.getProcessMemory(pid)
	uptime := e.getProcessUptime(pid)

	return &ProcessInfo{
		PID:     pid,
		Memory:  memory,
		Uptime:  uptime,
		Command: cmd,
	}
}

// getProcessMemory gets human-readable memory usage for a process
func (e *Engine) getProcessMemory(pid int) string {
	// Try Linux /proc first (most common for this project)
	if data, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid)); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "VmRSS:") {
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					kb := strings.TrimSuffix(parts[1], "kB")
					if memKB, err := strconv.Atoi(kb); err == nil {
						if memKB > 1024 {
							return fmt.Sprintf("%.1f MB", float64(memKB)/1024)
						}
						return fmt.Sprintf("%d KB", memKB)
					}
				}
			}
		}
	}

	// Fallback to ps command (works on macOS and Linux)
	// ps -o rss= -p <pid>  -> RSS in KB
	out, err := exec.Command("ps", "-o", "rss=", "-p", fmt.Sprintf("%d", pid)).Output()
	if err != nil {
		return "Unknown"
	}

	rssStr := strings.TrimSpace(string(out))
	if rssKB, err := strconv.Atoi(rssStr); err == nil && rssKB > 0 {
		if rssKB > 1024 {
			return fmt.Sprintf("%.1f MB", float64(rssKB)/1024)
		}
		return fmt.Sprintf("%d KB", rssKB)
	}

	return "Unknown"
}

// getProcessUptime gets human-readable uptime for a process
func (e *Engine) getProcessUptime(pid int) string {
	// Try Linux /proc first (most common for this project)
	if data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid)); err == nil {
		fields := strings.Fields(string(data))
		if len(fields) >= 22 {
			startJiffies, err := strconv.ParseInt(fields[21], 10, 64)
			if err == nil {
				if uptimeData, err := os.ReadFile("/proc/uptime"); err == nil {
					uptimeFields := strings.Fields(string(uptimeData))
					if len(uptimeFields) >= 1 {
						systemUptime, err := strconv.ParseFloat(uptimeFields[0], 64)
						if err == nil {
							clockTicks := float64(100)
							processUptimeSeconds := systemUptime - (float64(startJiffies) / clockTicks)
							return formatUptime(processUptimeSeconds)
						}
					}
				}
			}
		}
	}

	// Fallback to ps command (works on macOS and Linux)
	// ps -o etime= -p <pid>  -> Elapsed time in [[DD-]HH:]MM:SS format
	out, err := exec.Command("ps", "-o", "etime=", "-p", fmt.Sprintf("%d", pid)).Output()
	if err != nil {
		return "Unknown"
	}

	elapsedTime := strings.TrimSpace(string(out))
	// Parse [[DD-]HH:]MM:SS or MM:SS or HH:MM:SS
	if elapsedTime != "" {
		// Remove brackets if present
		elapsedTime = strings.Trim(elapsedTime, "[]")
		// ps etime format: [[DD-]HH:]MM:SS or just HH:MM:SS or MM:SS
		// Simplify: just return as-is since it's already human-readable
		if elapsedTime != "" {
			return elapsedTime
		}
	}

	return "Unknown"
}

// formatUptime converts seconds to human-readable uptime
func formatUptime(seconds float64) string {
	if seconds < 60 {
		return fmt.Sprintf("%.0fs", seconds)
	}

	hours := int(seconds) / 3600
	minutes := int(seconds) % 3600 / 60

	if hours > 0 {
		return fmt.Sprintf("%dh%dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}

// sendSessionStatus sends detailed session status to user
func (e *Engine) sendSessionStatus(msg bot.BotMessage, status *SessionStatus) {
	response := fmt.Sprintf("📊 **Session Status: %s**\n\n", status.Name)

	// State
	var stateIcon string
	switch status.State {
	case StateProcessing:
		stateIcon = "🟢"
	case StateIdle:
		stateIcon = "⏸️"
	case StateWaitingInput:
		stateIcon = "⏳"
	case StateError:
		stateIcon = "❌"
	default:
		stateIcon = "⚪"
	}

	if !status.IsAlive {
		stateIcon = "⚫"
		response += "⚠️  **Status**: Not running\n\n"
	} else {
		response += fmt.Sprintf("**State**: %s %s\n\n", stateIcon, status.State)
	}

	// Basic info
	response += "📋 **Basic Info**\n"
	response += fmt.Sprintf("  • CLI: %s\n", status.CLIType)
	response += fmt.Sprintf("  • WorkDir: %s\n", status.WorkDir)

	if status.IsDynamic {
		response += fmt.Sprintf("  • Type: Dynamic (created by %s)\n", status.CreatedBy)
	} else {
		response += "  • Type: Static (configured)\n"
	}

	// Process info
	if status.IsAlive && status.ProcessInfo != nil {
		response += "\n💻 **Process Info**\n"
		response += fmt.Sprintf("  • PID: %d\n", status.ProcessInfo.PID)
		if status.ProcessInfo.Command != "" {
			response += fmt.Sprintf("  • Command: %s\n", status.ProcessInfo.Command)
		}
		response += fmt.Sprintf("  • Memory: %s\n", status.ProcessInfo.Memory)
		response += fmt.Sprintf("  • Uptime: %s\n", status.ProcessInfo.Uptime)
	}

	e.SendToBot(msg.Platform, msg.Channel, response)
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

// removeTypingIndicatorAsync removes typing indicator after a delay
// This is a shared helper to avoid code duplication
func (e *Engine) removeTypingIndicatorAsync(platform, messageID string) {
	// Get the bot adapter for this platform
	e.sessionMu.RLock()
	botAdapter, exists := e.activeBots[platform]
	e.sessionMu.RUnlock()

	if !exists || !botAdapter.SupportsTypingIndicator() {
		return
	}

	// Remove typing indicator after a short delay in a goroutine
	go func() {
		select {
		case <-e.ctx.Done():
			// Context cancelled, don't remove typing indicator
			return
		case <-time.After(constants.TypingIndicatorRemoveDelay):
			// Remove typing indicator after response is sent
			botAdapter.RemoveTypingIndicator(messageID)
		}
	}()
}

func (e *Engine) SendResponseToSession(sessionName, message string) {
	e.sessionMu.RLock()
	botChannel, exists := e.sessionChannels[sessionName]
	session, sessExists := e.sessions[sessionName]
	e.sessionMu.RUnlock()

	if !exists {
		logger.WithField("session", sessionName).Warn("no-bot-channel-found-for-session")
		return
	}

	// Skip empty messages
	if strings.TrimSpace(message) == "" {
		logger.WithFields(logrus.Fields{
			"session": sessionName,
			"event":   "skip_empty_response",
		}).Info("skipping-empty-response-delivery")
		return
	}

	finalMessage := message

	// Append Session Stats if enabled
	if sessExists && e.config.Session.ShowSessionStats {
		adapter, ok := e.cliAdapters[session.CLIType]
		if ok {
			// Retrieve bot username for platform-specific linking
			var botUsername string
			if botAdapter, ok := e.activeBots[botChannel.Platform]; ok {
				botUsername = botAdapter.GetBotUsername()
			}

			stats, err := adapter.GetSessionStats(sessionName, botUsername)
			if err == nil && len(stats) > 0 {
				workDir := ""
				if wd, ok := stats["work_dir"].(string); ok {
					workDir = wd
				}
				usagePerc := 0.0
				if up, ok := stats["usage_perc"].(float64); ok {
					usagePerc = up
				}
				sessionTitle := ""
				if st, ok := stats["session_title"].(string); ok {
					sessionTitle = st
				}

				// Markdown Format: 📂 `[dir]` | 💬 [id](...): [summary](...) | 🧠 `[usage]%` used
				statsBar := fmt.Sprintf("\n\n---\n📂 `%s` | 💬 %s | 🧠 `%.0f%%` used",
					workDir, sessionTitle, usagePerc)
				finalMessage += statsBar
			}
		}
	}

	logger.WithFields(logrus.Fields{
		"session":         sessionName,
		"platform":        botChannel.Platform,
		"channel":         botChannel.Channel,
		"response_length": len(finalMessage),
	}).Info("sending-response-to-user")

	// Send the message
	e.SendToBot(botChannel.Platform, botChannel.Channel, finalMessage)

	// Remove typing indicator after a short delay if supported
	if botChannel.MessageID != "" {
		e.removeTypingIndicatorAsync(botChannel.Platform, botChannel.MessageID)
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

// startWatchdogWithContext starts monitoring with a cancellable context
// This prevents goroutine leaks when multiple messages are sent rapidly
// Only hook mode is supported now
func (e *Engine) startWatchdogWithContext(ctx context.Context, session *Session, userPrompt string, beforeCapture string) error {
	// Check if session needs watchdog monitoring
	if !session.NeedsWatchdog() {
		logger.WithFields(logrus.Fields{
			"session": session.Name,
			"cliType": session.CLIType,
		}).Debug("skipping-watchdog-for-async-adapter")
		return nil
	}

	logger.WithField("session", session.Name).Debug("hook-mode-watchdog-started")

	// Hook mode is event-driven
	// The engine waits for hook notifications via HTTP
	// Actual hook handling is done in handleHookRequest
	logger.WithField("session", session.Name).Debug("hook-mode-watchdog-waiting")

	// In hook mode, we just wait for the hook to trigger
	// The actual processing happens when the hook is received
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

	// Stop all active sessions first to prevent orphaned processes
	// This handles graceful shutdown; on Linux, Pdeathsig handles crash scenarios
	e.sessionMu.Lock()
	sessionNames := make([]string, 0, len(e.sessions))
	for name := range e.sessions {
		sessionNames = append(sessionNames, name)
	}
	e.sessionMu.Unlock()

	for _, sessionName := range sessionNames {
		e.sessionMu.RLock()
		session := e.sessions[sessionName]
		e.sessionMu.RUnlock()

		if session != nil {
			logger.WithField("session", sessionName).Info("stopping-session-on-engine-shutdown")
			if err := e.stopSession(session); err != nil {
				logger.WithFields(logrus.Fields{
					"session": sessionName,
					"error":   err,
				}).Error("failed-to-stop-session-during-shutdown")
			}
		}
	}

	// Close CLI adapters
	for cliType, adapter := range e.cliAdapters {
		if closer, ok := adapter.(interface{ Close() error }); ok {
			logger.WithField("adapter", cliType).Info("closing-cli-adapter")
			if err := closer.Close(); err != nil {
				logger.WithFields(logrus.Fields{
					"adapter": cliType,
					"error":   err,
				}).Error("failed-to-close-cli-adapter")
			}
		}
	}

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

// fmtCmd formats a command for the specific platform to allow pre-filling/linking
func (e *Engine) fmtCmd(msg bot.BotMessage, cmd string) string {
	// For Telegram, use tg://msg?text= link for direct pre-fill (no /)
	if msg.Platform == "telegram" {
		// Just the command name part for the link if it has args, but usually clibot commands or session names
		parts := strings.Split(cmd, " ")
		baseCmd := parts[0]
		return fmt.Sprintf("<a href=\"tg://msg?text=%s\">%s</a>", baseCmd, cmd)
	}
	// Default to monospace/code tag for other platforms
	return cmd
}
