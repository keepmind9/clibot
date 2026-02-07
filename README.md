# clibot

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Report Card](https://goreportcard.com/badge/github.com/keepmind9/clibot)](https://goreportcard.com/report/github.com/keepmind9/clibot)
[![GoDoc](https://pkg.go.dev/badge/github.com/keepmind9/clibot.svg)](https://pkg.go.dev/github.com/keepmind9/clibot)

English | [中文版](./README_zh.md)

clibot is a lightweight middleware that connects various IM platforms (Feishu, Discord, Telegram, etc.) with AI CLI tools (Claude Code, Gemini CLI, OpenCode, etc.), enabling users to remotely use AI programming assistants through chat interfaces.

## Features

- **No Public IP Required**: All bots connect via long-connections (WebSocket/Long Polling). You can deploy clibot on your home or office computer behind NAT without any port forwarding or public IP.
- **Access Anywhere**: Use powerful desktop AI CLI tools from your mobile phone or tablet via IM
- **Unified Entry Point**: Manage multiple AI CLI tools through a single IM bot with easy switching
- **Flexible Extension**: Abstract interface design - add new CLI or Bot by simply implementing interfaces
- **Transparent Proxy**: Most inputs are directly passed through to CLI, maintaining native user experience
- **Zero Configuration**: Optional polling mode requires no CLI configuration (see modes below)

## Quick Start

### Prerequisites

### Operating System

**Supported Platforms**:
- ✅ **Linux** - Fully supported (Ubuntu, Debian, Fedora, CentOS, Arch, etc.)
- ✅ **macOS** - Fully supported
- ⚠️ **Windows** - Only via WSL2 (Windows Subsystem for Linux)

**Why not Windows native?**
clibot depends on `tmux` for session management, which is not available natively on Windows.

**Windows users**: Use WSL2 for the best experience:
```bash
# Install WSL2 on Windows 10/11
wsl --install

# After installation, setup WSL2 with Ubuntu
wsl --set-default-version 2

# Then follow Linux instructions in WSL terminal
```

See [Windows Setup Guide](#windows-setup) below for detailed instructions.

### Required Software

- **Go**: 1.24 or higher
- **tmux**: Required for session management (clibot creates and manages tmux sessions)
- **Git**: For cloning the repository (if installing from source)

**Installing tmux**:
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

### Windows Setup (WSL2)

clibot can run on Windows using WSL2 (Windows Subsystem for Linux).

**Step 1: Install WSL2**

Open PowerShell or Command Prompt as Administrator:

```powershell
# Enable WSL
wsl --install

# Restart your computer when prompted
```

**Step 2: Set WSL2 as default**

```powershell
wsl --set-default-version 2
```

**Step 3: Install Ubuntu (or other Linux distribution)**

```powershell
# View available distributions
wsl --list --online

# Install Ubuntu (recommended)
wsl --install -d Ubuntu
```

**Step 4: Complete Ubuntu setup**

1. Launch Ubuntu from Start menu
2. Create a username and password
3. Update packages:

```bash
# Inside WSL Ubuntu terminal
sudo apt update && sudo apt upgrade -y
```

**Step 5: Install required tools in WSL**

```bash
# Install Go
sudo apt install golang-go -y

# Or install latest Go from website
wget https://go.dev/dl/go1.24.0.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.24.0.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc

# Install tmux
sudo apt install tmux -y

# Install Git
sudo apt install git -y
```

**Step 6: Install and run clibot**

```bash
# Inside WSL Ubuntu terminal
go install github.com/keepmind9/clibot@latest

# Configure
mkdir -p ~/.config/clibot
cp /mnt/c/path/to/clibot/configs/config.yaml ~/.config/clibot/config.yaml
nano ~/.config/clibot/config.yaml

# Run clibot
clibot serve --config ~/.config/clibot/config.yaml
```

**Windows + WSL2 Tips**:

- Access Windows files from WSL: `/mnt/c/Users/YourName/...`
- Access WSL files from Windows: `\\wsl$\Ubuntu\home\yourname\...`
- Run clibot as background service: Use systemd inside WSL
- No firewall configuration needed (bots use long-connections, no inbound ports)
- All communication is outbound to IM platforms (WebSocket/Long Polling)

**Limitations**:
- Clipboard integration may not work seamlessly
- File path conversions needed (WSL ↔ Windows)
- Performance slightly lower than native Linux

### Installation

```bash
go install github.com/keepmind9/clibot@latest
```

### Configuration

1. Copy the configuration template:
```bash
cp configs/config.yaml ~/.config/clibot/config.yaml
```

2. Edit the configuration file and fill in your bot credentials and whitelist users

3. Choose your mode (see below):

**Option A: Hook Mode (Default, Recommended)**
- Requires CLI hook configuration
- Real-time notifications
- See [CLI Hook Configuration Guide](./docs/en/setup/cli-hooks.md) for detailed setup.

**Option B: Polling Mode (Zero Config)**
- No CLI configuration required
- Automatic tmux polling
- Perfect for quick start

```yaml
cli_adapters:
  claude:
    use_hook: false  # Enable polling mode
    poll_interval: "1s"
```

### Usage

```bash
# Run clibot as a service
clibot serve --config ~/.config/clibot/config.yaml

# Check status
clibot status
```

## Commands

### serve

Start the clibot service to handle IM messages and manage CLI sessions.

```bash
clibot serve [flags]
```

**Flags:**
- `-c, --config <file>`: Configuration file path (default: `~/.config/clibot/config.yaml`)
- `--validate`: Validate configuration and exit

**Examples:**
```bash
clibot serve
clibot serve --config /etc/clibot/config.yaml
clibot serve --config ~/.config/clibot/config.yaml
```

### validate

Validate the clibot configuration file without starting the service.

```bash
clibot validate [flags]
```

**Flags:**
- `-c, --config <file>`: Configuration file path (auto-detects default locations)
- `--show`: Show full configuration details
- `--json`: Output in JSON format

**Exit Codes:**
- `0`: Configuration is valid
- `1`: Configuration has errors

**Examples:**
```bash
clibot validate
clibot validate --config ~/my-config.yaml
clibot validate --show
clibot validate --json
```

### status

Show clibot status and version information.

```bash
clibot status [flags]
```

**Flags:**
- `-p, --port <number>`: Check if the service is running on the specified port
- `--json`: Output in JSON format

**Examples:**
```bash
clibot status
clibot status --port 8080
clibot status --json
```

**Output:**
- Shows clibot version
- Checks if service is running (when `--port` is specified)

### version

Show detailed version information.

```bash
clibot version [flags]
```

**Flags:**
- `--json`: Output in JSON format

**Examples:**
```bash
clibot version
clibot version --json
```

**Output includes:**
- Version number
- Build time
- Git branch
- Git commit hash

### hook

Internal command called by CLI hooks to notify the main process of events. This is used by the hook mode configuration.

```bash
clibot hook --cli-type <type> [flags]
```

**Flags:**
- `--cli-type <type>`: CLI type (claude/gemini/opencode) **[required]**
- `-p, --port <number>`: Hook server port (default: 8080)

**Usage:**
This command reads JSON event data from stdin and forwards it to the main process via HTTP.

**Examples:**
```bash
echo '{"session":"my-session","event":"completed"}' | clibot hook --cli-type claude
cat hook-data.json | clibot hook --cli-type gemini
cat hook-data.json | clibot hook --cli-type claude --port 9000
```

**Note:** This command is typically called automatically by CLI tools configured with hooks, not manually by users.

See [CLI Hook Configuration Guide](./docs/en/setup/cli-hooks.md) for detailed setup instructions.

## Operation Modes

clibot supports two modes for detecting when the CLI has finished responding:

### Hook Mode (Default)

**Configuration:**
```yaml
cli_adapters:
  claude:
    use_hook: true
```

**How it works:**
1. CLI sends HTTP hook when it completes a task
2. clibot receives notification immediately
3. Captures tmux output and sends to user

**Pros:**
- ✅ Real-time (instant notification)
- ✅ Accurate (exact completion time)
- ✅ Efficient (no polling overhead)

**Cons:**
- ⚠️ Requires CLI hook configuration
- ⚠️ Higher setup complexity

**Best for:** Production environments, performance-critical applications

### Polling Mode

**Configuration:**
```yaml
cli_adapters:
  claude:
    use_hook: false
    poll_interval: "1s"  # Check every second
    stable_count: 3      # Require 3 identical outputs
```

**How it works:**
1. clibot polls tmux output at regular intervals
2. Checks if output remains unchanged for N consecutive checks
3. When stable, considers CLI complete and sends response

**Pros:**
- ✅ Zero configuration (no CLI setup needed)
- ✅ Works with any CLI tool
- ✅ Simple to get started

**Cons:**
- ⚠️ Slight delay (1-3 seconds typically)
- ⚠️ Periodic CPU usage (minimal)

**Best for:** Quick testing, CLIs without hook support, low-frequency usage

**Configuration Tips:**
- `poll_interval`: 1-2 seconds is usually optimal
- `stable_count`: 2-3 balances speed and reliability
- Faster intervals = quicker response but more CPU
- Higher `stable_count` = more reliable but slower

**Example Comparison:**
```yaml
# Hook mode - fast and accurate
claude:
  use_hook: true
  # No polling config needed

# Polling mode - zero config
claude_simple:
  use_hook: false
  poll_interval: "1s"
  stable_count: 3

# Polling mode - optimized for speed
claude_fast:
  use_hook: false
  poll_interval: "500ms"  # Check every 0.5s
  stable_count: 2

# Polling mode - optimized for efficiency
claude_efficient:
  use_hook: false
  poll_interval: "2s"
  stable_count: 4
```

## Project Structure

```
clibot/
├── cmd/                    # CLI entry point
│   └── clibot/             # Main program
│       ├── main.go         # Main function
│       ├── root.go         # Cobra root command
│       ├── serve.go        # serve command
│       ├── hook.go         # hook command
│       └── status.go       # status command
├── internal/
│   ├── core/               # Core logic
│   ├── cli/                # CLI adapters
│   ├── bot/                # Bot adapters
│   ├── watchdog/           # Watchdog monitoring
│   └── hook/               # HTTP Hook server
└── configs/                # Configuration templates
```

## Special Commands

```
slist                              # List all sessions (static and dynamic)
suse <session>                     # Switch current session
snew <name> <cli_type> <work_dir> [cmd]  # Create new dynamic session (admin only)
sdel <name>                        # Delete dynamic session (admin only)
whoami                             # Display your current session info
status                             # Display all session status
view [lines]                       # View CLI output (default: 20 lines)
echo                               # Echo your IM info (Platform, UserID, Channel)
help                               # Show help information
```

### Dynamic Session Management

clibot supports creating and managing dynamic sessions through IM commands:

**Create a new session** (admin only):
```bash
snew myproject claude ~/projects/myproject
snew backend gemini ~/backend my-custom-gemini
```

**Delete a dynamic session** (admin only):
```bash
sdel myproject
```

**Switch between sessions**:
```bash
suse myproject    # Switch to session 'myproject'
suse backend      # Switch to session 'backend'
```

**Session types**:
- **Static sessions**: Configured in config.yaml, persist across restarts
- **Dynamic sessions**: Created via IM commands, stored in memory only, lost on restart

**Notes**:
- Only admins can create/delete dynamic sessions
- Work directory must exist before creating a session
- Dynamic sessions count against `max_dynamic_sessions` limit (default: 50)
- Static sessions cannot be deleted via IM (must modify config file manually)
- Each user can have their own current session selection (independent of others)

## Special Keywords

Send special keys directly to the CLI tool (no prefix needed):

```
tab            # Send Tab key (for autocomplete)
esc            # Send Escape key
stab/s-tab     # Send Shift+Tab
enter          # Send Enter key
ctrlc/ctrl-c    # Send Ctrl+C (interrupt)
ctrlt/ctrl-t    # Send Ctrl+T
```

**Examples:**
- `tab` → Trigger autocomplete in CLI
- `s-tab` → Navigate back through suggestions
- `ctrl-c` → Interrupt current process
- `ctrl-t` → Trigger Ctrl+T action

## Deployment

For production deployment, clibot can be run as a system service using systemd or supervisor.

**Quick setup (systemd)**:
```bash
# Create dedicated user
sudo useradd -r -s /bin/false clibot

# Install service file
sudo cp deploy/clibot.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable clibot
sudo systemctl start clibot
```

**Quick setup (supervisor)**:
```bash
# Install supervisor
sudo apt-get install supervisor  # Ubuntu/Debian

# Install config file
sudo cp deploy/clibot.conf /etc/supervisor/conf.d/
sudo supervisorctl reread
sudo supervisorctl update
sudo supervisorctl start clibot
```

**Quick setup (management script)**:
```bash
# For development and testing
chmod +x deploy/clibot.sh
./deploy/clibot.sh start
./deploy/clibot.sh status
./deploy/clibot.sh logs
```

For detailed deployment instructions, including:
- Creating dedicated user
- Configuration management
- Log rotation
- Troubleshooting
- Security best practices

See [Deployment Guide](./deploy/DEPLOYMENT.md)

## Security

clibot is essentially a remote code execution tool. **User whitelist must be enabled**. By default, `whitelist_enabled: true`, meaning only whitelisted users can use the system.

## Contributing

Please read [AGENTS.md](AGENTS.md) for development guidelines and language requirements before contributing.

## License

MIT
