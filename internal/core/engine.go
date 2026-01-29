package core

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/keepmind9/clibot/internal/bot"
	"github.com/keepmind9/clibot/internal/cli"
)

// Engine is the core scheduling engine that manages CLI sessions and bot connections
type Engine struct {
	config      *Config
	cliAdapters map[string]cli.CLIAdapter  // CLI type -> adapter
	activeBots  map[string]bot.BotAdapter  // Bot type -> adapter
	sessions    map[string]*Session        // Session name -> Session
	sessionMu   sync.RWMutex               // Mutex for session access
	messageChan chan bot.BotMessage        // Bot message channel
	responseChan chan ResponseEvent        // CLI response channel
	hookServer  *http.Server               // HTTP server for hooks
}

// NewEngine creates a new Engine instance
func NewEngine(config *Config) *Engine {
	return &Engine{
		config:      config,
		cliAdapters: make(map[string]cli.CLIAdapter),
		activeBots:  make(map[string]bot.BotAdapter),
		sessions:    make(map[string]*Session),
		messageChan: make(chan bot.BotMessage, 100),
		responseChan: make(chan ResponseEvent, 100),
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
	log.Println("Starting clibot engine...")

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

// runEventLoop runs the main event loop for processing messages and responses
func (e *Engine) runEventLoop() {
	log.Println("Engine event loop started")

	for {
		select {
		case msg := <-e.messageChan:
			e.HandleUserMessage(msg)

		case event := <-e.responseChan:
			e.HandleCLIResponse(event)
		}
	}
}

// HandleBotMessage is the callback function for bots to deliver messages
func (e *Engine) HandleBotMessage(msg bot.BotMessage) {
	e.messageChan <- msg
}

// HandleUserMessage processes a message from a user
func (e *Engine) HandleUserMessage(msg bot.BotMessage) {
	log.Printf("[%s] User %s: %s", msg.Platform, msg.UserID, msg.Content)

	// Step 0: Security check - verify user is in whitelist
	if !e.config.IsUserAuthorized(msg.Platform, msg.UserID) {
		e.SendToBot(msg.Platform, msg.Channel, "❌ Unauthorized: Please contact the administrator to add your user ID")
		return
	}

	// Step 1: Check if it's a special command
	prefix := e.config.CommandPrefix
	if len(msg.Content) > len(prefix) && msg.Content[:len(prefix)] == prefix {
		cmd := msg.Content[len(prefix):]
		e.HandleSpecialCommand(cmd, msg)
		return
	}

	// Step 2: Get the active session for this channel
	session := e.GetActiveSession(msg.Channel)
	if session == nil {
		e.SendToBot(msg.Platform, msg.Channel,
			fmt.Sprintf("❌ No active session. Use '%ssessions' to list available sessions", prefix))
		return
	}

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
		e.SendToBot(msg.Platform, msg.Channel, fmt.Sprintf("❌ Failed to send input: %v", err))
		return
	}

	// Update session state
	e.updateSessionState(session.Name, StateProcessing)

	// Step 5: Start Watchdog and timeout timer
	go func(sessionName string) {
		// Start watchdog
		e.startWatchdog(session)

		// Wait for hook event or timeout
		select {
		case resp := <-e.responseChan:
			if resp.SessionName == sessionName {
				e.updateSessionState(sessionName, StateIdle)
				e.SendToAllBots(resp.Response)
			}
		case <-time.After(5 * time.Minute):
			e.updateSessionState(sessionName, StateError)
			e.SendToAllBots("⚠️ CLI response timeout\nSuggestion: Use !!status to check status")
		}
	}(session.Name)
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
		log.Printf("Session %s state: %s -> %s", sessionName, oldState, newState)
	}
}

