package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/coder/acp-go-sdk"
	"github.com/keepmind9/clibot/internal/logger"
	"github.com/sirupsen/logrus"
	"syscall"
)

// - "" or "stdio://" → stdio with no address
// - "tcp://host:port" → TCP with address
// - "unix:///path" → Unix socket with path
func parseTransportURL(transportURL string) (transportType ACPTransportType, address string) {
	if transportURL == "" || transportURL == "stdio://" {
		return ACPTransportStdio, ""
	}

	if strings.HasPrefix(transportURL, "tcp://") {
		addr := strings.TrimPrefix(transportURL, "tcp://")
		return ACPTransportTCP, addr
	}

	if strings.HasPrefix(transportURL, "unix://") {
		path := strings.TrimPrefix(transportURL, "unix://")
		return ACPTransportUnix, path
	}

	// Default to stdio if unrecognized
	return ACPTransportStdio, ""
}

type ACPAdapter struct {
	config            ACPAdapterConfig
	mu                sync.Mutex
	sessions          map[string]*acpSession
	currentEngine     Engine     // Engine reference for sending responses
	contextUsageLimit float64    // Threshold to trigger auto-reset (0.0 to 1.0, e.g., 0.5 for 50%)
}

type acpSession struct {
	ctx           context.Context
	cancel        context.CancelFunc
	active        bool
	connReady     chan struct{} // Closed when connection is ready for this session
	sessionId     string        // ACP session ID from server
	workDir       string        // Saved workDir for recreation
	startCmd      string        // Saved startCmd for recreation
	lastUsagePerc float64       // Last recorded context usage percentage (0-100)
	
	// Per-session resources
	conn     *acp.ClientSideConnection
	cmd      *exec.Cmd
	client   *acpClient
	isRemote bool
}

// acpClient implements acp.Client interface for ACP callbacks
type acpClient struct {
	adapter          *ACPAdapter
	sessionName      string // Session name for this client instance
	responseBuf      strings.Builder
	mu               sync.Mutex     // Protects responseBuf
	activityChan     chan time.Time // Channel for activity notifications
	lastActivityLock sync.RWMutex   // Protects lastActivityTime
	lastActivityTime time.Time      // Last time we received activity from agent
}

// NewACPAdapter creates a new ACP adapter
func NewACPAdapter(config ACPAdapterConfig) (*ACPAdapter, error) {
	// Handle backward compatibility: RequestTimeout -> IdleTimeout
	if config.RequestTimeout > 0 && config.IdleTimeout == 0 {
		config.IdleTimeout = config.RequestTimeout
	}

	// Set default idle timeout if not specified
	if config.IdleTimeout == 0 {
		config.IdleTimeout = defaultACPIdleTimeout
	}

	// Set default max total timeout if not specified
	if config.MaxTotalTimeout == 0 {
		config.MaxTotalTimeout = defaultACPMaxTotalTimeout
	}

	logger.WithFields(logrus.Fields{
		"idle_timeout":      config.IdleTimeout,
		"max_total_timeout": config.MaxTotalTimeout,
		"env_count":         len(config.Env),
		"env_vars":          config.Env,
	}).Info("acp-adapter-configured")

	return &ACPAdapter{
		config:            config,
		sessions:          make(map[string]*acpSession),
		contextUsageLimit: 0.6, // Default to 60%
	}, nil
}

// SetEngine sets the engine reference for sending responses
func (a *ACPAdapter) SetEngine(engine Engine) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.currentEngine = engine
}

// UseHook returns false - ACP doesn't use hook mode
func (a *ACPAdapter) UseHook() bool {
	return false
}

// GetPollInterval returns polling interval (ACP uses request/response)
func (a *ACPAdapter) GetPollInterval() time.Duration {
	return acpPollInterval
}

// GetStableCount returns stable count (not used in ACP mode)
func (a *ACPAdapter) GetStableCount() int {
	return 1
}

// GetPollTimeout returns request timeout (for compatibility with polling mode)
func (a *ACPAdapter) GetPollTimeout() time.Duration {
	return a.config.IdleTimeout
}

// HandleHookData - not used in ACP mode
func (a *ACPAdapter) HandleHookData(data []byte) (string, string, string, error) {
	return "", "", "", fmt.Errorf("ACP mode does not use hook data")
}

// IsSessionAlive checks if session is active
func (a *ACPAdapter) IsSessionAlive(sessionName string) bool {
	a.mu.Lock()
	sess, ok := a.sessions[sessionName]
	a.mu.Unlock()

	if !ok {
		return false
	}

	active := sess.active
	if !active {
		logger.WithField("session", sessionName).Debug("session-alive-check-failed-inactive")
		return false
	}

	alive := a.isSessionActive(sess)
	if !alive {
		logger.WithField("session", sessionName).Debug("session-alive-check-failed-process-died")
	}
	return alive
}

