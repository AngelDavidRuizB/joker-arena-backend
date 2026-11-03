package room

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"game-engine-ms/internal/poker"
)

type capturedEvent struct {
	key     string
	payload map[string]any
}

type fakePublisher struct {
	mu     sync.Mutex
	events []capturedEvent
}

func (f *fakePublisher) Publish(key string, payload any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	m, _ := payload.(map[string]any)
	f.events = append(f.events, capturedEvent{key, m})
}

func (f *fakePublisher) byKey(key string) []map[string]any {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []map[string]any
	for _, e := range f.events {
		if e.key == key {
			out = append(out, e.payload)
		}
	}
	return out
}

type fakeBot struct {
	calls  int
	action string
	idx    []int
	err    error
}

func (b *fakeBot) Decide(_ context.Context, req BotRequest) (string, []int, error) {
	b.calls++
	if b.err != nil {
		return "", nil, b.err
	}
	return b.action, b.idx, nil
}

func newIntegratedRoom(t *testing.T, target int, pub EventPublisher, rules RulesSource, bot BotDecider) *Room {
	t.Helper()
	m := NewManager(&fakePersister{})
	m.SetDefaults(Options{Publisher: pub, Rules: rules, Bot: bot})
	t.Cleanup(m.Close)
	return m.Create(Options{MatchID: "m-1", Code: "JKR-EVT1", TargetScore: target, WithBot: true, Host: host, Seed: 7})
}

func TestRoomPublishesTheMatchLifecycle(t *testing.T) {
	pub := &fakePublisher{}
	r := newIntegratedRoom(t, 1, pub, nil, nil) // target 1: the first play wins
	c := connect(t, r, host)
	expect[StatePayload](t, c, MsgRoomState)
	r.Play(c, []int{0})
	fin, _, _ := drainUntilFinished(t, c)
	if fin.Result != "WIN" {
		t.Fatalf("expected WIN, got %+v", fin)
	}
	deadline := time.Now().Add(time.Second)
	for len(pub.byKey(EventMatchEnded)) == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	started, scored, ended := pub.byKey(EventMatchStarted), pub.byKey(EventHandScored), pub.byKey(EventMatchEnded)
	if len(started) != 1 || len(scored) != 1 || len(ended) != 1 {
		t.Fatalf("events: started=%d scored=%d ended=%d", len(started), len(scored), len(ended))
	}
	if scored[0]["player_id"] != host.ID || scored[0]["ruleset_version"] != 1 || scored[0]["is_bot"] != false {
		t.Fatalf("hand.scored payload: %+v", scored[0])
	}
	results := ended[0]["results"].([]map[string]any)
	if len(results) != 2 || results[0]["won"] != true || results[1]["player_id"] != BotPlayerID || results[1]["is_bot"] != true {
		t.Fatalf("match.ended results: %+v", results)
	}
}

func TestRoomScoresWithTheCurrentRuleset(t *testing.T) {
	pub := &fakePublisher{}
	rs := poker.DefaultRuleset()
	rs.Version = 5
	rs.HappyHour.Enabled, rs.HappyHour.GlobalMult = true, 3
	r := newIntegratedRoom(t, 1000000, pub, func() poker.Ruleset { return rs }, &fakeBot{action: "PLAY", idx: []int{0}})
	c := connect(t, r, host)
	expect[StatePayload](t, c, MsgRoomState)
	r.Play(c, []int{0})
	hs := expect[HandScoredPayload](t, c, MsgHandScored)
	if hs.RulesetVersion != 5 {
		t.Fatalf("hand must be scored with ruleset v5, got v%d", hs.RulesetVersion)
	}
	// High card with 3 discards: (5 + chips + 90) x ((1 + 4) x 3)
	if hs.FinalMult != 15 {
		t.Fatalf("happy hour x3 must give mult 15, got %d", hs.FinalMult)
	}
}

func TestRoomUsesRemoteBotAndFallsBackOnError(t *testing.T) {
	remote := &fakeBot{action: "PLAY", idx: []int{0, 1}}
	r := newIntegratedRoom(t, 1000000, &fakePublisher{}, nil, remote)
	c := connect(t, r, host)
	expect[StatePayload](t, c, MsgRoomState)
	r.Play(c, []int{0})
	expect[HandScoredPayload](t, c, MsgHandScored)
	expect[StatePayload](t, c, MsgRoomState)
	ba := expect[BotActionPayload](t, c, MsgBotAction)
	if remote.calls != 1 || ba.Action != "PLAY" || len(ba.CardIndices) != 2 {
		t.Fatalf("remote bot not used: calls=%d action=%+v", remote.calls, ba)
	}

	broken := &fakeBot{err: errors.New("bot-ai down")}
	r2 := newIntegratedRoom(t, 1000000, &fakePublisher{}, nil, broken)
	r2.opts.Code = "JKR-EVT2"
	c2 := connect(t, r2, host)
	expect[StatePayload](t, c2, MsgRoomState)
	r2.Play(c2, []int{0})
	expect[HandScoredPayload](t, c2, MsgHandScored)
	expect[StatePayload](t, c2, MsgRoomState)
	if ba2 := expect[BotActionPayload](t, c2, MsgBotAction); ba2.Action == "" || broken.calls != 1 {
		t.Fatalf("fallback did not produce an action: %+v", ba2)
	}
}

func TestInvalidRemoteBotAnswerIsRejected(t *testing.T) {
	r := newIntegratedRoom(t, 1000000, &fakePublisher{}, nil, &fakeBot{action: "PLAY", idx: []int{9, 9}})
	c := connect(t, r, host)
	expect[StatePayload](t, c, MsgRoomState)
	r.Play(c, []int{0})
	expect[HandScoredPayload](t, c, MsgHandScored)
	expect[StatePayload](t, c, MsgRoomState)
	ba := expect[BotActionPayload](t, c, MsgBotAction)
	for _, i := range ba.CardIndices {
		if i < 0 || i >= HandSize {
			t.Fatalf("invalid index reached the room: %v", ba.CardIndices)
		}
	}
}
