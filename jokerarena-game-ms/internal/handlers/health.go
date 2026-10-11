package handlers

import (
	"context"
	"net/http"
	"time"

	"jokerarena-game-ms/internal/models"
)

// HandleHealth reports service and storage health. When PostgreSQL is unreachable the
// endpoint answers 503 (status "degraded") so orchestrators and users see the real state.
func (h *Handler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 1*time.Second)
	defer cancel()

	resp := models.HealthResponse{
		Status:   "ok",
		Service:  "jokerarena-game-ms",
		Postgres: "connected",
		Storage:  h.repo.Mode(),
	}
	code := http.StatusOK
	if err := h.repo.Ping(ctx); err != nil {
		resp.Status = "degraded"
		resp.Postgres = "unavailable"
		code = http.StatusServiceUnavailable
	}
	if h.repo.Mode() == "memory" {
		resp.Postgres = "not-used"
	}
	WriteJSON(w, code, resp)
}
