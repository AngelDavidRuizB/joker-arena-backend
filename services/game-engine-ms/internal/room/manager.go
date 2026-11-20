package room

import (
	"context"
	"crypto/rand"
	"log"
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
	defaults  Options // integrations copied into every new room
}

// SetDefaults sets the integrations (publisher, ruleset, bot) used by new rooms.
func (m *Manager) SetDefaults(d Options) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.defaults = d
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
	if opts.Publisher == nil {
		opts.Publisher = m.defaults.Publisher
	}
	if opts.Rules == nil {
		opts.Rules = m.defaults.Rules
	}
	if opts.Bot == nil {
		opts.Bot = m.defaults.Bot
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

// Reap stops and removes sessions without clients that were idle for longer than maxIdle,
// closing their in-progress matches as abandoned, and does the same for matches left without
// a session by an engine restart. It returns how many sessions were removed.
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
		r.abandon()
		r.Stop()
	}
	m.closeOrphans(maxIdle)
	return len(stale)
}

// closeOrphans abandons stored IN_PROGRESS matches that have no live session.
func (m *Manager) closeOrphans(maxIdle time.Duration) {
	finder, ok := m.persister.(OrphanFinder)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	orphans, err := finder.OrphanMatches(ctx, maxIdle)
	if err != nil {
		log.Printf("[WARN] listing orphan matches failed: %v", err)
		return
	}
	for _, o := range orphans {
		if _, live := m.Get(o.RoomCode); live {
			continue
		}
		progress := o.Progress
		if progress == nil {
			progress = &MatchProgress{HandsLeft: InitialHands, DiscardsLeft: InitialDiscards,
				BotHandsLeft: InitialHands, BotDiscardsLeft: InitialDiscards}
		}
		r := m.Create(Options{MatchID: o.MatchID, Code: o.RoomCode, TargetScore: o.TargetScore, WithBot: o.WithBot,
			Host: o.Host, Difficulty: o.Difficulty, Resume: progress})
		r.abandon()
		m.mu.Lock()
		delete(m.rooms, r.Code())
		m.mu.Unlock()
		r.Stop()
	}
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
