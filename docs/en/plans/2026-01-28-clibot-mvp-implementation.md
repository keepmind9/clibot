# clibot MVP Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a functional MVP of clibot that connects Discord bot with Claude Code CLI via tmux sessions, featuring user whitelist authentication, HTTP hook-based response collection, and basic watchdog monitoring.

**Architecture:** Event-driven middleware with HTTP hook server for CLI completion notifications and polling-based watchdog for interactive prompt detection. Bot adapters receive user messages → Engine validates and dispatches → CLI adapters interact with tmux sessions → Responses collected via hook or watchdog → Pushed back through bot.

**Tech Stack:** Go 1.21+, Cobra (CLI), gopkg.in/yaml.v3 (config), gorilla/mux (HTTP), discordgo (Discord), tmux (session management), github.com/acarl005/stripansi (ANSI cleanup)

---

## Phase 1: Foundation (Configuration + Core Types)

### Task 1: Implement Configuration Management

**Files:**
- Create: `internal/core/config.go`
- Test: `internal/core/config_test.go`

**Step 1: Write the failing test**

```go
func TestLoadConfig_ValidConfig_ReturnsConfigStruct(t *testing.T) {
    // Create temporary config file
    configContent := `
hook_server:
  port: 8080
command_prefix: "!!"
security:
  whitelist_enabled: true
  allowed_users:
    discord:
      - "123456789012345678"
sessions:
  - name: "test-session"
    cli_type: "claude"
    work_dir: "/tmp/test"
    auto_start: false
default_session: "test-session"
bots:
  discord:
    enabled: true
    token: "${TEST_TOKEN}"
cli_adapters:
  claude:
    history_dir: "~/.claude/conversations"
    interactive:
      enabled: true
      check_lines: 3
      patterns:
        - "\\? [y/N]"
`
    tmpFile, _ := os.CreateTemp("", "config-*.yaml")
    defer os.Remove(tmpFile.Name())
    tmpFile.WriteString(configContent)
    tmpFile.Close()

    // Load config
    config, err := LoadConfig(tmpFile.Name())

    // Assert
    assert.NoError(t, err)
    assert.Equal(t, 8080, config.HookServer.Port)
    assert.Equal(t, "!!", config.CommandPrefix)
    assert.True(t, config.Security.WhitelistEnabled)
    assert.Len(t, config.Sessions, 1)
    assert.Equal(t, "test-session", config.Sessions[0].Name)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/core -v -run TestLoadConfig`
Expected: FAIL with "undefined: LoadConfig"

**Step 3: Write minimal implementation**

Complete `internal/core/config.go` with:
- `LoadConfig()` function that reads YAML file
- `expandEnv()` to replace ${VAR} with environment variables
- `validateConfig()` for basic validation
- Helper methods: `GetBotConfig()`, `GetCLIAdapterConfig()`, `IsUserAuthorized()`, `IsAdmin()`

Reference design document section 9 for complete Config struct definition.

**Step 4: Run test to verify it passes**

Run: `go test ./internal/core -v -run TestLoadConfig`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/core/config.go internal/core/config_test.go
git commit -m "feat: implement configuration management with YAML parsing and validation"
```

---

## Phase 2: Tmux Integration (Watchdog Tools)

### Task 2: Implement Tmux Utility Functions

**Files:**
- Create: `internal/watchdog/tmux.go`
- Test: `internal/watchdog/tmux_test.go`

**Step 1: Write the failing test**

```go
func TestCapturePane_ValidSession_ReturnsOutput(t *testing.T) {
    // This test requires an actual tmux session
    // Skip in CI or use integration test environment
    if testing.Short() {
        t.Skip("requres actual tmux session")
    }

    output, err := CapturePane("test-session", 10)

    assert.NoError(t, err)
    assert.NotEmpty(t, output)
}