// isSessionActive checks if the underlying process or connection for a session is still alive.
func (a *ACPAdapter) isSessionActive(sess *acpSession) bool {
	if sess.isRemote {
		if sess.conn == nil {
			return false
		}
		select {
		case <-sess.conn.Done():
			return false
		default:
			return true
		}
	} else {
		if sess.cmd == nil || sess.cmd.Process == nil {
			return false
		}
		return sess.cmd.Process.Signal(os.Signal(syscall.Signal(0))) == nil
	}
}

// ResetSession starts a new conversation without deleting history
func (a *ACPAdapter) ResetSession(sessionName string) error {
	logger.WithField("session", sessionName).Info("starting-new-acp-conversation")

	a.mu.Lock()
	_, ok := a.sessions[sessionName]
	a.mu.Unlock()

	if !ok {
		return fmt.Errorf("session %s not found", sessionName)
	}

	// Send /session new to Gemini CLI via ACP
	// This will keep existing .json files and create a new one
	return a.SendInput(sessionName, "/session new")
}

// SwitchWorkDir changes the working directory for an ACP session
func (a *ACPAdapter) SwitchWorkDir(sessionName, newWorkDir string) error {
	logger.WithFields(logrus.Fields{
		"session":      sessionName,
		"new_work_dir": newWorkDir,
	}).Info("switching-acp-work-dir")

	a.mu.Lock()
	sess, ok := a.sessions[sessionName]
	if !ok {
		a.mu.Unlock()
		return fmt.Errorf("session %s not found", sessionName)
	}
	startCmd := sess.startCmd
	// Use stdio as default, but we should probably detect if it was remote
	// For now, Gemini CLI is mostly used via stdio in clibot
	a.mu.Unlock()

	if err := a.DeleteSession(sessionName); err != nil {
		logger.WithField("error", err).Warn("failed-to-delete-session-during-switch")
	}

	return a.CreateSession(sessionName, newWorkDir, startCmd, "stdio://")
}

// ListSessions lists available Gemini history sessions for the project associated
// with this ACP session. It reads session-*.json files from ~/.gemini/tmp/{hash}/chats,
// the same directory Gemini CLI uses regardless of the transport mode.
func (a *ACPAdapter) ListSessions(sessionName string) ([]string, error) {
	a.mu.Lock()
	sess, ok := a.sessions[sessionName]
	var workDir string
	if ok {
		workDir = sess.workDir
	}
	a.mu.Unlock()

	if workDir == "" {
		return nil, fmt.Errorf("ACP session '%s' has no recorded work directory", sessionName)
	}

	return listGeminiSessionsByWorkDir(workDir)
}

// SwitchSession switches the Gemini CLI (running behind ACP) to a different
// history session by updating the session ID used for future prompt requests.
func (a *ACPAdapter) SwitchSession(sessionName, cliSessionID string) (string, error) {
	logger.WithFields(logrus.Fields{
		"session":     sessionName,
		"cli_session": cliSessionID,
	}).Info("switching-acp-gemini-session")

	a.mu.Lock()
	sess, ok := a.sessions[sessionName]
	var workDir string
	if ok {
		workDir = sess.workDir
	}
	a.mu.Unlock()

	if !ok {
		return "", fmt.Errorf("session %s not found", sessionName)
	}

	if workDir != "" {
		fullID, err := resolveFullSessionID(workDir, cliSessionID)
		if err != nil {
			return "", err
		}
		cliSessionID = fullID
	}

	a.mu.Lock()
	sess.sessionId = cliSessionID
	a.mu.Unlock()

	return getGeminiSessionContext(workDir, cliSessionID), nil
}

// ensureGeminiChatsDir ensures that the Gemini chats directory exists
// Gemini stores history in: ~/.gemini/tmp/{project_hash}/chats
func ensureGeminiChatsDir(workDir string) error {
	chatsDir, err := findGeminiChatsDir(workDir)
	if err != nil {
		return err
	}

	// Create directory with permissions 0755 (cross-platform)
	if err := os.MkdirAll(chatsDir, 0755); err != nil {
		return fmt.Errorf("failed to create gemini chats directory: %w", err)
	}

	logger.WithFields(logrus.Fields{
		"work_dir":  workDir,
		"chats_dir": chatsDir,
	}).Info("gemini-chats-directory-ensured")

	return nil
}

