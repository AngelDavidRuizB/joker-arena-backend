package room

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"jokerarena-game-ms/internal/models"
	"jokerarena-game-ms/internal/poker"
)

type fakePersister struct {
	mu         sync.Mutex
	inProgress int
	finished   []bool
}

func (f *fakePersister) MarkMatchInProgress(context.Context, string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.inProgress++
	return nil
}

func (f *fakePersister) FinishMatch(_ context.Context, _ string, won bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.finished = append(f.finished, won)
	return nil
}

func (f *fakePersister) snapshot() (int, []bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.inProgress, append([]bool(nil), f.finished...)
}

var host = PlayerInfo{ID: "1", Alias: "Host"}

func newTestRoom(t *testing.T, target int, seed uint64, withBot bool) (*Room, *fakePersister) {
	t.Helper()
	p := &fakePersister{}
	m := NewManager(p)
	t.Cleanup(m.Close)
	r := m.Create(Options{MatchID: "10", Code: "JKR-TEST", TargetScore: target, WithBot: withBot, Host: host, Seed: seed})
	return r, p
}

func connect(t *testing.T, r *Room, p PlayerInfo) *Client {
	t.Helper()
	c := NewClient(p.ID)
	if !r.Join(c, p) {
		t.Fatal("join rejected")
	}
	return c
}

// next waits for the next message of the client.
func next(t *testing.T, c *Client) (string, json.RawMessage) {
	t.Helper()
	select {
	case frame, ok := <-c.Send:
		if !ok {
			t.Fatal("client channel closed")
		}
		var env Envelope
		if err := json.Unmarshal(frame, &env); err != nil {
			t.Fatalf("bad frame %s: %v", frame, err)
		}
		return env.Type, env.Payload
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for a message")
	}
	return "", nil
}

