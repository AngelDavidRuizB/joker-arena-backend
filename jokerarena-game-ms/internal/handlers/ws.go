package handlers

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"jokerarena-game-ms/internal/models"
	"jokerarena-game-ms/internal/room"
)

var wsGUID = []byte("258EAFA5-E914-47DA-95CA-C5AB0DC85B11")

type RoomStatePayload struct {
	RoomCode     string        `json:"room_code"`
	TargetScore  int           `json:"target_score"`
	HandsLeft    int           `json:"hands_left"`
	DiscardsLeft int           `json:"discards_left"`
	CurrentScore int           `json:"current_score"`
	DeckCount    int           `json:"deck_count"`
	ActiveJokers []string      `json:"active_jokers"`
	CardsInHand  []models.Card `json:"cards_in_hand"`
}

// HandleWebSocket manages RFC 6455 raw WebSocket upgrade and room messaging loop.
func (h *Handler) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Extract room code from path value or path split
	roomCode := r.PathValue("code")
	if roomCode == "" {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) >= 2 {
			roomCode = parts[1]
		}
	}
	roomCode = strings.ToUpper(strings.TrimSpace(roomCode))

	// Check WebSocket Upgrade header
	if strings.ToLower(r.Header.Get("Upgrade")) != "websocket" {
		http.Error(w, "Expected WebSocket Upgrade", http.StatusBadRequest)
		return
	}

	secKey := r.Header.Get("Sec-WebSocket-Key")
	if secKey == "" {
		http.Error(w, "Missing Sec-WebSocket-Key", http.StatusBadRequest)
		return
	}

	// Calculate Sec-WebSocket-Accept = base64(sha1(key + GUID))
	hash := sha1.New()
	hash.Write([]byte(secKey))
	hash.Write(wsGUID)
	acceptVal := base64.StdEncoding.EncodeToString(hash.Sum(nil))

	// Hijack connection
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "WebSocket hijacking not supported", http.StatusInternalServerError)
		return
	}

	conn, bufrw, err := hijacker.Hijack()
	if err != nil {
		log.Printf("Hijack error: %v", err)
		return
	}
	defer conn.Close()

	// Write 101 Switching Protocols
	resp := fmt.Sprintf("HTTP/1.1 101 Switching Protocols\r\n"+
		"Upgrade: websocket\r\n"+
		"Connection: Upgrade\r\n"+
		"Sec-WebSocket-Accept: %s\r\n\r\n", acceptVal)
	if _, err := bufrw.WriteString(resp); err != nil {
		return
	}
	_ = bufrw.Flush()

	// Retrieve or create room session
	session, found := h.roomMgr.GetRoom(roomCode)
	if !found {
		if roomCode == "" {
			roomCode = room.GenerateRoomCode()
		}
		session = h.roomMgr.CreateRoom(uuid.New().String(), roomCode, 300, true, "anonymous")
	}

	// Send initial ROOM_STATE_UPDATE
	details := session.ToDetailsResponse()
	initialState := map[string]interface{}{
		"type": "ROOM_STATE_UPDATE",
		"payload": RoomStatePayload{
			RoomCode:     details.RoomCode,
			TargetScore:  details.TargetScore,
			HandsLeft:    details.HandsLeft,
			DiscardsLeft: details.DiscardsLeft,
			CurrentScore: details.CurrentScore,
			DeckCount:    details.DeckCount,
			ActiveJokers: details.ActiveJokers,
			CardsInHand:  details.CardsInHand,
		},
	}
	initialBytes, _ := json.Marshal(initialState)
	_ = writeWSFrame(bufrw, 0x1, initialBytes)
	_ = bufrw.Flush()

	// Frame processing loop
	for {
		msgBytes, opcode, err := readWSFrame(bufrw)
		if err != nil {
			break // Client disconnected
		}

		if opcode == 0x8 { // Close
			_ = writeWSFrame(bufrw, 0x8, []byte{})
			_ = bufrw.Flush()
			break
		}

		if opcode == 0x9 { // Ping -> Pong
			_ = writeWSFrame(bufrw, 0xA, msgBytes)
			_ = bufrw.Flush()
			continue
		}

		if opcode == 0x1 { // Text
			var incoming struct {
				Type    string          `json:"type"`
				Payload json.RawMessage `json:"payload"`
			}
			if err := json.Unmarshal(msgBytes, &incoming); err == nil {
				curDetails := session.ToDetailsResponse()
				handsLeft := curDetails.HandsLeft
				discardsLeft := curDetails.DiscardsLeft
				currentScore := curDetails.CurrentScore

				switch incoming.Type {
				case "PLAY_HAND":
					if handsLeft > 0 {
						handsLeft--
						currentScore += 120
					}
				case "DISCARD_CARDS":
					if discardsLeft > 0 {
						discardsLeft--
					}
				}

				update := map[string]interface{}{
					"type": "ROOM_STATE_UPDATE",
					"payload": RoomStatePayload{
						RoomCode:     curDetails.RoomCode,
						TargetScore:  curDetails.TargetScore,
						HandsLeft:    handsLeft,
						DiscardsLeft: discardsLeft,
						CurrentScore: currentScore,
						DeckCount:    curDetails.DeckCount,
						ActiveJokers: curDetails.ActiveJokers,
						CardsInHand:  curDetails.CardsInHand,
					},
				}
				upBytes, _ := json.Marshal(update)
				_ = writeWSFrame(bufrw, 0x1, upBytes)
				_ = bufrw.Flush()
			}
		}
	}
}

func readWSFrame(r *bufio.ReadWriter) ([]byte, byte, error) {
	b1, err := r.ReadByte()
	if err != nil {
		return nil, 0, err
	}
	opcode := b1 & 0x0F

	b2, err := r.ReadByte()
	if err != nil {
		return nil, 0, err
	}
	isMasked := (b2 & 0x80) != 0
	payloadLen := int(b2 & 0x7F)

	if payloadLen == 126 {
		var l uint16
		if err := binary.Read(r, binary.BigEndian, &l); err != nil {
			return nil, 0, err
		}
		payloadLen = int(l)
	} else if payloadLen == 127 {
		var l uint64
		if err := binary.Read(r, binary.BigEndian, &l); err != nil {
			return nil, 0, err
		}
		payloadLen = int(l)
	}

	var mask [4]byte
	if isMasked {
		if _, err := io.ReadFull(r, mask[:]); err != nil {
			return nil, 0, err
		}
	}

	payload := make([]byte, payloadLen)
	if _, err := io.ReadFull(r, payload); err != nil {
		return nil, 0, err
	}

	if isMasked {
		for i := 0; i < payloadLen; i++ {
			payload[i] ^= mask[i%4]
		}
	}

	return payload, opcode, nil
}

func writeWSFrame(w *bufio.ReadWriter, opcode byte, payload []byte) error {
	if err := w.WriteByte(0x80 | opcode); err != nil {
		return err
	}

	length := len(payload)
	if length <= 125 {
		if err := w.WriteByte(byte(length)); err != nil {
			return err
		}
	} else if length <= 65535 {
		if err := w.WriteByte(126); err != nil {
			return err
		}
		if err := binary.Write(w, binary.BigEndian, uint16(length)); err != nil {
			return err
		}
	} else {
		if err := w.WriteByte(127); err != nil {
			return err
		}
		if err := binary.Write(w, binary.BigEndian, uint64(length)); err != nil {
			return err
		}
	}

	_, err := w.Write(payload)
	return err
}
