# Guía de sustentación — JokerArena (Prototipo 1)

Sustentación: martes 13 de octubre de 2026. Duración sugerida: 10–12 min.

## 0. Antes de entrar a clase (5 min)
```bash
cp .env.example .env            # solo si no existe; completar claves
docker compose up --build -d
docker compose ps               # 5 servicios en "healthy"
```
Abrir de antemano: `http://localhost:3000`, `http://localhost:8000/docs` y una terminal en la carpeta del proyecto. Si 5432/8080/27017 están ocupados, ajustar puertos en `.env` y volver a ejecutar `docker compose up -d`.

## 1. Mensaje de apertura (1 min)
- Sistema distribuido en **5 contenedores**: SPA React (TypeScript), motor de juego en **Go**, telemetría en **Python**, **PostgreSQL** y **MongoDB** (persistencia políglota, una BD por servicio).
- **Servidor autoritativo**: el navegador solo dibuja; barajado, validación de jugadas, evaluación de manos y puntaje ocurren en Go.
- Conectores: **REST** (C1, registro/salas), **WebSocket** (C2, tiempo real), SQL (C6) y MongoDB (C7). Mostrar el diagrama C&C de `p1_X.md` §3.1.

## 2. Demo en vivo (5 min)
| Paso | Qué hacer | Qué señalar |
|---|---|---|
| 1 | Abrir `localhost:3000`, escribir alias `JugadorDemo`, **Registrarme y jugar**. | F1: el alias queda en PostgreSQL (se verifica en el paso 8). Validación inmediata de alias. |
| 2 | En el lobby elegir meta **1200**, dejar *Jugar contra Bot*, **Crear sala (con Bot)**. | F2: código `JKR-XXXX` único. Alternativa: *Unirme* con un código inexistente muestra error real (sin datos falsos). |
| 3 | En el tapete aparecen 8 cartas animadas. | F3: llegaron por WebSocket (`ROOM_STATE_UPDATE`), baraja de 52 barajada con semilla en el servidor. Indicador *En línea*. |
| 4 | Seleccionar 6 cartas: solo permite 5. Probar teclas `1–8`. | Selección reactiva, accesible (`aria-pressed`). |
| 5 | **Jugar mano**. | F4/F5: cascada Fichas × Mult. Banner (+30 por descarte), Joker Clásico (+4 Mult), The Duo (×2 con pareja). El panel *Auditoría en vivo* muestra la fórmula exacta. |
| 6 | **Descartar** 2 cartas. | Las cartas se reponen desde el mazo; baja el contador de descartes. |
| 7 | Recargar la página (F5). | La sala y la mano se conservan (estado en el servidor, reconexión automática). |
| 8 | Terminal: ver el comando de §3. | Persistencia relacional: jugador, sala y partida (WAITING → IN_PROGRESS → WIN/LOSE). |
| 9 | Jugar hasta alcanzar/perder la meta. | `MATCH_FINISHED`: pantalla de victoria (confeti) o derrota; resultado guardado en `matches.result`. |

## 3. Comandos de verificación
```bash
# Estado de los servicios
docker compose ps
curl http://localhost:8080/health          # {"status":"ok","postgres":"connected",...}
curl http://localhost:8000/health          # {"status":"ok","mongodb":"connected",...}

# Persistencia en PostgreSQL (esquema del equipo: players / rooms / matches)
docker compose exec jokerarena-game-db psql -U appuser -d appdb \
  -c "SELECT id_player, username FROM players ORDER BY id_player DESC LIMIT 3;" \
  -c "SELECT r.name AS sala, r.description, m.id_player, m.result FROM matches m JOIN rooms r USING (id_room) ORDER BY m.id_match DESC LIMIT 4;"

# MongoDB con autenticación
docker compose exec jokerarena-telemetry-db mongosh -u admin -p "<clave>" --authenticationDatabase admin casino_games \
  --eval "db.getCollectionNames()"
```

## 4. Preguntas probables y respuestas cortas
- **¿Por qué servidor autoritativo?** Evita manipular puntajes desde el cliente; el cliente solo envía índices de cartas y el servidor valida (1–5, únicos, en rango, saldo de manos/descartes).
- **¿Cómo manejan la concurrencia?** Cada sala es una goroutine que procesa comandos desde un canal (patrón actor); las lecturas REST usan instantáneas protegidas. Probado con `go test -race`.
- **¿Qué pasa si PostgreSQL cae?** La API responde `503` y `/health` pasa a `degraded`: no se simula persistencia. El motor reintenta la conexión con *backoff*.
- **¿Cómo se adaptó el esquema del equipo?** `players/rooms/matches` no tienen campos de juego; una capa adaptadora mapea alias→`username`, código de sala→`rooms.name`, meta y Bot→`rooms.description`, y una fila de `matches` por participante (`result`: WAITING/IN_PROGRESS/WIN/LOSE). Ver `p1_X.md` §3.3.1.
- **¿Dónde está el catálogo de Jokers?** En el código (`internal/poker`), porque el esquema compartido no tiene tabla `jokers`.
- **¿Qué falta respecto al 100%?** F6/F7 completos: despacho gRPC Go→Python (C5), streaming SSE (C4), estadísticas finales (C3) y el Bot jugando solo. El servidor gRPC, el contrato `.proto` y la ingesta en Mongo ya existen y están probados.

## 5. Plan B si algo falla
- Un puerto ocupado: cambiar el puerto en `.env` y `docker compose up -d`.
- El frontend no carga: `docker compose logs jokerarena-fe`; verificar `curl localhost:3000/health`.
- Sin Docker: mostrar `go test ./...` (en contenedor), `npm test` y capturas del flujo.
