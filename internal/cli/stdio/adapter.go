package stdio

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/keepmind9/clibot/internal/logger"
	"github.com/sirupsen/logrus"
)

const (
	// defaultPermissionTimeout is the max time to wait for a permission response.
	defaultPermissionTimeout = 5 * time.Minute

	// defaultIdleTimeout is the max time with no activity before cancelling.
	defaultIdleTimeout = 10 * time.Minute
)

// StdioAdapterConfig holds configuration for stdio adapters.
type StdioAdapterConfig struct {
	PermissionTimeout time.Duration
	IdleTimeout       time.Duration
	Env               map[string]string
}

// PendingPermission holds a pending permission request waiting for user response.
type PendingPermission struct {
	Request   *PermissionRequest
	Session   string
	Timer     *time.Timer
	Responded bool
}

// StdioAdapter implements cli.CLIAdapter for stdio-based CLI tools.
// It supports multiple CLI tools via the CLISpec interface.
type StdioAdapter struct {
	spec     CLISpec
	config   StdioAdapterConfig
	engine   Engine
	mu       sync.Mutex
	sessions map[string]*stdioSession
}

type stdioSession struct {
	name        string
	process     *StdioProcess
	workDir     string
	env         map[string]string
	pendingPerm *PendingPermission
	permMu      sync.Mutex
	cancelCtx   context.CancelFunc
	sessionID   string // CLI-side session ID for resume support
	completed   bool   // True after first successful per-turn
}

// Engine interface matches cli.Engine — duplicated here to avoid
// circular import (this package is under internal/cli/).
type Engine interface {
	SendToBot(platform, channel, message string)
	SendResponseToSession(sessionName, message string)
	SendPermissionPrompt(sessionName, message string)
}

// NewStdioAdapter creates a new stdio adapter for the given CLISpec.
func NewStdioAdapter(spec CLISpec, config StdioAdapterConfig) *StdioAdapter {
	if config.PermissionTimeout == 0 {
		config.PermissionTimeout = defaultPermissionTimeout
	}
	if config.IdleTimeout == 0 {
		config.IdleTimeout = defaultIdleTimeout
	}
	return &StdioAdapter{
		spec:     spec,
		config:   config,
		sessions: make(map[string]*stdioSession),
	}
}

// SetEngine sets the engine reference for sending responses.
func (a *StdioAdapter) SetEngine(engine Engine) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.engine = engine
}

// CreateSession creates a new session and starts the CLI process.
func (a *StdioAdapter) CreateSession(sessionName, workDir, startCmd, transportURL string, env map[string]string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Idempotent: if session exists and process is alive, skip
	if sess, ok := a.sessions[sessionName]; ok && sess.process != nil {
		return nil
	}

	// Merge environment: adapter-level + session-level
	mergedEnv := make(map[string]string)
	for k, v := range a.config.Env {
		mergedEnv[k] = v
	}
	for k, v := range env {
		mergedEnv[k] = v
	}

	ctx, cancel := context.WithCancel(context.Background())

	opts := StartOptions{
		WorkDir: workDir,
		Env:     mergedEnv,
		Context: ctx,
	}

	proc, err := NewStdioProcess(ctx, a.spec, opts)
	if err != nil {
		cancel()
		return fmt.Errorf("failed to start %s: %w", a.spec.Name(), err)
	}

	sess := &stdioSession{
		name:      sessionName,
		process:   proc,
		workDir:   workDir,
		env:       mergedEnv,
		cancelCtx: cancel,
	}
	a.sessions[sessionName] = sess

	// Start event loop
	go a.eventLoop(sess)

	logger.WithFields(logrus.Fields{
		"cli":     a.spec.Name(),
		"session": sessionName,
		"workdir": workDir,
		"pid":     proc.Pid(),
	}).Info("stdio-session-created")

	return nil
}

