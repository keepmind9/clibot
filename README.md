# clibot

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Report Card](https://goreportcard.com/badge/github.com/keepmind9/clibot?v=20260303)](https://goreportcard.com/report/github.com/keepmind9/clibot)
[![GoDoc](https://pkg.go.dev/badge/github.com/keepmind9/clibot.svg)](https://pkg.go.dev/github.com/keepmind9/clibot)

English | [中文版](./README_zh.md)

clibot is a lightweight middleware that bridges ACP-compatible AI CLI tools (Claude Code, Gemini CLI, OpenCode) to IM platforms (Discord, Telegram, Feishu, DingTalk, QQ). Use powerful desktop AI programming assistants from your phone with streaming responses - no public IP required.

## ✨ Features

- **🌍 No Public IP Required**: All bots connect via long-connections (WebSocket/Long Polling). Deploy on your home/office computer behind NAT.
- **📱 Access Anywhere**: Use desktop AI tools from mobile phone via IM apps
- **🎯 Unified Entry Point**: Manage multiple AI tools through a single bot
- **🔌 Flexible Extension**: Add new CLI or Bot by implementing interfaces
- **⚡ ACP Support**: Streaming responses, no tmux required (for compatible CLIs)

## ✨ Claude Code Skill

For Claude Code users, an interactive skill is available for one-command setup:

```bash
/clibot setup
```

The skill handles everything: binary download, bot token setup, config generation, and service startup — all through a guided wizard. No manual config editing required.

See [`skills/clibot/`](skills/clibot/) for details.

## 🚀 Quick Start

### Prerequisites

- **Go 1.24+**
- [**Bot Account**](#setup-bot) (Feishu/Discord/Telegram)
- [**ACP-Compatible CLI**](#acp-mode-recommended) (e.g., claude-agent-acp) OR **tmux** (for Hook Mode)

For detailed installation instructions, see [INSTALL.md](INSTALL.md).

### Install

```bash
go install github.com/keepmind9/clibot@latest
```

The binary will be installed at `~/go/bin/clibot`. Make sure it's in your PATH:

```bash
export PATH=$PATH:~/go/bin
```

## 🌐 Network Proxy

clibot supports network proxies for accessing IM platforms in restricted networks.

### Quick Setup

```bash
# Using environment variables
export HTTP_PROXY="http://127.0.0.1:7890"
clibot serve

# Or configure in config.yaml
proxy:
  enabled: true
  url: "http://127.0.0.1:7890"
```

### Supported Protocols

- HTTP/HTTPS proxy
- SOCKS5 proxy
- Optional username/password authentication

### Documentation

See [Proxy Configuration Guide](docs/en/setup/proxy.md) for details.

## 🔑 Get Your User ID

Before configuring clibot, you need to get your user ID from the IM platform for whitelist and admin setup.

### Quick Method

**Step 1:** Start clibot with whitelist temporarily disabled:

```yaml
# ~/temp_config.yaml
security:
  whitelist_enabled: false  # Temporarily disable

bots:
  telegram:
    enabled: true
    token: "YOUR_BOT_TOKEN"
```

**Step 2:** Run clibot:

```bash
clibot serve --config ~/temp_config.yaml
```

**Step 3:** Send `echo` command to your bot:

```
echo
```

**Step 4:** Bot replies with your user ID:

```
🔍 Your IM Information

Platform: telegram
User ID: 123456789
```

**Step 5:** Update your actual config with your user ID:

```yaml
security:
  whitelist_enabled: true
  allowed_users:
    telegram:
      - "123456789"  # Your actual user ID
  admins:
    telegram:
      - "123456789"  # Your actual user ID
```

**Important:** Delete `~/temp_config.yaml` and restart with proper config.

### Configure

```bash
# Create config directory
mkdir -p ~/.config/clibot

# Copy configuration template
cp configs/config.mini.yaml ~/.config/clibot/config.yaml

# Edit configuration (replace YOUR_* placeholders)
nano ~/.config/clibot/config.yaml
```

### Run

```bash
clibot serve --config ~/.config/clibot/config.yaml
```

## 💡 Operation Modes

### ACP Mode (Recommended) ⭐

**Best for:** claude-agent-acp, Gemini CLI with ACP, OpenCode with ACP

**Advantages:**
- ✅ No tmux required
- ✅ Streaming responses (real-time)
- ✅ Full-duplex communication
- ✅ Works on all platforms

**Configuration:**
```yaml
sessions:
  - name: "my-project"
    work_dir: "/path/to/project"
    start_cmd: "claude-agent-acp"
    transport: "stdio://"
```

**Setup ACP CLI:**
```bash
# Install ACP adapter for Claude Code
npm install -g @zed-industries/claude-agent-acp

# Gemini CLI
gemini --experimental-acp

# OpenCode CLI
opencode --acp
```

### Hook Mode

**Best for:** Claude Code, Gemini CLI, OpenCode (default mode)

**Advantages:**
- ✅ Real-time notifications
- ✅ Accurate completion detection

**Requirements:**
- ⚠️ Requires tmux
- ⚠️ Requires CLI hook configuration

**Configuration:**
```yaml
sessions:
  - name: "my-project"
    work_dir: "/path/to/project"
    start_cmd: "claude"
```

See [CLI Hook Configuration Guide](./docs/en/setup/cli-hooks.md) for detailed setup.

### Mode Selection

**Priority: ACP > Hook**

ACP Mode provides better user experience and should be preferred when available.

## 📱 Setup Bot

### Feishu (Recommended)

1. Create a Feishu app at [Open Platform](https://open.feishu.cn/)
2. Get App ID and App Secret
3. Configure bot:

```yaml
bots:
  feishu:
    enabled: true
    app_id: "cli_xxxxxxxxx"
    app_secret: "xxxxxxxxxxxxxxxx"
```

### Discord

1. Create a Discord application at [Discord Developer Portal](https://discord.com/developers/applications)
2. Create a bot and get token
3. Invite bot to your server
4. Configure:

```yaml
bots:
  discord:
    enabled: true
    token: "YOUR_BOT_TOKEN"
    channel_id: "YOUR_CHANNEL_ID"
```

### Telegram

1. Create a bot via [BotFather](https://t.me/BotFather)
2. Get bot token
3. Configure:

```yaml
bots:
  telegram:
    enabled: true
    token: "YOUR_BOT_TOKEN"
```

### QQ

1. Create a QQ bot at [QQ Open Platform](https://bots.qq.com)
2. Get App ID and App Secret
3. Configure:

```yaml
bots:
  qq:
    enabled: true
    app_id: "YOUR_QQ_APP_ID"
    app_secret: "YOUR_QQ_APP_SECRET"
```

**Note:** QQ bot uses WebSocket for receiving messages and HTTP API for sending. Supports C2C (private chat) messages.

## 🎮 Usage

### Special Commands

```
slist                              # List all sessions
suse <session>                     # Switch to session
snew <name> <type> <dir> [cmd]     # Create new session (admin only)
sdel <name>                        # Delete session (admin only)
sclose [name]                      # Close session
sstatus [name]                     # Show session status
whoami                             # Show your info
status                             # Show all session status
echo                               # Show your IM info
help                               # Show help
```

### Special Keywords

**⚠️ Hook Mode Only:** These keywords only work in Hook mode with tmux.

```
tab           # Send Tab key (autocomplete)
esc           # Send Escape key
s-tab         # Send Shift+Tab
enter         # Send Enter key
ctrl-c        # Send Ctrl+C (interrupt)
```

**Note:** These keywords simulate key presses via `tmux send-keys`. They only work in Hook mode. ACP mode uses direct protocol communication and does not support these keywords.

### Example Workflow

```
You:  slist
Bot:  Available Sessions:
     • claude (acp)
     • gemini (gemini)

You:  suse claude
Bot:  ✓ Switched to session: claude

You:  help me write a python function to parse json
Bot:  [AI response...]
```

## 🔧 Deployment

### Run as systemd service (Linux/macOS)

```bash
# Create systemd user directory
mkdir -p ~/.config/systemd/user

# Install service file
cp deploy/clibot.service ~/.config/systemd/user/
systemctl --user daemon-reload
systemctl --user enable clibot
systemctl --user start clibot

# View logs
journalctl --user -u clibot -f
```

### Run as supervisor service

```bash
# Install supervisor
sudo apt-get install supervisor

# Install config file
sudo cp deploy/clibot.conf /etc/supervisor/conf.d/
sudo supervisorctl reread
sudo supervisorctl update
sudo supervisorctl start clibot
```

For detailed deployment guide, see [deploy/DEPLOYMENT.md](deploy/DEPLOYMENT.md).

## 🔒 Security

⚠️ **User whitelist must be enabled** (default: `whitelist_enabled: true`)

Only whitelisted users can use clibot. Always configure `allowed_users` and `admins` in your config file.

## 🏗️ Project Structure

```
clibot/
├── cmd/                    # CLI entry point
├── internal/
│   ├── core/               # Core logic
│   ├── cli/                # CLI adapters
│   └── bot/                # Bot adapters
├── configs/                # Configuration templates
└── docs/                   # Documentation
```

## 📚 Documentation

- [INSTALL.md](INSTALL.md) - Installation guide
- [docs/en/setup/cli-hooks.md](docs/en/setup/cli-hooks.md) - CLI hook configuration
- [deploy/DEPLOYMENT.md](deploy/DEPLOYMENT.md) - Deployment guide
- [AGENTS.md](AGENTS.md) - Development guidelines

## 🤝 Contributing

Contributions are welcome! Please read [AGENTS.md](AGENTS.md) for development guidelines.

## 📄 License

[MIT](LICENSE)
