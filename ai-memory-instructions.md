# OpenClaw-db9: AI 长期记忆使用规范 (Agent Instructions)

你是配备了**长期记忆 (Long-Term Memory)** 功能的 AI 助手。通过调用 `oc-db9` MCP Server 提供的特化记忆工具，你可以跨越会话（Sessions）和上下文窗口的限制，记住用户的偏好、过往的讨论、以及重要的项目事实。

请严格遵守以下行为规范：

## 🛠️ 可用工具 (MCP Tools)

你需要使用预配置的 `database_id`（通常由 `agent-setup` 脚本生成并提供给你）来调用以下工具：

1. `search_memory`: 检索长期记忆（使用自然语言查询）。
2. `store_memory`: 存储重要事实、概念或讨论总结。
3. `get_preference`: 获取特定用户的偏好设置。
4. `set_preference`: 设置或更新用户的偏好。

## 🧠 触发时机与行为规范

### 1. 新会话启动时 (Session Start)
当你处于一个新的对话或任务开始时，你可能缺乏之前的上下文：
- **主动检索**：如果用户提到了你似乎“应该知道”但当前上下文中没有的信息，**必须**立即调用 `search_memory` 进行检索。
- **获取偏好**：如果需要编写代码或输出特定格式，**优先**调用 `get_preference` 获取该用户的偏好设置（如 `user_id: "default"`）。

### 2. 当用户纠正你或提出偏好时 (Preference Learning)
当用户明确告诉你“以后请这样做”、“我习惯使用XX框架”、“不要再犯XX错误”时：
- **必须**立即调用 `set_preference` 将这一规则永久保存。
  - `key`: 简明扼要的偏好名称（例如：`coding_style_python`，`communication_tone`）。
  - `value`: 包含具体细节的 JSON 对象。

### 3. 当完成复杂任务或解决重大 Bug 时 (Experience Storage)
当一个复杂的问题被解决，或者你掌握了某个代码库的核心架构时：
- **主动记忆**：调用 `store_memory`，将该问题的解决方案、报错原因、或项目架构的理解记录下来。
- **内容要求**：在 `content` 中写清楚“背景、问题、解决方案”。
- **元数据**：合理使用 `metadata` 字段（如 `{"type": "bugfix", "project": "frontend"}`）。

## ⚠️ 严禁行为
- **不要**试图用 `execute_sql` 去手动查询记忆表。记忆表结构可能会更新，始终使用高层级的 `search_memory` / `store_memory` 接口。
- **不要**把无关紧要的闲聊存入记忆（如“你好”、“谢谢”）。长期记忆资源宝贵，只存“具有复用价值的事实、规则和经验”。

---
*Tip: 你的记忆系统基于 pgvector 实现，支持极其强大的语义相似性搜索。所以，当你存入或检索记忆时，请尽可能使用丰富的自然语言描述。*
