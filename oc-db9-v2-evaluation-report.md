# OpenClaw-db9 v2 回归评估报告

**评估时间：** 2026-03-21（第二轮）  
**测试环境：** Windows 11, PostgreSQL 16 (pgvector), MinIO, Go 编译本地运行  
**版本来源：** https://github.com/yihui504/alternative_db9 (最新 main)

---

## 总体结论

v2 修复了上一轮发现的 4 个关键 bug 中的 3 个，核心 SQL 执行引擎的可靠性和性能均有显著提升。作为 AI agent 的数据层，已经具备实用价值。唯一遗留的严重问题是分支创建因连接池缓存而失败，以及一个新引入的问题。整体评价从"基本可用"提升到"生产就绪前的最后一个里程碑"。

---

## v1 -> v2 Bug 修复验证

### Bug #1 — 约束违反错误被静默吞掉 -> 已修复

v1 中 UNIQUE/FK 约束违反返回 HTTP 200，agent 无法感知数据错误。v2 在 `database.go` 第 256 行添加了 `rows.Err()` 检查，现在约束违反会正确返回 HTTP 500 和具体错误信息。

**回归测试结果：**
- UNIQUE 约束：重复插入被拒绝（PASS）
- FK 约束：不存在的父键被拒绝（PASS）

### Bug #2 — 多语句 SQL 被拒绝 -> 已修复

v2 新增了 `containsMultipleStatements()` 函数（database.go 第 214-231 行），对分号分隔的多语句使用 `Exec()` 而非 `Query()`。

**回归测试结果：**
- 简单两语句（CREATE TABLE + INSERT）：PASS
- 带引号的语句：PASS
- 带类型转换的语句（JSONB）：PASS
- 5 条语句一次执行（4 表 + 2 索引）：PASS

### Bug #3 — GET /branches/:id 路由缺失 -> 已修复

v2 在 `main.go` 第 86 行注册了 `branches.GET("/:id", handlers.GetBranch)`，`branch.go` 中已有完整实现（第 132-145 行）。

**回归测试状态：** 路由已注册且 handler 存在，但因分支创建失败（新 Bug #7）未能通过端到端验证。代码审查确认逻辑正确。

### Bug #4 — 每次请求重建连接池 -> 已修复

v2 新增了 `getCachedPool()` 函数（database.go 第 264+ 行），使用 `sync.Map` 缓存连接池实例。

**性能对比：**

| 指标 | v1 | v2 | 变化 |
|------|----|----|------|
| 单次 SELECT 均值 | ~6ms | ~3ms | 快 50% |
| 100 次 SELECT 总耗时 | ~578ms | ~323ms | 快 44% |
| QPS（单线程） | ~173 | ~310 | +79% |
| 50 次请求延迟稳定性 | 有抖动 | avg=4ms, min=2ms, max=11ms | 稳定 |

连接池缓存带来的性能提升非常显著，QPS 接近翻倍。

---

## Agent 实战模拟测试结果

模拟了一个 AI agent 使用 oc-db9 作为工作数据存储的典型场景，共 9 个场景：

| 场景 | 结果 | 说明 |
|------|------|------|
| S1 批量插入任务 | PASS | 多行 VALUES 一次插入 |
| S2 JSONB 深度查询 | PASS | `metadata->>'source'` 路径查询 |
| S3 更新任务状态 | PASS | UPDATE + 后续 SELECT 验证 |
| S4 聚合分析 | PASS | GROUP BY + COUNT + AVG |
| S5 CTE 多步分析 | PASS | WITH ... SELECT FROM subquery |
| S6 窗口函数 | PASS | ROW_NUMBER() OVER() |
| S7 工具结果存储 | PASS | JSONB 存储工具输入/输出 |
| S8 NOT NULL 约束感知 | PASS | 约束违反正确报错 |
| S10 并发写入 | PASS | 10 并发写入零数据丢失 |

**通过率：9/9（100%）**，核心 SQL 功能对 agent 来说完全够用。

---

## 新发现的问题

### Bug #7 — 分支创建因连接池缓存而失败（严重，新引入）

**现象：** `POST /api/v1/branches` 始终返回 500：
```
ERROR: source database "oc_xxxx" is being accessed by other users (SQLSTATE 55006)
```

**根因：** 分支创建使用 `CREATE DATABASE new_db WITH TEMPLATE source_db`，PostgreSQL 要求源数据库上无活动连接。v2 的连接池缓存会保持对源数据库的持久连接，使得这个条件几乎不可能满足。v1 中因为每次请求新建连接池，连接可能在请求间关闭，所以分支创建是偶尔成功的。

