# ==============================================================================
# telemetry-ms (Python 3.12 / FastAPI + Motor + aio-pika) - multi-etapa, no-root
# ==============================================================================
FROM python:3.12-slim AS build
WORKDIR /build
COPY requirements.txt .
RUN pip install --no-cache-dir --prefix=/install -r requirements.txt

FROM python:3.12-slim
ENV PYTHONDONTWRITEBYTECODE=1 PYTHONUNBUFFERED=1
COPY --from=build /install /usr/local
RUN useradd --system --uid 10001 app
WORKDIR /app
COPY app ./app
USER app
EXPOSE 8085
HEALTHCHECK --interval=10s --timeout=3s --start-period=10s --retries=5 \
  CMD python -c "import urllib.request; urllib.request.urlopen('http://127.0.0.1:8085/health', timeout=2)"
CMD ["uvicorn", "app.main:app", "--host", "0.0.0.0", "--port", "8085", "--no-access-log"]
