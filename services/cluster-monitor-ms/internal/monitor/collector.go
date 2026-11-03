package monitor

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"
)

// Config of the collector.
type Config struct {
	Project     string
	Interval    time.Duration
	RabbitURL   string
	RabbitUser  string
	RabbitPass  string
	PostgresDSN string
	RedisAddr   string
}

// Probes lets tests replace the network probes.
type Probes struct {
	Docker   DockerAPI
	Rabbit   func(ctx context.Context) RabbitInfo
	Postgres func(ctx context.Context) PostgresInfo
	Redis    func(ctx context.Context) RedisInfo
}

// Collector polls every source on a fixed interval and keeps the last snapshot.
// Subscribers (WebSocket clients) receive each new snapshot as JSON.
type Collector struct {
	cfg    Config
	probes Probes

	mu   sync.RWMutex
	last []byte
	subs map[chan []byte]struct{}
}

// NewCollector creates a collector.
func NewCollector(cfg Config, p Probes) *Collector {
	return &Collector{cfg: cfg, probes: p, subs: map[chan []byte]struct{}{}}
}

// Collect builds one snapshot. A failing source is marked degraded; the others still report.
func (c *Collector) Collect(ctx context.Context) Snapshot {
	s := Snapshot{GeneratedAt: time.Now().UTC(), IntervalSeconds: int(c.cfg.Interval.Seconds()), Sources: map[string]string{}}
	var wg sync.WaitGroup
	wg.Add(4)
	go func() {
		defer wg.Done()
		cs, err := CollectContainers(ctx, c.probes.Docker, c.cfg.Project)
		if err != nil {
			log.Printf("[WARN] docker no disponible: %v", err)
			s.Sources["docker"], s.Containers = StatusDegraded, []Container{}
			return
		}
		s.Sources["docker"], s.Containers = StatusOK, cs
	}()
	go func() { defer wg.Done(); s.RabbitMQ = c.probes.Rabbit(ctx) }()
	go func() { defer wg.Done(); s.Postgres = c.probes.Postgres(ctx) }()
	go func() { defer wg.Done(); s.Redis = c.probes.Redis(ctx) }()
	wg.Wait()
	s.Sources["rabbitmq"], s.Sources["postgres"], s.Sources["redis"] = s.RabbitMQ.Status, s.Postgres.Status, s.Redis.Status
	s.Alarms = Alarms(s)
	return s
}

// Tick collects once, stores the snapshot and fans it out.
func (c *Collector) Tick(ctx context.Context) {
	tctx, cancel := context.WithTimeout(ctx, c.cfg.Interval+5*time.Second)
	defer cancel()
	b, _ := json.Marshal(c.Collect(tctx))
	c.mu.Lock()
	c.last = b
	for ch := range c.subs {
		select {
		case ch <- b:
		default: // cliente lento: se salta este tick, nunca bloquea al poller
		}
	}
	c.mu.Unlock()
}

// Run polls until ctx ends.
func (c *Collector) Run(ctx context.Context) {
	c.Tick(ctx)
	t := time.NewTicker(c.cfg.Interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			c.Tick(ctx)
		}
	}
}

// Last returns the most recent snapshot (JSON) or nil.
func (c *Collector) Last() []byte {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.last
}

// Subscribe registers a receiver of snapshots. Call the returned func to unsubscribe.
func (c *Collector) Subscribe() (<-chan []byte, func()) {
	ch := make(chan []byte, 2)
	c.mu.Lock()
	c.subs[ch] = struct{}{}
	c.mu.Unlock()
	return ch, func() {
		c.mu.Lock()
		delete(c.subs, ch)
		c.mu.Unlock()
	}
}

// Subscribers returns how many clients are connected.
func (c *Collector) Subscribers() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.subs)
}