// CreateSession creates a new ACP session and starts connection
func (a *ACPAdapter) CreateSession(sessionName, workDir, startCmd, transportURL string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Check if session exists
	if sess, exists := a.sessions[sessionName]; exists {
		// If already active, return nil
		if sess.active {
			// Also check if process is really alive
			if a.isSessionActive(sess) {
				return nil
			}
			// Not really active, mark it as inactive and fall through to recreation
			logger.WithField("session", sessionName).Info("recreating-abandoned-session")
			sess.active = false
		}
		
		// Cleanup old inactive session resources if any
		if sess.cancel != nil {
			sess.cancel()
		}
	}

	// Ensure workDir is absolute
	absWorkDir, err := filepath.Abs(workDir)
	if err != nil {
		logger.WithField("error", err).Warn("failed-to-get-absolute-path-for-session")
		absWorkDir = workDir
	}

	// Create Gemini chats directory if using gemini CLI
	if strings.Contains(strings.ToLower(startCmd), "gemini") {
		if err := ensureGeminiChatsDir(absWorkDir); err != nil {
			logger.WithField("error", err).Warn("failed-to-create-gemini-chats-directory")
		}
	}

	// Parse transport URL
	transportType, address := parseTransportURL(transportURL)

	logger.WithFields(logrus.Fields{
		"session":   sessionName,
		"work_dir":  absWorkDir,
		"command":   startCmd,
		"transport": transportURL,
		"type":      transportType,
		"address":   address,
	}).Info("starting-acp-session")

	// Create connReady channel for this session
	connReady := make(chan struct{})

	// Create session context
	ctx, cancel := context.WithCancel(context.Background())
	
	// Initialize session object early so start methods can populate it
	sess := &acpSession{
		ctx:       ctx,
		cancel:    cancel,
		active:    true,
		connReady: connReady,
		workDir:   absWorkDir,
		startCmd:  startCmd,
	}
	a.sessions[sessionName] = sess

	// Start connection based on transport type
	var clientImpl *acpClient
	switch transportType {
	case ACPTransportStdio:
		clientImpl = &acpClient{
			adapter:      a,
			sessionName:  sessionName,
			activityChan: make(chan time.Time, 10), // Buffered channel to avoid blocking
		}
		sess.client = clientImpl
		err = a.startStdioServer(sess, absWorkDir, startCmd, clientImpl, connReady)
	case ACPTransportTCP, ACPTransportUnix:
		clientImpl = &acpClient{
			adapter:      a,
			sessionName:  sessionName,
			activityChan: make(chan time.Time, 10), // Buffered channel to avoid blocking
		}
		sess.client = clientImpl
		err = a.connectRemoteServer(sess, absWorkDir, transportType, address, clientImpl, connReady)
	default:
		err = fmt.Errorf("unsupported transport type: %s", transportType)
	}

	if err != nil {
		sess.active = false
		return err
	}

	logger.WithField("session", sessionName).Info("acp-session-created")

	return nil
}




