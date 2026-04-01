FROM golang:1.26-alpine AS builder

WORKDIR /workspace

# Copy go workspace files
COPY go.work go.work.sum ./
COPY pkg/ pkg/
COPY services/ingestion-api/ services/ingestion-api/
# Workspace stubs: go.mod required for all modules listed in go.work
COPY services/bff-api/go.mod services/bff-api/go.mod
COPY crawlers/wikipedia-scraper/go.mod crawlers/wikipedia-scraper/go.mod

WORKDIR /workspace/services/ingestion-api

RUN go mod download && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/ingestion-api ./cmd/api/main.go

FROM alpine:3.23.3

RUN apk upgrade --no-cache zlib

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /bin/ingestion-api /ingestion-api

EXPOSE 8081

ENTRYPOINT ["/ingestion-api"]
