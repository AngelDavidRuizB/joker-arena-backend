package models

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// BotAlias is the reserved username of the bot opponent in the players table.
const BotAlias = "Bot-JokerBot"

// Match status values exposed through the API. In PostgreSQL they are derived from
// matches.result: WAITING / IN_PROGRESS are stored verbatim; WIN / LOSE map to FINISHED.
const (
	StatusWaiting    = "WAITING"
	StatusInProgress = "IN_PROGRESS"
	StatusFinished   = "FINISHED"
)

// Result values stored in matches.result.
const (
	ResultWaiting    = "WAITING"
	ResultInProgress = "IN_PROGRESS"
	ResultWin        = "WIN"
	ResultLose       = "LOSE"
)

// roomCodeRegex accepts codes such as JKR-A7K2 (generated) or JKR-402 (legacy docs).
var roomCodeRegex = regexp.MustCompile(`^[A-Z0-9][A-Z0-9-]{2,11}$`)

// playerIDRegex validates the numeric (SERIAL) player identifier, exposed as string.
var playerIDRegex = regexp.MustCompile(`^[0-9]{1,9}$`)

// Match represents a game record reconstructed from the PostgreSQL rooms/matches tables.
// ID is the id_match of the host's participation row, exposed as string.
type Match struct {
	ID          string     `json:"id"`
	RoomCode    string     `json:"room_code"`
	TargetScore int        `json:"target_score"`
	WithBot     bool       `json:"with_bot"`
	Status      string     `json:"status"` // WAITING | IN_PROGRESS | FINISHED
	Result      string     `json:"result"` // host result: WAITING | IN_PROGRESS | WIN | LOSE
	HostID      string     `json:"host_id"`
	StartedAt   time.Time  `json:"started_at"`
	FinishedAt  *time.Time `json:"finished_at,omitempty"`
}

// CreateRoomRequest represents inbound POST /api/rooms payload.
type CreateRoomRequest struct {
	PlayerID    string `json:"player_id"`
	TargetScore int    `json:"target_score"`
	WithBot     *bool  `json:"with_bot"`
}

// CreateRoomResponse represents response for POST /api/rooms.
type CreateRoomResponse struct {
	RoomCode    string   `json:"room_code"`
	MatchID     string   `json:"match_id"`
	TargetScore int      `json:"target_score"`
	Status      string   `json:"status"`
	WithBot     bool     `json:"with_bot"`
	Players     []string `json:"players"`
}

// RoomDetailsResponse represents response for GET /api/rooms/{code}.
type RoomDetailsResponse struct {
	RoomCode     string    `json:"room_code"`
	MatchID      string    `json:"match_id"`
	TargetScore  int       `json:"target_score"`
	Status       string    `json:"status"`
	WithBot      bool      `json:"with_bot"`
	PlayerCount  int       `json:"player_count"`
	HandsLeft    int       `json:"hands_left"`
	DiscardsLeft int       `json:"discards_left"`
	CurrentScore int       `json:"current_score"`
	DeckCount    int       `json:"deck_count"`
	ActiveJokers []string  `json:"active_jokers"`
	CardsInHand  []Card    `json:"cards_in_hand"`
	Players      []string  `json:"players"`
	CreatedAt    time.Time `json:"created_at"`
}

// HealthResponse represents GET /health response.
type HealthResponse struct {
	Status   string `json:"status"`
	Service  string `json:"service"`
	Postgres string `json:"postgres"`
	Storage  string `json:"storage"`
}

// ErrorResponse represents standard REST JSON error envelope.
type ErrorResponse struct {
	Error string `json:"error"`
}

// ValidateRoomCode normalizes and validates room code syntax.
func ValidateRoomCode(raw string) (string, error) {
	code := strings.ToUpper(strings.TrimSpace(raw))
	if code == "" {
		return "", fmt.Errorf("room code required")
	}
	if !roomCodeRegex.MatchString(code) {
		return "", fmt.Errorf("invalid room code format: %s", code)
	}
	return code, nil
}

// IsValidPlayerID validates the numeric player identifier format.
func IsValidPlayerID(raw string) bool {
	return playerIDRegex.MatchString(strings.TrimSpace(raw))
}
