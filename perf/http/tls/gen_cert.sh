#!/usr/bin/env bash
# ==============================================================================
# JokerArena P3 · D2 · Certificado autofirmado SOLO para el banco de TLS.
# Mismo tipo de llave que el del Reverse Proxy (RSA 2048, SHA-256), para que el
# costo del handshake sea comparable. No se versiona (perf/http/.gitignore).
# Si ya existe edge/certs/ (scripts/gen_certs.sh del Escenario 1), se reutiliza.
# ==============================================================================
set -euo pipefail
cd "$(dirname "$0")"
mkdir -p certs
if [ -f ../../../edge/certs/server.crt ] && [ -f ../../../edge/certs/server.key ]; then
  cp ../../../edge/certs/server.crt ../../../edge/certs/server.key certs/
  echo "Usando el certificado de edge/certs/ (Escenario 1)."
  exit 0
fi
MSYS_NO_PATHCONV=1 openssl req -x509 -newkey rsa:2048 -sha256 -nodes -days 30 \
  -keyout certs/server.key -out certs/server.crt \
  -subj "/C=CO/O=JokerArena/OU=Equipo 2b/CN=localhost" \
  -addext "subjectAltName=DNS:localhost,IP:127.0.0.1" 2>/dev/null
chmod 644 certs/server.key certs/server.crt
echo "Certificado de prueba generado en perf/http/tls/certs/"
