# Antes y después de la corrección del gateway (mediana de las repeticiones)

| Prueba | Usuarios (VU) | Exitosas/s antes | Exitosas/s después | p95 antes (ms) | p95 después (ms) | Errores antes (%) | Errores después (%) |
|---|---:|---:|---:|---:|---:|---:|---:|
| Ranking con caché | 10 | 621,7 | 2.274,4 | 28,72 | 7,49 | 0,00 | 0,00 |
| Ranking con caché | 50 | 847,2 | 2.372,8 | 76,25 | 31,50 | 0,00 | 0,00 |
| Ranking con caché | 100 | 936,7 | 2.491,9 | 107,23 | 56,98 | 30,41 | 0,00 |
| Ranking con caché | 200 | 934,3 | 2.516,3 | 228,06 | 109,40 | 29,19 | 0,00 |
| Ranking sin caché | 10 | 269,8 | 169,3 | 60,35 | 91,83 | 0,00 | 0,00 |
| Ranking sin caché | 50 | 311,9 | 225,0 | 267,76 | 400,52 | 0,00 | 0,00 |
| Ranking sin caché | 100 | 314,0 | 227,0 | 498,03 | 728,98 | 0,00 | 0,00 |
| Ranking sin caché | 200 | 332,4 | 218,2 | 789,11 | 1.404,23 | 0,00 | 0,00 |
| Salas (con JWT) | 10 | 452,8 | 496,6 | 31,74 | 30,36 | 0,00 | 0,00 |
| Salas (con JWT) | 50 | 507,6 | 380,8 | 113,07 | 149,45 | 0,00 | 0,00 |
| Salas (con JWT) | 100 | 487,2 | 412,3 | 231,19 | 306,63 | 0,00 | 0,00 |
| Salas (con JWT) | 200 | 499,9 | 422,0 | 427,45 | 627,67 | 0,00 | 0,00 |
