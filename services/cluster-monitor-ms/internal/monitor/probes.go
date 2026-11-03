package monitor

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
)

// ---------------------------------------------------------------- Docker

// DockerAPI is the subset of the Docker Engine API we use (fake in tests).
type DockerAPI interface {
	ListContainers(ctx context.Context, project string) ([]Container, []string, error)
	Stats(ctx context.Context, id string) (cpu float64, mem, limit uint64, err error)
}

// SocketDocker talks HTTP over the (read-only mounted) Docker socket.
type SocketDocker struct{ client *http.Client }

// NewSocketDocker creates a client for a unix socket path.
func NewSocketDocker(socket string) *SocketDocker {
	tr := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", socket)
	}}
	return &SocketDocker{client: &http.Client{Transport: tr, Timeout: 5 * time.Second}}
}

func (d *SocketDocker) get(ctx context.Context, path string, out any) error {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://docker"+path, nil)
	resp, err := d.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("docker API %s: %d", path, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// ListContainers returns the containers of the compose project and their ids.
func (d *SocketDocker) ListContainers(ctx context.Context, project string) ([]Container, []string, error) {
	filters, _ := json.Marshal(map[string][]string{"label": {"com.docker.compose.project=" + project}})
	var raw []struct {
		ID     string            `json:"Id"`
		Names  []string          `json:"Names"`
		Image  string            `json:"Image"`
		State  string            `json:"State"`
		Status string            `json:"Status"`
		Labels map[string]string `json:"Labels"`
	}
	if err := d.get(ctx, "/containers/json?all=true&filters="+url.QueryEscape(string(filters)), &raw); err != nil {
		return nil, nil, err
	}
	cs := make([]Container, 0, len(raw))
	ids := make([]string, 0, len(raw))
	for _, r := range raw {
		name := r.ID[:12]
		if len(r.Names) > 0 {
			name = strings.TrimPrefix(r.Names[0], "/")
		}
		cs = append(cs, Container{Name: name, Service: r.Labels["com.docker.compose.service"], Image: r.Image, State: r.State, Health: HealthFromStatus(r.Status)})
		ids = append(ids, r.ID)
	}
	return cs, ids, nil
}

// HealthFromStatus extracts the healthcheck state from the Status text ("Up 3 minutes (healthy)").
func HealthFromStatus(status string) string {
	for _, h := range []string{"unhealthy", "healthy", "health: starting"} {
		if strings.Contains(status, "("+h+")") {
			if h == "health: starting" {
				return "starting"
			}
			return h
		}
	}
	return "none"
}

// Stats returns CPU %, memory usage and limit of one container.
func (d *SocketDocker) Stats(ctx context.Context, id string) (float64, uint64, uint64, error) {
	var s struct {
		CPU struct {
			Usage struct {
				Total uint64 `json:"total_usage"`
			} `json:"cpu_usage"`
			System uint64 `json:"system_cpu_usage"`
			Online int    `json:"online_cpus"`
		} `json:"cpu_stats"`
		Pre struct {
			Usage struct {
				Total uint64 `json:"total_usage"`
			} `json:"cpu_usage"`
			System uint64 `json:"system_cpu_usage"`
		} `json:"precpu_stats"`
		Mem struct {
			Usage uint64            `json:"usage"`
			Limit uint64            `json:"limit"`
			Stats map[string]uint64 `json:"stats"`
		} `json:"memory_stats"`
	}
	if err := d.get(ctx, "/containers/"+id+"/stats?stream=false", &s); err != nil {
		return 0, 0, 0, err
	}
	mem := s.Mem.Usage
	if cache, ok := s.Mem.Stats["inactive_file"]; ok && cache < mem {
		mem -= cache // como `docker stats`
	}
	return CPUPercent(s.CPU.Usage.Total, s.Pre.Usage.Total, s.CPU.System, s.Pre.System, s.CPU.Online), mem, s.Mem.Limit, nil
}

// CollectContainers lists the project containers and fetches their stats concurrently.
func CollectContainers(ctx context.Context, d DockerAPI, project string) ([]Container, error) {
	cs, ids, err := d.ListContainers(ctx, project)
	if err != nil {
		return nil, err
	}
	var wg sync.WaitGroup
	for i := range cs {
		if cs[i].State != "running" {
			continue
		}
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			cpu, mem, limit, err := d.Stats(ctx, ids[i])
			if err != nil {
				return
			}
			cs[i].CPUPercent, cs[i].MemBytes, cs[i].MemLimitBytes = cpu, mem, limit
			if limit > 0 {
				cs[i].MemPercent = round1(float64(mem) / float64(limit) * 100)
			}
		}(i)
	}
	wg.Wait()
	for i := range cs {
		cs[i].Light = Light(cs[i])
	}
	return cs, nil
}

// ---------------------------------------------------------------- RabbitMQ

// ParseRabbit combines /api/overview and /api/queues JSON bodies.
func ParseRabbit(overview, queues []byte) (RabbitInfo, error) {
	var ov struct {
		QueueTotals struct {
			Ready   int `json:"messages_ready"`
			Unacked int `json:"messages_unacknowledged"`
		} `json:"queue_totals"`
		MessageStats struct {
			Publish struct {
				Rate float64 `json:"rate"`
			} `json:"publish_details"`
			Deliver struct {
				Rate float64 `json:"rate"`
			} `json:"deliver_get_details"`
		} `json:"message_stats"`
	}
	if err := json.Unmarshal(overview, &ov); err != nil {
		return RabbitInfo{}, fmt.Errorf("overview inválido: %w", err)
	}
	var qs []struct {
		Name      string `json:"name"`
		Messages  int    `json:"messages"`
		Consumers int    `json:"consumers"`
	}
	if err := json.Unmarshal(queues, &qs); err != nil {
		return RabbitInfo{}, fmt.Errorf("queues inválido: %w", err)
	}
	info := RabbitInfo{Status: StatusOK, MessagesReady: ov.QueueTotals.Ready, MessagesUnacked: ov.QueueTotals.Unacked,
		PublishRate: round1(ov.MessageStats.Publish.Rate), DeliverRate: round1(ov.MessageStats.Deliver.Rate), Queues: []QueueInfo{}}
	for _, q := range qs {
		info.Queues = append(info.Queues, QueueInfo{q.Name, q.Messages, q.Consumers})
	}
	return info, nil
}

// ProbeRabbit queries the management API with the monitoring credentials.
func ProbeRabbit(ctx context.Context, base, user, pass string) RabbitInfo {
	start := time.Now()
	get := func(path string) ([]byte, error) {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(base, "/")+path, nil)
		req.SetBasicAuth(user, pass)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("status %d", resp.StatusCode)
		}
		var b strings.Builder
		_, err = bufio.NewReader(resp.Body).WriteTo(&b)
		return []byte(b.String()), err
	}
	ov, err1 := get("/api/overview")
	qs, err2 := get("/api/queues?columns=name,messages,consumers")
	if err1 != nil || err2 != nil {
		return RabbitInfo{Status: StatusDegraded, Queues: []QueueInfo{}}
	}
	info, err := ParseRabbit(ov, qs)
	if err != nil {
		return RabbitInfo{Status: StatusDegraded, Queues: []QueueInfo{}}
	}
	info.LatencyMs = time.Since(start).Milliseconds()
	return info
}

