// cluster-monitor-ms: NOC del clúster JokerArena (snapshot REST + WebSocket de métricas).
//
// Concesión de seguridad documentada: lee el socket de Docker montado en SOLO LECTURA y
// únicamente en este contenedor. Es una herramienta de observabilidad para administración,
// no una configuración para producción (ver README).
package main

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/gorilla/websocket"

	"cluster-monitor-ms/internal/monitor"
)

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

const (
	writeWait  = 5 * time.Second
	pingPeriod = 25 * time.Second
)

var upgrader = websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }} // detrás del gateway (/admin con JWT)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// NewRouter exposes /health, the snapshot and the WebSocket stream.
func NewRouter(c *monitor.Collector) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		status := "ok"
		if c.Last() == nil {
			status = "starting"
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": status, "service": "cluster-monitor-ms", "deps": map[string]string{}, "ws_clients": c.Subscribers()})
	})
	mux.HandleFunc("GET /admin/cluster/snapshot", func(w http.ResponseWriter, _ *http.Request) {
		last := c.Last()
		if last == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "no_snapshot_yet", "message": "Todavía no hay un snapshot; intente en unos segundos."})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(last)
	})
	mux.HandleFunc("/admin/cluster/metrics", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		ch, unsubscribe := c.Subscribe()
		defer unsubscribe()
		// Lector: detecta el cierre del cliente.
		closed := make(chan struct{})
		go func() {
			defer close(closed)
			for {
				if _, _, err := conn.ReadMessage(); err != nil {
					return
				}
			}
		}()
		send := func(b []byte) bool {
			_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
			return conn.WriteMessage(websocket.TextMessage, b) == nil
		}
		if last := c.Last(); last != nil && !send(last) {
			return
		}
		ping := time.NewTicker(pingPeriod)
		defer ping.Stop()
		for {
			select {
			case <-closed:
				return
			case b := <-ch:
				if !send(b) {
					return
				}
			case <-ping.C:
				_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
				if conn.WriteMessage(websocket.PingMessage, nil) != nil {
					return
				}
			}
		}
	})
	return mux
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--healthcheck" {
		resp, err := http.Get("http://127.0.0.1:" + env("PORT", "8088") + "/health")
		if err != nil || resp.StatusCode != http.StatusOK {
			os.Exit(1)
		}
		os.Exit(0)
	}
	interval, _ := strconv.Atoi(env("POLL_INTERVAL_SECONDS", "5"))
	cfg := monitor.Config{
		Project:     env("COMPOSE_PROJECT", "jokerarena"),
		Interval:    time.Duration(max(interval, 2)) * time.Second,
		RabbitURL:   env("RABBITMQ_MGMT_URL", "http://jokerarena-rabbitmq:15672"),
		RabbitUser:  env("RABBITMQ_MONITOR_USER", ""),
		RabbitPass:  os.Getenv("RABBITMQ_MONITOR_PASSWORD"),
		PostgresDSN: os.Getenv("POSTGRES_MONITOR_URL"),
		RedisAddr:   env("REDIS_ADDR", "jokerarena-redis:6379"),
	}
	collector := monitor.NewCollector(cfg, monitor.Probes{
		Docker: monitor.NewSocketDocker(env("DOCKER_SOCKET", "/var/run/docker.sock")),
		Rabbit: func(ctx context.Context) monitor.RabbitInfo {
			return monitor.ProbeRabbit(ctx, cfg.RabbitURL, cfg.RabbitUser, cfg.RabbitPass)
		},
		Postgres: func(ctx context.Context) monitor.PostgresInfo { return monitor.ProbePostgres(ctx, cfg.PostgresDSN) },
		Redis:    func(ctx context.Context) monitor.RedisInfo { return monitor.ProbeRedis(ctx, cfg.RedisAddr) },
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go collector.Run(ctx)

	srv := &http.Server{Addr: net.JoinHostPort("0.0.0.0", env("PORT", "8088")), Handler: NewRouter(collector), ReadHeaderTimeout: 5 * time.Second}
	go func() {
		log.Printf("cluster-monitor-ms escuchando en %s (proyecto %s, cada %s)", srv.Addr, cfg.Project, cfg.Interval)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	cancel()
	sctx, scancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer scancel()
	_ = srv.Shutdown(sctx)
}
