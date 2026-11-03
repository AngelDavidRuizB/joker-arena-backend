/**
 * LiveOpsEditor — placeholder tab.
 *
 * Esta vista será desarrollada por Jaisson Fabian Patiño Orjuela (Task 25).
 * Se conecta a: GET/PUT http://localhost:8080/admin/liveops/ruleset
 *
 * Para integrar: agregar el componente real aquí y recibir el token como prop.
 */
interface LiveOpsEditorProps {
  token: string;
}

export function LiveOpsEditor({ token: _token }: LiveOpsEditorProps) {
  return (
    <div style={styles.container}>
      <div style={styles.card}>
        <h2 style={styles.title}>✏️ Editor de Reglas LiveOps</h2>
        <p style={styles.description}>
          Esta sección está reservada para el <strong>Editor de Reglas en Caliente</strong>,
          desarrollado por <strong>Jaisson Fabian Patiño Orjuela</strong> (Task 25).
        </p>
        <div style={styles.infoBox}>
          <p style={styles.infoTitle}>Endpoints a integrar:</p>
          <ul style={styles.list}>
            <li>
              <code style={styles.code}>GET http://localhost:8080/admin/liveops/ruleset</code>
              <br />
              <span style={styles.hint}>Obtiene el Ruleset activo (jokers, multiplicadores, etc.)</span>
            </li>
            <li>
              <code style={styles.code}>PUT http://localhost:8080/admin/liveops/ruleset</code>
              <br />
              <span style={styles.hint}>Publica el nuevo Ruleset (efecto inmediato en partidas activas)</span>
            </li>
          </ul>
        </div>
        <p style={styles.soon}>🚧 Próximamente — Jaisson</p>
      </div>
    </div>
  );
}

const styles: Record<string, React.CSSProperties> = {
  container: {
    display: "flex",
    alignItems: "center",
    justifyContent: "center",
    minHeight: 400,
    padding: "2rem",
  },
  card: {
    background: "#16213e",
    border: "1px solid #0f3460",
    borderRadius: 12,
    padding: "2rem",
    maxWidth: 560,
    width: "100%",
    display: "flex",
    flexDirection: "column",
    gap: "1rem",
  },
  title: { margin: 0, fontSize: "1.2rem", color: "#e2e8f0" },
  description: { margin: 0, color: "#94a3b8", fontSize: "0.9rem", lineHeight: 1.6 },
  infoBox: {
    background: "#0f172a",
    border: "1px solid #1e293b",
    borderRadius: 8,
    padding: "1rem",
  },
  infoTitle: { margin: "0 0 0.5rem", fontSize: "0.8rem", color: "#64748b", textTransform: "uppercase" as const, letterSpacing: "0.05em" },
  list: { margin: 0, paddingLeft: "1.25rem", display: "flex", flexDirection: "column" as const, gap: "0.75rem" },
  code: { color: "#7dd3fc", fontFamily: "monospace", fontSize: "0.8rem" },
  hint: { color: "#475569", fontSize: "0.75rem" },
  soon: {
    textAlign: "center" as const,
    color: "#854d0e",
    background: "#422006",
    borderRadius: 6,
    padding: "0.5rem",
    fontSize: "0.85rem",
    fontWeight: 600,
  },
};
