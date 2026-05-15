# 安装指南

本指南涵盖 clibot 的系统要求和安装说明。

## 目录

- [快速安装](#快速安装)
- [系统要求](#系统要求)
- [手动下载](#手动下载)
- [从源码构建](#从源码构建)
- [自更新](#自更新)

## 快速安装

最快的安装方式：

**Linux / macOS:**
```bash
curl -sL https://raw.githubusercontent.com/keepmind9/clibot/main/scripts/install.sh | bash
```

**Windows (PowerShell):**
```powershell
irm https://raw.githubusercontent.com/keepmind9/clibot/main/scripts/install.ps1 | iex
```

脚本会自动：
- 检测操作系统和架构
- 从 GitHub 下载最新版本
- 安装到 `~/.local/bin/clibot`
- 如有需要则添加到 PATH

验证安装：
```bash
clibot version
```

## 系统要求

### 支持的平台

| 平台 | 状态 | 说明 |
|----------|--------|-------|
| **Linux** | ✅ 完全支持 | 所有模式原生运行 |
| **macOS** | ✅ 完全支持 | 所有模式原生运行 |
| **Windows** | ✅ ACP/Stdio 模式 | 原生二进制，ACP 和 Stdio 模式无需 WSL |

### 模式要求

| 模式 | 要求 | 说明 |
|------|------|------|
| **ACP 模式** ⭐ | 无 | 推荐，流式响应 |
| **Stdio 模式** | 无 | 零配置，per-turn CLI |
| **Hook 模式** | tmux | Windows 原生不支持 |

## 手动下载

从 [GitHub Releases](https://github.com/keepmind9/clibot/releases/latest) 下载对应平台的二进制文件。

**Linux (AMD64):**
```bash
curl -LO https://github.com/keepmind9/clibot/releases/latest/download/clibot-linux-amd64
chmod +x clibot-linux-amd64
mkdir -p ~/.local/bin
mv clibot-linux-amd64 ~/.local/bin/clibot
```

**Linux (ARM64):**
```bash
curl -LO https://github.com/keepmind9/clibot/releases/latest/download/clibot-linux-arm64
chmod +x clibot-linux-arm64
mkdir -p ~/.local/bin
mv clibot-linux-arm64 ~/.local/bin/clibot
```

**macOS (Apple Silicon):**
```bash
curl -LO https://github.com/keepmind9/clibot/releases/latest/download/clibot-darwin-arm64
chmod +x clibot-darwin-arm64
mkdir -p ~/.local/bin
mv clibot-darwin-arm64 ~/.local/bin/clibot
```

**macOS (Intel):**
```bash
curl -LO https://github.com/keepmind9/clibot/releases/latest/download/clibot-darwin-amd64
chmod +x clibot-darwin-amd64
mkdir -p ~/.local/bin
mv clibot-darwin-amd64 ~/.local/bin/clibot
```

**Windows (AMD64):**
```powershell
Invoke-WebRequest -Uri "https://github.com/keepmind9/clibot/releases/latest/download/clibot-windows-amd64.exe" -OutFile "clibot.exe"
```

将 `~/.local/bin` 添加到 PATH：
```bash
export PATH="$HOME/.local/bin:$PATH"
```

## 从源码构建

需要 **Go 1.24+**。

```bash
go install github.com/keepmind9/clibot@latest
```

或从仓库构建：
```bash
git clone https://github.com/keepmind9/clibot.git
cd clibot
make build
sudo make install
```

## 自更新

clibot 支持自更新：

```bash
# 检查并下载最新版本
clibot update

# 应用已下载的更新（替换二进制）
clibot update --apply
```

特性：
- 支持断点续传
- 自动二进制替换（Unix: rename-old 方式，Windows: 延迟替换）

## 后续步骤

安装完成后：

1. **配置 clibot**：
   ```bash
   mkdir -p ~/.config/clibot
   cp configs/config.mini.yaml ~/.config/clibot/config.yaml
   nano ~/.config/clibot/config.yaml
   ```

2. **选择运行模式**：
   - **ACP 模式**（推荐）：无需 tmux
   - **Stdio 模式**：零配置，无需 tmux
   - **Hook 模式**：需要 tmux + CLI hook 配置

3. **启动 clibot**：
   ```bash
   clibot serve --config ~/.config/clibot/config.yaml
   ```

详细配置和使用说明，请参阅 [README_zh.md](README_zh.md)。
