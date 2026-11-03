package handlers

import (
	"encoding/json"
	"net/http"
)

// WriteJSON writes status code, JSON Content-Type header, and marshaled payload.
func WriteJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

// WriteError writes the common error shape {"error": code, "message": text}.
func WriteError(w http.ResponseWriter, status int, code, message string) {
	WriteJSON(w, status, map[string]string{"error": code, "message": message})
}