// SendToBot sends a message to a specific bot
func (e *Engine) SendToBot(platform, channel, message string) {
	if botAdapter, exists := e.activeBots[platform]; exists {
		if err := botAdapter.SendMessage(channel, message); err != nil {
			log.Printf("Failed to send message to %s: %v", platform, err)
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

// HandleCLIResponse handles a response event from CLI
func (e *Engine) HandleCLIResponse(event ResponseEvent) {
	log.Printf("Received response from session %s", event.SessionName)

	// Send to response channel (will be picked up by the waiting goroutine)
	e.responseChan <- event
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

	log.Printf("Hook server listening on %s", addr)

	// Start server (blocking)
	// When Shutdown() is called, ListenAndServe will return ErrServerClosed
	if err := e.hookServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Printf("Hook server error: %v", err)
	}

	log.Println("Hook server stopped")
}

// handleHookRequest handles HTTP hook requests
func (e *Engine) handleHookRequest(w http.ResponseWriter, r *http.Request) {
	// Only accept POST requests
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get cli_type from query parameter
	cliType := r.URL.Query().Get("cli_type")
	if cliType == "" {
		log.Printf("Missing cli_type query parameter")
		http.Error(w, "Missing cli_type parameter", http.StatusBadRequest)
		return
	}

	// Parse JSON body (raw data from CLI)
	var data map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		log.Printf("Failed to decode hook payload: %v", err)
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	// Extract session and event from data (flexible - different CLIs may use different field names)
	var sessionName, event string

	if s, ok := data["session"].(string); ok {
		sessionName = s
	}
	if s, ok := data["session_name"].(string); ok {
		sessionName = s
	}

	if ev, ok := data["event"].(string); ok {
		event = ev
	}
	if ev, ok := data["event_type"].(string); ok {
		event = ev
	}

	log.Printf("Hook received: cli_type=%s, session=%s, event=%s", cliType, sessionName, event)

	// Validate required fields
	if sessionName == "" {
		log.Printf("Missing session name in hook data")
		http.Error(w, "Missing session name", http.StatusBadRequest)
		return
	}

	if event == "" {
		log.Printf("Missing event type in hook data")
		http.Error(w, "Missing event type", http.StatusBadRequest)
		return
	}

	// Handle completed event
	if event == "completed" {
		// Get session info
		e.sessionMu.RLock()
		session, exists := e.sessions[sessionName]
		e.sessionMu.RUnlock()

		if !exists {
			log.Printf("Session %s not found", sessionName)
			http.Error(w, "Session not found", http.StatusNotFound)
			return
		}

		// Get CLI adapter
		adapter, exists := e.cliAdapters[session.CLIType]
		if !exists {
			log.Printf("No adapter found for CLI type: %s", session.CLIType)
			http.Error(w, "CLI adapter not found", http.StatusInternalServerError)
			return
		}

		// Get response from CLI adapter using hook data
		response, err := adapter.HandleHookData(data)
		if err != nil {
			log.Printf("Failed to get response from session %s: %v", sessionName, err)
			http.Error(w, "Failed to get response", http.StatusInternalServerError)
			return
		}

		// Send to response channel
		e.responseChan <- ResponseEvent{
			SessionName: sessionName,
			Response:    response,
			Timestamp:   time.Now().Format(time.RFC3339),
		}
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Hook received")
}

// Stop gracefully stops the engine
func (e *Engine) Stop() error {
	log.Println("Stopping clibot engine...")

	// Stop hook server with graceful shutdown
	if e.hookServer != nil {
		log.Println("Stopping hook server...")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := e.hookServer.Shutdown(ctx); err != nil {
			log.Printf("Failed to gracefully stop hook server: %v", err)
			// Force close if graceful shutdown fails
			e.hookServer.Close()
		} else {
			log.Println("Hook server stopped gracefully")
		}
	}

	// Stop all bots
	for botType, botAdapter := range e.activeBots {
		log.Printf("Stopping %s bot...", botType)
		if err := botAdapter.Stop(); err != nil {
			log.Printf("Failed to stop %s bot: %v", botType, err)
		}
	}

	log.Println("Engine stopped")
	return nil
}
