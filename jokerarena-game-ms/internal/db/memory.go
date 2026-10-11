package db

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"jokerarena-game-ms/internal/models"
)

type memMatchRow struct {
	matchID  int
	playerID int
	result   string
	endedAt  *time.Time
	started  time.Time
}

type memRoom struct {
	desc string
	rows []*memMatchRow
}

// MemoryRepository is a thread-safe in-memory Repository for unit tests and explicit
// STORAGE_MODE=memory runs. It mirrors the semantics of PostgresRepository (numeric ids,
// one participation row per participant) but nothing survives a restart.
type MemoryRepository struct {
	mu         sync.Mutex
	nextPlayer int
	nextMatch  int
	players    map[int]*models.Player
	byAlias    map[string]int
	rooms      map[string]*memRoom
}

// NewMemoryRepository creates an empty in-memory repository.
func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		nextPlayer: 1,
		nextMatch:  1,
		players:    make(map[int]*models.Player),
		byAlias:    make(map[string]int),
		rooms:      make(map[string]*memRoom),
	}
}

func (m *MemoryRepository) addPlayerLocked(alias string) *models.Player {
	id := m.nextPlayer
	m.nextPlayer++
	p := &models.Player{ID: strconv.Itoa(id), Alias: alias, CreatedAt: time.Now().UTC()}
	m.players[id] = p
	m.byAlias[strings.ToLower(alias)] = id
	return p
}

// RegisterPlayer registers or retrieves a player by alias idempotently.
func (m *MemoryRepository) RegisterPlayer(_ context.Context, alias string) (*models.Player, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if id, ok := m.byAlias[strings.ToLower(alias)]; ok {
		cp := *m.players[id]
		return &cp, false, nil
	}
	p := m.addPlayerLocked(alias)
	cp := *p
	return &cp, true, nil
}

// GetPlayerByID returns a player by numeric id.
func (m *MemoryRepository) GetPlayerByID(_ context.Context, id string) (*models.Player, error) {
	n, err := parsePlayerID(id)
	if err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.players[n]
	if !ok {
		return nil, fmt.Errorf("player %s: %w", id, ErrNotFound)
	}
	cp := *p
	return &cp, nil
}

// CreateRoomMatch stores the room and its participation rows.
func (m *MemoryRepository) CreateRoomMatch(_ context.Context, roomCode string, targetScore int, hostPlayerID string, withBot bool) (string, error) {
	hostID, err := parsePlayerID(hostPlayerID)
	if err != nil {
		return "", err
	}
	code := strings.ToUpper(strings.TrimSpace(roomCode))
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, taken := m.rooms[code]; taken {
		return "", ErrCodeTaken
	}
	now := time.Now().UTC()
	room := &memRoom{desc: encodeRoomDescription(targetScore, withBot)}
	hostRow := &memMatchRow{matchID: m.nextMatch, playerID: hostID, result: models.ResultWaiting, started: now}
	m.nextMatch++
	room.rows = append(room.rows, hostRow)
	if withBot {
		botID, ok := m.byAlias[strings.ToLower(models.BotAlias)]
		if !ok {
			botID, _ = strconv.Atoi(m.addPlayerLocked(models.BotAlias).ID)
		}
		room.rows = append(room.rows, &memMatchRow{matchID: m.nextMatch, playerID: botID, result: models.ResultWaiting, started: now})
		m.nextMatch++
	}
	m.rooms[code] = room
	return strconv.Itoa(hostRow.matchID), nil
}

// GetMatchByCode reconstructs the match and participant ids.
func (m *MemoryRepository) GetMatchByCode(_ context.Context, roomCode string) (*models.Match, []string, error) {
	code := strings.ToUpper(strings.TrimSpace(roomCode))
	m.mu.Lock()
	defer m.mu.Unlock()
	room, ok := m.rooms[code]
	if !ok {
		return nil, nil, fmt.Errorf("room %s: %w", code, ErrNotFound)
	}
	host := room.rows[0]
	target, withBot := decodeRoomDescription(room.desc)
	match := &models.Match{
		ID:          strconv.Itoa(host.matchID),
		RoomCode:    code,
		TargetScore: target,
		WithBot:     withBot,
		Status:      statusFromResult(host.result),
		Result:      host.result,
		HostID:      strconv.Itoa(host.playerID),
		StartedAt:   host.started,
		FinishedAt:  host.endedAt,
	}
	players := make([]string, 0, len(room.rows))
	for _, row := range room.rows {
		players = append(players, strconv.Itoa(row.playerID))
	}
	return match, players, nil
}

// MarkMatchInProgress flips WAITING rows to IN_PROGRESS.
func (m *MemoryRepository) MarkMatchInProgress(_ context.Context, roomCode string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	room, ok := m.rooms[strings.ToUpper(strings.TrimSpace(roomCode))]
	if !ok {
		return ErrNotFound
	}
	for _, row := range room.rows {
		if row.result == models.ResultWaiting {
			row.result = models.ResultInProgress
		}
	}
	return nil
}

// FinishMatch stores WIN/LOSE for the host and the bot.
func (m *MemoryRepository) FinishMatch(_ context.Context, roomCode string, hostWon bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	room, ok := m.rooms[strings.ToUpper(strings.TrimSpace(roomCode))]
	if !ok {
		return ErrNotFound
	}
	now := time.Now().UTC()
	for i, row := range room.rows {
		won := hostWon
		if i > 0 { // bot row
			won = !hostWon
		}
		if won {
			row.result = models.ResultWin
		} else {
			row.result = models.ResultLose
		}
		t := now
		row.endedAt = &t
	}
	return nil
}

// Ping always succeeds for the memory store.
func (m *MemoryRepository) Ping(context.Context) error { return nil }

// Mode returns the storage mode label.
func (m *MemoryRepository) Mode() string { return "memory" }

// Close is a no-op for the memory store.
func (m *MemoryRepository) Close() {}
