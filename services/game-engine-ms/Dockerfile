# ==============================================================================
# game-engine-ms (Go) - imagen multi-etapa, usuario no-root
# ==============================================================================
FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /out/game-engine .

FROM alpine:3.20
RUN addgroup -S -g 10001 app && adduser -S -u 10001 -G app -h /app app
WORKDIR /app
COPY --from=build /out/game-engine /app/game-engine
USER app
EXPOSE 8081
HEALTHCHECK --interval=10s --timeout=3s --start-period=10s --retries=5 \
  CMD wget -qO- http://127.0.0.1:8081/health >/dev/null || exit 1
ENTRYPOINT ["/app/game-engine"]