// SendInput sends input to the ACP server
func (a *ACPAdapter) SendInput(sessionName, input string) error {
	a.mu.Lock()
	sess, ok := a.sessions[sessionName]
	if !ok {
		a.mu.Unlock()
		return fmt.Errorf("session %s not found", sessionName)
	}
	clientImpl := sess.client
	a.mu.Unlock()

	if !sess.active {
		return fmt.Errorf("session %s is inactive", sessionName)
	}

	// Wait for connection to be ready with timeout
	select {
	case <-sess.connReady:
		// Connection is ready
	case <-time.After(acpConnectionReadyTimeout):
		return fmt.Errorf("timeout waiting for ACP connection to be ready")
	case <-sess.ctx.Done():
		return fmt.Errorf("session cancelled while waiting for connection")
	}

	if sess.conn == nil {
		// Connection not established, mark session as inactive
		a.mu.Lock()
		sess.active = false
		a.mu.Unlock()
		return fmt.Errorf("ACP connection for session %s not established", sessionName)
	}

	logger.WithFields(logrus.Fields{
		"session":   sessionName,
		"sessionId": sess.sessionId,
		"input":     input,
	}).Debug("sending-input-to-acp-server")

	// Create cancellable context for this request
	// We'll use activity monitoring to cancel if idle for too long
	ctx, cancel := context.WithCancel(sess.ctx)
	defer cancel()

	// Start activity monitor goroutine
	monitorDone := make(chan struct{})
	monitorStopped := make(chan struct{})
	if clientImpl != nil {
		go func() {
			a.monitorActivity(sessionName, ctx, cancel, clientImpl, monitorDone)
			close(monitorStopped)
		}()
		defer func() {
			close(monitorDone) // Signal monitor to stop
			select {
			case <-monitorStopped: // Wait for monitor to exit
			case <-time.After(5 * time.Second):
				logger.WithField("session", sessionName).Warn("acp-monitor-goroutine-did-not-exit-in-time")
			}
		}()
	}

	// Send prompt using ACP Prompt method
	// Use sessionId if set, otherwise empty string (server may auto-create session)
	resp, err := sess.conn.Prompt(ctx, acp.PromptRequest{
		SessionId: acp.SessionId(sess.sessionId),
		Prompt: []acp.ContentBlock{
			{Text: &acp.ContentBlockText{Text: input}},
		},
	})
	if err != nil {
		// If error is not a timeout, mark session as inactive to prevent further requests
		if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
			logger.WithFields(logrus.Fields{
				"session": sessionName,
				"error":   err,
			}).Error("acp-connection-error-marking-session-inactive")

			a.mu.Lock()
			sess.active = false
			a.mu.Unlock()
		} else if errors.Is(err, context.Canceled) {
			return fmt.Errorf("request cancelled due to inactivity (idle timeout: %v)", a.config.IdleTimeout)
		}
		return fmt.Errorf("ACP prompt failed: %w", err)
	}

	logger.WithFields(logrus.Fields{
		"stop_reason": resp.StopReason,
	}).Debug("acp-prompt-completed")

	// After Prompt completes, send buffered response to user
	// Prompt is synchronous, so when it returns, all response chunks
	// should have been received via SessionUpdate callback
	if clientImpl != nil && clientImpl.responseBuf.Len() > 0 {
		clientImpl.mu.Lock()
		response := clientImpl.responseBuf.String()
		clientImpl.responseBuf.Reset()
		clientImpl.mu.Unlock()

		logger.WithFields(logrus.Fields{
			"session":         sessionName,
			"response_length": len(response),
		}).Info("acp-prompt-response-completed")

		// Send response to user via engine if not already fully streamed
		// NOTE: In streaming mode, some chunks might have been sent already.
		// However, for bots that don't support streaming (like current Telegram implementation),
		// we still rely on this final response for the full message.
		a.mu.Lock()
		engine := a.currentEngine
		a.mu.Unlock()

		if engine != nil && sessionName != "" {
			engine.SendResponseToSession(sessionName, response)
		}
	}

	return nil
}

// monitorActivity monitors activity from the agent and cancels the context if idle
// This allows long-running requests to complete as long as they're actively working,
// while cancelling truly hung requests that don't produce any output.
func (a *ACPAdapter) monitorActivity(sessionName string, baseCtx context.Context, cancelFunc context.CancelFunc, client *acpClient, done <-chan struct{}) {
	logger.WithFields(logrus.Fields{
		"session":      sessionName,
		"idle_timeout": a.config.IdleTimeout,
		"max_timeout":  a.config.MaxTotalTimeout,
	}).Debug("acp-activity-monitor-started")

	// Initialize last activity time
	client.lastActivityLock.Lock()
	client.lastActivityTime = time.Now()
	client.lastActivityLock.Unlock()

	// Create ticker for periodic checks
	ticker := time.NewTicker(acpActivityCheckInterval)
	defer ticker.Stop()

	// Track start time for max total timeout
	startTime := time.Now()

	for {
		select {
		case <-done:
			// Monitor is being stopped normally
			logger.WithField("session", sessionName).Debug("acp-activity-monitor-stopped")
			return

		case <-baseCtx.Done():
			// Session context was cancelled
			logger.WithField("session", sessionName).Debug("acp-activity-monitor-session-cancelled")
			return

		case activityTime := <-client.activityChan:
			// Received activity notification
			client.lastActivityLock.Lock()
			client.lastActivityTime = activityTime
			client.lastActivityLock.Unlock()

			logger.WithField("session", sessionName).
				Trace("acp-activity-received")

		case <-ticker.C:
			// Periodic check for timeout
			client.lastActivityLock.RLock()
			lastActivity := client.lastActivityTime
			client.lastActivityLock.RUnlock()

			idleTime := time.Since(lastActivity)
			totalTime := time.Since(startTime)

			// Check max total timeout (hard limit)
			if totalTime >= a.config.MaxTotalTimeout {
				logger.WithFields(logrus.Fields{
					"session":     sessionName,
					"total_time":  totalTime,
					"max_timeout": a.config.MaxTotalTimeout,
				}).Warn("acp-max-total-timeout-reached-cancelling")

				cancelFunc()
				return
			}

			// Check idle timeout
			if idleTime >= a.config.IdleTimeout {
				logger.WithFields(logrus.Fields{
					"session":      sessionName,
					"idle_time":    idleTime,
					"idle_timeout": a.config.IdleTimeout,
				}).Warn("acp-idle-timeout-reached-cancelling")

				cancelFunc()
				return
			}

			logger.WithFields(logrus.Fields{
				"session":    sessionName,
				"idle_time":  idleTime,
				"total_time": totalTime,
			}).Trace("acp-activity-check")
		}
	}
}