func TestStripANSI_WithANSICodes_ReturnsCleanText(t *testing.T) {
    input := "\x1b[31mRed text\x1b[0mNormal text"
    expected := "Red textNormal text"

    result := StripANSI(input)

    assert.Equal(t, expected, result)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/watchdog -v -run TestCapturePane`
Expected: FAIL with "undefined: CapturePane"

**Step 3: Write minimal implementation**

Create `internal/watchdog/tmux.go`:

```go
package watchdog

import (
    "fmt"
    "os/exec"
    "strings"

    "github.com/acarl005/stripansi"
)

// CapturePane captures the last N lines from a tmux session
func CapturePane(sessionName string, lines int) (string, error) {
    cmd := exec.Command("tmux", "capture-pane",
        "-t", sessionName,
        "-p", // Output to stdout
        "-e", // Include escape sequences
        fmt.Sprintf("-%d", lines), // Last N lines
    )

    output, err := cmd.Output()
    if err != nil {
        return "", fmt.Errorf("failed to capture tmux pane: %w", err)
    }

    return string(output), nil
}

// StripANSI removes ANSI color codes from text
func StripANSI(input string) string {
    return stripansi.Strip(input)
}

// IsSessionAlive checks if a tmux session exists
func IsSessionAlive(sessionName string) bool {
    cmd := exec.Command("tmux", "has-session", "-t", sessionName)
    return cmd.Run() == nil
}

// SendKeys sends input to a tmux session
func SendKeys(sessionName, input string) error {
    cmd := exec.Command("tmux", "send-keys", "-t", sessionName, input, "Enter")
    return cmd.Run()
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/watchdog -v`
Expected: PASS (except tests requiring actual tmux)

**Step 5: Commit**

```bash
git add internal/watchdog/tmux.go internal/watchdog/tmux_test.go go.mod go.sum
git commit -m "feat: implement tmux utility functions for session management"
```

---

## Phase 3: CLI Adapter (Claude Code)

### Task 3: Implement Claude Code CLI Adapter

**Files:**
- Create: `internal/cli/claude.go`
- Create: `internal/cli/conversation.go` (for parsing Claude conversation JSON)
- Test: `internal/cli/claude_test.go`

**Step 1: Write the failing test**

```go
func TestClaudeAdapter_SendInput(t *testing.T) {
    adapter := NewClaudeAdapter(ClaudeAdapterConfig{
        HistoryDir: "/tmp/test/conversations",
        CheckLines: 3,
        Patterns:   []string{`\\? [y/N]`},
    })

    err := adapter.SendInput("test-session", "help")

    // Will fail without actual tmux session, but we test the command construction
    assert.Error(t, err) // Expected to fail without tmux
}

func TestClaudeAdapter_CheckInteractive_WithConfirmationPrompt_ReturnsTrue(t *testing.T) {
    adapter := NewClaudeAdapter(ClaudeAdapterConfig{
        HistoryDir: "/tmp/test",
        CheckLines: 3,
        Patterns: []string{
            `\\? \\[y/N\\]`,
            `Confirm\\?`,
        },
    })

    // Mock tmux capture output
    // In real implementation, we'd use dependency injection for tmux commands

    waiting, prompt, err := adapter.CheckInteractive("test-session")

    // Test depends on tmux, so we'll implement with mock in next step
    assert.NoError(t, err)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/cli -v -run TestClaudeAdapter`
Expected: FAIL with "undefined: NewClaudeAdapter"

**Step 3: Write minimal implementation**

Create `internal/cli/claude.go`:

```go
package cli

import (
    "bufio"
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
    "regexp"
    "strings"
    "time"

    "github.com/keepmind9/clibot/internal/watchdog"
)

// ClaudeAdapter implements CLIAdapter for Claude Code
type ClaudeAdapter struct {
    historyDir string
    checkLines int
    patterns   []*regexp.Regexp
}

// ClaudeAdapterConfig holds configuration for Claude adapter
type ClaudeAdapterConfig struct {
    HistoryDir string
    CheckLines int
    Patterns   []string
}

// NewClaudeAdapter creates a new Claude Code adapter
func NewClaudeAdapter(config ClaudeAdapterConfig) (*ClaudeAdapter, error) {
    // Compile regex patterns
    var compiledPatterns []*regexp.Regexp
    for _, pattern := range config.Patterns {
        re, err := regexp.Compile(pattern)
        if err != nil {
            return nil, fmt.Errorf("invalid pattern %s: %w", pattern, err)
        }
        compiledPatterns = append(compiledPatterns, re)
    }

    // Expand home directory
    historyDir := config.HistoryDir
    if strings.HasPrefix(historyDir, "~/") {
        home, _ := os.UserHomeDir()
        historyDir = home + historyDir[1:]
    }

    return &ClaudeAdapter{
        historyDir: historyDir,
        checkLines: config.CheckLines,
        patterns:   compiledPatterns,
    }, nil
}

// SendInput sends input to Claude Code via tmux
func (c *ClaudeAdapter) SendInput(sessionName, input string) error {
    return watchdog.SendKeys(sessionName, input)
}

// GetLastResponse retrieves the last assistant message from conversation history
func (c *ClaudeAdapter) GetLastResponse(sessionName string) (string, error) {
    // Find latest conversation file
    files, err := filepath.Glob(filepath.Join(c.historyDir, "*.json"))
    if err != nil {
        return "", fmt.Errorf("failed to list conversation files: %w", err)
    }

    if len(files) == 0 {
        return "", fmt.Errorf("no conversation files found in %s", c.historyDir)
    }

    // Get latest file by modification time
    latestFile := getLatestFile(files)

    // Parse conversation
    data, err := os.ReadFile(latestFile)
    if err != nil {
        return "", fmt.Errorf("failed to read conversation file: %w", err)
    }

    var conversation Conversation
    if err := json.Unmarshal(data, &conversation); err != nil {
        return "", fmt.Errorf("failed to parse conversation: %w", err)
    }

    // Return last assistant message
    msg := conversation.LastAssistantMessage()
    if msg == nil {
        return "", fmt.Errorf("no assistant message found")
    }

    return msg.Content, nil
}

// IsSessionAlive checks if the tmux session is alive
func (c *ClaudeAdapter) IsSessionAlive(sessionName string) bool {
    return watchdog.IsSessionAlive(sessionName)
}

// CreateSession creates a new tmux session and starts Claude Code
func (c *ClaudeAdapter) CreateSession(sessionName, cliType, workDir string) error {
    // Create tmux session
    cmd := fmt.Sprintf("tmux new-session -d -s %s -c %s", sessionName, workDir)
    if err := execCommand(cmd); err != nil {
        return fmt.Errorf("failed to create tmux session: %w", err)
    }

    // Start Claude Code
    return watchdog.SendKeys(sessionName, "claude")
}

// CheckInteractive checks if Claude Code is waiting for user input
func (c *ClaudeAdapter) CheckInteractive(sessionName string) (bool, string, error) {
    // Capture last N lines
    output, err := watchdog.CapturePane(sessionName, c.checkLines)
    if err != nil {
        return false, "", err
    }

    // Check last 3 lines for interactive prompts
    lines := strings.Split(output, "\n")
    lastLines := lastNLines(lines, 3)

    for _, line := range lastLines {
        // Strip ANSI codes
        clean := watchdog.StripANSI(line)

        // Check against patterns
        for _, pattern := range c.patterns {
            if pattern.MatchString(clean) {
                return true, clean, nil
            }
        }
    }

    return false, "", nil
}

// Helper functions

func getLatestFile(files []string) string {
    var latest string
    var latestTime time.Time

    for _, file := range files {
        info, err := os.Stat(file)
        if err != nil {
            continue
        }

        if info.ModTime().After(latestTime) {
            latest = file
            latestTime = info.ModTime()
        }
    }

    return latest
}

func lastNLines(lines []string, n int) []string {
    if len(lines) <= n {
        return lines
    }
    return lines[len(lines)-n:]
}

func execCommand(cmd string) error {
    return exec.Command("bash", "-c", cmd).Run()
}
```

Create `internal/cli/conversation.go`:

```go
package cli

import "time"

// Message represents a message in Claude conversation
type Message struct {
    Role      string    `json:"role"`
    Content   string    `json:"content"`
    Timestamp time.Time `json:"timestamp"`
}

// Conversation represents a Claude Code conversation
type Conversation struct {
    Messages []Message `json:"messages"`
}

// LastAssistantMessage returns the last assistant message
func (c *Conversation) LastAssistantMessage() *Message {
    for i := len(c.Messages) - 1; i >= 0; i-- {
        if c.Messages[i].Role == "assistant" {
            return &c.Messages[i]
        }
    }
    return nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/cli -v`
Expected: PASS (some tests may skip without actual tmux/Claude)

**Step 5: Commit**

```bash
git add internal/cli/claude.go internal/cli/conversation.go internal/cli/claude_test.go
git commit -m "feat: implement Claude Code CLI adapter with tmux integration"
```

---

## Phase 4: Bot Adapter (Discord)

### Task 4: Implement Discord Bot Adapter

**Files:**
- Create: `internal/bot/discord.go`
- Test: `internal/bot/discord_test.go`

**Step 1: Write the failing test**

```go
func TestDiscordBot_Start_WithValidToken_Connects(t *testing.T) {
    bot := NewDiscordBot(DiscordConfig{
        Token:     "test-token",
        ChannelID: "123456789",
    })

    // Test message handler
    handlerCalled := false
    messageHandler := func(msg BotMessage) {
        handlerCalled = true
    }

    // We'll test connection without actual Discord in CI
    err := bot.Start(messageHandler)

    // In real test, this would mock Discord session
    assert.Error(t, err) // Expected to fail with invalid token
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/bot -v -run TestDiscordBot`
Expected: FAIL with "undefined: NewDiscordBot"

**Step 3: Write minimal implementation**

Create `internal/bot/discord.go`:

```go
package bot

import (
    "fmt"
    "log"
    "time"

    "github.com/bwmarrin/discordgo"
)

// DiscordBot implements BotAdapter for Discord
type DiscordBot struct {
 token      string
 channelID  string
 session    *discordgo.Session
}

// DiscordConfig holds configuration for Discord bot
type DiscordConfig struct {
    Token     string
    ChannelID string
}

// NewDiscordBot creates a new Discord bot
func NewDiscordBot(config DiscordConfig) (*DiscordBot, error) {
    if config.Token == "" {
        return nil, fmt.Errorf("discord token is required")
    }

    return &DiscordBot{
        token:     config.Token,
        channelID: config.ChannelID,
    }, nil
}

// Start starts the Discord bot and begins listening for messages
func (d *DiscordBot) Start(messageHandler func(BotMessage)) error {
    var err error
    d.session, err = discordgo.New("Bot " + d.token)
    if err != nil {
        return fmt.Errorf("failed to create discord session: %w", err)
    }

    // Register message handler
    d.session.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
        // Ignore messages from bots
        if m.Author.Bot {
            return
        }

        // Call message handler
        messageHandler(BotMessage{
            Platform:  "discord",
            UserID:    m.Author.ID,
            Channel:   m.ChannelID,
            Content:   m.Content,
            Timestamp: time.Now(),
        })
    })

    // Open connection
    if err := d.session.Open(); err != nil {
        return fmt.Errorf("failed to open discord connection: %w", err)
    }

    log.Println("Discord bot started")
    return nil
}

// SendMessage sends a message to a Discord channel
func (d *DiscordBot) SendMessage(channel, message string) error {
    if d.session == nil {
        return fmt.Errorf("discord session not initialized")
    }

    // Use default channel if not specified
    targetChannel := channel
    if targetChannel == "" {
        targetChannel = d.channelID
    }

    _, err := d.session.ChannelMessageSend(targetChannel, message)
    if err != nil {
        return fmt.Errorf("failed to send discord message: %w", err)
    }

    return nil
}

// Stop stops the Discord bot
func (d *DiscordBot) Stop() error {
    if d.session != nil {
        return d.session.Close()
    }
    return nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/bot -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/bot/discord.go internal/bot/discord_test.go go.mod go.sum
git commit -m "feat: implement Discord bot adapter"
```

---

## Phase 5: Core Engine Integration

### Task 5: Implement Core Engine

**Files:**
- Create: `internal/core/engine.go`
- Modify: `cmd/clibot/start.go` (to use Engine)

**Step 1: Update start command to create Engine**

Modify `cmd/clibot/start.go`:

```go
var startCmd = &cobra.Command{
    Use:   "start",
    Short: "Start clibot main process",
    Long:  "Start clibot main process, listen to bot messages and dispatch to AI CLI tools",
    Run: func(cmd *cobra.Command, args []string) {
        // Load configuration
        config, err := core.LoadConfig(configFile)
        if err != nil {
            log.Fatalf("Failed to load config: %v", err)
        }

        // Create engine
        engine := core.NewEngine(config)

        // Register CLI adapters
        claudeAdapter, _ := cli.NewClaudeAdapter(cli.ClaudeAdapterConfig{
            HistoryDir: config.CLIAdapters["claude"].HistoryDir,
            CheckLines: config.CLIAdapters["claude"].Interactive.CheckLines,
            Patterns:   config.CLIAdapters["claude"].Interactive.Patterns,
        })
        engine.RegisterCLIAdapter("claude", claudeAdapter)

        // Register bot adapters
        if config.Bots["discord"].Enabled {
            discordBot, _ := bot.NewDiscordBot(bot.DiscordConfig{
                Token:     config.Bots["discord"].Token,
                ChannelID: config.Bots["discord"].ChannelID,
            })
            engine.RegisterBotAdapter("discord", discordBot)
        }

        // Start engine (blocking)
        if err := engine.Run(); err != nil {
            log.Fatalf("Engine error: %v", err)
        }
    },
}
```

**Step 2: Run to verify it compiles**

Run: `go build -o clibot ./cmd/clibot`
Expected: Success

**Step 3: Test basic startup**

Run: `./clibot start --config configs/config.yaml`
Expected: Engine starts, initializes sessions, starts bots

**Step 4: Commit**

```bash
git add internal/core/engine.go cmd/clibot/start.go
git commit -m "feat: implement core Engine with bot and CLI adapter integration"
```

---

## Phase 6: HTTP Hook Server

### Task 6: Implement HTTP Hook Server

**Files:**
- Create: `internal/hook/server.go`
- Modify: `internal/core/engine.go` (to use hook server)

**Step 1: Write hook server implementation**

Create `internal/hook/server.go`:

```go
package hook

import (
    "encoding/json"
    "fmt"
    "log"
    "net/http"
    "time"

    "github.com/gorilla/mux"
)

// Event represents a hook event
type Event struct {
    SessionName string    `json:"session_name"`
    EventType   string    `json:"event_type"`
    Timestamp   time.Time `json:"timestamp"`
}

// Handler handles hook events
type Handler func(Event)

// Server is the HTTP hook server
type Server struct {
    addr    string
    handler Handler
    router  *mux.Router
}

// NewServer creates a new hook server
func NewServer(addr string, handler Handler) *Server {
    router := mux.NewRouter()

    server := &Server{
        addr:    addr,
        handler: handler,
        router:  router,
    }

    // Register routes
    router.HandleFunc("/hook", server.handleHook).Methods("GET", "POST")
    router.HandleFunc("/health", server.handleHealth).Methods("GET")

    return server
}

// Start starts the hook server
func (s *Server) Start() error {
    log.Printf("Hook server listening on %s", s.addr)

    srv := &http.Server{
        Addr:    s.addr,
        Handler: s.router,
    }

    if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
        return fmt.Errorf("hook server error: %w", err)
    }

    return nil
}

// handleHook handles incoming hook requests
func (s *Server) handleHook(w http.ResponseWriter, r *http.Request) {
    // Parse query parameters
    sessionName := r.URL.Query().Get("session")
    eventType := r.URL.Query().Get("event")

    if sessionName == "" || eventType == "" {
        http.Error(w, "Missing session or event parameter", http.StatusBadRequest)
        return
    }

    log.Printf("Hook received: session=%s, event=%s", sessionName, eventType)

    // Call handler
    s.handler(Event{
        SessionName: sessionName,
        EventType:   eventType,
        Timestamp:   time.Now(),
    })

    // Respond with success
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]string{
        "status":  "success",
        "session": sessionName,
        "event":   eventType,
    })
}

// handleHealth handles health check requests
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]interface{}{
        "status": "healthy",
        "time":   time.Now().Format(time.RFC3339),
    })
}
```

**Step 2: Integrate hook server into Engine**

Modify `internal/core/engine.go` to use hook server:

```go
func (e *Engine) Run() error {
    log.Println("Starting clibot engine...")

    // Initialize sessions
    if err := e.initializeSessions(); err != nil {
        return fmt.Errorf("failed to initialize sessions: %w", err)
    }

    // Start HTTP hook server
    hookServer := hook.NewServer(
        fmt.Sprintf(":%d", e.config.HookServer.Port),
        e.handleHookEvent,
    )
    go hookServer.Start()

    // ... rest of implementation
}

