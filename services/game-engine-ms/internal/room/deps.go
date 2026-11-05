package room

import (
	"context"
	"log"
	"sort"
	"time"

	"game-engine-ms/internal/models"
	"game-engine-ms/internal/poker"
)

// ReasonAbandoned ends a match whose players left and never came back.
const ReasonAbandoned = "ABANDONED"

// Routing keys published by the engine (contracts/events.md).
const (
	EventMatchStarted = "game.match.started"
	EventHandScored   = "game.hand.scored"
	EventMatchEnded   = "game.match.ended"
)

// EventPublisher sends domain events to the broker. Publish must not block the room actor
// for long and must never panic; failures are the publisher's concern (logged, not fatal).
type EventPublisher interface {
	Publish(routingKey string, payload any)
}

// RulesSource returns the LiveOps ruleset currently in force.
type RulesSource func() poker.Ruleset

// BotRequest is what the engine sends to the bot decider for one bot turn.
type BotRequest struct {
	Hand         []models.Card
	HandsLeft    int
	DiscardsLeft int
	TargetScore  int
	CurrentScore int
	Difficulty   string
	Ruleset      poker.Ruleset
}

// BotDecider chooses the bot action: "PLAY" or "DISCARD" and the card indices (0-based).
type BotDecider interface {
	Decide(ctx context.Context, req BotRequest) (action string, indices []int, err error)
}

// MatchOutcome is the final result of a match, stored by persisters that support it.
type MatchOutcome struct {
	RoomCode    string
	MatchID     string
	HostWon     bool
	HostScore   int
	BotScore    int
	HandsPlayed int
	Reason      string
}

// MatchProgress is the resumable state of a match. Cards are not part of it: after an engine
// restart the match keeps its scores and counters and deals a new hand.
type MatchProgress struct {
	Seq             int `json:"seq"` // grows with every save, so late writes cannot go backwards
	Score           int `json:"score"`
	HandsLeft       int `json:"hands_left"`
	DiscardsLeft    int `json:"discards_left"`
	HandNumber      int `json:"hand_number"`
	BotScore        int `json:"bot_score"`
	BotHandsLeft    int `json:"bot_hands_left"`
	BotDiscardsLeft int `json:"bot_discards_left"`
	BotHandNumber   int `json:"bot_hand_number"`
}

// ProgressRecorder is an optional extension of Persister that stores MatchProgress.
type ProgressRecorder interface {
	SaveProgress(ctx context.Context, matchID string, p MatchProgress) error
}

// OrphanMatch is a match still IN_PROGRESS in storage that may have no live session.
type OrphanMatch struct {
	MatchID     string
	RoomCode    string
	Host        PlayerInfo
	WithBot     bool
	TargetScore int
	Difficulty  string
	Progress    *MatchProgress
}

// OrphanFinder is an optional extension of Persister that lists matches idle for too long.
type OrphanFinder interface {
	OrphanMatches(ctx context.Context, idle time.Duration) ([]OrphanMatch, error)
}

// OutcomeRecorder is an optional extension of Persister that stores final scores.
type OutcomeRecorder interface {
	RecordOutcome(ctx context.Context, o MatchOutcome) error
}

// MatchCreator is an optional extension of Persister that stores the match when the
// session is created.
type MatchCreator interface {
	CreateMatch(ctx context.Context, matchID, roomCode, hostID, hostAlias string, withBot bool, targetScore int, difficulty string) error
}

type noopPublisher struct{}

func (noopPublisher) Publish(string, any) {}

// localBot is the in-process strategy of Prototype 1, used when bot-ai-ms is unavailable.
type localBot struct{}

func (localBot) Decide(_ context.Context, req BotRequest) (string, []int, error) {
	action, idx := poker.BotDecision(req.Hand, req.DiscardsLeft)
	return action, idx, nil
}

// LocalBot exposes the in-process fallback strategy.
func LocalBot() BotDecider { return localBot{} }

