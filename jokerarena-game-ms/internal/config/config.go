package config

import (
	"os"
)

// Config holds runtime configuration parameters for the Game Engine microservice.
type Config struct {
	Port              string
	DatabaseURL       string
	TelemetryGRPCAddr string
}

// Load loads configuration from environment variables with sensible defaults.
func Load() *Config {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	dbURL := os.Getenv("DB_URL")
	if dbURL == "" {
		dbURL = os.Getenv("GAME_ENGINE_DB_URL")
	}
	if dbURL == "" {
		dbURL = "postgres://postgres:postgres@jokerarena-game-db:5432/jokerarena_db?sslmode=disable"
	}

	telemetryAddr := os.Getenv("TELEMETRY_GRPC_ADDR")
	if telemetryAddr == "" {
		telemetryAddr = "jokerarena-telemetry-ms:50051"
	}

	return &Config{
		Port:              port,
		DatabaseURL:       dbURL,
		TelemetryGRPCAddr: telemetryAddr,
	}
}
