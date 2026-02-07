# clibot 管理脚本使用说明

## 简介

`clibot.sh` 是一个用于开发和测试环境的 bash 管理脚本，可以方便地启动、停止、重启和查看 clibot 服务状态。

**⚠️ 重要提示**：
- 此脚本仅适用于**开发和测试环境**
- **生产环境**请使用 systemd 或 supervisor
- 详见：[部署指南](./DEPLOYMENT_zh.md)

## 快速开始

### 1. 赋予执行权限

```bash
chmod +x deploy/clibot.sh
```

### 2. 基本使用

```bash
# 启动 clibot
./deploy/clibot.sh start

# 查看状态
./deploy/clibot.sh status

# 查看日志
./deploy/clibot.sh logs

# 停止 clibot
./deploy/clibot.sh stop

# 重启 clibot
./deploy/clibot.sh restart
```

## 命令说明

| 命令 | 说明 |
|------|------|
| `start` | 启动 clibot 服务（后台运行） |
| `stop` | 停止 clibot 服务 |
| `restart` | 重启 clibot 服务 |
| `status` | 显示服务状态和进程信息 |
| `logs` | 实时查看日志（类似 tail -f） |
| `help` | 显示帮助信息 |

## 自定义配置

### 环境变量

可以通过环境变量自定义路径：

```bash
# 使用自定义配置文件
CONFIG_FILE=/etc/clibot/config.yaml ./deploy/clibot.sh start

# 使用自定义二进制文件
CLIBOT_BIN=/usr/local/bin/clibot ./deploy/clibot.sh start

# 使用自定义 PID 文件
PID_FILE=/var/run/clibot.pid ./deploy/clibot.sh start

# 使用自定义日志文件
LOG_FILE=/var/log/clibot/clibot.log ./deploy/clibot.sh start
```

### 默认路径

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `CLIBOT_BIN` | `clibot` | 二进制文件路径 |
| `CONFIG_FILE` | `~/.config/clibot/config.yaml` | 配置文件路径 |
| `PID_FILE` | `/tmp/clibot.pid` | PID 文件路径 |
| `LOG_FILE` | `/tmp/clibot.log` | 日志文件路径 |

## 使用场景

### 适合场景

✅ **本地开发** - 快速启停服务进行测试
✅ **功能验证** - 测试新配置或功能
✅ **调试模式** - 配合 IDE 或编辑器使用
✅ **临时测试** - 不需要配置系统服务

### 不适合场景

❌ **生产环境** - 请使用 systemd 或 supervisor
❌ **长期运行** - 缺少自动重启和监控
❌ **多实例管理** - 建议使用 supervisor

## 常见问题

### 1. 找不到 clibot 二进制

**错误信息**：
```
[ERROR] clibot binary not found: clibot
```

**解决方案**：
```bash
# 安装 clibot
go install github.com/keepmind9/clibot@latest

# 或设置自定义路径
CLIBOT_BIN=/path/to/clibot ./deploy/clibot.sh start
```

### 2. 找不到配置文件

**错误信息**：
```
[ERROR] Config file not found: ~/.config/clibot/config.yaml
```

**解决方案**：
```bash
# 创建配置目录
mkdir -p ~/.config/clibot

# 复制配置模板
cp configs/config.yaml ~/.config/clibot/config.yaml

# 编辑配置文件
nano ~/.config/clibot/config.yaml
```

### 3. 端口已被占用

**错误信息**：
```
Failed to bind to port 8080
```

**解决方案**：
```bash
# 查找占用端口的进程
lsof -i :8080

# 或在配置文件中更改端口
nano ~/.config/clibot/config.yaml
```

### 4. 查看帮助信息

```bash
./deploy/clibot.sh help
```

## 进阶用法

### 创建快捷命令

```bash
# 添加到 PATH
sudo ln -s $(pwd)/deploy/clibot.sh /usr/local/bin/clibot-ctl

# 使用快捷命令
clibot-ctl start
clibot-ctl status
clibot-ctl logs
```

### 配置 alias

在 `~/.bashrc` 或 `~/.zshrc` 中添加：

```bash
# clibot 管理快捷命令
alias clibot-start='~/path/to/deploy/clibot.sh start'
alias clibot-stop='~/path/to/deploy/clibot.sh stop'
alias clibot-restart='~/path/to/deploy/clibot.sh restart'
alias clibot-status='~/path/to/deploy/clibot.sh status'
alias clibot-logs='~/path/to/deploy/clibot.sh logs'
```

然后执行：

```bash
source ~/.bashrc  # 或 source ~/.zshrc

# 使用快捷命令
clibot-start
clibot-status
```

## 与 systemd/supervisor 对比

| 特性 | clibot.sh | systemd | supervisor |
|------|-----------|---------|------------|
| 易用性 | ⭐⭐⭐⭐⭐ | ⭐⭐⭐ | ⭐⭐⭐⭐ |
| 自动重启 | ❌ | ✅ | ✅ |
| 开机自启 | ❌ | ✅ | ✅ |
| 日志管理 | 基础 | 完善 | 完善 |
| 进程监控 | 无 | 完善 | 完善 |
| 适用场景 | 开发测试 | 生产环境 | 生产环境 |

## 相关文档

- [部署指南](./DEPLOYMENT_zh.md) - 完整的部署说明
- [systemd 部署](./DEPLOYMENT_zh.md#使用-systemd-部署) - 生产环境推荐
- [supervisor 部署](./DEPLOYMENT_zh.md#使用-supervisor-部署) - 跨平台方案
- [README](../README_zh.md) - 项目主文档
