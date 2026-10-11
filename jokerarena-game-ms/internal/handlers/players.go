package handlers

import (
	"encoding/json"
	"net/http"

	"jokerarena-game-ms/internal/models"
)

// HandleRegisterPlayer processes POST /api/players with anti-XSS validation and idempotent upsert.
func (h *Handler) HandleRegisterPlayer(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	var req models.RegisterPlayerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	cleanAlias, err := models.ValidateAlias(req.Alias)
	if err != nil {
		WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	player, isNew, err := h.repo.RegisterPlayer(r.Context(), cleanAlias)
	if err != nil {
		WriteStorageError(w, err, http.StatusInternalServerError, "Failed to register player")
		return
	}

	status := http.StatusOK
	if isNew {
		status = http.StatusCreated
	}

	WriteJSON(w, status, player)
}
