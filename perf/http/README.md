# Pruebas de carga HTTP (D1, D2 y D5)

Responsable: José Alejandro Solano Garcés. Los resultados y el análisis están en
[`docs/p3/rendimiento/http.md`](../../docs/p3/rendimiento/http.md).

| Archivo | Para qué sirve |
|---|---|
| `ranking.js` | Carga sobre `GET /api/leaderboard/top?limit=10` (ruta pública). Mide el porcentaje de respuestas que salen de la caché (`ranking_cache_hit`). |
| `salas.js` | Carga sobre `GET /api/rooms` y `GET /api/rooms/{code}` (rutas con JWT: gateway → Identidad → Salas → PostgreSQL). |
| `login_limite.js` | Evidencia del límite de 10 peticiones por segundo por IP en `/api/auth/*`: 40 peticiones por segundo durante 20 s. |
| `lib/config.js` | Opciones comunes (`BASE_URL`, `VUS`, `DURATION`, `MODO`, `NO_REUSE`) y el registro gradual de jugadores. |
| `seed/` | Datos de prueba para el ranking en Neo4j (1.000 jugadores y 4.000 partidas con prefijo `perf-`) y el script que los borra. |
| `sin-cache/compose.sin-cache.yml` | Override que recrea solo el Servicio de Ranking sin Redis, para medir el ranking sin caché. |
| `tls/` | Banco del costo de TLS (D2): un nginx con la misma configuración TLS del Escenario 1, que sirve el mismo proxy en HTTP (`:18081`) y en HTTPS (`:18443`). |
| `run_http.ps1` | Corre todas las pruebas y guarda un JSON por corrida, el comando exacto, `docker stats` y los datos de la máquina. |
| `resumir.py` | Saca de esos JSON `resumen.csv`, `tablas.md` y las gráficas PNG. |
| `resultados/<fecha>/` | Resultados de cada sesión de pruebas. |

## Requisitos

- k6 instalado en el computador: `winget install k6`. Si k6 corre dentro de Docker,
  `localhost` apunta al contenedor de k6 y no al sistema.
- Docker Desktop y el sistema levantado (P2: `docker compose -f docker-compose.p2.yml up -d --build --wait`).
- Python 3 con matplotlib (`pip install matplotlib`) para las gráficas.
- Git Bash, para generar el certificado del banco de TLS (`bash perf/http/tls/gen_cert.sh`).

## Cómo correrlas

Desde la raíz del repositorio, en PowerShell:

```powershell
bash perf/http/tls/gen_cert.sh
powershell -ExecutionPolicy Bypass -File perf/http/run_http.ps1
python perf/http/resumir.py perf/http/resultados/<fecha>
```

Si el sistema se levantó desde otra carpeta (otro clon con su propio `.env`), se
indica con `-ComposeDir`:

```powershell
powershell -ExecutionPolicy Bypass -File perf/http/run_http.ps1 -ComposeDir C:\ruta\al\clon
```

La sesión completa son 88 corridas de 30 s (D1: 36, D2: 52) más la prueba del
login. Con el gateway corregido (keepalive) dura cerca de una hora. Con el gateway
del P2 sin corregir dura unas dos horas, porque antes de cada corrida el script
espera a que el gateway libere los puertos en `TIME_WAIT` (ver `http.md`).

Se puede correr solo una parte, separando con comas:

```powershell
powershell -ExecutionPolicy Bypass -File perf/http/run_http.ps1 -Pruebas d1,login -Vus 10,50
```

Para comparar dos sesiones (por ejemplo, antes y después de un cambio en el gateway):

```powershell
python perf/http/resumir.py perf/http/resultados/<despues> --antes perf/http/resultados/<antes>
```

Eso escribe `comparacion.md` y `graficas/comparacion_*.png` en la carpeta `<despues>`.

### Sobre el P3 (domingo, con el reverse proxy y TLS unidos)

```powershell
powershell -ExecutionPolicy Bypass -File perf/http/run_http.ps1 -BaseUrl https://localhost -Pruebas d1,login
```

k6 acepta la CA local porque los scripts usan `insecureSkipTLSVerify`, que equivale a
`--insecure-skip-tls-verify`. En el P3 el gateway ya no publica el 8080, así que el
banco de TLS (D2) se corre sobre el P2, o en el P3 con `PERF_NET=app-net` y `-BaseUrl`
apuntando al banco.

### Una sola corrida a mano

```powershell
k6 run -e BASE_URL=http://localhost:8080 -e VUS=50 -e DURATION=30s -e MODO=cache perf/http/ranking.js
k6 run -e BASE_URL=https://localhost -e VUS=50 perf/http/salas.js
k6 run -e BASE_URL=http://localhost:8080 -e RATE=40 -e DURATION=20s perf/http/login_limite.js
```

## Limpieza

```powershell
docker cp perf/http/seed/limpiar_ranking.cypher jokerarena-neo4j:/tmp/limpiar.cypher
docker exec jokerarena-neo4j sh -c 'cypher-shell -u neo4j -p "${NEO4J_AUTH#neo4j/}" -f /tmp/limpiar.cypher'
docker exec jokerarena-redis redis-cli DEL leaderboard:top
```

Las pruebas de salas registran 20 jugadores y crean una sala por corrida. Esos datos
quedan en PostgreSQL, así que las pruebas se corren solo en el entorno local.
