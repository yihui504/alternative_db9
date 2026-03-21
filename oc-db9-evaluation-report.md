# OpenClaw-db9 全面评估报告

**评估时间：** 2026-03-21  
**测试环境：** Windows 11, Docker Desktop, PostgreSQL 16 (pgvector), MinIO  
**版本来源：** https://github.com/yihui504/alternative_db9 (main branch)

---

## 总体结论

OpenClaw-db9 的核心功能**基本可用**，适合作为 agent 的数据层来使用，但存在若干需要修复的 bug，在生产级场景下尚不成熟。

---

## 测试覆盖与结果汇总

| 测试维度 | 用例数 | 通过 | 失败 |
|----------|--------|------|------|
| 数据完整性 | 10 | 8 | 2 |
| 并发与隔离 | 5 | 5 | 0 |
| 分支功能 | 4 | 3 | 1 |
| 边界条件 | 10 | 8 | 2 |
| 向量搜索 | 6 | 5 | 1 |
| **合计** | **35** | **29 (83%)** | **6 (17%)** |

---

## 性能基准

- **单次 SQL 延迟：** 均值 ~6ms（100 次 `SELECT 1+1` 基准）
- **吞吐量（单线程）：** ~173 QPS
- **并发读（20 并发）：** 20/20 成功，均值 65ms，峰值 76ms
- **并发写（20 并发）：** 20/20 成功，200 行数据零丢失
- **结论：** 轻量场景性能完全够用，吞吐量受限于每次请求都新建 pgxpool 连接池（见 Bug #4）

---

## 发现的 Bug（按严重程度排序）

### Bug #1 — 约束违反错误被静默吞掉（严重）

**位置：** `api/internal/handlers/database.go` 第 202 行 `ExecuteSQL`

**现象：** 向 UNIQUE 约束列插入重复值，或向 FK 约束列插入不存在的父键，API 均返回 `HTTP 200 {"results": null}`，而不是报错。数据库约束本身**是生效的**（从 `pg_constraint` 可以确认），但错误被 Go 代码静默丢弃。

**根因：** pgx 的 `Query()` 对 DML 语句的执行错误放在 `rows.Err()` 中，而不是 `Query()` 的返回 `err`。代码只检查了 `err`，没有在循环后调用 `rows.Err()`。

**修复方案：**
```go
// 在 rows.Close() 之后，c.JSON() 之前，添加：
if err := rows.Err(); err != nil {
    c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
    return
}
```

**影响：** agent 无法感知约束违反，可能导致数据一致性问题。

---

### Bug #2 — 多语句 SQL 被拒绝（中等）

**现象：** 执行 `SELECT 'x'; DROP TABLE t; --` 这类分号分隔的多语句 SQL 时，API 返回：
```
ERROR: cannot insert multiple commands into a prepared statement (SQLSTATE 42601)
```

**说明：** pgx 使用 prepared statement 模式，PostgreSQL 的协议层禁止 prepared statement 中出现多条语句。这一限制**实际上有安全价值**（天然阻止了简单 SQL 注入），但会导致合法的批量 SQL 也被拒绝。

**修复方案：** 对 SQL 进行预处理，若包含多语句则改用 `Exec()` 而非 `Query()`；或在文档中明确说明此限制，让 agent 知道需要分批执行。

---

### Bug #3 — 分支 GET by ID 路由缺失（低）

**现象：** `GET /api/v1/branches/:id` 返回 404。

**根因：** `main.go` 第 82-87 行分支路由组只注册了 `POST /` 和 `GET /`（列表），以及 `DELETE /:id`，没有注册 `GET /:id`（详情）。

**修复方案：** 在 `main.go` 中添加：
```go
branches.GET("/:id", handlers.GetBranch)
```
并在 `branch.go` 中实现对应 handler。

---

### Bug #4 — ExecuteSQL 每次请求新建连接池（性能隐患）

