FROM golang:1.26.1-alpine3.23 AS builder

WORKDIR /workspace

# Copy go workspace files
COPY go.work go.work.sum ./
COPY pkg/ pkg/
COPY services/bff-api/ services/bff-api/
# Workspace stubs: go.mod required for all modules listed in go.work
COPY services/ingestion-api/go.mod services/ingestion-api/go.mod
COPY crawlers/rss-crawler/go.mod crawlers/rss-crawler/go.mod

WORKDIR /workspace/services/bff-api

RUN go mod download && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/bff-api ./cmd/server/main.go

FROM alpine:3.23.3

RUN apk upgrade --no-cache zlib

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /bin/bff-api /bff-api

EXPOSE 8080

ENTRYPOINT ["/bff-api"]
