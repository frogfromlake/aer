# AĒR Implementation, Refactoring & Scaling Roadmap

This roadmap defines the steps to transition the AĒR base architecture into a scalable, maintainable system developed according to modern standards (CLEAN Code, DRY, Event-Driven).

---

# Completed Phases (1–13)

## Phase 1: Base Foundation & DRY Principle (Go Workspace & Tooling) - [x] DONE
* [x] **Set up Go Workspace:** `pkg/` folder for shared logic.
* [x] **Initialize `go.work`:** Linking of services.
* [x] **Centralized Configuration Management:** `viper` config loader for `.env`.
* [x] **Standardized Logging:** Custom JSON/Text logger (Go `slog`).

## Phase 2: Clean Architecture (`ingestion-api`) - [x] DONE
* [x] **Adapt folder structure:** `cmd/api`, `internal/config`, `internal/storage`, `internal/core`.
* [x] **Implement Dependency Injection:** Clean wiring in `main.go`.
* [x] **Remove hardcoded credentials:** Use of `.env`.

## Phase 3: Infrastructure as Code (IaC) - [x] DONE
* [x] **Cleanup app logic:** Bucket creation removed from Go code.
* [x] **Init Scripts / IaC:** Docker init container (`minio-init`) creates buckets.

## Phase 4: Event-Driven Communication - [x] DONE
* [x] **Technology decision:** NATS JetStream chosen.
* [x] **Infrastructure:** NATS added to `compose.yaml`.
* [x] **Producer (MinIO):** Automatic event trigger for new files in the "bronze" layer.
* [x] **Consumer (Python):** Python worker listens asynchronously and logs incoming events.

## Phase 5: Observability & Production Readiness - [x] DONE
* [x] **Observability Infrastructure:** Set up OTel-Collector, Grafana Tempo (Traces), Prometheus (Metrics), and Grafana (Dashboards) in Docker.
* [x] **Configuration:** Create YAML configs for the collector and routing.
* [x] **Documentation:** Document Architecture Decision Records (ADR) for OTel in arc42.

## Phase 6: Proof of Concept (End-to-End "Closing the Loop") - [x] DONE
* [x] **Bronze Layer (Go):** The `ingestion-api` uploads a real JSON document to the `bronze` bucket.
* [x] **NATS Trigger & Silver Layer (Python):** The Python worker receives the event, downloads the JSON, applies a simple transformation (e.g., lowercase), and saves it in the `silver` bucket.
* [x] **Gold Layer (ClickHouse):** Introduction of ClickHouse into the infrastructure. The Python worker extracts a dummy metric and saves it as a time series in the gold database.
* [x] **Tracing Instrumentation:** Integration of OTel libraries in Go and Python so that this exact flow becomes visible as a continuous trace in Grafana.

## Phase 7: Data Governance & Resilience (The Silver Contract) - [x] DONE
* [x] **Silver Schema Contract:** Introduction of `Pydantic` in the Python worker for strict validation and normalization of heterogeneous bronze data into a unified AĒR format.
* [x] **Dead Letter Queue (DLQ):** Faulty JSON (parsing errors) is intercepted and moved to a quarantine bucket (`bronze-quarantine`) instead of crashing the worker.
* [x] **Idempotency:** Adjust ClickHouse and Python worker so that duplicate NATS events (redeliveries) are ignored and metrics are not counted twice.

## Phase 8: The Metadata Index (PostgreSQL) - [x] DONE
* [x] **Database Schema:** Creation of the tables for `sources`, `ingestion_jobs`, and `documents` in PostgreSQL.
* [x] **Go Tracking:** The Ingestion API saves metadata (timestamp, source, MinIO path) in Postgres before loading the document into the data lake.
* [x] **Trace Linking:** The OTel Trace ID is stored as a foreign key in the database to enable later audit trails.

