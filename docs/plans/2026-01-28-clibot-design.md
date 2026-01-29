# clibot 设计文档

**版本**: v0.4
**日期**: 2026-01-29
**状态**: 设计阶段（已整合安全白名单机制、长连接架构）

---

## 1. 项目概述

### 1.1 定位

clibot 是一个轻量级的中间层，将各种 IM 平台（飞书、Discord、Telegram 等）与 AI CLI 工具（Claude Code、Gemini CLI、OpenCode 等）连接起来，让用户可以通过聊天界面远程使用 AI 编程助手。

### 1.2 核心价值

- **随时随地**: 在手机、平板等设备上通过 IM 使用强大的桌面端 AI CLI
- **无需公网 IP**: 通过长连接架构，可在家庭/办公室网络部署，无需外网 IP
- **统一入口**: 一个 IM Bot 管理多个 AI CLI 工具，切换简单
- **灵活扩展**: 抽象接口设计，新增 CLI 或 Bot 只需实现接口
- **透明代理**: 绝大部分输入透传给 CLI，保持原生使用体验

### 1.3 使用场景

个人开发者在手机上收到紧急 bug 报告，打开飞书发送 `分析这段错误日志`，clibot 将命令透传给本地的 Claude Code，AI 分析完代码后，clibot 将结果推送回飞书，整个过程无需打开电脑。

---

## 2. 架构设计

### 2.1 整体架构

```
用户（飞书/Discord/Telegram）
    ↓
Bot 适配器（通过长连接接收消息）
    ↓
clibot 核心（Engine 调度）
    ↓
CLI 适配器（通过 tmux 与 CLI 交互）
    ↓
AI CLI 工具（在 tmux session 中运行）

连接方式（Bot → 平台）：
- Discord: WebSocket Gateway (wss://gateway.discord.gg)
- Telegram: Long Polling (HTTP GET /bot/getUpdates)
- 飞书: 待调研（优先支持 Discord + Telegram）
```

### 2.2 核心设计原则

- **抽象接口**: CLI 和 Bot 都通过接口定义，新接入只需实现接口
- **透明代理**: 除少数管理命令外，所有输入直接透传给 CLI
- **CLI 维度隔离**: 同一 CLI 串行执行，不同 CLI 可并发
- **同步调用**: 阻塞等待 CLI 执行完成（通过 hook 事件触发）
- **基于 tmux session**: CLI 运行在 tmux session 中，适配器通过 tmux 命令交互
- **长连接架构**: Bot 通过 WebSocket/Long Polling 连接平台，无需公网 IP

### 2.3 长连接架构设计

#### 2.3.1 架构选择

**传统 Webhook 方式（不采用）**:
```
Discord/Telegram 平台
    ↓ HTTP POST Webhook
clibot 服务器（需要公网 IP:8080）
```

**问题**:
- ❌ 需要公网 IP 地址
- ❌ 家庭/办公室网络无法部署（NAT 防火墙）
- ❌ 需要配置端口转发/DDNS
- ❌ 暴露服务端口到互联网（安全风险）

**长连接方式（已采用）**:
```
clibot 服务器（任何网络）
    ↓ WebSocket / Long Polling（主动连接）
Discord/Telegram 平台
    ↓
实时接收消息事件
```

**优势**:
- ✅ 无需公网 IP - clibot 主动连接到平台
- ✅ 支持任何网络环境 - 家庭、办公室、云服务器均可
- ✅ 更安全 - 不暴露服务端口到互联网
- ✅ 部署简单 - 无需复杂网络配置

#### 2.3.2 各平台连接方式

**Discord - WebSocket Gateway**:
- **技术**: WebSocket Gateway API
- **库**: `github.com/bwmarrin/discordgo`
- **实现**: `discordgo.New()` → `session.Open()` 自动建立 WebSocket 连接
- **特点**:
  - 官方库内置 WebSocket 支持
  - 自动重连机制
  - 实时双向通信

**Telegram - Long Polling**:
- **技术**: Bot API getUpdates 方法
- **实现**: HTTP 长轮询，`timeout=30` 秒
- **特点**:
  - 简单的 HTTP 实现
  - 服务端保持连接直到有消息或超时
  - 无需 WebSocket 库

**飞书 - 待调研**:
- **状态**: 需要确认是否支持长连接
- **备选方案**:
  - 如果支持长轮询：实现类似 Telegram 的方式
  - 如果不支持：暂时跳过，或使用内网穿透（frp/ngrok）
  - 企业版：可能有事件订阅 API

#### 2.3.3 连接管理

**Discord WebSocket**:
```go
// 自动管理 WebSocket 连接
session, err := discordgo.New("Bot " + token)
session.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
    // 处理接收到的消息
})
session.Open()  // 建立 WebSocket，自动保持连接
```

**Telegram Long Polling**:
```go
for {
    // 长轮询获取更新
    resp, err := http.Get("https://api.telegram.org/bot<token>/getUpdates?timeout=30")
    // 处理响应
    // 继续下一次轮询
}
```

**重连策略**:
- **Discord**: `discordgo` 自动处理 WebSocket 重连
- **Telegram**: 轮询失败时等待 5 秒后重试

#### 2.3.4 部署场景对比

