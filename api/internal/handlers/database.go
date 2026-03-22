package handlers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/openclaw-db9/api/internal/config"
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

type ExecuteSQLRequest struct {
	SQL     string        `json:"sql" binding:"required"`
	Params  []interface{} `json:"params"`
	Timeout int           `json:"timeout"`
	Limit   int           `json:"limit"`
	Offset  int           `json:"offset"`
}

type SQLResponse struct {
	Results        []map[string]interface{} `json:"results"`
	Truncated      bool                     `json:"truncated"`
	TotalRows      int                      `json:"total_rows"`
	OriginalLimit  int                      `json:"original_limit,omitempty"`
	OriginalOffset int                      `json:"original_offset,omitempty"`
}

var dbPool *pgxpool.Pool
var dbBaseURL string
var appConfig *config.Config

var poolCache sync.Map

type cachedPool struct {
	pool      *pgxpool.Pool
	createdAt time.Time
	lastUsed  time.Time
}

func SetDBPool(pool *pgxpool.Pool) {
	dbPool = pool
	dbBaseURL = os.Getenv("DATABASE_URL")
}

func SetConfig(cfg *config.Config) {
	appConfig = cfg
}

func InitDB(connString string) error {
	var err error
	dbPool, err = pgxpool.New(context.Background(), connString)
	return err
}

func StartPoolCleaner() {
	if appConfig == nil {
		return
	}
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			cleanExpiredPools()
		}
	}()
}

func cleanExpiredPools() {
	now := time.Now()
	var toDelete []string
	poolCount := 0

	poolCache.Range(func(key, value interface{}) bool {
		poolCount++
		cp := value.(*cachedPool)
		age := now.Sub(cp.createdAt)
		idle := now.Sub(cp.lastUsed)

		if age > appConfig.PoolTTL || idle > appConfig.PoolIdleTTL {
			toDelete = append(toDelete, key.(string))
		}
		return true
	})

	for _, connString := range toDelete {
		if cached, ok := poolCache.LoadAndDelete(connString); ok {
			cp := cached.(*cachedPool)
			cp.pool.Close()
			log.Printf("Closed expired pool for database: %s", connString)
			poolCount--
		}
	}

	if poolCount > appConfig.MaxPools {
		evictLRUPools(appConfig.MaxPools / 2)
	}
}

