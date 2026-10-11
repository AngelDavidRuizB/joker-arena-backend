package room

import (
	"crypto/rand"
	"math/big"
	"strings"
	"sync"
	"time"

	"jokerarena-game-ms/internal/models"
)

const BotPlayerID = "00000000-0000-0000-0000-000000000001"

// RoomSession represents an active in-memory game room session.
type RoomSession struct {
	mu           sync.RWMutex
	RoomCode     string        `json:"room_code"`
	MatchID      string        `json:"match_id"`
	TargetScore  int           `json:"target_score"`
	Status       string        `json:"status"`
	WithBot      bool          `json:"with_bot"`
	HandsLeft    int           `json:"hands_left"`
	DiscardsLeft int           `json:"discards_left"`
	CurrentScore int           `json:"current_score"`
	DeckCount    int           `json:"deck_count"`
	ActiveJokers []string      `json:"active_jokers"`
	CardsInHand  []models.Card `json:"cards_in_hand"`
	Deck         []models.Card `json:"-"`
	Players      []string      `json:"players"`
	CreatedAt    time.Time     `json:"created_at"`
}

// ToDetailsResponse converts RoomSession to models.RoomDetailsResponse safely.
func (s *RoomSession) ToDetailsResponse() models.RoomDetailsResponse {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cardsCopy := make([]models.Card, len(s.CardsInHand))
	copy(cardsCopy, s.CardsInHand)

	jokersCopy := make([]string, len(s.ActiveJokers))
	copy(jokersCopy, s.ActiveJokers)

	playersCopy := make([]string, len(s.Players))
	copy(playersCopy, s.Players)

	return models.RoomDetailsResponse{
		RoomCode:     s.RoomCode,
		MatchID:      s.MatchID,
		TargetScore:  s.TargetScore,
		Status:       s.Status,
		WithBot:      s.WithBot,
		HandsLeft:    s.HandsLeft,
		DiscardsLeft: s.DiscardsLeft,
		CurrentScore: s.CurrentScore,
		DeckCount:    s.DeckCount,
		ActiveJokers: jokersCopy,
		CardsInHand:  cardsCopy,
		Players:      playersCopy,
		CreatedAt:    s.CreatedAt,
	}
}

// RoomManager manages active room sessions in memory.
type RoomManager struct {
	mu    sync.RWMutex
	rooms map[string]*RoomSession
}

// NewRoomManager initializes RoomManager.
func NewRoomManager() *RoomManager {
	return &RoomManager{
		rooms: make(map[string]*RoomSession),
	}
}

// CreateRoom initializes a new session, generates the 52-card French deck, and deals 8 cards.
func (m *RoomManager) CreateRoom(matchID, roomCode string, targetScore int, withBot bool, hostPlayerID string) *RoomSession {
	m.mu.Lock()
	defer m.mu.Unlock()

	normCode := strings.ToUpper(strings.TrimSpace(roomCode))
	if targetScore <= 0 {
		targetScore = 300
	}

	deck := GenerateDeck()
	hand := make([]models.Card, 8)
	copy(hand, deck[:8])
	remainingDeck := make([]models.Card, len(deck)-8)
	copy(remainingDeck, deck[8:])

	players := []string{hostPlayerID}
	if withBot {
		players = append(players, BotPlayerID)
	}

	session := &RoomSession{
		RoomCode:     normCode,
		MatchID:      matchID,
		TargetScore:  targetScore,
		Status:       "WAITING",
		WithBot:      withBot,
		HandsLeft:    4,
		DiscardsLeft: 3,
		CurrentScore: 0,
		DeckCount:    len(remainingDeck),
		ActiveJokers: []string{"banner", "joker", "the_duo"},
		CardsInHand:  hand,
		Deck:         remainingDeck,
		Players:      players,
		CreatedAt:    time.Now().UTC(),
	}

	m.rooms[normCode] = session
	return session
}

// GetRoom retrieves an active room session by code (case-insensitive).
func (m *RoomManager) GetRoom(code string) (*RoomSession, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	normCode := strings.ToUpper(strings.TrimSpace(code))
	session, found := m.rooms[normCode]
	return session, found
}

// GenerateRoomCode generates a cryptographically random 6-character alphanumeric room code.
func GenerateRoomCode() string {
	const charset = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	b := make([]byte, 6)
	for i := range b {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		b[i] = charset[n.Int64()]
	}
	return string(b)
}

// GenerateDeck produces a 52-card French deck shuffled deterministically using Fisher-Yates.
func GenerateDeck() []models.Card {
	deck := make([]models.Card, 0, 52)
	for _, suit := range models.FrenchSuits {
		for _, rank := range models.FrenchRanks {
			deck = append(deck, models.Card{
				Rank:  rank,
				Suit:  suit,
				Chips: models.RankChips[rank],
			})
		}
	}

	// Fisher-Yates shuffle
	for i := len(deck) - 1; i > 0; i-- {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		j := int(n.Int64())
		deck[i], deck[j] = deck[j], deck[i]
	}

	return deck
}
