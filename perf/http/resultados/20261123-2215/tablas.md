# Resultados (mediana de las repeticiones)

## D1 · Ranking con y sin caché

| Prueba | Usuarios (VU) | Peticiones/s | Exitosas/s | p50 (ms) | p95 (ms) | p99 (ms) | Errores (%) | Desde caché (%) |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| Ranking con caché | 10 | 1.856,9 | 1.856,9 | 4,77 | 8,38 | 13,52 | 0,00 | 100,0 |
| Ranking con caché | 50 | 2.149,2 | 2.149,2 | 21,69 | 32,16 | 46,91 | 0,00 | 99,9 |
| Ranking con caché | 100 | 2.243,8 | 2.243,8 | 42,38 | 60,91 | 78,21 | 0,00 | 100,0 |
| Ranking con caché | 200 | 2.642,5 | 2.642,5 | 72,08 | 106,21 | 160,97 | 0,00 | 100,0 |
| Ranking sin caché | 10 | 250,3 | 250,3 | 35,85 | 61,94 | 83,33 | 0,00 | 0,0 |
| Ranking sin caché | 50 | 333,6 | 333,6 | 136,84 | 252,74 | 338,17 | 0,00 | 0,0 |
| Ranking sin caché | 100 | 334,3 | 334,3 | 269,40 | 484,13 | 641,03 | 0,00 | 0,0 |
| Ranking sin caché | 200 | 333,6 | 333,6 | 570,31 | 874,07 | 2.134,26 | 0,00 | 0,0 |

## D1 · Salas (ruta protegida con JWT)

| Prueba | Usuarios (VU) | Peticiones/s | Exitosas/s | p50 (ms) | p95 (ms) | p99 (ms) | Errores (%) |
|---|---:|---:|---:|---:|---:|---:|---:|
| Salas (con JWT) | 10 | 477,5 | 477,5 | 16,75 | 24,83 | 30,47 | 0,00 |
| Salas (con JWT) | 50 | 485,6 | 485,6 | 86,18 | 104,78 | 114,60 | 0,00 |
| Salas (con JWT) | 100 | 475,3 | 475,3 | 173,40 | 206,89 | 230,21 | 0,00 |
| Salas (con JWT) | 200 | 477,4 | 477,4 | 344,42 | 428,35 | 463,30 | 0,00 |

## D1 · Límite del login

| Peticiones enviadas (40/s × 20 s) | Aceptadas (200) | Rechazadas por el límite (503) | Otros códigos | Esperadas aceptadas (20 + 10/s × 20 s) | p95 (ms) |
|---:|---:|---:|---:|---:|---:|
| 801 | 220 (27,5 %) | 581 (72,5 %) | 0 | 220 | 3.029,29 |

## D2 · Costo de TLS · ranking (con caché)

| Usuarios (VU) | p50 HTTP directo | p50 proxy HTTP | p50 proxy HTTPS | p95 HTTP directo | p95 proxy HTTP | p95 proxy HTTPS | Costo TLS en p50 | Costo TLS en p95 | Peticiones/s HTTP → HTTPS |
|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|

## D2 · Costo de TLS · salas

| Usuarios (VU) | p50 HTTP directo | p50 proxy HTTP | p50 proxy HTTPS | p95 HTTP directo | p95 proxy HTTP | p95 proxy HTTPS | Costo TLS en p50 | Costo TLS en p95 | Peticiones/s HTTP → HTTPS |
|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|

## D2 · Peor caso: una conexión nueva por petición (ranking)

| Usuarios (VU) | Modo | Peticiones/s | p50 (ms) | p95 (ms) | p99 (ms) | Handshake TLS promedio (ms) |
|---:|---|---:|---:|---:|---:|---:|
