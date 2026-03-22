# Clibot Quick Start Guide

This guide provides common configuration examples and workflows for getting started with clibot.

## Important Note

**This guide refers to clibot functionality.** Commands like `clibot install`, `clibot setup`, etc. are provided by the **clibot** deployment helper, not the clibot main program.

When using this skill in Claude Code:
- Use `/clibot setup` to trigger the interactive wizard
- Or run scripts directly: `./path/to/skill/scripts/setup.js`

The clibot main program is a separate binary that gets installed by this skill.

## Prerequisites

- **Node.js >= 20** (for ACP mode)
- **Claude Code CLI** installed and authenticated
- Bot account(s) on your preferred platform(s)

## Installation

### Option 1: Automated Setup (Recommended)

```bash
npx skills add keepmind9/clibot
/clibot setup
```

### Option 2: Manual Installation

```bash
# Download binary
/clibot install

# Run setup wizard
/clibot setup

# Start service
/clibot start
```

## Common Configurations

### Configuration 1: Telegram Only

Minimal setup for Telegram only:

```yaml
sessions:
  - name: claude
    cli_type: claude
    cli_adapter: acp
    work_dir: /home/user/work
    start_cmd: claude
    auto_start: true

telegram:
  enabled: true
  token: "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11"
  webhook_url: ""

discord:
  enabled: false

feishu:
  enabled: false

admins:
  - 123456789

whitelist:
  enabled: false
```

### Configuration 2: Multiple Platforms

Enable Telegram and Discord:

```yaml
sessions:
  - name: claude
    cli_type: claude
    cli_adapter: acp
    work_dir: /home/user/work
    start_cmd: claude

telegram:
  enabled: true
  token: "YOUR_TELEGRAM_BOT_TOKEN"
  webhook_url: ""

discord:
  enabled: true
  token: "YOUR_DISCORD_BOT_TOKEN"
  guild_id: ""

feishu:
  enabled: false

admins:
  - 123456789
  - 987654321

whitelist:
  enabled: false
```

### Configuration 3: With Whitelist Mode

For private bot (only whitelisted users can interact):

```yaml
sessions:
  - name: claude
    cli_type: claude
    cli_adapter: acp
    work_dir: /home/user/work
    start_cmd: claude

telegram:
  enabled: true
  token: "YOUR_TELEGRAM_BOT_TOKEN"

admins:
  - 123456789

whitelist:
  enabled: true
  users:
    - 123456789
    - 111222333
    - 444555666
```

### Configuration 4: Multiple Sessions

Run multiple Claude instances:

```yaml
sessions:
  - name: work
    cli_type: claude
    cli_adapter: acp
    work_dir: /home/user/work
    start_cmd: claude
    auto_start: true

  - name: personal
    cli_type: claude
    cli_adapter: acp
    work_dir: /home/user/personal
    start_cmd: claude
    auto_start: false

telegram:
  enabled: true
  token: "YOUR_TELEGRAM_BOT_TOKEN"

admins:
  - 123456789

whitelist:
  enabled: false
```

## Workflows

### Workflow 1: First-Time Setup

```bash
# Step 1: Install binary
/clibot install

# Step 2: Run setup wizard
/clibot setup

# Step 3: Start service
/clibot start

# Step 4: Check status
/clibot status

# Step 5: Test your bot
# Send a message to your bot on Telegram/Discord
```

### Workflow 2: Add Platform to Existing Setup

```bash
# Step 1: Edit config
nano ~/.clibot/config.yaml

# Step 2: Enable new platform (e.g., discord)
# discord:
#   enabled: true
#   token: "YOUR_DISCORD_BOT_TOKEN"

# Step 3: Validate config
/clibot validate

# Step 4: Restart service
/clibot restart
```

### Workflow 3: Update Binary

```bash
# Step 1: Stop service
/clibot stop

# Step 2: Install new version
/clibot install --version latest

# Step 3: Start service
/clibot start
```

### Workflow 4: Configure Permissions

```bash
# Step 1: Get your user ID
# Send /echo to your bot

# Step 2: Edit config
nano ~/.clibot/config.yaml

# Step 3: Add your user ID to admins
# admins:
#   - YOUR_USER_ID

# Step 4: Optional: Enable whitelist
# whitelist:
#   enabled: true
#   users:
#     - YOUR_USER_ID
#     - OTHER_USER_ID

# Step 5: Restart service
/clibot restart
```

## CLI Adapter Modes

### ACP Mode (Recommended)

Uses Claude Agent SDK via stdio. Minimal dependencies.

```yaml
sessions:
  - name: claude
    cli_adapter: acp
    # ... other config
```

**Pros:**
- Stable and reliable
- No tmux dependency
- Direct protocol communication

**Cons:**
- Requires Claude Agent SDK support

### Tmux Mode

Uses tmux for session management.

```yaml
sessions:
  - name: claude
    cli_adapter: tmux
    # ... other config
```

**Pros:**
- Works with any CLI tool
- Supports special keywords (tab, ctrl-c, etc.)

**Cons:**
- Requires tmux installation
- Requires Hook mode setup

## Service Management

### Start Service

```bash
/clibot start
```

Service runs in background. Logs written to `~/.clibot/logs/clibot.log`.

### Stop Service

```bash
/clibot stop
```

Gracefully stops the service (waits up to 10 seconds).

### Restart Service

```bash
/clibot restart
```

Stops and starts the service. Useful for config changes.

### Check Status

```bash
/clibot status
```

Shows:
- Running status
- PID and uptime
- Memory usage
- Recent log entries

## Log Management

### View Logs

```bash
# Follow logs in real-time
tail -f ~/.clibot/logs/clibot.log

# View last 50 lines
tail -n 50 ~/.clibot/logs/clibot.log

# Search for errors
grep ERROR ~/.clibot/logs/clibot.log
```

### Rotate Logs

```bash
# Archive current log
mv ~/.clibot/logs/clibot.log ~/.clibot/logs/clibot.log.$(date +%Y%m%d)

# Restart to create new log
/clibot restart
```

## Troubleshooting

### Service Won't Start

1. Check logs: `tail -f ~/.clibot/logs/clibot.log`
2. Validate config: `/clibot validate`
3. Check port availability: `lsof -i :8080` (if applicable)

### Bot Not Responding

1. Check service status: `clibot status`
2. Verify bot token is correct
3. Check you're in the whitelist (if enabled)
4. Send `/echo` to test connectivity

### Permission Errors

1. Check config file permissions: `ls -la ~/.clibot/config.yaml`
2. Should be 600 or 644: `chmod 600 ~/.clibot/config.yaml`
3. Check binary permissions: `ls -la $(which clibot)`
4. Should be executable: `chmod +x $(which clibot)`

## Advanced Configuration

### Environment Variables

```yaml
sessions:
  - name: claude
    cli_type: claude
    cli_adapter: acp
    work_dir: /home/user/work
    start_cmd: claude
    env:
      CLAUDE_API_KEY: "your-api-key"
      ANTHROPIC_LOG: "debug"
```

### Webhook Configuration

For production deployments with webhooks:

```yaml
telegram:
  enabled: true
  token: "YOUR_TOKEN"
  webhook_url: "https://your-domain.com/webhook/telegram"
```

Note: Webhooks require HTTPS and proper domain configuration.

## Getting Help

- **Quick help**: `/clibot help`
- **Setup wizard**: `clibot setup`
- **Validate config**: `/clibot validate`
- **Check logs**: `clibot status`
- **Full docs**: https://github.com/keepmind9/clibot
