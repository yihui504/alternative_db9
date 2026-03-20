package handlers

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Database struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	PostgresDBName string    `json:"postgres_db_name"`
	CreatedAt      time.Time `json:"created_at"`
}

type CreateDatabaseRequest struct {
	Name string `json:"name" binding:"required"`
	From string `json:"from"`
	Seed string `json:"seed"`
}

var dbPool *pgxpool.Pool
var dbBaseURL string

func SetDBPool(pool *pgxpool.Pool) {
	dbPool = pool
	dbBaseURL = os.Getenv("DATABASE_URL")
}

func InitDB(connString string) error {
	var err error
	dbPool, err = pgxpool.New(context.Background(), connString)
	return err
}

func getOrCreateDefaultAccount(ctx context.Context) (uuid.UUID, error) {
	var accountID uuid.UUID

	err := dbPool.QueryRow(ctx,
		"SELECT id FROM oc_accounts WHERE name = 'default'").Scan(&accountID)
	if err == nil {
		return accountID, nil
	}

	accountID = uuid.New()
	_, err = dbPool.Exec(ctx,
		"INSERT INTO oc_accounts (id, name) VALUES ($1, 'default')",
		accountID)
	if err != nil {
		return uuid.Nil, err
	}

	return accountID, nil
}

func CreateDatabase(c *gin.Context) {
	var req CreateDatabaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	accountID, err := getOrCreateDefaultAccount(context.Background())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	dbID := uuid.New()
	pgDBName := fmt.Sprintf("oc_%s", dbID.String()[:8])

	_, err = dbPool.Exec(context.Background(),
		"INSERT INTO oc_databases (id, account_id, name, postgres_db_name) VALUES ($1, $2, $3, $4)",
		dbID, accountID, req.Name, pgDBName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	_, err = dbPool.Exec(context.Background(), fmt.Sprintf("CREATE DATABASE %s", pgDBName))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":               dbID.String(),
		"name":             req.Name,
		"postgres_db_name": pgDBName,
	})
}

func ListDatabases(c *gin.Context) {
	accountID, err := getOrCreateDefaultAccount(context.Background())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	rows, err := dbPool.Query(context.Background(),
		"SELECT id, name, postgres_db_name, created_at FROM oc_databases WHERE account_id = $1",
		accountID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	var databases []Database
	for rows.Next() {
		var db Database
		if err := rows.Scan(&db.ID, &db.Name, &db.PostgresDBName, &db.CreatedAt); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		databases = append(databases, db)
	}

	c.JSON(http.StatusOK, databases)
}

func GetDatabase(c *gin.Context) {
	id := c.Param("id")

	var db Database
	err := dbPool.QueryRow(context.Background(),
		"SELECT id, name, postgres_db_name, created_at FROM oc_databases WHERE id = $1",
		id).Scan(&db.ID, &db.Name, &db.PostgresDBName, &db.CreatedAt)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "database not found"})
		return
	}

	c.JSON(http.StatusOK, db)
}

func DeleteDatabase(c *gin.Context) {
	id := c.Param("id")

	var pgDBName string
	err := dbPool.QueryRow(context.Background(),
		"SELECT postgres_db_name FROM oc_databases WHERE id = $1", id).Scan(&pgDBName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "database not found"})
		return
	}

	_, err = dbPool.Exec(context.Background(), fmt.Sprintf("DROP DATABASE IF EXISTS %s", pgDBName))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	_, err = dbPool.Exec(context.Background(), "DELETE FROM oc_databases WHERE id = $1", id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "database deleted"})
}

func ExecuteSQL(c *gin.Context) {
	id := c.Param("id")

	var req struct {
		SQL string `json:"sql" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var pgDBName string
	err := dbPool.QueryRow(context.Background(),
		"SELECT postgres_db_name FROM oc_databases WHERE id = $1", id).Scan(&pgDBName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "database not found"})
		return
	}

	lastSlash := strings.LastIndex(dbBaseURL, "/")
	if lastSlash == -1 {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid database URL"})
		return
	}
	connString := dbBaseURL[:lastSlash+1] + pgDBName
	userPool, err := pgxpool.New(context.Background(), connString)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer userPool.Close()

	rows, err := userPool.Query(context.Background(), req.SQL)
	if err != nil {
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

	c.JSON(http.StatusOK, gin.H{"results": results})
}

func GetConnectionInfo(c *gin.Context) {
	id := c.Param("id")

	var pgDBName string
	err := dbPool.QueryRow(context.Background(),
		"SELECT postgres_db_name FROM oc_databases WHERE id = $1", id).Scan(&pgDBName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "database not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"host":              "localhost",
		"port":              5432,
		"database":          pgDBName,
		"user":              "postgres",
		"connection_string": fmt.Sprintf("postgresql://postgres:postgres@localhost:5432/%s", pgDBName),
	})
}
