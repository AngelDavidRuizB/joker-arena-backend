# Pruebas de rendimiento P3 con k6 (D3, D4 y D5)

Los scripts usan el gateway de P2. Ejecútalos solo contra una instancia local/de pruebas:
D3 crea una cuenta y una sala por VU; esos registros quedan en PostgreSQL.
No apuntes las pruebas de carga a producción.

## Requisitos

- Docker Desktop iniciado.
- Stack P2 saludable: `docker compose -f docker-compose.p2.yml up -d --build --wait`.
- Proxy inverso disponible en el puerto 80 del host.
- PowerShell abierto en la raíz del repositorio.
- k6 se ejecuta en un contenedor, por lo que no hace falta instalarlo en `.venv`.

Preparar la carpeta para los resultados:

```powershell
New-Item -ItemType Directory -Force .\results\k6 | Out-Null
```

En Windows Docker Desktop, el reverse proxy del host se alcanza desde el contenedor k6
mediante `host.docker.internal`.

## D3 — Juego por WebSocket

`tests/k6/game-websocket.js` prepara una cuenta y una sala contra bot por cada VU.
Luego conecta simultáneamente las partidas, espera `ROOM_STATE_UPDATE`, envía una
jugada válida de una carta y mide milisegundos desde `PLAY_HAND` hasta
`HAND_SCORED` del jugador. La preparación queda fuera de esa métrica.
El gateway valida el token de la query WebSocket, igual que exige el contrato.

Ejecuta cada volumen por separado y repite cada escenario tres veces. Ejemplo para
10 partidas:

Usa este bloque para ejecutar los tres tamaños, tres veces cada uno:

```powershell
$scripts = "${PWD}\tests\k6:/scripts:ro"
$results = "${PWD}\results\k6:/results"
foreach ($vus in @(10, 50, 100)) {
  for ($run = 1; $run -le 3; $run++) {
    $file = "d3-vus-$vus-r$run.json"
    docker run --rm -v $scripts -v $results grafana/k6 run "--summary-export=/results/$file" -e BASE_URL=http://host.docker.internal -e "VUS=$vus" /scripts/game-websocket.js
    if ($LASTEXITCODE -ne 0) { throw "Falló D3: $file" }
  }
}
```

Cada ejecución crea usuarios y salas nuevas. El proyecto no tiene una ruta de
limpieza de esos fixtures; conserva los resultados y ejecuta esto solo en la base
local/de pruebas.

Los nombres generados serán:

| Partidas simultáneas | Nombre del resumen |
|---:|---|
| 10 | `d3-vus-10-r1.json`, `d3-vus-10-r2.json`, `d3-vus-10-r3.json` |
| 50 | `d3-vus-50-r1.json`, `d3-vus-50-r2.json`, `d3-vus-50-r3.json` |
| 100 | `d3-vus-100-r1.json`, `d3-vus-100-r2.json`, `d3-vus-100-r3.json` |

El resumen incluye `ws_play_to_hand_scored_ms` (avg, mediana, p95, p99),
`ws_sessions_connected`, `ws_play_actions_sent`, `ws_play_actions_scored`,
`ws_session_errors` y `ws_session_timeouts`. Una acción no puntuada o una sesión
con timeout debe investigarse, no descartarse de los resultados.

Tabla para el informe:

| Partidas | Repetición | p50 (ms) | p95 (ms) | p99 (ms) | Conectadas | Puntuadas | Errores | Timeouts |
|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| 10 | 1 | 10,00 | 27,95 | 31,19 | 10/10 | 10/10 | 0 | 0 |
| 10 | 2 | 27,00 | 42,20 | 43,64 | 10/10 | 10/10 | 0 | 0 |
| 10 | 3 | 14,50 | 30,20 | 31,64 | 10/10 | 10/10 | 0 | 0 |
| 50 | 1 | 40,00 | 88,10 | 101,24 | 50/50 | 50/50 | 0 | 0 |
| 50 | 2 | 25,00 | 62,65 | 74,67 | 50/50 | 50/50 | 0 | 0 |
| 50 | 3 | 17,00 | 31,65 | 36,04 | 50/50 | 50/50 | 0 | 0 |
| 100 | 1 | 31,00 | 70,00 | 79,25 | 100/100 | 100/100 | 0 | 0 |
| 100 | 2 | 60,00 | 144,05 | 151,18 | 100/100 | 100/100 | 0 | 0 |
| 100 | 3 | 25,00 | 59,05 | 65,06 | 100/100 | 100/100 | 0 | 0 |

