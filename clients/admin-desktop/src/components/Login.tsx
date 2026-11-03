import { useState, type FormEvent } from "react";

interface LoginProps {
  onLogin: (alias: string, password: string) => void;
  loading: boolean;
  error: string | null;
}

export function Login({ onLogin, loading, error }: LoginProps) {
  const [alias, setAlias] = useState("");
  const [password, setPassword] = useState("");

  const handleSubmit = (e: FormEvent) => {
    e.preventDefault();
    if (alias.trim()) onLogin(alias.trim(), password);
  };

  return (
    <div style={styles.overlay}>
      <div style={styles.card}>
        {/* Logo / título */}
        <div style={styles.logoRow}>
          <span style={styles.logoEmoji}>🃏</span>
          <div>
            <h1 style={styles.title}>JokerArena</h1>
            <p style={styles.subtitle}>Panel de Administración</p>
          </div>
        </div>

        <form onSubmit={handleSubmit} style={styles.form}>
          <label style={styles.label}>
            Alias de administrador
            <input
              style={styles.input}
              type="text"
              value={alias}
              onChange={(e) => setAlias(e.target.value)}
              placeholder="admin"
              autoFocus
              disabled={loading}
              required
            />
          </label>

          <label style={styles.label}>
            Contraseña
            <input
              style={styles.input}
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="••••••••"
              disabled={loading}
            />
          </label>

          {error && <p style={styles.error}>⚠ {error}</p>}

          <button style={styles.button} type="submit" disabled={loading || !alias.trim()}>
            {loading ? "Autenticando…" : "Ingresar"}
          </button>
        </form>

        <p style={styles.hint}>
          Conecta al Gateway en{" "}
          <code style={styles.code}>http://localhost:8080</code>
        </p>
      </div>
    </div>
  );
}

const styles: Record<string, React.CSSProperties> = {
  overlay: {
    minHeight: "100vh",
    display: "flex",
    alignItems: "center",
    justifyContent: "center",
    background: "linear-gradient(135deg, #0f0f1a 0%, #1a1a2e 100%)",
  },
  card: {
    background: "#16213e",
    border: "1px solid #0f3460",
    borderRadius: 12,
    padding: "2.5rem",
    width: 360,
    boxShadow: "0 8px 32px rgba(0,0,0,0.6)",
    display: "flex",
    flexDirection: "column",
    gap: "1.5rem",
  },
  logoRow: {
    display: "flex",
    alignItems: "center",
    gap: "1rem",
  },
  logoEmoji: { fontSize: "2.5rem" },
  title: { margin: 0, fontSize: "1.5rem", color: "#e2e8f0" },
  subtitle: { margin: 0, fontSize: "0.8rem", color: "#94a3b8" },
  form: { display: "flex", flexDirection: "column", gap: "1rem" },
  label: { display: "flex", flexDirection: "column", gap: 4, fontSize: "0.85rem", color: "#94a3b8" },
  input: {
    background: "#0f3460",
    border: "1px solid #1e4a8a",
    borderRadius: 6,
    padding: "0.5rem 0.75rem",
    color: "#e2e8f0",
    fontSize: "1rem",
    outline: "none",
  },
  error: { color: "#f87171", fontSize: "0.85rem", margin: 0 },
  button: {
    background: "#e94560",
    color: "#fff",
    border: "none",
    borderRadius: 6,
    padding: "0.75rem",
    fontSize: "1rem",
    cursor: "pointer",
    fontWeight: 600,
    marginTop: 4,
  },
  hint: { fontSize: "0.75rem", color: "#475569", textAlign: "center", margin: 0 },
  code: { color: "#7dd3fc", fontFamily: "monospace" },
};
