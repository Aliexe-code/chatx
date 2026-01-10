package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"websocket-demo/internal/hub"
	"websocket-demo/internal/server"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hub := hub.NewHub(ctx)
	go hub.Run()

	srv := server.NewServer(hub)
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
