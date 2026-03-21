# OpenClaw-db9 Skill

## 概述

OpenClaw-db9 (oc-db9) 是一个完全自托管的 db9.ai 替代品，为 AI Agent 提供完整的数据库基础设施。

**核心特性**：
- 即时数据库创建与管理
- 内置向量搜索 (pgvector)
- 文件存储 (MinIO)
- 数据库分支
- 定时任务
- 类型生成 (TypeScript/Python/Go)
- 备份与恢复

## 安装

### 前置要求

- Docker & Docker Compose
- Go 1.21+ (用于编译 CLI)

### 快速开始

```bash
# 1. 克隆仓库
git clone https://github.com/yourname/openclaw-db9.git
cd openclaw-db9

# 2. 配置环境变量
cp .env.example .env
# 编辑 .env 设置密码

# 3. 启动服务
docker-compose up -d

# 4. 安装 CLI
cd cmd/oc-db9 && go install

# 5. 验证安装
oc-db9 --version
```

## 核心功能

### 创建数据库

```bash
# CLI 方式
oc-db9 db create myapp

# API 方式
curl -X POST http://localhost:8080/api/v1/databases \
  -H "Content-Type: application/json" \
  -d '{"name": "myapp"}'
```

### 执行 SQL

```bash
# CLI 方式
oc-db9 db sql <database-id> -c "CREATE TABLE users (id SERIAL PRIMARY KEY, name TEXT)"
oc-db9 db sql <database-id> -c "INSERT INTO users (name) VALUES ('alice')"
oc-db9 db sql <database-id> -c "SELECT * FROM users"

# API 方式
curl -X POST http://localhost:8080/api/v1/databases/<id>/sql \
  -H "Content-Type: application/json" \
  -d '{"sql": "SELECT * FROM users"}'
```

### 获取连接信息

```bash
oc-db9 db connect <database-id>
```

输出：
```
Host: localhost
Port: 5432
Database: oc_xxxxxxxx
User: postgres
Connection String: postgresql://postgres:postgres@localhost:5432/oc_xxxxxxxx
```

### 文件操作

```bash
# 上传文件
oc-db9 fs cp ./document.pdf /docs/ --db <database-id>

# 列出文件
oc-db9 fs ls /docs/ --db <database-id>

# 下载文件
oc-db9 fs cat <file-id>

# 删除文件
oc-db9 fs rm <file-id>
```

### 向量搜索

```bash
# 创建向量表
curl -X POST http://localhost:8080/api/v1/embeddings/tables \
  -H "Content-Type: application/json" \
  -d '{"database_id": "<id>", "table_name": "documents", "dimension": 768}'

# 插入向量（自动生成 embedding）
curl -X POST http://localhost:8080/api/v1/embeddings/insert \
  -H "Content-Type: application/json" \
  -d '{"database_id": "<id>", "table_name": "documents", "content": "Hello world"}'

# 相似度搜索
curl -X POST http://localhost:8080/api/v1/embeddings/search \
  -H "Content-Type: application/json" \
  -d '{"database_id": "<id>", "table_name": "documents", "query": "Hello", "limit": 5}'
```

### 定时任务

```bash
# 创建定时任务
curl -X POST http://localhost:8080/api/v1/cron \
  -H "Content-Type: application/json" \
  -d '{
    "database_id": "<id>",
    "name": "cleanup",
    "schedule": "0 */6 * * *",
    "sql_command": "DELETE FROM logs WHERE created_at < NOW() - INTERVAL '7 days'"
  }'

# 列出定时任务
curl "http://localhost:8080/api/v1/cron?database_id=<id>"

# 查看任务日志
curl "http://localhost:8080/api/v1/cron/<job-id>/logs"
```

### 分支管理

```bash
# 创建分支
curl -X POST http://localhost:8080/api/v1/branches \
  -H "Content-Type: application/json" \
  -d '{"database_id": "<id>", "name": "feature-x"}'

# 列出分支
curl "http://localhost:8080/api/v1/branches?database_id=<id>"

# 删除分支
curl -X DELETE http://localhost:8080/api/v1/branches/<branch-id>
```

### 类型生成

```bash
# TypeScript
oc-db9 gen types --db <database-id> --lang typescript --output ./types.ts

# Python
oc-db9 gen types --db <database-id> --lang python --output ./models.py

# Go
oc-db9 gen types --db <database-id> --lang go --output ./models.go
```

### 备份与恢复

```bash
# 创建备份
curl -X POST http://localhost:8080/api/v1/backups \
  -H "Content-Type: application/json" \
  -d '{"database_id": "<id>", "name": "daily-backup"}'

# 列出备份
curl "http://localhost:8080/api/v1/backups?database_id=<id>"

# 恢复备份
curl -X POST http://localhost:8080/api/v1/backups/restore \
  -H "Content-Type: application/json" \
  -d '{"backup_id": "<backup-id>"}'

# 下载备份
curl http://localhost:8080/api/v1/backups/<id>/download -o backup.dump
```

