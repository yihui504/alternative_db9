# OpenClaw-db9：自托管 db9.ai 替代品开发计划

## 项目概述

本项目旨在构建一个完全自托管的 db9.ai 替代品，命名为 **OpenClaw-db9**（简称 oc-db9）。通过组合成熟的开源组件，实现 db9.ai 的核心功能，同时完全摆脱对官方服务的依赖，数据完全自主可控。

**核心目标**：
- 功能覆盖 db9.ai 90% 实用场景
- 完全开源、自托管、零外部依赖
- 单服务器即可部署，支持 Docker Compose 一键启动
- 提供与 db9 风格一致的 CLI 工具

---

## 技术架构选型

### 组件清单

| 功能模块 | 选型方案 | 替代 db9 功能 | 备注 |
|---------|---------|--------------|------|
| 数据库核心 | PostgreSQL 16 + Supabase | Serverless PostgreSQL | 自托管 Supabase 提供完整生态 |
| 向量存储 | pgvector | 向量 Embedding 存储 | 标准扩展 |
| Embedding 生成 | pgml (PostgresML) | 内置 `embedding()` 函数 | 本地运行模型，无需 API |
| HTTP 请求 | pg_net | SQL 发 HTTP 请求 | Supabase 官方扩展 |
| 定时任务 | pg_cron | `db9 db cron` | 标准扩展 |
| 文件存储 | MinIO | fs9 文件系统 | S3 兼容，本地部署 |
| 对象存储网关 | 自定义 Go 服务 | `db9 fs` CLI | 包装 MinIO API |
| CLI 工具 | Cobra (Go) | `db9` 命令行 | 完整命令集 |
| 类型生成 | 自定义脚本 | `db9 gen types` | 基于 SQL introspection |

### 架构图

```
┌─────────────────────────────────────────────────────────────┐
│                      oc-db9 CLI (Go)                        │
│  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐           │
│  │ create  │ │ db sql  │ │ fs cp   │ │ branch  │           │
│  │ list    │ │ inspect │ │ ls      │ │ create  │           │
│  │ delete  │ │ dump    │ │ mount   │ │ switch  │           │
│  └────┬────┘ └────┬────┘ └────┬────┘ └────┬────┘           │
└───────┼───────────┼───────────┼───────────┼─────────────────┘
        │           │           │           │
        ▼           ▼           ▼           ▼
┌─────────────────────────────────────────────────────────────┐
│                    API Gateway (Go)                         │
│         统一认证、路由、请求转发                              │
└─────────────────────────────────────────────────────────────┘
        │           │           │           │
        ▼           ▼           ▼           ▼
┌───────────┐ ┌───────────┐ ┌───────────┐ ┌───────────┐
│PostgreSQL │ │  pgml     │ │  pg_net   │ │  MinIO    │
│+ pgvector │ │(embedding)│ │(HTTP ext) │ │(文件存储) │
│+ pg_cron  │ │           │ │           │ │           │
└───────────┘ └───────────┘ └───────────┘ └───────────┘
```

---

## 功能模块详细设计

### Phase 1: 核心基础设施（Week 1）

#### 1.1 Docker Compose 部署栈

```yaml
# docker-compose.yml 核心服务
version: '3.8'
services:
  postgres:
    image: supabase/postgres:15.1.1.78
    environment:
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}
    volumes:
      - postgres_data:/var/lib/postgresql/data
      - ./init-scripts:/docker-entrypoint-initdb.d
    ports:
      - "5432:5432"
  
  minio:
    image: minio/minio:latest
    command: server /data --console-address ":9001"
    environment:
      MINIO_ROOT_USER: ${MINIO_ROOT_USER}
      MINIO_ROOT_PASSWORD: ${MINIO_ROOT_PASSWORD}
    volumes:
      - minio_data:/data
    ports:
      - "9000:9000"
      - "9001:9001"
  
  oc-db9-api:
    build: ./api
    environment:
      DATABASE_URL: postgres://postgres:${POSTGRES_PASSWORD}@postgres:5432/postgres
      MINIO_ENDPOINT: minio:9000
      MINIO_ACCESS_KEY: ${MINIO_ROOT_USER}
      MINIO_SECRET_KEY: ${MINIO_ROOT_PASSWORD}
    ports:
      - "8080:8080"
    depends_on:
      - postgres
      - minio
```

**初始化脚本** (`init-scripts/01-extensions.sql`):
```sql
-- 启用必要扩展
CREATE EXTENSION IF NOT EXISTS pgvector;
CREATE EXTENSION IF NOT EXISTS pg_net;
CREATE EXTENSION IF NOT EXISTS pg_cron;

-- pgml 需要额外安装，参考 PostgresML 文档
-- CREATE EXTENSION IF NOT EXISTS pgml;
```