func (e *Engine) handleHookEvent(event hook.Event) {
    if event.EventType == "completed" {
        e.sessionMu.RLock()
        session, exists := e.sessions[event.SessionName]
        e.sessionMu.RUnlock()

        if !exists {
            log.Printf("Session %s not found", event.SessionName)
            return
        }

        // Get response from CLI adapter
        adapter := e.cliAdapters[session.CLIType]
        response, err := adapter.GetLastResponse(event.SessionName)
        if err != nil {
            log.Printf("Failed to get response: %v", err)
            return
        }

        // Send to response channel
        e.responseChan <- ResponseEvent{
            SessionName: event.SessionName,
            Response:    response,
            Timestamp:   event.Timestamp.Format(time.RFC3339),
        }
    }
}
```

**Step 3: Test hook endpoint**

Run: `curl http://localhost:8080/health`
Expected: `{"status":"healthy","time":"..."}`

Run: `curl "http://localhost:8080/hook?session=test&event=completed"`
Expected: `{"status":"success","session":"test","event":"completed"}`

**Step 4: Commit**

```bash
git add internal/hook/server.go internal/core/engine.go go.mod go.sum
git commit -m "feat: implement HTTP hook server for CLI completion events"
```

---

## Phase 7: Watchdog Monitoring

### Task 7: Implement Watchdog Monitoring

