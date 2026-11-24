#!/usr/bin/env bash
# ==============================================================================
# JokerArena P3 · Escenario 1 (Secure Channel)
# Genera una CA local y el certificado TLS del Reverse Proxy en edge/certs/.
# Uso:
#   bash scripts/gen_certs.sh           genera los certificados si no existen
#   bash scripts/gen_certs.sh --force   regenera la CA y el certificado del servidor
# Variables opcionales:
#   CERTS_DIR=ruta    directorio de salida (por defecto: edge/certs)
#   EXTRA_SANS=...    SAN adicionales separados por coma (ej. DNS:mi-host,IP:192.168.1.10)
# ==============================================================================
set -euo pipefail

cd "$(dirname "$0")/.."

OUT_DIR="${CERTS_DIR:-edge/certs}"
FORCE=0

for arg in "$@"; do
  case "$arg" in
    --force)
      FORCE=1
      ;;
    -h|--help)
      echo "Uso: bash scripts/gen_certs.sh [--force]"
      exit 0
      ;;
    *)
      echo "Argumento desconocido: $arg" >&2
      echo "Uso: bash scripts/gen_certs.sh [--force]" >&2
      exit 1
      ;;
  esac
done

if ! command -v openssl >/dev/null 2>&1; then
  echo "Error: no se encontró 'openssl' en el PATH." >&2
  exit 1
fi

CA_KEY="$OUT_DIR/jokerarena-ca.key"
CA_CRT="$OUT_DIR/jokerarena-ca.crt"
SRV_KEY="$OUT_DIR/server.key"
SRV_CRT="$OUT_DIR/server.crt"

if [ "$FORCE" -eq 0 ] && [ -f "$CA_CRT" ] && [ -f "$SRV_CRT" ] && [ -f "$SRV_KEY" ]; then
  echo "Los certificados ya existen en $OUT_DIR (usa --force para regenerarlos)."
  openssl verify -CAfile "$CA_CRT" "$SRV_CRT"
  exit 0
fi

mkdir -p "$OUT_DIR"
TMP_DIR="$(mktemp -d "$OUT_DIR/.tmp.XXXXXX")"
trap 'rm -rf "$TMP_DIR"' EXIT

if [ "$FORCE" -eq 1 ] || [ ! -f "$CA_KEY" ] || [ ! -f "$CA_CRT" ]; then
  echo "Generando CA local en $CA_CRT ..."
  cat > "$TMP_DIR/ca.cnf" <<'EOF'
[req]
distinguished_name = dn
x509_extensions = v3_ca
prompt = no

[dn]
C = CO
O = JokerArena
OU = Equipo 2b
CN = JokerArena Local Root CA

[v3_ca]
subjectKeyIdentifier = hash
authorityKeyIdentifier = keyid:always,issuer
basicConstraints = critical, CA:TRUE
keyUsage = critical, digitalSignature, cRLSign, keyCertSign
EOF

  openssl genrsa -out "$CA_KEY" 4096 2>/dev/null
  openssl req -x509 -new -nodes -key "$CA_KEY" -sha256 -days 3650 \
    -config "$TMP_DIR/ca.cnf" -out "$CA_CRT"
  chmod 600 "$CA_KEY"
  chmod 644 "$CA_CRT"
fi

SAN_LIST="DNS:localhost, DNS:jokerarena-reverse-proxy, IP:127.0.0.1, IP:::1"
if [ -n "${EXTRA_SANS:-}" ]; then
  SAN_LIST="${SAN_LIST}, ${EXTRA_SANS}"
fi

cat > "$TMP_DIR/server.ext" <<EOF
basicConstraints = CA:FALSE
keyUsage = critical, digitalSignature, keyEncipherment
extendedKeyUsage = serverAuth
subjectKeyIdentifier = hash
authorityKeyIdentifier = keyid,issuer
subjectAltName = ${SAN_LIST}
EOF

echo "Generando certificado TLS del servidor en $SRV_CRT ..."
openssl genrsa -out "$SRV_KEY" 2048 2>/dev/null
MSYS_NO_PATHCONV=1 openssl req -new -key "$SRV_KEY" \
  -subj "/C=CO/O=JokerArena/OU=Equipo 2b/CN=localhost" \
  -out "$TMP_DIR/server.csr"
openssl x509 -req -in "$TMP_DIR/server.csr" \
  -CA "$CA_CRT" -CAkey "$CA_KEY" -CAserial "$TMP_DIR/jokerarena-ca.srl" -CAcreateserial \
  -out "$SRV_CRT" -days 365 -sha256 -extfile "$TMP_DIR/server.ext"

# El reverse proxy nginx corre con usuario no-root y monta edge/certs en solo lectura (:ro);
# en Linux necesita permiso de lectura para ese UID. Nunca se versiona en Git (.gitignore).
chmod 644 "$SRV_KEY" "$SRV_CRT"

echo "Verificando cadena de confianza..."
openssl verify -CAfile "$CA_CRT" "$SRV_CRT"

echo
echo "Certificados listos en $OUT_DIR/:"
echo "  - CA pública : $CA_CRT"
echo "  - Llave CA   : $CA_KEY"
echo "  - Certificado: $SRV_CRT"
echo "  - Llave srv  : $SRV_KEY"
echo
echo "Pruebas sugeridas:"
echo "  curl --cacert $CA_CRT https://localhost/health"
echo "  JOKERARENA_CA=$CA_CRT jokerarena leaderboard"
