# CLI Hook Configuration

This document provides detailed instructions on how to configure hooks for various AI CLI tools to enable real-time notifications in clibot (Hook Mode).

## Claude Code

To enable real-time responses with Claude Code, add the following to your `~/.claude/settings.json`:

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

## Gemini CLI

To enable real-time responses with Gemini CLI, add the following to your `~/.gemini/settings.json`:

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

## OpenCode

OpenCode supports real-time notifications via its plugin system. Create a file named `clibot.ts` in your project's `.opencode/plugin/` directory (or globally in `~/.config/opencode/plugin/`):

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

