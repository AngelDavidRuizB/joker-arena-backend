package db

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"jokerarena-game-ms/internal/models"
)

const BotPlayerID = "00000000-0000-0000-0000-000000000001"

// PostgresRepository implements Repository using pgxpool over PostgreSQL 16.
type PostgresRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresRepository wraps an active pgxpool as a Repository.
func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

// RegisterPlayer registers or retrieves a player by alias using idempotent upsert.
func (r *PostgresRepository) RegisterPlayer(ctx context.Context, alias string) (*models.Player, bool, error) {
	query := `
		INSERT INTO players (alias)
		VALUES ($1)
		ON CONFLICT (alias) DO UPDATE
			SET alias = EXCLUDED.alias
		RETURNING id::text, alias, total_wins, created_at, (xmax = 0) AS is_new;
	`
	var p models.Player
	var isNew bool

	err := r.pool.QueryRow(ctx, query, alias).Scan(&p.ID, &p.Alias, &p.TotalWins, &p.CreatedAt, &isNew)
	if err != nil {
		return nil, false, fmt.Errorf("register player sql error: %w", err)
	}
	return &p, isNew, nil
}

// GetPlayerByID retrieves a player by UUID string.
func (r *PostgresRepository) GetPlayerByID(ctx context.Context, id string) (*models.Player, error) {
	if !models.IsValidUUID(id) {
		return nil, fmt.Errorf("invalid UUID format: %s", id)
	}

	query := `
		SELECT id::text, alias, total_wins, created_at
		FROM players
		WHERE id = $1::uuid;
	`
	var p models.Player
	err := r.pool.QueryRow(ctx, query, id).Scan(&p.ID, &p.Alias, &p.TotalWins, &p.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("player not found: %s", id)
		}
		return nil, fmt.Errorf("query player by id error: %w", err)
	}
	return &p, nil
}

// CreateRoomMatch executes an atomic transaction creating a match and linking participants.
func (r *PostgresRepository) CreateRoomMatch(ctx context.Context, matchID, roomCode string, targetScore int, hostPlayerID string, withBot bool) error {
	if !models.IsValidUUID(matchID) {
		return fmt.Errorf("invalid match UUID format: %s", matchID)
	}
	if !models.IsValidUUID(hostPlayerID) {
		return fmt.Errorf("invalid host player UUID format: %s", hostPlayerID)
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	// 1. Insert match
	matchQuery := `
		INSERT INTO matches (id, room_code, target_score, status)
		VALUES ($1::uuid, $2, $3, 'WAITING');
	`
	if _, err := tx.Exec(ctx, matchQuery, matchID, strings.ToUpper(roomCode), targetScore); err != nil {
		return fmt.Errorf("insert match failed: %w", err)
	}

	// 2. Insert host player into match_players
	hostQuery := `
		INSERT INTO match_players (match_id, player_id, is_bot, hands_left, discards_left)
		VALUES ($1::uuid, $2::uuid, false, 4, 3);
	`
	if _, err := tx.Exec(ctx, hostQuery, matchID, hostPlayerID); err != nil {
		return fmt.Errorf("insert host match_player failed: %w", err)
	}

	// 3. Link bot participant if requested
	if withBot {
		ensureBotQuery := `
			INSERT INTO players (id, alias, total_wins)
			VALUES ($1::uuid, 'Bot-JokerBot', 0)
			ON CONFLICT (id) DO NOTHING;
		`
		if _, err := tx.Exec(ctx, ensureBotQuery, BotPlayerID); err != nil {
			return fmt.Errorf("ensure bot player failed: %w", err)
		}

		botPlayerQuery := `
			INSERT INTO match_players (match_id, player_id, is_bot, hands_left, discards_left)
			VALUES ($1::uuid, $2::uuid, true, 4, 3);
		`
		if _, err := tx.Exec(ctx, botPlayerQuery, matchID, BotPlayerID); err != nil {
			return fmt.Errorf("insert bot match_player failed: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction failed: %w", err)
	}
	return nil
}

// GetMatchByCode queries match details and participant list by room code.
func (r *PostgresRepository) GetMatchByCode(ctx context.Context, roomCode string) (*models.Match, []string, error) {
	normCode := strings.ToUpper(strings.TrimSpace(roomCode))
	matchQuery := `
		SELECT id::text, room_code, target_score, status, started_at, finished_at, winner_id::text
		FROM matches
		WHERE UPPER(room_code) = $1;
	`
	var m models.Match
	var winnerID *string
	err := r.pool.QueryRow(ctx, matchQuery, normCode).Scan(
		&m.ID, &m.RoomCode, &m.TargetScore, &m.Status, &m.StartedAt, &m.FinishedAt, &winnerID,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, fmt.Errorf("match not found for room code: %s", normCode)
		}
		return nil, nil, fmt.Errorf("query match by code failed: %w", err)
	}
	m.WinnerID = winnerID

	playersQuery := `
		SELECT player_id::text
		FROM match_players
		WHERE match_id = $1::uuid
		ORDER BY is_bot ASC;
	`
	rows, err := r.pool.Query(ctx, playersQuery, m.ID)
	if err != nil {
		return &m, nil, nil
	}
	defer rows.Close()

	var players []string
	for rows.Next() {
		var pID string
		if err := rows.Scan(&pID); err == nil {
			players = append(players, pID)
		}
	}

	return &m, players, nil
}

// EnsureBotPlayer seeds the default bot player if missing.
func (r *PostgresRepository) EnsureBotPlayer(ctx context.Context) error {
	query := `
		INSERT INTO players (id, alias, total_wins)
		VALUES ($1::uuid, 'Bot-JokerBot', 0)
		ON CONFLICT (id) DO NOTHING;
	`
	_, err := r.pool.Exec(ctx, query, BotPlayerID)
	return err
}

// Ping checks PostgreSQL connectivity.
func (r *PostgresRepository) Ping(ctx context.Context) error {
	if r.pool == nil {
		return errors.New("pgxpool is nil")
	}
	return r.pool.Ping(ctx)
}

// Close closes the pgxpool connection pool.
func (r *PostgresRepository) Close() {
	if r.pool != nil {
		r.pool.Close()
	}
}