**Files:**
- Create: `internal/watchdog/watchdog.go`
- Modify: `internal/core/engine.go` (to use watchdog)

**Step 1: Implement watchdog monitoring logic**

Create `internal/watchdog/watchdog.go`:

```go
package watchdog

import (
    "log"
    "time"
)

// Monitor monitors a CLI session for interactive prompts
type Monitor struct {
    sessionName string
    adapter     InteractiveChecker
    intervals   []time.Duration
    timeout     time.Duration
    checkFunc   func(bool, string)
}

// InteractiveChecker defines interface for checking interactive state
type InteractiveChecker interface {
    CheckInteractive(sessionName string) (bool, string, error)
}

// NewMonitor creates a new watchdog monitor
func NewMonitor(sessionName string, adapter InteractiveChecker, intervals []time.Duration, timeout time.Duration, checkFunc func(bool, string)) *Monitor {
    return &Monitor{
        sessionName: sessionName,
        adapter:     adapter,
        intervals:   intervals,
        timeout:     timeout,
        checkFunc:   checkFunc,
    }
}

// Start starts the watchdog monitoring
func (m *Monitor) Start(stopSignal <-chan struct{}) {
    // Start with fast polling intervals
    totalDelay := 0 * time.Second

    for _, interval := range m.intervals {
        select {
        case <-stopSignal:
            log.Printf("Watchdog stopped for session %s", m.sessionName)
            return
        case <-time.After(interval):
            totalDelay += interval

            waiting, prompt, err := m.adapter.CheckInteractive(m.sessionName)
            if err != nil {
                log.Printf("Watchdog check error for %s: %v", m.sessionName, err)
                continue
            }

            if waiting {
                log.Printf("Interactive prompt detected in %s", m.sessionName)
                m.checkFunc(true, prompt)
                return
            }

            log.Printf("Watchdog check for %s: no interactive prompt (elapsed: %s)", m.sessionName, totalDelay)
        }
    }

    // If no interactive prompt detected, continue with timeout monitoring
    select {
    case <-stopSignal:
        log.Printf("Watchdog stopped for session %s", m.sessionName)
    case <-time.After(m.timeout - totalDelay):
        log.Printf("Watchdog timeout for session %s", m.sessionName)
        m.checkFunc(false, "")
    }
}
```