// SendInput sends a user message to the CLI process.
// For PersistentMode: writes to stdin and waits for result events.
// For PerTurnMode: spawns a new process per message.
func (a *StdioAdapter) SendInput(sessionName, input string) error {
	a.mu.Lock()
	sess, ok := a.sessions[sessionName]
	a.mu.Unlock()

	if !ok {
		return fmt.Errorf("session %q not found", sessionName)
	}

	switch a.spec.Mode() {
	case PersistentMode:
		return a.sendPersistent(sess, input)
	case PerTurnMode:
		return a.sendPerTurn(sess, input)
	default:
		return fmt.Errorf("unknown stdio mode")
	}
}

// HandleHookData returns an error — stdio adapters don't use hooks.
func (a *StdioAdapter) HandleHookData(data []byte) (string, string, string, error) {
	return "", "", "", fmt.Errorf("stdio adapter does not support hooks")
}

// IsSessionAlive checks if the CLI process is still running.
func (a *StdioAdapter) IsSessionAlive(sessionName string) bool {
	a.mu.Lock()
	sess, ok := a.sessions[sessionName]
	a.mu.Unlock()

	if !ok || sess.process == nil {
		return false
	}
	return sess.process.Pid() > 0
}

// RespondPermission sends a permission response to the CLI process.
func (a *StdioAdapter) RespondPermission(sessionName, requestID, optionID string) error {
	a.mu.Lock()
	sess, ok := a.sessions[sessionName]
	a.mu.Unlock()

	if !ok {
		return fmt.Errorf("session %q not found", sessionName)
	}

	// Per-turn mode does not support permission prompts
	if a.spec.Mode() == PerTurnMode {
		return fmt.Errorf("permissions not supported in per-turn mode")
	}

	sess.permMu.Lock()
	defer sess.permMu.Unlock()

	if sess.pendingPerm == nil {
		return fmt.Errorf("no pending permission for session %q", sessionName)
	}

	// Stop the timeout timer
	if sess.pendingPerm.Timer != nil {
		sess.pendingPerm.Timer.Stop()
	}
	sess.pendingPerm.Responded = true
	sess.pendingPerm = nil

	if err := sess.process.WritePermissionResponse(requestID, optionID); err != nil {
		return fmt.Errorf("write permission response: %w", err)
	}

	logger.WithFields(logrus.Fields{
		"cli":        a.spec.Name(),
		"session":    sessionName,
		"request_id": requestID,
		"option_id":  optionID,
	}).Info("stdio-permission-responded")

	return nil
}

// GetPendingPermission returns the pending permission request for a session.
func (a *StdioAdapter) GetPendingPermission(sessionName string) *PendingPermission {
	a.mu.Lock()
	sess, ok := a.sessions[sessionName]
	a.mu.Unlock()

	if !ok {
		return nil
	}

	sess.permMu.Lock()
	defer sess.permMu.Unlock()
	return sess.pendingPerm
}

// Close shuts down all sessions and releases resources.
func (a *StdioAdapter) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	var errs []string
	for name, sess := range a.sessions {
		if sess.process != nil {
			if err := sess.process.Close(); err != nil {
				errs = append(errs, fmt.Sprintf("%s: %v", name, err))
			}
		}
		if sess.cancelCtx != nil {
			sess.cancelCtx()
		}
	}
	a.sessions = make(map[string]*stdioSession)

	if len(errs) > 0 {
		return fmt.Errorf("close errors: %s", strings.Join(errs, "; "))
	}
	return nil
}

// sendPersistent writes input to the persistent process's stdin.
func (a *StdioAdapter) sendPersistent(sess *stdioSession, input string) error {
	return sess.process.WriteInput(input)
}