| 部署环境 | Webhook 方式 | 长连接方式 |
|---------|-------------|-----------|
| 家庭网络（NAT） | ❌ 不可用 | ✅ 完美支持 |
| 办公网络（防火墙） | ❌ 不可用 | ✅ 完美支持 |
| 云 VPS（有公网 IP） | ✅ 可用但需配置 | ✅ 开箱即用 |
| 安全性 | ⚠️ 暴露端口 | ✅ 不暴露端口 |

**结论**: 长连接架构使 clibot 真正实现"随时随地"的目标，特别是在家庭和办公室网络中。

---

## 3. 核心接口定义

### 3.1 CLI 适配器接口

```go
// CLI 适配器接口
type CLIAdapter interface {
    // 发送输入到 CLI（通过 tmux send-keys）
    SendInput(sessionName, input string) error

    // 获取最新的完整回复（读取 CLI 历史文件）
    GetLastResponse(sessionName string) (string, error)

    // 检查 session 是否存活
    IsSessionAlive(sessionName string) bool

    // 创建新 session（可选）
    CreateSession(sessionName, cliType, workDir string) error

    // ========== 新增：交互检测方法 ==========

    // CheckInteractive 检查 CLI 是否在等待用户输入
    // 返回: (是否在等待, 提示文本, 错误)
    // 用于处理中间交互场景，如确认执行命令、澄清歧义等
    CheckInteractive(sessionName string) (bool, string, error)
}
```

