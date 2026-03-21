# OpenClaw Quick-Setup 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 实现 OpenClaw 通过标准链接自动配置 oc-db9，使其能够自行快速配置并顺利融入使用

**Architecture:** 通过预置配置模板 + Quick-Setup API，让 OpenClaw 发送包含模板参数的请求即可完成数据库创建、schema 初始化、向量化配置等全部工作

**Tech Stack:** Go + Gin, PostgreSQL/pgvector, oc-db9 existing codebase

---

## 目录结构

```
api/internal/
├── handlers/
│   ├── setup.go          # 新增: Quick-Setup API
│   └── database.go       # 修改: 增加模板初始化逻辑
├── config/
│   └── templates.go      # 新增: 模板定义
└── cmd/api/main.go       # 修改: 注册新路由
cmd/
└── oc-db9/
    └── setup.go          # 新增: CLI quick-start 命令
docs/
└── superpowers/
    └── plans/
        └── 2026-03-21-openclaw-quick-setup.md  # 本计划
```

---

## 任务 1: 模板系统定义

**Files:**
- Create: `api/internal/config/templates.go`

- [ ] **Step 1: 创建模板定义文件**

```go
package config

type TableSchema struct {
	Name    string   `json:"name"`
	Columns []Column `json:"columns"`
}

type Column struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	IsVector bool  `json:"is_vector,omitempty"`
}

type Index struct {
	Name   string `json:"name"`
	Type   string `json:"type"` // btree, gist, ivfflat
	Column string `json:"column"`
}

type Trigger struct {
	Name   string `json:"name"`
	Timing string `json:"timing"` // BEFORE, AFTER
	Event  string `json:"event"`  // INSERT, UPDATE, DELETE
	Func   string `json:"function"`
}

type DatabaseTemplate struct {
	ID            string        `json:"id"`
	Name          string        `json:"name"`
	Description   string        `json:"description"`
	Tables        []TableSchema `json:"tables"`
	Indexes       []Index       `json:"indexes"`
	Triggers      []Trigger     `json:"triggers"`
	VectorDim     int           `json:"vector_dimension,omitempty"`
	RetentionDays int           `json:"retention_days,omitempty"`
}

var Templates = map[string]DatabaseTemplate{
	"ai-memory": {
		ID:          "ai-memory",
		Name:        "AI Agent Memory",
		Description: "AI Agent 跨对话记忆存储模板，包含用户偏好、对话历史、知识库",
		Tables: []TableSchema{
			{
				Name: "user_preferences",
				Columns: []Column{
					{Name: "user_id", Type: "UUID PRIMARY KEY"},
					{Name: "key", Type: "TEXT NOT NULL"},
					{Name: "value", Type: "JSONB NOT NULL"},
					{Name: "updated_at", Type: "TIMESTAMPTZ DEFAULT NOW()"},
				},
			},
			{
				Name: "conversation_history",
				Columns: []Column{
					{Name: "id", Type: "UUID PRIMARY KEY DEFAULT gen_random_uuid()"},
					{Name: "user_id", Type: "UUID NOT NULL"},
					{Name: "session_id", Type: "TEXT NOT NULL"},
					{Name: "role", Type: "TEXT NOT NULL"}, // user/assistant/system
					{Name: "content", Type: "TEXT NOT NULL"},
					{Name: "metadata", Type: "JSONB DEFAULT '{}'"},
					{Name: "created_at", Type: "TIMESTAMPTZ DEFAULT NOW()"},
				},
			},
			{
				Name: "knowledge_base",
				Columns: []Column{
					{Name: "id", Type: "UUID PRIMARY KEY DEFAULT gen_random_uuid()"},
					{Name: "content", Type: "TEXT NOT NULL"},
					{Name: "embedding", Type: "vector(1536)"},
					{Name: "metadata", Type: "JSONB DEFAULT '{}'"},
					{Name: "created_at", Type: "TIMESTAMPTZ DEFAULT NOW()"},
				},
			},
		},
		Indexes: []Index{
			{Name: "idx_conv_user", Type: "btree", Column: "user_id"},
			{Name: "idx_conv_session", Type: "btree", Column: "session_id"},
			{Name: "idx_kb_embedding", Type: "ivfflat", Column: "embedding"},
		},
		VectorDim:     1536,
		RetentionDays: 90,
	},
	"workflow-state": {
		ID:          "workflow-state",
		Name:        "Workflow State",
		Description: "工作流状态管理模板，包含任务状态、执行历史、事件日志",
		Tables: []TableSchema{
			{
				Name: "workflow_instances",
				Columns: []Column{
					{Name: "id", Type: "UUID PRIMARY KEY DEFAULT gen_random_uuid()"},
					{Name: "name", Type: "TEXT NOT NULL"},
					{Name: "status", Type: "TEXT NOT NULL DEFAULT 'pending'"},
					{Name: "state", Type: "JSONB DEFAULT '{}'"},
					{Name: "created_at", Type: "TIMESTAMPTZ DEFAULT NOW()"},
					{Name: "updated_at", Type: "TIMESTAMPTZ DEFAULT NOW()"},
				},
			},
			{
				Name: "workflow_events",
				Columns: []Column{
					{Name: "id", Type: "UUID PRIMARY KEY DEFAULT gen_random_uuid()"},
					{Name: "instance_id", Type: "UUID REFERENCES workflow_instances(id)"},
					{Name: "event_type", Type: "TEXT NOT NULL"},
					{Name: "payload", Type: "JSONB"},
					{Name: "created_at", Type: "TIMESTAMPTZ DEFAULT NOW()"},
				},
			},
		},
		Indexes: []Index{
			{Name: "idx_wf_status", Type: "btree", Column: "status"},
			{Name: "idx_wf_event_instance", Type: "btree", Column: "instance_id"},
		},
	},
	"knowledge-base": {
		ID:          "knowledge-base",
		Name:        "Knowledge Base",
		Description: "企业知识库模板，支持向量检索和标签分类",
		Tables: []TableSchema{
			{
				Name: "documents",
				Columns: []Column{
					{Name: "id", Type: "UUID PRIMARY KEY DEFAULT gen_random_uuid()"},
					{Name: "title", Type: "TEXT NOT NULL"},
					{Name: "content", Type: "TEXT NOT NULL"},
					{Name: "embedding", Type: "vector(1536)"},
					{Name: "tags", Type: "TEXT[] DEFAULT '{}'"},
					{Name: "metadata", Type: "JSONB DEFAULT '{}'"},
					{Name: "created_at", Type: "TIMESTAMPTZ DEFAULT NOW()"},
					{Name: "updated_at", Type: "TIMESTAMPTZ DEFAULT NOW()"},
				},
			},
			{
				Name: "tags",
				Columns: []Column{
					{Name: "id", Type: "UUID PRIMARY KEY DEFAULT gen_random_uuid()"},
					{Name: "name", Type: "TEXT UNIQUE NOT NULL"},
					{Name: "color", Type: "TEXT DEFAULT '#6366f1'"},
				},
			},
		},
		Indexes: []Index{
			{Name: "idx_doc_embedding", Type: "ivfflat", Column: "embedding"},
			{Name: "idx_doc_tags", Type: "gin", Column: "tags"},
		},
		VectorDim: 1536,
	},
}
```