// ---------------------------------------------------------------- PostgreSQL

// ProbePostgres counts connections with the read-only monitoring user (pg_monitor).
func ProbePostgres(ctx context.Context, dsn string) PostgresInfo {
	start := time.Now()
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return PostgresInfo{Status: StatusDegraded}
	}
	defer conn.Close(context.Background())
	var used, max int
	err = conn.QueryRow(ctx, `SELECT (SELECT count(*) FROM pg_stat_activity)::int,
	                                  current_setting('max_connections')::int`).Scan(&used, &max)
	if err != nil {
		return PostgresInfo{Status: StatusDegraded}
	}
	usage := 0.0
	if max > 0 {
		usage = round1(float64(used) / float64(max) * 100)
	}
	return PostgresInfo{Status: StatusOK, Connections: used, MaxConnections: max, UsagePercent: usage, LatencyMs: time.Since(start).Milliseconds()}
}

// ---------------------------------------------------------------- Redis

// ParseRedisClients reads connected_clients from an INFO clients reply.
func ParseRedisClients(info string) (int, error) {
	for _, line := range strings.Split(info, "\n") {
		if v, ok := strings.CutPrefix(strings.TrimSpace(line), "connected_clients:"); ok {
			return strconv.Atoi(v)
		}
	}
	return 0, fmt.Errorf("connected_clients no encontrado")
}

// ProbeRedis sends INFO clients over a raw RESP connection (no client library needed).
func ProbeRedis(ctx context.Context, addr string) RedisInfo {
	start := time.Now()
	d := net.Dialer{Timeout: 2 * time.Second}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return RedisInfo{Status: StatusDegraded}
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := conn.Write([]byte("*2\r\n$4\r\nINFO\r\n$7\r\nclients\r\n")); err != nil {
		return RedisInfo{Status: StatusDegraded}
	}
	r := bufio.NewReader(conn)
	header, err := r.ReadString('\n')
	if err != nil || !strings.HasPrefix(header, "$") {
		return RedisInfo{Status: StatusDegraded}
	}
	n, err := strconv.Atoi(strings.TrimSpace(header[1:]))
	if err != nil {
		return RedisInfo{Status: StatusDegraded}
	}
	buf := make([]byte, n)
	if _, err := ioReadFull(r, buf); err != nil {
		return RedisInfo{Status: StatusDegraded}
	}
	clients, err := ParseRedisClients(string(buf))
	if err != nil {
		return RedisInfo{Status: StatusDegraded}
	}
	return RedisInfo{Status: StatusOK, ConnectedClients: clients, LatencyMs: time.Since(start).Milliseconds()}
}

func ioReadFull(r *bufio.Reader, buf []byte) (int, error) {
	total := 0
	for total < len(buf) {
		n, err := r.Read(buf[total:])
		total += n
		if err != nil {
			return total, err
		}
	}
	return total, nil
}
