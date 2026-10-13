package room

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"jokerarena-game-ms/internal/models"
	"jokerarena-game-ms/internal/poker"
)

// Game constants of Prototype 1.
const (
	HandSize        = 8
	InitialHands    = 20
	InitialDiscards = 3
	MaxParticipants = 4
)

// Persister stores match lifecycle changes. db.Repository satisfies it.
type Persister interface {
	MarkMatchInProgress(ctx context.Context, roomCode string) error
	FinishMatch(ctx context.Context, roomCode string, hostWon bool) error
}

// Options configures a new room.
type Options struct {
	MatchID     string
	Code        string
	TargetScore int
	WithBot     bool
	Host        PlayerInfo
	Seed        uint64 // 0 means a random seed
}

type cmdKind int

const (
	cmdJoin cmdKind = iota
	cmdLeave
	cmdPlay
	cmdDiscard
	cmdResend
	cmdReject
)

type command struct {
	kind    cmdKind
	client  *Client
	player  PlayerInfo
	indices []int
	code    string // cmdReject: error code
	message string // cmdReject: error message
}

// Room is the authoritative game session. All state changes happen in a single goroutine
// (the room actor) that consumes commands from a channel, so players can never race each
// other; REST handlers read consistent snapshots under a read lock.
type Room struct {
	opts      Options
	persister Persister
	cmds      chan command
	done      chan struct{}
	stopOnce  sync.Once

	clientCount atomic.Int32

	mu           sync.RWMutex
	status       string
	handsLeft    int
	discardsLeft int
	score        int
	handNumber   int
	dealer       *poker.Dealer
	hand         []models.Card
	players      []PlayerInfo
	result       *MatchFinishedPayload
	lastActive   time.Time
	createdAt    time.Time

	// Bot opponent state (populated only when opts.WithBot). The bot owns its own dealer
	// derived from the host seed, so its deck is deterministic yet independent of the host's.
	botDealer       *poker.Dealer
	botHand         []models.Card
	botScore        int
	botHandsLeft    int
	botDiscardsLeft int
	botHandNumber   int

	clients map[*Client]struct{} // owned by the actor goroutine
}

// newRoom builds a room, deals the first eight cards and starts the actor goroutine.
func newRoom(opts Options, persister Persister) *Room {
	if opts.TargetScore <= 0 {
		opts.TargetScore = models.DefaultTargetScore
	}
	opts.Code = strings.ToUpper(strings.TrimSpace(opts.Code))
	seed := opts.Seed
	if seed == 0 {
		seed = poker.RandomSeed()
	}
	dealer := poker.NewDealer(seed)
	hand := dealer.Draw(HandSize)

	players := []PlayerInfo{opts.Host}
	if opts.WithBot {
		players = append(players, PlayerInfo{ID: BotPlayerID, Alias: models.BotAlias, IsBot: true})
	}

	now := time.Now().UTC()
	r := &Room{
		opts:         opts,
		persister:    persister,
		cmds:         make(chan command, 64),
		done:         make(chan struct{}),
		status:       models.StatusWaiting,
		handsLeft:    InitialHands,
		discardsLeft: InitialDiscards,
		dealer:       dealer,
		hand:         hand,
		players:      players,
		lastActive:   now,
		createdAt:    now,
		clients:      make(map[*Client]struct{}),
	}
	if opts.WithBot {
		r.botDealer = poker.NewDealer(seed ^ 0xB07)
		r.botHand = r.botDealer.Draw(HandSize)
		r.botHandsLeft = InitialHands
		r.botDiscardsLeft = InitialDiscards
	}
	go r.run()
	return r
}

// BotPlayerID labels the bot opponent inside sessions.
const BotPlayerID = "bot"

// Code returns the room code.
func (r *Room) Code() string { return r.opts.Code }

// HostID returns the id of the player who owns the game.
func (r *Room) HostID() string { return r.opts.Host.ID }

// Stop terminates the actor goroutine and disconnects every client.
func (r *Room) Stop() { r.stopOnce.Do(func() { close(r.done) }) }

// Submit hands a command to the actor; it reports false once the room has stopped.
func (r *Room) submit(c command) bool {
	select {
	case r.cmds <- c:
		return true
	case <-r.done:
		return false
	}
}

// Join registers a client (host, additional player or spectator) and sends it the state.
func (r *Room) Join(c *Client, p PlayerInfo) bool {
	return r.submit(command{kind: cmdJoin, client: c, player: p})
}