- [ ] **Step 2: 提交**

```bash
cd "c:\Users\11428\Desktop\手搓db9"
git add api/internal/config/templates.go
git commit -m "feat: add database template definitions"
```

---

## 任务 2: Quick-Setup API 实现

**Files:**
- Create: `api/internal/handlers/setup.go`
- Modify: `api/internal/handlers/database.go` - 添加 initTemplateTables 函数
- Modify: `api/cmd/api/main.go` - 注册路由

- [ ] **Step 1: 创建 setup.go**

```go
package handlers

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/openclaw-db9/api/internal/config"
)

type QuickSetupRequest struct {
	Template string                 `json:"template" binding:"required"`
	Name     string                 `json:"name" binding:"required"`
	Options  map[string]interface{} `json:"options,omitempty"`
}

type QuickSetupResponse struct {
	DatabaseID       string   `json:"database_id"`
	DatabaseName     string   `json:"database_name"`
	ConnectionString string   `json:"connection_string"`
	DashboardURL     string   `json:"dashboard_url"`
	Tables           []string `json:"tables_created"`
	VectorEnabled    bool     `json:"vector_enabled"`
}

func QuickSetup(c *gin.Context) {
	var req QuickSetupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	template, ok := config.Templates[req.Template]
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":        fmt.Sprintf("unknown template: %s", req.Template),
			"available":    []string{"ai-memory", "workflow-state", "knowledge-base"},
		})
		return
	}

	createReq := CreateDatabaseRequest{Name: req.Name}
	dbID, pgDBName, err := createDatabaseWithTemplate(template, req.Options)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	tables := make([]string, len(template.Tables))
	for i, t := range template.Tables {
		tables[i] = t.Name
	}

	c.JSON(http.StatusCreated, QuickSetupResponse{
		DatabaseID:       dbID,
		DatabaseName:     pgDBName,
		ConnectionString: fmt.Sprintf("postgresql://postgres:postgres@localhost:5432/%s", pgDBName),
		DashboardURL:     "http://localhost:8080/",
		Tables:          tables,
		VectorEnabled:    template.VectorDim > 0,
	})
}

type ListTemplatesResponse struct {
	Templates []config.DatabaseTemplate `json:"templates"`
}

func ListTemplates(c *gin.Context) {
	templates := make([]config.DatabaseTemplate, 0, len(config.Templates))
	for _, t := range config.Templates {
		templates = append(templates, t)
	}
	c.JSON(http.StatusOK, ListTemplatesResponse{Templates: templates})
}
```