## Phase 9: The Serving Layer (Backend-for-Frontend) - [x] DONE
* [x] **Contract-First API:** Definition of the REST interfaces (e.g., for time series queries) in an `openapi.yaml`.
* [x] **BFF Code Generation:** Use of `oapi-codegen` to generate the Go boilerplate (Router, Structs) for the `bff-api` from the OpenAPI specification.
* [x] **ClickHouse Integration:** Implementation of the official `clickhouse-go` driver in the BFF API to read aggregated data performantly.

## Phase 10: Testing & Continuous Integration (CI) - [x] DONE
* [x] **Python Unit Testing:** Introduction of `pytest` for strict verification of data harmonization (Bronze -> Silver) and deterministic metric extraction.
* [x] **Go Integration Testing:** Use of `testcontainers-go` to validate the interactions of the Ingestion API with MinIO and PostgreSQL in isolated test containers.
* [x] **CI Pipeline (GitHub Actions):** Setup of automated workflows for linting (`golangci-lint`, `ruff`) and executing test suites on every push/pull request.

## Phase 11: Data Lifecycle Management & Graceful Degradation - [x] DONE
* [x] **Resilience (Go):** Implementation of exponential backoff (via `cenkalti/backoff/v5`) when establishing connections to PostgreSQL and MinIO.
* [x] **Data Lake Lifecycle:** Extension of the `minio-init` container with `mc ilm` policies to automatically clean up/archive raw bronze data after a defined period (e.g., 90 days).
* [x] **Analytics TTL & Migrations:** Extraction of ClickHouse table creation from the Python code into dedicated IaC/init scripts and introduction of Time-To-Live (TTL) rules for data aggregation.

## Phase 12: System Resilience, Consistency & Performance Optimization (Technical Debt) - [x] DONE
* [x] **Infrastructure & Networks:** Introduction of an explicit Docker network (`aer-network`) in the `compose.yaml` for better isolation and DNS resolution.
* [x] **Robust Init Scripts:** Adding Docker `healthcheck`s (e.g., for MinIO) and shifting the `depends_on` logic to `condition: service_healthy` to avoid boot race conditions.
* [x] **Lossless Event Processing (Python):** Migration of the `analysis-worker` from Core NATS to true NATS JetStream (`js.subscribe`, durable consumers) including manual `msg.ack()` to prevent data loss during restarts.
* [x] **Concurrency Control (Python):** Decoupling of NATS callbacks from CPU-intensive tasks using asynchronous queues (`asyncio.Queue`) or thread/process pools to prevent blocking the event loop.
* [x] **Idempotency Optimization (Python):** Replacement of network-intensive MinIO queries (`stat_object` for Silver/Quarantine) with performant PostgreSQL lookups to avoid bottlenecks under high throughput.
* [x] **Resolve Partial Failures (Go - Ingestion API):** Introduction of a "Pending" status in PostgreSQL prior to MinIO upload. Update to "Uploaded" only after success to prevent "Dark Data" (files without a metadata entry).
* [x] **Resolve Partial Failures (Python - Worker):** Transaction-safe resolution of the sequence "MinIO Upload (Silver) -> ClickHouse Insert (Gold)". Adjustment of the retry logic and status tracking so that metrics are not lost forever in case of a ClickHouse timeout.

## Phase 13: Distributed Systems Hardening & Idempotency - [x] DONE
* [x] **Idempotent Metrics (Worker):** Replacement of `datetime.now()` with deterministic timestamps (from MinIO event metadata) during the ClickHouse insert to prevent duplicates on NATS redeliveries.
* [x] **OOM Prevention (BFF-API):** Implementation of downsampling (e.g., aggregation on a minute/hour basis) and limits in the ClickHouse queries of the Go BFF API to prevent memory overflows during large time ranges.
* [x] **Clean Graceful Shutdown (Worker):** Refactoring the Python worker from hard `task.cancel()` to sentinel values (`None`) in the task queue to avoid torn database connections during restarts.
* [x] **Macro-Level Error Tracking (Ingestion):** Adaptation of the Go `IngestionService` logic to track faulty individual documents and correctly set the overarching job status to `failed` or `completed_with_errors` at the end.
* [x] **Boot Race Conditions (Infra):** Adding native Docker `healthcheck`s for PostgreSQL and ClickHouse in the `compose.yaml` including `depends_on: condition: service_healthy` for dependent services.

