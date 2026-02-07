# clibot Management Script Guide

## Overview

`clibot.sh` is a bash management script for development and testing environments. It provides convenient commands to start, stop, restart, and monitor clibot service status.

**⚠️ Important**:
- This script is for **development and testing environments only**
- For **production**, use systemd or supervisor
- See: [Deployment Guide](./DEPLOYMENT.md)

## Quick Start

### 1. Make Script Executable

```bash
chmod +x deploy/clibot.sh
```

### 2. Basic Usage

```bash
# Start clibot
./deploy/clibot.sh start

# Check status
./deploy/clibot.sh status

# View logs
./deploy/clibot.sh logs

# Stop clibot
./deploy/clibot.sh stop

# Restart clibot
./deploy/clibot.sh restart
```

## Commands

| Command | Description |
|---------|-------------|
| `start` | Start clibot service (runs in background) |
| `stop` | Stop clibot service |
| `restart` | Restart clibot service |
| `status` | Display service status and process info |
| `logs` | View logs in real-time (like tail -f) |
| `help` | Show help message |

## Customization

### Environment Variables

Customize paths using environment variables:

```bash
# Use custom config file
CONFIG_FILE=/etc/clibot/config.yaml ./deploy/clibot.sh start

# Use custom binary
CLIBOT_BIN=/usr/local/bin/clibot ./deploy/clibot.sh start

# Use custom PID file
PID_FILE=/var/run/clibot.pid ./deploy/clibot.sh start

# Use custom log file
LOG_FILE=/var/log/clibot/clibot.log ./deploy/clibot.sh start
```

### Default Paths

| Variable | Default | Description |
|----------|---------|-------------|
| `CLIBOT_BIN` | `clibot` | Binary file path |
| `CONFIG_FILE` | `~/.config/clibot/config.yaml` | Config file path |
| `PID_FILE` | `/tmp/clibot.pid` | PID file path |
| `LOG_FILE` | `/tmp/clibot.log` | Log file path |

## Use Cases

### Suitable For

✅ **Local Development** - Quick start/stop for testing
✅ **Feature Validation** - Test new configs or features
✅ **Debug Mode** - Use with IDE or editors
✅ **Temporary Testing** - No need for system service config

### Not Suitable For

❌ **Production** - Use systemd or supervisor instead
❌ **Long-term Running** - No auto-restart or monitoring
❌ **Multiple Instances** - Use supervisor for better management

## Troubleshooting

### 1. Binary Not Found

**Error**:
```
[ERROR] clibot binary not found: clibot
```

**Solution**:
```bash
# Install clibot
go install github.com/keepmind9/clibot@latest

# Or set custom path
CLIBOT_BIN=/path/to/clibot ./deploy/clibot.sh start
```

### 2. Config File Not Found

**Error**:
```
[ERROR] Config file not found: ~/.config/clibot/config.yaml
```

**Solution**:
```bash
# Create config directory
mkdir -p ~/.config/clibot

# Copy config template
cp configs/config.yaml ~/.config/clibot/config.yaml

# Edit config file
nano ~/.config/clibot/config.yaml
```

### 3. Port Already in Use

**Error**:
```
Failed to bind to port 8080
```

**Solution**:
```bash
# Find process using port
lsof -i :8080

# Or change port in config
nano ~/.config/clibot/config.yaml
```

### 4. View Help

```bash
./deploy/clibot.sh help
```

## Advanced Usage

### Create Shortcut Command

```bash
# Add to PATH
sudo ln -s $(pwd)/deploy/clibot.sh /usr/local/bin/clibot-ctl

# Use shortcut
clibot-ctl start
clibot-ctl status
clibot-ctl logs
```

### Configure Aliases

Add to `~/.bashrc` or `~/.zshrc`:

```bash
# clibot management shortcuts
alias clibot-start='~/path/to/deploy/clibot.sh start'
alias clibot-stop='~/path/to/deploy/clibot.sh stop'
alias clibot-restart='~/path/to/deploy/clibot.sh restart'
alias clibot-status='~/path/to/deploy/clibot.sh status'
alias clibot-logs='~/path/to/deploy/clibot.sh logs'
```

Then:

```bash
source ~/.bashrc  # or source ~/.zshrc

# Use aliases
clibot-start
clibot-status
```

## Comparison

| Feature | clibot.sh | systemd | supervisor |
|---------|-----------|---------|------------|
| Ease of Use | ⭐⭐⭐⭐⭐ | ⭐⭐⭐ | ⭐⭐⭐⭐ |
| Auto Restart | ❌ | ✅ | ✅ |
| Boot Start | ❌ | ✅ | ✅ |
| Log Management | Basic | Advanced | Advanced |
| Process Monitor | None | Advanced | Advanced |
| Best For | Dev/Test | Production | Production |

## Related Documentation

- [Deployment Guide](./DEPLOYMENT.md) - Complete deployment instructions
- [systemd Deployment](./DEPLOYMENT.md#deployment-with-systemd) - Recommended for production
- [supervisor Deployment](./DEPLOYMENT.md#deployment-with-supervisor) - Cross-platform solution
- [README](../README.md) - Main project documentation
