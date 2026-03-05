package cli

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/coder/acp-go-sdk"
	"github.com/keepmind9/clibot/internal/logger"
	"github.com/sirupsen/logrus"
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

// ACPAdapter implements CLIAdapter using Agent Client Protocol
type ACPAdapter struct {
	config        ACPAdapterConfig
	conn          *acp.ClientSideConnection
	cmd           *exec.Cmd
	mu            sync.Mutex
	sessions      map[string]*acpSession
	isRemote      bool       // Tracks if connection is remote (tcp/unix) vs local (stdio)
	currentEngine Engine     // Engine reference for sending responses
	currentClient *acpClient // Reference to current client for response buffer access
}

type acpSession struct {
	ctx       context.Context
	cancel    context.CancelFunc
	active    bool
	connReady chan struct{} // Closed when connection is ready for this session
	sessionId string        // ACP session ID from server
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
		config:   config,
		sessions: make(map[string]*acpSession),
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
	defer a.mu.Unlock()

	sess, ok := a.sessions[sessionName]
	return ok && sess.active
}

// ensureGeminiChatsDir ensures that the Gemini chats directory exists
// Gemini stores history in: ~/.gemini/tmp/{project_hash}/chats
func ensureGeminiChatsDir(workDir string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	projectHash := computeProjectHash(workDir)
	chatsDir := filepath.Join(homeDir, ".gemini", "tmp", projectHash, "chats")

	// Create directory with permissions 0755 (cross-platform)
	if err := os.MkdirAll(chatsDir, 0755); err != nil {
		return fmt.Errorf("failed to create gemini chats directory: %w", err)
	}

	logger.WithFields(logrus.Fields{
		"work_dir":  workDir,
		"chats_dir": chatsDir,
	}).Info("gemini-chats-directory-created")

	return nil
}

// CreateSession creates a new ACP session and starts connection
func (a *ACPAdapter) CreateSession(sessionName, workDir, startCmd, transportURL string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if _, exists := a.sessions[sessionName]; exists {
		return nil // Already exists
	}

	// Create Gemini chats directory if using gemini CLI
	if strings.Contains(strings.ToLower(startCmd), "gemini") {
		if err := ensureGeminiChatsDir(workDir); err != nil {
			logger.WithField("error", err).Warn("failed-to-create-gemini-chats-directory")
		}
	}

	// Parse transport URL
	transportType, address := parseTransportURL(transportURL)

	logger.WithFields(logrus.Fields{
		"session":   sessionName,
		"work_dir":  workDir,
		"command":   startCmd,
		"transport": transportURL,
		"type":      transportType,
		"address":   address,
	}).Info("starting-acp-session")

	// Create connReady channel for this session
	connReady := make(chan struct{})

	// Start connection based on transport type
	var err error
	var clientImpl *acpClient
	switch transportType {
	case ACPTransportStdio:
		clientImpl = &acpClient{
			adapter:      a,
			sessionName:  sessionName,
			activityChan: make(chan time.Time, 10), // Buffered channel to avoid blocking
		}
		err = a.startStdioServer(sessionName, workDir, startCmd, clientImpl, connReady)
	case ACPTransportTCP, ACPTransportUnix:
		clientImpl = &acpClient{
			adapter:      a,
			sessionName:  sessionName,
			activityChan: make(chan time.Time, 10), // Buffered channel to avoid blocking
		}
		err = a.connectRemoteServer(sessionName, workDir, transportType, address, clientImpl, connReady)
	default:
		err = fmt.Errorf("unsupported transport type: %s", transportType)
	}

	if err != nil {
		return err
	}

	// Save client reference for accessing response buffer
	a.currentClient = clientImpl

	// Create session context
	ctx, cancel := context.WithCancel(context.Background())
	a.sessions[sessionName] = &acpSession{
		ctx:       ctx,
		cancel:    cancel,
		active:    true,
		connReady: connReady,
	}

	logger.WithField("session", sessionName).Info("acp-session-created")

	return nil
}

