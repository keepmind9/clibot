# Clibot Troubleshooting Guide

This guide helps you diagnose and resolve common issues with clibot.

## Table of Contents

1. [Installation Issues](#installation-issues)
2. [Service Issues](#service-issues)
3. [Bot Issues](#bot-issues)
4. [Configuration Issues](#configuration-issues)
5. [Session Issues](#session-issues)
6. [Permission Issues](#permission-issues)

---

## Installation Issues

### Issue: Binary Download Fails

**Symptoms:**
```
❌ Failed to download binary
```

**Possible Causes:**
- No internet connection
- GitHub is blocked
- Firewall restrictions

**Solutions:**

1. Check internet connectivity:
```bash
ping -c 3 github.com
```

2. Try downloading manually:
```bash
# Detect your platform first
uname -s   # Linux or Darwin
uname -m   # x86_64 or arm64

# Download (replace with correct filename)
curl -LO https://github.com/keepmind9/clibot/releases/latest/download/clibot-linux-amd64
chmod +x clibot-linux-amd64
mv clibot-linux-amd64 ~/.local/bin/clibot
```

3. Use specific version:
```bash
/clibot install --version v0.1.0
```

4. Check proxy settings:
```bash
export https_proxy=your_proxy_server
/clibot install
```

### Issue: Permission Denied When Installing

**Symptoms:**
```
mv: cannot move 'clibot' to '/usr/local/bin/clibot': Permission denied
```

This means you tried to install to a system directory. Use `~/.local/bin` instead:
```bash
/clibot install --prefix ~/.local/bin
```

**Solution:**

Install to user directory instead:
```bash
/clibot install --prefix ~/.local/bin
export PATH="$PATH:$HOME/.local/bin"
```

### Issue: Command Not Found

**Symptoms:**
```
bash: clibot: command not found
```

**Solutions:**

1. Check installation:
```bash
which clibot
find ~/.local/bin -name clibot 2>/dev/null
```

2. Add to PATH if needed:
```bash
# Add to ~/.bashrc or ~/.zshrc
export PATH="$PATH:$HOME/.local/bin"
source ~/.bashrc
```

3. Verify executable:
```bash
ls -la $(which clibot)
# Should show -rwxr-xr-x (executable)
```

---

## Service Issues

### Issue: Service Won't Start

**Symptoms:**
```
❌ Failed to start service
```

**Diagnosis:**

1. Check logs:
```bash
tail -n 50 ~/.clibot/logs/clibot.log
```

2. Validate configuration:
```bash
/clibot validate
```

3. Check for running processes:
```bash
ps aux | grep clibot
```

**Common Solutions:**

1. **Port already in use:**
```bash
# Find process using the port
lsof -i :8080  # or your configured port
# Kill the process or change port in config
```

2. **Config file missing:**
```bash
# Check config exists
ls -la ~/.clibot/config.yaml
# If missing, run setup
/clibot setup
```

3. **Missing dependencies:**
```bash
# For ACP mode: check Node.js
node --version  # Should be >= 20

# For tmux mode: check tmux
tmux -V
```

### Issue: Service Stops Unexpectedly

**Symptoms:**
Service starts but stops after a few seconds.

**Diagnosis:**

1. Check logs for errors:
```bash
tail -f ~/.clibot/logs/clibot.log
```

2. Look for specific error patterns:
```bash
grep -i "error\|panic\|fatal" ~/.clibot/logs/clibot.log
```

**Common Solutions:**

1. **Invalid bot token:**
```bash
# Validate token format
/clibot validate
# Re-run setup if needed
/clibot setup
```

2. **Missing session directory:**
```bash
# Create work directory
mkdir -p ~/work
```

3. **Out of memory:**
```bash
# Check system resources
free -h
# Consider reducing session count
```

### Issue: Service Won't Stop

**Symptoms:**
```
⚠ Graceful shutdown timeout, forcing...
```

**Solution:**

Force kill the process:
```bash
# Find the process
ps aux | grep clibot
# Kill it
kill -9 <PID>
# Clean up PID file
rm -f ~/.clibot/clibot.pid
```

---

## Bot Issues

### Issue: Bot Not Responding

**Symptoms:**
You send messages but bot doesn't reply.

**Diagnosis:**

1. Check service status:
```bash
/clibot status
```

2. Check logs:
```bash
tail -f ~/.clibot/logs/clibot.log
```

**Common Solutions:**

1. **Service not running:**
```bash
/clibot start
```

2. **Bot token invalid:**
```bash
# Test with Telegram
curl https://api.telegram.org/bot<YOUR_TOKEN>/getMe

# Should return bot info
# If error, token is invalid - get new token from BotFather
```

3. **Not in whitelist (if enabled):**
```bash
# Check your user ID
# Send /echo to bot (if service is running)
# Add your ID to config
nano ~/.clibot/config.yaml
# Add to whitelist.users or admins
/clibot restart
```

4. **Webhook not configured (if using webhooks):**
```bash
# Switch to polling mode
nano ~/.clibot/config.yaml
# Set webhook_url to ""
/clibot restart
```

### Issue: Bot Commands Not Working

**Symptoms:**
Bot responds but commands like `/slist` don't work.

**Diagnosis:**

1. Check if you're admin:
```bash
# Send /echo to see your user ID
# Verify it's in admins list
```

2. Check logs for errors:
```bash
grep -i "command\|error" ~/.clibot/logs/clibot.log
```

**Solutions:**

1. **Not authorized:**
```bash
# Add yourself to admins
nano ~/.clibot/config.yaml
# Add to admins list
/clibot restart
```

2. **Session not created:**
```bash
# Send /slist to check sessions
# Create session if needed: /snew ...
```

### Issue: Telegram Bot: Webhook Error

**Symptoms:**
```
Error setting webhook: 404 Not Found
```

**Solution:**

Disable webhook or fix URL:
```yaml
telegram:
  enabled: true
  token: "YOUR_TOKEN"
  webhook_url: ""  # Empty for polling mode
```

---

## Configuration Issues

### Issue: Config Validation Fails

**Symptoms:**
```
❌ Configuration validation failed
```

**Diagnosis:**

1. Run validation:
```bash
/clibot validate
```

2. Check YAML syntax:
```bash
python3 -c "import yaml; yaml.safe_load(open('~/.clibot/config.yaml'))"
```

**Common Solutions:**

1. **YAML syntax error:**
```bash
# Use yaml linting
yamllint ~/.clibot/config.yaml

# Fix indentation (use spaces, not tabs)
# Fix quote marks
# Fix list format
```

2. **Missing required fields:**
```bash
# Ensure sessions and at least one bot are configured
sessions:
  - name: claude
    # ...

telegram:
  enabled: true
  token: "..."
```

3. **Invalid token format:**
```bash
# Telegram: should be "123456:ABC-DEF..."
# Discord: should be 59+ characters alphanumeric
# Regenerate token if needed
```

### Issue: Changes Not Applied

**Symptoms:**
You edited config but nothing changed.

**Solution:**

Restart the service:
```bash
/clibot restart
```

### Issue: Config File Not Found

**Symptoms:**
```
✗ Config file not found: ~/.clibot/config.yaml
```

**Solution:**

Run setup wizard:
```bash
/clibot setup
```

Or create manually:
```bash
mkdir -p ~/.clibot
nano ~/.clibot/config.yaml
# Add your configuration
```

---

## Session Issues

### Issue: Session Won't Start

**Symptoms:**
```
❌ Failed to start session 'claude'
```

**Diagnosis:**

1. Check logs:
```bash
tail -f ~/.clibot/logs/clibot.log
```

2. Check session status:
```bash
# Send /sstatus to bot
```

**Common Solutions:**

1. **Invalid work directory:**
```bash
# Create directory
mkdir -p ~/work
```

2. **CLI command not found:**
```bash
# Verify command exists
which claude
# Add to PATH if needed
export PATH="$PATH:/path/to/claude"
```

3. **Missing dependencies:**
```bash
# For ACP mode: check Node.js
node --version

# For tmux mode: check tmux
tmux -V
```

4. **Permission denied:**
```bash
# Check directory permissions
ls -la ~/work
# Should be writable by user
```

### Issue: Session Not Responding

**Symptoms:**
Session is running but messages not processed.

**Diagnosis:**

1. Check session state:
```bash
# Send /sstatus to bot
# Look for "processing" or "error" state
```

2. Check logs:
```bash
grep -i "session\|claude" ~/.clibot/logs/clibot.log | tail -20
```

**Solutions:**

1. **Session crashed:**
```bash
# Restart session
# Send /srestart to bot
# Or restart service: /clibot restart
```

2. **Claude Code not responding:**
```bash
# Check Claude Code status
claude --version
# Re-authenticate if needed
claude auth login
```

3. **Session stuck in processing:**
```bash
# Force restart session
# Send /sforce or /sclose then /suse
```

### Issue: Multiple Sessions Confusion

**Symptoms:**
Messages go to wrong session.

**Solution:**

1. Check current session:
```bash
# Send /whoami to bot
```

2. Switch session:
```bash
# Send /suse <session_name>
```

3. List all sessions:
```bash
# Send /slist to bot
```

---

## Permission Issues

### Issue: Access Denied Errors

**Symptoms:**
```
❌ Permission denied
```

**Diagnosis:**

1. Check file permissions:
```bash
ls -la ~/.clibot/config.yaml
```

2. Check process owner:
```bash
ps aux | grep clibot
```

**Solutions:**

1. **Fix config file permissions:**
```bash
chmod 600 ~/.clibot/config.yaml
```

2. **Fix log directory permissions:**
```bash
chmod 755 ~/.clibot
chmod 755 ~/.clibot/logs
```

3. **Don't run as root:**
```bash
# Run as regular user, not sudo
/clibot start  # NOT sudo /clibot start
```

### Issue: Can't Create Session (Non-Admin)

**Symptoms:**
```
❌ Only admins can create sessions
```

**Solution:**

Add your user ID to admins:
```bash
# Get your user ID
# Send /echo to bot

# Add to config
nano ~/.clibot/config.yaml
admins:
  - YOUR_USER_ID

# Restart
/clibot restart
```

### Issue: Whitelist Blocking Access

**Symptoms:**
You're admin but can't use bot.

**Diagnosis:**

1. Check whitelist status:
```bash
grep -A 2 "whitelist:" ~/.clibot/config.yaml
```

2. Check if you're in whitelist:
```bash
grep -A 10 "whitelist:" ~/.clibot/config.yaml | grep YOUR_ID
```

**Solution:**

Add yourself to whitelist or disable it:
```yaml
whitelist:
  enabled: false
# OR
  enabled: true
  users:
    - YOUR_USER_ID
```

---

## Getting More Help

If none of these solutions work:

1. **Check logs for specific errors:**
```bash
tail -100 ~/.clibot/logs/clibot.log
```

2. **Enable debug logging:**
```yaml
# In config.yaml
logging:
  level: debug
```

3. **Report issue on GitHub:**
https://github.com/keepmind9/clibot/issues

Include:
- clibot version: `clibot --version`
- OS and architecture
- Full error message
- Relevant log entries
- Configuration (with tokens redacted)
