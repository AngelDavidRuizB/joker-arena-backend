package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"game-engine-ms/internal/room"
)

// Postgres implements Repository on game.matches.
type Postgres struct {
	pool *pgxpool.Pool
}

// NewPostgres creates the pool without requiring the database to be up yet: connections
// are opened lazily, so the engine starts even if PostgreSQL is still booting.
func NewPostgres(ctx context.Context, dsn string) (*Postgres, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("invalid database URL: %w", err)
	}
	cfg.MaxConns = 10
	cfg.ConnConfig.ConnectTimeout = 3 * time.Second
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return &Postgres{pool: pool}, nil
}

func wrap(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return fmt.Errorf("%w: %v", ErrUnavailable, err)
}

// CreateMatch inserts a new WAITING match.
func (p *Postgres) CreateMatch(ctx context.Context, matchID, roomCode, hostID, hostAlias string, withBot bool, targetScore int, difficulty string) error {
	_, err := p.pool.Exec(ctx, `
		INSERT INTO game.matches (match_id, room_code, host_id, host_alias, with_bot, target_score, difficulty)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (match_id) DO NOTHING`,
		matchID, roomCode, hostID, hostAlias, withBot, targetScore, difficulty)
	return wrap(err)
}

// MarkMatchInProgress flips the open match of the room to IN_PROGRESS.
func (p *Postgres) MarkMatchInProgress(ctx context.Context, roomCode string) error {
	_, err := p.pool.Exec(ctx, `
		UPDATE game.matches SET status = 'IN_PROGRESS', started_at = now()
		WHERE room_code = $1 AND status = 'WAITING'`, roomCode)
	return wrap(err)
}

// FinishMatch closes the open match of the room.
func (p *Postgres) FinishMatch(ctx context.Context, roomCode string, hostWon bool) error {
	_, err := p.pool.Exec(ctx, `
		UPDATE game.matches SET status = 'FINISHED', host_won = $2, ended_at = now()
		WHERE room_code = $1 AND status <> 'FINISHED'`, roomCode, hostWon)
	return wrap(err)
}

// RecordOutcome stores the final scores of the match.
func (p *Postgres) RecordOutcome(ctx context.Context, o room.MatchOutcome) error {
	_, err := p.pool.Exec(ctx, `
		UPDATE game.matches
		SET status = 'FINISHED', host_won = $2, host_score = $3, bot_score = $4,
		    hands_played = $5, end_reason = $6, ended_at = COALESCE(ended_at, now())
		WHERE match_id = $1`,
		o.MatchID, o.HostWon, o.HostScore, o.BotScore, o.HandsPlayed, o.Reason)
	return wrap(err)
}

// LatestMatch returns the most recent match of the room.
func (p *Postgres) LatestMatch(ctx context.Context, roomCode string) (*MatchRow, error) {
	var m MatchRow
	err := p.pool.QueryRow(ctx, `
		SELECT match_id::text, room_code, status FROM game.matches
		WHERE room_code = $1 ORDER BY created_at DESC LIMIT 1`, roomCode).
		Scan(&m.MatchID, &m.RoomCode, &m.Status)
	if err != nil {
		return nil, wrap(err)
	}
	return &m, nil
}

// Ping checks connectivity.
func (p *Postgres) Ping(ctx context.Context) error { return wrap(p.pool.Ping(ctx)) }

// Mode returns "postgres".
func (p *Postgres) Mode() string { return "postgres" }

// Close releases the pool.
func (p *Postgres) Close() { p.pool.Close() }
