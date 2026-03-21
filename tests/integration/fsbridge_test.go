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
