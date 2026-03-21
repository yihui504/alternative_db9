# OpenClaw-db9 API 参考文档

本文档详细描述了 OpenClaw-db9 的所有 API 端点。

## 基础信息

- **Base URL**: `http://localhost:8080/api/v1`
- **Content-Type**: `application/json`
- **认证**: 当前版本暂无认证（生产环境建议添加）

## 通用响应格式

### 成功响应

```json
{
  "id": "uuid",
  "name": "string",
  "created_at": "2026-03-21T00:00:00Z"
}
```

### 错误响应

```json
{
  "error": "错误描述信息"
}
```

---

## 数据库管理

### 创建数据库

创建一个新的数据库实例。

**请求**

```
POST /api/v1/databases
```

**请求体**

```json
{
  "name": "mydb",
  "description": "可选的数据库描述"
}
```

| 字段 | 类型 | 必填 | 描述 |
|------|------|------|------|
| name | string | 是 | 数据库名称（仅支持字母、数字、下划线） |
| description | string | 否 | 数据库描述 |

**响应**

```json
{
  "id": "746d6a2a-b3c7-4145-a376-d1a2be69b573",
  "name": "mydb",
  "postgres_db_name": "oc_746d6a2a",
  "created_at": "2026-03-21T00:09:18.543058+08:00"
}
```

**示例**

```bash
curl -X POST http://localhost:8080/api/v1/databases \
  -H "Content-Type: application/json" \
  -d '{"name":"mydb"}'
```

---

### 列出数据库

获取所有数据库列表。

**请求**

```
GET /api/v1/databases
```

**响应**

```json
[
  {
    "id": "746d6a2a-b3c7-4145-a376-d1a2be69b573",
    "name": "mydb",
    "postgres_db_name": "oc_746d6a2a",
    "created_at": "2026-03-21T00:09:18.543058+08:00"
  }
]
```

**示例**

```bash
curl http://localhost:8080/api/v1/databases
```

---

### 获取数据库详情

获取单个数据库的详细信息。

**请求**

```
GET /api/v1/databases/:id
```

**路径参数**

| 参数 | 类型 | 描述 |
|------|------|------|
| id | string | 数据库 UUID |

**响应**

```json
{
  "id": "746d6a2a-b3c7-4145-a376-d1a2be69b573",
  "name": "mydb",
  "postgres_db_name": "oc_746d6a2a",
  "created_at": "2026-03-21T00:09:18.543058+08:00"
}
```

---

### 删除数据库

删除指定的数据库。

**请求**

```
DELETE /api/v1/databases/:id
```

**路径参数**

| 参数 | 类型 | 描述 |
|------|------|------|
| id | string | 数据库 UUID |

**响应**

```json
{
  "message": "Database deleted successfully"
}
```

---

### 执行 SQL

在指定数据库中执行 SQL 语句。

**请求**

```
POST /api/v1/databases/:id/sql
```

**请求体**

```json
{
  "SQL": "SELECT * FROM users WHERE id = $1",
  "params": [1]
}
```

| 字段 | 类型 | 必填 | 描述 |
|------|------|------|------|
| SQL | string | 是 | SQL 语句 |
| params | array | 否 | 参数化查询参数 |

**响应**

```json
{
  "results": [
    {"id": 1, "name": "Alice", "email": "alice@example.com"}
  ]
}
```

**示例**

```bash
# 创建表
curl -X POST http://localhost:8080/api/v1/databases/{id}/sql \
  -H "Content-Type: application/json" \
  -d '{"SQL":"CREATE TABLE users (id SERIAL PRIMARY KEY, name TEXT, email TEXT)"}'

# 插入数据
curl -X POST http://localhost:8080/api/v1/databases/{id}/sql \
  -H "Content-Type: application/json" \
  -d '{"SQL":"INSERT INTO users (name, email) VALUES ($1, $2)", "params":["Alice","alice@example.com"]}'

# 查询数据
curl -X POST http://localhost:8080/api/v1/databases/{id}/sql \
  -H "Content-Type: application/json" \
  -d '{"SQL":"SELECT * FROM users"}'
```

