# OpenClaw 跨对话记忆配置 Prompt

请按以下步骤为 OpenClaw 配置跨对话记忆能力：

---

## 步骤 1：搜索并部署 oc-db9

```bash
# 克隆项目
git clone https://github.com/yihui504/alternative_db9.git
cd alternative_db9

# 启动数据库服务
docker-compose up -d postgres minio
```

---

## 步骤 2：启动 API 服务

```bash
# 进入 api 目录并编译
cd api
go build -o oc-db9 ./cmd/api/

# 设置环境变量并启动
export DATABASE_URL="postgres://postgres:postgres@localhost:5432/postgres"
export MINIO_ENDPOINT="localhost:9000"
export MINIO_ACCESS_KEY="minioadmin"
export MINIO_SECRET_KEY="minioadmin"

# 运行 API
./oc-db9
```

---

## 步骤 3：一键配置记忆数据库

在另一个终端执行：

```bash
# 创建 AI Agent 记忆数据库
curl -X POST http://localhost:8080/api/v1/databases/quick-setup \
  -H "Content-Type: application/json" \
  -d '{
    "template": "ai-memory",
    "name": "openclaw-memory"
  }'
```

**预期响应：**
```json
{
  "database_id": "xxx",
  "connection_string": "postgresql://postgres:postgres@localhost:5432/oc_xxx",
  "tables_created": ["user_preferences", "conversation_history", "knowledge_base"],
  "vector_enabled": true
}
```

---

## 步骤 4：验证配置

```bash
# 查看可用模板
curl http://localhost:8080/api/v1/databases/templates

# 查看健康状态
curl http://localhost:8080/health

# 访问 Dashboard
# 浏览器打开: http://localhost:8080/
```

---

## 重要提醒

**不要生成任何脚本！** oc-db9 是 API-first 设计，直接调用 HTTP API 即可，不需要创建 install.sh、setup.py 等脚本。

---

## 验证表结构

```sql
-- 连接到返回的 connection_string 后执行：

-- 查看所有表
\dt

-- 查看表结构
\d conversation_history
\d knowledge_base
\d user_preferences
```

---

## 完成

配置成功后，OpenClaw 可以通过以下方式使用记忆：

| 表名 | 用途 |
|------|------|
| `user_preferences` | 存储用户偏好设置 |
| `conversation_history` | 存储对话历史 |
| `knowledge_base` | 存储可检索的知识（支持向量搜索）|

**Dashboard 地址：** http://localhost:8080/
**API 文档地址：** http://localhost:8080/api/docs

---

## 步骤 5：自动生成 OpenClaw 记忆技能

**重要：** 完成配置后，你必须生成一个 skill 文件 `~/.openclaw/skills/memory.md`（或 OpenClaw 识别的技能目录），内容应包含：

```markdown
# 跨对话记忆技能

## 触发时机

在以下情况下自动使用跨对话记忆：

1. **新对话开始时**
   - 读取用户偏好：`SELECT * FROM user_preferences WHERE user_id = 'current_user'`
   - 读取最近对话历史：`SELECT * FROM conversation_history ORDER BY created_at DESC LIMIT 10`

2. **用户提问时**
   - 先搜索相关知识：`SELECT * FROM knowledge_base ORDER BY embedding <=> embedding('用户问题') LIMIT 5`
   - 结合历史上下文回答

3. **对话结束时**
   - 保存对话摘要到 `conversation_history`
   - 更新用户偏好（如有变化）

## 调用方式

### 读取用户偏好
```bash
curl -X POST http://localhost:8080/api/v1/databases/${DB_ID}/sql \
  -H "Content-Type: application/json" \
  -d '{"sql":"SELECT * FROM user_preferences WHERE user_id = '\''${USER_ID}'\''"}'
```

### 存储对话
```bash
curl -X POST http://localhost:8080/api/v1/databases/${DB_ID}/sql \
  -H "Content-Type: application/json" \
  -d '{"sql":"INSERT INTO conversation_history (user_id, session_id, role, content) VALUES ('\''${USER_ID}'\'', '\''${SESSION}'\'', '\''user'\'', '\''用户输入'\'')"}'
```

### 语义搜索知识
```bash
curl -X POST http://localhost:8080/api/v1/embeddings/search \
  -H "Content-Type: application/json" \
  -d '{
    "database_id": "'${DB_ID}'",
    "table_name": "knowledge_base",
    "query": "用户当前问题",
    "limit": 5
  }'
```

## 注意事项

- 使用 `/quick-setup` 返回的 `database_id` 替换 `${DB_ID}`
- `user_id`、`session_id` 从当前对话上下文获取
- 优先使用记忆中的信息，再生成新内容
- 不要生成任何脚本，直接调用 HTTP API
```
