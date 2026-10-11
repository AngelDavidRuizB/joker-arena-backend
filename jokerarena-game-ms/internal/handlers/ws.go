package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"jokerarena-game-ms/internal/db"
	"jokerarena-game-ms/internal/models"
	"jokerarena-game-ms/internal/room"
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
		CheckOrigin:     func(r *http.Request) bool { return originAllowed(r, h.allowedOrigins) },
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

// HandleWebSocket upgrades GET /ws/{code} to a WebSocket (connector C2) and binds the
// connection to the authoritative room session. The player is identified by a JOIN_ROOM
// message (or the optional player_id query parameter).
func (h *Handler) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	code, err := models.ValidateRoomCode(r.PathValue("code"))
	if err != nil {
		WriteError(w, http.StatusBadRequest, "A valid room code is required: /ws/{code}")
		return
	}

	conn, err := h.upgrader().Upgrade(w, r, nil)
	if err != nil {
		// Upgrade already wrote the HTTP error (bad handshake / origin not allowed).
		return
	}
	conn.SetReadLimit(maxMessageSize)

	playerID := strings.TrimSpace(r.URL.Query().Get("player_id"))
	if playerID == "" {
		_ = conn.SetReadDeadline(time.Now().Add(joinTimeout))
		var env room.Envelope
		if err := conn.ReadJSON(&env); err != nil || env.Type != room.MsgJoinRoom {
			writeDirect(conn, room.ErrCodeNotJoined, "send JOIN_ROOM with your player_id first")
			_ = conn.Close()
			return
		}
		var jp room.JoinPayload
		_ = json.Unmarshal(env.Payload, &jp)
		if jp.RoomCode != "" && !strings.EqualFold(jp.RoomCode, code) {
			writeDirect(conn, room.ErrCodeInvalidMessage, "room_code does not match the connection path")
			_ = conn.Close()
			return
		}
		playerID = strings.TrimSpace(jp.PlayerID)
	}

	player, errCode, errMsg := h.lookupPlayer(r, playerID)
	if player == nil {
		writeDirect(conn, errCode, errMsg)
		_ = conn.Close()
		return
	}

	session, errCode, errMsg := h.getOrRestoreRoom(r, code)
	if session == nil {
		writeDirect(conn, errCode, errMsg)
		_ = conn.Close()
		return
	}

	client := room.NewClient(player.ID)
	s := &wsSession{conn: conn, client: client}
	go s.writePump()
	if !session.Join(client, room.PlayerInfo{ID: player.ID, Alias: player.Alias}) {
		_ = conn.Close()
		return
	}
	defer session.Leave(client)

	h.readLoop(conn, session, client)
}

// lookupPlayer validates the player id against the repository.
func (h *Handler) lookupPlayer(r *http.Request, playerID string) (*models.Player, string, string) {
	if !models.IsValidPlayerID(playerID) {
		return nil, room.ErrCodeUnknownPlayer, "a valid player_id is required"
	}
	p, err := h.repo.GetPlayerByID(r.Context(), playerID)
	switch {
	case err == nil:
		return p, "", ""
	case errors.Is(err, db.ErrNotFound):
		return nil, room.ErrCodeUnknownPlayer, "unknown player_id"
	case errors.Is(err, db.ErrUnavailable):
		return nil, room.ErrCodeUnavailable, "database unavailable, try again shortly"
	default:
		log.Printf("[ERROR] ws player lookup: %v", err)
		return nil, room.ErrCodeUnavailable, "could not verify the player"
	}
}

// getOrRestoreRoom returns the live session for a code. If the service restarted and the room
// only exists in the database (and is not finished) a fresh game is dealt for the same match.
func (h *Handler) getOrRestoreRoom(r *http.Request, code string) (*room.Room, string, string) {
	if s, ok := h.roomMgr.Get(code); ok {
		return s, "", ""
	}
	match, _, err := h.repo.GetMatchByCode(r.Context(), code)
	switch {
	case err == nil:
	case errors.Is(err, db.ErrNotFound):
		return nil, room.ErrCodeRoomNotFound, "room not found"
	case errors.Is(err, db.ErrUnavailable):
		return nil, room.ErrCodeUnavailable, "database unavailable, try again shortly"
	default:
		log.Printf("[ERROR] ws room lookup: %v", err)
		return nil, room.ErrCodeUnavailable, "could not load the room"
	}
	if match.Status == models.StatusFinished {
		return nil, room.ErrCodeMatchFinished, "this match has already finished"
	}
	host, err := h.repo.GetPlayerByID(r.Context(), match.HostID)
	if err != nil {
		return nil, room.ErrCodeUnavailable, "could not load the room host"
	}
	s := h.roomMgr.Create(room.Options{
		MatchID:     match.ID,
		Code:        match.RoomCode,
		TargetScore: match.TargetScore,
		WithBot:     match.WithBot,
		Host:        room.PlayerInfo{ID: host.ID, Alias: host.Alias},
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