Las nueve corridas válidas conectaron y puntuaron todas sus partidas, sin errores
ni timeouts. Una preparación inicial de 100 VU, repetición 2, falló con HTTP 401;
ese intento parcial está archivado y no forma parte de la tabla. La repetición se
ejecutó de nuevo correctamente.

Con los valores de la tabla, crea una gráfica de líneas con partidas simultáneas
en X y latencia p95/p99 en Y. Usa la mediana de las tres repeticiones por volumen
y conserva las repeticiones individuales en la tabla.

## D4 — Escalabilidad horizontal del ranking

El servicio `jokerarena-leaderboard-stats-ms` no fija `container_name`, así Compose
puede crear varias réplicas. Todas comparten Redis, Neo4j y RabbitMQ; nginx resuelve
el nombre del servicio mediante el DNS interno de Docker. El script
`tests/k6/leaderboard.js` pide el top 10 y mide RPS y latencia HTTP p95/p99.

Para cada cantidad de réplicas (1, 2 y 3), escala el ranking y espera solo sus
chequeos de salud. No uses `--wait` en este bucle: Compose esperaría también por
servicios ajenos a D4, como telemetría, y un timeout suyo puede bloquear la prueba.

Para comparar 1, 2 y 3 réplicas con 50 VU y tres repeticiones por configuración,
ejecuta:

```powershell
$scripts = "${PWD}\tests\k6:/scripts:ro"
$results = "${PWD}\results\k6:/results"
foreach ($replicas in @(1, 2, 3)) {
  docker compose -f docker-compose.p2.yml up -d --scale "jokerarena-leaderboard-stats-ms=$replicas" jokerarena-leaderboard-stats-ms
  if ($LASTEXITCODE -ne 0) { throw "No se pudo levantar $replicas réplica(s)" }

  $ready = $false
  for ($attempt = 0; $attempt -lt 30; $attempt++) {
    $containers = @(docker compose -f docker-compose.p2.yml ps -q jokerarena-leaderboard-stats-ms)
    $allHealthy = $containers.Count -eq $replicas
    foreach ($container in $containers) {
      $health = docker inspect --format '{{.State.Health.Status}}' $container
      if ($health -ne 'healthy') { $allHealthy = $false }
    }
    if ($allHealthy) { $ready = $true; break }
    Start-Sleep -Seconds 2
  }
  if (-not $ready) {
    docker compose -f docker-compose.p2.yml ps -a jokerarena-leaderboard-stats-ms
    throw "El ranking no quedó saludable con $replicas réplica(s)"
  }

  docker compose -f docker-compose.p2.yml ps jokerarena-leaderboard-stats-ms
  for ($run = 1; $run -le 3; $run++) {
    $warmupStatus = curl.exe -sS -o NUL -w "%{http_code}" "http://localhost/api/leaderboard/top?limit=10"
    if ($LASTEXITCODE -ne 0 -or $warmupStatus -ne "200") { throw "Falló el warm-up del ranking" }
    $file = "d4-replicas-$replicas-vus-50-r$run.json"
    docker run --rm -v $scripts -v $results grafana/k6 run "--summary-export=/results/$file" -e BASE_URL=http://host.docker.internal -e "REPLICAS=$replicas" -e VUS=50 -e DURATION=1m /scripts/leaderboard.js
    if ($LASTEXITCODE -ne 0) { throw "Falló D4: $file" }
  }
}
```

Mantén VU, duración, datos y caché comparables. Guarda p95/p99 de
`http_req_duration`, RPS de `http_reqs.rate`, `checks` y errores HTTP. Para
verificar DNS y réplicas:

```powershell
docker run --rm --network jokerarena-net busybox:1.36 nslookup jokerarena-leaderboard-stats-ms
$replicas = docker compose -f docker-compose.p2.yml ps -q jokerarena-leaderboard-stats-ms
docker inspect --format '{{.Name}} {{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' $replicas
```

Guarda esa salida para analizar el DNS de Docker y confirmar cuántas instancias
están presentes. Durante una ejecución k6, abre otra PowerShell y captura recursos:

```powershell
$replicas = docker compose -f docker-compose.p2.yml ps -q jokerarena-leaderboard-stats-ms
docker stats $replicas
```

