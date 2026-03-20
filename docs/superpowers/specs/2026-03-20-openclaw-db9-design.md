# OpenClaw-db9 设计文档

## 概述

OpenClaw-db9 (oc-db9) 是一个完全自托管的 db9.ai 替代品，为 AI Agent 提供完整的数据库基础设施。

### 核心目标

- 功能覆盖 db9.ai 90% 实用场景
- 完全开源、自托管、零外部依赖
- 单服务器即可部署，支持 Docker Compose 一键启动
- 提供与 db9 风格一致的 CLI 工具

## 架构设计

### 整体架构

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
│  + pg_net     │     │               │     │               │
└───────────────┘     └───────────────┘     └───────────────┘
```

### 技术选型对比

| 功能模块 | db9.ai 方案 | OpenClaw-db9 方案 | 理由 |
|--------|-------------|-------------------|------|
| 数据库核心 | 自研 Serverless PG | PostgreSQL 16 | 成熟稳定 |
| 向量存储 | 内置 pgvector | pgvector 扩展 | 标准扩展 |
| Embedding | 编译到二进制 | Ollama + pg_net | 避免 PostgresML 复杂安装 |
| 文件系统 | fs9 内置扩展 | MinIO + API 封装 | 成熟稳定，S3 兼容 |
| HTTP 请求 | 内置 http 扩展 | pg_net 扩展 | Supabase 官方扩展 |
| 定时任务 | pg_cron | pg_cron | 标准扩展 |
| 分支 | CoW 毫秒级 | pg_dump 秒级 | 简化实现 |
| 连接端口 | 5433 | 5432 | 标准端口 |

## 数据库 Schema

### 核心表结构

```sql
-- 账户表
CREATE TABLE oc_accounts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- 数据库实例表
CREATE TABLE oc_databases (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id UUID REFERENCES oc_accounts(id),
    name TEXT NOT NULL,
    postgres_db_name TEXT NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(account_id, name)
);

-- 文件元数据表
CREATE TABLE oc_files (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    database_id UUID REFERENCES oc_databases(id),
    path TEXT NOT NULL,
    bucket_path TEXT NOT NULL,
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
    schedule TEXT NOT NULL,
    sql_command TEXT NOT NULL,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- 分支记录表
CREATE TABLE oc_branches (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    database_id UUID REFERENCES oc_databases(id),
    name TEXT NOT NULL,
    source_branch TEXT,
    snapshot_path TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
```

## API 设计

### REST API 端点

| 方法 | 路径 | 描述 |
|------|------|------|
| POST | /api/v1/databases | 创建数据库 |
| GET | /api/v1/databases | 列出数据库 |
| GET | /api/v1/databases/:id | 获取数据库详情 |
| DELETE | /api/v1/databases/:id | 删除数据库 |
| POST | /api/v1/databases/:id/sql | 执行 SQL |
| GET | /api/v1/databases/:id/connect | 获取连接信息 |
| POST | /api/v1/files/upload | 上传文件 |
| GET | /api/v1/files | 列出文件 |
| GET | /api/v1/files/:id | 下载文件 |
| DELETE | /api/v1/files/:id | 删除文件 |

## CLI 命令设计

### 命令清单

```bash
# 账户管理
oc-db9 account create <name>
oc-db9 account list
oc-db9 account use <name>
oc-db9 account delete <name>

# 数据库操作
oc-db9 db create <name> [--from <source>]
oc-db9 db list
oc-db9 db delete <name>
oc-db9 db sql <name> [--file <file>] [--command <sql>]
oc-db9 db inspect <name>
oc-db9 db dump <name> [--output <file>]
oc-db9 db restore <name> --from <file>

# 文件系统
oc-db9 fs cp <local-path> <db-path> [--db <name>]
oc-db9 fs ls <path> [--db <name>]
oc-db9 fs rm <path> [--db <name>]
oc-db9 fs cat <path> [--db <name>]

# 定时任务
oc-db9 db cron list [--db <name>]
oc-db9 db cron create <name> --schedule "*/5 * * * *" --command "SELECT ..."
oc-db9 db cron delete <name> [--db <name>]

# 分支管理
oc-db9 branch create <name> [--from <source>] [--db <name>]
oc-db9 branch list [--db <name>]
oc-db9 branch switch <name> [--db <name>]
oc-db9 branch delete <name> [--db <name>]

# 类型生成
oc-db9 gen types --db <name> --lang typescript --output ./types.ts
```

## 与 db9.ai 的差异

| 功能 | db9.ai | OpenClaw-db9 |
|------|--------|--------------|
| 分支速度 | 毫秒级（CoW）| 秒级（pg_dump）|
| 文件事务 | 与 SQL 同一事务 | 应用层两阶段提交 |
| 托管方式 | SaaS | 完全自托管 |
| 成本 | 按量付费 | 服务器成本 |
| Embedding | 内置 | Ollama 外部服务 |

## 实现计划

### Phase 1: 核心基础设施

- Docker Compose 部署栈
- 数据库 Schema 初始化
- API Gateway 骨架

### Phase 2: CLI 工具

- Go CLI 框架搭建
- 基础命令实现

### Phase 3: 高级功能

- Embedding 集成
- 分支管理
- 类型生成器
