#!/usr/bin/env python3
# =============================================================================
# JokerArena P3 · D1, D2 y D5 · Resume los JSON de k6 en tablas y gráficas
# Responsable: José Alejandro Solano Garcés
#
#   python perf/http/resumir.py perf/http/resultados/<fecha>
#   python perf/http/resumir.py perf/http/resultados/<despues> --antes perf/http/resultados/<antes>
#
# Con --antes, además compara dos sesiones (antes y después de un cambio) y
# escribe comparacion.md y graficas/comparacion_*.png en la carpeta <despues>.
#
# Genera en esa carpeta:
#   resumen.csv            una fila por corrida (todas las repeticiones)
#   tablas.md              tablas en Markdown con la mediana de las repeticiones
#   graficas/*.png         gráficas para el documento
# Requiere matplotlib (pip install matplotlib).
# =============================================================================
import csv
import json
import re
import statistics
import sys
from pathlib import Path

NOMBRE = re.compile(r"^(?P<prueba>d[12]-[a-z-]+?)-(?P<modo>cache|sin-cache|jwt|directo-http|proxy-http|proxy-https)-v(?P<vus>\d+)-r(?P<rep>\d+)\.json$")

# Paleta categórica validada (slots 1-3, modo claro) y tinta del texto.
SERIE = ["#2a78d6", "#eb6834", "#1baf7a"]
TXT1, TXT2, GRID, FONDO = "#0b0b0b", "#52514e", "#e4e3df", "#fcfcfb"

ETIQUETAS = {
    "cache": "Ranking con caché",
    "sin-cache": "Ranking sin caché",
    "jwt": "Salas (con JWT)",
    "directo-http": "Gateway directo · HTTP (P2)",
    "proxy-http": "Reverse proxy · HTTP",
    "proxy-https": "Reverse proxy · HTTPS (P3)",
    "antes": "Antes (P2 actual)",
    "despues": "Con keepalive en el gateway",
}


def leer(carpeta: Path):
    manifiesto = {}
    if (carpeta / "manifiesto.csv").exists():
        with open(carpeta / "manifiesto.csv", encoding="utf-8-sig") as fh:
            manifiesto = {r["archivo"]: r for r in csv.DictReader(fh)}
    filas = []
    for f in sorted(carpeta.glob("*.json")):
        m = NOMBRE.match(f.name)
        if not m:
            continue
        met = json.loads(f.read_text(encoding="utf-8-sig"))["metrics"]
        # Latencia solo de las respuestas exitosas: un 503 del gateway por falta de
        # puertos se responde en ~0 ms y bajaría artificialmente los percentiles.
        dur = met.get("http_req_duration{expected_response:true}") or met.get("http_req_duration", {})
        reqs = met.get("http_reqs", {})
        fallidas = met.get("http_req_failed", {})
        hit = met.get("ranking_cache_hit", {})
        tls = met.get("http_req_tls_handshaking", {})
        man = manifiesto.get(f.name, {})
        filas.append({
            "archivo": f.name,
            "prueba": m["prueba"],
            "modo": m["modo"],
            "vus": int(m["vus"]),
            "rep": int(m["rep"]),
            "peticiones": int(reqs.get("count", 0)),
            "rps": round(reqs.get("rate", 0.0), 1),
            "rps_ok": round(reqs.get("rate", 0.0) * (1 - fallidas.get("value", 0.0)), 1),
            "time_wait_inicio": man.get("time_wait_inicio", ""),
            "time_wait_fin": man.get("time_wait_fin", ""),
            "p50_ms": round(dur.get("med", 0.0), 2),
            "p95_ms": round(dur.get("p(95)", 0.0), 2),
            "p99_ms": round(dur.get("p(99)", 0.0), 2),
            "max_ms": round(dur.get("max", 0.0), 2),
            "errores_pct": round(100 * fallidas.get("value", 0.0), 3),
            "p95_todas_ms": round(met.get("http_req_duration", {}).get("p(95)", 0.0), 2),
            "cache_hit_pct": round(100 * hit["value"], 1) if "value" in hit else "",
            "tls_handshake_avg_ms": round(tls.get("avg", 0.0), 3) if tls else "",
        })
    return filas


def medianas(filas):
    grupos = {}
    for f in filas:
        grupos.setdefault((f["prueba"], f["modo"], f["vus"]), []).append(f)
    out = {}
    for clave, fs in grupos.items():
        out[clave] = {
            k: round(statistics.median(x[k] for x in fs), 2)
            for k in ("rps", "rps_ok", "p50_ms", "p95_ms", "p99_ms", "errores_pct", "peticiones")
        }
        hits = [x["cache_hit_pct"] for x in fs if x["cache_hit_pct"] != ""]
        out[clave]["cache_hit_pct"] = round(statistics.median(hits), 1) if hits else ""
        tls = [x["tls_handshake_avg_ms"] for x in fs if x["tls_handshake_avg_ms"] != ""]
        out[clave]["tls_handshake_avg_ms"] = round(statistics.median(tls), 3) if tls else ""
        out[clave]["reps"] = len(fs)
    return out


