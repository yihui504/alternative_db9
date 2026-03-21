package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	DatabaseURL    string
	MinioEndpoint  string
	MinioAccessKey string
	MinioSecretKey string
	OllamaEndpoint string
	APIPort        string
	Environment    string

	QueryTimeout  int
	DDLTimeout    int
	MaxRows       int
	PoolTTL       time.Duration
	PoolIdleTTL   time.Duration
	MaxPools      int

	MaxDatabases  int
	MaxDBSizeMB   int64
}

func Load() *Config {
	return &Config{
		DatabaseURL:    getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/postgres"),
		MinioEndpoint:  getEnv("MINIO_ENDPOINT", "localhost:9000"),
		MinioAccessKey: getEnv("MINIO_ACCESS_KEY", "minioadmin"),
		MinioSecretKey: getEnv("MINIO_SECRET_KEY", "minioadmin"),
		OllamaEndpoint: getEnv("OLLAMA_ENDPOINT", "localhost:11434"),
		APIPort:        getEnv("API_PORT", "8080"),
		Environment:    getEnv("ENVIRONMENT", "development"),

		QueryTimeout: getEnvInt("OC_QUERY_TIMEOUT", 30),
		DDLTimeout:   getEnvInt("OC_DDL_TIMEOUT", 120),
		MaxRows:      getEnvInt("OC_MAX_ROWS", 1000),
		PoolTTL:      getEnvDuration("OC_POOL_TTL", 30*time.Minute),
		PoolIdleTTL:  getEnvDuration("OC_POOL_IDLE_TTL", 10*time.Minute),
		MaxPools:     getEnvInt("OC_MAX_POOLS", 50),

		MaxDatabases: getEnvInt("OC_MAX_DATABASES", 100),
		MaxDBSizeMB:  getEnvInt64("OC_MAX_DB_SIZE_MB", 500),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getEnvInt64(key string, defaultValue int64) int64 {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.ParseInt(value, 10, 64); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}
