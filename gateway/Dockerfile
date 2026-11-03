# JokerArena P2 · API Gateway (nginx, usuario no-root, puerto 8080)
FROM nginx:1.27-alpine
COPY nginx.conf /etc/nginx/nginx.conf
USER nginx
EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=5 \
  CMD wget -qO- http://127.0.0.1:8080/health >/dev/null || exit 1
CMD ["nginx", "-g", "daemon off;"]
