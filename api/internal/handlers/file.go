package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

var minioClient *minio.Client
var minioBucket = "oc-db9-files"

func InitMinio(endpoint, accessKey, secretKey string) error {
	var err error
	minioClient, err = minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: false,
	})
	if err != nil {
		return err
	}

	ctx := context.Background()
	exists, err := minioClient.BucketExists(ctx, minioBucket)
	if err != nil {
		return err
	}
	if !exists {
		err = minioClient.MakeBucket(ctx, minioBucket, minio.MakeBucketOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}

type File struct {
	ID         string    `json:"id"`
	DatabaseID string    `json:"database_id"`
	Path       string    `json:"path"`
	Size       int64     `json:"size"`
	Checksum   string    `json:"checksum"`
	CreatedAt  time.Time `json:"created_at"`
}

func UploadFile(c *gin.Context) {
	databaseID := c.PostForm("database_id")
	if databaseID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "database_id is required"})
		return
	}

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	defer file.Close()

	path := c.PostForm("path")
	if path == "" {
		path = "/" + header.Filename
	}

	hash := sha256.New()
	tee := io.TeeReader(file, hash)

	objectName := fmt.Sprintf("%s%s", databaseID, path)
	_, err = minioClient.PutObject(context.Background(), minioBucket, objectName, tee, header.Size, minio.PutObjectOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	checksum := hex.EncodeToString(hash.Sum(nil))
	fileID := uuid.New().String()

	_, err = dbPool.Exec(context.Background(),
		"INSERT INTO oc_files (id, database_id, path, bucket_path, size, checksum) VALUES ($1, $2, $3, $4, $5, $6)",
		fileID, databaseID, path, objectName, header.Size, checksum)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":          fileID,
		"database_id": databaseID,
		"path":        path,
		"size":        header.Size,
		"checksum":    checksum,
	})
}

func ListFiles(c *gin.Context) {
	databaseID := c.Query("database_id")
	if databaseID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "database_id is required"})
		return
	}

	rows, err := dbPool.Query(context.Background(),
		"SELECT id, database_id, path, size, checksum, created_at FROM oc_files WHERE database_id = $1",
		databaseID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	var files []File
	for rows.Next() {
		var f File
		if err := rows.Scan(&f.ID, &f.DatabaseID, &f.Path, &f.Size, &f.Checksum, &f.CreatedAt); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		files = append(files, f)
	}

	c.JSON(http.StatusOK, files)
}

func DownloadFile(c *gin.Context) {
	id := c.Param("id")

	var bucketPath string
	err := dbPool.QueryRow(context.Background(),
		"SELECT bucket_path FROM oc_files WHERE id = $1", id).Scan(&bucketPath)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
		return
	}

	object, err := minioClient.GetObject(context.Background(), minioBucket, bucketPath, minio.GetObjectOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer object.Close()

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", bucketPath))
	c.Stream(func(w io.Writer) bool {
		_, err := io.Copy(w, object)
		return err == nil
	})
}

func DeleteFile(c *gin.Context) {
	id := c.Param("id")

	var bucketPath string
	err := dbPool.QueryRow(context.Background(),
		"SELECT bucket_path FROM oc_files WHERE id = $1", id).Scan(&bucketPath)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
		return
	}

	err = minioClient.RemoveObject(context.Background(), minioBucket, bucketPath, minio.RemoveObjectOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	_, err = dbPool.Exec(context.Background(), "DELETE FROM oc_files WHERE id = $1", id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "file deleted"})
}
