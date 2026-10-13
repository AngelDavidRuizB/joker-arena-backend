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
	if st.HandsLeft != InitialHands || st.DiscardsLeft != 3 || st.CurrentScore != 0 || st.Status != models.StatusWaiting {
		t.Fatalf("unexpected initial state %+v", st)
	}
	if st.HandsTotal != InitialHands || st.DiscardsTotal != InitialDiscards {
		t.Fatalf("totals hands=%d discards=%d", st.HandsTotal, st.DiscardsTotal)
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
	if after.CurrentScore != want.Score || after.HandsLeft != InitialHands-1 || after.DiscardsLeft != 3 {
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
	if st.DiscardsLeft != 2 || st.HandsLeft != InitialHands || st.CurrentScore != 0 {
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

// drainUntilFinished reads every queued message, tracking the last host and bot scores
// seen in STATE/HAND_SCORED frames, and returns once MATCH_FINISHED arrives. The win rule
// changed to a host-vs-bot score comparison, so the expected winner is derived from the
// observed final scores rather than hard-coded.
func drainUntilFinished(t *testing.T, c *Client) (fin MatchFinishedPayload, hostScore, botScore int) {
	t.Helper()
	for {
		typ, payload := next(t, c)
		switch typ {
		case MsgHandScored:
			var hs HandScoredPayload
			if err := json.Unmarshal(payload, &hs); err != nil {
				t.Fatal(err)
			}
			if hs.PlayerID == BotPlayerID {
				botScore = hs.AccumulatedScore
			} else {
				hostScore = hs.AccumulatedScore
			}
		case MsgRoomState:
			var st StatePayload
			if err := json.Unmarshal(payload, &st); err != nil {
				t.Fatal(err)
			}
			hostScore, botScore = st.CurrentScore, st.BotScore
		case MsgBotAction:
			// Announced before the bot mutates its hand; not needed for score tracking.
		case MsgMatchDone:
			if err := json.Unmarshal(payload, &fin); err != nil {
				t.Fatal(err)
			}
			return fin, hostScore, botScore
		default:
			t.Fatalf("unexpected message %s while draining", typ)
		}
	}
}

func TestLoseWhenHandsExhausted(t *testing.T) {
	// Unreachable target forces HANDS_EXHAUSTED; the outcome is now a host-vs-bot score
	// comparison (the win rule genuinely changed from the old forfeit rule). The host plays
	// a fixed, deliberately weak selection every turn so the heuristic bot outscores it and
	// wins. We assert the winner follows the comparison rather than hard-coding WIN/LOSE.
	r, p := newTestRoom(t, 1000000000, 1, true)
	c := connect(t, r, host)
	expect[StatePayload](t, c, MsgRoomState)
	for i := 0; i < InitialHands; i++ {
		r.Play(c, []int{0}) // single lowest-index card: the weakest possible host play
	}
	fin, hostScore, botScore := drainUntilFinished(t, c)
	if fin.TargetReached || fin.Reason != "HANDS_EXHAUSTED" || fin.HandsPlayed != InitialHands {
		t.Fatalf("finished %+v", fin)
	}
	// The heuristic bot is expected to outscore the fixed weak host selection with this seed.
	if hostScore > botScore {
		t.Fatalf("expected the bot to outscore the host, host=%d bot=%d", hostScore, botScore)
	}
	if fin.Result != "LOSE" || fin.WinnerID != BotPlayerID {
		t.Fatalf("host %d <= bot %d must LOSE to bot, got %+v", hostScore, botScore, fin)
	}
	time.Sleep(50 * time.Millisecond)
	if _, finished := p.snapshot(); len(finished) != 1 || finished[0] {
		t.Fatalf("expected one LOSE persisted, got %v", finished)
	}
}

func TestInitialBotState(t *testing.T) {
	r, _ := newTestRoom(t, 300, 42, true)
	st := r.Snapshot()
	if st.BotScore != 0 || st.BotHandNumber != 0 {
		t.Fatalf("bot score/handNumber should start at 0: %+v", st)
	}
	if st.BotHandsLeft != InitialHands || st.BotDiscardsLeft != InitialDiscards {
		t.Fatalf("bot hands/discards left %d/%d, want %d/%d", st.BotHandsLeft, st.BotDiscardsLeft, InitialHands, InitialDiscards)
	}
	if len(st.BotHand) != HandSize {
		t.Fatalf("snapshot bot_hand size %d, want %d", len(st.BotHand), HandSize)
	}
	r.mu.RLock()
	botHand := append([]models.Card(nil), r.botHand...)
	r.mu.RUnlock()
	if len(botHand) != HandSize {
		t.Fatalf("bot hand size %d, want %d", len(botHand), HandSize)
	}
}

func TestNoBotStateIsZero(t *testing.T) {
	r, _ := newTestRoom(t, 300, 42, false)
	st := r.Snapshot()
	if st.BotScore != 0 || st.BotHandsLeft != 0 || st.BotDiscardsLeft != 0 || st.BotHandNumber != 0 {
		t.Fatalf("bot state must be zero without a bot: %+v", st)
	}
	if len(st.BotHand) != 0 {
		t.Fatalf("bot_hand must be nil/empty without a bot: %+v", st.BotHand)
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.botDealer != nil || r.botHand != nil {
		t.Fatalf("bot dealer/hand must be nil without a bot")
	}
}

func TestBotSeedDoesNotAffectHostDeal(t *testing.T) {
	// The bot dealer derives from seed^0xB07; the host's own hand/deck must be identical
	// whether or not a bot is present (same seed).
	withBot, _ := newTestRoom(t, 300, 123, true)
	noBot, _ := newTestRoom(t, 300, 123, false)
	a, b := withBot.Snapshot(), noBot.Snapshot()
	if a.DeckCount != b.DeckCount {
		t.Fatalf("host deck count differs: %d vs %d", a.DeckCount, b.DeckCount)
	}
	for i := range a.CardsInHand {
		if a.CardsInHand[i] != b.CardsInHand[i] {
			t.Fatalf("host hand differs at %d: %v vs %v", i, a.CardsInHand[i], b.CardsInHand[i])
		}
	}
}

func TestBotActsOnceAfterHostPlay(t *testing.T) {
	// Unreachable target so no match-end interferes. After one host PLAY the bot takes
	// exactly one action: either a PLAY (host HAND_SCORED, host STATE, bot HAND_SCORED, bot
	// STATE) or a DISCARD (host HAND_SCORED, host STATE, bot STATE).
	r, _ := newTestRoom(t, 1000000000, 55, true)
	c := connect(t, r, host)
	expect[StatePayload](t, c, MsgRoomState)

	r.Play(c, []int{0, 1, 2})
	hostScored := expect[HandScoredPayload](t, c, MsgHandScored)
	if hostScored.PlayerID != host.ID {
		t.Fatalf("first HAND_SCORED must be the host, got %s", hostScored.PlayerID)
	}
	expect[StatePayload](t, c, MsgRoomState) // host STATE

	// BOT_ACTION announces the bot's decision before it mutates its hand.
	botAction := expect[BotActionPayload](t, c, MsgBotAction)
	if botAction.Action != "PLAY" && botAction.Action != "DISCARD" {
		t.Fatalf("bot action %q, want PLAY or DISCARD", botAction.Action)
	}
	if len(botAction.CardIndices) < 1 || len(botAction.CardIndices) > 5 {
		t.Fatalf("bot action card indices %v, want 1..5", botAction.CardIndices)
	}

	typ, payload := next(t, c)
	if botAction.Action == "PLAY" && typ != MsgHandScored {
		t.Fatalf("BOT_ACTION PLAY must precede the bot HAND_SCORED, got %s", typ)
	}
	if botAction.Action == "DISCARD" && typ != MsgRoomState {
		t.Fatalf("BOT_ACTION DISCARD must precede the bot STATE, got %s", typ)
	}
	if typ == MsgHandScored {
		var botScored HandScoredPayload
		if err := json.Unmarshal(payload, &botScored); err != nil {
			t.Fatal(err)
		}
		if botScored.PlayerID != BotPlayerID || botScored.HandNumber != 1 {
			t.Fatalf("bot HAND_SCORED %+v", botScored)
		}
		if botScored.AccumulatedScore <= 0 {
			t.Fatalf("bot accumulated score should be > 0 on a play: %+v", botScored)
		}
		st := expect[StatePayload](t, c, MsgRoomState)
		if st.BotHandsLeft != InitialHands-1 || st.BotHandNumber != 1 || st.BotScore != botScored.AccumulatedScore {
			t.Fatalf("bot state after play %+v", st)
		}
	} else if typ == MsgRoomState {
		var st StatePayload
		if err := json.Unmarshal(payload, &st); err != nil {
			t.Fatal(err)
		}
		// A discard: no bot hand consumed, one discard spent, no bot HAND_SCORED.
		if st.BotHandsLeft != InitialHands || st.BotDiscardsLeft != InitialDiscards-1 || st.BotScore != 0 {
			t.Fatalf("bot discard state %+v", st)
		}
	} else {
		t.Fatalf("unexpected message after host turn: %s", typ)
	}

	// Exactly one bot action: nothing else is queued right now.
	select {
	case extra := <-c.Send:
		t.Fatalf("bot acted more than once; extra frame: %s", extra)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestBotDoesNotActAfterHostDiscard(t *testing.T) {
	r, _ := newTestRoom(t, 1000000000, 55, true)
	c := connect(t, r, host)
	expect[StatePayload](t, c, MsgRoomState)

	r.Discard(c, []int{0, 1})
	st := expect[StatePayload](t, c, MsgRoomState)
	// Host discard: host discard spent, bot untouched.
	if st.DiscardsLeft != InitialDiscards-1 {
		t.Fatalf("host discards left %d", st.DiscardsLeft)
	}
	if st.BotHandsLeft != InitialHands || st.BotDiscardsLeft != InitialDiscards || st.BotScore != 0 || st.BotHandNumber != 0 {
		t.Fatalf("bot must not act after a host discard: %+v", st)
	}
	// No further frames: the bot stayed put.
	select {
	case extra := <-c.Send:
		t.Fatalf("bot acted after a host discard; extra frame: %s", extra)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestFinishRuleHostTargetWin(t *testing.T) {
	r, _ := newTestRoom(t, 100, 7, true)
	r.mu.Lock()
	r.score = 150
	r.botScore = 999
	fin := r.finishAfterHostPlayLocked()
	r.mu.Unlock()
	if fin == nil || fin.Result != models.ResultWin || fin.WinnerID != host.ID || fin.Reason != "TARGET_SCORE_REACHED" {
		t.Fatalf("host at target must WIN regardless of bot score: %+v", fin)
	}
}

func TestFinishRuleHandsExhaustedHostAhead(t *testing.T) {
	r, _ := newTestRoom(t, 1000000000, 7, true)
	r.mu.Lock()
	r.handsLeft = 0
	r.score = 500
	r.botScore = 400
	fin := r.finishAfterHostPlayLocked()
	r.mu.Unlock()
	if fin == nil || fin.Result != models.ResultWin || fin.WinnerID != host.ID || fin.Reason != "HANDS_EXHAUSTED" {
		t.Fatalf("host ahead on exhaustion must WIN: %+v", fin)
	}
}

func TestFinishRuleHandsExhaustedBotAheadOrTie(t *testing.T) {
	for _, tc := range []struct {
		name       string
		host, bot  int
	}{
		{"bot ahead", 400, 500},
		{"tie", 400, 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, _ := newTestRoom(t, 1000000000, 7, true)
			r.mu.Lock()
			r.handsLeft = 0
			r.score = tc.host
			r.botScore = tc.bot
			fin := r.finishAfterHostPlayLocked()
			r.mu.Unlock()
			if fin == nil || fin.Result != models.ResultLose || fin.WinnerID != BotPlayerID || fin.Reason != "HANDS_EXHAUSTED" {
				t.Fatalf("%s must LOSE to bot: %+v", tc.name, fin)
			}
		})
	}
}

func TestFinishRuleNoBotHandsExhaustedLegacyLose(t *testing.T) {
	r, _ := newTestRoom(t, 1000000000, 7, false)
	r.mu.Lock()
	r.handsLeft = 0
	r.score = 500
	fin := r.finishAfterHostPlayLocked()
	r.mu.Unlock()
	if fin == nil || fin.Result != models.ResultLose || fin.WinnerID != "" || fin.Reason != "HANDS_EXHAUSTED" {
		t.Fatalf("no-bot exhaustion must LOSE with empty winner (legacy): %+v", fin)
	}
}

func TestBotReachesTargetFirstLose(t *testing.T) {
	// A low target the bot reaches on its turn after the host's first (non-winning) play.
	// Chosen seed/target so the host's single-card play stays below target while the bot's
	// best play clears it, yielding LOSE with the bot as winner.
	const target = 1000
	r, p := newTestRoom(t, target, 23, true)
	c := connect(t, r, host)
	expect[StatePayload](t, c, MsgRoomState)

	r.Play(c, []int{0}) // weak single-card play: host stays below target (~500 for this seed)
	fin, hostScore, botScore := drainUntilFinished(t, c)
	if hostScore >= target {
		t.Fatalf("host reached target on its own (%d); seed assumption invalid", hostScore)
	}
	if botScore < target {
		t.Fatalf("expected the bot to reach the target, bot=%d", botScore)
	}
	if fin.Result != models.ResultLose || fin.WinnerID != BotPlayerID || fin.Reason != "TARGET_SCORE_REACHED" {
		t.Fatalf("bot reaching target first must LOSE to bot: %+v", fin)
	}
	time.Sleep(50 * time.Millisecond)
	if _, finished := p.snapshot(); len(finished) != 1 || finished[0] {
		t.Fatalf("expected one LOSE persisted, got %v", finished)
	}
}

func TestFullMatchNeverRunsOutOfCards(t *testing.T) {
	// 3 discards of 5 plus InitialHands plays of 5 = 115 cards > 52: the dealer must recycle.
	r, _ := newTestRoom(t, 1000000000, 21, false)
	c := connect(t, r, host)
	expect[StatePayload](t, c, MsgRoomState)
	check := func(step string) {
		t.Helper()
		r.mu.RLock()
		defer r.mu.RUnlock()
		if len(r.hand) != HandSize {
			t.Fatalf("%s: hand size %d", step, len(r.hand))
		}
		seen := map[string]bool{}
		for _, card := range r.hand {
			seen[card.Rank+card.Suit] = true
		}
		if len(seen) != HandSize {
			t.Fatalf("%s: duplicate cards in hand", step)
		}
		if r.dealer.Total()+len(r.hand) != 52 {
			t.Fatalf("%s: dealer %d + hand %d != 52", step, r.dealer.Total(), len(r.hand))
		}
		if r.dealer.DeckCount() > 52-HandSize {
			t.Fatalf("%s: deck count %d", step, r.dealer.DeckCount())
		}
	}
	for i := 0; i < InitialDiscards; i++ {
		r.Discard(c, []int{0, 1, 2, 3, 4})
		expect[StatePayload](t, c, MsgRoomState)
		check("discard")
	}
	for i := 0; i < InitialHands; i++ {
		r.Play(c, []int{0, 1, 2, 3, 4})
		expect[HandScoredPayload](t, c, MsgHandScored)
		st := expect[StatePayload](t, c, MsgRoomState)
		if len(st.CardsInHand) != HandSize {
			t.Fatalf("hand size %d after play %d", len(st.CardsInHand), i+1)
		}
		check("play")
	}
	r.mu.RLock()
	n := r.dealer.Reshuffles()
	r.mu.RUnlock()
	if n < 2 {
		t.Fatalf("reshuffles %d, want >= 2", n)
	}
}

func TestSameSeedSameRecycling(t *testing.T) {
	run := func() []models.Card {
		r, _ := newTestRoom(t, 1000000000, 77, false)
		c := connect(t, r, host)
		expect[StatePayload](t, c, MsgRoomState)
		for i := 0; i < InitialDiscards; i++ {
			r.Discard(c, []int{0, 1, 2, 3, 4})
			expect[StatePayload](t, c, MsgRoomState)
		}
		for i := 0; i < InitialHands; i++ {
			r.Play(c, []int{0, 1, 2, 3, 4})
			expect[HandScoredPayload](t, c, MsgHandScored)
			expect[StatePayload](t, c, MsgRoomState)
		}
		return r.Snapshot().CardsInHand
	}
	a, b := run(), run()
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("hands differ at %d: %v vs %v", i, a[i], b[i])
		}
	}
}

func TestDefaultTargetScoreWhenZero(t *testing.T) {
	r, _ := newTestRoom(t, 0, 1, false)
	if got := r.Snapshot().TargetScore; got != models.DefaultTargetScore {
		t.Fatalf("target %d, want %d", got, models.DefaultTargetScore)
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