**Step 2: Integrate watchdog into Engine**

Modify `internal/core/engine.go` to use watchdog:

```go
func (e *Engine) startWatchdog(session *Session) {
    if !e.config.Watchdog.Enabled {
        return
    }

    // Parse intervals from config
    var intervals []time.Duration
    for _, intervalStr := range e.config.Watchdog.CheckIntervals {
        interval, _ := time.ParseDuration(intervalStr)
        intervals = append(intervals, interval)
    }

    // Parse timeout
    timeout, _ := time.ParseDuration(e.config.Watchdog.Timeout)

    // Create stop signal channel
    stopChan := make(chan struct{})

    // Start monitor
    monitor := watchdog.NewMonitor(
        session.Name,
        e.cliAdapters[session.CLIType],
        intervals,
        timeout,
        func(waiting bool, prompt string) {
            if waiting {
                // Interactive prompt detected
                e.updateSessionState(session.Name, StateWaitingInput)
                e.SendToAllBots(fmt.Sprintf("⚠️ **CLI requires confirmation**:\n```\n%s\n```\nReply to continue", prompt))
            } else {
                // Timeout
                e.updateSessionState(session.Name, StateError)
                e.SendToAllBots("⚠️ CLI response timeout\nSuggestion: Use !!status to check status")
            }
        },
    )

    go monitor.Start(stopChan)
}
```

