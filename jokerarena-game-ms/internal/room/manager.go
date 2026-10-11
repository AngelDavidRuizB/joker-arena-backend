package room

import (
	"context"
	"crypto/rand"
	"math/big"
	"strings"
	"sync"
	"time"
)

// Manager keeps the active room sessions in memory.
type Manager struct {
	mu        sync.RWMutex
	rooms     map[string]*Room
	persister Persister
}

// NewManager creates a Manager. persister may be nil (match results are then not stored).
func NewManager(persister Persister) *Manager {
	return &Manager{rooms: make(map[string]*Room), persister: persister}
}

// Create starts a new room session, or returns the existing one for the same code.
func (m *Manager) Create(opts Options) *Room {
	code := strings.ToUpper(strings.TrimSpace(opts.Code))
	m.mu.Lock()
	defer m.mu.Unlock()
	if existing, ok := m.rooms[code]; ok {
		return existing
	}
	r := newRoom(opts, m.persister)
	m.rooms[code] = r
	return r
}

// Get returns the active session for a code (case-insensitive).
func (m *Manager) Get(code string) (*Room, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	r, ok := m.rooms[strings.ToUpper(strings.TrimSpace(code))]
	return r, ok
}

// Reap stops and removes sessions without clients that were idle for longer than maxIdle
// (or finished sessions idle for longer than finishedIdle). It returns how many were removed.
func (m *Manager) Reap(maxIdle time.Duration) int {
	now := time.Now().UTC()
	var stale []*Room
	m.mu.Lock()
	for code, r := range m.rooms {
		if r.ClientCount() == 0 && now.Sub(r.LastActive()) > maxIdle {
			delete(m.rooms, code)
			stale = append(stale, r)
		}
	}
	m.mu.Unlock()
	for _, r := range stale {
		r.Stop()
	}
	return len(stale)
}

// StartJanitor periodically reaps idle sessions until ctx is cancelled.
func (m *Manager) StartJanitor(ctx context.Context, interval, maxIdle time.Duration) {
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				m.Reap(maxIdle)
			}
		}
	}()
}

// Close stops every session.
func (m *Manager) Close() {
	m.mu.Lock()
	rooms := m.rooms
	m.rooms = make(map[string]*Room)
	m.mu.Unlock()
	for _, r := range rooms {
		r.Stop()
	}
}

// GenerateRoomCode generates a cryptographically random room code of the form JKR-XXXX,
// using an alphabet without ambiguous characters (no 0/O, 1/I/L).
func GenerateRoomCode() string {
	const charset = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"
	b := make([]byte, 4)
	for i := range b {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		b[i] = charset[n.Int64()]
	}
	return "JKR-" + string(b)
}
