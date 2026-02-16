// Package cli provides adapters for AI-powered CLI tools.
package cli

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/creack/pty"
	"github.com/keepmind9/clibot/internal/logger"
	"github.com/sirupsen/logrus"
)

// PTY adapter platform compatibility:
//
// Linux/macOS:
//   - Full PTY support using pseudo-terminals
//   - Process group termination works correctly (kills children)
//   - All features fully supported
//
// Windows (Two Options):
//
//   Option 1: WSL 2 (Recommended) ⭐
//   - Real Linux kernel with full PTY support
//   - Perfect UTF-8 and ANSI support
//   - All Unix tools work (vim, tmux, screen, etc.)
//   - Requires: WSL 2 installation
//   - Configuration: pty_type: "wsl" or pty_type: "auto"
//   - Distribution: Ubuntu, Debian, docker-desktop, etc.
//
//   Option 2: ConPTY (Windows Native)
//   - Windows 10 1809+ (build 17763, October 2018)
//   - No additional installation required
//   - Supported by github.com/creack/pty v1.1.18+
//   - Limitations: UTF-8 issues, partial ANSI support
//   - Configuration: pty_type: "conpty"
//
// Windows Recommendations:
//   1. For development or complex tools: Use WSL 2
//   2. For simple tools or quick testing: Use ConPTY
//   3. Always use Windows Terminal (not cmd.exe)
//
// Example Configurations:
//   # Windows with WSL 2 (Recommended)
//   sessions:
//     - name: "wsl-claude"
//       cli_type: "pty"
//       pty_type: "wsl"
//       distro: "Ubuntu"
//       start_cmd: "claude"
//
//   # Windows with ConPTY (Simple)
//   sessions:
//     - name: "win-claude"
//       cli_type: "pty"
//       pty_type: "conpty"
//       start_cmd: "claude"
//
//   # Auto-detect (Prefers WSL if available)
//   sessions:
//     - name: "auto-claude"
//       cli_type: "pty"
//       pty_type: "auto"
//       start_cmd: "claude"

// PTYAdapter implements CLIAdapter using a pseudo-terminal (PTY).
// This adapter is useful for controlling any standard CLI application
// that runs in a terminal.
type PTYAdapter struct {
	mu       sync.Mutex
	sessions map[string]*ptySession
	config   PTYAdapterConfig
	engine   Engine // Engine reference for sending responses
}

// PTYAdapterConfig holds configuration for the PTY adapter.
// PTYType defines the PTY backend type
type PTYType string

const (
	PTYTypeAuto   PTYType = "auto"   // Auto-detect based on OS (WSL on Windows, ConPTY otherwise)
	PTYTypeConPTY PTYType = "conpty" // Windows ConPTY (native)
	PTYTypeWSL    PTYType = "wsl"    // WSL 2 (uses Linux PTY internally)
)

// PTYAdapterConfig holds configuration for the PTY adapter.
type PTYAdapterConfig struct {
	// PTYType specifies which PTY backend to use
	// - "auto" (default): Automatically choose based on environment
	//   - On Windows: prefer WSL if available, otherwise use ConPTY
	//   - On Linux/macOS: use native PTY
	// - "conpty": Force Windows ConPTY (Windows 10 1809+)
	// - "wsl": Force WSL 2 (requires WSL installation)
	PTYType PTYType `yaml:"pty_type"`

	// Distro specifies the WSL distribution to use (only for pty_type: wsl)
	// Examples: "Ubuntu", "Ubuntu-22.04", "Debian", "docker-desktop", etc.
	// If empty, uses WSL's default distribution
	Distro string `yaml:"distro"`

	// Env specifies environment variables to be set for the command.
	Env map[string]string `yaml:"env"`
}

// ptySession stores the state for an active PTY session.
type ptySession struct {
	cmd     *exec.Cmd
	ptmx    *os.File
	active  bool
	workDir string
}

// NewPTYAdapter creates a new PTY adapter.
func NewPTYAdapter(config PTYAdapterConfig) (*PTYAdapter, error) {
	return &PTYAdapter{
		sessions: make(map[string]*ptySession),
		config:   config,
	}, nil
}