Detén `docker stats` con Ctrl+C al terminar la carga.

| Réplicas | VU | Repetición | RPS | p95 (ms) | p99 (ms) | Checks fallidos | Errores |
|---:|---:|---:|---:|---:|---:|---:|---:|
| 1 | 50 | 1 | 264,35 | 341,43 | 562,21 | 0 | 0 |
| 1 | 50 | 2 | 206,47 | 446,18 | 698,66 | 0 | 0 |
| 1 | 50 | 3 | 280,39 | 371,40 | 708,65 | 0 | 0 |
| 2 | 50 | 1 | 538,76 | 220,26 | 390,01 | 0 | 0 |
| 2 | 50 | 2 | 533,01 | 211,05 | 365,04 | 0 | 0 |
| 2 | 50 | 3 | 498,27 | 242,80 | 368,65 | 0 | 0 |
| 3 | 50 | 1 | 575,98 | 201,02 | 297,86 | 0 | 0 |
| 3 | 50 | 2 | 554,90 | 194,56 | 279,38 | 0 | 0 |
| 3 | 50 | 3 | 665,02 | 162,26 | 229,81 | 0 | 0 |

Grafica RPS y p95/p99 frente al número de réplicas. Explica si la capacidad
aumenta y si la latencia baja, se mantiene o empeora; no asumas escalabilidad
lineal. Incluye la salida del DNS, CPU y memoria como evidencia.
Para la comparación final, resume las tres repeticiones con la mediana de cada métrica.

El motor de juego no se escala igual: mantiene sesiones y actores de sala en la
memoria de cada proceso, además de conexiones WebSocket persistentes. Sin afinidad
de sala o estado/particionado distribuido, distintas réplicas podrían crear actores
independientes para el mismo código de sala. Aumentar réplicas del motor requiere
resolver esa propiedad de estado; no basta con escalar el contenedor.

## D5 — Conclusiones

1. **WebSocket:** la mediana de p95/p99 pasó de 30,20/31,64 ms con 10 partidas a
   62,65/74,67 ms con 50 y 70,00/79,25 ms con 100. Las 480 partidas de las nueve
   corridas válidas se conectaron y puntuaron, sin errores ni timeouts. La prueba
   describe este entorno local y no permite extrapolar a producción.
2. **Ranking:** con 50 VU, el RPS mediano fue 264,35, 533,01 y 575,98 para 1, 2 y
   3 réplicas. El salto de 1 a 2 fue aproximadamente 2,02x; de 2 a 3 fue 8,1%.
   Las medianas de p95/p99 bajaron de 371,40/698,66 ms a 194,56/279,38 ms entre 1
   y 3 réplicas. No se observó escalabilidad lineal.
3. **DNS y dependencias:** con 3 réplicas, la resolución DNS del servicio devolvió
   las IP 172.19.0.14, 172.19.0.17 y 172.19.0.18. Redis, Neo4j y RabbitMQ estaban
   saludables durante la prueba. Las IP son asignaciones dinámicas de esta red
   Docker, no direcciones permanentes.
4. **Calidad de medición y recursos:** los checks D4 fueron 100% y no hubo errores
   HTTP. Sin embargo, 8 de los 9 JSON D4 tienen una latencia mínima negativa, y
   no se capturaron CPU/memoria durante las corridas. Por eso, los percentiles y
   conclusiones D4 son provisionales y no permiten identificar un cuello de
   botella; conviene repetir D4 con el reloj estable y capturar `docker stats`.
5. **Arquitectura:** el resultado D4 solo escala el ranking. El motor de juego
   mantiene actores y sesiones en memoria; esta prueba no demuestra que sea seguro
   escalar el motor. Antes de hacerlo se requiere afinidad de sala o coordinación
   y estado distribuido.

**Entorno:** Windows con Docker Desktop; Intel Core i5-10210U (4 núcleos, 8
procesadores lógicos), 15,9 GiB de RAM; k6 v2.4.0; gateway en el puerto 8080;
pruebas ejecutadas el 2026-11-22. D3: 10/50/100 VU, una iteración por VU, tres
repeticiones. D4: 50 VU durante 1 minuto, tres repeticiones por configuración.
No se midió utilización de CPU/RAM durante la carga.

Resultados guardados en `results/k6/d3-20261122-170950/` y
`results/k6/d4-20261122-162222/`, incluidos los JSON individuales y los CSV
resumidos.
