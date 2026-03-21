# OpenClaw-db9 v4 实施计划

**基于：** v4 优化方案（OpenClaw 验收 v1.0 后改进建议）
**创建日期：** 2026-03-21
**目标版本：** v4（代号：agent-ready）

---

## 一、计划概述

本计划将 v4 优化方案的 13 个任务分解为具体的代码修改，分为 4 个批次执行：

| 批次 | 任务 | 优先级 | 核心内容 | 预估工作量 |
|------|------|--------|----------|------------|
| 第一批 | T1 + T5 + T6 | P0/P1 | 超时保护 + 连接池生命周期 + Embedding 降级 | 1-2 天 |
| 第二批 | T2 + T3 + T4 | P0/P1 | 参数化查询 + 事务 API + 分页 | 3-5 天 |
| 第三批 | T7 + T10 + T9 | P1/P2 | 分支 API + 监控 + 跨平台 | 2-3 天 |
| 第四批 | T8 + T11 + T12 + T13 | P2/P3 | Dashboard + 配额 + 文档 + Webhook | 5-7 天 |

---

## 二、第一批详细设计（T1 + T5 + T6）

### T1: SQL 执行超时保护

**文件修改：** `api/internal/handlers/database.go`

**新增配置项（config.go）：**
```go
type Config struct {
    // ... 现有字段
    QueryTimeout int // SQL 执行超时（秒），默认 30
    DDLTimeout   int // DDL 执行超时（秒），默认 120
}
```

**ExecuteSQL handler 修改点：**
1. 从 context 中提取或创建带超时的 context
2. 检查请求体中是否有 `timeout` 字段覆盖默认配置
3. DDL 语句（CREATE/ALTER/DROP）使用更长的超时
4. 超时时返回 HTTP 408 + `{"error": "Query timed out after 30s"}`

**新增请求结构：**
```go
type ExecuteSQLRequest struct {
    SQL     string `json:"sql" binding:"required"`
    Timeout int    `json:"timeout"` // 可选，覆盖默认超时（秒）
}
```

**超时后清理：** 使用 `pg_terminate_backend` 清理可能残留的连接

---

### T5: 连接池生命周期管理

**文件修改：** `api/internal/handlers/database.go`

**cachedPool 结构扩展：**
```go
type cachedPool struct {
    pool      *pgxpool.Pool
    createdAt time.Time
    lastUsed  time.Time
}
```

**新增配置项（config.go）：**
```go
type Config struct {
    // ... 现有字段
    PoolTTL     time.Duration // 连接池最大存活时间，默认 30 分钟
    PoolIdleTTL time.Duration // 空闲连接池回收时间，默认 10 分钟
    MaxPools    int            // 最大缓存连接池数，默认 50
}
```

**getCachedPool 修改逻辑：**
1. 更新 `lastUsed` 时间戳
2. 检查 `poolTTL`，过期则重建连接池
3. 检查缓存数量，超限则 LRU 淘汰最旧的连接池

**新增清理协程：**
```go
func startPoolCleaner() {
    ticker := time.NewTicker(5 * time.Minute)
    for range ticker.C {
        // 遍历所有缓存的连接池
        // 如果 idle > PoolIdleTTL 或 age > PoolTTL，关闭并删除
    }
}
```

**DropDatabase 修改：** 在删除数据库后调用 `closeCachedPool(dbName)`

**启动时调用：** 在 `main.go` 中启动 `startPoolCleaner()` 协程

---

### T6: Embedding 服务优雅降级

**文件修改：** `api/internal/handlers/embedding.go`

**新增函数（可复用）：**
```go
func checkOllamaAvailable() (bool, string) {
    client := &http.Client{Timeout: 3 * time.Second}
    resp, err := client.Get(ollamaEndpoint + "/api/tags")
    if err != nil {
        return false, err.Error()
    }
    resp.Body.Close()
    return true, ""
}
```