#### 1.2 数据库 Schema 设计

```sql
-- 账户/项目隔离表
CREATE TABLE oc_accounts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- 数据库实例表（逻辑隔离）
CREATE TABLE oc_databases (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id UUID REFERENCES oc_accounts(id),
    name TEXT NOT NULL,
    postgres_db_name TEXT NOT NULL, -- 实际的 PG 数据库名
    created_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(account_id, name)
);

-- 文件元数据表（MinIO 元数据镜像）
CREATE TABLE oc_files (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    database_id UUID REFERENCES oc_databases(id),
    path TEXT NOT NULL, -- 虚拟路径，如 /documents/report.pdf
    bucket_path TEXT NOT NULL, -- 实际 MinIO 路径
    size BIGINT,
    checksum TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(database_id, path)
);

-- 定时任务表
CREATE TABLE oc_cron_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    database_id UUID REFERENCES oc_databases(id),
    name TEXT NOT NULL,
    schedule TEXT NOT NULL, -- cron 表达式
    sql_command TEXT NOT NULL,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- 分支记录表（模拟分支）
CREATE TABLE oc_branches (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    database_id UUID REFERENCES oc_databases(id),
    name TEXT NOT NULL,
    source_branch TEXT,
    snapshot_path TEXT, -- pg_dump 文件路径
    created_at TIMESTAMPTZ DEFAULT NOW()
);
```

### Phase 2: CLI 工具开发（Week 2-3）

#### 2.1 项目结构

```
oc-db9/
├── cmd/
│   └── oc-db9/
│       └── main.go
├── internal/
│   ├── cmd/           # Cobra 命令定义
│   │   ├── root.go
│   │   ├── account.go
│   │   ├── db.go
│   │   ├── fs.go
│   │   ├── cron.go
│   │   ├── branch.go
│   │   └── gen.go
│   ├── api/           # API 客户端
│   ├── config/        # 配置文件管理
│   └── utils/         # 工具函数
├── pkg/
│   └── types/         # 共享类型定义
├── api/               # API Gateway 服务（独立模块）
│   ├── main.go
│   ├── handlers/
│   └── middleware/
├── docker-compose.yml
├── Dockerfile
└── skill.md           # AI Agent 配置文档
```

#### 2.2 命令实现清单

**账户管理** (`oc-db9 account`):
```go
// oc-db9 account create <name>
// oc-db9 account list
// oc-db9 account use <name>  // 设置默认账户
// oc-db9 account delete <name>
```

**数据库操作** (`oc-db9 db`):
```go
// oc-db9 db create <name> [--from <source>]
// oc-db9 db list
// oc-db9 db delete <name>
// oc-db9 db sql <name> [--file <file>] [--command <sql>]
// oc-db9 db inspect <name>
// oc-db9 db dump <name> [--output <file>]
// oc-db9 db restore <name> --from <file>
```

**文件系统** (`oc-db9 fs`):
```go
// oc-db9 fs cp <local-path> <db-path> [--db <name>]
// oc-db9 fs cp <db-path> <local-path> [--db <name>]
// oc-db9 fs ls <path> [--db <name>]
// oc-db9 fs rm <path> [--db <name>]
// oc-db9 fs cat <path> [--db <name>]
// oc-db9 fs mount <db-path> <local-mount> [--db <name>]  // FUSE 挂载（可选）
```

**定时任务** (`oc-db9 db cron`):
```go
// oc-db9 db cron list [--db <name>]
// oc-db9 db cron create <name> --schedule "*/5 * * * *" --command "SELECT ..."
// oc-db9 db cron delete <name> [--db <name>]
// oc-db9 db cron logs <name> [--db <name>]
```

**分支管理** (`oc-db9 branch`):
```go
// oc-db9 branch create <name> [--from <source>] [--db <name>]
// oc-db9 branch list [--db <name>]
// oc-db9 branch switch <name> [--db <name>]
// oc-db9 branch delete <name> [--db <name>]
```

**类型生成** (`oc-db9 gen`):
```go
// oc-db9 gen types --db <name> --lang typescript --output ./types.ts
// oc-db9 gen types --db <name> --lang python --output ./models.py
// oc-db9 gen types --db <name> --lang go --output ./models.go
```

### Phase 3: Embedding 与向量功能（Week 3）

#### 3.1 PostgresML 集成

**安装 PostgresML**:
```bash
# 方法1：使用 PostgresML 官方 Docker 镜像
# 方法2：在现有 PG 上编译安装（较复杂）
# 推荐方法1，替换 docker-compose 中的 postgres 服务
```

