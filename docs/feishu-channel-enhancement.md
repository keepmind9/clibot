# PRD: Rich Channel 增强

> 参考 [feishu-claude-code-bridge](https://github.com/anthropics/feishu-claude-code-bridge)，为 bot adapter 层增加富交互能力。
> Feishu 是第一个实现这些特性的 channel。所有能力在接口层抽象，Discord、Telegram 等其他 channel 后续可复用。

---

## 设计原则

1. **接口层抽象** — 新能力定义为 `BotAdapter` 的可选接口（optional interface），channel 按需实现，engine 通过类型断言检测支持情况。
2. **优雅降级** — channel 未实现富接口时，engine 自动回退到纯文本模式。
3. **零侵入** — 所有新接口方法提供默认 no-op 实现，现有 adapter 无需任何改动即可编译运行。

---

## 现状

| 能力 | 状态 |
|------|------|
| WebSocket 长连接 | 已完成 |
| 文本消息收发 | 已完成 |
| 打字指示器（emoji 表态） | 已完成 |
| 代理支持 | 已完成 |

## 待增强（参考 feishu-bridge）

| 特性 | 优先级 | 工作量 | 价值 |
|------|--------|--------|------|
| P1: 流式富回复 | 高 | 大 | 核心体验 | ✅ 已完成 |
| P2: 图片/文件处理 | 高 | 中 | 多模态输入 | ✅ 已完成 |
| P3: 消息引用上下文 | 中 | 小 | 上下文质量 | ✅ 已完成 |
| P4: 话题群独立会话 | 中 | 小 | 群聊可用性 | ✅ 已完成 |
| P5: 云文档评论回复 | 低 | 中 | 新场景 | ⏸️ 已搁置 |
| P6: 消息防抖 | 低 | 小 | 稳定性 | ✅ 已完成 |
| P7: Bridge 上下文注入 | 高 | 小 | 消息质量 | ✅ 已完成 |
| P8: @提及策略 | 中 | 小 | 群聊体验 | ✅ 已完成 |
| P9: 回复引用线 | 中 | 小 | 消息可读性 | ✅ 已完成 |
| P10: 并发池控制 | 低 | 小 | 资源保护 | ✅ 已完成 |
| P11: Keepalive 增强 | 低 | 小 | 连接稳定性 | ✅ 已完成 |

---

## 架构差异说明

feishu-bridge 是独立的 Node.js 应用，clibot 是 Go 引擎 + adapter 架构。以下差异影响 PRD 设计：

| 维度 | feishu-bridge | clibot |
|------|--------------|--------|
| 会话模型 | scope-based（一个群共享一个 Claude session） | user-based（每个用户有独立 session） |
| CLI 调用模式 | 每次消息启动新 `claude -p` 进程，通过 `--resume` 恢复 | 持久进程（tmux/stdio），stdin/stdout 管道 |
| 事件流 | 逐事件流式输出（stream-json），驱动卡片增量更新 | hook/stdio 回调返回完整文本，无中间事件 |
| 引擎角色 | bridge 自身就是引擎（channel.ts 包含所有编排逻辑） | engine.go 负责编排，bot/adapter 只做 IO |

**关键设计决策**：P1（流式富回复）依赖 CLI adapter 提供中间事件。当前只有 claude-stdio adapter 支持 stream-json 事件流，hook mode 的 adapter 只能回退到 text 模式。Engine 需要新增事件流协议（详见 P1）。

---

## 接口扩展

所有新能力定义为可选接口。adapter 只实现自己支持的部分。

```go
// RichMessenger 流式富回复接口
// Feishu: CardKit 卡片 | Discord: Embed | Telegram: 编辑消息
type RichMessenger interface {
    CreateRichMessage(channel string, opts RichMessageOptions) (RichMessageHandle, error)
}

type RichMessageOptions struct {
    Title    string            // 可选标题
    StopText string            // 取消按钮文案（如 "Stop"）
    StopData string            // 取消回调数据
    Meta     map[string]string // channel 特有元数据
}

type RichMessageHandle interface {
    Channel() string
    Update(blocks []ContentBlock) error  // 增量更新
    Finish(blocks []ContentBlock) error  // 最终更新，移除交互元素
}

type ContentBlock struct {
    Type      ContentBlockType  // text, tool_call, tool_result, thinking, status
    Title     string            // 块标题（如工具名）
    Content   string            // 渲染内容（markdown）
    Collapsed bool              // 是否默认折叠
    Meta      map[string]string // 工具特有元数据（如文件路径、命令行）
}

// MediaSupporter 多媒体支持接口
// Feishu, Telegram, Discord 等均可实现
type MediaSupporter interface {
    SupportsMediaType(mediaType string) bool
}

// Quotable 消息引用接口
// Feishu, Discord, Telegram 等均可实现
type Quotable interface {
    FetchQuotedMessage(ctx context.Context, channelID, messageID string) (*QuotedMessage, error)
}

type QuotedMessage struct {
    SenderID    string
    SenderName  string
    Content     string
    Timestamp   time.Time
    Attachments []Attachment
}

// Threadable 话题/线程隔离接口
// Feishu(话题群), Discord(threads), Slack(threads)
type Threadable interface {
    ThreadScope(channelID string, msg BotMessage) string
}

// Replyable 回复引用接口
// 支持将 bot 回复关联到用户消息，形成视觉线程
// Feishu(replyInThread), Discord(reply), Telegram(reply_to_message)
type Replyable interface {
    SendMessageWithReply(channel, message, replyToMessageID string) error
}

// MentionPolicy 定义群聊 @提及策略
type MentionPolicy interface {
    ShouldRespond(msg BotMessage) bool
}

// Debounceable 消息防抖接口
type Debounceable interface {
    DebounceWindow() int  // 返回静默窗口（ms），0 表示禁用
}
```

### BotMessage 扩展

```go
type BotMessage struct {
    Platform    string
    UserID      string
    Channel     string
    MessageID   string
    Content     string
    Timestamp   time.Time
    ThreadID    string       // 可选：话题/线程 ID
    QuoteID     string       // 可选：被引用消息 ID
    ChatType    string       // 可选："p2p", "group", "topic"
    SenderName  string       // 可选：发送者显示名
    Attachments []Attachment // 可选：媒体内容
}

type Attachment struct {
    Type     string // "image", "file", "audio", "video"
    FilePath string // 本地下载路径
    FileName string // 原始文件名
    MimeType string // MIME 类型
    Size     int64  // 字节数
}
```

### Engine 检测模式

```go
// engine 通过类型断言检测能力：
if rich, ok := botAdapter.(RichMessenger); ok {
    handle, _ := rich.CreateRichMessage(channel, opts)
    // 驱动流式更新...
} else {
    botAdapter.SendMessage(channel, plainText)  // 回退纯文本
}
```

---

## P1: 流式富回复

### 问题

当前 `SendMessage` 在 Claude 完成后才发送纯文本。无进度反馈、无工具调用可见性，长任务体验差。

### 前置条件：CLI 事件流协议

当前 engine 只接收 CLI 的完整文本响应。要驱动流式卡片更新，需要 CLI adapter 提供中间事件。

**方案**：扩展 `CLIAdapter` 接口，新增可选方法：

```go
// StreamingCLI 支持 emit 中间事件的 CLI adapter
type StreamingCLI interface {
    // SendInputStreaming 发送输入并返回事件通道
    // 每个 event 是 CLI 的中间状态（text chunk、tool call、tool result 等）
    SendInputStreaming(sessionName, input string) (<-chan CLIEvent, error)
}

type CLIEventType string

const (
    CLIEventText       CLIEventType = "text"        // 文本输出
    CLIEventToolUse    CLIEventType = "tool_use"     // 工具调用开始
    CLIEventToolResult CLIEventType = "tool_result"  // 工具调用结果
    CLIEventThinking   CLIEventType = "thinking"     // 思考过程
    CLIEventDone       CLIEventType = "done"         // 完成
    CLIEventUsage      CLIEventType = "usage"        // token 用量
)

type CLIEvent struct {
    Type    CLIEventType
    Content string            // 文本内容
    ToolID  string            // tool_use 时：工具调用 ID
    ToolName string           // tool_use 时：工具名
    Meta    map[string]string // 附加信息
}
```

claude-stdio adapter 已有 stream-json 解析能力，可以天然实现 `StreamingCLI`。其他 adapter 不实现此接口则自动回退。

### 三种回复模式

| 模式 | 说明 | 适用场景 |
|------|------|---------|
| `card` | CardKit 2.0 流式卡片：可折叠工具面板、停止按钮、状态底栏 | 支持 `StreamingCLI` + `RichMessenger` |
| `markdown` | 流式 markdown 更新：逐步展示文本，无工具面板 | 仅需 `RichMessenger` 的基础更新能力 |
| `text` | 纯文本，完成后一次性发送 | 不支持流式的降级模式 |

默认 `text`（向后兼容，与当前行为一致）。用户主动配置 `card` 或 `markdown` 时才启用流式。自动降级链：`card` → `markdown` → `text`。

### Feishu 实现（CardKit 2.0）

1. **卡片生命周期**：
   - `cardkit.v1.card.create` 创建卡片实例，获取 `card_id`
   - 通过 `im.v1.message.create` 发送卡片引用（`msg_type: "interactive"`, `data: {card_id}`）
   - `cardkit.v1.card.update` 增量更新，带 `sequence` 序号
2. **内容渲染**：
   - 文本块：直接渲染 markdown
   - 工具调用：按工具类型智能摘要（Bash 显示命令、Read/Edit 显示文件路径、Grep 显示搜索词）
   - 超过 N 个工具调用时自动折叠为摘要面板（避免卡片超限）
   - 状态底栏：模型名、token 用量、耗时
3. **卡片大小控制**：
   - 单字段上限 600 字符，工具输出上限 1200 字符
   - 总 body 上限 2500 字符
   - 推理过程上限 1500 字符
   - 超限时裁剪最早的内容块
4. **交互元素**："停止" 按钮，点击后回调 engine 取消当前 run
5. **降级**：卡片创建失败 → markdown 回复

### Engine 集成

```
engine.HandleUserMessage()
  → 检测 StreamingCLI + RichMessenger 是否同时可用
  → 是：走流式路径
    → RichMessenger.CreateRichMessage() 创建卡片（内嵌 reply-to 逻辑，无需单独调 Replyable）
    → CLI SendInputStreaming() 返回事件通道
    → goroutine 消费事件：
      → CLIEvent 到达 → 构建 ContentBlock → RichMessageHandle.Update()
      → CLIEventDone → RichMessageHandle.Finish()
  → 否：走现有路径
    → 检测 Replyable，优先用 SendMessageWithReply() 发送回复
    → 否则用 SendMessage() 发送纯文本
```

**优先级**：RichMessenger > Replyable > SendMessage。RichMessenger 激活时，reply-to 逻辑内嵌在卡片创建中，不单独调用 Replyable。

### 各 channel 未来映射

| Channel | 富回复机制 |
|---------|-----------|
| Feishu | CardKit 2.0 流式卡片 |
| Discord | Embed 消息 + 字段更新 |
| Telegram | 编辑消息 + markdown |
| Slack | Block Kit + update |

---

## P2: 图片与文件处理

### 问题

用户发送截图、文档、图片，当前 adapter 完全忽略非文本消息。

### 方案

1. **下载媒体**：adapter 从平台 API 下载文件（`message_type != text` 时）
2. **过滤规则**：支持 `image`、`file`、`audio`、`video`；显式跳过 `sticker`（贴纸）
3. **附加到 BotMessage**：填充 `Attachments` 字段
4. **Engine 路由**：Engine 将文件引用包含在发给 CLI adapter 的 prompt 中
5. **清理**：按 chat 分目录存储，TTL 过期后批量 GC

### Feishu 实现

- 通过 `im.v1.messageResource.get` 下载
- 存储路径：`~/.clibot/media/feishu/<chat_id>/`
- 文件去重：按 `file_key` 跳过已下载文件
- GC：后台 goroutine 定期扫描，删除超过 TTL 的文件

### Engine 流程

```
飞书图片消息
  → FeishuBot 下载文件，填充 BotMessage.Attachments
  → Engine 检测到 attachments
  → Engine 格式化 prompt（文件引用注入到用户输入中）
  → CLIAdapter.SendInput() 接收增强输入
  → CLI 工具处理多模态输入
  → 临时文件由 GC 定期清理
```

### 配置

```yaml
bots:
  feishu:
    media_dir: ""           # 默认：~/.clibot/media/feishu
    media_ttl: "1h"         # 清理间隔
    max_media_size: "20MB"  # 超过此大小拒绝
```

---

## P3: 消息引用上下文

### 问题

用户回复某条消息时，被引用的内容丢失。Claude 无法理解用户在说什么。

### 方案

1. **检测引用**：`BotMessage.QuoteID` 非空时，engine 调用 `Quotable.FetchQuotedMessage()`
2. **注入上下文**：在用户 prompt 前拼接 `<quoted_message>` 块

### 格式

```
<quoted_message sender="ou_xxx" sender_name="张三" time="2026-05-24T10:30:00Z">
被引用的消息内容
</quoted_message>

用户的实际回复
```

### Engine 集成

```go
if quotable, ok := botAdapter.(Quotable); ok && msg.QuoteID != "" {
    quoted, _ := quotable.FetchQuotedMessage(ctx, msg.Channel, msg.QuoteID)
    msg.Content = formatQuoteBlock(quoted) + "\n\n" + msg.Content
}
```

### Feishu 实现

- 从消息事件提取 `parent_id` → 设置 `QuoteID`
- 通过 `im.v1.message.get` 获取被引用消息
- 解析内容、提取发送者信息

---

## P4: 话题群独立会话

### 问题

飞书话题群中，所有话题共享 `chat_id`。不同话题的用户被路由到同一个 CLI session，上下文混乱。

### 方案

1. **ThreadScope**：实现 `Threadable` 的 channel 提供隔离的 channel ID
2. **Engine 路由**：使用 `ThreadScope()` 返回值作为会话查找的 effective channel ID
3. **向后兼容**：未实现 `Threadable` 的 channel 继续使用 `channelID`

### Engine 集成

```go
effectiveChannel := msg.Channel
if threadable, ok := botAdapter.(Threadable); ok {
    effectiveChannel = threadable.ThreadScope(msg.Channel, msg)
}
```

### Feishu 实现

需先通过 `im.v1.chat.get` 获取群类型（p2p/group/topic），然后判断：
- p2p / 普通 group：scope = `chatID`
- topic group + 有 threadID：scope = `chatID:threadID`

建议缓存群类型查询结果，避免每次消息都调 API。

```go
func (f *FeishuBot) ThreadScope(channelID string, msg BotMessage) string {
    if msg.ChatType == "topic" && msg.ThreadID != "" {
        return channelID + ":" + msg.ThreadID
    }
    return channelID
}
```

---

## P5: 云文档评论回复

### 问题

用户在飞书云文档中 @bot，bot 完全忽略这些事件。

### 状态

低优先级，Feishu 特有场景，不抽象为接口。

### 实现要点

- 注册 `im.message.receive_v1` 处理器，过滤 doc comment 来源的事件
- Wiki 节点需解析 `obj_token` / `obj_type`（wiki token → 实际文档 token 的映射）
- 通过 Drive API `fileComment.get` 获取评论上下文，失败时回退 `fileComment.list` 分页
- 以文档评论方式回复（不同于聊天消息的 API 路径：`drive/v2/comment`）
- strip markdown（飞书文档评论格式支持有限）
- 错误码 1069302 表示回复线程已存在，需回退为顶级评论
- 支持文件类型：doc, docx, sheet, file（slide/bitable 排除）
- 每个云文档独立 session（scope: `doc:<fileToken>`）

---

## P6: 消息防抖

### 问题

用户快速连发多条消息，每条触发一次 CLI run，浪费资源且响应碎片化。

### 方案

1. **按 channel 防抖**：在静默窗口内累积消息，窗口结束后合并为一条输入
2. **运行中阻塞**：当前 run 未完成时，新消息入队但**不启动计时器**。run 结束后**重新开始**静默窗口（不是立即 flush）
3. **通用中间件**：在 engine 层实现，所有 channel 复用

### Engine 集成

```go
type PendingQueue struct {
    windows map[string]*debounceWindow  // 按 effective channel 索引
    mu      sync.Mutex
}

type debounceWindow struct {
    messages []BotMessage
    timer    *time.Timer
    blocked  bool  // 有活跃 run 时为 true
}

// engine 检查 adapter 是否支持防抖：
if deb, ok := botAdapter.(Debounceable); ok && deb.DebounceWindow() > 0 {
    q.Add(effectiveChannel, msg, deb.DebounceWindow())
} else {
    // 直接处理
}

// run 开始时：
q.Block(effectiveChannel)

// run 结束时：
q.Unblock(effectiveChannel)  // 解除阻塞并启动新的静默窗口
```

### 配置

```yaml
bots:
  feishu:
    debounce_ms: 600   # 静默窗口（0 = 禁用）
```

---

## P7: Bridge 上下文注入

### 问题

当前用户消息直接传给 CLI，Claude 不知道消息来自哪个群、谁发的、群类型是什么。

### 方案

在用户消息前注入 `<bridge_context>` XML 块，提供对话元数据：

```xml
<bridge_context chat_id="oc_xxx" chat_type="group" sender_id="ou_xxx" sender_name="张三" thread_id="">
用户消息内容
</bridge_context>
```

### Engine 集成

```go
// engine 在发送给 CLI 之前注入上下文
if msg.Platform != "" {
    msg.Content = fmt.Sprintf(
        "<bridge_context chat_id=%q chat_type=%q sender_id=%q sender_name=%q thread_id=%q>\n%s\n</bridge_context>",
        msg.Channel, msg.ChatType, msg.UserID, msg.SenderName, msg.ThreadID, msg.Content,
    )
}
```

这是平台无关的通用能力，不需要接口定义，所有 channel 自动受益。

---

## P8: @提及策略

### 问题

群聊中 bot 对所有消息都响应，包括与它无关的对话。应只在被 @时才响应。

### 方案

1. **策略接口**：`MentionPolicy.ShouldRespond(msg)` 判断是否响应
2. **默认行为**：p2p 总是响应；群聊中只响应 @bot 消息
3. **可配置**：`mention_in_group: true`（默认）表示群聊需要 @

### Engine 集成

```go
// engine 在消息处理前检查
if policy, ok := botAdapter.(MentionPolicy); ok {
    if !policy.ShouldRespond(msg) {
        return // 静默忽略
    }
}
```

### Feishu 实现

- 消息事件中检查是否包含 `@_user_1`（bot 的 mention 标记）
- 群聊 + `mention_in_group: true` + 无 @ → 忽略
- `@所有人` 永远不响应
- p2p 不受此策略影响

### 配置

```yaml
bots:
  feishu:
    mention_in_group: true   # 群聊是否需要 @bot 才响应（默认 true）
```

---

## P9: 回复引用线

### 问题

bot 的回复和用户的提问在聊天流中没有视觉关联，难以对应。

### 方案

1. **引用接口**：`Replyable.SendMessageWithReply(channel, message, replyToMessageID)`
2. **Topic 群特殊处理**：在话题群中回复必须落在同一话题内（`replyInThread: true`）

### Engine 集成

```go
// engine 发送响应时优先使用引用回复
if replyable, ok := botAdapter.(Replyable); ok && msg.MessageID != "" {
    replyable.SendMessageWithReply(msg.Channel, response, msg.MessageID)
} else {
    botAdapter.SendMessage(msg.Channel, response)
}
```

---

## P10: 并发池控制

### 问题

多个群同时触发 CLI run，可能耗尽资源。

### 方案

全局 FIFO 信号量，限制同时运行的 CLI 数量（默认 10）。

### Engine 集成

```go
type ProcessPool struct {
    sem  chan struct{}  // 缓冲 channel 作为信号量
    cap  int
}

// engine 在启动 run 前获取许可：
pool.Acquire()
defer pool.Release()

// 等待的 run 排队，有许可空出后自动执行
```

### 配置

```yaml
session:
  max_concurrent_runs: 10   # 全局最大并发 CLI run 数
```

---

## P11: Keepalive 增强

### 问题

当前依赖 Go SDK 的 `ws.WithAutoReconnect(true)`，无应用层探测。网络波动可能导致静默断连。

### 方案

1. **应用层心跳**：15s 定时探测，检测睡眠恢复（>30s gap 重置计数器）
2. **预检机制**：检测到断连时先 HTTP 探测飞书 API 可达性，避免在网络整体不可用时频繁重连
3. **计数器防抖**：连续 3 次探测失败才触发强制重连，避免偶发丢包误判

### Feishu 实现

在 `feishu.go` 中新增 keepalive goroutine，独立于 SDK 的内部 ping。

---

## 质量要求

每个功能点（P1-P11）完成后必须满足以下条件，方可视为完成：

### 1. 代码质量
- 逻辑正确，简洁优雅，无冗余代码
- 遵循项目现有代码风格和命名规范
- 错误处理清晰，不吞异常、不静默失败
- 无竞态条件（goroutine 安全、锁粒度合理）

### 2. 单元测试
- 每个新增函数/方法必须有对应测试
- 使用 `testify` + 表驱动测试覆盖正常路径和边界条件
- Mock 外部依赖（飞书 API、CLI adapter），不依赖真实网络调用
- 测试覆盖率目标：新增代码 ≥ 80%
- 测试文件与源文件同目录（如 `feishu/quote.go` → `feishu/quote_test.go`）

### 3. Codex Review
- 每个功能点完成后执行 `codex-review` 进行多轮 review
- Review 重点：逻辑正确性、接口设计合理性、测试充分性、边界处理
- Review 发现的问题必须在同一功能点内修复，不遗留到下一阶段

### 4. 构建验证
- `make fmt` 通过（代码格式化）
- `go test ./...` 全量通过（无失败用例）
- `go build ./...` 编译通过（无错误）
- 以上三项作为每个功能点的 done gate，不通过不进入下一个功能点

### 完成流程（每个功能点）

```
编码 + 单元测试
  → make fmt && go build ./...
  → go test ./...
  → codex-review（多轮，直到无新问题）
  → 修复 review 发现的问题
  → 重新 make fmt && go test ./...
  → git commit（一个功能点一个 commit，类型前缀 feat/fix/refactor）
  → 标记完成，进入下一个功能点
```

### Commit 规则

- 每个功能点完成后单独 commit，不混合多个特性
- commit message 格式：`<type>: <description>`（不超过 150 字符）
- type: `feat`（新功能）、`fix`（修复）、`refactor`（重构）
- 示例：
  ```
  feat: add optional interfaces for rich channel capabilities
  feat: add bridge context injection for all channels
  feat: add feishu message quote context support
  feat: add feishu thread-scoped session routing
  feat: add feishu mention policy for group chats
  feat: add message debounce middleware
  ```

---

## 实施计划

| 阶段 | 特性 | 预计工时 |
|------|------|---------|
| Phase 1 | 接口定义 + P7（上下文注入）+ P3（引用）+ P4（话题）+ P8（@提及）+ P6（防抖） | 4-5 天 |
| Phase 2 | P2（图片/文件）+ P9（回复引用线）+ P11（keepalive） | 3-4 天 |
| Phase 3 | P1（流式富回复）— CLIEvent 协议 + RichMessenger + Feishu CardKit | 7-10 天 |
| Phase 4 | P10（并发池）+ P5（云文档） | 4-5 天 |

Phase 1 优先：定义接口契约 + 低成本高价值特性。P7 上下文注入最简单且对所有 channel 都有益，放在最前面。

### Engine 变更策略

P3/P4/P6/P7/P8/P9 共 6 个特性都需要改动 `engine.go`。为避免反复修改同一文件造成冲突，Phase 1 中将 engine 侧的改动**一次性完成**：

1. **接口定义**（`internal/bot/interface.go`）— 所有可选接口 + 通用类型
2. **BotMessage 扩展**（`internal/bot/interface.go`，BotMessage 现有定义在此文件）— 新增可选字段
3. **Engine 检测逻辑**（`internal/core/engine.go`）— 在 `HandleUserMessage` 中统一添加：
   - ThreadScope 解析（P4）
   - MentionPolicy 过滤（P8）
   - Bridge context 注入（P7）
   - Quotable 引用注入（P3）
   - Debounce 路由（P6）
   - Replyable 发送（P9，非流式模式）
   - RichMessenger + StreamingCLI 流式路径（P1，仅 Phase 3）
4. **配置解析**（`internal/core/types.go`）— BotConfig 新增字段及默认值
5. **StreamingCLI 接口**（`internal/cli/interface.go`）— CLI 层可选接口（Phase 3）

Phase 1 commit 拆分：
- **commit 1**：接口定义 + BotMessage 扩展 + BotConfig 新增字段 + engine 检测框架（无任何 feishu 代码，纯通用基础设施）
- **commit 2-N**：每个 feishu 实现单独 commit（P7 注入、P3 引用、P4 话题、P8 提及、P6 防抖）

Phase 2 及之后只新增 feishu 子包的实现代码和 adapter 注册逻辑，不再改动 engine。

---

## 文件组织

```
internal/bot/
  interface.go                # BotAdapter + 可选接口（通用）
  types.go                    # BotMessage, ContentBlock, Attachment 等（通用）
  debounce.go                 # PendingQueue（通用，可复用）
  process_pool.go             # ProcessPool 并发池（通用）
  discord.go                  # 简单 adapter 保持 flat
  telegram.go                 # 简单 adapter 保持 flat
  dingtalk.go                 # 简单 adapter 保持 flat
  qq.go                       # 简单 adapter 保持 flat
  weixin.go                   # 简单 adapter 保持 flat
  feishu/                     # Feishu 完整实现（独立子包）
    bot.go                    # FeishuBot 结构体 + BotAdapter 实现（从 bot/feishu.go 迁入）
    card.go                   # RichMessenger 实现（CardKit 流式卡片）
    card_builder.go           # 卡片 JSON 构建（ContentBlock → CardKit 元素）
    media.go                  # 媒体下载 + GC + MediaSupporter 实现
    quote.go                  # Quotable 实现（消息引用获取）
    thread.go                 # Threadable 实现（话题群隔离 + 群类型缓存）
    mention.go                # MentionPolicy 实现（@提及策略）
    reply.go                  # Replyable 实现（回复引用线）
    keepalive.go              # 应用层心跳 + 重连逻辑
    types.go                  # Feishu 特有类型（卡片结构、事件辅助等）
internal/cli/
  interface.go                # CLIAdapter + StreamingCLI 可选接口
internal/core/
  engine.go                   # Engine 检测逻辑（Phase 1 一次性完成）
  types.go                    # BotConfig 新增字段
```

### 子包策略

- 复杂 adapter（需要多个实现文件）独立为子包：`internal/bot/feishu/`
- 简单 adapter（单文件足够）保持 flat：`internal/bot/discord.go`
- serve.go 注册改为 `feishu.NewBot(...)` 返回 `bot.BotAdapter` 接口
- 可选接口（RichMessenger 等）定义在 `bot` 包，实现在 `feishu` 包 — Go 接口断言天然支持

---

## 架构约束

1. **仅可选接口** — 新能力通过类型断言检测，不作为必须方法
2. **默认 no-op 回退** — 未实现富接口的 channel 无需任何改动
3. **BotMessage 新增字段均为可选** — `ThreadID`、`QuoteID`、`ChatType`、`SenderName`、`Attachments` 默认零值
4. **Engine 层编排** — 防抖、引用注入、富回复驱动、上下文注入均在 engine 中，非 per-adapter
5. **不引入新的外部 Go 依赖**，仅使用现有 `larksuite/oapi-sdk-go`
6. **所有代码和文档使用英文**（开源项目要求），本 PRD 除外
7. **配置向后兼容** — 新增配置项必须有合理默认值，旧配置文件不添加这些字段也能正常运行。默认值：`reply_mode: "text"`、`debounce_ms: 0`（禁用）、`mention_in_group: true`、`max_concurrent_runs: 0`（无限制）

---

## 与 feishu-bridge 的特性对照

以下为 feishu-bridge 有但本 PRD **有意不纳入**的特性，及原因：

| feishu-bridge 特性 | 不纳入原因 |
|-------------------|-----------|
| 斜杠命令系统（/new, /reset, /cd, /ws, /resume 等） | clibot 已有自己的命令系统（snew, suse, slist 等），功能不同但定位重叠，后续可按需扩展 |
| 首次运行向导（QR 码注册） | clibot 使用配置文件，不走 QR 码注册流程 |
| Daemon 守护进程（systemd/launchd） | 部署层面，不属于 channel 增强范畴 |
| 多进程注册表 | clibot 是单进程架构 |
| 加密密钥库（AES-256-GCM） | 凭证管理属于全局安全策略，非 channel 特有 |
| lark-cli 集成（Claude 调用飞书 API） | 依赖外部工具，且是 Claude tool call 层面的集成，非 channel 层面 |
| 账号热切换（/account change） | 凭证管理范畴 |
| AI 诊断（/doctor） | 可作为独立功能后续考虑 |
| 创建私聊群（/new chat） | 低频场景 |
