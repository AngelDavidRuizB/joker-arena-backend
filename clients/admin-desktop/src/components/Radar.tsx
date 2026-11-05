import type { Snapshot } from "../types/snapshot";
import { ContainerCard } from "./ContainerCard";

interface RadarProps {
  snapshot: Snapshot | null;
  status: "connecting" | "connected" | "reconnecting" | "disconnected";
  lastUpdated: Date | null;
  onReconnect: () => void;
  demoMode: boolean;
}

// ---- Demo mock data (used when demoMode=true or snapshot is null in demo) ----
const MOCK_SNAPSHOT: Snapshot = {
  generated_at: new Date().toISOString(),
  interval_seconds: 5,
  containers: [
    { name: "jokerarena-gateway-1",        service: "jokerarena-gateway",        image: "jokerarena/gateway:latest",        state: "running", health: "healthy",   cpu_percent: 2.1,  mem_bytes: 52_428_800,   mem_limit_bytes: 268_435_456, mem_percent: 19.5, light: "green"  },
    { name: "jokerarena-auth-ms-1",        service: "auth-ms",                   image: "jokerarena/auth-ms:latest",        state: "running", health: "healthy",   cpu_percent: 0.8,  mem_bytes: 38_797_312,   mem_limit_bytes: 268_435_456, mem_percent: 14.4, light: "green"  },
    { name: "jokerarena-game-engine-ms-1", service: "game-engine-ms",            image: "jokerarena/game-engine-ms:latest", state: "running", health: "healthy",   cpu_percent: 11.3, mem_bytes: 104_857_600,  mem_limit_bytes: 536_870_912, mem_percent: 19.5, light: "green"  },
    { name: "jokerarena-bot-ai-ms-1",      service: "bot-ai-ms",                 image: "jokerarena/bot-ai-ms:latest",      state: "running", health: "healthy",   cpu_percent: 0.3,  mem_bytes: 20_971_520,   mem_limit_bytes: 268_435_456, mem_percent: 7.8,  light: "green"  },
    { name: "jokerarena-rooms-ms-1",       service: "rooms-ms",                  image: "jokerarena/rooms-ms:latest",       state: "running", health: "healthy",   cpu_percent: 1.5,  mem_bytes: 45_088_768,   mem_limit_bytes: 268_435_456, mem_percent: 16.8, light: "green"  },
    { name: "jokerarena-telemetry-ms-1",   service: "telemetry-ms",              image: "jokerarena/telemetry-ms:latest",   state: "running", health: "healthy",   cpu_percent: 3.2,  mem_bytes: 62_914_560,   mem_limit_bytes: 268_435_456, mem_percent: 23.4, light: "green"  },
    { name: "jokerarena-stats-ms-1",       service: "stats-ms",                  image: "jokerarena/stats-ms:latest",       state: "running", health: "starting",  cpu_percent: 0.1,  mem_bytes: 15_728_640,   mem_limit_bytes: 268_435_456, mem_percent: 5.9,  light: "yellow" },
    { name: "jokerarena-liveops-rules-ms-1", service: "liveops-rules-ms",        image: "jokerarena/liveops-rules:latest",  state: "running", health: "healthy",   cpu_percent: 0.5,  mem_bytes: 31_457_280,   mem_limit_bytes: 268_435_456, mem_percent: 11.7, light: "green"  },
    { name: "jokerarena-cluster-monitor-1", service: "cluster-monitor-ms",       image: "jokerarena/cluster-monitor:latest",state: "running", health: "healthy",   cpu_percent: 1.9,  mem_bytes: 41_943_040,   mem_limit_bytes: 268_435_456, mem_percent: 15.6, light: "green"  },
  ],
  rabbitmq: { status: "ok", messages_ready: 12, messages_unacked: 2, publish_rate: 4.3, deliver_rate: 4.1, queues: [{ name: "game-events", messages: 12, consumers: 3 }], latency_ms: 8 },
  postgres:  { status: "ok", connections: 24, max_connections: 100, usage_percent: 24.0, latency_ms: 3 },
  redis:     { status: "ok", connected_clients: 7, latency_ms: 1 },
  sources:   { docker: "ok", rabbitmq: "ok", postgres: "ok", redis: "ok" },
  alarms:    [],
};

const STATUS_LABELS: Record<string, string> = {
  connecting:    "⏳ Conectando…",
  connected:     "🟢 Conectado",
  reconnecting:  "🔄 Reconectando…",
  disconnected:  "🔴 Desconectado",
};