---

### 获取连接信息

获取数据库的连接字符串信息。

**请求**

```
GET /api/v1/databases/:id/connect
```

**响应**

```json
{
  "host": "localhost",
  "port": 5432,
  "database": "oc_746d6a2a",
  "user": "postgres",
  "password": "postgres",
  "connection_string": "postgres://postgres:postgres@localhost:5432/oc_746d6a2a"
}
```

---

## 文件存储

### 上传文件

上传文件到存储服务。

**请求**

```
POST /api/v1/files/upload
```

**请求体** (multipart/form-data)

| 字段 | 类型 | 必填 | 描述 |
|------|------|------|------|
| file | file | 是 | 要上传的文件 |
| database_id | string | 否 | 关联的数据库 ID |

**响应**

```json
{
  "id": "file-uuid",
  "name": "document.pdf",
  "size": 1024,
  "content_type": "application/pdf",
  "url": "/api/v1/files/file-uuid"
}
```

---

### 列出文件

获取文件列表。

**请求**

```
GET /api/v1/files
```

**查询参数**

| 参数 | 类型 | 描述 |
|------|------|------|
| database_id | string | 按数据库 ID 过滤 |

**响应**

```json
[
  {
    "id": "file-uuid",
    "name": "document.pdf",
    "size": 1024,
    "content_type": "application/pdf",
    "created_at": "2026-03-21T00:00:00Z"
  }
]
```

---

### 下载文件

下载指定文件。

**请求**

```
GET /api/v1/files/:id
```

**响应**

文件内容（二进制流）

---

### 删除文件

删除指定文件。

**请求**

```
DELETE /api/v1/files/:id
```

**响应**

```json
{
  "message": "File deleted successfully"
}
```

---

### 查询文件内容

将文件内容作为表格数据查询，支持 CSV 和 JSONL 格式。

**请求**

```
GET /api/v1/files/query
```

**查询参数**

| 参数 | 类型 | 必填 | 描述 |
|------|------|------|------|
| database_id | string | 是 | 数据库 ID |
| path | string | 是 | 文件路径 |

**响应**

```json
{
  "results": [
    {
      "_line_number": 1,
      "_path": "/data/users.csv",
      "name": "Alice",
      "age": 30,
      "city": "NYC"
    },
    {
      "_line_number": 2,
      "_path": "/data/users.csv",
      "name": "Bob",
      "age": 25,
      "city": "LA"
    }
  ],
  "count": 2
}
```

**示例**

```bash
# 查询 CSV 文件
curl "http://localhost:8080/api/v1/files/query?database_id={db-id}&path=/data/users.csv"

# 使用 CLI
oc-db9 fs query /data/users.csv --db {db-id}
```

**支持的文件格式**

- CSV (自动类型推断)
- JSONL (动态列发现)
- Parquet (列式存储，高性能)

**Parquet 优势**

Parquet 是大数据领域标准的列式存储格式：
- 查询速度快（只读取需要的列，比 CSV 快 10-100 倍）
- 压缩率高（节省 50-90% 存储空间）
- 自带数据类型信息

**元数据列**

| 列名 | 描述 |
|------|------|
| `_line_number` | 行号 |
| `_path` | 文件路径 |

---

## 分支管理

### 创建分支

从现有数据库创建分支。

**请求**

```
POST /api/v1/branches
```

**请求体**

```json
{
  "name": "feature-branch",
  "source_database_id": "source-db-uuid"
}
```

| 字段 | 类型 | 必填 | 描述 |
|------|------|------|------|
| name | string | 是 | 分支名称 |
| source_database_id | string | 是 | 源数据库 ID |

**响应**

```json
{
  "id": "branch-uuid",
  "name": "feature-branch",
  "source_database_id": "source-db-uuid",
  "postgres_db_name": "oc_branch_uuid",
  "created_at": "2026-03-21T00:00:00Z"
}
```

