# Builder — digest pinned for reproducibility (Phase 84).
# Upstream: https://hub.docker.com/_/golang — golang:1.26.2-alpine3.23
FROM golang:1.26.2-alpine3.23@sha256:c2a1f7b2095d046ae14b286b18413a05bb82c9bca9b25fe7ff5efef0f0826166 AS builder

WORKDIR /workspace

# Crawler module is self-contained (no `pkg/` dependency), so we copy
# only its source tree — no go.work stubs needed, unlike bff-api and
# ingestion-api.
COPY crawlers/rss-crawler/ ./

RUN go mod download && \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /bin/rss-crawler .

# Runtime — digest pinned for reproducibility (Phase 84).
# Upstream: https://hub.docker.com/_/alpine — alpine:3.23.3
FROM alpine:3.23.3@sha256:25109184c71bdad752c8312a8623239686a9a2071e8825f20acb8f2198c3f659

RUN apk upgrade --no-cache zlib libcrypto3 libssl3 musl musl-utils \
    && adduser -S -D -H -u 10001 aer \
    && mkdir -p /state \
    && chown aer:nogroup /state

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /bin/rss-crawler /rss-crawler
COPY crawlers/rss-crawler/feeds.yaml /app/feeds.yaml

WORKDIR /app

USER aer

# One-shot job: no long-running process to probe. Explicit NONE keeps
# the Hard Rule 2 "no log-parsing healthchecks" intent satisfied while
# signalling that a liveness probe would be meaningless here.
HEALTHCHECK NONE

ENTRYPOINT ["/rss-crawler"]
CMD ["-config", "/app/feeds.yaml", "-state", "/state/rss-crawler-state.json"]
