package db

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// getEnvInt reads an environment variable as an integer with a default value
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil && intValue > 0 {
			return intValue
		}
		log.Printf("Invalid value for %s, using default: %d", key, defaultValue)
	}
	return defaultValue
}

// getEnvDuration reads an environment variable as a duration with a default value
func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil && duration > 0 {
			return duration
		}
		log.Printf("Invalid value for %s, using default: %v", key, defaultValue)
	}
	return defaultValue
}

// NewPool creates a new database connection pool with best practices
func NewPool(ctx context.Context, connString string) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("unable to parse database config: %w", err)
	}

	// Configure connection pool with environment variables or defaults
	config.MaxConns = int32(getEnvInt("DB_MAX_CONNECTIONS", 25))
	config.MinConns = int32(getEnvInt("DB_MIN_CONNECTIONS", 5))
	config.MaxConnLifetime = getEnvDuration("DB_MAX_CONN_LIFETIME", 1*time.Hour)
	config.MaxConnIdleTime = getEnvDuration("DB_MAX_CONN_IDLE_TIME", 30*time.Minute)
	config.HealthCheckPeriod = getEnvDuration("DB_HEALTH_CHECK_PERIOD", 1*time.Minute)
	config.MaxConnLifetimeJitter = getEnvDuration("DB_MAX_CONN_LIFETIME_JITTER", 5*time.Minute)

	// Ensure min connections don't exceed max connections
	if config.MinConns > config.MaxConns {
		log.Printf("DB_MIN_CONNECTIONS (%d) cannot be greater than DB_MAX_CONNECTIONS (%d), setting min to max", config.MinConns, config.MaxConns)
		config.MinConns = config.MaxConns
	}

	log.Printf("Database pool configured - Min: %d, Max: %d, Max Lifetime: %v, Max Idle: %v",
		config.MinConns, config.MaxConns, config.MaxConnLifetime, config.MaxConnIdleTime)

	// Enable prepared statement cache for better performance (configurable)
	statementCacheSize := getEnvInt("DB_STATEMENT_CACHE_SIZE", 100)
	config.ConnConfig.RuntimeParams["statement_cache_mode"] = "prepare"
	config.ConnConfig.RuntimeParams["statement_cache_size"] = strconv.Itoa(statementCacheSize)

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("unable to create connection pool: %w", err)
	}

	// Verify connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("unable to ping database: %w", err)
	}

	log.Println("Database connection pool created successfully")
	return pool, nil
}

// ClosePool gracefully closes the database connection pool
func ClosePool(pool *pgxpool.Pool) {
	if pool != nil {
		pool.Close()
		log.Println("Database connection pool closed")
	}
}
