# ---------------------------------------------------------------------------
# Stage 1: hf-prefetch — Tier-2 BERT + BERTopic embedding model download.
#
# Deliberately split from the Python `builder` stage so this layer only
# invalidates when the prefetch script or the Language Capability Manifest
# change. The previous layout chained HF prefetch *after* `pip install`,
# meaning every `requirements.lock.txt` bump re-ran `cp -a` of 10+ GB into
# a new builder layer — and the runtime stage's `COPY --from=builder
# /hf-cache` then re-copied that 10 GB into the runtime image.
#
# The script's only runtime deps are huggingface_hub + PyYAML, pinned to
# the same versions as `requirements.lock.txt` so the toolchain that
# downloads the weights matches what the worker uses at runtime. Hashes
# aren't required here: this stage's output is the model bytes themselves,
# which are independently revision-pinned in the manifest — supply-chain
# integrity is anchored on the model revisions, not on the downloader.
# ---------------------------------------------------------------------------
FROM python:3.14.3-slim-bookworm@sha256:f21c0d5a44c56805654c15abccc1b2fd576c8d93aca0a3f74b4aba2dc92510e2 AS hf-prefetch

WORKDIR /tmp/prefetch

RUN --mount=type=cache,target=/root/.cache/pip,id=aer-hf-prefetch-pip \
    pip install \
        'huggingface_hub==0.36.2' \
        'PyYAML==6.0.2'

ENV HF_HOME=/hf-cache \
    HF_HUB_DOWNLOAD_TIMEOUT=300

COPY services/analysis-worker/configs/language_capabilities.yaml /tmp/prefetch/language_capabilities.yaml
COPY services/analysis-worker/scripts/prefetch_bert_models.py /tmp/prefetch/prefetch_bert_models.py

# BuildKit cache mount for the HF download cache. Bumping a single model
# revision in the manifest re-downloads only the changed model; the rest
# come from cache. The `cp -a` materialises the cache into a regular
# directory so the runtime stage can `COPY --from=hf-prefetch` it.
RUN --mount=type=cache,target=/hf-build-cache,id=aer-analysis-worker-hf-models \
    mkdir -p /hf-cache \
    && python /tmp/prefetch/prefetch_bert_models.py \
        /tmp/prefetch/language_capabilities.yaml \
        /hf-build-cache \
    && cp -a /hf-build-cache/. /hf-cache/

# ---------------------------------------------------------------------------
# Stage 2: builder — Python deps (pip) + SentiWS lexicon download.
#
# Digest pinned for reproducibility (Phase 84).
# Upstream: https://hub.docker.com/_/python — python:3.14.3-slim-bookworm
# ---------------------------------------------------------------------------
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
    libxml2-dev libxslt1-dev \
    pkg-config ninja-build \
    && rm -rf /var/lib/apt/lists/*

COPY services/analysis-worker/requirements.lock.txt .

# BuildKit cache mount for pip's HTTP/wheel cache. Without this, every layer
# rebuild re-downloads every wheel — including the 567 MB
# de_core_news_lg-3.8.0 spaCy model and the heavy scientific stack
# (numpy/scipy/scikit-learn/hdbscan sdists). The cache is content-addressed
# by URL+hash, so --require-hashes verification still runs against every
# artefact (cached or freshly downloaded). The cache lives outside the
# image — image determinism is unchanged.
RUN --mount=type=cache,target=/root/.cache/pip,id=aer-analysis-worker-pip \
    pip install --prefix=/install --require-hashes -r requirements.lock.txt

# Download SentiWS lexicon (CC-BY-SA 4.0, Leipzig University).
# Hash-pinned (Phase 84) — supply-chain integrity for a build-time download.
# SENTIWS_URL is overridable (compose build arg / .env) so a byte-identical
# fallback mirror (e.g. an Internet Archive snapshot) can be used when the
# canonical Leipzig server is down; the SHA256 check below guarantees integrity
# regardless of source. The wget is retried (flaky academic mirror).
ARG SENTIWS_SHA256=4dd6ce99a44b5122c04fe0e7ca36db1f94d738201095edabb0a16cc98f160b91
ARG SENTIWS_URL=https://downloads.wortschatz-leipzig.de/etc/SentiWS/SentiWS_v2.0.zip
RUN apt-get update && apt-get install -y --no-install-recommends unzip wget \
    && mkdir -p /sentiws \
    && wget -q --tries=5 --waitretry=10 --timeout=30 --retry-connrefused \
        -O /tmp/sentiws.zip "${SENTIWS_URL}" \
    && echo "${SENTIWS_SHA256}  /tmp/sentiws.zip" | sha256sum -c - \
    && unzip /tmp/sentiws.zip -d /sentiws \
    && rm /tmp/sentiws.zip \
    && rm -rf /var/lib/apt/lists/*

# Runtime — digest pinned for reproducibility (Phase 84).
FROM python:3.14.3-slim-bookworm@sha256:f21c0d5a44c56805654c15abccc1b2fd576c8d93aca0a3f74b4aba2dc92510e2

WORKDIR /app

RUN apt-get update \
    && apt-get install -y --only-upgrade libssl3 openssl \
    && apt-get install -y --no-install-recommends libpq5 libgomp1 libxml2 libxslt1.1 \
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
COPY --chown=aer:aer --from=hf-prefetch /hf-cache /hf-cache

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
