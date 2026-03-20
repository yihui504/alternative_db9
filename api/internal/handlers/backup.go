package handlers

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
)

type Backup struct {
	ID          string    `json:"id"`
	DatabaseID  string    `json:"database_id"`
	Name        string    `json:"name"`
	Size        int64     `json:"size"`
	BucketPath  string    `json:"bucket_path"`
	CreatedAt   time.Time `json:"created_at"`
}

var backupBucket = "oc-db9-backups"

func init() {
	ctx := context.Background()
	if minioClient != nil {
		exists, _ := minioClient.BucketExists(ctx, backupBucket)
		if !exists {
			minioClient.MakeBucket(ctx, backupBucket, minio.MakeBucketOptions{})
		}
	}
}

func CreateBackup(c *gin.Context) {
	var req struct {
		DatabaseID string `json:"database_id" binding:"required"`
		Name       string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var pgDBName string
	err := dbPool.QueryRow(context.Background(),
		"SELECT postgres_db_name FROM oc_databases WHERE id = $1", req.DatabaseID).Scan(&pgDBName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "database not found"})
		return
	}

	backupID := uuid.New().String()
	backupName := req.Name
	if backupName == "" {
		backupName = fmt.Sprintf("backup_%s", time.Now().Format("20060102_150405"))
	}

	dumpFile := fmt.Sprintf("/tmp/%s_%s.dump", pgDBName, backupID)
	
	pgPassword := os.Getenv("PGPASSWORD")
	if pgPassword == "" {
		pgPassword = "postgres"
	}

	cmd := exec.Command("pg_dump", "-Fc", "-U", "postgres", "-h", "postgres", "-d", pgDBName, "-f", dumpFile)
	cmd.Env = append(os.Environ(), fmt.Sprintf("PGPASSWORD=%s", pgPassword))
	output, err := cmd.CombinedOutput()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("pg_dump failed: %v, output: %s", err, string(output))})
		return
	}

	fileInfo, _ := os.Stat(dumpFile)
	
	objectName := fmt.Sprintf("%s/%s.dump", req.DatabaseID, backupID)
	_, err = minioClient.FPutObject(context.Background(), backupBucket, objectName, dumpFile, minio.PutObjectOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to upload backup: %v", err)})
		return
	}

	os.Remove(dumpFile)

	_, err = dbPool.Exec(context.Background(),
		"INSERT INTO oc_backups (id, database_id, name, size, bucket_path) VALUES ($1, $2, $3, $4, $5)",
		backupID, req.DatabaseID, backupName, fileInfo.Size(), objectName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":           backupID,
		"database_id":  req.DatabaseID,
		"name":         backupName,
		"size":         fileInfo.Size(),
		"bucket_path":  objectName,
	})
}

func ListBackups(c *gin.Context) {
	databaseID := c.Query("database_id")
	if databaseID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "database_id is required"})
		return
	}

	rows, err := dbPool.Query(context.Background(),
		"SELECT id, database_id, name, size, bucket_path, created_at FROM oc_backups WHERE database_id = $1 ORDER BY created_at DESC",
		databaseID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	var backups []Backup
	for rows.Next() {
		var b Backup
		if err := rows.Scan(&b.ID, &b.DatabaseID, &b.Name, &b.Size, &b.BucketPath, &b.CreatedAt); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		backups = append(backups, b)
	}

	c.JSON(http.StatusOK, backups)
}

func RestoreBackup(c *gin.Context) {
	var req struct {
		BackupID   string `json:"backup_id" binding:"required"`
		DatabaseID string `json:"database_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var backup Backup
	err := dbPool.QueryRow(context.Background(),
		"SELECT id, database_id, name, size, bucket_path, created_at FROM oc_backups WHERE id = $1", req.BackupID).Scan(
		&backup.ID, &backup.DatabaseID, &backup.Name, &backup.Size, &backup.BucketPath, &backup.CreatedAt)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "backup not found"})
		return
	}

	targetDBID := req.DatabaseID
	if targetDBID == "" {
		targetDBID = backup.DatabaseID
	}

	var pgDBName string
	err = dbPool.QueryRow(context.Background(),
		"SELECT postgres_db_name FROM oc_databases WHERE id = $1", targetDBID).Scan(&pgDBName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "target database not found"})
		return
	}

	dumpFile := fmt.Sprintf("/tmp/restore_%s.dump", backup.ID)
	
	err = minioClient.FGetObject(context.Background(), backupBucket, backup.BucketPath, dumpFile, minio.GetObjectOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to download backup: %v", err)})
		return
	}
	defer os.Remove(dumpFile)

	pgPassword := os.Getenv("PGPASSWORD")
	if pgPassword == "" {
		pgPassword = "postgres"
	}

	cmd := exec.Command("pg_restore", "-U", "postgres", "-h", "postgres", "-d", pgDBName, "--clean", "--if-exists", dumpFile)
	cmd.Env = append(os.Environ(), fmt.Sprintf("PGPASSWORD=%s", pgPassword))
	output, err := cmd.CombinedOutput()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("pg_restore failed: %v, output: %s", err, string(output))})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":     "backup restored successfully",
		"backup_id":   backup.ID,
		"database_id": targetDBID,
	})
}

func DeleteBackup(c *gin.Context) {
	id := c.Param("id")

	var bucketPath string
	err := dbPool.QueryRow(context.Background(),
		"SELECT bucket_path FROM oc_backups WHERE id = $1", id).Scan(&bucketPath)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "backup not found"})
		return
	}

	err = minioClient.RemoveObject(context.Background(), backupBucket, bucketPath, minio.RemoveObjectOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to delete backup file: %v", err)})
		return
	}

	_, err = dbPool.Exec(context.Background(), "DELETE FROM oc_backups WHERE id = $1", id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "backup deleted"})
}

func DownloadBackup(c *gin.Context) {
	id := c.Param("id")

	var bucketPath, name string
	err := dbPool.QueryRow(context.Background(),
		"SELECT bucket_path, name FROM oc_backups WHERE id = $1", id).Scan(&bucketPath, &name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "backup not found"})
		return
	}

	object, err := minioClient.GetObject(context.Background(), backupBucket, bucketPath, minio.GetObjectOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer object.Close()

	filename := fmt.Sprintf("%s.dump", strings.ReplaceAll(name, " ", "_"))
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Stream(func(w io.Writer) bool {
		_, err := io.Copy(w, object)
		return err == nil
	})
}
