# Resultados (mediana de las repeticiones)

## D1 · Ranking con y sin caché

| Prueba | Usuarios (VU) | Peticiones/s | Exitosas/s | p50 (ms) | p95 (ms) | p99 (ms) | Errores (%) | Desde caché (%) |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| Ranking con caché | 10 | 621,7 | 621,7 | 13,86 | 28,72 | 41,95 | 0,00 | 99,9 |
| Ranking con caché | 50 | 847,2 | 847,2 | 57,10 | 76,25 | 108,73 | 0,00 | 99,9 |
| Ranking con caché | 100 | 1.346,0 | 936,7 | 73,25 | 107,23 | 125,01 | 30,41 | 99,9 |
| Ranking con caché | 200 | 1.319,5 | 934,3 | 144,71 | 228,06 | 272,21 | 29,19 | 100,0 |
| Ranking sin caché | 10 | 269,8 | 269,8 | 33,90 | 60,35 | 78,27 | 0,00 | 0,0 |
| Ranking sin caché | 50 | 311,9 | 311,9 | 136,69 | 267,76 | 375,89 | 0,00 | 0,0 |
| Ranking sin caché | 100 | 314,0 | 314,0 | 277,17 | 498,03 | 611,59 | 0,00 | 0,0 |
| Ranking sin caché | 200 | 332,4 | 332,4 | 568,17 | 789,11 | 930,74 | 0,00 | 0,0 |

## D1 · Salas (ruta protegida con JWT)

| Prueba | Usuarios (VU) | Peticiones/s | Exitosas/s | p50 (ms) | p95 (ms) | p99 (ms) | Errores (%) |
|---|---:|---:|---:|---:|---:|---:|---:|
| Salas (con JWT) | 10 | 452,8 | 452,8 | 16,76 | 31,74 | 56,60 | 0,00 |
| Salas (con JWT) | 50 | 507,6 | 507,6 | 81,99 | 113,07 | 127,29 | 0,00 |
| Salas (con JWT) | 100 | 487,2 | 487,2 | 170,97 | 231,19 | 255,52 | 0,00 |
| Salas (con JWT) | 200 | 499,9 | 499,9 | 340,09 | 427,45 | 474,69 | 0,00 |

## D1 · Límite del login

| Peticiones enviadas (40/s × 20 s) | Aceptadas (200) | Rechazadas por el límite (503) | Otros códigos | Esperadas aceptadas (20 + 10/s × 20 s) | p95 (ms) |
|---:|---:|---:|---:|---:|---:|
| 801 | 220 (27,5 %) | 581 (72,5 %) | 0 | 220 | 122,03 |

## D2 · Costo de TLS · ranking (con caché)

| Usuarios (VU) | p50 HTTP directo | p50 proxy HTTP | p50 proxy HTTPS | p95 HTTP directo | p95 proxy HTTP | p95 proxy HTTPS | Costo TLS en p50 | Costo TLS en p95 | Peticiones/s HTTP → HTTPS |
|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| 10 | 8,76 | 7,53 | 7,50 | 24,23 | 19,36 | 23,62 | -0,03 ms | +4,26 ms | 1.093,8 → 978,4 (-10,6 %) |
| 50 | 36,27 | 34,65 | 35,14 | 56,13 | 46,13 | 48,14 | +0,49 ms | +2,01 ms | 1.361,1 → 1.257,0 (-7,6 %) |
| 100 | 68,76 | 71,81 | 72,82 | 95,94 | 98,75 | 98,53 | +1,01 ms | -0,22 ms | 1.303,8 → 1.277,6 (-2,0 %) |
| 200 | 160,49 | 153,69 | 156,63 | 237,72 | 203,82 | 200,74 | +2,94 ms | -3,08 ms | 1.278,1 → 1.225,8 (-4,1 %) |

## D2 · Costo de TLS · salas

| Usuarios (VU) | p50 HTTP directo | p50 proxy HTTP | p50 proxy HTTPS | p95 HTTP directo | p95 proxy HTTP | p95 proxy HTTPS | Costo TLS en p50 | Costo TLS en p95 | Peticiones/s HTTP → HTTPS |
|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| 10 | 16,72 | 16,55 | 16,06 | 33,08 | 32,53 | 33,95 | -0,49 ms | +1,42 ms | 464,3 → 472,5 (+1,8 %) |
| 50 | 84,94 | 85,33 | 79,22 | 112,97 | 116,57 | 109,78 | -6,11 ms | -6,79 ms | 489,8 → 517,4 (+5,6 %) |
| 100 | 179,49 | 174,55 | 167,06 | 246,73 | 231,06 | 216,85 | -7,49 ms | -14,21 ms | 481,8 → 499,4 (+3,7 %) |
| 200 | 374,70 | 363,44 | 347,23 | 530,94 | 467,34 | 423,40 | -16,21 ms | -43,94 ms | 464,4 → 485,6 (+4,5 %) |

## D2 · Peor caso: una conexión nueva por petición (ranking)

| Usuarios (VU) | Modo | Peticiones/s | p50 (ms) | p95 (ms) | p99 (ms) | Handshake TLS promedio (ms) |
|---:|---|---:|---:|---:|---:|---:|
| 10 | Reverse proxy · HTTP | 816,6 | 9,22 | 21,80 | 30,86 | 0,00 |
| 10 | Reverse proxy · HTTPS (P3) | 491,2 | 7,26 | 14,49 | 22,80 | 7,59 |
| 50 | Reverse proxy · HTTP | 774,3 | 54,48 | 95,63 | 144,59 | 0,00 |
| 50 | Reverse proxy · HTTPS (P3) | 540,8 | 30,69 | 69,02 | 124,18 | 44,95 |
