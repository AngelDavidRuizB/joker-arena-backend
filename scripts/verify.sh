#!/usr/bin/env bash
# ==============================================================================
# JokerArena P3 · verificación del sistema completo (tarea E1)
#   bash scripts/verify.sh          verifica un sistema ya levantado
#   bash scripts/verify.sh --up     lo levanta primero (build + wait)
#
# Todo entra por el Reverse Proxy en https://localhost, validando el certificado
# con la CA local (bash scripts/gen_certs.sh). Además de las rutas del P2 revisa
# lo nuevo del P3: el 80 redirige, el 8080 ya no está expuesto, el proxy no
# revela la versión, la segmentación de redes, el bloqueo de cuenta y que
# LiveOps rechaza cambios sin token aunque la llamada venga de la red interna.
# ==============================================================================
set -u
cd "$(dirname "$0")/.."
COMPOSE="docker compose -f ${COMPOSE_FILE_P3:-docker-compose.p2.yml}"
BASE="https://localhost"
CA="edge/certs/jokerarena-ca.crt"
fail=0
ok()   { printf '  \033[32m✔\033[0m %s\n' "$1"; }
bad()  { printf '  \033[31m✘\033[0m %s\n' "$1"; fail=1; }
warn() { printf '  \033[33m•\033[0m %s\n' "$1"; }

# En Windows (Git Bash) curl usa Schannel, que intenta revisar la revocación del
# certificado y falla con una CA local: --ssl-no-revoke desactiva solo esa revisión.
TLS=(--cacert "$CA")
curl -V | head -1 | grep -q Schannel && TLS+=(--ssl-no-revoke)
c() { curl -s "${TLS[@]}" "$@"; }

echo "1. Configuración"
if $COMPOSE config -q; then ok "${COMPOSE_FILE_P3:-docker-compose.p2.yml} es válido"; else bad "compose inválido"; exit 1; fi
[ -f "$CA" ] && ok "CA local en $CA" || { bad "falta $CA: corra 'bash scripts/gen_certs.sh'"; exit 1; }
git check-ignore -q edge/certs/server.key && ok "edge/certs/ está fuera de git" || bad "edge/certs/ no está en .gitignore"

if [ "${1:-}" = "--up" ]; then
  echo "2. Levantando el sistema (puede tardar varios minutos la primera vez)"
  $COMPOSE up -d --build --wait || bad "algún contenedor no llegó a healthy"
fi

echo "3. Estado de los contenedores"
for cont in $($COMPOSE ps --format '{{.Name}}'); do
  st=$(docker inspect -f '{{.State.Status}} {{if .State.Health}}{{.State.Health.Status}}{{end}}' "$cont")
  case "$st" in *healthy*) [[ "$st" == *unhealthy* ]] && bad "$cont: $st" || ok "$cont: $st";; *) bad "$cont: $st";; esac
done
n=$($COMPOSE ps --format '{{.Name}}' | wc -l)
# 15 = 8 servicios de lógica + gateway + reverse proxy + 4 datos + broker.
# El cliente web (Next.js SSR) vive en el repositorio joker-arena-frontend.
[ "$n" -ge 15 ] && ok "$n contenedores (8 lógica + gateway + reverse proxy + 4 datos + broker)" \
  || bad "se esperaban al menos 15 contenedores, hay $n"

echo "4. Borde: Secure Channel (escenario 1) y Reverse Proxy (escenario 2)"
c -o /dev/null "$BASE/health" && ok "https://localhost responde y el certificado es válido para la CA local" \
  || bad "https://localhost no responde o el certificado no es válido"