**GenerateEmbedding 修改点：**
1. 调用 `checkOllamaAvailable()` 前置检查
2. 如果不可达，返回友好的错误信息
3. 错误信息包含启动 Ollama 的命令提示

**InsertVector / SimilaritySearch 修改：** 同样添加前置检查

**main.go 启动时检查：**
```go
if available, err := handlers.CheckOllamaAvailable(); !available {
    log.Printf("Warning: Ollama is not available: %v. Embedding features will not work.", err)
} else {
    log.Println("Ollama is available")
}
```

---

## 三、第二批详细设计（T2 + T3 + T4）

### T2: 参数化查询 / 预编译语句支持

**新增文件：** `api/internal/handlers/query.go`

**新增 API 端点：**
```
POST /api/v1/databases/:id/query
```

**新增请求结构：**
```go
type ParameterizedQueryRequest struct {
    SQL        string        `json:"sql" binding:"required"`
    Params     []interface{} `json:"params"`
    ParamTypes []string      `json:"param_types"` // 可选，如 ["text", "jsonb"]
    Timeout    int           `json:"timeout"`
    Limit      int           `json:"limit"`
    Offset     int           `json:"offset"`
}
```

**实现要点：**
1. 使用 `pool.Query(ctx, sql, params...)` 原生参数化
2. 支持 `param_types` 指定复杂类型（JSONB、UUID、数组）
3. 复用 T1 的超时保护机制
4. 复用 T4 的分页逻辑

**main.go 路由添加：**
```go
databases.POST("/:id/query", handlers.ParameterizedQuery)
```

---

### T3: 事务 API

**新增文件：** `api/internal/handlers/transaction.go`

**事务存储结构：**
```go
type Transaction struct {
    ID        string
    Tx        pgx.Tx
    Pool      *pgxpool.Pool
    CreatedAt time.Time
    ExpiresAt time.Time
}

var transactions sync.Map // map[string]*Transaction

const defaultTxTimeout = 60 * time.Second
```

**新增 API 端点：**
```
POST   /api/v1/databases/:id/transactions           — 开启事务
PUT    /api/v1/databases/:id/transactions/:tid/sql   — 在事务中执行 SQL
POST   /api/v1/databases/:id/transactions/:tid/commit — 提交
POST   /api/v1/databases/:id/transactions/:tid/rollback — 回滚
GET    /api/v1/databases/:id/transactions/:tid        — 获取事务状态
```

**开启事务响应：**
```go
type BeginTransactionResponse struct {
    TransactionID string    `json:"transaction_id"`
    ExpiresAt     time.Time `json:"expires_at"`
}
```

**事务中执行 SQL 请求：** 与 ExecuteSQLRequest 相同

**后台清理协程：**
```go
func startTransactionCleaner() {
    ticker := time.NewTicker(10 * time.Second)
    for range ticker.C {
        transactions.Range(func(key, value any) bool {
            tx := value.(*Transaction)
            if time.Now().After(tx.ExpiresAt) {
                tx.Tx.Rollback(context.Background())
                tx.Pool.Close()
                transactions.Delete(key)
            }
            return true
        })
    }
}
```

**main.go 路由添加：**
```go
transactions := v1.Group("/databases/:id/transactions")
{
    transactions.POST("", handlers.BeginTransaction)
    transactions.GET("/:tid", handlers.GetTransaction)
    transactions.PUT("/:tid/sql", handlers.ExecuteInTransaction)
    transactions.POST("/:tid/commit", handlers.CommitTransaction)
    transactions.POST("/:tid/rollback", handlers.RollbackTransaction)
}
```

---

### T4: 查询结果分页与行数限制

**文件修改：** `api/internal/handlers/database.go`

**新增配置项（config.go）：**
```go
type Config struct {
    // ... 现有字段
    MaxRows int // 最大返回行数，默认 1000
}
```

**ExecuteSQLRequest 扩展：**
```go
type ExecuteSQLRequest struct {
    SQL     string `json:"sql" binding:"required"`
    Timeout int    `json:"timeout"`
    Limit   int    `json:"limit"`
    Offset  int    `json:"offset"`
}
```