// DeleteSession terminates an ACP session
func (a *ACPAdapter) DeleteSession(sessionName string) error {
	a.mu.Lock()
	sess, exists := a.sessions[sessionName]
	if !exists {
		a.mu.Unlock()
		return fmt.Errorf("session %s not found", sessionName)
	}

	// Cancel context
	sess.cancel()
	sess.active = false

	// Remove from sessions map
	delete(a.sessions, sessionName)
	a.mu.Unlock()

	// Debug logging
	logger.WithFields(logrus.Fields{
		"session":  sessionName,
		"isRemote": sess.isRemote,
		"cmd":      sess.cmd != nil,
		"process":  sess.cmd != nil && sess.cmd.Process != nil,
	}).Debug("acp-delete-session-check")

	// For local stdio connections, terminate the ACP server process
	if !sess.isRemote && sess.cmd != nil && sess.cmd.Process != nil {
		if err := a.killProcess(sess); err != nil {
			return err
		}
	}

	// Close ACP connection
	if sess.conn != nil {
		<-sess.conn.Done()
	}

	logger.WithField("session", sessionName).Info("acp-session-deleted")

	return nil
}


// getSessionTitle attempts to extract a descriptive title for a session
func (a *ACPAdapter) getSessionTitle(workDir, sessionID string) (string, string) {
	if sessionID == "" {
		return "new-session", ""
	}

	// Try to find the JSON file for this session
	chatsDir, err := findGeminiChatsDir(workDir)
	if err != nil {
		return sessionID, sessionID
	}

	searchID := sessionID
	if len(searchID) >= 36 {
		searchID = searchID[:8] // first 8 hex chars of UUID
	}

	// Try direct match first
	sessionPath := filepath.Join(chatsDir, fmt.Sprintf("session-%s.json", sessionID))
	if _, err := os.Stat(sessionPath); err != nil {
		// Try middle-matching for Gemini's timestamped filenames
		matches, _ := filepath.Glob(filepath.Join(chatsDir, fmt.Sprintf("session-*%s*.json", searchID)))
		if len(matches) > 0 {
			sessionPath = matches[0]
			// Update IDs to use the full timestamped version for display
			sessionID = strings.TrimSuffix(strings.TrimPrefix(filepath.Base(sessionPath), "session-"), ".json")
		}
	}

	if _, err := os.Stat(sessionPath); err == nil {
		// Read file and parse first user message
		data, err := os.ReadFile(sessionPath)
		if err == nil {
			var sessionData struct {
				Title    string `json:"title"`
				Name     string `json:"name"`
				Messages []struct {
					Type    string      `json:"type"`
					Content interface{} `json:"content"`
				} `json:"messages"`
			}
			if err := json.Unmarshal(data, &sessionData); err == nil {
				// 1. Check for explicit title or name
				if sessionData.Title != "" {
					return sessionData.Title, sessionID
				}
				if sessionData.Name != "" {
					return sessionData.Name, sessionID
				}

				// 2. Extract from first user message
				for _, msg := range sessionData.Messages {
					msgType := strings.ToLower(msg.Type)
					if msgType == "user" || msgType == "human" {
						var contentStr string
						if s, ok := msg.Content.(string); ok {
							contentStr = s
						} else if arr, ok := msg.Content.([]interface{}); ok && len(arr) > 0 {
							if m, ok := arr[0].(map[string]interface{}); ok {
								if text, ok := m["text"].(string); ok {
									contentStr = text
								}
							}
						}
						
						// Extract first 30 chars of first user message as title
						title := strings.TrimSpace(contentStr)
						title = strings.ReplaceAll(title, "\n", " ")
						if len(title) > 30 {
							title = title[:27] + "..."
						}
						return fmt.Sprintf("%s: %s", sessionID, title), sessionID
					}
				}
			}
		}
	}

	return sessionID, sessionID
}

// GetSessionStats returns diagnostic stats for the session (e.g., context usage)
func (a *ACPAdapter) GetSessionStats(sessionName string) (map[string]interface{}, error) {
	a.mu.Lock()
	sess, ok := a.sessions[sessionName]
	a.mu.Unlock()

	if !ok {
		return nil, fmt.Errorf("session %s not found", sessionName)
	}

	stats := make(map[string]interface{})
	stats["work_dir"] = sess.workDir
	stats["usage_perc"] = sess.lastUsagePerc
	
	title, actualID := a.getSessionTitle(sess.workDir, sess.sessionId)
	stats["session_title"] = title
	stats["session_id"] = actualID
	
	return stats, nil
}

