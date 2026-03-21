# 补全关键差距：SQL 直接查询文件系统 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 实现 db9.ai 最核心的创新功能 - 用 SQL 直接查询文件（CSV/JSONL/Parquet），打通文件系统与 SQL 的鸿沟

**Architecture:** 通过 PostgreSQL 外部数据包装器(FDW) + 自定义扩展，让 MinIO 中的文件可以像表一样被 SQL 查询。同时提供内置 embedding 函数和 HTTP 请求能力。

**Tech Stack:** PostgreSQL FDW, Go, MinIO, pgvector, Ollama

---

## 背景与核心差距分析

db9.ai 最核心的创新是 **"文件即表"** 的融合体验：

```bash
# 用户只需要复制文件
$ db9 fs cp ./users.csv :/data/import/users.csv

# 然后可以直接用 SQL 查询文件内容
$ db9 db sql -c "SELECT * FROM extensions.fs9('/data/import/users.csv')"
```

我们当前的项目中：
- ✅ 文件存储在 MinIO
- ✅ SQL 查询 PostgreSQL
- ❌ **两者是分离的，无法用 SQL 直接查询文件**

本计划将补全这个关键差距。

---

## 文件结构

```
api/
├── internal/
│   ├── handlers/
│   │   ├── file.go (已存在，需扩展)
│   │   ├── file_query.go (新建 - SQL查询文件API)
│   │   └── embedding.go (已存在，需增强)
│   └── fsbridge/ (新建目录 - 文件-SQL桥接核心)
│       ├── parser.go       # CSV/JSONL/Parquet 解析器
│       ├── fdw.go          # FDW 接口封装
│       └── virtual_table.go # 虚拟表实现
├── init-scripts/
│   ├── 01-extensions.sql (已存在，需扩展)
│   └── 02-fsbridge.sql (新建 - FDW 和函数定义)
└── cmd/api/main.go (已存在，需集成)

internal/cmd/
├── fs.go (已存在，需扩展)
└── db.go (已存在，需扩展查询命令)
```

---

## Phase 1: 文件解析器基础 (Parser Foundation)

### Task 1: CSV 解析器

**Files:**
- Create: `api/internal/fsbridge/parser.go`
- Test: `api/internal/fsbridge/parser_test.go`

- [ ] **Step 1: 定义解析器接口和 CSV 实现**

```go
package fsbridge

import (
	"encoding/csv"
	"io"
	"strconv"
)

// FileParser 定义文件解析接口
type FileParser interface {
	Parse(r io.Reader) (*VirtualTable, error)
	Supports(contentType string) bool
}

// Column 定义虚拟表列
type Column struct {
	Name     string
	Type     string // text, integer, float, boolean
	Nullable bool
}

// VirtualTable 代表解析后的虚拟表
type VirtualTable struct {
	Columns []Column
	Rows    [][]interface{}
}

// CSVParser CSV 文件解析器
type CSVParser struct {
	hasHeader bool
}

func NewCSVParser(hasHeader bool) *CSVParser {
	return &CSVParser{hasHeader: hasHeader}
}

func (p *CSVParser) Supports(contentType string) bool {
	return contentType == "text/csv" || contentType == "application/csv"
}

func (p *CSVParser) Parse(r io.Reader) (*VirtualTable, error) {
	reader := csv.NewReader(r)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	if len(records) == 0 {
		return &VirtualTable{Columns: []Column{}, Rows: [][]interface{}{}}, nil
	}

	var columns []Column
	var rows [][]interface{}
	startRow := 0

	if p.hasHeader {
		for _, name := range records[0] {
			columns = append(columns, Column{
				Name:     name,
				Type:     "text",
				Nullable: true,
			})
		}
		startRow = 1
	} else {
		// 无表头，生成列名 c1, c2, c3...
		for i := range records[0] {
			columns = append(columns, Column{
				Name:     fmt.Sprintf("c%d", i+1),
				Type:     "text",
				Nullable: true,
			})
		}
	}

	// 解析数据行，尝试类型推断
	for i := startRow; i < len(records); i++ {
		row := make([]interface{}, len(records[i]))
		for j, val := range records[i] {
			row[j] = inferType(val)
			// 更新列类型（如果更具体）
			if i == startRow {
				columns[j].Type = detectType(val)
			}
		}
		rows = append(rows, row)
	}

	return &VirtualTable{
		Columns: columns,
		Rows:    rows,
	}, nil
}

func inferType(val string) interface{} {
	if val == "" {
		return nil
	}
	// 尝试整数
	if i, err := strconv.ParseInt(val, 10, 64); err == nil {
		return i
	}
	// 尝试浮点数
	if f, err := strconv.ParseFloat(val, 64); err == nil {
		return f
	}
	// 尝试布尔值
	if val == "true" || val == "TRUE" {
		return true
	}
	if val == "false" || val == "FALSE" {
		return false
	}
	// 默认文本
	return val
}

func detectType(val string) string {
	if val == "" {
		return "text"
	}
	if _, err := strconv.ParseInt(val, 10, 64); err == nil {
		return "integer"
	}
	if _, err := strconv.ParseFloat(val, 64); err == nil {
		return "float"
	}
	if val == "true" || val == "false" || val == "TRUE" || val == "FALSE" {
		return "boolean"
	}
	return "text"
}
```

