# 动态 Session 管理 - 任务分解

**相关文档**: [2026-02-06-dynamic-session-management.md](./2026-02-06-dynamic-session-management.md)
**创建日期**: 2026-02-06
**状态**: 待开始

---

## 任务概览

共 **10 个任务**，预计完成时间：2-3 天

```
基础架构 (2) → 核心功能 (2) → 用户界面 (3) → 安全完善 (3) → 测试 (1)
```

---

## Task 1: 扩展数据结构

**优先级**: P0
**预计时间**: 30 分钟
**状态**: ⏸️ 待开始

### 描述
添加动态 session 支持所需的数据结构字段

### 实施步骤

1. **修改 Session 结构体** (`internal/core/types.go`)
   ```go
   type Session struct {
       Name      string
       CLIType   string
       WorkDir   string
       StartCmd  string
       State     SessionState
       CreatedAt string
       IsDynamic bool           // 新增：标记是否为动态创建
       CreatedBy string         // 新增：创建者 "platform:userID"
       cancelCtx context.CancelFunc
   }
   ```

2. **修改 SessionGlobalConfig 结构体** (`internal/core/types.go`)
   ```go
   type SessionGlobalConfig struct {
       InputHistorySize   int `yaml:"input_history_size"`
       MaxDynamicSessions int `yaml:"max_dynamic_sessions"` // 新增
   }
   ```

3. **更新配置文件** (`configs/config.yaml`)
   ```yaml
   session:
     input_history_size: 10
     max_dynamic_sessions: 50  # 新增
   ```

4. **更新 engine 初始化** (`internal/core/engine.go`)
   - 如果 MaxDynamicSessions == 0，设置默认值为 50

### 验收标准
- [ ] 字段编译成功
- [ ] 没有破坏现有代码
- [ ] 所有现有测试通过

### 依赖
- 无

---

## Task 2: 注册 new 和 delete 命令

**优先级**: P0
**预计时间**: 20 分钟
**状态**: ⏸️ 待开始

### 依赖
- Task 1

### 描述
在引擎中注册新的特殊命令

### 实施步骤

1. **更新 specialCommands 映射** (`internal/core/engine.go:36`)
   ```go
   var specialCommands = map[string]struct{}{
       "help":     {},
       "status":   {},
       "sessions": {},
       "whoami":   {},
       "view":     {},
       "echo":     {},
       "new":      {}, // 新增
       "delete":   {}, // 新增
   }
   ```

2. **添加命令路由** (`HandleSpecialCommandWithArgs`)
   ```go
   switch command {
   // ... existing cases
   case "new":
       e.handleNewSession(args, msg)
   case "delete":
       e.handleDeleteSession(args, msg)
   default:
       // unknown command
   }
   ```

3. **创建存根方法** (`internal/core/engine.go`)
   ```go
   func (e *Engine) handleNewSession(args []string, msg bot.BotMessage) {
       e.SendToBot(msg.Platform, msg.Channel,
           "⚠️  'new' command not implemented yet")
   }

   func (e *Engine) handleDeleteSession(args []string, msg bot.BotMessage) {
       e.SendToBot(msg.Platform, msg.Channel,
           "⚠️  'delete' command not implemented yet")
   }
   ```

### 验收标准
- [ ] 命令已注册并可调用
- [ ] 输入 "new" 或 "delete" 返回消息
- [ ] 无编译错误

---

## Task 3: 实现 new session 创建逻辑

**优先级**: P0
**预计时间**: 2-3 小时
**状态**: ⏸️ 待开始

### 依赖
- Task 1
- Task 2

### 描述
实现完整的 session 创建功能

### 实施步骤

1. **权限检查**
   ```go
   if !e.config.IsAdmin(msg.Platform, msg.UserID) {
       e.SendToBot(msg.Platform, msg.Channel, "❌ Permission denied: admin only")
       return
   }
   ```

