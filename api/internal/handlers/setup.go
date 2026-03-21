package handlers

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/openclaw-db9/api/internal/config"
)

type QuickSetupRequest struct {
	Template string                 `json:"template" binding:"required"`
	Name     string                 `json:"name" binding:"required"`
	Options  map[string]interface{} `json:"options,omitempty"`
}

type QuickSetupResponse struct {
	DatabaseID       string   `json:"database_id"`
	DatabaseName     string   `json:"database_name"`
	ConnectionString string   `json:"connection_string"`
	DashboardURL     string   `json:"dashboard_url"`
	Tables           []string `json:"tables_created"`
	VectorEnabled    bool     `json:"vector_enabled"`
}

func QuickSetup(c *gin.Context) {
	var req QuickSetupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	template, ok := config.Templates[req.Template]
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":     fmt.Sprintf("unknown template: %s", req.Template),
			"available": []string{"ai-memory", "workflow-state", "knowledge-base"},
		})
		return
	}

	dbID, pgDBName, err := createDatabaseWithTemplate(template, req.Options)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	tables := make([]string, len(template.Tables))
	for i, t := range template.Tables {
		tables[i] = t.Name
	}

	c.JSON(http.StatusCreated, QuickSetupResponse{
		DatabaseID:       dbID,
		DatabaseName:     pgDBName,
		ConnectionString: fmt.Sprintf("postgresql://postgres:postgres@localhost:5432/%s", pgDBName),
		DashboardURL:     "http://localhost:8080/",
		Tables:          tables,
		VectorEnabled:    template.VectorDim > 0,
	})
}

type ListTemplatesResponse struct {
	Templates []config.DatabaseTemplate `json:"templates"`
}

func ListTemplates(c *gin.Context) {
	templates := make([]config.DatabaseTemplate, 0, len(config.Templates))
	for _, t := range config.Templates {
		templates = append(templates, t)
	}
	c.JSON(http.StatusOK, ListTemplatesResponse{Templates: templates})
}
