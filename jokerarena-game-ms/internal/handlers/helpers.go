package handlers

import (
	"encoding/json"
	"net/http"

	"jokerarena-game-ms/internal/models"
)

// WriteJSON writes status code, JSON Content-Type header, and marshaled payload.
func WriteJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

// WriteError writes a standardized JSON error response.
func WriteError(w http.ResponseWriter, status int, message string) {
	WriteJSON(w, status, models.ErrorResponse{Error: message})
}
