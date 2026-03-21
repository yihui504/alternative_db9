# OpenClaw-db9 Skill

OpenClaw-db9 是一个面向 AI Agent 的数据库平台，融合了 PostgreSQL 和文件系统。

## 核心能力

1. **即时创建数据库** - 秒级创建独立 PostgreSQL 数据库
2. **SQL 查询文件** - 用 SQL 直接查询 CSV/JSONL/Parquet 文件
3. **向量搜索** - 内置 embedding 和相似度搜索
4. **文件存储** - 上传下载文件，与 SQL 数据共存

## 快速开始

### 1. 安装

```bash
# 克隆仓库
git clone https://github.com/yihui504/alternative_db9.git
cd alternative_db9

# 启动服务
docker-compose up -d

# 构建 CLI
go build -o oc-db9 ./internal/cmd
```

### 2. 创建数据库

```bash
# 创建数据库
./oc-db9 db create myapp

# 获取连接信息
./oc-db9 db connect myapp
```

### 3. 上传和查询文件

```bash
# 上传 CSV 文件
./oc-db9 fs cp ./users.csv :/data/users.csv --db myapp

# 查询文件内容（像查表一样！）
./oc-db9 fs query /data/users.csv --db myapp

# 用 SQL 查询
./oc-db9 db sql myapp -q "SELECT * FROM fs9('/data/users.csv')"
```

### 4. 导入到正式表

```bash
# 创建表
./oc-db9 db sql myapp -q "CREATE TABLE users (id SERIAL PRIMARY KEY, name TEXT, age INT)"

# 从文件导入数据
./oc-db9 db sql myapp -q "
  INSERT INTO users (name, age)
  SELECT data->>'name', (data->>'age')::int
  FROM fs9('/data/users.csv')
"
```

## 支持的文件格式

| 格式 | 特点 | 适用场景 |
|------|------|---------|
| CSV | 自动类型推断 | 小数据、简单交换 |
| JSONL | 动态列发现 | 日志、事件流 |
| Parquet | 列式存储，高性能 | 大数据、分析查询 |

## 向量搜索

```bash
# 创建带向量列的表
./oc-db9 db sql myapp -q "
  CREATE TABLE docs (
    id SERIAL PRIMARY KEY,
    content TEXT,
    vec vector(768)
  )
"

# 生成 embedding
./oc-db9 db sql myapp -q "
  UPDATE docs SET vec = embedding(content)
  WHERE vec IS NULL
"

# 相似度搜索
./oc-db9 db sql myapp -q "
  SELECT content
  FROM docs
  ORDER BY vec <-> embedding('search query')
  LIMIT 5
"
```

## 常用命令

```bash
# 数据库管理
./oc-db9 db create <name>          # 创建数据库
./oc-db9 db list                   # 列出数据库
./oc-db9 db sql <name> -q "..."    # 执行 SQL
./oc-db9 db connect <name>         # 获取连接信息

# 文件管理
./oc-db9 fs cp <local> :<remote> --db <id>   # 上传文件
./oc-db9 fs ls --db <id>                      # 列出文件
./oc-db9 fs query <path> --db <id>           # 查询文件内容
./oc-db9 fs cat <path> --db <id>             # 查看文件内容

# 类型生成
./oc-db9 gen types <name>          # 生成 TypeScript 类型
```

## 最佳实践

1. **大数据用 Parquet** - 比 CSV 快 10-100 倍，节省 90% 空间
2. **先用 fs query 预览** - 再决定如何导入正式表
3. **利用向量搜索** - 自动 embedding，无需外部服务
4. **类型安全** - 使用生成的类型避免运行时错误

## 故障排除

```bash
# 检查服务状态
docker-compose ps

# 查看日志
docker-compose logs -f api

# 重置数据库
docker-compose down -v
docker-compose up -d
```

## 与 db9.ai 对比

| 功能 | OpenClaw-db9 | db9.ai |
|------|-------------|--------|
| 自托管 | ✅ | ❌ |
| 开源 | ✅ MIT | 部分 |
| SQL 查询文件 | ✅ | ✅ |
| Parquet 支持 | ✅ | ✅ |
| 向量搜索 | ✅ | ✅ |
| 云端托管 | ❌ | ✅ |

## 示例工作流

### 场景：数据清洗管道

```bash
# 1. 上传原始数据
./oc-db9 fs cp ./raw_data.csv :/raw/data.csv --db pipeline

# 2. 预览数据结构
./oc-db9 fs query /raw/data.csv --db pipeline

# 3. 清洗并导入正式表
./oc-db9 db sql pipeline -q "
  CREATE TABLE clean_data AS
  SELECT 
    data->>'user_id' as user_id,
    (data->>'amount')::decimal as amount,
    data->>'status' as status
  FROM fs9('/raw/data.csv')
  WHERE data->>'status' = 'completed'
    AND data->>'amount' IS NOT NULL
"

# 4. 生成类型定义
./oc-db9 gen types pipeline
```

---

**提示**：如果遇到问题，查看 `docker-compose logs` 获取详细信息。
