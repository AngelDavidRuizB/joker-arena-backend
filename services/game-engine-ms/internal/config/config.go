// Package config reads the engine configuration from environment variables
// (names from section 0.4 of the team guide).
package config

import (
	"net"
	"net/url"
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

// BuildPostgresURL composes a DSN, percent-encoding user and password so that
// characters such as '@' in credentials cannot break the URL.
func BuildPostgresURL(host, port, user, password, name, sslmode string) string {
	u := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(user, password),
		Host:   net.JoinHostPort(host, port),
		Path:   "/" + name,
	}
	q := url.Values{}
	q.Set("sslmode", sslmode)
	u.RawQuery = q.Encode()
	return u.String()
}

// BuildRabbitMQURL composes an AMQP URL, percent-encoding user and password so
// that characters such as '@' in credentials cannot break the URL.
func BuildRabbitMQURL(host, port, user, password string) string {
	if host == "" {
		return ""
	}
	u := url.URL{
		Scheme: "amqp",
		User:   url.UserPassword(user, password),
		Host:   net.JoinHostPort(host, port),
	}
	return u.String()
}

// Load loads configuration from environment variables with sensible defaults.
// The PostgreSQL DSN is built from the DB_* components with a percent-encoded
// password (url.UserPassword) so credentials containing '@' (or other URL
// reserved characters) cannot break the connection string.
func Load() *Config {
	host := getenv("DB_HOST", "jokerarena-game-db")
	port := getenv("DB_PORT", "5432")
	user := getenv("DB_USER", "appuser")
	name := getenv("DB_NAME", "appdb")
	sslmode := getenv("DB_SSLMODE", "disable")
	password := os.Getenv("DB_PASSWORD")

	// Explicit full DSN override (some orchestrators prefer a single URL).
	dsn := strings.TrimSpace(os.Getenv("POSTGRES_URL"))
	if dsn == "" {
		dsn = BuildPostgresURL(host, port, user, password, name, sslmode)
	}

	// RabbitMQ: build the URL from components too, for the same reason.
	rabbitHost := getenv("RABBITMQ_HOST", "")
	rabbitPort := getenv("RABBITMQ_PORT", "5672")
	rabbitUser := getenv("RABBITMQ_USER", "jokerarena")
	rabbitPassword := os.Getenv("RABBITMQ_PASSWORD")
	rabbitURL := strings.TrimSpace(os.Getenv("RABBITMQ_URL"))
	if rabbitURL == "" {
		rabbitURL = BuildRabbitMQURL(rabbitHost, rabbitPort, rabbitUser, rabbitPassword)
	}

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
