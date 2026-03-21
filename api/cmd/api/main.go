package main

import (
	"context"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/openclaw-db9/api/internal/config"
	"github.com/openclaw-db9/api/internal/handlers"
	"github.com/openclaw-db9/api/internal/middleware"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	cfg := config.Load()

	dbPool, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer dbPool.Close()

	if err := dbPool.Ping(context.Background()); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}
	log.Println("Connected to PostgreSQL")

	if err := handlers.InitMinio(cfg.MinioEndpoint, cfg.MinioAccessKey, cfg.MinioSecretKey); err != nil {
		log.Printf("Warning: Failed to initialize MinIO: %v", err)
	} else {
		log.Println("Connected to MinIO")
		handlers.InitFileBridgeService()
		log.Println("File bridge service initialized")
	}

	handlers.SetDBPool(dbPool)

	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.Default()

	router.Use(middleware.CORS())
	router.Use(middleware.Logger())

	router.GET("/health", handlers.HealthCheck)

	v1 := router.Group("/api/v1")
	{
		monitor := v1.Group("/monitor")
		{
			monitor.GET("/system", handlers.GetSystemStats)
			monitor.GET("/stats", handlers.GetDatabaseStats)
			monitor.GET("/health", handlers.GetHealthStatus)
		}

		databases := v1.Group("/databases")
		{
			databases.POST("", handlers.CreateDatabase)
			databases.GET("", handlers.ListDatabases)
			databases.GET("/:id", handlers.GetDatabase)
			databases.DELETE("/:id", handlers.DeleteDatabase)
			databases.POST("/:id/sql", handlers.ExecuteSQL)
			databases.GET("/:id/connect", handlers.GetConnectionInfo)
		}

		files := v1.Group("/files")
		{
			files.POST("/upload", handlers.UploadFile)
			files.GET("", handlers.ListFiles)
			files.GET("/:id", handlers.DownloadFile)
			files.DELETE("/:id", handlers.DeleteFile)
			files.GET("/query", handlers.QueryFile)
		}

		branches := v1.Group("/branches")
		{
			branches.POST("", handlers.CreateBranch)
			branches.GET("", handlers.ListBranches)
			branches.DELETE("/:id", handlers.DeleteBranch)
		}

		cron := v1.Group("/cron")
		{
			cron.POST("", handlers.CreateCronJob)
			cron.GET("", handlers.ListCronJobs)
			cron.DELETE("/:id", handlers.DeleteCronJob)
			cron.GET("/:id/logs", handlers.GetCronJobLogs)
		}

		embeddings := v1.Group("/embeddings")
		{
			embeddings.POST("/generate", handlers.GenerateEmbedding)
			embeddings.POST("/tables", handlers.CreateVectorTable)
			embeddings.POST("/insert", handlers.InsertVector)
			embeddings.POST("/search", handlers.SimilaritySearch)
		}

		backups := v1.Group("/backups")
		{
			backups.POST("", handlers.CreateBackup)
			backups.GET("", handlers.ListBackups)
			backups.POST("/restore", handlers.RestoreBackup)
			backups.DELETE("/:id", handlers.DeleteBackup)
			backups.GET("/:id/download", handlers.DownloadBackup)
		}
	}

	port := cfg.APIPort
	if port == "" {
		port = "8080"
	}

	log.Printf("Starting API server on port %s", port)
	if err := router.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
