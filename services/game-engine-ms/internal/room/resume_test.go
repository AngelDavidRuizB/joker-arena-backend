package room

import (
	"context"
	"sync"
	"testing"
	"time"
)

type recordingPublisher struct {
	mu     sync.Mutex
	events []map[string]any
}

func (p *recordingPublisher) Publish(key string, payload any) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if key == EventMatchEnded {
		p.events = append(p.events, payload.(map[string]any))
	}
}

func (p *recordingPublisher) ended() []map[string]any {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]map[string]any(nil), p.events...)
}

type orphanStore struct{ orphans []OrphanMatch }

func (orphanStore) MarkMatchInProgress(context.Context, string) error { return nil }
func (orphanStore) FinishMatch(context.Context, string, bool) error   { return nil }
func (s orphanStore) OrphanMatches(context.Context, time.Duration) ([]OrphanMatch, error) {
	return s.orphans, nil
}

func progress() *MatchProgress {
	return &MatchProgress{Seq: 6, Score: 2215, HandsLeft: 17, DiscardsLeft: 1, HandNumber: 3,
		BotScore: 4488, BotHandsLeft: 17, BotDiscardsLeft: 2, BotHandNumber: 3}
}

func TestResumeKeepsScoresAndCounters(t *testing.T) {
	r := NewManager(nil).Create(Options{MatchID: "m1", Code: "JKR-AB23", WithBot: true,
		Host: PlayerInfo{ID: "h", Alias: "H"}, Resume: progress()})
	defer r.Stop()
	s := r.Snapshot()
	if s.CurrentScore != 2215 || s.HandsLeft != 17 || s.DiscardsLeft != 1 || s.HandNumber != 3 ||
		s.BotScore != 4488 || s.BotHandsLeft != 17 || s.Status != "IN_PROGRESS" {
		t.Fatalf("state not resumed: %+v", s)
	}
}

func TestReapClosesAnAbandonedMatch(t *testing.T) {
	pub := &recordingPublisher{}
	m := NewManager(nil)
	m.SetDefaults(Options{Publisher: pub})
	m.Create(Options{MatchID: "m1", Code: "JKR-AB23", WithBot: true, Host: PlayerInfo{ID: "h"}, Resume: progress()})
	time.Sleep(2 * time.Millisecond)
	if n := m.Reap(time.Millisecond); n != 1 {
		t.Fatalf("reaped %d sessions", n)
	}
	ended := pub.ended()
	if len(ended) != 1 || ended[0]["reason"] != ReasonAbandoned {
		t.Fatalf("expected one ABANDONED match.ended, got %v", ended)
	}
}

func TestReapLeavesAWaitingRoomOpen(t *testing.T) {
	pub := &recordingPublisher{}
	m := NewManager(nil)
	m.SetDefaults(Options{Publisher: pub})
	m.Create(Options{MatchID: "m1", Code: "JKR-AB23", Host: PlayerInfo{ID: "h"}})
	time.Sleep(2 * time.Millisecond)
	m.Reap(time.Millisecond)
	if ended := pub.ended(); len(ended) != 0 {
		t.Fatalf("a match that never started must not end: %v", ended)
	}
}

func TestReapClosesMatchesOrphanedByARestart(t *testing.T) {
	pub := &recordingPublisher{}
	m := NewManager(orphanStore{orphans: []OrphanMatch{{MatchID: "m1", RoomCode: "JKR-AB23", WithBot: true,
		TargetScore: 22000, Host: PlayerInfo{ID: "h"}, Progress: progress()}}})
	m.SetDefaults(Options{Publisher: pub})
	m.Reap(time.Hour)
	ended := pub.ended()
	if len(ended) != 1 || ended[0]["match_id"] != "m1" || ended[0]["reason"] != ReasonAbandoned {
		t.Fatalf("expected the orphan to be closed, got %v", ended)
	}
	if _, live := m.Get("JKR-AB23"); live {
		t.Fatal("the orphan session must not stay in memory")
	}
}