- [ ] **Step 2: 编写单元测试**

```go
package fsbridge

import (
	"strings"
	"testing"
)

func TestCSVParser_WithHeader(t *testing.T) {
	csv := `name,age,active
Alice,30,true
Bob,25,false
Charlie,35,true`

	parser := NewCSVParser(true)
	table, err := parser.Parse(strings.NewReader(csv))

	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(table.Columns) != 3 {
		t.Errorf("Expected 3 columns, got %d", len(table.Columns))
	}

	if table.Columns[0].Name != "name" {
		t.Errorf("Expected column name 'name', got %s", table.Columns[0].Name)
	}

	if len(table.Rows) != 3 {
		t.Errorf("Expected 3 rows, got %d", len(table.Rows))
	}
}

func TestCSVParser_WithoutHeader(t *testing.T) {
	csv := `Alice,30,true
Bob,25,false`

	parser := NewCSVParser(false)
	table, err := parser.Parse(strings.NewReader(csv))

	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if table.Columns[0].Name != "c1" {
		t.Errorf("Expected column name 'c1', got %s", table.Columns[0].Name)
	}
}

func TestDetectType(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"123", "integer"},
		{"12.34", "float"},
		{"true", "boolean"},
		{"hello", "text"},
	}

	for _, tt := range tests {
		result := detectType(tt.input)
		if result != tt.expected {
			t.Errorf("detectType(%s) = %s, expected %s", tt.input, result, tt.expected)
		}
	}
}
```

- [ ] **Step 3: 运行测试**

```bash
cd api
go test ./internal/fsbridge/... -v
```

Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add api/internal/fsbridge/
git commit -m "feat(fsbridge): add CSV parser for virtual table generation"
```

---

### Task 2: JSONL 解析器

**Files:**
- Modify: `api/internal/fsbridge/parser.go`

- [ ] **Step 1: 添加 JSONL 解析器**

```go
import (
	"bufio"
	"encoding/json"
	"fmt"
)

// JSONLParser JSON Lines 解析器
type JSONLParser struct{}

func NewJSONLParser() *JSONLParser {
	return &JSONLParser{}
}

func (p *JSONLParser) Supports(contentType string) bool {
	return contentType == "application/jsonl" || contentType == "application/x-jsonlines"
}

