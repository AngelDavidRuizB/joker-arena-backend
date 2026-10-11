package handlers

import (
	"context"
	"net/http"
	"time"

	"jokerarena-game-ms/internal/models"
)

// HandleHealth checks database health and returns service status.
func (h *Handler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 1*time.Second)
	defer cancel()

	pgStatus := "connected"
	if err := h.repo.Ping(ctx); err != nil {
		pgStatus = "fallback"
	}

	WriteJSON(w, http.StatusOK, models.HealthResponse{
		Status:   "ok",
		Service:  "jokerarena-game-ms",
		Postgres: pgStatus,
	})
}
