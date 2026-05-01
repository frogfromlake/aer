# Builder — digest pinned for reproducibility (Phase 84).
# Upstream: https://hub.docker.com/_/golang — golang:1.26.2-alpine3.23
FROM golang:1.26.2-alpine3.23@sha256:f85330846cde1e57ca9ec309382da3b8e6ae3ab943d2739500e08c86393a21b1 AS builder

WORKDIR /workspace

# Copy go workspace files
COPY go.work go.work.sum ./
COPY pkg/ pkg/
COPY services/ingestion-api/ services/ingestion-api/
# Workspace stubs: go.mod required for all modules listed in go.work
COPY services/bff-api/go.mod services/bff-api/go.mod
COPY crawlers/rss-crawler/go.mod crawlers/rss-crawler/go.mod

WORKDIR /workspace/services/ingestion-api

RUN go mod download && \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /bin/ingestion-api ./cmd/api/main.go

# Runtime — digest pinned for reproducibility (Phase 84).
# Upstream: https://hub.docker.com/_/alpine — alpine:3.23.3
FROM alpine:3.23.3@sha256:25109184c71bdad752c8312a8623239686a9a2071e8825f20acb8f2198c3f659

RUN apk upgrade --no-cache zlib libcrypto3 libssl3 musl musl-utils \
    && adduser -S -D -H -u 10001 aer

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /bin/ingestion-api /ingestion-api
COPY infra/postgres/migrations/ /migrations/

USER aer

EXPOSE 8081

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO- http://localhost:8081/api/v1/healthz || exit 1

ENTRYPOINT ["/ingestion-api"]