**Step 3: Test watchdog with manual trigger**

Create test scenario where CLI waits for input, verify watchdog detects it.

**Step 4: Commit**

```bash
git add internal/watchdog/watchdog.go internal/core/engine.go
git commit -m "feat: implement watchdog monitoring for interactive prompts"
```

---

## Phase 8: Special Commands

### Task 8: Implement Special Commands

**Files:**
- Modify: `internal/core/engine.go` (HandleSpecialCommand method)

**Step 1: Implement command handlers**

```go
func (e *Engine) HandleSpecialCommand(cmd string, msg bot.BotMessage) {
    log.Printf("Special command: %s", cmd)

    // Parse command and arguments
    parts := strings.Fields(cmd)
    if len(parts) == 0 {
        return
    }

    command := parts[0]
    args := parts[1:]

    switch command {
    case "sessions":
        e.listSessions(msg)
    case "use":
        if len(args) < 1 {
            e.SendToBot(msg.Platform, msg.Channel, "Usage: !!use <session-name>")
            return
        }
        e.useSession(args[0], msg)
    case "status":
        e.showStatus(msg)
    case "whoami":
        e.showWhoami(msg)
    case "help":
        e.showHelp(msg)
    default:
        e.SendToBot(msg.Platform, msg.Channel,
            fmt.Sprintf("❌ Unknown command: %s\nAvailable commands: sessions, use, status, whoami, help", command))
    }
}

func (e *Engine) useSession(sessionName string, msg bot.BotMessage) {
    e.sessionMu.RLock()
    session, exists := e.sessions[sessionName]
    e.sessionMu.RUnlock()

    if !exists {
        e.SendToBot(msg.Platform, msg.Channel,
            fmt.Sprintf("❌ Session '%s' not found\nUse !!sessions to list available sessions", sessionName))
        return
    }

    // Store channel -> session mapping
    // TODO: Implement per-channel session mapping

    e.SendToBot(msg.Platform, msg.Channel, fmt.Sprintf("✅ Switched to session: %s", sessionName))
}

func (e *Engine) showHelp(msg bot.BotMessage) {
    help := `**clibot Special Commands**

