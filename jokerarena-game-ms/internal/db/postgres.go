package db

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"jokerarena-game-ms/internal/models"
)

// guestPasswordSentinel is stored in players.password. It is not a valid hash for any
// supported algorithm, so these lightweight alias-only accounts can never authenticate.
const guestPasswordSentinel = "!guest-no-login"

// PostgresRepository implements Repository using pgxpool over the team's PostgreSQL schema:
//
//	players(id_player, username, email, password, created_at, updated_at)
//	rooms(id_room, name, description, capacity, created_at, updated_at)
//	matches(id_match, id_player, id_room, started_at, ended_at, result, created_at, updated_at)
//
// Mapping: alias -> players.username; room code -> rooms.name; target score and bot flag ->
// rooms.description; one matches row per participant (host and optional bot).
type PostgresRepository struct {
	pool atomic.Pointer[pgxpool.Pool]
}

// NewPostgresRepositoryFromPool wraps an already established pool (used by tests).
func NewPostgresRepositoryFromPool(pool *pgxpool.Pool) *PostgresRepository {
	r := &PostgresRepository{}
	r.pool.Store(pool)
	return r
}

// NewPostgresRepository returns immediately and keeps trying to connect in the background
// with capped backoff until ctx is cancelled. While disconnected every operation returns
// ErrUnavailable (the API answers 503), so the service never pretends to persist data.
func NewPostgresRepository(ctx context.Context, dsn, safeTarget string) *PostgresRepository {
	r := &PostgresRepository{}
	go func() {
		delay := time.Second
		for attempt := 1; ; attempt++ {
			pool, err := NewPgxPool(ctx, dsn)
			if err == nil {
				r.pool.Store(pool)
				log.Printf("[INFO] Connected to PostgreSQL at %s (attempt %d).", safeTarget, attempt)
				return
			}
			log.Printf("[WARN] PostgreSQL unavailable at %s (attempt %d): %v", safeTarget, attempt, err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(delay):
			}
			if delay < 5*time.Second {
				delay += time.Second
			}
		}
	}()
	return r
}

func (r *PostgresRepository) db() (*pgxpool.Pool, error) {
	p := r.pool.Load()
	if p == nil {
		return nil, ErrUnavailable
	}
	return p, nil
}

// wrapErr converts connectivity failures into ErrUnavailable and keeps other errors intact.
func wrapErr(op string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrUnavailable) || errors.Is(err, ErrNotFound) || errors.Is(err, ErrCodeTaken) {
		return err
	}
	var netErr interface{ Timeout() bool }
	if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &netErr) && netErr.Timeout()) {
		return fmt.Errorf("%s: %w", op, ErrUnavailable)
	}
	return fmt.Errorf("%s: %w", op, err)
}

func parsePlayerID(id string) (int, error) {
	if !models.IsValidPlayerID(id) {
		return 0, fmt.Errorf("%w: %q", ErrInvalidID, id)
	}
	n, err := strconv.Atoi(strings.TrimSpace(id))
	if err != nil {
		return 0, fmt.Errorf("%w: %q", ErrInvalidID, id)
	}
	return n, nil
}

