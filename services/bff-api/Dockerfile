# Builder — digest pinned for reproducibility (Phase 84).
# Upstream: https://hub.docker.com/_/golang — golang:1.26.2-alpine3.23
FROM golang:1.26.2-alpine3.23@sha256:f85330846cde1e57ca9ec309382da3b8e6ae3ab943d2739500e08c86393a21b1 AS builder

WORKDIR /workspace

# Copy go workspace files
COPY go.work go.work.sum ./
COPY pkg/ pkg/
COPY services/bff-api/ services/bff-api/
# Phase 118a / ADR-024: the Language Capability Manifest is authored once
# in the analysis worker's configs and consumed by both the worker and the
# BFF (`?language=` validator). Copying it in at build time keeps the
# manifest a single SoT.
COPY services/analysis-worker/configs/language_capabilities.yaml services/bff-api/configs/language_capabilities.yaml
# Workspace stubs: go.mod required for all modules listed in go.work
COPY services/ingestion-api/go.mod services/ingestion-api/go.mod
COPY crawlers/rss-crawler/go.mod crawlers/rss-crawler/go.mod

WORKDIR /workspace/services/bff-api

RUN go mod download && \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /bin/bff-api ./cmd/server/main.go

# Runtime — digest pinned for reproducibility (Phase 84).
# Upstream: https://hub.docker.com/_/alpine — alpine:3.23.3
FROM alpine:3.23.3@sha256:25109184c71bdad752c8312a8623239686a9a2071e8825f20acb8f2198c3f659

RUN apk upgrade --no-cache zlib libcrypto3 libssl3 musl musl-utils \
    && adduser -S -D -H -u 10001 aer

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /bin/bff-api /app/bff-api
COPY --from=builder /workspace/services/bff-api/configs /app/configs

WORKDIR /app

USER aer

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO- http://localhost:8080/api/v1/healthz || exit 1

ENTRYPOINT ["/app/bff-api"]
