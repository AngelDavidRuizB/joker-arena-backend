"""Genera los diagramas C&C de JokerArena fraccionados por funcionalidad (SVG)."""
import os

OUT = os.path.dirname(os.path.abspath(__file__))
COL = {"fe": ("#dbeafe", "#2a5d9f"), "svc": ("#dcfce7", "#2b7a2b"), "db": ("#ffedd5", "#a86a12")}
FE_X, MID_X, DB_X = 40, 520, 1090
FE_W, MID_W, DB_W = 300, 380, 250
TOP = 110


class View:
    def __init__(self, name, title, subtitle, height):
        self.name, self.title, self.subtitle, self.h = name, title, subtitle, height
        self.s, self.comps = [], {}

    def comp(self, cid, x, y, w, kind, title, sub, lines, ports, h=None, stereotype="component"):
        h = h or (125 + 18 * len(lines) + 20)
        fill, stroke = COL[kind]
        c = {"x": x, "y": y, "w": w, "h": h, "ports": {}}
        self.comps[cid] = c
        cx = x + w / 2
        a = self.s.append
        a(f'<rect x="{x}" y="{y}" width="{w}" height="{h}" rx="8" fill="{fill}" stroke="{stroke}" stroke-width="2"/>')
        a(f'<text x="{cx}" y="{y+22}" text-anchor="middle" font-size="13">«{stereotype}»</text>')
        a(f'<text x="{cx}" y="{y+42}" text-anchor="middle" font-size="16" font-weight="bold">{title}</text>')
        a(f'<text x="{cx}" y="{y+59}" text-anchor="middle" font-size="12">{sub}</text>')
        for i, t in enumerate(lines):
            a(f'<text x="{x+14}" y="{y+135+18*i}" font-size="12">{t}</text>')
        for name, side, off in ports:
            if side == "L":
                px, py = x, y + off
                lab = f'<text x="{x+14}" y="{py+4}" font-size="11">{name}</text>'
            elif side == "R":
                px, py = x + w, y + off
                lab = f'<text x="{x+w-14}" y="{py+4}" text-anchor="end" font-size="11">{name}</text>'
            elif side == "B":
                px, py = x + w / 2, y + h
                lab = f'<text x="{px+14}" y="{py-8}" font-size="11">{name}</text>'
            else:  # T
                px, py = x + w / 2, y
                lab = f'<text x="{px+14}" y="{py+18}" font-size="11">{name}</text>'
            a(f'<rect x="{px-8}" y="{py-8}" width="16" height="16" fill="#fff" stroke="#222" stroke-width="2"/>')
            a(lab)
            c["ports"][name] = (px, py, side)

    def pt(self, cid, port):
        return self.comps[cid]["ports"][port]

    def conn(self, src, dst, label, sub, bidir=False, dashed=False, via=None, lab_pos=None):
        (x1, y1, s1), (x2, y2, s2) = self.pt(*src), self.pt(*dst)
        off = lambda x, y, s, d: (x + 8 * d if s in "LR" else x, y + 8 * d if s in "TB" else y)
        d1 = 1 if s1 in "RB" else -1
        d2 = 1 if s2 in "RB" else -1
        p1, p2 = off(x1, y1, s1, d1), off(x2, y2, s2, d2)
        pts = [p1] + (via or []) + [p2]
        dash = ' stroke-dasharray="8 4"' if dashed else ""
        ms = ' marker-start="url(#arrS)"' if bidir else ""
        self.s.append(f'<polyline points="{" ".join(f"{a},{b}" for a, b in pts)}" fill="none" stroke="#222" stroke-width="2"{dash}{ms} marker-end="url(#arr)"/>')
        lx, ly = (lab_pos or ((p1[0] + p2[0]) / 2, min(p1[1], p2[1]) - 16))[:2]
        anchor = "middle" if not lab_pos or len(lab_pos) < 3 else lab_pos[2]
        self.s.append(f'<text x="{lx}" y="{ly}" text-anchor="{anchor}" font-size="12" font-weight="bold" fill="#b00020">{label}</text>')
        self.s.append(f'<text x="{lx}" y="{ly+14}" text-anchor="{anchor}" font-size="11" fill="#444">{sub}</text>')

    def note(self, x, y, w, lines):
        h = 14 + 16 * len(lines)
        self.s.append(f'<rect x="{x}" y="{y}" width="{w}" height="{h}" fill="#fffde7" stroke="#bba800"/>')
        for i, t in enumerate(lines):
            bold = ' font-weight="bold"' if i == 0 else ""
            self.s.append(f'<text x="{x+10}" y="{y+19+16*i}" font-size="11.5"{bold}>{t}</text>')

    def save(self):
        head = f'''<svg xmlns="http://www.w3.org/2000/svg" width="1400" height="{self.h}" viewBox="0 0 1400 {self.h}" font-family="Segoe UI, Arial, sans-serif">
<defs>
<marker id="arr" markerWidth="10" markerHeight="8" refX="9" refY="4" orient="auto"><path d="M0,0 L10,4 L0,8 z" fill="#222"/></marker>
<marker id="arrS" markerWidth="10" markerHeight="8" refX="1" refY="4" orient="auto-start-reverse"><path d="M0,0 L10,4 L0,8 z" fill="#222"/></marker>
</defs>
<rect width="1400" height="{self.h}" fill="#fff"/>
<text x="700" y="32" text-anchor="middle" font-size="22" font-weight="bold">{self.title}</text>
<text x="700" y="56" text-anchor="middle" font-size="13" fill="#555">{self.subtitle}</text>
<g transform="translate(40,{self.h-40})" font-size="11"><rect x="0" y="-9" width="10" height="10" fill="#fff" stroke="#222" stroke-width="2"/><text x="16" y="0">Puerto</text>
<line x1="70" y1="-4" x2="110" y2="-4" stroke="#222" stroke-width="2" marker-end="url(#arr)"/><text x="116" y="0">Conector</text>
<line x1="190" y1="-4" x2="230" y2="-4" stroke="#222" stroke-width="2" stroke-dasharray="8 4" marker-end="url(#arr)"/><text x="236" y="0">Conector asíncrono / streaming</text></g>
'''
        path = os.path.join(OUT, f"JokerArena_CyC_{self.name}.svg")
        with open(path, "w", encoding="utf-8") as f:
            f.write(head + "\n".join(self.s) + "\n</svg>\n")
        print(path)


