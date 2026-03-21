package fsbridge

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strconv"

	"github.com/xitongsys/parquet-go/parquet"
	"github.com/xitongsys/parquet-go/reader"
	"github.com/xitongsys/parquet-go/source"
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

// ParquetParser Parquet 文件解析器
type ParquetParser struct{}

func NewParquetParser() *ParquetParser {
	return &ParquetParser{}
}

func (p *ParquetParser) Supports(contentType string) bool {
	return contentType == "application/parquet" ||
		contentType == "application/x-parquet" ||
		contentType == "application/octet-stream"
}

func (p *ParquetParser) Parse(r io.Reader) (*VirtualTable, error) {
	// 读取所有数据到内存（Parquet 需要 seek）
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read parquet data: %w", err)
	}

	// 创建内存文件
	memFile := &memFile{data: data}

	// 创建 Parquet reader，使用空结构体指针
	pr, err := reader.NewParquetReader(memFile, nil, 4)
	if err != nil {
		return nil, fmt.Errorf("failed to create parquet reader: %w", err)
	}
	defer pr.ReadStop()

	// 获取列信息
	schema := pr.SchemaHandler
	var columns []Column
	columnNames := []string{}
	for i := 0; i < len(schema.SchemaElements); i++ {
		elem := schema.SchemaElements[i]
		if elem.NumChildren == nil && elem.Name != "" { // 叶子节点（实际列）
			col := Column{
				Name:     elem.Name,
				Type:     parquetTypeToString(elem.Type),
				Nullable: elem.RepetitionType != nil && *elem.RepetitionType != parquet.FieldRepetitionType_REQUIRED,
			}
			columns = append(columns, col)
			columnNames = append(columnNames, elem.Name)
		}
	}

	// 使用 ColumnReader 读取列数据
	numRows := int(pr.GetNumRows())
	var rows [][]interface{}

	// 初始化行数据
	for i := 0; i < numRows; i++ {
		rows = append(rows, make([]interface{}, len(columns)))
	}

	// 逐列读取数据
	for colIdx, colName := range columnNames {
		values, _, _, _ := pr.ReadColumnByPath(colName, int64(numRows))
		for rowIdx, val := range values {
			if rowIdx < len(rows) {
				rows[rowIdx][colIdx] = val
			}
		}
	}

	return &VirtualTable{
		Columns: columns,
		Rows:    rows,
	}, nil
}

// structToSlice 将结构体转换为切片
func structToSlice(v interface{}, numFields int) []interface{} {
	result := make([]interface{}, numFields)
	val := reflect.ValueOf(v)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	if val.Kind() != reflect.Struct {
		return result
	}

	for i := 0; i < val.NumField() && i < numFields; i++ {
		field := val.Field(i)
		result[i] = field.Interface()
	}
	return result
}

// 内存文件实现，用于 parquet reader
type memFile struct {
	data   []byte
	offset int
}

func (m *memFile) Read(p []byte) (n int, err error) {
	if m.offset >= len(m.data) {
		return 0, io.EOF
	}
	n = copy(p, m.data[m.offset:])
	m.offset += n
	return n, nil
}

func (m *memFile) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
		m.offset = int(offset)
	case io.SeekCurrent:
		m.offset += int(offset)
	case io.SeekEnd:
		m.offset = len(m.data) + int(offset)
	}
	if m.offset < 0 {
		m.offset = 0
	}
	return int64(m.offset), nil
}

func (m *memFile) Close() error {
	return nil
}

func (m *memFile) Create(name string) (source.ParquetFile, error) {
	return nil, fmt.Errorf("Create not supported for memFile")
}

func (m *memFile) Open(name string) (source.ParquetFile, error) {
	return m, nil
}

func (m *memFile) Write(p []byte) (n int, err error) {
	return 0, fmt.Errorf("Write not supported for memFile")
}

func parquetTypeToString(t *parquet.Type) string {
	if t == nil {
		return "text"
	}
	switch *t {
	case parquet.Type_BOOLEAN:
		return "boolean"
	case parquet.Type_INT32, parquet.Type_INT64:
		return "integer"
	case parquet.Type_FLOAT, parquet.Type_DOUBLE:
		return "float"
	default:
		return "text"
	}
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
