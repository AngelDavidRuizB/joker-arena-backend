package handlers

import (
	"context"
	"net/http"
	"time"
)

// HandleHealth reports the engine and its dependencies. It always answers 200: the engine
// keeps serving games (degraded) when the database or the broker are down.
func (h *Handler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), time.Second)
	defer cancel()
	deps := map[string]string{"postgres": "ok", "rabbitmq": "ok"}
	if h.d.Repo.Mode() == "memory" {
		deps["postgres"] = "not-used"
	} else if err := h.d.Repo.Ping(ctx); err != nil {
		deps["postgres"] = "down"
	}
	if h.d.BrokerUp == nil {
		deps["rabbitmq"] = "not-used"
	} else if !h.d.BrokerUp() {
		deps["rabbitmq"] = "down"
	}
	status := "ok"
	for _, v := range deps {
		if v == "down" {
			status = "degraded"
		}
	}
	WriteJSON(w, http.StatusOK, map[string]any{
		"status":          status,
		"service":         "game-engine-ms",
		"deps":            deps,
		"ruleset_version": h.d.Rules().Version,
	})
}
