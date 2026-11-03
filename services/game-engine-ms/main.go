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

	"game-engine-ms/internal/auth"
	"game-engine-ms/internal/botclient"
	"game-engine-ms/internal/config"
	"game-engine-ms/internal/db"
	"game-engine-ms/internal/events"
	"game-engine-ms/internal/handlers"
	"game-engine-ms/internal/lobby"
	"game-engine-ms/internal/room"
	"game-engine-ms/internal/ruleset"
)

func main() {
	cfg := config.Load()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Storage: game schema in PostgreSQL (memory only for local tests).
	var repo db.Repository
	if cfg.StorageMode == "memory" {
		log.Printf("[WARN] STORAGE_MODE=memory: matches will NOT be persisted")
		repo = db.NewMemory()
	} else {
		pg, err := db.NewPostgres(ctx, cfg.PostgresURL)
		if err != nil {
			log.Fatalf("invalid POSTGRES_URL: %v", err)
		}
		repo = pg
	}
	defer repo.Close()

	// LiveOps ruleset: default first, then GET from liveops and hot reload by events.
	rules := ruleset.NewProvider()
	rules.SeedInBackground(ctx, cfg.LiveopsURL)

	// Broker: publish domain events and consume liveops.ruleset.*.
	var publisher room.EventPublisher = events.Noop{}
	var brokerUp func() bool
	if cfg.RabbitMQURL != "" {
		bus := events.NewBus(cfg.RabbitMQURL, func(b []byte) error { return rules.ApplyJSON(b, "liveops-event") })
		go bus.Run(ctx)
		publisher, brokerUp = bus, bus.Connected
	} else {
		log.Printf("[WARN] RABBITMQ_URL not set: events disabled (standalone mode)")
	}

	// Bot: remote bot-ai-ms with the local strategy as fallback.
	var bot room.BotDecider
	if cfg.BotAIURL != "" {
		bot = botclient.New(cfg.BotAIURL)
	}

	mgr := room.NewManager(repo)
	mgr.SetDefaults(room.Options{Publisher: publisher, Rules: rules.Current, Bot: bot})
	defer mgr.Close()
	mgr.StartJanitor(ctx, 5*time.Minute, 2*time.Hour)

	router := handlers.NewRouter(handlers.Deps{
		Repo:           repo,
		Rooms:          mgr,
		Verifier:       auth.NewVerifier(cfg.AuthJWKSURL),
		Lobby:          lobby.NewClient(cfg.RoomsLobbyURL),
		Rules:          rules.Current,
		RulesSource:    rules.Source,
		BrokerUp:       brokerUp,
		AllowedOrigins: cfg.AllowedOrigins,
	})

	addr := fmt.Sprintf("0.0.0.0:%s", cfg.Port)
	server := &http.Server{Addr: addr, Handler: router, ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second}
	log.Printf("game-engine-ms listening on %s (storage=%s, bot-ai=%t, liveops=%t)",
		addr, repo.Mode(), cfg.BotAIURL != "", cfg.LiveopsURL != "")

	errs := make(chan error, 1)
	go func() { errs <- server.ListenAndServe() }()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	select {
	case err := <-errs:
		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	case sig := <-stop:
		log.Printf("received %v, shutting down", sig)
		cancel()
		sctx, scancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer scancel()
		_ = server.Shutdown(sctx)
	}
}