func evictLRUPools(targetCount int) {
	type poolInfo struct {
		key      string
		lastUsed time.Time
	}

	var pools []poolInfo
	poolCache.Range(func(key, value interface{}) bool {
		cp := value.(*cachedPool)
		pools = append(pools, poolInfo{key: key.(string), lastUsed: cp.lastUsed})
		return true
	})

	for i := 0; i < len(pools)-1; i++ {
		for j := i + 1; j < len(pools); j++ {
			if pools[j].lastUsed.Before(pools[i].lastUsed) {
				pools[i], pools[j] = pools[j], pools[i]
			}
		}
	}

	for i := 0; i < len(pools)-targetCount && i < len(pools); i++ {
		if cached, ok := poolCache.LoadAndDelete(pools[i].key); ok {
			cp := cached.(*cachedPool)
			cp.pool.Close()
			log.Printf("LRU eviction: closed pool for database: %s", pools[i].key)
		}
	}
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

	if appConfig != nil && appConfig.MaxDatabases > 0 {
		var dbCount int
		err := dbPool.QueryRow(context.Background(),
			"SELECT COUNT(*) FROM oc_databases").Scan(&dbCount)
		if err == nil && dbCount >= appConfig.MaxDatabases {
			c.JSON(http.StatusForbidden, gin.H{
				"error":   fmt.Sprintf("Maximum database limit reached (%d)", appConfig.MaxDatabases),
				"limit":   appConfig.MaxDatabases,
				"current": dbCount,
			})
			return
		}
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

	if err := createDefaultExtensions(pgDBName); err != nil {
		fmt.Printf("Warning: failed to create extensions in %s: %v\n", pgDBName, err)
	}

	SendWebhook("database.created", map[string]string{
		"database_id":   dbID.String(),
		"database_name": pgDBName,
		"name":          req.Name,
	})

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

	closeCachedPool(pgDBName)

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

	SendWebhook("database.deleted", map[string]string{
		"database_id":   id,
		"database_name": pgDBName,
	})

	c.JSON(http.StatusOK, gin.H{"message": "database deleted"})
}

func ExecuteSQL(c *gin.Context) {
	id := c.Param("id")

	var req ExecuteSQLRequest
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

	userPool := getCachedPool(connString)
	if userPool == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get connection pool"})
		return
	}

	timeout := appConfig.QueryTimeout
	if req.Timeout > 0 {
		timeout = req.Timeout
	}

	isDDL := isDDLStatement(req.SQL)
	if isDDL && timeout < appConfig.DDLTimeout {
		timeout = appConfig.DDLTimeout
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), time.Duration(timeout)*time.Second)
	defer cancel()

	hasMultipleStatements := containsMultipleStatements(req.SQL)

	if len(req.Params) > 0 {
		var args []interface{}
		for _, p := range req.Params {
			args = append(args, p)
		}
		
		_, err = userPool.Exec(ctx, req.SQL, args...)
		
		if err != nil {
			if ctx.Err() == context.DeadlineExceeded {
				c.JSON(http.StatusRequestTimeout, gin.H{"error": fmt.Sprintf("Query timed out after %ds", timeout)})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"results": nil,
			"message": "SQL executed successfully",
			"type":    "exec",
		})
		return
	}

	if hasMultipleStatements {
		// 如果有多个语句，我们不支持参数化查询，强制转为普通执行
		_, err = userPool.Exec(ctx, req.SQL)
		if err != nil {
			if ctx.Err() == context.DeadlineExceeded {
				c.JSON(http.StatusRequestTimeout, gin.H{"error": fmt.Sprintf("Query timed out after %ds", timeout)})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"results": nil,
			"message": "SQL executed successfully",
			"type":    "exec",
		})
		return
	}

	sqlToExecute := req.SQL
	if req.Limit > 0 || req.Offset > 0 {
		sqlToExecute = applyPagination(req.SQL, req.Limit, req.Offset)
	}

	var args []interface{}
	for _, p := range req.Params {
		args = append(args, p)
	}

	rows, err := userPool.Query(ctx, sqlToExecute, args...)
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
	totalRows := 0
	maxRows := appConfig.MaxRows
	if req.Limit > 0 && req.Limit < maxRows {
		maxRows = req.Limit
	}

	for rows.Next() {
		totalRows++
		if totalRows > maxRows {
			c.JSON(http.StatusOK, SQLResponse{
				Results:        results,
				Truncated:      true,
				TotalRows:      totalRows,
				OriginalLimit:  req.Limit,
				OriginalOffset: req.Offset,
			})
			return
		}

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

	c.JSON(http.StatusOK, SQLResponse{
		Results:        results,
		Truncated:      false,
		TotalRows:      totalRows,
		OriginalLimit:  req.Limit,
		OriginalOffset: req.Offset,
	})
}

func isDDLStatement(sql string) bool {
	sql = strings.ToUpper(strings.TrimSpace(sql))
	ddlKeywords := []string{"CREATE", "ALTER", "DROP", "TRUNCATE", "GRANT", "REVOKE"}
	for _, keyword := range ddlKeywords {
		if strings.HasPrefix(sql, keyword) {
			return true
		}
	}
	return false
}

func applyPagination(sql string, limit, offset int) string {
	if limit > 0 {
		return fmt.Sprintf("SELECT * FROM (%s) AS _paginated LIMIT %d OFFSET %d", sql, limit, offset)
	}
	return sql
}

func getCachedPool(connString string) *pgxpool.Pool {
	if appConfig != nil {
		now := time.Now()
		if cached, ok := poolCache.Load(connString); ok {
			cp := cached.(*cachedPool)
			cp.lastUsed = now
			if now.Sub(cp.createdAt) > appConfig.PoolTTL {
				cp.pool.Close()
				poolCache.Delete(connString)
			} else {
				return cp.pool
			}
		}
	}

	pool, err := pgxpool.New(context.Background(), connString)
	if err != nil {
		return nil
	}

	poolCache.Store(connString, &cachedPool{
		pool:      pool,
		createdAt: time.Now(),
		lastUsed:  time.Now(),
	})

	return pool
}

func createDefaultExtensions(dbName string) error {
	lastSlash := strings.LastIndex(dbBaseURL, "/")
	if lastSlash == -1 {
		return fmt.Errorf("invalid dbBaseURL")
	}
	connString := dbBaseURL[:lastSlash+1] + dbName

	pool, err := pgxpool.New(context.Background(), connString)
	if err != nil {
		return err
	}
	defer pool.Close()

	extensions := []string{
		"CREATE EXTENSION IF NOT EXISTS vector",
		"CREATE EXTENSION IF NOT EXISTS \"uuid-ossp\"",
	}

	for _, ext := range extensions {
		if _, err := pool.Exec(context.Background(), ext); err != nil {
			return fmt.Errorf("failed to create extension: %s, error: %w", ext, err)
		}
	}

	return nil
}

