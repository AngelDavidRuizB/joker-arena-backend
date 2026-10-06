# ==============================================================================
# Stage 1: Builder
# ==============================================================================
FROM golang:1.22-alpine AS builder

WORKDIR /build

# Install CA certificates and tzdata for TLS validation and correct timezone handling
RUN apk add --no-cache ca-certificates tzdata git

# Cache Go dependency downloads using layer caching
COPY go.mod go.sum* ./
RUN go mod download

# Copy application source code
COPY . .

# Compile static Linux binary without CGO and stripped of debug symbols
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s" \
    -o /build/game-engine .

# ==============================================================================
# Stage 2: Runtime
# ==============================================================================
# alpine:3.19 is strictly required to provide BusyBox wget for docker-compose healthchecks
FROM alpine:3.19

# Install base security certificates and timezone database
RUN apk add --no-cache ca-certificates tzdata

# Create dedicated unprivileged non-root user and group
RUN addgroup -S -g 10001 appgroup && \
    adduser -S -u 10001 -G appgroup -h /app -s /bin/sh appuser

WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /build/game-engine /app/game-engine

# Assign ownership to unprivileged user
RUN chown -R appuser:appgroup /app

# Switch to non-root user
USER appuser:appgroup

# Expose Game Engine HTTP REST and WebSocket port
EXPOSE 8080

# Run Game Engine binary
ENTRYPOINT ["/app/game-engine"]
