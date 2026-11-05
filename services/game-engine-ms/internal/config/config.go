// Package config reads the engine configuration from environment variables
// (names from section 0.4 of the team guide).
package config

import (
	"os"
	"strings"
	"time"
)

// Config holds runtime configuration parameters for the Game Engine microservice.
type Config struct {
	Port            string
	StorageMode     string // "postgres" (default) or "memory" (tests / standalone)
	PostgresURL     string // never log it: contains the password
	RabbitMQURL     string // empty: events disabled (standalone)
	AuthJWKSURL     string
	LiveopsURL      string // empty: default ruleset only
	BotAIURL        string // empty: local bot strategy only
	RoomsLobbyURL   string
	AllowedOrigins  []string
	RoomIdleTimeout time.Duration // a match without clients for this long is closed as abandoned
}

func getenv(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

// Load loads configuration from environment variables with sensible defaults.
func Load() *Config {
	mode := strings.ToLower(getenv("STORAGE_MODE", "postgres"))
	if mode != "memory" {
		mode = "postgres"
	}
	var origins []string
	for _, o := range strings.Split(os.Getenv("ALLOWED_ORIGINS"), ",") {
		if o = strings.TrimSpace(o); o != "" {
			origins = append(origins, o)
		}
	}
	idle, err := time.ParseDuration(getenv("ROOM_IDLE_TIMEOUT", "15m"))
	if err != nil || idle <= 0 {
		idle = 15 * time.Minute
	}
	return &Config{
		Port:            getenv("PORT", "8081"),
		StorageMode:     mode,
		PostgresURL:     os.Getenv("POSTGRES_URL"),
		RabbitMQURL:     os.Getenv("RABBITMQ_URL"),
		AuthJWKSURL:     getenv("AUTH_JWKS_URL", "http://jokerarena-auth-identity-ms:8082/.well-known/jwks.json"),
		LiveopsURL:      os.Getenv("LIVEOPS_URL"),
		BotAIURL:        os.Getenv("BOT_AI_URL"),
		RoomsLobbyURL:   getenv("ROOMS_LOBBY_URL", "http://jokerarena-rooms-lobby-ms:8083"),
		AllowedOrigins:  origins,
		RoomIdleTimeout: idle,
	}
}