func expect[T any](t *testing.T, c *Client, wantType string) T {
	t.Helper()
	typ, payload := next(t, c)
	if typ != wantType {
		t.Fatalf("got %s (%s), want %s", typ, payload, wantType)
	}
	var out T
	if err := json.Unmarshal(payload, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func expectError(t *testing.T, c *Client, wantCode string) {
	t.Helper()
	e := expect[ErrorPayload](t, c, MsgError)
	if e.Code != wantCode {
		t.Fatalf("error code %s (%s), want %s", e.Code, e.Message, wantCode)
	}
}

func TestJoinDealsEightCardsFromA52Deck(t *testing.T) {
	r, _ := newTestRoom(t, 300, 1, true)
	c := connect(t, r, host)
	st := expect[StatePayload](t, c, MsgRoomState)
	if len(st.CardsInHand) != 8 || st.DeckCount != 44 {
		t.Fatalf("hand=%d deck=%d, want 8 and 44", len(st.CardsInHand), st.DeckCount)
	}
	if st.HandsLeft != 4 || st.DiscardsLeft != 3 || st.CurrentScore != 0 || st.Status != models.StatusWaiting {
		t.Fatalf("unexpected initial state %+v", st)
	}
	if len(st.ActiveJokers) != 3 || st.ActiveJokers[0] != "banner" {
		t.Fatalf("jokers %v", st.ActiveJokers)
	}
	if st.HostID != "1" || !st.WithBot || len(st.Players) != 2 || !st.Players[1].IsBot {
		t.Fatalf("players %+v", st)
	}
	seen := map[string]bool{}
	for _, card := range st.CardsInHand {
		seen[card.Rank+card.Suit] = true
	}
	if len(seen) != 8 {
		t.Fatal("hand contains duplicate cards")
	}
}

func TestSameSeedDealsSameHand(t *testing.T) {
	a, _ := newTestRoom(t, 300, 99, false)
	b, _ := newTestRoom(t, 300, 99, false)
	ha, hb := a.Snapshot().CardsInHand, b.Snapshot().CardsInHand
	for i := range ha {
		if ha[i] != hb[i] {
			t.Fatalf("hands differ at %d: %v vs %v", i, ha[i], hb[i])
		}
	}
}

func TestPlayHandScoresAndRefills(t *testing.T) {
	r, p := newTestRoom(t, 100000, 7, true)
	c := connect(t, r, host)
	before := expect[StatePayload](t, c, MsgRoomState)

	idx := []int{0, 2, 4}
	want, err := poker.Score([]models.Card{before.CardsInHand[0], before.CardsInHand[2], before.CardsInHand[4]}, before.DiscardsLeft)
	if err != nil {
		t.Fatal(err)
	}

	r.Play(c, idx)
	scored := expect[HandScoredPayload](t, c, MsgHandScored)
	if scored.Score != want.Score || scored.PokerHand != want.PokerHand || scored.HandNumber != 1 {
		t.Fatalf("scored %+v, want score %d hand %s", scored, want.Score, want.PokerHand)
	}
	if scored.AccumulatedScore != want.Score || scored.PlayerID != "1" || len(scored.Jokers) != 3 {
		t.Fatalf("unexpected audit payload %+v", scored)
	}
	after := expect[StatePayload](t, c, MsgRoomState)
	if after.CurrentScore != want.Score || after.HandsLeft != 3 || after.DiscardsLeft != 3 {
		t.Fatalf("state after play %+v", after)
	}
	if len(after.CardsInHand) != 8 || after.DeckCount != 41 || after.Status != models.StatusInProgress {
		t.Fatalf("hand=%d deck=%d status=%s", len(after.CardsInHand), after.DeckCount, after.Status)
	}
	// Unplayed cards stay, in order, at the front of the hand.
	kept := []models.Card{before.CardsInHand[1], before.CardsInHand[3], before.CardsInHand[5], before.CardsInHand[6], before.CardsInHand[7]}
	for i, card := range kept {
		if after.CardsInHand[i] != card {
			t.Fatalf("kept card %d changed: %v vs %v", i, after.CardsInHand[i], card)
		}
	}
	time.Sleep(50 * time.Millisecond)
	if inProgress, _ := p.snapshot(); inProgress != 1 {
		t.Fatalf("expected one IN_PROGRESS persistence call, got %d", inProgress)
	}
}

func TestDiscardRefillsWithoutScoring(t *testing.T) {
	r, _ := newTestRoom(t, 300, 3, false)
	c := connect(t, r, host)
	expect[StatePayload](t, c, MsgRoomState)

	r.Discard(c, []int{0, 1})
	st := expect[StatePayload](t, c, MsgRoomState)
	if st.DiscardsLeft != 2 || st.HandsLeft != 4 || st.CurrentScore != 0 {
		t.Fatalf("state %+v", st)
	}
	if len(st.CardsInHand) != 8 || st.DeckCount != 42 {
		t.Fatalf("hand=%d deck=%d", len(st.CardsInHand), st.DeckCount)
	}
}

func TestDiscardsRunOut(t *testing.T) {
	r, _ := newTestRoom(t, 300, 3, false)
	c := connect(t, r, host)
	expect[StatePayload](t, c, MsgRoomState)
	for i := 0; i < InitialDiscards; i++ {
		r.Discard(c, []int{0})
		expect[StatePayload](t, c, MsgRoomState)
	}
	r.Discard(c, []int{0})
	expectError(t, c, ErrCodeNoDiscards)
}

func TestInvalidSelectionsAreRejectedWithoutChangingState(t *testing.T) {
	r, _ := newTestRoom(t, 300, 5, false)
	c := connect(t, r, host)
	expect[StatePayload](t, c, MsgRoomState)
	before := r.Snapshot()

	bad := [][]int{nil, {}, {0, 1, 2, 3, 4, 5}, {0, 0}, {-1}, {8}, {99}}
	for _, sel := range bad {
		r.Play(c, sel)
		expectError(t, c, ErrCodeInvalidSelection)
		r.Discard(c, sel)
		expectError(t, c, ErrCodeInvalidSelection)
	}
	after := r.Snapshot()
	if after.HandsLeft != before.HandsLeft || after.DiscardsLeft != before.DiscardsLeft ||
		after.CurrentScore != before.CurrentScore || after.DeckCount != before.DeckCount || after.Status != models.StatusWaiting {
		t.Fatalf("state changed by invalid input: %+v vs %+v", before, after)
	}
}

func TestOnlyHostCanActAndSpectatorsSeeBroadcasts(t *testing.T) {
	r, _ := newTestRoom(t, 100000, 11, true)
	h := connect(t, r, host)
	expect[StatePayload](t, h, MsgRoomState)
	guest := PlayerInfo{ID: "2", Alias: "Guest"}
	g := connect(t, r, guest)
	// Both receive the state broadcast triggered by the join.
	expect[StatePayload](t, h, MsgRoomState)
	st := expect[StatePayload](t, g, MsgRoomState)
	if len(st.Players) != 3 {
		t.Fatalf("players %+v", st.Players)
	}

	r.Play(g, []int{0})
	expectError(t, g, ErrCodeNotHost)

	r.Play(h, []int{0})
	expect[HandScoredPayload](t, g, MsgHandScored) // spectator sees the audit
	expect[StatePayload](t, g, MsgRoomState)
	expect[HandScoredPayload](t, h, MsgHandScored)
}

func TestRoomFull(t *testing.T) {
	r, _ := newTestRoom(t, 300, 2, true) // host + bot
	connect(t, r, host)
	connect(t, r, PlayerInfo{ID: "2", Alias: "B"})
	connect(t, r, PlayerInfo{ID: "3", Alias: "C"})
	over := connect(t, r, PlayerInfo{ID: "4", Alias: "D"})
	for {
		typ, payload := next(t, over)
		if typ == MsgError {
			var e ErrorPayload
			_ = json.Unmarshal(payload, &e)
			if e.Code != ErrCodeRoomFull {
				t.Fatalf("code %s", e.Code)
			}
			return
		}
	}
}

func TestWinWhenTargetReached(t *testing.T) {
	r, p := newTestRoom(t, 1, 4, true) // any hand reaches 1 point
	c := connect(t, r, host)
	expect[StatePayload](t, c, MsgRoomState)

	r.Play(c, []int{0})
	expect[HandScoredPayload](t, c, MsgHandScored)
	st := expect[StatePayload](t, c, MsgRoomState)
	if st.Status != models.StatusFinished {
		t.Fatalf("status %s", st.Status)
	}
	fin := expect[MatchFinishedPayload](t, c, MsgMatchDone)
	if fin.Result != "WIN" || !fin.TargetReached || fin.Reason != "TARGET_SCORE_REACHED" || fin.WinnerID != "1" || fin.HandsPlayed != 1 {
		t.Fatalf("finished %+v", fin)
	}

	r.Play(c, []int{0})
	expectError(t, c, ErrCodeMatchFinished)

	time.Sleep(50 * time.Millisecond)
	if _, finished := p.snapshot(); len(finished) != 1 || !finished[0] {
		t.Fatalf("expected one WIN persisted, got %v", finished)
	}

	// A late joiner receives the final result too.
	late := connect(t, r, host)
	expect[StatePayload](t, late, MsgRoomState)
	expect[MatchFinishedPayload](t, late, MsgMatchDone)
}

func TestLoseWhenHandsExhausted(t *testing.T) {
	r, p := newTestRoom(t, 1000000000, 8, true)
	c := connect(t, r, host)
	expect[StatePayload](t, c, MsgRoomState)
	for i := 0; i < InitialHands; i++ {
		r.Play(c, []int{0, 1, 2, 3, 4})
		expect[HandScoredPayload](t, c, MsgHandScored)
		expect[StatePayload](t, c, MsgRoomState)
	}
	fin := expect[MatchFinishedPayload](t, c, MsgMatchDone)
	if fin.Result != "LOSE" || fin.TargetReached || fin.Reason != "HANDS_EXHAUSTED" || fin.WinnerID != BotPlayerID || fin.HandsPlayed != 4 {
		t.Fatalf("finished %+v", fin)
	}
	time.Sleep(50 * time.Millisecond)
	if _, finished := p.snapshot(); len(finished) != 1 || finished[0] {
		t.Fatalf("expected one LOSE persisted, got %v", finished)
	}
}

func TestFullMatchNeverRunsOutOfCards(t *testing.T) {
	// Worst case: 3 discards of 5 cards plus 4 hands of 5 cards = 35 <= 44 cards in the deck.
	r, _ := newTestRoom(t, 1000000000, 21, false)
	c := connect(t, r, host)
	expect[StatePayload](t, c, MsgRoomState)
	for i := 0; i < InitialDiscards; i++ {
		r.Discard(c, []int{0, 1, 2, 3, 4})
		expect[StatePayload](t, c, MsgRoomState)
	}
	for i := 0; i < InitialHands; i++ {
		r.Play(c, []int{0, 1, 2, 3, 4})
		expect[HandScoredPayload](t, c, MsgHandScored)
		st := expect[StatePayload](t, c, MsgRoomState)
		if len(st.CardsInHand) != 8 {
			t.Fatalf("hand size %d after play %d", len(st.CardsInHand), i+1)
		}
	}
	if st := r.Snapshot(); st.DeckCount != 44-35 {
		t.Fatalf("deck %d, want 9", st.DeckCount)
	}
}

func TestSlowClientIsDroppedWithoutBlockingRoom(t *testing.T) {
	r, _ := newTestRoom(t, 100000, 6, false)
	slow := connect(t, r, host) // never reads its queue
	for i := 0; i < sendBuffer+5; i++ {
		r.Resend(slow)
	}
	deadline := time.After(3 * time.Second)
	for r.ClientCount() != 0 {
		select {
		case <-deadline:
			t.Fatalf("slow client was not dropped (clients=%d)", r.ClientCount())
		case <-time.After(10 * time.Millisecond):
		}
	}
	// The room keeps working for new connections.
	fresh := connect(t, r, host)
	expect[StatePayload](t, fresh, MsgRoomState)
}

func TestReapRemovesIdleRoomsOnly(t *testing.T) {
	m := NewManager(nil)
	defer m.Close()
	idle := m.Create(Options{MatchID: "1", Code: "JKR-IDLE", Host: host, Seed: 1})
	busy := m.Create(Options{MatchID: "2", Code: "JKR-BUSY", Host: host, Seed: 1})
	connect(t, busy, host)
	time.Sleep(20 * time.Millisecond)
	if n := m.Reap(10 * time.Millisecond); n != 1 {
		t.Fatalf("reaped %d, want 1", n)
	}
	if _, ok := m.Get("jkr-idle"); ok {
		t.Fatal("idle room should be gone")
	}
	if _, ok := m.Get("jkr-busy"); !ok {
		t.Fatal("busy room must remain")
	}
	_ = idle
}

func TestGenerateRoomCodeFormat(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		code := GenerateRoomCode()
		if _, err := models.ValidateRoomCode(code); err != nil || len(code) != 8 || code[:4] != "JKR-" {
			t.Fatalf("bad code %q (%v)", code, err)
		}
		seen[code] = true
	}
	if len(seen) < 150 {
		t.Fatalf("codes are not random enough: %d unique of 200", len(seen))
	}
}