# ---------------------------------------------------------------- 1. Salas
v = View("1_Gestion_Salas", "Funcionalidad 1 — Gestión de salas y jugadores",
         "Creación/unión a salas (código alfanumérico, 2 a 4 jugadores), modo Bot y registro de jugadores/partidas", 640)
v.comp("fe", FE_X, TOP, FE_W, "fe", "Web Frontend Client", "TypeScript / React (:3000)",
       ["• components/Lobby (crear / unirse a sala)", "• services/api (cliente REST)", "• hooks/ (conexión WebSocket)",
        "• state/ y types/ (ROOM_STATE_UPDATE)"], [("p_rest_out", "R", 60), ("p_ws_io", "R", 100)])
v.comp("ge", MID_X, TOP, MID_W, "svc", "Game Engine &amp; Session Manager", "Go (:8080)",
       ["• internal/api — handlers REST (salas, /health)", "• internal/ws — conexiones y broadcast de sala",
        "• internal/room — Session Manager (goroutine/sala,", "   código único, 2-4 jugadores, modo Bot)", "• internal/store — acceso a PostgreSQL (pgx)"],
       [("p_rest_srv", "L", 60), ("p_ws_srv", "L", 100), ("p_sql_out", "R", 60)])
v.comp("pg", DB_X, TOP, DB_W, "db", "PostgreSQL", "SQL relacional ACID (:5432)",
       ["• players (cuentas, total_victories)", "• matches (registro de partidas)"], [("p_sql_in", "L", 60)], stereotype="datastore")