2. **参数解析**
   ```go
   if len(args) < 3 {
       e.SendToBot(msg.Platform, msg.Channel,
           "❌ Invalid arguments\nUsage: new <name> <cli_type> <work_dir> [start_cmd]")
       return
   }

   name := args[0]
   cliType := args[1]
   workDir := args[2]
   startCmd := cliType
   if len(args) >= 4 {
       startCmd = args[3]
   }
   ```

3. **参数验证**
   - 名称非空且格式合法（只包含字母、数字、连字符、下划线）
   - CLI 类型在 cliAdapters 中存在
   - 工作目录存在且可访问
   - 路径安全检查（防止 `../../../etc/passwd`）

4. **重复检查**
   ```go
   e.sessionMu.Lock()
   if _, exists := e.sessions[name]; exists {
       e.sessionMu.Unlock()
       e.SendToBot(msg.Platform, msg.Channel,
           fmt.Sprintf("❌ Session '%s' already exists", name))
       return
   }
   ```

5. **创建 session 对象**
   ```go
   session := &Session{
       Name:      name,
       CLIType:   cliType,
       WorkDir:   workDir,
       StartCmd:  startCmd,
       State:     StateIdle,
       CreatedAt: time.Now().Format(time.RFC3339),
       IsDynamic: true,
       CreatedBy: fmt.Sprintf("%s:%s", msg.Platform, msg.UserID),
   }
   ```

6. **调用 adapter 创建**
   ```go
   adapter := e.cliAdapters[cliType]
   if err := adapter.CreateSession(name, workDir, startCmd); err != nil {
       e.sessionMu.Unlock()
       e.SendToBot(msg.Platform, msg.Channel,
           fmt.Sprintf("❌ Failed to create session: %v", err))
       return
   }
   ```

7. **加入 sessions map**
   ```go
   e.sessions[name] = session
   e.sessionMu.Unlock()
   ```

8. **成功反馈**
   ```go
   e.SendToBot(msg.Platform, msg.Channel,
       fmt.Sprintf("✅ Session '%s' created successfully\nCLI: %s\nWorkDir: %s\nStartCmd: %s",
           name, cliType, workDir, startCmd))
   ```

### 错误消息规范

| 场景 | 错误消息 |
|------|---------|
| 权限不足 | `❌ Permission denied: admin only` |
| 参数不足 | `❌ Invalid arguments\nUsage: new <name> <cli_type> <work_dir> [start_cmd]` |
| 无效 CLI 类型 | `❌ Invalid CLI type: 'xxx' (supported: claude, gemini, opencode)` |
| 目录不存在 | `❌ Work directory does not exist: /path/to/dir` |
| Session 已存在 | `❌ Session 'xxx' already exists` |
| 达到上限 | `❌ Maximum dynamic session limit reached (50)` |
| 名称格式错误 | `❌ Invalid session name: 'xxx' (use letters, numbers, hyphen, underscore only)` |

### 验收标准
- [ ] Admin 可以通过 IM 创建 session
- [ ] 非 Admin 收到权限错误
- [ ] 无效参数显示有帮助的错误消息
- [ ] 创建的 session 出现在 sessions/status 输出中
- [ ] 创建的 session 可以正常使用

---

## Task 4: 实现 delete session 逻辑

**优先级**: P0
**预计时间**: 1-2 小时
**状态**: ⏸️ 待开始

### 依赖
- Task 1
- Task 2

### 描述
实现动态 session 删除功能

### 实施步骤

1. **权限检查**
   ```go
   if !e.config.IsAdmin(msg.Platform, msg.UserID) {
       e.SendToBot(msg.Platform, msg.Channel, "❌ Permission denied: admin only")
       return
   }
   ```

2. **参数解析**
   ```go
   if len(args) < 1 {
       e.SendToBot(msg.Platform, msg.Channel,
           "❌ Invalid arguments\nUsage: delete <name>")
       return
   }

   name := args[0]
   ```

3. **检查 session 是否存在**
   ```go
   e.sessionMu.Lock()
   session, exists := e.sessions[name]
   if !exists {
       e.sessionMu.Unlock()
       e.SendToBot(msg.Platform, msg.Channel,
           fmt.Sprintf("❌ Session '%s' not found", name))
       return
   }
   ```

