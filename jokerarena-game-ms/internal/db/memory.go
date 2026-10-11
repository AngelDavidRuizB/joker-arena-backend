package db

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"jokerarena-game-ms/internal/models"
)

// MemoryRepository provides a thread-safe in-memory Repository implementation for local/offline execution.
type MemoryRepository struct {
	mu           sync.RWMutex
	players      map[string]*models.Player
	byAlias      map[string]*models.Player
	matches      map[string]*models.Match
	matchPlayers map[string][]string // matchID -> []playerID
}

// NewMemoryRepository initializes an in-memory repository pre-seeded with Bot-JokerBot.
func NewMemoryRepository() *MemoryRepository {
	repo := &MemoryRepository{
		players:      make(map[string]*models.Player),
		byAlias:      make(map[string]*models.Player),
		matches:      make(map[string]*models.Match),
		matchPlayers: make(map[string][]string),
	}
	_ = repo.EnsureBotPlayer(context.Background())
	return repo
}

// RegisterPlayer registers or retrieves player by alias idempotently.
func (m *MemoryRepository) RegisterPlayer(ctx context.Context, alias string) (*models.Player, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	lower := strings.ToLower(strings.TrimSpace(alias))
	if existing, found := m.byAlias[lower]; found {
		return existing, false, nil
	}

	player := &models.Player{
		ID:        uuid.New().String(),
		Alias:     strings.TrimSpace(alias),
		TotalWins: 0,
		CreatedAt: time.Now().UTC(),
	}
	m.players[player.ID] = player
	m.byAlias[lower] = player
	return player, true, nil
}

// GetPlayerByID returns player by UUID.
func (m *MemoryRepository) GetPlayerByID(ctx context.Context, id string) (*models.Player, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	player, found := m.players[id]
	if !found {
		return nil, fmt.Errorf("player not found: %s", id)
	}
	return player, nil
}

// CreateRoomMatch stores match and links participants in memory.
func (m *MemoryRepository) CreateRoomMatch(ctx context.Context, matchID, roomCode string, targetScore int, hostPlayerID string, withBot bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	normCode := strings.ToUpper(strings.TrimSpace(roomCode))
	now := time.Now().UTC()
	match := &models.Match{
		ID:          matchID,
		RoomCode:    normCode,
		TargetScore: targetScore,
		Status:      "WAITING",
		StartedAt:   now,
	}
	m.matches[normCode] = match

	participants := []string{hostPlayerID}
	if withBot {
		participants = append(participants, BotPlayerID)
	}
	m.matchPlayers[matchID] = participants
	return nil
}

// GetMatchByCode retrieves match and participant list by room code.
func (m *MemoryRepository) GetMatchByCode(ctx context.Context, roomCode string) (*models.Match, []string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	normCode := strings.ToUpper(strings.TrimSpace(roomCode))
	match, found := m.matches[normCode]
	if !found {
		return nil, nil, fmt.Errorf("match not found for room code: %s", normCode)
	}
	players := m.matchPlayers[match.ID]
	return match, players, nil
}

// EnsureBotPlayer seeds the default bot player.
func (m *MemoryRepository) EnsureBotPlayer(ctx context.Context) error {
	bot := &models.Player{
		ID:        BotPlayerID,
		Alias:     "Bot-JokerBot",
		TotalWins: 0,
		CreatedAt: time.Now().UTC(),
	}
	m.players[BotPlayerID] = bot
	m.byAlias[strings.ToLower(bot.Alias)] = bot
	return nil
}

// Ping for MemoryRepository always succeeds.
func (m *MemoryRepository) Ping(ctx context.Context) error {
	return nil
}

// Close is a no-op for memory store.
func (m *MemoryRepository) Close() {}