**设计要点**:
- `SendInput` 通过 `tmux send-keys` 发送输入
- `GetLastResponse` 读取各 CLI 的历史文件（Claude: ~/.claude/conversations/*.json）
- 不需要 `WaitForCompletion`，改用 hook 事件驱动
- 各 CLI 适配器根据自身历史文件格式实现 `GetLastResponse`
- **`CheckInteractive`**: 每个 CLI 实现自己的交互检测逻辑（正则匹配、上下文检测等）

### 3.2 Bot 适配器接口

```go
// Bot 适配器接口
type BotAdapter interface {
    // 启动 Bot，建立长连接并开始监听消息
    // - Discord: WebSocket Gateway 连接
    // - Telegram: HTTP Long Polling 轮询
    // 连接从 clibot 发起到平台，无需公网 IP
    Start(messageHandler func(BotMessage)) error

    // 发送消息到 IM 平台
    // 通常使用 HTTP REST API 发送
    SendMessage(channel, message string) error

    // 停止 Bot，关闭连接并清理资源
    // - Discord: 关闭 WebSocket 连接
    // - Telegram: 停止轮询循环
    Stop() error
}

// Bot 消息结构
type BotMessage struct {
    Platform  string    // feishu/discord/telegram
    UserID    string    // 用户唯一标识（用于权限控制）
    Channel   string    // 频道/会话 ID
    Content   string    // 消息内容
    Timestamp time.Time
}
```

**设计要点**:
- **长连接架构**: Bot 主动连接到平台（WebSocket/Long Polling），平台无需回调 clibot
- **无需公网 IP**: clibot 可部署在家庭/办公室网络（NAT 后面），仍能正常工作
- **Start 内部处理授权认证，建立长连接，启动消息监听循环
- 通过注入的 `messageHandler` 回调将消息传给 Engine
- `SendMessage` 使用 HTTP REST API，支持 Markdown 格式（平台负责渲染）
- **连接管理**: 自动重连（Discord）、重试逻辑（Telegram）、优雅关闭

---

## 4. 项目结构

```
clibot/
├── cmd/
│   ├── main.go              # 主程序入口
│   ├── root.go              # cobra 根命令
│   ├── start.go             # start 命令（启动主进程）
│   ├── hook.go              # hook 命令（被 CLI hook 调用）
│   └── status.go            # status 命令（查看状态，可选）
├── internal/
│   ├── core/
│   │   ├── engine.go        # 核心调度引擎
│   │   ├── config.go        # 配置管理
│   │   ├── session.go       # Session 管理（含状态机）
│   │   └── logger.go        # 日志模块
│   ├── cli/
│   │   ├── interface.go     # CLI 适配器接口定义
│   │   ├── claude.go        # Claude Code 实现
│   │   ├── gemini.go        # Gemini CLI 实现
│   │   └── opencode.go      # OpenCode 实现
│   ├── bot/
│   │   ├── interface.go     # Bot 适配器接口定义
│   │   ├── feishu.go        # 飞书实现
│   │   ├── discord.go       # Discord 实现
│   │   └── telegram.go      # Telegram 实现
│   ├── watchdog/
│   │   ├── watchdog.go      # Watchdog 监控逻辑
│   │   └── tmux.go          # Tmux 工具函数
│   └── hook/
│       └── server.go        # HTTP Hook 服务器
├── configs/
│   └── config.yaml          # 配置文件模板
└── README.md
```

---

## 5. 核心组件设计

### 5.1 Engine 调度引擎

```go
type Engine struct {
    config       *Config
    cliAdapters  map[string]CLIAdapter      // cli 类型 → 适配器
    activeBots   map[string]BotAdapter      // bot 类型 → 适配器
    sessions     map[string]*Session         // session name → Session
    messageChan  chan BotMessage             // Bot 消息 channel
    responseChan chan ResponseEvent          // CLI 响应 channel
    hookServer   *http.Server                // HTTP 服务器
}

// Session 状态定义
type SessionState string

const (
    StateIdle         SessionState = "idle"            // 空闲
    StateProcessing   SessionState = "processing"      // 处理中
    StateWaitingInput SessionState = "waiting_input"   // 等待用户输入（中间交互）
    StateError        SessionState = "error"           // 错误
)

type Session struct {
    Name      string          // tmux session 名称
    CLIType   string          // claude/gemini/opencode
    WorkDir   string          // 工作目录
    State     SessionState    // 当前状态
    CreatedAt time.Time
}

type ResponseEvent struct {
    SessionName string
    Response    string
    Timestamp   time.Time
}
```

**主循环**:
```go
func (e *Engine) Run() {
    // 启动 HTTP hook 服务器
    go e.StartHookServer(":8080")

    // 启动所有 Bot
    for _, bot := range e.activeBots {
        go bot.Start(e.HandleBotMessage)
    }

    // 主事件循环
    for {
        select {
        case msg := <-e.messageChan:
            e.HandleUserMessage(msg)
        case event := <-e.responseChan:
            e.HandleCLIResponse(event)
        }
    }
}
```

**消息处理流程**:
```go
func (e *Engine) HandleUserMessage(msg BotMessage) {
    // 0. 安全检查：验证用户是否在白名单中
    if !e.isUserAuthorized(msg) {
        e.SendToBot(msg.Platform, msg.Channel, "❌ 未授权用户：请联系管理员添加你的用户ID")
        return
    }

    // 1. 检查是否是特殊命令（根据配置的前缀）
    prefix := e.config.CommandPrefix
    if strings.HasPrefix(msg.Content, prefix) {
        cmd := strings.TrimPrefix(msg.Content, prefix)
        e.HandleSpecialCommand(cmd, msg)
        return
    }

    // 2. 获取当前激活的 session
    session := e.GetActiveSession(msg.Channel)

    // 3. 如果 session 正在等待输入（中间交互状态），直接透传
    if session.State == StateWaitingInput {
        adapter := e.cliAdapters[session.CLIType]
        adapter.SendInput(session.Name, msg.Content)
        session.State = StateProcessing  // 恢复处理状态
        go e.startWatchdog(session)      // 继续监控
        return
    }

    // 4. 正常流程：发送到 CLI
    adapter := e.cliAdapters[session.CLIType]
    adapter.SendInput(session.Name, msg.Content)
    session.State = StateProcessing

    // 5. 启动 Watchdog（监控中间交互）和超时计时器
    go func() {
        // 启动 Watchdog
        e.startWatchdog(session)

        // 等待 Hook 事件或超时
        select {
        case resp := <-e.responseChan:
            if resp.SessionName == session.Name {
                session.State = StateIdle
                e.SendToAllBots(resp.Response)
            }
        case <-time.After(5 * time.Minute):
            session.State = StateError
            e.SendToAllBots("⚠️ CLI 响应超时\n建议: 使用 !!status 检查状态")
        }
    }()
}
```

**Watchdog 监控逻辑**:
```go
// startWatchdog 启动监控，检测 CLI 是否在等待用户输入
func (e *Engine) startWatchdog(session *Session) {
    // 分阶段轮询策略（避免频繁查询）
    intervals := []time.Duration{
        1 * time.Second,  // 第1秒：检测立即交互
        2 * time.Second,  // 第3秒：检测快速交互
        5 * time.Second,  // 第8秒：检测慢速交互
    }

    for _, interval := range intervals {
        time.Sleep(interval)

        // 如果已经完成或出错，停止监控
        if session.State != StateProcessing {
            return
        }

        // 调用适配器的检测方法
        adapter := e.cliAdapters[session.CLIType]
        waiting, prompt, err := adapter.CheckInteractive(session.Name)

        if waiting && err == nil {
            // 更新状态
            session.State = StateWaitingInput

            // 推送给用户
            message := fmt.Sprintf("⚠️ **CLI 需要确认**:\n```\n%s\n```\n回复确认继续", prompt)
            e.SendToAllBots(message)

            return  // 停止 Watchdog，等待用户回复
        }
    }
}
```

**用户授权检查**:
```go
// isUserAuthorized 检查用户是否在白名单中
func (e *Engine) isUserAuthorized(msg BotMessage) bool {
    // 如果白名单未启用，允许所有用户（警告：生产环境应该启用）
    if !e.config.Security.WhitelistEnabled {
        return true
    }

    // 获取该平台的白名单用户列表
    userIDs, ok := e.config.Security.AllowedUsers[msg.Platform]
    if !ok {
        return false
    }

    // 检查用户是否在白名单中
    for _, uid := range userIDs {
        if uid == msg.UserID {
            return true
        }
    }

    return false
}
```

### 5.2 HTTP Hook 服务器

**Hook 触发流程**:

```
1. Claude Code 完成
2. 触发 hook → 执行命令: clibot hook --session project-a --event completed
3. clibot hook 命令 → 发送 HTTP 请求到主进程
4. 主进程收到通知 → 获取响应 → 推送给 Bot
```

**HTTP Server 实现**:
```go
func (e *Engine) StartHookServer(addr string) {
    http.HandleFunc("/hook", func(w http.ResponseWriter, r *http.Request) {
        session := r.URL.Query().Get("session")
        event := r.URL.Query().Get("event")

        if event == "completed" {
            // 获取响应
            sessionInfo := e.sessions[session]
            adapter := e.cliAdapters[sessionInfo.CLIType]
            response, _ := adapter.GetLastResponse(session)

            // 发送到 channel
            e.responseChan <- ResponseEvent{
                SessionName: session,
                Response:    response,
                Timestamp:   time.Now(),
            }
        }

        w.WriteHeader(200)
    })

    http.ListenAndServe(addr, nil)
}
```

**Hook 命令实现**:
```go
var hookCmd = &cobra.Command{
    Use: "hook --session <name> --event <type>",
    Run: func(cmd *cobra.Command, args []string) {
        session, _ := cmd.Flags().GetString("session")
        event, _ := cmd.Flags().GetString("event")

        // 发送 HTTP 请求到主进程
        url := fmt.Sprintf("http://localhost:8080/hook?session=%s&event=%s", session, event)
        resp, err := http.Get(url)
        if err != nil {
            log.Fatal("Hook 请求失败:", err)
        }
        defer resp.Body.Close()
    },
}
```

**CLI Hook 配置示例**（Claude Code）:
```json
{
  "hooks": {
    "onCompletion": "clibot hook --session $CLIBOT_SESSION --event completed"
  }
}
```

---

## 6. CLI 适配器实现

### 6.1 Claude Code 适配器

```go
type ClaudeAdapter struct {
    historyDir string // ~/.claude/conversations
    checkLines int    // 检查最后几行（配置）
    patterns   []string // 交互模式（配置）
}

func (c *ClaudeAdapter) SendInput(sessionName, input string) error {
    // 通过 tmux send-keys 发送输入
    cmd := exec.Command("tmux", "send-keys", "-t", sessionName, input, "Enter")
    return cmd.Run()
}

func (c *ClaudeAdapter) GetLastResponse(sessionName string) (string, error) {
    // 1. 找到最新的对话文件
    files, _ := filepath.Glob(filepath.Join(c.historyDir, "*.json"))
    latestFile := getLatestFile(files)

    // 2. 解析 JSON，提取最后一条 assistant 消息
    data, _ := os.ReadFile(latestFile)
    var conversation Conversation
    json.Unmarshal(data, &conversation)

    // 3. 返回纯文本内容
    return conversation.LastAssistantMessage().Content, nil
}

func (c *ClaudeAdapter) IsSessionAlive(sessionName string) bool {
    // 检查 tmux session 是否存在
    cmd := exec.Command("tmux", "has-session", "-t", sessionName)
    return cmd.Run() == nil
}

// ========== 新增：CheckInteractive 实现 ==========

func (c *ClaudeAdapter) CheckInteractive(sessionName string) (bool, string, error) {
    // 1. 捕获屏幕（最后 N 行）
    output, err := tmux.CapturePane(sessionName, c.checkLines)
    if err != nil {
        return false, "", err
    }

    // 2. Claude Code 特有的交互模式
    patterns := []string{
        `\? \[y/N\]`,           // Execute? [y/N]
        `\? \(y/n\)`,           // Confirm? (y/n)
        `Press Enter to continue`,
        `onfirm\?`,
    }

    // 3. 只检查最后 3 行（通常提示符在这里）
    lines := strings.Split(output, "\n")
    lastLines := lastN(lines, 3)

    for _, line := range lastLines {
        // 清理 ANSI 颜色码
        clean := stripansi.Strip(line)

        // 匹配交互模式
        for _, pattern := range patterns {
            matched, _ := regexp.MatchString(pattern, clean)
            if matched {
                return true, clean, nil  // 返回清理后的提示文本
            }
        }
    }

    return false, "", nil
}
```

### 6.2 Gemini CLI 适配器

```go
type GeminiAdapter struct {
    historyDB string // ~/.gemini/history.sqlite
    checkLines int
    patterns   []string
}

func (g *GeminiAdapter) GetLastResponse(sessionName string) (string, error) {
    // 1. 查询 SQLite，获取最新对话
    db, _ := sql.Open("sqlite3", g.historyDB)
    defer db.Close()

    var content string
    db.QueryRow("SELECT content FROM messages WHERE role='assistant' ORDER BY timestamp DESC LIMIT 1").Scan(&content)

    return content, nil
}

// SendInput 和 IsSessionAlive 实现类似

// ========== 新增：CheckInteractive 实现 ==========

func (g *GeminiAdapter) CheckInteractive(sessionName string) (bool, string, error) {
    output, err := tmux.CapturePane(sessionName, g.checkLines)
    if err != nil {
        return false, "", err
    }

    // Gemini 特有的模式（可能有多行警告）
    patterns := []string{
        `⚠️ .* \? \(yes/no\)`,           // 警告 + 确认
        `Select an option \[\d+-\d+\]:`, // 数字选择
        `Enter to proceed`,
    }

    // Gemini 可能需要检查更多行
    lines := strings.Split(output, "\n")
    lastLines := lastN(lines, 5)

    for _, line := range lastLines {
        clean := stripansi.Strip(line)

        for _, pattern := range patterns {
            matched, _ := regexp.MatchString(pattern, clean)
            if matched {
                // 提取多行上下文（Gemini 可能有详细的警告信息）
                context := g.extractContext(lastLines)
                return true, context, nil
            }
        }
    }

    return false, "", nil
}

// extractContext 提取多行上下文
func (g *GeminiAdapter) extractContext(lines []string) string {
    var context []string
    for _, line := range lines {
        clean := stripansi.Strip(line)
        if clean != "" {
            context = append(context, clean)
        }
    }
    return strings.Join(context, "\n")
}
```

---

## 7. Bot 适配器实现

### 7.1 飞书 Bot

```go
type FeishuBot struct {
    appID      string
    appSecret  string
    client     *lark.Client
}

func (f *FeishuBot) Start(messageHandler func(BotMessage)) error {
    // 初始化飞书客户端
    f.client = lark.NewClient(f.appID, f.appSecret)

    // 启动消息监听
    f.client.Event.Subscribe(&lark.MessageReceivedEvent{}, func(ctx context.Context, event *lark.Event) {
        msg := event.(*lark.MessageReceivedEvent)

        messageHandler(BotMessage{
            Platform:  "feishu",
            Channel:   msg.Message.ChatID,
            Content:   msg.Message.Content,
            Timestamp: time.Now(),
        })
    })

    return nil
}

func (f *FeishuBot) SendMessage(channel, message string) error {
    // 发送富文本消息（支持 Markdown）
    return f.client.Message.Send(&lark.SendMessageRequest{
        MessageType: "text",
        ReceiveID:   channel,
        Content:     message,
    })
}
```

### 7.2 Discord Bot

```go
type DiscordBot struct {
    token    string
    session  *discordgo.Session
}

func (d *DiscordBot) Start(messageHandler func(BotMessage)) error {
    var err error
    d.session, err = discordgo.New("Bot " + d.token)
    if err != nil {
        return err
    }

    // 注册消息处理器
    d.session.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
        messageHandler(BotMessage{
            Platform:  "discord",
            Channel:   m.ChannelID,
            Content:   m.Content,
            Timestamp: time.Now(),
        })
    })

    return d.session.Open()
}

func (d *DiscordBot) SendMessage(channel, message string) error {
    _, err := d.session.ChannelMessageSend(channel, message)
    return err
}
```

---

## 8. 特殊命令

### 8.1 命令前缀配置

```yaml
# 特殊命令前缀（可自定义，避免与 CLI 的 / 命令冲突）
command_prefix: "!!"   # 可选: !! >> ## @+ . 等
```

**设计理由**:
1. **避免冲突**: CLI 常用 `/` 前缀（如 `/exit`, `/help`），使用其他前缀避免冲突
2. **手机友好**: `!` 或其他符号比 `/` 在手机上更容易输入
3. **灵活定制**: 用户可以选择自己习惯的前缀

### 8.2 支持的命令

```
!!sessions              # 列出所有 session
!!use <session>         # 切换当前 session
!!new <name> <cli>      # 创建新 session
!!whoami                # 显示当前 session 信息
!!status                # 显示所有 session 状态
!!help                  # 帮助信息
```

### 8.3 示例交互

```
配置: command_prefix: "!!"

用户: !!sessions
Bot: 📋 可用 Sessions:
     • project-a (claude) - idle
     • backend-test (gemini) - working

用户: !!use backend-test
Bot: ✅ 已切换到 backend-test

用户: /help         # 直接透传给 CLI（Claude Code 的 help）
Bot: [Claude Code 的帮助信息]

用户: /exit         # 直接透传给 CLI（退出 Claude Code）
Bot: [已退出]

用户: 分析这段代码   # 无前缀，透传
Bot: [AI 的分析结果]
```

---

## 9. 配置文件

### 9.1 完整配置示例

```yaml
# HTTP Hook 服务端口
hook_server:
  port: 8080

# 特殊命令前缀
command_prefix: "!!"

# ========== 安全配置（白名单机制） ==========
# clibot 本质上是远程代码执行工具，必须启用用户白名单
security:
  # 白名单开关（默认启用）
  whitelist_enabled: true

  # 允许的用户列表（平台 -> 用户ID）
  allowed_users:
    telegram:
      - 123456789      # Telegram user_id
      - 987654321
    discord:
      - "123456789012345678"  # Discord user_id
    feishu:
      - "ou_xxx"        # 飞书 open_id

  # 管理员列表（可以执行危险命令）
  admins:
    telegram:
      - 123456789

# ========== 新增：Watchdog 配置 ==========
watchdog:
  enabled: true
  # 分阶段轮询间隔（避免频繁查询）
  check_intervals:
    - 1s    # 第1秒：检测立即交互
    - 2s    # 第3秒：检测快速交互
    - 5s    # 第8秒：检测慢速交互
  timeout: 5m  # 超时时间

# tmux session 管理
sessions:
  # 自动发现已有 tmux session，或手动配置
  - name: "project-a"
    cli_type: "claude"      # claude/gemini/opencode
    work_dir: "/home/user/project-a"
    auto_start: true        # 如果不存在是否自动创建

  - name: "backend-test"
    cli_type: "gemini"
    work_dir: "/home/user/tests"
    auto_start: false

# 默认 session（Bot 未指定时使用）
default_session: "project-a"

# Bot 配置
bots:
  feishu:
    enabled: true
    app_id: "${FEISHU_APP_ID}"        # 从环境变量读取
    app_secret: "${FEISHU_APP_SECRET}"
    default_channel: "ou_xxx"

  discord:
    enabled: true
    token: "${DISCORD_TOKEN}"
    channel_id: "123456789"

  telegram:
    enabled: false
    token: "${TELEGRAM_TOKEN}"

# CLI 适配器配置
cli_adapters:
  claude:
    history_dir: "~/.claude/conversations"
    hook_command: "clibot hook --session $SESSION --event completed"

    # ========== 新增：交互检测配置 ==========
    interactive:
      enabled: true
      check_lines: 3              # 检查最后几行
      patterns:                   # 用户可以添加自定义模式
        - "\\? [y/N]"
        - "Confirm\\?"
        - "Press Enter"

  gemini:
    history_db: "~/.gemini/history.sqlite"
    hook_command: "clibot hook --session $SESSION --event completed"

    interactive:
      enabled: true
      check_lines: 5              # Gemini 可能需要更多行
      patterns:
        - "⚠️ .* \\? \\(yes/no\\)"
        - "Select an option"

  opencode:
    history_file: "~/.opencode/sessions.log"
    hook_command: "clibot hook --session $SESSION --event completed"

    interactive:
      enabled: true
      check_lines: 3
      patterns:
        - "\\[Y/n\\]"
        - "Continue\\?"

# 日志配置
logging:
  level: "info"
  file: "/var/log/clibot/app.log"
```

### 9.2 配置加载策略

**混合方式**（安全 + 便捷）:
- 基础配置: `config.yaml`
- 敏感信息: 环境变量（如 `${FEISHU_APP_ID}`）
- 启动时合并，敏感信息优先从环境变量读取

---

## 10. 数据流

### 10.1 完整消息处理流程（正常场景）

```
1. 用户在飞书发送: "帮我优化这个函数"

2. Bot 飞书适配器接收消息
   └─> 回调传给 Engine.messageChan

3. Engine 解析消息
   ├─> 特殊命令? (如 !!status)
   │   └─> 是 → Engine 直接处理
   └─> 否 → 透传给当前激活的 session

4. CLI 适配器调用
   ├─> SendInput("project-a", "帮我优化这个函数")
   ├─> 通过 tmux send-keys 发送到 Claude Code
   └─> Engine 启动 Watchdog + 超时计时器

5. Claude Code 完成 → 触发 hook
   └─> 执行: clibot hook --session project-a --event completed

6. clibot hook 命令
   └─> HTTP GET http://localhost:8080/hook?session=project-a&event=completed

7. Engine HTTP server 收到请求
   ├─> 调用 ClaudeAdapter.GetLastResponse("project-a")
   ├─> 读取 ~/.claude/conversations/最新文件
   └─> 发送到 Engine.responseChan

8. Engine 收到响应
   └─> 调用所有 Bot.SendMessage() 推送消息

9. 用户在 IM 中看到 AI 的回复
```

### 10.2 中间交互处理流程（Watchdog 场景）

```
1. 用户在飞书发送: "删除所有临时文件"

2. Engine 发送到 CLI
   └─> SendInput("project-a", "删除所有临时文件")

3. Engine 启动 Watchdog（分阶段轮询）

4. Watchdog 第1轮（1秒后）
   └─> CheckInteractive() → false（继续等待）

5. Claude Code 显示确认: "Execute 'rm -rf ./temp'? [y/N]"
   └─> Hook 未触发（任务未完成）

6. Watchdog 第2轮（3秒后）
   └─> CheckInteractive() → true
   └─> 捕获屏幕: "Execute 'rm -rf ./temp'? [y/N]"
   └─> 更新 Session.State = StateWaitingInput

7. Engine 推送消息给用户
   └─> "⚠️ **CLI 需要确认**:\n```\nExecute 'rm -rf ./temp'? [y/N]\n```\n回复确认继续"

8. 用户回复: "y"

9. Engine 检测到 StateWaitingInput
   └─> 直接透传: SendInput("project-a", "y")
   └─> 恢复 StateProcessing
   └─> 重新启动 Watchdog

10. Claude Code 完成 → 触发 hook
    └─> 后续流程同正常场景
```

### 10.3 时序图（含 Watchdog）

```
正常场景（无中间交互）:
用户      Bot       Engine      Watchdog    CLI      Hook
 |         |          |             |          |        |
 |--发送消息->       |             |          |        |
 |         |--消息---->|             |          |        |
 |         |          |--发送输入---->|          |        |
 |         |          |             |--启动轮询-->       |
 |         |          |             |          |--处理->|
 |         |          |             |<--无交互---|       |
 |         |          |             |          |--完成->|
 |         |          |             |          |--触发-->|
 |         |          |             |<----------HTTP----|
 |         |          |<-响应事件----------------------|
 |         |<-推送回复-|             |          |        |
 |--收到回复----------|             |          |        |

中间交互场景（Watchdog 检测）:
用户      Bot       Engine      Watchdog    CLI
 |         |          |             |          |
 |--发送消息->       |             |          |
 |         |--消息---->|             |          |
 |         |          |--发送输入---->|          |
 |         |          |             |--启动轮询-->|
 |         |          |             |          |--显示 [y/N]--|
 |         |          |             |<--检测到交互-----|
 |         |          |<-推送确认请求----------|       |
 |         |<-收到确认-|             |          |       |
 |--回复 "y"-------->|             |          |       |
 |         |          |--发送 "y"---->|          |       |
 |         |          |             |--重启轮询-->      |
 |         |          |             |          |--继续->|
 |         |          |             |<----------完成----|
 |         |          |<-最终响应-----------------------|
 |         |<-推送结果-|             |          |       |
 |--收到结果----------|             |          |       |
```

---

## 11. 中间交互处理（Watchdog 机制）

### 11.1 问题背景

在原始设计中，我们假定 CLI 工具的交互模式是原子化的：
`用户输入 -> CLI 处理 -> CLI 完成(触发 Hook) -> 读取历史文件 -> 返回结果`

但在实际场景中，AI CLI 工具经常会产生**中间交互请求**，例如：
- 确认执行高危命令：`Execute 'rm -rf ./temp'? [y/N]`
- 澄清歧义：`Do you mean file A or file B? (Select 1-2)`

**"半回合死锁"问题**：
1. **Hook 未触发**：CLI 处于"等待输入"状态，任务尚未完成，因此不会触发 `onCompletion` Hook
2. **历史文件未更新**：通常 CLI 只有在完成一个完整回合（收到用户回答后）才会将对话写入历史文件
3. **结果**：Engine 等 Hook，CLI 等输入 → 死锁，用户最终收到"超时错误"

### 11.2 解决方案：混合监听模式

为了解决中间交互问题，我们在 Hook 架构基础上增加了 **Watchdog（超时监视器）**。

#### 核心逻辑

Engine 在发送消息给 CLI 后，启动双轨监听：

**主轨道（Happy Path）**：等待 `Hook` 事件
- **适用场景**：95% 的正常对话、分析、代码生成
- **行为**：收到 Hook → 读取历史文件 (JSON/SQLite) → 推送结果
- **优势**：内容完整，无 ANSI 乱码，格式完美

**辅轨道（Watchdog）**：状态轮询兜底
- **适用场景**：中间确认、卡顿检测
- **行为**：
  1. 启动后，分阶段轮询（1s → 3s → 8s）
  2. 调用适配器的 `CheckInteractive()` 方法
  3. 适配器通过 `tmux capture-pane` 抓取屏幕最后几行
  4. **正则特征匹配**：检查是否存在等待输入的特征符
  5. **触发交互**：
     - 如果匹配成功，判定为"阻塞中"
     - 抓取屏幕上的提示文本（清理 ANSI codes）
     - 立即推送给 IM 用户："⚠️ **CLI 需要确认**: `[屏幕截图内容]`"
     - 标记 Session 状态为 `waiting_for_input`

### 11.3 Watchdog 设计优势

✅ **保持原架构**：95% 正常场景仍用 Hook（完整、干净的历史文件）
✅ **安全网兜底**：处理中间交互 + CLI 崩溃/Hook 失效
✅ **智能轮询**：分阶段检测，避免频繁查询
✅ **CLI 专属**：每个 CLI 实现自己的交互检测逻辑
✅ **上下文感知**：只在提示符行匹配，减少误报

### 11.4 不同 CLI 的交互模式差异

不同 CLI 的交互判断逻辑确实不一样，所以我们让每个 CLI 适配器实现自己的 `CheckInteractive()` 方法：

**Claude Code**:
```
Execute 'rm -rf ./temp'? [y/N]
Continue? (y/n)
Press Enter to continue...
```

**Gemini CLI**:
```
⚠️ This will modify 3 files. Proceed? (yes/no)
Select an option [1-3]:
```

**OpenCode**:
```
Confirm action? [Y/n]
Waiting for user input...
```

每个适配器可以：
- 定义自己的交互模式（正则表达式）
- 设置检查的行数
- 实现特殊的上下文提取逻辑（如 Gemini 的多行警告）
- 处理特殊的交互类型（文本输入、密码等）

### 11.5 状态机设计

```go
type SessionState string

const (
    StateIdle         SessionState = "idle"            // 空闲
    StateProcessing   SessionState = "processing"      // 处理中
    StateWaitingInput SessionState = "waiting_input"   // 等待用户输入
    StateError        SessionState = "error"           // 错误
)
```

**状态转换**:
```
Idle → Processing: 发送消息到 CLI
Processing → Idle: Hook 触发，收到完整响应
Processing → WaitingInput: Watchdog 检测到交互请求
WaitingInput → Processing: 用户回复，继续处理
Processing → Error: 超时或异常
```

### 11.6 Tmux 工具函数

Watchdog 需要一些 tmux 工具函数：

```go
package watchdog

// CapturePane 捕获 tmux session 的屏幕输出
func CapturePane(sessionName string, lines int) (string, error) {
    cmd := exec.Command("tmux", "capture-pane",
        "-t", sessionName,
        "-p",              // 输出到 stdout
        "-e",              # 包含转义序列（用于清理 ANSI）
        fmt.Sprintf("-%d", lines))  # 最后 N 行
    output, err := cmd.Output()
    return string(output), err
}

// StripANSI 清理 ANSI 颜色码
func StripANSI(input string) string {
    // 使用第三方库: github.com/acarl005/stripansi
    return stripansi.Strip(input)
}
```

---

## 12. 错误处理

### 12.1 错误处理策略

**友好提示**:
- CLI 执行失败 → 在 IM 中显示错误提示和可能的原因
- Session 不存在 → 提示使用 `!!new` 创建或 `!!sessions` 查看
- 超时 → 提示响应超时，建议检查 CLI 状态
- **中间交互** → Watchdog 检测到时，清晰推送确认请求

**日志记录**:
- 所有错误详情记录到日志文件
- 日志级别: ERROR / WARN / INFO / DEBUG
- 日志轮转: 按日期/大小切分

### 12.2 错误场景示例

```go
// Session 不存在
if !adapter.IsSessionAlive(sessionName) {
    bot.SendMessage(channel,
        fmt.Sprintf("❌ Session '%s' 不存在\n使用 !!sessions 查看可用 session", sessionName))
    return
}

// CLI 响应超时
select {
case resp := <-e.responseChan:
    // 正常处理
case <-time.After(5 * time.Minute):
    e.SendToAllBots("⚠️ CLI 响应超时\n" +
        "可能原因:\n" +
        "1. CLI 进程卡死\n" +
        "2. 网络问题\n" +
        "3. API 限流\n\n" +
        "建议: 使用 !!status 检查状态")
}
```

---

## 13. 技术栈

### 13.1 核心依赖

- **语言**: Go 1.21+
- **CLI 框架**: Cobra (命令行接口)
- **HTTP**: Go 标准库 net/http
- **配置**: Viper (配置管理)
- **日志**: Zap / Logrus (结构化日志)
- **并发**: Go 标准库 (goroutine + channel)
- **ANSI 清理**: github.com/acarl005/stripansi

### 13.2 Bot SDK

- **飞书**: lark-go (第三方 SDK)
- **Discord**: discordgo (官方推荐)
- **Telegram**: telegram-bot-api (官方库)

---

## 14. 后续扩展方向

### 14.1 短期优化

1. **消息格式优化**: 结构化渲染，根据平台能力自适应
2. **流式输出**: 支持 AI 流式响应的实时推送
3. **文件传输**: 支持上传图片/文件给 AI 分析

### 14.2 长期规划

1. **Web 管理界面**: 查看 session 状态、历史记录、配置管理
2. **数据持久化**: 消息持久化、使用统计、计费
3. **多用户支持**: 团队协作场景
4. **智能路由**: 根据任务类型自动选择最合适的 CLI

---

## 15. 附录

### 15.1 命令示例

```bash
# 启动主程序
clibot start --config config.yaml

# 手动触发 hook（测试用）
clibot hook --session project-a --event completed

# 查看状态
clibot status
```

### 15.2 tmux 操作示例

```bash
# 查看所有 session
tmux list-sessions

# 创建新 session 并启动 claude
tmux new-session -d -s project-a -c ~/project-a
tmux send-keys -t project-a "claude" Enter

# 查看特定 session 的输出
tmux capture-pane -t project-a -p
```

### 15.3 Watchdog 调试示例

```bash
# 手动测试 tmux capture-pane
tmux capture-pane -t project-a -p -e -S -10

# 测试 ANSI 清理
echo $'\e[31m红字\e[0m普通' | stripansi

# 测试交互模式匹配
# 在 CLI 中触发确认请求，然后执行
clibot hook --session project-a --event test
```

---

## 16. 版本历史

### v0.4 (2026-01-29)
- ✅ **架构升级：采用长连接架构**
  - Bot 通过 WebSocket/Long Polling 主动连接平台
  - 无需公网 IP，可部署在家庭/办公室网络
  - 更安全：不暴露服务端口到互联网
  - Discord: 使用 WebSocket Gateway（已实现）
  - Telegram: 使用 Long Polling（待实现）
  - 飞书：待调研长连接支持
- ✅ 更新核心设计原则，增加长连接架构说明
- ✅ 新增 2.3 节：长连接架构设计详细说明
- ✅ 更新 BotAdapter 接口文档，反映长连接方式
- ✅ 确认 Discord 实现已符合长连接架构

### v0.3 (2026-01-28)
- ✅ 整合安全白名单机制（用户认证）
- ✅ BotMessage 结构新增 UserID 字段
- ✅ Engine 增加 isUserAuthorized() 授权检查
- ✅ 配置文件增加 security 配置段（whitelist_enabled, allowed_users, admins）
- ✅ HandleUserMessage 增加安全检查步骤

### v0.2 (2026-01-28)
- ✅ 整合中间交互处理设计（Watchdog 机制）
- ✅ CLIAdapter 接口新增 `CheckInteractive()` 方法
- ✅ Session 状态机设计（idle/processing/waiting_input/error）
- ✅ Engine 集成 Watchdog 监控逻辑
- ✅ 配置文件增加 watchdog 和 interactive 配置项
- ✅ 数据流更新，增加中间交互处理流程
- ✅ 项目结构新增 watchdog 模块

### v0.1 (2026-01-28)
- ✅ 初始设计
- ✅ 核心接口定义（CLI + Bot）
- ✅ Engine 调度引擎设计
- ✅ HTTP Hook 机制
- ✅ 特殊命令设计（可配置前缀）

---

**文档结束**
