# clibot

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Report Card](https://goreportcard.com/badge/github.com/keepmind9/clibot)](https://goreportcard.com/report/github.com/keepmind9/clibot)
[![GoDoc](https://pkg.go.dev/badge/github.com/keepmind9/clibot.svg)](https://pkg.go.dev/github.com/keepmind9/clibot)

[English](./README.md) | 中文版

clibot 是一个轻量级的中间层，将各种 IM 平台（飞书、Discord、Telegram 等）与 AI CLI 工具（Claude Code、Gemini CLI、OpenCode 等）连接起来，让用户可以通过聊天界面远程使用 AI 编程助手。

## 特性

- **无需公网 IP**: 所有 Bot 均采用长连接方式（WebSocket/长轮询）接入。您可以将 clibot 部署在家庭或公司内网电脑上，无需任何端口转发或公网 IP 即可在外部访问。
- **随时随地**: 在手机、平板等设备上通过 IM 使用强大的桌面端 AI CLI
- **统一入口**: 一个 IM Bot 管理多个 AI CLI 工具，切换简单
- **灵活扩展**: 抽象接口设计 - 只需实现接口即可添加新的 CLI 或 Bot
- **透明代理**: 绝大部分输入直接透传给 CLI，保持原生使用体验
- **零配置**: 可选的轮询模式（Polling Mode）无需对 CLI 进行任何配置（详见下文）

## 快速开始

### 前置要求

### 操作系统

**支持的平台**：
- ✅ **Linux** - 完全支持（Ubuntu、Debian、Fedora、CentOS、Arch 等）
- ✅ **macOS** - 完全支持
- ⚠️ **Windows** - 仅通过 WSL2（Windows Subsystem for Linux）支持

**为什么不支持 Windows 原生？**
clibot 依赖 `tmux` 进行会话管理，而 Windows 原生不支持 tmux。

**Windows 用户**：建议使用 WSL2 以获得最佳体验：
```bash
# 在 Windows 10/11 上安装 WSL2
wsl --install

# 安装后，将 WSL2 设置为默认版本
wsl --set-default-version 2

# 然后在 WSL 终端中按照 Linux 说明操作
```

详见下方的 [Windows 安装指南](#windows-安装指南)。

### 必需软件

- **Go**：1.24 或更高版本
- **tmux**：会话管理所需（clibot 创建和管理 tmux 会话）
- **Git**：克隆仓库所需（如果从源码安装）

**安装 tmux**：
```bash
# Ubuntu/Debian
sudo apt-get install tmux

# macOS
brew install tmux

# Fedora/CentOS/RHEL
sudo dnf install tmux

# Arch Linux
sudo pacman -S tmux
```

### Windows 安装指南 (WSL2)

clibot 可以在 Windows 上使用 WSL2（Windows Subsystem for Linux）运行。

**步骤 1：安装 WSL2**

以管理员身份打开 PowerShell 或命令提示符：

```powershell
# 启用 WSL
wsl --install

# 出现提示时重启计算机
```

**步骤 2：将 WSL2 设置为默认版本**

```powershell
wsl --set-default-version 2
```

**步骤 3：安装 Ubuntu（或其他 Linux 发行版）**

```powershell
# 查看可用的发行版
wsl --list --online

# 安装 Ubuntu（推荐）
wsl --install -d Ubuntu
```

**步骤 4：完成 Ubuntu 设置**

1. 从开始菜单启动 Ubuntu
2. 创建用户名和密码
3. 更新软件包：

```bash
# 在 WSL Ubuntu 终端中
sudo apt update && sudo apt upgrade -y
```

**步骤 5：在 WSL 中安装必需工具**

```bash
# 安装 Go
sudo apt install golang-go -y

# 或从网站安装最新版 Go
wget https://go.dev/dl/go1.24.0.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.24.0.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc

# 安装 tmux
sudo apt install tmux -y

# 安装 Git
sudo apt install git -y
```

**步骤 6：安装并运行 clibot**

```bash
# 在 WSL Ubuntu 终端中
go install github.com/keepmind9/clibot@latest

# 配置
mkdir -p ~/.config/clibot
cp /mnt/c/path/to/clibot/configs/config.yaml ~/.config/clibot/config.yaml
nano ~/.config/clibot/config.yaml

# 运行 clibot
clibot serve --config ~/.config/clibot/config.yaml
```

**Windows + WSL2 使用技巧**：

- 从 WSL 访问 Windows 文件：`/mnt/c/Users/你的用户名/...`
- 从 Windows 访问 WSL 文件：`\\wsl$\Ubuntu\home\你的用户名\...`
- 将 clibot 作为后台服务运行：在 WSL 内使用 systemd
- 无需配置防火墙（Bot 使用长连接，无需开放入站端口）
- 所有通信都是出站到 IM 平台（WebSocket/长轮询）

**限制**：
- 剪贴板集成可能不够流畅
- 需要文件路径转换（WSL ↔ Windows）
- 性能略低于原生 Linux

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
- 详见 [CLI Hook 配置指南](./docs/zh-CN/setup/cli-hooks.md)。

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
# 以服务模式运行
clibot serve --config ~/.config/clibot/config.yaml

# 检查状态
clibot status
```

## 命令

### serve

启动 clibot 服务以处理 IM 消息和管理 CLI 会话。

```bash
clibot serve [flags]
```

**参数:**
- `-c, --config <file>`: 配置文件路径（默认: `~/.config/clibot/config.yaml`）
- `--validate`: 验证配置后退出

**示例:**
```bash
clibot serve
clibot serve --config /etc/clibot/config.yaml
clibot serve --config ~/.config/clibot/config.yaml
```

### validate

验证 clibot 配置文件而不启动服务。

```bash
clibot validate [flags]
```

**参数:**
- `-c, --config <file>`: 配置文件路径（自动检测默认位置）
- `--show`: 显示完整配置详情
- `--json`: 以 JSON 格式输出

**退出码:**
- `0`: 配置有效
- `1`: 配置有错误

**示例:**
```bash
clibot validate
clibot validate --config ~/my-config.yaml
clibot validate --show
clibot validate --json
```

### status

显示 clibot 状态和版本信息。

```bash
clibot status [flags]
```

**参数:**
- `-p, --port <number>`: 检查服务是否在指定端口运行
- `--json`: 以 JSON 格式输出

**示例:**
```bash
clibot status
clibot status --port 8080
clibot status --json
```

**输出:**
- 显示 clibot 版本
- 检查服务是否运行（当使用 `--port` 时）

### version

显示详细的版本信息。

```bash
clibot version [flags]
```

**参数:**
- `--json`: 以 JSON 格式输出

**示例:**
```bash
clibot version
clibot version --json
```

**输出包括:**
- 版本号
- 构建时间
- Git 分支
- Git 提交哈希

### hook

内部命令，由 CLI hook 调用，用于通知主进程事件。此命令由 hook 模式配置使用。

```bash
clibot hook --cli-type <type> [flags]
```

**参数:**
- `--cli-type <type>`: CLI 类型（claude/gemini/opencode）**[必填]**
- `-p, --port <number>`: Hook 服务器端口（默认: 8080）

**用法:**
此命令从标准输入读取 JSON 事件数据，并通过 HTTP 转发到主进程。

**示例:**
```bash
echo '{"session":"my-session","event":"completed"}' | clibot hook --cli-type claude
cat hook-data.json | clibot hook --cli-type gemini
cat hook-data.json | clibot hook --cli-type claude --port 9000
```

**注意:** 此命令通常由配置了 hook 的 CLI 工具自动调用，而不是由用户手动调用。

详见 [CLI Hook 配置指南](./docs/zh-CN/setup/cli-hooks.md)。

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
│       ├── serve.go        # serve 命令
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
slist                              # 列出所有会话（静态和动态）
suse <session>                     # 切换当前会话
snew <name> <cli_type> <work_dir> [cmd]  # 创建新的动态会话（仅管理员）
sdel <name>                        # 删除动态会话（仅管理员）
whoami                             # 显示您当前会话信息
status                             # 显示所有会话状态
view [lines]                       # 查看 CLI 输出（默认: 20 行）
echo                               # 回显您的 IM 信息（平台, 用户ID, 频道ID）
help                               # 显示帮助信息
```

### 动态会话管理

clibot 支持通过 IM 命令创建和管理动态会话：

**创建新会话**（仅管理员）：
```bash
snew myproject claude ~/projects/myproject
snew backend gemini ~/backend my-custom-gemini
```

**删除动态会话**（仅管理员）：
```bash
sdel myproject
```

**切换会话**：
```bash
suse myproject    # 切换到会话 'myproject'
suse backend      # 切换到会话 'backend'
```

**会话类型**：
- **静态会话**：在 config.yaml 中配置，重启后保留
- **动态会话**：通过 IM 命令创建，仅存储在内存中，重启后丢失

**注意事项**：
- 只有管理员可以创建/删除动态会话
- 工作目录必须在创建会话前存在
- 动态会话会计入 `max_dynamic_sessions` 限制（默认: 50）
- 静态会话无法通过 IM 删除（需要手动修改配置文件）
- 每个用户可以独立选择自己的当前会话（互不影响）

## 特殊关键词

直接向 CLI 工具发送特殊按键（无需前缀）：

```
tab            # 发送 Tab 键 (用于自动补全)
esc            # 发送 Escape 键
stab/s-tab     # 发送 Shift+Tab
enter          # 发送 Enter 键
ctrlc/ctrl-c    # 发送 Ctrl+C (中断)
ctrlt/ctrl-t    # 发送 Ctrl+T
```

**示例:**
- `tab` → 触发 CLI 中的自动补全
- `s-tab` → 在建议中向后导航
- `ctrl-c` → 中断当前进程
- `ctrl-t` → 触发 Ctrl+T 操作

## 部署

在生产环境中，clibot 可以使用 systemd 或 supervisor 作为系统服务运行。

**快速设置（systemd）**：
```bash
# 创建专用用户
sudo useradd -r -s /bin/false clibot

# 安装服务文件
sudo cp deploy/clibot.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable clibot
sudo systemctl start clibot
```

**快速设置（supervisor）**：
```bash
# 安装 supervisor
sudo apt-get install supervisor  # Ubuntu/Debian

# 安装配置文件
sudo cp deploy/clibot.conf /etc/supervisor/conf.d/
sudo supervisorctl reread
sudo supervisorctl update
sudo supervisorctl start clibot
```

**快速设置（管理脚本）**：
```bash
# 用于开发和测试
chmod +x deploy/clibot.sh
./deploy/clibot.sh start
./deploy/clibot.sh status
./deploy/clibot.sh logs
```

详细的部署说明，包括：
- 创建专用用户
- 配置管理
- 日志轮转
- 故障排查
- 安全最佳实践

参见 [部署指南](./deploy/DEPLOYMENT_zh.md)

## 安全

clibot 本质上是一个远程代码执行工具。**必须启用用户白名单**。默认情况下 `whitelist_enabled: true`，即只有白名单中的用户可以使用系统。

## 贡献

请在贡献前阅读 [AGENTS.md](AGENTS.md) 了解开发指南和语言要求。

## 开源协议

MIT
