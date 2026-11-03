// Package botclient calls bot-ai-ms (POST /bot/decide).
package botclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"game-engine-ms/internal/room"
)

// Client is a room.BotDecider backed by bot-ai-ms.
type Client struct {
	url  string
	http *http.Client
}

// New creates a client for the given base URL.
func New(base string) *Client {
	return &Client{url: strings.TrimRight(base, "/") + "/bot/decide", http: &http.Client{}}
}

type card struct {
	Rank string `json:"rank"`
	Suit string `json:"suit"`
}

type request struct {
	Hand         []card `json:"hand"`
	HandsLeft    int    `json:"hands_left"`
	DiscardsLeft int    `json:"discards_left"`
	TargetScore  int    `json:"target_score"`
	CurrentScore int    `json:"current_score"`
	Difficulty   string `json:"difficulty"`
	Ruleset      any    `json:"ruleset"`
}

type response struct {
	Action      string `json:"action"`
	CardIndices []int  `json:"card_indices"`
}

// Decide implements room.BotDecider. The caller sets the timeout through ctx.
func (c *Client) Decide(ctx context.Context, req room.BotRequest) (string, []int, error) {
	body := request{
		HandsLeft: req.HandsLeft, DiscardsLeft: req.DiscardsLeft, TargetScore: req.TargetScore,
		CurrentScore: req.CurrentScore, Difficulty: req.Difficulty, Ruleset: req.Ruleset,
	}
	if body.Difficulty == "" {
		body.Difficulty = "normal"
	}
	for _, cd := range req.Hand {
		body.Hand = append(body.Hand, card{Rank: cd.Rank, Suit: cd.Suit})
	}
	raw, _ := json.Marshal(body)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(raw))
	if err != nil {
		return "", nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(httpReq)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", nil, fmt.Errorf("bot-ai-ms answered %d", resp.StatusCode)
	}
	var out response
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", nil, fmt.Errorf("invalid bot-ai response: %w", err)
	}
	return out.Action, out.CardIndices, nil
}
