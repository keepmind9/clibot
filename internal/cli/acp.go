package cli

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/coder/acp-go-sdk"
	"github.com/keepmind9/clibot/internal/logger"
	"github.com/sirupsen/logrus"
)

// parseTransportURL parses a transport URL into transport type and address
// Formats:
//   - "" or "stdio://" → stdio with no address
//   - "tcp://host:port" → TCP with address
//   - "unix:///path" → Unix socket with path
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

// Engine defines the interface for sending responses to users
type Engine interface {
	SendToBot(platform, channel, message string)
	SendResponseToSession(sessionName, message string)
}

type acpSession struct {
	ctx    context.Context
	cancel context.CancelFunc
	active bool
}

// acpClient implements acp.Client interface for ACP callbacks
type acpClient struct {
	adapter     *ACPAdapter
	sessionName string // Session name for this client instance
	responseBuf strings.Builder
	mu          sync.Mutex // Protects responseBuf
}

// NewACPAdapter creates a new ACP adapter
func NewACPAdapter(config ACPAdapterConfig) (*ACPAdapter, error) {
	// Set default timeout if not specified
	if config.RequestTimeout == 0 {
		config.RequestTimeout = 5 * time.Minute
	}

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
	return 1 * time.Second
}

// GetStableCount returns stable count (not used in ACP mode)
func (a *ACPAdapter) GetStableCount() int {
	return 1
}

// GetPollTimeout returns request timeout
func (a *ACPAdapter) GetPollTimeout() time.Duration {
	return a.config.RequestTimeout
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

// CreateSession creates a new ACP session and starts the connection
func (a *ACPAdapter) CreateSession(sessionName, workDir, startCmd, transportURL string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if _, exists := a.sessions[sessionName]; exists {
		return nil // Already exists
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

	// Start connection based on transport type
	var err error
	var clientImpl *acpClient
	switch transportType {
	case ACPTransportStdio:
		clientImpl = &acpClient{adapter: a, sessionName: sessionName}
		err = a.startStdioServer(sessionName, workDir, startCmd, clientImpl)
	case ACPTransportTCP, ACPTransportUnix:
		clientImpl = &acpClient{adapter: a, sessionName: sessionName}
		err = a.connectRemoteServer(sessionName, transportType, address, clientImpl)
	default:
		err = fmt.Errorf("unsupported transport type: %s", transportType)
	}

	if err != nil {
		return err
	}

	// Save client reference for accessing response buffer
	a.mu.Lock()
	a.currentClient = clientImpl
	a.mu.Unlock()

	// Create session context
	ctx, cancel := context.WithCancel(context.Background())
	a.sessions[sessionName] = &acpSession{
		ctx:    ctx,
		cancel: cancel,
		active: true,
	}

	logger.WithField("session", sessionName).Info("acp-session-created")

	return nil
}

// SendInput sends input to the ACP server
func (a *ACPAdapter) SendInput(sessionName, input string) error {
	a.mu.Lock()
	sess, ok := a.sessions[sessionName]
	a.mu.Unlock()

	if !ok || !sess.active {
		return fmt.Errorf("session %s not found or inactive", sessionName)
	}

	if a.conn == nil {
		return fmt.Errorf("ACP connection not established")
	}

	ctx, cancel := context.WithTimeout(sess.ctx, a.config.RequestTimeout)
	defer cancel()

	logger.WithFields(logrus.Fields{
		"session": sessionName,
		"input":   input,
	}).Debug("sending-input-to-acp-server")

	// Send prompt using ACP Prompt method
	resp, err := a.conn.Prompt(ctx, acp.PromptRequest{
		Prompt: []acp.ContentBlock{
			{Text: &acp.ContentBlockText{Text: input}},
		},
	})
	if err != nil {
		return fmt.Errorf("ACP prompt failed: %w", err)
	}

	logger.WithFields(logrus.Fields{
		"stop_reason": resp.StopReason,
	}).Debug("acp-prompt-completed")

	// After Prompt completes, send the buffered response to user
	// Prompt is synchronous, so when it returns, all response chunks
	// should have been received via SessionUpdate callback
	a.mu.Lock()
	clientImpl := a.currentClient
	a.mu.Unlock()
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
		// For remote connections, just close the connection
		logger.Info("acp-remote-connection-closed")
	} else {
		// For local stdio, terminate the process
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
func (a *ACPAdapter) startStdioServer(sessionName, workDir, command string, clientImpl *acpClient) error {
	cmd := exec.Command("sh", "-c", command)

	// Set working directory
	if workDir != "" {
		expandedDir, err := expandHome(workDir)
		if err != nil {
			return fmt.Errorf("invalid work_dir: %w", err)
		}
		cmd.Dir = expandedDir
	}

	// Set environment
	env := os.Environ()
	for k, v := range a.config.Env {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	cmd.Env = env

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

	// Create ACP client-side connection
	a.conn = acp.NewClientSideConnection(clientImpl, stdin, stdout)

	// Set logger for connection
	a.conn.SetLogger(slog.Default())

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
func (a *ACPAdapter) connectRemoteServer(sessionName string, transportType ACPTransportType, address string, clientImpl *acpClient) error {
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
	conn, err := net.DialTimeout(network, address, 10*time.Second)
	if err != nil {
		return fmt.Errorf("failed to connect to %s server at %s: %w", transportType, address, err)
	}

	a.isRemote = true

	// Create ACP client-side connection with network connection
	// net.Conn implements io.ReadWriter, so we can pass conn directly
	a.conn = acp.NewClientSideConnection(clientImpl, conn, conn)

	// Set logger for connection
	a.conn.SetLogger(slog.Default())

	logger.WithFields(logrus.Fields{
		"network": network,
		"address": address,
		"session": sessionName,
	}).Info("acp-remote-connected")

	return nil
}

// ========== acp.Client Interface Implementation ==========

// ReadTextFile handles file read requests from the agent
func (c *acpClient) ReadTextFile(ctx context.Context, params acp.ReadTextFileRequest) (acp.ReadTextFileResponse, error) {
	return acp.ReadTextFileResponse{}, fmt.Errorf("file operations not implemented")
}

// WriteTextFile handles file write requests from the agent
func (c *acpClient) WriteTextFile(ctx context.Context, params acp.WriteTextFileRequest) (acp.WriteTextFileResponse, error) {
	return acp.WriteTextFileResponse{}, fmt.Errorf("file operations not implemented")
}

// RequestPermission handles permission requests from the agent
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

// SessionUpdate receives session updates from the agent
func (c *acpClient) SessionUpdate(ctx context.Context, params acp.SessionNotification) error {
	// Log session update (contains AI responses)
	logger.WithFields(logrus.Fields{
		"session_id":   params.SessionId,
		"session_name": c.sessionName,
		"update":       params.Update,
	}).Debug("acp-session-update")

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