## Phase 17: Ingestion API Redesign (From Batch Job to Real Service) - [x] DONE
*Transformation of the `ingestion-api` from a one-off PoC script into a long-running, HTTP-capable microservice.*

* [x] **Introduce HTTP Server:** `chi` router with `POST /api/v1/ingest`, configurable via `INGESTION_PORT` (default: 8081).
* [x] **Remove PoC Test Data:** Hardcoded `testCases` replaced by `IngestDocuments(ctx, sourceID, []Document)`.
* [x] **Health Check Endpoints:** `/healthz` (Liveness) and `/readyz` (checks Postgres + MinIO).
* [x] **Graceful Shutdown with HTTP:** 5s timeout analogous to the BFF API.
* [x] **OTel Instrumentation:** `otelhttp` middleware for automatic span tracking.

## Phase 15: Configuration Hardening & Environment Consistency - [x] DONE
*Elimination of all hardcoded values and establishment of a consistent, environment-independent configuration across all services. Prerequisite for all further phases — without a clean config, no service can be meaningfully configured or scaled.*

* [x] **Externalize OTel Endpoint (Go):** `pkg/telemetry/otel.go` accepts the collector endpoint as a parameter instead of hardwiring `localhost:4317`. Configuration via `OTEL_EXPORTER_OTLP_ENDPOINT` from the `.env`.
* [x] **Externalize ClickHouse Address (BFF):** `bff-api/cmd/server/main.go` reads the ClickHouse address (`CLICKHOUSE_HOST`, `CLICKHOUSE_PORT`) from the config instead of hardcoding `localhost:9002`.
* [x] **Python Worker Config Refactoring:** NATS URL (`NATS_URL`), OTel Endpoint (`OTEL_EXPORTER_OTLP_ENDPOINT`), and `WORKER_COUNT` are made configurable via `python-dotenv` / environment variables. `storage.py` functions keep their `os.getenv()` calls with sensible defaults — DI refactoring follows in Phase 21.
* [x] **Externalize BFF Server Port:** Read port `:8080` from config instead of hardcoding it.
* [x] **Complete `.env.example`:** Missing variables added: `CLICKHOUSE_USER`, `CLICKHOUSE_PASSWORD`, `CLICKHOUSE_DB`, `CLICKHOUSE_HOST`, `CLICKHOUSE_PORT`, `POSTGRES_HOST`, `POSTGRES_PORT`, `NATS_URL`, `WORKER_COUNT`, `BFF_PORT`, `GF_SECURITY_ADMIN_USER`, `GF_SECURITY_ADMIN_PASSWORD`.
* [x] **Decouple Grafana Credentials:** Separate `GF_SECURITY_ADMIN_USER` / `GF_SECURITY_ADMIN_PASSWORD` variables instead of reusing MinIO credentials.
* [x] **Make replace directive consistent:** `bff-api/go.mod` receives the same `replace` directive as `ingestion-api/go.mod` for local `pkg` reference.

## Phase 21: Code Quality & Logger Refactoring - [x] DONE
*Can be worked on in parallel with Phase 17. Resolution of code quality issues and unification of the logging strategy before the codebase grows with crawlers — afterwards refactoring becomes more expensive.*