4. **只允许删除动态 session**
   ```go
   if !session.IsDynamic {
       e.sessionMu.Unlock()
       e.SendToBot(msg.Platform, msg.Channel,
           fmt.Sprintf("❌ Cannot delete configured session '%s'\n"+
               "Please remove it from the config file manually", name))
       return
   }
   ```

5. **终止 tmux session**
   ```go
   cmd := exec.Command("tmux", "kill-session", "-t", name)
   if err := cmd.Run(); err != nil {
       logger.WithField("error", err).Warn("failed-to-kill-tmux-session")
   }
   ```

6. **从内存中移除**
   ```go
   delete(e.sessions, name)
   e.sessionMu.Unlock()
   ```

7. **成功反馈**
   ```go
   e.SendToBot(msg.Platform, msg.Channel,
       fmt.Sprintf("✅ Session '%s' deleted successfully", name))
   ```

### 错误消息规范

| 场景 | 错误消息 |
|------|---------|
| 权限不足 | `❌ Permission denied: admin only` |
| 参数不足 | `❌ Invalid arguments\nUsage: delete <name>` |
| Session 不存在 | `❌ Session 'xxx' not found` |
| 不能删除静态 Session | `❌ Cannot delete configured session 'xxx'\nPlease remove it from the config file manually` |

### 验收标准
- [ ] Admin 可以删除动态 session
- [ ] 不能删除静态（配置文件中的）session
- [ ] 删除的 session 从列表中消失
- [ ] tmux session 被正确清理

---

## Task 5: 更新 status 命令显示

**优先级**: P1
**预计时间**: 30 分钟
**状态**: ⏸️ 待开始

### 依赖
- Task 1

### 描述
修改 status 命令以显示 session 来源标记

### 实施步骤

1. **修改 showStatus 方法** (`internal/core/engine.go:494`)
   ```go
   response := "📊 clibot Status:\n\n"
   response += "Sessions:\n"
   for _, session := range e.sessions {
       alive := false
       if adapter, exists := e.cliAdapters[session.CLIType]; exists {
           alive = adapter.IsSessionAlive(session.Name)
       }
       status := "❌"
       if alive {
           status = "✅"
       }

       // 添加来源标记
       origin := "[static]"
       if session.IsDynamic {
           origin = fmt.Sprintf("[dynamic, created by %s]", session.CreatedBy)
       }

       response += fmt.Sprintf("  %s %s (%s) - %s %s\n",
           status, session.Name, session.CLIType, session.State, origin)
   }
   ```

### 输出格式示例
```
📊 clibot Status:

Sessions:
  ✅ project-a (claude) - idle [static]
  ✅ temp-test (claude) - processing [dynamic, created by discord:123456789]
  ❌ offline-session (gemini) - error [static]
```

### 验收标准
- [ ] status 显示 [static] 标记用于配置的 session
- [ ] status 显示 [dynamic, created by ...] 用于动态 session
- [ ] 格式清晰易读

---

## Task 6: 更新 sessions 命令显示

**优先级**: P1
**预计时间**: 30 分钟
**状态**: ⏸️ 待开始

### 依赖
- Task 1

### 描述
修改 sessions 命令以分类显示 session

### 实施步骤

1. **修改 listSessions 方法** (`internal/core/engine.go:481`)
   ```go
   func (e *Engine) listSessions(msg bot.BotMessage) {
       e.sessionMu.RLock()
       defer e.sessionMu.RUnlock()

       response := "📋 Available Sessions:\n\n"

       // 分类显示
       var staticSessions, dynamicSessions []*Session
       for _, session := range e.sessions {
           if session.IsDynamic {
               dynamicSessions = append(dynamicSessions, session)
           } else {
               staticSessions = append(staticSessions, session)
           }
       }

       // 静态 Session
       if len(staticSessions) > 0 {
           response += "Static Sessions (configured):\n"
           for _, session := range staticSessions {
               response += fmt.Sprintf("  • %s (%s) - %s [static]\n",
                   session.Name, session.CLIType, session.State)
           }
           response += "\n"
       }

       // 动态 Session
       if len(dynamicSessions) > 0 {
           response += "Dynamic Sessions (created via IM):\n"
           for _, session := range dynamicSessions {
               response += fmt.Sprintf("  • %s (%s) - %s [dynamic, created by %s]\n",
                   session.Name, session.CLIType, session.State, session.CreatedBy)
           }
       }

       e.SendToBot(msg.Platform, msg.Channel, response)
   }
   ```

