package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ParameterizedQueryRequest struct {
	SQL        string        `json:"sql" binding:"required"`
	Params     []interface{} `json:"params"`
	ParamTypes []string      `json:"param_types"`
	Timeout    int           `json:"timeout"`
	Limit      int           `json:"limit"`
	Offset     int           `json:"offset"`
}

type ParameterizedQueryResponse struct {
	Results        []map[string]interface{} `json:"results"`
	Truncated      bool                    `json:"truncated"`
	TotalRows      int                     `json:"total_rows"`
	OriginalLimit  int                     `json:"original_limit,omitempty"`
	OriginalOffset int                     `json:"original_offset,omitempty"`
}

func ParameterizedQuery(c *gin.Context) {
	id := c.Param("id")

	var req ParameterizedQueryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if len(req.Params) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "params are required for parameterized query"})
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

	sqlToExecute := req.SQL
	if req.Limit > 0 || req.Offset > 0 {
		sqlToExecute = applyPagination(req.SQL, req.Limit, req.Offset)
	}

	var rows pgx.Rows
	var queryErr error

	if len(req.ParamTypes) > 0 {
		rows, queryErr = userPool.Query(ctx, sqlToExecute, req.Params...)
	} else {
		rows, queryErr = userPool.Query(ctx, sqlToExecute, req.Params...)
	}

	if queryErr != nil {
		if ctx.Err() == context.DeadlineExceeded {
			c.JSON(http.StatusRequestTimeout, gin.H{"error": fmt.Sprintf("Query timed out after %ds", timeout)})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": queryErr.Error()})
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
			c.JSON(http.StatusOK, ParameterizedQueryResponse{
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

	c.JSON(http.StatusOK, ParameterizedQueryResponse{
		Results:        results,
		Truncated:      false,
		TotalRows:      totalRows,
		OriginalLimit:  req.Limit,
		OriginalOffset: req.Offset,
	})
}

func getPoolForDatabase(pgDBName string) *pgxpool.Pool {
	lastSlash := strings.LastIndex(dbBaseURL, "/")
	if lastSlash == -1 {
		return nil
	}
	connString := dbBaseURL[:lastSlash+1] + pgDBName
	return getCachedPool(connString)
}