---

### 列出分支

获取所有分支列表。

**请求**

```
GET /api/v1/branches
```

**响应**

```json
[
  {
    "id": "branch-uuid",
    "name": "feature-branch",
    "source_database_id": "source-db-uuid",
    "created_at": "2026-03-21T00:00:00Z"
  }
]
```

---

### 删除分支

删除指定分支。

**请求**

```
DELETE /api/v1/branches/:id
```

**响应**

```json
{
  "message": "Branch deleted successfully"
}
```

---

## 定时任务

### 创建定时任务

创建一个新的定时任务。

**请求**

```
POST /api/v1/cron
```

**请求体**

```json
{
  "name": "cleanup-task",
  "schedule": "0 0 * * * *",
  "sql": "DELETE FROM logs WHERE created_at < NOW() - INTERVAL '7 days'",
  "database_id": "db-uuid"
}
```

| 字段 | 类型 | 必填 | 描述 |
|------|------|------|------|
| name | string | 是 | 任务名称 |
| schedule | string | 是 | Cron 表达式（支持秒级） |
| sql | string | 是 | 要执行的 SQL |
| database_id | string | 是 | 目标数据库 ID |

**Cron 表达式格式**

```
秒 分 时 日 月 周
*  *  *  *  *  *
```

示例：
- `0 0 * * * *` - 每小时执行
- `0 0 0 * * *` - 每天午夜执行
- `0 */30 * * * *` - 每30分钟执行

**响应**

```json
{
  "id": "cron-uuid",
  "name": "cleanup-task",
  "schedule": "0 0 * * * *",
  "active": true,
  "created_at": "2026-03-21T00:00:00Z"
}
```

---

### 列出定时任务

获取所有定时任务列表。

**请求**

```
GET /api/v1/cron
```

**响应**

```json
[
  {
    "id": "cron-uuid",
    "name": "cleanup-task",
    "schedule": "0 0 * * * *",
    "active": true,
    "created_at": "2026-03-21T00:00:00Z"
  }
]
```

---

### 删除定时任务

删除指定定时任务。

**请求**

```
DELETE /api/v1/cron/:id
```

**响应**

```json
{
  "message": "Cron job deleted successfully"
}
```

---

### 获取任务执行日志

获取定时任务的执行日志。

**请求**

```
GET /api/v1/cron/:id/logs
```

**响应**

```json
[
  {
    "id": "log-uuid",
    "cron_job_id": "cron-uuid",
    "status": "success",
    "message": "Executed successfully",
    "executed_at": "2026-03-21T00:00:00Z"
  }
]
```

---

## 向量搜索

### 生成嵌入向量

使用 Ollama 生成文本的嵌入向量。

**请求**

```
POST /api/v1/embeddings/generate
```

**请求体**

```json
{
  "text": "这是一段需要生成向量的文本",
  "model": "nomic-embed-text"
}
```

| 字段 | 类型 | 必填 | 描述 |
|------|------|------|------|
| text | string | 是 | 要生成向量的文本 |
| model | string | 否 | 使用的模型（默认 nomic-embed-text） |

**响应**

```json
{
  "embedding": [0.1, 0.2, 0.3, "..."],
  "dimensions": 768
}
```

---

### 创建向量表

在数据库中创建向量存储表。

**请求**

```
POST /api/v1/embeddings/tables
```

**请求体**

```json
{
  "database_id": "db-uuid",
  "table_name": "documents",
  "dimensions": 768
}
```

| 字段 | 类型 | 必填 | 描述 |
|------|------|------|------|
| database_id | string | 是 | 数据库 ID |
| table_name | string | 是 | 表名 |
| dimensions | int | 是 | 向量维度 |

**响应**

```json
{
  "message": "Vector table created successfully",
  "table_name": "documents"
}
```

---

### 插入向量

向向量表中插入数据。

**请求**

```
POST /api/v1/embeddings/insert
```

**请求体**

