---
name: clibot
description: Bridge AI CLI tools (Claude Code, Gemini CLI, OpenCode, etc.) to IM platforms (Telegram, Discord, Feishu/Lark). Use when user wants to install, configure, or manage clibot service. Handles binary download, configuration generation, bot token setup, and service lifecycle. Ideal for users who want to interact with AI CLI tools through chat platforms.
---

# Clibot Skill

## Overview

Clibot is a lightweight middleware that connects AI CLI tools (Claude Code, Gemini CLI, OpenCode, etc.) to IM platforms (Telegram, Discord, Feishu/Lark). This skill automates: binary installation, bot configuration, session setup, permission management, and service lifecycle.

Uses ACP mode (Claude Agent SDK via stdio) — no tmux required.

## Quick Start

```
/clibot setup
```

Wizard auto-detects OS/arch, downloads the correct binary if needed, then guides through:
1. Platform selection (Telegram/Discord/Feishu)
2. Bot token collection with inline instructions
3. Session configuration (defaults work for most users)
4. Permission setup (admins + optional whitelist)
5. Auto-starts the service when done

## Workflow Decision Tree

```
User request
    ↓
Is clibot installed?
    ├─ No → Auto-downloads binary first, then config wizard
    └─ Yes → Config wizard
            ↓
Has config.yaml?
    ├─ No → Generate config
    └─ Yes → Ask to overwrite or use existing
            ↓
Config valid? (auto-validated by /clibot validate)
    ├─ No → Show errors, guide fix
    └─ Yes → Start service
            ↓
Service running?
    ├─ No → /clibot start
    └─ Yes → /clibot status
```

## Commands

| Command | Description |
|---------|-------------|
| `/clibot setup` | Full wizard (installs binary if missing, generates config, starts service) |
| `/clibot install` | Download binary only |
| `/clibot config` | Generate/edit config.yaml |
| `/clibot validate` | Check config for errors |
| `/clibot start` | Start clibot daemon |
| `/clibot stop` | Stop clibot daemon |
| `/clibot restart` | Stop + start |
| `/clibot status` | Show PID, uptime, recent logs |
| `/clibot logs` | View logs (`-f` to follow) |

## Key Defaults

- **Install path**: `~/.local/bin/clibot` (no sudo required)
- **Config path**: `~/.clibot/config.yaml`
- **Log path**: `~/.clibot/logs/clibot.log`
- **CLI adapter**: `acp` (Claude Agent SDK, no tmux needed)
- **Session name**: `claude`, command: `claude`

## References

- `references/quickstart.md` — Common configuration examples and workflows
- `references/bot-setup.md` — Step-by-step bot creation guides (Telegram/Discord/Feishu)
- `references/troubleshooting.md` — Error diagnoses and solutions

## Permissions

This skill may invoke external tools (`npm`, `npm install`, `npm link`) during installation. Network access is required to download binaries from GitHub releases.
