# AĒR Implementation, Refactoring & Scaling Roadmap

This roadmap defines the steps to transition the AĒR base architecture into a scalable, maintainable system developed according to modern standards (CLEAN Code, DRY, Event-Driven).

---

# Completed Phases

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
* [x] **Align Makefile `test-python` with CI:** `make test-python` invokes `./.venv/bin/python -m pytest`, which requires a local venv. CI installs pip globally without a venv. Add a conditional: if `.venv/bin/python` exists use it, otherwise fall back to `python -m pytest`. This makes the Makefile target portable across local and CI environments.

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

## Phase 29: Database Migration Tooling & Source Registry (D-3 + D-5) - [x] DONE
*The current schema-via-init.sql approach has no migration path. Any schema change requires a full volume wipe — this is a structural risk that blocks all future schema evolution. D-5 (hardcoded dummy source) is resolved in the same phase because both share the same root cause: the absence of a proper seeding/migration layer. This phase is a hard prerequisite before Phase 30 (Gold schema extension).*

* [x] **Introduce `golang-migrate` for PostgreSQL:** Add `golang-migrate/migrate/v4` to `services/ingestion-api/go.mod`. Implement a migration runner that executes on service startup (or as a dedicated init container). Versioned SQL files live in `infra/postgres/migrations/` (e.g., `000001_initial_schema.up.sql`, `000001_initial_schema.down.sql`).
* [x] **Migrate existing `init.sql` to Migration 001:** Move the current table definitions (`sources`, `ingestion_jobs`, `documents`) into `000001_initial_schema.up.sql`. The `init.sql` becomes a no-op stub that only creates the `migrate` user and the schema namespace — it no longer creates tables.
* [x] **Remove Hardcoded Dummy Source (D-5):** Delete the `INSERT INTO sources ... 'AER Dummy Generator'` from `init.sql`. Replace it with a dedicated seed migration (`000002_seed_wikipedia_source.up.sql`) that is clearly marked as a dev-only seed. The Wikipedia crawler must resolve its `source_id` dynamically via a `GET /api/v1/sources?name=wikipedia` lookup (or a new admin endpoint), not by assuming `source_id=1`.
* [x] **Introduce Versioned ClickHouse Init Scripts:** ClickHouse has no native migration framework. Implement a convention: `infra/clickhouse/migrations/` with sequentially numbered `.sql` files, executed by the `clickhouse-init` container on startup via a simple idempotent shell runner (`clickhouse-client --multiline < migration.sql`).
* [x] **Add ADR-014: Database Migration Strategy:** Document the decision in `docs/arc42/09_architecture_decisions.md`. Content: rationale for `golang-migrate` over alternatives (Flyway, Liquibase — JVM overhead violates Occam's Razor), the ClickHouse convention, and the no-downtime migration contract.
* [x] **Update `11_risks_and_technical_debts.md`:** Mark D-3 and D-5 as `Resolved (Phase 29)`.

---

## Phase 30: Gold Schema Extension & ClickHouse Migration (D-7) - [x] DONE
*The `aer_gold.metrics` table has only `timestamp` and `value`. With migration tooling now in place (Phase 29), this schema debt can be resolved safely. This phase is blocked by Phase 29 — do not start before migrations are operational.*

* [x] **Define Migration 002 for ClickHouse:** Created `infra/clickhouse/migrations/000002_extend_metrics_schema.sql`. Added `source String`, `metric_name String`, and `article_id Nullable(String)` columns to `aer_gold.metrics` via `ALTER TABLE ... ADD COLUMN IF NOT EXISTS` for safe idempotency.
* [x] **Update Python Worker Insert Logic:** Extended the `INSERT INTO aer_gold.metrics` statement in `services/analysis-worker/internal/processor.py` to populate `source` (from `SilverRecord.source`), `metric_name` (`"word_count"`), and `article_id` (derived from the MinIO object key).
* [x] **Update BFF-API Query Layer:** Extended the ClickHouse query in `services/bff-api/internal/storage/clickhouse.go` to filter by `source` and `metric_name` via optional query parameters. Updated `openapi.yaml` with new parameters (`source`, `metricName`). Regenerated Go code via `oapi-codegen`.
* [x] **Update Testcontainers & Integration Tests:** Updated the Go Testcontainer for ClickHouse and the Python unit tests to validate the extended schema. The `get_compose_image()` / `pkg/testutils` SSoT pattern is maintained.
* [x] **Update `11_risks_and_technical_debts.md`:** Marked D-7 as `Resolved (Phase 30)`.

## Phase 31: Production Dependency Hardening — `psycopg2` Source Build (D-2) - [x] DONE
*`psycopg2-binary` is explicitly not recommended for production by its maintainers due to bundled, potentially outdated `libpq` and SSL/TLS incompatibilities. This is a single-file change with low risk and immediate impact on Trivy scan surface and production correctness.*

* [x] **Switch Production Dockerfile to `psycopg2` (source build):** In `services/analysis-worker/Dockerfile`, add `libpq-dev gcc python3-dev` to the builder stage's `apt-get install`. Replace `psycopg2-binary` with `psycopg2` in `requirements.txt` (production). Keep `psycopg2-binary` in `requirements-dev.txt` for local development and CI to avoid native compilation overhead in test environments.
* [x] **Verify Trivy Scan Passes:** Confirm that the rebuilt image passes the `trivy-scan` CI job without new HIGH/CRITICAL findings. The goal is to eliminate the statically linked `libpq` from the image.
* [x] **Update `11_risks_and_technical_debts.md`:** Mark D-2 as `Resolved (Phase 31)`.

## Phase 32: Silver Layer Retention Policy (R-3) - [x] DONE
*The Silver bucket grows unboundedly. Bronze expires after 90 days, Quarantine after 30 — but Silver has no ILM policy. At current scale this is a low-urgency item, but it must be addressed before the second crawler ships to prevent unbounded storage costs. This phase should be executed once actual Silver growth is measurable (i.e., after Phase 30 and at least one full week of real crawl data).*

* [x] **Measure Silver Bucket Growth Rate:** Before setting a TTL, observe actual Silver object counts over one week of production-like ingestion. Use `mc du minio/silver` or the MinIO Console metrics to establish a baseline.
* [x] **Define and Apply Silver ILM Policy:** Based on measured growth, apply a MinIO ILM expiration rule to the `silver` bucket. A conservative starting value of 365 days is recommended — the Gold layer (ClickHouse) retains all derived metrics independently. Add the policy to `infra/minio/setup.sh` alongside the existing Bronze and Quarantine rules.
* [x] **Document Retention Decision:** Add the chosen TTL and its rationale to `docs/arc42/08_concepts.md` under Data Lifecycle Management (§8.8). Reference the observation period and growth data that informed the decision.
* [x] **Update `11_risks_and_technical_debts.md`:** Mark R-3 as `Resolved (Phase 32)`.

## Phase 33: Ingestion API Authentication (R-5) - [x] DONE
*The Ingestion API has no authentication. This is currently mitigated by network segmentation (it is not exposed via Traefik). However, the moment a remote crawler is introduced, this changes from `Low` to `High` severity. This phase is intentionally deferred until remote crawlers are planned — implementing auth for a purely localhost service would violate Occam's Razor.*

**Trigger condition:** Execute this phase when the first crawler is deployed outside the `aer-backend` Docker network (e.g., a remote VPS, a GitHub Actions runner making ingest calls).

* [x] **Add API-Key Middleware to Ingestion API:** Extracted `apiKeyAuth()` from `services/bff-api/cmd/server/main.go` into `pkg/middleware/apikey.go` (exported as `APIKeyAuth`) to satisfy DRY. Both BFF and Ingestion API now import from the shared package. Applied to all routes except `/api/v1/healthz` and `/api/v1/readyz`.
* [x] **Add `INGESTION_API_KEY` to `.env.example` and Config:** Added the new environment variable to `services/ingestion-api/internal/config/config.go`, `.env.example`, and `compose.yaml`.
* [x] **Update E2E Smoke Test:** `scripts/e2e_smoke_test.sh` now sends the `X-API-Key` header via the `INGESTION_API_KEY` environment variable in the `wget` call to the Ingestion API.
* [x] **Update `11_risks_and_technical_debts.md`:** Marked R-5 as `Resolved (Phase 33)`. Removed stale entries (R-5, D-2) from the risk quadrant chart.

## Phase 34: Persistent Tempo Trace Storage (R-7)  - [x] DONE
*Tempo currently stores traces under `/tmp/tempo/` with no persistent volume. Restarting the container loses all traces. For development this is acceptable, but for production audit trails it is not. This phase is deferred until long-term tracing is a stated operational requirement.*

**Trigger condition:** Execute this phase when operational SLAs require trace retention beyond a single container lifecycle (e.g., post-incident analysis, audit requirements).

* [x] **Mount a Named Docker Volume for Tempo:** Add a `tempo_data` named volume to `compose.yaml`. Mount it at `/var/tempo` inside the Tempo container. Update `infra/observability/tempo.yaml` to point `wal.path` and `local.path` to `/var/tempo/wal` and `/var/tempo/blocks`.
* [x] **Increase Block Retention:** Increase `block_retention` in `tempo.yaml` from `1h` to a value appropriate for the use case (e.g., `72h` for development, `720h` for production). Document the chosen value and its rationale.
* [x] **Update `11_risks_and_technical_debts.md`:** Mark R-7 as `Resolved (Phase 34)`.
---

## Phase 35: CI/Runtime Parity & Dependency Hygiene - [x] DONE
*The CI pipeline tests Python code against a different runtime version than the production Dockerfile. This violates the SSoT principle and undermines scientific reproducibility — a test passing on 3.12 does not guarantee correctness on 3.14. Additionally, an outdated `pytest-asyncio` pin creates latent risk for the async-heavy analysis worker.*

* [x] **Align Python Version in CI with Production Dockerfile:** Updated `.github/workflows/ci.yml` both `python-pipeline` and `dependency-audit` jobs from `python-version: '3.12'` to `python-version: '3.14'` to match the production Dockerfile base image (`python:3.14.3-slim-bookworm`).
* [x] **Fix `pytest-asyncio` Version Pin:** `pytest-asyncio==1.3.0` is the current latest stable release — pin confirmed valid, no change required.
* [x] **Update `11_risks_and_technical_debts.md`:** Added `D-8: CI/Production Python Version Mismatch` and marked it as `Resolved (Phase 35)`.

---

## Phase 36: Observability Scaling Preparedness (R-8) - [x] DONE
*The current OpenTelemetry configuration samples 100% of all traces (`AlwaysSample()`). This is correct for development and low-volume operation, but will become a storage and performance bottleneck when multiple crawlers run concurrently at production throughput. This phase introduces configurable sampling before the first real crawler ships.*

* [x] **Introduce Configurable Trace Sampling:** Replace `sdktrace.AlwaysSample()` in `pkg/telemetry/otel.go` with `sdktrace.ParentBased(sdktrace.TraceIDRatioBased(rate))`, where `rate` is read from a new environment variable `OTEL_TRACE_SAMPLE_RATE` (default: `1.0` — preserving current behavior). Update `pkg/telemetry/otel.go` signature to accept the rate as a parameter. Update both `services/ingestion-api/internal/config/config.go` and `services/bff-api/internal/config/config.go` to include the new variable.
* [x] **Add `OTEL_TRACE_SAMPLE_RATE` to `.env.example` and `compose.yaml`:** Default value `1.0` for development. Document recommended production value (`0.1` — 10% sampling) as an inline comment.
* [x] **Register Risk R-8 in `11_risks_and_technical_debts.md`:** Add `R-8: 100% Trace Sampling Does Not Scale` with severity `Low` and immediately mark it as `Resolved (Phase 36)`. Reference the new environment variable and the recommended production tuning.
* [x] **Update `docs/arc42/08_concepts.md` §8.4 (Observability):** Document the sampling strategy, the environment variable, and the rationale for `ParentBased` (ensures child spans inherit the parent's sampling decision, preventing orphaned trace fragments).

## Phase 37: Architecture Review Documentation & Hardening - [x] DONE
*Final documentation pass before entering the scientific/crawler implementation phase. Ensures all architecture review findings are captured in the canonical documentation, and minor hardening items are addressed.*

* [x] **Verify `GET /api/v1/sources` Endpoint Exists:** Confirm that the Ingestion API exposes `GET /api/v1/sources?name=<n>` as specified in ADR-014 and Phase 29. The `GetSourceByName` method exists in the PostgreSQL adapter, but the HTTP route must be verified in `services/ingestion-api/internal/handler/handler.go`. If missing, implement the handler and add a unit test. This is a hard prerequisite for the Wikipedia crawler's dynamic `source_id` resolution.
* [x] **Verify BFF ClickHouse Query Has Hard Row `LIMIT`:** The documentation (Chapter 4, Chapter 10 QS-P1) describes a hard row limit on `GET /api/v1/metrics` to prevent OOM. Verify that `services/bff-api/internal/storage/clickhouse.go` `GetMetrics()` includes a `LIMIT` clause in the SQL query. If missing, add `LIMIT 10000` (configurable via `BFF_QUERY_ROW_LIMIT` environment variable) and update the BFF config struct.
* [x] **Update `docs/arc42/11_risks_and_technical_debts.md`:** Add `D-9: Ingestion API Source Lookup Endpoint Unverified` — mark as `Resolved (Phase 37)` after verification/implementation.
* [x] **Update `README.md` — Crawler Development Section:** Add a brief "Developing a Crawler" section to `README.md` documenting: (1) the dynamic source resolution pattern (`GET /api/v1/sources?name=<n>`), (2) the Ingestion Contract JSON format (already documented, add cross-reference), (3) the requirement to send the `X-API-Key` header. This prepares the README for the crawler implementation phase that follows.

## Phase 38: Infrastructure Baseline Snapshot & Operations Documentation - [x] DONE
*The infrastructure layer is stable and mature. Before introducing business logic (crawlers, metrics, Silver/Gold schema evolution), we freeze the current state as a recoverable baseline and create a comprehensive operations reference for onboarding developers.*

* [x] **Create Snapshot Branch:** Create a `baseline/v1-infrastructure` branch from `main` at the current HEAD. This branch serves as a read-only fallback — a known-good state of the complete infrastructure before any business-logic changes. The branch must never be pushed to again after creation.
* [x] **Operations Playbook:** Create `docs/operations_playbook.md` as a practical How-To reference for accessing, inspecting, and debugging every infrastructure component (PostgreSQL, ClickHouse, MinIO, NATS, Grafana, Tempo, Prometheus, OTel Collector, application services). The document is deliberately placed outside the arc42 structure — arc42 describes *why* and *what*, the playbook describes *how*. Added as a top-level entry in `mkdocs.yml` navigation.
* [ ] **Update `mkdocs.yml`:** Add `Operations Playbook` as a top-level navigation entry above the arc42 chapters.
---

### BRANCHED FROM MAIN -> baseline/v1-infrastructure ###


## Phase 39: Evolvable Silver Architecture & Source Adapter Pattern - [x] DONE
*The current `SilverRecord` is a flat, monolithic Pydantic model hardwired to one data shape. This is the single biggest architectural blocker for AĒR's scientific evolution. Every future change — new data source, new metadata field, new analytical dimension — currently requires modifying a shared model, risking regressions across the entire pipeline. This phase does not define the final Silver schema (that is a scientific decision, not an engineering one). Instead, it builds the architectural scaffolding that makes schema evolution a routine operation rather than a structural risk.*

*Guiding insight: AĒR is a research instrument, not a product. The Silver schema, Gold metrics, and analysis pipeline will undergo radical changes as interdisciplinary collaboration matures (Chapter 13, §13.6 Open Research Questions). The architecture must treat schema evolution as the normal case, not the exception. Every structural decision in this phase is evaluated against one question: "Does this make it easier or harder to change the schema in six months?"*

* [x] **ADR-015: Evolvable Silver Contract.** Document the architectural decision in `docs/arc42/09_architecture_decisions.md`:
  - The Silver layer is split into two tiers: **`SilverCore`** (universal minimum contract) and **`SilverMeta`** (source-specific context, typed by `source_type`).
  - `SilverCore` defines the absolute minimum a document must have for *any* NLP pipeline to operate: `document_id`, `source`, `source_type`, `raw_text`, `cleaned_text`, `language`, `timestamp`, `url`. These fields are *instrumentally* motivated (the pipeline needs them), not *scientifically* motivated (they don't represent analytical conclusions).
  - `SilverMeta` is a discriminated union that preserves source-specific richness without polluting the core. Each source type defines its own Pydantic model. The meta envelope is explicitly marked as **unstable** — source adapters may add, rename, or restructure meta fields without a formal ADR. Only `SilverCore` changes require an ADR.
  - The ADR must explicitly state: **Both `SilverCore` and `SilverMeta` are provisional.** They represent the current best understanding of what the pipeline needs. As interdisciplinary research (Chapter 13) produces new requirements — new metadata fields, new normalization steps, new language-specific processing — the schema will evolve. The architecture must support this without pipeline-wide regressions.
  - Document the **Schema Evolution Strategy**: new fields are added as `Optional` with defaults. Removed fields are deprecated (kept in the model with a deprecation marker) for one release cycle, then dropped. The Silver bucket is append-only — existing objects are never re-processed to match a new schema version. A `schema_version: int` field in `SilverCore` enables the worker to handle multiple schema generations simultaneously.

* [x] **Implement `SilverCore` Pydantic Model.** Replace `SilverRecord` in `services/analysis-worker/internal/models.py`. Critical changes vs. the current model:
  - Add `document_id: str` — deterministic SHA-256 hash of `source + bronze_object_key`. Enables idempotency checks without MinIO HEAD requests.
  - Add `cleaned_text: str` — separate from `raw_text`. The current processor overwrites `raw_text` with cleaned text, destroying provenance. This violates the Bronze immutability principle at the Silver level.
  - Add `language: str` — ISO 639-1. Default `"und"` (undetermined). Set by the source adapter if known, validated/overridden by a future language detection extractor.
  - Add `source_type: str` — discriminator for `SilverMeta` lookup (e.g., `"rss"`, `"forum"`, `"social"`).
  - Add `schema_version: int` — starts at `2` (v1 = the current `SilverRecord`).
  - Remove `metric_value` and `status` from the Silver model — metric extraction belongs in the Gold layer, not Silver. Processing status belongs in PostgreSQL.

* [x] **Implement Source Adapter Protocol.** Create `services/analysis-worker/internal/adapters/`:
  - `base.py` — `SourceAdapter` protocol: `def harmonize(self, raw: dict, event_time: datetime) -> tuple[SilverCore, SilverMeta | None]`. The adapter is responsible for mapping source-specific raw data to the universal `SilverCore` + its own `SilverMeta`.
  - `registry.py` — `dict[str, SourceAdapter]` mapping `source_type` to adapter instance. The processor looks up the adapter. Unknown `source_type` → DLQ with a clear error message. The registry is assembled in `main.py` (dependency injection), not hardcoded in the processor.
  - `rss.py` — First concrete adapter (Phase 40). Not implemented yet in this phase — only the protocol and registry scaffolding.
  - `legacy.py` — Backward-compatible adapter for existing Wikipedia-era Bronze objects (no `source_type` field). Maps old-format documents to `SilverCore` with `source_type = "legacy"` and `schema_version = 1`.

* [x] **Refactor `DataProcessor`.** The processor becomes source-agnostic again: fetch Bronze → lookup adapter by `source_type` → call `adapter.harmonize()` → validate `SilverCore` → write Silver → pass to metric extractors → write Gold. The processor itself has zero knowledge of RSS, forums, or any specific data source.

* [x] **Update Tests.** Refactor `tests/test_processor.py`: test adapter registry lookup, test legacy adapter backward compatibility, test unknown `source_type` → DLQ, test `schema_version` is written to Silver objects, test `document_id` determinism. All existing tests must continue to pass via the legacy adapter.

* [x] **Update Arc42 Documentation.** Update Chapter 5 (§5.1.2: describe the adapter pattern and schema evolution strategy), Chapter 6 (§6.1: harmonization step now includes adapter lookup), Chapter 12 (Glossary: `SilverCore`, `SilverMeta`, `Source Adapter`, `Schema Version`). Add a paragraph to Chapter 13 (§13.3) noting that all Tier 1/2/3 metrics operate on `SilverCore.cleaned_text` and that `SilverMeta` is available for source-specific enrichment but excluded from core metrics.


## Phase 40: RSS Crawler — Provisional German Institutional Probe - [x] DONE
*This phase implements AĒR's first real data source. The source selection is explicitly **provisional** — it is driven by pragmatic engineering criteria (structured data, ethical simplicity, linguistic homogeneity for NLP validation), not by scientific probe methodology. The Manifesto's Probe Principle (§IV) requires interdisciplinary dialogue for valid probe selection; this dialogue has not yet occurred. The RSS feeds selected here serve as **calibration data** for the pipeline, not as a scientifically representative sample of German discourse.*

*This distinction must be documented clearly: the first probe is an engineering decision, not a research finding. Future probes will be selected through the research process outlined in Chapter 13 (§13.5 Outreach Strategy, §13.6 Open Research Questions).*

* [x] **Document Probe Rationale in Chapter 13.** Add `§13.8 Probe 0: Pipeline Calibration (German Institutional RSS)` to `docs/arc42/13_scientific_foundations.md`. Content:
  - **Purpose:** Engineering calibration of the AĒR pipeline. Validate end-to-end data flow, Silver Contract evolution, metric extraction, and BFF serving with real-world data.
  - **Source Selection Criteria (engineering, not scientific):** publicly available, structured format (RSS/Atom), no authentication required, no ToS restrictions, no personal data, predictable document volume, German-language for NLP model validation.
  - **Milieu Bias Acknowledgment (per Manifesto §III):** This probe captures exclusively institutional and editorial voice. It does not represent "the German public," grassroots discourse, social media dynamics, or any specific demographic. This bias is a documented parameter of the observation, not a defect.
  - **Selected Sources (provisional, subject to change without ADR):**
    - bundesregierung.de RSS — Federal government press releases.
    - tagesschau.de RSS — Public broadcasting news (ARD).
    - 1–2 additional quality press feeds (if publicly accessible).
  - **Limitations:** Editorial content only. No user-generated content. No engagement metrics. No threading or reply structure. Limited to German language. RSS feeds may be incomplete (truncated descriptions, no full article text).
  - **Exit Criteria:** This probe is superseded when a scientifically motivated probe selection is made through the research process (§13.5). The RSS crawler remains operational as one data source among many.

* [x] **Register RSS Sources in PostgreSQL.** Seed migration `infra/postgres/migrations/000003_seed_rss_sources.up.sql`. Each feed as a separate `sources` entry. The migration is additive — existing Wikipedia source is not removed.

* [x] **Implement `crawlers/rss-crawler/`.** Standalone Go binary, own `go.mod`:
  - `main.go` — CLI entry point. Flags: `-config <path>`, `-api-url`, `-api-key`. Reads feed config, iterates feeds, resolves `source_id` per feed via `GET /api/v1/sources?name=<n>`, fetches/parses/translates/submits.
  - `internal/feed/parser.go` — RSS/Atom parsing via `gofeed`. Extracts `title`, `description` (as `raw_text`), `link`, `published`, `categories`, `author`.
  - `internal/feed/translator.go` — Maps parsed items to Ingestion Contract. Sets `source_type: "rss"` in the `data` payload. Key pattern: `rss/<source_name>/<item-guid-hash>/<date>.json`.
  - `internal/state/dedup.go` — Local JSON state file tracking submitted GUIDs per feed. Prevents re-ingestion on repeated runs. State file location configurable via flag.
  - Rate limiting: configurable per-feed delay (default 1s). Respects `robots.txt` where applicable.
  - Feed config file (`feeds.yaml`) as a simple list of `{name, url}` entries. Adding a feed = one YAML entry + one PG seed migration.

* [x] **Implement `RSSAdapter` in the Analysis Worker.** Create `services/analysis-worker/internal/adapters/rss.py` implementing the `SourceAdapter` protocol from Phase 39. Maps RSS-specific raw fields to `SilverCore` + `RSSMeta(feed_url, categories, author, feed_title)`. Register in the adapter registry.

* [x] **Add to `go.work`, Makefile, and CI.** `go.work` entry, lint/test targets, CI pipeline inclusion.

* [x] **Write Tests.** Crawler: parser test against static RSS fixture in `testdata/`, translator contract compliance test, dedup logic test. Worker: `RSSAdapter` unit tests with mock Bronze data.


## Phase 41: Analysis Worker — Extractor Pipeline Architecture - [x] DONE
*The current worker extracts one metric (word count) in a hardcoded step. This phase builds the **extensible extraction framework** — the architectural spine that all future metrics (Tier 1, 2, and 3 from Chapter 13) will plug into. The framework itself is stable infrastructure; the extractors that plug into it are scientifically motivated and will evolve continuously.*

*Critical design constraint: The extractor pipeline must support two processing modes that will coexist long-term:*
- ***Per-document extraction*** *(current model): Each Bronze event triggers extraction on a single document. Suitable for word count, sentiment, NER, temporal stats.*
- ***Corpus-level extraction*** *(future, not implemented in this phase but architecturally anticipated): Methods like TF-IDF, topic modeling (LDA), and co-occurrence networks require statistics across the entire corpus or time windows. These cannot run per-document — they need batch processing on accumulated Silver data. The architecture must not preclude this.*

* [x] **Define `MetricExtractor` Protocol.** Create `services/analysis-worker/internal/extractors/base.py`:
  - `MetricExtractor` protocol: `name: str`, `def extract(self, core: SilverCore) -> list[GoldMetric]`.
  - `GoldMetric` dataclass: `timestamp`, `value`, `source`, `metric_name`, `article_id`. Maps 1:1 to the existing `aer_gold.metrics` ClickHouse schema.
  - `GoldEntity` dataclass (for NER and future structured outputs): `timestamp`, `source`, `article_id`, `entity_text`, `entity_label`, `start_char`, `end_char`. Maps to the future `aer_gold.entities` table (created in Phase 42).
  - The protocol returns `list[GoldMetric]` — one extractor can produce multiple metrics per document (e.g., sentiment produces `sentiment_score` + `sentiment_subjectivity`).

* [x] **Define `CorpusExtractor` Protocol (Interface Only).** Create the protocol definition for future corpus-level extractors: `def extract_batch(self, cores: list[SilverCore], window: TimeWindow) -> list[GoldMetric]`. **Do not implement any corpus extractor in this phase.** The protocol exists to ensure per-document extractors don't accidentally preclude corpus-level analysis. Document in arc42 Chapter 13 (§13.3) that TF-IDF, LDA, and co-occurrence networks will use this interface. Add a note in Chapter 11 (Risks) that corpus-level extraction requires a scheduling mechanism (cron or NATS-triggered batch jobs) not yet implemented.

* [x] **Refactor `DataProcessor` to Use Extractor Pipeline.** Replace hardcoded word count logic:
  - Constructor accepts `extractors: list[MetricExtractor]` (injected in `main.py`).
  - After Silver validation, iterate extractors, collect all `GoldMetric` results, batch-insert into ClickHouse in a single round-trip.
  - Entity handling: extractors that produce `GoldEntity` objects return them separately. The processor routes metrics to `aer_gold.metrics` and entities to `aer_gold.entities` (once the table exists in Phase 42). Until Phase 42, entity-producing extractors are simply not registered.

* [x] **Migrate Word Count to First Extractor.** Move word count logic into `extractors/word_count.py`. The processor no longer knows what `word_count` means — it just runs whatever extractors are registered.

* [x] **Update Tests.** Test extractor registration and pipeline execution. Test that adding/removing an extractor doesn't affect other extractors. Test batch insert with multiple metrics per document. Test graceful handling of a failing extractor (one extractor fails → other extractors' results are still inserted, failed extractor is logged, document is NOT sent to DLQ — partial metric extraction is acceptable).

* [x] **Update Arc42 Documentation.** Chapter 5 (§5.1.2: extractor pipeline pattern), Chapter 6 (§6.1 step 8: N metrics per document), Chapter 8 (add §8.10: Extractor Registration Pattern — how to add a new metric), Chapter 13 (§13.3: note that per-document extractors are now operational, corpus-level is architecturally anticipated but not implemented).


## Phase 42: Provisional Tier 1 Metrics — PoC NLP Extractors - [x] DONE
*This phase implements the first NLP-based metric extractors from Chapter 13 (§13.3.1 Tier 1). Every extractor in this phase is explicitly **provisional** — a proof-of-concept that validates the extractor pipeline architecture with real NLP operations. The specific lexicons, models, and parameters chosen here are engineering defaults, not scientifically validated choices. They will be revisited, replaced, or recalibrated when interdisciplinary collaboration (§13.5) provides methodological grounding.*

*Each extractor documents its own limitations and provisional status in its docstring and in Chapter 13.*

* [x] **Language Detection Extractor (Provisional).** `extractors/language.py`. Uses `langdetect` (or `lingua-py`) with a fixed seed for determinism. Produces `metric_name = "language_confidence"`. Sets `SilverCore.language` during adapter harmonization. **Provisional note:** Language detection accuracy varies by text length and domain. Short RSS descriptions may produce unreliable results. A production-grade implementation may require corpus-level language profiling or multilingual model stacking. Document limitations in Chapter 13.

* [x] **Lexicon-Based Sentiment Extractor (Provisional).** `extractors/sentiment.py`. Uses SentiWS (Leipzig University, CC-BY-SA) for the German probe. Produces `metric_name = "sentiment_score"`. **Provisional note:** SentiWS is a word-level polarity lexicon. It does not handle negation, irony, domain-specific language, or compositionality. It is chosen because it is deterministic, auditable, and German-language — not because it is the best sentiment method. The specific lexicon, scoring algorithm, and normalization will change when CSS researchers (§13.5) provide validated alternatives. Pin lexicon version. Store `lexicon_version` hash as a separate metric for auditability.

* [x] **Temporal Distribution Extractor.** `extractors/temporal.py`. Pure metadata, no NLP. Produces `publication_hour` and `publication_weekday`. This is the one extractor in this phase that is *not* provisional — temporal metadata extraction is methodologically stable.

* [x] **Named Entity Extraction (Provisional).** `extractors/entities.py`. Uses spaCy `de_core_news_lg`. **Provisional note:** spaCy NER on RSS feed descriptions (which are often short, truncated summaries) will produce different results than NER on full articles. Entity linking (resolving "Merkel" to a canonical entity) is not implemented — raw entity spans are stored. The model version, entity taxonomy, and post-processing will evolve with the research. Pin model version in `requirements.txt`.
  - **ClickHouse Migration 003:** `aer_gold.entities` table (`timestamp`, `source`, `article_id`, `entity_text`, `entity_label`, `start_char`, `end_char`). MergeTree, ordered by `(timestamp, source)`, 365-day TTL.
  - Emit `entity_count` as a metric in `aer_gold.metrics` for dashboard aggregation.

* [x] **spaCy Model Management.** The `de_core_news_lg` model (~500MB) is downloaded via `requirements.txt` with exact version pin. Document the download URL and version in Chapter 13. Consider caching in a named Docker volume.

* [x] **Update Dependencies.** Add to `requirements.txt` with exact pins: `spacy`, `de-core-news-lg`, `langdetect` (or `lingua-py`). Run `pip-audit`. Update `requirements-dev.txt`.

* [x] **Update Tests.** Per-extractor unit tests with deterministic inputs. Each test asserts that the extractor produces expected `metric_name` values and that output values are within expected ranges (sentiment ∈ [-1, 1], language confidence ∈ [0, 1], hour ∈ [0, 23]). Integration test: process a real German text through the full extractor pipeline.

* [x] **Update Arc42 Documentation.** Chapter 13 (§13.3.1): mark Tier 1 methods as "Provisional PoC — Phase 42" (not "Implemented"). Add limitation notes for each method. Chapter 5 (§5.1.4: document `aer_gold.entities` table). Chapter 11 (add risk: spaCy model dependency, ~500MB download). Chapter 12 (Glossary: `SentiWS`, `MetricExtractor`, `Provisional Metric`).

 Key Design Decisions
  - No SilverCore mutation — Extractors receive immutable SilverCore. Language detection results go to Gold metrics, not back into the Silver record.                                                        
  - Graceful degradation — Missing SentiWS files → no sentiment metrics. Missing spaCy model → no NER. No crashes.                                                                                           
  - Doc caching — NER extractor caches spaCy doc between extract() and extract_entities() to avoid processing text twice.                                                                                    
  - SentiWS not bundled — Lexicon files must be downloaded separately (CC-BY-SA license). Extractor produces empty results without them.

## Phase 43: BFF API Extension & End-to-End Pipeline Validation - [x] DONE
*The BFF API currently serves one endpoint returning flat time-series data. With multiple metric types and entities, the API needs targeted extensions. This phase also validates the complete pipeline end-to-end — from RSS crawl through Gold metrics to API response — and retires the Wikipedia PoC.*

* [x] **Extend `MetricDataPoint` Response Schema.** Extended from `{timestamp, value}` to `{timestamp, value, source, metricName}`. Updated ClickHouse query to include `source` and `metric_name` in `SELECT` and `GROUP BY`. Storage layer returns named `MetricRow` struct. Regenerated stubs via `make codegen`.

* [x] **Add `GET /api/v1/entities` Endpoint.** Queries `aer_gold.entities` with aggregation: `SELECT entity_text, entity_label, count() as count, groupArray(DISTINCT source) as sources ... GROUP BY entity_text, entity_label ORDER BY count DESC`. Parameters: `startDate`, `endDate` (required via handler validation), `source`, `label` (optional), `limit` (default 100, max 1000). Full OpenAPI spec, codegen, handler, ClickHouse query, handler unit tests, and storage integration tests.

* [x] **Add `GET /api/v1/metrics/available` Endpoint.** Returns `SELECT DISTINCT metric_name FROM aer_gold.metrics ORDER BY metric_name`. Simple JSON string array response. Enables future frontends to discover metrics dynamically.

* [x] **Update OpenAPI Specification.** Added `/entities` and `/metrics/available` paths with schemas (`EntityResult.yaml`) and parameters (`label.yaml`, `limit.yaml`). Extended `MetricDataPoint.yaml` with `source` and `metricName` fields. Ran `make codegen`. Implemented handlers and storage layer. Added integration tests for all new endpoints.

* [x] **End-to-End Validation Script.** Rewrote `scripts/e2e_smoke_test.sh`:
  1. Starts full stack via `docker compose up --build --wait`.
  2. Runs a Python HTTP fixture server on `aer-backend` serving deterministic RSS XML (`scripts/e2e_fixtures/test_feed.xml`).
  3. Runs the RSS crawler binary inside a temporary Alpine container on the Docker network, pointed at the fixture server and the ingestion API.
  4. Waits for pipeline processing.
  5. Asserts `GET /api/v1/metrics?metricName=word_count` returns data.
  6. Asserts `GET /api/v1/metrics?metricName=sentiment_score` returns values within `[-1, 1]`.
  7. Asserts `GET /api/v1/entities` returns results.
  8. Asserts `GET /api/v1/metrics/available` lists all expected metric names.

* [x] **Retire Wikipedia PoC Crawler.** Removed `crawlers/wikipedia-scraper/`. Removed `go.work` entry. The `wikipedia` source in PostgreSQL seeds (`000002_seed_wikipedia_source.sql`) is kept for backward compatibility with existing test data and Silver objects.

* [x] **Update Arc42 Documentation.** Chapter 3 (§3.2.1: new BFF endpoints in External Interfaces table and business context diagram). Chapter 5 (§5.1.3: extended API contract with three data endpoints; §5.1.7: documented Wikipedia scraper retirement). Chapter 10 (added QS-R5 for entity label filtering, QS-P4 for multi-metric filtering). Chapter 7 (§7.8: port table updated with new endpoints). Chapter 13 (§13.8: Probe 0 status updated to "operational").


## Code Review: Phasen 39–43

## Phase 44: Extractor Pipeline Hardening — Protocol Correctness & DRY (Findings 1, 2, 5) - [x] DONE
*The NER extractor uses fragile `id()`-based caching and an implicit `extract_entities()` method that is not part of the `MetricExtractor` protocol. The processor calls it via `hasattr()` — ad-hoc polymorphism that bypasses the protocol system. Additionally, the processor duplicates the quarantine routing block three times. This phase makes the extractor contract explicit and the processor DRY.*

* [x] **Introduce `EntityExtractor` Protocol.** Create a second protocol in `extractors/base.py`: `EntityExtractor(MetricExtractor)` with `def extract_entities(self, core: SilverCore, article_id: str | None) -> list[GoldEntity]`. The `NamedEntityExtractor` implements `EntityExtractor`. The processor checks `isinstance(extractor, EntityExtractor)` instead of `hasattr()`. This makes the contract explicit and type-checkable.
* [x] **Replace `id()`-Based Doc Caching with Single-Pass Extraction.** Refactor `NamedEntityExtractor` to process the spaCy doc once in a unified method (e.g., `_extract_all()`) called by both `extract()` and `extract_entities()`. The method returns `(list[GoldMetric], list[GoldEntity])`. The processor calls the unified path for `EntityExtractor` instances. Remove `self._last_doc` and `self._last_core_id` — no mutable instance-level cache. Document in `extractors/base.py` that extractors must be stateless between documents.
* [x] **Extract Quarantine Helper in Processor.** Refactor the three identical quarantine blocks in `processor.py` into a single `_quarantine(self, obj_key, raw_content, reason, span)` method. Each call site passes only the reason string. Reduces ~30 lines of duplication to ~3 call sites.
* [x] **Update Tests.** Add test for `isinstance(NamedEntityExtractor, EntityExtractor)`. Add test that an extractor with a non-callable `extract_entities` attribute does not crash the processor. Verify quarantine helper produces identical span attributes and metric increments.
* [x] **Update Arc42 Documentation.** Chapter 8 (§8.10): document the `EntityExtractor` sub-protocol. Chapter 5 (§5.1.2): note stateless extractor requirement.


## Phase 45: Language Detection — Persist Detected Language (Finding 3) - [x] DONE
*The `LanguageDetectionExtractor` stores `language_confidence` but discards the detected language code itself. A confidence score without the corresponding classification is analytically useless — one cannot answer "what percentage of documents are German?" from the Gold layer alone.*

> **Inline ADR — Phase 45 Decision: Dedicated Table vs. Metric Encoding.**
> Option (a) — a dedicated `aer_gold.language_detections` table — was chosen over option (b) — encoding language codes as metric values via hash/enum mapping. Rationale: (1) language codes are categorical, not numerical; forcing them into the `float64` `value` column of `aer_gold.metrics` would require a lossy encoding and a separate lookup table to decode, violating the transparency principle. (2) A dedicated table allows storing ranked candidates (rank 1–N from `detect_langs()`), preserving the full probabilistic output for downstream analysis. (3) The pattern is consistent with `aer_gold.entities` (Phase 42) — structured extraction output gets its own table. The table schema includes a `rank UInt8` column not in the original specification, enabling storage of all language candidates per document rather than just the top-1.

* [x] **Add `detected_language` String Metric.** Chose option (a): dedicated `aer_gold.language_detections` table (see inline ADR above).
* [x] **Create ClickHouse Migration 004.** `aer_gold.language_detections` table: `(timestamp DateTime, source String, article_id Nullable(String), detected_language String, confidence Float64, rank UInt8)`. MergeTree, ordered by `(timestamp, source)`, 365-day TTL.
* [x] **Extend `LanguageDetectionExtractor`.** Implements `LanguageDetectionPersistExtractor` protocol (following the `EntityExtractor` pattern from Phase 44). Single-pass `extract_all()` returns both `GoldMetric` (language_confidence) and `GoldLanguageDetection` records. Processor dispatches via `isinstance()`.
* [x] **Add BFF Endpoint `GET /api/v1/languages`.** Returns aggregated language distribution per source: `SELECT detected_language, count() as count, avg(confidence) as avg_confidence ... GROUP BY detected_language ORDER BY count DESC`. Added to OpenAPI spec, codegen, handler, storage, and tests.
* [x] **Update E2E Smoke Test.** Assert that `GET /api/v1/languages` returns at least one entry with `detected_language = "de"`.
* [x] **Update Arc42 Documentation.** Chapter 5 (§5.1.3: new BFF endpoint). Chapter 5 (§5.1.4: new ClickHouse table). Chapter 13 (§13.3.1: update language detection status).

## Phase 46: Sentiment Provenance & Metric Hygiene (Findings 4, 8) - [x] DONE
*The `lexicon_version` metric stores a truncated hash as a float — it is neither human-readable nor useful as a time-series value. Provenance metadata (which lexicon version produced which score) does not belong in the metrics table. This phase moves provenance to the correct layer and cleans up the metric schema.*

* [x] **Remove `lexicon_version` Metric from `SentimentExtractor`.** The sentiment extractor should produce only `sentiment_score`. Lexicon version provenance belongs in the Silver envelope (as part of `SilverMeta`) or as a structured log, not as a ClickHouse time-series metric.
* [x] **Add Lexicon Version to Silver Envelope.** Extend `SilverMeta` or introduce a new `extraction_provenance` field in `SilverEnvelope` that records which extractor versions, model versions, and lexicon hashes were used during processing. This is a metadata concern, not an analytical one. The exact schema is deferred to this phase — keep it minimal (a `dict[str, str]` mapping extractor name to version hash).
* [x] **Update E2E Smoke Test.** Remove `lexicon_version` from the expected metrics list in `EXPECTED_METRICS`. Add `lexicon_version` absence assertion. Verify `sentiment_score` is still present.
* [x] **Update Arc42 Documentation.** Chapter 8 (§8.10): document the provenance pattern. Chapter 13 (§13.3.1): remove `lexicon_version` metric reference.

## Phase 47: BFF API Consistency & Input Validation (Findings 6, 7, 10) - [x] DONE
*The BFF API has inconsistent date parameter handling (`/metrics` silently defaults, `/entities` rejects), LIMIT is validated in the wrong layer, and ClickHouse queries use string interpolation for integer parameters.*

* [x] **Unify Date Parameter Handling.** Make `startDate` and `endDate` required for all data endpoints (`/metrics`, `/entities`, `/metrics/available`). Remove the silent 24-hour fallback from `GetMetrics`. Update the OpenAPI spec to mark both parameters as `required: true`. Regenerate stubs via `make codegen`. This is a breaking API change — document it in the changelog and bump the API version comment in the spec.
* [x] **Move `limit` Validation to Handler Layer.** In `GetEntities`, validate `limit` in the handler: `limit < 1 || limit > 1000 → 400 Bad Request`. Remove the silent correction in `clickhouse.go`. The storage layer should trust its inputs (defense in depth remains as a panic guard, not as business logic).
* [x] **Parameterize LIMIT in ClickHouse Queries.** Replace `fmt.Sprintf(..., limit)` and `fmt.Sprintf(..., s.rowLimit)` with proper query parameter binding (`$N`). Verify that the ClickHouse Go driver supports parameterized LIMIT clauses. If not, document the limitation as an inline comment and add an explicit `if limit < 0 { limit = 100 }` guard before interpolation.
* [x] **Update Handler Unit Tests.** Add test: `GetMetrics` without `startDate` or `endDate` returns 400. Add test: `GetEntities` with `limit=0` or `limit=5000` returns 400. Update existing tests that relied on the silent defaults.
* [x] **Update Arc42 Documentation.** Chapter 5 (§5.1.3): document breaking change in date parameter semantics. Chapter 10: update quality scenarios for input validation.

## Phase 48: Temporal Extractor Defensive Guards & Extractor Robustness (Finding 9) - [x] DONE
*The `TemporalDistributionExtractor` assumes UTC timestamps without validation. While adapters currently set UTC correctly, the extractor should be self-defending — a non-UTC timestamp would silently produce wrong hour/weekday metrics without any indication of error.*

* [x] **Add UTC Assertion in `TemporalDistributionExtractor`.** Before extracting `hour` and `weekday`, assert `core.timestamp.tzinfo is not None` and that the UTC offset is zero. If the timestamp is naive or non-UTC, log a warning and return an empty list (consistent with other extractors' graceful degradation). Do not raise an exception — extractors must not crash the pipeline.
* [x] **Add UTC Assertion in `SilverCore` Pydantic Validator.** Add a Pydantic `field_validator` on `timestamp` that ensures the value is timezone-aware. Naive datetimes should be rejected at the Silver contract level, not at individual extractors. This is the architecturally correct fix — the extractor guard is defense-in-depth.
* [x] **Update Tests.** Add test: `TemporalDistributionExtractor` with naive datetime returns empty list. Add test: `SilverCore` with naive timestamp raises `ValidationError`. Ensure all existing test fixtures use `tzinfo=timezone.utc` (they already do — verify no regressions).
* [x] **Update Arc42 Documentation.** Chapter 5 (§5.1.2): document UTC enforcement at the Silver contract level.

## Phase 49: BFF Query Performance — Available Metrics Caching (Finding 11) - [x] DONE
*`GET /api/v1/metrics/available` executes `SELECT DISTINCT metric_name` on every call — a full table scan on a growing table. With only a handful of distinct metric names that change infrequently (only when new extractors are deployed), this is wasteful. This phase adds a minimal in-process cache.*

* [x] **Implement TTL Cache for `GetAvailableMetrics`.** Add a simple in-process cache in `clickhouse.go`: a `sync.RWMutex`-protected struct holding `([]string, time.Time)`. Cache TTL: 60 seconds (configurable via `BFF_METRICS_CACHE_TTL_SECONDS`, default `60`). On cache miss or expiry, execute the query and refresh. The cache is invalidated on TTL expiry only — no event-driven invalidation needed at this scale.
* [x] **Add Cache TTL to Config.** Add `BFF_METRICS_CACHE_TTL_SECONDS` to `.env.example`, `compose.yaml`, and `services/bff-api/internal/config/config.go`.
* [x] **Update Tests.** Add test: two consecutive calls within TTL result in only one ClickHouse query. Add test: call after TTL expiry triggers a fresh query. Verify thread safety under concurrent access.
* [x] **Update Arc42 Documentation.** Chapter 8 (§8.4 or new §8.11): document the caching strategy and its rationale (Occam's Razor — no Redis, no distributed cache, just in-process TTL).

## Phase 50: CI Pinning Compliance & Makefile Portability (Findings 1, 2, 3, 4, 9) - [x] DONE
*These findings represent violations of the project's internal SSoT/Pinning policy and portability gaps that could block new contributors. Addressing them ensures consistent builds across environments.*

* [x] **Pin CI Tooling Versions.** Hard-pin `oapi-codegen`, `govulncheck`, and `pip-audit` to exact versions in the GitHub Actions workflow or Makefile to prevent silent breakages from upstream updates.
* [x] **Add .venv Fallback for `make lint`.** Update the `lint` target in the Makefile to include a virtual environment fallback for the Python analysis worker, mirroring the existing `if/else` logic used in the `test-python` target.
* [x] **Enforce Environment Variables for `make crawl`.** Ensure the `make crawl` target explicitly loads the `.env` file, or update the Makefile to document/enforce the required flags so the crawler doesn't fail due to missing credentials.
* [x] **Adjust tests / e2e-smoke test if necessary: scripts/e2e_smoke_test.sh** — No changes required; the script already sources `.env` at startup.
* [x] **Document the changes in the necessary files (arc42, README.md, operational_playbook.md, Makefile if necessary)**

## Phase 51: Cache Correctness & Crawler Resilience (Findings 7, 8, 10) - [x] DONE
*Finding 7 is a functional bug in the newly introduced metrics cache, while findings 8 and 10 pose liveness risks as the system scales. Fixing these improves data accuracy and crawler stability.*

* [x] **Fix `GetAvailableMetrics` Cache Keying.** The current cache does not account for date ranges. Update the cache to use `(startDate, endDate)` as part of the key, or invalidate the cached data when a query with a different date range is received.
* [x] **Configure HTTP Timeouts in RSS Crawler.** Enforce a strict HTTP timeout (e.g., 30 seconds) in the RSS crawler's HTTP client to prevent indefinite hangs on unresponsive upstream feeds.
* [x] **Propagate `context.Context` and Delay State Writes.** Thread `context.Context` through the crawler for proper cancellation handling. Ensure the deduplication state file (`.rss-crawler-state.json`) is only written to disk *after* a successful batch ingestion to prevent data loss on intermediate failures.
* [x] **Adjust tests / e2e-smoke test if necessary: scripts/e2e_smoke_test.sh**
* [x] **Document the changes in the necessary files (arc42, README.md, operational_playbook.md, Makefile if necessary)**

## Phase 52: Metadata Lifecycle & Extractor Dispatch Refactoring (Findings 11, 12) - [x] DONE
*These represent accepted technical debt. Addressing them now creates a cleaner and more scalable foundation before onboarding additional uncoupled data sources.*

* [x] **Implement PostgreSQL Retention Policy.** Application-level cleanup routine chosen over `pg_cron` (requires external extension) and table partitioning (over-engineering at current scale). Migration `000004` adds `idx_documents_ingested_at` and `idx_ingestion_jobs_started_at` indexes. `startRetentionCleanup` goroutine in `ingestion-api/cmd/api/main.go` runs every 24h, deleting records older than 90 days (matching MinIO bronze ILM). Documents deleted first (FK constraint), then orphaned completed/failed jobs.
* [x] **Unify Extractor Protocol.** `ExtractionResult` dataclass introduced in `extractors/base.py`. `MetricExtractor` protocol now requires a single `extract_all() -> ExtractionResult`. `EntityExtractor` and `LanguageDetectionPersistExtractor` sub-protocols removed. Processor dispatch loop reduced to three lines — no isinstance checks. All five extractors updated. 76 Python tests pass.
* [x] **Adjust tests / e2e-smoke test if necessary: scripts/e2e_smoke_test.sh**
* [x] **Document the changes in the necessary files (arc42, README.md, operational_playbook.md, Makefile if necessary)**


## Phase 53: Infrastructure Startup Consistency (Findings 5, 6) - [x] DONE
*The `make infra-up` command must deterministically boot the complete backend stack to avoid developer confusion and manual interventions.*

* [x] **Include `traefik` and `nats-init` in `infra-up`.** Explicitly added `traefik` and `nats-init` to the `docker compose up` command in `infra-up`; `traefik` also added to `infra-down`.
* [x] **Update Operations Playbook.** Updated `make infra-up` description to reflect Traefik inclusion.
* [x] **Check if make debug-up is working.** `debug-up` is correct: uses `--profile debug` to start the `debug-ports` socat proxy. Requires `make up` first (enforced by `depends_on: ingestion-api`), which is correctly documented in the ops playbook.
* [x] **Adjust tests / e2e-smoke test if necessary: scripts/e2e_smoke_test.sh.** No changes needed — e2e test uses `docker compose up --build --wait -d` which starts the full stack.
* [x] **Document the changes in the necessary files (arc42, README.md, operational_playbook.md, Makefile if necessary).** Updated `README.md`, `docs/operations_playbook.md`, `docs/arc42/07_deployment_view.md`, and `Makefile`.

## Phase 54: `.tool-versions` as SSoT for Developer Tooling Versions - [x] DONE
*The CI workflow (`ci.yml`) previously served as the de-facto source of truth for developer tool versions (golangci-lint, oapi-codegen, govulncheck, pip-audit). The Makefile `setup` target extracted these versions via fragile `grep` patterns against YAML — a brittle coupling that breaks when the workflow format changes. A dedicated `.tool-versions` file provides a format that both Make (`include`) and CI (`$GITHUB_ENV`) can consume natively, without parsing YAML.*

* [x] **Create `.tool-versions` SSoT file.** Plain `KEY=VALUE` format, compatible with Make `include` and shell `source`. Contains: `GOLANGCI_LINT_VERSION`, `OAPI_CODEGEN_VERSION`, `GOVULNCHECK_VERSION`, `PIP_AUDIT_VERSION`.
* [x] **Update Makefile.** `include .tool-versions` at the top. Simplified `setup` target to use Make variables directly instead of grep-based extraction from `ci.yml`.
* [x] **Update CI workflow (`.github/workflows/ci.yml`).** Load `.tool-versions` into `$GITHUB_ENV` via `grep -E '^[A-Z_]+=' .tool-versions >> $GITHUB_ENV` (strict positive matching, no negation). Replaced hardcoded version strings with `${{ env.VAR }}` references. Updated Go tools cache key from `hashFiles('.github/workflows/ci.yml')` to `hashFiles('.tool-versions')`.
* [x] **Update Arc42 documentation.** Migrated all tool version references from "versions declared in `ci.yml`" to "versions declared in `.tool-versions`":
  - `02_architecture_constraints.md` — Removed hardcoded version strings from Local Dependency Auditing constraint, added `.tool-versions` as SSoT constraint row.
  - `08_concepts.md` — Updated tooling version pinning paragraph and cache key description.
  - `12_glossary.md` — Extended SSoT definition to include `.tool-versions` for developer tooling.
* [x] **Update README.md.** Updated `make setup` description and CI pipeline section to reference `.tool-versions` as SSoT for tool versions.
* [x] **Update CLAUDE.md.** Added `.tool-versions` to repository structure, added hard rule 1b, updated CI section.
* [x] **Adjust tests / e2e-smoke test if necessary: `scripts/e2e_smoke_test.sh`.** No changes needed — e2e test does not reference tool versions.
* [x] **Validate.** `make lint` passes. `make -n setup` dry-run confirms versions are correctly resolved from `.tool-versions`.

## Phase 55: Privacy Architecture & Responsible Use License - [x] DONE
*Created by feedback from Prof. Dr. Dirk Helbing (ETH Zürich COSS), this phase operationalizes AĒR's ethical commitment (Manifesto §VI) as architectural constraints and legal safeguards. The privacy framework addresses re-identification risks for future probes beyond Probe 0, while the license update codifies the absolute prohibition of surveillance, micro-targeting, and political manipulation.*

* [x] **Add WP-006 §7: Data Protection by Design.** Insert new section after §6 (Reflexive Architecture) in both English and German versions. Content: anonymization framework (irreversible identifier removal at Bronze→Silver boundary, k-Anonymity/l-Diversity enforcement at Silver→Gold boundary, entity anonymization for private persons, explicit data exclusions). Add two new research questions (Q8: minimum aggregation granularity, Q9: stylometric fingerprinting risk). Renumber existing §7 (Open Questions) to §8.
* [x] **Update Manifesto §VI.** Expand the ethical commitment from a two-sentence statement to a four-pillar framework: Collective Anonymity, Methodological Transparency, Prohibited Use, No Digital Twins. Reference the license §3 and WP-006 §7.
* [x] **Add Arc42 §13.9: Data Protection Architecture.** New section documenting anonymization-by-layer, explicit data exclusions, and privacy risk classification by probe type (institutional: low, public media: low, social media: high, forums: medium-high).
* [x] **Update LICENSE.md with Responsible Use Restrictions.** Replace existing license with expanded version containing §3 (Responsible Use Restrictions): eight absolute prohibitions (surveillance, micro-targeting, political manipulation, commercial profiling, military/intelligence use, disinformation, digital twin construction, discourse suppression). Add §3.2 (Permitted Scientific Use with ethics board requirement). Add §3.3 (Enforcement: automatic termination on violation).
* [x] **Update Working Papers License to CC BY-NC 4.0.** Change from CC BY 4.0 to CC BY-NC 4.0 to prevent commercial exploitation of the methodological frameworks for purposes incompatible with AĒR's mission. Add Responsible Use clause referencing the main project license.
* [x] **Update WP-006 Appendix C (Consolidated Research Question Index).** Add the new privacy-related questions (Q8, Q9) under a new target discipline category: "Privacy Engineering / Data Protection."
* [x] **Validate documentation consistency.** Verify all cross-references between Manifesto §VI, WP-006 §7, Arc42 §13.9, and LICENSE.md §3 are correct. Verify the research question numbering is consistent across English and German versions.


## Phase 56: Structural Decomposition — Analysis Worker Business Logic - [x] DONE
*The Analysis Worker's `processor.py` and `storage.py` have accumulated multiple responsibilities in single files. This phase decomposes them along existing responsibility boundaries — no new abstractions, no new patterns. The architecture (Adapter Pattern, Extractor Protocol, Medallion flow) remains unchanged.*

* [x] **Split `internal/processor.py` into focused modules.** The processor currently handles orchestration, quarantine routing, and Silver envelope construction in one file. Decompose into:
  - `internal/processor.py` — Orchestration only: fetch Bronze → lookup adapter → validate → extract → persist. Thin dispatcher delegating to the modules below. All public method signatures remain identical.
  - `internal/quarantine.py` — `_quarantine()` helper, DLQ serialization logic, quarantine bucket constants. Extracted from the three formerly duplicated quarantine blocks (consolidated in Phase 48).
  - `internal/silver.py` — Silver envelope construction, `SilverMeta` assembly, provenance metadata, MinIO Silver upload logic.
* [x] **Split `internal/storage.py` into one file per backend.** The current file initializes PostgreSQL, MinIO, and ClickHouse with connection pooling and retry logic in a single module. Decompose into:
  - `internal/storage/__init__.py` — Re-exports all public symbols for backward compatibility. No external import path changes.
  - `internal/storage/minio_client.py` — MinIO initialization, `tenacity` retry decorator, health check.
  - `internal/storage/clickhouse_client.py` — `ClickHousePool` (backed by `queue.Queue`), batch insert, health check.
  - `internal/storage/postgres_client.py` — `psycopg2.ThreadedConnectionPool`, document status queries, retention cleanup queries.
* [x] **Update `internal/main.py` imports.** Adjust dependency injection wiring to use the new module paths. No behavioral change.
* [x] **Validate.** `make test-python` (all 76+ unit tests pass), `make lint` (ruff clean), `make audit-python` (pip-audit clean). No test logic changes in this phase — tests are refactored separately in Phase 55.


## Phase 57: Structural Decomposition — Analysis Worker Tests - [x] DONE
*`tests/test_processor.py` exceeds 1000 lines and covers 7+ distinct concerns. This phase splits it into focused test modules organized by the aspect of the pipeline they validate. Shared fixtures move to `conftest.py`.*

* [x] **Extract shared test infrastructure into `tests/conftest.py`.** Move all shared fixtures, constants, and helper classes:
  - Fixtures: `mock_minio`, `mock_clickhouse`, `mock_pg_pool`, `adapter_registry`, `dummy_span`, `processor`.
  - Constants: `VALID_BRONZE_DATA`, `VALID_RSS_BRONZE_DATA`, `DUMMY_EVENT_TIME`, `EXPECTED_WORD_COUNT`.
  - Helper classes: `StubExtractor`, `FailingExtractor`, `MalformedExtractor`.
  - Helper function: `_make_processor()`.
* [x] **Split `tests/test_processor.py` into focused test modules:**
  - `tests/test_bronze_fetch.py` — Bronze object retrieval, malformed JSON, missing objects, MinIO connection failures.
  - `tests/test_silver_validation.py` — Silver Contract validation, DLQ/quarantine routing, quarantine payload encoding, quarantine payload length, quarantine span attributes.
  - `tests/test_adapter_registry.py` — Adapter lookup for known/unknown `source_type`, `supported_types()`, legacy adapter backward compatibility.
  - `tests/test_extractor_pipeline.py` — Multiple extractors, failing extractors, malformed extractors, empty extractor list, all-extractors-fail scenario, protocol compliance (`MetricExtractor` satisfaction).
  - `tests/test_entity_extraction.py` — NER-specific tests, `NamedEntityExtractor` protocol compliance, entity count metrics.
  - `tests/test_language_detection.py` — Language detection persistence, confidence values, `language_detections` ClickHouse insert, pipeline with/without language detection.
  - `tests/test_idempotency.py` — Duplicate document detection, `_get_document_status` checks, skip-on-processed behavior.
  - `tests/test_full_pipeline.py` — End-to-end integration tests: RSS document through all Tier 1 extractors, Silver upload verification, Gold insert verification, status transitions.
* [x] **Delete original `tests/test_processor.py`.** All tests now live in their respective modules. No test is removed — the total test count remains identical.
* [x] **Validate.** `make test-python` (identical test count, all green), `make lint`, `make audit-python`. CI pipeline (`python-pipeline` job) passes without modification.

## Phase 58: Structural Decomposition — BFF API Business Logic - [x] DONE
*The BFF API's `clickhouse.go` storage layer accumulates all query logic (metrics, entities, available metrics, caching) in a single file. This phase splits it by query domain. Handler logic is already cleanly separated from generated code via `oapi-codegen` — no handler changes needed.*

* [x] **Split `internal/storage/clickhouse.go` into domain-specific query modules:**
  - `internal/storage/clickhouse.go` — Connection setup (`NewClickHouseStorage`), health check, shared types (`ClickHouseStorage` struct), interface definition (`MetricsStore`).
  - `internal/storage/metrics_query.go` — `GetMetrics()`, `GetAvailableMetrics()`, metrics response cache logic.
  - `internal/storage/entities_query.go` — `GetEntities()` with label/source filtering and aggregation.
* [x] **Verify interface compliance.** The `MetricsStore` interface in `internal/handler/` must remain satisfied. No signature changes.
* [x] **Validate.** `make test-go` (all integration tests pass), `make test-go-pkg`, `make lint`, `make audit-go`, `make codegen && git diff --exit-code` (no contract drift).

## Phase 59: Structural Decomposition — BFF API Tests - [x] DONE
*The BFF API's ClickHouse integration tests cover metrics queries, entity queries, and available-metrics queries in a single test file. This phase splits them to mirror the storage module decomposition from Phase 56.*

* [x] **Split `internal/storage/clickhouse_test.go` into domain-specific test files:**
  - `internal/storage/clickhouse_test.go` — Shared test setup: Testcontainer initialization, schema bootstrapping, `TestMain` or `TestSuite` setup, shared helper functions.
  - `internal/storage/metrics_query_test.go` — Integration tests for `GetMetrics()` and `GetAvailableMetrics()`: time-range queries, downsampling, source/metricName filtering, cache behavior, empty result sets.
  - `internal/storage/entities_query_test.go` — Integration tests for `GetEntities()`: label filtering, source filtering, aggregation, limit enforcement, empty result sets.
* [x] **Split handler unit tests if applicable.** If `internal/handler/handler_test.go` exceeds 300 lines, split into `metrics_handler_test.go` and `entities_handler_test.go`. If under 300 lines, leave as-is.
* [x] **Validate.** `make test-go`, `make lint`, `make audit-go`. CI pipeline (`go-pipeline` job) passes without modification.

## Phase 60: Structural Decomposition — Ingestion API - [x] DONE
*The Ingestion API follows Clean Architecture (Phase 26) with interface-based DI. This phase evaluates whether any files exceed the complexity threshold and splits them if necessary. The scope is intentionally smaller — the Ingestion API has fewer responsibilities than the Analysis Worker or BFF.*

* [x] **Evaluate `internal/storage/postgres.go`.** At 155 lines (threshold: 250), below the threshold but split proactively for consistency with BFF API and Analysis Worker — the concerns (connection, documents, jobs) are genuinely distinct and files will grow:
  - `internal/storage/postgres.go` — Connection setup, pool initialization, `GetSourceByName`, `Ping`.
  - `internal/storage/postgres_documents.go` — Document CRUD: `LogDocument()`, `UpdateDocumentStatus()`, `DeleteOldDocuments()`.
  - `internal/storage/postgres_jobs.go` — Ingestion job lifecycle: `CreateIngestionJob()`, `UpdateJobStatus()`, `DeleteOldIngestionJobs()`.
* [x] **Evaluate `internal/storage/minio.go`.** At 85 lines (threshold: 200) — keep as-is.
* [x] **Evaluate `internal/core/service.go`.** At 156 lines (threshold: 300) — keep as-is.
* [x] **Split test files to mirror source structure.** `postgres_test.go` → shared `setupTestDB` helper. `postgres_documents_test.go` → document tests. `postgres_jobs_test.go` → source lookup + job lifecycle tests.
* [x] **Validate.** `make test-go`, `make test-go-pkg`, `make lint`, `make audit-go`. CI pipeline passes without modification.

## Phase 61: Structural Decomposition — E2E & Cross-Cutting Cleanup - [x] DONE
*Final cleanup phase. Addresses the E2E smoke test script, verifies all validation gates, and updates Arc42 documentation to reflect the new file structure.*

* [x] **Refactor `scripts/e2e_smoke_test.sh`.** Extract shared helper functions (`log_ok`, `log_fail`, `log_step`, `log_info`, color codes, timestamp formatting) into `scripts/e2e_helpers.sh`. Source it from the main script. No behavioral change — identical test assertions, identical exit codes.
* [x] **Run full validation suite:**
  - `make test` (Go integration + Python unit tests — all green)
  - `make test-go-pkg` (shared `pkg/` module tests — all green)
  - `make test-e2e` (end-to-end smoke test — all green)
  - `make lint` (golangci-lint + ruff — clean)
  - `make audit` (govulncheck + pip-audit — clean)
  - `make codegen && git diff --exit-code` (no OpenAPI contract drift)
* [x] **Update Arc42 Documentation:**
  - `08_concepts.md` §8.3 (Clean Architecture): Update Python directory structure to reflect `internal/storage/` subpackage and `internal/quarantine.py` / `internal/silver.py` modules. Update Go directory structure if BFF storage was split.
  - `08_concepts.md` §8.1 (Testing Strategy): Add a sentence noting that test files are organized by concern, mirroring the source module structure, with shared fixtures in `conftest.py` (Python) and shared Testcontainer setup in `_test.go` files (Go).
  - No other Arc42 chapters affected — the architecture, patterns, and runtime behavior are unchanged.
* [x] **Update `README.md` if the project structure section references specific file paths** that changed during decomposition.

# AĒR ROADMAP — New Phases from Working Paper Series (WP-001 – WP-006)

*These phases implement the concrete architectural recommendations from the Scientific Methodology Working Paper Series. They are ordered by dependency — earlier phases provide the schema and infrastructure that later phases build on. All phases target Probe 0 (German institutional RSS) as the implementation context.*

*Design principle: These phases build **scientific infrastructure** — database schemas, API parameters, metadata models, workflow templates — without preempting open research questions. Tables are created empty where their content requires interdisciplinary validation. API endpoints that depend on validated data include explicit gates that return errors until validation has occurred. The distinction between "engineering scaffolding" and "scientific decision" is maintained throughout; see Phase 65 (`normalization` validation gate) for the canonical example.*

## Phase 62: Functional Probe Taxonomy & Source Classification Schema (WP-001) - [x] DONE
*WP-001 proposes a Functional Probe Taxonomy with four discourse functions and an Etic/Emic Dual Tagging System. This phase implements the database schema, the Pydantic models, and the initial classification for Probe 0. The classification itself is a manual scientific act — the schema enables it.*

* [x] **PostgreSQL Migration: `source_classifications` Table.** Create migration `000005_source_classifications.up.sql`:
  - `source_id INTEGER REFERENCES sources(id)`, `primary_function VARCHAR(30) NOT NULL`, `secondary_function VARCHAR(30)`, `function_weights JSONB`, `emic_designation TEXT NOT NULL`, `emic_context TEXT NOT NULL`, `emic_language VARCHAR(10)`, `classified_by VARCHAR(100) NOT NULL`, `classification_date DATE NOT NULL`, `review_status VARCHAR(30) DEFAULT 'pending'` (valid values: `provisional_engineering`, `pending`, `reviewed`, `contested`), `PRIMARY KEY (source_id, classification_date)`.
  - The table is additive — it does not modify existing `sources` entries. Multiple classification records per source enable temporal tracking of functional transitions.
* [x] **Seed Probe 0 Classification.** Create migration `000006_seed_probe0_classification.up.sql` that inserts etic/emic classifications for the existing RSS sources (tagesschau.de, bundesregierung.de). The primary/secondary function assignments are qualitatively justified by WP-001 §3 and §6 (Probe 0 classification). Numerical `function_weights` are **not** seeded — quantifying the relative strength of discourse functions requires the formalized classification process (WP-001 §4.4, Steps 1–2: area expert nomination and peer review), which has not yet occurred. The `review_status` is set to `provisional_engineering` to make this explicit:
  - tagesschau.de: `primary_function = 'epistemic_authority'`, `secondary_function = 'power_legitimation'`, `function_weights = NULL`, `emic_designation = 'Tagesschau'`, `emic_context = 'State-funded public broadcaster (ARD). Norm-setting through informational baseline. Editorial independence structurally influenced by inter-party proportional governance.'`, `classified_by = 'WP-001/Probe-0'`, `review_status = 'provisional_engineering'`.
  - bundesregierung.de: `primary_function = 'power_legitimation'`, `secondary_function = 'epistemic_authority'`, `function_weights = NULL`, `emic_designation = 'Bundesregierung'`, `emic_context = 'Official government communication channel. Structural power legitimation through agenda-setting and framing.'`, `classified_by = 'WP-001/Probe-0'`, `review_status = 'provisional_engineering'`.
  - The SQL migration file must include an inline comment: `-- function_weights intentionally NULL. Quantification requires WP-001 §4.4 classification process. See docs/scientific_operations_guide.md.`
* [x] **Pydantic Models: `ProbeEticTag`, `ProbeEmicTag`, `DiscourseContext`.** Create `services/analysis-worker/internal/models/discourse.py` with the three models defined in WP-001 §4.2 and §7.2. `DiscourseContext` (containing `primary_function`, `secondary_function`, `emic_designation`) is the propagation model for `SilverMeta`.
* [x] **Extend `RSSAdapter` to Propagate Discourse Context.** The adapter reads the `source_classifications` table (via a new `get_source_classification(source_id)` query in the storage layer) and populates `DiscourseContext` in `RssMeta` during harmonization. If no classification exists, the field is `None` — the pipeline does not fail.
* [x] **Extend Gold Metrics with `discourse_function` Column.** Add `discourse_function String DEFAULT ''` to `aer_gold.metrics` and `aer_gold.entities` via ClickHouse migration. The extractor pipeline writes the `primary_function` from `DiscourseContext` into this column. This enables aggregation by discourse function.
* [x] **Probe Registration Record Template.** Create `docs/templates/probe_registration_template.yaml` containing the full registration template from WP-001 Appendix B. This is the manual form future researchers fill out when proposing new probes.
* [x] **Update Arc42 Documentation.** Chapter 5 (§5.1.2: SilverMeta now includes `DiscourseContext`). Chapter 12 (Glossary: `Discourse Function`, `Etic Tag`, `Emic Tag`, `Probe Classification`). Chapter 13 (§13.8: Probe 0 is formally classified as Functions 1–2).
* [x] **Validate.** `make test`, `make lint`, `make audit`, `make test-e2e`.


## Phase 63: Metric Validity Infrastructure (WP-002) - [x] DONE
*WP-002 proposes a five-step validation protocol and recommends Option C (hybrid tier architecture: Tier 1 as immutable baseline, Tier 2/3 as validated enrichments). This phase builds the infrastructure to store and expose validation metadata — not the validation studies themselves (those require interdisciplinary collaborators).*

* [x] **ClickHouse Table: `aer_gold.metric_validity`.** Create via ClickHouse init migration:
  - Schema: `metric_name String`, `context_key String` (e.g., `de:rss:epistemic_authority`), `validation_date DateTime`, `alpha_score Float32`, `correlation Float32`, `n_annotated UInt32`, `error_taxonomy String` (JSON blob), `valid_until DateTime`.
  - `ENGINE = ReplacingMergeTree(validation_date)`, `ORDER BY (metric_name, context_key)`.
  - This table is initially empty — it will be populated when validation studies are conducted.
* [x] **BFF API: Expose Validation Status.** Extend `GET /api/v1/metrics/available` response to include a `validation_status` field per metric (`unvalidated`, `validated`, `expired`). The BFF queries `aer_gold.metric_validity` and joins with available metrics. Unvalidated metrics (no entry in the validity table) return `unvalidated`. Update OpenAPI spec, regenerate stubs, implement handler + storage.
* [x] **Document Extractor Limitation Metadata.** Create `docs/methodology/extractor_limitations.md` documenting the known limitations of all Phase 42 extractors as identified in WP-002 §3: SentiWS negation blindness, compound word failure, spaCy NER entity linking absence, language detection short-text degradation. This file is the human-readable complement to the `metric_validity` table.
* [x] **ADR-016: Hybrid Tier Architecture (Option C).** Document the decision in `docs/arc42/09_architecture_decisions.md`: Tier 1 metrics are the immutable baseline, always displayed. Tier 2/3 metrics are validated enrichments, available via Progressive Disclosure. The dashboard never hides the Tier 1 score behind a Tier 2/3 score.
* [x] **Update Arc42 Documentation.** Chapter 8 (§8.12: document the hybrid tier principle). Chapter 13 (§13.3: mark validation table as implemented). Chapter 12 (Glossary: `Metric Validity`, `Validation Protocol`, `Context Key`).
* [x] **Validate.** `make test`, `make lint`, `make audit`, `make codegen && git diff --exit-code`.

## Phase 64: Bias Documentation & SilverMeta Extension (WP-003) - [x] DONE
*WP-003 proposes standardized `BiasContext` fields in `SilverMeta` and a "document, don't filter" approach to non-human actors. This phase implements the metadata fields — authenticity extractors and coordination detectors are deferred to later phases as they require the `CorpusExtractor` path (R-9).*

* [x] **Pydantic Model: `BiasContext`.** Create the model in `services/analysis-worker/internal/models/bias.py`:
  - Fields: `platform_type: str`, `access_method: str`, `visibility_mechanism: str`, `moderation_context: str`, `engagement_data_available: bool`, `account_metadata_available: bool`.
* [x] **Extend `RssMeta` with `BiasContext`.** Add `bias_context: BiasContext` to `RssMeta`. The `RSSAdapter` populates it with static values for RSS sources: `platform_type='rss'`, `access_method='public_rss'`, `visibility_mechanism='chronological'`, `moderation_context='editorial'`, `engagement_data_available=False`, `account_metadata_available=False`.
* [x] **Extend Source Adapter Protocol.** Update `adapters/base.py` to include `BiasContext` as an optional field in the `SourceAdapter` protocol documentation. Future adapters (social media, forums) will populate it with platform-specific values.
* [x] **Document Probe 0 Bias Profile.** Create `docs/methodology/probe0_bias_profile.md` following the WP-003 platform-bias framework: document the known biases of tagesschau.de and bundesregierung.de (editorial bias, state-funding bias, absence of engagement data, absence of algorithmic amplification). This is a manual scientific document, not code.
* [x] **Update Arc42 Documentation.** Chapter 5 (§5.1.2: `SilverMeta` now includes `BiasContext`). Chapter 11 (add R-12: `Authenticity extractors not yet implemented — per WP-003 §8.2`). Chapter 12 (Glossary: `BiasContext`, `Visibility Mechanism`, `Authenticity Extractor`).
* [x] **Validate.** `make test`, `make lint`, `make audit`.

## Phase 65: Cross-Cultural Comparability Infrastructure (WP-004) - [x] DONE
*WP-004 proposes a Metric Equivalence Registry, baseline computation, and z-score normalization. This phase implements the Gold layer extensions and BFF API normalization parameter. Actual equivalence entries require interdisciplinary validation and are deferred.*

* [x] **ClickHouse Table: `aer_gold.metric_baselines`.** Create via ClickHouse init migration:
  - Schema: `metric_name String`, `source String`, `language String`, `baseline_value Float64`, `baseline_std Float64`, `window_start DateTime`, `window_end DateTime`, `n_documents UInt32`, `compute_date DateTime`.
  - `ENGINE = ReplacingMergeTree(compute_date)`, `ORDER BY (metric_name, source, language)`.
* [x] **ClickHouse Table: `aer_gold.metric_equivalence`.** Create via ClickHouse init migration:
  - Schema: `etic_construct String` (e.g., `evaluative_polarity`), `metric_name String` (e.g., `sentiment_score_sentiws`), `language String`, `source_type String`, `equivalence_level String` (`temporal`, `deviation`, `absolute`), `validated_by String`, `validation_date DateTime`, `confidence Float32`.
  - `ENGINE = ReplacingMergeTree(validation_date)`, `ORDER BY (etic_construct, metric_name, language)`.
  - Initially empty — populated when validation studies establish equivalence.
* [x] **Baseline Computation Script.** Create `scripts/compute_baselines.py` — a standalone Python script (not part of the real-time pipeline) that queries `aer_gold.metrics` for a specified time window, computes mean and standard deviation per `(metric_name, source, language)`, and inserts results into `aer_gold.metric_baselines`. Intended to be run periodically (weekly/monthly) by a researcher or as a cron job.
* [x] **BFF API: `normalization` Query Parameter.** Extend `GET /api/v1/metrics` to accept `?normalization=raw|zscore`:
  - `raw` (default): current behavior.
  - `zscore`: join with `metric_baselines`, return `(value - baseline_value) / baseline_std` as the value. **Validation gate:** the endpoint returns HTTP 400 with a descriptive error if (a) no baseline exists in `metric_baselines` for the requested `(metric_name, source)` pair, or (b) no entry in `metric_equivalence` confirms at least `deviation`-level equivalence for this metric-context combination. This prevents normalized comparisons from being served before interdisciplinary validation has established their scientific legitimacy (WP-004 §7.3, Q7: whether baseline normalization constitutes cultural erasure is an open research question that must be answered per-context, not assumed).
  - Update OpenAPI spec, regenerate stubs, implement in handler + ClickHouse query layer.
* [x] **BFF API: Equivalence Metadata in `/metrics/available`.** Extend the response to include `etic_construct` and `equivalence_level` per metric if an entry exists in the equivalence registry. Unregistered metrics return `null`.
* [x] **Update Arc42 Documentation.** Chapter 5 (§5.1.3: document `normalization` parameter). Chapter 13 (§13.3: cross-reference WP-004 Gold layer extensions). Chapter 12 (Glossary: `Metric Equivalence`, `Baseline`, `Z-Score Normalization`, `Etic Construct`).
* [x] **Validate.** `make test`, `make lint`, `make audit`, `make codegen && git diff --exit-code`, `make test-e2e`.


## Phase 66: Multi-Resolution Temporal Framework (WP-005) - [x] DONE
*WP-005 defines five temporal scales and proposes multi-resolution ClickHouse aggregation, BFF API extensions, and a tiered retention strategy. This phase implements the query-time aggregation (no materialized views yet — those are a performance optimization deferred until query latency requires them).*

* [x] **BFF API: `resolution` Query Parameter.** Extend `GET /api/v1/metrics` to accept `?resolution=5min|hourly|daily|weekly|monthly`:
  - Map to ClickHouse aggregation functions: `toStartOfFiveMinute()`, `toStartOfHour()`, `toStartOfDay()`, `toStartOfWeek()`, `toStartOfMonth()`.
  - Adjust the `rowLimit` OOM guard per resolution: wider windows produce fewer rows, so relax the limit proportionally (e.g., `hourly` → `rowLimit * 12`, `daily` → `rowLimit * 288`).
  - Default remains `5min` for backward compatibility.
  - Update OpenAPI spec, regenerate stubs, implement in handler + ClickHouse query layer.
* [x] **BFF API: Minimum Meaningful Window Metadata.** Extend `GET /api/v1/metrics/available` response to include `min_meaningful_resolution` per metric-source pair. Initially hardcoded based on Probe 0 publication rates (tagesschau.de ≈ 50 articles/day → `hourly`; bundesregierung.de ≈ 5 articles/day → `daily`). Stored as a config map in the BFF, not in ClickHouse.
* [x] **ClickHouse Materialized Views (Deferred Preparation).** Add the SQL definitions for `aer_gold.metrics_hourly`, `aer_gold.metrics_daily`, `aer_gold.metrics_monthly` as commented-out migration scripts in `infra/clickhouse/`. Document in the migration file that these should be activated when query latency exceeds acceptable thresholds (WP-005 §5.4). Do not activate yet — query-time aggregation is sufficient at current scale.
* [x] **Tiered Retention Strategy Documentation.** Document the proposed retention tiers (0–30d: full, 30–365d: hourly, 1–5y: daily, 5y+: monthly) in `docs/arc42/08_concepts.md` §8.8 as the target architecture. Mark as "planned — not yet active" to distinguish from current flat 365-day TTL.
* [x] **Update Arc42 Documentation.** Chapter 5 (§5.1.3: document `resolution` parameter). Chapter 8 (§8.13: multi-resolution downsampling strategy). Chapter 12 (Glossary: `Temporal Scale`, `Minimum Meaningful Window`, `Tiered Retention`).
* [x] **Validate.** `make test`, `make lint`, `make audit`, `make codegen && git diff --exit-code`, `make test-e2e`.


## Phase 67: Reflexive Architecture — Methodological Transparency (WP-006) - [x] DONE
*WP-006 proposes five design principles for reflexive architecture. This phase implements the two that have immediate technical consequences: Methodological Transparency and Reflexive Documentation. The remaining three (Non-Prescriptive Visualization, Governed Openness, Interpretive Humility) are dashboard/governance concerns deferred to the frontend phase.*

* [x] **BFF API: Metric Provenance Endpoint.** Create `GET /api/v1/metrics/{metricName}/provenance` that returns:
  - `tier_classification` (1, 2, or 3), `algorithm_description`, `known_limitations` (list of strings), `validation_status` (from `metric_validity` table), `extractor_version_hash`, `cultural_context_notes` (from `metric_equivalence` table if available).
  - Data is assembled from a static config file (`configs/metric_provenance.yaml` in the BFF service) combined with dynamic lookups in `metric_validity` and `metric_equivalence`.
  - Update OpenAPI spec, regenerate stubs, implement handler.
* [x] **Static Provenance Config: `metric_provenance.yaml`.** Create the config file documenting each currently implemented metric:
  - `word_count`: Tier 1, deterministic, no known limitations.
  - `sentiment_score`: Tier 1 (SentiWS), known limitations: negation blindness, compound word failure (WP-002 §3).
  - `language`: Tier 1 (langdetect), known limitation: short-text degradation.
  - `temporal_distribution`: Tier 1, deterministic.
  - NER entities: Tier 1 (spaCy), known limitation: no entity linking, Western bias in entity ontology.
* [x] **Link Source Documentation to `sources` Table.** Add `documentation_url VARCHAR(255)` column to the PostgreSQL `sources` table via migration `000007_sources_documentation_url.up.sql`. Populate for existing RSS sources with links to `docs/methodology/probe0_bias_profile.md`. The BFF API exposes this field via a new `GET /api/v1/sources` endpoint (or extends the existing source metadata response).
* [x] **ADR-017: Reflexive Architecture Principles.** Document the five principles from WP-006 §6 in `docs/arc42/09_architecture_decisions.md` as an architectural commitment. Mark principles 1 (Methodological Transparency) and 3 (Reflexive Documentation) as "implemented" and principles 2, 4, 5 as "deferred to dashboard phase."
* [x] **Non-Prescriptive Visualization Guidelines.** Create `docs/design/visualization_guidelines.md` documenting the WP-006 §6.2 requirements: viridis color scale, no red/green encoding, no normative labels, uncertainty alongside point estimates, multiple visualization modes. This file guides future frontend development.
* [x] **Update Arc42 Documentation.** Chapter 5 (§5.1.3: document provenance endpoint). Chapter 8 (add §8.14: Reflexive Architecture). Chapter 12 (Glossary: `Reflexive Documentation`, `Methodological Transparency`, `Non-Prescriptive Visualization`).
* [x] **Validate.** `make test`, `make lint`, `make audit`, `make codegen && git diff --exit-code`.

## Phase 68: Seed Files & Configuration Templates (Cross-WP) - [x] DONE
*Phases 62–64 are implemented; Phases 65–67 will complete the schema and API work and are implemented as well. Before documenting workflows (Phase 71), the system needs the static seed files and configuration templates that those workflows reference. Phase 62 already created `probe_registration_template.yaml` and Phase 64 already created `probe0_bias_profile.md` — this phase creates only the remaining templates.*

* [x] **Cultural Calendar Annotation Seed.** Create `configs/cultural_calendars/de.yaml` as a seed file for the cultural calendar metadata proposed in WP-005 §4.3. Contains German public holidays, federal election dates, and significant recurring media events (e.g., Berlinale, Buchmesse). Format: `{date, name, type, expected_discourse_effect}`. This is a static lookup — no service required for the POC.
* [x] **Validation Study Record Template.** Create `docs/templates/validation_study_template.yaml` with fields from WP-002 §6.2: `metric_name`, `context_key`, `annotation_scheme`, `annotator_count`, `krippendorff_alpha`, `correlation`, `error_taxonomy`, `transfer_boundary`, `longitudinal_window`. This is the structured record a researcher fills out after completing a validation study — the data that gets inserted into `aer_gold.metric_validity`.
* [x] **Observer Effect Assessment Template.** Create `docs/templates/observer_effect_assessment.yaml` based on WP-006 §8.4 Q7: `cultural_region`, `beneficial_effects`, `harmful_effects`, `vulnerable_populations`, `recommended_safeguards`, `assessed_by`, `assessment_date`. This is the structured record completed during Step 4 (ethical review) of the probe classification process.
* [x] **Validate.** File format review — all YAML templates parse cleanly. No code changes.


## Phase 69: Arc42 Consolidation & Cross-Reference Audit - [x] DONE
*Phases 62–64 each included inline Arc42 updates (Chapter 5, 8, 11, 12, 13) for the features they introduced. Phases 65–67 will do the same. This phase does not repeat those updates — it verifies their consistency, fills gaps that only become visible when viewing the documentation as a whole, and ensures all cross-references are intact after eleven phases of changes.*

* [x] **Chapter 3 (System Scope and Context).** Technical Context / Business Context diagrams extended with the Phase 63–67 BFF endpoints (`/provenance`, `/sources`, `/languages`, extended `/metrics/available`). External Interfaces table now documents `normalization` and `resolution` query parameters. Added "Interdisciplinary Researcher" as a stakeholder class in the Business Context.
* [x] **Chapter 5 (Building Block View).** §5.1.2 already documented `DiscourseContext` (Phase 62) and `BiasContext` (Phase 64). §5.1.3 already covered `normalization`, `resolution`, and the provenance endpoint. §5.1.4 was missing `aer_gold.metric_validity` (Phase 63) — now added alongside `metric_baselines` and `metric_equivalence`. `source_classifications` and `discourse_function` column documentation verified.
* [x] **Chapter 8 (Cross-cutting Concepts).** Verified existing coverage: §8.12 Hybrid Tier Architecture (ADR-016), §8.13 Multi-Resolution Temporal Framework, §8.8.1 Tiered Retention "planned — not yet active", §8.14 Reflexive Architecture (ADR-017). (Historical ROADMAP-text references to §8.6/§8.10/§8.12 — actual section numbers §8.13/§8.12/§8.14 — were corrected in Phase 80.)
* [x] **Chapter 9 (Architecture Decisions).** ADR-015, ADR-016 (Phase 63, Hybrid Tier), ADR-017 (Phase 67, Reflexive Architecture) all present and correctly numbered. ADR-015 cross-references still accurate — `SilverMeta` is explicitly described as unstable, so Phase 62/64 additions (DiscourseContext, BiasContext) require no ADR update.
* [x] **Chapter 11 (Risks and Technical Debts).** R-12 (Authenticity Extractors, Phase 64) verified present. Added **R-13: Scientific Infrastructure Tables Are Empty** — warns that `metric_validity`, `metric_equivalence`, and `source_classifications.function_weights` are empty or carry provisional defaults, making the BFF surface look "validation-ready" when it is not. Added R-13 to the risk matrix overview.
* [x] **Chapter 12 (Glossary).** Added missing terms `Cultural Calendar` and `Validation Study`. Verified all other Phase 62–67 terms (`Baseline`, `BiasContext`, `Discourse Function`, `Emic Tag`, `Etic Construct`, `Etic Tag`, `Metric Equivalence`, `Metric Validity`, `Methodological Transparency`, `Minimum Meaningful Window`, `Non-Prescriptive Visualization`, `Probe Classification`, `Reflexive Documentation`, `Temporal Scale`, `Tiered Retention`, `Validation Protocol`, `Visibility Mechanism`, `Z-Score Normalization`) are present.
* [x] **Chapter 13 (Scientific Foundations).** §13.3 already cross-references `metric_validity` (Phase 63) and `metric_baselines`/`metric_equivalence` (Phase 65). Added §13.5.2 "Manual Scientific Workflows" subsection listing the six researcher-facing workflows and pointing forward to the Scientific Operations Guide (Phase 71). Renumbered the Probe 0 section from `## 14` to `## 13.10` (it was stranded outside the chapter numbering) and fixed the corresponding `§13.8 → §13.10` back-references in Chapter 5, Chapter 12, README, WP-001 EN and WP-001 DE. §13.9 (Data Protection) verified still accurate. Fixed 12 broken Working Paper links in §13.6 and §13.7 that pointed at the pre-bilingual `../methodology/WP-XXX-...md` paths — now resolve to `../methodology/en/WP-XXX-en-...md`.
* [x] **`mkdocs.yml` Navigation.** Added `methodology/probe0_bias_profile.md`, `methodology/extractor_limitations.md`, and `design/visualization_guidelines.md` to the nav. Left a commented-out placeholder for `docs/scientific_operations_guide.md` (Phase 71).
* [x] **Cross-Reference Integrity Audit.** Grep sweep for `§13.x`, `methodology/WP-`, and `ADR-0` references across `docs/`, `ROADMAP.md`, and `README.md`. All broken or stale references identified by this audit were fixed in the bullets above.
* [x] **Validate.** `mkdocs build --strict` passes (0 warnings, exit 0) against `squidfunk/mkdocs-material:9.7.6`.


## Phase 70: Probe Dossier Pattern, Documentation Consolidation & Operations Playbook Extension - [x] DONE

*Die bisherige Dokumentationsstruktur unter `docs/methodology/` ist ad-hoc gewachsen: `probe0_bias_profile.md` dokumentiert nur Bias (WP-003), `extractor_limitations.md` dupliziert Informationen, die bereits in `metric_provenance.yaml` und WP-002 §3 leben. Diese Phase konsolidiert die Dokumentation entlang zweier sauberer Achsen: Per-Probe-Dokumentation (Probe Dossier Pattern) und Per-Metrik-Dokumentation (SSoT: `metric_provenance.yaml`). Gleichzeitig wird das Operations Playbook um die wissenschaftlichen Touchpoints erweitert — jeder Abschnitt enthält sowohl ein generisches Template als auch ein konkretes Probe-0-Beispiel.*

### Probe Dossier Pattern

* [x] **Verzeichnisstruktur anlegen.** `docs/probes/probe-0-de-institutional-rss/` mit fünf Dateien:

* [x] **`README.md` — Probe Overview & WP Coverage Matrix.** Zweck der Probe, beteiligte Quellen (tagesschau.de, bundesregierung.de), Engineering-Kalibrierungsstatus (nicht wissenschaftlich motiviert — vgl. §13.10), Exit-Kriterien. Enthält eine WP Coverage Matrix: tabellarische Zuordnung jedes der sechs Working Papers zur Datei oder Tabelle, in der die probe-spezifische Dokumentation lebt. WP-001 → `classification.md`. WP-003 → `bias_assessment.md`. WP-005 → `temporal_profile.md`. WP-006 → `observer_effect.md`. WP-002 → Verweis auf `aer_gold.metric_validity` (systemweit, nicht probe-spezifisch; Validierungsstudien werden pro `(metric_name, context_key)`-Paar durchgeführt). WP-004 → Verweis auf `aer_gold.metric_baselines` und `aer_gold.metric_equivalence` (cross-probe per Definition).

* [x] **`classification.md` — Probe Classification (WP-001).** Dokumentiert die etic/emic Klassifikation beider Quellen: `primary_function`, `secondary_function`, `emic_designation`, `emic_context` — exakt die Werte aus der `source_classifications`-Tabelle (Migration 000006). Dokumentiert den `review_status = 'provisional_engineering'` und erklärt, warum `function_weights = NULL` (WP-001 §4.4 Schritte 1–2 ausstehend). Cross-Referenz: Scientific Operations Guide Workflow 1.

* [x] **`bias_assessment.md` — Platform Bias (WP-003).** Inhaltlich identisch mit dem bestehenden `docs/methodology/probe0_bias_profile.md` — verschoben, nicht neu geschrieben. Enthält die `BiasContext`-Werte und die fünf dokumentierten strukturellen Biases. Cross-Referenz: Scientific Operations Guide Workflow 5.

* [x] **`temporal_profile.md` — Temporal Characteristics (WP-005).** Publikationsmuster pro Quelle: tagesschau.de ≈ 50 Artikel/Tag (→ `min_meaningful_resolution = hourly`), bundesregierung.de ≈ 5 Artikel/Tag (→ `min_meaningful_resolution = daily`). Verweis auf den Cultural Calendar (`configs/cultural_calendars/de.yaml`). Hinweis auf die geplante Tiered Retention Strategy (§8.8, "planned — not yet active"). Cross-Referenz: Scientific Operations Guide Workflow 6.

* [x] **`observer_effect.md` — Observer Effect Assessment (WP-006).** Initiale Bewertung basierend auf dem `observer_effect_assessment.yaml`-Template (Phase 68). Für Probe 0 ist das Risiko gering: öffentliche RSS-Feeds, kein Login, kein Engagement-Signal, das Rückkopplungen erzeugen könnte. Dokumentiert: `cultural_region = DE`, `beneficial_effects`, `harmful_effects`, `vulnerable_populations`, `recommended_safeguards`. Cross-Referenz: Scientific Operations Guide Workflow 1, Step 4.

### Documentation Consolidation

* [x] **`docs/methodology/extractor_limitations.md` löschen.** Das Dokument ist redundant: `metric_provenance.yaml` enthält die strukturierten `known_limitations` pro Metrik (SSoT für die API), WP-002 §3 enthält die wissenschaftliche Analyse. Die Prosa in `extractor_limitations.md` dupliziert beides, ohne eigenen Informationswert zu liefern. Cross-cutting Concerns (Graceful Degradation, Immutable Input, Validation Required) sind bereits in Arc42 §8.3 dokumentiert.

* [x] **`metric_provenance.yaml` anreichern.** `wp_reference`-Feld pro Metrik hinzufügen (YAML-only, nicht über die API exponiert, da der Go-YAML-Parser unbekannte Felder ignoriert). `known_limitations`-Einträge von knappen Labels zu vollständigen Sätzen erweitern, die das Problem und seine Konsequenz erklären. Beispiel: statt `"Negation blindness"` → `"Negation blindness — SentiWS does not propagate negation scope; 'nicht gut' scores as positive (WP-002 §3)."` Die bestehenden Einträge sind bereits nahe an diesem Format; die fehlenden werden angeglichen.

* [x] **`docs/methodology/probe0_bias_profile.md` entfernen.** Inhalt lebt jetzt in `docs/probes/probe-0-de-institutional-rss/bias_assessment.md`. Redirect-Kommentar in der gelöschten Datei ist nicht nötig — alle Verweise werden in dieser Phase aktualisiert.

* [x] **PostgreSQL Migration: `documentation_url` aktualisieren.** Migration `000008_update_documentation_url.up.sql`: `UPDATE sources SET documentation_url = 'docs/probes/probe-0-de-institutional-rss/' WHERE name IN ('bundesregierung', 'tagesschau');` — zeigt auf das Dossier-Verzeichnis statt auf eine einzelne Datei.

* [x] **`mkdocs.yml` Navigation anpassen.** Unter "Methodology (EN)": `probe0_bias_profile.md` und `extractor_limitations.md` entfernen. Neuen Abschnitt "Probes" hinzufügen mit `docs/probes/probe-0-de-institutional-rss/README.md` als Einstieg. Die Unterseiten (`classification.md`, `bias_assessment.md`, `temporal_profile.md`, `observer_effect.md`) als Kinder darunter.

### Operations Playbook Extension

*Jeder neue Abschnitt enthält (a) ein generisches Template mit Platzhaltern für beliebige Probes und (b) ein konkretes Probe-0-Beispiel mit echten Werten. Die wissenschaftliche Begründung ("warum") lebt im Scientific Operations Guide (Phase 71) — das Playbook beschreibt nur "was tippen".*

* [x] **Playbook Section: `source_classifications`-Tabelle.** PostgreSQL-Abschnitt: Inspect-Query (`SELECT * FROM source_classifications ORDER BY classification_date DESC`), generisches Template-INSERT mit allen Pflichtfeldern und Kommentaren, konkretes Probe-0-Beispiel (tagesschau.de-Werte aus Migration 000006). Update-Query für `review_status`-Lifecycle. Cross-Referenz: Scientific Operations Guide Workflow 1.

* [x] **Playbook Section: `metric_validity`-Tabelle.** ClickHouse-Abschnitt: Inspect-Query, generisches Template-INSERT mit allen Pflichtfeldern (aus `validation_study_template.yaml`), konkretes Probe-0-Beispiel (hypothetische Validation Study für `sentiment_score` im Kontext `de:rss:epistemic_authority`). Cross-Referenz: Scientific Operations Guide Workflow 2.

* [x] **Playbook Section: `metric_baselines` und `metric_equivalence`.** ClickHouse-Abschnitt: Ausführung von `scripts/compute_baselines.py` (Flags, Umgebungsvariablen, erwarteter Output), generisches Template-INSERT für Equivalence-Einträge, konkretes Probe-0-Beispiel (Baseline für `word_count` auf tagesschau.de). Cross-Referenz: Scientific Operations Guide Workflows 3–4.

* [x] **Playbook Section: `metric_provenance.yaml`.** BFF-API-Abschnitt: Dateipfad (`services/bff-api/configs/metric_provenance.yaml`), Pflichtfelder pro Metrik-Eintrag, Trigger für Updates (neue Extractor-Registrierung), `wp_reference` als YAML-interner Kommentar für Entwickler. Konkretes Probe-0-Beispiel: die bestehenden fünf Metriken.

* [x] **Playbook Section: Cultural Calendar Files.** Konfigurations-Abschnitt: `configs/cultural_calendars/`-Verzeichnis, YAML-Format (`{date, name, type, expected_discourse_effect}`), Anleitung zum Hinzufügen einer neuen Region. Konkretes Probe-0-Beispiel: Verweis auf `de.yaml` und ausgewählte Einträge.

* [x] **Playbook Section: Probe Dossier.** Neuer Abschnitt: Verzeichnisstruktur (`docs/probes/<probe-id>/`), welche Dateien Pflicht sind, wie man ein neues Dossier anlegt (copy template, fill in), Zusammenhang mit der `documentation_url`-Spalte in `sources`. Konkretes Probe-0-Beispiel: die soeben erstellten fünf Dateien.

### README & Arc42

* [x] **`README.md`-Update.** Projektstruktur-Abschnitt aktualisieren: `docs/probes/` hinzufügen, `docs/methodology/probe0_bias_profile.md` und `docs/methodology/extractor_limitations.md` entfernen. "Developing a Crawler"-Abschnitt: Hinweis, dass eine neue Datenquelle das Probe-Dossier und den Probe Classification Workflow erfordert (Verweis auf Scientific Operations Guide).

* [x] **Arc42-Updates.** Chapter 5 (§5.1.3): `documentation_url` zeigt auf Dossier-Verzeichnis statt Einzeldatei. Chapter 8: §8.15 Probe Dossier Pattern als Cross-cutting Concept — Verzeichnisstruktur, WP Coverage Matrix, Beziehung zu `source_classifications` und `documentation_url`. Chapter 11: R-13 aktualisieren — Probe 0 hat jetzt ein vollständiges Dossier; Risiko bleibt bis zur wissenschaftlichen Validierung bestehen, aber die Dokumentationslücke ist geschlossen. Chapter 12 (Glossary): `Probe Dossier`.

* [x] **Cross-Reference Audit.** Grep-Sweep für alle Verweise auf `probe0_bias_profile.md` und `extractor_limitations.md` in `docs/`, `ROADMAP.md`, `README.md`, Migrations, YAML-Configs. Alle Verweise aktualisieren oder entfernen.

* [x] **Validate.** `mkdocs build --strict` (0 Warnings). Alle SQL-Beispiele im Playbook gegen laufenden Dev-Stack ausführen. `make test`, `make lint`, `make audit`, `make codegen && git diff --exit-code`.


## Phase 71: Scientific Operations Guide — The Bridge Document - [x] DONE

*AĒR hat zwei operationale Zielgruppen: Entwickler (Operations Playbook — "was tippen") und Wissenschaftler (Working Papers — "warum diese Methodik"). Keines der Dokumente erklärt den Handoff zwischen ihnen. Diese Phase erstellt `docs/scientific_operations_guide.md` — das Dokument, das jeden Punkt kartiert, an dem wissenschaftliches Urteil in die Pipeline eintritt.*

*Strukturprinzip: Organisiert nach Workflow, nicht nach Working Paper. Jeder Workflow beschreibt eine wissenschaftliche Aktivität end-to-end: Trigger, Rolle (wer), Prozess (WP-Referenz), technische Schritte (Playbook-Referenz), Templates (Phase-68-Referenz), Outputs (Tabelle/Datei/Config), und einen konkreten Probe-0-Walkthrough mit echten Werten.*

### Workflows

* [x] **Workflow 1: Classifying a New Probe.** WP-001 §4.4 Fünf-Schritt-Prozess: (1) Area Expert Nomination → `probe_registration_template.yaml` ausfüllen, (2) Peer Review → Disagreements dokumentieren, (3) Technische Machbarkeit → Entwickler bewertet Crawler-Viabilität, (4) Ethical Review → `observer_effect_assessment.yaml` ausfüllen → Ergebnis in Probe Dossier `observer_effect.md`, (5) Registration → INSERT in `source_classifications` (Playbook §PostgreSQL) + Probe Dossier anlegen (`docs/probes/<probe-id>/`). `review_status`-Lifecycle: `provisional_engineering` → `pending` → `reviewed` / `contested`. Erklärt, warum `function_weights = NULL` bis Schritte 1–2 quantifiziert sind.
  - **Probe 0 Walkthrough:** Chronologische Rekonstruktion, wie Probe 0 klassifiziert wurde — von der Quellenwahl (§13.10: Engineering-Kalibrierung) über Migration 000006 (`provisional_engineering`) bis zum fertigen Dossier. Zeigt den aktuellen Status und die offenen Schritte (function_weights, Peer Review).

* [x] **Workflow 2: Validating a Metric.** WP-002 §6.2 Fünf-Schritt-Protokoll: (1) Annotationsstudie (≥ 3 Annotator:innen, Krippendorff's Alpha ≥ 0.667), `validation_study_template.yaml` ausfüllen, (2) Baseline-Vergleich → Metrik auf annotiertem Sample ausführen, (3) Error Taxonomy → Klassifikationsschema erstellen, (4) Cross-Context Transfer → was konstituiert einen anderen Kontext, (5) Longitudinale Stabilität → mindestens 6-Monats-Fenster. Output: INSERT in `aer_gold.metric_validity` (Playbook §ClickHouse). Dokumentiert was bei `valid_until`-Ablauf passiert — Metrik revertiert zu `unvalidated` in der BFF API.
  - **Probe 0 Walkthrough:** Hypothetische (aber vollständige) Validation Study für `sentiment_score` im Kontext `de:rss:epistemic_authority`. Alle fünf Schritte mit Beispielwerten durchgespielt bis zum INSERT-Statement. Markiert als hypothetisch — keine Studie wurde tatsächlich durchgeführt.

* [x] **Workflow 3: Establishing Metric Equivalence.** WP-004 §5.2: Wann zwei Metriken aus verschiedenen Instrumenten cross-kulturell vergleichbar sind. Drei Equivalence Levels (`temporal`, `deviation`, `absolute`), erforderliche Evidenz pro Level, INSERT in `aer_gold.metric_equivalence` (Playbook §ClickHouse). Erklärt den Validation Gate auf `?normalization=zscore` — warum die BFF 400 zurückgibt ohne Equivalence-Eintrag.
  - **Probe 0 Walkthrough:** Erklärt, warum dieses Workflow für Probe 0 (monolingual, monokulturell) nicht anwendbar ist — cross-kulturelle Vergleichbarkeit setzt mindestens zwei Probes aus verschiedenen kulturellen Kontexten voraus. Dokumentiert den erwarteten HTTP-400-Response bei `?normalization=zscore` als bewusstes Design.

* [x] **Workflow 4: Computing and Updating Baselines.** WP-004 §6.1: Trigger (signifikantes Corpus-Wachstum, neue Quelle hinzugefügt), Ausführung über `scripts/compute_baselines.py` (Playbook §ClickHouse), Interpretation der Ergebnisse, wie Baseline-Staleness die z-Score-Zuverlässigkeit beeinflusst.
  - **Probe 0 Walkthrough:** Konkreter Durchlauf von `compute_baselines.py` gegen die existierenden Probe-0-Daten. Erwarteter Output für `word_count` und `sentiment_score` auf tagesschau.de. Zeigt den resultierenden INSERT in `metric_baselines`.

* [x] **Workflow 5: Assessing Bias for a Data Source.** WP-003 §8.1: `BiasContext`-Felder für einen neuen Source Adapter ausfüllen. Dokumentiert welche Felder objektive Plattform-Eigenschaften sind (Entwickler) vs. welche Domain-Expertise erfordern (Forscher:in). Output: `BiasContext`-Werte im Adapter-Code + Prosa im Probe Dossier (`bias_assessment.md`).
  - **Probe 0 Walkthrough:** Zeigt die sechs `BiasContext`-Werte für RSS-Quellen und verlinkt auf `docs/probes/probe-0-de-institutional-rss/bias_assessment.md` als fertiges Beispiel. Erklärt die Entscheidung, dass für RSS alle Felder vom Entwickler gesetzt werden können (kein Domain-Expertise-Split nötig, weil RSS keine algorithmische Amplifikation hat).

* [x] **Workflow 6: Updating the Cultural Calendar.** WP-005 §4.3: Trigger (neue Probe-Region), Inhalt (Feiertage, Wahlen, religiöse Feste, Medienereignisse), Format (`configs/cultural_calendars/<region>.yaml`), Konsumation (aktuell statisches Lookup).
  - **Probe 0 Walkthrough:** Zeigt ausgewählte Einträge aus `configs/cultural_calendars/de.yaml` und erklärt, wie ein neuer Eintrag (z.B. Bundestagswahl 2025) hinzugefügt wird.

### Probe 0 End-to-End Walkthrough

* [x] **Zusammenhängender Walkthrough.** Chronologische Erzählung, die alle sechs Workflows in der Reihenfolge durchgeht, in der sie für Probe 0 anfallen oder anfallen würden. Beginnt mit Workflow 1 (Klassifikation — bereits geschehen als `provisional_engineering`), führt über Workflow 5 (Bias Assessment — bereits dokumentiert), Workflow 6 (Cultural Calendar — `de.yaml` existiert), Workflow 4 (Baseline Computation — kann sofort ausgeführt werden), und endet mit Workflow 2 (Metric Validation — ausstehend) und Workflow 3 (Equivalence — nicht anwendbar für eine einzelne Probe). Pro Schritt: Status (done / can do now / requires collaborators), konkretes SQL oder Kommando, erwarteter Output, Verweis auf Playbook-Sektion.

### Provenance Inventory

* [x] **Alle manuell gesetzten Werte.** Tabelle: Wert, Ort (Tabelle/Datei/Config), gesetzt von, Datum, Autorität (WP-Referenz), aktueller Review-Status. Für Probe 0: 2 × `primary_function`, 2 × `secondary_function`, 2 × `function_weights = NULL`, 2 × `BiasContext`-Statische Werte (6 Felder je Quelle), 5 × `known_limitations` in `metric_provenance.yaml`, 2 × `min_meaningful_resolution`-Heuristiken, Cultural Calendar `de.yaml` (N Einträge). Lebender Abschnitt — bei jedem neuen manuell gesetzten Wert aktualisiert.

### Integration

* [x] **`mkdocs.yml`.** `Scientific Operations Guide` als Top-Level-Eintrag registrieren, zwischen `Operations Playbook` und `Probes`. Navigation:
  ```yaml
  - "Operations":
      - "Operations Playbook": operations_playbook.md
      - "Scientific Operations Guide": scientific_operations_guide.md
  - "Probes":
      - "Probe 0 — DE Institutional RSS":
          - "Overview": probes/probe-0-de-institutional-rss/README.md
          - "Classification (WP-001)": probes/probe-0-de-institutional-rss/classification.md
          - "Bias Assessment (WP-003)": probes/probe-0-de-institutional-rss/bias_assessment.md
          - "Temporal Profile (WP-005)": probes/probe-0-de-institutional-rss/temporal_profile.md
          - "Observer Effect (WP-006)": probes/probe-0-de-institutional-rss/observer_effect.md
  ```

* [x] **Strukturelle Parallelität mit Operations Playbook.** Jeder Workflow im Scientific Operations Guide verweist auf den korrespondierenden Playbook-Abschnitt für die technischen Kommandos. Jeder Playbook-Abschnitt verweist zurück auf den korrespondierenden Workflow für die wissenschaftliche Begründung. Die Verweise sind bidirektional und verwenden konsistente Anker-IDs.

* [x] **Cross-Reference Audit and Arc42 Dokumentation** Grep-Sweep für alle Verweise auf `Scientific Operations Guide`.

* [x] **Validate.** Dokumentations-Review. Alle Cross-References zu Playbook-Sektionen, Working-Paper-Sektionen, Arc42-Kapiteln, Phase-68-Templates und Probe-Dossier-Dateien auflösen. `mkdocs build --strict`.


## Phase 72: Test Coverage for Scientific Infrastructure (Phases 62–68) - [x] DONE

*Phases 62–64 sind implementiert und bestehen `make test` — aber die Test-Gates verifizierten keine Regressionen, nicht dass neue Funktionalität durch dedizierte Tests abgedeckt ist. Phases 65–67 folgen dem gleichen Muster. Diese Phase schließt die Test-Lücke über alle drei Schichten (Unit, Integration, E2E) gemäß der Hybrid Testing Strategy (ADR-005).*

### Python Unit Tests (Analysis Worker)

* [x] **`tests/test_discourse_context.py` (Phase 62 — retroaktiv).** `DiscourseContext`-Propagation im `RSSAdapter`:
  - Adapter erhält Mock-`source_classifications`-Zeile → `RssMeta.discourse_context` korrekt populiert mit `primary_function`, `secondary_function`, `emic_designation`.
  - Keine Klassifikation für die Quelle → `RssMeta.discourse_context` ist `None`, Pipeline failt nicht.
  - Klassifikation hat `function_weights = NULL` → Feld ist `None` im Model, kein Crash.
  - `review_status = 'provisional_engineering'` wird korrekt propagiert.

* [x] **`tests/test_bias_context.py` (Phase 64 — retroaktiv).** `BiasContext`-Population im `RSSAdapter`:
  - Adapter produziert korrekte statische Werte für RSS-Quellen (`platform_type='rss'`, `visibility_mechanism='chronological'`, etc.).
  - Alle `BiasContext`-Felder sind non-null für RSS-Quellen.
  - `BiasContext`-Model validiert — fehlende Pflichtfelder raisen `ValidationError`.

* [x] **`tests/test_discourse_function_gold.py` (Phase 62 — retroaktiv).** Extractor Pipeline schreibt `discourse_function` in ClickHouse-Insert-Rows:
  - `DiscourseContext` vorhanden → `discourse_function`-Spalte enthält `primary_function`-Wert.
  - `DiscourseContext` ist `None` → `discourse_function`-Spalte enthält leeren String (`DEFAULT ''`).

* [x] **`tests/test_compute_baselines.py` (Phase 65).** `scripts/compute_baselines.py`-Logik (in testbare Funktion extrahiert):
  - Bekannter Satz von Metrik-Werten → korrekte Mean und Standardabweichung.
  - Leerer Metrik-Satz → kein Insert, kein Crash.
  - Einzelwert-Metrik-Satz → Standardabweichung ist 0, graceful behandelt.

### Go Integration Tests (BFF API)

* [x] **`internal/storage/metrics_query_test.go` — Normalization Tests (Phase 65).** Extend:
  - `?normalization=zscore` ohne Baseline in `metric_baselines` → HTTP 400 mit deskriptiver Fehlermeldung.
  - `?normalization=zscore` mit Baseline aber ohne `metric_equivalence`-Eintrag → HTTP 400.
  - `?normalization=zscore` mit valider Baseline und Equivalence-Eintrag → korrekte z-Score-Werte.
  - `?normalization=raw` (Default) → unverändertes Verhalten, bestehende Tests grün.

* [x] **`internal/storage/metrics_query_test.go` — Resolution Tests (Phase 66).** Extend:
  - `?resolution=hourly` → ClickHouse liefert stündlich aggregierte Datenpunkte (Timestamp-Bucketing verifizieren).
  - `?resolution=daily` → weniger Datenpunkte als `hourly` für denselben Zeitraum.
  - `?resolution=monthly` → korrekte Monatsstart-Timestamps.
  - Default (kein Parameter) → 5-Minuten-Bucketing, backward-compatible.
  - `rowLimit`-Anpassung → weitere Resolutions erlauben proportional mehr Rows.

* [x] **`internal/handler/provenance_handler_test.go` (Phase 67).** Neues Test-File:
  - `GET /metrics/word_count/provenance` → Tier 1, Algorithm Description, leere Limitations-Liste.
  - `GET /metrics/sentiment_score/provenance` → Tier 1, Known Limitations (Negation Blindness, Compound Word Failure).
  - `GET /metrics/nonexistent/provenance` → HTTP 404.
  - Validation Status Join: Metrik mit Eintrag in `metric_validity` → `validation_status = 'validated'`. Metrik ohne Eintrag → `validation_status = 'unvalidated'`.

* [x] **`internal/storage/metrics_query_test.go` — Available Metrics Extensions (Phases 63, 65, 66).** Extend:
  - Response enthält `validation_status` pro Metrik (Default `unvalidated`).
  - Response enthält `min_meaningful_resolution` wenn konfiguriert.
  - Response enthält `etic_construct` und `equivalence_level` wenn `metric_equivalence`-Eintrag existiert.

### Go Integration Tests (Ingestion API)

* [x] **`internal/storage/postgres_test.go` — Source Classifications (Phase 62 — retroaktiv).** Extend:
  - `get_source_classification(source_id)` liefert korrekte Klassifikation für geseedete Quelle.
  - `get_source_classification(unknown_id)` liefert `nil`, kein Error.
  - Mehrere Klassifikationen für dieselbe Quelle (verschiedene `classification_date`) → liefert die neueste.
  - Foreign-Key-Integrität: Klassifikation referenziert nicht-existente `source_id` → Insert failt.

### E2E Smoke Test Extension

* [x] **Extend `scripts/e2e_smoke_test.sh` (Phase 62 — retroaktiv).** Nach bestehenden Assertions (word_count, sentiment_score, entities):
  - Query `GET /api/v1/metrics?metricName=word_count&startDate=...&endDate=...` und verify: Response enthält non-empty `discourse_function`-Feld.

* [x] **Extend `scripts/e2e_smoke_test.sh` (Phase 66).** Query `GET /api/v1/metrics?resolution=hourly&startDate=...&endDate=...` und verify: Response liefert Daten.

* [x] **Extend `scripts/e2e_smoke_test.sh` (Phase 67).** Query `GET /api/v1/metrics/word_count/provenance` und verify: Response enthält `tier_classification` und `algorithm_description`.

### Validate

* [x] **Full Validation Suite.** `make test` (alle neuen + bestehenden Tests grün), `make test-e2e` (erweiterter Smoke Test grün), `make lint`, `make audit`, `make codegen && git diff --exit-code`.

* [x] **Arc42 §8.1 (Testing Strategy) aktualisieren.** Paragraph zur Testabdeckung für Scientific Infrastructure Tables und das Validation-Gate Testing Pattern (HTTP 400 für Endpoints, die validierte wissenschaftliche Daten erfordern).

## Post-Review Phases (73–82)

*Die Phasen 73–81 sind das Ergebnis eines kritischen Code Reviews nach Abschluss von Phase 72. Sie sind nach Dringlichkeit geordnet und so aufgeteilt, dass jede Phase nicht zu gross ist und in sich abgeschlossene Gates hat. Prioritäten:*

- **P0 (Correctness / Hard-Rule-Violations):** Phasen 73, 74
- **P1 (Security Hygiene):** Phase 75
- **P2 (Data Quality & Boundaries):** Phasen 76, 77, 78
- **P3 (Robustness & Clean-ups):** Phasen 79
- **P-Docs (parallel):** Phasen 80, 81

## Phase 73: Worker Hard-Rule-5 Restoration & IaC Fix [P0] - [x] DONE

*Der `analysis-worker` erstellt zur Laufzeit JetStream-Streams und initialisiert Extractors so, dass ein fehlender spaCy-Modell oder SentiWS-Lexicon den gesamten Worker crasht — entgegen der zugesagten Graceful Degradation. Kleinster Fix mit höchstem Gewinn, weil er zwei explizite Zusicherungen der Dokumentation mit dem Code in Einklang bringt.*

* [x] **`js.add_stream` aus `services/analysis-worker/main.py` entfernen.** Zeile ~143: stream wird bereits vom `nats-init` Container idempotent angelegt. Die Python-Zeile ersatzlos löschen — der `depends_on: nats-init: service_completed_successfully` Gate ist ausreichend. Hard Rule 5 wieder erfüllt.
* [x] **`infra/nats/` anlegen.** Verzeichnis mit `streams/AER_LAKE.json` als versionierte Stream-Definition. `nats-init`-Container konsumiert die Datei per `nats stream add --config /config/streams/AER_LAKE.json`. Dokumentationsbehauptung ("infra/nats/") damit eingelöst.
* [x] **Extractor-Init: einzeln + graceful.** `main.py` konstruiert die Extractor-Liste in einem einzigen `try/except` — ein Init-Fehler killt alle. Stattdessen iterieren:

  ```python
  extractor_classes = [WordCountExtractor, TemporalDistributionExtractor, ...]
  extractors = []
  for cls in extractor_classes:
      try:
          extractors.append(cls())
      except Exception as e:
          logger.warning("Extractor init failed — skipping", extractor=cls.__name__, error=str(e))
  ```
* [x] **Test: Worker startet ohne SentiWS-Lexicon.** Neuer Test in `tests/test_main_bootstrap.py`: leeres `data/sentiws/`-Verzeichnis gemockt → Worker startet, Extractor-Liste enthält keinen `SentimentExtractor`, verarbeitet Dokumente ohne Crash.
* [x] **Validate.** `make test`, `make test-e2e`, `make lint`.

## Phase 74: Worker Idempotency — ReplacingMergeTree Dedup Gate [P0] - [x] DONE

*Der Processor schreibt sequenziell in drei ClickHouse-Tabellen, bevor `document_status = 'processed'` gesetzt wird. Bei NATS-Redelivery nach Teil-Erfolg werden bereits gelandete Rows dupliziert. Aktuelle Tabellen sind plain `MergeTree` — sie deduplizieren nicht. Fix: Engine auf `ReplacingMergeTree` umstellen.*

* [x] **ClickHouse-Migration `000010_replacing_merge_tree.sql`.** Drei `RENAME TABLE` + `CREATE TABLE ... ENGINE = ReplacingMergeTree(ingestion_version) ORDER BY (article_id, metric_name)` + `INSERT INTO new SELECT * FROM old` + `DROP old` für `aer_gold.metrics`, `aer_gold.entities`, `aer_gold.language_detections`. `ingestion_version` ist eine monotone UInt64-Spalte (Event-Zeitstempel als Unix-Nanos).
* [x] **Processor schreibt `ingestion_version`.** `processor.py` setzt `ingestion_version = int(event_time.timestamp() * 1e9)` auf allen drei Insert-Batches.
* [x] **Dedup-Test.** Integrationstest mit Testcontainers: zwei Kopien desselben NATS-Events verarbeiten → nach `OPTIMIZE TABLE ... FINAL` ist je nur eine Row pro `(article_id, metric_name)` übrig.
* [x] **R-14 in Arc42 Kapitel 11 als "resolved via Phase 74" markieren** (Erstanlage erfolgt in Phase 80).
* [x] **Validate.** `make test`, `make test-e2e`.


## Phase 75: BFF Security Hardening [P1] - [x] DONE

*Drei unabhängige, nicht-breaking Security-Patches im BFF und dem geteilten `pkg/middleware`. Jeder Patch ist eine Einzel-Datei-Änderung plus Test.*

* [x] **`err.Error()` Leaks in `handler/handler.go` entfernen.** 9 Stellen in `handler.go` (und 1 in `main.go`) geben interne Fehlermeldungen an den Client. Ersetzen durch generische `"internal server error"`-Message + strukturiertes `slog.Error("handler failure", "op", "GetMetrics", "error", err)`. OpenAPI-Contract unverändert.
* [x] **Handler-Tests anpassen.** `entities_handler_test.go`, `metrics_handler_test.go`, `provenance_handler_test.go`: alle `Message`-Assertions auf die neue generische Message.
* [x] **`pkg/middleware/apikey.go` auf Constant-Time-Vergleich.** `subtle.ConstantTimeCompare([]byte(token), []byte(apiKey)) == 1`. Gleichzeitig: `w.Header().Set("Content-Type", "application/json")` vor `WriteHeader(401)`, statt `http.Error` (das setzt text/plain).
* [x] **`apikey_test.go`.** Neue Tests: (a) Content-Type-Header ist `application/json` bei 401, (b) Mismatch-Timing: Vergleich mit 1-Char-Differenz vs. voller Länge ist zeitlich ununterscheidbar (optional — schwer zu testen, deshalb nur Sanity-Check, dass `subtle` verwendet wird).
* [x] **API-Key-Boot-Validation.** `services/bff-api/internal/config/config.go` und `services/ingestion-api/internal/config/config.go`: bei leerem Key `return nil, fmt.Errorf("BFF_API_KEY must be set")`. Gleiche Behandlung für `CLICKHOUSE_PASSWORD` und `POSTGRES_PASSWORD` in beiden Services.
* [x] **Request-Logger: Trace-ID aus OTel-Context.** `services/bff-api/cmd/server/main.go` `requestLogger` liest aktuell `r.Header.Get("Traceparent")` — das ist der eingehende Header, nicht die vom Otelhttp-Middleware aufgebaute Trace-ID. Stattdessen: `trace.SpanFromContext(r.Context()).SpanContext().TraceID().String()`. Damit werden Access-Log und Tempo-Spans überhaupt erst korrelierbar.
* [x] **Validate.** `make test-go-pkg`, `make test-go`, `make lint`.


## Phase 76: Analysis Worker Data Quality [P2] - [x] DONE

*Vier kleine Adapter- und Processor-Fixes, die alle denselben Kern-Widerspruch behandeln: der Worker "weiß" Dinge, die er eigentlich dynamisch beobachten sollte, oder widerspricht sich selbst.*

* [x] **`RssAdapter.language` nicht mehr hardcoden.** `adapters/rss.py`: `language=""` (explizit "unknown"), der `LanguageDetectionExtractor` ist die SSoT. Alternativ leer lassen und in `core.language` einen Sentinel (`"und"` = undetermined, ISO 639-3) verwenden. Test in `test_rss_adapter.py`.
* [x] **N+1 Classification-Query cachen.** `adapters/rss.py` ruft `get_source_classification(self._pg_pool, source)` pro Dokument. Einfachster Patch: im Adapter ein `dict[str, tuple[dict|None, float]]` mit 60-Sekunden-TTL pro `source`. Ein einzelner `time.monotonic()`-Check reicht, kein LRU nötig (Sources sind O(10)).
* [x] **Word-Count-Dopplung entfernen.** `WordCountExtractor` liest `core.word_count` statt `core.cleaned_text.split()` neu zu tokenisieren. `legacy.py` / `rss.py` bleiben die einzige Quelle der Wahrheit für die Tokenisierung.
* [x] **Processor/Meta-Contract klarstellen.** `processor.py` liest `meta.discourse_context.primary_function` direkt — das ist der einzige Ort, an dem Gold-Row-Assembly `SilverMeta` berührt. Zwei Optionen:
  - **(a)** Code bleibt, aber kleine Refaktorisierung: ein Helper `_derive_discourse_function(meta) -> str` isoliert den einzigen Punkt, an dem der Contract "bricht", und ist unit-testbar.
  - **(b)** `extract_all(core, meta, article_id)` als Breaking-Protocol-Change.

  **Empfehlung (a)** — kleinster Patch, klare Lokation für zukünftige Erweiterung.
* [x] **Validate.** `make test-python`, `make test-e2e`.

## Phase 77: Ingestion API Semantic Fixes [P2] - [x] DONE

*Zwei kleine, aber semantisch wichtige Korrekturen im `ingestion-api`.*

* [x] **`errorCount` aufteilen.** `internal/core/service.go` `IngestDocuments`: zwei Zähler (`uploadFailures`, `statusUpdateFailures`). Job-Final-Status basiert nur auf `uploadFailures`. Ein Dokument mit `uploaded`-Objekt in MinIO, aber fehlgeschlagenem Status-Update, darf den Job nicht als `failed` markieren. Der Status-Update-Fehler wird als `slog.Error` mit `op="status_update"` geloggt und als Prometheus-Counter surface'd (`ingestion_status_update_failures_total`).
* [x] **Bronze-Bucketname konfigurierbar.** `internal/core/service.go` hat `"bronze"` zweimal hart-kodiert. Auf `s.bucket` + Config-Feld `BronzeBucket` (ENV `INGESTION_BRONZE_BUCKET`, Default `bronze`). `.env.example`, `compose.yaml` (ingestion-api + analysis-worker) erweitern. Worker `processor.py` bekommt Konfiguration via ENV `WORKER_BRONZE_BUCKET` (gleicher Default). **Zwei Services, eine Wahrheit.**
* [x] **Test**: `postgres_test.go` Regression — Status-Update-Failure darf Job-Status nicht kippen. *(Regression lives in `internal/core/service_test.go::TestIngestDocuments_StatusUpdateFailureDoesNotFailJob` — pure unit test on the core contract, no PG container needed.)*
* [x] **Validate.** `make test-go`, `make test-e2e`.

## Phase 78: BFF Query Robustness [P2] - [x] DONE

*Zwei Verhaltensfragen im BFF, die aktuell implizit falsch antworten.*

* [x] **`GetNormalizedMetrics`: `LEFT JOIN` statt `INNER JOIN` auf `language_detections`.** `storage/metrics_query.go`: Metriken ohne zugehörige Language-Detection verschwinden still. Auf `LEFT JOIN` umstellen, `WHERE ld.detected_language IS NOT NULL` — und der Handler gibt zusätzlich einen `excluded_count` im Response-Envelope zurück (OpenAPI-Änderung, `make codegen`). Alternativ: `excluded_count` nur loggen, Response-Schema unverändert lassen. **Empfehlung: sauberste Lösung verwenden. Entscheidung für Option 1 wurde getroffen**
* [x] **`NewClickHouseStorage` Backoff-Budget auf 60s.** `MaxElapsedTime` von 30s auf 60s, cold Docker-Start braucht oft länger.
* [x] **Regressionstest.** Integrationstest mit Metrik ohne Language-Row: Ergebnis-Anzahl stabil, Log-Eintrag vorhanden.
* [x] **Validate.** `make test-go`, `make test-e2e`.

## Phase 79: Infra & Credentials Cleanup [P3] - [x] DONE

*Zwei Housekeeping-Items. Beide reduzieren Angriffsoberfläche, ohne den Happy Path zu berühren.*

* [x] **MinIO Service Accounts.** `infra/minio/setup.sh` erweitern um `mc admin user add aer_worker ...` und `mc admin user add aer_ingestion ...` mit `readwrite`-Policies auf den jeweils benötigten Buckets. `compose.yaml` für beide Services auf die neuen Credentials umstellen. `MINIO_ROOT_USER` bleibt nur für `minio-init` und `setup.sh`.
* [x] **Alle Credentials aus `.env.example` auf GitHub Actions Secrets migrieren.** Phase 75 hat nur die vier boot-validierten Secrets abgedeckt (`BFF_API_KEY`, `INGESTION_API_KEY`, `CLICKHOUSE_PASSWORD`, `POSTGRES_PASSWORD`). Die übrigen Credentials müssen ebenfalls aus GitHub Secrets statt `.env.example`-Defaults stammen: `MINIO_ROOT_USER`, `MINIO_ROOT_PASSWORD`, `MINIO_ACCESS_KEY`, `MINIO_SECRET_KEY` (nach Service-Account-Split: `aer_worker`/`aer_ingestion`), `GF_SECURITY_ADMIN_PASSWORD`. `.env.example` enthält am Ende nur noch nicht-sensitive Defaults oder Platzhalter. CI-Job `e2e-smoke` erweitert den `sed`-Block entsprechend. Ebenfalls `DB_URL` anpassen und das Secret updaten.
* [x] **ClickHouse Memory-Limits verifizieren.** `compose.yaml` auf `deploy.resources.limits` für `clickhouse` prüfen — falls fehlend, 2G/1 CPU als Default setzen. Phase 20 hat das beansprucht, aber ein Re-Check ist billig.
* [x] **Rate-Limiter-Beschreibung korrigieren (Code oder Doku).** Entweder auf Per-API-Key (Phase 16 Anspruch) umstellen — oder Operations Playbook klarstellen, dass der Limiter global ist. **Empfehlung: Letzteres, bis es mindestens zwei Konsumenten gibt.**
* [x] **Metrics-Cache: Text oder LRU.** Arc42 §8.11 beschreibt einen "Metrics Cache" implizit als Multi-Slot; tatsächlich ist es ein Single-Slot. Entweder auf `hashicorp/golang-lru/v2` mit 16 Slots umstellen, oder Text präzisieren. **Empfehlung: Text präzisieren.**
* [x] **Validate.** `make test`, `make up` Smoke-Check.


## Phase 80: Arc42 Structural Fix [P-Docs] - [x] DONE

*Arc42 Kapitel 8 hat nach Phasen 62–72 strukturelle Drift. ROADMAP.md hat ein Duplikat. Beides wird isoliert konsolidiert, ohne Code-Änderungen. Läuft parallel zu Phasen 73–79.*

* [x] **Phase 67 Duplikat in ROADMAP.md entfernen.** Zwei identische Blöcke.
* [x] **Arc42 Kapitel 8 Neu-Nummerierung.** Aktuelle Reihenfolge im File: `8.1 … 8.8 … 8.9 … 8.10 … 8.9.3 Configuration … 8.11 … 8.12 … 8.13 … 8.8 (addendum) … 8.14 … 8.15`. Defekte:
  - `8.9.3 Configuration Management` steht **hinter** `8.10` — muss zu `8.9` vor 8.10 umgesetzt werden.
  - `8.8 (addendum, Phase 66)` ist ein zweites `§8.8` — sollte als `8.8.1 Tiered Retention (Planned)` subsummiert werden.
  - Phase 69 hat bereits notiert, dass die ROADMAP `§8.6/§8.10/§8.12` referenziert, die tatsächlich `§8.13/§8.12/§8.14` sind — diese alten Referenzen in ROADMAP-Historie und Cross-Doku korrigieren.
* [x] **R-14: Triple-Insert-Risiko in `11_risks_and_technical_debts.md`.** Neuer Eintrag; da Phase 74 bereits committed ist, wurde der Eintrag direkt als "Resolved (Phase 74)" mit Phase-74-Mitigation angelegt.
* [x] **ADR-018 / ADR-019 in `09_architecture_decisions.md`.**
  - **ADR-018: Constant-Time API-Key-Vergleich.** Begründung, Impl-Referenz, Non-Goals.
  - **ADR-019: IaC-only NATS-Stream-Provisionierung.** Worum es geht, warum `js.add_stream` entfernt wurde, Referenz auf `infra/nats/streams/`.
* [x] **`mkdocs build --strict`** grün, Quer-Verweise stichprobenartig geprüft.


## Phase 81: Documentation Alignment — CLAUDE.md, README, Playbook, WPs [P-Docs] - [x] DONE

*Parallel zu Phase 80. Aktualisiert Dokumentation, die sich durch Phasen 73–79 ändert. Keine Vorab-Änderungen — diese Phase folgt dem Code.*

* [x] **CLAUDE.md Hard Rule 5 präzisieren.** Zusatz: "Diese Regel schließt NATS-Stream-Provisionierung ein. Siehe `infra/nats/streams/` und den `nats-init`-Container."
* [x] **CLAUDE.md "Extractors receive immutable SilverCore" korrigieren.** Nach Phase 76 präziser: "Extractors receive SilverCore. The processor may enrich Gold rows with `SilverMeta`-derived context (e.g. `discourse_function`) via a dedicated helper — this is the only sanctioned point where meta influences Gold."
* [x] **README.md: `infra/nats/` Verweis reparieren.** Linkziel an das nach Phase 73 erstellte Verzeichnis anpassen.
* [x] **Operations Playbook: neue ENV-Variablen.** `INGESTION_BRONZE_BUCKET`, `WORKER_BRONZE_BUCKET` (Phase 77), MinIO-Service-Account-Credentials (Phase 79).
* [x] **Arc42 §8.7.1 Constant-Time Compare dokumentieren** (nach Phase 75).
* [x] **Arc42 §8.11 Metrics Cache Wortlaut** (falls in Phase 79 Text-Option gewählt). — bereits in Phase 79 (commit 39ca291) auf "single-slot" präzisiert; keine weitere Änderung nötig.
* [x] **WP-001..006 Cross-Reference Sweep.** Grep über alle sechs Papers (DE+EN) auf `§` und `WP-XXX`. Ziel: Arc42-Abschnittsnummern nach Phase 80 stimmen, Playbook-Referenzen stimmen. — WP-004 / WP-005 (DE+EN) Header-Refs von `§8.6` (ClickHouse OLAP / BFF Downsampling, heute Observability) auf `§8.13` (Multi-Resolution Temporal Framework) bzw. `§8.8` (Data Lifecycle) umgezogen.
* [x] **`mkdocs build --strict`** grün, `make lint` grün.

## Phase 82: Ingestion API Edge Hardening [P0] - [x] Done

*Der `ingestion-api` ist der einzige Pfad, über den externe Daten ins System kommen. Drei kleine, aber load-bearing Lücken, die noch aus der Pre-Phase-75-Ära stammen und beim BFF-Review (Phase 75) symmetrisch übersehen wurden.*

* [x] **HTTP-Server-Timeouts.** `services/ingestion-api/cmd/api/main.go` konstruiert den `http.Server` nur mit `Addr` und `Handler`. Keine `ReadHeaderTimeout`, `ReadTimeout`, `WriteTimeout`, `IdleTimeout`, `MaxHeaderBytes` → Slowloris-tauglich, unbegrenzt offene Idle-Connections. **Werte**: `ReadHeaderTimeout: 5s`, `ReadTimeout: 30s`, `WriteTimeout: 60s` (Batch-Upload muss reichen), `IdleTimeout: 120s`, `MaxHeaderBytes: 1 << 20`. Der `bff-api` erbt dieselbe Lücke (`services/bff-api/cmd/server/main.go`) — im selben Commit mitziehen.
* [x] **Fehler-Leak im Handler schließen (Symmetrie zu Phase 75).** `internal/handler/handler.go` Zeile 34 gibt `"invalid JSON body: " + err.Error()` an den Client zurück, Zeile 62 leakt die rohe Service-Fehlermeldung. Phase 75 hat genau dieses Muster im BFF gefixt, den Ingestion-Handler aber übersehen. Generische Messages an den Client, Detail nur in `slog.Error` mit `request_id`.
* [x] **Request-Body-Size-Limit.** Aktuell kein `http.MaxBytesReader` im Ingest-Handler → ein 10-GB-POST frisst Memory, bis der OOM-Killer greift. Hard Limit `INGESTION_MAX_BODY_BYTES` (Default 16 MiB) via Middleware vor dem JSON-Decoder.
* [x] **Graceful-Shutdown-Budget auf 30s, konfigurierbar.** Der 5s-Timeout (Zeile 169) ist knapper als `WriteTimeout: 60s` — ein laufender Batch wird hart abgebrochen. Auf 30s erhöhen, `INGESTION_SHUTDOWN_TIMEOUT_SECONDS` aus Config. Gleicher Fix im BFF (dort ebenfalls 5s).
* [x] **Tests.** Handler-Test, der bei fehlerhaftem JSON keinen `err.Error()`-Inhalt im Response-Body sieht. Handler-Test, der bei 32-MiB-Body ein `http.StatusRequestEntityTooLarge` (413) erhält.
* [x] **Validate.** `make test-go`, `make test-e2e`.


## Phase 83: Analysis Worker Backpressure & Poison-Pill Containment [P0] - [x] Done

*Der `analysis-worker` hat zwei Stellen, an denen ein einzelner Fehlertyp das System umkippen kann: eine unbegrenzte In-Process-Queue und ein Retry-Loop, der schlechte Nachrichten ewig reshippt.*

* [x] **`asyncio.Queue` begrenzen.** `services/analysis-worker/main.py` Zeile 147: `task_queue = asyncio.Queue()` — unbounded. Kombiniert mit einem `message_handler`, der nur `await task_queue.put(msg)` macht, hat das System keine Backpressure: eine langsame Extractor-Pipeline lässt die Queue (und den Python-Heap) ins Unendliche wachsen. **Fix**: `asyncio.Queue(maxsize=config.worker_count * 4)`. `put()` blockiert damit, wenn die Worker nicht hinterherkommen — NATS JetStream liefert dann durch `max_ack_pending` von sich aus Backpressure.
* [x] **NATS-Consumer-Safety-Parameter.** `js.subscribe(...)` (Zeile 175ff.) setzt weder `max_ack_pending`, noch `ack_wait`, noch `max_deliver`. **Werte**: `max_ack_pending = config.worker_count * 4` (matches queue size), `ack_wait = 60s` (muss länger als die durchschnittliche Verarbeitungszeit eines Dokuments sein), `max_deliver = 5`. Nach 5 Zustellversuchen wandert die Nachricht in eine Dead-Letter-Subjekt-Semantik (siehe nächster Punkt).
* [x] **Poison-Pill-DLQ statt NAK-Loop.** `worker_task` NAKt heute jede Exception (siehe `main.py`), was bei einem deterministischen Fehler (z.B. ein Adapter-Bug, den die Nachricht jedes Mal auslöst) einen endlosen Redelivery-Loop erzeugt. **Fix**: Wenn `msg.metadata.num_delivered >= max_deliver`, die Nachricht in den bestehenden `bronze-quarantine`-DLQ-Pfad schreiben (neuer Helper `DataProcessor.quarantine_poison_message(msg_data, error_type, error_text)`), **dann** `msg.ack()` statt `nak()`. Counter `analysis_worker_poison_messages_total` (Label `reason`). Sollte der Quarantine-Write selbst scheitern, fällt der Handler auf `nak()` zurück, damit NATS den Stuck-Zustand über `max_deliver`-Metriken sichtbar macht.
* [x] **`OTEL_TRACE_SAMPLE_RATE` ehren.** `init_telemetry` in `main.py` konstruiert den Tracer-Provider ohne Sampler-Argument → 100 % Sampling, egal was in `.env` steht. Symmetrie zu den Go-Services herstellen: `ParentBased(TraceIdRatioBased(rate))`, Default `1.0` wie heute, aber Production-ready.
* [x] **Tests.** `tests/test_backpressure_and_poison_pill.py`: 8 Unit-Tests pinnen (1) den Poison-Pill-Pfad mit immer-failendem Mock-Processor inkl. Fallback bei Quarantine-Write-Fehler, (2) das Recovery-vs-Synthetic-Envelope-Verhalten von `quarantine_poison_message`, (3) das Blocking-Verhalten einer `maxsize=4` Queue gegen 100 parallele `put`-Calls, (4) das Sampler-Wiring in `init_telemetry`.
* [x] **Validate.** `make test-python` (125 passed), `make test-e2e` (12 passed).


## Phase 84: Supply Chain & Container Hardening [P1] - [x] Done

*Das Projekt ist noch nicht deployed — genau der richtige Moment, die Container-Images zu härten, bevor ein Registry-Push sie für immer zementiert.*

* [x] **`.dockerignore` anlegen.** Weder Repo-Root noch Service-Directories haben ein `.dockerignore`. Konsequenz: Der `analysis-worker`-Build (`services/analysis-worker/Dockerfile` Zeile 33: `COPY services/analysis-worker/ .`) kopiert das gesamte `.venv/` ins Final-Image → aufgeblähtes Image, redundante Python-Deps. **Fix**: `.dockerignore` im Repo-Root, das `**/.venv/`, `.git/`, `**/tests/`, `*.pyc`, `__pycache__/`, `.pytest_cache/`, `.ruff_cache/`, `**/bin/`, `.env`, `.env.*`, `logs/`, `.pids/` ausschließt. (`**/`-Prefix ist nötig, damit Patterns auch Service-Unterverzeichnisse erwischen.)
* [x] **Alpine-Basis-Images SHA256-pinnen.** `services/ingestion-api/Dockerfile`, `services/bff-api/Dockerfile` und das Builder-Stage des Workers verwenden `alpine:3.XX` als Tag. Tag ≠ Immutable. **Fix**: `FROM alpine:3.XX@sha256:...` — Digest aus der aktuell gepullten Image via `docker inspect`. Kommentar mit Upstream-Link, damit der nächste Pin-Refresh nachvollziehbar ist. Gilt symmetrisch für `golang:1.26.2-alpine3.23` (Builder) und `python:3.14.3-slim-bookworm` (Worker).
* [x] **`USER`-Direktive + `HEALTHCHECK`.** Alle drei Service-Dockerfiles laufen heute als `root`. Nicht-Root-User einführen (`adduser -S aer -u 10001`, `USER aer`). `HEALTHCHECK --interval=30s --timeout=5s CMD wget -qO- http://localhost:$PORT/api/v1/healthz || exit 1` für die Go-Services, Python-Pendant für den Worker (Healthcheck via Prometheus-Port 8001 `/metrics`).
* [x] **Go-Builds mit `-trimpath` und `-ldflags="-s -w"`.** Reproduzierbarkeit + kleinere Binaries, gratis.
* [x] **SentiWS-Hash-Pin.** `services/analysis-worker/Dockerfile` lädt SentiWS zur Build-Zeit ohne SHA256-Verifikation. **Fix**: `ARG SENTIWS_SHA256=...` + `echo "${SENTIWS_SHA256}  sentiws.zip" | sha256sum -c -` nach dem `wget`.
* [x] **`requirements.txt` mit `--hash=` pinnen.** `services/analysis-worker/requirements.txt` hat Version-Pins, aber keine Hash-Pins → ein kompromittierter PyPI-Account könnte eine Malicious-Version mit derselben Version zurückziehen/republishen. **Fix**: `pip-compile --generate-hashes --allow-unsafe` via `pip-tools`, CI-Step, der gegen den generierten Hash-Lockfile installiert (`pip install --require-hashes -r requirements.lock.txt`). Regenerierung immer im `python:3.14.3-slim-bookworm`-Container, damit Hashes zum Runtime-Python passen.
* [x] **Trivy-Severity auf `MEDIUM` anheben (soft fail).** `.github/workflows/ci.yml` scannt nur `HIGH,CRITICAL`. Zweiter Trivy-Step für `MEDIUM` mit `exit-code: 0` — reporting-only, damit Drift früh sichtbar wird, ohne den Build zu blockieren.
* [x] **Validate.** `make test-e2e` (12/12 passed). Image-Size-Vergleich: `aer-analysis-worker` 3.72 GB → 1.94 GB (−48 %, `.venv` war im alten Image); `aer-bff-api` und `aer-ingestion-api` unverändert bei 54 MB / 54 MB.


## Phase 85: Scalability Symmetry & Query-Path-Polish [P2] - [x] DONE

*Horizontale Skalierbarkeit steht und fällt mit symmetrischen Pool-Größen und nicht-serialisierten Batch-Operationen. Vier kleine, unabhängige Verbesserungen.*

* [x] **PostgreSQL-Pool-Symmetrie (Go-Seite).** `services/ingestion-api/internal/storage/postgres.go` nimmt jetzt eine `PoolConfig`-Struktur mit `MaxOpenConns`, `MaxIdleConns`, `ConnMaxLifetime`. Defaults (25/5/30m) in `INGESTION_DB_MAX_OPEN_CONNS`, `INGESTION_DB_MAX_IDLE_CONNS`, `INGESTION_DB_CONN_MAX_LIFETIME_MINUTES`.
* [x] **PG/CH-Pool-Symmetrie (Python-Seite).** `init_postgres(maxconn=...)` propagiert jetzt `WORKER_COUNT + PG_POOL_HEADROOM` (2) vom `main.py` nach unten. Kein Trockenlaufen mehr, wenn `WORKER_COUNT` skaliert wird.
* [x] **Konkurrenter MinIO-Upload im Ingestion-Batch.** `IngestDocuments` nutzt `errgroup.Group` mit `SetLimit(cfg.MinioUploadConcurrency)` (Default 8). Per-Index-`outcomes`-Slice bewahrt die Reihenfolge; neue Regression `TestIngestDocuments_ConcurrentUploadsPreserveOrdering` prüft Ordering, Accepted/Failed-Counts und tatsächliche Parallelität via Laufzeit-Budget.
* [x] **BFF Metrics-Cache auf Hot-Path anwenden.** `internal/storage/query_cache.go` liefert einen generischen `singleSlot[T]`. `GetNormalizedMetrics`, `GetEntities`, `GetLanguageDetections` prüfen einen eigenen Slot mit Key `endpoint|{params}` und TTL `BFF_METRICS_CACHE_TTL_SECONDS`. Bestehender Slot für `GetAvailableMetrics` unverändert.
* [x] **RssAdapter `_classification_cache` bounden.** `_classification_cache` ist jetzt eine `OrderedDict` mit LRU-Eviction bei `CLASSIFICATION_CACHE_MAX_ENTRIES=4096`. TTL bleibt 60s, Worker-Heap ist gegen pathologische Source-Kardinalität geschützt.
* [x] **Validate.** `make lint`, `make test-python` (125 passed), `make test-go` (all packages green inkl. neuer Ordering-Test), `make test-e2e` (12/12 passed).


## Phase 86: Observability Wiring [P3] - [x] DONE

*Kleine, aber nervige Lücken in der Telemetry-Pipeline. Jede einzeln wäre ein One-Liner-Bugfix, gesammelt ergeben sie ein ehrliches Observability-Fundament.*

* [x] **Prometheus-Scrape für `analysis-worker` repariert.** `infra/observability/prometheus.yml` zielt jetzt auf `analysis-worker:8001` im `aer-backend`-DNS. Der Worker-Dockerfile `EXPOSE`t Port 8001 bereits seit Phase 84; das `extra_hosts: host.docker.internal`-Workaround in `compose.yaml` wurde ersatzlos entfernt.
* [x] **Prometheus-Scrape für `bff-api` hinzugefügt.** Neuer Job `bff-api` mit `targets: ['bff-api:8080']` und `metrics_path: /metrics`. Der BFF-Router mountet `promhttp.Handler()` jetzt root-level *vor* der API-Key-Group, symmetrisch zum Ingestion-Service — die Scrape-Target bleibt damit unauthentifiziert (begründet durch Zero-Trust-Backbone-Isolation, siehe Arc42 §8.12).
* [x] **OTel-Collector-Pipeline mit Processors.** `infra/observability/otel-collector.yaml` hat jetzt einen `processors`-Block mit `memory_limiter` (check_interval 1s, limit 512 MiB, spike 128 MiB) und `batch` (timeout 10s, send_batch_size 8192). Beide Processors sind in den `traces`- und `metrics`-Pipelines verdrahtet (Reihenfolge: `[memory_limiter, batch]`). Kein `logs`-Pipeline, weil keine Log-Receiver konfiguriert sind — wird erst relevant, wenn strukturierte Logs über OTLP statt stdout fließen.
* [x] **RSS-Crawler-Metriken (Textfile-Collector).** `crawlers/rss-crawler/main.go` nimmt jetzt einen `--metrics-file`-Flag (env `PROMETHEUS_TEXTFILE_PATH`). Am Ende des Runs schreibt `writeTextfileMetrics` fünf Kennzahlen atomar (temp + rename) als Prometheus-Exposition-Format: `rss_crawler_feeds_crawled_total`, `rss_crawler_items_submitted_total`, `rss_crawler_items_skipped_total`, `rss_crawler_duration_seconds`, `rss_crawler_last_successful_crawl_timestamp`. Ohne Flag: No-op — Backwards-Compatible.
* [x] **BFF-CORS `X-API-Key` allowen.** `AllowedHeaders` in `services/bff-api/cmd/server/main.go` listet jetzt `X-API-Key` neben `Accept` und `Content-Type`. Preflight-Requests aus einem Browser-Frontend werden damit nicht mehr stillschweigend am CORS-Layer abgewiesen.
* [x] **NATS-Stream `num_replicas` dokumentiert.** ADR-019 bekommt einen neuen "Single-node defaults"-Absatz, der explizit sagt, dass `num_replicas: 1` + `max_age: 0` korrekt für Single-Node ist und dass ein Multi-Node-Deployment `num_replicas` auf eine quorum-sichere odd number (typischerweise 3) und `max_age` passend zur Bronze-TTL erhöhen muss. Keine Code-Änderung nötig — beide Werte sind bereits deklarativ in `infra/nats/streams/AER_LAKE.json` und werden von `nats-init` idempotent angewendet.
* [x] **Validate.** `make lint` grün. `docker compose up -d --build` brachte den vollen Stack hoch; alle vier Prometheus-Scrape-Targets (`analysis-worker`, `bff-api`, `ingestion-api`, `otel-collector`) melden `health: up` in `/api/v1/targets`. OTel-Collector-Log bestätigt `memory_limiter configured {limit_mib: 512, spike_limit_mib: 128}`. `make test-e2e` (12/12 passed), `make test-go-crawlers` (alle grün), `make test-python` (125 passed).


## Phase 87: Source-of-Truth Drift Resolution [P-Docs] - [x] DONE

*Zwei konkrete SSoT-Lecks, die nach den Code-Phasen 82–86 offen bleiben. Das einzige Phase-87-Item mit realer Code-Änderung ist die BFF-Sources-Entscheidung; der Rest ist Config-Cleanup.*

* [x] **BFF `sources.yaml` vs. Postgres `sources`-Tabelle auflösen.** Option A implementiert: `services/bff-api/configs/source_documentation.yaml` und `config.LoadSources` gelöscht. BFF öffnet einen kleinen Read-Only-Pool als neuer `bff_readonly`-Rolle (nur `SELECT` auf `public.sources`) und serviert `GET /api/v1/sources` aus einem TTL-gecachten `storage.SourceStore` mit Stale-Fallback. Die Rolle wird vom neuen Init-Container `postgres-init-roles` (`infra/postgres/init-roles.sh`) idempotent angelegt — Hard Rule 5 bleibt erhalten. Neue Env-Vars: `BFF_DB_USER`, `BFF_DB_PASSWORD`, `BFF_SOURCES_CACHE_TTL_SECONDS` (plus `POSTGRES_HOST`/`_PORT`/`_DB` für BFF).
* [x] **`.env.example`-Kommentare auf den aktuellen Stand.** Phase-79-Kommentare entfernt; boot-validierter Abschnitt listet jetzt auch `BFF_DB_USER` / `BFF_DB_PASSWORD` und referenziert den `postgres-init-roles`-Container.
* [x] **Validate.** `make lint` grün. BFF-Handler-Tests für `/sources` (`sources_handler_test.go`) decken Happy Path, Lister-Fehler und nil-Lister ab.

## Phase 88: Doc Sweep & Dependency Update Automation [P-Docs] - [x] DONE

*Strukturierter Arc42-Sweep nach den Code-Phasen 82–86 plus das Supply-Chain-Runbook, das Phase 84 offengelassen hat. Phase 88 setzt voraus, dass Phase 87 die SSoT-Entscheidung bereits gefällt hat — sonst referenziert der Sweep veraltete Architektur.*

* [x] **Arc42-Drift-Sweep nach Phasen 82–86.** §8.7 erweitert um §8.7.4 (HTTP-Server-Hardening inkl. Timeouts + Body-Cap + Generic-500-Masking, Phase 82), §8.7.5 (Container-Hardening, Phase 84) und §8.7.6 (Supply Chain mit `make deps-refresh`-Cross-Ref). Neues §8.16 "Analysis Worker Resilience (Phase 83)" mit §8.16.1 Bounded Queue + NATS-Backpressure, §8.16.2 Poison-Pill-DLQ, §8.16.3 OTel-Sampling. §8.11.1 "BFF Sources Cache (Phase 87)" dokumentiert die TTL-gepufferte Postgres-Anbindung mit Stale-Fallback. Kapitel 10 erhält QS-P5 (Slow-Loris / HTTP-Timeouts) und QS-P6 (16 MiB Body-Cap). Kapitel 11 erhält R-15 "Unbounded Task Queue OOM Under Burst Load" als Resolved (Phase 83). Stale Phase-87-Referenzen in §8.14, §8.15 und Ch05 entfernt.
* [x] **CLAUDE.md-Update.** Hard Rule 2 um Dockerfile-`HEALTHCHECK`-Direktiven ergänzt. Hard Rule 1b um `pip-tools`, `scripts/deps_refresh.sh` und `make deps-refresh`-Runbook-Cross-Ref erweitert. Neuer Abschnitt "Security Defaults" listet API-Auth (constant-time), Boot-Secret-Validation, HTTP-Server-Timeouts, Body-Size-Caps, Generic 5xx, Graceful Shutdown, Container-Hardening, Postgres Read-Only Roles, IaC-only Provisioning und No-Log-Parsing-Healthchecks als non-negotiable Defaults.
* [x] **Dependency / Image-Update Runbook + `make deps-refresh` Automation.** Nach Phase 84 sind vier supply-chain-Artefakte hash- bzw. digest-gepinnt: Dockerfile-`FROM @sha256:...`, `requirements.lock.txt` mit `--hash=`, `SENTIWS_SHA256`-ARG, Trivy-Allowlist. Sobald Trivy (oder `pip-audit` / `govulncheck`) eine Version-Bump erzwingt oder eine Upstream-Release erscheint, muss der Pfad für *jede* dieser Oberflächen eindeutig sein. **Deliverable (1) — Makefile-Target `deps-refresh`**: ein einziges Kommando automatisiert die 90 %, die mechanisch sind:
  1. Für jede gepinnte Base-Image-Zeile in den drei Dockerfiles: `docker pull <image>:<tag>` → neuen Digest aus dem Output extrahieren (`docker inspect --format='{{index .RepoDigests 0}}'`) → `sed -i` ersetzt die `@sha256:...`-Sektion hinter dem jeweiligen Tag. Die `FROM`-Zeilen haben eine stabile Form (`FROM <image>:<tag>@sha256:<digest>[ AS <stage>]`), sed-Regex ist deshalb load-bearing, aber nicht fragil.
  2. `docker run --rm -v $PWD/services/analysis-worker:/work -w /work python:3.14.3-slim-bookworm bash -c "pip install pip-tools && pip-compile --generate-hashes --allow-unsafe --output-file=requirements.lock.txt requirements.txt"` → regeneriert das Hash-Lockfile im Runtime-Python.
  3. `curl -sL https://downloads.wortschatz-leipzig.de/etc/SentiWS/SentiWS_v2.0.zip | sha256sum` → gibt den neuen Hash aus; `sed -i` ersetzt die `SENTIWS_SHA256=...`-Default-Zeile im Worker-Dockerfile.
  4. Smoke-Test: `docker compose build` ohne Cache, anschließend `make test-e2e`.
  Trivy-Entscheidungen (ignore vs. bump vs. wait) bleiben manuell — das Target liefert nur die mechanische Vorarbeit.
  **Deliverable (2) — Runbook `docs/operations_playbook.md#updating-pinned-dependencies`**: beschreibt, *wann* man `make deps-refresh` aufruft (neue Trivy-Findings, geplante Wartungsfenster, CVE-Alerts), wie man das Diff reviewt, und vor allem die manuellen Pfade für die Edge-Cases, die das Target *nicht* abdeckt:
  - **Happy path**: `make deps-refresh` → Diff reviewen → Commit mit Changelog-Zeile pro Artefakt.
  - **Neue Python-Dependency hinzufügen**: `requirements.txt` editieren *bevor* `make deps-refresh` läuft; das Target regeneriert das Lockfile automatisch. Bei Version-Conflicts manuell `pip-compile` mit `--upgrade-package <name>` aufrufen.
  - **Manuelle Base-Image-Tag-Bumps** (z. B. `python:3.14.3` → `3.14.4`): zuerst den Tag in allen drei Dockerfiles ersetzen, dann `make deps-refresh`, damit der neue Tag den frischen Digest bekommt.
  - **SentiWS-URL ändert sich** (Upstream-Moved-Szenario): URL im Worker-Dockerfile updaten, dann `make deps-refresh` — das Target curl't die *im Dockerfile definierte* URL, nicht eine harte Makefile-Konstante.
  - **Trivy-Finding-Triage**: (a) Package-Bump fixt's → `make deps-refresh`, (b) Kein Fix verfügbar → `.trivyignore`-Eintrag mit CVE + Ablaufdatum + Begründung, (c) False Positive → ignorieren mit Kommentar. In allen Fällen Commit-Message mit CVE-ID, betroffener Komponente und Entscheidungsbegründung.
  - **Cross-Ref**: Arc42 §8.7 (Security) und `CLAUDE.md` Hard Rule 1 um einen Verweis auf dieses Runbook + `make deps-refresh` ergänzen, damit die SSoT-Quellen (`compose.yaml`, `.tool-versions`, `requirements.lock.txt`, Dockerfile-Digests) und ihre Update-Pfade zentral auffindbar bleiben. `README.md` Abschnitt "Contributing" um einen Einzeiler erweitern: "Pinned dependency updates: run `make deps-refresh`, see `docs/operations_playbook.md#updating-pinned-dependencies`."
* [x] **Validate.** Arc42 von stale Phase-79- und `source_documentation.yaml`-Referenzen bereinigt (Ch05, §8.14, §8.15). `make deps-refresh --dry-run` läuft auf dem aktuellen Tree zur Verifikation — der tatsächliche Clean-Baseline-Run erfolgt beim maintainerseitigen Trigger direkt nach dem Merge.

## Phase 89: Shutdown & Resource Lifecycle Correctness [P0] - [x] DONE

*Five correctness gaps around process shutdown and resource cleanup. The shutdown timeout arithmetic from Phase 82 left both services with a drain budget (30s) that is shorter than their own WriteTimeout (60s) — a mid-flight request will be force-killed before it can finish. The Python worker never closes its database pools on exit and leaks MinIO HTTP connections during normal operation.*

* [x] **Shutdown timeout defaults must exceed WriteTimeout.** `.env.example` lines 80 and 100 both set `*_SHUTDOWN_TIMEOUT_SECONDS=30` with comments that explicitly state the value *"must exceed the HTTP WriteTimeout (60s)"*. `CLAUDE.md` Security Defaults confirm the same constraint. The `ingestion-api` (`cmd/api/main.go:168`) and `bff-api` (`cmd/server/main.go:158`) both set `WriteTimeout: 60s`. A batch that starts writing its response at t=59s will be hard-killed at t=30s of the shutdown window. **Fix**: Raise both defaults to `65` in `.env.example` and in the Viper `SetDefault` calls in `services/ingestion-api/internal/config/config.go:62` and `services/bff-api/internal/config/config.go:65`.
* [x] **Close MinIO `get_object()` response in processor.** `services/analysis-worker/internal/processor.py:84–85`: `response = self.minio.get_object(self.bronze_bucket, obj_key)` followed by `response.read()`, but `response.close()` is never called. Same pattern at line 251–252 in `quarantine_poison_message`. Under sustained throughput, leaked HTTP connections accumulate toward the OS file-descriptor limit. **Fix**: Wrap both call sites in `try/finally` with `response.close()`.
* [x] **Close ClickHouse and PostgreSQL pools on worker shutdown.** `services/analysis-worker/main.py:300–314`: the `finally` block drains NATS and awaits worker tasks but never closes the `ClickHousePool` or the `ThreadedConnectionPool`. Connections remain open until the container is killed. **Fix**: Add a `close_all()` method to `ClickHousePool` that drains the queue and calls `client.close()` on each client. Call `pg_pool.closeall()` and `ch_pool.close_all()` in the `finally` block.
* [x] **Guard `utcoffset()` return value in temporal extractor.** `services/analysis-worker/internal/extractors/temporal.py:32`: `ts.utcoffset().total_seconds() != 0` — Python's `datetime` spec allows `utcoffset()` to return `None` even when `tzinfo` is not `None` (e.g. abstract `tzinfo` subclass). If that happens, `.total_seconds()` raises `AttributeError`. Risk is low (SilverCore enforces timezone-aware timestamps), but it is a latent crash path in a defense-in-depth guard. **Fix**: `if ts.tzinfo is None or ts.utcoffset() is None or ts.utcoffset().total_seconds() != 0:`.
* [x] **Parameterize bucket name in MinIO client init.** `services/ingestion-api/internal/storage/minio.go:37`: `client.BucketExists(cancelCtx, "bronze")` hardcodes the bucket name, ignoring the configurable `cfg.BronzeBucket` (`INGESTION_BRONZE_BUCKET`). If the env var were changed, the boot health check would fail looking for `"bronze"` while actual uploads go to the configured bucket. **Fix**: Add a `bucketName string` parameter to `NewMinioClient` and use it in the existence check.
* [x] **Tests.** Python: unit test that `ClickHousePool.close_all()` drains all clients. Unit test that `response.close()` is called after `get_object()` in the processor (mock `get_object` returning a mock response, assert `close()` called). Go: ingestion-api test that `NewMinioClient` checks the configured bucket name, not a hardcoded one.
* [x] **Validate.** `make lint`, `make test-python`, `make test-go`, `make test-e2e`.

## Phase 90: Worker Boot-Time Credential Validation & Crawler Input Safety [P1] - [x] DONE

*The Go services (ingestion-api, bff-api) refuse to boot on empty credentials since Phases 75/82. The Python worker does not — it falls through to hardcoded defaults or empty strings and fails only at first use. The RSS crawler has a URL-encoding gap in the source-lookup request.*

* [x] **Remove hardcoded default password in Python worker.** `services/analysis-worker/internal/storage/postgres_client.py:51`: `password=os.getenv("POSTGRES_PASSWORD", "aer_secret")`. If the env var is unset, the worker silently authenticates with a weak default instead of failing fast. **Fix**: Remove the `"aer_secret"` default. Add a boot-time validation function (called from `main.py` before `init_postgres`) that checks `POSTGRES_PASSWORD`, `MINIO_ACCESS_KEY`, `MINIO_SECRET_KEY`, and `CLICKHOUSE_PASSWORD` are non-empty and raises `SystemExit` with a clear message if any is missing. This mirrors the Go pattern (`config.Load()` returns `fmt.Errorf`).
* [x] **Validate MinIO credentials at worker startup.** `services/analysis-worker/internal/storage/minio_client.py:31–32`: `access_key=os.getenv("MINIO_ACCESS_KEY", "")`, `secret_key=os.getenv("MINIO_SECRET_KEY", "")`. The service boots and fails only when the first MinIO operation is attempted. **Fix**: Covered by the centralized boot-time validation above.
* [x] **Validate ClickHouse password at worker startup.** `services/analysis-worker/internal/storage/clickhouse_client.py:34`: `password=os.getenv("CLICKHOUSE_PASSWORD", "")`. Same pattern. **Fix**: Covered by the centralized boot-time validation above.
* [x] **URL-encode feed name in RSS crawler source lookup.** `crawlers/rss-crawler/main.go:205`: `sourcesURL+"?name="+name` concatenates the feed name directly into the URL without `url.QueryEscape`. Feed names containing spaces, `&`, `#`, or `?` corrupt or hijack the query string. **Fix**: `sourcesURL + "?name=" + url.QueryEscape(name)`. Add `"net/url"` import.
* [x] **Tests.** Python: unit test that `validate_required_env()` (or equivalent) raises `SystemExit` when `POSTGRES_PASSWORD` is empty. Go crawler: unit test for `resolveSourceID` with a feed name containing special characters (e.g. `"Süddeutsche Zeitung & More"`), asserting the request URL is properly encoded.
* [x] **Validate.** `make lint`, `make test-python`, `make test-go-crawlers`, `make test-e2e`.

## Phase 91: Worker Resilience — Timeouts, Thread Safety & Partial-Failure Handling [P2] - [x] DONE

*Four resilience gaps in the analysis worker that can cause deadlocks, thread-safety violations, or wasteful reprocessing under stress.*

* [x] **Add timeout to ClickHouse pool `getconn()`.** `services/analysis-worker/internal/storage/clickhouse_client.py:39`: `self._pool.get()` blocks indefinitely. If a ClickHouse query hangs (network partition, overloaded server), all subsequent workers queue behind the pool and the entire pipeline deadlocks silently. **Fix**: `self._pool.get(timeout=30)`. Catch `queue.Empty` and raise a descriptive `TimeoutError`. Make the timeout configurable via `CLICKHOUSE_POOL_TIMEOUT_SECONDS` (default 30).
* [x] **Add PostgreSQL query timeout in worker.** `services/analysis-worker/internal/storage/postgres_client.py:96–118`: all queries execute without `statement_timeout`. A slow or deadlocked Postgres query blocks the worker thread indefinitely. Since workers use `asyncio.to_thread`, the thread is consumed but the async loop is not blocked — however the worker slot is lost for the duration. **Fix**: Pass `options="-c statement_timeout=5000"` in the `ThreadedConnectionPool` constructor, or add a configurable `WORKER_PG_STATEMENT_TIMEOUT_MS` (default 5000) that is set on each connection checkout.
* [x] **Add threading lock to RssAdapter classification cache.** `services/analysis-worker/internal/adapters/rss.py:55–68`: the `OrderedDict` (`_classification_cache`) is read and mutated (`move_to_end`, `popitem`, `__setitem__`) by multiple worker threads (via `asyncio.to_thread` in `main.py:164`). `OrderedDict` mutations are not atomic; concurrent access can corrupt the dict or lose cache entries. **Fix**: Add a `threading.Lock` to `RssAdapter.__init__` and acquire it around all cache operations in `_get_classification_cached()`.
* [x] **Wrap Gold-layer inserts in try/except for partial-failure handling.** `services/analysis-worker/internal/processor.py:150–193`: three separate `self.ch.insert()` calls (metrics, entities, language_detections) with no error handling. If the metrics insert succeeds but the entities insert fails, the exception propagates, the message is NAK'd, and redelivered. On redeliver, status is not `"processed"` (line 203 was never reached), so the idempotency guard at line 75 does not fire — all extractions and inserts run again. ReplacingMergeTree collapses the duplicate metrics rows eventually, but the redeliver cycle is wasteful and generates log noise. **Fix**: Wrap the three insert blocks (lines 150–200) in a single `try/except`. On `Exception`, log the error with the `obj_key` and the insert that failed, then continue to mark the document as `"processed"` — the successfully inserted rows are correct, and the extractor pipeline already degrades gracefully per extractor. Alternatively, if partial Gold data is unacceptable, route to quarantine on insert failure.
* [x] **Tests.** Unit test for `ClickHousePool.getconn()` timeout: mock a pool with zero available clients, assert `TimeoutError` is raised within the configured window. Unit test for `RssAdapter` cache under concurrent access: spawn N threads calling `_get_classification_cached()` simultaneously, assert no `RuntimeError` or lost entries. Unit test for processor partial insert failure: mock `ch.insert` to raise on the second call, assert the document is still marked `"processed"` (or quarantined, depending on chosen strategy).
* [x] **Validate.** `make lint`, `make test-python`, `make test-e2e`.

## Phase 92: PostgreSQL Schema Integrity [P3] - [x] DONE

*Two missing constraints in the original Postgres schema that can cause slow queries and silent data duplicates.*

* [x] **Add index on `documents.job_id`.** `infra/postgres/migrations/000001_initial_schema.up.sql:21`: `job_id INTEGER REFERENCES ingestion_jobs(id)` has no index. Migration 004 added `idx_documents_ingested_at` for retention cleanup, but queries joining documents by `job_id` (e.g. checking all documents for a job in the ingestion service) still trigger sequential scans as the table grows. **Fix**: New migration `infra/postgres/migrations/000009_add_documents_job_id_index.up.sql`: `CREATE INDEX IF NOT EXISTS idx_documents_job_id ON documents(job_id);`. Corresponding down migration: `DROP INDEX IF EXISTS idx_documents_job_id;`.
* [x] **Add UNIQUE constraint on `sources.url`.** `infra/postgres/migrations/000001_initial_schema.up.sql:7`: `url VARCHAR(500) NOT NULL` allows duplicate URLs. Without a unique constraint, two crawlers registering the same source URL create silent duplicates that fragment metrics by `source_id`. **Fix**: New migration `infra/postgres/migrations/000010_add_sources_url_unique.up.sql`: `CREATE UNIQUE INDEX IF NOT EXISTS idx_sources_url_unique ON sources(url);`. Down migration: `DROP INDEX IF EXISTS idx_sources_url_unique;`. Before applying, verify no existing duplicates with `SELECT url, COUNT(*) FROM sources GROUP BY url HAVING COUNT(*) > 1`.
* [x] **Validate.** `make test-go` (Testcontainers run all migrations), `make test-e2e`.

## Phase 93: Doc Sweep (Post Phases 89–92) [P-Docs] - [x] DONE

*Documentation housekeeping after the code phases. Must run after Phase 92 so all code changes are final.*

* [x] **Fix stale ADR-014 reference in Chapter 13.** `docs/arc42/13_scientific_foundations.md:171` states *"The introduction of Tier 3 methods requires a formal ADR (to be filed as ADR-014)"* — but ADR-014 already exists as "Database Migration Strategy" (Chapter 9). **Fix**: Update the parenthetical to reference the next available ADR number, or remove it and state that a Tier 3 ADR will be filed when interdisciplinary validation begins.
* [x] **Document ClickHouse migration 000010 in Arc42.** Migration `infra/clickhouse/migrations/000010_replacing_merge_tree.sql` (ReplacingMergeTree conversion for idempotent NATS redelivery) exists but is not referenced in the Arc42 cross-cutting concepts. **Fix**: Add a brief reference in §8.8 (Data Lifecycle) or §8.16 (Analysis Worker Resilience) noting the schema change and its rationale.
* [x] **CLAUDE.md update.** Reflect any new env vars or validation behavior introduced by Phases 89–91 in the "Security Defaults" section (boot-time secret validation list, new timeout vars).
* [x] **Validate.** `mkdocs build --strict` green, `make lint` green. Grep for stale phase references.

---

**Review note (Phases 89–93):** Five findings are P0 (correctness/safety) — the shutdown timeout arithmetic is the most load-bearing since it can truncate in-flight batches on every deploy. P1 establishes boot-time credential validation symmetry between Go and Python services plus a URL-encoding fix in the crawler. P2 addresses four resilience paths that can cause deadlocks or wasteful reprocessing under stress. P3 is schema polish. Recommended order: 89 → 90 (correctness base), 91 (resilience), 92 (schema), 93 (docs last, after code changes settle). Phase 93 must run after 89–92 so the doc sweep captures all changes.


## Phase 94: Frontend Foundation — Documentation Integration & Arc42 Anchoring [P-Docs] - [x] DONE

*ADR-020 (Frontend Technology Stack) has been ratified. The Design Brief exists under `docs/design/design_brief.md`. Before any frontend code is written, the documentation must be fully integrated into the Arc42 structure, the MkDocs navigation, and the cross-reference graph. This phase is docs-only — zero code changes to services. It establishes the epistemic foundation that the subsequent code phases refer back to.*

* [x] **MkDocs navigation extension.** Add the Design Brief and visualization guidelines to `mkdocs.yml` under a new top-level tab `Design` (peer to `Architecture (arc42)`, `Methodology`, `Operations`). Structure:
  ```yaml
  - "Design":
      - "Design Brief": design/design_brief.md
      - "Visualization Guidelines": design/visualization_guidelines.md
  ```
  Verify with `mkdocs build --strict` (zero warnings allowed). Verify live-reload picks up the new files in the `docs` container.

* [x] **Arc42 §8.17 — Frontend Architecture (Cross-cutting Concept).** Add a new section `8.17 Frontend Architecture` to `docs/arc42/08_concepts.md`. Content (short, ~40–60 lines): one-paragraph description of the dashboard as a service, the three surfaces (Atmosphere / Function Lanes / Reflection), the five layers of progressive descent, the four visualization domains (§5.9), and explicit cross-references to the Design Brief and ADR-020. This section is the *bridge* from Arc42 readers to the full Design Brief — it must be brief and direct, not a summary of the brief. Link form: `See [Design Brief](../design/design_brief.md) for the full architecture.`

* [x] **Arc42 §5 — Building Block View update.** Extend `docs/arc42/05_building_block_view.md` to include the `dashboard` service as a new building block alongside `ingestion-api`, `bff-api`, and `analysis-worker`. Single row in the existing service table: responsibility ("User-facing dashboard serving three surfaces"), language (TypeScript/Svelte), dependencies (BFF API only), position in the container network (`aer-frontend` only).

* [x] **Arc42 §7 — Deployment View placeholder.** Extend `docs/arc42/07_deployment_view.md` §7.4 with a placeholder entry for the `dashboard` service in the service table — image base, port, network, memory, CPU. Values marked as `TBD (Phase 97)` where the Dockerfile does not yet exist. This ensures §7 is consistent with the intended deployment even before Phase 97 implements it.

* [x] **Arc42 §2 — Architecture Constraints extension.** Add a row to the Technical Constraints table in `docs/arc42/02_architecture_constraints.md` §2.2: "Frontend Stack — TypeScript 5.5+, Svelte 5, SvelteKit static adapter. Enforced by ADR-020, `package.json` engines field, CI workflow."

* [x] **Arc42 §9 — ADR-020 is already inserted.** Verify that ADR-020 reads consistently after ADR-019, with the same formatting conventions (Status / Context / Decision / Consequences). Check that all cross-references (ADR-003, ADR-008, ADR-016, ADR-017) resolve to the correct in-document anchors.

* [x] **Arc42 §12 — Glossary additions.** Add six new glossary entries to `docs/arc42/12_glossary.md`:
  - **Dashboard** — The AĒR user-facing application. A static SvelteKit build serving three surfaces (Atmosphere, Function Lanes, Reflection). Deployed behind Traefik on `aer-frontend` network. See ADR-020 and the Design Brief.
  - **Surface (Dashboard)** — One of three top-level encounter modes: Atmosphere (the 3D globe), Function Lanes (discourse functions from WP-001), Reflection (methodological prose). Surfaces are orthogonal to Layers. See Design Brief §3.
  - **Layer (Progressive Descent)** — One of five depths at which any surface can be viewed: Immersion (L0), Orientation (L1), Exploration (L2), Analysis (L3), Provenance (L4), Evidence (L5). Layers are orthogonal to Surfaces. See Design Brief §4.
  - **Progressive Semantics** — The Dual-Register communication pattern: every data point carries both a semantic and a methodological register in the DOM, with only one prominent at a time. See Design Brief §5.7.
  - **Epistemic Weight** — The visual prominence of a rendered metric scales with its methodological backing (Tier 1 unvalidated → moderate weight; Tier 2 validated → full weight; Tier 3 LLM-augmented → distinct styling). See Design Brief §5.8.
  - **Content Catalog** — The BFF subsystem serving Dual-Register content (semantic + methodological text for metrics, probes, discourse functions, and refusal types) from YAML sources under `services/bff-api/configs/content/`. New in Phase 95. See ADR-020 Layer 4.

* [x] **Cross-reference audit.** `grep -r "docs/design/" docs/` and `grep -r "ADR-020" docs/` — verify that every reference resolves and that the Design Brief is cited from at least `arc42/08_concepts.md §8.17`, `arc42/09_architecture_decisions.md` (ADR-020), `arc42/05_building_block_view.md`, and `arc42/12_glossary.md`.

* [x] **Update `docs/index.md`.** Add a fourth pillar under "Documentation Structure": "**Design** — The design language of the AĒR dashboard. The Design Brief defines visual identity, interaction architecture, and extensibility commitments; the Visualization Guidelines constrain individual rendering decisions." One paragraph, in the same tone as the existing three pillars.

* [x] **Update `CLAUDE.md`.** Update CLAUDE.md so it reflects the current repository with the new Frontend plans if necessary or check if it is up to date.

* [x] **Validation.** `mkdocs build --strict` green, `make lint` green (any future Python/Go linting still passes — no code changes in this phase), pre-push hooks green.

**Exit criteria:** The Design Brief is discoverable from the MkDocs landing page in two clicks. ADR-020 is findable both via Arc42 §9 and from the Design section's cross-links. `arc42/08_concepts.md` has a §8.17 entry that connects Arc42 readers to the Design Brief without requiring them to read the full brief first. Every glossary term used anywhere in the new docs is defined in §12. Any document inside the docs/ directory is visible via `mkdocs`. CLAUDE.md is ont the most recent state.


## Phase 95: Content Catalog — BFF Backend for Dual-Register Content [P1] - [x] Done

*The Dual-Register content required by Design Brief §5.7 lives outside the frontend. The BFF serves both semantic and methodological register text for every metric, probe, discourse function, and refusal type — from YAML source files under `services/bff-api/configs/content/`. This phase implements the backend side only; the frontend integrates it in Phase 97+. Content authoring happens as a separate scientific workflow, not code.*

* [x] **Content directory structure.** Create `services/bff-api/configs/content/` with subdirectories:
  ```
  configs/content/
  ├── en/
  │   ├── metrics/
  │   │   ├── sentiment_score.yaml
  │   │   ├── word_count.yaml
  │   │   ├── temporal_distribution.yaml
  │   │   ├── language_confidence.yaml
  │   │   └── entity_count.yaml
  │   ├── probes/
  │   │   └── probe-0-de-institutional-rss.yaml
  │   ├── discourse_functions/
  │   │   ├── epistemic_authority.yaml
  │   │   ├── power_legitimation.yaml
  │   │   ├── cohesion_identity.yaml
  │   │   └── subversion_friction.yaml
  │   └── refusals/
  │       ├── normalization_equivalence_missing.yaml
  │       ├── validation_missing.yaml
  │       └── k_anonymity_threshold_not_met.yaml
  └── de/
      └── (same structure, German content)
  ```

* [x] **Content schema (Pydantic / Go struct).** Define the content record schema for each entity type. Both registers must exist; both have a `short` (≤ 200 characters, for hover/badge surfaces) and `long` variant (prose, ≤ 2000 characters, for Layer 4). Required metadata: `entityId`, `entityType`, `locale`, `contentVersion` (semver-like string), `lastReviewedBy`, `lastReviewedDate` (ISO 8601), optional `workingPaperAnchors` (list of strings like `"WP-002 §3"`). Shape:
  ```yaml
  entityId: sentiment_score
  entityType: metric
  locale: en
  registers:
    semantic:
      short: "A score from −1 to +1 that reflects..."
      long: "Multiple paragraphs..."
    methodological:
      short: "Tier 1 lexicon-based score (SentiWS v2.0). Unvalidated."
      long: "Multiple paragraphs with WP-002 citations..."
  contentVersion: "v2026-04-a"
  lastReviewedBy: "Fabian Quist"
  lastReviewedDate: "2026-04-17"
  workingPaperAnchors:
    - "WP-002 §3"
    - "WP-002 §4"
  ```

* [x] **Seed content for Phase 42 metrics (EN + DE).** Draft the initial semantic and methodological registers for all five current metrics (`word_count`, `sentiment_score`, `language_confidence`, `temporal_distribution`, `entity_count`). Draft from WP-002 §3 (known limitations), `services/bff-api/configs/metric_provenance.yaml` (existing methodology), and the Working Papers. EN drafts first; DE translations in a single commit after EN review passes. Both registers need to be pedagogically honest — the semantic register explains *what* without dumbing down; the methodological register explains *how* without assuming familiarity with the pipeline.

* [x] **Seed content for Probe 0 (EN + DE).** Draft the probe dossier content for `probe-0-de-institutional-rss`. Pull from existing `docs/probes/probe-0-de-institutional-rss/README.md` and related dossier files. This content is consumed by Surface III (Reflection) when the probe is inspected.

* [x] **Seed content for four discourse functions (EN + DE).** Draft semantic and methodological registers for Epistemic Authority, Power Legitimation, Cohesion & Identity, and Subversion & Friction. Source: WP-001 §3. These content entries populate the empty-lane captions on Surface II (Design Brief §3.2, §5.7 example).

* [x] **Seed content for refusal types (EN + DE).** Three refusals must be authored first: (1) `normalization_equivalence_missing` (HTTP 400 from z-score gate), (2) `validation_missing` (metric accessed at a confidence claim above its validation tier), (3) `k_anonymity_threshold_not_met` (L5 descent refused due to WP-006 §7). Follow the refusal pattern from Design Brief §5.4 + §5.7: semantic = plain-language explanation, methodological = exact gate + WP anchor + alternatives.

* [x] **BFF content loader.** Extend the BFF API's configuration package to load `configs/content/**/*.yaml` at startup into an in-memory read-through cache. The loader validates each file against the content schema; malformed files abort startup with a clear error. Files are hot-reloadable in the `docs` dev container via an fs-watcher (optional; defer if adds complexity).

* [x] **OpenAPI contract extension.** Add to `services/bff-api/api/openapi.yaml`:
  ```yaml
  /content/{entityType}/{entityId}:
    get:
      summary: Get Dual-Register content for an entity
      parameters:
        - name: entityType
          in: path
          required: true
          schema:
            type: string
            enum: [metric, probe, discourse_function, refusal]
        - name: entityId
          in: path
          required: true
          schema:
            type: string
        - name: locale
          in: query
          required: false
          schema:
            type: string
            enum: [en, de]
            default: en
      responses:
        '200':
          description: Content found
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ContentResponse'
        '404':
          description: Content not found for the given entity or locale
        '400':
          description: Invalid entityType or entityId
  ```
  Define the `ContentResponse` schema matching the YAML record shape.

* [x] **BFF handler implementation.** Implement `GET /api/v1/content/{entityType}/{entityId}` in `services/bff-api/internal/handler/`. Dispatch to the content loader; return 404 if no matching entry for the requested locale (with a `Content-Language` header). Include trace instrumentation consistent with existing BFF endpoints.

* [x] **Content contract test.** Add tests in `services/bff-api/internal/handler/` covering: (a) successful fetch for each entity type, (b) 404 on missing entity, (c) 404 on missing locale, (d) invalid entity type → 400. Tests use fixture content files under a test configs directory.

* [x] **E2E smoke test extension.** Extend `scripts/e2e_smoke_test.sh` with assertions against the new content endpoints for at least one metric (sentiment_score), one probe (probe-0-de-institutional-rss), one discourse function (epistemic_authority), and one refusal (normalization_equivalence_missing). Both locales (en, de) verified.

* [x] **Codegen regeneration.** `make codegen` to regenerate the Go server stubs from the updated OpenAPI. Verify that the CI drift check (`git diff --exit-code`) passes.

* [x] **Arc42 documentation.** Add §8.18 to `docs/arc42/08_concepts.md` describing the Content Catalog subsystem — storage format, API shape, i18n model, versioning, and the relationship to the Dual-Register content in Design Brief §5.7. Cross-reference in §12 Glossary ("Content Catalog" entry from Phase 94 gets a §8.18 link appended).

* [x] **Validation.** `make lint` green, `make test` green, `make codegen` clean, `scripts/e2e_smoke_test.sh` green, `mkdocs build --strict` green.

**Exit criteria:** `curl -H 'X-API-Key: ...' http://localhost:8080/api/v1/content/metric/sentiment_score?locale=de` returns a well-formed JSON response with both registers and content metadata. All current Phase 42 metrics, Probe 0, four discourse functions, and three refusal types are authored in EN and DE.


## Phase 96: OpenAPI Contract Consolidation — Consistency, Coverage & Developer Tooling [P1] - [x] DONE

*A code review of the service contracts surfaced three gaps before the frontend work in Phase 97 consumes them: no browsable API reference, inconsistent `$ref` styles in the modularized BFF spec, and no OpenAPI contract for the ingestion API. This phase closes all three with a principled two-style `$ref` convention (dictated by JSON Reference semantics), a custom bundler, and a Swagger UI entry point. No runtime behavior changes.*

**Principled two-style `$ref` convention.** Investigation revealed the initial "normalize to external-file refs everywhere" plan was technically impossible without regressing `oapi-codegen` output. The convention that actually holds is:

1. **Path-level refs to top-level components** MAY use `#/components/schemas/X`. This is the only way to produce named Go types via `kin-openapi`'s path-item flattening; switching to `../schemas/X.yaml` collapses the schema to an anonymous inline type. These refs are safe because `kin-openapi` resolves them against the root document *after* inlining the path-item.
2. **All other refs** (schema→schema, response→schema, and any ref *inside* an external file) MUST use external-file refs (`../schemas/X.yaml`). JSON Reference's `#` means "current document", so `#/components/...` inside an external file is unresolvable under strict bundlers (`redocly`, `swagger-cli`).

A CI lint gate enforces rule 2; rule 1 is checked implicitly by `make codegen` drift detection.

* [x] **Ingestion API OpenAPI contract — decision.** Added `services/ingestion-api/api/openapi.yaml` modeled after the BFF layout. Documents `/ingest`, `/sources`, `/healthz`, `/readyz` with `X-API-Key` security and 400/401/404/413/500 error shapes. Codegen emits types-only into `internal/apicontract/` (separate package — avoids colliding with the existing hand-written `handler.Handler` type during the transitional period; full `strict-server` migration can happen in a later phase without re-opening the contract). ADR-021 records the contract-first posture for all HTTP services.

* [x] **`$ref` style convention — two-style, enforced.** BFF spec retains the four legitimate path-level `#/components/...` refs (in `sources.yaml`, `content.yaml`, `metrics_provenance.yaml`, `metrics_available.yaml`) to keep named-type generation. All refs inside external files use `../schemas/X.yaml` form. Zero drift on `services/bff-api/internal/handler/generated.go`.

* [x] **Lint gate for `$ref` style.** `scripts/openapi_ref_style_check.sh` fails if any `*.yaml` under `services/*/api/{schemas,parameters,responses}/` contains a `$ref: '#/...'`. Path files are intentionally exempt (path-level `#/components/...` is the sanctioned form). Wired into `make lint` via the new `openapi-lint` target.

* [x] **Bundled spec artifact.** `scripts/openapi_bundle.py` produces a single-file bundle per service by emulating `kin-openapi`'s path-item flattening — it inlines path files first (so their `#/components/...` refs become valid against the root), then resolves external-file refs. `redocly bundle` could not be used because its strict JSON Reference implementation rejects the path-level `#/components/...` form this repo relies on for named Go types. Bundles are gitignored and rebuilt via `make openapi-bundle`. No new developer-tool dependency: Python + PyYAML (already present via the analysis worker).

* [x] **Swagger UI service.** Added a `swagger-ui` container (`swaggerapi/swagger-ui:v5.17.14`) to `compose.yaml` under the `dev` compose profile so it is absent by default in production (started via `docker compose --profile dev up swagger-ui`). Bound to `127.0.0.1:8089` (loopback-only), mounts both `openapi.bundle.yaml` files, exposes a multi-spec dropdown.

* [x] **Codegen targets.** `make codegen` regenerates both services (BFF → `internal/handler/generated.go`, ingestion → `internal/apicontract/generated.go`) directly from the modular `api/openapi.yaml` sources. Codegen does **not** depend on the bundled artifact — `kin-openapi` resolves the modular tree correctly via path-item flattening. The bundle is for Swagger UI and future TS client codegen.

* [x] **ADR-021 + docs.** ADR-021 ("Contract-first for all HTTP services") added to `docs/arc42/09_architecture_decisions.md`; §8.19 ("API Contract Layout & Tooling") added to `docs/arc42/08_concepts.md` documenting the two-style ref convention, bundler, Swagger UI, and CI checks; "OpenAPI Bundle", "Swagger UI", and "Two-Style `$ref` Convention" entries added to `docs/arc42/12_glossary.md`.

* [x] **Operations playbook + Arc42 cross-refs.** "API Contract Reference (Swagger UI)" section added to `docs/operations_playbook.md` with start/stop/refresh instructions, editing workflow, and troubleshooting table. BFF and Ingestion API entries in `docs/arc42/05_building_block_view.md` now cross-reference the contract, §8.19, and the Operations Playbook.

* [x] **CI integration.** `.github/workflows/ci.yml` (go-pipeline job) runs `make openapi-lint` (ref-style gate), `make codegen && git diff` drift check covering both `services/bff-api/internal/handler/generated.go` and `services/ingestion-api/internal/apicontract/generated.go`, and `make openapi-bundle` to catch structural breaks before they reach Swagger UI.

* [x] **Validation.** `make lint` green (incl. new `openapi-lint`), `make test` green (Go integration tests + 139 Python unit tests), `make test-e2e` green (19/19 assertions, full pipeline), `make codegen` clean for both services, `make openapi-bundle` produces both bundles, `mkdocs build --strict` green. Swagger UI loads both specs from bundles under the `dev` compose profile.

**Exit criteria:** A developer can run `docker compose --profile dev up swagger-ui` and browse both APIs interactively at `http://localhost:8089`. Every external YAML file (schemas, responses, parameters) uses external-file refs only; path-level `#/components/...` refs are retained where kin-openapi requires them for named Go types. ADR-021 captures the contract-first posture. CI fails any future PR that reintroduces an in-document ref inside a schema/parameter/response file, or drifts the generated code from the spec.


## Phase 96a: Query-Parameter Enum Validation Gate [P2] - [x] DONE

*Der BFF-Handler prüft `?normalization` und `?resolution` aktuell nur auf
Exaktmatch gegen den Happy-Path-Enum. Jeder andere Wert fällt still auf den
Default zurück statt `400` zu liefern. Die generierten `Valid()`-Methoden
existieren in `generated.go`, werden aber nicht aufgerufen. Für das
kommende Frontend-Debugging macht stiller Fallback Fehlerdiagnose
unnötig teuer.*

* [x] **`GetMetrics`: explicit enum gate.** In `handler.go` vor der
  `useZscore`-Ableitung: `if request.Params.Normalization != nil &&
  !request.Params.Normalization.Valid()` → `GetMetrics400JSONResponse`
  mit klarem Message (`"invalid normalization; must be one of raw, zscore"`).
  Analog für `Resolution`.
* [x] **Audit der übrigen Enum-Parameter.** `GetContent` prüft bereits
  via `request.EntityType.Valid()`. `GetEntities.Label`, `GetLanguages.Language`
  sind freie Strings laut OpenAPI (keine Enums) — OK. Keine weiteren Stellen.
* [x] **Tests.** Jeweils einen Fall pro Parameter mit Garbage-Wert →
  assert 400 mit dem spezifischen Message-Prefix.
* [x] **Validate.** `make test`, `make codegen && git diff --exit-code`.

**Exit criteria:** Ein `curl .../metrics?normalization=zcore` (Typo) liefert
`400` mit deskriptiver Message statt `200` mit Raw-Daten.


## Phase 96b: ADR-020 & ADR-021 Ratification Hygiene [P-Docs] - [x] DONE

*ADR-020 (Frontend Stack) und ADR-021 (Contract-first) tragen noch
"pending author review"-Marker obwohl die Phasen 94–96 bereits auf
beiden aufgebaut haben. Vor Phase 97 ratifizieren, damit der FE-Code
gegen ein formell beschlossenes Stack-Commitment geschrieben wird.*

* [x] **ADR-020 Decision Record.** `Ratified: [date TBD]` → konkretes
  Datum, Status auf `Accepted` heben.
* [x] **ADR-021 Decision Record.** "ratification pending user/author
  review" → konkretes Ratifizierungsdatum.
* [x] **Cross-Ref-Check.** `grep -r "TBD" docs/arc42/09_architecture_decisions.md`
  muss leer sein (ausser ausdrücklich geplanten Tier-3-Defers).
* [x] **Validate.** `mkdocs build --strict`.


## Phase 96c: Ingestion API strict-server Konvergenz [P2] - [x] DONE

*ADR-021 hat bewusst die `types`-only-Codegen-Strategie für ingestion-api
gewählt um Scope-Creep zu vermeiden. Der hand-written Handler kann vom
Contract driften; aktuell fangen nur Integration-Tests das ab. Vor Phase 97
ist die FE-Seite des Contracts (`openapi-typescript`) der BFF zugewandt,
daher hat diese Phase niedrigere Dringlichkeit — aber sie schliesst die
letzte Asymmetrie in ADR-021.*

* [x] **codegen-Konfiguration.** `services/ingestion-api/api/codegen.yaml`
  von `types`-only auf `strict-server + types + chi-server` umstellen
  (analog BFF). Output: `internal/handler/generated.go`.
* [x] **Handler-Migration.** Den bestehenden `Handler.Ingest` auf das
  `StrictServerInterface`-Pattern heben (Request/Response-Objects
  statt `http.ResponseWriter`/`*http.Request`). Existierende Tests
  bleiben fast unverändert — der HTTP-Router-Eintrittspunkt ist der
  einzige Unterschied.
* [x] **Router-Wiring.** `cmd/api/main.go`: `handler.HandlerFromMuxWithBaseURL(
  handler.NewStrictHandler(serverLogic, nil), r, "/api/v1")` statt
  direktem `r.Post("/ingest", h.Ingest)`.
* [x] **Drift-Check.** Nach Migration muss `make codegen && git diff
  --exit-code` ingestion-api-generated-Datei nicht mehr rot.
* [x] **Tests.** Existierende Handler-Tests laufen gegen die neue
  Interface-Form. Contract-Drift ist ein Compile-Error (Handler-Methode
  fehlt im Interface).
* [x] **ADR-021 Update.** Den "Ingestion handler is not yet generated
  from its contract"-Satz in Consequences auf "Resolved in Phase 96c"
  gesetzt, Non-Goal-Abschnitt entsprechend aktualisiert.
* [x] **Validate.** `make test` passes, `make codegen` clean.

**Exit criteria:** Beide HTTP-Services nutzen `strict-server`. Der
Contract-Drift-Check in CI deckt beide generierten Dateien byte-genau.

---

# Open Phases

## Phase 97: Frontend Scaffolding — SvelteKit Static + Infrastructure Integration [P0] - [ ] TODO

*This phase creates `services/dashboard/` as a new service in the monorepo. It produces a minimal "Hello AĒR" page that renders through Traefik, emits OTel traces into the existing collector, and passes the same CI/supply-chain rigor as the Go services. No user-facing features yet — this is purely the foundation on which all subsequent frontend work stands.*

* [ ] **Service directory structure.** Create `services/dashboard/` with the following structure:
  ```
  services/dashboard/
  ├── src/
  │   ├── lib/              # Svelte components (empty initially)
  │   ├── routes/
  │   │   ├── +layout.svelte
  │   │   └── +page.svelte  # "Hello AĒR" landing
  │   ├── app.html
  │   └── app.d.ts
  ├── static/
  │   └── favicon.svg
  ├── tests/
  │   ├── unit/             # Vitest
  │   └── e2e/              # Playwright (single smoke test)
  ├── package.json
  ├── pnpm-lock.yaml
  ├── svelte.config.js
  ├── vite.config.ts
  ├── tsconfig.json
  ├── .npmrc
  ├── .gitignore
  ├── Dockerfile
  └── README.md
  ```

* [ ] **Svelte / SvelteKit configuration.** Install Svelte 5, SvelteKit with `@sveltejs/adapter-static`, TypeScript 5.5+. `svelte.config.js` configures the static adapter with `prerender: true` and fallback `index.html` for SPA routing. `tsconfig.json` has strict mode enabled (`"strict": true`, `"noUncheckedIndexedAccess": true`, `"exactOptionalPropertyTypes": true`).

* [ ] **pnpm and lockfile.** pnpm 9+ as the package manager. `pnpm install --frozen-lockfile` enforced in CI. `.npmrc` with `strict-peer-dependencies=true`. Pin all dev dependencies to exact versions — no caret ranges in `package.json`.

* [ ] **TypeScript API client codegen.** Install `openapi-typescript`. Add `make codegen-ts` target to the root `Makefile` that runs `openapi-typescript services/bff-api/api/openapi.yaml -o services/dashboard/src/lib/api/types.ts`. Add a `.tool-versions` entry for `openapi-typescript`. CI workflow step: `make codegen-ts && git diff --exit-code` — mirror of the existing Go drift check.

* [ ] **ESLint + Prettier.** ESLint with `@typescript-eslint` and the official Svelte plugin. Prettier with the Svelte plugin. Same formatting conventions as the Go/Python code where applicable (2-space indentation for TS/Svelte, LF line endings, single quotes for strings).

* [ ] **Makefile integration.** Add to the root Makefile:
  ```make
  fe-install:    ## Install frontend dependencies
  fe-lint:       ## Lint frontend (ESLint + Prettier check + svelte-check)
  fe-typecheck:  ## TypeScript strict typecheck
  fe-test:       ## Vitest unit tests
  fe-test-e2e:   ## Playwright end-to-end smoke test
  fe-build:      ## Production build
  fe-check:      ## Composite: lint + typecheck + test + bundle-size gate
  ```
  Extend the existing `make lint` target to include `fe-lint`. Extend `make test` to include `fe-test`. The existing `make codegen` target remains Go-only; `make codegen-ts` is its frontend peer.

* [ ] **Bundle-size gate.** Add a Vite plugin (`rollup-plugin-visualizer` or equivalent) that emits bundle stats. Add a CI step that fails the build if the initial bundle (shell + router + runtime) exceeds 80 kB gzipped. Budget exists with headroom — the 180 kB total budget (Design Brief §7) is enforced at phase 98 when actual Surface I code lands.

* [ ] **OpenTelemetry Web SDK.** Install `@opentelemetry/sdk-trace-web`, `@opentelemetry/instrumentation-fetch`, `@opentelemetry/exporter-trace-otlp-http`. Configure in a dedicated `src/lib/observability/otel.ts` module. Lazy-load after first paint (do not block initial render). OTLP endpoint configurable via build-time env var; default points at the existing `otel-collector:4318` internal service for development. Resource attributes: `service.name=aer-dashboard`, `service.version` (from `package.json`), `deployment.environment` (build-time env var).

* [ ] **Dockerfile (multi-stage).** Build stage uses `node:22-alpine3.23` with pinned digest; runtime stage uses `nginx:1.27-alpine3.23` with pinned digest. Build output from SvelteKit's static adapter is copied to `/usr/share/nginx/html`. Nginx config serves `index.html` as SPA fallback. Image is pinned via the SSoT pattern from Arc42 §2.3. Build output size gate: refuse to build if image exceeds 50 MB.

* [ ] **Supply-chain hardening (Phase 84 parity).** Image build pipeline produces:
  - Trivy scan (fails on HIGH/CRITICAL vulnerabilities)
  - Cosign signature (keyless via OIDC in CI)
  - Syft SBOM (attached as OCI artifact)
  Extend existing CI workflow steps to cover the dashboard image symmetrically to the Go service images.

* [ ] **`compose.yaml` integration.** Add the `dashboard` service to `compose.yaml`:
  ```yaml
  dashboard:
    build:
      context: ./services/dashboard
      dockerfile: Dockerfile
    image: aer-dashboard:local
    networks:
      - aer-frontend
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.dashboard.rule=PathPrefix(`/`) && !PathPrefix(`/api`)"
      - "traefik.http.routers.dashboard.entrypoints=websecure"
      - "traefik.http.routers.dashboard.tls=true"
      - "traefik.http.services.dashboard.loadbalancer.server.port=80"
    healthcheck:
      test: ["CMD", "wget", "--spider", "-q", "http://localhost/"]
      interval: 30s
      timeout: 3s
      retries: 3
      start_period: 5s
    deploy:
      resources:
        limits:
          memory: 128M
          cpus: "0.25"
  ```
  The dashboard is only on `aer-frontend` — it has no access to backend services except through the BFF API via Traefik. This enforces ADR-008 Zero-Trust symmetrically.

* [ ] **Traefik routing verification.** With `make up`, confirm that `https://localhost/` serves the dashboard and `https://localhost/api/...` still routes to the BFF. Check that the BFF is *not* reachable via the dashboard's routing.

* [ ] **Git hooks extension.** Extend `scripts/hooks/pre-commit` and `scripts/hooks/pre-push` to include `make fe-lint` (pre-commit) and `make fe-check` (pre-push). Hooks only run the frontend steps if `services/dashboard/` files have changed — otherwise skip to preserve the Go-only fast path.

* [ ] **CI workflow extension.** Extend `.github/workflows/ci.yml` with a `frontend` job that runs in parallel to `backend` jobs: install → lint → typecheck → unit test → build → e2e (Playwright) → bundle-size gate → image build → Trivy scan. The job runs on every PR that touches `services/dashboard/` or the OpenAPI spec.

* [ ] **Health & readiness.** The dashboard container responds to `GET /healthz` (just returns 200 from Nginx on a static file) for Docker healthchecks. No dynamic readiness needed — it's a static bundle.

* [ ] **README.md.** Service-level README at `services/dashboard/README.md` with: one-paragraph description, cross-references to ADR-020 and the Design Brief, developer setup commands, and an architecture overview that defers to the central docs. No duplicate content — only pointers.

* [ ] **Arc42 §7 update.** Fill in the `dashboard` service row in §7.4 (Deployment View) that was a placeholder in Phase 94: actual image base, port, network, memory, CPU, healthcheck command.

* [ ] **Validation.** `make up` brings the stack up including the dashboard. `make fe-check` green. `make lint` green. `make test` green. The dashboard loads in a browser via Traefik. OTel traces for dashboard page loads are visible in Grafana Tempo.

**Exit criteria:** A blank but fully instrumented dashboard service runs alongside the existing stack, emits traces, follows the monorepo's supply-chain hardening, and passes the same quality gates as the Go services. Zero user-facing features — but the foundation is production-grade.

---

## Phase 98: Design System Foundation [P1] - [ ] TODO

*Before rendering any real data, the dashboard needs a design system — typography, color tokens, spacing, focus-rings, dark-mode defaults. This phase establishes those primitives and sets up Histoire for isolated component testing. The design system is additive: later phases extend it, but never rewrite it.*

* [ ] **CSS custom properties as design tokens.** Define the AĒR design token set in `src/lib/design/tokens.css`: colors (Viridis-family palette stops, UI grays, accent cyan, semantic colors for validation status), typography scale (Inter Variable UI, IBM Plex Mono for numbers; modular scale with ratios), spacing scale (4px baseline grid), radius scale, elevation/shadow scale, motion scale (durations + easings). All values are CSS custom properties — no Tailwind, no framework-specific tokens. Follows the approach of Distill.pub and other scientific publishing projects.

* [ ] **Dark-mode-first approach.** Design Brief §5.5 mandates dark mode as primary. Tokens provide a `data-theme="dark"` (default) and `data-theme="light"` (fallback). CSS-only switching via custom properties — no JS needed for theme application. System preference honored via `@media (prefers-color-scheme)` at first load.

* [ ] **Typography rendering.** Self-host Inter Variable and IBM Plex Mono via `@fontsource-variable/inter` and `@fontsource/ibm-plex-mono` — no Google Fonts dependency (Zero-Trust compatibility, no third-party requests from the user's browser). Subsetting to Latin-Extended and Greek (for ἀήρ etc.). Font files total ~180 kB compressed — budgeted against the shell's 180 kB total, not the initial paint.

* [ ] **Viridis color scale module.** `src/lib/design/viridis.ts` exports the Viridis-256 and Cividis-256 arrays (colorblind-safe variants) plus a `viridis(t: number): string` interpolator. These are the canonical color scales used by every visualization module per Design Brief §5.2 and `visualization_guidelines.md` §1. No ad-hoc palette definitions anywhere else.

* [ ] **Epistemic Weight styles.** `src/lib/design/epistemic-weight.css` defines the visual treatments from Design Brief §5.8: CSS classes `.weight-tier1-unvalidated`, `.weight-tier1-validated`, `.weight-tier2-validated`, `.weight-tier3`, `.weight-expired`. Each class defines line-weight, opacity, and decoration (dashed, hatched, badge) conventions. Visualization modules consume these classes rather than hardcoding visual treatments.

* [ ] **Accessibility primitives.** Focus-ring tokens with minimum 3:1 contrast against any background. `:focus-visible` for keyboard-only focus. `prefers-reduced-motion` honored by the motion-scale tokens (reduces all motion to < 0.01s when set). Skip-link styling. ARIA label conventions documented in `src/lib/design/a11y.md`.

* [ ] **Base component primitives.** Minimum set of Svelte components in `src/lib/components/base/`:
  - `Button.svelte` (primary, secondary, ghost variants; keyboard-accessible; loading state)
  - `Dialog.svelte` (modal with focus-trap; WCAG 2.2 AA compliant)
  - `Tooltip.svelte` (ARIA-described; positions via CSS anchor-positioning where supported)
  - `Badge.svelte` (status badges consuming Epistemic Weight classes)
  - `SkipLink.svelte` (screen-reader navigation)
  No business logic — these are pure primitives.

* [ ] **Histoire setup.** Install `@histoire/plugin-svelte` — lighter than Storybook, Svelte-native. Configure under `histoire.config.ts`. Every base component has a `.story.svelte` file demonstrating all variants. Runs via `pnpm histoire dev` on a separate port; CI runs `pnpm histoire build` as a smoke check.

* [ ] **Visual regression baseline.** Playwright visual regression configured. Baseline snapshots captured for each base component at both `data-theme="dark"` and `data-theme="light"`. Snapshots committed to the repo. Changes require explicit snapshot approval.

* [ ] **Accessibility baseline testing.** Install `@axe-core/playwright`. E2E test runs axe-core against every base component story and fails on any WCAG 2.2 AA violation. This becomes the permanent a11y gate — every new component must pass before merging.

* [ ] **Design documentation.** Add `docs/design/design_system.md` (new) documenting the token set, the Viridis scale, the Epistemic Weight classes, and the base components. Cross-referenced from the Design Brief (§5.2 color, §5.5 typography, §5.8 epistemic weight). MkDocs nav updated.

* [ ] **Validation.** `make fe-check` green. Histoire builds without errors. All accessibility tests pass. Visual regression baseline captured. Bundle-size gate: initial bundle now ~60–70 kB gzipped (design tokens + base components + fonts); within budget.

**Exit criteria:** Design tokens, typography, color scales, Epistemic Weight classes, base components, and the Histoire environment are all in place. A developer starting Phase 99 can compose features from existing primitives rather than inventing new ones. The a11y gate enforces WCAG 2.2 AA from this phase onward.

---

## Phase 99: Surface I — 3D Atmosphere Engine & First Contact [P1] - [ ] TODO

*The first user-facing feature. A rotating 3D globe with custom GLSL shaders for terminator and probe glow, served from live BFF data. Built framework-agnostically under `services/dashboard/packages/engine-3d/` per ADR-020 §5.9 consequence. High-Fidelity is the only mode shipped in this phase; a minimal text fallback covers browsers without WebGL2 (no full Low-Fi yet — that is a later phase once we know what we are reducing from). This phase proves the end-to-end pipeline — BFF query → typed client → Svelte state → engine API → WebGL rendering — under realistic constraints.*

* [ ] **3D engine package setup.** Create `services/dashboard/packages/engine-3d/` as a pnpm workspace package. `package.json` declares three.js as a peer dependency for correct tree-shaking into the lazy chunk. TypeScript strict mode. Own Histoire stories, own Vitest tests.

* [ ] **Three.js import discipline.** Import only what is needed: `WebGLRenderer`, `Scene`, `PerspectiveCamera`, `SphereGeometry`, `Mesh`, `ShaderMaterial`, `Vector3`, `Color`, `Clock`, `Raycaster`. `OrbitControls` from `three/examples/jsm/controls/`. Nothing else from `three/examples/*`. Verify: after full engine build, the three.js footprint is < 100 kB gzipped.

* [ ] **Globe geometry and textures.** Blue-Marble-style Earth texture (public domain NASA imagery, baked to 2048×1024 WebP, ~80 kB). Single sphere mesh. Texture loaded lazily after the shell's first paint — the engine canvas shows a dark sphere while the texture streams in, preserving Design Brief §1.1 (stillness).

* [ ] **Custom terminator shader (GLSL).** `packages/engine-3d/src/shaders/terminator.glsl` — fragment shader computing per-pixel day/night via `dot(normal, sunDirection)`. Smooth transition via `smoothstep` over a ~5° angular band for realistic twilight. Day side: unaltered Earth texture. Night side: darkened texture with subtle city-lights overlay (optional second texture, ~60 kB, lazy-loaded). Uniform `uSunDirection` updated per frame from current Unix timestamp.

* [ ] **Custom probe glow shader (GLSL).** Billboarded quad per probe point with radial gradient glow. Shader uniforms: `uCoreBrightness` (from publication volume), `uPulseRate` (from current activity density — documents per hour in the last rolling window), `uPulsePhase` (time-driven). Pulse modulation is bounded: `uPulseRate` is clamped so the fastest pulse completes no more than one cycle per ~4 seconds, honoring §1.1 "stillness with motion beneath". No post-processing pass — the glow is computed in-shader to respect the frame budget.

* [ ] **Custom reach-aura shader (GLSL).** Translucent volumetric field rendered as a soft sphere-projected geometry per probe. Input: a reach-region descriptor from the Probe Dossier API (polygon in lat/lon space, potentially non-contiguous for diasporic probes). Shader blends additively where multiple auras overlap. Aura is static — no animation. Alpha is low enough that overlapping auras compose without saturating. For Probe 0 (Germany), the reach is a simple country polygon; the engine treats it generically so a diasporic probe later "just works".

* [ ] **Propagation-arc shader slot (GLSL, reserved).** `packages/engine-3d/src/shaders/propagation.glsl` — great-circle arc shader wired into the engine from day one but inactive until multi-probe propagation data is available. The engine exposes `setPropagationEvents(events: PropagationEvent[])` in its API; when the array is empty, no geometry is drawn and no fragment cycles are spent. This phase commits only to the plumbing; rendering comes when data arrives in a later phase.

* [ ] **Atmospheric scattering shader.** Rayleigh/Mie halo around the globe edge. Standard analytic approximation (Bruneton-Neyret or equivalent reference). Ornamental yet informational per Design Brief §5.1 — the halo is real physics.

* [ ] **Camera controls.** `OrbitControls` with damping. Zoom limits: no closer than 1.2× globe radius, no farther than 8×. Auto-rotation at 0.05 rad/s when the user has not interacted in 10s. Reduced-motion respected: auto-rotation disabled when `prefers-reduced-motion: reduce` is set.

* [ ] **Raycaster for probe interaction.** Per-frame raycast on mouse-move identifies the closest probe point under the cursor. Hover: probe glow intensifies slightly; tooltip shows the emic designation (from the Content Catalog, Phase 95). Click: dispatches a `probe-selected` custom event to the framework shell, which opens the side panel.

* [ ] **Engine imperative API.** Minimal surface exposed to the shell:
  ```typescript
  export interface AtmosphereEngine {
    mount(element: HTMLCanvasElement): void;
    setProbes(probes: ProbeMarker[]): void;        // probes with reach-region data
    setActivity(activity: ProbeActivity[]): void;  // per-probe pulse and brightness
    setPropagationEvents(events: PropagationEvent[]): void;  // empty until Rhizome data
    setPillarMode(mode: 'aleph' | 'episteme' | 'rhizome'): void;
    setTimeRange(from: Date, to: Date): void;
    flyTo(lat: number, lon: number, durationMs?: number): void;
    on(event: 'probe-selected', handler: (probe: Probe) => void): void;
    dispose(): void;
  }
  ```
  The shell calls this API; the engine never reaches into the shell. Per ADR-020 §5.9.

* [ ] **Data layer foundation.** `src/lib/api/client.ts` wraps the generated types (from `make codegen-ts`) with a typed fetch client. `src/lib/api/queries.ts` defines query functions using TanStack Query (Svelte adapter). Query keys follow a stable schema. Error handling: typed discriminated unions for success / refusal / network error.

* [ ] **Refusal surface primitive.** `src/lib/components/RefusalSurface.svelte` — renders BFF HTTP 400 responses as methodological statements per Design Brief §5.4. Queries the Content Catalog (Phase 95) for the corresponding text. Renders semantic + methodological registers via Progressive Semantics (§5.7). First end-to-end consumer of the Content Catalog.

* [ ] **Progressive Semantics primitive.** `src/lib/components/ProgressiveSemantics.svelte` — renders any entity in Dual-Register form. Semantic register primary; methodological register accessible via a compact badge affordance. ARIA-Expanded for transitions. Both registers always present in DOM; CSS controls prominence.

* [ ] **Atmosphere route.** `src/routes/+page.svelte` — replaces the "Hello AĒR" landing.
  - Queries `/api/v1/probes` for active probes (including reach-region data from the Probe Dossier)
  - Queries `/api/v1/metrics/temporal_distribution` for the last 24 hours per probe → feeds pulse rate
  - Mounts the engine into a full-viewport canvas
  - On probe click: side panel opens with probe metadata rendered via Progressive Semantics
  - Refusal surfaces render in the panel when queries fail on methodological gates

* [ ] **Time scrubber component.** `src/lib/components/TimeScrubber.svelte` — horizontal slider for time-range selection. URL-persisted (`?from=...&to=...`). Keyboard-accessible (arrow keys, Home/End). Uses the base Button from Phase 98.

* [ ] **URL-state synchronization.** `src/lib/state/url.ts` — Svelte 5 Runes store that reads/writes state (`activeProbe`, `timeRange`, `resolution`, `viewingMode`) to the URL. §5.5 mandates deep-linkable state.

* [ ] **WebGL2 fallback.** Capability detection on startup. If WebGL2 is unavailable: render a plain text view listing active probes, their reach regions (as text), their current publication rate, and a clear notice: "This version of AĒR requires WebGL2 for the full experience. A fully accessible alternative is planned." No partial 3D; no canvas 2D substitute — that is the future Low-Fi phase. The fallback is 30 lines of Svelte; it exists to avoid a broken screen, not to be a feature.

* [ ] **Isolated engine stories (Histoire).** One story with Probe 0 (Germany), one with three synthetic probes across continents, one with the globe paused, one demonstrating fly-to, one with reach auras overlapping, one with the propagation slot fed synthetic arcs (for visual verification that the plumbing works). Each story exercises the imperative API directly.

* [ ] **Integration test.** Playwright test on WebGL2-capable Chromium: load dashboard → verify 3D globe renders → verify terminator visible → verify Probe 0 visible with reach aura over Germany → verify pulse animates → click the probe → verify the side panel opens with emic content from the Content Catalog → verify the trace appears in Tempo. Second test with WebGL2 disabled: verify the text fallback shows.

* [ ] **Performance verification.** Benchmark on 2021 M1 MacBook Air (Design Brief §7 high-fi target): 60fps sustained during orbit with Probe 0 visible + reach aura + terminator. Bundle size after this phase: shell + engine chunk + textures together ≤ 350 kB gzipped (shell remains ≤ 80 kB; engine chunk ≤ 250 kB; textures stream lazily and do not count toward initial paint). Frame times during scrubber interaction < 16 ms/frame.

* [ ] **Accessibility verification.** The 3D canvas has an ARIA-label describing the scene ("AĒR atmosphere: 3D rotating Earth showing active observation probes"). Keyboard navigation: Tab moves through probes; Enter opens the selected probe's panel. Screen readers get a textual state description that updates on meaningful state changes. The text fallback passes full WCAG 2.2 AA on its own.

* [ ] **Arc42 update.** Extend §8.17 (Frontend Architecture) with a brief note on the engine/shell separation and the imperative API. Cross-reference the engine package location.

* [ ] **Validation.** `make fe-check` green. All tests green. `make up` brings the full stack up; the atmosphere surface loads with live Probe 0 data. Lighthouse CI: first paint on 50 Mbps < 1.5s (shell paints first, engine and textures upgrade within ~3s). Histoire stories render correctly. Visual regression snapshots stable.

**Exit criteria:** A researcher can open the dashboard locally and see the 3D Earth rotating slowly, with Probe 0's reach aura over Germany and a gently pulsing luminous point over its source locations. The terminator is live. Clicking a probe opens a side panel with its emic context from the Content Catalog. The full pipeline — BFF → typed client → Svelte → engine → GLSL — is validated end-to-end. No Low-Fi yet (browsers without WebGL2 see a text fallback); that comes later, once there is more to reduce.

---

## Phase 100: Surface I Enrichment — Multi-Probe Composition & Layer-Depth Transitions [P1] - [ ] TODO

*With the 3D engine running and Probe 0 visible, Phase 100 exercises the architecture at breadth (multiple-probe composition, even if simulated) and depth (descent from L0 to L3 within Surface I). This phase does not add new surfaces — it proves that the L0→L1→L2→L3 descent on Surface I works as specified in Design Brief §4. Rhizome remains latent; the propagation-arc slot from Phase 99 stays empty until a second probe is live.*

* [ ] **Multi-probe rendering verification (synthetic).** Add a build-time flag that loads a small set of synthetic probes (e.g. three probes on three continents) for visual and performance verification. The synthetic data never leaves development builds. Confirms that reach-aura composition, pulse rate variation, and probe distribution render correctly at realistic multi-probe scale.

* [ ] **L1 Orientation overlay.** Soft top-bar overlay (fade-in on first mouse movement): current time range, active probe count, normalization mode, active cultural contexts (emic designations from the Content Catalog). Fade-out after 10s of inactivity. Design Brief §4.2 Atmosphere/L1 cell.

* [ ] **L2 Exploration controls.** Time-range scrubber (already built in Phase 99) becomes the L2 primary control. Add: resolution switch (5min / hourly / daily / weekly / monthly), pillar-mode toggle (Aleph/Episteme/Rhizome — but currently all three default to the same render, since Rhizome has no data and Episteme/Aleph differ only in time-window framing). Region zoom via mouse wheel / pinch.

* [ ] **L3 Analysis companion panel.** On probe click, the panel that opens is no longer just metadata — it becomes the L3 Analysis view. Inside: a small time-series chart (using uPlot, framework-agnostic per §5.9) showing the selected metric for the selected probe over the selected time range. Uncertainty bands. Epistemic Weight styling from Phase 98. The 3D globe stays present behind the panel, dimmed to 30% opacity per §4.1 rule 2 ("no layer replaces").

* [ ] **uPlot integration.** Add `uPlot` to the shell. Build a thin Svelte wrapper `src/lib/components/TimeSeriesChart.svelte` that renders a uPlot instance. Framework-agnostic at its core — the Svelte wrapper is < 30 lines, all actual rendering is uPlot's. Bundle impact: +40 kB gzipped, loaded when L3 activates (intent-based, starts preloading on probe hover).

* [ ] **Descent animation via View Transitions API.** Descending from L0 to L3 (clicking a probe) uses the View Transitions API for a morphing transition: the probe point expands visually into the panel position; the globe fades behind; the panel's chart fades in. Ascent (closing the panel) reverses. Fallback for browsers without View Transitions: instant state change (still correct, just less elegant).

* [ ] **L4 Provenance fly-out (partial).** The L3 panel's "why this shape?" affordance at the chart's corner opens an L4 Provenance fly-out. Content: tier classification, validation status, known limitations, equivalence level — all from the existing `/api/v1/metrics/{metricName}/provenance` endpoint, rendered via Progressive Semantics. This is the first full use of the Content Catalog's methodological register at the layer where it dominates (per §5.7 "primary at Layer 4").

* [ ] **Keyboard navigation across layers.** Tab cycles through visible probes. Enter descends to L3 on the selected probe. Escape ascends one layer at a time (L3 → L0). Shift+Tab at L0 focuses the overlay controls (L1 bar). This is not a bolt-on — it defines the invisible interaction grammar that §4.1 rule 1 ("each layer reachable in one interaction") requires.

* [ ] **URL state extended.** Descent state encoded in URL: `?probe=probe-0-de-institutional-rss&metric=sentiment_score&view=analysis`. Deep-linking into L3 works; refreshing the page preserves descent.

* [ ] **Negative Space overlay toggle (structural only).** Add a keyboard shortcut (e.g. `Shift+N`) and a quiet UI affordance for the Negative Space overlay. In this phase the toggle exists and is hooked up to a Svelte 5 `$state` rune; the actual visual reweighting (Design Brief §3.4, §4.4) is deferred to a later phase along with the demographic-skew annotations from WP-003 §6. The toggle is visible but idempotent until then.

* [ ] **Accessibility across layers.** `@axe-core/playwright` tests against every descent state: L0, L1, L2, L3, L4 fly-out. Zero WCAG 2.2 AA violations at any depth. Screen-reader announcements on descent: "Descending to Analysis view for Tagesschau probe" or equivalent.

* [ ] **Performance across layers.** Lighthouse CI: descent from L0 to L3 under 500 ms (§7 high-fi budget). Ascent equally fast. Frame budget during descent transitions: < 33 ms (below the 60fps hard ceiling is acceptable during active transition; idle state returns to 16 ms).

* [ ] **Integration tests.** Playwright E2E: (a) full descent L0→L1→L2→L3→L4, verifying each layer's state; (b) keyboard-only descent (no mouse); (c) deep-link directly to L4 via URL, verify correct initial render; (d) multi-probe view with synthetic probes — confirm visual non-overlap of reach auras and correct pulse behavior.

* [ ] **Arc42 update.** Extend §8.17 with notes on the descent mechanism and the URL-state encoding. Record the View Transitions API dependency and its graceful-degradation behavior.

* [ ] **Validation.** `make fe-check` green. All tests green. Visual regression snapshots for each layer state captured and stable.

**Exit criteria:** A researcher can click a probe, descend into a time-series analysis view with provenance access, return to the atmospheric view, and share the state via URL. The fractal pillars (Aleph at L0, Episteme tightening at L2-L3) are observable in the interaction. Rhizome remains architecturally ready but latently invisible. The entire Surface I is complete through L4; Surface II and III and the full Negative Space overlay come in subsequent phases.

---

### Low-Fidelity Mode — deferred

The Low-Fidelity rendering mode from Design Brief §5.6 is **not** part of Phases 94–99. The original plan placed it before the 3D engine; that ordering was reversed because:

1. The Low-Fi mode is a *reduction of* the High-Fi experience. Building it before High-Fi would require guessing what the High-Fi version looks like — introducing rework.
2. Hardware-equity performance can only be measured against a complete High-Fi baseline.
3. The minimal WebGL2 text fallback from Phase 99 is sufficient to avoid a broken screen on weak hardware while Low-Fi is developed later.

Low-Fidelity becomes its own phase (tentatively Phase 104 or later, after Surfaces II and III are in place and the full system is a known quantity to reduce from). The commitment from Design Brief §5.6 stands; only the sequencing changes.

---

**Begründung für die Neufassung:**
- Phase 99 ist jetzt die High-Fi 3D Atmosphere (vorher Phase 100). Sie trägt die vollen fraktalen L0-Aspekte aus dem aktualisierten Brief §3.1 — Reach Aura, Aktivitäts-Pulse, Propagations-Slot.
- Phase 100 ist neu: sie erweitert die bestehende Surface I um Descent-Mechanik (L0→L4) und erste Multi-Probe-Aspekte. Sie berührt noch nicht Surface II oder III.
- Low-Fi verschiebt sich — ohne aus dem Brief oder ADR verschwinden. Der Brief-Commitment bleibt.
- Ein trivialer Text-Fallback in Phase 99 fängt Browser ohne WebGL2 ab, damit keine barriere-freie Lücke entsteht.