def num(x, dec=1):
    if x == "":
        return "—"
    s = f"{x:,.{dec}f}"
    return s.replace(",", "X").replace(".", ",").replace("X", ".")  # formato es-CO


def tabla(med, prueba, modos, cache=False):
    cab = "| Prueba | Usuarios (VU) | Peticiones/s | Exitosas/s | p50 (ms) | p95 (ms) | p99 (ms) | Errores (%) |"
    sep = "|---|---:|---:|---:|---:|---:|---:|---:|"
    if cache:
        cab += " Desde caché (%) |"
        sep += "---:|"
    lineas = [cab, sep]
    for modo in modos:
        for (p, mo, v), r in sorted(med.items(), key=lambda kv: kv[0][2]):
            if p != prueba or mo != modo:
                continue
            fila = f"| {ETIQUETAS[mo]} | {v} | {num(r['rps'])} | {num(r['rps_ok'])} | {num(r['p50_ms'],2)} | {num(r['p95_ms'],2)} | {num(r['p99_ms'],2)} | {num(r['errores_pct'],2)} |"
            if cache:
                fila += f" {num(r['cache_hit_pct'])} |"
            lineas.append(fila)
    return "\n".join(lineas)


def tabla_tls(med, prueba):
    lineas = [
        "| Usuarios (VU) | p50 HTTP directo | p50 proxy HTTP | p50 proxy HTTPS | p95 HTTP directo | p95 proxy HTTP | p95 proxy HTTPS | Costo TLS en p50 | Costo TLS en p95 | Peticiones/s HTTP → HTTPS |",
        "|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|",
    ]
    for v in sorted({k[2] for k in med if k[0] == prueba}):
        d, h, s = (med.get((prueba, m, v)) for m in ("directo-http", "proxy-http", "proxy-https"))
        if not (d and h and s):
            continue
        c50 = s["p50_ms"] - h["p50_ms"]
        c95 = s["p95_ms"] - h["p95_ms"]
        rps = 100 * (s["rps"] - h["rps"]) / h["rps"] if h["rps"] else 0
        lineas.append(
            f"| {v} | {num(d['p50_ms'],2)} | {num(h['p50_ms'],2)} | {num(s['p50_ms'],2)} | {num(d['p95_ms'],2)} | {num(h['p95_ms'],2)} | {num(s['p95_ms'],2)} "
            f"| {'+' if c50 >= 0 else ''}{num(c50,2)} ms | {'+' if c95 >= 0 else ''}{num(c95,2)} ms | {num(h['rps'])} → {num(s['rps'])} ({'+' if rps >= 0 else ''}{num(rps,1)} %) |"
        )
    return "\n".join(lineas)


def tabla_noreuse(filas):
    lineas = [
        "| Usuarios (VU) | Modo | Peticiones/s | p50 (ms) | p95 (ms) | p99 (ms) | Handshake TLS promedio (ms) |",
        "|---:|---|---:|---:|---:|---:|---:|",
    ]
    for f in sorted((f for f in filas if f["prueba"] == "d2-ranking-noreuse"), key=lambda f: (f["vus"], f["modo"])):
        lineas.append(
            f"| {f['vus']} | {ETIQUETAS[f['modo']]} | {num(f['rps'])} | {num(f['p50_ms'],2)} | {num(f['p95_ms'],2)} | {num(f['p99_ms'],2)} | {num(f['tls_handshake_avg_ms'],2) if f['tls_handshake_avg_ms'] != '' else '—'} |"
        )
    return "\n".join(lineas)


def tabla_login(carpeta: Path):
    f = carpeta / "d1-login-limite-rate40.json"
    if not f.exists():
        return "(no se corrió la prueba del límite del login)"
    met = json.loads(f.read_text(encoding="utf-8-sig"))["metrics"]
    ok = int(met.get("login_aceptado_200", {}).get("count", 0))
    rech = int(met.get("login_rechazado_503", {}).get("count", 0))
    otro = int(met.get("login_otro_codigo", {}).get("count", 0))
    tot = ok + rech + otro
    dur = met.get("http_req_duration", {})
    return "\n".join([
        "| Peticiones enviadas (40/s × 20 s) | Aceptadas (200) | Rechazadas por el límite (503) | Otros códigos | Esperadas aceptadas (20 + 10/s × 20 s) | p95 (ms) |",
        "|---:|---:|---:|---:|---:|---:|",
        f"| {tot} | {ok} ({num(100*ok/tot if tot else 0)} %) | {rech} ({num(100*rech/tot if tot else 0)} %) | {otro} | 220 | {num(dur.get('p(95)',0),2)} |",
    ])


