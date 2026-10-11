package config

import (
	"net"
	"net/url"
	"os"
	"strings"
)

// Config holds runtime configuration parameters for the Game Engine microservice.
type Config struct {
	Port              string
	StorageMode       string // "postgres" (default) or "memory" (tests only)
	DatabaseURL       string // full DSN, includes the percent-encoded password (never log it)
	DatabaseTarget    string // host:port/dbname, safe to log
	TelemetryGRPCAddr string
	AllowedOrigins    []string // extra origins allowed for WebSocket/CORS (same-origin always allowed)
}

func getenv(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

// BuildPostgresURL composes a DSN, percent-encoding user and password so that characters
// such as '@' in credentials cannot break the URL.
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

// Load loads configuration from environment variables with sensible defaults.
func Load() *Config {
	host := getenv("DB_HOST", "jokerarena-game-db")
	port := getenv("DB_PORT", "5432")
	user := getenv("DB_USER", "appuser")
	name := getenv("DB_NAME", "appdb")
	sslmode := getenv("DB_SSLMODE", "disable")
	password := os.Getenv("DB_PASSWORD")

	dsn := strings.TrimSpace(os.Getenv("DB_URL")) // explicit full DSN override
	if dsn == "" {
		dsn = BuildPostgresURL(host, port, user, password, name, sslmode)
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

	return &Config{
		Port:              getenv("PORT", "8080"),
		StorageMode:       mode,
		DatabaseURL:       dsn,
		DatabaseTarget:    net.JoinHostPort(host, port) + "/" + name,
		TelemetryGRPCAddr: getenv("TELEMETRY_GRPC_ADDR", "jokerarena-telemetry-ms:50051"),
		AllowedOrigins:    origins,
	}
}