// RegisterPlayer registers or retrieves a player by alias. Usernames are not UNIQUE in the
// shared schema, so uniqueness is enforced with a transaction-scoped advisory lock.
func (r *PostgresRepository) RegisterPlayer(ctx context.Context, alias string) (*models.Player, bool, error) {
	pool, err := r.db()
	if err != nil {
		return nil, false, err
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, false, wrapErr("begin register tx", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, "player:"+strings.ToLower(alias)); err != nil {
		return nil, false, wrapErr("lock register", err)
	}

	var p models.Player
	var id int
	err = tx.QueryRow(ctx, `
		SELECT id_player, username, created_at
		FROM players
		WHERE lower(username) = lower($1)
		ORDER BY id_player
		LIMIT 1`, alias).Scan(&id, &p.Alias, &p.CreatedAt)
	switch {
	case err == nil:
		p.ID = strconv.Itoa(id)
		if err := tx.Commit(ctx); err != nil {
			return nil, false, wrapErr("commit register", err)
		}
		return &p, false, nil
	case !errors.Is(err, pgx.ErrNoRows):
		return nil, false, wrapErr("select player", err)
	}

	err = tx.QueryRow(ctx, `
		INSERT INTO players (username, email, password, created_at, updated_at)
		VALUES ($1, $2, $3, (now() AT TIME ZONE 'utc'), (now() AT TIME ZONE 'utc'))
		RETURNING id_player, username, created_at`,
		alias, strings.ToLower(alias)+"@guest.jokerarena.local", guestPasswordSentinel,
	).Scan(&id, &p.Alias, &p.CreatedAt)
	if err != nil {
		return nil, false, wrapErr("insert player", err)
	}
	p.ID = strconv.Itoa(id)
	if err := tx.Commit(ctx); err != nil {
		return nil, false, wrapErr("commit register", err)
	}
	return &p, true, nil
}

// GetPlayerByID retrieves a player by numeric id.
func (r *PostgresRepository) GetPlayerByID(ctx context.Context, id string) (*models.Player, error) {
	pool, err := r.db()
	if err != nil {
		return nil, err
	}
	n, err := parsePlayerID(id)
	if err != nil {
		return nil, err
	}
	var p models.Player
	var pid int
	err = pool.QueryRow(ctx, `
		SELECT id_player, username, created_at FROM players WHERE id_player = $1`, n,
	).Scan(&pid, &p.Alias, &p.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("player %s: %w", id, ErrNotFound)
		}
		return nil, wrapErr("query player", err)
	}
	p.ID = strconv.Itoa(pid)
	return &p, nil
}

// ensureBot returns the id of the bot player, creating the row when missing.
func ensureBot(ctx context.Context, tx pgx.Tx) (int, error) {
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, "player:"+strings.ToLower(models.BotAlias)); err != nil {
		return 0, err
	}
	var id int
	err := tx.QueryRow(ctx, `SELECT id_player FROM players WHERE username = $1 ORDER BY id_player LIMIT 1`, models.BotAlias).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return 0, err
	}
	err = tx.QueryRow(ctx, `
		INSERT INTO players (username, email, password, created_at, updated_at)
		VALUES ($1, $2, $3, (now() AT TIME ZONE 'utc'), (now() AT TIME ZONE 'utc'))
		RETURNING id_player`,
		models.BotAlias, "bot@guest.jokerarena.local", guestPasswordSentinel).Scan(&id)
	return id, err
}

// CreateRoomMatch creates the room plus one participation row per participant atomically.
func (r *PostgresRepository) CreateRoomMatch(ctx context.Context, roomCode string, targetScore int, hostPlayerID string, withBot bool) (string, error) {
	pool, err := r.db()
	if err != nil {
		return "", err
	}
	hostID, err := parsePlayerID(hostPlayerID)
	if err != nil {
		return "", err
	}
	code := strings.ToUpper(strings.TrimSpace(roomCode))

	tx, err := pool.Begin(ctx)
	if err != nil {
		return "", wrapErr("begin room tx", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, "room:"+code); err != nil {
		return "", wrapErr("lock room", err)
	}
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM rooms WHERE name = $1)`, code).Scan(&exists); err != nil {
		return "", wrapErr("check room code", err)
	}
	if exists {
		return "", ErrCodeTaken
	}

	capacity := 1
	if withBot {
		capacity = 2
	}
	var roomID int
	err = tx.QueryRow(ctx, `
		INSERT INTO rooms (name, description, capacity, created_at, updated_at)
		VALUES ($1, $2, $3, (now() AT TIME ZONE 'utc'), (now() AT TIME ZONE 'utc'))
		RETURNING id_room`,
		code, encodeRoomDescription(targetScore, withBot), capacity).Scan(&roomID)
	if err != nil {
		return "", wrapErr("insert room", err)
	}

	var matchID int
	err = tx.QueryRow(ctx, `
		INSERT INTO matches (id_player, id_room, started_at, ended_at, result, created_at, updated_at)
		VALUES ($1, $2, (now() AT TIME ZONE 'utc'), NULL, $3, (now() AT TIME ZONE 'utc'), (now() AT TIME ZONE 'utc'))
		RETURNING id_match`,
		hostID, roomID, models.ResultWaiting).Scan(&matchID)
	if err != nil {
		return "", wrapErr("insert host match", err)
	}

	if withBot {
		botID, err := ensureBot(ctx, tx)
		if err != nil {
			return "", wrapErr("ensure bot", err)
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO matches (id_player, id_room, started_at, ended_at, result, created_at, updated_at)
			VALUES ($1, $2, (now() AT TIME ZONE 'utc'), NULL, $3, (now() AT TIME ZONE 'utc'), (now() AT TIME ZONE 'utc'))`,
			botID, roomID, models.ResultWaiting)
		if err != nil {
			return "", wrapErr("insert bot match", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return "", wrapErr("commit room", err)
	}
	return strconv.Itoa(matchID), nil
}

