package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"jokerarena-game-ms/internal/config"
	"jokerarena-game-ms/internal/db"
	"jokerarena-game-ms/internal/handlers"
	"jokerarena-game-ms/internal/room"
)

func main() {
	cfg := config.Load()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize Resilient Database Repository (PostgreSQL via pgxpool with Memory fallback)
	repo := db.NewResilientRepository(ctx, cfg.DatabaseURL)
	defer repo.Close()

	// Initialize In-Memory Active Room Session Manager
	roomMgr := room.NewRoomManager()

	// Wire HTTP & WebSocket Handlers and Router
	h := handlers.NewHandler(repo, roomMgr)
	router := handlers.NewRouter(h)

	addr := fmt.Sprintf("0.0.0.0:%s", cfg.Port)
	server := &http.Server{
		Addr:              addr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// Server startup log
	log.Printf("==========================================================")
	log.Printf("JokerArena Game Engine listening on http://%s", addr)
	log.Printf("Health:    http://%s/health", addr)
	log.Printf("REST API:  POST /api/players, POST /api/rooms, GET /api/rooms/{code}")
	log.Printf("WebSocket: ws://%s/ws/{code}", addr)
	log.Printf("Database:  %s", cfg.DatabaseURL)
	log.Printf("==========================================================")

	// Graceful shutdown handling
	serverErrors := make(chan error, 1)
	go func() {
		serverErrors <- server.ListenAndServe()
	}()

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	case sig := <-shutdown:
		log.Printf("Received shutdown signal %v, gracefully shutting down...", sig)
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("Graceful shutdown failed: %v", err)
			_ = server.Close()
		}
	}
}
