# OpenCode 存储解析逻辑

本文档总结了 OpenCode 适配器如何从本地存储中提取最新的用户输入和模型回复。

## 1. 存储机制

OpenCode 使用基于文件的存储系统，默认路径为：
`~/.local/share/opencode/storage/`

数据结构如下：
*   **会话信息**: `session/{projectID}/{sessionID}.json`
*   **消息内容**: `message/{sessionID}/msg_{messageID}.json` (每个消息一个独立文件)

## 2. 定位会话 (Session)

为了提取正确的对话，适配器首先需要定位到具体的会话：

1.  **项目识别 (ProjectID)**: 
    *   如果是 Git 仓库，OpenCode 使用仓库的**第一个 commit hash** 作为 `projectID`。
    *   如果不是 Git 仓库，则使用 `global`。
2.  **自动识别最新会话**:
    *   如果 Hook 未提供 `session_id`，适配器会扫描 `storage/session/{projectID}/` 目录下所有的 JSON 文件。
    *   解析这些文件并比较 `time.updated` 字段，选择更新时间最晚的会话。

## 3. 提取逻辑 (`extractLatestInteractionFromStorage`)

一旦确定了 `sessionID`，提取过程如下：

### 第一步：收集消息
1.  进入 `storage/message/{sessionID}/` 目录。
2.  获取所有 `.json` 文件（文件名格式为 `msg_{顺序ID}.json`）。
3.  按**文件名升序排序**，以确保消息的时间顺序正确。
4.  逐个读取文件并反序列化为 `OpenCodeMessageInfo` 结构。

### 第二步：定位最新用户输入
1.  从消息列表末尾向前遍历。
2.  查找第一个 `role` 为 `"user"` 的消息。
3.  提取该消息中所有类型为 `"text"` 的 `parts` 内容，合并为 `lastUserPrompt`。

### 第三步：收集助手回复
1.  从找到的用户输入索引位置开始向后遍历。
2.  收集所有 `role` 为 `"assistant"` 的后续消息。
3.  提取每条助手消息中所有类型为 `"text"` 的 `parts` 内容。
4.  使用双换行符 (`

`) 将这些内容块拼接为最终的回复。

## 4. 关键特性

*   **分布式存储支持**: 与 Claude 的单文件 JSONL 不同，OpenCode 每个消息都是独立文件，适配器通过文件名排序保证了逻辑一致性。
*   **多 Part 支持**: 能够解析并提取消息中嵌套的 `parts` 数组，过滤掉思考过程 (`reasoning`) 或工具调用 (`tool-invocation`) 等非文本内容。
*   ** projectID 隔离**: 严格遵循 OpenCode 的项目计算逻辑，确保在不同工程间切换时的准确性。
