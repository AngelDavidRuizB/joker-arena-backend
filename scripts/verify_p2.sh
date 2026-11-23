#!/usr/bin/env bash
# ==============================================================================
# JokerArena P2 · verificación del despliegue completo
#   bash scripts/verify_p2.sh          verifica un sistema ya levantado
#   bash scripts/verify_p2.sh --up     lo levanta primero (build + wait)
# ==============================================================================
set -u
cd "$(dirname "$0")/.."
COMPOSE="docker compose -f docker-compose.p2.yml"
GW="${JOKERARENA_GATEWAY:-http://localhost}"
WEB="${JOKERARENA_WEB:-http://localhost}"
fail=0
ok()   { printf '  \033[32m✔\033[0m %s\n' "$1"; }
bad()  { printf '  \033[31m✘\033[0m %s\n' "$1"; fail=1; }

echo "1. Configuración de compose"
if $COMPOSE config -q; then ok "docker-compose.p2.yml es válido"; else bad "compose inválido"; exit 1; fi

if [ "${1:-}" = "--up" ]; then
  echo "2. Levantando el sistema (puede tardar varios minutos la primera vez)"
  $COMPOSE up -d --build --wait || bad "algún contenedor no llegó a healthy"
fi

echo "3. Estado de los contenedores"
for c in $($COMPOSE ps --format '{{.Name}}'); do
  st=$(docker inspect -f '{{.State.Status}} {{if .State.Health}}{{.State.Health.Status}}{{end}}' "$c")
  case "$st" in *healthy*) [[ "$st" == *unhealthy* ]] && bad "$c: $st" || ok "$c: $st";; *) bad "$c: $st";; esac
done
n=$($COMPOSE ps --format '{{.Name}}' | wc -l)
[ "$n" -ge 16 ] && ok "$n contenedores (8 lógica + gateway + web + proxy + 4 datos + broker)" || bad "se esperaban 16 contenedores, hay $n"

echo "4. Entrada pública y cabeceras del reverse proxy"
for pair in "jokerarena-reverse-proxy 80" "jokerarena-reverse-proxy 443"; do
  read -r service target <<< "$pair"
  mapping=$($COMPOSE port "$service" "$target" 2>/dev/null)
  [ -n "$mapping" ] && ok "$service publica el puerto $target" || bad "$service no publica el puerto $target"
done
for pair in "jokerarena-gateway 8080" "jokerarena-web-ssr 3000" \
  "jokerarena-auth-identity-ms 8082" "jokerarena-rooms-lobby-ms 8083" \
  "jokerarena-game-engine-ms 8081" "jokerarena-bot-ai-ms 8084" \
  "jokerarena-telemetry-ms 8085" "jokerarena-leaderboard-stats-ms 8086" \
  "jokerarena-liveops-rules-ms 8087" "jokerarena-cluster-monitor-ms 8088"; do
  read -r service target <<< "$pair"
  mapping=$($COMPOSE port "$service" "$target" 2>/dev/null)
  case "$mapping" in
    ""|:0) ok "$service:$target no se publica al host" ;;
    *) bad "$service:$target está publicado en $mapping" ;;
  esac
done
for pair in "jokerarena-postgres 5432" "jokerarena-mongo 27017" \
  "jokerarena-neo4j 7474" "jokerarena-rabbitmq 15672"; do
  read -r service target <<< "$pair"
  mapping=$($COMPOSE port "$service" "$target" 2>/dev/null)
  case "$mapping" in
    127.0.0.1:*) ok "$service:$target solo está enlazado a localhost" ;;
    *) bad "$service:$target debería estar enlazado solo a localhost (actual: $mapping)" ;;
  esac
done
https_code=$(curl -k -s -o /dev/null -w '%{http_code}' "$GW/health")
[ "$https_code" = "200" ] && ok "HTTPS responde en el proxy" || bad "HTTPS /health → $https_code"
headers=$(curl -sSI "$GW/")
echo "$headers" | grep -qi '^X-Content-Type-Options: nosniff' && ok "cabecera X-Content-Type-Options" || bad "falta X-Content-Type-Options"
echo "$headers" | grep -qi '^X-Frame-Options: SAMEORIGIN' && ok "cabecera X-Frame-Options" || bad "falta X-Frame-Options"

echo "5. Rutas a través del gateway ($GW)"
check() { # método ruta código-esperado [args curl...]
  local m=$1 p=$2 want=$3; shift 3
  local got; got=$(curl -s -o /dev/null -w '%{http_code}' -X "$m" "$GW$p" "$@")
  [ "$got" = "$want" ] && ok "$m $p → $got" || bad "$m $p → $got (esperado $want)"
}
check GET /health 200
alias="verify$RANDOM"
token=$(curl -s -X POST "$GW/api/auth/register" -H 'Content-Type: application/json' -d "{\"alias\":\"$alias\"}" | sed -n 's/.*"token":"\([^"]*\)".*/\1/p')
[ -n "$token" ] && ok "POST /api/auth/register → token JWT" || bad "no se obtuvo token"
check GET /api/auth/me 200 -H "Authorization: Bearer $token"
check GET /api/rooms 401
check GET /api/rooms 200 -H "Authorization: Bearer $token"
check POST /api/rooms 201 -H "Authorization: Bearer $token" -H 'Content-Type: application/json' -d '{"mode":"bot"}'
check GET /api/telemetry/matches 200 -H "Authorization: Bearer $token"
check GET "/api/leaderboard/top?limit=5" 200
check GET /api/game/ruleset 200
check GET /admin/liveops/ruleset 403 -H "Authorization: Bearer $token"
check GET /admin/cluster/snapshot 403 -H "Authorization: Bearer $token"

echo "6. Web SSR ($WEB)"
code=$(curl -s -o /tmp/jkr_lb.html -w '%{http_code}' "$WEB/leaderboard")
[ "$code" = "200" ] && grep -q "Salón de la fama" /tmp/jkr_lb.html && ok "/leaderboard renderizado en el servidor" || bad "/leaderboard → $code"

echo "7. Broker"
if $COMPOSE exec -T jokerarena-rabbitmq rabbitmqctl -q list_exchanges name | grep -q '^jokerarena.events$'; then ok "exchange jokerarena.events"; else bad "falta el exchange"; fi
for q in telemetry-ms.events leaderboard-stats-ms.players leaderboard-stats-ms.matches rooms-lobby-ms.game game-engine-ms.ruleset; do
  $COMPOSE exec -T jokerarena-rabbitmq rabbitmqctl -q list_queues name | grep -q "^$q$" && ok "cola $q" || bad "falta la cola $q"
done

echo
[ $fail -eq 0 ] && echo "✅ Todo en orden." || echo "❌ Hay verificaciones fallidas (ver arriba)."
exit $fail