func closeCachedPool(dbName string) {
	lastSlash := strings.LastIndex(dbBaseURL, "/")
	if lastSlash == -1 {
		return
	}
	connString := dbBaseURL[:lastSlash+1] + dbName

	if cached, ok := poolCache.LoadAndDelete(connString); ok {
		cp := cached.(*cachedPool)
		cp.pool.Close()
	}
}

func CloseAllPools() {
	poolCache.Range(func(key, value interface{}) bool {
		cp := value.(*cachedPool)
		cp.pool.Close()
		poolCache.Delete(key)
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

func containsMultipleStatements(sql string) bool {
	sql = removeComments(sql)

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

func removeComments(sql string) string {
	var result strings.Builder
	inBlockComment := false
	inLineComment := false

	for i := 0; i < len(sql); i++ {
		if i+1 < len(sql) {
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

func createDatabaseWithTemplate(template config.DatabaseTemplate, options map[string]interface{}) (string, string, error) {
	accountID, err := getOrCreateDefaultAccount(context.Background())
	if err != nil {
		return "", "", err
	}

	dbID := uuid.New()
	pgDBName := fmt.Sprintf("oc_%s", dbID.String()[:8])

	_, err = dbPool.Exec(context.Background(),
		"INSERT INTO oc_databases (id, account_id, name, postgres_db_name) VALUES ($1, $2, $3, $4)",
		dbID, accountID, pgDBName, pgDBName)
	if err != nil {
		return "", "", err
	}

	_, err = dbPool.Exec(context.Background(), fmt.Sprintf("CREATE DATABASE %s", pgDBName))
	if err != nil {
		return "", "", err
	}

	if err := createDefaultExtensions(pgDBName); err != nil {
		fmt.Printf("Warning: failed to create extensions: %v\n", err)
	}

	if err := initTemplateTables(pgDBName, template); err != nil {
		fmt.Printf("Warning: failed to init template tables: %v\n", err)
	}

	return dbID.String(), pgDBName, nil
}

func initTemplateTables(pgDBName string, template config.DatabaseTemplate) error {
	lastSlash := strings.LastIndex(dbBaseURL, "/")
	if lastSlash == -1 {
		return fmt.Errorf("invalid dbBaseURL")
	}
	connString := dbBaseURL[:lastSlash+1] + pgDBName

	pool, err := pgxpool.New(context.Background(), connString)
	if err != nil {
		return err
	}
	defer pool.Close()

	for _, table := range template.Tables {
		columns := make([]string, len(table.Columns))
		for i, col := range table.Columns {
			columns[i] = fmt.Sprintf("%s %s", col.Name, col.Type)
		}
		createSQL := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (%s)", table.Name, joinStrings(columns, ", "))
		if _, err := pool.Exec(context.Background(), createSQL); err != nil {
			return fmt.Errorf("failed to create table %s: %w", table.Name, err)
		}
	}

	for _, idx := range template.Indexes {
		var idxSQL string
		switch idx.Type {
		case "btree", "gin":
			idxSQL = fmt.Sprintf("CREATE INDEX IF NOT EXISTS %s ON %s USING %s (%s)", idx.Name, idx.Column, idx.Type, idx.Column)
		case "ivfflat":
			idxSQL = fmt.Sprintf("CREATE INDEX IF NOT EXISTS %s ON %s USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100)", idx.Name, idx.Column)
		default:
			idxSQL = fmt.Sprintf("CREATE INDEX IF NOT EXISTS %s ON %s (%s)", idx.Name, idx.Column, idx.Column)
		}
		if _, err := pool.Exec(context.Background(), idxSQL); err != nil {
			fmt.Printf("Warning: failed to create index %s: %v\n", idx.Name, err)
		}
	}

	return nil
}

func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}

func queryAllRows(pool *pgxpool.Pool, query string, args ...interface{}) ([]map[string]interface{}, error) {
	rows, err := pool.Query(context.Background(), query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return nil, err
		}
		fields := rows.FieldDescriptions()
		row := make(map[string]interface{})
		for i, field := range fields {
			row[string(field.Name)] = values[i]
		}
		results = append(results, row)
	}
	return results, rows.Err()
}
