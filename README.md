# OpenClaw-db9 (oc-db9)

一个完全自托管的 db9.ai 开源替代方案，提供即时数据库创建、向量搜索、文件存储、分支管理等功能。

## 特性

- **即时数据库创建** - 秒级创建独立的 PostgreSQL 数据库
- **向量搜索** - 基于 pgvector 的嵌入向量存储和相似性搜索
- **文件存储** - S3 兼容的文件存储服务（MinIO）
- **数据库分支** - 基于 PostgreSQL 模板的快速分支功能
- **定时任务** - 应用级 Cron 调度器
- **备份恢复** - 基于 pg_dump 的完整备份方案
- **类型生成** - 自动生成 TypeScript、Python、Go 类型定义
- **监控接口** - 系统健康检查和统计信息

## 技术栈

| 组件     | 技术                       |
| ------ | ------------------------ |
| 数据库    | PostgreSQL 16 + pgvector |
| 文件存储   | MinIO (S3 兼容)            |
| 嵌入模型   | Ollama (可选)              |
| API 框架 | Go + Gin                 |
| CLI 工具 | Go + Cobra               |
| 容器编排   | Docker Compose           |

## 快速开始

### 前置要求

- Docker 20.10+
- Docker Compose 2.0+
- (可选) Go 1.23+ 用于本地开发

### 启动服务

```bash
# 克隆项目
git clone https://github.com/yihui504/alternative_db9.git
cd alternative_db9

# 启动所有服务
docker-compose up -d

# 查看服务状态
docker-compose ps
```

服务启动后：

- API 服务：http://localhost:8080
- MinIO 控制台：http://localhost:9001 (用户名/密码: minioadmin/minioadmin)
- PostgreSQL：localhost:5432 (用户名/密码: postgres/postgres)

### 验证安装

```bash
# 健康检查
curl http://localhost:8080/health

# 创建数据库
curl -X POST http://localhost:8080/api/v1/databases \
  -H "Content-Type: application/json" \
  -d '{"name":"mydb"}'

# 列出数据库
curl http://localhost:8080/api/v1/databases
```

## 项目结构

```
oc-db9/
├── api/                    # API 服务
│   ├── cmd/api/           # 入口点
│   ├── internal/
│   │   ├── config/        # 配置管理
│   │   ├── handlers/      # HTTP 处理器
│   │   └── middleware/    # 中间件
│   └── Dockerfile
├── cmd/oc-db9/            # CLI 工具
├── init-scripts/          # 数据库初始化脚本
├── docs/                  # 文档
├── docker-compose.yml     # 服务编排
└── README.md
```

## API 概览

### 数据库管理

| 端点                              | 方法     | 描述      |
| ------------------------------- | ------ | ------- |
| `/api/v1/databases`             | POST   | 创建数据库   |
| `/api/v1/databases`             | GET    | 列出所有数据库 |
| `/api/v1/databases/:id`         | GET    | 获取数据库详情 |
| `/api/v1/databases/:id`         | DELETE | 删除数据库   |
| `/api/v1/databases/:id/sql`     | POST   | 执行 SQL  |
| `/api/v1/databases/:id/connect` | GET    | 获取连接信息  |

### 文件存储

| 端点                     | 方法     | 描述   |
| ---------------------- | ------ | ---- |
| `/api/v1/files/upload` | POST   | 上传文件 |
| `/api/v1/files`        | GET    | 列出文件 |
| `/api/v1/files/:id`    | GET    | 下载文件 |
| `/api/v1/files/:id`    | DELETE | 删除文件 |

### 分支管理

| 端点                     | 方法     | 描述   |
| ---------------------- | ------ | ---- |
| `/api/v1/branches`     | POST   | 创建分支 |
| `/api/v1/branches`     | GET    | 列出分支 |
| `/api/v1/branches/:id` | DELETE | 删除分支 |

### 定时任务

| 端点                      | 方法     | 描述     |
| ----------------------- | ------ | ------ |
| `/api/v1/cron`          | POST   | 创建定时任务 |
| `/api/v1/cron`          | GET    | 列出任务   |
| `/api/v1/cron/:id`      | DELETE | 删除任务   |
| `/api/v1/cron/:id/logs` | GET    | 获取执行日志 |

### 向量搜索

| 端点                            | 方法   | 描述     |
| ----------------------------- | ---- | ------ |
| `/api/v1/embeddings/generate` | POST | 生成嵌入向量 |
| `/api/v1/embeddings/tables`   | POST | 创建向量表  |
| `/api/v1/embeddings/insert`   | POST | 插入向量   |
| `/api/v1/embeddings/search`   | POST | 相似性搜索  |

### 备份恢复

| 端点                             | 方法     | 描述   |
| ------------------------------ | ------ | ---- |
| `/api/v1/backups`              | POST   | 创建备份 |
| `/api/v1/backups`              | GET    | 列出备份 |
| `/api/v1/backups/restore`      | POST   | 恢复备份 |
| `/api/v1/backups/:id`          | DELETE | 删除备份 |
| `/api/v1/backups/:id/download` | GET    | 下载备份 |

