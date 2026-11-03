import type { Container } from "../types/snapshot";

interface ContainerCardProps {
  container: Container;
}

const LIGHT_COLORS: Record<string, { bg: string; border: string; dot: string }> = {
  green:  { bg: "#052e16", border: "#166534", dot: "#4ade80" },
  yellow: { bg: "#422006", border: "#92400e", dot: "#fbbf24" },
  red:    { bg: "#2d0a0a", border: "#7f1d1d", dot: "#f87171" },
};

function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 ** 2) return `${(bytes / 1024).toFixed(0)} KB`;
  if (bytes < 1024 ** 3) return `${(bytes / 1024 ** 2).toFixed(1)} MB`;
  return `${(bytes / 1024 ** 3).toFixed(2)} GB`;
}

export function ContainerCard({ container }: ContainerCardProps) {
  const light = container.light ?? "yellow";
  const colors = LIGHT_COLORS[light] ?? LIGHT_COLORS.yellow;

  return (
    <div
      style={{
        background: colors.bg,
        border: `1px solid ${colors.border}`,
        borderRadius: 8,
        padding: "0.75rem 1rem",
        display: "flex",
        flexDirection: "column",
        gap: "0.4rem",
        minWidth: 180,
        flex: "1 1 180px",
      }}
    >
      {/* Header row */}
      <div style={{ display: "flex", alignItems: "center", gap: "0.5rem" }}>
        <span
          title={light}
          style={{
            width: 10,
            height: 10,
            borderRadius: "50%",
            background: colors.dot,
            flexShrink: 0,
            boxShadow: `0 0 6px ${colors.dot}`,
          }}
        />
        <span
          style={{
            fontWeight: 700,
            fontSize: "0.8rem",
            color: "#e2e8f0",
            overflow: "hidden",
            textOverflow: "ellipsis",
            whiteSpace: "nowrap",
          }}
          title={container.name}
        >
          {container.service}
        </span>
      </div>

      {/* Metrics */}
      <MetricRow label="CPU" value={`${container.cpu_percent.toFixed(1)} %`} />
      <MetricRow
        label="RAM"
        value={`${formatBytes(container.mem_bytes)} / ${formatBytes(container.mem_limit_bytes)}`}
      />
      <MetricRow label="Estado" value={container.state} />
      <MetricRow label="Health" value={container.health} />
    </div>
  );
}

function MetricRow({ label, value }: { label: string; value: string }) {
  return (
    <div style={{ display: "flex", justifyContent: "space-between", fontSize: "0.72rem" }}>
      <span style={{ color: "#64748b" }}>{label}</span>
      <span style={{ color: "#cbd5e1", fontFamily: "monospace" }}>{value}</span>
    </div>
  );
}
