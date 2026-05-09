# Builder — digest pinned for reproducibility (Phase 84).
# Upstream: https://hub.docker.com/_/python — python:3.14.3-slim-bookworm
FROM python:3.14.3-slim-bookworm@sha256:f21c0d5a44c56805654c15abccc1b2fd576c8d93aca0a3f74b4aba2dc92510e2 AS builder

WORKDIR /app

# Build toolchain — Scrapy pulls Twisted, lxml, cryptography. No cp314
# manylinux wheels for some of these (early 2026), so pip falls back to
# sdist and needs C/C++ + libxml2/libxslt + libssl headers.
RUN apt-get update && apt-get install -y --no-install-recommends \
    gcc g++ make \
    libpq-dev python3-dev \
    libxml2-dev libxslt1-dev \
    libssl-dev libffi-dev \
    pkg-config \
    && rm -rf /var/lib/apt/lists/*

COPY crawlers/web-crawler/pyproject.toml .

# Install dependencies declared in pyproject.toml. Scrapy + ultimate-
# sitemap-parser + feedparser + psycopg2 + courlan are the only runtime
# deps; everything else (trafilatura, extruct, htmldate, readability) is
# a worker-side concern per ADR-028.
#
# BuildKit cache mount for pip's HTTP/wheel cache — without it, every
# rebuild re-downloads Twisted/lxml/cryptography sdists (the cp314 wheel
# gap pushes these to source). Cache lives outside the image; the layer
# remains a deterministic function of source.
RUN --mount=type=cache,target=/root/.cache/pip,id=aer-web-crawler-pip \
    pip install --prefix=/install \
        "scrapy==2.13.3" \
        "ultimate-sitemap-parser==1.4.0" \
        "feedparser==6.0.11" \
        "psycopg2-binary==2.9.11" \
        "PyYAML==6.0.2" \
        "requests==2.33.0" \
        "structlog==25.5.0" \
        "courlan==1.3.2"

# Runtime — digest pinned for reproducibility (Phase 84).
FROM python:3.14.3-slim-bookworm@sha256:f21c0d5a44c56805654c15abccc1b2fd576c8d93aca0a3f74b4aba2dc92510e2

WORKDIR /app

RUN apt-get update \
    && apt-get install -y --only-upgrade libssl3 openssl \
    && apt-get install -y --no-install-recommends libpq5 libxml2 libxslt1.1 \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd -r aer --gid 10001 \
    && useradd -r -g aer --uid 10001 --no-create-home --shell /usr/sbin/nologin aer

COPY --from=builder /install /usr/local
COPY --chown=aer:aer crawlers/web-crawler/main.py ./
COPY --chown=aer:aer crawlers/web-crawler/internal ./internal
COPY --chown=aer:aer crawlers/web-crawler/probes ./probes

USER aer

ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    PYTHONPATH=/app

# One-shot job: no long-running process to probe. Explicit NONE keeps
# the Hard Rule 2 "no log-parsing healthchecks" intent satisfied while
# signalling that a liveness probe would be meaningless here.
HEALTHCHECK NONE

ENTRYPOINT ["python", "main.py"]
CMD ["--probe", "probe0"]