**创建 embedding 函数**:
```sql
-- 包装 pgml.embed，提供与 db9 类似的接口
CREATE OR REPLACE FUNCTION embedding(
    input_text TEXT,
    model_name TEXT DEFAULT 'intfloat/multilingual-e5-base'
) RETURNS vector(768) AS $$
BEGIN
    RETURN pgml.embed(model_name, input_text)::vector;
END;
$$ LANGUAGE plpgsql;

-- 使用示例
SELECT embedding('这是一段中文文本');
```

**备选方案（无 GPU 环境）**:
```sql
-- 使用 Ollama + pg_net 实现
CREATE OR REPLACE FUNCTION embedding_via_ollama(
    input_text TEXT,
    model_name TEXT DEFAULT 'nomic-embed-text'
) RETURNS vector AS $$
DECLARE
    response JSONB;
BEGIN
    SELECT content::JSONB INTO response
    FROM net.http_post(
        url := 'http://ollama:11434/api/embeddings',
        body := jsonb_build_object(
            'model', model_name,
            'prompt', input_text
        )
    );
    RETURN (response->>'embedding')::vector;
END;
$$ LANGUAGE plpgsql;
```

#### 3.2 向量搜索示例

```sql
-- 创建带向量的表
CREATE TABLE documents (
    id SERIAL PRIMARY KEY,
    content TEXT,
    embedding vector(768)
);

-- 自动更新向量（可选触发器）
CREATE OR REPLACE FUNCTION update_document_embedding()
RETURNS TRIGGER AS $$
BEGIN
    NEW.embedding := embedding(NEW.content);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- 相似度搜索
SELECT content, embedding <=> embedding('查询文本') AS distance
FROM documents
ORDER BY distance
LIMIT 5;
```

### Phase 4: 分支机制实现（Week 4）

#### 4.1 简化版分支（基于 pg_dump）

```go
// internal/branch/manager.go

func (m *BranchManager) CreateBranch(dbName, branchName, sourceBranch string) error {
    // 1. 获取源数据库信息
    sourceDB := m.getDB(sourceBranch)
    
    // 2. 创建新的 PG 数据库
    newDBName := fmt.Sprintf("%s_%s", dbName, branchName)
    m.execSQL(fmt.Sprintf("CREATE DATABASE %s", newDBName))
    
    // 3. pg_dump 源数据库
    dumpFile := fmt.Sprintf("/backups/%s_%s.dump", dbName, time.Now().Unix())
    cmd := exec.Command("pg_dump", "-Fc", sourceDB, "-f", dumpFile)
    cmd.Run()
    
    // 4. pg_restore 到新数据库
    cmd = exec.Command("pg_restore", "-d", newDBName, dumpFile)
    cmd.Run()
    
    // 5. 克隆 MinIO bucket 内容
    m.cloneMinIOBucket(sourceBranch, branchName)
    
    // 6. 记录分支元数据
    m.recordBranch(dbName, branchName, sourceBranch, dumpFile)
    
    return nil
}
```

#### 4.2 分支切换机制

```go
func (m *BranchManager) SwitchBranch(dbName, branchName string) error {
    // 更新数据库连接配置，指向分支数据库
    // 实际实现：修改连接池配置或返回新的连接字符串
    branchDBName := fmt.Sprintf("%s_%s", dbName, branchName)
    return m.updateActiveBranch(dbName, branchDBName)
}
```

### Phase 5: 类型生成器（Week 4）

#### 5.1 SQL Introspection

```go
// internal/gen/introspect.go

type TableInfo struct {
    Name    string
    Columns []ColumnInfo
}

type ColumnInfo struct {
    Name     string
    DataType string
    Nullable bool
    IsArray  bool
}

func IntrospectDatabase(db *sql.DB) ([]TableInfo, error) {
    query := `
        SELECT 
            table_name,
            column_name,
            data_type,
            is_nullable = 'YES' as nullable
        FROM information_schema.columns
        WHERE table_schema = 'public'
        ORDER BY table_name, ordinal_position
    `
    // 执行查询并解析结果...
}
```

#### 5.2 TypeScript 生成器

```go
func GenerateTypeScript(tables []TableInfo) string {
    var buf bytes.Buffer
    buf.WriteString("// Auto-generated by oc-db9\n\n")
    
    for _, table := range tables {
        buf.WriteString(fmt.Sprintf("export interface %s {\n", PascalCase(table.Name)))
        for _, col := range table.Columns {
            tsType := mapPostgresToTypeScript(col.DataType, col.IsArray)
            optional := ""
            if col.Nullable {
                optional = "?"
            }
            buf.WriteString(fmt.Sprintf("  %s%s: %s;\n", col.Name, optional, tsType))
        }
        buf.WriteString("}\n\n")
    }
    
    return buf.String()
}
```

