package handlers

import (
	"context"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SystemStats struct {
	Uptime     string `json:"uptime"`
	GoVersion  string `json:"go_version"`
	GoRoutines int    `json:"goroutines"`
	MemoryMB   uint64 `json:"memory_mb"`
	CPUCount   int    `json:"cpu_count"`
}

type DatabaseStats struct {
	TotalDatabases int `json:"total_databases"`
	TotalFiles     int `json:"total_files"`
	TotalBranches  int `json:"total_branches"`
	TotalCronJobs  int `json:"total_cron_jobs"`
}

type HealthStatus struct {
	PostgreSQL string `json:"postgresql"`
	MinIO      string `json:"minio"`
	API        string `json:"api"`
}

var startTime = time.Now()

func GetSystemStats(c *gin.Context) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	stats := SystemStats{
		Uptime:     time.Since(startTime).String(),
		GoVersion:  runtime.Version(),
		GoRoutines: runtime.NumGoroutine(),
		MemoryMB:   m.Alloc / 1024 / 1024,
		CPUCount:   runtime.NumCPU(),
	}

	c.JSON(http.StatusOK, stats)
}

func GetDatabaseStats(c *gin.Context) {
	stats := DatabaseStats{}

	dbPool.QueryRow(context.Background(),
		"SELECT COUNT(*) FROM oc_databases").Scan(&stats.TotalDatabases)

	dbPool.QueryRow(context.Background(),
		"SELECT COUNT(*) FROM oc_files").Scan(&stats.TotalFiles)

	dbPool.QueryRow(context.Background(),
		"SELECT COUNT(*) FROM oc_branches").Scan(&stats.TotalBranches)

	dbPool.QueryRow(context.Background(),
		"SELECT COUNT(*) FROM oc_cron_jobs").Scan(&stats.TotalCronJobs)

	c.JSON(http.StatusOK, stats)
}

func GetHealthStatus(c *gin.Context) {
	status := HealthStatus{
		PostgreSQL: "healthy",
		MinIO:      "unknown",
		API:        "healthy",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := dbPool.Ping(ctx); err != nil {
		status.PostgreSQL = "unhealthy"
	}

	if minioClient != nil {
		_, err := minioClient.ListBuckets(ctx)
		if err != nil {
			status.MinIO = "unhealthy"
		} else {
			status.MinIO = "healthy"
		}
	}

	c.JSON(http.StatusOK, status)
}

func GetDatabaseMetrics(c *gin.Context) {
	dbID := c.Param("id")

	var pgDBName string
	err := dbPool.QueryRow(context.Background(),
		"SELECT postgres_db_name FROM oc_databases WHERE id = $1", dbID).Scan(&pgDBName)
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

	var tableCount, indexCount int64
	var totalRows int64
	userPool.QueryRow(context.Background(),
		"SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'public'").Scan(&tableCount)
	userPool.QueryRow(context.Background(),
		"SELECT COUNT(*) FROM pg_indexes WHERE schemaname = 'public'").Scan(&indexCount)

	rows, _ := userPool.Query(context.Background(),
		"SELECT reltuples::bigint FROM pg_class WHERE relkind = 'r' AND relnamespace = (SELECT oid FROM pg_namespace WHERE nspname = 'public')")
	defer rows.Close()
	for rows.Next() {
		var count int64
		if err := rows.Scan(&count); err == nil {
			totalRows += count
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"database_id":    dbID,
		"database_name":  pgDBName,
		"tables":         tableCount,
		"indexes":        indexCount,
		"estimated_rows": totalRows,
	})
}
