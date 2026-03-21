package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Branch struct {
	ID           string    `json:"id"`
	DatabaseID   string    `json:"database_id"`
	Name         string    `json:"name"`
	SourceBranch string    `json:"source_branch"`
	SnapshotPath string    `json:"snapshot_path"`
	CreatedAt    time.Time `json:"created_at"`
}

func CreateBranch(c *gin.Context) {
	var req struct {
		DatabaseID   string `json:"database_id" binding:"required"`
		Name         string `json:"name" binding:"required"`
		SourceBranch string `json:"source_branch"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var sourceDBName string
	var err error
	if req.SourceBranch != "" {
		err = dbPool.QueryRow(context.Background(),
			"SELECT snapshot_path FROM oc_branches WHERE database_id = $1 AND name = $2",
			req.DatabaseID, req.SourceBranch).Scan(&sourceDBName)
	} else {
		err = dbPool.QueryRow(context.Background(),
			"SELECT postgres_db_name FROM oc_databases WHERE id = $1",
			req.DatabaseID).Scan(&sourceDBName)
	}
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "source database not found"})
		return
	}

	branchID := uuid.New().String()
	newDBName := fmt.Sprintf("oc_br_%s_%d", strings.ReplaceAll(req.Name, "-", "_"), time.Now().Unix())

	_, err = dbPool.Exec(context.Background(),
		fmt.Sprintf("CREATE DATABASE %s WITH TEMPLATE %s", newDBName, sourceDBName))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to create branch database: %v", err)})
		return
	}

	_, err = dbPool.Exec(context.Background(),
		"INSERT INTO oc_branches (id, database_id, name, source_branch, snapshot_path) VALUES ($1, $2, $3, $4, $5)",
		branchID, req.DatabaseID, req.Name, req.SourceBranch, newDBName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":            branchID,
		"database_id":   req.DatabaseID,
		"name":          req.Name,
		"source_branch": req.SourceBranch,
		"snapshot_path": newDBName,
	})
}

func ListBranches(c *gin.Context) {
	databaseID := c.Query("database_id")
	if databaseID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "database_id is required"})
		return
	}

	rows, err := dbPool.Query(context.Background(),
		"SELECT id, database_id, name, source_branch, snapshot_path, created_at FROM oc_branches WHERE database_id = $1",
		databaseID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	var branches []Branch
	for rows.Next() {
		var b Branch
		if err := rows.Scan(&b.ID, &b.DatabaseID, &b.Name, &b.SourceBranch, &b.SnapshotPath, &b.CreatedAt); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		branches = append(branches, b)
	}

	c.JSON(http.StatusOK, branches)
}

func DeleteBranch(c *gin.Context) {
	id := c.Param("id")

	var snapshotPath string
	err := dbPool.QueryRow(context.Background(),
		"SELECT snapshot_path FROM oc_branches WHERE id = $1", id).Scan(&snapshotPath)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "branch not found"})
		return
	}

	_, err = dbPool.Exec(context.Background(), fmt.Sprintf("DROP DATABASE IF EXISTS %s", snapshotPath))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to drop database: %v", err)})
		return
	}

	_, err = dbPool.Exec(context.Background(), "DELETE FROM oc_branches WHERE id = $1", id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "branch deleted"})
}

func GetBranch(c *gin.Context) {
	id := c.Param("id")

	var branch Branch
	err := dbPool.QueryRow(context.Background(),
		"SELECT id, database_id, name, source_branch, snapshot_path, created_at FROM oc_branches WHERE id = $1",
		id).Scan(&branch.ID, &branch.DatabaseID, &branch.Name, &branch.SourceBranch, &branch.SnapshotPath, &branch.CreatedAt)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "branch not found"})
		return
	}

	c.JSON(http.StatusOK, branch)
}
