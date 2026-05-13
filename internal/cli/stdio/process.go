package stdio

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/keepmind9/clibot/internal/logger"
	"github.com/sirupsen/logrus"
)

const (
	// maxLineSize is the maximum size of a single line from stdout (10MB).
	maxLineSize = 10 * 1024 * 1024

	// eventBufSize is the capacity of the event channel.
	eventBufSize = 64

	// shutdownGracePeriod is the time to wait after closing stdin
	// before sending SIGTERM.
	shutdownGracePeriod = 10 * time.Second

	// shutdownKillDelay is the time after SIGTERM before SIGKILL.
	shutdownKillDelay = 5 * time.Second
)

// StdioProcess manages a CLI subprocess with stdin/stdout pipes.
type StdioProcess struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	events chan Event

	spec CLISpec
	wg   sync.WaitGroup
	ctx  context.Context
	mu   sync.Mutex // protects stdin writes
}

// NewStdioProcess spawns a new CLI process and starts reading stdout.
func NewStdioProcess(ctx context.Context, spec CLISpec, opts StartOptions) (*StdioProcess, error) {
	binary := spec.Binary()
	args := spec.BuildArgs(opts)

	cmd := exec.CommandContext(ctx, binary, args...)

	// Process group management
	attrs := &syscall.SysProcAttr{}
	attrs.Setpgid = true
	cmd.SysProcAttr = attrs

	// Working directory
	if opts.WorkDir != "" {
		cmd.Dir = opts.WorkDir
	}

	// Environment: inherit current + merge extras
	cmd.Env = os.Environ()
	for k, v := range opts.Env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	// Create pipes
	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	p := &StdioProcess{
		cmd:    cmd,
		stdin:  stdinPipe,
		stdout: stdoutPipe,
		events: make(chan Event, eventBufSize),
		spec:   spec,
		ctx:    ctx,
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start %s: %w", spec.Name(), err)
	}

	logger.WithFields(logrus.Fields{
		"cli":    spec.Name(),
		"pid":    cmd.Process.Pid,
		"binary": binary,
		"args":   strings.Join(args, " "),
	}).Info("stdio-process-started")

	// Start reading stdout in a goroutine
	p.wg.Add(1)
	go p.readLoop()

	// Log stderr
	p.wg.Add(1)
	go p.stderrLoop(stderrPipe)

	return p, nil
}

// Events returns the channel of events from the CLI process.
func (p *StdioProcess) Events() <-chan Event {
	return p.events
}

// WriteInput writes a formatted user message to stdin.
// If FormatInput returns nil, the write is skipped (for CLIs that take prompt as arg).
func (p *StdioProcess) WriteInput(message string) error {
	data, err := p.spec.FormatInput(message)
	if err != nil {
		return fmt.Errorf("format input: %w", err)
	}
	if data == nil {
		return nil
	}
	return p.write(data)
}

// WritePermissionResponse writes a formatted permission response to stdin.
func (p *StdioProcess) WritePermissionResponse(requestID, optionID string) error {
	data, err := p.spec.FormatPermissionResponse(requestID, optionID)
	if err != nil {
		return fmt.Errorf("format permission response: %w", err)
	}
	return p.write(data)
}

// Close shuts down the process gracefully.
func (p *StdioProcess) Close() error {
	// Phase 1: Close stdin to signal EOF
	p.stdin.Close()

	// Phase 2: Wait for process to exit or send SIGTERM
	done := make(chan error, 1)
	go func() {
		done <- p.cmd.Wait()
	}()

	select {
	case err := <-done:
		p.wg.Wait()
		return err
	case <-time.After(shutdownGracePeriod):
		logger.WithFields(logrus.Fields{
			"cli": p.spec.Name(),
			"pid": p.cmd.Process.Pid,
		}).Warn("stdio-process-shutdown-timeout-sending-term")
		p.cmd.Process.Signal(syscall.SIGTERM)
	}

	select {
	case err := <-done:
		p.wg.Wait()
		return err
	case <-time.After(shutdownKillDelay):
		logger.WithFields(logrus.Fields{
			"cli": p.spec.Name(),
			"pid": p.cmd.Process.Pid,
		}).Warn("stdio-process-force-kill")
		p.cmd.Process.Kill()
		err := <-done
		p.wg.Wait()
		return err
	}
}

// Pid returns the process ID.
func (p *StdioProcess) Pid() int {
	if p.cmd.Process != nil {
		return p.cmd.Process.Pid
	}
	return 0
}

// write writes bytes to stdin with mutex protection.
func (p *StdioProcess) write(data []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	_, err := p.stdin.Write(append(data, '\n'))
	return err
}

// readLoop reads JSON lines from stdout and dispatches events.
// Closes the events channel when stdout reaches EOF.
func (p *StdioProcess) readLoop() {
	defer p.wg.Done()
	defer close(p.events)

	scanner := bufio.NewScanner(p.stdout)
	scanner.Buffer(make([]byte, 0, maxLineSize), maxLineSize)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		events, err := p.spec.ParseLine(line)
		if err != nil {
			logger.WithFields(logrus.Fields{
				"cli":  p.spec.Name(),
				"line": truncate(line, 200),
				"err":  err.Error(),
			}).Debug("stdio-parse-line-error")
			continue
		}

		for _, evt := range events {
			select {
			case p.events <- evt:
			case <-p.ctx.Done():
				return
			}
		}
	}

	if err := scanner.Err(); err != nil {
		select {
		case p.events <- Event{Type: EventError, Error: fmt.Errorf("stdout read error: %w", err)}:
		case <-p.ctx.Done():
		}
	}
}

// stderrLoop reads stderr for logging.
func (p *StdioProcess) stderrLoop(r io.Reader) {
	defer p.wg.Done()
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		logger.WithFields(logrus.Fields{
			"cli": p.spec.Name(),
			"pid": p.cmd.Process.Pid,
		}).Debug("stdio-stderr: " + scanner.Text())
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
