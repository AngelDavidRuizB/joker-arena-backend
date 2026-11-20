package handlers

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"game-engine-ms/internal/auth"
	"game-engine-ms/internal/db"
	"game-engine-ms/internal/lobby"
	"game-engine-ms/internal/poker"
	"game-engine-ms/internal/room"
)

type stubVerifier struct{}

func (stubVerifier) Verify(tok string) (*auth.Claims, error) {
	if tok == "good-ana" {
		return &auth.Claims{Subject: "player-ana", Alias: "Ana"}, nil
	}
	return nil, auth.ErrInvalidToken
}

type stubLobby struct{ rooms map[string]*lobby.Room }

func (s stubLobby) GetRoom(_ context.Context, code string) (*lobby.Room, error) {
	if r, ok := s.rooms[code]; ok {
		return r, nil
	}
	return nil, lobby.ErrNotFound
}

func newServer(t *testing.T) (*httptest.Server, *db.Memory) {
	t.Helper()
	repo := db.NewMemory()
	mgr := room.NewManager(repo)
	t.Cleanup(mgr.Close)
	lb := stubLobby{rooms: map[string]*lobby.Room{
		"JKR-AAAA": {RoomCode: "JKR-AAAA", Mode: "bot", Difficulty: "normal", TargetScore: 1, Status: "open", CreatedBy: "player-ana",
			Players: []lobby.Player{{PlayerID: "player-ana", Alias: "Ana"}}},
		"JKR-DONE": {RoomCode: "JKR-DONE", Mode: "bot", TargetScore: 100, Status: "closed", CreatedBy: "player-ana"},
	}}
	srv := httptest.NewServer(NewRouter(Deps{
		Repo: repo, Rooms: mgr, Verifier: stubVerifier{}, Lobby: lb, Rules: poker.DefaultRuleset,
	}))
	t.Cleanup(srv.Close)
	return srv, repo
}

func dial(t *testing.T, srv *httptest.Server, path string) *websocket.Conn {
	t.Helper()
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http")+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func read(t *testing.T, conn *websocket.Conn) room.Envelope {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	var env room.Envelope
	if err := conn.ReadJSON(&env); err != nil {
		t.Fatal(err)
	}
	return env
}

func TestWebSocketPlaysAFullMatch(t *testing.T) {
	srv, repo := newServer(t)
	conn := dial(t, srv, "/ws/jkr-aaaa?token=good-ana")
	env := read(t, conn)
	if env.Type != room.MsgRoomState {
		t.Fatalf("first frame must be the state, got %s", env.Type)
	}
	var st room.StatePayload
	_ = json.Unmarshal(env.Payload, &st)
	if st.HostID != "player-ana" || !st.WithBot || st.RoomCode != "JKR-AAAA" {
		t.Fatalf("state: %+v", st)
	}
	_ = conn.WriteJSON(map[string]any{"type": room.MsgPlayHand, "payload": map[string]any{"card_indices": []int{0}}})
	for {
		if read(t, conn).Type == room.MsgMatchDone {
			break
		}
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if row, err := repo.LatestMatch(context.Background(), "JKR-AAAA"); err == nil && row.Status == db.StatusFinished {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("the match was not persisted as FINISHED")
}

func TestWebSocketRejections(t *testing.T) {
	srv, _ := newServer(t)
	cases := map[string]string{
		"/ws/JKR-AAAA":                ErrCodeUnauthorized,
		"/ws/JKR-AAAA?token=bad":      ErrCodeUnauthorized,
		"/ws/JKR-NOPE?token=good-ana": room.ErrCodeRoomNotFound,
		"/ws/JKR-DONE?token=good-ana": room.ErrCodeMatchFinished,
	}
	for path, want := range cases {
		env := read(t, dial(t, srv, path))
		var e room.ErrorPayload
		_ = json.Unmarshal(env.Payload, &e)
		if env.Type != room.MsgError || e.Code != want {
			t.Errorf("%s: want %s, got %s %+v", path, want, env.Type, e)
		}
	}
}

func TestHealthReportsDependencies(t *testing.T) {
	srv, _ := newServer(t)
	resp, err := srv.Client().Get(srv.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if body["service"] != "game-engine-ms" || body["status"] != "ok" || body["ruleset_version"] != float64(1) {
		t.Fatalf("health: %+v", body)
	}
}
