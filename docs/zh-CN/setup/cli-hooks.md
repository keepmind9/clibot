# CLI Hook 配置指南

本文档提供了如何为各种 AI CLI 工具配置 Hook 的详细说明，以实现 clibot 的实时响应（Hook 模式）。

## Claude Code

要实现 Claude Code 的实时响应，请在 `~/.claude/settings.json` 中添加以下配置：

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

要实现 Gemini CLI 的实时响应，请在 `~/.gemini/settings.json` 中添加以下配置：

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

OpenCode 通过其插件系统支持实时通知。在项目的 `.opencode/plugin/` 目录（或全局目录 `~/.config/opencode/plugin/`）中创建一个名为 `clibot.ts` 的文件：

```typescript
import { Plugin } from "@opencode-ai/plugin";

export const ClibotPlugin: Plugin = async (ctx) => {
  return {
    event: async ({ event }) => {
      // 当会话进入空闲状态（任务完成）时触发
      if (event.type === "session.idle") {
        const { sessionID } = event.properties;
        const cwd = ctx.directory;

        const payload = JSON.stringify({
          cwd,
          session_id: sessionID,
          hook_event_name: "Completed"
        });

        // 确保 clibot 在您的 PATH 环境变量中
        await ctx.$`echo ${payload} | clibot hook --cli-type opencode`.quiet();
      }
    },
  };
};
```