func (p *JSONLParser) Parse(r io.Reader) (*VirtualTable, error) {
	scanner := bufio.NewScanner(r)
	var rows [][]interface{}
	var columns []Column
	columnIndex := make(map[string]int)

	for scanner.Scan() {
		var obj map[string]interface{}
		if err := json.Unmarshal(scanner.Bytes(), &obj); err != nil {
			continue // 跳过无效行
		}

		// 动态发现列
		for key := range obj {
			if _, exists := columnIndex[key]; !exists {
				columnIndex[key] = len(columns)
				columns = append(columns, Column{
					Name:     key,
					Type:     "json",
					Nullable: true,
				})
			}
		}

		// 构建行数据
		row := make([]interface{}, len(columns))
		for key, val := range obj {
			idx := columnIndex[key]
			row[idx] = val
		}
		rows = append(rows, row)
	}

	return &VirtualTable{
		Columns: columns,
		Rows:    rows,
	}, nil
}
```

- [ ] **Step 2: 添加测试**

```go
func TestJSONLParser(t *testing.T) {
	jsonl := `{"name": "Alice", "age": 30}
{"name": "Bob", "age": 25}
{"name": "Charlie", "age": 35, "city": "NYC"}`

	parser := NewJSONLParser()
	table, err := parser.Parse(strings.NewReader(jsonl))

	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(table.Rows) != 3 {
		t.Errorf("Expected 3 rows, got %d", len(table.Rows))
	}

	// 检查动态列发现
	hasCity := false
	for _, col := range table.Columns {
		if col.Name == "city" {
			hasCity = true
			break
		}
	}
	if !hasCity {
		t.Error("Expected 'city' column to be discovered")
	}
}
```

- [ ] **Step 3: 运行测试**

```bash
go test ./internal/fsbridge/... -v -run JSONL
```

Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add api/internal/fsbridge/parser.go
git commit -m "feat(fsbridge): add JSONL parser with dynamic column discovery"
```

---

## Phase 2: 文件-SQL 桥接服务

### Task 3: 创建文件查询服务

**Files:**
- Create: `api/internal/fsbridge/service.go`
- Modify: `api/internal/handlers/file.go` (添加查询端点)

- [ ] **Step 1: 实现桥接服务**

```go
package fsbridge

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/minio/minio-go/v7"
)

// FileBridgeService 文件桥接服务
type FileBridgeService struct {
	minioClient *minio.Client
	bucket      string
	parsers     []FileParser
}

func NewFileBridgeService(minioClient *minio.Client, bucket string) *FileBridgeService {
	return &FileBridgeService{
		minioClient: minioClient,
		bucket:      bucket,
		parsers: []FileParser{
			NewCSVParser(true),  // 默认带表头
			NewJSONLParser(),
		},
	}
}

// QueryFile 查询文件内容，返回虚拟表
func (s *FileBridgeService) QueryFile(ctx context.Context, databaseID, filePath string) (*VirtualTable, error) {
	// 构建 MinIO 对象名
	objectName := fmt.Sprintf("%s%s", databaseID, filePath)

	// 从 MinIO 获取文件
	obj, err := s.minioClient.GetObject(ctx, s.bucket, objectName, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get file: %w", err)
	}
	defer obj.Close()

	// 检测文件类型
	contentType := detectContentType(filePath)

	// 选择合适的解析器
	parser := s.findParser(contentType)
	if parser == nil {
		return nil, fmt.Errorf("unsupported file type: %s", contentType)
	}

	return parser.Parse(obj)
}

// QueryFileAsRows 将文件内容作为行数据返回（用于 SQL 结果）
func (s *FileBridgeService) QueryFileAsRows(ctx context.Context, databaseID, filePath string) ([]map[string]interface{}, error) {
	table, err := s.QueryFile(ctx, databaseID, filePath)
	if err != nil {
		return nil, err
	}

	var rows []map[string]interface{}
	for _, row := range table.Rows {
		rowMap := make(map[string]interface{})
		for i, col := range table.Columns {
			if i < len(row) {
				rowMap[col.Name] = row[i]
			}
		}
		// 添加元数据列
		rowMap["_line_number"] = len(rows) + 1
		rowMap["_path"] = filePath
		rows = append(rows, rowMap)
	}

	return rows, nil
}

func (s *FileBridgeService) findParser(contentType string) FileParser {
	for _, p := range s.parsers {
		if p.Supports(contentType) {
			return p
		}
	}
	return nil
}

func detectContentType(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".csv":
		return "text/csv"
	case ".jsonl":
		return "application/jsonl"
	case ".json":
		return "application/json"
	case ".parquet":
		return "application/parquet"
	default:
		return "text/plain"
	}
}
```

