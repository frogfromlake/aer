# CLAUDE.md

This file contains instructions for Claude Code (claude.ai/code) when working with this repository.

## Your Role

You are an experienced Senior Software Architect and Pair Programmer. Your task is to co-develop this project with me. Think critically, flag potential architecture mistakes or edge cases, and write clean, idiomatic code.

## Language Rules

* **Communication (Chat):** Always respond in German.
* **Code & Documentation:** All variables, functions, code comments, commit messages, and documentation files must always be in English.

## Project Overview

**AĒR** (ancient Greek ἀήρ: the lower atmosphere, the surrounding climate) is a polyglot, event-driven data ingestion and analysis pipeline implementing the **Medallion Architecture** (Bronze → Silver → Gold). Its purpose is to observe large-scale patterns in global digital discourse — a "societal macroscope." Scientific integrity is the highest priority: raw data is never mutated, all transformations are deterministic and auditable.

## Commands

All orchestration is managed via `make`. Run `make` without arguments to see available targets.

### Full Stack
```bash
make up              # Starts the entire stack (infrastructure + all three services)
make down            # Stops everything
make restart         # Stops and restarts the entire stack
make stop            # Alias for make down
make logs            # Shows combined logs of all services (Ctrl+C is safe — services keep running)
```

### Infrastructure Only
```bash
make infra-up             # Starts MinIO, PostgreSQL, ClickHouse, NATS, Grafana, Prometheus, Tempo, Docs
make infra-down           # Stops infrastructure
make infra-restart        # Restarts infrastructure
make infra-clean          # Deletes all volumes (requires confirmation)
make infra-clean-postgres # Wipes PostgreSQL volume only
make infra-clean-minio    # Wipes MinIO volume only
make infra-clean-clickhouse # Wipes ClickHouse volume only
```

### Debug Port Access
```bash
make debug-up        # Forwards all backend ports to localhost (opt-in, Zero-Trust default hides them)
make debug-down      # Closes debug port forwarding (backend services keep running internally)
```

### Application Services
```bash
make services-up     / make services-down    / make services-restart
make ingestion-up    / make ingestion-down   / make ingestion-restart
make worker-up       / make worker-down      / make worker-restart
make bff-up          / make bff-down         / make bff-restart
make services-clean  # Stops services and removes PID/log files
```

### Development
```bash
make test            # All tests (Go integration + Python unit)
make test-go         # Go integration tests via Testcontainers (requires Docker)
make test-go-pkg     # Go tests for the shared pkg/ module
make test-python     # Python unit tests via pytest
make test-e2e        # End-to-end smoke test (full Docker Compose stack, with teardown)
make lint            # golangci-lint (all Go modules) + ruff (Python)
make lint-go-pkg     # golangci-lint for pkg/ only
make codegen         # Regenerates Go types from services/bff-api/api/openapi.yaml
make build-services  # Compiles Go binaries into ./bin/
make tidy            # Runs go mod tidy across all modules
make help            # Prints a formatted overview of all available targets
```

Run a single Python test: `cd services/analysis-worker && python -m pytest tests/test_processor.py::TestName -v`

Run a single Go test: `cd services/ingestion-api && go test ./... -run TestName`

## Architecture

Three microservices communicate **exclusively** via shared storage and NATS — there are no direct HTTP calls between services.

```
[ingestion-api (Go, :8081)]
    → uploads raw JSON → MinIO bronze bucket
    → logs metadata → PostgreSQL (trace_id, object_key, job status)
    → MinIO emits NATS event on bucket PUT → topic: aer.lake.bronze

[analysis-worker (Python, NATS Consumer)]
    ← subscribes to aer.lake.bronze (JetStream, durable, at-least-once)
    → validates with Pydantic, harmonizes Bronze → Silver in MinIO
    → extracts metrics → ClickHouse aer_gold.metrics
    → malformed data → MinIO bronze-quarantine (DLQ, 30-day TTL)
    → manual NATS Ack after processing

[bff-api (Go, :8080)]
    ← REST GET /api/v1/metrics?startDate=...&endDate=...
    ← Protected by API key (X-API-Key header or Authorization: Bearer <key>)
    → queries ClickHouse for time-series aggregations (5-min downsampling, LIMIT 10000)

[traefik (Reverse Proxy, :80/:443)]
    → TLS termination via Let's Encrypt for BFF-API and Grafana
    → Only public-facing ingress point in production
```

**All three services** emit OpenTelemetry traces with context propagation via NATS message headers. Visible in Grafana Tempo.

### Crawlers

Crawlers are **not** integrated into the ingestion-api. They run as standalone external programs that POST data to `http://<ingestion-api>/api/v1/ingest`. This follows the "Dumb Pipes, Smart Endpoints" pattern. Long-term vision: hundreds of specialized crawlers deliver data via the HTTP interface.

Current crawlers:
- `crawlers/wikipedia-scraper/` — Go program fetching random Wikipedia article summaries.

### Network Segmentation

