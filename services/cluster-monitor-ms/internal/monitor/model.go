// Package monitor aggregates the health of the JokerArena cluster.
package monitor

import "time"

// Container is the observed state of one compose container.
type Container struct {
	Name          string  `json:"name"`
	Service       string  `json:"service"`
	Image         string  `json:"image"`
	State         string  `json:"state"`  // running | exited | restarting ...
	Health        string  `json:"health"` // healthy | unhealthy | starting | none
	CPUPercent    float64 `json:"cpu_percent"`
	MemBytes      uint64  `json:"mem_bytes"`
	MemLimitBytes uint64  `json:"mem_limit_bytes"`
	MemPercent    float64 `json:"mem_percent"`
	Light         string  `json:"light"` // green | yellow | red
}

// QueueInfo summarizes one RabbitMQ queue.
type QueueInfo struct {
	Name      string `json:"name"`
	Messages  int    `json:"messages"`
	Consumers int    `json:"consumers"`
}

// RabbitInfo comes from the management API.
type RabbitInfo struct {
	Status          string      `json:"status"`
	MessagesReady   int         `json:"messages_ready"`
	MessagesUnacked int         `json:"messages_unacked"`
	PublishRate     float64     `json:"publish_rate"`
	DeliverRate     float64     `json:"deliver_rate"`
	Queues          []QueueInfo `json:"queues"`
	LatencyMs       int64       `json:"latency_ms"`
}

// PostgresInfo comes from pg_stat_activity.
type PostgresInfo struct {
	Status         string  `json:"status"`
	Connections    int     `json:"connections"`
	MaxConnections int     `json:"max_connections"`
	UsagePercent   float64 `json:"usage_percent"`
	LatencyMs      int64   `json:"latency_ms"`
}

// RedisInfo comes from INFO clients.
type RedisInfo struct {
	Status           string `json:"status"`
	ConnectedClients int    `json:"connected_clients"`
	LatencyMs        int64  `json:"latency_ms"`
}

// Alarm is a condition the operator should look at.
type Alarm struct {
	Source   string `json:"source"`
	Severity string `json:"severity"` // warning | critical
	Message  string `json:"message"`
}

// Snapshot is the full picture streamed on /admin/cluster/metrics.
type Snapshot struct {
	GeneratedAt     time.Time         `json:"generated_at"`
	IntervalSeconds int               `json:"interval_seconds"`
	Containers      []Container       `json:"containers"`
	RabbitMQ        RabbitInfo        `json:"rabbitmq"`
	Postgres        PostgresInfo      `json:"postgres"`
	Redis           RedisInfo         `json:"redis"`
	Sources         map[string]string `json:"sources"` // docker|rabbitmq|postgres|redis -> ok|degraded
	Alarms          []Alarm           `json:"alarms"`
}

// Thresholds of the alarms (documented in the README).
const (
	CPUWarnPercent      = 80.0
	MemWarnPercent      = 85.0
	PGConnWarnPercent   = 80.0
	QueueBacklogWarning = 1000
	StatusOK            = "ok"
	StatusDegraded      = "degraded"
)
