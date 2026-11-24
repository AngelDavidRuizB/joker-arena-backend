# Resultados (mediana de las repeticiones)

## D1 · Ranking con y sin caché

| Prueba | Usuarios (VU) | Peticiones/s | Exitosas/s | p50 (ms) | p95 (ms) | p99 (ms) | Errores (%) | Desde caché (%) |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| Ranking con caché | 10 | 2.274,4 | 2.274,4 | 3,62 | 7,49 | 13,35 | 0,00 | 100,0 |
| Ranking con caché | 50 | 2.372,8 | 2.372,8 | 19,23 | 31,50 | 40,07 | 0,00 | 100,0 |
| Ranking con caché | 100 | 2.491,9 | 2.491,9 | 37,77 | 56,98 | 72,96 | 0,00 | 100,0 |
| Ranking con caché | 200 | 2.516,3 | 2.516,3 | 71,48 | 109,40 | 132,86 | 0,00 | 100,0 |
| Ranking sin caché | 10 | 169,3 | 169,3 | 54,17 | 91,83 | 122,81 | 0,00 | 0,0 |
| Ranking sin caché | 50 | 225,0 | 225,0 | 203,06 | 400,52 | 509,78 | 0,00 | 0,0 |
| Ranking sin caché | 100 | 227,0 | 227,0 | 414,73 | 728,98 | 1.117,27 | 0,00 | 0,0 |
| Ranking sin caché | 200 | 218,2 | 218,2 | 846,51 | 1.404,23 | 2.601,78 | 0,00 | 0,0 |

## D1 · Salas (ruta protegida con JWT)

| Prueba | Usuarios (VU) | Peticiones/s | Exitosas/s | p50 (ms) | p95 (ms) | p99 (ms) | Errores (%) |
|---|---:|---:|---:|---:|---:|---:|---:|
| Salas (con JWT) | 10 | 496,6 | 496,6 | 15,41 | 30,36 | 39,78 | 0,00 |
| Salas (con JWT) | 50 | 380,8 | 380,8 | 103,74 | 149,45 | 185,76 | 0,00 |
| Salas (con JWT) | 100 | 412,3 | 412,3 | 183,39 | 306,63 | 350,56 | 0,00 |
| Salas (con JWT) | 200 | 422,0 | 422,0 | 359,76 | 627,67 | 691,18 | 0,00 |

## D1 · Límite del login

(no se corrió la prueba del límite del login)

## D2 · Costo de TLS · ranking (con caché)

| Usuarios (VU) | p50 HTTP directo | p50 proxy HTTP | p50 proxy HTTPS | p95 HTTP directo | p95 proxy HTTP | p95 proxy HTTPS | Costo TLS en p50 | Costo TLS en p95 | Peticiones/s HTTP → HTTPS |
|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| 10 | 3,90 | 4,00 | 4,38 | 7,83 | 7,66 | 8,90 | +0,38 ms | +1,24 ms | 2.131,4 → 1.925,5 (-9,7 %) |
| 50 | 17,55 | 18,56 | 17,89 | 32,54 | 33,33 | 30,12 | -0,67 ms | -3,21 ms | 2.415,2 → 2.533,7 (+4,9 %) |
| 100 | 33,86 | 34,44 | 36,11 | 57,89 | 57,36 | 59,27 | +1,67 ms | +1,91 ms | 2.637,7 → 2.512,6 (-4,7 %) |
| 200 | 67,39 | 71,41 | 76,28 | 104,56 | 101,48 | 110,40 | +4,87 ms | +8,92 ms | 2.610,3 → 2.425,8 (-7,1 %) |

## D2 · Costo de TLS · salas

| Usuarios (VU) | p50 HTTP directo | p50 proxy HTTP | p50 proxy HTTPS | p95 HTTP directo | p95 proxy HTTP | p95 proxy HTTPS | Costo TLS en p50 | Costo TLS en p95 | Peticiones/s HTTP → HTTPS |
|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| 10 | 19,44 | 19,87 | 22,50 | 32,32 | 30,37 | 34,84 | +2,63 ms | +4,47 ms | 411,4 → 367,6 (-10,6 %) |
| 50 | 86,94 | 89,05 | 92,91 | 138,13 | 124,07 | 147,48 | +3,86 ms | +23,41 ms | 469,5 → 438,3 (-6,6 %) |
| 100 | 177,51 | 173,25 | 178,92 | 273,76 | 271,06 | 257,56 | +5,67 ms | -13,50 ms | 465,1 → 453,4 (-2,5 %) |
| 200 | 357,35 | 371,79 | 373,68 | 531,89 | 511,06 | 491,68 | +1,89 ms | -19,38 ms | 445,1 → 450,4 (+1,2 %) |

## D2 · Peor caso: una conexión nueva por petición (ranking)

| Usuarios (VU) | Modo | Peticiones/s | p50 (ms) | p95 (ms) | p99 (ms) | Handshake TLS promedio (ms) |
|---:|---|---:|---:|---:|---:|---:|
| 10 | Reverse proxy · HTTP | 1.136,3 | 6,49 | 13,37 | 20,09 | 0,00 |
| 10 | Reverse proxy · HTTPS (P3) | 402,1 | 9,10 | 18,88 | 30,04 | 11,29 |
| 50 | Reverse proxy · HTTP | 649,0 | 68,58 | 108,93 | 189,36 | 0,00 |
| 50 | Reverse proxy · HTTPS (P3) | 494,4 | 42,27 | 109,81 | 152,83 | 8,29 |
