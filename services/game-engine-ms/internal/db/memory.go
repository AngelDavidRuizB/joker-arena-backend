package db

import (
	"context"
	"sync"
	"time"

	"game-engine-ms/internal/room"
)

type memMatch struct {
	row       MatchRow
	createdAt time.Time
	outcome   *room.MatchOutcome
}

// Memory is an in-process Repository for tests and STORAGE_MODE=memory.
type Memory struct {
	mu      sync.Mutex
	matches map[string]*memMatch // by match_id
}

// NewMemory creates an empty in-memory repository.
func NewMemory() *Memory { return &Memory{matches: map[string]*memMatch{}} }

// CreateMatch stores a WAITING match.
func (m *Memory) CreateMatch(_ context.Context, matchID, roomCode, _, _ string, _ bool, _ int, _ string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.matches[matchID]; !ok {
		m.matches[matchID] = &memMatch{row: MatchRow{MatchID: matchID, RoomCode: roomCode, Status: StatusWaiting}, createdAt: time.Now()}
	}
	return nil
}

func (m *Memory) update(roomCode string, fn func(*memMatch)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, mm := range m.matches {
		if mm.row.RoomCode == roomCode && mm.row.Status != StatusFinished {
			fn(mm)
		}
	}
}

// MarkMatchInProgress flips WAITING to IN_PROGRESS.
func (m *Memory) MarkMatchInProgress(_ context.Context, roomCode string) error {
	m.update(roomCode, func(mm *memMatch) {
		if mm.row.Status == StatusWaiting {
			mm.row.Status = StatusInProgress
		}
	})
	return nil
}

// FinishMatch closes the open match.
func (m *Memory) FinishMatch(_ context.Context, roomCode string, _ bool) error {
	m.update(roomCode, func(mm *memMatch) { mm.row.Status = StatusFinished })
	return nil
}

// RecordOutcome stores the final scores.
func (m *Memory) RecordOutcome(_ context.Context, o room.MatchOutcome) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if mm, ok := m.matches[o.MatchID]; ok {
		mm.row.Status = StatusFinished
		mm.outcome = &o
	}
	return nil
}

// Outcome returns the stored outcome of a match (tests).
func (m *Memory) Outcome(matchID string) *room.MatchOutcome {
	m.mu.Lock()
	defer m.mu.Unlock()
	if mm, ok := m.matches[matchID]; ok {
		return mm.outcome
	}
	return nil
}

// LatestMatch returns the most recent match of a room.
func (m *Memory) LatestMatch(_ context.Context, roomCode string) (*MatchRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var best *memMatch
	for _, mm := range m.matches {
		if mm.row.RoomCode == roomCode && (best == nil || mm.createdAt.After(best.createdAt)) {
			best = mm
		}
	}
	if best == nil {
		return nil, ErrNotFound
	}
	row := best.row
	return &row, nil
}

// Ping always succeeds.
func (m *Memory) Ping(context.Context) error { return nil }

// Mode returns "memory".
func (m *Memory) Mode() string { return "memory" }

// Close does nothing.
func (m *Memory) Close() {}
