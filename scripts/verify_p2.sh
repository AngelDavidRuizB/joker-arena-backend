#!/usr/bin/env bash
# ==============================================================================
# JokerArena P2 · verificación del despliegue completo
#   bash scripts/verify_p2.sh          verifica un sistema ya levantado
#   bash scripts/verify_p2.sh --up     lo levanta primero (build + wait)
# ==============================================================================
set -u
cd "$(dirname "$0")/.."
COMPOSE="docker compose -f docker-compose.p2.yml"
GW="http://localhost:${GATEWAY_PORT:-8080}"
WEB="http://localhost:${WEB_PORT:-3000}"
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
[ "$n" -ge 15 ] && ok "$n contenedores (8 lógica + gateway + web + 4 datos + broker)" || bad "se esperaban 15 contenedores, hay $n"

echo "4. Rutas a través del gateway ($GW)"
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

echo "5. Web SSR ($WEB)"
code=$(curl -s -o /tmp/jkr_lb.html -w '%{http_code}' "$WEB/leaderboard")
[ "$code" = "200" ] && grep -q "Salón de la fama" /tmp/jkr_lb.html && ok "/leaderboard renderizado en el servidor" || bad "/leaderboard → $code"

echo "6. Broker"
if $COMPOSE exec -T jokerarena-rabbitmq rabbitmqctl -q list_exchanges name | grep -q '^jokerarena.events$'; then ok "exchange jokerarena.events"; else bad "falta el exchange"; fi
for q in telemetry-ms.events leaderboard-stats-ms.players leaderboard-stats-ms.matches rooms-lobby-ms.game game-engine-ms.ruleset; do
  $COMPOSE exec -T jokerarena-rabbitmq rabbitmqctl -q list_queues name | grep -q "^$q$" && ok "cola $q" || bad "falta la cola $q"
done

echo
[ $fail -eq 0 ] && echo "✅ Todo en orden." || echo "❌ Hay verificaciones fallidas (ver arriba)."
exit $fail
