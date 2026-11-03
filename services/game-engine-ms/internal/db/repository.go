// Package db persists matches in the "game" schema of PostgreSQL (game.matches).
// Players and rooms are owned by auth-identity-ms and rooms-lobby-ms; the engine only
// stores their identifiers (no foreign keys across services).
package db

import (
	"context"
	"errors"

	"game-engine-ms/internal/room"
)

var (
	// ErrUnavailable is returned when the storage backend cannot be reached.
	ErrUnavailable = errors.New("storage unavailable")
	// ErrNotFound is returned when there is no match for the room.
	ErrNotFound = errors.New("not found")
)

// Match statuses stored in game.matches.status.
const (
	StatusWaiting    = "WAITING"
	StatusInProgress = "IN_PROGRESS"
	StatusFinished   = "FINISHED"
)

// MatchRow is the latest match stored for a room.
type MatchRow struct {
	MatchID  string
	RoomCode string
	Status   string
}

// Repository is the engine's storage port. It satisfies room.Persister plus the optional
// room.MatchCreator and room.OutcomeRecorder extensions.
type Repository interface {
	CreateMatch(ctx context.Context, matchID, roomCode, hostID, hostAlias string, withBot bool, targetScore int, difficulty string) error
	MarkMatchInProgress(ctx context.Context, roomCode string) error
	FinishMatch(ctx context.Context, roomCode string, hostWon bool) error
	RecordOutcome(ctx context.Context, o room.MatchOutcome) error
	// LatestMatch returns the most recent match of a room (to resume after a restart).
	LatestMatch(ctx context.Context, roomCode string) (*MatchRow, error)
	Ping(ctx context.Context) error
	Mode() string
	Close()
}

var (
	_ room.Persister       = Repository(nil)
	_ room.MatchCreator    = Repository(nil)
	_ room.OutcomeRecorder = Repository(nil)
)