```json
{
  "database_id": "db-uuid",
  "table_name": "documents",
  "content": "文档内容",
  "embedding": [0.1, 0.2, 0.3, "..."],
  "metadata": {
    "source": "web",
    "category": "tech"
  }
}
```

**响应**

```json
{
  "id": "vector-uuid",
  "message": "Vector inserted successfully"
}
```

---

### 相似性搜索

执行向量相似性搜索。

**请求**

```
POST /api/v1/embeddings/search
```

**请求体**

```json
{
  "database_id": "db-uuid",
  "table_name": "documents",
  "embedding": [0.1, 0.2, 0.3, "..."],
  "limit": 10
}
```

| 字段 | 类型 | 必填 | 描述 |
|------|------|------|------|
| database_id | string | 是 | 数据库 ID |
| table_name | string | 是 | 表名 |
| embedding | array | 是 | 查询向量 |
| limit | int | 否 | 返回结果数量（默认 10） |

**响应**

```json
{
  "results": [
    {
      "id": "vector-uuid",
      "content": "文档内容",
      "similarity": 0.95,
      "metadata": {
        "source": "web",
        "category": "tech"
      }
    }
  ]
}
```

---

## 备份恢复

### 创建备份

创建数据库备份。

**请求**

```
POST /api/v1/backups
```

**请求体**

```json
{
  "database_id": "db-uuid",
  "name": "daily-backup"
}
```

**响应**

```json
{
  "id": "backup-uuid",
  "name": "daily-backup",
  "database_id": "db-uuid",
  "size": 1024000,
  "status": "completed",
  "created_at": "2026-03-21T00:00:00Z"
}
```

---

### 列出备份

获取备份列表。

**请求**

```
GET /api/v1/backups
```

**查询参数**

| 参数 | 类型 | 描述 |
|------|------|------|
| database_id | string | 按数据库 ID 过滤 |

**响应**

```json
[
  {
    "id": "backup-uuid",
    "name": "daily-backup",
    "database_id": "db-uuid",
    "size": 1024000,
    "status": "completed",
    "created_at": "2026-03-21T00:00:00Z"
  }
]
```

---

### 恢复备份

从备份恢复数据库。

**请求**

```
POST /api/v1/backups/restore
```

**请求体**

```json
{
  "backup_id": "backup-uuid",
  "target_database_id": "target-db-uuid"
}
```

**响应**

```json
{
  "message": "Database restored successfully"
}
```

---

### 下载备份

下载备份文件。

**请求**

```
GET /api/v1/backups/:id/download
```

**响应**

备份文件（二进制流）

---

### 删除备份

删除指定备份。

**请求**

```
DELETE /api/v1/backups/:id
```

**响应**

```json
{
  "message": "Backup deleted successfully"
}
```

---

## 监控

### 健康检查

检查所有服务的健康状态。

**请求**

```
GET /api/v1/monitor/health
```

**响应**

```json
{
  "postgresql": "healthy",
  "minio": "healthy",
  "api": "healthy"
}
```

---

### 数据库统计

获取数据库使用统计。

**请求**

```
GET /api/v1/monitor/stats
```

**响应**

```json
{
  "total_databases": 5,
  "total_files": 100,
  "total_branches": 2,
  "total_cron_jobs": 3
}
```

---

### 系统信息

获取系统运行信息。

**请求**

```
GET /api/v1/monitor/system
```

**响应**

```json
{
  "version": "1.0.0",
  "uptime": "2h30m",
  "go_version": "go1.23",
  "memory_usage": "50MB",
  "goroutines": 25
}
```

---

## 错误码

| HTTP 状态码 | 描述 |
|------------|------|
| 200 | 成功 |
| 201 | 创建成功 |
| 400 | 请求参数错误 |
| 404 | 资源不存在 |
| 500 | 服务器内部错误 |

---

## CORS 配置

API 默认启用 CORS，支持跨域请求。生产环境建议配置具体的允许域名。

---

## 速率限制

当前版本未实现速率限制。生产环境建议添加。