// Leave unregisters a client.
func (r *Room) Leave(c *Client) bool { return r.submit(command{kind: cmdLeave, client: c}) }

// Resend asks for the current state to be sent again to a client.
func (r *Room) Resend(c *Client) bool { return r.submit(command{kind: cmdResend, client: c}) }

// Reject sends an ERROR message to one client (used for protocol errors found by the
// connection handler). The room goroutine is the only writer of client queues.
func (r *Room) Reject(c *Client, code, message string) bool {
	return r.submit(command{kind: cmdReject, client: c, code: code, message: message})
}

// Play submits a PLAY_HAND action with the selected card indices.
func (r *Room) Play(c *Client, indices []int) bool {
	return r.submit(command{kind: cmdPlay, client: c, indices: indices})
}

// Discard submits a DISCARD_CARDS action with the selected card indices.
func (r *Room) Discard(c *Client, indices []int) bool {
	return r.submit(command{kind: cmdDiscard, client: c, indices: indices})
}

// ClientCount returns the number of connected clients.
func (r *Room) ClientCount() int { return int(r.clientCount.Load()) }

// LastActive returns the time of the last client or game activity.
func (r *Room) LastActive() time.Time {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.lastActive
}

// Snapshot returns a consistent copy of the public room state.
func (r *Room) Snapshot() StatePayload {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.snapshotLocked()
}

func (r *Room) snapshotLocked() StatePayload {
	var botHand []models.Card
	if r.opts.WithBot {
		botHand = append([]models.Card(nil), r.botHand...)
	}
	return StatePayload{
		RoomCode:     r.opts.Code,
		MatchID:      r.opts.MatchID,
		Status:       r.status,
		TargetScore:  r.opts.TargetScore,
		HandsLeft:    r.handsLeft,
		DiscardsLeft: r.discardsLeft,
		CurrentScore: r.score,
		DeckCount:    r.dealer.DeckCount(),
		ActiveJokers: append([]string(nil), poker.ActiveJokers...),
		CardsInHand:  append([]models.Card(nil), r.hand...),
		HandNumber:   r.handNumber,
		WithBot:      r.opts.WithBot,
		HostID:       r.opts.Host.ID,
		Players:      append([]PlayerInfo(nil), r.players...),

		HandsTotal:    InitialHands,
		DiscardsTotal: InitialDiscards,

		BotScore:        r.botScore,
		BotHandsLeft:    r.botHandsLeft,
		BotHandNumber:   r.botHandNumber,
		BotDiscardsLeft: r.botDiscardsLeft,
		BotHand:         botHand,
	}
}

// ToDetailsResponse converts the snapshot to the REST representation of GET /api/rooms/{code}.
func (r *Room) ToDetailsResponse() models.RoomDetailsResponse {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ids := make([]string, len(r.players))
	for i, p := range r.players {
		ids[i] = p.ID
	}
	return models.RoomDetailsResponse{
		RoomCode:     r.opts.Code,
		MatchID:      r.opts.MatchID,
		TargetScore:  r.opts.TargetScore,
		Status:       r.status,
		WithBot:      r.opts.WithBot,
		PlayerCount:  len(r.players),
		HandsLeft:    r.handsLeft,
		DiscardsLeft: r.discardsLeft,
		CurrentScore: r.score,
		DeckCount:    r.dealer.DeckCount(),
		ActiveJokers: append([]string(nil), poker.ActiveJokers...),
		CardsInHand:  append([]models.Card(nil), r.hand...),
		Players:      ids,
		CreatedAt:    r.createdAt,
	}
}

// ---------------------------------------------------------------------------------------
// Actor
// ---------------------------------------------------------------------------------------

func (r *Room) run() {
	for {
		select {
		case c := <-r.cmds:
			r.handle(c)
		case <-r.done:
			for cl := range r.clients {
				cl.Close()
			}
			r.clients = map[*Client]struct{}{}
			r.clientCount.Store(0)
			return
		}
	}
}

func (r *Room) touch() {
	r.mu.Lock()
	r.lastActive = time.Now().UTC()
	r.mu.Unlock()
}

func (r *Room) handle(c command) {
	r.touch()
	switch c.kind {
	case cmdJoin:
		r.handleJoin(c)
	case cmdLeave:
		r.removeClient(c.client)
	case cmdResend:
		if _, ok := r.clients[c.client]; ok {
			r.sendState(c.client)
		}
	case cmdReject:
		if _, ok := r.clients[c.client]; ok {
			r.sendError(c.client, c.code, c.message)
		}
	case cmdPlay:
		r.handleAction(c, true)
	case cmdDiscard:
		r.handleAction(c, false)
	}
}

