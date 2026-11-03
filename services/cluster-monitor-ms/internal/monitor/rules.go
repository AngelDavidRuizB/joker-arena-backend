package monitor

import (
	"fmt"
	"sort"
)

// CPUPercent computes the CPU usage like `docker stats`.
func CPUPercent(cpuTotal, preCPUTotal, system, preSystem uint64, onlineCPUs int) float64 {
	if onlineCPUs <= 0 {
		onlineCPUs = 1
	}
	if cpuTotal < preCPUTotal || system <= preSystem {
		return 0
	}
	cpuDelta := float64(cpuTotal - preCPUTotal)
	sysDelta := float64(system - preSystem)
	return round1(cpuDelta / sysDelta * float64(onlineCPUs) * 100)
}

func round1(f float64) float64 { return float64(int64(f*10+0.5)) / 10 }

// Light classifies a container for the traffic-light grid.
func Light(c Container) string {
	switch {
	case c.State != "running" || c.Health == "unhealthy":
		return "red"
	case c.Health == "starting" || c.CPUPercent >= CPUWarnPercent || c.MemPercent >= MemWarnPercent:
		return "yellow"
	default:
		return "green"
	}
}

// Alarms derives the alarm list from a snapshot (deterministic order).
func Alarms(s Snapshot) []Alarm {
	var out []Alarm
	for _, c := range s.Containers {
		switch {
		case c.State != "running":
			out = append(out, Alarm{c.Service, "critical", fmt.Sprintf("%s no está en ejecución (%s)", c.Name, c.State)})
		case c.Health == "unhealthy":
			out = append(out, Alarm{c.Service, "critical", fmt.Sprintf("%s reporta unhealthy", c.Name)})
		case c.CPUPercent >= CPUWarnPercent:
			out = append(out, Alarm{c.Service, "warning", fmt.Sprintf("%s usa %.1f%% de CPU", c.Name, c.CPUPercent)})
		case c.MemPercent >= MemWarnPercent:
			out = append(out, Alarm{c.Service, "warning", fmt.Sprintf("%s usa %.1f%% de memoria", c.Name, c.MemPercent)})
		}
	}
	names := make([]string, 0, len(s.Sources))
	for k := range s.Sources {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, k := range names {
		if s.Sources[k] != StatusOK {
			out = append(out, Alarm{k, "critical", fmt.Sprintf("%s no responde a las sondas", k)})
		}
	}
	if s.Postgres.Status == StatusOK && s.Postgres.UsagePercent >= PGConnWarnPercent {
		out = append(out, Alarm{"postgres", "warning", fmt.Sprintf("conexiones al %.0f%% del máximo", s.Postgres.UsagePercent)})
	}
	for _, q := range s.RabbitMQ.Queues {
		if q.Messages >= QueueBacklogWarning {
			out = append(out, Alarm{"rabbitmq", "warning", fmt.Sprintf("cola %s con %d mensajes pendientes", q.Name, q.Messages)})
		}
		if q.Consumers == 0 {
			out = append(out, Alarm{"rabbitmq", "warning", fmt.Sprintf("cola %s sin consumidores", q.Name)})
		}
	}
	if out == nil {
		out = []Alarm{}
	}
	return out
}
