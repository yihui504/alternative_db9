package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/openclaw-db9/api/internal/fsbridge"
)

var fileBridgeService *fsbridge.FileBridgeService

func InitFileBridgeService() {
	fileBridgeService = fsbridge.NewFileBridgeService(minioClient, minioBucket)
}

// QueryFile 用 SQL-like 方式查询文件
func QueryFile(c *gin.Context) {
	databaseID := c.Query("database_id")
	filePath := c.Query("path")

	if databaseID == "" || filePath == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "database_id and path are required",
		})
		return
	}

	rows, err := fileBridgeService.QueryFileAsRows(c.Request.Context(), databaseID, filePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"results": rows,
		"count":   len(rows),
	})
}
