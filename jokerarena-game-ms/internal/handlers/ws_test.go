package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"jokerarena-game-ms/internal/db"
	"jokerarena-game-ms/internal/room"
)

type wsEnv struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

type wsFixture struct {
	srv    *httptest.Server
	repo   *db.MemoryRepository
	wsBase string
}

func newWSFixture(t *testing.T) *wsFixture {
	t.Helper()
	repo := db.NewMemoryRepository()
	mgr := room.NewManager(repo)
	h := NewHandler(repo, mgr)
	srv := httptest.NewServer(NewRouter(h, nil))
	t.Cleanup(func() { srv.Close(); mgr.Close() })
	return &wsFixture{srv: srv, repo: repo, wsBase: "ws" + strings.TrimPrefix(srv.URL, "http")}
}

func (f *wsFixture) createRoom(t *testing.T, alias string, target int, withBot bool) (playerID, code string) {
	t.Helper()
	_, player := postJSON(t, f.srv.URL+"/api/players", map[string]string{"alias": alias})
	playerID, _ = player["id"].(string)
	resp, body := postJSON(t, f.srv.URL+"/api/rooms", map[string]any{"player_id": playerID, "target_score": target, "with_bot": withBot})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create room: %d %v", resp.StatusCode, body)
	}
	code, _ = body["room_code"].(string)
	return playerID, code
}