**ExecuteSQL 修改逻辑：**
1. 查询执行后，检查结果集行数
2. 如果超过 `MaxRows`，截断结果并设置 `truncated: true`
3. 响应中增加元数据：
```go
type SQLResponse struct {
    Results        []map[string]interface{} `json:"results"`
    Truncated      bool                     `json:"truncated"`
    TotalRows      int                      `json:"total_rows"`
    OriginalLimit  int                      `json:"original_limit,omitempty"`
    OriginalOffset int                      `json:"original_offset,omitempty"`
}
```

**applyPagination 包装逻辑：**
```go
func applyPagination(sql string, limit, offset int) string {
    if limit > 0 {
        return fmt.Sprintf("SELECT * FROM (%s) AS _paginated LIMIT %d OFFSET %d", sql, limit, offset)
    }
    return sql
}
```

---

## 四、第三批详细设计（T7 + T10 + T9）

### T7: 分支列表 API 改进

**文件修改：** `api/internal/handlers/branch.go`

**ListBranches 修改：**
1. `database_id` 从必需改为可选
2. 可选参数时，列出所有分支
3. 响应增加元数据：

```go
type ListBranchesResponse struct {
    Branches   []Branch `json:"branches"`
    Total      int      `json:"total"`
    DatabaseID string   `json:"database_id,omitempty"`
}
```

**main.go 路由保持不变**，行为自动变化

---

### T10: 健康检查与监控增强

**文件修改：** `api/internal/handlers/health.go`

**新增响应结构：**
```go
type HealthResponse struct {
    Status        string            `json:"status"`
    Version       string            `json:"version"`
    UptimeSeconds int64             `json:"uptime_seconds"`
    Components    map[string]string `json:"components"`
    Stats         *Stats           `json:"stats,omitempty"`
}

type Stats struct {
    DatabasesCount   int `json:"databases_count"`
    BranchesCount     int `json:"branches_count"`
    ActiveConnections int `json:"active_connections"`
    TotalRequests     int `json:"total_requests"`
}
```

**GET /health 增强：**
1. 检查 PostgreSQL 连接
2. 检查 MinIO 连接
3. 检查 Ollama 可用性（可选）
4. 返回各组件状态

**新增监控端点：**
```
GET /api/v1/monitor/stats        — 全局统计
GET /api/v1/monitor/db/:id        — 单数据库统计（大小、表数、行数估计）
```

---

### T9: 跨平台安装支持

**文件修改：** `install.ps1`（已存在，检查完整性）

**需要检查/补充的功能：**
1. 下载 GitHub Releases 的 Windows 二进制
2. 解压到指定目录
3. 检查 Docker 和 Docker Compose 是否安装
4. 启动 docker compose

**建议的 Release 打包：**
```
oc-db9-windows-amd64.zip
├── oc-db9.exe
├── docker-compose.yml
└── README_Windows.md
```

---

## 五、第四批详细设计（T8 + T11 + T12 + T13）

### T8: Web 控制台 / Dashboard

**新增文件：**
```
api/internal/dashboard/
    dashboard.go      — Dashboard 路由和嵌入
    static/
        index.html     — SPA 主页面
        style.css      — 样式
        app.js         — 前端逻辑
```

**Dashboard 功能：**
1. 数据库列表和 CRUD
2. SQL 查询界面（代码编辑器 + 结果表格）
3. 分支管理
4. 监控信息展示

**嵌入方式：**
```go
import "embed"

//go:embed static/*
var staticFS embed.FS

func (h *DashboardHandler) Register(r *gin.Engine) {
    r.GET("/", func(c *gin.Context) {
        c.FileFromFS("/static/index.html", staticFS)
    })
    r.StaticFS("/static", staticFS)
}
```

---

### T11: 数据库配额与资源限制