- [ ] **Step 2: 创建文件查询 API Handler**

Create `api/internal/handlers/file_query.go`:

```go
package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yihui504/oc-db9/api/internal/fsbridge"
)

var fileBridgeService *fsbridge.FileBridgeService

func InitFileBridgeService() {
	fileBridgeService = fsbridge.NewFileBridgeService(minioClient, minioBucket)
}

// QueryFile 用 SQL-like 方式查询文件
func QueryFile(c *gin.Context) {
	databaseID := c.Query("database_id")
	filePath := c.Query("path")

	if databaseID == "" || filePath == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "database_id and path are required",
		})
		return
	}

	rows, err := fileBridgeService.QueryFileAsRows(c.Request.Context(), databaseID, filePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"results": rows,
		"count":   len(rows),
	})
}
```

- [ ] **Step 3: 在 main.go 中初始化服务**

Modify `api/cmd/api/main.go`，在 InitMinio 后添加：

```go
func main() {
	// ... 现有代码 ...
	
	// 初始化 MinIO
	if err := handlers.InitMinio(cfg.MinioEndpoint, cfg.MinioAccessKey, cfg.MinioSecretKey); err != nil {
		log.Fatal("Failed to init MinIO:", err)
	}

	// 初始化文件桥接服务
	handlers.InitFileBridgeService()
	
	// ... 后续代码 ...
}
```

- [ ] **Step 4: 添加路由**

在路由配置中添加：

```go
router.GET("/api/v1/files/query", handlers.QueryFile)
```

- [ ] **Step 5: Commit**

```bash
git add api/internal/fsbridge/service.go api/internal/handlers/file_query.go
git commit -m "feat(fsbridge): add file bridge service and query API"
```

---

## Phase 3: PostgreSQL 扩展集成

### Task 4: 创建 fs9() SQL 函数

**Files:**
- Create: `api/init-scripts/02-fsbridge.sql`

- [ ] **Step 1: 创建 PL/pgSQL 函数**

```sql
-- fs9 文件系统桥接扩展
-- 提供 SQL 接口查询 MinIO 中的文件

-- 创建文件查询函数（通过 dblink 或 http 调用 API）
CREATE EXTENSION IF NOT EXISTS http;

-- fs9() 函数 - 查询文件内容
CREATE OR REPLACE FUNCTION fs9(file_path TEXT)
RETURNS TABLE (
    _line_number BIGINT,
    _path TEXT,
    data JSONB
) AS $$
DECLARE
    api_url TEXT;
    response JSONB;
    row_data JSONB;
BEGIN
    -- 从配置获取 API URL
    api_url := current_setting('app.api_url', true);
    IF api_url IS NULL OR api_url = '' THEN
        api_url := 'http://localhost:8080';
    END IF;

    -- 调用 API 获取文件内容
    SELECT content::JSONB INTO response
    FROM http_get(
        api_url || '/api/v1/files/query?database_id=' || 
        current_setting('app.current_database_id', true) || 
        '&path=' || uri_encode(file_path)
    );

    -- 返回结果集中的每一行
    FOR row_data IN SELECT jsonb_array_elements(response->'results')
    LOOP
        _line_number := (row_data->>'_line_number')::BIGINT;
        _path := row_data->>'_path';
        data := row_data - '_line_number' - '_path';
        RETURN NEXT;
    END LOOP;

    RETURN;
END;
$$ LANGUAGE plpgsql;

-- 辅助函数：URI 编码
CREATE OR REPLACE FUNCTION uri_encode(str TEXT)
RETURNS TEXT AS $$
DECLARE
    result TEXT := '';
    ch TEXT;
BEGIN
    FOR i IN 1..length(str) LOOP
        ch := substr(str, i, 1);
        IF ch ~ '[A-Za-z0-9_.~-]' THEN
            result := result || ch;
        ELSE
            result := result || '%' || encode(ch::bytea, 'hex');
        END IF;
    END LOOP;
    RETURN result;
END;
$$ LANGUAGE plpgsql IMMUTABLE;

-- 创建视图简化查询
CREATE OR REPLACE VIEW fs9_files AS
SELECT 
    f.id,
    f.database_id,
    f.path,
    f.size,
    f.created_at
FROM oc_files f;

-- 添加注释
COMMENT ON FUNCTION fs9(TEXT) IS 'Query file content as virtual table. Usage: SELECT * FROM fs9(''/path/to/file.csv'')';
```

