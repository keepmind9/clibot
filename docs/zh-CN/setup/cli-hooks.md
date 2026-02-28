# CLI Hook 配置指南

本文档提供了如何为各种 AI CLI 工具配置 Hook 的详细说明，以实现 clibot 的实时响应（Hook 模式）。

## 配置位置说明

为了保持项目独立性和避免侵入用户目录，建议优先使用**项目级配置**：

| 配置位置 | 范围 | 推荐场景 |
|---------|------|---------|
| 项目根目录 `.claude/settings.json` | 当前项目 | ✅ **推荐** - 团队协作，版本控制 |
| 用户目录 `~/.claude/settings.json` | 全局 | 个人开发，多项目共享 |

## Claude Code

Claude Code 支持两种项目级配置文件，建议根据场景选择：

### 方式一：个人配置（推荐）

在项目根目录创建 `.claude/settings.local.json`：

**适用场景**：
- ✅ 个人开发环境，hook 配置不共享
- ✅ 每个开发者 clibot 服务地址不同
- ✅ `.gitignore` 已忽略 `settings.local.json`（Claude Code 默认配置）

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

### 方式二：团队共享配置（可选）

如果团队所有成员都使用相同的 clibot 配置，在项目根目录创建 `.claude/settings.json`：

**注意**：
- ⚠️ 此文件会被提交到 Git
- ⚠️ 确保所有团队成员的 clibot 配置兼容

### 配置文件优先级

Claude Code 按以下顺序加载配置（后加载的覆盖先加载的）：

1. `~/.claude/settings.json`（全局配置）
2. 项目根目录 `.claude/settings.json`（团队配置）
3. 项目根目录 `.claude/settings.local.json`（个人配置）

**建议**：将 hook 配置放在 `settings.local.json` 中，避免影响团队成员。

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

### 项目级配置（推荐）

Gemini CLI 支持项目级配置，在项目根目录创建 `.gemini/settings.json`：

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

### 全局配置（可选）

如需在所有项目中启用，在 `~/.gemini/settings.json` 中添加相同配置。

## OpenCode

OpenCode 原生支持项目级插件系统，在项目的 `.opencode/plugin/` 目录中创建 `clibot.ts`：

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