### 输出格式示例
```
📋 Available Sessions:

Static Sessions (configured):
  • project-a (claude) - idle [static]
  • backend (gemini) - processing [static]

Dynamic Sessions (created via IM):
  • temp-test (claude) - idle [dynamic, created by discord:123456789]
  • quick-debug (opencode) - processing [dynamic, created by telegram:987654321]
```

### 验收标准
- [ ] Session 被正确分类
- [ ] 来源标记被显示
- [ ] 格式符合规范

---

## Task 7: 更新帮助文档

**优先级**: P1
**预计时间**: 15 分钟
**状态**: ⏸️ 待开始

### 依赖
- Task 2

### 描述
在 help 命令中添加 new 和 delete 的说明

### 实施步骤

1. **修改 showHelp 方法** (`internal/core/engine.go:529`)
   ```go
   help := `📖 **clibot Help**

**Special Commands** (no prefix required):
  help         - Show this help message
  sessions     - List all available sessions
  status       - Show status of all sessions
  whoami       - Show current session info
  view [n]     - View CLI output (default: 20 lines)
  echo         - Echo your IM user info (for whitelist config)
  new <name> <cli_type> <work_dir> [cmd] - Create new session (admin only)
  delete <name> - Delete dynamic session (admin only)

... rest of help
`
   ```

### 验收标准
- [ ] help 显示新命令
- [ ] 使用格式正确
- [ ] Admin 要求被注明

---

## Task 8: 实现资源限制检查

**优先级**: P2
**预计时间**: 30 分钟
**状态**: ⏸️ 待开始

### 依赖
- Task 1
- Task 3

### 描述
添加动态 session 数量限制，防止资源耗尽

### 实施步骤

1. **读取配置** (engine 初始化中)
   ```go
   maxDynamicSessions := e.config.Session.MaxDynamicSessions
   if maxDynamicSessions == 0 {
       maxDynamicSessions = 50 // 默认值
   }
   ```

2. **在 handleNewSession 中检查**
   ```go
   // 计算当前动态 session 数量
   dynamicCount := 0
   for _, s := range e.sessions {
       if s.IsDynamic {
           dynamicCount++
       }
   }

   if dynamicCount >= e.config.Session.MaxDynamicSessions {
       e.sessionMu.Unlock()
       e.SendToBot(msg.Platform, msg.Channel,
           fmt.Sprintf("❌ Maximum dynamic session limit reached (%d)",
               e.config.Session.MaxDynamicSessions))
       return
   }
   ```

3. **确保线程安全**
   - 检查在 sessionMu.Lock() 保护下进行

### 验收标准
- [ ] 不能创建超过 MaxDynamicSessions 个动态 session
- [ ] 限制可通过 config.yaml 配置
- [ ] 数量检查是线程安全的

---

## Task 9: 添加审计日志

**优先级**: P2
**预计时间**: 20 分钟
**状态**: ⏸️ 待开始

### 依赖
- Task 3
- Task 4

### 描述
为创建和删除操作添加完整的审计日志

### 实施步骤

1. **创建 session 日志** (handleNewSession)
   ```go
   logger.WithFields(logrus.Fields{
       "action":     "create_session",
       "session":    name,
       "platform":   msg.Platform,
       "user_id":    msg.UserID,
       "cli_type":   cliType,
       "work_dir":   workDir,
       "start_cmd":  startCmd,
       "is_dynamic": true,
   }).Info("admin-created-dynamic-session")
   ```

2. **删除 session 日志** (handleDeleteSession)
   ```go
   logger.WithFields(logrus.Fields{
       "action":   "delete_session",
       "session":  name,
       "platform": msg.Platform,
       "user_id":  msg.UserID,
   }).Info("admin-deleted-dynamic-session")
   ```

