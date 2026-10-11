package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"jokerarena-game-ms/internal/models"
	"jokerarena-game-ms/internal/room"
)

// HandleCreateRoom processes POST /api/rooms.
func (h *Handler) HandleCreateRoom(w http.ResponseWriter, r *http.Request) {
	var req models.CreateRoomRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req.TargetScore = 300
	}

	targetScore := req.TargetScore
	if targetScore <= 0 {
		targetScore = 300
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

	// Validate player exists in repository (satisfies boundary B5.3)
	if _, err := h.repo.GetPlayerByID(r.Context(), playerID); err != nil {
		WriteError(w, http.StatusBadRequest, "Unknown player_id: "+playerID)
		return
	}

	// Generate 6-char alphanumeric room code and UUID match ID
	roomCode := room.GenerateRoomCode()
	matchID := uuid.New().String()

	// Persist match and participants into repository
	if err := h.repo.CreateRoomMatch(r.Context(), matchID, roomCode, targetScore, playerID, withBot); err != nil {
		WriteError(w, http.StatusInternalServerError, "Failed to persist room match: "+err.Error())
		return
	}

	// Initialize active game session in memory with 8 dealt cards
	session := h.roomMgr.CreateRoom(matchID, roomCode, targetScore, withBot, playerID)

	resp := models.CreateRoomResponse{
		RoomCode:    session.RoomCode,
		MatchID:     session.MatchID,
		TargetScore: session.TargetScore,
		Status:      session.Status,
		WithBot:     session.WithBot,
		Players:     session.Players,
	}

	WriteJSON(w, http.StatusCreated, resp)
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

	// 1. Check in-memory session (contains active dealt cards)
	if session, found := h.roomMgr.GetRoom(cleanCode); found {
		WriteJSON(w, http.StatusOK, session.ToDetailsResponse())
		return
	}

	// 2. Fallback to database query if session not in active memory
	match, players, err := h.repo.GetMatchByCode(r.Context(), cleanCode)
	if err != nil || match == nil {
		WriteError(w, http.StatusNotFound, "Room not found")
		return
	}

	// Reconstruct response from persistent match
	resp := models.RoomDetailsResponse{
		RoomCode:     match.RoomCode,
		MatchID:      match.ID,
		TargetScore:  match.TargetScore,
		Status:       match.Status,
		WithBot:      len(players) > 1,
		HandsLeft:    4,
		DiscardsLeft: 3,
		CurrentScore: 0,
		DeckCount:    44,
		ActiveJokers: []string{"banner", "joker", "the_duo"},
		CardsInHand:  []models.Card{},
		Players:      players,
		CreatedAt:    match.StartedAt,
	}

	WriteJSON(w, http.StatusOK, resp)
}