// SendInput sends input to the ACP server
func (a *ACPAdapter) SendInput(sessionName, input string) error {
	a.mu.Lock()
	sess, ok := a.sessions[sessionName]
	clientImpl := a.currentClient
	a.mu.Unlock()

	if !ok || !sess.active {
		return fmt.Errorf("session %s not found or inactive", sessionName)
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

	if a.conn == nil {
		// Connection not established, mark session as inactive
		a.mu.Lock()
		if sess, exists := a.sessions[sessionName]; exists {
			sess.active = false
		}
		a.mu.Unlock()
		return fmt.Errorf("ACP connection not established")
	}

	// Reload session to get latest state (including sessionId if set)
	a.mu.Lock()
	sess, _ = a.sessions[sessionName]
	a.mu.Unlock()

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
	resp, err := a.conn.Prompt(ctx, acp.PromptRequest{
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
			if sess, exists := a.sessions[sessionName]; exists {
				sess.active = false
			}
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
		}).Info("acp-sending-complete-response")

		// Send response to user via engine
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
	defer a.mu.Unlock()

	sess, exists := a.sessions[sessionName]
	if !exists {
		return fmt.Errorf("session %s not found", sessionName)
	}

	// Cancel context
	sess.cancel()
	sess.active = false

	// Remove from sessions map
	delete(a.sessions, sessionName)

	// Debug logging
	logger.WithFields(logrus.Fields{
		"session":  sessionName,
		"isRemote": a.isRemote,
		"cmd":      a.cmd != nil,
		"process":  a.cmd != nil && a.cmd.Process != nil,
	}).Debug("acp-delete-session-check")

	// For local stdio connections, terminate the ACP server process
	// Note: ACPAdapter currently only supports one active process (a.cmd)
	// When deleting a session in stdio mode, we need to kill the process
	// and close the connection since there's only one process for all sessions
	if !a.isRemote && a.cmd != nil && a.cmd.Process != nil {
		logger.WithField("session", sessionName).Info("killing-acp-process")

		var killErr error
		if runtime.GOOS == "windows" {
			// Windows: Process.Kill() terminates the process tree
			killErr = a.cmd.Process.Kill()
		} else {
			// Unix/Linux/macOS: Kill entire process group using negative PID
			// The Setpgid: true in buildShellCommand ensures the process
			// is the process group leader, so -pid kills the entire group
			killErr = syscall.Kill(-a.cmd.Process.Pid, syscall.SIGKILL)
		}

		if killErr != nil {
			logger.WithField("error", killErr).Warn("failed-to-kill-acp-process")
		}

		// Wait for process to exit
		a.cmd.Wait()
		a.cmd = nil
	}

	// Close ACP connection
	if a.conn != nil {
		<-a.conn.Done()
		a.conn = nil
	}

	logger.WithField("session", sessionName).Info("acp-session-deleted")

	return nil
}

// Close cleans up ACP adapter resources
func (a *ACPAdapter) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Cancel all sessions
	for name := range a.sessions {
		sess := a.sessions[name]
		sess.cancel()
		sess.active = false
	}

	// Wait for ACP connection to close
	if a.conn != nil {
		<-a.conn.Done()
	}

	// Terminate ACP server process or close network connection
	if a.isRemote {
		// For remote connections, just close connection
		logger.Info("acp-remote-connection-closed")
	} else {
		// For local stdio, terminate process
		if a.cmd != nil && a.cmd.Process != nil {
			if err := a.cmd.Process.Kill(); err != nil {
				logger.WithField("error", err).Warn("failed-to-kill-acp-process")
			}
			// Wait for process to exit
			a.cmd.Wait()
		}
	}

	a.sessions = make(map[string]*acpSession)
	a.conn = nil
	a.cmd = nil

	logger.Info("acp-adapter-closed")
	return nil
}