### 验收标准
- [ ] 所有创建操作都被记录
- [ ] 所有删除操作都被记录
- [ ] 日志包含 session 详细信息和用户身份

---

## Task 10: 编写综合测试

**优先级**: P0
**预计时间**: 2-3 小时
**状态**: ⏸️ 待开始

### 依赖
- Task 3
- Task 4

### 描述
为新功能编写完整的单元测试和集成测试

### 测试用例

#### new session 测试
```go
func TestEngine_HandleNewSession_Success(t *testing.T)
func TestEngine_HandleNewSession_PermissionDenied(t *testing.T)
func TestEngine_HandleNewSession_DuplicateSession(t *testing.T)
func TestEngine_HandleNewSession_InvalidCLIType(t *testing.T)
func TestEngine_HandleNewSession_WorkDirNotExists(t *testing.T)
func TestEngine_HandleNewSession_MaxSessionsReached(t *testing.T)
func TestEngine_HandleNewSession_InvalidSessionName(t *testing.T)
func TestEngine_HandleNewSession_PathTraversalAttack(t *testing.T)
func TestEngine_HandleNewSession_EmptySessionName(t *testing.T)
func TestEngine_HandleNewSession_MissingArguments(t *testing.T)
```

#### delete session 测试
```go
func TestEngine_HandleDeleteSession_Success(t *testing.T)
func TestEngine_HandleDeleteSession_PermissionDenied(t *testing.T)
func TestEngine_HandleDeleteSession_StaticSession(t *testing.T)
func TestEngine_HandleDeleteSession_SessionNotFound(t *testing.T)
func TestEngine_HandleDeleteSession_MissingArgument(t *testing.T)
```

#### 集成测试
```go
func TestDynamicSessionLifecycle(t *testing.T)
func TestMultipleDynamicSessions(t *testing.T)
func TestCreateAndUseDynamicSession(t *testing.T)
func TestDeleteAndVerifyRemoval(t *testing.T)
```

### 验收标准
- [ ] 所有单元测试通过
- [ ] 边缘情况被覆盖
- [ ] 安全场景被测试
- [ ] 测试覆盖率 > 80%

---

## 执行顺序

### 第 1 阶段：基础设施 (Day 1 上午)
- Task 1: 扩展数据结构
- Task 2: 注册命令

### 第 2 阶段：核心功能 (Day 1 下午 - Day 2 上午)
- Task 3: 实现 new 命令
- Task 4: 实现 delete 命令

### 第 3 阶段：用户界面 (Day 2 下午)
- Task 5: 更新 status
- Task 6: 更新 sessions
- Task 7: 更新 help

### 第 4 阶段：安全完善 (Day 3 上午)
- Task 8: 资源限制
- Task 9: 审计日志

### 第 5 阶段：测试验证 (Day 3 下午)
- Task 10: 编写测试

---

## 进度追踪

| Task | 状态 | 完成时间 |
|------|------|----------|
| 1. 扩展数据结构 | ⏸️ 待开始 | - |
| 2. 注册命令 | ⏸️ 待开始 | - |
| 3. 实现 new 命令 | ⏸️ 待开始 | - |
| 4. 实现 delete 命令 | ⏸️ 待开始 | - |
| 5. 更新 status | ⏸️ 待开始 | - |
| 6. 更新 sessions | ⏸️ 待开始 | - |
| 7. 更新 help | ⏸️ 待开始 | - |
| 8. 资源限制 | ⏸️ 待开始 | - |
| 9. 审计日志 | ⏸️ 待开始 | - |
| 10. 编写测试 | ⏸️ 待开始 | - |

---

## 备注

### 状态图标
- ⏸️ 待开始
- 🚧 进行中
- ✅ 已完成
- ❌ 已取消
- ⚠️ 被阻塞

### 优先级
- **P0**: 核心功能，必须实现
- **P1**: 重要功能，增强体验
- **P2**: 完善功能，可选实现
