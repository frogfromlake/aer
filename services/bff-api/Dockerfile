FROM golang:1.23-alpine AS builder

WORKDIR /workspace

# Copy go workspace files
COPY go.work go.work.sum ./
COPY pkg/ pkg/
COPY services/bff-api/ services/bff-api/

WORKDIR /workspace/services/bff-api

RUN go mod download && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/bff-api ./cmd/server/main.go

FROM scratch

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /bin/bff-api /bff-api

EXPOSE 8080

ENTRYPOINT ["/bff-api"]
