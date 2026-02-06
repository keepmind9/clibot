# 动态 Session 管理功能设计

**版本**: v1.0
**日期**: 2026-02-06
**状态**: 设计阶段
**优先级**: P1

---

## 1. 功能概述

### 1.1 背景

当前 clibot 的 session 必须在配置文件中预先定义，不够灵活。用户需要：
- 快速创建临时测试 session
- 无需重启 clibot 即可添加新项目
- 动态管理 session 生命周期

### 1.2 目标

实现 `new` 和 `delete` 特殊命令，支持：
- 通过 IM 动态创建 session（仅内存）
- 删除动态创建的 session
- 与现有 session 管理无缝集成

### 1.3 范围

**包含**：
- `new` 命令：创建动态 session
- `delete` 命令：删除动态 session
- Session 标记：区分静态/动态
- 状态显示优化：显示 session 来源

**不包含**：
- Session 持久化到配置文件（未来可扩展）
- Session 模板功能
- Session 批量操作

---

## 2. 方案设计

### 2.1 核心决策

| 决策点 | 选择方案 | 说明 |
|--------|---------|------|
| 持久化策略 | 仅内存（临时） | 重启后丢失，简单无副作用 |
| 参数格式 | 位置参数 | `new <name> <cli_type> <work_dir> [start_cmd]` |
| 权限控制 | 仅 Admin | 安全优先，防止滥用 |
| 工作目录 | 必须指定 | 避免误操作，明确意图 |
| 名称冲突 | 严格拒绝 | 不提供自动替换，安全优先 |
| 删除功能 | 只删除动态的 | 配置文件中的 session 需手动修改 |
| 资源限制 | 全局限制 | `max_dynamic_sessions: 50` |
| Session 标记 | 显示标记 | status 中显示 [static] / [dynamic] |

### 2.2 命令规范

#### new 命令
```
用法：new <name> <cli_type> <work_dir> [start_cmd]

参数：
  name        Session 名称（必填，不能重复）
  cli_type    CLI 类型（必填：claude/gemini/opencode）
  work_dir    工作目录（必填，必须存在）
  start_cmd   启动命令（可选，默认为 cli_type）

示例：
  new myproject claude ~/projects/myproject
  new backend gemini ~/backend my-custom-gemini

限制：
  - 需要 admin 权限
  - 最多 50 个动态 session
  - 不能与现有 session 重名

错误处理：
  ❌ Permission denied: admin only
  ❌ Invalid CLI type: 'xxx' (supported: claude, gemini, opencode)
  ❌ Work directory does not exist: /path/to/dir
  ❌ Session 'xxx' already exists
  ❌ Maximum dynamic session limit reached (50)
  ❌ Session name contains invalid characters
```

#### delete 命令
```
用法：delete <name>

参数：
  name        Session 名称（必填）

示例：
  delete temp-test

限制：
  - 需要 admin 权限
  - 只能删除动态创建的 session

错误处理：
  ❌ Permission denied: admin only
  ❌ Session 'xxx' not found
  ❌ Cannot delete configured session 'xxx'
```

---

## 3. 技术设计

### 3.1 数据结构变更

#### Session 结构体
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

#### SessionGlobalConfig
```go
type SessionGlobalConfig struct {
    InputHistorySize      int `yaml:"input_history_size"`
    MaxDynamicSessions    int `yaml:"max_dynamic_sessions"` // 新增：动态 session 上限
}
```

### 3.2 配置文件更新

#### configs/config.yaml
```yaml
session:
  input_history_size: 10
  max_dynamic_sessions: 50  # 新增：动态创建的 session 上限
```

### 3.3 命令注册

#### specialCommands 映射
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

### 3.4 核心逻辑

