package core

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
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
)

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

	// Step 1: Check if it's a special command
	prefix := e.config.CommandPrefix
	if len(msg.Content) > len(prefix) && msg.Content[:len(prefix)] == prefix {
		cmd := msg.Content[len(prefix):]
		logger.WithFields(logrus.Fields{
			"command": cmd,
			"user":    msg.UserID,
		}).Info("special-command-received")
		e.HandleSpecialCommand(cmd, msg)
		return
	}

	// Step 2: Get the active session for this channel
	session := e.GetActiveSession(msg.Channel)
	if session == nil {
		logger.WithFields(logrus.Fields{
			"channel": msg.Channel,
		}).Warn("no-active-session-found-for-channel")
		e.SendToBot(msg.Platform, msg.Channel,
			fmt.Sprintf("❌ No active session. Use '%ssessions' to list available sessions", prefix))
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
	if session.State == StateWaitingInput {
		adapter := e.cliAdapters[session.CLIType]
		if err := adapter.SendInput(session.Name, msg.Content); err != nil {
			e.SendToBot(msg.Platform, msg.Channel, fmt.Sprintf("❌ Failed to send input: %v", err))
			return
		}

		// Update session state
		e.updateSessionState(session.Name, StateProcessing)

		// Resume watchdog monitoring
		go func(s *Session) {
			defer func() {
				if r := recover(); r != nil {
					logger.WithFields(logrus.Fields{
						"session": s.Name,
						"panic":   r,
					}).Error("watchdog-panic-recovered")
				}
			}()
			if err := e.startWatchdog(s); err != nil {
				logger.WithFields(logrus.Fields{
					"session": s.Name,
					"error":   err,
				}).Error("watchdog-failed")
			}
		}(session)

		return
	}

	// Step 4: Normal flow - send to CLI
	adapter := e.cliAdapters[session.CLIType]
	if err := adapter.SendInput(session.Name, msg.Content); err != nil {
		logger.WithFields(logrus.Fields{
			"session": session.Name,
			"error":   err,
		}).Error("failed-to-send-input-to-cli")
		e.SendToBot(msg.Platform, msg.Channel, fmt.Sprintf("❌ Failed to send input: %v", err))
		return
	}

	logger.WithFields(logrus.Fields{
		"session": session.Name,
		"cli":     session.CLIType,
	}).Info("input-sent-to-cli")

	// Update session state
	e.updateSessionState(session.Name, StateProcessing)

	// Start watchdog monitoring (for detecting interactive prompts)
	go func(s *Session) {
		defer func() {
			if r := recover(); r != nil {
				logger.WithFields(logrus.Fields{
					"session": s.Name,
					"panic":   r,
				}).Error("watchdog-panic-recovered")
			}
		}()
		if err := e.startWatchdog(s); err != nil {
			logger.WithFields(logrus.Fields{
				"session": s.Name,
				"error":   err,
			}).Error("watchdog-failed")
		}
	}(session)
}

// HandleSpecialCommand handles special clibot commands
func (e *Engine) HandleSpecialCommand(cmd string, msg bot.BotMessage) {
	log.Printf("Special command: %s", cmd)

	// Parse command and arguments
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		e.SendToBot(msg.Platform, msg.Channel, "❌ Empty command")
		return
	}

	command := parts[0]

	// Handle commands with arguments
	switch command {
	case "sessions":
		e.listSessions(msg)
	case "status":
		e.showStatus(msg)
	case "whoami":
		e.showWhoami(msg)
	case "tmux":
		e.captureTmux(msg, parts)
	default:
		e.SendToBot(msg.Platform, msg.Channel,
			fmt.Sprintf("❌ Unknown command: %s\nAvailable commands:\n  sessions - List all sessions\n  status - Show session status\n  whoami - Show current session\n  tmux [lines] - Capture tmux pane (default: 50 lines)", command))
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

// captureTmux captures and displays tmux pane content
// Usage: tmux [lines]
// If lines is not provided, defaults to 50
func (e *Engine) captureTmux(msg bot.BotMessage, parts []string) {
	// Parse line count parameter (default: 50)
	lines := tmuxCapturePaneLine
	if len(parts) >= 2 {
		if _, err := fmt.Sscanf(parts[1], "%d", &lines); err != nil {
			e.SendToBot(msg.Platform, msg.Channel, fmt.Sprintf("❌ Invalid line count: %s\nUsage: tmux [lines]", parts[1]))
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

	// Capture tmux pane content
	output, err := watchdog.CapturePane(session.Name, lines)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"session": session.Name,
			"lines":   lines,
			"error":   err,
		}).Error("failed-to-capture-tmux-pane")
		e.SendToBot(msg.Platform, msg.Channel, fmt.Sprintf("❌ Failed to capture tmux pane: %v", err))
		return
	}

	// Strip ANSI codes for cleaner output
	cleanOutput := watchdog.StripANSI(output)
	// Send response with header
	response := fmt.Sprintf("📺 Tmux Capture (%s, last %d lines):\n```\n%s\n```", session.Name, lines, cleanOutput)
	e.SendToBot(msg.Platform, msg.Channel, response)

	logger.WithFields(logrus.Fields{
		"session":        session.Name,
		"lines_requested": lines,
		"output_length":  len(cleanOutput),
	}).Info("tmux-capture-command-executed")
}

// GetActiveSession gets the active session for a channel
func (e *Engine) GetActiveSession(channel string) *Session {
	e.sessionMu.RLock()
	defer e.sessionMu.RUnlock()

	// For now, return the default session
	// TODO: Implement per-channel session mapping
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
func (e *Engine) startWatchdog(session *Session) error {
	// TODO: Implement watchdog monitoring logic
	// Issue: https://github.com/keepmind9/clibot/issues/123
	logger.WithField("session", session.Name).Warn("watchdog-not-implemented")
	return nil
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
		const maxRetries = constants.MaxHookRetries
		const initialDelay = constants.DefaultInitialDelay // Initial delay to let UI render
		const retryDelay = constants.DefaultRetryDelay     // Delay between retry attempts
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
// Removes trailing slashes and expands relative paths if needed
func normalizePath(path string) string {
	// Remove trailing slash
	path = strings.TrimSuffix(path, "/")

	// TODO: Expand relative paths to absolute paths
	// For now, just return the cleaned path
	return path
}