**修复建议：** 在创建分支前，先断开源数据库的所有连接：
```go
// 1. 终止源数据库的所有活动连接
dbPool.Exec(ctx, fmt.Sprintf("SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = '%s' AND pid <> pg_backend_pid()", sourceDBName))
// 2. 然后再 CREATE DATABASE ... WITH TEMPLATE
```
或者在 `getCachedPool` 中提供主动关闭池的方法。

### Bug #8 — pgvector 扩展需在每个用户数据库中手动启用（中等）

**现象：** 新创建的数据库中执行 `CREATE TABLE ... (embedding vector(768))` 报错 `type "vector" does not exist`。

**原因：** PostgreSQL 的 extension 默认只在安装它的数据库中可用。需要在创建数据库时或建表前执行 `CREATE EXTENSION IF NOT EXISTS vector`。

**修复建议：** 在 `CreateDatabase` handler 中自动为每个新数据库创建常用扩展，或提供 `auto_extensions` 配置项。

---

## 功能可用性评级更新

| 功能 | v1 评级 | v2 评级 | 变化 |
|------|---------|---------|------|
| SQL 执行（单语句） | 完全可用 | 完全可用 | - |
| 多语句批量 SQL | 不可用 | 完全可用 | 修复 |
| 约束（UNIQUE/FK/NOT NULL） | 部分可用 | 完全可用 | 修复 |
| 连接池性能 | 有性能隐患 | 完全可用 | 修复 |
| JSONB 查询 | 完全可用 | 完全可用 | - |
| CTE / 窗口函数 | 完全可用 | 完全可用 | - |
| 数据持久化 | 完全可用 | 完全可用 | - |
| 并发读写 | 完全可用 | 完全可用 | - |
| 多数据库隔离 | 完全可用 | 完全可用 | - |
| 分支功能 | 基本可用 | 不可用 | 回退（新 Bug #7） |
| 向量搜索 | 完全可用 | 需手动初始化 | 回退（Bug #8） |
| GET /branches/:id | 不可用 | 已修复 | 修复（但因 #7 无法端到端验证） |

---

## 作为 AI Agent 数据层的实用性评估

### 优势

1. **RESTful API 简洁直觉：** agent 只需知道 database ID 和 SQL，不需要管理连接字符串、驱动版本、连接池配置。创建数据库、执行 SQL、查看结果，三步搞定。这对没有"运维经验"的 agent 来说大大降低了使用门槛。

2. **多数据库隔离天然适配 agent 工作流：** 每个项目、每个任务可以有自己的数据库，互不干扰。分支概念（如果能修好）非常适合"先在分支上实验，确认后再合并"的 agent 决策模式。

3. **PostgreSQL 全功能可用：** JSONB、CTE、窗口函数、聚合、pgvector 这些 agent 常用的查询能力全部可用，不需要做任何妥协。

4. **性能足够：** v2 的 ~3ms 单次查询延迟和 ~310 QPS，对于 agent 的典型使用模式（每步操作几次数据库查询，总步骤几十到几百次）绰绰有余。即使考虑网络开销，一个 50 步的 agent 任务增加的数据库延迟也就 150ms 级别。

5. **错误报告可靠：** v2 修复了约束错误静默吞掉的问题，agent 现在能正确感知 NOT NULL、UNIQUE、FK 等约束违反，这对于 agent 做出正确的重试或回退决策至关重要。

### 不足与风险

1. **分支功能是核心卖点但目前不可用：** 数据库分支是 oc-db9 与普通 PostgreSQL 实例的最大差异化特性。Bug #7 使得分支创建必现失败，这是需要优先修复的。

2. **缺少事务控制 API：** 当前只有单条 SQL（或多语句）执行，没有显式的 BEGIN/COMMIT/ROLLBACK API。agent 无法在多个 API 调用间维持事务。不过这可以通过多语句 SQL（`BEGIN; ...; COMMIT;`）变通实现。

3. **没有查询超时机制：** 一个写法不当的 SQL（如缺少 WHERE 的全表 UPDATE）可以长时间运行，没有超时保护。对于 agent 来说，应该有默认超时（比如 30s）并返回超时错误。

4. **pgvector 需要手动初始化：** 对 agent 来说，应该做到"开箱即用"。如果需要在建表前手动执行 `CREATE EXTENSION`，就增加了一个认知负担。

### 一句话结论

oc-db9 v2 作为 AI agent 的"自托管 PostgreSQL 即服务"是**有实际价值的**。它的 REST API + 多数据库隔离 + 全功能 PostgreSQL 的组合，解决了 agent 需要"零配置、零运维地用上数据库"的核心痛点。性能和可靠性已经达到了实用门槛。修好分支功能和 pgvector 自动初始化之后，就可以作为 agent 数据层的首选方案之一了。
