# 部署指南

本指南介绍如何使用 systemd 或 supervisor 在生产环境中部署 clibot。

## 目录

- [前置要求](#前置要求)
- [创建专用用户](#创建专用用户)
- [使用 systemd 部署](#使用-systemd-部署)
- [使用 Supervisor 部署](#使用-supervisor-部署)
- [验证部署](#验证部署)
- [故障排查](#故障排查)
- [卸载](#卸载)

## 前置要求

1. **安装 clibot**：
```bash
go install github.com/keepmind9/clibot@latest
```

2. **安装 tmux**（必需）：
```bash
# Ubuntu/Debian
sudo apt-get install tmux

# macOS
brew install tmux

# Fedora/CentOS/RHEL
sudo dnf install tmux
```

**注意**：您可以选择任意配置文件位置。常见选项：
- `/etc/clibot/config.yaml` - 系统级配置（下文示例使用此路径）
- `~/.config/clibot/config.yaml` - 用户级配置
- `/opt/clibot/config.yaml` - 自定义位置

如果使用自定义位置，需要更新以下路径：
- systemd 服务文件中的 `ExecStart=` 行
- supervisor 配置文件中的 `command=` 行
- 下文所有引用 `--config` 的命令

3. **配置 clibot**：
```bash
# 创建配置目录（以 /etc/clibot 为例）
sudo mkdir -p /etc/clibot

# 复制配置模板
sudo cp configs/config.yaml /etc/clibot/config.yaml

# 编辑配置文件
sudo nano /etc/clibot/config.yaml
```

**重要提示**：在配置文件中填写您的 Bot 凭据和白名单用户。

## 创建专用用户

为了安全起见，创建专用用户来运行 clibot：

```bash
# 创建 clibot 用户（无登录权限，无主目录）
sudo useradd -r -s /bin/false clibot

# 创建必要目录
sudo mkdir -p /etc/clibot
sudo mkdir -p /var/log/clibot

# 设置所有权
sudo chown -R clibot:clibot /etc/clibot
sudo chown -R clibot:clibot /var/log/clibot

# 设置权限
sudo chmod 750 /etc/clibot
sudo chmod 750 /var/log/clibot
```

## 使用 systemd 部署

systemd 是现代 Linux 发行版的初始化系统（Ubuntu 16.04+、CentOS 7+ 等）。

### 安装步骤

1. **复制服务文件**：
```bash
sudo cp deploy/clibot.service /etc/systemd/system/clibot.service
```

**自定义路径**（可选）：
如果使用不同的配置位置或二进制路径，编辑服务文件：
```bash
sudo nano /etc/systemd/system/clibot.service
```

需要自定义的关键行：
- `ExecStart=/usr/local/bin/clibot serve --config /etc/clibot/config.yaml`
  - 更改二进制路径（如果安装在其他位置）
  - 更改 `--config` 路径到您的配置文件位置
- `User=clibot` 和 `Group=clibot`
  - 如果使用不同用户则更改
- `WorkingDirectory=/opt/clibot`
  - 更改到您首选的工作目录

2. **重新加载 systemd**：
```bash
sudo systemctl daemon-reload
```

3. **启用 clibot** 开机自启：
```bash
sudo systemctl enable clibot
```

4. **启动 clibot**：
```bash
sudo systemctl start clibot
```

### 管理命令

```bash
# 查看状态
sudo systemctl status clibot

# 停止 clibot
sudo systemctl stop clibot

# 重启 clibot
sudo systemctl restart clibot

# 查看日志
sudo journalctl -u clibot -f

# 查看自上次启动以来的日志
sudo journalctl -u clibot -b

# 查看最近 100 行
sudo journalctl -u clibot -n 100
```

### 日志轮转

systemd 通过 journald 自动处理日志轮转。配置持久化日志：

```bash
# 创建 journal 目录
sudo mkdir -p /var/log/journal

# 重启 journald
sudo systemctl restart systemd-journald
```

## 使用 Supervisor 部署

Supervisor 是一个类 Unix 操作系统的进程控制系统。

### 安装步骤

1. **安装 supervisor**：
```bash
# Ubuntu/Debian
sudo apt-get install supervisor

# Fedora/CentOS/RHEL
sudo dnf install supervisor

# macOS
brew install supervisor
```

2. **复制配置文件**：
```bash
sudo cp deploy/clibot.conf /etc/supervisor/conf.d/clibot.conf
```

**自定义路径**（可选）：
如果使用不同的配置位置或二进制路径，编辑配置文件：
```bash
sudo nano /etc/supervisor/conf.d/clibot.conf
```

需要自定义的关键行：
- `command=/usr/local/bin/clibot serve --config /etc/clibot/config.yaml`
  - 更改二进制路径（如果安装在其他位置）
  - 更改 `--config` 路径到您的配置文件位置
- `user=clibot`
  - 如果使用不同用户则更改
- `stdout_logfile=/var/log/clibot/clibot.log`
  - 更改到您首选的日志位置

3. **重新读取并更新 supervisor**：
```bash
sudo supervisorctl reread
sudo supervisorctl update
```

4. **启动 clibot**：
```bash
sudo supervisorctl start clibot
```

### 管理命令

```bash
# 查看状态
sudo supervisorctl status clibot

# 停止 clibot
sudo supervisorctl stop clibot

# 重启 clibot
sudo supervisorctl restart clibot

# 查看日志
sudo supervisorctl tail -f clibot

# 查看 stderr 日志
sudo supervisorctl tail -f clibot stderr

# 查看 stdout 日志
sudo supervisorctl tail -f clibot stdout
```

### 日志轮转

Supervisor 根据 `clibot.conf` 中的设置自动处理日志轮转：
- `stdout_logfile_maxbytes=50MB` - 达到 50MB 时轮转
- `stdout_logfile_backups=10` - 保留 10 个备份文件

## 验证部署

### 验证 clibot 是否运行

```bash
# 检查 tmux 会话是否存在
sudo -u clibot tmux list-sessions

# 检查 clibot 是否监听 8080 端口
sudo netstat -tlnp | grep 8080

# 或使用 ss
sudo ss -tlnp | grep 8080
```

### 从 IM 测试

向您的 Bot 发送消息：
```
status
```

您应该会收到状态响应。

## 故障排查

### clibot 无法启动

1. **检查服务状态**：
```bash
# systemd
sudo systemctl status clibot

# supervisor
sudo supervisorctl status clibot
```

2. **检查日志**：
```bash
# systemd
sudo journalctl -u clibot -n 100

# supervisor
sudo tail -100 /var/log/clibot/clibot.log
```

3. **常见问题**：

   **问题**：权限被拒绝
   **解决方案**：
   ```bash
   sudo chown -R clibot:clibot /etc/clibot
   sudo chown -R clibot:clibot /var/log/clibot
   ```

   **问题**：找不到配置文件
   **解决方案**：确保 `/etc/clibot/config.yaml` 存在且可读

   **问题**：端口已被占用
   **解决方案**：
   ```bash
   # 查找占用 8080 端口的进程
   sudo lsof -i :8080
   # 在 config.yaml 中更改端口
   ```

   **问题**：找不到 tmux
   **解决方案**：
   ```bash
   # 安装 tmux
   sudo apt-get install tmux  # Ubuntu/Debian
   ```

### 手动测试

手动运行 clibot 以调试问题：

```bash
# 以 clibot 用户身份运行
sudo -u clibot /usr/local/bin/clibot serve --config /etc/clibot/config.yaml

# 或使用 debug 日志级别运行
sudo -u clibot /usr/local/bin/clibot serve --config /etc/clibot/config.yaml --log-level debug
```

## 卸载

### systemd

```bash
# 停止并禁用
sudo systemctl stop clibot
sudo systemctl disable clibot

# 删除服务文件
sudo rm /etc/systemd/system/clibot.service
sudo systemctl daemon-reload

# 删除用户和目录（可选）
sudo userdel clibot
sudo rm -rf /etc/clibot
sudo rm -rf /var/log/clibot
```

### Supervisor

```bash
# 停止 clibot
sudo supervisorctl stop clibot

# 删除配置
sudo rm /etc/supervisor/conf.d/clibot.conf

# 重新读取并更新
sudo supervisorctl reread
sudo supervisorctl update

# 删除用户和目录（可选）
sudo userdel clibot
sudo rm -rf /etc/clibot
sudo rm -rf /var/log/clibot
```

## 生产环境建议

### 安全性

1. **使用专用用户**（如上所示）
2. **启用白名单** 在 config.yaml 中：
```yaml
security:
  whitelist_enabled: true
  allowed_users:
    discord:
      - "your-user-id"
```

3. **使用环境变量存储密钥**：
```yaml
bots:
  discord:
    token: "${DISCORD_TOKEN}"
```

设置方式：
```bash
# 对于 systemd
sudo nano /etc/systemd/system/clibot.service
# 添加: Environment=DISCORD_TOKEN=your_token

# 对于 supervisor
sudo nano /etc/supervisor/conf.d/clibot.conf
# 添加: environment=DISCORD_TOKEN="your_token"
```

### 性能

1. **限制动态会话数**：
```yaml
session:
  max_dynamic_sessions: 50
```

2. **调整日志级别** 用于生产环境：
```yaml
logging:
  level: info  # 或 warn
  enable_stdout: false
```

3. **监控资源**：
```bash
# 检查内存使用
ps aux | grep clibot

# 检查 tmux 会话
sudo -u clibot tmux list-sessions
```

### 备份

备份您的配置：
```bash
sudo cp /etc/clibot/config.yaml /etc/clibot/config.yaml.backup
```

## 其他资源

- [README.md](../README_zh.md) - 主文档
- [SECURITY.md](../SECURITY.md) - 安全最佳实践
- [配置指南](../README_zh.md#配置) - 配置文件参考
