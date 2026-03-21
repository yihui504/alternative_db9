# OpenClaw-db9 v4 优化方案

**基于：** v1-v3 四轮评估（35+9+11 项测试，发现并修复 8 个 Bug）  
**目标版本：** v4（代号建议：agent-ready）  
**核心思路：** 从"能用"到"好用"，补齐生产环境必备的安全性和可靠性基础设施

---

## 一、优先级总览

| 优先级 | 编号 | 优化项 | 预估工作量 | 影响范围 |
|--------|------|--------|------------|----------|
| P0 | T1 | SQL 执行超时保护 | 小 | 安全性 |
| P0 | T2 | 参数化查询 / 预编译语句支持 | 中 | 安全性 |
| P0 | T3 | 事务 API | 中 | 功能完整性 |
| P1 | T4 | 查询结果分页与行数限制 | 中 | 性能与稳定性 |
| P1 | T5 | 数据库连接池生命周期管理 | 中 | 稳定性 |
| P1 | T6 | Embedding 服务优雅降级 | 小 | 用户体验 |
| P1 | T7 | 分支列表 API 改进 | 小 | API 易用性 |
| P2 | T8 | Web 控制台 / Dashboard | 大 | 用户体验 |
| P2 | T9 | 跨平台安装支持（Windows） | 中 | 安装体验 |
| P2 | T10 | 健康检查与监控增强 | 中 | 运维可观测性 |
| P2 | T11 | 数据库配额与资源限制 | 中 | 多租户安全 |
| P3 | T12 | OpenAPI / Swagger 文档自动生成 | 小 | 开发者体验 |
| P3 | T13 | Webhook / 事件通知 | 中 | 生态扩展 |

---

## 二、P0 — 必须实现（安全性基础）

### T1: SQL 执行超时保护

**问题：** 当前 ExecuteSQL 没有任何超时限制。一条缺少 WHERE 的 UPDATE/DELETE 或一个笛卡尔积 JOIN 可以无限运行，不仅阻塞该请求，还可能耗尽数据库资源，影响其他数据库的操作。

**方案：** 在 `database.go` 的 `ExecuteSQL` handler 中，为每个查询设置上下文超时。

```go
// 在 ExecuteSQL handler 中
ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
defer cancel()

// 将 ctx 传入所有 pgx 操作
rows, err := pool.Query(ctx, sql)
// ...
```

**实现要点：**

- 默认超时 30 秒，可通过环境变量 `OC_DB_QUERY_TIMEOUT` 配置
- DDL 语句（CREATE/ALTER/DROP）建议更长的超时或单独配置 `OC_DB_DDL_TIMEOUT`
- 超时返回 HTTP 408 + JSON 错误体：`{"error": "Query timed out after 30s"}`
- 使用 `pg_terminate_backend` 清理超时后可能残留的后端连接

**建议的 API 变更：** 在请求体中支持可选的 `timeout` 字段，允许单次请求覆盖默认值：

```json
{
  "database_id": "xxx",
  "sql": "SELECT ...",
  "timeout": 10
}
```

---

### T2: 参数化查询 / 预编译语句支持

**问题：** 当前 API 直接接受原始 SQL 字符串执行。对于 agent 自己构造 SQL 的场景，风险可控。但如果 oc-db9 的 API 暴露给前端应用或其他消费者，SQL 注入风险显著。

**方案：** 新增一个参数化查询端点，与现有 raw SQL 端点并行存在。

**新增 API：**

```
POST /api/v1/databases/:id/query
```

请求体：

```json
{
  "sql": "INSERT INTO users (name, email) VALUES ($1, $2) RETURNING id",
  "params": ["张三", "zhangsan@example.com"],
  "timeout": 10
}
```

响应体与现有 ExecuteSQL 一致。

**实现要点：**

- 使用 pgx 的 `pool.Query(ctx, sql, params...)` 原生参数化
- 参数类型自动推断（pgx 的 DefaultTypeCodec 处理 string/int/float/bool/null）
- 对于复杂类型（JSONB、数组、UUID），支持显式类型标注：

```json
{
  "sql": "INSERT INTO logs (level, metadata) VALUES ($1, $2)",
  "params": ["info", {"key": "value"}],
  "param_types": ["text", "jsonb"]
}
```

- 现有 raw SQL 端点保留不变，保持向后兼容
- 建议在文档中标注 raw SQL 端点为"仅供受信任的 agent 使用"

---

### T3: 事务 API

**问题：** 当前没有显式的事务控制。Agent 可以用多语句 `BEGIN; ...; COMMIT;` 变通，但存在几个问题：(1) 无法在多个 API 调用间维持事务；(2) 出错时需要手动 ROLLBACK，增加 agent 认知负担；(3) 长事务没有超时保护。

