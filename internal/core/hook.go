package core

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/keepmind9/clibot/internal/logger"
	"github.com/keepmind9/clibot/internal/watchdog"
	"github.com/keepmind9/clibot/pkg/constants"
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

	if !adapter.UseHook() {
		logger.WithField("cli_type", cliType).Warn("no-usehook-for-cli-type")
		http.Error(w, "CLI adapter can't useHook", http.StatusBadRequest)
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
		response = e.captureResponseWithRetry(session, lastUserPrompt)
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

// captureResponseWithRetry captures response from tmux with retry mechanism
//
// This function handles the case where the CLI adapter returns empty response,
// requiring fallback to tmux capture. It implements a retry mechanism to handle:
// - Initial delay for UI rendering
// - "Thinking" state detection
// - Multiple retry attempts with configurable delays
//
// Parameters:
//
//	session: The CLI session
//	lastUserPrompt: The last user input prompt (used for content filtering)
//
// Returns:
//
//	The captured response, or empty string if all retries fail
func (e *Engine) captureResponseWithRetry(session *Session, lastUserPrompt string) string {
	logger.WithFields(logrus.Fields{
		"session":          session.Name,
		"last_user_prompt": lastUserPrompt,
		"reason":           "adapter returned empty response",
	}).Info("using-tmux-capture-as-fallback-with-user-prompt-filtering")

	// Get retry configuration
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
	var lastAlgorithmUsed string

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

		// Try incremental extraction first (if snapshot exists)
		var filteredOutput string
		var algorithmUsed string
		if e.inputTracker != nil {
			beforeSnapshot, _, snapshotErr := e.inputTracker.GetSnapshotPair(session.Name, session.CLIType)
			if snapshotErr == nil && beforeSnapshot != "" {
				// Use incremental extraction (after - before)
				filteredOutput = watchdog.ExtractIncrement(cleanOutput, beforeSnapshot)
				algorithmUsed = "incremental_snapshot"
			} else {
				// Fallback to prompt matching
				filteredOutput = watchdog.ExtractContentAfterPrompt(cleanOutput, lastUserPrompt)
				algorithmUsed = "prompt_matching"
			}

			logger.WithFields(logrus.Fields{
				"attempt":          attempt,
				"filtered_length":  len(filteredOutput),
				"filtered_preview": truncateString(filteredOutput, 200),
				"is_thinking":      watchdog.IsThinking(filteredOutput),
				"algorithm":        algorithmUsed,
				"event":            "parser_hook_using_" + algorithmUsed,
			}).Info("parser_hook_using_algorithm")
		} else {
			// No input tracker, use prompt matching
			filteredOutput = watchdog.ExtractContentAfterPrompt(cleanOutput, lastUserPrompt)
			algorithmUsed = "prompt_matching"
			logger.WithFields(logrus.Fields{
				"attempt":          attempt,
				"filtered_length":  len(filteredOutput),
				"filtered_preview": truncateString(filteredOutput, 200),
				"is_thinking":      watchdog.IsThinking(filteredOutput),
				"algorithm":        "prompt_matching",
				"event":            "parser_hook_using_prompt_matching",
			}).Info("parser_hook_using_prompt_matching_fallback")
		}

		// Save algorithm used for this attempt
		lastAlgorithmUsed = algorithmUsed

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
		response := watchdog.RemoveUIStatusLines(filteredOutput)

		// Validate response - just check if not empty
		// We already have thinking check and UI filtering, so any content is valid
		if response != "" {
			logger.WithFields(logrus.Fields{
				"source":           "tmux",
				"attempt":          attempt,
				"response_length":  len(response),
				"response_preview": truncateString(response, 200),
				"algorithm":        lastAlgorithmUsed,
				"event":            "parser_hook_successfully_extracted_response",
			}).Info("parser_hook_successfully_extracted_response")
			return response // Got valid response, return immediately
		}

		logger.WithFields(logrus.Fields{
			"attempt":         attempt,
			"response_length": len(response),
			"reason":          "empty response",
		}).Debug("response-validation-failed-will-retry")
	}

	// If still no valid response after all retries, use the last capture
	if lastResponse != "" {
		logger.WithFields(logrus.Fields{
			"max_retries":        maxRetries,
			"using_last_attempt": true,
			"response_length":    len(lastResponse),
		}).Info("using-last-attempt-capture-still-may-be-thinking")
		return watchdog.RemoveUIStatusLines(lastResponse)
	}

	logger.WithField("max_retries", maxRetries).Warn("all-retry-attempts-exhausted-and-no-valid-response")
	return ""
}

// truncateString truncates a string to a maximum length
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