#### new 命令处理流程
```go
func (e *Engine) handleNewSession(args []string, msg bot.BotMessage) {
    // 1. 权限检查
    if !e.config.IsAdmin(msg.Platform, msg.UserID) {
        e.SendToBot(msg.Platform, msg.Channel, "❌ Permission denied: admin only")
        return
    }

    // 2. 参数解析
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

    // 3. 参数验证
    // - 名称格式检查
    // - CLI 类型检查
    // - 目录存在性检查
    // - 路径安全检查

    // 4. 资源限制检查
    e.sessionMu.Lock()
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

    // 5. 创建 session
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

    // 6. 启动 session
    adapter := e.cliAdapters[cliType]
    if err := adapter.CreateSession(name, workDir, startCmd); err != nil {
        e.sessionMu.Unlock()
        e.SendToBot(msg.Platform, msg.Channel,
            fmt.Sprintf("❌ Failed to create session: %v", err))
        return
    }

    e.sessions[name] = session
    e.sessionMu.Unlock()

    // 7. 成功反馈
    e.SendToBot(msg.Platform, msg.Channel,
        fmt.Sprintf("✅ Session '%s' created successfully\nCLI: %s\nWorkDir: %s\nStartCmd: %s",
            name, cliType, workDir, startCmd))
}
```

#### delete 命令处理流程
```go
func (e *Engine) handleDeleteSession(args []string, msg bot.BotMessage) {
    // 1. 权限检查
    if !e.config.IsAdmin(msg.Platform, msg.UserID) {
        e.SendToBot(msg.Platform, msg.Channel, "❌ Permission denied: admin only")
        return
    }

    // 2. 参数解析
    if len(args) < 1 {
        e.SendToBot(msg.Platform, msg.Channel,
            "❌ Invalid arguments\nUsage: delete <name>")
        return
    }

    name := args[0]

    // 3. 检查 session 是否存在
    e.sessionMu.Lock()
    session, exists := e.sessions[name]
    if !exists {
        e.sessionMu.Unlock()
        e.SendToBot(msg.Platform, msg.Channel,
            fmt.Sprintf("❌ Session '%s' not found", name))
        return
    }

    // 4. 只能删除动态 session
    if !session.IsDynamic {
        e.sessionMu.Unlock()
        e.SendToBot(msg.Platform, msg.Channel,
            fmt.Sprintf("❌ Cannot delete configured session '%s'\n"+
                "Please remove it from the config file manually", name))
        return
    }

    // 5. 删除 tmux session
    adapter := e.cliAdapters[session.CLIType]
    if err := exec.Command("tmux", "kill-session", "-t", name).Run(); err != nil {
        logger.WithField("error", err).Warn("failed-to-kill-tmux-session")
    }

    // 6. 从内存中移除
    delete(e.sessions, name)
    e.sessionMu.Unlock()

    // 7. 成功反馈
    e.SendToBot(msg.Platform, msg.Channel,
        fmt.Sprintf("✅ Session '%s' deleted successfully", name))
}
```

### 3.5 UI 优化

#### status 命令输出更新
```
📊 clibot Status:

Sessions:
  ✅ project-a (claude) - idle [static]
  ✅ temp-test (claude) - processing [dynamic, created by discord:123456789]
  ❌ offline-session (gemini) - error [static]
```

#### sessions 命令输出更新
```
📋 Available Sessions:

Static Sessions (configured):
  • project-a (claude) - idle
  • backend (gemini) - processing

Dynamic Sessions (created via IM):
  • temp-test (claude) - idle [created by discord:123456789]
  • quick-debug (opencode) - processing [created by telegram:987654321]
```

---

## 4. 安全性考虑

### 4.1 权限控制
- 只有 admin 才能创建/删除动态 session
- 防止普通用户滥用资源

### 4.2 路径安全
- 防止路径遍历攻击：`../../../etc/passwd`
- 验证工作目录必须在用户可控范围内
- 建议限制：不允许绝对路径，或限制在 `$HOME` 下

### 4.3 资源限制
- 限制动态 session 总数（默认 50）
- 防止资源耗尽攻击

### 4.4 审计日志
```go
logger.WithFields(logrus.Fields{
    "action":    "create_session",
    "session":   name,
    "platform":  msg.Platform,
    "user_id":   msg.UserID,
    "cli_type":  cliType,
    "work_dir":  workDir,
}).Info("admin-created-dynamic-session")
```

---

## 5. 实现计划

### 5.1 优先级

#### P0（核心功能）- 第一阶段
1. ✅ 数据结构扩展
   - Session 添加 `IsDynamic` 和 `CreatedBy` 字段
   - SessionGlobalConfig 添加 `MaxDynamicSessions` 字段

2. ✅ new 命令实现
   - 命令注册
   - 参数解析和验证
   - Admin 权限检查
   - Session 创建逻辑
   - 错误处理

