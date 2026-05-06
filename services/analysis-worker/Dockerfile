# Builder — digest pinned for reproducibility (Phase 84).
# Upstream: https://hub.docker.com/_/python — python:3.14.3-slim-bookworm
FROM python:3.14.3-slim-bookworm@sha256:f21c0d5a44c56805654c15abccc1b2fd576c8d93aca0a3f74b4aba2dc92510e2 AS builder

WORKDIR /app

# Build toolchain. Phase 120 added BERTopic, which transitively pulls
# scikit-learn, scipy, hdbscan and numpy — none of which ship cp314
# manylinux wheels yet (early 2026), so pip falls back to sdist and
# Meson/CMake need a full C/C++/Fortran toolchain plus BLAS/LAPACK
# headers. The runtime image (final FROM stage below) does NOT carry
# these — only the compiled wheels copied from /install. Drop these
# build-deps once cp314 wheels become available upstream.
RUN apt-get update && apt-get install -y --no-install-recommends \
    gcc g++ gfortran make \
    libpq-dev python3-dev \
    libopenblas-dev liblapack-dev \
    pkg-config ninja-build \
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

# Phase 119 (ADR-023) / Phase 120 — Pre-download Tier-2 BERT sentiment models
# AND the BERTopic embedding model (intfloat/multilingual-e5-large, ~2.2 GB)
# so the runtime image is self-contained and can run with
# TRANSFORMERS_OFFLINE=1. Revisions are read from the Language Capability
# Manifest (the same SoT the extractors use at construction time per
# ADR-024), keeping supply-chain integrity consistent with SentiWS
# (hash-pinned above) and the spaCy model wheels. The prefetch script
# retries with backoff to survive transient HuggingFace timeouts.
ENV HF_HOME=/hf-cache \
    HF_HUB_DOWNLOAD_TIMEOUT=300 \
    PYTHONPATH=/install/lib/python3.14/site-packages
COPY services/analysis-worker/configs/language_capabilities.yaml /tmp/language_capabilities.yaml
COPY services/analysis-worker/scripts/prefetch_bert_models.py /tmp/prefetch_bert_models.py
# BuildKit cache mount keeps downloaded model weights between rebuilds on
# the same host — bumping a single model revision then redownloads only
# the changed model. The cache does not affect the resulting image layer:
# weights are copied into /hf-cache regardless of cache hit/miss, so the
# image is a deterministic function of source.
RUN --mount=type=cache,target=/hf-build-cache,id=aer-analysis-worker-hf-models \
    mkdir -p /hf-cache \
    && python /tmp/prefetch_bert_models.py /tmp/language_capabilities.yaml /hf-build-cache \
    && cp -a /hf-build-cache/. /hf-cache/

# Runtime — digest pinned for reproducibility (Phase 84).
FROM python:3.14.3-slim-bookworm@sha256:f21c0d5a44c56805654c15abccc1b2fd576c8d93aca0a3f74b4aba2dc92510e2

WORKDIR /app

RUN apt-get update \
    && apt-get install -y --only-upgrade libssl3 openssl \
    && apt-get install -y --no-install-recommends libpq5 libgomp1 \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd -r aer --gid 10001 \
    && useradd -r -g aer --uid 10001 --no-create-home --shell /usr/sbin/nologin aer

COPY --from=builder /install /usr/local
# Phase 120b: --chown on COPY instead of a follow-up `RUN chown -R`. The
# /hf-cache directory is ~10 GB; chowning it via a separate RUN layer
# triggers overlayfs copy-on-write of every file and *doubles* the image
# size (one layer of root-owned files + a second layer of aer-owned
# duplicates). --chown applies the ownership during the COPY, producing a
# single layer.
COPY --chown=aer:aer services/analysis-worker/ .
COPY --chown=aer:aer --from=builder /sentiws /app/data/sentiws
COPY --chown=aer:aer --from=builder /hf-cache /hf-cache

USER aer

ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    HF_HOME=/hf-cache \
    TRANSFORMERS_OFFLINE=1 \
    HF_HUB_OFFLINE=1

EXPOSE 8001

HEALTHCHECK --interval=30s --timeout=5s --start-period=30s --retries=3 \
    CMD python -c "import urllib.request,sys; sys.exit(0) if urllib.request.urlopen('http://localhost:8001/metrics', timeout=3).status == 200 else sys.exit(1)" || exit 1

ENTRYPOINT ["python", "main.py"]
