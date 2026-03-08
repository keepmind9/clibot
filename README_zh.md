# clibot

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Report Card](https://goreportcard.com/badge/github.com/keepmind9/clibot?v=20260303)](https://goreportcard.com/report/github.com/keepmind9/clibot)
[![GoDoc](https://pkg.go.dev/badge/github.com/keepmind9/clibot.svg)](https://pkg.go.dev/github.com/keepmind9/clibot)

[English](./README.md) | 中文版

clibot 是一个轻量级中间件，将各种 IM 平台（飞书、Discord、Telegram 等）与 AI CLI 工具（Claude Code、Gemini CLI、OpenCode 等）连接起来，让你可以直接在手机或平板上使用强大的桌面 AI 编程助手。

## ✨ 特性

- **🌍 无需公网 IP**：所有机器人通过长连接（WebSocket/长轮询）连接，可在 NAT 后的家用/办公电脑上部署
- **📱 随时随地访问**：通过手机 IM 应用使用桌面 AI 工具
- **🎯 统一入口**：通过单个机器人管理多个 AI 工具
- **🔌 灵活扩展**：通过实现接口添加新的 CLI 或 Bot
- **⚡ ACP 支持**：流式响应，无需 tmux（兼容的 CLI）

## 🚀 快速开始

### 前置要求

- **Go 1.24+**
- [**机器人账号**](#配置机器人)（飞书/Discord/Telegram）
- [**ACP 兼容的 CLI**](#acp-模式推荐)（如 claude-agent-acp）或 **tmux**（Hook 模式）

详细的安装说明请参阅 [INSTALL.md](INSTALL.md)。

### 安装

```bash
go install github.com/keepmind9/clibot@latest
```

二进制文件将安装到 `~/go/bin/clibot`。确保它在你的 PATH 中：

```bash
export PATH=$PATH:~/go/bin
```

## 🌐 网络代理

clibot 支持网络代理，用于在受限网络中访问 IM 平台。

### 快速配置

```bash
# 使用环境变量
export HTTP_PROXY="http://127.0.0.1:7890"
clibot serve

# 或在 config.yaml 中配置
proxy:
  enabled: true
  url: "http://127.0.0.1:7890"
```

### 支持的协议

- HTTP/HTTPS 代理
- SOCKS5 代理
- 可选的用户名/密码认证

### 文档

详见[代理配置指南](docs/zh-CN/setup/proxy.md)。

## 🔑 获取你的用户 ID

配置 clibot 之前，你需要从 IM 平台获取你的用户 ID，用于配置白名单和管理员。

### 快速方法

**步骤 1：** 临时禁用白名单启动 clibot：

```yaml
# ~/temp_config.yaml
security:
  whitelist_enabled: false  # 临时禁用

bots:
  telegram:
    enabled: true
    token: "YOUR_BOT_TOKEN"
```

**步骤 2：** 运行 clibot：

```bash
clibot serve --config ~/temp_config.yaml
```

**步骤 3：** 向你的机器人发送 `echo` 命令：

```
echo
```

**步骤 4：** 机器人回复你的用户 ID：

```
🔍 你的 IM 信息

平台: telegram
用户 ID: 123456789
```

**步骤 5：** 使用你的实际用户 ID 更新配置：

```yaml
security:
  whitelist_enabled: true
  allowed_users:
    telegram:
      - "123456789"  # 你的实际用户 ID
  admins:
    telegram:
      - "123456789"  # 你的实际用户 ID
```

**重要：** 删除 `~/temp_config.yaml` 并使用正确的配置重启。

### 配置

```bash
# 创建配置目录
mkdir -p ~/.config/clibot

# 复制配置模板
cp configs/config.mini.yaml ~/.config/clibot/config.yaml

# 编辑配置（替换 YOUR_* 占位符）
nano ~/.config/clibot/config.yaml
```

### 运行

```bash
clibot serve --config ~/.config/clibot/config.yaml
```

## 💡 运行模式

### ACP 模式（推荐）⭐

**适用于：** claude-agent-acp、支持 ACP 的 Gemini CLI、支持 ACP 的 OpenCode

**优势：**
- ✅ 无需 tmux
- ✅ 流式响应（实时）
- ✅ 全双工通信
- ✅ 适用于所有平台

**配置：**
```yaml
sessions:
  - name: "my-project"
    work_dir: "/path/to/project"
    start_cmd: "claude-agent-acp"
    transport: "stdio://"
```

**设置 ACP CLI：**
```bash
# 为 Claude Code 安装 ACP 适配器
npm install -g @zed-industries/claude-agent-acp

# Gemini CLI
gemini --experimental-acp

# OpenCode CLI
opencode --acp
```

### Hook 模式

**适用于：** Claude Code、Gemini CLI、OpenCode（默认模式）

**优势：**
- ✅ 实时通知
- ✅ 精确的完成检测

**要求：**
- ⚠️ 需要 tmux
- ⚠️ 需要 CLI hook 配置

**配置：**
```yaml
sessions:
  - name: "my-project"
    work_dir: "/path/to/project"
    start_cmd: "claude"
```

详细配置请参阅 [CLI Hook 配置指南](./docs/zh/setup/cli-hooks.md)。

### 模式选择

**优先级：ACP > Hook**

ACP 模式提供更好的用户体验，在可用时应优先选择。

## 📱 配置机器人

### 飞书（推荐）

1. 在[开放平台](https://open.feishu.cn/)创建飞书应用
2. 获取 App ID 和 App Secret
3. 配置机器人：

```yaml
bots:
  feishu:
    enabled: true
    app_id: "cli_xxxxxxxxx"
    app_secret: "xxxxxxxxxxxxxxxx"
```

### Discord

1. 在 [Discord 开发者门户](https://discord.com/developers/applications)创建 Discord 应用
2. 创建机器人并获取令牌
3. 邀请机器人到你的服务器
4. 配置：

```yaml
bots:
  discord:
    enabled: true
    token: "YOUR_BOT_TOKEN"
    channel_id: "YOUR_CHANNEL_ID"
```

### Telegram

1. 通过 [BotFather](https://t.me/BotFather)创建机器人
2. 获取机器人令牌
3. 配置：

```yaml
bots:
  telegram:
    enabled: true
    token: "YOUR_BOT_TOKEN"
```

## 🎮 使用方法

### 特殊命令

```
slist                              # 列出所有会话
suse <session>                     # 切换到指定会话
snew <name> <type> <dir> [cmd]     # 创建新会话（仅管理员）
sdel <name>                        # 删除会话（仅管理员）
sclose [name]                      # 关闭会话
sstatus [name]                     # 显示会话状态
whoami                             # 显示你的信息
status                             # 显示所有会话状态
echo                               # 显示你的 IM 信息
help                               # 显示帮助
```

### 特殊关键词

**⚠️ 仅 Hook 模式：** 这些关键词仅在 Hook 模式下有效。

```
tab           # 发送 Tab 键（自动完成）
esc           # 发送 Escape 键
s-tab         # 发送 Shift+Tab
enter         # 发送 Enter 键
ctrl-c        # 发送 Ctrl+C（中断）
```

**注意：** 这些关键词通过 `tmux send-keys` 模拟按键。仅在使用 Hook 模式时有效。ACP 模式使用直接协议通信，不支持这些关键词。

### 使用示例

```
你:  slist
Bot: 可用会话：
     • claude (acp)
     • gemini (gemini)

你:  suse claude
Bot: ✓ 已切换到会话：claude

你:  帮我写一个 Python 函数来解析 JSON
Bot:  [AI 响应...]
```

## 🔧 部署

### 作为 systemd 服务运行（Linux/macOS）

```bash
# 创建 systemd 用户目录
mkdir -p ~/.config/systemd/user

# 安装服务文件
cp deploy/clibot.service ~/.config/systemd/user/
systemctl --user daemon-reload
systemctl --user enable clibot
systemctl --user start clibot

# 查看日志
journalctl --user -u clibot -f
```

### 作为 supervisor 服务运行

```bash
# 安装 supervisor
sudo apt-get install supervisor

# 安装配置文件
sudo cp deploy/clibot.conf /etc/supervisor/conf.d/
sudo supervisorctl reread
sudo supervisorctl update
sudo supervisorctl start clibot
```

详细部署指南请参阅 [deploy/DEPLOYMENT.md](deploy/DEPLOYMENT.md)。

## 🔒 安全性

⚠️ **必须启用用户白名单**（默认：`whitelist_enabled: true`）

只有白名单用户才能使用 clibot。始终在配置文件中配置 `allowed_users` 和 `admins`。

## 🏗️ 项目结构

```
clibot/
├── cmd/                    # CLI 入口
├── internal/
│   ├── core/               # 核心逻辑
│   ├── cli/                # CLI 适配器
│   └── bot/                # Bot 适配器
├── configs/                # 配置模板
└── docs/                   # 文档
```

## 📚 文档

- [INSTALL.md](INSTALL.md) - 安装指南
- [docs/zh/setup/cli-hooks.md](docs/zh/setup/cli-hooks.md) - CLI hook 配置
- [deploy/DEPLOYMENT.md](deploy/DEPLOYMENT.md) - 部署指南
- [AGENTS.md](AGENTS.md) - 开发指南

## 🤝 贡献

欢迎贡献！请阅读 [AGENTS.md](AGENTS.md) 了解开发指南。

## 📄 许可证

[MIT](LICENSE)
