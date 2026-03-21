package handlers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const defaultTxTimeout = 60 * time.Second

type Transaction struct {
	ID        string
	Tx        pgx.Tx
	Pool      *pgxpool.Pool
	CreatedAt time.Time
	ExpiresAt time.Time
}

var transactions sync.Map

type TransactionInfo struct {
	ID        string    `json:"transaction_id"`
	DatabaseID string    `json:"database_id"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

type BeginTransactionResponse struct {
	TransactionID string    `json:"transaction_id"`
	ExpiresAt     time.Time `json:"expires_at"`
}

type ExecuteInTransactionRequest struct {
	SQL     string `json:"sql" binding:"required"`
	Timeout int    `json:"timeout"`
}

type ExecuteInTransactionResponse struct {
	Results   []map[string]interface{} `json:"results"`
	RowsAffected int64                 `json:"rows_affected,omitempty"`
	Type      string                   `json:"type"`
}

func BeginTransaction(c *gin.Context) {
	id := c.Param("id")

	var pgDBName string
	err := dbPool.QueryRow(context.Background(),
		"SELECT postgres_db_name FROM oc_databases WHERE id = $1", id).Scan(&pgDBName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "database not found"})
		return
	}

	pool := getPoolForDatabase(pgDBName)
	if pool == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get connection pool"})
		return
	}

	tx, err := pool.Begin(context.Background())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to begin transaction: %v", err)})
		return
	}

	txID := uuid.New().String()
	now := time.Now()
	txEntry := &Transaction{
		ID:        txID,
		Tx:        tx,
		Pool:      pool,
		CreatedAt: now,
		ExpiresAt: now.Add(defaultTxTimeout),
	}

	transactions.Store(txID, txEntry)

	log.Printf("Transaction %s started for database %s, expires at %s", txID, pgDBName, txEntry.ExpiresAt.Format(time.RFC3339))

	c.JSON(http.StatusCreated, BeginTransactionResponse{
		TransactionID: txID,
		ExpiresAt:     txEntry.ExpiresAt,
	})
}

func GetTransaction(c *gin.Context) {
	txID := c.Param("tid")

	entry, ok := transactions.Load(txID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "transaction not found"})
		return
	}

	tx := entry.(*Transaction)
	if time.Now().After(tx.ExpiresAt) {
		tx.Tx.Rollback(context.Background())
		transactions.Delete(txID)
		c.JSON(http.StatusNotFound, gin.H{"error": "transaction expired"})
		return
	}

	c.JSON(http.StatusOK, TransactionInfo{
		ID:        tx.ID,
		CreatedAt: tx.CreatedAt,
		ExpiresAt: tx.ExpiresAt,
	})
}

func ExecuteInTransaction(c *gin.Context) {
	txID := c.Param("tid")
	_ = c.Param("id")

	entry, ok := transactions.Load(txID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "transaction not found"})
		return
	}

	tx := entry.(*Transaction)

	if time.Now().After(tx.ExpiresAt) {
		tx.Tx.Rollback(context.Background())
		transactions.Delete(txID)
		c.JSON(http.StatusRequestTimeout, gin.H{"error": "transaction expired"})
		return
	}

	var req ExecuteInTransactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tx.ExpiresAt = time.Now().Add(defaultTxTimeout)

	timeout := appConfig.QueryTimeout
	if req.Timeout > 0 {
		timeout = req.Timeout
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), time.Duration(timeout)*time.Second)
	defer cancel()

	hasMultipleStatements := containsMultipleStatements(req.SQL)

	if hasMultipleStatements {
		_, err := tx.Tx.Exec(ctx, req.SQL)
		if err != nil {
			if ctx.Err() == context.DeadlineExceeded {
				c.JSON(http.StatusRequestTimeout, gin.H{"error": fmt.Sprintf("Query timed out after %ds", timeout)})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, ExecuteInTransactionResponse{
			Type: "exec",
		})
		return
	}

	rows, err := tx.Tx.Query(ctx, req.SQL)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			c.JSON(http.StatusRequestTimeout, gin.H{"error": fmt.Sprintf("Query timed out after %ds", timeout)})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		fields := rows.FieldDescriptions()
		row := make(map[string]interface{})
		for i, field := range fields {
			row[string(field.Name)] = values[i]
		}
		results = append(results, row)
	}

	if err := rows.Err(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			c.JSON(http.StatusRequestTimeout, gin.H{"error": fmt.Sprintf("Query timed out after %ds", timeout)})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, ExecuteInTransactionResponse{
		Results: results,
		Type:    "query",
	})
}

func CommitTransaction(c *gin.Context) {
	txID := c.Param("tid")

	entry, ok := transactions.Load(txID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "transaction not found"})
		return
	}

	tx := entry.(*Transaction)

	if err := tx.Tx.Commit(context.Background()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to commit transaction: %v", err)})
		return
	}

	transactions.Delete(txID)
	log.Printf("Transaction %s committed successfully", txID)

	c.JSON(http.StatusOK, gin.H{
		"message":         "transaction committed",
		"transaction_id":   txID,
	})
}

func RollbackTransaction(c *gin.Context) {
	txID := c.Param("tid")

	entry, ok := transactions.Load(txID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "transaction not found"})
		return
	}

	tx := entry.(*Transaction)

	if err := tx.Tx.Rollback(context.Background()); err != nil {
		log.Printf("Warning: failed to rollback transaction %s: %v", txID, err)
	}

	transactions.Delete(txID)
	log.Printf("Transaction %s rolled back", txID)

	c.JSON(http.StatusOK, gin.H{
		"message":       "transaction rolled back",
		"transaction_id": txID,
	})
}

func StartTransactionCleaner() {
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			cleanExpiredTransactions()
		}
	}()
}

func cleanExpiredTransactions() {
	now := time.Now()

	transactions.Range(func(key, value interface{}) bool {
		tx := value.(*Transaction)
		if now.After(tx.ExpiresAt) {
			if err := tx.Tx.Rollback(context.Background()); err != nil {
				log.Printf("Warning: failed to rollback expired transaction %s: %v", tx.ID, err)
			}
			transactions.Delete(key)
			log.Printf("Expired transaction %s cleaned up", tx.ID)
		}
		return true
	})
}
