#!/bin/sh
set -eu

cert=/etc/nginx/certs/fullchain.pem
key=/etc/nginx/certs/privkey.pem

if [ -s "$cert" ] && [ -s "$key" ]; then
    exit 0
fi

if [ -e "$cert" ] || [ -e "$key" ]; then
    echo "Both fullchain.pem and privkey.pem must exist and be non-empty." >&2
    exit 1
fi

mkdir -p /etc/nginx/certs
openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
    -keyout "$key" \
    -out "$cert" \
    -subj "/CN=localhost" \
    -addext "subjectAltName=DNS:localhost,IP:127.0.0.1"
chmod 600 "$key"
chmod 644 "$cert"
echo "Generated a local self-signed certificate. Replace it with a trusted certificate outside development." >&2