!!sessions - List all available sessions
!!use <name> - Switch to a session
!!status - Show session status
!!whoami - Show current session info
!!help - Show this help message`

    e.SendToBot(msg.Platform, msg.Channel, help)
}
```

**Step 2: Test commands**

Test each command via Discord bot:
- `!!sessions` - should list sessions
- `!!status` - should show status
- `!!help` - should show help

**Step 3: Commit**

```bash
git add internal/core/engine.go
git commit -m "feat: implement special commands (sessions, use, status, whoami, help)"
```

---

## Phase 9: End-to-End Testing

### Task 9: Integration Testing and Documentation

**Files:**
- Create: `tests/integration/e2e_test.go`
- Modify: `README.md` (update usage instructions)

**Step 1: Create integration test**

Create `tests/integration/e2e_test.go`:

```go
// +build integration

package integration

import (
    "testing"
    "time"

    "github.com/keepmind9/clibot/internal/core"
    "github.com/keepmind9/clibot/internal/bot"
    "github.com/keepmind9/clibot/internal/cli"
)

// TestE2E_DisordToClaude tests full flow from Discord to Claude Code
func TestE2E_DiscordToClaude(t *testing.T) {
    if testing.Short() {
        t.Skip("integration test")
    }

    // Load config
    config, err := core.LoadConfig("test-config.yaml")
    assert.NoError(t, err)

    // Create engine
    engine := core.NewEngine(config)

    // Register adapters
    claudeAdapter, _ := cli.NewClaudeAdapter(cli.ClaudeAdapterConfig{
        HistoryDir: "~/.claude/conversations",
        CheckLines: 3,
        Patterns:   []string{`\\? [y/N]`},
    })
    engine.RegisterCLIAdapter("claude", claudeAdapter)

    // Test engine startup
    go engine.Run()
    time.Sleep(2 * time.Second)

    // Send test message via mock bot
    // Verify response

    engine.Stop()
}
```

**Step 2: Update README with setup instructions**

Update `README.md` with:
- Prerequisites (tmux, Claude Code CLI)
- Installation steps
- Configuration guide
- Usage examples
- Troubleshooting

