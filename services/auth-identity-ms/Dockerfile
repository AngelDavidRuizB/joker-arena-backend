# ==============================================================================
# auth-identity-ms (Python 3.12 / FastAPI) - multi-etapa, usuario no-root
# ==============================================================================
FROM python:3.12-slim AS build
WORKDIR /build
COPY requirements.txt .
RUN pip install --no-cache-dir --prefix=/install -r requirements.txt

FROM python:3.12-slim
ENV PYTHONDONTWRITEBYTECODE=1 PYTHONUNBUFFERED=1 PORT=8082
COPY --from=build /install /usr/local
RUN useradd --system --uid 10001 --create-home app && mkdir -p /data && chown app /data
WORKDIR /app
COPY app ./app
USER app
EXPOSE 8082
HEALTHCHECK --interval=10s --timeout=3s --start-period=10s --retries=5 \
  CMD python -c "import urllib.request; urllib.request.urlopen('http://127.0.0.1:8082/health', timeout=2)"
CMD ["uvicorn", "app.main:app", "--host", "0.0.0.0", "--port", "8082", "--no-access-log"]
