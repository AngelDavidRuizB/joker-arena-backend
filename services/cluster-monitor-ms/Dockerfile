# ==============================================================================
# cluster-monitor-ms (Go) - multi-etapa, usuario no-root
# Para leer el socket de Docker, compose agrega el grupo dueño del socket (group_add).
# ==============================================================================
FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-w -s" -o /out/cluster-monitor .

FROM alpine:3.20
RUN adduser -S -u 10001 app
COPY --from=build /out/cluster-monitor /usr/local/bin/cluster-monitor
USER app
EXPOSE 8088
HEALTHCHECK --interval=10s --timeout=3s --start-period=10s --retries=5 CMD ["cluster-monitor", "--healthcheck"]
ENTRYPOINT ["cluster-monitor"]