// Close cleans up ACP adapter resources
func (a *ACPAdapter) Close() error {
	a.mu.Lock()
	// Create a list of names to avoid concurrent map access issues
	var sessionNames []string
	for name := range a.sessions {
		sessionNames = append(sessionNames, name)
	}
	a.mu.Unlock()

	// Delete each session properly
	for _, name := range sessionNames {
		if err := a.DeleteSession(name); err != nil {
			logger.WithFields(logrus.Fields{
				"session": name,
				"error":   err,
			}).Warn("failed-to-delete-session-during-close")
		}
	}

	logger.Info("acp-adapter-closed")
	return nil
}

// startStdioServer starts ACP server as subprocess with stdio transport
func (a *ACPAdapter) startStdioServer(sess *acpSession, workDir, command string, clientImpl *acpClient, connReady chan struct{}) error {
	sessionName := sess.client.sessionName
	cmd := buildShellCommand(command)

	// Set working directory
	if workDir != "" {
		expandedDir, err := expandHome(workDir)
		if err != nil {
			return fmt.Errorf("invalid work_dir: %w", err)
		}
		cmd.Dir = expandedDir
	}

	// Set environment variables
	env := os.Environ()
	envVarCount := 0
	for k, v := range a.config.Env {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
		envVarCount++
	}
	cmd.Env = env

	logger.WithFields(logrus.Fields{
		"session":       sessionName,
		"env_var_count": envVarCount,
		"env_vars":      a.config.Env,
	}).Debug("acp-environment-variables-set")

	// Setup stdio pipes
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start process
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start ACP server: %w", err)
	}

	sess.cmd = cmd
	sess.isRemote = false

	// Create ACP client-side connection in goroutine to avoid blocking
	// IMPORTANT: NewClientSideConnection may block during handshake
	go func() {
		conn := acp.NewClientSideConnection(clientImpl, stdin, stdout)
		logger.Info("acp-client-connection-created")
		
		if conn != nil {
			sess.conn = conn
			conn.SetLogger(slog.Default())

			// Try to call NewSession to get sessionId with retries
			time.Sleep(acpConnectionStabilizeDelay)

			var newSessionResp acp.NewSessionResponse
			var err error
			maxRetries := acpNewSessionMaxRetries
			retryDelay := acpNewSessionRetryDelay

			for attempt := 1; attempt <= maxRetries; attempt++ {
				ctx, cancel := context.WithTimeout(context.Background(), acpNewSessionTimeout)

				logger.WithField("attempt", attempt).Info("acp-calling-new-session")
				newSessionResp, err = conn.NewSession(ctx, acp.NewSessionRequest{
					Cwd:        workDir,
					McpServers: []acp.McpServer{}, // Pass empty array instead of nil
				})
				cancel()

				if err == nil {
					// Success - save sessionId and break
					sess.sessionId = string(newSessionResp.SessionId)
					logger.WithFields(logrus.Fields{
						"session":   sessionName,
						"sessionId": sess.sessionId,
						"attempt":   attempt,
					}).Info("acp-session-id-saved")
					break
				}

				// Log failure
				logger.WithFields(logrus.Fields{
					"attempt": attempt,
					"error":   err,
				}).Warn("acp-new-session-attempt-failed")

				if attempt < maxRetries {
					logger.WithField("delay", retryDelay).Info("acp-retrying-new-session")
					time.Sleep(retryDelay)
				}
			}

			// If NewSession failed after all retries, mark session as inactive
			// so that SendInput won't attempt to use an empty sessionId.
			if err != nil {
				logger.WithFields(logrus.Fields{
					"session": sessionName,
					"error":   err,
				}).Error("acp-new-session-all-retries-failed-marking-inactive")
				a.mu.Lock()
				sess.active = false
				a.mu.Unlock()
			}

			// Signal that connection setup is complete (success or failure)
			close(connReady)
		}
	}()

	// Log stderr for debugging
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := stderr.Read(buf)
			if n > 0 {
				logger.WithField("stream", "stderr").Debug(string(buf[:n]))
			}
			if err != nil {
				break
			}
		}
	}()

	logger.WithFields(logrus.Fields{
		"pid":     cmd.Process.Pid,
		"session": sessionName,
	}).Info("acp-stdio-server-started")

	return nil
}

