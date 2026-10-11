package db

import (
	"context"

	"jokerarena-game-ms/internal/models"
)

// Repository defines data access operations for player registration and match persistence.
type Repository interface {
	// RegisterPlayer performs idempotent player registration (returns player, isNew, error).
	RegisterPlayer(ctx context.Context, alias string) (*models.Player, bool, error)

	// GetPlayerByID retrieves a player by their UUID string.
	GetPlayerByID(ctx context.Context, id string) (*models.Player, error)

	// CreateRoomMatch transactionally creates a match and links match_players (host and optional bot).
	CreateRoomMatch(ctx context.Context, matchID, roomCode string, targetScore int, hostPlayerID string, withBot bool) error

	// GetMatchByCode retrieves match metadata and player IDs by room code (case-insensitive).
	GetMatchByCode(ctx context.Context, roomCode string) (*models.Match, []string, error)

	// EnsureBotPlayer idempotently seeds the default Bot-JokerBot player.
	EnsureBotPlayer(ctx context.Context) error

	// Ping checks database connectivity.
	Ping(ctx context.Context) error

	// Close terminates any active connection pools.
	Close()
}