- [ ] **Step 2: 在 database.go 中添加 createDatabaseWithTemplate**

在 database.go 末尾添加：

```go
func createDatabaseWithTemplate(template config.DatabaseTemplate, options map[string]interface{}) (string, string, error) {
	accountID, err := getOrCreateDefaultAccount(context.Background())
	if err != nil {
		return "", "", err
	}

	dbID := uuid.New()
	pgDBName := fmt.Sprintf("oc_%s", dbID.String()[:8])

	_, err = dbPool.Exec(context.Background(),
		"INSERT INTO oc_databases (id, account_id, name, postgres_db_name) VALUES ($1, $2, $3, $4)",
		dbID, accountID, pgDBName, pgDBName)
	if err != nil {
		return "", "", err
	}

	_, err = dbPool.Exec(context.Background(), fmt.Sprintf("CREATE DATABASE %s", pgDBName))
	if err != nil {
		return "", "", err
	}

	if err := createDefaultExtensions(pgDBName); err != nil {
		fmt.Printf("Warning: failed to create extensions: %v\n", err)
	}

	if err := initTemplateTables(pgDBName, template); err != nil {
		fmt.Printf("Warning: failed to init template tables: %v\n", err)
	}

	return dbID.String(), pgDBName, nil
}

func initTemplateTables(pgDBName string, template config.DatabaseTemplate) error {
	lastSlash := strings.LastIndex(dbBaseURL, "/")
	if lastSlash == -1 {
		return fmt.Errorf("invalid dbBaseURL")
	}
	connString := dbBaseURL[:lastSlash+1] + pgDBName

	pool, err := pgxpool.New(context.Background(), connString)
	if err != nil {
		return err
	}
	defer pool.Close()

	for _, table := range template.Tables {
		columns := make([]string, len(table.Columns))
		for i, col := range table.Columns {
			columns[i] = fmt.Sprintf("%s %s", col.Name, col.Type)
		}
		createSQL := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (%s)", table.Name, joinStrings(columns, ", "))
		if _, err := pool.Exec(context.Background(), createSQL); err != nil {
			return fmt.Errorf("failed to create table %s: %w", table.Name, err)
		}
	}

	for _, idx := range template.Indexes {
		var idxSQL string
		switch idx.Type {
		case "btree", "gin":
			idxSQL = fmt.Sprintf("CREATE INDEX IF NOT EXISTS %s ON %s USING %s (%s)", idx.Name, idx.Column, idx.Type, idx.Column)
		case "ivfflat":
			idxSQL = fmt.Sprintf("CREATE INDEX IF NOT EXISTS %s ON %s USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100)", idx.Name, idx.Column)
		default:
			idxSQL = fmt.Sprintf("CREATE INDEX IF NOT EXISTS %s ON %s (%s)", idx.Name, idx.Column, idx.Column)
		}
		if _, err := pool.Exec(context.Background(), idxSQL); err != nil {
			fmt.Printf("Warning: failed to create index %s: %v\n", idx.Name, err)
		}
	}

	return nil
}

func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}
```

- [ ] **Step 3: 在 main.go 中注册路由**

在 databases group 中添加：

```go
databases.POST("/quick-setup", handlers.QuickSetup)
databases.GET("/templates", handlers.ListTemplates)
```

- [ ] **Step 4: 验证构建**

```bash
cd "c:\Users\11428\Desktop\手搓db9\api"
go build ./...
```

- [ ] **Step 5: 提交**

```bash
git add api/internal/handlers/setup.go api/internal/handlers/database.go api/cmd/api/main.go
git commit -m "feat: add Quick-Setup API with template support"
```

---

## 任务 3: 更新 skill.md 供 OpenClaw 自我配置

**Files:**
- Modify: `docs/skill.md`

- [ ] **Step 1: 添加 Quick-Setup 章节到 skill.md**

在 skill.md 末尾添加：