def grafica(med, series, metrica, titulo, eje_y, archivo, nota=None):
    import matplotlib
    matplotlib.use("Agg")
    import matplotlib.pyplot as plt

    plt.rcParams.update({"font.family": "DejaVu Sans", "font.size": 10})
    fig, ax = plt.subplots(figsize=(7.2, 4.2), dpi=160)
    fig.patch.set_facecolor(FONDO)
    ax.set_facecolor(FONDO)
    finales = []
    for i, (prueba, modo) in enumerate(series):
        pts = sorted((v, r[metrica]) for (p, m, v), r in med.items() if p == prueba and m == modo)
        if not pts:
            continue
        xs, ys = zip(*pts)
        ax.plot(xs, ys, color=SERIE[i], linewidth=2, marker="o", markersize=6,
                markeredgecolor=FONDO, markeredgewidth=2, label=ETIQUETAS[modo], zorder=3)
        finales.append((xs[-1], ys[-1], num(ys[-1], 0 if metrica.startswith("rps") else 1)))
    ax.set_title(titulo, loc="left", fontsize=12, color=TXT1, fontweight="bold", pad=26)
    ax.set_xlabel("Usuarios virtuales simultáneos (VU)", color=TXT2)
    ax.set_ylabel(eje_y, color=TXT2)
    ax.set_xticks(sorted({k[2] for k in med}))
    ax.set_ylim(bottom=0)
    ax.grid(axis="y", color=GRID, linewidth=0.8)
    ax.tick_params(colors=TXT2, length=0)
    for s in ax.spines.values():
        s.set_visible(False)
    ax.spines["bottom"].set_visible(True)
    ax.spines["bottom"].set_color(GRID)
    if ax.get_legend_handles_labels()[0]:
        # Leyenda en una fila sobre el área de datos, para que no tape ninguna línea
        ax.legend(frameon=False, loc="lower left", bbox_to_anchor=(0, 1.0), ncol=3, fontsize=8.5,
                  labelcolor=TXT1, borderaxespad=0.2, handlelength=1.6, columnspacing=1.2)
    ax.set_xlim(right=max(k[2] for k in med) * 1.1)
    # Etiqueta directa solo en el último punto; si dos quedan muy cerca se separan
    alto = ax.get_ylim()[1]
    hueco = alto * 0.055
    finales.sort(key=lambda t: t[1])
    ys_txt = []
    for _, y, _ in finales:
        ys_txt.append(max(y, ys_txt[-1] + hueco) if ys_txt else y)
    for (x, _, txt), yt in zip(finales, ys_txt):
        ax.annotate(txt, (x, yt), textcoords="offset points", xytext=(6, 0), va="center", fontsize=8.5,
                    color=TXT2, annotation_clip=False)
    if nota:
        fig.text(0.01, 0.01, nota, fontsize=7.5, color=TXT2)
    fig.tight_layout(rect=(0, 0.03, 1, 1))
    fig.savefig(archivo, facecolor=FONDO)
    plt.close(fig)


def comparar(med_a, med_d, carpeta: Path):
    """Tabla y gráficas antes/después para el ranking con caché (y el resto de D1 como control)."""
    lineas = [
        "| Prueba | Usuarios (VU) | Exitosas/s antes | Exitosas/s después | p95 antes (ms) | p95 después (ms) | Errores antes (%) | Errores después (%) |",
        "|---|---:|---:|---:|---:|---:|---:|---:|",
    ]
    for prueba, modo in (("d1-ranking", "cache"), ("d1-ranking", "sin-cache"), ("d1-salas", "jwt")):
        for v in sorted({k[2] for k in med_d if k[0] == prueba and k[1] == modo}):
            a, d = med_a.get((prueba, modo, v)), med_d.get((prueba, modo, v))
            if not (a and d):
                continue
            lineas.append(
                f"| {ETIQUETAS[modo]} | {v} | {num(a['rps_ok'])} | {num(d['rps_ok'])} | {num(a['p95_ms'],2)} | {num(d['p95_ms'],2)} "
                f"| {num(a['errores_pct'],2)} | {num(d['errores_pct'],2)} |"
            )
    (carpeta / "comparacion.md").write_text(
        "# Antes y después de la corrección del gateway (mediana de las repeticiones)\n\n" + "\n".join(lineas) + "\n",
        encoding="utf-8")

    sinte = {}
    for (p, m, v), r in med_a.items():
        if (p, m) == ("d1-ranking", "cache"):
            sinte[("cmp", "antes", v)] = r
    for (p, m, v), r in med_d.items():
        if (p, m) == ("d1-ranking", "cache"):
            sinte[("cmp", "despues", v)] = r
    g = carpeta / "graficas"
    g.mkdir(exist_ok=True)
    nota = "Ranking con caché. Mediana de 3 repeticiones de 30 s por nivel."
    serie = [("cmp", "antes"), ("cmp", "despues")]
    grafica(sinte, serie, "rps_ok", "Ranking con caché · peticiones exitosas por segundo",
            "Peticiones exitosas por segundo", g / "comparacion_rps.png", nota)
    grafica(sinte, serie, "errores_pct", "Ranking con caché · errores (503 del gateway)",
            "Errores (%)", g / "comparacion_errores.png", nota)