// startStdioServer starts ACP server as subprocess with stdio transport
func (a *ACPAdapter) startStdioServer(sessionName, workDir, command string, clientImpl *acpClient, connReady chan struct{}) error {
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

	a.cmd = cmd
	a.isRemote = false

	// Create ACP client-side connection in goroutine to avoid blocking
	// IMPORTANT: NewClientSideConnection may block during handshake
	go func() {
		a.conn = acp.NewClientSideConnection(clientImpl, stdin, stdout)
		logger.Info("acp-client-connection-created")
		// Set logger for connection in goroutine to avoid blocking
		if a.conn != nil {
			a.conn.SetLogger(slog.Default())

			// Try to call NewSession to get sessionId with retries
			time.Sleep(acpConnectionStabilizeDelay)

			var newSessionResp acp.NewSessionResponse
			var err error
			maxRetries := acpNewSessionMaxRetries
			retryDelay := acpNewSessionRetryDelay

			for attempt := 1; attempt <= maxRetries; attempt++ {
				ctx, cancel := context.WithTimeout(context.Background(), acpNewSessionTimeout)

				logger.WithField("attempt", attempt).Info("acp-calling-new-session")
				newSessionResp, err = a.conn.NewSession(ctx, acp.NewSessionRequest{
					Cwd:        workDir,
					McpServers: []acp.McpServer{}, // Pass empty array instead of nil
				})
				cancel()

				if err == nil {
					// Success - save sessionId and break
					a.mu.Lock()
					if sess, exists := a.sessions[sessionName]; exists {
						sess.sessionId = string(newSessionResp.SessionId)
						logger.WithFields(logrus.Fields{
							"session":   sessionName,
							"sessionId": sess.sessionId,
							"attempt":   attempt,
						}).Info("acp-session-id-saved")
					}
					a.mu.Unlock()
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

			// Signal that connection is ready (regardless of NewSession success)
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
func (a *ACPAdapter) connectRemoteServer(sessionName string, workDir string, transportType ACPTransportType, address string, clientImpl *acpClient, connReady chan struct{}) error {
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
	conn, err := net.DialTimeout(network, address, acpDialTimeout)
	if err != nil {
		return fmt.Errorf("failed to connect to %s server at %s: %w", transportType, address, err)
	}

	a.isRemote = true

	// Create ACP client-side connection in goroutine to avoid blocking
	// IMPORTANT: NewClientSideConnection may block during handshake
	go func() {
		a.conn = acp.NewClientSideConnection(clientImpl, conn, conn)
		logger.Info("acp-client-connection-created")
		// Set logger for connection in goroutine to avoid blocking
		if a.conn != nil {
			a.conn.SetLogger(slog.Default())

			// Try to call NewSession to get sessionId with retries
			time.Sleep(acpConnectionStabilizeDelay)

			var newSessionResp acp.NewSessionResponse
			var err error
			maxRetries := acpNewSessionMaxRetries
			retryDelay := acpNewSessionRetryDelay

			for attempt := 1; attempt <= maxRetries; attempt++ {
				ctx, cancel := context.WithTimeout(context.Background(), acpNewSessionTimeout)

				logger.WithField("attempt", attempt).Info("acp-calling-new-session")
				newSessionResp, err = a.conn.NewSession(ctx, acp.NewSessionRequest{
					Cwd:        workDir,
					McpServers: []acp.McpServer{}, // Pass empty array instead of nil
				})
				cancel()

				if err == nil {
					// Success - save sessionId and break
					a.mu.Lock()
					if sess, exists := a.sessions[sessionName]; exists {
						sess.sessionId = string(newSessionResp.SessionId)
						logger.WithFields(logrus.Fields{
							"session":   sessionName,
							"sessionId": sess.sessionId,
							"attempt":   attempt,
						}).Info("acp-session-id-saved")
					}
					a.mu.Unlock()
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

			// Signal that connection is ready (regardless of NewSession success)
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