- [ ] **Step 2: 更新 01-extensions.sql**

在 `api/init-scripts/01-extensions.sql` 末尾添加：

```sql
-- 文件桥接需要的扩展
CREATE EXTENSION IF NOT EXISTS http;  -- HTTP 请求
```

- [ ] **Step 3: Commit**

```bash
git add api/init-scripts/
git commit -m "feat(db): add fs9() SQL function for file querying"
```

---

## Phase 4: CLI 增强

### Task 5: 增强 db sql 命令支持文件查询

**Files:**
- Modify: `internal/cmd/db.go`
- Modify: `internal/cmd/fs.go`

- [ ] **Step 1: 添加 fs query 子命令**

在 `internal/cmd/fs.go` 中添加：

```go
var fsQueryCmd = &cobra.Command{
	Use:   "query <path>",
	Short: "Query file content as table",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		path := args[0]
		dbID, _ := cmd.Flags().GetString("db")

		if dbID == "" {
			fmt.Fprintln(os.Stderr, "Error: --db is required")
			os.Exit(1)
		}

		resp, err := http.Get(apiURL + "/api/v1/files/query?database_id=" + dbID + "&path=" + url.QueryEscape(path))
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
		defer resp.Body.Close()

		data, _ := io.ReadAll(resp.Body)

		var result struct {
			Results []map[string]interface{} `json:"results"`
			Count   int                      `json:"count"`
		}
		json.Unmarshal(data, &result)

		// 表格输出
		if len(result.Results) > 0 {
			// 打印表头
			for key := range result.Results[0] {
				fmt.Printf("%s\t", key)
			}
			fmt.Println()

			// 打印分隔线
			for range result.Results[0] {
				fmt.Printf("%s\t", "----")
			}
			fmt.Println()

			// 打印数据
			for _, row := range result.Results {
				for _, val := range row {
					fmt.Printf("%v\t", val)
				}
				fmt.Println()
			}
		}

		fmt.Printf("\nTotal: %d rows\n", result.Count)
	},
}

func init() {
	// ... existing init code ...
	fsCmd.AddCommand(fsQueryCmd)
	fsQueryCmd.Flags().String("db", "", "Database ID")
}
```

- [ ] **Step 2: 增强 db sql 命令支持特殊语法**

添加一个辅助命令来生成查询文件的 SQL：

```go
var dbFsQueryCmd = &cobra.Command{
	Use:   "fs-query <path>",
	Short: "Generate SQL to query file content",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		path := args[0]
		
		// 生成示例 SQL
		fmt.Printf("-- Query file content\n")
		fmt.Printf("SELECT * FROM fs9('%s');\n\n", path)
		
		fmt.Printf("-- Query with filtering\n")
		fmt.Printf("SELECT * FROM fs9('%s') WHERE data->>'name' = 'Alice';\n\n", path)
		
		fmt.Printf("-- Aggregate query\n")
		fmt.Printf("SELECT COUNT(*) FROM fs9('%s');\n", path)
	},
}
```

