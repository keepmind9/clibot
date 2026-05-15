# Installation Guide

This guide covers system requirements and installation instructions for clibot.

## Table of Contents

- [Quick Install](#quick-install)
- [System Requirements](#system-requirements)
- [Install from Source](#install-from-source)
- [Self-Update](#self-update)

## Quick Install

The fastest way to install clibot:

**Linux / macOS:**
```bash
curl -sL https://raw.githubusercontent.com/keepmind9/clibot/main/scripts/install.sh | bash
```

**Windows (PowerShell):**
```powershell
irm https://raw.githubusercontent.com/keepmind9/clibot/main/scripts/install.ps1 | iex
```

The script will:
- Detect your OS and architecture
- Download the latest release from GitHub
- Install to `~/.local/bin/clibot`
- Add to PATH if needed

Verify the installation:
```bash
clibot version
```

## System Requirements

### Supported Platforms

| Platform | Status | Notes |
|----------|--------|-------|
| **Linux** | ✅ Fully Supported | All modes work natively |
| **macOS** | ✅ Fully Supported | All modes work natively |
| **Windows** | ✅ ACP/Stdio Mode | Native binary, ACP and Stdio modes work without WSL |

### Mode Requirements

| Mode | Requirements | Notes |
|------|-------------|-------|
| **ACP Mode** ⭐ | None | Recommended, streaming responses |
| **Stdio Mode** | None | Zero config, per-turn CLIs |
| **Hook Mode** | tmux | Not available on Windows native |

## Install from Source

Requires **Go 1.24+**.

```bash
go install github.com/keepmind9/clibot@latest
```

Or build from the repository:
```bash
git clone https://github.com/keepmind9/clibot.git
cd clibot
make build
sudo make install
```

## Self-Update

clibot can update itself:

```bash
# Check and download the latest version
clibot update

# Apply the downloaded update (replace binary)
clibot update --apply
```

Features:
- Resume support for interrupted downloads
- Automatic binary replacement (Unix: rename-old trick, Windows: delayed swap)

## Next Steps

After installation:

1. **Configure clibot**:
   ```bash
   mkdir -p ~/.config/clibot
   cp configs/config.mini.yaml ~/.config/clibot/config.yaml
   nano ~/.config/clibot/config.yaml
   ```

2. **Choose your mode**:
   - **ACP Mode** (Recommended): No tmux required
   - **Stdio Mode**: Zero config, no tmux required
   - **Hook Mode**: Requires tmux + CLI hook configuration

3. **Start clibot**:
   ```bash
   clibot serve --config ~/.config/clibot/config.yaml
   ```

For detailed configuration and usage, see [README.md](README.md).
