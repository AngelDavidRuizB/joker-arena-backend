package handlers

import (
	"net/http"

	"jokerarena-game-ms/internal/db"
	"jokerarena-game-ms/internal/room"
)

// Handler holds dependencies for all Game Engine endpoints.
type Handler struct {
	repo           db.Repository
	roomMgr        *room.Manager
	allowedOrigins []string
}

// NewHandler creates a new Handler instance.
func NewHandler(repo db.Repository, roomMgr *room.Manager) *Handler {
	return &Handler{
		repo:    repo,
		roomMgr: roomMgr,
	}
}

// NewRouter constructs an http.Handler with Go 1.22 ServeMux and CORS middleware.
func NewRouter(h *Handler, allowedOrigins []string) http.Handler {
	h.allowedOrigins = allowedOrigins
	mux := http.NewServeMux()

	// Health check
	mux.HandleFunc("GET /health", h.HandleHealth)

	// REST API Endpoints (Contract C1)
	mux.HandleFunc("POST /api/players", h.HandleRegisterPlayer)
	mux.HandleFunc("POST /api/rooms", h.HandleCreateRoom)
	mux.HandleFunc("GET /api/rooms/{code}", h.HandleGetRoom)

	// Boundary routes for empty room code
	mux.HandleFunc("GET /api/rooms", func(w http.ResponseWriter, r *http.Request) {
		WriteError(w, http.StatusNotFound, "Room code required")
	})
	mux.HandleFunc("GET /api/rooms/", func(w http.ResponseWriter, r *http.Request) {
		WriteError(w, http.StatusNotFound, "Room code required")
	})

	// WebSocket Routes (Contract C2)
	mux.HandleFunc("/ws", h.HandleWebSocket)
	mux.HandleFunc("/ws/{code}", h.HandleWebSocket)

	return CORSMiddleware(mux, allowedOrigins)
}
