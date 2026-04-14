# Builder — digest pinned for reproducibility (Phase 84).
# Upstream: https://hub.docker.com/_/python — python:3.14.3-slim-bookworm
FROM python:3.14.3-slim-bookworm@sha256:f21c0d5a44c56805654c15abccc1b2fd576c8d93aca0a3f74b4aba2dc92510e2 AS builder

WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends \
    gcc \
    libpq-dev \
    python3-dev \
    && rm -rf /var/lib/apt/lists/*

COPY services/analysis-worker/requirements.lock.txt .

RUN pip install --no-cache-dir --prefix=/install --require-hashes -r requirements.lock.txt

# Download SentiWS lexicon (CC-BY-SA 4.0, Leipzig University).
# Hash-pinned (Phase 84) — supply-chain integrity for a build-time download.
ARG SENTIWS_SHA256=4dd6ce99a44b5122c04fe0e7ca36db1f94d738201095edabb0a16cc98f160b91
RUN apt-get update && apt-get install -y --no-install-recommends unzip wget \
    && mkdir -p /sentiws \
    && wget -q -O /tmp/sentiws.zip https://downloads.wortschatz-leipzig.de/etc/SentiWS/SentiWS_v2.0.zip \
    && echo "${SENTIWS_SHA256}  /tmp/sentiws.zip" | sha256sum -c - \
    && unzip /tmp/sentiws.zip -d /sentiws \
    && rm /tmp/sentiws.zip \
    && rm -rf /var/lib/apt/lists/*

# Runtime — digest pinned for reproducibility (Phase 84).
FROM python:3.14.3-slim-bookworm@sha256:f21c0d5a44c56805654c15abccc1b2fd576c8d93aca0a3f74b4aba2dc92510e2

WORKDIR /app

RUN apt-get update \
    && apt-get install -y --only-upgrade libssl3 openssl \
    && apt-get install -y --no-install-recommends libpq5 \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd -r aer --gid 10001 \
    && useradd -r -g aer --uid 10001 --no-create-home --shell /usr/sbin/nologin aer

COPY --from=builder /install /usr/local
COPY services/analysis-worker/ .
COPY --from=builder /sentiws /app/data/sentiws

RUN chown -R aer:aer /app

USER aer

ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1

EXPOSE 8001

HEALTHCHECK --interval=30s --timeout=5s --start-period=30s --retries=3 \
    CMD python -c "import urllib.request,sys; sys.exit(0) if urllib.request.urlopen('http://localhost:8001/metrics', timeout=3).status == 200 else sys.exit(1)" || exit 1

ENTRYPOINT ["python", "main.py"]
