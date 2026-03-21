package fsbridge

import (
	"context"
	"fmt"
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
			NewCSVParser(true), // 默认带表头
			NewJSONLParser(),
			NewParquetParser(), // 添加 Parquet 支持
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