func (r *Room) removeClient(cl *Client) {
	if _, ok := r.clients[cl]; ok {
		delete(r.clients, cl)
		r.clientCount.Store(int32(len(r.clients)))
		cl.Close()
	}
}

func (r *Room) sendError(cl *Client, code, msg string) {
	if !cl.enqueue(marshal(MsgError, ErrorPayload{Code: code, Message: msg})) {
		r.removeClient(cl)
	}
}

func (r *Room) handleJoin(c command) {
	r.mu.Lock()
	known := false
	for _, p := range r.players {
		if p.ID == c.player.ID {
			known = true
			break
		}
	}
	if !known {
		if len(r.players) >= MaxParticipants {
			r.mu.Unlock()
			r.sendError(c.client, ErrCodeRoomFull, "the room already has the maximum number of participants")
			c.client.Close()
			return
		}
		r.players = append(r.players, c.player)
	}
	r.mu.Unlock()

	r.clients[c.client] = struct{}{}
	r.clientCount.Store(int32(len(r.clients)))
	// Everyone sees the new participant; the joiner also gets the current result if finished.
	r.broadcastState()
	if res := r.currentResult(); res != nil {
		if !c.client.enqueue(marshal(MsgMatchDone, res)) {
			r.removeClient(c.client)
		}
	}
}

func (r *Room) currentResult() *MatchFinishedPayload {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.result
}

func (r *Room) sendState(cl *Client) {
	if !cl.enqueue(marshal(MsgRoomState, r.Snapshot())) {
		r.removeClient(cl)
	}
}

func (r *Room) broadcast(frame []byte) {
	for cl := range r.clients {
		if !cl.enqueue(frame) {
			r.removeClient(cl) // too slow: drop it, never block the room
		}
	}
}

func (r *Room) broadcastState() { r.broadcast(marshal(MsgRoomState, r.Snapshot())) }

// validateSelection checks 1..5 unique in-range indices and returns them sorted.
func validateSelection(indices []int, handLen int) ([]int, error) {
	if len(indices) < 1 || len(indices) > poker.MaxPlayCards {
		return nil, fmt.Errorf("select between 1 and %d cards", poker.MaxPlayCards)
	}
	seen := make(map[int]bool, len(indices))
	out := make([]int, 0, len(indices))
	for _, i := range indices {
		if i < 0 || i >= handLen {
			return nil, fmt.Errorf("card index %d is out of range", i)
		}
		if seen[i] {
			return nil, fmt.Errorf("card index %d is repeated", i)
		}
		seen[i] = true
		out = append(out, i)
	}
	sort.Ints(out)
	return out, nil
}

// replaceCards removes the cards at the (sorted) indices from the hand and refills it from
// the deck up to HandSize. Caller holds the write lock.
func (r *Room) replaceCards(sorted []int) []models.Card {
	removed := make(map[int]bool, len(sorted))
	for _, i := range sorted {
		removed[i] = true
	}
	kept := make([]models.Card, 0, len(r.hand))
	taken := make([]models.Card, 0, len(sorted))
	for i, c := range r.hand {
		if removed[i] {
			taken = append(taken, c)
		} else {
			kept = append(kept, c)
		}
	}
	// Draw BEFORE discarding so the cards just removed cannot come straight back.
	kept = append(kept, r.dealer.Draw(HandSize-len(kept))...)
	r.dealer.Discard(taken)
	if len(kept) < HandSize {
		log.Printf("[ERROR] dealer short draw in %s: hand has %d cards", r.opts.Code, len(kept))
	}
	r.hand = kept
	return taken
}

