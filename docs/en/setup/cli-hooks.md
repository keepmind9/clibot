# CLI Hook Configuration

This document provides detailed instructions on how to configure hooks for various AI CLI tools to enable real-time notifications in clibot (Hook Mode).

## Configuration Location

To maintain project independence and avoid invading user directories, **project-level configuration is recommended**:

| Configuration Location | Scope | Recommended For |
|------------------------|-------|-----------------|
| Project root `.claude/settings.json` | Current project | ✅ **Recommended** - Team collaboration, version control |
| User directory `~/.claude/settings.json` | Global | Personal development, multi-project sharing |

## Claude Code

Claude Code supports two types of project-level configuration files. Choose based on your scenario:

### Method 1: Personal Configuration (Recommended)

Create `.claude/settings.local.json` in your project root:

**Use cases**:
- ✅ Personal development environment, non-shared hook configuration
- ✅ Each developer has different clibot service addresses
- ✅ `.gitignore` already ignores `settings.local.json` (Claude Code default)

```json
{
  "hooks": {
    "Stop": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "clibot hook --cli-type claude"
          }
        ]
      }
    ],
    "Notification": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "clibot hook --cli-type claude"
          }
        ]
      }
    ]
  }
}
```

### Method 2: Team-shared Configuration (Optional)

If all team members use the same clibot configuration, create `.claude/settings.json` in your project root:

**Caution**:
- ⚠️ This file will be committed to Git
- ⚠️ Ensure all team members' clibot configurations are compatible

### Configuration File Priority

Claude Code loads configurations in the following order (later loaded overrides earlier):

1. `~/.claude/settings.json` (global configuration)
2. Project root `.claude/settings.json` (team configuration)
3. Project root `.claude/settings.local.json` (personal configuration)

**Recommendation**: Put hook configuration in `settings.local.json` to avoid affecting team members.

## Gemini CLI

### Project-level Configuration (Recommended)

Gemini CLI supports project-level configuration. Create `.gemini/settings.json` in your project root:

```json
{
  "tools": {
    "enableHooks": true
  },
  "hooks": {
    "AfterAgent": [
      {
        "hooks": [
          {
            "name": "clibot-post-command-hook",
            "type": "command",
            "command": "clibot hook --cli-type gemini",
            "description": "post command hook for clibot"
          }
        ]
      }
    ],
    "Notification": [
      {
        "hooks": [
          {
            "name": "clibot-notification-command-hook",
            "type": "command",
            "command": "clibot hook --cli-type gemini",
            "description": "notification command hook for clibot"
          }
        ]
      }
    ]
  }
}
```

### Global Configuration (Optional)

To enable across all projects, add the same configuration to `~/.gemini/settings.json`.

## OpenCode

OpenCode natively supports project-level plugins. Create a file named `clibot.ts` in your project's `.opencode/plugin/` directory:

```typescript
import { Plugin } from "@opencode-ai/plugin";

export const ClibotPlugin: Plugin = async (ctx) => {
  return {
    event: async ({ event }) => {
      // Trigger when the session becomes idle (task finished)
      if (event.type === "session.idle") {
        const { sessionID } = event.properties;
        const cwd = ctx.directory;

        const payload = JSON.stringify({
          cwd,
          session_id: sessionID,
          hook_event_name: "Completed"
        });

        // Ensure clibot is in your PATH
        await ctx.$`echo ${payload} | clibot hook --cli-type opencode`.quiet();
      }
    },
  };
};
```

