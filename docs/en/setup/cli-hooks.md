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
