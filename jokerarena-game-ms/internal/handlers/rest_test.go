package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"jokerarena-game-ms/internal/db"
	"jokerarena-game-ms/internal/models"
	"jokerarena-game-ms/internal/room"
)

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	h := NewHandler(db.NewMemoryRepository(), room.NewManager(nil))
	srv := httptest.NewServer(NewRouter(h, nil))
	t.Cleanup(srv.Close)
	return srv
}

func postJSON(t *testing.T, url string, body any) (*http.Response, map[string]any) {
	t.Helper()
	b, _ := json.Marshal(body)
	resp, err := http.Post(url, "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp, out
}

func TestRegisterAndCreateRoomFlow(t *testing.T) {
	srv := newTestServer(t)

	resp, player := postJSON(t, srv.URL+"/api/players", map[string]string{"alias": "JokerPro99"})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("register: status %d body %v", resp.StatusCode, player)
	}
	id, _ := player["id"].(string)
	if id == "" || player["alias"] != "JokerPro99" {
		t.Fatalf("unexpected player %v", player)
	}

	// Idempotent registration returns 200 with the same id.
	resp, again := postJSON(t, srv.URL+"/api/players", map[string]string{"alias": "jokerpro99"})
	if resp.StatusCode != http.StatusOK || again["id"] != id {
		t.Fatalf("idempotent register: %d %v", resp.StatusCode, again)
	}

	resp, roomResp := postJSON(t, srv.URL+"/api/rooms", map[string]any{"player_id": id, "target_score": 300, "with_bot": true})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create room: %d %v", resp.StatusCode, roomResp)
	}
	code, _ := roomResp["room_code"].(string)
	if !strings.HasPrefix(code, "JKR-") || len(code) != 8 {
		t.Fatalf("unexpected room code %q", code)
	}

	getResp, err := http.Get(srv.URL + "/api/rooms/" + strings.ToLower(code))
	if err != nil {
		t.Fatal(err)
	}
	defer getResp.Body.Close()
	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("get room: %d", getResp.StatusCode)
	}
	var details map[string]any
	_ = json.NewDecoder(getResp.Body).Decode(&details)
	if details["room_code"] != code || details["player_count"] != float64(2) {
		t.Fatalf("unexpected details %v", details)
	}
	if cards, _ := details["cards_in_hand"].([]any); len(cards) != 8 {
		t.Fatalf("expected 8 cards in hand, got %d", len(cards))
	}
}

func TestRegisterValidation(t *testing.T) {
	srv := newTestServer(t)
	for _, alias := range []string{"", "a", strings.Repeat("x", 31), "<script>", "bad alias", "Bot-JokerBot"} {
		resp, body := postJSON(t, srv.URL+"/api/players", map[string]string{"alias": alias})
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("alias %q: want 400, got %d (%v)", alias, resp.StatusCode, body)
		}
	}
	resp, err := http.Post(srv.URL+"/api/players", "application/json", strings.NewReader("{not json"))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("malformed JSON: want 400, got %d", resp.StatusCode)
	}
}

func TestCreateRoomValidation(t *testing.T) {
	srv := newTestServer(t)
	cases := []struct {
		name string
		body map[string]any
	}{
		{"missing player", map[string]any{"target_score": 300}},
		{"non numeric player", map[string]any{"player_id": "abc"}},
		{"unknown player", map[string]any{"player_id": "424242"}},
	}
	for _, c := range cases {
		resp, body := postJSON(t, srv.URL+"/api/rooms", c.body)
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("%s: want 400, got %d (%v)", c.name, resp.StatusCode, body)
		}
	}

	_, player := postJSON(t, srv.URL+"/api/players", map[string]string{"alias": "Valid1"})
	resp, body := postJSON(t, srv.URL+"/api/rooms", map[string]any{"player_id": player["id"], "target_score": -5})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("negative target: want 400, got %d (%v)", resp.StatusCode, body)
	}
}

func TestCreateRoomTargetScore(t *testing.T) {
	srv := newTestServer(t)
	_, player := postJSON(t, srv.URL+"/api/players", map[string]string{"alias": "Target1"})
	cases := []struct {
		name   string
		body   map[string]any
		status int
		want   float64
	}{
		{"default", map[string]any{"player_id": player["id"]}, http.StatusCreated, models.DefaultTargetScore},
		{"26000", map[string]any{"player_id": player["id"], "target_score": 26000}, http.StatusCreated, 26000},
		{"one", map[string]any{"player_id": player["id"], "target_score": 1}, http.StatusCreated, 1},
		{"too big", map[string]any{"player_id": player["id"], "target_score": 100001}, http.StatusBadRequest, 0},
		{"negative", map[string]any{"player_id": player["id"], "target_score": -5}, http.StatusBadRequest, 0},
	}
	for _, c := range cases {
		resp, body := postJSON(t, srv.URL+"/api/rooms", c.body)
		if resp.StatusCode != c.status {
			t.Errorf("%s: want %d, got %d (%v)", c.name, c.status, resp.StatusCode, body)
			continue
		}
		if c.status == http.StatusCreated && body["target_score"] != c.want {
			t.Errorf("%s: target_score %v, want %v", c.name, body["target_score"], c.want)
		}
	}
}

func TestGetRoomNotFound(t *testing.T) {
	srv := newTestServer(t)
	for _, code := range []string{"JKR-NOPE", "x", "bad%20code"} {
		resp, err := http.Get(srv.URL + "/api/rooms/" + code)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("code %q: want 404, got %d", code, resp.StatusCode)
		}
	}
}

func TestHealthMemoryMode(t *testing.T) {
	srv := newTestServer(t)
	resp, err := http.Get(srv.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var body map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if resp.StatusCode != http.StatusOK || body["status"] != "ok" || body["storage"] != "memory" {
		t.Fatalf("unexpected health %d %v", resp.StatusCode, body)
	}
}

func TestOriginPolicy(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://game.example/ws/JKR-AAAA", nil)
	req.Host = "game.example"
	cases := []struct {
		origin  string
		allowed []string
		want    bool
	}{
		{"", nil, true},
		{"http://game.example", nil, true},
		{"http://evil.example", nil, false},
		{"http://evil.example", []string{"http://evil.example"}, true},
		{"http://localhost:5173", nil, true},
	}
	for _, c := range cases {
		req.Header.Set("Origin", c.origin)
		if got := originAllowed(req, c.allowed); got != c.want {
			t.Errorf("origin %q allowed=%v: got %v want %v", c.origin, c.allowed, got, c.want)
		}
	}
}
