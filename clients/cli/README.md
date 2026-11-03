# jokerarena (CLI)

Cliente de terminal. **Responsable:** José Alejandro Solano Garcés · **Python (Typer, httpx, Rich, websockets)**.

Habla **solo** con el API Gateway (`http://localhost:8080` o la variable `JOKERARENA_GATEWAY`). Guarda la sesión en `~/.jokerarena/session.json`.

```bash
pip install ./clients/cli
```

| Comando | Qué hace |
|---|---|
| `jokerarena login <alias> [--password]` | Entra (o se registra) y guarda el token. `admin` requiere contraseña. |
| `jokerarena rooms list [--status all]` | Lista salas. |
| `jokerarena rooms create [--solo] [--target 26000] [--difficulty hard]` | Crea una sala. |
| `jokerarena watch <código>` | Sigue una partida en vivo por WebSocket. |
| `jokerarena leaderboard` | Ranking global (indica si vino de la caché de Redis). |
| `jokerarena matches` · `jokerarena match <match_id>` | Partidas y auditoría mano a mano. |
| `jokerarena ruleset` | Ruleset de LiveOps que el motor aplica ahora. |

## Pruebas

```bash
pip install pytest && pytest -q
```