// sendPerTurn spawns a new process for this message, collects output, and delivers response.
// Note: SendInput must not be called concurrently for the same session.
func (a *StdioAdapter) sendPerTurn(sess *stdioSession, input string) error {
	ctx, cancel := context.WithTimeout(context.Background(), a.config.IdleTimeout)
	defer cancel()

	opts := StartOptions{
		WorkDir:   sess.workDir,
		Env:       sess.env,
		Context:   ctx,
		Prompt:    input,
		Resume:    sess.completed,
		SessionID: sess.sessionID,
	}

	proc, err := NewStdioProcess(ctx, a.spec, opts)
	if err != nil {
		return fmt.Errorf("start per-turn process: %w", err)
	}

	// Write input (prompt via stdin)
	if err := proc.WriteInput(input); err != nil {
		proc.Close()
		return fmt.Errorf("write input: %w", err)
	}

	// Collect events until process exits — only use EventResult for content
	var result string
	for evt := range proc.Events() {
		switch evt.Type {
		case EventResult:
			if evt.Text != "" {
				result += evt.Text
			}
			if evt.Done {
				sess.completed = true
			}
		case EventError:
			logger.WithFields(logrus.Fields{
				"cli": a.spec.Name(),
				"err": evt.Error,
			}).Error("stdio-per-turn-error")
		}
		// Capture session ID for resume support
		if evt.SessionID != "" {
			sess.sessionID = evt.SessionID
		}
	}
	proc.Close()

	// Deliver response
	resp := strings.TrimSpace(result)
	if resp != "" {
		a.mu.Lock()
		engine := a.engine
		a.mu.Unlock()

		if engine != nil {
			engine.SendResponseToSession(sess.name, resp)
		}
	}

	return nil
}

// eventLoop reads events from the process and dispatches them.
// Only used for PersistentMode.
func (a *StdioAdapter) eventLoop(sess *stdioSession) {
	for evt := range sess.process.Events() {
		switch evt.Type {
		case EventPermission:
			a.handlePermissionEvent(sess, evt.Permission)

		case EventResult:
			if evt.Text == "" {
				continue
			}
			resp := strings.TrimSpace(evt.Text)
			if resp == "" {
				continue
			}

			a.mu.Lock()
			engine := a.engine
			a.mu.Unlock()

			if engine != nil {
				engine.SendResponseToSession(sess.name, resp)
			}

		case EventError:
			logger.WithFields(logrus.Fields{
				"cli":     a.spec.Name(),
				"session": sess.name,
				"err":     evt.Error,
			}).Error("stdio-event-error")
		}
	}

	logger.WithFields(logrus.Fields{
		"cli":     a.spec.Name(),
		"session": sess.name,
	}).Info("stdio-event-loop-ended")
}

// handlePermissionEvent processes a permission request from the CLI.
func (a *StdioAdapter) handlePermissionEvent(sess *stdioSession, perm *PermissionRequest) {
	sess.permMu.Lock()
	// Cancel any previous pending permission
	if sess.pendingPerm != nil && sess.pendingPerm.Timer != nil {
		sess.pendingPerm.Timer.Stop()
	}

	// Set up timeout auto-deny
	timer := time.AfterFunc(a.config.PermissionTimeout, func() {
		sess.permMu.Lock()
		defer sess.permMu.Unlock()

		if sess.pendingPerm == nil || sess.pendingPerm.Responded {
			return
		}

		// Auto-deny: look for option with ID "deny", fall back to last option
		var denyOpt *PermissionOption
		for i := range perm.Options {
			if perm.Options[i].ID == "deny" {
				denyOpt = &perm.Options[i]
				break
			}
		}
		if denyOpt == nil && len(perm.Options) > 0 {
			denyOpt = &perm.Options[len(perm.Options)-1]
		}
		if denyOpt != nil {
			logger.WithFields(logrus.Fields{
				"cli":        a.spec.Name(),
				"session":    sess.name,
				"request_id": perm.RequestID,
				"option":     denyOpt.Text,
			}).Warn("stdio-permission-timeout-auto-deny")

			sess.pendingPerm.Responded = true
			sess.pendingPerm = nil
			sess.process.WritePermissionResponse(perm.RequestID, denyOpt.ID)
		}
	})

	sess.pendingPerm = &PendingPermission{
		Request: perm,
		Session: sess.name,
		Timer:   timer,
	}
	sess.permMu.Unlock()

	// Notify engine
	a.mu.Lock()
	engine := a.engine
	a.mu.Unlock()

	if engine != nil {
		engine.SendPermissionPrompt(sess.name, perm.FormatOptions())
	}

	logger.WithFields(logrus.Fields{
		"cli":        a.spec.Name(),
		"session":    sess.name,
		"request_id": perm.RequestID,
		"tool":       perm.ToolName,
		"options":    len(perm.Options),
	}).Info("stdio-permission-requested")
}
