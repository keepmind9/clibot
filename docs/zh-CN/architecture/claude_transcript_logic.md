# Claude Transcript 解析逻辑

本文档描述了如何根据 `transcript_path` 获取最新的用户输入和模型回复。

## 1. 核心概念

*   **`transcript_path`**: 主对话记录文件，格式为 JSONL。
*   **`subagents` 目录**: 位于与 `transcript_path` 同级的目录中，存储子 Agent 的执行记录。
*   **`sessionId`**: 用于关联用户输入和助手回复的会话标识符。

## 2. 获取最新用户输入 (`extractLastUserPrompt`)

目标是从 `transcript_path` 文件中找到最后一个“真实”的用户输入。

1.  **解析 `transcript_path` 文件**: 读取所有行并解析为 `TranscriptMessage` 结构。
2.  **倒序遍历消息**: 从最后一条消息开始向前查找。
3.  **判断条件**:
    *   消息类型 (`type`) 必须为 `"user"`。
    *   **排除元数据**: `isMeta` 字段必须为 `false`（或不存在）。
    *   **排除内部命令**: 消息内容不能以 `<local-command-` 或 `<command-name>` 开头。这些通常是 CLI 内部执行命令的记录，而非用户显式输入。
4.  **提取内容**:
    *   如果找到满足条件的消息，提取其文本内容 (`message.content` 或 `message.content[].text`)。
    *   同时记录该消息的 `sessionId`。
5.  **返回**: 用户输入内容和 `sessionId`。

## 3. 获取模型回复 (`extractFromTranscriptFile`)

目标是获取针对上述用户输入的最新模型回复。回复可能存在于主文件或子 Agent 文件中。

### 步骤 1: 确定上下文

1.  首先执行“获取最新用户输入”的逻辑，找到最后一个真实用户的 `lastUserIndex` 和对应的 `sessionId`。
2.  如果没有找到用户输入，则无法提取回复。

### 步骤 2: 尝试从子 Agent 获取回复 (Case 2)

某些情况下，复杂的任务由子 Agent 执行，回复记录在 `subagents` 目录下。

1.  **定位 `subagents` 目录**: 路径通常为 `transcript_path` 去掉扩展名后的同名目录下的 `subagents` 文件夹。
2.  **查找最新文件**: 在 `subagents` 目录下查找修改时间最新的 `.jsonl` 文件。
3.  **解析子 Agent 文件**: 读取该文件内容。
4.  **提取回复**:
    *   遍历文件中的消息。
    *   查找类型 (`type`) 为 `"assistant"` 的消息。
    *   **关键过滤**: 消息的 `sessionId` 必须为空或者与用户输入的 `sessionId` 一致。
    *   收集所有文本内容块 (`content[].text`)。
5.  **成功判定**: 如果提取到了文本内容，则将其作为最终回复返回。

### 步骤 3: 从主文件获取回复 (Case 1)

如果子 Agent 中没有找到回复，则回退到主 `transcript_path` 文件。

1.  **遍历消息**: 从 `lastUserIndex` 的下一条消息开始，直到文件末尾。
2.  **提取回复**:
    *   查找类型 (`type`) 为 `"assistant"` 的消息。
    *   **关键过滤**: 消息的 `sessionId` 必须为空或者与用户输入的 `sessionId` 一致。
    *   收集所有文本内容块 (`content[].text`)。
3.  **返回**: 将收集到的所有文本块拼接（通常用换行符）作为最终回复。

## 4. 总结

*   优先从子 Agent 记录中查找回复，因为这通常包含了具体的执行结果。
*   严格使用 `sessionId` 匹配，确保回复是针对当前用户输入的。
*   过滤掉系统生成的元数据和内部命令记录，确保获取的是用户的真实意图。