// connectRemoteServer connects to a remote ACP server via TCP or Unix socket
func (a *ACPAdapter) connectRemoteServer(sess *acpSession, workDir string, transportType ACPTransportType, address string, clientImpl *acpClient, connReady chan struct{}) error {
	sessionName := sess.client.sessionName
	if address == "" {
		return fmt.Errorf("address is required for %s transport", transportType)
	}

	// Determine network type
	var network string
	switch transportType {
	case ACPTransportTCP:
		network = "tcp"
	case ACPTransportUnix:
		network = "unix"
	default:
		return fmt.Errorf("unsupported transport: %s", transportType)
	}

	// Connect to remote server with timeout
	connNet, err := net.DialTimeout(network, address, acpDialTimeout)
	if err != nil {
		return fmt.Errorf("failed to connect to %s server at %s: %w", transportType, address, err)
	}

	sess.isRemote = true

	// Create ACP client-side connection in goroutine to avoid blocking
	// IMPORTANT: NewClientSideConnection may block during handshake
	go func() {
		conn := acp.NewClientSideConnection(clientImpl, connNet, connNet)
		logger.Info("acp-client-connection-created")

		if conn != nil {
			sess.conn = conn
			conn.SetLogger(slog.Default())

			// Try to call NewSession to get sessionId with retries
			time.Sleep(acpConnectionStabilizeDelay)

			var newSessionResp acp.NewSessionResponse
			var err error
			maxRetries := acpNewSessionMaxRetries
			retryDelay := acpNewSessionRetryDelay

			for attempt := 1; attempt <= maxRetries; attempt++ {
				ctx, cancel := context.WithTimeout(context.Background(), acpNewSessionTimeout)

				logger.WithField("attempt", attempt).Info("acp-calling-new-session")
				newSessionResp, err = conn.NewSession(ctx, acp.NewSessionRequest{
					Cwd:        workDir,
					McpServers: []acp.McpServer{}, // Pass empty array instead of nil
				})
				cancel()

				if err == nil {
					// Success - save sessionId and break
					sess.sessionId = string(newSessionResp.SessionId)
					logger.WithFields(logrus.Fields{
						"session":   sessionName,
						"sessionId": sess.sessionId,
						"attempt":   attempt,
					}).Info("acp-session-id-saved")
					break
				}

				// Log failure
				logger.WithFields(logrus.Fields{
					"attempt": attempt,
					"error":   err,
				}).Warn("acp-new-session-attempt-failed")

				if attempt < maxRetries {
					logger.WithField("delay", retryDelay).Info("acp-retrying-new-session")
					time.Sleep(retryDelay)
				}
			}

			// If NewSession failed after all retries, mark session as inactive
			// so that SendInput won't attempt to use an empty sessionId.
			if err != nil {
				logger.WithFields(logrus.Fields{
					"session": sessionName,
					"error":   err,
				}).Error("acp-new-session-all-retries-failed-marking-inactive")
				a.mu.Lock()
				sess.active = false
				a.mu.Unlock()
			}

			// Signal that connection setup is complete (success or failure)
			close(connReady)
		}
	}()

	logger.WithFields(logrus.Fields{
		"network": network,
		"address": address,
		"session": sessionName,
	}).Info("acp-remote-connected")

	return nil
}

// ========== acp.Client Interface Implementation ==========

// ReadTextFile handles file read requests from agent
func (c *acpClient) ReadTextFile(ctx context.Context, params acp.ReadTextFileRequest) (acp.ReadTextFileResponse, error) {
	return acp.ReadTextFileResponse{}, fmt.Errorf("file operations not implemented")
}

// WriteTextFile handles file write requests from agent
func (c *acpClient) WriteTextFile(ctx context.Context, params acp.WriteTextFileRequest) (acp.WriteTextFileResponse, error) {
	return acp.WriteTextFileResponse{}, fmt.Errorf("file operations not implemented")
}

// RequestPermission handles permission requests from agent
func (c *acpClient) RequestPermission(ctx context.Context, params acp.RequestPermissionRequest) (acp.RequestPermissionResponse, error) {
	// Auto-approve all permissions for now
	var optionID acp.PermissionOptionId
	if len(params.Options) > 0 {
		optionID = params.Options[0].OptionId
	}
	return acp.RequestPermissionResponse{
		Outcome: acp.NewRequestPermissionOutcomeSelected(optionID),
	}, nil
}

