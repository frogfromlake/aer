# AĒR — Societal Discourse Macroscope

A modular system for the real-time analysis and long-term observation of societal discourses. AĒR aggregates global digital data streams and extracts meaningful patterns from the collective output of connected civilization — functioning as an atmospheric sensor for human discourse rather than a surveillance instrument for individuals.

The name derives from ancient Greek ἀήρ: the lower atmosphere, the surrounding climate.

⚠️ Project Status: Under Construction & Research Phase > AĒR is currently in active development. While the underlying microservice architecture is nearing its final state—designed to be secure, resilient, and horizontally scalable—the core analytical business logic is pending ongoing scientific research. At present, the system operates with a proof-of-concept crawler limited to Wikipedia articles, and the Python analysis workers are focused solely on harmonizing this specific dataset. Development of comprehensive sociological metrics and integration of additional data sources will commence once the foundational research is complete.

---

## Architecture Overview

AĒR implements a polyglot microservice pipeline based on the Medallion Architecture (Bronze / Silver / Gold). Data flows strictly from left to right through deterministic, independently testable stages. No microservice communicates with another via direct HTTP — all inter-service coordination is mediated through shared object storage (MinIO) and the NATS JetStream message broker.

```
Crawler  →  Ingestion API (Go)  →  MinIO Bronze  →  NATS JetStream
                                                          ↓
                                                  Analysis Worker (Python)
                                                    ↓           ↓
                                             MinIO Silver   ClickHouse Gold
                                                                  ↓
                                                           BFF API (Go)
                                                                  ↓
                                                      Dashboard / Analyst
```

**Ingestion API (Go):** Source-agnostic HTTP receiver. Stores raw documents verbatim in MinIO (write-once, immutable). Logs metadata and trace IDs to PostgreSQL.

**Analysis Worker (Python):** Validates documents against the Silver Contract (Pydantic), extracts deterministic metrics, inserts into ClickHouse. Malformed data is routed to a Dead Letter Queue without crashing the pipeline.

**BFF API (Go):** Contract-first REST API generated from OpenAPI 3.0. Queries ClickHouse with server-side downsampling and hard row limits. The only service exposed to the internet, authenticated via API key, TLS-terminated by Traefik.

**Crawlers:** Standalone external programs under `crawlers/`. Each crawler fetches from one upstream source and translates it into the generic AĒR ingestion contract. Crawlers are deliberately outside the system boundary — adding a new data source requires no changes to any AĒR service. Currently includes the Wikipedia scraper (PoC) and the RSS crawler (German institutional feeds for pipeline calibration).

Full architectural documentation (arc42) is available at `http://localhost:8000` when the stack is running.

---

## Technology Stack

| Layer | Technology | Rationale |
| :--- | :--- | :--- |
| Ingestion / BFF / Crawlers | Go 1.26.1+ | High-concurrency I/O, minimal memory footprint |
| Analysis / Processing | Python 3.12+ | Deterministic data science ecosystem (spaCy, Pydantic) |
| Object Storage / Event Publisher | MinIO | S3-compatible data lake with native JetStream notification |
| Event Broker | NATS JetStream | Durable, at-least-once delivery; replaces synchronous polling |
| Analytics Database | ClickHouse | Column-oriented OLAP; mandatory for sub-second time-series queries |
| Metadata Index | PostgreSQL | Relational tracking of ingestion jobs, document lifecycle, trace IDs |
| Reverse Proxy / TLS | Traefik | Self-signed TLS in dev; ACME/Let's Encrypt in production via `compose.prod.yaml` overlay |
| Observability | OpenTelemetry + Grafana LGTM | End-to-end distributed tracing across the NATS boundary |
| Containerization | Docker + Compose | `compose.yaml` is the Single Source of Truth for the entire stack |

---

## Prerequisites

The only required host installations are:

- Docker with Compose plugin
- Go 1.26.1 or higher
- Python 3.12 or higher
- GNU Make

No databases or runtimes are installed directly on the host. All services run in containers.

---

## Getting Started

**1. Clone the repository and configure environment variables:**

```bash
git clone <repository-url>
cd aer
cp .env.example .env
# Edit .env — all credentials and endpoints are configured here
```

**2. Install Git hooks:**

```bash
cp scripts/hooks/pre-commit .git/hooks/pre-commit
cp scripts/hooks/pre-push   .git/hooks/pre-push
chmod +x .git/hooks/pre-commit .git/hooks/pre-push
```

The pre-commit hook runs `make lint`. The pre-push hook runs `make lint` followed by `make test`. Both block on failure.

**3. Start the full stack:**

```bash
make up
```

This starts infrastructure (databases, NATS, observability, documentation server) and all three application services. The first run builds application images from source.

**4. Verify the pipeline:**

