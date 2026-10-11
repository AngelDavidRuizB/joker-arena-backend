package models

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

var roomCodeRegex = regexp.MustCompile(`^[A-Z0-9]{4,10}$`)
var uuidRegex = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// Match represents a persisted game record in the PostgreSQL matches table.
type Match struct {
	ID          string     `json:"id"`
	RoomCode    string     `json:"room_code"`
	TargetScore int        `json:"target_score"`
	Status      string     `json:"status"` // 'WAITING', 'IN_PROGRESS', 'FINISHED'
	WinnerID    *string    `json:"winner_id,omitempty"`
	StartedAt   time.Time  `json:"started_at"`
	FinishedAt  *time.Time `json:"finished_at,omitempty"`
}

// MatchPlayer represents a participant linked to a match in match_players.
type MatchPlayer struct {
	MatchID      string `json:"match_id"`
	PlayerID     string `json:"player_id"`
	IsBot        bool   `json:"is_bot"`
	FinalScore   int    `json:"final_score"`
	HandsLeft    int    `json:"hands_left"`
	DiscardsLeft int    `json:"discards_left"`
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

// IsValidUUID validates UUID format.
func IsValidUUID(raw string) bool {
	return uuidRegex.MatchString(strings.TrimSpace(raw))
}