// SessionUpdate receives session updates from agent
func (c *acpClient) SessionUpdate(ctx context.Context, params acp.SessionNotification) error {
	// Send activity notification to monitor (non-blocking)
	// Use select with timeout to prevent blocking if monitor is not running
	select {
	case c.activityChan <- time.Now():
		logger.WithField("session", c.sessionName).Trace("acp-activity-notification-sent")
	case <-time.After(100 * time.Millisecond):
		// Monitor might not be running or channel is blocked
		logger.WithField("session", c.sessionName).Debug("acp-activity-notification-timeout-monitor-not-running")
	}

	// Log session update (contains AI responses)
	logger.WithFields(logrus.Fields{
		"session_id":   params.SessionId,
		"session_name": c.sessionName,
		"update":       params.Update,
	}).Debug("acp-session-update")

	// Save sessionId if this is the first update
	c.adapter.mu.Lock()
	if sess, exists := c.adapter.sessions[c.sessionName]; exists {
		if sess.sessionId == "" {
			sess.sessionId = string(params.SessionId)
			logger.WithFields(logrus.Fields{
				"session_name": c.sessionName,
				"session_id":   sess.sessionId,
			}).Info("acp-session-id-saved")
		}
	}
	c.adapter.mu.Unlock()

	// Handle different update types
	switch {
	case params.Update.AgentMessageChunk != nil:
		// Agent is sending a response (streaming)
		if params.Update.AgentMessageChunk.Content.Text != nil {
			chunk := params.Update.AgentMessageChunk.Content.Text.Text
			logger.WithField("chunk", chunk).Debug("acp-agent-chunk")

			// CONTEXT MONITORING: Parse /stats model output
			// Expected format: "... X% context used ..."
			if strings.Contains(chunk, "context used") {
				// Use a regex to find the percentage
				re := regexp.MustCompile(`(\d+)%\s+context used`)
				matches := re.FindStringSubmatch(chunk)
				if len(matches) > 1 {
					perc, _ := strconv.ParseFloat(matches[1], 64)
					c.adapter.mu.Lock()
					if sess, ok := c.adapter.sessions[c.sessionName]; ok {
						sess.lastUsagePerc = perc
						logger.WithFields(logrus.Fields{
							"session": c.sessionName,
							"usage":   perc,
						}).Info("captured-context-usage-percentage")

						// Auto-reset if usage > limit (threshold 0.6 = 60%)
						if perc/100.0 >= c.adapter.contextUsageLimit {
							// Notify user before reset
							if c.adapter.currentEngine != nil {
								msg := fmt.Sprintf("⚠️ Context usage has reached %.0f%%. Automatically switching to a new session to maintain performance...", perc)
								c.adapter.currentEngine.SendResponseToSession(c.sessionName, msg)
							}

							// Trigger reset in goroutine to not block update
							go c.adapter.ResetSession(c.sessionName)
						}
					}
					c.adapter.mu.Unlock()
				}
			}

			c.mu.Lock()
			c.responseBuf.WriteString(chunk)
			c.mu.Unlock()
		}
	case params.Update.ToolCall != nil:
		logger.WithFields(logrus.Fields{
			"tool_call_id": params.Update.ToolCall.ToolCallId,
		}).Debug("acp-tool-call")
	case params.Update.Plan != nil:
		logger.WithField("plan", params.Update.Plan).Debug("acp-agent-plan")
	}

	return nil
}

// CreateTerminal handles terminal creation requests
func (c *acpClient) CreateTerminal(ctx context.Context, params acp.CreateTerminalRequest) (acp.CreateTerminalResponse, error) {
	return acp.CreateTerminalResponse{}, fmt.Errorf("terminal operations not implemented")
}

// KillTerminalCommand handles terminal kill requests
func (c *acpClient) KillTerminalCommand(ctx context.Context, params acp.KillTerminalCommandRequest) (acp.KillTerminalCommandResponse, error) {
	return acp.KillTerminalCommandResponse{}, fmt.Errorf("terminal operations not implemented")
}

// TerminalOutput handles terminal output requests
func (c *acpClient) TerminalOutput(ctx context.Context, params acp.TerminalOutputRequest) (acp.TerminalOutputResponse, error) {
	return acp.TerminalOutputResponse{}, fmt.Errorf("terminal operations not implemented")
}

// ReleaseTerminal handles terminal release requests
func (c *acpClient) ReleaseTerminal(ctx context.Context, params acp.ReleaseTerminalRequest) (acp.ReleaseTerminalResponse, error) {
	return acp.ReleaseTerminalResponse{}, fmt.Errorf("terminal operations not implemented")
}

// WaitForTerminalExit handles terminal wait requests
func (c *acpClient) WaitForTerminalExit(ctx context.Context, params acp.WaitForTerminalExitRequest) (acp.WaitForTerminalExitResponse, error) {
	return acp.WaitForTerminalExitResponse{}, fmt.Errorf("terminal operations not implemented")
}