v.conn(("fe", "p_rest_out"), ("ge", "p_rest_srv"), "C1 · HTTP REST (JSON)", "crear / consultar / unirse a sala")
v.conn(("fe", "p_ws_io"), ("ge", "p_ws_srv"), "C2 · WebSocket", "ROOM_STATE_UPDATE a la sala", bidir=True)
v.conn(("ge", "p_sql_out"), ("pg", "p_sql_in"), "C5 · TCP nativo PostgreSQL", "pool pgx: players, matches")
v.note(40, 380, 860, ["Flujo (pasos 1-2 de 4.5)", "1. El jugador crea la sala con «Simular Oponente (Bot)» por C1 y recibe su código alfanumérico.",
                      "2. El cliente abre el WebSocket (C2); el Session Manager registra al jugador y difunde el estado de la sala.",
                      "3. Al concluir la partida, el Game Engine registra el resultado en matches y actualiza total_victories (C5)."])
v.save()

# ---------------------------------------------------------------- 2. Core loop
v = View("2_Core_Game_Loop", "Funcionalidad 2 — Core Game Loop (motor de juego)",
         "Baraja de 52 cartas, selección/descarte de hasta 5 cartas, reconocimiento de manos y puntaje Chips × Mult (servidor autoritativo)", 680)
v.comp("fe", FE_X, TOP, FE_W, "fe", "Web Frontend Client", "TypeScript / React (:3000)",
       ["• components/ (Tapete, Mano, Cartas)", "• botones «Jugar Mano» / «Descartar»", "• hooks/ (conexión WebSocket)",
        "• state/ y types/ (PLAY_HAND,", "   DISCARD_CARDS, ROOM_STATE_UPDATE)"], [("p_ws_io", "R", 70)])
v.comp("ge", MID_X, TOP, MID_W, "svc", "Game Engine &amp; Session Manager", "Go (:8080) — servidor autoritativo",
       ["• internal/ws — recepción de acciones y broadcast", "• internal/room — estado de ronda, descartes y manos",
        "• internal/deck — generación, barajado y reparto", "• internal/evaluator — reconocimiento de manos",
        "• internal/scoring — pipeline Chips × Mult"], [("p_ws_srv", "L", 70)])
v.conn(("fe", "p_ws_io"), ("ge", "p_ws_srv"), "C2 · WebSocket (RFC 6455)", "PLAY_HAND / DISCARD_CARDS  ⇄  ROOM_STATE_UPDATE", bidir=True)
v.note(40, 400, 860, ["Flujo (pasos 3-5 de 4.5)", "1. El jugador selecciona hasta 5 cartas y envía PLAY_HAND o DISCARD_CARDS por C2.",
                      "2. El Game Engine valida la acción, reconoce la mano (Carta Alta … Escalera Real) y calcula el puntaje.",
                      "3. Difunde ROOM_STATE_UPDATE a todos los jugadores; el cliente solo renderiza (no valida reglas).",
                      "Puntaje = Fichas Totales × Multiplicador Total"])
v.save()

# ---------------------------------------------------------------- 3. Jokers
v = View("3_Mecanica_Jokers", "Funcionalidad 3 — Mecánica de modificadores (Jokers)",
         "Cascada: (Chips Base + Chips Jokers) × (Mult Base + Mult Jokers) × Π(Mult Exponenciales)", 700)
v.comp("ge", MID_X, TOP, MID_W, "svc", "Game Engine &amp; Session Manager", "Go (:8080)",
       ["• internal/evaluator — mano detectada (p. ej. Par)", "• internal/scoring — pipeline de puntuación",
        "• internal/jokers — cascada de modificadores:", "    Joker Clásico: +4 Mult (FLAT_MULT)",
        "    Banner: +40 Chips x descarte (FLAT_CHIPS)", "    The Duo: ×2 Mult si hay Par (EXP_MULT)", "• internal/store — lectura del catálogo"],
       [("p_ws_srv", "L", 70), ("p_sql_out", "R", 70)])
v.comp("fe", FE_X, TOP, FE_W, "fe", "Web Frontend Client", "TypeScript / React (:3000)",
       ["• components/ (Tapete: animación ×Mult)", "• hooks/ (WebSocket)", "• state/ (ROOM_STATE_UPDATE)"], [("p_ws_io", "R", 70)])