* [x] **Logger Refactoring (`pkg/logger`):** The `TintHandler` currently calls `fmt.Printf` directly, thereby bypassing the slog system. Refactoring: Correctly delegate the underlying `slog.Handler` or use a proven library like `lmittmann/tint` that implements the slog interface correctly.
* [x] **Isolate Python OTel Setup:** Move the Tracer/Provider setup from the global module scope into an explicit `init_telemetry()` function called in `main()`. This enables clean testing without global side effects.
* [x] **Python Dependency Injection:** `DataProcessor.__init__` already accepts infrastructure clients — apply the same principle to `main.py` so that the NATS subscription and worker configuration are testable and configurable.
* [x] **Document `psycopg2-binary`:** Explicit comment in `requirements.txt` that `psycopg2-binary` is only suitable for Development/CI. For production: `psycopg2` with libpq dependency in the Dockerfile.
* [x] **Unify Makefile Language:** The shell scripts (`clean_infra.sh`, etc.) contain German comments and UI texts. Switch to English according to the project language constraint (ADR in `02_architecture_constraints.md`).

## Phase 14: Real Data Ingestion (The First Real Crawler) - [x] DONE
*Replacement of dummy JSON with real data. Key architectural decision ("Dumb Pipes, Smart Endpoints"): Crawlers are NOT integrated into the `ingestion-api` but run as external scripts that deliver data via HTTP POST. Long-term vision: Hundreds of specialized crawlers deliver data into the bronze layer via the HTTP interface of the Ingestion API.*

* [x] **Standalone Go Crawler:** Creation of an independent Go program under `crawlers/wikipedia-scraper/` that fetches the public Wikipedia JSON API (e.g., Article of the Day) and sends the JSON via POST to `http://localhost:8081/api/v1/ingest`.
* [x] **Worker Adaptation (Python):** Adaptation of `models.py`, `processor.py`, and `test_processor.py` in the `analysis-worker` to the new Wikipedia format. Logic: Clean text, extract rudimentary n-grams/word counters, and send them as a metric to ClickHouse.

## Phase 16: API Hardening & HTTP Middleware Stack - [x] DONE
*Securing and professionalizing the HTTP layer of the BFF API for production use. With real data in the system, the BFF API will be externally accessible — it must be secured.*

* [x] **Recovery Middleware:** Integrate `chi` recovery middleware to catch panics in handlers and return them as `500 Internal Server Error` instead of crashing the process.
* [x] **Request Logging Middleware:** Structured access logging (`slog`) for every incoming HTTP request (Method, Path, Status, Duration, Trace-ID).
* [x] **CORS Middleware:** Configurable Cross-Origin Resource Sharing for the future frontend (allowed origins via `.env`, `CORS_ALLOWED_ORIGINS`).
* [ ] **Rate Limiting:** Token-bucket or sliding-window rate limiter as middleware, configurable via environment variables.
* [x] **Health Check Endpoints:** `GET /api/v1/healthz` (Liveness) and `GET /api/v1/readyz` (Readiness, checks ClickHouse connection) as standardized Kubernetes-compatible endpoints.
* [x] **Request Timeout Middleware:** Global context timeout per request (30s) to limit hanging ClickHouse queries.

## Phase 18: Observability Completion - [x] DONE
*Closing all gaps in the monitoring and tracing stack. Now there is real data to observe — without observability, problems with real crawlers are invisible.*

* [x] **BFF API OTel Instrumentation:** Integration of `otelhttp` middleware and tracer into the BFF API so that traces do not end at the Python worker but are visible all the way to the API response.
* [x] **Python Prometheus Metrics:** Export of business metrics from the worker: `events_processed_total`, `events_quarantined_total`, `event_processing_duration_seconds`, `dlq_size` (Counter/Histogram via `opentelemetry-sdk` Metrics API or `prometheus_client`).
* [x] **DLQ Monitoring:** Periodic check of the object count in the `bronze-quarantine` bucket. Alert upon exceeding a threshold.
* [x] **Grafana Dashboard Provisioning:** Creation of a pre-built JSON dashboard (`infra/observability/grafana-dashboards/`) with panels for: pipeline throughput, DLQ rate, ClickHouse query latency, NATS consumer lag. Automatic provisioning via `grafana.ini` / provisioning volume.
* [x] **Alerting Rules:** Definition of Prometheus alerting rules (`alert.rules.yml`): worker-down, DLQ overflow, ClickHouse latency > threshold, NATS consumer lag > threshold.