export function Radar({ snapshot, status, lastUpdated, onReconnect, demoMode }: RadarProps) {
  const data: Snapshot | null = demoMode ? MOCK_SNAPSHOT : snapshot;

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: "1rem", padding: "1rem" }}>
      {/* Status bar */}
      <div style={styles.statusBar}>
        <span>{STATUS_LABELS[status]}</span>
        {lastUpdated && !demoMode && (
          <span style={{ color: "#64748b", fontSize: "0.75rem" }}>
            Última actualización: {lastUpdated.toLocaleTimeString()}
          </span>
        )}
        {demoMode && <span style={styles.demoBadge}>DEMO</span>}
        {(status === "disconnected" || status === "reconnecting") && !demoMode && (
          <button onClick={onReconnect} style={styles.reconnectBtn}>
            Reconectar
          </button>
        )}
      </div>

      {!data ? (
        <div style={styles.empty}>
          Sin datos — esperando primer snapshot del cluster…
        </div>
      ) : (
        <>
          {/* Alarms */}
          {data.alarms.length > 0 && (
            <div style={styles.alarmBanner}>
              <strong>⚠ Alarmas activas ({data.alarms.length})</strong>
              {data.alarms.map((a, i) => (
                <div key={i} style={{ color: a.severity === "critical" ? "#f87171" : "#fbbf24", fontSize: "0.8rem" }}>
                  [{a.severity.toUpperCase()}] {a.source}: {a.message}
                </div>
              ))}
            </div>
          )}

          {/* Container grid */}
          <section>
            <h3 style={styles.sectionTitle}>
              🐳 Contenedores ({data.containers.length})
            </h3>
            <div style={styles.grid}>
              {data.containers.map((c) => (
                <ContainerCard key={c.name} container={c} />
              ))}
            </div>
          </section>

          {/* Infra row */}
          <div style={styles.infraRow}>
            {/* RabbitMQ */}
            <InfraCard title="🐇 RabbitMQ" status={data.rabbitmq.status}>
              <KV k="Listos"      v={data.rabbitmq.messages_ready} />
              <KV k="Sin ack"     v={data.rabbitmq.messages_unacked} />
              <KV k="Pub/s"       v={data.rabbitmq.publish_rate.toFixed(1)} />
              <KV k="Deliver/s"   v={data.rabbitmq.deliver_rate.toFixed(1)} />
              <KV k="Latencia"    v={`${data.rabbitmq.latency_ms} ms`} />
            </InfraCard>

            {/* Postgres */}
            <InfraCard title="🐘 PostgreSQL" status={data.postgres.status}>
              <KV k="Conexiones"  v={`${data.postgres.connections} / ${data.postgres.max_connections}`} />
              <KV k="Uso"         v={`${data.postgres.usage_percent.toFixed(1)} %`} />
              <KV k="Latencia"    v={`${data.postgres.latency_ms} ms`} />
            </InfraCard>

            {/* Redis */}
            <InfraCard title="⚡ Redis" status={data.redis.status}>
              <KV k="Clientes"    v={data.redis.connected_clients} />
              <KV k="Latencia"    v={`${data.redis.latency_ms} ms`} />
            </InfraCard>
          </div>

          {/* Footer */}
          <div style={styles.footer}>
            Snapshot generado: {new Date(data.generated_at).toLocaleString()} — intervalo{" "}
            {data.interval_seconds}s
          </div>
        </>
      )}
    </div>
  );
}

function InfraCard({
  title,
  status,
  children,
}: {
  title: string;
  status: string;
  children: React.ReactNode;
}) {
  const isOk = status === "ok";
  return (
    <div
      style={{
        ...styles.infraCard,
        borderColor: isOk ? "#166534" : "#7f1d1d",
        background: isOk ? "#052e16" : "#2d0a0a",
      }}
    >
      <div style={{ display: "flex", justifyContent: "space-between", marginBottom: "0.5rem" }}>
        <strong style={{ fontSize: "0.85rem", color: "#e2e8f0" }}>{title}</strong>
        <span style={{ fontSize: "0.75rem", color: isOk ? "#4ade80" : "#f87171" }}>
          {isOk ? "● OK" : "● DEGRADED"}
        </span>
      </div>
      {children}
    </div>
  );
}

function KV({ k, v }: { k: string; v: string | number }) {
  return (
    <div style={{ display: "flex", justifyContent: "space-between", fontSize: "0.72rem" }}>
      <span style={{ color: "#64748b" }}>{k}</span>
      <span style={{ color: "#cbd5e1", fontFamily: "monospace" }}>{v}</span>
    </div>
  );
}

const styles: Record<string, React.CSSProperties> = {
  statusBar: {
    display: "flex",
    alignItems: "center",
    gap: "1rem",
    background: "#0f172a",
    border: "1px solid #1e293b",
    borderRadius: 6,
    padding: "0.5rem 1rem",
    fontSize: "0.85rem",
    color: "#94a3b8",
  },
  demoBadge: {
    background: "#854d0e",
    color: "#fef9c3",
    borderRadius: 4,
    padding: "2px 8px",
    fontSize: "0.7rem",
    fontWeight: 700,
    letterSpacing: "0.1em",
  },
  reconnectBtn: {
    marginLeft: "auto",
    background: "#1e40af",
    color: "#fff",
    border: "none",
    borderRadius: 4,
    padding: "4px 12px",
    cursor: "pointer",
    fontSize: "0.8rem",
  },
  empty: {
    textAlign: "center",
    color: "#475569",
    padding: "3rem",
    fontSize: "0.9rem",
  },
  alarmBanner: {
    background: "#2d0a0a",
    border: "1px solid #7f1d1d",
    borderRadius: 8,
    padding: "0.75rem 1rem",
    display: "flex",
    flexDirection: "column",
    gap: 4,
    color: "#e2e8f0",
  },
  sectionTitle: { margin: "0 0 0.5rem", fontSize: "0.9rem", color: "#94a3b8" },
  grid: {
    display: "flex",
    flexWrap: "wrap",
    gap: "0.75rem",
  },
  infraRow: {
    display: "flex",
    gap: "0.75rem",
    flexWrap: "wrap",
  },
  infraCard: {
    flex: "1 1 180px",
    border: "1px solid",
    borderRadius: 8,
    padding: "0.75rem 1rem",
    display: "flex",
    flexDirection: "column",
    gap: "0.3rem",
  },
  footer: {
    fontSize: "0.7rem",
    color: "#334155",
    textAlign: "right",
  },
};

