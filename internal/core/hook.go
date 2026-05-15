package core

import (
	"fmt"
	"io"
	"net/http"

	"github.com/keepmind9/clibot/internal/logger"
	"github.com/sirupsen/logrus"
)

// startHookServer starts the HTTP hook server in a separate goroutine
// This server listens for completion notifications from CLI tools
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

// handleHookRequest handles HTTP hook requests from CLI tools
//
// This function:
// 1. Validates the request (POST method, cli_type parameter)
// 2. Reads raw data from request body
// 3. Delegates parsing to CLI adapter (protocol-agnostic)
// 4. Matches session by identifier (e.g., working directory)
// 5. Captures response from tmux (with retry mechanism)
// 6. Sends response to user via bot
//
// The retry mechanism handles cases where:
// - Adapter returns empty response (fallback to tmux capture)
// - CLI is still "thinking" (IsThinking check)
// - UI hasn't finished rendering (delay and retry)
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
		http.Error(w, "CLI adapter not found", http.StatusBadRequest)
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
		"cli_type":  cliType,
		"hook_data": string(data),
	}).Debug("hook-data-received")

	// Delegate to CLI adapter (protocol-agnostic)
	// The adapter parses the data and returns: (identifier, lastUserPrompt, response, error)
	// identifier is used to match the session (e.g., cwd, session name, etc.)
	identifier, _, response, err := adapter.HandleHookData(data)
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

	// Ignore hook requests for ACP sessions (they use ACP protocol, not hooks)
	if session.CLIType == "acp" {
		logger.WithFields(logrus.Fields{
			"session":  session.Name,
			"cli_type": session.CLIType,
		}).Debug("ignoring-hook-request-for-acp-session-uses-acp-protocol")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Hook received (ACP mode - ignored)")
		return
	}

	// If adapter returned empty response, return error to user
	if response == "" {
		logger.WithFields(logrus.Fields{
			"session":  session.Name,
			"cli_type": session.CLIType,
		}).Error("adapter-returned-empty-response-check-transcript-file")
		http.Error(w, "Adapter returned empty response - check CLI transcript file", http.StatusInternalServerError)
		return
	}

	// Update session state to idle
	e.updateSessionState(session.Name, StateIdle)
	e.touchSession(session.Name)

	// Get the bot channel for this session
	channels := e.snapshotSessionChannels(session.Name)
	if len(channels) == 0 {
		// No active channel - user might be operating CLI directly
		logger.WithFields(logrus.Fields{
			"session": session.Name,
		}).Debug("no-active-channel-found-skipping-bot-notification")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Hook received (no active bot channel)")
		return
	}

	// Send to all bot channels for this session
	logger.WithFields(logrus.Fields{
		"session":       session.Name,
		"channel_count": len(channels),
	}).Info("sending-hook-response-to-bot")

	for _, botChannel := range channels {
		e.SendToBot(botChannel.Platform, botChannel.Channel, response)
		if botChannel.MessageID != "" {
			e.removeTypingIndicatorAsync(botChannel.Platform, botChannel.MessageID)
		}
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Hook received")
}

// truncateString truncates a string to a maximum length
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
