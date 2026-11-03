"""jokerarena: cliente de terminal de JokerArena (Prototipo 2)."""
import asyncio
import json
from typing import Optional

import typer
from rich.console import Console
from rich.table import Table

from .api import CliError, Gateway, clear_session, gateway_url, load_session, save_session

app = typer.Typer(help="JokerArena · cliente de terminal. Todo pasa por el API Gateway (JOKERARENA_GATEWAY).", no_args_is_help=True)
rooms_app = typer.Typer(help="Salas del lobby.", no_args_is_help=True)
app.add_typer(rooms_app, name="rooms")
console = Console()


def gw(auth: bool = False) -> Gateway:
    s = load_session()
    if auth and not s:
        raise CliError("No hay sesión. Ejecuta primero: jokerarena login <alias>")
    return Gateway(token=(s or {}).get("token"))


def run(fn):
    try:
        fn()
    except CliError as exc:
        console.print(f"[bold red]✗[/] {exc}")
        raise typer.Exit(1)


@app.command()
def login(alias: str, password: Optional[str] = typer.Option(None, help="Solo para el usuario admin.")):
    """Entra con un alias (lo registra si no existe) y guarda el token."""
    def go():
        res = Gateway().enter(alias, password)
        save_session({"token": res["token"], **res["player"]})
        console.print(f"[bold green]✓[/] Sesión iniciada como [cyan]{res['player']['alias']}[/] ({res['player']['role']}) en {gateway_url()}")
    run(go)


@app.command()
def logout():
    """Cierra la sesión local."""
    clear_session()
    console.print("Sesión cerrada.")


@app.command()
def whoami():
    """Muestra la sesión guardada."""
    s = load_session()
    console.print(f"{s['alias']} · {s['role']} · {s['player_id']}" if s else "Sin sesión.")


@rooms_app.command("list")
def rooms_list(status: str = typer.Option("open", help="open | in_progress | closed | all")):
    """Lista las salas."""
    def go():
        rooms = gw(auth=True).rooms(status)
        t = Table(title=f"Salas ({status})")
        for col in ("Código", "Modo", "Dificultad", "Meta", "Estado", "Anfitrión"):
            t.add_column(col)
        for r in rooms:
            host = r["players"][0]["alias"] if r["players"] else "—"
            t.add_row(r["room_code"], r["mode"], r["difficulty"], f"{r['target_score']:,}", r["status"], host)
        console.print(t if rooms else "No hay salas.")
    run(go)


@rooms_app.command("create")
def rooms_create(bot: bool = typer.Option(True, "--bot/--solo"), target: int = 22000,
                 difficulty: str = typer.Option("normal", help="easy | normal | hard")):
    """Crea una sala (por defecto contra el bot)."""
    def go():
        r = gw(auth=True).create_room(bot, target, difficulty)
        console.print(f"[bold green]✓[/] Sala [bold cyan]{r['room_code']}[/] creada ({r['mode']}, {r['difficulty']}, meta {r['target_score']:,}).")
        console.print(f"  Juega en la web: http://localhost:3000/room/{r['room_code']}  ·  o mírala aquí: jokerarena watch {r['room_code']}")
    run(go)


@app.command()
def leaderboard(limit: int = 10):
    """Ranking global (público)."""
    def go():
        data = gw().leaderboard(limit)
        t = Table(title="Ranking global" + (" (caché Redis)" if data.get("cached") else " (Neo4j)"))
        for col in ("#", "Jugador", "Victorias", "Partidas", "Mejor puntaje"):
            t.add_column(col, justify="right" if col != "Jugador" else "left")
        for p in data["players"]:
            t.add_row(str(p["rank"]), p.get("alias") or p["player_id"][:8], str(p["wins"]), str(p["matches"]), f"{p['best_score']:,}")
        console.print(t if data["players"] else "Todavía no hay partidas terminadas.")
    run(go)


@app.command()
def matches(limit: int = 10):
    """Últimas partidas registradas por telemetría."""
    def go():
        rows = gw(auth=True).matches(limit)
        t = Table(title="Últimas partidas")
        for col in ("match_id", "Sala", "Estado", "Resultado"):
            t.add_column(col)
        for m in rows:
            res = ", ".join(f"{r.get('alias') or r['player_id'][:6]} {r['score']:,}{' 🏆' if r['won'] else ''}" for r in m.get("results", []))
            t.add_row(m["match_id"], m.get("room_code", ""), m.get("status", ""), res or "—")
        console.print(t if rows else "Sin partidas.")
    run(go)


@app.command()
def match(match_id: str):
    """Auditoría mano a mano de una partida."""
    def go():
        data = gw(auth=True).match(match_id)
        m = data.get("match") or {}
        t = Table(title=f"Partida {m.get('room_code', match_id)} · {m.get('status', '')}")
        for col in ("Mano", "Quién", "Jugada", "Chips×Mult", "Puntos", "Acumulado", "Jokers", "Ruleset"):
            t.add_column(col)
        for h in data["hands"]:
            jokers = ", ".join(j["name"] for j in h.get("jokers", []) if j.get("triggered")) or "—"
            t.add_row(str(h["hand_number"]), "Bot" if h.get("is_bot") else "Anfitrión", h["hand_type"], f"{h['chips']}×{h['mult']}",
                      f"{h['score']:,}", f"{h['accumulated_score']:,}", jokers, f"v{h.get('ruleset_version', 1)}")
        console.print(t)
    run(go)


@app.command()
def ruleset():
    """Ruleset de LiveOps que el motor está aplicando ahora."""
    def go():
        console.print_json(json.dumps(gw().ruleset()))
    run(go)


@app.command()
def watch(code: str):
    """Sigue una partida en vivo por WebSocket (Ctrl+C para salir)."""
    import websockets

    s = load_session()
    if not s:
        console.print("[bold red]✗[/] No hay sesión. Ejecuta primero: jokerarena login <alias>")
        raise typer.Exit(1)
    url = gateway_url().replace("http", "ws", 1) + f"/ws/{code.upper()}?token={s['token']}"

    async def follow():
        async with websockets.connect(url) as ws:
            async for raw in ws:
                msg = json.loads(raw)
                p, kind = msg.get("payload", {}), msg.get("type")
                if kind == "ROOM_STATE_UPDATE":
                    console.print(f"[dim]estado[/] mano {p.get('hand_number')} · puntos {p.get('current_score'):,} vs bot {p.get('bot_score', 0):,} · manos {p.get('hands_left')}")
                elif kind == "HAND_SCORED":
                    who = "Bot" if p.get("player_id") == "bot" else "Anfitrión"
                    console.print(f"[bold]{who}[/] {p.get('poker_hand')} → {p.get('final_chips')}×{p.get('final_mult')} = [green]{p.get('score'):,}[/] (ruleset v{p.get('ruleset_version', 1)})")
                elif kind == "MATCH_FINISHED":
                    console.print(f"[bold magenta]Fin:[/] {p.get('result')} · {p.get('reason')} · {p.get('final_score'):,}")
                    return
                elif kind == "ERROR":
                    console.print(f"[red]{p.get('code')}[/]: {p.get('message')}")
                    return

    try:
        asyncio.run(follow())
    except KeyboardInterrupt:
        pass
    except OSError as exc:
        console.print(f"[bold red]✗[/] No se pudo abrir el WebSocket: {exc}")
        raise typer.Exit(1)


if __name__ == "__main__":
    app()
