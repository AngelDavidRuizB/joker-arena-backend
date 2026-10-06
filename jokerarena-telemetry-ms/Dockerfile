# ==============================================================================
# Python 3.12 Slim Runtime for Telemetry & Analytics Microservice
# ==============================================================================
FROM python:3.12-slim

# Prevent Python from writing bytecode (.pyc) and enable unbuffered output for live logging
ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    PYTHONPATH=/app:/app/proto

WORKDIR /app

# Install minimal OS dependencies if needed, keeping apt cache clean
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Create unprivileged system user
RUN groupadd -g 10001 appgroup && \
    useradd -u 10001 -g appgroup -d /app -s /bin/bash appuser

# Install Python dependencies using Pip caching layer
COPY requirements.txt .
RUN pip install --no-cache-dir --upgrade pip && \
    pip install --no-cache-dir -r requirements.txt

# Copy microservice application source code
COPY . .

# Assign file ownership to unprivileged user
RUN chown -R appuser:appgroup /app

# Switch to unprivileged runtime user
USER appuser:appgroup

# Expose HTTP REST / OpenAPI Docs (8000) and gRPC Ingestion (50051)
EXPOSE 8000 50051

# Entrypoint runs the dual-protocol server (FastAPI HTTP + gRPC Server)
CMD ["python", "main.py"]