**方案：** 新增事务管理端点。

**新增 API：**

```
POST /api/v1/databases/:id/transactions    — 开启事务
PUT  /api/v1/databases/:id/transactions/:tid/sql  — 在事务中执行 SQL
POST /api/v1/databases/:id/transactions/:tid/commit — 提交
POST /api/v1/databases/:id/transactions/:tid/rollback — 回滚
```

**设计要点：**

- 开启事务返回 `transaction_id`（UUID）
- 事务存储在内存中的 `sync.Map`，key 为 transaction_id，value 为 `pgx.Tx`
- 事务自动超时（默认 60 秒），超时自动 ROLLBACK 并清理
- 每个事务绑定一个连接（从连接池取出后不归还，直到 COMMIT/ROLLBACK）
- 在事务中执行 SQL 的请求体与 ExecuteSQL 一致，结果格式也一致

**使用示例：**

```bash
# 1. 开启事务
curl -X POST /api/v1/databases/xxx/transactions
# {"transaction_id": "tid_001"}

# 2. 在事务中执行多条 SQL
curl -X PUT /api/v1/databases/xxx/transactions/tid_001/sql \
  -d '{"sql": "UPDATE accounts SET balance = balance - 100 WHERE id = 1"}'
curl -X PUT /api/v1/databases/xxx/transactions/tid_001/sql \
  -d '{"sql": "UPDATE accounts SET balance = balance + 100 WHERE id = 2"}'

# 3. 提交
curl -X POST /api/v1/databases/xxx/transactions/tid_001/commit
```

**实现细节：**

```go
type Transaction struct {
    ID        string
    Tx        pgx.Tx
    CreatedAt time.Time
    ExpiresAt time.Time
}

// 内存存储
var transactions sync.Map // map[string]*Transaction

// 后台 goroutine 定期清理过期事务（每 10 秒扫描一次）
func startTransactionCleaner() {
    ticker := time.NewTicker(10 * time.Second)
    for range ticker.C {
        transactions.Range(func(key, value any) bool {
            tx := value.(*Transaction)
            if time.Now().After(tx.ExpiresAt) {
                tx.Tx.Rollback(context.Background())
                transactions.Delete(key)
            }
            return true
        })
    }
}
```

---

## 三、P1 — 应该实现（可靠性与易用性）

### T4: 查询结果分页与行数限制

**问题：** 当前 ExecuteSQL 返回查询的全部行。对于大结果集（比如几十万行），API 响应会非常大，可能导致内存溢出或网络超时。

**方案：** 两层防护。

**第一层 — 服务端硬限制：**

- 环境变量 `OC_DB_MAX_ROWS` 配置最大返回行数，默认 1000
- 超过限制时截断结果，并在响应中附加元数据：

```json
{
  "results": [...],  // 前 1000 行
  "truncated": true,
  "total_rows_affected": 50000
}
```

**第二层 — 客户端分页支持：**

在请求体中支持 `limit` 和 `offset`：

```json
{
  "database_id": "xxx",
  "sql": "SELECT * FROM orders",
  "limit": 100,
  "offset": 200
}
```

实现方式：不是简单拼接 `LIMIT/OFFSET`（因为用户 SQL 可能已有），而是先包装用户 SQL 为子查询：

```go
func applyPagination(sql string, limit, offset int) string {
    // 包装为子查询，确保不影响用户 SQL 的语义
    if limit > 0 {
        sql = fmt.Sprintf("SELECT * FROM (%s) AS _paginated_subquery LIMIT %d OFFSET %d", sql, limit, offset)
    }
    return sql
}
```

对于 DML 和 DDL 语句，`limit`/`offset` 参数被忽略，保持现有行为。

---

### T5: 数据库连接池生命周期管理

**问题：** v3 使用 `sync.Map` 缓存连接池，解决了每次请求重建的问题。但缺少主动的连接池回收机制——数据库被删除后，其连接池仍在缓存中；长时间不用的数据库也占用连接资源。

**方案：** 为连接池缓存增加 TTL 和主动回收。

```go
type PoolEntry struct {
    Pool      *pgxpool.Pool
    CreatedAt time.Time
    LastUsed  time.Time
}

// 配置
const (
    poolTTL       = 30 * time.Minute  // 连接池最大存活时间
    poolIdleTTL   = 10 * time.Minute  // 空闲连接池回收时间
    maxPools      = 50                // 最大缓存连接池数
)

// getCachedPool 增加逻辑：
// 1. 更新 LastUsed
// 2. 检查 TTL，过期则重建
// 3. 检查缓存数量，超限则 LRU 淘汰

// DropDatabase 时主动从缓存移除并关闭连接池
```