def main():
    args = sys.argv[1:]
    antes = None
    if "--antes" in args:
        i = args.index("--antes")
        antes = Path(args[i + 1])
        del args[i:i + 2]
    if len(args) != 1:
        sys.exit("Uso: python perf/http/resumir.py perf/http/resultados/<fecha> [--antes perf/http/resultados/<otra>]")
    carpeta = Path(args[0])
    filas = leer(carpeta)
    if not filas:
        sys.exit(f"No hay resultados de k6 en {carpeta}")

    with open(carpeta / "resumen.csv", "w", newline="", encoding="utf-8") as fh:
        w = csv.DictWriter(fh, fieldnames=list(filas[0].keys()))
        w.writeheader()
        w.writerows(filas)

    med = medianas(filas)
    md = [
        "# Resultados (mediana de las repeticiones)\n",
        "## D1 · Ranking con y sin caché\n", tabla(med, "d1-ranking", ["cache", "sin-cache"], cache=True),
        "\n## D1 · Salas (ruta protegida con JWT)\n", tabla(med, "d1-salas", ["jwt"]),
        "\n## D1 · Límite del login\n", tabla_login(carpeta),
        "\n## D2 · Costo de TLS · ranking (con caché)\n", tabla_tls(med, "d2-ranking"),
        "\n## D2 · Costo de TLS · salas\n", tabla_tls(med, "d2-salas"),
        "\n## D2 · Peor caso: una conexión nueva por petición (ranking)\n", tabla_noreuse(filas),
    ]
    (carpeta / "tablas.md").write_text("\n".join(md) + "\n", encoding="utf-8")

    g = carpeta / "graficas"
    g.mkdir(exist_ok=True)
    d1 = [("d1-ranking", "cache"), ("d1-ranking", "sin-cache"), ("d1-salas", "jwt")]
    nota = "Mediana de 3 repeticiones de 30 s por nivel. Carga cerrada: cada VU repite sin pausa."
    grafica(med, d1, "rps_ok", "D1 · Peticiones exitosas por segundo", "Peticiones exitosas por segundo", g / "d1_rps.png", nota)
    grafica(med, d1, "p95_ms", "D1 · Latencia p95", "Latencia p95 (ms)", g / "d1_p95.png", nota)
    grafica(med, d1, "p99_ms", "D1 · Latencia p99", "Latencia p99 (ms)", g / "d1_p99.png", nota)
    nota2 = "Mediana de 2 repeticiones de 30 s por nivel, modos intercalados."
    tls = [("d2-ranking", "directo-http"), ("d2-ranking", "proxy-http"), ("d2-ranking", "proxy-https")]
    grafica(med, tls, "p95_ms", "D2 · Costo de TLS en el ranking · latencia p95", "Latencia p95 (ms)", g / "d2_ranking_p95.png", nota2)
    grafica(med, tls, "rps_ok", "D2 · Costo de TLS en el ranking · peticiones exitosas por segundo", "Peticiones exitosas por segundo", g / "d2_ranking_rps.png", nota2)
    tls_s = [("d2-salas", "directo-http"), ("d2-salas", "proxy-http"), ("d2-salas", "proxy-https")]
    grafica(med, tls_s, "p95_ms", "D2 · Costo de TLS en salas · latencia p95", "Latencia p95 (ms)", g / "d2_salas_p95.png", nota2)
    print(f"{len(filas)} corridas -> {carpeta / 'resumen.csv'}, {carpeta / 'tablas.md'}, {g}")
    if antes:
        comparar(medianas(leer(antes)), med, carpeta)
        print(f"Comparacion con {antes} -> {carpeta / 'comparacion.md'}")


if __name__ == "__main__":
    main()
