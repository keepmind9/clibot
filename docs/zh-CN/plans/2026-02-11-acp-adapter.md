# ACP Adapter Implementation Plan

## Overview

Implement a new `ACPAdapter` as a fourth `cli_type` option alongside existing `claude`, `gemini`, and `opencode` adapters.

## Motivation

| cli_type | Mode | Config | Deployment |
|----------|------|--------|------------|
| claude/gemini/opencode | hook | Required | Local only |
| claude/gemini/opencode | polling | None | Local only |
| **acp** | ACP protocol | Simple | Local/Remote ✨ |

**ACP Advantages:**
- **Standardized protocol** - JSON-RPC 2.0, works with multiple AI CLI tools
- **Unified adapter** - Single adapter for all ACP-compatible tools
- **Remote support** - TCP/Unix socket for remote deployment
- **Cleaner architecture** - No tmux dependency

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│  clibot                                                      │
│  ┌─────────────────────────────────────────────────────┐   │
│  │  CLI Adapters (cli_type)                            │   │
│  │  ├── claude   → ClaudeAdapter  (hook/polling)       │   │
│  │  ├── gemini   → GeminiAdapter  (hook/polling)       │   │
│  │  ├── opencode → OpenCodeAdapter (hook/polling)      │   │
│  │  └── acp      → ACPAdapter      (ACP protocol) ✨   │   │
│  └─────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
                          │
                  ACP Protocol (JSON-RPC 2.0)
                          │
┌─────────────────────────────────────────────────────────────┐
│  ACP Server (CLI Tools)                                     │
│  - claude-code-acp                                          │
│  - gemini --experimental-acp                                │
│  - opencode acp                                             │
└─────────────────────────────────────────────────────────────┘
```

## Configuration Design

### Global ACP Configuration

```yaml
cli_adapters:
  # Existing adapters
  claude:
    use_hook: true

  gemini:
    use_hook: true

  opencode:
    use_hook: true

  # New ACP adapter (same level as others)
  acp:
    # Transport type: stdio (default), tcp, unix
    transport: stdio

    # TCP address (when transport=tcp)
    address: localhost:9000

    # Unix socket path (when transport=unix)
    socket_path: /tmp/acp.sock

    # Request timeout
    request_timeout: "5m"

    # Environment variables for ACP server process
    env:
      ANTHROPIC_API_KEY: ${ANTHROPIC_API_KEY}
      GEMINI_API_KEY: ${GEMINI_API_KEY}
```

### Session Configuration

```yaml
sessions:
  # Traditional Claude adapter (hook mode)
  - name: "claude-hook-session"
    cli_type: "claude"
    work_dir: "~/projects/demo"
    start_cmd: "claude"

  # ACP adapter with claude
  - name: "claude-acp-session"
    cli_type: "acp"
    work_dir: "~/projects/demo"
    start_cmd: "claude-code-acp"

  # ACP adapter with gemini
  - name: "gemini-acp-session"
    cli_type: "acp"
    work_dir: "~/projects/demo"
    start_cmd: "gemini --experimental-acp"

  # ACP adapter with opencode
  - name: "opencode-acp-session"
    cli_type: "acp"
    work_dir: "~/projects/demo"
    start_cmd: "opencode acp"
```

**Key Points:**
- `cli_type: "acp"` → Use ACPAdapter
- `start_cmd` → Specific ACP server command per session
- Same ACPAdapter can handle different tools via different `start_cmd`

## Implementation Plan

### Phase 1: Dependencies

```bash
go get github.com/coder/acp-go-sdk
```

### Phase 2: Type Definitions

**File:** `internal/cli/acp_types.go`

```go
package cli

import "time"

// ACPTransport defines the transport type for ACP communication
type ACPTransport string

const (
    ACPTransportStdio  ACPTransport = "stdio"
    ACPTransportTCP    ACPTransport = "tcp"
    ACPTransportUnix   ACPTransport = "unix"
)

// ACPAdapterConfig configuration for ACP adapter
type ACPAdapterConfig struct {
    Transport       ACPTransport
    Address         string        // For TCP
    SocketPath      string        // For Unix socket
    RequestTimeout  time.Duration
    Env             map[string]string
}
```

### Phase 3: ACP Adapter Implementation

**File:** `internal/cli/acp.go`

```go
package cli

import (
    "context"
    "fmt"
    "os"
    "os/exec"
    "sync"

    "github.com/coder/acp-go-sdk"
    "github.com/keepmind9/clibot/internal/logger"
    "github.com/sirupsen/logrus"
)

// ACPAdapter implements CLIAdapter using Agent Client Protocol
type ACPAdapter struct {
    config   ACPAdapterConfig
    client   acp.Client
    cmd      *exec.Cmd
    mu       sync.Mutex
    sessions map[string]*acpSession
}

type acpSession struct {
    ctx    context.Context
    cancel context.CancelFunc
    active bool
}

