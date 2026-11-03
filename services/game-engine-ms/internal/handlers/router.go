package handlers

import (
	"context"
	"net/http"

	"game-engine-ms/internal/auth"
	"game-engine-ms/internal/db"
	"game-engine-ms/internal/lobby"
	"game-engine-ms/internal/poker"
	"game-engine-ms/internal/room"
)

// TokenVerifier validates the JWT sent by clients on the WebSocket.
type TokenVerifier interface {
	Verify(token string) (*auth.Claims, error)
}

// RoomLookup resolves a room code against rooms-lobby-ms.
type RoomLookup interface {
	GetRoom(ctx context.Context, code string) (*lobby.Room, error)
}

// Deps groups what the handlers need.
type Deps struct {
	Repo           db.Repository
	Rooms          *room.Manager
	Verifier       TokenVerifier
	Lobby          RoomLookup
	Rules          func() poker.Ruleset
	RulesSource    func() string
	BrokerUp       func() bool
	AllowedOrigins []string
}

// Handler holds dependencies for all Game Engine endpoints.
type Handler struct{ d Deps }

// NewRouter builds the HTTP router: /health, the WebSocket and a read-only ruleset view.
// Players and rooms are no longer created here (auth-identity-ms and rooms-lobby-ms).
func NewRouter(d Deps) http.Handler {
	h := &Handler{d: d}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", h.HandleHealth)
	mux.HandleFunc("GET /ruleset", h.HandleRuleset)
	mux.HandleFunc("/ws/{code}", h.HandleWebSocket)
	return CORSMiddleware(mux, d.AllowedOrigins)
}

// HandleRuleset shows the ruleset the engine is applying right now (useful for the demo).
func (h *Handler) HandleRuleset(w http.ResponseWriter, _ *http.Request) {
	source := "default"
	if h.d.RulesSource != nil {
		source = h.d.RulesSource()
	}
	WriteJSON(w, http.StatusOK, map[string]any{"source": source, "ruleset": h.d.Rules()})
}