func (r *Room) handleAction(c command, play bool) {
	cl := c.client
	if _, ok := r.clients[cl]; !ok {
		return // not joined
	}
	if cl.PlayerID != r.opts.Host.ID {
		r.sendError(cl, ErrCodeNotHost, "only the player who owns the match can play or discard")
		return
	}

	r.mu.RLock()
	status, hands, discards, handLen := r.status, r.handsLeft, r.discardsLeft, len(r.hand)
	r.mu.RUnlock()

	if status == models.StatusFinished {
		r.sendError(cl, ErrCodeMatchFinished, "the match has already finished")
		return
	}
	sorted, err := validateSelection(c.indices, handLen)
	if err != nil {
		r.sendError(cl, ErrCodeInvalidSelection, err.Error())
		return
	}
	if play && hands <= 0 {
		r.sendError(cl, ErrCodeNoHands, "no hands left")
		return
	}
	if !play && discards <= 0 {
		r.sendError(cl, ErrCodeNoDiscards, "no discards left")
		return
	}

	if play {
		r.doPlay(sorted, discards)
	} else {
		r.mu.Lock()
		r.replaceCards(sorted)
		r.discardsLeft--
		r.markInProgressLocked()
		r.mu.Unlock()
		r.broadcastState()
	}
}

// markInProgressLocked flips WAITING to IN_PROGRESS on the first action. Caller holds mu.
func (r *Room) markInProgressLocked() {
	if r.status != models.StatusWaiting {
		return
	}
	r.status = models.StatusInProgress
	if r.persister != nil {
		code, p := r.opts.Code, r.persister
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := p.MarkMatchInProgress(ctx, code); err != nil {
				log.Printf("[WARN] persist IN_PROGRESS for %s failed: %v", code, err)
			}
		}()
	}
}

func (r *Room) doPlay(sorted []int, discardsLeft int) {
	r.mu.Lock()
	selected := make([]models.Card, len(sorted))
	for i, idx := range sorted {
		selected[i] = r.hand[idx]
	}
	breakdown, err := poker.Score(selected, discardsLeft)
	if err != nil { // cannot happen after validateSelection
		r.mu.Unlock()
		log.Printf("[ERROR] score failed in %s: %v", r.opts.Code, err)
		return
	}
	r.replaceCards(sorted)
	r.handsLeft--
	r.handNumber++
	r.score += breakdown.Score
	r.markInProgressLocked()

	scored := HandScoredPayload{
		RoomCode:         r.opts.Code,
		MatchID:          r.opts.MatchID,
		PlayerID:         r.opts.Host.ID,
		HandNumber:       r.handNumber,
		AccumulatedScore: r.score,
		Timestamp:        time.Now().UTC(),
		Breakdown:        *breakdown,
	}

	// The host PLAY may already decide the match: either the host hit the target, or the
	// host ran out of hands (settled by score comparison against the bot). When it does,
	// the bot does NOT take a turn.
	finished := r.finishAfterHostPlayLocked()
	r.mu.Unlock()

	r.broadcast(marshal(MsgHandScored, scored))
	r.broadcastState()
	if finished != nil {
		r.broadcast(marshal(MsgMatchDone, finished))
		return
	}
	if r.opts.WithBot {
		r.botTakeTurn()
	}
}

// finishAfterHostPlayLocked applies the end-of-match rule triggered by a host PLAY. Caller
// holds mu. It returns a finished payload (and marks the room FINISHED) when the match ends,
// or nil to let play continue (and, with a bot, let the bot take its turn).
//
// Rules (host perspective):
//   - host reaches the target        -> WIN,  winner=host, TARGET_SCORE_REACHED
//   - host hands exhausted (no bot)   -> LOSE, HANDS_EXHAUSTED (unchanged legacy behavior)
//   - host hands exhausted (with bot) -> score comparison: host>bot WIN winner=host,
//     otherwise LOSE winner=bot; HANDS_EXHAUSTED
func (r *Room) finishAfterHostPlayLocked() *MatchFinishedPayload {
	switch {
	case r.score >= r.opts.TargetScore:
		return r.finishLocked(true, "TARGET_SCORE_REACHED")
	case r.handsLeft <= 0:
		if r.opts.WithBot && r.score <= r.botScore {
			return r.finishLocked(false, "HANDS_EXHAUSTED")
		}
		// No bot: legacy LOSE. With bot and host ahead: WIN.
		return r.finishLocked(r.opts.WithBot && r.score > r.botScore, "HANDS_EXHAUSTED")
	}
	return nil
}

