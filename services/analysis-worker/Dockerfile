FROM python:3.14.3-slim-bookworm AS builder

WORKDIR /app

COPY services/analysis-worker/requirements.txt .

RUN pip install --no-cache-dir --prefix=/install -r requirements.txt

FROM python:3.14.3-slim-bookworm

WORKDIR /app

COPY --from=builder /install /usr/local
COPY services/analysis-worker/ .

ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1

ENTRYPOINT ["python", "main.py"]
