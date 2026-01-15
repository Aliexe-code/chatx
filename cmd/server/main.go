package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"websocket-demo/internal/config"
	"websocket-demo/internal/db"
	"websocket-demo/internal/hub"
	"websocket-demo/internal/nats"
	"websocket-demo/internal/repository"
	"websocket-demo/internal/server"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize database connection
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL environment variable is required")
	}

	pool, err := db.NewPool(ctx, dbURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()

	queries := db.New(pool)
	repo := repository.NewRepository(queries)

	// Initialize NATS client if enabled
	var natsClient *nats.Client
	if cfg.NATSEnable {
		// Retry NATS connection with backoff to handle startup timing
		maxRetries := 5
		retryDelay := 2 * time.Second

		for i := 0; i < maxRetries; i++ {
			natsCfg := nats.Config{
				URL:            cfg.NATSURL,
				MaxReconnects:  10,
				ReconnectWait:  2 * time.Second,
				Timeout:        10 * time.Second,
				EnableJetStream: false,
			}
			natsClient, err = nats.NewClient(natsCfg)
			if err == nil {
				log.Println("Successfully connected to NATS")
				break
			}

			if i < maxRetries-1 {
				log.Printf("Failed to connect to NATS (attempt %d/%d): %v, retrying in %v...", i+1, maxRetries, err, retryDelay)
				time.Sleep(retryDelay)
			}
		}

		if err != nil {
			log.Printf("Failed to connect to NATS after %d attempts: %v", maxRetries, err)
			log.Println("Continuing without NATS support")
			natsClient = nil
		}
	}

	hub := hub.NewHub(ctx, repo, natsClient)
	hub.LoadRoomsFromDB()
	go hub.Run()

	srv := server.NewServer(hub, repo)
	srv.SetupRoutes()

	go func() {
		addr := ":" + cfg.ServerPort
		if err := srv.Start(addr); err != nil {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	// Wait for interrupt signal for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	cancel()

	if err := srv.Shutdown(); err != nil {
		log.Printf("Error during server shutdown: %v", err)
	}

	log.Println("Server stopped")
}