### 监控

| 端点                       | 方法  | 描述    |
| ------------------------ | --- | ----- |
| `/api/v1/monitor/health` | GET | 健康状态  |
| `/api/v1/monitor/stats`  | GET | 数据库统计 |
| `/api/v1/monitor/system` | GET | 系统信息  |

## CLI 工具

```bash
# 构建 CLI
go build -o oc-db9.exe ./cmd/oc-db9

# 创建数据库
./oc-db9.exe db create --name mydb

# 列出数据库
./oc-db9.exe db list

# 执行 SQL
./oc-db9.exe db sql --id <database-id> --query "SELECT * FROM users"

# 生成类型定义
./oc-db9.exe gen types --id <database-id> --lang typescript
./oc-db9.exe gen types --id <database-id> --lang python
./oc-db9.exe gen types --id <database-id> --lang go
```

## 配置

### 环境变量

| 变量                  | 描述               | 默认值                                                   |
| ------------------- | ---------------- | ----------------------------------------------------- |
| `DATABASE_URL`      | PostgreSQL 连接字符串 | `postgres://postgres:postgres@postgres:5432/postgres` |
| `MINIO_ENDPOINT`    | MinIO 端点         | `minio:9000`                                          |
| `MINIO_ACCESS_KEY`  | MinIO 访问密钥       | `minioadmin`                                          |
| `MINIO_SECRET_KEY`  | MinIO 密钥         | `minioadmin`                                          |
| `OLLAMA_BASE_URL`   | Ollama 服务地址      | `http://ollama:11434`                                 |
| `API_PORT`          | API 服务端口         | `8080`                                                |
| `POSTGRES_PASSWORD` | PostgreSQL 密码    | `postgres`                                            |

### Docker Compose 配置

编辑 `docker-compose.yml` 或创建 `.env` 文件：

```env
POSTGRES_PASSWORD=your_secure_password
MINIO_ROOT_USER=admin
MINIO_ROOT_PASSWORD=your_secret_key
```

## 开发指南

### 本地开发

```bash
# 安装依赖
cd api
go mod download

# 运行 API 服务
DATABASE_URL="postgres://postgres:postgres@localhost:5432/postgres" \
MINIO_ENDPOINT="localhost:9000" \
MINIO_ACCESS_KEY="minioadmin" \
MINIO_SECRET_KEY="minioadmin" \
go run ./cmd/api
```

### 构建 Docker 镜像

```bash
docker-compose build
```

## 与 db9.ai 对比

| 功能      | db9.ai     | OpenClaw-db9 |
| ------- | ---------- | ------------ |
| 即时数据库   | ✅          | ✅            |
| 内置嵌入    | ✅          | ✅ (Ollama)   |
| 文件存储    | ✅ fs9      | ✅ MinIO      |
| SQL 查询文件 | ✅          | ✅ **新增**   |
| HTTP 扩展 | ✅ pg_net  | ⚠️ 应用层       |
| 定时任务    | ✅ pg_cron | ✅ 应用层        |
| 数据库分支   | ✅ CoW      | ✅ 模板复制       |
| 向量搜索    | ✅          | ✅ pgvector   |
| 类型生成    | ✅          | ✅            |
| 自托管     | ❌          | ✅            |
| 开源      | 部分         | ✅ MIT        |

## 核心特性：SQL 查询文件

OpenClaw-db9 实现了 db9.ai 最核心的创新 - 用 SQL 直接查询文件：

```bash
# 1. 上传文件
$ oc-db9 fs cp ./users.csv :/data/users.csv --db mydb

# 2. 用 SQL 查询文件内容
$ oc-db9 db sql mydb --command "SELECT * FROM fs9('/data/users.csv')"

# 3. 复杂查询 - 去重并插入到正式表
$ oc-db9 db sql mydb --command "
  INSERT INTO users (name, age, city)
  SELECT DISTINCT 
    data->>'name',
    (data->>'age')::int,
    data->>'city'
  FROM fs9('/data/users.csv')
  WHERE data->>'name' IS NOT NULL
"
```

支持文件格式：
- CSV (自动类型推断)
- JSONL (动态列发现)
- Parquet (列式存储，高性能) ✅ 新增

**Parquet 优势**

Parquet 是列式存储格式，适合大数据场景：
- 查询速度快（只读取需要的列）
- 压缩率高（节省 50-90% 存储空间）
- 自带数据类型信息

```bash
# 查询 Parquet 文件（比 CSV 快 10-100 倍）
$ oc-db9 fs query /data/large_dataset.parquet --db mydb
```

## 文档

- [API 参考文档](docs/api-reference.md)
- [部署指南](docs/deployment-guide.md)
- [CLI 使用文档](docs/cli-reference.md)

## 许可证

MIT License

## 贡献

欢迎提交 Issue 和 Pull Request！

## 支持

- GitHub Issues: https://github.com/yihui504/alternative_db9/issues
