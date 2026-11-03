package room

import (
	"encoding/json"
	"sync"
)

// sendBuffer is the per-client outbound queue size. A client that cannot keep up is dropped
// instead of blocking the room goroutine.
const sendBuffer = 64

// Client is a connected WebSocket peer as seen by the room. Only the room goroutine writes
// to Send and closes it; the connection's write pump drains it.
type Client struct {
	PlayerID string
	Send     chan []byte

	closeOnce sync.Once
}

// NewClient creates a client for the given player.
func NewClient(playerID string) *Client {
	return &Client{PlayerID: playerID, Send: make(chan []byte, sendBuffer)}
}

// enqueue queues a frame without blocking; it reports false when the buffer is full.
func (c *Client) enqueue(frame []byte) bool {
	select {
	case c.Send <- frame:
		return true
	default:
		return false
	}
}

// Close closes the send channel exactly once. It must only be called by the room goroutine
// for registered clients, or by the connection owner for clients that never joined.
func (c *Client) Close() {
	c.closeOnce.Do(func() { close(c.Send) })
}

func marshal(msgType string, payload any) []byte {
	b, err := json.Marshal(outbound{Type: msgType, Payload: payload})
	if err != nil {
		// Payloads are plain structs; a failure here is a programming error.
		b, _ = json.Marshal(outbound{Type: MsgError, Payload: ErrorPayload{Code: "INTERNAL", Message: "serialization error"}})
	}
	return b
}