// SetEngine sets the engine reference for sending responses.
func (a *PTYAdapter) SetEngine(engine Engine) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.engine = engine
}

// CreateSession starts a new CLI process in a PTY.
// The transportURL is ignored by this adapter.
func (a *PTYAdapter) CreateSession(sessionName, workDir, startCmd, transportURL string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if _, exists := a.sessions[sessionName]; exists {
		return nil // Session already exists
	}

	// Determine which PTY backend to use
	ptyType := a.determinePTYType()

	logger.WithFields(logrus.Fields{
		"session":  sessionName,
		"pty_type": ptyType,
		"distro":   a.config.Distro,
	}).Info("pty-creating-session")

	if ptyType == PTYTypeWSL {
		return a.createWSLSession(sessionName, workDir, startCmd)
	}
	return a.createConPTYSession(sessionName, workDir, startCmd)
}

// determinePTYType determines which PTY backend to use based on config and environment
func (a *PTYAdapter) determinePTYType() PTYType {
	// If explicitly specified, use that
	if a.config.PTYType != "" && a.config.PTYType != PTYTypeAuto {
		return a.config.PTYType
	}

	// Auto-detect: prefer WSL on Windows if available
	if runtime.GOOS == "windows" {
		if a.isWSLAvailable() {
			return PTYTypeWSL
		}
		return PTYTypeConPTY
	}

	// On Linux/macOS, use native PTY (ConPTY on these systems is just regular PTY)
	return PTYTypeConPTY
}

// isWSLAvailable checks if WSL is available on the system
func (a *PTYAdapter) isWSLAvailable() bool {
	cmd := exec.Command("wsl.exe", "--version")
	return cmd.Run() == nil
}

// createWSLSession creates a session using WSL
func (a *PTYAdapter) createWSLSession(sessionName, workDir, startCmd string) error {
	distro := a.config.Distro
	if distro == "" {
		// Get default WSL distro
		cmd := exec.Command("wsl.exe", "--list", "--quiet")
		output, err := cmd.Output()
		if err == nil && len(output) > 0 {
			distro = string(output)
			// Remove trailing newline and spaces
			distro = strings.TrimSpace(distro)
		} else {
			return fmt.Errorf("failed to detect WSL distro: %w", err)
		}
	}

	// Build WSL command: wsl.exe -d <distro> -- <command>
	// The -- signals that what follows should be executed in Linux
	args := []string{"wsl.exe"}
	if distro != "" {
		args = append(args, "-d", distro)
	}
	args = append(args, "--", startCmd)

	cmd := exec.Command(args[0], args[1:]...)

	// Set working directory (Linux path)
	if workDir != "" {
		cmd.Dir = workDir
	}

	// Set environment variables
	env := os.Environ()
	for k, v := range a.config.Env {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	cmd.Env = env

	// Start the command in a PTY (WSL uses Linux PTY internally)
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return fmt.Errorf("failed to start WSL PTY: %w", err)
	}

	sess := &ptySession{
		cmd:     cmd,
		ptmx:    ptmx,
		active:  true,
		workDir: workDir,
	}
	a.sessions[sessionName] = sess

	// Start a goroutine to read from the PTY and send output to the engine
	go a.readPTYOutput(sessionName, sess)

	logger.WithFields(logrus.Fields{
		"session": sessionName,
		"distro":  distro,
	}).Info("wsl-session-created")

	return nil
}

// createConPTYSession creates a session using Windows ConPTY
func (a *PTYAdapter) createConPTYSession(sessionName, workDir, startCmd string) error {
	cmd := buildShellCommand(startCmd)

	// Set working directory
	if workDir != "" {
		expandedDir, err := expandHome(workDir)
		if err != nil {
			return fmt.Errorf("invalid work_dir for pty: %w", err)
		}
		cmd.Dir = expandedDir
	}

	// Set environment variables
	env := os.Environ()
	for k, v := range a.config.Env {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	cmd.Env = env

	// Start the command in a PTY
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return fmt.Errorf("failed to start ConPTY: %w", err)
	}

	sess := &ptySession{
		cmd:     cmd,
		ptmx:    ptmx,
		active:  true,
		workDir: workDir,
	}
	a.sessions[sessionName] = sess

	// Start a goroutine to read from the PTY and send output to the engine
	go a.readPTYOutput(sessionName, sess)

	logger.WithField("session", sessionName).Info("conpty-session-created")

	return nil
}

