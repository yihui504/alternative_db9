package fsbridge

import (
	"os"
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

func TestParquetParser(t *testing.T) {
	// 创建临时测试文件
	tmpFile := "testdata/test.parquet"
	os.MkdirAll("testdata", 0755)
	createTestParquetFile(t, tmpFile)
	defer os.Remove(tmpFile)

	// 打开文件
	file, err := os.Open(tmpFile)
	if err != nil {
		t.Fatalf("Failed to open test file: %v", err)
	}
	defer file.Close()

	// 解析
	parser := NewParquetParser()
	table, err := parser.Parse(file)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// 验证列
	if len(table.Columns) != 3 {
		t.Errorf("Expected 3 columns, got %d", len(table.Columns))
	}

	// 验证行数
	if len(table.Rows) != 3 {
		t.Errorf("Expected 3 rows, got %d", len(table.Rows))
	}

	// 验证第一行数据
	if len(table.Rows) > 0 {
		row := table.Rows[0]
		if len(row) != 3 {
			t.Errorf("Expected 3 values in first row, got %d", len(row))
		}
	}
}

func TestParquetParser_Supports(t *testing.T) {
	parser := NewParquetParser()

	tests := []struct {
		contentType string
		expected    bool
	}{
		{"application/parquet", true},
		{"application/x-parquet", true},
		{"application/octet-stream", true},
		{"text/csv", false},
		{"application/json", false},
	}

	for _, tt := range tests {
		result := parser.Supports(tt.contentType)
		if result != tt.expected {
			t.Errorf("Supports(%s) = %v, expected %v", tt.contentType, result, tt.expected)
		}
	}
}