**后台清理协程：**

```go
func startPoolCleaner() {
    ticker := time.NewTicker(5 * time.Minute)
    for range ticker.C {
        pools.Range(func(key, value any) bool {
            entry := value.(*PoolEntry)
            idle := time.Since(entry.LastUsed)
            age := time.Since(entry.CreatedAt)

            if idle > poolIdleTTL || age > poolTTL {
                entry.Pool.Close()
                pools.Delete(key)
                log.Printf("Closed idle pool for database: %s", key)
            }
            return true
        })
    }
}
```

**DropDatabase handler 中增加：**

```go
func (h *DatabaseHandler) DropDatabase(c *gin.Context) {
    // ... 现有删除逻辑 ...
    
    // 清理连接池缓存
    closeCachedPool(dbName)
    
    log.Printf("Database %s dropped and pool cleaned up", dbName)
}
```

---

### T6: Embedding 服务优雅降级

**问题：** Ollama 未运行时，embedding API 返回原始 DNS 解析错误：`lookup ollama on 127.0.0.1:53: no such host`，对用户毫无帮助。

**方案：** 在调用 Ollama 前做预检查，返回有意义的错误信息。

```go
func (h *EmbeddingHandler) GenerateEmbedding(c *gin.Context) {
    // 检查 Ollama 是否可达
    ollamaURL := os.Getenv("OLLAMA_BASE_URL")
    if ollamaURL == "" {
        ollamaURL = "http://localhost:11434"
    }
    
    client := &http.Client{Timeout: 3 * time.Second}
    resp, err := client.Get(ollamaURL + "/api/tags")
    if err != nil {
        c.JSON(http.StatusServiceUnavailable, gin.H{
            "error": fmt.Sprintf(
                "Embedding service (Ollama) is not available at %s. "+
                "Please start Ollama with: ollama serve",
                ollamaURL,
            ),
        })
        return
    }
    resp.Body.Close()
    
    // ... 现有 embedding 逻辑 ...
}
```

**额外改进：** 在应用启动时检查 Ollama 可用性，打印一条警告日志而非报错，让用户知道 embedding 功能的状态。

---

### T7: 分支列表 API 改进

**问题：** `GET /branches` 当前要求 `database_id` 作为必需参数。实际使用中，agent 有时需要列出所有数据库的所有分支。

**方案：** 改为可选查询参数。

```
GET /api/v1/branches              — 列出所有分支
GET /api/v1/branches?database_id=xxx  — 列出指定数据库的分支
```

**响应增加元数据：**

```json
{
  "branches": [...],
  "total": 15,
  "database_id": "xxx"  // 如果指定了筛选条件
}
```

---

## 四、P2 — 建议实现（提升完整度）

### T8: Web 控制台 / Dashboard

**价值：** oc-db9 目前是纯 API 服务，没有可视化界面。一个简单的 Web 控制台可以显著降低用户的上手门槛，也方便调试和验证。

**建议的最小可行功能：**

- 数据库列表（创建/删除）
- SQL 查询界面（带结果表格展示）
- 分支管理（创建/列表/删除）
- 基本的监控信息（请求数、数据库数、连接池状态）

**技术选型建议：** 不需要复杂的框架，一个单页 HTML + 原生 JavaScript 调用现有 API 即可。嵌入到 Go 二进制中，通过 `embed.FS` 提供。

```
GET /                  — 返回 Dashboard SPA
GET /api/...           — 现有 API 路由
```

---

### T9: 跨平台安装支持

**问题：** `install.sh` 仅支持 Linux/macOS。Windows 用户需要手动操作。

**方案：**

1. 新增 `install.ps1`（PowerShell 脚本），逻辑与 install.sh 对齐
2. 提供预编译的二进制发布（GitHub Releases），支持 Windows/macOS/Linux
3. 在 README 中增加 Windows 手动安装步骤

**install.ps1 核心逻辑：**

```powershell
$version = "v1.1.0"
$repo = "yihui504/alternative_db9"
$downloadUrl = "https://github.com/$repo/releases/download/$version/oc-db9-windows-amd64.zip"

Write-Host "Downloading oc-db9 $version..."
Invoke-WebRequest -Uri $downloadUrl -OutFile "oc-db9.zip"
Expand-Archive -Path "oc-db9.zip" -DestinationPath "oc-db9" -Force
Write-Host "Installed to ./oc-db9/"
Write-Host "Run: cd oc-db9; docker compose up -d"
```

---

### T10: 健康检查与监控增强

**现状：** `/health` 端点只返回固定文本。

**增强方案：**