```markdown
## OpenClaw 快速配置

OpenClaw 可以通过 Quick-Setup API 自行配置 oc-db9 作为记忆存储层。

### 支持的模板

| 模板 | 用途 | 主要表 |
|------|------|--------|
| `ai-memory` | AI Agent 跨对话记忆 | user_preferences, conversation_history, knowledge_base |
| `workflow-state` | 工作流状态管理 | workflow_instances, workflow_events |
| `knowledge-base` | 企业知识库 | documents, tags |

### 使用示例

#### 1. 一键创建记忆数据库

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

#### 2. 查看可用模板

```bash
curl http://localhost:8080/api/v1/databases/templates
```

#### 3. 在 SQL 中使用记忆表

```sql
-- 存储用户偏好
INSERT INTO user_preferences (user_id, key, value)
VALUES ('user-123', 'theme', '"dark"'::jsonb);

-- 存储对话历史
INSERT INTO conversation_history (user_id, session_id, role, content)
VALUES ('user-123', 'session-1', 'user', '你好');

-- 语义搜索知识库
SELECT content, 1 - (embedding <=> embedding('如何学习编程')) as similarity
FROM knowledge_base
ORDER BY embedding <=> embedding('如何学习编程')
LIMIT 5;
```

### OpenClaw 自动配置流程

1. **检测需求**: OpenClaw 发现需要持久化存储时
2. **选择模板**: 根据用途选择 `ai-memory` 模板
3. **调用 API**: `POST /api/v1/databases/quick-setup`
4. **获取连接**: 收到 connection_string
5. **更新配置**: 将连接字符串写入 OpenClaw 配置
6. **验证连接**: 执行测试查询确认可用

### 连接信息

- **API Base**: `http://localhost:8080/api/v1`
- **Dashboard**: `http://localhost:8080/`
- **API Docs**: `http://localhost:8080/api/docs`
```

- [ ] **Step 2: 提交**

```bash
git add docs/skill.md
git commit -m "docs: add OpenClaw Quick-Setup guide to skill.md"
```

---

## 任务 4: CLI quick-start 命令（可选增强）

**Files:**
- Create: `cmd/oc-db9/setup.go`

- [ ] **Step 1: 创建 CLI setup 命令**

```go
package cmd

import (
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/spf13/cobra"
)

var templateFlag string

var setupCmd = &cobra.Command{
	Use:   "setup [name]",
	Short: "Quick setup a new database with template",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]

		reqBody := fmt.Sprintf(`{"template":"%s","name":"%s"}`, templateFlag, name)

		resp, err := http.Post("http://localhost:8080/api/v1/databases/quick-setup",
			"application/json", io.NopCloser(io.NopCloser(nil))))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			body, _ := io.ReadAll(resp.Body)
			fmt.Fprintf(os.Stderr, "Error: %s\n", string(body))
			os.Exit(1)
		}

		fmt.Println("Database created successfully!")
		fmt.Printf("Connection: postgresql://postgres:postgres@localhost:5432/%s\n", name)
		fmt.Printf("Dashboard: http://localhost:8080/\n")
	},
}

func init() {
	setupCmd.Flags().StringVar(&templateFlag, "template", "ai-memory", "Template to use (ai-memory, workflow-state, knowledge-base)")
	rootCmd.AddCommand(setupCmd)
}
```

- [ ] **Step 2: 验证构建**

```bash
cd "c:\Users\11428\Desktop\手搓db9"
go build ./cmd/oc-db9/...
```

- [ ] **Step 3: 提交**

```bash
git add cmd/oc-db9/setup.go
git commit -m "feat(cli): add quick-start command"
```

---

## 使用流程总结

### OpenClaw 自行配置流程

```
OpenClaw 启动
    ↓
检测到需要记忆存储
    ↓
POST /api/v1/databases/quick-setup
{
  "template": "ai-memory",
  "name": "openclaw-memory"
}
    ↓
收到响应:
{
  "connection_string": "postgresql://...",
  "tables_created": [...]
}
    ↓
保存连接字符串到配置
    ↓
验证连接
    ↓
配置完成，开始使用
```

### 用户发链接流程

```
用户提供链接:
https://oc-db9.com/setup?template=ai-memory&name=myproject

OpenClaw 解析参数:
- template = ai-memory
- name = myproject

调用 Quick-Setup API → 创建数据库 → 返回连接信息
```

---

## 验证测试

### 手动测试

```bash
# 1. 启动服务
docker-compose up -d

# 2. 创建 ai-memory 数据库
curl -X POST http://localhost:8080/api/v1/databases/quick-setup \
  -H "Content-Type: application/json" \
  -d '{"template":"ai-memory","name":"test-memory"}'

# 3. 验证返回的 tables_created 包含预期表
# 4. 访问 Dashboard 确认数据库存在
# 5. 执行 SQL 测试表结构
```

---

**Plan complete.** 两个执行选项:

**1. Subagent-Driven (recommended)** - dispatch fresh subagent per task

**2. Inline Execution** - execute tasks in this session using executing-plans

**Which approach?**