```bash
# Run the end-to-end smoke test
bash scripts/e2e_smoke_test.sh
```

The smoke test ingests a test document, waits for pipeline processing, and queries the BFF API to verify end-to-end data flow.

---

## Make Targets

### Global Stack

| Target | Description |
| :--- | :--- |
| `make up` | Start the entire stack (infrastructure + all application services) |
| `make down` | Stop everything |
| `make restart` | Stop and restart the entire stack |
| `make stop` | Alias for `make down` |

### Infrastructure

| Target | Description |
| :--- | :--- |
| `make infra-up` | Start infrastructure only (databases, NATS, observability, docs) |
| `make infra-down` | Stop infrastructure |
| `make infra-restart` | Restart infrastructure |
| `make infra-clean` | Wipe all persistent volumes (requires interactive confirmation) |
| `make infra-clean-postgres` | Wipe PostgreSQL volume only |
| `make infra-clean-minio` | Wipe MinIO volume only |
| `make infra-clean-clickhouse` | Wipe ClickHouse volume only |

### Debug Port Access

| Target | Description |
| :--- | :--- |
| `make debug-up` | Expose all backend ports to `localhost` for debugging (opt-in) |
| `make debug-down` | Close debug port forwarding (backend services keep running) |

By default, all backend ports (databases, NATS, OTel, Ingestion API) are accessible only within the Docker network. `make debug-up` forwards them to localhost via the `debug` Compose profile. See [Network Segmentation](#network-segmentation) and [Exposed Ports](#exposed-ports).

### Application Services

| Target | Description |
| :--- | :--- |
| `make services-up` | Start all three application services in the background |
| `make services-down` | Stop all application services |
| `make services-restart` | Restart all application services |
| `make services-clean` | Stop services and remove PID/log files |
| `make logs` | Tail combined logs of all background services (Ctrl+C safe) |

Individual services can be controlled independently:

```bash
make ingestion-up    make ingestion-down    make ingestion-restart
make worker-up       make worker-down       make worker-restart
make bff-up          make bff-down          make bff-restart
```

### Help

```bash
make        # or: make help — prints a formatted overview of all available targets
```

### Development & Utilities

| Target | Description |
| :--- | :--- |
| `make test` | Run full test suite (Go integration tests + Go crawler tests + Python unit tests) |
| `make test-go` | Run Go integration tests (Testcontainers — requires Docker) |
| `make test-go-pkg` | Run Go tests for the shared `pkg/` module |
| `make test-go-crawlers` | Run Go crawler tests (RSS parser, translator, dedup) |
| `make test-e2e` | Run Docker Compose end-to-end smoke test |
| `make lint` | Run `golangci-lint` (Go, all modules) and `ruff` (Python) |
| `make lint-go-pkg` | Run `golangci-lint` for `pkg/` only |
| `make codegen` | Regenerate Go types and server stubs from `openapi.yaml` |
| `make build-services` | Compile Go binaries into `./bin/` |
| `make tidy` | Run `go mod tidy` across all modules |

---

## Exposed Ports

AĒR uses a Zero-Trust network posture. Only Traefik, the BFF API, and the documentation server expose ports to the host by default. All backend services communicate exclusively over the internal Docker network.

### Default (always exposed)

| Port | Service | Purpose |
| :--- | :--- | :--- |
| `80` | Traefik | HTTP — redirects to HTTPS |
| `443` | Traefik | HTTPS — routes to BFF API, Grafana, MinIO Console |
| `8000` | MkDocs | Arc42 architecture documentation |
| `8080` | BFF API | `GET /api/v1/metrics`, `/api/v1/healthz`, `/api/v1/readyz` |

### Debug profile only (`make debug-up`)

These ports are not exposed in the default stack. Run `make debug-up` to forward them to localhost for local debugging. They are closed again with `make debug-down`.

| Port | Service | Purpose |
| :--- | :--- | :--- |
| `3000` | Grafana | Monitoring dashboards (also accessible via Traefik HTTPS) |
| `4222` | NATS | Client connections |
| `4317` | OTel Collector | OpenTelemetry gRPC receiver |
| `4318` | OTel Collector | OpenTelemetry HTTP receiver |
| `5432` | PostgreSQL | Direct database access |
| `8081` | Ingestion API | `POST /api/v1/ingest`, `/api/v1/healthz`, `/api/v1/readyz` |
| `8123` | ClickHouse | HTTP interface and query playground (`/play`) |
| `8222` | NATS | Monitoring dashboard |
| `9000` | MinIO | S3-compatible API |
| `9001` | MinIO | Web console (also accessible via Traefik HTTPS) |
| `9002` | ClickHouse | Native protocol |

All credentials are sourced from `.env`. See `.env.example` for defaults. Both APIs require an API key — set `BFF_API_KEY` and `INGESTION_API_KEY` to strong secrets in production.

---

## API Reference

The BFF API contract is defined in `services/bff-api/api/openapi.yaml`. The OpenAPI spec is the Single Source of Truth — Go server stubs are generated from it via `oapi-codegen` and must never be edited manually.

All endpoints except `/healthz` and `/readyz` require authentication:

```
X-API-Key: <your-key>
# or
Authorization: Bearer <your-key>
```

**Retrieve aggregated metrics:**

```
GET /api/v1/metrics?startDate=2026-01-01T00:00:00Z&endDate=2026-04-01T00:00:00Z
```

Optional filters: `source` (e.g., `wikipedia`) and `metricName` (e.g., `word_count`) narrow results by data source and metric dimension.

Results are downsampled to 5-minute intervals. A hard row limit is applied server-side to prevent OOM on large time ranges.

---

## Ingestion Contract

All endpoints except `/api/v1/healthz` and `/api/v1/readyz` require authentication:

```
X-API-Key: <your-ingestion-key>
# or
Authorization: Bearer <your-ingestion-key>
```

The key is configured via the `INGESTION_API_KEY` environment variable.

Every crawler — regardless of upstream source — must submit data in this format:

```json
{
  "source_id": 1,
  "documents": [
    {
      "key": "wikipedia/article-slug/2026-03-28.json",
      "data": {
        "source": "wikipedia",
        "title": "Example Article",
        "raw_text": "The full unstructured text content...",
        "url": "https://en.wikipedia.org/wiki/Example",
        "timestamp": "2026-03-28T12:00:00Z"
      }
    }
  ]
}
```

The `key` determines the MinIO object path in the Bronze bucket. The `data` field is stored verbatim and never modified. `source_id` references a registered entry in the PostgreSQL `sources` table.

---

## Developing a Crawler

Crawlers are standalone programs under `crawlers/`. Each crawler fetches data from one upstream source and submits it to the Ingestion API using the contract above. Adding a new data source requires no changes to any AĒR service.

**1. Dynamic Source Resolution**

Every crawler must resolve its `source_id` at startup by querying the Ingestion API:

```
GET /api/v1/sources?name=wikipedia
→ {"id": 1, "name": "wikipedia"}
```

The returned `id` is used as `source_id` in all subsequent ingest calls. Hard-coding source IDs is discouraged — the lookup ensures correctness across environments. Sources are registered via PostgreSQL seed migrations in `infra/postgres/migrations/`.

**2. Ingestion Contract Format**

Documents are submitted as JSON batches to `POST /api/v1/ingest`. See the [Ingestion Contract](#ingestion-contract) section above for the full schema. The `key` field determines the Bronze bucket object path and should follow the pattern `<source>/<identifier>/<date>.json`.

**3. Source Type (Phase 39+)**

Crawlers for new data sources should include a `source_type` field in the `data` payload (e.g., `"rss"`, `"forum"`, `"social"`). This field is used by the analysis worker to select the correct Source Adapter for harmonization. Documents without a `source_type` are handled by the legacy adapter. See ADR-015 in the architecture documentation.

**4. Authentication**

All Ingestion API endpoints (except `/api/v1/healthz` and `/api/v1/readyz`) require the `X-API-Key` header (or `Authorization: Bearer <key>`). The key is configured via the `INGESTION_API_KEY` environment variable in `.env`.

### RSS Crawler

The RSS crawler (`crawlers/rss-crawler/`) is AĒR's first real data source. It fetches German institutional RSS feeds for pipeline calibration (see architecture documentation, Chapter 13, §13.8).

**Usage:**

```bash
# Build
go build -o bin/rss-crawler ./crawlers/rss-crawler

# Run (requires the AĒR stack to be running)
./bin/rss-crawler \
  -config crawlers/rss-crawler/feeds.yaml \
  -api-url http://localhost:8081/api/v1/ingest \
  -sources-url http://localhost:8081/api/v1/sources \
  -api-key <your-ingestion-key>
```

**Feed configuration** is defined in `feeds.yaml`. Adding a new RSS feed requires:
1. A new entry in `feeds.yaml` (`name` + `url`).
2. A PostgreSQL seed migration in `infra/postgres/migrations/` registering the source name.

The crawler maintains a local state file (`.rss-crawler-state.json`) to prevent re-ingestion of previously submitted items across runs.

---

## Testing

```bash
# Full test suite
make test

# Go integration tests only (uses Testcontainers — requires Docker)
cd services/ingestion-api && go test ./...
cd services/bff-api       && go test ./...

# Python unit tests only
cd services/analysis-worker && python -m pytest
```

**Testcontainers SSoT enforcement:** Both Go (`pkg/testutils/compose.go`) and Python (`get_compose_image()`) parse image tags dynamically from `compose.yaml` at test time. No image tag is hardcoded in any test file. Tests run against the exact same database versions as development and production.

**OpenAPI contract check:**

```bash
make codegen
git diff --exit-code services/bff-api/internal/handler/generated.go
```

This is enforced automatically in CI on every push to `main`.

---

## CI/CD Pipeline

The GitHub Actions pipeline (`.github/workflows/ci.yml`) runs four parallel jobs on every push and pull request to `main`:

| Job | Steps |
| :--- | :--- |
| `python-pipeline` | Ruff lint → pytest (unit + integration) |
| `go-pipeline` | golangci-lint → OpenAPI contract check → Testcontainers integration tests |
| `dependency-audit` | govulncheck (Go) → pip-audit (Python) |
| `container-security-scan` | Docker build → Trivy scan (HIGH/CRITICAL CVEs, `exit-code: 1`) |

Testcontainers images are cached as tarballs via `actions/cache@v4` to avoid registry pulls on cache hits.

---

## Image Pinning Policy

All Docker images in `compose.yaml` use hard-pinned, immutable patch-level version tags. The use of `latest`, release-candidate, or major/minor-only tags is prohibited. Upgrades are performed manually after changelog review and full local stack validation.

`compose.yaml` is the Single Source of Truth for all image versions. Testcontainers in both Go and Python resolve their images from this file at test time.

---

## Data Lifecycle

| Layer | Storage | Retention | Mechanism |
| :--- | :--- | :--- | :--- |
| Bronze (raw) | MinIO `bronze` | 90 days | MinIO ILM |
| Quarantine (DLQ) | MinIO `bronze-quarantine` | 30 days | MinIO ILM |
| Silver (harmonized) | MinIO `silver` | 365 days | MinIO ILM |
| Gold (metrics) | ClickHouse `aer_gold.metrics` | 365 days | ClickHouse TTL |
| Metadata | PostgreSQL | Unlimited | — |

All retention policies are defined in infrastructure scripts (`infra/`). No application code manages data expiration.

---

## TLS Configuration

By default, `compose.yaml` configures Traefik with a **self-signed TLS certificate** — suitable for local development and CI where real domains are not available. No ACME provider is contacted, so there are no certificate errors for `*.example.com` hosts.

For **production** deployments with real domains and automatic Let's Encrypt certificates, use the production overlay:

```bash
docker compose -f compose.yaml -f compose.prod.yaml up -d
```

`compose.prod.yaml` adds the ACME certificate resolver to Traefik and switches all routers from self-signed TLS to Let's Encrypt. Make sure to set real domain names for `GRAFANA_HOST` and `MINIO_CONSOLE_HOST` in `.env`, and provide a valid `ACME_EMAIL`.

---

## Network Segmentation

The stack is split into two isolated Docker bridge networks:

- **`aer-frontend`:** Traefik, BFF API, Grafana — internet-facing services.
- **`aer-backend`:** All databases, NATS, the analysis worker, init containers, and the observability stack — unreachable from the internet.

Only the BFF API and Grafana bridge both networks. Databases and internal services are inaccessible from the `aer-frontend` network. A compromised Traefik instance cannot reach PostgreSQL, MinIO, or NATS.

---

## Observability

**Grafana** is available at `http://localhost:3000`. Pre-provisioned datasources (Tempo, Prometheus) and dashboards load automatically on startup — no manual configuration required.

**Distributed Tracing:** Every document is traceable end-to-end from the crawler's HTTP POST through ingestion, NATS delivery, worker processing, and ClickHouse insertion. Trace context propagates across the NATS boundary via message headers.

**Prometheus Alerting Rules:**

| Alert | Condition | Severity |
| :--- | :--- | :--- |
| `WorkerDown` | Worker scrape target unreachable for > 1 minute | Critical |
| `DLQOverflow` | DLQ size > 50 objects for > 5 minutes | Warning |
| `HighEventProcessingLatency` | p95 processing duration > 5 seconds for > 5 minutes | Warning |

---

## Philosophical Foundation

AĒR's analytical framework rests on three structural pillars reflected in its name:

**A — Aleph** (Borges): The single point containing all other points. AĒR aggregates fragmented global data streams into one coherent view of human interaction.

**E — Episteme** (Foucault): The underlying rule-set of an epoch defining what can be thought and expressed. AĒR tracks discourse shifts to measure how the boundaries of the expressible form and change across cultures.

**R — Rhizome** (Deleuze / Guattari): A decentralized, proliferating network. AĒR models how information and cultural patterns spread non-linearly through global networks.

The system operates as a phenomenological instrument — observation and understanding, not surveillance or manipulation. Raw data is never altered. Algorithms are deterministic, simple, and fully traceable (Ockham's Razor).

---

## License

See `LICENSE` for terms.
