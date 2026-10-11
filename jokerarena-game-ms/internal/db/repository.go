package db

import (
	"context"
	"errors"

	"jokerarena-game-ms/internal/models"
)

var (
	// ErrUnavailable is returned when the storage backend cannot be reached.
	ErrUnavailable = errors.New("storage unavailable")
	// ErrNotFound is returned when the requested player or match does not exist.
	ErrNotFound = errors.New("not found")
	// ErrCodeTaken is returned when a generated room code already exists.
	ErrCodeTaken = errors.New("room code already taken")
	// ErrInvalidID is returned when an identifier does not have the expected format.
	ErrInvalidID = errors.New("invalid identifier")
)

// Repository defines data access operations over the team's relational schema
// (players, rooms, matches). Identifiers are numeric SERIAL values exposed as strings.
type Repository interface {
	// RegisterPlayer performs idempotent player registration by alias (returns player, isNew, error).
	RegisterPlayer(ctx context.Context, alias string) (*models.Player, bool, error)

	// GetPlayerByID retrieves a player by numeric id (string form).
	GetPlayerByID(ctx context.Context, id string) (*models.Player, error)

	// CreateRoomMatch transactionally creates the room and the participation rows
	// (host and optional bot) with result WAITING. Returns the match id (host row).
	// Returns ErrCodeTaken if the room code already exists.
	CreateRoomMatch(ctx context.Context, roomCode string, targetScore int, hostPlayerID string, withBot bool) (string, error)

	// GetMatchByCode retrieves match metadata and participant ids by room code (case-insensitive).
	GetMatchByCode(ctx context.Context, roomCode string) (*models.Match, []string, error)

	// MarkMatchInProgress sets result IN_PROGRESS on every participation row still WAITING.
	MarkMatchInProgress(ctx context.Context, roomCode string) error

	// FinishMatch stores WIN/LOSE (host vs bot opponent) and ended_at for the match.
	FinishMatch(ctx context.Context, roomCode string, hostWon bool) error

	// Ping checks storage connectivity.
	Ping(ctx context.Context) error

	// Mode returns the storage mode label ("postgres" or "memory").
	Mode() string

	// Close terminates any active connection pools.
	Close()
}
