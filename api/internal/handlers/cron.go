package handlers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/robfig/cron/v3"
)

type CronJob struct {
	ID         string    `json:"id"`
	DatabaseID string    `json:"database_id"`
	Name       string    `json:"name"`
	Schedule   string    `json:"schedule"`
	SQLCommand string    `json:"sql_command"`
	IsActive   bool      `json:"is_active"`
	CreatedAt  time.Time `json:"created_at"`
}

type CronJobLog struct {
	ID         string    `json:"id"`
	JobID      string    `json:"job_id"`
	Status     string    `json:"status"`
	Message    string    `json:"message"`
	ExecutedAt time.Time `json:"executed_at"`
}

var cronScheduler *cron.Cron
var cronJobsMutex sync.RWMutex
var cronEntryMap = make(map[string]cron.EntryID)

func InitCronScheduler() {
	cronScheduler = cron.New(cron.WithSeconds())
	cronScheduler.Start()
	log.Println("Cron scheduler started")
}

func CreateCronJob(c *gin.Context) {
	var req struct {
		DatabaseID string `json:"database_id" binding:"required"`
		Name       string `json:"name" binding:"required"`
		Schedule   string `json:"schedule" binding:"required"`
		SQLCommand string `json:"sql_command" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	jobID := uuid.New().String()

	_, err := dbPool.Exec(context.Background(),
		"INSERT INTO oc_cron_jobs (id, database_id, name, schedule, sql_command) VALUES ($1, $2, $3, $4, $5)",
		jobID, req.DatabaseID, req.Name, req.Schedule, req.SQLCommand)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	scheduleJob(jobID, req.DatabaseID, req.Schedule, req.SQLCommand)

	c.JSON(http.StatusCreated, gin.H{
		"id":          jobID,
		"database_id": req.DatabaseID,
		"name":        req.Name,
		"schedule":    req.Schedule,
		"sql_command": req.SQLCommand,
		"is_active":   true,
	})
}

func scheduleJob(jobID, databaseID, schedule, sqlCommand string) {
	if cronScheduler == nil {
		InitCronScheduler()
	}

	cronJobsMutex.Lock()
	defer cronJobsMutex.Unlock()

	if _, exists := cronEntryMap[jobID]; exists {
		cronScheduler.Remove(cronEntryMap[jobID])
	}

	entryID, err := cronScheduler.AddFunc(schedule, func() {
		executeCronJob(jobID, databaseID, sqlCommand)
	})
	if err != nil {
		log.Printf("Failed to schedule job %s: %v", jobID, err)
		return
	}

	cronEntryMap[jobID] = entryID
	log.Printf("Cron job %s scheduled: %s", jobID, schedule)
}

func executeCronJob(jobID, databaseID, sqlCommand string) {
	var pgDBName string
	err := dbPool.QueryRow(context.Background(),
		"SELECT postgres_db_name FROM oc_databases WHERE id = $1", databaseID).Scan(&pgDBName)
	if err != nil {
		logJobExecution(jobID, "failed", fmt.Sprintf("Database not found: %v", err))
		return
	}

	connString := dbBaseURL[:strings.LastIndex(dbBaseURL, "/")+1] + pgDBName
	userPool, err := pgxpool.New(context.Background(), connString)
	if err != nil {
		logJobExecution(jobID, "failed", fmt.Sprintf("Connection failed: %v", err))
		return
	}
	defer userPool.Close()

	_, err = userPool.Exec(context.Background(), sqlCommand)
	if err != nil {
		logJobExecution(jobID, "failed", fmt.Sprintf("SQL execution failed: %v", err))
		return
	}

	logJobExecution(jobID, "success", "Job executed successfully")
}

func logJobExecution(jobID, status, message string) {
	logID := uuid.New().String()
	_, err := dbPool.Exec(context.Background(),
		"INSERT INTO oc_cron_job_logs (id, job_id, status, message, executed_at) VALUES ($1, $2, $3, $4, $5)",
		logID, jobID, status, message, time.Now())
	if err != nil {
		log.Printf("Failed to log job execution: %v", err)
	}
	log.Printf("Cron job %s: %s - %s", jobID, status, message)
}

func ListCronJobs(c *gin.Context) {
	databaseID := c.Query("database_id")
	if databaseID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "database_id is required"})
		return
	}

	rows, err := dbPool.Query(context.Background(),
		"SELECT id, database_id, name, schedule, sql_command, is_active, created_at FROM oc_cron_jobs WHERE database_id = $1",
		databaseID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	var jobs []CronJob
	for rows.Next() {
		var j CronJob
		if err := rows.Scan(&j.ID, &j.DatabaseID, &j.Name, &j.Schedule, &j.SQLCommand, &j.IsActive, &j.CreatedAt); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		jobs = append(jobs, j)
	}

	c.JSON(http.StatusOK, jobs)
}

func DeleteCronJob(c *gin.Context) {
	id := c.Param("id")

	cronJobsMutex.Lock()
	if entryID, exists := cronEntryMap[id]; exists {
		cronScheduler.Remove(entryID)
		delete(cronEntryMap, id)
	}
	cronJobsMutex.Unlock()

	_, err := dbPool.Exec(context.Background(), "DELETE FROM oc_cron_jobs WHERE id = $1", id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "cron job deleted"})
}

func GetCronJobLogs(c *gin.Context) {
	jobID := c.Param("id")
	if jobID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "job_id is required"})
		return
	}

	rows, err := dbPool.Query(context.Background(),
		"SELECT id, job_id, status, message, executed_at FROM oc_cron_job_logs WHERE job_id = $1 ORDER BY executed_at DESC LIMIT 100",
		jobID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	var logs []CronJobLog
	for rows.Next() {
		var l CronJobLog
		if err := rows.Scan(&l.ID, &l.JobID, &l.Status, &l.Message, &l.ExecutedAt); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		logs = append(logs, l)
	}

	c.JSON(http.StatusOK, logs)
}

func LoadActiveCronJobs() {
	rows, err := dbPool.Query(context.Background(),
		"SELECT id, database_id, schedule, sql_command FROM oc_cron_jobs WHERE is_active = true")
	if err != nil {
		log.Printf("Failed to load cron jobs: %v", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var id, databaseID, schedule, sqlCommand string
		if err := rows.Scan(&id, &databaseID, &schedule, &sqlCommand); err != nil {
			continue
		}
		scheduleJob(id, databaseID, schedule, sqlCommand)
	}
}