v.comp("pg", DB_X, TOP, DB_W, "db", "PostgreSQL", "SQL relacional ACID (:5432)",
       ["• jokers (id, name, modifier_type,", "   default_value) — catálogo estático"], [("p_sql_in", "L", 70)], stereotype="datastore")
v.conn(("fe", "p_ws_io"), ("ge", "p_ws_srv"), "C2 · WebSocket", "jugada ⇄ puntaje con Jokers aplicados", bidir=True)
v.conn(("ge", "p_sql_out"), ("pg", "p_sql_in"), "C5 · TCP nativo PostgreSQL", "consulta del catálogo jokers")
v.note(40, 420, 860, ["Ejemplo verificado (sección 6.3)", "Chips: 40 base + 38 de cartas = 78   |   Mult: 4 base + 4 (Joker Clásico) = 8, × 2 (The Duo, hay Par) = 16",
                      "Puntaje de la mano = 78 × 16 = 1248"])
v.save()

# ---------------------------------------------------------------- 4. Telemetría
v = View("4_Telemetria_Auditoria", "Funcionalidad 4 — Consola de telemetría y auditoría en vivo",
         "Despacho asíncrono de jugadas, Audit Trail inmutable en MongoDB y streaming SSE del desglose Chips × Mult", 860)
v.comp("fe", FE_X, TOP, FE_W, "fe", "Web Frontend Client", "TypeScript / React (:3000)",
       ["• components/TelemetryPanel", "   (desglose Fichas × Multiplicador)", "• hooks/ (suscripción SSE)"], [("p_sse_in", "R", 70)])
v.comp("ge", MID_X, TOP, MID_W, "svc", "Game Engine &amp; Session Manager", "Go (:8080)",
       ["• internal/scoring + jokers — resultado de la mano", "• internal/telemetry — cliente HTTP asíncrono", "   (goroutine, no bloqueante)"],
       [("p_async_dispatch", "B", 0)])
ge = v.comps["ge"]
ty = ge["y"] + ge["h"] + 120
v.comp("tel", MID_X, ty, MID_W, "svc", "Telemetry &amp; Analytics Service", "Python / FastAPI (:8000)",
       ["• Ruta de ingesta — POST JSON de jugadas", "• Servicio de analítica — eficiencia de descartes,", "   manos recurrentes, multiplicador pico",
        "• Repositorio Mongo — cliente motor (async)", "• Ruta de streaming — /api/telemetry/stream"],
       [("p_ingest_rest", "T", 0), ("p_sse_out", "L", 70), ("p_mongo_out", "R", 70)])
v.comp("mg", DB_X, ty, DB_W, "db", "MongoDB", "NoSQL documental (:27017)",
       ["• colección hand_telemetry", "• Audit Trail Ledger inmutable"], [("p_mongo_in", "L", 70)], stereotype="datastore")
t = v.comps["tel"]
v.conn(("ge", "p_async_dispatch"), ("tel", "p_ingest_rest"), "C4 · HTTP REST asíncrono", "sin bloquear el ciclo de juego", dashed=True,
       lab_pos=(MID_X + MID_W / 2 + 20, ge["y"] + ge["h"] + 55, "start"))
v.conn(("tel", "p_mongo_out"), ("mg", "p_mongo_in"), "C6 · MongoDB Wire (BSON)", "inserción / consulta de documentos")
fx, fy, _ = v.pt("fe", "p_sse_in")
sx, sy, _ = v.pt("tel", "p_sse_out")
v.conn(("tel", "p_sse_out"), ("fe", "p_sse_in"), "C3 · SSE (text/event-stream)", "telemetría en vivo (unidireccional)", dashed=True,
       via=[(430, sy), (430, fy)], lab_pos=(415, sy - 30, "end"))
v.note(40, ty + 170, 440, ["Flujo (pasos 6-8 de 4.5)", "6. C4: el Game Engine despacha la jugada en una goroutine.",
                           "7. C6: el servicio persiste el documento y actualiza métricas.", "8. C3: emite el evento SSE al panel de telemetría."])
v.save()
