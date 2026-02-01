# clibot

clibot is a lightweight middleware that connects various IM platforms (Feishu, Discord, Telegram, etc.) with AI CLI tools (Claude Code, Gemini CLI, OpenCode, etc.), enabling users to remotely use AI programming assistants through chat interfaces.

## Features

- **Access Anywhere**: Use powerful desktop AI CLI tools from your mobile phone or tablet via IM
- **Unified Entry Point**: Manage multiple AI CLI tools through a single IM bot with easy switching
- **Flexible Extension**: Abstract interface design - add new CLI or Bot by simply implementing interfaces
- **Transparent Proxy**: Most inputs are directly passed through to CLI, maintaining native user experience

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

3. Configure Claude Code Hook (example):
```json
{
  "hooks": {
    "onCompletion": "clibot hook --session $CLIBOT_SESSION --event completed"
  }
}
```

### Usage

```bash
# Start the main process
clibot start --config ~/.config/clibot/config.yaml

# Check status
clibot status
```

## Project Structure

```
clibot/
├── cmd/                    # CLI entry point
│   └── clibot/             # Main program
│       ├── main.go         # Main function
│       ├── root.go         # Cobra root command
│       ├── start.go        # start command
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
!!sessions              # List all sessions
!!use <session>         # Switch current session
!!new <name> <cli>      # Create new session
!!whoami                # Display current session info
!!status                # Display all session status
!!view [lines]          # View CLI output (default: 20 lines)
!!help                  # Show help information
```

## Special Keywords

Send special keys directly to the CLI tool (no prefix needed):

```
tab          # Send Tab key (for autocomplete)
esc          # Send Escape key
stab/s-tab   # Send Shift+Tab
enter        # Send Enter key
ctrlc/ctrl-c # Send Ctrl+C (interrupt)
```

**Examples:**
- `tab` → Trigger autocomplete in CLI
- `s-tab` → Navigate back through suggestions
- `ctrl-c` → Interrupt current process

## Security

clibot is essentially a remote code execution tool. **User whitelist must be enabled**. By default, `whitelist_enabled: true`, meaning only whitelisted users can use the system.

## Version

Current version: v0.3

For detailed design documentation, see: [docs/plans/2026-01-28-clibot-design.md](docs/plans/2026-01-28-clibot-design.md)

## Contributing

Please read [AGENTS.md](AGENTS.md) for development guidelines and language requirements before contributing.

## License

MIT
