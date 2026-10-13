package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"jokerarena-game-ms/internal/db"
	"jokerarena-game-ms/internal/models"
	"jokerarena-game-ms/internal/room"
)

const maxTargetScore = 100000

// HandleCreateRoom processes POST /api/rooms.
func (h *Handler) HandleCreateRoom(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	var req models.CreateRoomRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	targetScore := req.TargetScore
	if targetScore == 0 {
		targetScore = models.DefaultTargetScore
	}
	if targetScore < 0 || targetScore > maxTargetScore {
		WriteError(w, http.StatusBadRequest, "target_score must be between 1 and 100000")
		return
	}

	withBot := true
	if req.WithBot != nil {
		withBot = *req.WithBot
	}

	playerID := strings.TrimSpace(req.PlayerID)
	if playerID == "" {
		WriteError(w, http.StatusBadRequest, "player_id is required")
		return
	}
	if !models.IsValidPlayerID(playerID) {
		WriteError(w, http.StatusBadRequest, "Invalid player_id: "+playerID)
		return
	}

	// Resolve the host (also validates it is a registered player).
	host, err := h.repo.GetPlayerByID(r.Context(), playerID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			WriteError(w, http.StatusBadRequest, "Unknown player_id: "+playerID)
			return
		}
		WriteStorageError(w, err, http.StatusInternalServerError, "Failed to look up player")
		return
	}

	// Generate a unique JKR-XXXX code; the repository guarantees uniqueness atomically.
	var roomCode, matchID string
	for attempt := 0; attempt < 10; attempt++ {
		roomCode = room.GenerateRoomCode()
		matchID, err = h.repo.CreateRoomMatch(r.Context(), roomCode, targetScore, playerID, withBot)
		if !errors.Is(err, db.ErrCodeTaken) {
			break
		}
	}
	if err != nil {
		WriteStorageError(w, err, http.StatusInternalServerError, "Failed to persist room")
		return
	}

	// Initialize the authoritative game session (shuffled deck, 8 cards dealt).
	session := h.roomMgr.Create(room.Options{
		MatchID:     matchID,
		Code:        roomCode,
		TargetScore: targetScore,
		WithBot:     withBot,
		Host:        room.PlayerInfo{ID: host.ID, Alias: host.Alias},
	})
	d := session.ToDetailsResponse()

	WriteJSON(w, http.StatusCreated, models.CreateRoomResponse{
		RoomCode:    d.RoomCode,
		MatchID:     d.MatchID,
		TargetScore: d.TargetScore,
		Status:      d.Status,
		WithBot:     d.WithBot,
		Players:     d.Players,
	})
}

// HandleGetRoom processes GET /api/rooms/{code} with case-insensitive lookup.
func (h *Handler) HandleGetRoom(w http.ResponseWriter, r *http.Request) {
	rawCode := r.PathValue("code")
	if strings.TrimSpace(rawCode) == "" {
		WriteError(w, http.StatusNotFound, "Room code required")
		return
	}

	cleanCode, err := models.ValidateRoomCode(rawCode)
	if err != nil {
		WriteError(w, http.StatusNotFound, "Room not found")
		return
	}

	// 1. Active in-memory session (contains the live game state)
	if session, found := h.roomMgr.Get(cleanCode); found {
		WriteJSON(w, http.StatusOK, session.ToDetailsResponse())
		return
	}

	// 2. Persisted room that is not active in memory (e.g. after a restart)
	match, players, err := h.repo.GetMatchByCode(r.Context(), cleanCode)
	if err != nil {
		WriteStorageError(w, err, http.StatusInternalServerError, "Room not found")
		return
	}

	WriteJSON(w, http.StatusOK, models.RoomDetailsResponse{
		RoomCode:     match.RoomCode,
		MatchID:      match.ID,
		TargetScore:  match.TargetScore,
		Status:       match.Status,
		WithBot:      match.WithBot,
		PlayerCount:  len(players),
		HandsLeft:    0,
		DiscardsLeft: 0,
		CurrentScore: 0,
		DeckCount:    0,
		ActiveJokers: []string{"banner", "joker", "the_duo"},
		CardsInHand:  []models.Card{},
		Players:      players,
		CreatedAt:    match.StartedAt,
	})
}
