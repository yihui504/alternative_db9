package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ollamaEndpoint string

func init() {
	ollamaEndpoint = os.Getenv("OLLAMA_ENDPOINT")
	if ollamaEndpoint == "" {
		ollamaEndpoint = "http://localhost:11434"
	}
}

type EmbeddingRequest struct {
	Text      string `json:"text" binding:"required"`
	Model     string `json:"model"`
	TableName string `json:"table_name"`
}

type EmbeddingResponse struct {
	Embedding []float32 `json:"embedding"`
	Model     string    `json:"model"`
	Dimension int       `json:"dimension"`
}

type OllamaEmbeddingRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

type OllamaEmbeddingResponse struct {
	Embedding []float32 `json:"embedding"`
}

func GenerateEmbedding(c *gin.Context) {
	var req EmbeddingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	model := req.Model
	if model == "" {
		model = "nomic-embed-text"
	}

	ollamaReq := OllamaEmbeddingRequest{
		Model:  model,
		Prompt: req.Text,
	}

	body, _ := json.Marshal(ollamaReq)
	url := fmt.Sprintf("http://%s/api/embeddings", ollamaEndpoint)

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Ollama connection failed: %v", err)})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Ollama returned status %d: %s", resp.StatusCode, string(respBody)),
		})
		return
	}

	var ollamaResp OllamaEmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, EmbeddingResponse{
		Embedding: ollamaResp.Embedding,
		Model:     model,
		Dimension: len(ollamaResp.Embedding),
	})
}

type CreateVectorTableRequest struct {
	DatabaseID string `json:"database_id" binding:"required"`
	TableName  string `json:"table_name" binding:"required"`
	Dimension  int    `json:"dimension"`
}