**新增配置项（config.go）：**
```go
type Config struct {
    // ... 现有字段
    MaxDatabases    int // 最大数据库数量，默认 100
    MaxDBSizeMB     int // 单数据库最大磁盘占用（MB），默认 500
    MaxQueryRows    int // 单次查询最大返回行数，默认 1000
    QueryTimeout    int // SQL 执行超时（秒），默认 30
}
```

**CreateDatabase 增加检查：**
```go
func CreateDatabase(c *gin.Context) {
    // 1. 检查全局数据库数量限制
    count := getDatabaseCount()
    if count >= cfg.MaxDatabases {
        c.JSON(http.StatusForbidden, gin.H{"error": "Maximum database limit reached"})
        return
    }
    // ... 现有逻辑
}
```

**定时检查数据库大小：**
```go
func checkDatabaseSizes() {
    // 查询 pg_database_size() 检查每个数据库大小
    // 如果超过 MaxDBSizeMB，记录警告或拒绝写入
}
```

---

### T12: OpenAPI / Swagger 文档自动生成

**新增依赖：**
```go
import _ "github.com/swaggo/swag/cmd/swag"
```

**注释规范（database.go 示例）：**
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
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/databases/{database_id}/sql [post]
func ExecuteSQL(c *gin.Context) { ... }
```

**生成命令：**
```bash
swag init -g api/cmd/api/main.go -o api/docs
```

**访问：** `GET /swagger/index.html`

---

### T13: Webhook / 事件通知

**新增文件：** `api/internal/handlers/webhook.go`

**配置结构：**
```go
type WebhookConfig struct {
    URL    string
    Events []string
}

var globalWebhooks []WebhookConfig
```

**支持的事件：**
- `database.created`
- `database.deleted`
- `branch.created`
- `branch.deleted`
- `query.error`
- `pool.exhausted`

**事件发送：**
```go
func sendWebhook(event string, data interface{}) {
    for _, wh := range globalWebhooks {
        if contains(wh.Events, event) {
            go func(url string) {
                body, _ := json.Marshal(map[string]interface{}{
                    "event":     event,
                    "timestamp": time.Now(),
                    "data":      data,
                })
                http.Post(url, "application/json", bytes.NewBuffer(body))
            }(wh.URL)
        }
    }
}
```

**API 端点：**
```
POST /api/v1/webhooks           — 注册 webhook
GET  /api/v1/webhooks           — 列出已注册的 webhooks
DELETE /api/v1/webhooks/:id     — 删除 webhook
```

---

## 六、测试策略

### 单元测试
- 每个 handler 的独立逻辑（分页、超时、参数化）
- 连接池缓存的 LRU 淘汰逻辑
- 事务超时的自动回滚

### 集成测试
- SQL 超时后连接是否正确释放
- 事务超时后是否自动 ROLLBACK
- 连接池在数据库删除后是否正确清理

### 回归测试
- 使用之前的评估用例确保不破坏已有功能
- T1 超时可能影响长事务场景
- T5 连接池回收可能影响正在使用的连接

---

## 七、文件修改清单

| 批次 | 文件 | 修改类型 |
|------|------|----------|
| 1 | `api/internal/config/config.go` | 修改 |
| 1 | `api/internal/handlers/database.go` | 修改 |
| 1 | `api/internal/handlers/embedding.go` | 修改 |
| 1 | `api/cmd/api/main.go` | 修改 |
| 2 | `api/internal/handlers/query.go` | 新增 |
| 2 | `api/internal/handlers/transaction.go` | 新增 |
| 2 | `api/internal/handlers/database.go` | 修改 |
| 3 | `api/internal/handlers/branch.go` | 修改 |
| 3 | `api/internal/handlers/health.go` | 修改 |
| 3 | `install.ps1` | 修改 |
| 4 | `api/internal/dashboard/dashboard.go` | 新增 |
| 4 | `api/internal/dashboard/static/*` | 新增 |
| 4 | `api/internal/handlers/webhook.go` | 新增 |

---

*计划版本：v4-implementation-plan | 创建于 2026-03-21*
