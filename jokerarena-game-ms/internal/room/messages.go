package room

import (
	"encoding/json"
	"time"

	"jokerarena-game-ms/internal/models"
	"jokerarena-game-ms/internal/poker"
)

// Message types of the WebSocket protocol (connector C2).
const (
	// Client -> server
	MsgJoinRoom = "JOIN_ROOM"
	MsgPlayHand = "PLAY_HAND"
	MsgDiscard  = "DISCARD_CARDS"
	// Server -> client
	MsgRoomState  = "ROOM_STATE_UPDATE"
	MsgHandScored = "HAND_SCORED"
	MsgMatchDone  = "MATCH_FINISHED"
	MsgError      = "ERROR"
)

// Error codes carried by ERROR messages.
const (
	ErrCodeInvalidMessage   = "INVALID_MESSAGE"
	ErrCodeInvalidSelection = "INVALID_SELECTION"
	ErrCodeNotHost          = "NOT_ALLOWED"
	ErrCodeMatchFinished    = "MATCH_FINISHED"
	ErrCodeNoHands          = "NO_HANDS_LEFT"
	ErrCodeNoDiscards       = "NO_DISCARDS_LEFT"
	ErrCodeRoomFull         = "ROOM_FULL"
	ErrCodeRoomNotFound     = "ROOM_NOT_FOUND"
	ErrCodeUnknownPlayer    = "UNKNOWN_PLAYER"
	ErrCodeUnavailable      = "STORAGE_UNAVAILABLE"
	ErrCodeNotJoined        = "NOT_JOINED"
)

// Envelope is the standard wrapper of every message: {"type": ..., "payload": ...}.
type Envelope struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// outbound is the server-side counterpart of Envelope with a typed payload.
type outbound struct {
	Type    string `json:"type"`
	Payload any    `json:"payload"`
}

// SelectionPayload is the payload of PLAY_HAND and DISCARD_CARDS.
type SelectionPayload struct {
	CardIndices []int `json:"card_indices"`
}

// JoinPayload is the payload of JOIN_ROOM.
type JoinPayload struct {
	RoomCode string `json:"room_code"`
	PlayerID string `json:"player_id"`
}

// PlayerInfo describes a participant of the room.
type PlayerInfo struct {
	ID    string `json:"id"`
	Alias string `json:"alias"`
	IsBot bool   `json:"is_bot"`
}

// StatePayload is the payload of ROOM_STATE_UPDATE (see section 3.2.2 of the specification,
// extended with status, host and participants).
type StatePayload struct {
	RoomCode     string        `json:"room_code"`
	MatchID      string        `json:"match_id"`
	Status       string        `json:"status"`
	TargetScore  int           `json:"target_score"`
	HandsLeft    int           `json:"hands_left"`
	DiscardsLeft int           `json:"discards_left"`
	CurrentScore int           `json:"current_score"`
	DeckCount    int           `json:"deck_count"`
	ActiveJokers []string      `json:"active_jokers"`
	CardsInHand  []models.Card `json:"cards_in_hand"`
	HandNumber   int           `json:"hand_number"`
	WithBot      bool          `json:"with_bot"`
	HostID       string        `json:"host_id"`
	Players      []PlayerInfo  `json:"players"`

	HandsTotal    int `json:"hands_total"`
	DiscardsTotal int `json:"discards_total"`
}

// HandScoredPayload is the payload of HAND_SCORED: the full algebraic audit of a played hand.
type HandScoredPayload struct {
	RoomCode         string    `json:"room_code"`
	MatchID          string    `json:"match_id"`
	PlayerID         string    `json:"player_id"`
	HandNumber       int       `json:"hand_number"`
	AccumulatedScore int       `json:"accumulated_score"`
	Timestamp        time.Time `json:"timestamp"`
	poker.Breakdown
}

// MatchFinishedPayload is the payload of MATCH_FINISHED.
type MatchFinishedPayload struct {
	RoomCode      string `json:"room_code"`
	MatchID       string `json:"match_id"`
	WinnerID      string `json:"winner_id"`
	Result        string `json:"result"` // WIN | LOSE (from the host's perspective)
	FinalScore    int    `json:"final_score"`
	TargetScore   int    `json:"target_score"`
	TargetReached bool   `json:"target_reached"`
	Reason        string `json:"reason"` // TARGET_SCORE_REACHED | HANDS_EXHAUSTED
	HandsPlayed   int    `json:"hands_played"`
}

// ErrorPayload is the payload of ERROR messages.
type ErrorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
