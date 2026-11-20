package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"game-engine-ms/internal/auth"
	"game-engine-ms/internal/db"
	"game-engine-ms/internal/events"
	"game-engine-ms/internal/lobby"
	"game-engine-ms/internal/models"
	"game-engine-ms/internal/room"
)

const (
	maxMessageSize    = 4096
	writeWait         = 10 * time.Second
	pongWait          = 60 * time.Second
	pingPeriod        = 25 * time.Second
	joinTimeout       = 10 * time.Second
	maxMessagesPerSec = 30
)

func (h *Handler) upgrader() *websocket.Upgrader {
	return &websocket.Upgrader{
		ReadBufferSize:  2048,
		WriteBufferSize: 2048,
		CheckOrigin:     func(r *http.Request) bool { return originAllowed(r, h.d.AllowedOrigins) },
	}
}

// wsConn serializes writes on a gorilla connection (it supports one concurrent writer).
type wsSession struct {
	conn   *websocket.Conn
	client *room.Client
}

// writePump drains the client's send queue to the socket and sends periodic pings.
func (s *wsSession) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = s.conn.Close()
	}()
	for {
		select {
		case frame, ok := <-s.client.Send:
			_ = s.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok { // room closed the client
				_ = s.conn.WriteMessage(websocket.CloseMessage,
					websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
				return
			}
			if err := s.conn.WriteMessage(websocket.TextMessage, frame); err != nil {
				return
			}
		case <-ticker.C:
			_ = s.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := s.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// writeDirect sends one message before the write pump exists (join failures).
func writeDirect(conn *websocket.Conn, code, message string) {
	b, _ := json.Marshal(map[string]any{"type": room.MsgError, "payload": room.ErrorPayload{Code: code, Message: message}})
	_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
	_ = conn.WriteMessage(websocket.TextMessage, b)
	_ = conn.WriteMessage(websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.ClosePolicyViolation, code))
}

// HandleWebSocket upgrades GET /ws/{code}?token=<jwt> to a WebSocket and binds it to the
// authoritative room session. The player comes from the JWT (auth-identity-ms) and the room
// from rooms-lobby-ms. The first connection creates the session for the room.
func (h *Handler) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	code, err := models.ValidateRoomCode(r.PathValue("code"))
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_room_code", "A valid room code is required: /ws/{code}")
		return
	}
	token := strings.TrimSpace(r.URL.Query().Get("token"))
	if token == "" {
		token = strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
	}

	conn, err := h.upgrader().Upgrade(w, r, nil)
	if err != nil {
		return // Upgrade already wrote the HTTP error
	}
	conn.SetReadLimit(maxMessageSize)

	claims, err := h.d.Verifier.Verify(token)
	if err != nil {
		code := room.ErrCodeUnknownPlayer
		if errors.Is(err, auth.ErrInvalidToken) || token == "" {
			code = ErrCodeUnauthorized
		}
		writeDirect(conn, code, "a valid session token is required (?token=...)")
		_ = conn.Close()
		return
	}
	player := room.PlayerInfo{ID: claims.Subject, Alias: claims.Alias}

	session, errCode, errMsg := h.getOrCreateRoom(r.Context(), code, player)
	if session == nil {
		writeDirect(conn, errCode, errMsg)
		_ = conn.Close()
		return
	}

	client := room.NewClient(player.ID)
	s := &wsSession{conn: conn, client: client}
	go s.writePump()
	if !session.Join(client, player) {
		_ = conn.Close()
		return
	}
	defer session.Leave(client)

	h.readLoop(conn, session, client)
}

// ErrCodeUnauthorized is sent when the token is missing or invalid.
const ErrCodeUnauthorized = "UNAUTHORIZED"

// getOrCreateRoom returns the live session for a code or creates it from the lobby's data.
func (h *Handler) getOrCreateRoom(ctx context.Context, code string, player room.PlayerInfo) (*room.Room, string, string) {
	if s, ok := h.d.Rooms.Get(code); ok {
		return s, "", ""
	}
	lctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	info, err := h.d.Lobby.GetRoom(lctx, code)
	switch {
	case err == nil:
	case errors.Is(err, lobby.ErrNotFound):
		return nil, room.ErrCodeRoomNotFound, "room not found"
	default:
		log.Printf("[WARN] lobby lookup for %s failed: %v", code, err)
		return nil, room.ErrCodeUnavailable, "rooms service unavailable, try again shortly"
	}
	if info.Status == "closed" {
		return nil, room.ErrCodeMatchFinished, "this match has already finished"
	}

	// After an engine restart continue the match (scores and counters); otherwise start a new one.
	matchID := events.NewEventID()
	var resume *room.MatchProgress
	if row, err := h.d.Repo.LatestMatch(lctx, code); err == nil {
		if row.Status == db.StatusFinished {
			return nil, room.ErrCodeMatchFinished, "this match has already finished"
		}
		matchID, resume = row.MatchID, row.Progress
	}

	host := room.PlayerInfo{ID: info.CreatedBy, Alias: info.HostAlias()}
	if host.ID == "" || host.ID == player.ID {
		host = player
	}
	s := h.d.Rooms.Create(room.Options{
		MatchID:     matchID,
		Code:        info.RoomCode,
		TargetScore: info.TargetScore,
		WithBot:     info.Mode != "pvp",
		Difficulty:  info.Difficulty,
		Host:        host,
		Resume:      resume,
	})
	return s, "", ""
}

// readLoop reads client messages until the connection closes.
func (h *Handler) readLoop(conn *websocket.Conn, session *room.Room, client *room.Client) {
	_ = conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	windowStart, count := time.Now(), 0
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			return // closed, timed out or protocol error
		}
		_ = conn.SetReadDeadline(time.Now().Add(pongWait))

		if now := time.Now(); now.Sub(windowStart) >= time.Second {
			windowStart, count = now, 0
		}
		if count++; count > maxMessagesPerSec {
			_ = conn.WriteControl(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "rate limit"),
				time.Now().Add(writeWait))
			return
		}

		var env room.Envelope
		if err := json.Unmarshal(data, &env); err != nil {
			sendClientError(client, session, room.ErrCodeInvalidMessage, "malformed JSON message")
			continue
		}
		switch env.Type {
		case room.MsgJoinRoom:
			session.Resend(client)
		case room.MsgPlayHand, room.MsgDiscard:
			var sel room.SelectionPayload
			if err := json.Unmarshal(env.Payload, &sel); err != nil {
				sendClientError(client, session, room.ErrCodeInvalidMessage, "payload must be {\"card_indices\": [...]}")
				continue
			}
			if env.Type == room.MsgPlayHand {
				session.Play(client, sel.CardIndices)
			} else {
				session.Discard(client, sel.CardIndices)
			}
		default:
			sendClientError(client, session, room.ErrCodeInvalidMessage, "unknown message type: "+env.Type)
		}
	}
}

// sendClientError reports a protocol error to one client through the room goroutine's queue
// owner rules: the room is the only writer of client.Send, so errors go through Resend-like
// commands. Protocol errors are rare; we serialize them via a dedicated command.
func sendClientError(client *room.Client, session *room.Room, code, msg string) {
	session.Reject(client, code, msg)
}