// readPTYOutput is a shared goroutine for reading PTY output
func (a *PTYAdapter) readPTYOutput(sessionName string, sess *ptySession) {
	buf := make([]byte, 4096)
	for {
		// Check if session still exists before reading
		a.mu.Lock()
		_, sessionExists := a.sessions[sessionName]
		engine := a.engine
		a.mu.Unlock()

		if !sessionExists {
			// Session was deleted, exit goroutine
			break
		}

		n, err := sess.ptmx.Read(buf)
		if n > 0 {
			// We have output, send it to the engine
			if engine != nil {
				// Send without holding lock to prevent deadlock
				engine.SendResponseToSession(sessionName, string(buf[:n]))
			}
		}
		if err != nil {
			// Error reading from PTY, session is likely dead
			a.mu.Lock()
			a.terminateSession(sessionName)
			a.mu.Unlock()
			break
		}
	}
}

// SendInput sends the given input string to the PTY.
func (a *PTYAdapter) SendInput(sessionName, input string) error {
	a.mu.Lock()
	sess, ok := a.sessions[sessionName]
	a.mu.Unlock()

	if !ok || !sess.active {
		return fmt.Errorf("pty session %s not found or inactive", sessionName)
	}

	// Write input to the PTY. Appending a newline is crucial
	// to simulate the user pressing Enter.
	data := []byte(input + "\n")
	n, err := sess.ptmx.Write(data)
	if err != nil {
		return fmt.Errorf("failed to write to pty: %w", err)
	}
	if n != len(data) {
		return fmt.Errorf("partial write to pty: %d/%d bytes", n, len(data))
	}
	return nil
}

// IsSessionAlive checks if the PTY session's process is still running.
func (a *PTYAdapter) IsSessionAlive(sessionName string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	sess, ok := a.sessions[sessionName]
	if !ok || !sess.active {
		return false
	}

	// If the process has exited, ProcessState will be non-nil.
	// This is a reliable way to check if the process is still running.
	return sess.cmd != nil && sess.cmd.ProcessState == nil
}

// HandleHookData is not used in PTY mode.
func (a *PTYAdapter) HandleHookData(data []byte) (string, string, string, error) {
	return "", "", "", fmt.Errorf("PTY mode does not use hook data")
}

// UseHook returns false, as PTY mode does not use hooks.
func (a *PTYAdapter) UseHook() bool {
	return false
}

// GetPollInterval returns a short duration, as PTY is interactive.
func (a *PTYAdapter) GetPollInterval() time.Duration {
	return 200 * time.Millisecond
}

// GetStableCount returns 1, as we don't need output stability checks.
func (a *PTYAdapter) GetStableCount() int {
	return 1
}

// GetPollTimeout returns a long timeout.
func (a *PTYAdapter) GetPollTimeout() time.Duration {
	return 1 * time.Hour
}

// CloseAllSessions terminates all active PTY sessions.
func (a *PTYAdapter) CloseAllSessions() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	for name := range a.sessions {
		if err := a.terminateSession(name); err != nil {
			// Log the error but continue trying to close other sessions
		}
	}
	return nil
}

// terminateSession kills the process and closes the PTY file for a given session.
func (a *PTYAdapter) terminateSession(sessionName string) error {
	// This helper function should be called within a lock.
	sess, ok := a.sessions[sessionName]
	if !ok {
		return nil // Session already gone
	}

	if sess.cmd != nil && sess.cmd.Process != nil {
		// Kill the entire process group to ensure child processes are also terminated.
		if err := killProcessGroup(sess.cmd); err != nil {
			// Fallback to killing just the process if killing the group fails.
			_ = sess.cmd.Process.Kill()
		}
	}

	if sess.ptmx != nil {
		sess.ptmx.Close()
	}

	delete(a.sessions, sessionName)
	return nil
}
