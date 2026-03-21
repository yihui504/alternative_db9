package fsbridge

import (
	"testing"

	"github.com/xitongsys/parquet-go-source/local"
	"github.com/xitongsys/parquet-go/writer"
)

// 测试用的简单结构体
type TestUser struct {
	Name string `parquet:"name=name, type=BYTE_ARRAY, convertedtype=UTF8"`
	Age  int32  `parquet:"name=age, type=INT32"`
	City string `parquet:"name=city, type=BYTE_ARRAY, convertedtype=UTF8"`
}

// createTestParquetFile 创建测试用的 Parquet 文件
func createTestParquetFile(t *testing.T, path string) {
	var err error
	fw, err := local.NewLocalFileWriter(path)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	pw, err := writer.NewParquetWriter(fw, new(TestUser), 4)
	if err != nil {
		t.Fatalf("Failed to create parquet writer: %v", err)
	}

	users := []TestUser{
		{Name: "Alice", Age: 30, City: "NYC"},
		{Name: "Bob", Age: 25, City: "LA"},
		{Name: "Charlie", Age: 35, City: "Chicago"},
	}

	for _, user := range users {
		if err := pw.Write(user); err != nil {
			t.Fatalf("Failed to write user: %v", err)
		}
	}

	if err := pw.WriteStop(); err != nil {
		t.Fatalf("Failed to stop writer: %v", err)
	}

	fw.Close()
}