// NewACPAdapter creates a new ACP adapter
func NewACPAdapter(config ACPAdapterConfig) (*ACPAdapter, error) {
    return &ACPAdapter{
        config:   config,
        sessions: make(map[string]*acpSession),
    }, nil
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

// CreateSession creates a new ACP session and starts the server
func (a *ACPAdapter) CreateSession(sessionName, workDir, startCmd string) error {
    a.mu.Lock()
    defer a.mu.Unlock()

    if _, exists := a.sessions[sessionName]; exists {
        return nil // Already exists
    }

    // Start ACP server process
    if err := a.startServer(sessionName, workDir, startCmd); err != nil {
        return err
    }

    // Create session context
    ctx, cancel := context.WithCancel(context.Background())
    a.sessions[sessionName] = &acpSession{
        ctx:    ctx,
        cancel: cancel,
        active: true,
    }

    logger.WithFields(logrus.Fields{
        "session":  sessionName,
        "work_dir": workDir,
        "command":  startCmd,
    }).Info("acp-session-created")

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

    ctx, cancel := context.WithTimeout(sess.ctx, a.config.RequestTimeout)
    defer cancel()

    logger.WithFields(logrus.Fields{
        "session": sessionName,
        "input":   input,
    }).Debug("sending-input-to-acp-server")

    // Call ACP method
    result, err := a.client.SendRequest(ctx, "ask", map[string]interface{}{
        "prompt":  input,
        "session": sessionName,
    })
    if err != nil {
        return fmt.Errorf("ACP request failed: %w", err)
    }

    logger.WithField("result", result).Debug("acp-response-received")
    return nil
}

// startServer starts the ACP server subprocess
func (a *ACPAdapter) startServer(sessionName, workDir, command string) error {
    if a.client != nil {
        return nil // Already running
    }

    logger.WithFields(logrus.Fields{
        "session": sessionName,
        "command": command,
    }).Info("starting-acp-server")

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

    // Create ACP client with stdio transport
    a.client = acp.NewClient(acp.ClientConfig{
        Stdin:  stdin,
        Stdout: stdout,
        Stderr: stderr,
    })

    logger.WithField("pid", cmd.Process.Pid).Info("acp-server-started")

    return nil
}
```

### Phase 4: Configuration Loading

**File:** `internal/config/config.go`

Add ACP config loading:

```go
type CLIAdaptersConfig struct {
    Claude   ClaudeAdapterConfig   `yaml:"claude"`
    Gemini   GeminiAdapterConfig   `yaml:"gemini"`
    OpenCode OpenCodeAdapterConfig `yaml:"opencode"`
    ACP      ACPAdapterConfig      `yaml:"acp"` // Add this
}
```

### Phase 5: Engine Integration

**File:** `internal/engine/engine.go` (or similar)

Update adapter factory:

```go
func NewCLIAdapter(cliType string, config interface{}) (cli.CLIAdapter, error) {
    switch cliType {
    case "acp":
        acpConfig, ok := config.(cli.ACPAdapterConfig)
        if !ok {
            return nil, fmt.Errorf("invalid ACP config type")
        }
        return cli.NewACPAdapter(acpConfig)
    case "claude":
        // ... existing code
    case "gemini":
        // ... existing code
    case "opencode":
        // ... existing code
    default:
        return nil, fmt.Errorf("unknown CLI type: %s", cliType)
    }
}
```

### Phase 6: Tests

**File:** `internal/cli/acp_test.go`

```go
package cli

import (
    "testing"
    "time"
)

func TestACPAdapterConfigDefaults(t *testing.T) {
    // Test default config values
}

func TestACPAdapterUseHook(t *testing.T) {
    adapter, _ := NewACPAdapter(ACPAdapterConfig{
        RequestTimeout: 5 * time.Minute,
    })

    if adapter.UseHook() {
        t.Error("ACP adapter should not use hook mode")
    }
}

func TestACPAdapterSessionLifecycle(t *testing.T) {
    // Test session creation, lifecycle
}

// Integration tests (require actual ACP server)
func TestACPAdapterIntegration(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test")
    }
    // Test with real ACP server
}
```

### Phase 7: Documentation

**Files to update:**

1. **README.md / README_zh.md**
   - Add ACP adapter documentation
   - Explain when to use ACP vs hook/polling

2. **configs/config.yaml**
   - Add ACP configuration section with comments

3. **Internal documentation**
   - Update architecture diagrams
   - Document ACP protocol usage

## Implementation Order

1. ✅ Add dependency (`go get github.com/coder/acp-go-sdk`)
2. ✅ Create type definitions (`internal/cli/acp_types.go`)
3. ✅ Implement ACPAdapter (`internal/cli/acp.go`)
4. ✅ Update configuration loading (`internal/config/config.go`)
5. ✅ Update engine factory (`internal/engine/engine.go`)
6. ✅ Write unit tests (`internal/cli/acp_test.go`)
7. ✅ Write integration tests
8. ✅ Update documentation

## File Structure Summary

```
internal/cli/
├── interface.go         # CLIAdapter interface (existing)
├── base.go            # BaseAdapter (existing)
├── claude.go          # ClaudeAdapter (existing)
├── gemini.go          # GeminiAdapter (existing)
├── opencode.go        # OpenCodeAdapter (existing)
├── acp_types.go       # NEW: ACP type definitions
├── acp.go             # NEW: ACPAdapter implementation
├── acp_test.go        # NEW: ACP adapter tests
└── ...
```

## Risks & Mitigations

| Risk | Mitigation |
|------|------------|
| SDK API changes | Pin version, monitor releases |
| Subprocess management | Robust cleanup, signal handling |
| Session isolation | Use session IDs in ACP requests |
| Transport complexity | Start with stdio only |
| Missing SDK features | Implement fallback logic |

## Future Enhancements

- [ ] TCP transport support
- [ ] Unix socket transport support
- [ ] Connection pooling for remote servers
- [ ] Health check / heartbeat mechanism
- [ ] Automatic reconnection on failure
- [ ] Metrics (request latency, success rate)
- [ ] Support for streaming responses

## References

- [ACP Specification](https://github.com/agentclientprotocol/agent-client-protocol)
- [coder/acp-go-sdk](https://github.com/coder/acp-go-sdk)
- [Claude Code ACP Adapter](https://github.com/zed-industries/claude-code-acp)
- [JetBrains ACP Documentation](https://www.jetbrains.com/help/ai-assistant/acp.html)
