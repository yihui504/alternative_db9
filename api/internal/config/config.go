package config

import "os"

type Config struct {
	DatabaseURL    string
	MinioEndpoint  string
	MinioAccessKey string
	MinioSecretKey string
	OllamaEndpoint string
	APIPort        string
	Environment    string
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
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
