# jokerarena (CLI)

Cliente de terminal. **Responsable:** José Alejandro Solano Garcés (actualizado a HTTPS/WSS por Ángel David Ruiz Barbosa en P3) · **Python (Typer, httpx, Rich, websockets)**.

Habla **solo** con el borde seguro / API Gateway (`https://localhost` por defecto, o la variable `JOKERARENA_GATEWAY`). Guarda la sesión en `~/.jokerarena/session.json`.

Para verificar los certificados TLS autofirmados de desarrollo:
1. Genera la CA local desde la raíz del repositorio: `bash scripts/gen_certs.sh`.
2. La CLI detecta automáticamente `edge/certs/jokerarena-ca.crt` (o puedes indicar otra ruta con `JOKERARENA_CA=/ruta/jokerarena-ca.crt`).

```bash
pip install ./clients/cli
```

| Comando | Qué hace |
|---|---|
| `jokerarena login <alias> [--password]` | Entra (o se registra) y guarda el token. `admin` requiere contraseña. |
| `jokerarena rooms list [--status all]` | Lista salas. |
| `jokerarena rooms create [--solo] [--target 26000] [--difficulty hard]` | Crea una sala. |
| `jokerarena watch <código>` | Sigue una partida en vivo por WebSocket (`wss://`). |
| `jokerarena leaderboard` | Ranking global (indica si vino de la caché de Redis). |
| `jokerarena matches` · `jokerarena match <match_id>` | Partidas y auditoría mano a mano. |
| `jokerarena ruleset` | Ruleset de LiveOps que el motor aplica ahora. |

## Pruebas

```bash
pip install pytest && pytest -q
```
