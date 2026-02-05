# clibot

[English](./README.md) | 中文版

clibot 是一个轻量级的中间层，将各种 IM 平台（飞书、Discord、Telegram 等）与 AI CLI 工具（Claude Code、Gemini CLI、OpenCode 等）连接起来，让用户可以通过聊天界面远程使用 AI 编程助手。

## 特性

- **随时随地**: 在手机、平板等设备上通过 IM 使用强大的桌面端 AI CLI
- **统一入口**: 一个 IM Bot 管理多个 AI CLI 工具，切换简单
- **灵活扩展**: 抽象接口设计 - 只需实现接口即可添加新的 CLI 或 Bot
- **透明代理**: 绝大部分输入直接透传给 CLI，保持原生使用体验
- **零配置**: 可选的轮询模式（Polling Mode）无需对 CLI 进行任何配置（详见下文）

## 快速开始

### 安装

```bash
go install github.com/keepmind9/clibot@latest
```

### 配置

1. 复制配置模板：
```bash
cp configs/config.yaml ~/.config/clibot/config.yaml
```

2. 编辑配置文件，填写您的 Bot 凭据和白名单用户。

3. 选择运行模式（见下文）：

**方案 A: Hook 模式（默认，推荐）**
- 需要配置 CLI 的 Hook
- 实时通知
- 适合生产环境

配置 Claude Code Hook (`~/.claude/settings.json`):
```json
{
  "hooks": {
    "Stop": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "clibot hook --cli-type claude"
          }
        ]
      }
    ],
    "Notification": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "clibot hook --cli-type claude"
          }
        ]
      }
    ]
  }
}
```

配置 Gemini CLI Hook (`~/.gemini/settings.json`):
```json
{
  "tools": {
    "enableHooks": true
  },
  "hooks": {
    "AfterAgent": [
      {
        "hooks": [
          {
            "name": "clibot-post-command-hook",
            "type": "command",
            "command": "clibot hook --cli-type gemini",
            "description": "post command hook for clibot"
          }
        ]
      }
    ],
    "Notification": [
      {
        "hooks": [
          {
            "name": "clibot-notification-command-hook",
            "type": "command",
            "command": "clibot hook --cli-type gemini",
            "description": "notification command hook for clibot"
          }
        ]
      }
    ]
  }
}
```

**方案 B: 轮询模式（零配置）**
- 无需对 CLI 进行任何配置
- 自动轮询 tmux 输出
- 适合快速上手

```yaml
cli_adapters:
  claude:
    use_hook: false  # 启用轮询模式
    poll_interval: "1s"
```

### 使用

```bash
# 启动主进程
clibot start --config ~/.config/clibot/config.yaml

# 检查状态
clibot status
```

## 运行模式

clibot 支持两种模式来检测 CLI 何时完成响应：

### Hook 模式（默认）

**配置:**
```yaml
cli_adapters:
  claude:
    use_hook: true
```

**工作原理:**
1. CLI 在完成任务时发送 HTTP Hook
2. clibot 立即收到通知
3. 捕获 tmux 输出并发送给用户

**优点:**
- ✅ 实时（即时通知）
- ✅ 准确（精确的完成时间）
- ✅ 高效（无轮询开销）

**缺点:**
- ⚠️ 需要配置 CLI Hook
- ⚠️ 设置略微复杂

**适用场景:** 生产环境、对性能要求高的应用

### 轮询模式 (Polling Mode)

**配置:**
```yaml
cli_adapters:
  claude:
    use_hook: false
    poll_interval: "1s"  # 每秒检查一次
    stable_count: 3      # 需要连续 3 次输出一致
```

**工作原理:**
1. clibot 定期轮询 tmux 输出
2. 检查输出是否在连续 N 次检查中保持不变
3. 当输出稳定时，认为 CLI 已完成并发送响应

**优点:**
- ✅ 零配置（无需设置 CLI）
- ✅ 适用于任何 CLI 工具
- ✅ 上手简单

**缺点:**
- ⚠️ 存在轻微延迟（通常为 1-3 秒）
- ⚠️ 产生周期性 CPU 开销（极小）

**适用场景:** 快速测试、不支持 Hook 的 CLI、低频使用

**配置建议:**
- `poll_interval`: 1-2 秒通常是最佳选择
- `stable_count`: 2-3 可以在速度和可靠性之间取得平衡

## 项目结构

```
clibot/
├── cmd/                    # CLI 入口
│   └── clibot/             # 主程序
│       ├── main.go         # 入口函数
│       ├── root.go         # Cobra 根命令
│       ├── start.go        # start 命令
│       ├── hook.go         # hook 命令
│       └── status.go       # status 命令
├── internal/
│   ├── core/               # 核心逻辑
│   ├── cli/                # CLI 适配器
│   ├── bot/                # Bot 适配器
│   ├── watchdog/           # Watchdog 监控
│   └── hook/               # HTTP Hook 服务
└── configs/                # 配置模板
```

## 特殊命令

```
!!sessions              # 列出所有会话
!!use <session>         # 切换当前会话
!!new <name> <cli>      # 创建新会话
!!whoami                # 显示当前会话信息
!!status                # 显示所有会话状态
!!view [lines]          # 查看 CLI 输出 (默认: 20 行)
!!help                  # 显示帮助信息
```

## 特殊关键词

直接向 CLI 工具发送特殊按键（无需前缀）：

```
tab          # 发送 Tab 键 (用于自动补全)
esc          # 发送 Escape 键
stab/s-tab   # 发送 Shift+Tab
enter        # 发送 Enter 键
ctrlc/ctrl-c # 发送 Ctrl+C (中断)
```

**示例:**
- `tab` → 触发 CLI 中的自动补全
- `s-tab` → 在建议中向后导航
- `ctrl-c` → 中断当前进程

## 安全

clibot 本质上是一个远程代码执行工具。**必须启用用户白名单**。默认情况下 `whitelist_enabled: true`，即只有白名单中的用户可以使用系统。

## 贡献

请在贡献前阅读 [AGENTS.md](AGENTS.md) 了解开发指南和语言要求。

## 开源协议

MIT
