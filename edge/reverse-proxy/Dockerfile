# JokerArena P3 · Reverse Proxy (Escenario 2) + terminación TLS (Escenario 1)
FROM nginx:1.27-alpine

RUN rm -f /etc/nginx/conf.d/default.conf
COPY nginx.conf /etc/nginx/nginx.conf
COPY conf.d/ /etc/nginx/conf.d/
COPY snippets/ /etc/nginx/snippets/
COPY locations.conf /etc/nginx/locations.conf

EXPOSE 80 443
HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=5 \
  CMD wget -qO- http://127.0.0.1/__edge_health >/dev/null || exit 1
CMD ["nginx", "-g", "daemon off;"]