### 监控

```bash
# 系统状态
curl http://localhost:8080/api/v1/monitor/system

# 数据库统计
curl http://localhost:8080/api/v1/monitor/stats

# 健康检查
curl http://localhost:8080/api/v1/monitor/health
```

## API 参考

### 数据库操作

| 方法 | 路径 | 描述 |
|------|------|------|
| POST | /api/v1/databases | 创建数据库 |
| GET | /api/v1/databases | 列出数据库 |
| POST | /api/v1/databases/quick-setup | 快速配置（模板） |
| GET | /api/v1/databases/templates | 列出可用模板 |
| GET | /api/v1/databases/:id | 获取数据库详情 |
| DELETE | /api/v1/databases/:id | 删除数据库 |
| POST | /api/v1/databases/:id/sql | 执行 SQL |
| GET | /api/v1/databases/:id/connect | 获取连接信息 |

### 文件操作

| 方法 | 路径 | 描述 |
|------|------|------|
| POST | /api/v1/files/upload | 上传文件 |
| GET | /api/v1/files | 列出文件 |
| GET | /api/v1/files/:id | 下载文件 |
| DELETE | /api/v1/files/:id | 删除文件 |

### 向量操作

| 方法 | 路径 | 描述 |
|------|------|------|
| POST | /api/v1/embeddings/generate | 生成 Embedding |
| POST | /api/v1/embeddings/tables | 创建向量表 |
| POST | /api/v1/embeddings/insert | 插入向量 |
| POST | /api/v1/embeddings/search | 相似度搜索 |

### 分支操作

| 方法 | 路径 | 描述 |
|------|------|------|
| POST | /api/v1/branches | 创建分支 |
| GET | /api/v1/branches | 列出分支 |
| DELETE | /api/v1/branches/:id | 删除分支 |

### 定时任务

| 方法 | 路径 | 描述 |
|------|------|------|
| POST | /api/v1/cron | 创建定时任务 |
| GET | /api/v1/cron | 列出定时任务 |
| DELETE | /api/v1/cron/:id | 删除定时任务 |
| GET | /api/v1/cron/:id/logs | 获取任务日志 |

### 备份操作

| 方法 | 路径 | 描述 |
|------|------|------|
| POST | /api/v1/backups | 创建备份 |
| GET | /api/v1/backups | 列出备份 |
| POST | /api/v1/backups/restore | 恢复备份 |
| DELETE | /api/v1/backups/:id | 删除备份 |
| GET | /api/v1/backups/:id/download | 下载备份 |

### 监控

| 方法 | 路径 | 描述 |
|------|------|------|
| GET | /api/v1/monitor/system | 系统状态 |
| GET | /api/v1/monitor/stats | 数据库统计 |
| GET | /api/v1/monitor/health | 健康检查 |

### 快速配置

OpenClaw 可以通过 Quick-Setup API 自行配置 oc-db9 作为记忆存储层。

#### 支持的模板

| 模板 | 用途 | 主要表 |
|------|------|--------|
| `ai-memory` | AI Agent 跨对话记忆 | user_preferences, conversation_history, knowledge_base |
| `workflow-state` | 工作流状态管理 | workflow_instances, workflow_events |
| `knowledge-base` | 企业知识库 | documents, tags |

#### 一键创建记忆数据库

```bash
curl -X POST http://localhost:8080/api/v1/databases/quick-setup \
  -H "Content-Type: application/json" \
  -d '{
    "template": "ai-memory",
    "name": "my-agent-memory"
  }'
```

响应：
```json
{
  "database_id": "xxx",
  "database_name": "oc_xxx",
  "connection_string": "postgresql://postgres:postgres@localhost:5432/oc_xxx",
  "dashboard_url": "http://localhost:8080/",
  "tables_created": ["user_preferences", "conversation_history", "knowledge_base"],
  "vector_enabled": true
}
```

#### 查看可用模板

```bash
curl http://localhost:8080/api/v1/databases/templates
```

#### OpenClaw 自动配置流程

1. **检测需求**: OpenClaw 发现需要持久化存储时
2. **选择模板**: 根据用途选择 `ai-memory` 模板
3. **调用 API**: `POST /api/v1/databases/quick-setup`
4. **获取连接**: 收到 connection_string
5. **更新配置**: 将连接字符串写入 OpenClaw 配置
6. **验证连接**: 执行测试查询确认可用

## 最佳实践

### 不要生成脚本

**❌ 错误做法：** 生成一堆 shell 脚本、Python 脚本
**✅ 正确做法：** 直接调用 HTTP API

oc-db9 是 API-first 设计，所有操作都应该通过 HTTP API 完成，不需要生成脚本。

