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
	"github.com/keepmind9/clibot/internal/watchdog"
	"github.com/sirupsen/logrus"
)

// Engine is the core scheduling engine that manages CLI sessions and bot connections
type Engine struct {
	config      *Config
	cliAdapters map[string]cli.CLIAdapter  // CLI type -> adapter
	activeBots  map[string]bot.BotAdapter  // Bot type -> adapter
	sessions    map[string]*Session        // Session name -> Session
	sessionMu   sync.RWMutex               // Mutex for session access
	messageChan chan bot.BotMessage        // Bot message channel
	hookServer  *http.Server               // HTTP server for hooks
	sessionChannels map[string]BotChannel  // Session name -> active bot channel (for routing responses)
}

// BotChannel represents a bot channel for sending responses
type BotChannel struct {
	Platform string  // "discord", "telegram", "feishu", etc.
	Channel  string  // Channel ID (platform-specific)
}

// NewEngine creates a new Engine instance
func NewEngine(config *Config) *Engine {
	return &Engine{
		config:         config,
		cliAdapters:    make(map[string]cli.CLIAdapter),
		activeBots:     make(map[string]bot.BotAdapter),
		sessions:       make(map[string]*Session),
		messageChan:    make(chan bot.BotMessage, 100),
		sessionChannels: make(map[string]BotChannel),
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
func (e *Engine) Run() error {
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
			if err := ba.Start(e.HandleBotMessage); err != nil {
				log.Printf("Failed to start %s bot: %v", bt, err)
			}
		}(botType, botAdapter)
	}

	// Start main event loop
	e.runEventLoop()

	return nil
}

