# Deployment Guide

This guide covers deploying clibot in production using systemd or supervisor.

## Platform Support

**Supported Platforms**:
- ✅ **Linux** - Fully supported and recommended for production
- ✅ **macOS** - Fully supported
- ⚠️ **Windows** - Only via WSL2 (not recommended for production)

**Windows users**:
- Use WSL2 for development/testing
- For production, deploy to a Linux server (VPS, cloud, etc.)
- See [Windows setup guide](../README.md#windows-setup-wsl2) for details

## Table of Contents

- [Prerequisites](#prerequisites)
- [Creating a Dedicated User](#creating-a-dedicated-user)
- [Deployment with systemd](#deployment-with-systemd)
- [Deployment with Supervisor](#deployment-with-supervisor)
- [Verification](#verification)
- [Troubleshooting](#troubleshooting)
- [Uninstallation](#uninstallation)

## Prerequisites

1. **Install clibot**:
```bash
go install github.com/keepmind9/clibot@latest
```

2. **Install tmux** (required):
```bash
# Ubuntu/Debian
sudo apt-get install tmux

# macOS
brew install tmux

# Fedora/CentOS/RHEL
sudo dnf install tmux
```

**Note**: You can choose any config file location. Common options:
- `/etc/clibot/config.yaml` - System-wide (used in examples below)
- `~/.config/clibot/config.yaml` - User-specific
- `/opt/clibot/config.yaml` - Custom location

If you use a custom location, update the path in:
- systemd service file (`ExecStart=` line)
- supervisor config file (`command=` line)
- All commands below that reference `--config`

3. **Configure clibot**:
```bash
# Create config directory (using /etc/clibot as example)
sudo mkdir -p /etc/clibot

# Copy config template
sudo cp configs/config.yaml /etc/clibot/config.yaml

# Edit configuration
sudo nano /etc/clibot/config.yaml
```

**Important**: Fill in your bot credentials and whitelist users in the config file.

## Creating a Dedicated User

For security, create a dedicated user to run clibot:

```bash
# Create clibot user (no login, no home directory)
sudo useradd -r -s /bin/false clibot

# Create necessary directories
sudo mkdir -p /etc/clibot
sudo mkdir -p /var/log/clibot

# Set ownership
sudo chown -R clibot:clibot /etc/clibot
sudo chown -R clibot:clibot /var/log/clibot

# Set permissions
sudo chmod 750 /etc/clibot
sudo chmod 750 /var/log/clibot
```

## Deployment with systemd

systemd is the init system for modern Linux distributions (Ubuntu 16.04+, CentOS 7+, etc.).

### Installation

1. **Copy the service file**:
```bash
sudo cp deploy/clibot.service /etc/systemd/system/clibot.service
```

**Customize paths** (optional):
If you're using a different config location or binary path, edit the service file:
```bash
sudo nano /etc/systemd/system/clibot.service
```

Key lines to customize:
- `ExecStart=/usr/local/bin/clibot serve --config /etc/clibot/config.yaml`
  - Change binary path if installed elsewhere
  - Change `--config` path to your config location
- `User=clibot` and `Group=clibot`
  - Change if using a different user
- `WorkingDirectory=/opt/clibot`
  - Change to your preferred working directory

2. **Reload systemd**:
```bash
sudo systemctl daemon-reload
```

3. **Enable clibot** to start on boot:
```bash
sudo systemctl enable clibot
```

4. **Start clibot**:
```bash
sudo systemctl start clibot
```

### Management Commands

```bash
# Check status
sudo systemctl status clibot

# Stop clibot
sudo systemctl stop clibot

# Restart clibot
sudo systemctl restart clibot

# View logs
sudo journalctl -u clibot -f

# View logs since last boot
sudo journalctl -u clibot -b

# View last 100 lines
sudo journalctl -u clibot -n 100
```

### Log Rotation

systemd handles log rotation automatically via journald. To configure persistent logging:

```bash
# Create journal directory
sudo mkdir -p /var/log/journal

# Restart journald
sudo systemctl restart systemd-journald
```

## Deployment with Supervisor

Supervisor is a process control system for Unix-like operating systems.

### Installation

1. **Install supervisor**:
```bash
# Ubuntu/Debian
sudo apt-get install supervisor

# Fedora/CentOS/RHEL
sudo dnf install supervisor

# macOS
brew install supervisor
```

2. **Copy the configuration file**:
```bash
sudo cp deploy/clibot.conf /etc/supervisor/conf.d/clibot.conf
```

**Customize paths** (optional):
If you're using a different config location or binary path, edit the config file:
```bash
sudo nano /etc/supervisor/conf.d/clibot.conf
```

Key lines to customize:
- `command=/usr/local/bin/clibot serve --config /etc/clibot/config.yaml`
  - Change binary path if installed elsewhere
  - Change `--config` path to your config location
- `user=clibot`
  - Change if using a different user
- `stdout_logfile=/var/log/clibot/clibot.log`
  - Change to your preferred log location

3. **Reread and update supervisor**:
```bash
sudo supervisorctl reread
sudo supervisorctl update
```

4. **Start clibot**:
```bash
sudo supervisorctl start clibot
```

### Management Commands

```bash
# Check status
sudo supervisorctl status clibot

# Stop clibot
sudo supervisorctl stop clibot

# Restart clibot
sudo supervisorctl restart clibot

# View logs
sudo supervisorctl tail -f clibot

# View stderr logs
sudo supervisorctl tail -f clibot stderr

# View stdout logs
sudo supervisorctl tail -f clibot stdout
```

### Log Rotation

Supervisor handles log rotation automatically based on the settings in `clibot.conf`:
- `stdout_logfile_maxbytes=50MB` - Rotate at 50MB
- `stdout_logfile_backups=10` - Keep 10 backup files

## Verification

### Verify clibot is running

```bash
# Check if tmux sessions exist
sudo -u clibot tmux list-sessions

# Check if clibot is listening on port 8080
sudo netstat -tlnp | grep 8080

# Or using ss
sudo ss -tlnp | grep 8080
```

### Test from IM

Send a message to your bot:
```
status
```

You should receive a status response.

## Troubleshooting

### clibot won't start

1. **Check the service status**:
```bash
# systemd
sudo systemctl status clibot

# supervisor
sudo supervisorctl status clibot
```

2. **Check the logs**:
```bash
# systemd
sudo journalctl -u clibot -n 100

# supervisor
sudo tail -100 /var/log/clibot/clibot.log
```

3. **Common issues**:

   **Issue**: Permission denied
   **Solution**:
   ```bash
   sudo chown -R clibot:clibot /etc/clibot
   sudo chown -R clibot:clibot /var/log/clibot
   ```

   **Issue**: Config file not found
   **Solution**: Ensure `/etc/clibot/config.yaml` exists and is readable

   **Issue**: Port already in use
   **Solution**:
   ```bash
   # Find process using port 8080
   sudo lsof -i :8080
   # Change port in config.yaml
   ```

   **Issue**: tmux not found
   **Solution**:
   ```bash
   # Install tmux
   sudo apt-get install tmux  # Ubuntu/Debian
   ```

### Manual testing

Run clibot manually to debug issues:

```bash
# Run as clibot user
sudo -u clibot /usr/local/bin/clibot serve --config /etc/clibot/config.yaml

# Or run with debug logging
sudo -u clibot /usr/local/bin/clibot serve --config /etc/clibot/config.yaml --log-level debug
```

## Uninstallation

### systemd

```bash
# Stop and disable
sudo systemctl stop clibot
sudo systemctl disable clibot

# Remove service file
sudo rm /etc/systemd/system/clibot.service
sudo systemctl daemon-reload

# Remove user and directories (optional)
sudo userdel clibot
sudo rm -rf /etc/clibot
sudo rm -rf /var/log/clibot
```

### Supervisor

```bash
# Stop clibot
sudo supervisorctl stop clibot

# Remove config
sudo rm /etc/supervisor/conf.d/clibot.conf

# Reread and update
sudo supervisorctl reread
sudo supervisorctl update

# Remove user and directories (optional)
sudo userdel clibot
sudo rm -rf /etc/clibot
sudo rm -rf /var/log/clibot
```

## Production Tips

### Security

1. **Use a dedicated user** (as shown above)
2. **Enable whitelist** in config.yaml:
```yaml
security:
  whitelist_enabled: true
  allowed_users:
    discord:
      - "your-user-id"
```

3. **Use environment variables for secrets**:
```yaml
bots:
  discord:
    token: "${DISCORD_TOKEN}"
```

Set via:
```bash
# For systemd
sudo nano /etc/systemd/system/clibot.service
# Add: Environment=DISCORD_TOKEN=your_token

# For supervisor
sudo nano /etc/supervisor/conf.d/clibot.conf
# Add: environment=DISCORD_TOKEN="your_token"
```

### Performance

1. **Limit dynamic sessions**:
```yaml
session:
  max_dynamic_sessions: 50
```

2. **Adjust log levels** for production:
```yaml
logging:
  level: info  # or warn
  enable_stdout: false
```

3. **Monitor resources**:
```bash
# Check memory usage
ps aux | grep clibot

# Check tmux sessions
sudo -u clibot tmux list-sessions
```

### Backup

Back up your configuration:
```bash
sudo cp /etc/clibot/config.yaml /etc/clibot/config.yaml.backup
```

## Additional Resources

- [README.md](../README.md) - Main documentation
- [SECURITY.md](../SECURITY.md) - Security best practices
- [Configuration Guide](../README.md#configuration) - Config file reference
