package main

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"cluster-monitor-ms/internal/monitor"
)

type noDocker struct{}

func (noDocker) ListContainers(context.Context, string) ([]monitor.Container, []string, error) {
	return []monitor.Container{{Name: "x", State: "running", Health: "healthy"}}, []string{"id"}, nil
}
func (noDocker) Stats(context.Context, string) (float64, uint64, uint64, error) { return 1, 1, 2, nil }

func TestWebSocketSendsInitialAndPeriodicSnapshots(t *testing.T) {
	c := monitor.NewCollector(monitor.Config{Interval: time.Second}, monitor.Probes{
		Docker:   noDocker{},
		Rabbit:   func(context.Context) monitor.RabbitInfo { return monitor.RabbitInfo{Status: "ok"} },
		Postgres: func(context.Context) monitor.PostgresInfo { return monitor.PostgresInfo{Status: "ok"} },
		Redis:    func(context.Context) monitor.RedisInfo { return monitor.RedisInfo{Status: "ok"} },
	})
	srv := httptest.NewServer(NewRouter(c))
	defer srv.Close()

	if resp, _ := srv.Client().Get(srv.URL + "/admin/cluster/snapshot"); resp.StatusCode != 503 {
		t.Fatalf("before the first tick the snapshot must be 503, got %d", resp.StatusCode)
	}
	c.Tick(context.Background())
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http")+"/admin/cluster/metrics", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	for i := 0; i < 2; i++ {
		if i == 1 {
			go c.Tick(context.Background())
		}
		_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
		_, msg, err := conn.ReadMessage()
		if err != nil {
			t.Fatal(err)
		}
		var s monitor.Snapshot
		if json.Unmarshal(msg, &s) != nil || len(s.Containers) != 1 || s.Containers[0].Light != "green" {
			t.Fatalf("message %d: %s", i, msg)
		}
	}
}