// runEventLoop runs the main event loop for processing messages
func (e *Engine) runEventLoop() {
	logger.Info("engine-event-loop-started")

	for {
		select {
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
		go e.startWatchdog(session)

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
	go e.startWatchdog(session)
}

// HandleSpecialCommand handles special clibot commands
func (e *Engine) HandleSpecialCommand(cmd string, msg bot.BotMessage) {
	log.Printf("Special command: %s", cmd)

	// Parse command
	// For now, implement basic commands
	switch cmd {
	case "sessions":
		e.listSessions(msg)
	case "status":
		e.showStatus(msg)
	case "whoami":
		e.showWhoami(msg)
	default:
		e.SendToBot(msg.Platform, msg.Channel,
			fmt.Sprintf("❌ Unknown command: %s\nAvailable commands: sessions, status, whoami", cmd))
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
func (e *Engine) startWatchdog(session *Session) {
	// TODO: Implement watchdog monitoring logic
	log.Printf("Watchdog started for session %s", session.Name)
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
		const maxRetries = 10
		const retryDelay = 800 * time.Millisecond  // 0.8 seconds (async hook: faster response expected)
		var lastResponse string

	for attempt := 1; attempt <= maxRetries; attempt++ {
			if attempt > 1 {
				logger.WithField("attempt", attempt).Info("retrying-tmux-capture")
				time.Sleep(retryDelay)
			}

			tmuxOutput, err := watchdog.CapturePane(session.Name, 200)
			if err != nil {
				logger.WithField("error", err).Warn("failed-to-capture-tmux-pane")
				break
			}

			cleanOutput := watchdog.StripANSI(tmuxOutput)

			// Extract content after the user's prompt
			filteredOutput := extractContentAfterPrompt(cleanOutput, lastUserPrompt)

			logger.WithFields(logrus.Fields{
				"attempt":          attempt,
				"filtered_length":  len(filteredOutput),
				"filtered_preview": filteredOutput[:min(200, len(filteredOutput))],
				"is_thinking":      isThinking(filteredOutput),
			}).Debug("extracted-content-after-user-prompt")

			// Check if still thinking in the filtered content
			if isThinking(filteredOutput) {
				logger.WithFields(logrus.Fields{
					"attempt":     attempt,
					"max_retries": maxRetries,
					"reason":      "thinking detected in filtered output",
				}).Debug("claude-is-still-thinking-in-response-area-will-retry")
				lastResponse = filteredOutput // Save for final attempt
				continue
			}

			// Not thinking anymore, remove UI status lines and validate response
			response = removeUIStatusLines(filteredOutput)

			// Validate response - just check if not empty
			// We already have thinking check and UI filtering, so any content is valid
			if response != "" {
				logger.WithFields(logrus.Fields{
					"source":           "tmux",
					"attempt":           attempt,
					"response_length":  len(response),
					"response_preview": response[:min(200, len(response))],
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
			response = removeUIStatusLines(lastResponse)
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

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// isPromptOrCommand checks if a line is a prompt/command rather than assistant output
func isPromptOrCommand(line string) bool {
	promptPatterns := []string{
		"user@",
		"$ ",
		">>>",
		"...",
		"[?]",
		"Press Enter",
		"Confirm?",
	}

	for _, pattern := range promptPatterns {
		if strings.Contains(line, pattern) {
			return true
		}
	}

	return false
}

// extractLastAssistantContent extracts the last meaningful assistant response from tmux output
// Filters out UI borders, prompts, and system messages
func extractLastAssistantContent(output string) string {
	lines := strings.Split(output, "\n")

	// Filter out borders and UI elements
	var contentLines []string
	skipBorder := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Skip empty lines
		if trimmed == "" {
			continue
		}

		// Detect and skip UI borders (box drawing characters)
		if strings.Contains(trimmed, "─") || strings.Contains(trimmed, "│") ||
		   strings.Contains(trimmed, "┌") || strings.Contains(trimmed, "└") ||
		   strings.Contains(trimmed, "┐") || strings.Contains(trimmed, "┘") ||
		   strings.Contains(trimmed, "├") || strings.Contains(trimmed, "┤") {
			if strings.Contains(trimmed, "Claude Code") {
				skipBorder = true
			}
			continue
		}

		// Skip while we're in the border area
		if skipBorder {
			if !strings.Contains(trimmed, "─") && !strings.Contains(trimmed, "│") {
				skipBorder = false
			} else {
				continue
			}
		}

		// Skip prompts and commands
		if isPromptOrCommand(trimmed) {
			continue
		}

		contentLines = append(contentLines, trimmed)
	}

	// Join with newlines
	result := strings.Join(contentLines, "\n")

	// Remove duplicate consecutive blank lines
	for strings.Contains(result, "\n\n\n") {
		result = strings.ReplaceAll(result, "\n\n\n", "\n\n")
	}

	return result
}

// extractContentAfterPrompt extracts content appearing after the user's prompt
// Searches from the END to find the LAST occurrence of the prompt
// This filters out historical messages and only returns the latest response
func extractContentAfterPrompt(tmuxOutput, userPrompt string) string {
	if userPrompt == "" {
		// No prompt available, use basic extraction
		return extractLastAssistantContent(tmuxOutput)
	}

	lines := strings.Split(tmuxOutput, "\n")

	// Search from the end to find the last occurrence of user prompt
	promptIndex := -1
	promptPrefix := userPrompt
	if len(userPrompt) > 30 {
		// Use first 30 chars as prefix for matching long prompts
		promptPrefix = userPrompt[:30]
	}

	// Search backwards for the prompt with improved matching logic
	for i := len(lines) - 1; i >= 0; i-- {
		trimmed := strings.TrimSpace(lines[i])

		// Priority 1: Exact match (most reliable)
		if trimmed == userPrompt || trimmed == "❯ "+userPrompt {
			promptIndex = i
			logger.WithFields(logrus.Fields{
				"line_index":     i,
				"prompt_matched": trimmed,
				"match_type":     "exact",
			}).Debug("found-user-prompt-exact-match")
			break
		}

		// Priority 2: Match with cursor prefix
		if strings.HasPrefix(trimmed, "❯ ") && strings.Contains(trimmed, userPrompt) {
			// Verify it's the actual user prompt, not AI content
			if isLikelyUserPromptLine(trimmed, userPrompt) {
				promptIndex = i
				logger.WithFields(logrus.Fields{
					"line_index":     i,
					"prompt_matched": trimmed,
					"match_type":     "cursor_prefix",
				}).Debug("found-user-prompt-with-cursor-prefix")
				break
			}
		}

		// Priority 3: Partial match (fallback, but with validation)
		if strings.Contains(trimmed, userPrompt) {
			// Only match if this looks like a user prompt line
			if isLikelyUserPromptLine(trimmed, userPrompt) {
				promptIndex = i
				logger.WithFields(logrus.Fields{
					"line_index":     i,
					"prompt_matched": trimmed,
					"match_type":     "partial",
				}).Debug("found-user-prompt-partial-match-with-validation")
				break
			}
		}

		// Priority 4: Prefix match for long prompts
		if len(userPrompt) > 30 && strings.Contains(trimmed, promptPrefix) {
			if isLikelyUserPromptLine(trimmed, userPrompt) {
				promptIndex = i
				logger.WithFields(logrus.Fields{
					"line_index":            i,
					"prompt_matched_prefix": trimmed,
					"match_type":            "prefix",
				}).Debug("found-user-prompt-prefix-match-with-validation")
				break
			}
		}
	}

	// If prompt not found, fall back to basic extraction
	if promptIndex == -1 {
		logger.Debug("user-prompt-not-found-in-tmux-output-using-basic-extraction")
		return extractLastAssistantContent(tmuxOutput)
	}

	// Collect content AFTER the prompt (forward from promptIndex + 1)
	var contentLines []string
	skipBorder := false

	for i := promptIndex + 1; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])

		// Skip empty lines
		if trimmed == "" {
			continue
		}

		// Detect and skip UI borders
		if strings.Contains(trimmed, "─") || strings.Contains(trimmed, "│") ||
		   strings.Contains(trimmed, "┌") || strings.Contains(trimmed, "└") {
			if strings.Contains(trimmed, "Claude Code") {
				skipBorder = true
			}
			continue
		}

		if skipBorder {
			if !strings.Contains(trimmed, "─") && !strings.Contains(trimmed, "│") {
				skipBorder = false
			} else {
				continue
			}
		}

		// Skip prompts and commands
		if isPromptOrCommand(trimmed) {
			continue
		}

		contentLines = append(contentLines, trimmed)
	}

	logger.WithFields(logrus.Fields{
		"total_lines":     len(lines),
		"prompt_index":    promptIndex,
		"content_lines":    len(contentLines),
	}).Debug("extracted-content-after-prompt")

	if len(contentLines) == 0 {
		logger.Debug("no-content-found-after-prompt-using-basic-extraction")
		return extractLastAssistantContent(tmuxOutput)
	}

	result := strings.Join(contentLines, "\n")

	// Clean up multiple consecutive newlines
	for strings.Contains(result, "\n\n\n") {
		result = strings.ReplaceAll(result, "\n\n\n", "\n\n")
	}

	return result
}

// isThinking checks if Claude Code is still thinking based on tmux output
// Uses universal keywords that work across different AI CLI tools
// Only checks the last N lines to accurately determine current state
func isThinking(output string) bool {
	// Only check the last 20 lines for accurate current state
	lines := strings.Split(output, "\n")
	startIndex := 0
	if len(lines) > 20 {
		startIndex = len(lines) - 20
	}
	recentLines := lines[startIndex:]

	// Universal thinking indicators (work across Claude, Gemini, etc.)
	thinkingIndicators := []string{
		"thinking",
		"esc to interrupt",
		"press escape to interrupt",
		"interrupt",
	}

	recentOutput := strings.Join(recentLines, "\n")
	outputLower := strings.ToLower(recentOutput)

	for _, indicator := range thinkingIndicators {
		if strings.Contains(outputLower, indicator) {
			logger.WithFields(logrus.Fields{
				"indicator":        indicator,
				"checked_lines":    len(recentLines),
				"total_lines":      len(lines),
			}).Debug("detected-thinking-state-in-recent-lines")
			return true
		}
	}

	return false
}

// looksLikeIncompleteResponse checks if the response seems incomplete
func looksLikeIncompleteResponse(response string) bool {
	// Very short responses might be incomplete
	if len(response) < 50 {
		// Check if it ends abruptly without proper punctuation
		lastChar := response[len(response)-1:]
		if lastChar != "." && lastChar != "!" && lastChar != "?" &&
		   lastChar != "。" && lastChar != "！" && lastChar != "？" &&
		   lastChar != "\n" && lastChar != "`" {
			return true
		}
	}

	// Check for code blocks that might be incomplete
	if strings.HasSuffix(response, "```") {
		return true // Incomplete code block
	}

	// Check for incomplete lists
	if strings.HasSuffix(response, "-") || strings.HasSuffix(response, "*") {
		return true
	}

	return false
}

// removeUIStatusLines removes Claude Code UI status lines from the response
// This should be called AFTER isThinking() check, when response is ready to send to user
// It removes UI artifacts like "running stop hook", "esc to interrupt", etc.
func removeUIStatusLines(output string) string {
	lines := strings.Split(output, "\n")
	var filteredLines []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Skip empty lines
		if trimmed == "" {
			continue
		}

		// Remove UI status lines
		if isUIStatusLine(trimmed) {
			logger.WithField("line", trimmed).Debug("removing-ui-status-line-from-response")
			continue
		}

		filteredLines = append(filteredLines, line)
	}

	result := strings.Join(filteredLines, "\n")

	// Clean up multiple consecutive newlines
	for strings.Contains(result, "\n\n\n") {
		result = strings.ReplaceAll(result, "\n\n\n", "\n\n")
	}

	return result
}

// isUIStatusLine checks if a line is a Claude Code UI status line
// UI status lines include indicators like "running stop hook", "esc to interrupt", etc.
// These lines are used for thinking detection but should be removed from final response
func isUIStatusLine(line string) bool {
	// UI status line patterns
	uiPatterns := []string{
		"Undulating…",
		"running stop hook",
		"esc to interrupt",
		"press escape",
		"? for shortcuts",
	}

	lowerLine := strings.ToLower(line)
	for _, pattern := range uiPatterns {
		if strings.Contains(lowerLine, strings.ToLower(pattern)) {
			return true
		}
	}

	// Check for single-character cursor indicators
	if line == "❯" || line == ">" || line == "$" {
		return true
	}

	return false
}

// isLikelyUserPromptLine checks if a line is likely to be the user's prompt input
// rather than AI-generated content containing the same keywords
// This prevents false matches like matching "test content follows" when user input is "test"
//
// Key insight: In tmux, user input typically has a cursor prefix "❯ "
func isLikelyUserPromptLine(line, userPrompt string) bool {
	// Priority 1: Lines with cursor prefix (most reliable indicator)
	if strings.HasPrefix(line, "❯ ") && strings.Contains(line, userPrompt) {
		logger.WithFields(logrus.Fields{
			"line":        line,
			"user_prompt": userPrompt,
			"reason":      "has cursor prefix",
		}).Debug("accepting-line-has-cursor-prefix")
		return true
	}

	// Priority 2: Exact match (for cases without cursor prefix)
	if line == userPrompt {
		logger.WithFields(logrus.Fields{
			"line":        line,
			"user_prompt": userPrompt,
			"reason":      "exact match",
		}).Debug("accepting-line-exact-match")
		return true
	}

	// Reject all other cases (including AI responses like "test content follows")
	logger.WithFields(logrus.Fields{
		"line":        line,
		"user_prompt": userPrompt,
		"line_len":    len(line),
		"prompt_len":  len(userPrompt),
		"reason":      "no cursor prefix and not exact match",
	}).Debug("rejecting-line-does-not-look-like-user-prompt")

	return false
}