- [ ] **Step 3: Commit**

```bash
git add internal/cmd/fs.go internal/cmd/db.go
git commit -m "feat(cli): add fs query command and SQL generation helpers"
```

---

## Phase 5: Embedding 增强

### Task 6: 内置 embedding() 函数

**Files:**
- Modify: `api/internal/handlers/embedding.go`
- Create: `api/init-scripts/03-embedding.sql`

- [ ] **Step 1: 创建 embedding SQL 函数**

```sql
-- embedding 扩展
-- 提供内置的向量嵌入功能

-- embedding() 函数 - 生成文本的向量表示
CREATE OR REPLACE FUNCTION embedding(
    input_text TEXT,
    model_name TEXT DEFAULT 'nomic-embed-text'
)
RETURNS vector(768) AS $$
DECLARE
    api_url TEXT;
    response JSONB;
    embedding_vector vector(768);
BEGIN
    -- 获取 API URL
    api_url := current_setting('app.api_url', true);
    IF api_url IS NULL OR api_url = '' THEN
        api_url := 'http://localhost:8080';
    END IF;

    -- 调用 embedding API
    SELECT content::JSONB INTO response
    FROM http_post(
        url := api_url || '/api/v1/embeddings/generate',
        body := jsonb_build_object(
            'text', input_text,
            'model', model_name
        )
    );

    -- 解析向量
    SELECT array_agg(x::float8)::vector(768) INTO embedding_vector
    FROM jsonb_array_elements_text(response->'embedding') AS x;

    RETURN embedding_vector;
END;
$$ LANGUAGE plpgsql;

-- 相似度搜索辅助函数
CREATE OR REPLACE FUNCTION similarity(
    vec1 vector,
    vec2 vector
)
RETURNS float AS $$
BEGIN
    RETURN 1 - (vec1 <=> vec2);  -- 余弦相似度
END;
$$ LANGUAGE plpgsql IMMUTABLE;

COMMENT ON FUNCTION embedding(TEXT, TEXT) IS 'Generate embedding vector for text using configured model';
```

- [ ] **Step 2: Commit**

```bash
git add api/init-scripts/03-embedding.sql
git commit -m "feat(db): add embedding() SQL function for vector generation"
```

---

## Phase 6: 集成测试

### Task 7: 端到端测试

**Files:**
- Create: `tests/integration/fsbridge_test.go`

- [ ] **Step 1: 创建集成测试**

```go
package integration

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"testing"
	"time"
)

func TestFileQueryWorkflow(t *testing.T) {
	baseURL := getBaseURL()

	// 1. 创建数据库
	dbID := createTestDatabase(t, baseURL)

	// 2. 上传 CSV 文件
	csvContent := `name,age,city
Alice,30,NYC
Bob,25,LA
Charlie,35,Chicago`

	uploadTestFile(t, baseURL, dbID, "/test/users.csv", csvContent)

	// 3. 查询文件内容
	queryURL := baseURL + "/api/v1/files/query?database_id=" + dbID + "&path=/test/users.csv"
	resp, err := http.Get(queryURL)
	if err != nil {
		t.Fatalf("Failed to query file: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Query failed: %s", string(body))
	}

	var result struct {
		Results []map[string]interface{} `json:"results"`
		Count   int                      `json:"count"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if result.Count != 3 {
		t.Errorf("Expected 3 rows, got %d", result.Count)
	}

	// 清理
	deleteTestDatabase(t, baseURL, dbID)
}