// botTakeTurn runs the bot's single reactive action after a host PLAY that did not finish
// the match. The bot either PLAYs (scoring, consuming a hand) or DISCARDs (refilling, no
// score, no hand consumed). It first announces its decision with a BOT_ACTION frame (before
// mutating its hand), then mutates the bot fields under r.mu, broadcasting the bot's
// HAND_SCORED (on a PLAY) followed by a ROOM_STATE_UPDATE, so the per-round order observed
// by clients is: host HAND_SCORED, host STATE, BOT_ACTION, then PLAY: bot HAND_SCORED + bot
// STATE; DISCARD: bot STATE.
func (r *Room) botTakeTurn() {
	r.mu.Lock()

	action, indices := poker.BotDecision(r.botHand, r.botDiscardsLeft)
	sorted := append([]int(nil), indices...)
	sort.Ints(sorted)

	// Announce the bot's chosen action BEFORE mutating its hand, so the card indices still
	// reference the bot hand as last broadcast in a ROOM_STATE_UPDATE. Snapshot the payload
	// fields under the lock, release it to broadcast, then re-acquire for the mutation.
	botAction := BotActionPayload{
		RoomCode:    r.opts.Code,
		Action:      action,
		CardIndices: append([]int(nil), sorted...),
		HandNumber:  r.botHandNumber,
	}
	r.mu.Unlock()
	r.broadcast(marshal(MsgBotAction, botAction))
	r.mu.Lock()

	if action == "DISCARD" {
		r.replaceBotCards(sorted)
		r.botDiscardsLeft--
		r.mu.Unlock()
		r.broadcastState()
		return
	}

	// PLAY: score the selection against the bot's remaining discards.
	selected := make([]models.Card, len(sorted))
	for i, idx := range sorted {
		selected[i] = r.botHand[idx]
	}
	breakdown, err := poker.Score(selected, r.botDiscardsLeft)
	if err != nil { // cannot happen: BestPlay only returns valid 1..5 selections
		r.mu.Unlock()
		log.Printf("[ERROR] bot score failed in %s: %v", r.opts.Code, err)
		return
	}
	r.replaceBotCards(sorted)
	r.botHandsLeft--
	r.botHandNumber++
	r.botScore += breakdown.Score

	scored := HandScoredPayload{
		RoomCode:         r.opts.Code,
		MatchID:          r.opts.MatchID,
		PlayerID:         BotPlayerID,
		HandNumber:       r.botHandNumber,
		AccumulatedScore: r.botScore,
		Timestamp:        time.Now().UTC(),
		Breakdown:        *breakdown,
	}

	// The bot can win by reaching the target before the host does (rule b).
	var finished *MatchFinishedPayload
	if r.status != models.StatusFinished && r.botScore >= r.opts.TargetScore && r.score < r.opts.TargetScore {
		finished = r.finishLocked(false, "TARGET_SCORE_REACHED")
	}
	r.mu.Unlock()

	r.broadcast(marshal(MsgHandScored, scored))
	r.broadcastState()
	if finished != nil {
		r.broadcast(marshal(MsgMatchDone, finished))
	}
}

// replaceBotCards is replaceCards for the bot's hand/dealer. Caller holds mu.
func (r *Room) replaceBotCards(sorted []int) {
	removed := make(map[int]bool, len(sorted))
	for _, i := range sorted {
		removed[i] = true
	}
	kept := make([]models.Card, 0, len(r.botHand))
	taken := make([]models.Card, 0, len(sorted))
	for i, c := range r.botHand {
		if removed[i] {
			taken = append(taken, c)
		} else {
			kept = append(kept, c)
		}
	}
	kept = append(kept, r.botDealer.Draw(HandSize-len(kept))...)
	r.botDealer.Discard(taken)
	if len(kept) < HandSize {
		log.Printf("[ERROR] bot dealer short draw in %s: hand has %d cards", r.opts.Code, len(kept))
	}
	r.botHand = kept
}

// finishLocked closes the match and persists the outcome. Caller holds mu.
func (r *Room) finishLocked(won bool, reason string) *MatchFinishedPayload {
	r.status = models.StatusFinished
	winner := ""
	result := models.ResultLose
	if won {
		winner, result = r.opts.Host.ID, models.ResultWin
	} else if r.opts.WithBot {
		winner = BotPlayerID
	}
	r.result = &MatchFinishedPayload{
		RoomCode:      r.opts.Code,
		MatchID:       r.opts.MatchID,
		WinnerID:      winner,
		Result:        result,
		FinalScore:    r.score,
		TargetScore:   r.opts.TargetScore,
		TargetReached: r.score >= r.opts.TargetScore,
		Reason:        reason,
		HandsPlayed:   r.handNumber,
	}
	if r.persister != nil {
		code, p := r.opts.Code, r.persister
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := p.FinishMatch(ctx, code, won); err != nil {
				log.Printf("[WARN] persist result for %s failed: %v", code, err)
			}
		}()
	}
	return r.result
}