## Phase 19: Testing Expansion & Contract Safety - [x] DONE
*Increasing test coverage across all critical paths. Makes sense now because the real data flow exists and can be tested.*

* [x] **BFF Handler Tests:** Unit tests for the handler logic in `handler.go` (time range fallback, error handling) with a mocked storage interface.
* [x] **OpenAPI Contract Tests:** Automated comparison to ensure the generated `generated.go` is in sync with `openapi.yaml`. Integration into CI (e.g., re-running `oapi-codegen` and checking `git diff`).
* [x] **Python Edge Case Tests:** Extension of `test_processor.py` with: empty strings after `.lower()`, nested/unexpected JSON structures, simulated network errors (MinIO `ConnectionError`, ClickHouse Timeout), `_move_to_quarantine` in isolation.
* [x] **Python Storage Tests:** Integration tests for `storage.py` with Testcontainers (Postgres, MinIO, ClickHouse) analogous to the Go strategy to validate the `@retry` logic and connection setup.
* [x] **End-to-End Smoke Test:** A single automated test that tests the entire flow: JSON → Ingestion → MinIO → NATS → Worker → ClickHouse → BFF API Response. Can run as a separate CI job with `docker compose up`.


## Phase 20: Infrastructure Hardening & Container Security - [x] DONE
*Securing the Docker infrastructure for long-term operation and preparation for scaling with hundreds of crawlers.*

* [x] **Pin Image Versions:** All `latest` tags in `compose.yaml` replaced with specific versions. **Upgrade Policy:** Image versions are raised manually and deliberately — no automatic upgrades. Before each upgrade, the changelog/release notes of the respective image are checked and the stack is tested locally with `make up`. The pinned versions are versioned in the Git log so that a rollback via `git revert` is possible at any time.
* [x] **Resource Limits:** `deploy.resources.limits` (Memory, CPU) set for each container in `compose.yaml`. Highly critical: ClickHouse (OLAP can consume unlimited memory).
* [x] **Restart Policies:** `restart: unless-stopped` for all persistent services (databases, NATS, Grafana).
* [x] **Network Segmentation:** Splitting the flat `aer-network` into at least two subnets: `aer-frontend` (BFF, Grafana, Docs) and `aer-backend` (databases, NATS, Worker). Only the BFF API connects both networks.
* [x] **Service Dockerfiles:** Multi-stage Dockerfiles created for `ingestion-api`, `bff-api`, and `analysis-worker`.
* [x] **CI Docker Layer Caching:** Manual layer caching introduced via `actions/cache@v4` in the GitHub Actions pipeline. Testcontainers images (`minio/minio:latest`, `postgres:16-alpine`, `clickhouse/clickhouse-server:23.8`) are cached as tarballs and loaded directly upon a cache hit without re-pulling. Additionally: `~/go/bin` cached to avoid reinstallation of `golangci-lint` and `oapi-codegen` given an unchanged CI config.

## Phase 23: Security Foundations - [x] DONE
*Introduction of foundational security mechanisms prior to deployment with real data.*

* [x] **API Authentication:** Introduction of an API key or JWT-based auth mechanism on the BFF API. At least one static API key as a middleware gate for the first iteration.
* [x] **TLS for external Endpoints:** HTTPS termination for BFF API and Grafana (e.g., via Traefik or Caddy as a reverse proxy in the Compose stack).
* [x] **Container Security Scanning:** Integrated `aquasecurity/trivy-action` into the CI pipeline. Scans all three Dockerfiles for HIGH/CRITICAL CVEs, fails the build on unfixed findings.
* [x] **Dependency Auditing:** `govulncheck ./...` for both Go services and `pip-audit -r requirements.txt` for the Python worker as a dedicated CI job (`dependency-audit`).