func createTestDatabase(t *testing.T, baseURL string) string {
	reqBody := map[string]string{"name": "test-db-" + time.Now().Format("20060102150405")}
	body, _ := json.Marshal(reqBody)

	resp, err := http.Post(baseURL+"/api/v1/databases", "application/json", bytes.NewBuffer(body))
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	return result["id"].(string)
}

func uploadTestFile(t *testing.T, baseURL, dbID, path, content string) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, _ := writer.CreateFormFile("file", "test.csv")
	io.WriteString(part, content)
	writer.WriteField("database_id", dbID)
	writer.WriteField("path", path)
	writer.Close()

	resp, err := http.Post(baseURL+"/api/v1/files/upload", writer.FormDataContentType(), &buf)
	if err != nil {
		t.Fatalf("Failed to upload file: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Upload failed: %s", string(body))
	}
}

func deleteTestDatabase(t *testing.T, baseURL, dbID string) {
	req, _ := http.NewRequest("DELETE", baseURL+"/api/v1/databases/"+dbID, nil)
	http.DefaultClient.Do(req)
}

func getBaseURL() string {
	if url := os.Getenv("API_URL"); url != "" {
		return url
	}
	return "http://localhost:8080"
}
```

- [ ] **Step 2: 运行集成测试**

```bash
# 先启动服务
docker-compose up -d

# 等待服务就绪
sleep 5

# 运行测试
cd tests/integration
go test -v -run TestFileQueryWorkflow
```

Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add tests/integration/
git commit -m "test(integration): add fsbridge end-to-end tests"
```

---

## Phase 7: 文档更新

### Task 8: 更新文档

**Files:**
- Modify: `README.md`
- Modify: `docs/api-reference.md`

- [ ] **Step 1: 更新 README 添加 fs9 功能说明**

在 README 的 "与 db9.ai 对比" 表格后添加：

```markdown
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
- Parquet (计划中)
```

- [ ] **Step 2: 更新 API 文档**

在 `docs/api-reference.md` 中添加文件查询端点：

```markdown
### 文件内容查询

| 端点 | 方法 | 描述 |
|------|------|------|
| `/api/v1/files/query` | GET | 将文件内容作为表格数据查询 |

**参数：**
- `database_id` (required): 数据库 ID
- `path` (required): 文件路径

**响应示例：**
```json
{
  "results": [
    {
      "_line_number": 1,
      "_path": "/data/users.csv",
      "name": "Alice",
      "age": 30,
      "city": "NYC"
    }
  ],
  "count": 1
}
```
```

- [ ] **Step 3: Commit**

```bash
git add README.md docs/api-reference.md
git commit -m "docs: update documentation with fs9 file querying feature"
```

---

## 实施总结

### 完成后的功能清单

| 功能 | 状态 |
|------|------|
| CSV 文件 SQL 查询 | ✅ |
| JSONL 文件 SQL 查询 | ✅ |
| 自动类型推断 | ✅ |
| 内置 embedding() 函数 | ✅ |
| CLI fs query 命令 | ✅ |
| 虚拟表元数据列 (_line_number, _path) | ✅ |

### 与 db9.ai 的差距缩小

| 差距项 | 实施前 | 实施后 |
|--------|--------|--------|
| SQL 直接查询文件 | ❌ | ✅ |
| 文件-SQL 融合体验 | ❌ | ✅ |
| 内置 embedding | ⚠️ | ✅ |

### 仍存在的差异

1. **分支速度**：我们使用 pg_dump（秒级），db9 使用 CoW（毫秒级）
2. **文件事务**：我们需要应用层协调，db9 是原生同一事务
3. **Parquet 支持**：需要额外实现

---

## 执行选项

**Plan complete and saved to `docs/superpowers/plans/2026-03-21-bridge-sql-file-gap.md`. Two execution options:**

**1. Subagent-Driven (recommended)** - I dispatch a fresh subagent per task, review between tasks, fast iteration

**2. Inline Execution** - Execute tasks in this session using executing-plans, batch execution with checkpoints for review

**Which approach?**
