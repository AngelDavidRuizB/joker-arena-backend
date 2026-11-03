package monitor

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

type fakeDocker struct {
	cs   []Container
	err  error
	cpu  float64
	mem  uint64
	lim  uint64
	busy time.Duration
}

func (f *fakeDocker) ListContainers(context.Context, string) ([]Container, []string, error) {
	if f.err != nil {
		return nil, nil, f.err
	}
	ids := make([]string, len(f.cs))
	for i := range ids {
		ids[i] = "id"
	}
	return append([]Container(nil), f.cs...), ids, nil
}

func (f *fakeDocker) Stats(context.Context, string) (float64, uint64, uint64, error) {
	time.Sleep(f.busy)
	return f.cpu, f.mem, f.lim, nil
}

func okProbes(d DockerAPI) Probes {
	return Probes{
		Docker: d,
		Rabbit: func(context.Context) RabbitInfo {
			return RabbitInfo{Status: StatusOK, Queues: []QueueInfo{{"q", 1, 1}}}
		},
		Postgres: func(context.Context) PostgresInfo {
			return PostgresInfo{Status: StatusOK, Connections: 10, MaxConnections: 100, UsagePercent: 10}
		},
		Redis: func(context.Context) RedisInfo { return RedisInfo{Status: StatusOK, ConnectedClients: 3} },
	}
}

func TestCPUPercentLikeDockerStats(t *testing.T) {
	if got := CPUPercent(2_000, 1_000, 20_000, 10_000, 2); got != 20 {
		t.Fatalf("got %v, want 20", got)
	}
	if CPUPercent(1, 2, 3, 4, 1) != 0 {
		t.Fatal("counter reset must give 0")
	}
}

func TestLightClassification(t *testing.T) {
	cases := []struct {
		c    Container
		want string
	}{
		{Container{State: "exited"}, "red"},
		{Container{State: "running", Health: "unhealthy"}, "red"},
		{Container{State: "running", Health: "starting"}, "yellow"},
		{Container{State: "running", Health: "healthy", CPUPercent: 95}, "yellow"},
		{Container{State: "running", Health: "healthy", MemPercent: 90}, "yellow"},
		{Container{State: "running", Health: "healthy", CPUPercent: 3}, "green"},
	}
	for _, tc := range cases {
		if got := Light(tc.c); got != tc.want {
			t.Errorf("%+v: got %s want %s", tc.c, got, tc.want)
		}
	}
}

func TestHealthFromStatus(t *testing.T) {
	for in, want := range map[string]string{
		"Up 2 minutes (healthy)": "healthy", "Up 5 seconds (health: starting)": "starting",
		"Up 1 hour (unhealthy)": "unhealthy", "Exited (1) 3 seconds ago": "none",
	} {
		if got := HealthFromStatus(in); got != want {
			t.Errorf("%q: got %s want %s", in, got, want)
		}
	}
}

func TestParsers(t *testing.T) {
	ov := []byte(`{"queue_totals":{"messages_ready":4,"messages_unacknowledged":1},"message_stats":{"publish_details":{"rate":2.34},"deliver_get_details":{"rate":1.0}}}`)
	qs := []byte(`[{"name":"telemetry-ms.events","messages":4,"consumers":1},{"name":"x","messages":1200,"consumers":0}]`)
	info, err := ParseRabbit(ov, qs)
	if err != nil || info.MessagesReady != 4 || info.PublishRate != 2.3 || len(info.Queues) != 2 {
		t.Fatalf("rabbit: %+v %v", info, err)
	}
	if _, err := ParseRabbit([]byte("{roto"), qs); err == nil {
		t.Fatal("bad overview must fail")
	}
	n, err := ParseRedisClients("# Clients\r\nconnected_clients:7\r\nblocked_clients:0\r\n")
	if err != nil || n != 7 {
		t.Fatalf("redis: %d %v", n, err)
	}
}

func TestAlarms(t *testing.T) {
	s := Snapshot{
		Containers: []Container{{Name: "pg", Service: "jokerarena-postgres", State: "exited"}, {Name: "ok", State: "running", Health: "healthy"}},
		Sources:    map[string]string{"docker": StatusOK, "redis": StatusDegraded},
		Postgres:   PostgresInfo{Status: StatusOK, UsagePercent: 90},
		RabbitMQ:   RabbitInfo{Queues: []QueueInfo{{"q", 1500, 0}}},
	}
	got := Alarms(s)
	if len(got) != 5 {
		t.Fatalf("expected 5 alarms, got %+v", got)
	}
	if got[0].Severity != "critical" || got[1].Source != "redis" {
		t.Fatalf("order/severity: %+v", got)
	}
}

func TestCollectorDegradesOnlyTheFailingSource(t *testing.T) {
	c := NewCollector(Config{Interval: time.Second}, okProbes(&fakeDocker{err: errors.New("socket no montado")}))
	s := c.Collect(context.Background())
	if s.Sources["docker"] != StatusDegraded || s.Sources["postgres"] != StatusOK || s.Redis.ConnectedClients != 3 {
		t.Fatalf("snapshot: %+v", s)
	}
	if len(s.Alarms) != 1 || s.Alarms[0].Source != "docker" {
		t.Fatalf("alarms: %+v", s.Alarms)
	}
}

func TestCollectorComputesMemoryAndLights(t *testing.T) {
	d := &fakeDocker{cs: []Container{{Name: "a", State: "running", Health: "healthy"}, {Name: "b", State: "exited"}}, cpu: 5, mem: 50, lim: 100}
	s := NewCollector(Config{Interval: time.Second}, okProbes(d)).Collect(context.Background())
	if s.Containers[0].MemPercent != 50 || s.Containers[0].Light != "green" || s.Containers[1].Light != "red" || s.Containers[1].CPUPercent != 0 {
		t.Fatalf("containers: %+v", s.Containers)
	}
}

func TestSlowSubscriberNeverBlocksThePoller(t *testing.T) {
	c := NewCollector(Config{Interval: time.Second}, okProbes(&fakeDocker{}))
	ch, unsub := c.Subscribe()
	defer unsub()
	done := make(chan struct{})
	go func() {
		for i := 0; i < 10; i++ { // nadie lee: el buffer (2) se llena y los ticks siguientes se saltan
			c.Tick(context.Background())
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("the poller blocked on a slow subscriber")
	}
	var s Snapshot
	if err := json.Unmarshal(<-ch, &s); err != nil || s.Sources["docker"] != StatusOK {
		t.Fatalf("snapshot: %v %+v", err, s)
	}
}
