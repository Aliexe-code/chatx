package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL string
	ServerPort  string
	ServerHost  string
	JWTSecret   string
	JWTExpiry   string
	TestMode    bool
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	// Load .env file if it exists
	if err := godotenv.Load(); err != nil {
		// .env file is optional, so we don't return an error
		fmt.Println("No .env file found, using environment variables")
	}

	cfg := &Config{
		DatabaseURL: getEnv("DATABASE_URL", ""),
		ServerPort:  getEnv("SERVER_PORT", "8080"),
		ServerHost:  getEnv("SERVER_HOST", "0.0.0.0"),
		JWTSecret:   getEnv("JWT_SECRET", ""),
		JWTExpiry:   getEnv("JWT_EXPIRATION", "24h"),
		TestMode:    getEnv("TEST_MODE", "false") == "true",
	}

	// Validate required fields
	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL environment variable is required")
	}

	// Validate JWT secret
	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET environment variable is required")
	}

	// Ensure JWT secret is at least 32 characters
	if len(cfg.JWTSecret) < 32 {
		return nil, fmt.Errorf("JWT_SECRET must be at least 32 characters long")
	}

	return cfg, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