## Phase 24: SSoT Enforcement & Deterministic Builds
*The compose.yaml is defined as SSoT for all container tags. Two images violate this rule, CI hardcodes tags instead of reading from compose, and a silent OTel port mismatch creates latent failures. These are foundational trust issues — every other phase builds on deterministic infrastructure.*

# Phases — Derived from Architecture Review (2026-04-01)

*The following phases were derived from a comprehensive code review against the project's own constraints (CLAUDE.md, Arc42 ADRs, Phase 20 Pinning Policy). The central axis: SSoT enforcement → DRY & hygiene → architecture alignment → network hardening → documentation. Existing open items (Phase 16, 22, 23) are integrated into their correct tiers.*

### Not documented in arc42, README.md and CLAUDE.md

* [x] **Hard-Pin Floating Tags:** Replace `prom/prometheus:v3` and `grafana/grafana:12.4` in `compose.yaml` with fully qualified, immutable versions (e.g. `prom/prometheus:v3.4.2`, `grafana/grafana:12.4.0`). Floating major/minor tags violate Phase 20 policy and produce non-reproducible builds.
* [x] **CI Tag Extraction from Compose (SSoT):** The `ci.yml` workflow hardcodes image tags for Testcontainers cache (`minio/minio:RELEASE.2025-09-07T16-13-09Z`, `postgres:18.3-alpine3.23`, `clickhouse/clickhouse-server:26.3.3-alpine`). Replace with a `yq`-based extraction step that reads tags from `compose.yaml` at pipeline runtime. This eliminates silent drift when compose tags are updated but CI is not.
* [x] **Pin `golangci-lint` Version in CI:** `go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest` uses an unpinned floating version. Pin to a specific release (e.g. `@v1.64.8`) to prevent CI breakage from upstream lint rule changes.
* [x] **Fix OTel Default Port Mismatch:** `ingestion-api/internal/config/config.go` defaults `OTEL_EXPORTER_OTLP_ENDPOINT` to `localhost:4317` (gRPC port), but `pkg/telemetry/otel.go` uses the `otlptracehttp` exporter (HTTP, port 4318). Align the default to `http://localhost:4318`. This bug is masked by `.env` in Docker but causes silent failures in local-without-env development.
* [x] **Remove Compiled Binary from Repo:** `crawlers/wikipedia-scraper/wikipedia-scraper` is a compiled Go binary checked into the repository. Delete it and add `crawlers/wikipedia-scraper/wikipedia-scraper` to `.gitignore`.

## Phase 25: DRY Consolidation & Dependency Hygiene - [x] DONE
*Duplicate code, dead modules, and mixed prod/dev dependencies increase maintenance cost and attack surface. These fixes are low-risk, high-value — they simplify the codebase before it grows with more crawlers.*

