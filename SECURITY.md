# Security Policy

## Supported Versions

Currently, only the latest version of clibot is supported with security updates.

| Version | Supported          |
| ------- | ------------------ |
| Latest  | :white_check_mark: |

## Reporting a Vulnerability

**If you discover a security vulnerability, please do NOT open a public issue.**

Instead, please send an email to: **security@keepmind9.com**

Please include:
- A description of the vulnerability
- Steps to reproduce the issue
- Potential impact assessment
- Any suggested mitigation (if known)

### What Happens Next?

1. **Confirmation**: We will acknowledge receipt of your report within 48 hours
2. **Investigation**: We will investigate the vulnerability and determine its severity
3. **Resolution**: We will work on a fix and coordinate disclosure with you
4. **Disclosure**: We will announce the security fix and release a patched version

Our goal is to resolve critical security issues within 7 days of confirmation.

## Security Best Practices

### For Deployment

clibot is a powerful tool that can execute arbitrary code remotely. **Follow these security guidelines:**

#### 1. Enable Whitelist (Required)
```yaml
security:
  whitelist_enabled: true  # MUST be true
  allowed_users:
    discord:
      - "your-user-id-1"
      - "your-user-id-2"
```

**NEVER deploy with `whitelist_enabled: false` in production.**

#### 2. Use Environment Variables for Secrets
```yaml
bots:
  discord:
    token: "${DISCORD_TOKEN}"  # Use environment variable
```

Set secrets via environment:
```bash
export DISCORD_TOKEN="your-secret-token"
clibot serve
```

#### 3. Restrict Bot Permissions
- Use the minimum required permissions for your bot
- Don't grant admin privileges unless absolutely necessary
- Regularly audit bot permissions in the IM platform

#### 4. Limit Dynamic Sessions
```yaml
session:
  max_dynamic_sessions: 50  # Reasonable limit
```

#### 5. Run as Non-Root User
```bash
# Create dedicated user
useradd -r -s /bin/false clibot

# Run as clibot user
sudo -u clibot clibot serve
```

#### 6. Use Firewall Rules
Only expose necessary ports:
```bash
# Only allow local connections (if using reverse proxy)
iptables -A INPUT -p tcp --dport 8080 -s 127.0.0.1 -j ACCEPT
iptables -A INPUT -p tcp --dport 8080 -j DROP
```

#### 7. Enable Logging and Monitoring
```yaml
logging:
  level: info
  file: /var/log/clibot/clibot.log
  enable_stdout: true
```

Regularly review logs for suspicious activity.

### For Users

#### 1. Understand the Risk
clibot enables remote code execution. Anyone with access to your bot can:
- Execute arbitrary commands
- Read/write files in work directories
- Access system resources
- Modify data

**Only whitelist trusted users.**

#### 2. Use Dedicated Work Directories
```yaml
sessions:
  - name: "my-session"
    work_dir: "/home/user/sandbox"  # Isolated directory
```

Avoid using:
- `/` (root directory)
- `/home` (all user data)
- System directories (`/etc`, `/var`, etc.)

#### 3. Regularly Review Whitelist
Periodically audit your `allowed_users` list and remove users who no longer need access.

#### 4. Monitor Usage
Check logs regularly for:
- Unusual command patterns
- Access from unexpected users
- Failed authorization attempts

### For Development

#### 1. Keep Dependencies Updated
```bash
go get -u ./...
go mod tidy
```

#### 2. Use Go's Security Features
- Always validate user input
- Use `context.Context` for timeouts
- Avoid `eval`-like operations
- Sanitize file paths

#### 3. Follow Security Best Practices
- Never log sensitive data (tokens, passwords)
- Use secure random number generation
- Implement rate limiting for API endpoints
- Validate all external input

## Security Features

### Built-in Protections

clibot includes several security features:

- **Whitelist Enforcement**: Only authorized users can interact
- **Admin Separation**: Sensitive operations require admin privileges
- **Session Isolation**: Each session runs in a separate tmux session
- **Input Validation**: All user input is validated before execution

### Audit Trail

All operations are logged with:
- Timestamp
- User identity (platform, user ID)
- Command executed
- Result

Enable detailed logging for security audits:
```yaml
logging:
  level: debug
```

## Responsible Disclosure

We appreciate responsible disclosure and will:

- Keep you informed throughout the process
- Credit you in the security advisory (if desired)
- Work with you on the disclosure timeline

## Security Updates

Security updates will be:
1. Announced in the release notes
2. Tagged with `security` in the changelog
3. Pushed as patch versions (e.g., `v1.0.1` → `v1.0.2`)

Subscribe to [Releases](https://github.com/keepmind9/clibot/releases) to receive notifications.

## Contact

For general security questions or concerns:
- Email: security@keepmind9.com
- GitHub Security: [https://github.com/keepmind9/clibot/security](https://github.com/keepmind9/clibot/security)

## Related Documentation

- [Configuration Guide](./README.md#configuration)
- [Deployment Guide](./docs/deployment.md)
- [Contributing Guidelines](./CONTRIBUTING.md)
