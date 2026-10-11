package db

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"jokerarena-game-ms/internal/models"
)

// ResilientRepository wraps PostgresRepository with transparent MemoryRepository fallback.
type ResilientRepository struct {
	mu      sync.RWMutex
	pgRepo  *PostgresRepository
	memRepo *MemoryRepository
}

// NewResilientRepository attempts connection to PostgreSQL, falling back to MemoryRepository if offline.
func NewResilientRepository(ctx context.Context, dbURL string) *ResilientRepository {
	mem := NewMemoryRepository()
	pool, err := NewPgxPool(ctx, dbURL)
	if err != nil {
		log.Printf("[WARN] PostgreSQL unavailable at startup (%v). Running in resilient in-memory fallback mode.", err)
		return &ResilientRepository{
			pgRepo:  nil,
			memRepo: mem,
		}
	}

	pg := NewPostgresRepository(pool)
	_ = pg.EnsureBotPlayer(ctx)
	log.Printf("[INFO] Connected successfully to PostgreSQL via pgxpool.")

	return &ResilientRepository{
		pgRepo:  pg,
		memRepo: mem,
	}
}

// IsConnected returns whether the primary PostgreSQL connection is alive.
func (r *ResilientRepository) IsConnected(ctx context.Context) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.pgRepo == nil {
		return false
	}
	pingCtx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()
	return r.pgRepo.Ping(pingCtx) == nil
}

// RegisterPlayer delegates to PostgreSQL or falls back to memory.
func (r *ResilientRepository) RegisterPlayer(ctx context.Context, alias string) (*models.Player, bool, error) {
	r.mu.RLock()
	pg := r.pgRepo
	r.mu.RUnlock()

	if pg != nil {
		p, isNew, err := pg.RegisterPlayer(ctx, alias)
		if err == nil {
			// Mirror to in-memory store
			_, _, _ = r.memRepo.RegisterPlayer(ctx, alias)
			return p, isNew, nil
		}
		log.Printf("[WARN] PostgreSQL RegisterPlayer error (%v). Falling back to memory store.", err)
	}
	return r.memRepo.RegisterPlayer(ctx, alias)
}

// GetPlayerByID delegates to PostgreSQL or falls back to memory.
func (r *ResilientRepository) GetPlayerByID(ctx context.Context, id string) (*models.Player, error) {
	r.mu.RLock()
	pg := r.pgRepo
	r.mu.RUnlock()

	if pg != nil {
		p, err := pg.GetPlayerByID(ctx, id)
		if err == nil {
			return p, nil
		}
		// If specific not-found error from postgres, propagate
		if err.Error() == "player not found: "+id {
			return nil, err
		}
		log.Printf("[WARN] PostgreSQL GetPlayerByID error (%v). Falling back to memory store.", err)
	}
	return r.memRepo.GetPlayerByID(ctx, id)
}

// CreateRoomMatch delegates to PostgreSQL or falls back to memory.
func (r *ResilientRepository) CreateRoomMatch(ctx context.Context, matchID, roomCode string, targetScore int, hostPlayerID string, withBot bool) error {
	r.mu.RLock()
	pg := r.pgRepo
	r.mu.RUnlock()

	if pg != nil {
		err := pg.CreateRoomMatch(ctx, matchID, roomCode, targetScore, hostPlayerID, withBot)
		if err == nil {
			// Mirror to memory store
			_ = r.memRepo.CreateRoomMatch(ctx, matchID, roomCode, targetScore, hostPlayerID, withBot)
			return nil
		}
		log.Printf("[WARN] PostgreSQL CreateRoomMatch error (%v). Falling back to memory store.", err)
	}
	return r.memRepo.CreateRoomMatch(ctx, matchID, roomCode, targetScore, hostPlayerID, withBot)
}

// GetMatchByCode delegates to PostgreSQL or falls back to memory.
func (r *ResilientRepository) GetMatchByCode(ctx context.Context, roomCode string) (*models.Match, []string, error) {
	r.mu.RLock()
	pg := r.pgRepo
	r.mu.RUnlock()

	if pg != nil {
		m, players, err := pg.GetMatchByCode(ctx, roomCode)
		if err == nil {
			return m, players, nil
		}
		log.Printf("[WARN] PostgreSQL GetMatchByCode error (%v). Falling back to memory store.", err)
	}
	return r.memRepo.GetMatchByCode(ctx, roomCode)
}

// EnsureBotPlayer ensures bot player is seeded in both stores.
func (r *ResilientRepository) EnsureBotPlayer(ctx context.Context) error {
	r.mu.RLock()
	pg := r.pgRepo
	r.mu.RUnlock()

	if pg != nil {
		_ = pg.EnsureBotPlayer(ctx)
	}
	return r.memRepo.EnsureBotPlayer(ctx)
}

// Ping checks whether PostgreSQL connection pool is healthy.
func (r *ResilientRepository) Ping(ctx context.Context) error {
	r.mu.RLock()
	pg := r.pgRepo
	r.mu.RUnlock()

	if pg == nil {
		return errors.New("postgres connection not established (running in fallback)")
	}
	return pg.Ping(ctx)
}

// Close closes database connection pool.
func (r *ResilientRepository) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.pgRepo != nil {
		r.pgRepo.Close()
		r.pgRepo = nil
	}
}