* [x] **Replace Fragile Compose Parsers with Proper YAML Parsing:** Both `pkg/testutils/compose.go` (Go) and `tests/test_storage.py::get_compose_image()` (Python) implement identical, fragile logic: manual line-by-line YAML parsing via indent counting. Replace the Go parser with `gopkg.in/yaml.v3``pinned version and the Python parser with `PyYAML` (`yaml.safe_load`). Both are already transitive dependencies in each ecosystem.
* [x] **Remove Dead `pkg/config/config.go`:** The shared `AppConfig` struct has only two fields (`Environment`, `LogLevel`) and is not imported by any service — each service defines its own, richer config. Delete `pkg/config/config.go` and its Viper dependency from `pkg/go.mod` to reduce the shared module's surface.
* [x] **Split `requirements.txt` into Prod and Dev:** `requirements.txt` bundles production dependencies (`minio`, `nats-py`, `pydantic`) with dev/test tools (`pytest`, `ruff`, `testcontainers`, `docker`). The production Dockerfile installs everything, inflating the image and expanding the Trivy scan surface. Split into `requirements.txt` (prod) and `requirements-dev.txt` (dev/test). Update Dockerfile to use only prod, CI to install both.
* [x] **Align Makefile `test-python` with CI:** `make test-python` invokes `./venv/bin/python -m pytest`, which requires a local venv. CI installs pip globally without a venv. Add a conditional: if `venv/bin/python` exists use it, otherwise fall back to `python -m pytest`. This makes the Makefile target portable across local and CI environments.

## Phase 26: Clean Architecture Completion (Ingestion API)
*The BFF-API correctly uses interface-based DI (`MetricsStore`), but the Ingestion API bypasses this pattern by depending on concrete storage types. This makes the core business logic untestable without real databases — a structural gap that must be closed before adding more crawlers.*

* [x] **Extract Storage Interfaces in `core/`:** Define `MetadataStore` and `ObjectStore` interfaces in `internal/core/` that abstract the PostgreSQL and MinIO operations used by `IngestionService`. Refactor `IngestionService` to accept these interfaces instead of `*storage.PostgresDB` and `*storage.MinioClient`.
* [x] **Unit-Test Core Logic with Mocks:** With interfaces in place, write unit tests for `core/service.go` covering: batch processing with partial failures, job status transitions (`running` → `completed` / `completed_with_errors` / `failed`), and the "dark data prevention" pattern (DB-first, then MinIO).
* [x] **Harmonize Health Endpoint Paths:** Ingestion-API serves `/healthz` and `/readyz` (no prefix). BFF-API serves `/api/v1/healthz` and `/api/v1/readyz`. Align Ingestion-API to `/api/v1/healthz` and `/api/v1/readyz` for consistent monitoring configuration and Traefik routing. Update `compose.yaml` healthchecks accordingly.


## Phase 22: Arc42 Documentation & Language Compliance (Corrected) - [x] DONE 
*Documentation must reflect the final architecture state. This phase also enforces the English-only language constraint and prepares the ClickHouse schema for multi-source metric ingestion.*

* [x] **Kapitel 3 — System Scope and Context:** Create a context diagram (System Boundary, external actors: data sources, analysts, dashboard users). Clearly separate Business Context and Technical Context.
* [x] **Kapitel 11 — Risks and Technical Debts:** Document known risks: Silver-Layer without Retention-Policy, dependency on MinIO event ordering, single-column Gold schema preventing multi-source differentiation.
* [x] **Kapitel 12 — Glossary:** Define core terms: Bronze/Silver/Gold Layer, DLQ, Silver Contract, Progressive Disclosure, Probe, Macroscope, Harmonization, Idempotency.
* [x] **Remove Stale `go.work` TODO:** Delete the bullet point *"`go.work`-Setup dokumentieren [...] da die Datei per `.gitignore` nicht versioniert wird"*. The `go.work` and `go.work.sum` files are intentionally versioned as SSoT for Docker multi-stage builds and CI. This is a deliberate monorepo pattern, not an omission.
* [x] **ADR-008: Network Zero-Trust Architecture:** (see Phase 28).
* [x] **Enforce English-Only Language Constraint:** `CLAUDE.md` and `ROADMAP.md` are written entirely in German, violating `02_architecture_constraints.md` ("The official project language is English. This applies strictly to all source code, documentation [...]"). Translate both files to English. All future documentation must be English-only.
* [x] **Extend ClickHouse Gold Schema for Multi-Source Metrics:** The current `aer_gold.metrics` table has only `timestamp` and `value` — no `source`, no `metric_name`. Once a second crawler ships, metrics become indistinguishable. Add `source String` and `metric_name String` columns to the schema. Update `infra/clickhouse/init.sql`, the Python worker's ClickHouse insert logic, and the BFF-API query layer. This is a prerequisite for scaling beyond one data source.

## Phase 16: API Hardening & HTTP Middleware Stack (Remaining) - [x] DONE
*Carried over from the original roadmap.*

* [x] **Rate Limiting:** Token-Bucket or Sliding-Window Rate Limiter as middleware on the BFF-API. Start with a simple in-process implementation (`golang.org/x/time/rate`) and integration tests. Distributed rate limiting via Redis is deferred until horizontal scaling requires it — adding Redis for a single-instance deployment violates Occam's Razor.

## Phase 27: Test & CI Completeness - [x] DONE
*Gaps in test coverage and CI scope reduce confidence in the codebase. These fixes ensure that the safety net catches regressions before they reach production.*

* [x] **Fix Postgres Log-Parsing in Go Testcontainers:** `services/ingestion-api/internal/storage/postgres_test.go` uses `wait.ForLog("database system is ready to accept connections")` — a healthcheck strategy explicitly forbidden by project rules. Replace with `wait.ForSQL()` using the `pgx` driver, consistent with how the Python tests use HTTP-based health probes.
* [x] **Add `pkg/` to CI Lint Scope:** The `go-pipeline` job in `ci.yml` lints only `services/ingestion-api` and `services/bff-api`. The shared `pkg/` module (logger, telemetry, testutils) is not linted. Add a `golangci-lint run` step for `pkg/`.
* [x] **Add `pkg/testutils` tests:** compose_test.go: 4 Tests — Happy Path gegen echte compose.yaml, unbekannter Service, malformed YAML, fehlende Datei.
* [x] **Add E2E Smoke Test to CI:** `scripts/e2e_smoke_test.sh` exists and validates the full pipeline (Ingestion → NATS → Worker → ClickHouse → BFF), but it is not part of CI. Add a dedicated `e2e-smoke` job that runs on `main` pushes (not PRs, to avoid long CI times). Use `docker compose up --build --wait` with the existing script.


## Phase 28: Network Zero-Trust & Port Hardening - [x] DONE
*Phase 20 introduced network segmentation (`aer-frontend` / `aer-backend`), but all backend services still expose ports directly to the host. This undermines the segmentation — any process on the host can bypass Traefik and access databases directly. The architecture should follow a Zero-Trust model: only the reverse proxy is reachable from outside the Docker network.*

* [x] **Remove Non-Essential Host Port Bindings:** Strip the `ports:` directives from all backend-only services in `compose.yaml`: PostgreSQL (5432), ClickHouse (8123, 9002), NATS (4222, 8222), OTel Collector (4317, 4318), MinIO API (9000), Tempo. These services communicate exclusively over the `aer-backend` Docker network — host exposure serves no production purpose. The Ingestion-API (8081) also moves behind the internal network; crawlers run as containers on `aer-backend` or use Traefik.
* [x] **Introduce Docker Compose Profiles for Dev Access:** To preserve developer ergonomics (direct DB access for debugging), introduce a `debug` profile. Services that need host-port exposure for development get `profiles: ["debug"]`. Running `docker compose --profile debug up` exposes them; the default `docker compose up` does not. This gives developers explicit opt-in without weakening the production posture.
* [x] **Route MinIO Console and Grafana Through Traefik:** MinIO Console (9001) and Grafana (3000) are UI-facing services that currently bypass Traefik. Add Traefik labels and route them through the reverse proxy with TLS, consistent with the BFF-API pattern. Remove their direct `ports:` bindings from the default profile.
* [x] **ADR-008: Network Zero-Trust Architecture:** Document the decision in `docs/arc42/09_architecture_decisions.md`. Content: rationale for removing host ports, the `debug` profile pattern, Traefik as the sole ingress point, and the threat model this addresses (lateral movement from host, accidental exposure on VPS).
---

---

### Tier 1: SSoT & Determinism (fix before any new feature work)


---

### Tier 2: Code Quality & Architecture Alignment

---

### Tier 3: Hardening for Production

---

### Tier 4: Documentation & Schema Evolution