**位置：** `database.go` 第 195 行

**现象：** 每个 SQL 请求都执行 `pgxpool.New()`，这意味着每次都会建立新的连接池，包含 TCP 握手和认证，显著增加延迟。

**修复方案：** 使用连接池缓存，按数据库名缓存 `*pgxpool.Pool` 实例，通过 `sync.Map` 管理生命周期。

---

### Bug #5 — 安装脚本 install.sh 不兼容 Windows（轻微）

**现象：** 提供的安装命令 `curl ... | sh` 在 Windows 上无法执行，需要手动用 Git Bash 或 WSL，且两者均因网络问题难以访问 raw.githubusercontent.com。

**建议：** 提供 PowerShell 安装脚本，或在文档中说明 Windows 需通过 ZIP 安装。

---

### Bug #6 — Ollama 服务未配置时缺少优雅降级（轻微）

**现象：** 在未启动 Ollama 的情况下调用 `POST /api/v1/embeddings/generate`，返回 DNS 解析错误的原始内部错误，而不是有意义的提示信息。

**建议：** 返回 `{"error": "Embedding service unavailable. Start with --profile ollama to enable."}` 这样的提示。

---

## 功能可用性评级

| 功能 | 可用性 | 备注 |
|------|--------|------|
| 创建/删除数据库 | 完全可用 | 秒级创建，隔离性完善 |
| SQL 执行（单语句） | 完全可用 | SELECT/INSERT/UPDATE/DELETE/DDL 均正常 |
| 多语句批量 SQL | 不可用 | 被 pgx prepared statement 限制阻断 |
| 约束（UNIQUE/FK） | 部分可用 | 约束本身有效，但 API 不抛出错误（Bug #1） |
| JSONB | 完全可用 | 查询、操作均正常 |
| 窗口函数 / CTE | 完全可用 | 测试通过 |
| 聚合函数 | 完全可用 | COUNT/SUM/AVG/MAX/MIN 全部正确 |
| 数据持久化 | 完全可用 | 重启后数据完整保留 |
| 并发读写 | 完全可用 | 20 并发零数据丢失 |
| 多数据库隔离 | 完全可用 | schema 级别隔离完善 |
| 文件上传/列表 | 完全可用 | MinIO 存储正常 |
| 数据库分支 | 基本可用 | 创建/隔离正常，缺 GET by ID 接口 |
| 向量搜索（pgvector） | 完全可用 | L2/余弦距离、HNSW/IVFFlat 索引全部工作 |
| Embedding 自动生成 | 需要 Ollama | 配置 ollama profile 后可用 |
| 备份/恢复 | API 可达 | 未深度测试 |
| 定时任务 | API 可达 | 未深度测试 |

---

## 与 db9.ai 的差异对比

| 特性 | db9.ai | OpenClaw-db9 |
|------|--------|--------------|
| 数据库分支 | Copy-on-Write（高效） | 模板复制（简单但 OK） |
| Embedding | 云端托管 | 本地 Ollama（可选） |
| 开源程度 | 部分开源 | 完全开源 |
| Windows 安装 | 不详 | 需手动操作 |
| 约束错误透出 | 应正常 | Bug，需修复 |

---

## 给开发团队的建议

**必须修复（影响正确性）：**
1. `rows.Err()` 未检查导致约束违反静默通过（Bug #1）
2. 明确多语句 SQL 的处理策略，或给出文档说明（Bug #2）

**应该修复（影响体验）：**
3. 补充 `GET /api/v1/branches/:id` 路由（Bug #3）
4. 改为连接池复用，避免每次请求重建（Bug #4）
5. Embedding 服务不可用时的友好错误提示（Bug #6）

**可选优化：**
- 提供 PowerShell/Windows 安装脚本
- 为 `ExecuteSQL` 添加超时控制（目前无超时）
- 考虑添加 SQL 语句分类（只读/写）以实现细粒度权限控制
