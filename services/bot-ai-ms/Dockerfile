# ==============================================================================
# bot-ai-ms (Rust / axum) - multi-etapa, binario release, usuario no-root
# ==============================================================================
FROM rust:1.83-slim AS build
WORKDIR /src
COPY Cargo.toml Cargo.lock ./
# Capa de dependencias en caché
RUN mkdir src && echo "fn main() {}" > src/main.rs && cargo build --release --locked && rm -rf src
COPY src ./src
RUN touch src/main.rs && cargo build --release --locked

FROM debian:bookworm-slim
RUN useradd --system --uid 10001 app
COPY --from=build /src/target/release/bot-ai-ms /usr/local/bin/bot-ai-ms
USER app
ENV PORT=8084
EXPOSE 8084
HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=5 CMD ["bot-ai-ms", "--healthcheck"]
ENTRYPOINT ["bot-ai-ms"]