```json
// GET /health
{
  "status": "healthy",
  "version": "v1.1.0",
  "uptime_seconds": 3600,
  "components": {
    "postgresql": "connected",
    "minio": "connected",
    "ollama": "disconnected"  // 可选组件
  },
  "stats": {
    "databases_count": 12,
    "active_connections": 5,
    "total_requests": 15000
  }
}
```

**新增监控端点：**

```
GET /api/v1/stats      — 详细统计信息
GET /api/v1/stats/db/:id  — 单数据库统计（大小、表数、行数估计）
```

---

### T11: 数据库配额与资源限制

**问题：** 当前没有任何资源限制。一个 agent 可以创建无限多的数据库，每个数据库可以存储无限量的数据。在多 agent 共享的环境中，这可能导致资源耗尽。

**方案：** 在全局和应用级别增加配额控制。

**配置项（环境变量或配置文件）：**

| 配置项 | 默认值 | 说明 |
|--------|--------|------|
| `OC_MAX_DATABASES` | 100 | 最大数据库数量 |
| `OC_MAX_DB_SIZE_MB` | 500 | 单数据库最大磁盘占用 |
| `OC_MAX_QUERY_ROWS` | 1000 | 单次查询最大返回行数 |
| `OC_QUERY_TIMEOUT` | 30 | SQL 执行超时（秒） |

**实现方式：** 在 CreateDatabase handler 中检查全局数据库数量限制。数据库大小检查通过定时任务查询 `pg_database_size()` 实现，超限时拒绝新的写入或发出警告。

---

## 五、P3 — 锦上添花

### T12: OpenAPI / Swagger 文档自动生成

**方案：** 使用 `swaggo/swag` 从代码注释自动生成 Swagger 文档。

```go
// @Summary 执行 SQL 查询
// @Description 在指定数据库中执行 SQL 语句
// @Tags Database
// @Accept json
// @Produce json
// @Param database_id path string true "数据库 ID"
// @Param request body ExecuteSQLRequest true "SQL 请求"
// @Success 200 {object} SQLResponse
// @Failure 400 {object} ErrorResponse
// @Router /api/v1/databases/{database_id}/sql [post]
func (h *DatabaseHandler) ExecuteSQL(c *gin.Context) { ... }
```

访问 `GET /swagger/index.html` 即可查看交互式 API 文档。

---

### T13: Webhook / 事件通知

**价值：** 当特定事件发生时（数据库创建/删除、SQL 执行错误、磁盘空间不足），主动通知外部系统。

**方案：** 在全局配置中增加 webhook URL 列表。

```
POST /api/v1/webhooks
{
  "url": "https://your-app.com/hooks/db-event",
  "events": ["database.created", "database.deleted", "query.error"]
}
```

**事件格式（POST JSON）：**

```json
{
  "event": "database.created",
  "timestamp": "2026-03-21T14:00:00Z",
  "data": {
    "database_id": "xxx",
    "database_name": "oc_xxxx"
  }
}
```

---

## 六、实施建议

**建议的开发顺序：**

1. **第一批（T1 + T5 + T6）**— 最小改动，最大收益。超时保护和连接池生命周期是基础设施改进，embedding 降级是用户体验修补。预计 1-2 天。

2. **第二批（T2 + T3 + T4）**— 核心功能增强。参数化查询和事务 API 是产品级功能，分页是稳定性保障。预计 3-5 天。

3. **第三批（T7 + T10 + T9）**— 易用性改进。分支 API 改进、监控增强、跨平台支持。预计 2-3 天。

4. **第四批（T8 + T11 + T12 + T13）**— 完整度提升。Dashboard、配额管理、文档、Webhook。预计 5-7 天。

**测试建议：**

每完成一批后，建议使用之前评估中的测试用例进行回归测试，确保新功能不破坏已有功能。特别关注：
- T1（超时）可能影响长事务和复杂查询场景
- T5（连接池回收）需要验证不会导致正在使用的连接被关闭
- T3（事务）需要测试超时自动回滚的正确性

---

## 附录：v3 已知但未修复的边界问题

以下问题在 v3 评估中发现，但 severity 较低，不阻塞生产使用：

1. **分支竞态（S11）：** 大量密集 SQL 操作后立即创建分支，偶现失败。实际场景中 agent 不会这样操作，非功能性缺陷。如果 T5（连接池生命周期管理）实现得当，这个问题可能自然消失。

2. **http 扩展安装：** pgvector 镜像可能不包含 pg_http 扩展，createDefaultExtensions 中的 http 扩展创建会失败并打印警告。建议在扩展创建失败时记录具体扩展名，方便排查。

---

*方案版本：v4-optimization-plan | 基于 oc-db9 v1.0.0 评估*