func CreateVectorTable(c *gin.Context) {
	var req CreateVectorTableRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	dimension := req.Dimension
	if dimension == 0 {
		dimension = 1536
	}

	var pgDBName string
	err := dbPool.QueryRow(context.Background(),
		"SELECT postgres_db_name FROM oc_databases WHERE id = $1", req.DatabaseID).Scan(&pgDBName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "database not found"})
		return
	}

	connString := dbBaseURL[:strings.LastIndex(dbBaseURL, "/")+1] + pgDBName
	userPool, err := pgxpool.New(context.Background(), connString)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer userPool.Close()

	_, err = userPool.Exec(context.Background(), "CREATE EXTENSION IF NOT EXISTS vector")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to create vector extension: %v", err)})
		return
	}

	createTableSQL := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			content TEXT,
			embedding vector(%d),
			metadata JSONB DEFAULT '{}'::jsonb,
			created_at TIMESTAMPTZ DEFAULT NOW()
		)`, req.TableName, dimension)
	
	_, err = userPool.Exec(context.Background(), createTableSQL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	createIndexSQL := fmt.Sprintf(`
		CREATE INDEX IF NOT EXISTS %s_embedding_idx ON %s 
		USING ivfflat (embedding vector_cosine_ops)
		WITH (lists = 100)`, req.TableName, req.TableName)
	
	_, err = userPool.Exec(context.Background(), createIndexSQL)
	if err != nil {
		log.Printf("Warning: Failed to create vector index: %v", err)
	}

	_, err = dbPool.Exec(context.Background(),
		"INSERT INTO oc_embeddings (database_id, table_name, column_name, model_name, dimension) VALUES ($1, $2, $3, $4, $5)",
		req.DatabaseID, req.TableName, "embedding", "default", dimension)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":   "vector table created",
		"table":     req.TableName,
		"dimension": dimension,
	})
}

type InsertVectorRequest struct {
	DatabaseID string          `json:"database_id" binding:"required"`
	TableName  string          `json:"table_name" binding:"required"`
	Content    string          `json:"content" binding:"required"`
	Embedding  []float32       `json:"embedding"`
	Metadata   json.RawMessage `json:"metadata"`
}

func InsertVector(c *gin.Context) {
	var req InsertVectorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if len(req.Embedding) == 0 {
		model := "nomic-embed-text"
		ollamaReq := OllamaEmbeddingRequest{
			Model:  model,
			Prompt: req.Content,
		}

		body, _ := json.Marshal(ollamaReq)
		url := fmt.Sprintf("http://%s/api/embeddings", ollamaEndpoint)

		resp, err := http.Post(url, "application/json", bytes.NewBuffer(body))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Ollama connection failed: %v", err)})
			return
		}
		defer resp.Body.Close()

		var ollamaResp OllamaEmbeddingResponse
		if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		req.Embedding = ollamaResp.Embedding
	}

	var pgDBName string
	err := dbPool.QueryRow(context.Background(),
		"SELECT postgres_db_name FROM oc_databases WHERE id = $1", req.DatabaseID).Scan(&pgDBName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "database not found"})
		return
	}

	connString := dbBaseURL[:strings.LastIndex(dbBaseURL, "/")+1] + pgDBName
	userPool, err := pgxpool.New(context.Background(), connString)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer userPool.Close()

	metadata := req.Metadata
	if metadata == nil {
		metadata = json.RawMessage("{}")
	}

	var id uuid.UUID
	err = userPool.QueryRow(context.Background(),
		fmt.Sprintf("INSERT INTO %s (content, embedding, metadata) VALUES ($1, $2, $3) RETURNING id", req.TableName),
		req.Content, req.Embedding, metadata).Scan(&id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":        id.String(),
		"content":   req.Content,
		"dimension": len(req.Embedding),
	})
}

type SimilaritySearchRequest struct {
	DatabaseID string    `json:"database_id" binding:"required"`
	TableName  string    `json:"table_name" binding:"required"`
	Query      string    `json:"query" binding:"required"`
	Embedding  []float32 `json:"embedding"`
	Limit      int       `json:"limit"`
}

type SearchResult struct {
	ID         string          `json:"id"`
	Content    string          `json:"content"`
	Metadata   json.RawMessage `json:"metadata"`
	Similarity float64         `json:"similarity"`
}

func SimilaritySearch(c *gin.Context) {
	var req SimilaritySearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	embedding := req.Embedding
	if len(embedding) == 0 {
		model := "nomic-embed-text"
		ollamaReq := OllamaEmbeddingRequest{
			Model:  model,
			Prompt: req.Query,
		}

		body, _ := json.Marshal(ollamaReq)
		url := fmt.Sprintf("http://%s/api/embeddings", ollamaEndpoint)

		resp, err := http.Post(url, "application/json", bytes.NewBuffer(body))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Ollama connection failed: %v", err)})
			return
		}
		defer resp.Body.Close()

		var ollamaResp OllamaEmbeddingResponse
		if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		embedding = ollamaResp.Embedding
	}

	limit := req.Limit
	if limit == 0 {
		limit = 5
	}

	var pgDBName string
	err := dbPool.QueryRow(context.Background(),
		"SELECT postgres_db_name FROM oc_databases WHERE id = $1", req.DatabaseID).Scan(&pgDBName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "database not found"})
		return
	}

	connString := dbBaseURL[:strings.LastIndex(dbBaseURL, "/")+1] + pgDBName
	userPool, err := pgxpool.New(context.Background(), connString)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer userPool.Close()

	rows, err := userPool.Query(context.Background(),
		fmt.Sprintf("SELECT id, content, metadata, 1 - (embedding <=> $1) as similarity FROM %s ORDER BY embedding <=> $1 LIMIT $2", req.TableName),
		embedding, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		if err := rows.Scan(&r.ID, &r.Content, &r.Metadata, &r.Similarity); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		results = append(results, r)
	}

	c.JSON(http.StatusOK, gin.H{
		"query":    req.Query,
		"results":  results,
		"count":    len(results),
		"model":    "nomic-embed-text",
		"provider": "ollama",
	})
}
