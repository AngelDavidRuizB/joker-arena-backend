package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"jokerarena-game-ms/internal/db"
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

// WriteStorageError maps repository errors to HTTP responses. Storage outages are reported
// honestly as 503 instead of pretending the operation succeeded.
func WriteStorageError(w http.ResponseWriter, err error, fallbackStatus int, message string) {
	switch {
	case errors.Is(err, db.ErrUnavailable):
		WriteError(w, http.StatusServiceUnavailable, "Database unavailable, try again shortly")
	case errors.Is(err, db.ErrNotFound):
		WriteError(w, http.StatusNotFound, message)
	case errors.Is(err, db.ErrInvalidID):
		WriteError(w, http.StatusBadRequest, "Invalid identifier")
	default:
		WriteError(w, fallbackStatus, message)
	}
}