r=$(curl -s -o /dev/null -w '%{http_code} %{redirect_url}' http://localhost/api/leaderboard/top)
[[ "$r" == "301 https://"* ]] && ok "el puerto 80 redirige a HTTPS ($r)" || bad "el puerto 80 no redirige con 301 a https:// ($r)"
code=$(curl -s -m 3 -o /dev/null -w '%{http_code}' http://localhost:8080/health)
[ "$code" = "000" ] && ok "el 8080 del gateway ya no está expuesto en el host" || bad "el 8080 responde ($code): el gateway sigue publicado"
pubs=$(docker ps --format '{{.Names}} {{.Ports}}' | grep '^jokerarena-' | grep -- '->' | awk '{print $1}' | sort -u | tr '\n' ' ')
[ "$pubs" = "jokerarena-reverse-proxy " ] && ok "solo el reverse proxy publica puertos" || bad "contenedores con puertos publicados: $pubs"
hdr=$(c -I "$BASE/api/leaderboard/top?limit=1")
grep -qi '^server: nginx/' <<<"$hdr" && bad "la cabecera Server revela la versión" || ok "la cabecera Server no revela la versión"
grep -qi '^strict-transport-security:' <<<"$hdr" && ok "HSTS presente" || bad "falta HSTS"
grep -qi '^x-content-type-options: nosniff' <<<"$hdr" && ok "cabeceras de seguridad presentes" || bad "faltan cabeceras de seguridad"
code=$(head -c 100000 /dev/zero | tr '\0' a | c -o /dev/null -w '%{http_code}' -X POST \
  -H 'Content-Type: application/json' --data-binary @- "$BASE/api/auth/login")
[ "$code" = "413" ] && ok "cuerpo de 100 KB → 413" || bad "cuerpo de 100 KB → $code (esperado 413)"

echo "5. Rutas a través del reverse proxy ($BASE)"
check() { # método ruta código-esperado [args curl...]
  local m=$1 p=$2 want=$3; shift 3
  local got; got=$(c -o /dev/null -w '%{http_code}' -X "$m" "$BASE$p" "$@")
  [ "$got" = "$want" ] && ok "$m $p → $got" || bad "$m $p → $got (esperado $want)"
}
check GET /health 200
user="verify$RANDOM"
password="verify-pass-$RANDOM"
token=$(c -X POST "$BASE/api/auth/register" -H 'Content-Type: application/json' \
  -d "{\"user\":\"$user\",\"password\":\"$password\",\"repeat_password\":\"$password\"}" \
  | sed -n 's/.*"token":"\([^"]*\)".*/\1/p')
[ -n "$token" ] && ok "POST /api/auth/register (usuario + contraseña) → token JWT" || bad "no se obtuvo token"
check POST /api/auth/login 200 -H 'Content-Type: application/json' -d "{\"user\":\"$user\",\"password\":\"$password\"}"
check GET /api/auth/me 200 -H "Authorization: Bearer $token"
check GET /api/rooms 401
check GET /api/rooms 200 -H "Authorization: Bearer $token"
check POST /api/rooms 201 -H "Authorization: Bearer $token" -H 'Content-Type: application/json' -d '{"mode":"bot"}'
check GET /api/telemetry/matches 200 -H "Authorization: Bearer $token"
check GET "/api/leaderboard/top?limit=5" 200
check GET /api/game/ruleset 200
check GET /admin/liveops/ruleset 403 -H "Authorization: Bearer $token"
check GET /admin/cluster/snapshot 403 -H "Authorization: Bearer $token"
room=$(c -X POST "$BASE/api/rooms" -H "Authorization: Bearer $token" -H 'Content-Type: application/json' -d '{"mode":"bot"}' \
  | sed -n 's/.*"room_code":"\([^"]*\)".*/\1/p')
code=$(c -o /dev/null -m 3 -w '%{http_code}' --http1.1 -H 'Connection: Upgrade' -H 'Upgrade: websocket' \
  -H 'Sec-WebSocket-Version: 13' -H 'Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==' "$BASE/ws/$room?token=$token")
[ "$code" = "101" ] && ok "WSS /ws/$room → 101 (WebSocket sobre TLS)" || bad "WSS /ws/$room → $code (esperado 101)"

echo "6. Segmentación de redes (escenario 3)"
if docker exec jokerarena-bot-ai-ms getent hosts jokerarena-postgres >/dev/null 2>&1; then
  bad "bot-ai-ms resuelve jokerarena-postgres (no debería estar en pg-net)"
else
  ok "bot-ai-ms no puede resolver jokerarena-postgres"
fi
docker exec jokerarena-rooms-lobby-ms getent hosts jokerarena-postgres >/dev/null 2>&1 \
  && ok "rooms-lobby-ms sí llega a jokerarena-postgres" || bad "rooms-lobby-ms no resuelve jokerarena-postgres"
for net in app-net broker-net pg-net redis-net mongo-net neo4j-net; do
  [ "$(docker network inspect -f '{{.Internal}}' "$net" 2>/dev/null)" = "true" ] \
    && ok "$net es interna" || bad "$net no existe o no es interna"
done

echo "7. Bloqueo de cuenta (escenario 4)"
# Se usa un usuario de prueba (no admin) para no bloquear la consola de administración.
lock="lock$RANDOM"
lockpw="lock-pass-$RANDOM"
c -o /dev/null -X POST "$BASE/api/auth/register" -H 'Content-Type: application/json' \
  -d "{\"user\":\"$lock\",\"password\":\"$lockpw\"}"
for i in 1 2 3 4 5; do
  last=$(c -o /dev/null -w '%{http_code}' -X POST "$BASE/api/auth/login" -H 'Content-Type: application/json' \
    -d "{\"user\":\"$lock\",\"password\":\"contrasena-incorrecta\"}")
done
[ "$last" = "423" ] && ok "el quinto fallo bloquea la cuenta → 423" || bad "el quinto fallo → $last (esperado 423)"
code=$(c -o /dev/null -w '%{http_code}' -X POST "$BASE/api/auth/login" -H 'Content-Type: application/json' \
  -d "{\"user\":\"$lock\",\"password\":\"$lockpw\"}")
[ "$code" = "423" ] && ok "con la contraseña correcta sigue bloqueada → 423" || bad "con la contraseña correcta → $code (esperado 423)"
docker exec jokerarena-redis redis-cli DEL "auth:lock:$lock" "auth:fail:$lock" >/dev/null 2>&1 \
  && ok "bloqueo de prueba limpiado (auth:lock:$lock)" || warn "no se pudo limpiar auth:lock:$lock en Redis"

echo "8. Autorización en LiveOps desde la red interna (escenario 5)"
code=$(docker run --rm --network app-net curlimages/curl -s -o /dev/null -w '%{http_code}' \
  -X PUT http://jokerarena-liveops-rules-ms:8087/liveops/ruleset 2>/dev/null)
[ "$code" = "401" ] && ok "PUT /liveops/ruleset sin token desde app-net → 401" || bad "PUT sin token desde app-net → $code (esperado 401)"
code=$(docker run --rm --network app-net curlimages/curl -s -o /dev/null -w '%{http_code}' \
  http://jokerarena-liveops-rules-ms:8087/liveops/ruleset 2>/dev/null)
[ "$code" = "200" ] && ok "GET /liveops/ruleset sigue abierto para el motor → 200" || bad "GET /liveops/ruleset → $code (esperado 200)"

echo "9. Cliente web (repositorio joker-arena-frontend)"
code=$(c -o /dev/null -w '%{http_code}' "$BASE/")
case "$code" in
  200|307|308) ok "https://localhost/ → $code (cliente web)";;
  502|503|504) warn "https://localhost/ → $code: el cliente web no está levantado (repo joker-arena-frontend)";;
  *) bad "https://localhost/ → $code";;
esac

echo
[ $fail -eq 0 ] && echo "Todo correcto." || { echo "Hay comprobaciones fallidas."; exit 1; }
