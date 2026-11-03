// Package lobby queries rooms-lobby-ms for the room a WebSocket wants to join.
package lobby

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ErrNotFound means the lobby does not know the room.
var ErrNotFound = errors.New("room not found")

// Player is a participant registered in the lobby.
type Player struct {
	PlayerID string `json:"player_id"`
	Alias    string `json:"alias"`
}

// Room is the lobby's view of a room (GET /rooms/{code}).
type Room struct {
	RoomCode    string   `json:"room_code"`
	Mode        string   `json:"mode"` // bot | pvp
	Difficulty  string   `json:"difficulty"`
	TargetScore int      `json:"target_score"`
	Status      string   `json:"status"` // open | in_progress | closed
	CreatedBy   string   `json:"created_by"`
	Players     []Player `json:"players"`
}

// HostAlias returns the alias of the creator, if listed.
func (r Room) HostAlias() string {
	for _, p := range r.Players {
		if p.PlayerID == r.CreatedBy {
			return p.Alias
		}
	}
	return ""
}

// Client calls rooms-lobby-ms.
type Client struct {
	base string
	http *http.Client
}

// NewClient creates a client for the given base URL.
func NewClient(base string) *Client {
	return &Client{base: strings.TrimRight(base, "/"), http: &http.Client{Timeout: 3 * time.Second}}
}

// GetRoom fetches GET /rooms/{code}.
func (c *Client) GetRoom(ctx context.Context, code string) (*Room, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/rooms/"+url.PathEscape(code), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("rooms-lobby-ms unavailable: %w", err)
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return nil, ErrNotFound
	default:
		return nil, fmt.Errorf("rooms-lobby-ms answered %d", resp.StatusCode)
	}
	var r Room
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, fmt.Errorf("invalid lobby response: %w", err)
	}
	return &r, nil
}
