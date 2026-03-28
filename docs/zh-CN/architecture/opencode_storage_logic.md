# OpenCode 存储解析逻辑

本文档总结了 OpenCode 适配器如何从本地存储中提取最新的用户输入和模型回复。

## 1. 存储机制

OpenCode 使用 **SQLite + 文件系统** 混合存储，默认路径为：
`~/.local/share/opencode/`

| 存储方式 | 路径 | 说明 |
|----------|------|------|
| SQLite 数据库 | `opencode.db` | 主存储，包含 session/message/part 三张表 |
| 文件存储 (旧) | `storage/` | 早期版本的文件存储，仅包含旧会话 |

适配器采用 **SQLite 优先、文件存储降级** 的策略。

### SQLite 数据库表结构

- **session**: `id`, `project_id`, `time_created`, `time_updated`, `data` (JSON)
- **message**: `id`, `session_id`, `time_created`, `time_updated`, `data` (JSON，含 `role` 字段)
- **part**: `id`, `message_id`, `session_id`, `time_created`, `time_updated`, `data` (JSON，含 `type` 和 `text` 字段)

### 文件存储结构 (旧版)

- **会话信息**: `storage/session/{projectID}/{sessionID}.json`
- **消息元数据**: `storage/message/{sessionID}/{messageID}.json` (含 `id`、`role`，不含 parts)
- **消息内容**: `storage/part/{messageID}/{partID}.json` (每个 part 独立文件)

## 2. 定位会话 (Session)

为了提取正确的对话，适配器首先需要定位到具体的会话：

1.  **项目识别 (ProjectID)**:
    *   如果是 Git 仓库，OpenCode 使用仓库的**第一个 commit hash** 作为 `projectID`。
    *   如果不是 Git 仓库，则使用 `global`。
2.  **自动识别最新会话**:
    *   SQLite: 查询 `session` 表按 `time_updated` 降序取第一条。
    *   文件存储: 扫描 `storage/session/{projectID}/` 目录，比较 `time.updated` 字段。

## 3. 提取逻辑 (`extractLatestInteractionFromStorage`)

适配器依次尝试两种数据源：

### 优先：SQLite 提取 (`extractLatestInteractionFromDB`)

1.  以只读模式打开 `opencode.db`。
2.  查询 `message` 表：`SELECT id, json_extract(data, '$.role') FROM message WHERE session_id = ? ORDER BY time_created`。
3.  从末尾向前查找最后一个 `role = "user"` 的消息。
4.  对该用户消息及后续的 `role = "assistant"` 消息，分别查询 `part` 表：`SELECT json_extract(data, '$.text') FROM part WHERE message_id = ? AND json_extract(data, '$.type') = 'text'`。
5.  用双换行符拼接所有 assistant 的文本 part 作为回复。

### 降级：文件存储提取 (`extractLatestInteractionFromFiles`)

当 SQLite 不可用（如数据库文件不存在）时降级为文件存储：

1.  读取 `storage/message/{sessionID}/*.json` 获取消息列表（按文件名排序）。
2.  定位最后一个 `role = "user"` 的消息。
3.  对每条消息，从 `storage/part/{messageID}/*.json` 读取 parts，提取 `type = "text"` 的内容。
4.  拼接 assistant 消息的文本 parts 作为回复。

## 4. 关键特性

*   **SQLite 优先**: 新版 OpenCode 会话仅存储在 SQLite 中，适配器优先查询数据库确保覆盖最新会话。
*   **文件存储降级**: 兼容早期仅使用文件存储的会话，确保旧数据仍可解析。
*   **Parts 独立存储**: 消息内容（text、reasoning、step-start 等）以独立 part 文件/记录存储，适配器仅提取 `type = "text"` 的内容，过滤掉思考过程和工具调用。
*   **ProjectID 隔离**: 严格遵循 OpenCode 的项目计算逻辑（首个 commit hash 排序取最小），确保在不同工程间切换时的准确性。
