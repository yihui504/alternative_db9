package handlers

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
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

// 连接池缓存，用于复用用户数据库连接
var poolCache sync.Map

type cachedPool struct {
	pool *pgxpool.Pool
	last time.Time
}

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

	// 使用连接池缓存
	userPool := getCachedPool(connString)
	if userPool == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get connection pool"})
		return
	}

	var results []map[string]interface{}

	// 检查是否包含多语句（分号分隔）
	hasMultipleStatements := containsMultipleStatements(req.SQL)

	if hasMultipleStatements {
		// 多语句使用 Exec()
		_, err = userPool.Exec(context.Background(), req.SQL)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		// 多语句执行成功，返回影响行数信息
		c.JSON(http.StatusOK, gin.H{
			"results": nil,
			"message": "SQL executed successfully",
			"type":    "exec",
		})
		return
	}

	// 单语句使用 Query()
	rows, err := userPool.Query(context.Background(), req.SQL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"results": results})
}

// getCachedPool 获取或创建缓存的连接池
func getCachedPool(connString string) *pgxpool.Pool {
	// 尝试从缓存获取
	if cached, ok := poolCache.Load(connString); ok {
		cp := cached.(*cachedPool)
		cp.last = time.Now()
		return cp.pool
	}

	// 创建新连接池
	pool, err := pgxpool.New(context.Background(), connString)
	if err != nil {
		return nil
	}

	// 存入缓存
	poolCache.Store(connString, &cachedPool{
		pool: pool,
		last: time.Now(),
	})

	return pool
}

// CloseAllPools 关闭所有缓存的连接池
func CloseAllPools() {
	poolCache.Range(func(key, value interface{}) bool {
		cp := value.(*cachedPool)
		cp.pool.Close()
		return true
	})
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

// containsMultipleStatements 检查 SQL 是否包含多语句（分号分隔）
// 忽略字符串字面量和注释中的分号
func containsMultipleStatements(sql string) bool {
	// 移除注释
	sql = removeComments(sql)

	// 简单检查：如果包含分号且分号不在字符串字面量中
	inString := false

	for i, ch := range sql {
		if ch == '\'' && (i == 0 || sql[i-1] != '\\') {
			inString = !inString
		}
		if ch == ';' && !inString {
			return true
		}
	}

	return false
}

// removeComments 移除 SQL 中的注释
func removeComments(sql string) string {
	var result strings.Builder
	inBlockComment := false
	inLineComment := false

	for i := 0; i < len(sql); i++ {
		if i+1 < len(sql) {
			// 块注释 /* */
			if sql[i] == '/' && sql[i+1] == '*' {
				inBlockComment = true
				i++
				continue
			}
			if sql[i] == '*' && sql[i+1] == '/' {
				inBlockComment = false
				i++
				continue
			}
		}

		// 行注释 --
		if i+1 < len(sql) && sql[i] == '-' && sql[i+1] == '-' {
			inLineComment = true
			i++
			continue
		}

		if sql[i] == '\n' {
			inLineComment = false
		}

		if !inBlockComment && !inLineComment {
			result.WriteByte(sql[i])
		}
	}

	return result.String()
}