3. ✅ delete 命令实现
   - 命令注册
   - 参数解析
   - Admin 权限检查
   - 只删除动态 session
   - 清理 tmux session

#### P1（增强功能）- 第二阶段
4. ✅ status 命令更新
   - 显示 [static] / [dynamic] 标记
   - 显示创建者信息

5. ✅ sessions 命令更新
   - 分类显示静态/动态 session

6. ✅ 帮助文档更新
   - showHelp 添加 new/delete 说明

#### P2（完善）- 第三阶段
7. ✅ 资源限制检查
   - max_dynamic_sessions 配置读取
   - 动态 session 计数
   - 超限检查

8. ✅ 审计日志
   - 创建/删除操作日志
   - 包含完整的上下文信息

9. ✅ 参数验证增强
   - Session 名称格式检查
   - 路径安全检查
   - 详细错误提示

### 5.2 测试计划

#### 单元测试
```go
func TestEngine_HandleNewSession_Success(t *testing.T)
func TestEngine_HandleNewSession_PermissionDenied(t *testing.T)
func TestEngine_HandleNewSession_DuplicateSession(t *testing.T)
func TestEngine_HandleNewSession_InvalidCLIType(t *testing.T)
func TestEngine_HandleNewSession_WorkDirNotExists(t *testing.T)
func TestEngine_HandleNewSession_MaxSessionsReached(t *testing.T)

func TestEngine_HandleDeleteSession_Success(t *testing.T)
func TestEngine_HandleDeleteSession_PermissionDenied(t *testing.T)
func TestEngine_HandleDeleteSession_StaticSession(t *testing.T)
func TestEngine_HandleDeleteSession_SessionNotFound(t *testing.T)
```

#### 集成测试
- 创建 session 后验证可以正常使用
- 删除 session 后验证从列表中消失
- 跨 session 的隔离性验证

---

## 6. 未来扩展

### 6.1 可能的增强功能

1. **Session 持久化**
   ```yaml
   new myproject claude ~/work --persist
   ```
   - 将动态 session 保存到配置文件
   - 重启后自动恢复

2. **Session 模板**
   ```yaml
   session_templates:
     default:
       cli_type: claude
       start_cmd: "claude --profile default"
     fast:
       cli_type: claude
       start_cmd: "claude --fast"
   ```
   ```
   new myproject default ~/work  # 使用模板
   ```

3. **Session 生命周期管理**
   ```yaml
   session:
     dynamic_session_ttl: 24h  # 动态 session 自动清理时间
   ```
   - 超过 TTL 未使用的自动删除

4. **Session 批量操作**
   ```
   list        # 列出所有 session
   stop all    # 停止所有动态 session
   ```

### 6.2 不包含的功能
- Session 导入/导出
- Session 克隆
- Session 依赖管理
- 跨机器 session 迁移

---

## 7. 风险与挑战

### 7.1 技术风险
| 风险 | 影响 | 缓解措施 |
|------|------|----------|
| tmux session 创建失败 | 中 | 详细错误日志 + 友好错误提示 |
| 并发创建同名 session | 低 | sessionMu 锁保护 |
| 工作目录权限问题 | 中 | 提前检查权限 + 明确错误提示 |

### 7.2 安全风险
| 风险 | 影响 | 缓解措施 |
|------|------|----------|
| 路径遍历攻击 | 高 | 路径验证 + 限制范围 |
| 资源耗尽攻击 | 中 | 限制动态 session 数量 |
| 权限提升 | 中 | Admin 权限检查 + 审计日志 |

---

## 8. 成功标准

- [ ] Admin 可以通过 IM 创建动态 session
- [ ] 创建的 session 可以正常使用
- [ ] 非 Admin 无法创建 session（权限检查生效）
- [ ] 可以删除动态创建的 session
- [ ] 无法删除配置文件中的 session
- [ ] status/sessions 命令正确显示 session 来源
- [ ] 达到上限时无法创建新 session
- [ ] 所有测试通过（单元测试 + 集成测试）
- [ ] 文档完整（命令说明 + 使用示例）

---

## 9. 参考文档

- [clibot 设计文档](./2026-01-28-clibot-design.md)
- [MVP 实现计划](./2026-01-28-clibot-mvp-implementation.md)
- [实现进度追踪](../en/status/implementation-progress.md)