func dial(t *testing.T, url string, header http.Header) *websocket.Conn {
	t.Helper()
	conn, resp, err := websocket.DefaultDialer.Dial(url, header)
	if err != nil {
		status := 0
		if resp != nil {
			status = resp.StatusCode
		}
		t.Fatalf("dial %s: %v (status %d)", url, err, status)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func readEnv(t *testing.T, conn *websocket.Conn) wsEnv {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	var env wsEnv
	if err := conn.ReadJSON(&env); err != nil {
		t.Fatalf("read: %v", err)
	}
	return env
}

func expectType(t *testing.T, conn *websocket.Conn, want string) json.RawMessage {
	t.Helper()
	env := readEnv(t, conn)
	if env.Type != want {
		t.Fatalf("got %s (%s), want %s", env.Type, env.Payload, want)
	}
	return env.Payload
}

func sendMsg(t *testing.T, conn *websocket.Conn, typ string, payload any) {
	t.Helper()
	if err := conn.WriteJSON(map[string]any{"type": typ, "payload": payload}); err != nil {
		t.Fatal(err)
	}
}

func TestWebSocketFullMatchFlow(t *testing.T) {
	f := newWSFixture(t)
	playerID, code := f.createRoom(t, "WsHero", 1, true) // target 1 => first hand wins

	conn := dial(t, f.wsBase+"/ws/"+code, nil)
	sendMsg(t, conn, "JOIN_ROOM", map[string]string{"room_code": code, "player_id": playerID})

	var st room.StatePayload
	_ = json.Unmarshal(expectType(t, conn, "ROOM_STATE_UPDATE"), &st)
	if len(st.CardsInHand) != 8 || st.DeckCount != 44 || st.HandsLeft != room.InitialHands || st.TargetScore != 1 || st.RoomCode != code {
		t.Fatalf("unexpected initial state %+v", st)
	}

	sendMsg(t, conn, "PLAY_HAND", map[string]any{"card_indices": []int{0, 1, 2}})
	var scored room.HandScoredPayload
	_ = json.Unmarshal(expectType(t, conn, "HAND_SCORED"), &scored)
	if scored.Score <= 0 || scored.AccumulatedScore != scored.Score || len(scored.Jokers) != 3 || len(scored.Cards) != 3 {
		t.Fatalf("unexpected HAND_SCORED %+v", scored)
	}
	var after room.StatePayload
	_ = json.Unmarshal(expectType(t, conn, "ROOM_STATE_UPDATE"), &after)
	if after.CurrentScore != scored.Score || after.HandsLeft != room.InitialHands-1 || len(after.CardsInHand) != 8 {
		t.Fatalf("unexpected state %+v", after)
	}
	var fin room.MatchFinishedPayload
	_ = json.Unmarshal(expectType(t, conn, "MATCH_FINISHED"), &fin)
	if fin.Result != "WIN" || fin.WinnerID != playerID || !fin.TargetReached {
		t.Fatalf("unexpected MATCH_FINISHED %+v", fin)
	}

	// The result is persisted through the repository.
	deadline := time.Now().Add(2 * time.Second)
	for {
		m, _, err := f.repo.GetMatchByCode(context.Background(), code)
		if err == nil && m.Status == "FINISHED" && m.Result == "WIN" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("match result was not persisted: %+v %v", m, err)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestWebSocketBotPlaysAfterHostPlay(t *testing.T) {
	f := newWSFixture(t)
	// High target so neither side finishes; the room seed is random, so the bot may DISCARD
	// (max 3 times) before it PLAYs. We issue host PLAYs until we observe the bot's
	// HAND_SCORED, asserting the per-round frame order every round.
	playerID, code := f.createRoom(t, "BotPlays", 100000, true)
	conn := dial(t, f.wsBase+"/ws/"+code, nil)
	sendMsg(t, conn, "JOIN_ROOM", map[string]string{"room_code": code, "player_id": playerID})

	var st room.StatePayload
	_ = json.Unmarshal(expectType(t, conn, "ROOM_STATE_UPDATE"), &st)
	if !st.WithBot || st.BotHandsLeft != room.InitialHands || st.BotDiscardsLeft != room.InitialDiscards {
		t.Fatalf("initial bot state %+v", st)
	}

	botPlayed := false
	for round := 0; round < room.InitialDiscards+2 && !botPlayed; round++ {
		sendMsg(t, conn, "PLAY_HAND", map[string]any{"card_indices": []int{0}})

		// Round order: host HAND_SCORED, host STATE, then bot HAND_SCORED+STATE (PLAY) or
		// just STATE (DISCARD).
		var hostScored room.HandScoredPayload
		_ = json.Unmarshal(expectType(t, conn, "HAND_SCORED"), &hostScored)
		if hostScored.PlayerID != playerID {
			t.Fatalf("first HAND_SCORED must be the host, got %q", hostScored.PlayerID)
		}
		var hostState room.StatePayload
		_ = json.Unmarshal(expectType(t, conn, "ROOM_STATE_UPDATE"), &hostState)

		// Peek the next frame: HAND_SCORED(bot) means the bot played; STATE means it discarded.
		env := readEnv(t, conn)
		switch env.Type {
		case "HAND_SCORED":
			var botScored room.HandScoredPayload
			_ = json.Unmarshal(env.Payload, &botScored)
			if botScored.PlayerID != room.BotPlayerID {
				t.Fatalf("bot HAND_SCORED player_id = %q, want %q", botScored.PlayerID, room.BotPlayerID)
			}
			if botScored.AccumulatedScore <= 0 || botScored.HandNumber < 1 {
				t.Fatalf("bot HAND_SCORED payload %+v", botScored)
			}
			var botState room.StatePayload
			_ = json.Unmarshal(expectType(t, conn, "ROOM_STATE_UPDATE"), &botState)
			if botState.BotScore != botScored.AccumulatedScore || botState.BotHandNumber != botScored.HandNumber {
				t.Fatalf("bot STATE does not match bot HAND_SCORED: state %+v scored %+v", botState, botScored)
			}
			botPlayed = true
		case "ROOM_STATE_UPDATE":
			var botState room.StatePayload
			_ = json.Unmarshal(env.Payload, &botState)
			// A bot discard: discards decremented, no hand consumed, no bot HAND_SCORED.
			if botState.BotDiscardsLeft >= room.InitialDiscards {
				t.Fatalf("expected bot discards to decrease on a discard round, got %+v", botState)
			}
		default:
			t.Fatalf("unexpected frame after host turn: %s", env.Type)
		}
	}

	if !botPlayed {
		t.Fatal("the bot never produced a HAND_SCORED within the expected number of rounds")
	}
}

func TestWebSocketAutoJoinWithQueryParam(t *testing.T) {
	f := newWSFixture(t)
	playerID, code := f.createRoom(t, "QueryJoin", 300, false)
	conn := dial(t, f.wsBase+"/ws/"+strings.ToLower(code)+"?player_id="+playerID, nil)
	var st room.StatePayload
	_ = json.Unmarshal(expectType(t, conn, "ROOM_STATE_UPDATE"), &st)
	if len(st.CardsInHand) != 8 || st.WithBot {
		t.Fatalf("unexpected state %+v", st)
	}
}

func TestWebSocketRejectsUnknownRoomAndPlayer(t *testing.T) {
	f := newWSFixture(t)
	playerID, code := f.createRoom(t, "Rejects", 300, false)

	conn := dial(t, f.wsBase+"/ws/JKR-NOPE", nil)
	sendMsg(t, conn, "JOIN_ROOM", map[string]string{"player_id": playerID})
	var e room.ErrorPayload
	_ = json.Unmarshal(expectType(t, conn, "ERROR"), &e)
	if e.Code != room.ErrCodeRoomNotFound {
		t.Fatalf("code %s", e.Code)
	}

	conn2 := dial(t, f.wsBase+"/ws/"+code, nil)
	sendMsg(t, conn2, "JOIN_ROOM", map[string]string{"player_id": "424242"})
	_ = json.Unmarshal(expectType(t, conn2, "ERROR"), &e)
	if e.Code != room.ErrCodeUnknownPlayer {
		t.Fatalf("code %s", e.Code)
	}

	conn3 := dial(t, f.wsBase+"/ws/"+code, nil)
	sendMsg(t, conn3, "PLAY_HAND", map[string]any{"card_indices": []int{0}}) // not joined
	_ = json.Unmarshal(expectType(t, conn3, "ERROR"), &e)
	if e.Code != room.ErrCodeNotJoined {
		t.Fatalf("code %s", e.Code)
	}
}

func TestWebSocketInvalidRoomCodeIsHTTP400(t *testing.T) {
	f := newWSFixture(t)
	resp, err := http.Get(f.srv.URL + "/ws/x")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

func TestWebSocketProtocolErrors(t *testing.T) {
	f := newWSFixture(t)
	playerID, code := f.createRoom(t, "Protocol", 100000, false)
	conn := dial(t, f.wsBase+"/ws/"+code+"?player_id="+playerID, nil)
	expectType(t, conn, "ROOM_STATE_UPDATE")

	_ = conn.WriteMessage(websocket.TextMessage, []byte("{not json"))
	var e room.ErrorPayload
	_ = json.Unmarshal(expectType(t, conn, "ERROR"), &e)
	if e.Code != room.ErrCodeInvalidMessage {
		t.Fatalf("code %s", e.Code)
	}

	sendMsg(t, conn, "FLY_TO_MOON", nil)
	_ = json.Unmarshal(expectType(t, conn, "ERROR"), &e)
	if e.Code != room.ErrCodeInvalidMessage {
		t.Fatalf("code %s", e.Code)
	}

	sendMsg(t, conn, "PLAY_HAND", map[string]any{"card_indices": []int{0, 0}})
	_ = json.Unmarshal(expectType(t, conn, "ERROR"), &e)
	if e.Code != room.ErrCodeInvalidSelection {
		t.Fatalf("code %s", e.Code)
	}

	// Re-sending JOIN_ROOM is idempotent and replays the current state.
	sendMsg(t, conn, "JOIN_ROOM", map[string]string{"player_id": playerID})
	expectType(t, conn, "ROOM_STATE_UPDATE")
}

func TestWebSocketRejectsForeignOrigin(t *testing.T) {
	f := newWSFixture(t)
	playerID, code := f.createRoom(t, "OriginTest", 300, false)
	hdr := http.Header{"Origin": []string{"http://evil.example"}}
	_, resp, err := websocket.DefaultDialer.Dial(f.wsBase+"/ws/"+code+"?player_id="+playerID, hdr)
	if err == nil {
		t.Fatal("handshake from a foreign origin must fail")
	}
	if resp == nil || resp.StatusCode != http.StatusForbidden {
		t.Fatalf("want 403, got %v", resp)
	}
}

func TestWebSocketGameStateSurvivesReconnect(t *testing.T) {
	f := newWSFixture(t)
	playerID, code := f.createRoom(t, "Reconnect", 100000, false)

	c1 := dial(t, f.wsBase+"/ws/"+code+"?player_id="+playerID, nil)
	expectType(t, c1, "ROOM_STATE_UPDATE")
	sendMsg(t, c1, "DISCARD_CARDS", map[string]any{"card_indices": []int{0, 1}})
	var st room.StatePayload
	_ = json.Unmarshal(expectType(t, c1, "ROOM_STATE_UPDATE"), &st)
	_ = c1.Close()

	c2 := dial(t, f.wsBase+"/ws/"+code+"?player_id="+playerID, nil)
	var again room.StatePayload
	_ = json.Unmarshal(expectType(t, c2, "ROOM_STATE_UPDATE"), &again)
	if again.DiscardsLeft != 2 || len(again.CardsInHand) != 8 || again.DeckCount != st.DeckCount {
		t.Fatalf("state was not preserved across reconnect: %+v vs %+v", again, st)
	}
	for i := range st.CardsInHand {
		if st.CardsInHand[i] != again.CardsInHand[i] {
			t.Fatal("cards changed across reconnect")
		}
	}
}

func TestWebSocketRestoresRoomAfterRestart(t *testing.T) {
	f := newWSFixture(t)
	playerID, code := f.createRoom(t, "Restart", 300, true)

	// A fresh manager simulates a service restart: the room only exists in the repository.
	mgr2 := room.NewManager(f.repo)
	defer mgr2.Close()
	srv2 := httptest.NewServer(NewRouter(NewHandler(f.repo, mgr2), nil))
	defer srv2.Close()
	ws2 := "ws" + strings.TrimPrefix(srv2.URL, "http")

	conn := dial(t, ws2+"/ws/"+code+"?player_id="+playerID, nil)
	var st room.StatePayload
	_ = json.Unmarshal(expectType(t, conn, "ROOM_STATE_UPDATE"), &st)
	if len(st.CardsInHand) != 8 || st.RoomCode != code || !st.WithBot || st.HostID != playerID {
		t.Fatalf("restored state %+v", st)
	}
}