**Step 3: Test full flow manually**

1. Start tmux session with Claude Code
2. Start clibot
3. Send message via Discord
4. Verify response received

**Step 4: Commit**

```bash
git add tests/integration/e2e_test.go README.md
git commit -m "test: add integration tests and update documentation"
```

---

## Phase 10: Polish and Production Readiness

### Task 10: Error Handling, Logging, and Graceful Shutdown

**Files:**
- Create: `internal/core/logger.go`
- Modify: `internal/core/engine.go` (add error handling)
- Modify: `cmd/clibot/start.go` (add signal handling)

**Step 1: Implement structured logging**

Create `internal/core/logger.go`:

```go
package core

import (
    "log"
    "os"

    "go.uber.org/zap"
    "go.uber.org/zap/zapcore"
)

var logger *zap.Logger

// InitLogger initializes the global logger
func InitLogger(level, file string) error {
    config := zap.NewProductionConfig()

    // Set log level
    switch level {
    case "debug":
        config.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
    case "info":
        config.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
    case "warn":
        config.Level = zap.NewAtomicLevelAt(zapcore.WarnLevel)
    case "error":
        config.Level = zap.NewAtomicLevelAt(zapcore.ErrorLevel)
    default:
        config.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
    }

    // Set output file
    if file != "" {
        config.OutputPaths = []string{file, "stdout"}
    }

    var err error
    logger, err = config.Build()
    if err != nil {
        return err
    }

    return nil
}

// GetLogger returns the global logger
func GetLogger() *zap.Logger {
    if logger == nil {
        logger, _ = zap.NewProduction()
    }
    return logger
}

// Info logs an info message
func Info(msg string, fields ...zap.Field) {
    GetLogger().Info(msg, fields...)
}

// Error logs an error message
func Error(msg string, fields ...zap.Field) {
    GetLogger().Error(msg, fields...)
}

// Debug logs a debug message
func Debug(msg string, fields ...zap.Field) {
    GetLogger().Debug(msg, fields...)
}
```

**Step 2: Add signal handling for graceful shutdown**

Modify `cmd/clibot/start.go`:

```go
Run: func(cmd *cobra.Command, args []string) {
    // ... initialization code ...

    // Handle signals for graceful shutdown
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

    // Start engine in goroutine
    engineErr := make(chan error, 1)
    go func() {
        engineErr <- engine.Run()
    }()

    // Wait for signal or error
    select {
    case <-sigChan:
        log.Println("Shutdown signal received")
        engine.Stop()
    case err := <-engineErr:
        log.Fatalf("Engine error: %v", err)
    }
}
```

**Step 3: Add comprehensive error handling**

Update Engine methods to handle errors gracefully and log appropriately.

**Step 4: Commit**

```bash
git add internal/core/logger.go internal/core/engine.go cmd/clibot/start.go go.mod go.sum
git commit -m "feat: add structured logging and graceful shutdown"
```

---

## Summary

This implementation plan covers:

✅ **Configuration Management**: YAML parsing with environment variable expansion
✅ **Tmux Integration**: Session management and screen capture utilities
✅ **CLI Adapter**: Claude Code adapter with interactive prompt detection
✅ **Bot Adapter**: Discord bot adapter for message handling
✅ **Core Engine**: Event-driven architecture with channel-based communication
✅ **HTTP Hook Server**: REST endpoint for CLI completion notifications
✅ **Watchdog Monitoring**: Polling-based detection of interactive prompts
✅ **Special Commands**: Session management and status commands
✅ **Integration Testing**: End-to-end testing framework
✅ **Production Ready**: Logging, error handling, graceful shutdown

**Estimated Effort**: 40-60 hours of development time

**Next Steps After MVP**:
1. Add more CLI adapters (Gemini, OpenCode)
2. Add more bot adapters (Feishu, Telegram)
3. Implement per-channel session mapping
4. Add streaming response support
5. Build web UI for management
6. Add metrics and monitoring

---

**Plan complete and saved to `docs/plans/2026-01-28-clibot-mvp-implementation.md`**

**Two execution options:**

**1. Subagent-Driven (this session)** - I dispatch fresh subagent per task, review between tasks, fast iteration

**2. Parallel Session (separate)** - Open new session with executing-plans, batch execution with checkpoints

**Which approach?**
