# Prueba A/B: gateway original contra gateway con keepalive

Sección 6.2 de [`docs/p3/rendimiento/http.md`](../../../../docs/p3/rendimiento/http.md).
23 de noviembre de 2026, 00:14–00:24, con el equipo en batería. Las dos variantes se
corrieron intercaladas, así que comparten las mismas condiciones.

Procedimiento:

1. Servicio de Ranking recreado sin caché (`perf/http/sin-cache/compose.sin-cache.yml`).
2. Para cada repetición (r1 y r2) y cada variante (`original` y `keepalive`):
   - se copia el `nginx.conf` de la variante dentro del gateway en ejecución y se
     recarga:
     ```
     docker cp nginx.conf jokerarena-gateway:/etc/nginx/nginx.conf
     docker exec jokerarena-gateway nginx -s reload
     ```
     `original` es `gateway/nginx.conf` de `main` y `keepalive` el de la rama
     `p3-fix-gateway-keepalive`;
   - se espera a que el gateway tenga menos de 500 conexiones en `TIME_WAIT`;
   - se corre:
     ```
     k6 run --quiet --summary-export ab-sin-cache-<variante>-v50-r<n>.json -e BASE_URL=http://localhost:8080 -e VUS=50 -e DURATION=30s -e MODO=sin-cache perf/http/ranking.js
     k6 run --quiet --summary-export ab-jwt-<variante>-v50-r<n>.json -e BASE_URL=http://localhost:8080 -e VUS=50 -e DURATION=30s -e MODO=jwt perf/http/salas.js
     ```
3. Al final se restauran el Servicio de Ranking con caché y el `nginx.conf` original.