Docker Compose defines two networks:
- `aer-frontend` — BFF-API, Grafana, Traefik (public-facing)
- `aer-backend` — all databases, NATS, workers, OTel (internal only)

Only the BFF-API and Grafana bridge both networks.

## Storage Layer

| Storage | Role | TTL |
|---------|------|-----|
| MinIO `bronze` | Immutable raw data | 90 days |
| MinIO `silver` | Harmonized data | — |
| MinIO `bronze-quarantine` | Dead Letter Queue | 30 days |
| PostgreSQL | Document metadata + lineage (trace_id ↔ object_key) | — |
| ClickHouse `aer_gold.metrics` | Aggregated time-series | 365 days |

The PostgreSQL schema is in `infra/postgres/init.sql`. The ClickHouse schema is in `infra/clickhouse/init.sql`. The MinIO bucket setup (including ILM policies and NATS event routing) is in `infra/minio/setup.sh`.

## Code Structure

- `pkg/` — Shared Go libraries (Logger, Telemetry, Testutils) used by both Go services via `go.work`.
- `services/ingestion-api/` — Go; entry point: `cmd/api/main.go`; business logic: `internal/core/service.go`; adapters: `internal/storage/`
- `services/analysis-worker/` — Python; entry point: `main.py`; processing: `internal/processor.py`; contracts: `internal/models.py`
- `services/bff-api/` — Go; entry point: `cmd/server/main.go`; OpenAPI spec: `api/openapi.yaml` (contract-first, types auto-generated via `make codegen`)
- `crawlers/` — Standalone crawler programs (currently: `wikipedia-scraper/`). Each crawler is an independent Go module.
- `infra/` — IaC scripts for all infrastructure (executed by init containers in `compose.yaml`)
- `docs/arc42/` — Architecture documentation in Arc42 format; accessible at `http://localhost:8000` via MkDocs

## Non-Negotiable Rules

### Docker Compose is the Single Source of Truth (SSoT)

- `compose.yaml` is the SSoT for **all** container image tags.
- **Hard-pinning only** — no `latest`, no floating major/minor tags (e.g. `v3`), no `rc`/`alpha`/`beta` versions.
- When upgrading an image version: check the changelog, test locally with `make up`, and commit the pinned tag.

### Testcontainers Must Parse Tags from Compose

- Go tests use `pkg/testutils.GetImageFromCompose()` to read the image tag dynamically from `compose.yaml`.
- Python tests use `tests/test_storage.py::get_compose_image()` for the same purpose.
- **Never hardcode image tags** in test files — always read from the SSoT.

### Healthchecks: HTTP Probes Only

- Healthchecks in `compose.yaml` and Testcontainers must always use **HTTP probes** (or native commands like `pg_isready`).
- **Never use log-parsing** (`wait.ForLog(...)`) as a readiness strategy.

### Git Hooks Are Mandatory

Pre-commit and pre-push hooks live in `scripts/hooks/`:
- **Pre-commit:** Runs `make lint` — blocks commit on lint failure.
- **Pre-push:** Runs `make lint` + `make test` — blocks push on lint or test failure.

Install them via: `git config core.hooksPath scripts/hooks`

### Go Workspace (`go.work`)

`go.work` and `go.work.sum` are **intentionally versioned** in the repository. They serve as the SSoT for Docker multi-stage builds and CI to deterministically resolve the shared `pkg/` module within the monorepo. Do not add them to `.gitignore`.

## Design Rules

1. **Timestamps are deterministic:** Use metadata from MinIO events, never `datetime.now()` or `time.Now()` in data processing paths.
2. **Idempotency:** Ingestion uses the `bronze_object_key` as unique key; reprocessing the same event must never create duplicates. The worker checks PostgreSQL document status before processing.
3. **No mutation of Bronze:** Raw data in the MinIO `bronze` bucket is write-once. Transformations create new objects in the `silver` bucket.
4. **BFF API is contract-first:** Edit `services/bff-api/api/openapi.yaml`, then run `make codegen` — never manually edit generated files.
5. **Shared Go code belongs in `pkg/`:** Both Go services depend on it via the Go workspace.
6. **Crawlers are external:** They POST to the ingestion-api HTTP endpoint. They are never embedded in or coupled to the ingestion service.
7. **Infrastructure is provisioned by IaC:** Microservices must never create databases, tables, or buckets on startup. They assume infrastructure is already present (provisioned by init containers or IaC scripts).

## Local Service URLs

| Service | URL |
|---------|-----|
| Ingestion API | `http://localhost:8081/api/v1/ingest` |
| BFF API | `http://localhost:8080/api/v1/metrics` |
| Grafana | `http://localhost:3000` |
| MinIO Console | `http://localhost:9001` |
| ClickHouse UI | `http://localhost:8123/play` |
| NATS Monitor | `http://localhost:8222` |
| Docs (MkDocs) | `http://localhost:8000` |

Credentials are in `.env` (copy from `.env.example`). The BFF API requires an `X-API-Key` header for all routes except `/api/v1/healthz` and `/api/v1/readyz`.