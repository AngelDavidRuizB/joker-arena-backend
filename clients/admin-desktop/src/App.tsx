import { useState } from "react";
import { useGateway } from "./hooks/useGateway";
import { useClusterWS } from "./hooks/useClusterWS";
import { Login } from "./components/Login";
import { Radar } from "./components/Radar";
import { LiveOpsEditor } from "./components/LiveOpsEditor";

type Tab = "radar" | "liveops";

export function App() {
  const { token, alias, role, error, loading, login, logout } = useGateway();
  const { snapshot, status, lastUpdated, reconnect } = useClusterWS(token);
  const [tab, setTab] = useState<Tab>("radar");
  const [demoMode, setDemoMode] = useState(false);

  if (!token) {
    return <Login onLogin={login} loading={loading} error={error} />;
  }

  return (
    <div style={styles.shell}>
      {/* Top navigation */}
      <nav style={styles.nav}>
        <div style={styles.navLeft}>
          <span style={styles.navLogo}>🃏</span>
          <span style={styles.navTitle}>JokerArena Admin</span>
        </div>

        <div style={styles.tabRow}>
          <TabButton active={tab === "radar"}   onClick={() => setTab("radar")}>
            📡 Radar NOC
          </TabButton>
          <TabButton active={tab === "liveops"} onClick={() => setTab("liveops")}>
            ✏️ Editor de Reglas
          </TabButton>
        </div>

        <div style={styles.navRight}>
          {/* Demo mode toggle */}
          <label style={styles.demoLabel}>
            <input
              type="checkbox"
              checked={demoMode}
              onChange={(e) => setDemoMode(e.target.checked)}
              style={{ cursor: "pointer" }}
            />
            &nbsp;Demo
          </label>

          {/* User info */}
          <span style={styles.userBadge}>
            {role === "admin" ? "🛡" : "👤"} {alias}
          </span>
          <button onClick={logout} style={styles.logoutBtn} title="Cerrar sesión">
            Salir
          </button>
        </div>
      </nav>

      {/* Content */}
      <main style={styles.main}>
        {tab === "radar" && (
          <Radar
            snapshot={snapshot}
            status={status}
            lastUpdated={lastUpdated}
            onReconnect={reconnect}
            demoMode={demoMode}
          />
        )}
        {tab === "liveops" && <LiveOpsEditor token={token} />}
      </main>
    </div>
  );
}

function TabButton({
  active,
  onClick,
  children,
}: {
  active: boolean;
  onClick: () => void;
  children: React.ReactNode;
}) {
  return (
    <button
      onClick={onClick}
      style={{
        background: active ? "#1e3a5f" : "transparent",
        color: active ? "#e2e8f0" : "#64748b",
        border: "none",
        borderBottom: active ? "2px solid #3b82f6" : "2px solid transparent",
        padding: "0.5rem 1rem",
        cursor: "pointer",
        fontSize: "0.85rem",
        fontWeight: active ? 600 : 400,
        transition: "all 0.15s",
      }}
    >
      {children}
    </button>
  );
}

const styles: Record<string, React.CSSProperties> = {
  shell: {
    minHeight: "100vh",
    background: "#0f0f1a",
    display: "flex",
    flexDirection: "column",
    color: "#e2e8f0",
    fontFamily: "'Inter', 'Segoe UI', sans-serif",
  },
  nav: {
    display: "flex",
    alignItems: "center",
    background: "#16213e",
    borderBottom: "1px solid #0f3460",
    padding: "0 1rem",
    height: 48,
    flexShrink: 0,
    gap: "1rem",
  },
  navLeft: { display: "flex", alignItems: "center", gap: "0.5rem", flex: "0 0 auto" },
  navLogo: { fontSize: "1.4rem" },
  navTitle: { fontWeight: 700, fontSize: "0.9rem", whiteSpace: "nowrap" },
  tabRow: { display: "flex", flex: 1, justifyContent: "center", gap: 4 },
  navRight: { display: "flex", alignItems: "center", gap: "0.75rem", flex: "0 0 auto" },
  demoLabel: { fontSize: "0.75rem", color: "#94a3b8", display: "flex", alignItems: "center", cursor: "pointer" },
  userBadge: { fontSize: "0.8rem", color: "#94a3b8", whiteSpace: "nowrap" },
  logoutBtn: {
    background: "transparent",
    color: "#ef4444",
    border: "1px solid #7f1d1d",
    borderRadius: 4,
    padding: "2px 10px",
    cursor: "pointer",
    fontSize: "0.75rem",
  },
  main: { flex: 1, overflow: "auto" },
};
