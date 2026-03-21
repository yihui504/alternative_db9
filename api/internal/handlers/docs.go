package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type APIEndpoint struct {
	Method     string   `json:"method"`
	Path       string   `json:"path"`
	Summary    string   `json:"summary"`
	Request    string   `json:"request,omitempty"`
	Response   string   `json:"response,omitempty"`
	Parameters []string `json:"parameters,omitempty"`
}

type APISection struct {
	Name      string        `json:"name"`
	Endpoints []APIEndpoint `json:"endpoints"`
}

func GetAPIDocs(c *gin.Context) {
	docs := []APISection{
		{
			Name: "Health & Stats",
			Endpoints: []APIEndpoint{
				{Method: "GET", Path: "/health", Summary: "Health check with component status"},
				{Method: "GET", Path: "/api/v1/monitor/system", Summary: "System statistics"},
				{Method: "GET", Path: "/api/v1/monitor/stats", Summary: "Database statistics"},
				{Method: "GET", Path: "/api/v1/monitor/health", Summary: "Component health status"},
			},
		},
		{
			Name: "Databases",
			Endpoints: []APIEndpoint{
				{Method: "POST", Path: "/api/v1/databases", Summary: "Create a new database", Request: `{"name": "mydb"}`},
				{Method: "GET", Path: "/api/v1/databases", Summary: "List all databases"},
				{Method: "GET", Path: "/api/v1/databases/:id", Summary: "Get database details"},
				{Method: "DELETE", Path: "/api/v1/databases/:id", Summary: "Delete a database"},
				{Method: "POST", Path: "/api/v1/databases/:id/sql", Summary: "Execute SQL query", Request: `{"sql": "SELECT * FROM users", "timeout": 30, "limit": 100}`},
				{Method: "POST", Path: "/api/v1/databases/:id/query", Summary: "Execute parameterized query", Request: `{"sql": "INSERT INTO users (name) VALUES ($1)", "params": ["John"]}`},
				{Method: "GET", Path: "/api/v1/databases/:id/connect", Summary: "Get connection info"},
			},
		},
		{
			Name: "Transactions",
			Endpoints: []APIEndpoint{
				{Method: "POST", Path: "/api/v1/databases/:id/transactions", Summary: "Begin a new transaction"},
				{Method: "GET", Path: "/api/v1/databases/:id/transactions/:tid", Summary: "Get transaction status"},
				{Method: "PUT", Path: "/api/v1/databases/:id/transactions/:tid/sql", Summary: "Execute SQL in transaction", Request: `{"sql": "UPDATE accounts SET balance = balance - 100"}`},
				{Method: "POST", Path: "/api/v1/databases/:id/transactions/:tid/commit", Summary: "Commit transaction"},
				{Method: "POST", Path: "/api/v1/databases/:id/transactions/:tid/rollback", Summary: "Rollback transaction"},
			},
		},
		{
			Name: "Branches",
			Endpoints: []APIEndpoint{
				{Method: "POST", Path: "/api/v1/branches", Summary: "Create a branch", Request: `{"database_id": "xxx", "name": "feature-branch"}`},
				{Method: "GET", Path: "/api/v1/branches", Summary: "List all branches (or filter by ?database_id=xxx)"},
				{Method: "GET", Path: "/api/v1/branches/:id", Summary: "Get branch details"},
				{Method: "DELETE", Path: "/api/v1/branches/:id", Summary: "Delete a branch"},
			},
		},
		{
			Name: "Files",
			Endpoints: []APIEndpoint{
				{Method: "POST", Path: "/api/v1/files/upload", Summary: "Upload a file", Request: `{"filename": "data.csv", "content": "base64..."}`},
				{Method: "GET", Path: "/api/v1/files", Summary: "List uploaded files"},
				{Method: "GET", Path: "/api/v1/files/:id", Summary: "Download file"},
				{Method: "DELETE", Path: "/api/v1/files/:id", Summary: "Delete file"},
				{Method: "GET", Path: "/api/v1/files/query", Summary: "Query file data", Parameters: []string{"bucket", "key", "format", "sql"}},
			},
		},
		{
			Name: "Embeddings",
			Endpoints: []APIEndpoint{
				{Method: "POST", Path: "/api/v1/embeddings/generate", Summary: "Generate embedding", Request: `{"text": "Hello world", "model": "nomic-embed-text"}`},
				{Method: "POST", Path: "/api/v1/embeddings/tables", Summary: "Create vector table", Request: `{"database_id": "xxx", "table_name": "doc_embeddings", "dimension": 1536}`},
				{Method: "POST", Path: "/api/v1/embeddings/insert", Summary: "Insert vector", Request: `{"database_id": "xxx", "table_name": "doc_embeddings", "content": "document text"}`},
				{Method: "POST", Path: "/api/v1/embeddings/search", Summary: "Similarity search", Request: `{"database_id": "xxx", "table_name": "doc_embeddings", "query": "search text", "limit": 5}`},
			},
		},
		{
			Name: "Webhooks",
			Endpoints: []APIEndpoint{
				{Method: "POST", Path: "/api/v1/webhooks", Summary: "Register webhook", Request: `{"url": "https://...", "events": ["database.created", "database.deleted"]}`},
				{Method: "GET", Path: "/api/v1/webhooks", Summary: "List webhooks"},
				{Method: "DELETE", Path: "/api/v1/webhooks/:id", Summary: "Delete webhook"},
			},
		},
		{
			Name: "Backups",
			Endpoints: []APIEndpoint{
				{Method: "POST", Path: "/api/v1/backups", Summary: "Create backup"},
				{Method: "GET", Path: "/api/v1/backups", Summary: "List backups"},
				{Method: "POST", Path: "/api/v1/backups/restore", Summary: "Restore backup", Request: `{"backup_id": "xxx"}`},
				{Method: "DELETE", Path: "/api/v1/backups/:id", Summary: "Delete backup"},
				{Method: "GET", Path: "/api/v1/backups/:id/download", Summary: "Download backup"},
			},
		},
		{
			Name: "Cron Jobs",
			Endpoints: []APIEndpoint{
				{Method: "POST", Path: "/api/v1/cron", Summary: "Create cron job", Request: `{"database_id": "xxx", "schedule": "*/5 * * * *", "sql": "SELECT 1"}`},
				{Method: "GET", Path: "/api/v1/cron", Summary: "List cron jobs"},
				{Method: "DELETE", Path: "/api/v1/cron/:id", Summary: "Delete cron job"},
				{Method: "GET", Path: "/api/v1/cron/:id/logs", Summary: "Get cron job logs"},
			},
		},
	}

	c.JSON(http.StatusOK, gin.H{
		"api_version": "v1.1.0",
		"title":       "OpenClaw-db9 API",
		"description": "Database API for AI agents with SQL execution, file bridging, vector embeddings, and more",
		"endpoints":   docs,
	})
}