// validBotSelection checks the indices returned by a remote decider before using them.
func validBotSelection(action string, indices []int, handLen int) ([]int, bool) {
	if action != "PLAY" && action != "DISCARD" {
		return nil, false
	}
	sorted, err := validateSelection(indices, handLen)
	if err != nil {
		return nil, false
	}
	return sorted, true
}

// decideBot asks the configured decider and falls back to the local strategy on any error
// or invalid answer, so a bot-ai-ms outage never blocks a match.
func (r *Room) decideBot(req BotRequest) (string, []int) {
	if r.opts.Bot != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 800*time.Millisecond)
		action, idx, err := r.opts.Bot.Decide(ctx, req)
		cancel()
		if err == nil {
			if sorted, ok := validBotSelection(action, idx, len(req.Hand)); ok {
				return action, sorted
			}
			log.Printf("[WARN] bot-ai answer rejected in %s: action=%q indices=%v", r.opts.Code, action, idx)
		} else {
			log.Printf("[WARN] bot-ai unavailable in %s, using local strategy: %v", r.opts.Code, err)
		}
	}
	action, idx := poker.BotDecision(req.Hand, req.DiscardsLeft)
	sorted := append([]int(nil), idx...)
	sort.Ints(sorted)
	return action, sorted
}

// saveProgressLocked persists the resumable state. Caller holds mu.
func (r *Room) saveProgressLocked() {
	rec, ok := r.persister.(ProgressRecorder)
	if !ok {
		return
	}
	r.progressSeq++
	p := MatchProgress{Seq: r.progressSeq, Score: r.score, HandsLeft: r.handsLeft, DiscardsLeft: r.discardsLeft,
		HandNumber: r.handNumber, BotScore: r.botScore, BotHandsLeft: r.botHandsLeft,
		BotDiscardsLeft: r.botDiscardsLeft, BotHandNumber: r.botHandNumber}
	id := r.opts.MatchID
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := rec.SaveProgress(ctx, id, p); err != nil {
			log.Printf("[WARN] persist progress of %s failed: %v", id, err)
		}
	}()
}

// abandon closes an in-progress match nobody came back to, so the lobby, the ranking and
// telemetry do not keep it open forever. Only call it on a session without clients.
func (r *Room) abandon() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.status == models.StatusInProgress {
		r.finishLocked(false, ReasonAbandoned)
	}
}

func (r *Room) rules() poker.Ruleset {
	if r.opts.Rules != nil {
		return r.opts.Rules()
	}
	return poker.DefaultRuleset()
}

func (r *Room) publish(routingKey string, payload any) {
	if r.opts.Publisher != nil {
		r.opts.Publisher.Publish(routingKey, payload)
	}
}

// handScoredEvent builds the game.hand.scored payload from an audit.
func handScoredEvent(code, matchID, playerID string, isBot bool, handNumber, accumulated int, b *poker.Breakdown) map[string]any {
	return map[string]any{
		"match_id":          matchID,
		"room_code":         code,
		"player_id":         playerID,
		"is_bot":            isBot,
		"hand_number":       handNumber,
		"hand_type":         b.PokerHand,
		"cards":             b.Cards,
		"base_chips":        b.BaseChips,
		"base_mult":         b.BaseMult,
		"card_chips":        b.CardChips,
		"discards_left":     b.DiscardsLeft,
		"chips":             b.FinalChips,
		"mult":              b.FinalMult,
		"score":             b.Score,
		"accumulated_score": accumulated,
		"jokers":            b.Jokers,
		"ruleset_version":   b.RulesetVersion,
	}
}

// decideBotLocalPlay forces a PLAY with the best local selection.
func (r *Room) decideBotLocalPlay(req BotRequest) (string, []int) {
	idx, _ := poker.BestPlay(req.Hand, req.DiscardsLeft)
	sorted := append([]int(nil), idx...)
	sort.Ints(sorted)
	return "PLAY", sorted
}
