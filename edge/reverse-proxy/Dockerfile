FROM nginx:1.27-alpine

RUN apk add --no-cache openssl
COPY nginx.conf /etc/nginx/nginx.conf
COPY snippets/ /etc/nginx/snippets/
COPY generate-dev-certificate.sh /docker-entrypoint.d/05-generate-dev-certificate.sh
RUN chmod +x /docker-entrypoint.d/05-generate-dev-certificate.sh

EXPOSE 80 443
HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=5 \
  CMD wget -qO- http://127.0.0.1/__edge_health >/dev/null || exit 1
CMD ["nginx", "-g", "daemon off;"]