// GetMatchByCode reconstructs the match and participant ids for a room code.
func (r *PostgresRepository) GetMatchByCode(ctx context.Context, roomCode string) (*models.Match, []string, error) {
	pool, err := r.db()
	if err != nil {
		return nil, nil, err
	}
	code := strings.ToUpper(strings.TrimSpace(roomCode))
	rows, err := pool.Query(ctx, `
		SELECT m.id_match, m.id_player, m.started_at, m.ended_at, COALESCE(m.result, ''), COALESCE(r.description, '')
		FROM rooms r
		JOIN matches m ON m.id_room = r.id_room
		WHERE r.name = $1
		ORDER BY m.id_match`, code)
	if err != nil {
		return nil, nil, wrapErr("query match", err)
	}
	defer rows.Close()

	var match *models.Match
	var players []string
	for rows.Next() {
		var matchID, playerID int
		var startedAt, endedAt *time.Time
		var result, desc string
		if err := rows.Scan(&matchID, &playerID, &startedAt, &endedAt, &result, &desc); err != nil {
			return nil, nil, wrapErr("scan match", err)
		}
		players = append(players, strconv.Itoa(playerID))
		if match == nil {
			target, withBot := decodeRoomDescription(desc)
			match = &models.Match{
				ID:          strconv.Itoa(matchID),
				RoomCode:    code,
				TargetScore: target,
				WithBot:     withBot,
				Status:      statusFromResult(result),
				Result:      strings.ToUpper(result),
				HostID:      strconv.Itoa(playerID),
				FinishedAt:  endedAt,
			}
			if startedAt != nil {
				match.StartedAt = *startedAt
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, nil, wrapErr("iterate match", err)
	}
	if match == nil {
		return nil, nil, fmt.Errorf("room %s: %w", code, ErrNotFound)
	}
	return match, players, nil
}

// MarkMatchInProgress flips WAITING participation rows to IN_PROGRESS.
func (r *PostgresRepository) MarkMatchInProgress(ctx context.Context, roomCode string) error {
	pool, err := r.db()
	if err != nil {
		return err
	}
	_, err = pool.Exec(ctx, `
		UPDATE matches m
		SET result = $2, updated_at = (now() AT TIME ZONE 'utc')
		FROM rooms r
		WHERE m.id_room = r.id_room AND r.name = $1 AND m.result = $3`,
		strings.ToUpper(strings.TrimSpace(roomCode)), models.ResultInProgress, models.ResultWaiting)
	return wrapErr("mark in progress", err)
}

// FinishMatch stores WIN/LOSE for host and bot opponent and sets ended_at.
func (r *PostgresRepository) FinishMatch(ctx context.Context, roomCode string, hostWon bool) error {
	pool, err := r.db()
	if err != nil {
		return err
	}
	hostResult, botResult := models.ResultLose, models.ResultWin
	if hostWon {
		hostResult, botResult = models.ResultWin, models.ResultLose
	}
	_, err = pool.Exec(ctx, `
		UPDATE matches m
		SET result = CASE WHEN p.username = $2 THEN $4 ELSE $3 END,
		    ended_at = (now() AT TIME ZONE 'utc'),
		    updated_at = (now() AT TIME ZONE 'utc')
		FROM rooms r, players p
		WHERE m.id_room = r.id_room AND m.id_player = p.id_player AND r.name = $1`,
		strings.ToUpper(strings.TrimSpace(roomCode)), models.BotAlias, hostResult, botResult)
	return wrapErr("finish match", err)
}

// Ping checks PostgreSQL connectivity.
func (r *PostgresRepository) Ping(ctx context.Context) error {
	pool, err := r.db()
	if err != nil {
		return err
	}
	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	return nil
}

// Mode returns the storage mode label.
func (r *PostgresRepository) Mode() string { return "postgres" }

// Close closes the pgxpool connection pool.
func (r *PostgresRepository) Close() {
	if p := r.pool.Swap(nil); p != nil {
		p.Close()
	}
}
