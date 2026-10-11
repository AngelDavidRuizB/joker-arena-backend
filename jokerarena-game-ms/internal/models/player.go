package models

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

var aliasRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// Player represents a registered participant persisted in PostgreSQL.
type Player struct {
	ID        string    `json:"id"`
	Alias     string    `json:"alias"`
	TotalWins int       `json:"total_wins"`
	CreatedAt time.Time `json:"created_at"`
}

// RegisterPlayerRequest represents inbound POST /api/players payload.
type RegisterPlayerRequest struct {
	Alias string `json:"alias"`
}

// ValidateAlias enforces alias length and anti-XSS alphanumeric constraints.
func ValidateAlias(raw string) (string, error) {
	alias := strings.TrimSpace(raw)
	if len(alias) < 2 || len(alias) > 30 {
		return "", fmt.Errorf("alias must be between 2 and 30 characters")
	}
	if !aliasRegex.MatchString(alias) {
		return "", fmt.Errorf("alias contains invalid characters (alphanumeric, underscore, and hyphen only)")
	}
	return alias, nil
}