### Phase 6: Skill 文档（Week 5）

#### 6.1 skill.md 模板

```markdown
# OpenClaw-db9 Skill

## 概述

OpenClaw-db9 是一个完全自托管的 db9.ai 替代品，提供面向 AI Agent 的完整数据库基础设施。

## 安装

### 前置要求
- Docker & Docker Compose
- Go 1.21+（用于编译 CLI）

### 快速开始

```bash
# 1. 克隆仓库
git clone https://github.com/yourname/openclaw-db9.git
cd openclaw-db9

# 2. 启动服务
docker-compose up -d

# 3. 安装 CLI
cd cmd/oc-db9 && go install

# 4. 配置 CLI
oc-db9 config set-api http://localhost:8080
```

## 核心功能

### 创建数据库
```bash
oc-db9 db create my-agent-db
```

### 执行 SQL
```bash
oc-db9 db sql my-agent-db --command "SELECT * FROM users"
```

### 文件操作
```bash
oc-db9 fs cp ./document.pdf /docs/ --db my-agent-db
oc-db9 fs ls /docs/ --db my-agent-db
```

### Embedding 向量
```sql
-- 在 SQL 中直接使用
SELECT embedding('这段文本的向量表示');
```

### 定时任务
```bash
oc-db9 db cron create sync-job \
  --schedule "0 */6 * * *" \
  --command "SELECT sync_data()"
```

### 分支管理
```bash
oc-db9 branch create feature-x --db my-agent-db
oc-db9 branch switch feature-x --db my-agent-db
```

## 类型生成

```bash
# 生成 TypeScript 类型
oc-db9 gen types --db my-agent-db --lang typescript --output ./types.ts
```

## 与 db9.ai 的差异

| 功能 | db9.ai | OpenClaw-db9 |
|------|--------|--------------|
| 分支速度 | 毫秒级（CoW）| 秒级（pg_dump）|
| 文件事务 | 与 SQL 同一事务 | 应用层两阶段提交 |
| 托管方式 | SaaS | 完全自托管 |
| 成本 | 按量付费 | 服务器成本 |
```

---

## 开发里程碑

### Milestone 1: MVP（Week 1-2）
- [ ] Docker Compose 部署栈
- [ ] 基础 CLI（create, list, delete, sql）
- [ ] MinIO 文件存储集成
- [ ] 基础 API Gateway

### Milestone 2: 核心功能（Week 3-4）
- [ ] pgvector + pgml 集成
- [ ] pg_cron 定时任务
- [ ] 分支管理（pg_dump 版）
- [ ] 完整 fs 命令集

### Milestone 3: 高级功能（Week 5-6）
- [ ] 类型生成器（TS/Python/Go）
- [ ] 监控与日志
- [ ] 备份与恢复
- [ ] 多账户隔离

### Milestone 4: 优化与文档（Week 7-8）
- [ ] 性能优化
- [ ] 完整测试覆盖
- [ ] 详细文档
- [ ] skill.md 完善

---

## 技术风险与缓解

| 风险 | 影响 | 缓解措施 |
|------|------|----------|
| PostgresML 安装复杂 | 高 | 提供预构建 Docker 镜像；提供 Ollama 备选方案 |
| pg_dump 分支速度慢 | 中 | 文档明确说明限制；后续可集成 Neon 存储引擎 |
| 文件与 SQL 非原子事务 | 中 | 应用层实现两阶段提交；文档说明使用模式 |
| MinIO 单点故障 | 低 | 文档说明生产环境应使用分布式 MinIO |

---

## 与 Trae 协作建议

使用 Trae 开发时，建议按以下顺序实现：

1. **先实现 API Gateway**（`api/` 目录）
   - 定义好 REST API 规范
   - 使用 Trae 的代码生成功能快速搭建 handlers

2. **并行开发 CLI 和数据库初始化**
   - CLI 使用 Cobra 框架，结构清晰
   - 数据库初始化脚本一次性写好

3. **Embedding 功能最后实现**
   - 先使用 mock 函数占位
   - 等核心功能稳定后再集成 pgml

4. **测试驱动**
   - 每个命令都写对应的集成测试
   - 使用 `docker-compose -f docker-compose.test.yml` 做测试环境

---

## 参考资源

- [db9.ai 官方文档](https://db9.ai/docs)
- [db9.ai skill.md](https://db9.ai/skill.md)
- [Supabase 自托管指南](https://supabase.com/docs/guides/self-hosting)
- [PostgresML 文档](https://postgresml.org/docs/)
- [pgvector 文档](https://github.com/pgvector/pgvector)
- [MinIO 文档](https://min.io/docs/)
