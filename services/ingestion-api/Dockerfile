# Builder — digest pinned for reproducibility (Phase 84).
# Upstream: https://hub.docker.com/_/golang — golang:1.26.4-alpine3.23
FROM golang:1.26.4-alpine3.23@sha256:f23e8b227fb4493eabe03bede4d5a32d04092da71962f1fb79b5f7d1e6c2a17f AS builder

WORKDIR /workspace

# Stage A — module graph only. Copying go.mod/go.sum (and workspace stubs)
# *before* the source tree means `go mod download` only re-runs when the
# dependency graph itself changes — code edits do not invalidate this layer.
COPY go.work go.work.sum ./
COPY pkg/go.mod pkg/go.sum pkg/
COPY services/ingestion-api/go.mod services/ingestion-api/go.sum services/ingestion-api/
# Workspace stubs: every module listed in go.work needs at least its go.mod.
COPY services/bff-api/go.mod services/bff-api/go.mod

# BuildKit cache mounts for $GOMODCACHE and $GOCACHE survive across builds,
# so module downloads and compiled-package artefacts persist even after
# layer invalidation. Without these, every dep change forces a full
# re-download AND re-compilation of the dependency graph from scratch.
RUN --mount=type=cache,target=/go/pkg/mod,id=aer-go-modcache \
    --mount=type=cache,target=/root/.cache/go-build,id=aer-go-buildcache \
    cd services/ingestion-api && go mod download

# Stage B — source. Copied after dep download so this is the only layer
# affected by code edits. The actual `go build` reuses the warm GOCACHE
# from the previous mount, recompiling only the changed packages.
COPY pkg/ pkg/
COPY services/ingestion-api/ services/ingestion-api/

WORKDIR /workspace/services/ingestion-api

RUN --mount=type=cache,target=/go/pkg/mod,id=aer-go-modcache \
    --mount=type=cache,target=/root/.cache/go-build,id=aer-go-buildcache \
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
