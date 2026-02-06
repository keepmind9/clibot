# clibot

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
use <session>                      # Switch current session (not implemented yet)
snew <name> <cli_type> <work_dir> [cmd]  # Create new dynamic session (admin only)
sdel <name>                        # Delete dynamic session (admin only)
whoami                             # Display current session info
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

**Session types**:
- **Static sessions**: Configured in config.yaml, persist across restarts
- **Dynamic sessions**: Created via IM commands, stored in memory only, lost on restart

**Notes**:
- Only admins can create/delete dynamic sessions
- Work directory must exist before creating a session
- Dynamic sessions count against `max_dynamic_sessions` limit (default: 50)
- Static sessions cannot be deleted via IM (must modify config file manually)

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

## Security

clibot is essentially a remote code execution tool. **User whitelist must be enabled**. By default, `whitelist_enabled: true`, meaning only whitelisted users can use the system.

## Contributing

Please read [AGENTS.md](AGENTS.md) for development guidelines and language requirements before contributing.

## License

MIT
