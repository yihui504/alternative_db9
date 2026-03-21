package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type HealthResponse struct {
	Status        string            `json:"status"`
	Version       string            `json:"version"`
	UptimeSeconds int64             `json:"uptime_seconds"`
	Components    map[string]string `json:"components"`
}

func HealthCheck(c *gin.Context) {
	components := make(map[string]string)

	components["postgresql"] = "connected"
	if err := dbPool.Ping(context.Background()); err != nil {
		components["postgresql"] = "disconnected"
	}

	components["minio"] = "unknown"
	if minioClient != nil {
		components["minio"] = "connected"
	}

	if available, _ := CheckOllamaAvailable(); available {
		components["ollama"] = "connected"
	} else {
		components["ollama"] = "disconnected"
	}

	overallStatus := "healthy"
	for _, status := range components {
		if status == "disconnected" {
			overallStatus = "degraded"
			break
		}
	}

	c.JSON(http.StatusOK, HealthResponse{
		Status:        overallStatus,
		Version:       "v1.1.0",
		UptimeSeconds: int64(time.Since(startTime).Seconds()),
		Components:    components,
	})
}