### 集成原则

1. **直接调用 API** - 不需要包装脚本
2. **复用连接** - 获取 connection_string 后直接使用
3. **使用模板** - 优先用 `/quick-setup` 而不是手动建表
4. **一行命令** - curl 调用足够简单，无需脚本

### 典型调用模式

```bash
# 创建数据库（一行命令）
curl -X POST http://localhost:8080/api/v1/databases/quick-setup \
  -H "Content-Type: application/json" \
  -d '{"template":"ai-memory","name":"my-db"}'

# 执行 SQL（一行命令）
curl -X POST http://localhost:8080/api/v1/databases/${DB_ID}/sql \
  -H "Content-Type: application/json" \
  -d '{"sql":"SELECT * FROM users LIMIT 10"}'
```

### 常见错误

| 错误 | 原因 | 正确做法 |
|------|------|----------|
| 生成 install.sh | 不需要 | 直接 `docker-compose up -d` |
| 生成 init_db.py | 不需要 | 用 `/quick-setup` API |
| 生成 backup.sh | 不需要 | 用 `/backups` API |
| 生成 wrapper 脚本 | 不需要 | 直接调用 HTTP API |

## 与 db9.ai 的差异

| 功能 | db9.ai | OpenClaw-db9 |
|------|--------|--------------|
| 分支速度 | 毫秒级 (CoW) | 秒级 (CREATE DATABASE WITH TEMPLATE) |
| 文件事务 | 与 SQL 同一事务 | 应用层两阶段提交 |
| 托管方式 | SaaS | 完全自托管 |
| 成本 | 按量付费 | 服务器成本 |
| Embedding | 内置 | Ollama 外部服务 |
| 向量存储 | 内置 | pgvector 扩展 |

## 架构

```
┌─────────────────────────────────────────────────────────────┐
│                    oc-db9 CLI (Go/Cobra)                    │
│  create | list | db sql | fs cp | branch | cron | gen      │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                   API Gateway (Go/Gin)                      │
│         认证 | 路由 | 多租户隔离 | 连接池管理                  │
└─────────────────────────────────────────────────────────────┘
                              │
        ┌─────────────────────┼─────────────────────┐
        ▼                     ▼                     ▼
┌───────────────┐     ┌───────────────┐     ┌───────────────┐
│  PostgreSQL   │     │    MinIO      │     │   Ollama      │
│  + pgvector   │     │  (文件存储)    │     │  (Embedding)  │
│  + pg_cron    │     │               │     │               │
└───────────────┘     └───────────────┘     └───────────────┘
```

## 配置

### 环境变量

| 变量 | 默认值 | 描述 |
|------|--------|------|
| POSTGRES_PASSWORD | postgres | PostgreSQL 密码 |
| MINIO_ROOT_USER | minioadmin | MinIO 访问密钥 |
| MINIO_ROOT_PASSWORD | minioadmin | MinIO 密钥 |
| API_PORT | 8080 | API 服务端口 |
| ENVIRONMENT | development | 运行环境 |

### Docker Compose 服务

| 服务 | 端口 | 描述 |
|------|------|------|
| postgres | 5432 | PostgreSQL 数据库 |
| minio | 9000/9001 | 文件存储 |
| ollama | 11434 | Embedding 服务 (可选) |
| api | 8080 | REST API |

## 故障排除

### 常见问题

**Q: 数据库创建失败**
```bash
# 检查 PostgreSQL 日志
docker logs oc-db9-postgres

# 确保扩展已安装
docker exec -it oc-db9-postgres psql -U postgres -c "SELECT * FROM pg_extension;"
```

**Q: MinIO 连接失败**
```bash
# 检查 MinIO 状态
docker logs oc-db9-minio

# 访问 MinIO 控制台
# http://localhost:9001
```

**Q: Embedding 生成失败**
```bash
# 启动 Ollama 服务
docker-compose --profile embedding up -d

# 拉取模型
docker exec -it oc-db9-ollama ollama pull nomic-embed-text
```

## 开发

### 项目结构

```
openclaw-db9/
├── api/                    # API Gateway
│   ├── cmd/api/main.go
│   ├── internal/
│   │   ├── config/
│   │   ├── handlers/
│   │   └── middleware/
│   └── Dockerfile
├── cmd/oc-db9/             # CLI 入口
├── internal/cmd/           # CLI 命令
├── init-scripts/           # 数据库初始化
├── docker-compose.yml
└── skill.md
```

### 构建

```bash
# 构建 API
cd api && docker build -t oc-db9-api .

# 构建 CLI
go build -o oc-db9 ./cmd/oc-db9
```

### 测试

```bash
# 运行 API 测试
go test ./api/internal/handlers/...

# 运行 CLI 测试
go test ./internal/cmd/...
```

## 许可证

MIT License
