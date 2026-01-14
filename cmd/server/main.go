package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"websocket-demo/internal/db"
	"websocket-demo/internal/hub"
	"websocket-demo/internal/repository"
	"websocket-demo/internal/server"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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

	hub := hub.NewHub(ctx, repo)
	hub.LoadRoomsFromDB()
	go hub.Run()

	srv := server.NewServer(hub, repo)
	srv.SetupRoutes()

	go func() {
		if err := srv.Start(":8080"); err != nil {
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
