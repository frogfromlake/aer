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

* [x] **Verzeichnisstruktur anlegen.** `docs/probes/probe-0-de-institutional-web/` mit fünf Dateien:

* [x] **`README.md` — Probe Overview & WP Coverage Matrix.** Zweck der Probe, beteiligte Quellen (tagesschau.de, bundesregierung.de), Engineering-Kalibrierungsstatus (nicht wissenschaftlich motiviert — vgl. §13.10), Exit-Kriterien. Enthält eine WP Coverage Matrix: tabellarische Zuordnung jedes der sechs Working Papers zur Datei oder Tabelle, in der die probe-spezifische Dokumentation lebt. WP-001 → `classification.md`. WP-003 → `bias_assessment.md`. WP-005 → `temporal_profile.md`. WP-006 → `observer_effect.md`. WP-002 → Verweis auf `aer_gold.metric_validity` (systemweit, nicht probe-spezifisch; Validierungsstudien werden pro `(metric_name, context_key)`-Paar durchgeführt). WP-004 → Verweis auf `aer_gold.metric_baselines` und `aer_gold.metric_equivalence` (cross-probe per Definition).

* [x] **`classification.md` — Probe Classification (WP-001).** Dokumentiert die etic/emic Klassifikation beider Quellen: `primary_function`, `secondary_function`, `emic_designation`, `emic_context` — exakt die Werte aus der `source_classifications`-Tabelle (Migration 000006). Dokumentiert den `review_status = 'provisional_engineering'` und erklärt, warum `function_weights = NULL` (WP-001 §4.4 Schritte 1–2 ausstehend). Cross-Referenz: Scientific Operations Guide Workflow 1.

* [x] **`bias_assessment.md` — Platform Bias (WP-003).** Inhaltlich identisch mit dem bestehenden `docs/methodology/probe0_bias_profile.md` — verschoben, nicht neu geschrieben. Enthält die `BiasContext`-Werte und die fünf dokumentierten strukturellen Biases. Cross-Referenz: Scientific Operations Guide Workflow 5.

* [x] **`temporal_profile.md` — Temporal Characteristics (WP-005).** Publikationsmuster pro Quelle: tagesschau.de ≈ 50 Artikel/Tag (→ `min_meaningful_resolution = hourly`), bundesregierung.de ≈ 5 Artikel/Tag (→ `min_meaningful_resolution = daily`). Verweis auf den Cultural Calendar (`configs/cultural_calendars/de.yaml`). Hinweis auf die geplante Tiered Retention Strategy (§8.8, "planned — not yet active"). Cross-Referenz: Scientific Operations Guide Workflow 6.

* [x] **`observer_effect.md` — Observer Effect Assessment (WP-006).** Initiale Bewertung basierend auf dem `observer_effect_assessment.yaml`-Template (Phase 68). Für Probe 0 ist das Risiko gering: öffentliche RSS-Feeds, kein Login, kein Engagement-Signal, das Rückkopplungen erzeugen könnte. Dokumentiert: `cultural_region = DE`, `beneficial_effects`, `harmful_effects`, `vulnerable_populations`, `recommended_safeguards`. Cross-Referenz: Scientific Operations Guide Workflow 1, Step 4.

### Documentation Consolidation

* [x] **`docs/methodology/extractor_limitations.md` löschen.** Das Dokument ist redundant: `metric_provenance.yaml` enthält die strukturierten `known_limitations` pro Metrik (SSoT für die API), WP-002 §3 enthält die wissenschaftliche Analyse. Die Prosa in `extractor_limitations.md` dupliziert beides, ohne eigenen Informationswert zu liefern. Cross-cutting Concerns (Graceful Degradation, Immutable Input, Validation Required) sind bereits in Arc42 §8.3 dokumentiert.

* [x] **`metric_provenance.yaml` anreichern.** `wp_reference`-Feld pro Metrik hinzufügen (YAML-only, nicht über die API exponiert, da der Go-YAML-Parser unbekannte Felder ignoriert). `known_limitations`-Einträge von knappen Labels zu vollständigen Sätzen erweitern, die das Problem und seine Konsequenz erklären. Beispiel: statt `"Negation blindness"` → `"Negation blindness — SentiWS does not propagate negation scope; 'nicht gut' scores as positive (WP-002 §3)."` Die bestehenden Einträge sind bereits nahe an diesem Format; die fehlenden werden angeglichen.

* [x] **`docs/methodology/probe0_bias_profile.md` entfernen.** Inhalt lebt jetzt in `docs/probes/probe-0-de-institutional-web/bias_assessment.md`. Redirect-Kommentar in der gelöschten Datei ist nicht nötig — alle Verweise werden in dieser Phase aktualisiert.

* [x] **PostgreSQL Migration: `documentation_url` aktualisieren.** Migration `000008_update_documentation_url.up.sql`: `UPDATE sources SET documentation_url = 'docs/probes/probe-0-de-institutional-web/' WHERE name IN ('bundesregierung', 'tagesschau');` — zeigt auf das Dossier-Verzeichnis statt auf eine einzelne Datei.

* [x] **`mkdocs.yml` Navigation anpassen.** Unter "Methodology (EN)": `probe0_bias_profile.md` und `extractor_limitations.md` entfernen. Neuen Abschnitt "Probes" hinzufügen mit `docs/probes/probe-0-de-institutional-web/README.md` als Einstieg. Die Unterseiten (`classification.md`, `bias_assessment.md`, `temporal_profile.md`, `observer_effect.md`) als Kinder darunter.

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
  - **Probe 0 Walkthrough:** Zeigt die sechs `BiasContext`-Werte für RSS-Quellen und verlinkt auf `docs/probes/probe-0-de-institutional-web/bias_assessment.md` als fertiges Beispiel. Erklärt die Entscheidung, dass für RSS alle Felder vom Entwickler gesetzt werden können (kein Domain-Expertise-Split nötig, weil RSS keine algorithmische Amplifikation hat).

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
          - "Overview": probes/probe-0-de-institutional-web/README.md
          - "Classification (WP-001)": probes/probe-0-de-institutional-web/classification.md
          - "Bias Assessment (WP-003)": probes/probe-0-de-institutional-web/bias_assessment.md
          - "Temporal Profile (WP-005)": probes/probe-0-de-institutional-web/temporal_profile.md
          - "Observer Effect (WP-006)": probes/probe-0-de-institutional-web/observer_effect.md
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
  │   │   └── probe-0-de-institutional-web.yaml
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

* [x] **Seed content for Probe 0 (EN + DE).** Draft the probe dossier content for `probe-0-de-institutional-web`. Pull from existing `docs/probes/probe-0-de-institutional-web/README.md` and related dossier files. This content is consumed by Surface III (Reflection) when the probe is inspected.

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

* [x] **E2E smoke test extension.** Extend `scripts/e2e_smoke_test.sh` with assertions against the new content endpoints for at least one metric (sentiment_score), one probe (probe-0-de-institutional-web), one discourse function (epistemic_authority), and one refusal (normalization_equivalence_missing). Both locales (en, de) verified.

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


## Phase 97: Frontend Scaffolding — SvelteKit Static + Infrastructure Integration [P0] - [x] DONE

*This phase creates `services/dashboard/` as a new service in the monorepo. It produces a minimal "Hello AĒR" page that renders through Traefik, emits OTel traces into the existing collector, and passes the same CI/supply-chain rigor as the Go services. No user-facing features yet — this is purely the foundation on which all subsequent frontend work stands.*

* [x] **Service directory structure.** Create `services/dashboard/` with the following structure:
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

Before installing anything, check if it is already installed and update to its below desired version.
All versions pinned like in the backend (if best practice)

* [x] **Svelte / SvelteKit configuration.** Install Svelte 5, SvelteKit with `@sveltejs/adapter-static`, TypeScript 6.0+. `svelte.config.js` configures the static adapter with `prerender: true` and fallback `index.html` for SPA routing. `tsconfig.json` has strict mode enabled (`"strict": true`, `"noUncheckedIndexedAccess": true`, `"exactOptionalPropertyTypes": true`).

* [x] **pnpm and lockfile.** pnpm 10.33+ as the package manager. `pnpm install --frozen-lockfile` enforced in CI. `.npmrc` with `strict-peer-dependencies=true`. Pin all dev dependencies to exact versions — no caret ranges in `package.json`.

* [x] **TypeScript API client codegen.** Install `openapi-typescript`. Add `make codegen-ts` target to the root `Makefile` that runs `openapi-typescript services/bff-api/api/openapi.yaml -o services/dashboard/src/lib/api/types.ts`. Add a `.tool-versions` entry for `openapi-typescript`. CI workflow step: `make codegen-ts && git diff --exit-code` — mirror of the existing Go drift check.

* [x] **ESLint + Prettier.** ESLint with `@typescript-eslint` and the official Svelte plugin. Prettier with the Svelte plugin. Same formatting conventions as the Go/Python code where applicable (2-space indentation for TS/Svelte, LF line endings, single quotes for strings).

* [x] **Makefile integration.** Add to the root Makefile:
  ```make
  fe-install:    ## Install frontend dependencies
  fe-lint:       ## Lint frontend (ESLint + Prettier check + svelte-check)
  fe-typecheck:  ## TypeScript strict typecheck
  fe-test:       ## Vitest unit tests
  fe-test-e2e:   ## Playwright end-to-end smoke test
  fe-build:      ## Production build
  fe-check:      ## Composite: lint + typecheck + test + bundle-size gate
  ```
  Maybe also think of `make frontend-up , down, restart`.

  Extend the existing `make lint` target to include `fe-lint`. Extend `make test` to include `fe-test`. The existing `make codegen` target remains Go-only; `make codegen-ts` is its frontend peer. Decide if we rename `make up, make down, make restart` etc. to specify for backend (e.g. make backend-up etc.) and do the same for the frontend OR/AND if we add the Frontend rules in `make up` so it starts after the backend.

* [x] **Bundle-size gate.** Add a Vite plugin (`rollup-plugin-visualizer` or equivalent) that emits bundle stats. Add a CI step that fails the build if the initial bundle (shell + router + runtime) exceeds 80 kB gzipped. Budget exists with headroom — the 180 kB total budget (Design Brief §7) is enforced at phase 98 when actual Surface I code lands.

* [x] **OpenTelemetry Web SDK.** Install `@opentelemetry/sdk-trace-web`, `@opentelemetry/instrumentation-fetch`, `@opentelemetry/exporter-trace-otlp-http`. Configure in a dedicated `src/lib/observability/otel.ts` module. Lazy-load after first paint (do not block initial render). OTLP endpoint configurable via build-time env var; default points at the existing `otel-collector:4318` internal service for development. Resource attributes: `service.name=aer-dashboard`, `service.version` (from `package.json`), `deployment.environment` (build-time env var).

* [x] **Dockerfile (multi-stage).** Build stage uses `node:22-alpine3.23` with pinned digest; runtime stage uses `nginx:1.27-alpine3.23` with pinned digest. Build output from SvelteKit's static adapter is copied to `/usr/share/nginx/html`. Nginx config serves `index.html` as SPA fallback. Image is pinned via the SSoT pattern from Arc42 §2.3. Build output size gate: refuse to build if image exceeds 50 MB.

* [x] **Supply-chain hardening (Phase 84 parity).** Image build pipeline produces:
  - Trivy scan (fails on HIGH/CRITICAL vulnerabilities; MEDIUM reporting-only).
  - Cosign signature and Syft SBOM deferred to match current Go-service CI parity (neither is applied to the backend images today). When Cosign/SBOM rolls out for Go, the dashboard image joins the same step symmetrically.
  Extended `container-security-scan` in `.github/workflows/ci.yml` to build and scan `aer/dashboard:ci` alongside the Go images.

* [x] **`compose.yaml` integration.** Add the `dashboard` service to `compose.yaml`:
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

* [x] **Traefik routing verification.** With `make up`, confirm that `https://localhost/` serves the dashboard and `https://localhost/api/...` still routes to the BFF. Check that the BFF is *not* reachable via the dashboard's routing.

* [x] **Git hooks extension.** No hook-file changes were needed: `scripts/hooks/pre-commit` already delegates to `make lint` and `scripts/hooks/pre-push` to `make lint` + `make test`, and both top-level targets were extended in Phase 97a to include `fe-lint`/`fe-test`. Frontend coverage therefore arrives automatically. A changed-file fast-path was deliberately not added — it would duplicate knowledge of which paths belong to which language and drift out of sync. Pre-commit on a Go-only change runs the frontend lint in ~2 s, which is well under the Go lint cost, so the "fast path" is unnecessary.

* [x] **CI workflow extension.** Added `frontend-pipeline` to `.github/workflows/ci.yml`: checkout → load `.tool-versions` → setup Node (pinned by `NODE_VERSION`) + Corepack pnpm → `pnpm install --frozen-lockfile` → `make openapi-bundle` → `make fe-codegen` + `git diff --exit-code services/dashboard/src/lib/api/types.ts` (drift check mirroring the Go codegen gate) → `make fe-lint` → `make fe-test` → `make fe-build` → `make fe-bundle-size`. Image build + Trivy scan live in the existing `container-security-scan` job for symmetry with the Go services. Playwright/E2E is intentionally deferred: no e2e smoke exists yet and shipping an empty `playwright test` in CI would be decorative — this edge arrives with Phase 98 base-component stories.

* [x] **Health & readiness.** The dashboard container responds to `GET /healthz` (just returns 200 from Nginx on a static file) for Docker healthchecks. No dynamic readiness needed — it's a static bundle.

* [x] **README.md.** Service-level README at `services/dashboard/README.md` with: one-paragraph description, cross-references to ADR-020 and the Design Brief, developer setup commands, and an architecture overview that defers to the central docs. No duplicate content — only pointers.

* [x] **Arc42 §7 update.** Fill in the `dashboard` service row in §7.4 (Deployment View) that was a placeholder in Phase 94: actual image base, port, network, memory, CPU, healthcheck command.

* [x] **Validation.** `make up` brings the stack up including the dashboard; `make fe-check`, `make lint` and `make test` all green; the dashboard loads in a browser through Traefik at `https://localhost/` with `<title>AĒR</title>` and the BFF stays reachable at `https://localhost/api/*`. End-to-end OTel traces in Grafana Tempo are carried over to Phase 98 alongside the build-arg plumbing for `PUBLIC_OTLP_ENDPOINT` / `PUBLIC_DEPLOYMENT_ENVIRONMENT` — the SvelteKit static adapter resolves these at build time, so they have to ship as Docker `ARG`s once the first consuming component lands.

**Exit criteria:** A blank but fully instrumented dashboard service runs alongside the existing stack, emits traces, follows the monorepo's supply-chain hardening, and passes the same quality gates as the Go services. Zero user-facing features — but the foundation is production-grade.

---


## Phase 98: Design System Foundation [P1] - [x] DONE

*Before rendering any real data, the dashboard needs a design system — typography, color tokens, spacing, focus-rings, dark-mode defaults. This phase establishes those primitives and wires the CI gates that enforce them. The design system is additive: later phases extend it, but never rewrite it. Delivered in four sub-phases (98a–98d), mirroring the 97a–97e cadence.*

* [x] **CSS custom properties as design tokens (98a).** AĒR design token set landed in `services/dashboard/src/lib/design/tokens.css`: colors (Viridis/Cividis palette stops, UI grays, accent cyan, `--color-accent-solid` / `--color-on-accent` for WCAG 2.2 AA primary-button contrast, semantic validation-status colors), typography scale (Inter Variable UI, IBM Plex Mono for numbers, 1.25 modular scale), spacing scale (4 px baseline grid), radius scale, elevation scale, motion scale (durations + eases). All values are CSS custom properties — no Tailwind, no framework-specific tokens.

* [x] **Dark-mode-first approach (98a).** Tokens provide `data-theme="dark"` (default) and `data-theme="light"` (accessible fallback). CSS-only switching via custom properties — no JS needed for theme application. `@media (prefers-color-scheme)` honored when no explicit attribute is set.

* [x] **Typography rendering (98a).** Self-hosted Inter Variable and IBM Plex Mono via `@fontsource-variable/inter` and `@fontsource/ibm-plex-mono`. `src/lib/design/fonts.css` declares `@font-face` for Latin, Latin-Extended, Greek, and Greek-Extended subsets (for ἀήρ and variants) plus the mono weights actually used (400, 600). Zero-Trust compliant — no Google Fonts requests from the user's browser.

* [x] **Viridis color scale module (98a).** `src/lib/design/viridis.ts` exports frozen `VIRIDIS_256` and `CIVIDIS_256` hex arrays plus `viridis(t)` / `cividis(t)` interpolators. Arrays generated via 6th-order polynomial approximation. These are the canonical scales consumed by every visualization module per Design Brief §5.2.

* [x] **Epistemic Weight styles (98a).** `src/lib/design/epistemic-weight.css` defines the Design Brief §5.8 treatments as CSS classes: `.weight-tier1-unvalidated`, `.weight-tier1-validated`, `.weight-tier2-validated`, `.weight-tier3`, `.weight-expired`, plus `.weight-expired-region` (hatched overlay) and `.weight-badge`. Visualization modules consume these classes rather than hardcoding visual treatments.

* [x] **Accessibility primitives (98a).** Focus-ring tokens with ≥ 3:1 contrast against any token background. `:focus-visible` for keyboard-only focus. `prefers-reduced-motion` collapses the motion scale to near-zero. `.skip-link` / `.sr-only` utilities in `global.css`. ARIA conventions documented in `src/lib/design/a11y.md` (updated in 98c for the Badge `role="img"` rule).

* [x] **Base component primitives (98b).** `src/lib/components/base/` ships `Button.svelte` (primary / secondary / ghost, two sizes, loading and disabled states; loading uses `opacity: 0` on the label so the accessible name survives), `Dialog.svelte` (`role="dialog"` + `aria-modal="true"`, focus trap, Escape + backdrop close, focus restoration), `Tooltip.svelte` (aria-described, hover + focus, four placements), `Badge.svelte` (`role="img"` with `aria-label`, Epistemic Weight tiers), and `SkipLink.svelte`. Pure primitives — no business logic.

* [x] **Story harness (98b, revised).** Histoire was attempted and dropped: `@histoire/plugin-svelte@1.0.0-beta.1` ships precompiled `.svelte.js` artifacts that import from `svelte/internal`, which is forbidden under Svelte 5; upstream Svelte 5 support still lives on the `add-svelte5-support` feature branch. Replaced with a route-based harness at `src/routes/stories/` — sidebar navigation, dark/light theme toggle, one `+page.svelte` per component demonstrating every variant. The harness builds through the same Vite pipeline as the app, which also gives the Phase 98c Playwright + axe gate a single surface to drive. See `docs/design/design_system.md` §5 for the rationale.

* [x] **Visual regression baseline (98c).** `services/dashboard/tests/e2e/visual.spec.ts` captures each story route at both themes (plus Dialog in its open state). 12 baseline PNGs committed under `tests/e2e/__snapshots__/`. Playwright runs inside the pinned `mcr.microsoft.com/playwright:v1.59.1-noble` image (declared as the `playwright-runner` service in `compose.yaml` per Hard Rule 1 SSoT) so baselines match CI byte-for-byte. Exposed via `make fe-test-e2e` and `make fe-test-e2e-update`.

* [x] **Accessibility baseline testing (98c).** `@axe-core/playwright` (pinned `4.11.2`) runs against `/`, `/stories`, and every component story plus the Dialog-open state, tagged `wcag2a`, `wcag2aa`, `wcag21aa`, `wcag22aa`. Any violation fails `make fe-test-e2e`. The first green run surfaced and fixed four real AA gaps: `--color-fg-subtle` contrast on both themes, the primary-button white-on-cyan contrast on light theme, the loading-button accessible-name regression from `visibility: hidden`, and the Badge `aria-label`-on-bare-span violation. The CI `frontend-pipeline` job runs the gate and uploads the Playwright report + `test-results/` on failure.

* [x] **Design documentation (98a + 98b + 98c).** `docs/design/design_system.md` documents the token set, typography, Viridis/Cividis scales, Epistemic Weight classes, base components, and the Playwright + axe gate. Cross-referenced from `design_brief.md` §9. MkDocs nav updated. `src/lib/design/a11y.md` is the canonical a11y spec.

* [x] **OTel wiring end-to-end (98d).** `PUBLIC_OTLP_ENDPOINT` and `PUBLIC_DEPLOYMENT_ENVIRONMENT` are promoted to Docker `ARG` in the dashboard builder stage (`services/dashboard/Dockerfile`), wired through the `args:` block of the `dashboard` service in `compose.yaml`, and documented in `.env.example`. Verified by building the image with `--build-arg PUBLIC_OTLP_ENDPOINT=…` and grepping the resulting static bundle — both values are inlined by the SvelteKit static adapter at build time. Cross-linked from `services/dashboard/README.md` (Configuration section) and Arc42 §7.4.4 (Application Services). Empty `PUBLIC_OTLP_ENDPOINT` is the recommended default for environments without observability; the Grafana Tempo handshake against a real deployed dashboard is not re-validated here — it is gated by the first deployment that sets a non-empty endpoint.

* [x] **Validation.** `make fe-check` green. `make fe-test-e2e` green (23 tests: 12 visual baselines × themes + 9 a11y routes + 2 smoke). Initial-bundle gate: 30.93 kB gzipped (budget 80 kB) — components remain tree-shaken from the landing page. `docker build` with PUBLIC_* build args verified end-to-end.

**Exit criteria:** Design tokens, typography, color scales, Epistemic Weight classes, base components, the route-based story harness, the Playwright visual-regression gate, and the axe-core WCAG 2.2 AA gate are all in place. The OTel build-arg plumbing is live; enabling traces in Grafana Tempo is now a deployment-time decision (set `PUBLIC_OTLP_ENDPOINT` and rebuild), not a code change. A developer starting Phase 99a can compose features from existing primitives rather than inventing new ones, with every new component gated by the visual + a11y suite.

---

## Phase 99a: Surface I — 3D Atmosphere Engine Foundation [P1] - [x] DONE

*The first user-facing WebGL surface, but still standalone. Phase 99a builds the 3D engine as a self-contained package (`services/dashboard/packages/engine-3d/`) with a rotating textured Earth, a live terminator, an atmospheric scattering halo, camera controls, and a minimal WebGL2-unavailable fallback. The engine is driven entirely through its imperative API and verified via the story harness; it is not yet wired to the BFF or mounted on a live dashboard route. Phase 99b takes this engine and connects it to real probe data. Splitting the phase this way lets the shader work land in a bounded review without also reviewing a data-layer overhaul in the same PR.*

* [x] **3D engine package setup.** Create `services/dashboard/packages/engine-3d/` as a pnpm workspace package. `package.json` declares three.js as a peer dependency for correct tree-shaking into the lazy chunk. TypeScript strict mode. Own stories under the route-based story harness (Phase 98b), own Vitest tests.

* [x] **Three.js import discipline.** Import only what is needed: `WebGLRenderer`, `Scene`, `PerspectiveCamera`, `SphereGeometry`, `Mesh`, `ShaderMaterial`, `Vector3`, `Color`, `Clock`, `Raycaster`. `OrbitControls` from `three/examples/jsm/controls/`. Nothing else from `three/examples/*`. Verify: after full engine build, the three.js footprint is < 100 kB gzipped.

* [x] **Globe geometry and vector landmasses.** Single sphere mesh for the ocean (deep, almost-black blue per Design Brief §3.1). Landmasses are rendered as a separate, slightly inset mesh built from **Natural Earth 1:50m land polygons** (public-domain), simplified and triangulated at build time via `earcut` into a static `BufferGeometry`. Ocean and land colors are uniforms on a shared `ShaderMaterial` (no raster texture; resolution-independent under zoom). Landmass color: a restrained lighter blue. The bake step is a one-shot `scripts/bake-landmass.mjs` that downloads from the canonical Natural Earth source, resizes/simplifies to the chosen tolerance, and emits a compact JSON asset (~60 kB) checked into `static/data/`. The engine canvas shows a dark sphere while the land asset streams in, preserving Design Brief §1.1 (stillness). *Rationale for choosing vectors over Blue-Marble-style raster: the Brief explicitly forbids satellite-photo realism (§3.1, "restrained silhouettes"); vectors stay crisp at country-level zoom (Phase 100+), avoid mipmap blur, and remove a layer of geographic specificity that competes with the discourse signal.*

* [x] **Optional borders layer (default off).** Engine API exposes `setBordersVisible(visible: boolean)`. Border geometry is built from Natural Earth Admin-0 (public-domain) by the same bake step into a separate `LineSegments` asset (~80 kB), lazy-loaded only on the first toggle-on. Default off; not exposed in the 99a stories beyond a single dedicated story.

* [x] **Custom terminator shader (GLSL).** `packages/engine-3d/src/shaders/terminator.glsl` — fragment shader computing per-pixel day/night via `dot(normal, sunDirection)`. Smooth transition via `smoothstep` over a ~5° angular band for realistic twilight. Day side: full ocean/land color. Night side: same colors darkened to a deep night register (no city-lights overlay — the surface is symbolic, not photoreal). Uniform `uSunDirection` updated per frame from the current Unix timestamp via a deterministic NOAA solar-position approximation in `sun.ts`.

* [x] **Atmospheric scattering shader.** Rayleigh/Mie halo around the globe edge. Standard analytic approximation (Bruneton-Neyret or equivalent reference). Ornamental yet informational per Design Brief §5.1 — the halo is real physics.

* [x] **Camera controls.** `OrbitControls` with damping. Zoom limits: no closer than 1.2× globe radius, no farther than 8×. Auto-rotation at 0.05 rad/s when the user has not interacted in 10s. Reduced-motion respected: auto-rotation disabled when `prefers-reduced-motion: reduce` is set.

* [x] **Engine imperative API (skeleton).** Minimal surface exposed to the shell, with the data-driven methods present but accepting empty inputs in 99a — the shader plumbing they drive (probe glow, reach-aura, propagation arcs, raycaster) lands in 99b:
  ```typescript
  export interface AtmosphereEngine {
    mount(element: HTMLCanvasElement): void;
    setProbes(probes: ProbeMarker[]): void;        // accepts empty array in 99a
    setActivity(activity: ProbeActivity[]): void;  // accepts empty array in 99a
    setPropagationEvents(events: PropagationEvent[]): void;  // accepts empty array in 99a
    setPillarMode(mode: 'aleph' | 'episteme' | 'rhizome'): void;
    setTimeRange(from: Date, to: Date): void;
    flyTo(lat: number, lon: number, durationMs?: number): void;
    on(event: 'probe-selected', handler: (probe: Probe) => void): void;  // wired in 99b
    dispose(): void;
  }
  ```
  The shell calls this API; the engine never reaches into the shell. Per ADR-020 §5.9.

* [x] **WebGL2 fallback.** Capability detection on startup. If WebGL2 is unavailable: render a plain text view with a clear notice: "This version of AĒR requires WebGL2 for the full experience. A fully accessible alternative is planned." No partial 3D; no canvas 2D substitute — that is the future Low-Fi phase. The fallback is ~30 lines of Svelte; it exists to avoid a broken screen, not to be a feature. In 99a the fallback is static; 99b extends it with live probe data.

* [x] **Isolated engine stories.** Under the Phase 98b route-based story harness: one story with a paused globe, one demonstrating fly-to, one showing the terminator at several fixed sun positions, one exercising the WebGL2-unavailable fallback. Each story exercises the imperative API directly.

* [x] **Performance verification (engine in isolation).** Benchmark on 2021 M1 MacBook Air (Design Brief §7 high-fi target): 60fps sustained during orbit with the textured Earth + terminator + atmospheric halo visible. Engine chunk (excluding textures) ≤ 250 kB gzipped. Textures stream lazily and do not count toward initial paint.

* [x] **Accessibility verification (engine in isolation).** The 3D canvas has an ARIA-label describing the scene ("AĒR atmosphere: 3D rotating Earth"). The text fallback passes full WCAG 2.2 AA on its own. (Probe-level keyboard navigation comes in 99b.)

* [x] **Arc42 update.** Extend §8.17 (Frontend Architecture) with a brief note on the engine/shell separation and the imperative API. Cross-reference the engine package location.

* [x] **Validation.** `make fe-check` green. All tests green. Story harness stories render correctly. Visual regression snapshots stable. Engine bundle size under budget.

**Exit criteria:** The 3D Atmosphere Engine exists as a reviewable standalone package. Running the story harness shows a rotating textured Earth with a live terminator and atmospheric halo, camera controls work, and the WebGL2-unavailable fallback renders on incapable browsers. The engine is not yet wired to the BFF — no probes, no live data. Phase 99b adds those on top.


## Phase 99b: Surface I — First Contact (Live Probe Data on the Atmosphere) [P1] - [x] DONE

*Wires the Phase 99a engine to live BFF data. Adds probe-specific shaders (glow, reserved propagation-arc slot), raycaster interaction, the typed data layer, the Refusal Surface and Progressive Semantics primitives, and the live Atmosphere route. This phase proves the end-to-end pipeline — BFF query → typed client → Svelte state → engine API → WebGL rendering — with Probe 0 as the canonical first consumer.*

**Scope decision — reach is not rendered.** The originally planned *reach-aura shader* is dropped from 99b. A probe's reach — where its content is actually read, cited, or discursively effective — cannot be derived from emission geometry or language alone, and inferring it would be an epistemic fiction incompatible with AĒR's stance (manifesto §3, Design Brief §5.7). `/api/v1/probes` therefore returns emission points only, not reach polygons. On the globe, each probe renders as glow(s) at its emission origin(s) and nothing more. A reach claim can only return once an actual consumption/citation signal exists; the aura shader is deferred to that future phase and explicitly *not* carried as a reserved slot, so we do not accrete dormant code for an unsupported claim. The methodological register at L3/L4 must state that reach is unmeasured.

* [x] **Custom probe glow shader (GLSL).** Billboarded quad per *emission point* with radial gradient glow. Shader uniforms: `uCoreBrightness` (from publication volume), `uPulseRate` (from current activity density — documents per hour in the last rolling window, computed client-side from `/api/v1/metrics` with `metricName=publication_hour`), `uPulsePhase` (time-driven). Pulse modulation is bounded: `uPulseRate` is clamped so the fastest pulse completes no more than one cycle per ~4 seconds, honoring §1.1 "stillness with motion beneath". A probe with N emission points renders N glows sharing the probe's pulse parameters — multi-city/federated probes compose without any implicit geometry between points. No post-processing pass — the glow is computed in-shader to respect the frame budget.

* [x] ~~**Custom reach-aura shader (GLSL).**~~ *Dropped — see Scope decision above. Not replaced by a reserved shader slot.*

* [x] **Propagation-arc shader slot (GLSL, reserved).** `packages/engine-3d/src/shaders/propagation.glsl` — great-circle arc shader wired into the engine from day one but inactive until multi-probe propagation data is available. `setPropagationEvents(events)` is already on the engine API from 99a; when the array is empty, no geometry is drawn and no fragment cycles are spent. This phase commits only to the plumbing; rendering comes when data arrives in a later phase.

* [x] **Raycaster for probe interaction.** Per-frame raycast on mouse-move identifies the closest probe point under the cursor. Hover: probe glow intensifies slightly; tooltip shows the emic designation (from the Content Catalog, Phase 94). Click: dispatches a `probe-selected` custom event to the framework shell, which opens the side panel.

* [x] **Data layer foundation.** `src/lib/api/client.ts` wraps the generated types (from `make codegen-ts`) with a typed fetch client. `src/lib/api/queries.ts` defines query functions using TanStack Query (Svelte adapter). Query keys follow a stable schema. Error handling: typed discriminated unions for success / refusal / network error.

* [x] **Refusal surface primitive.** `src/lib/components/RefusalSurface.svelte` — renders BFF HTTP 400 responses as methodological statements per Design Brief §5.4. Queries the Content Catalog (Phase 94) for the corresponding text. Renders semantic + methodological registers via Progressive Semantics (§5.7). First end-to-end consumer of the Content Catalog.

* [x] **Progressive Semantics primitive.** `src/lib/components/ProgressiveSemantics.svelte` — renders any entity in Dual-Register form. Semantic register primary; methodological register accessible via a compact badge affordance. ARIA-Expanded for transitions. Both registers always present in DOM; CSS controls prominence.

* [x] **Atmosphere route.** `src/routes/+page.svelte` — replaces the "Hello AĒR" landing.
  - Queries `/api/v1/probes` for active probes (structural data only: `probeId`, `language`, `sources`, `emissionPoints[]`). No reach data.
  - Queries `/api/v1/metrics` with `metricName=publication_hour` for the last 24 hours per bound source → client-side aggregation maps to per-probe `uPulseRate` (display logic only; BFF serves the window, the shader-uniform mapping is client-side)
  - Mounts the 99a engine into a full-viewport canvas
  - On probe click: side panel opens with probe metadata rendered via Progressive Semantics, including a methodological-register line stating that reach is unmeasured
  - Refusal surfaces render in the panel when queries fail on methodological gates

* [x] **Time scrubber component.** `src/lib/components/TimeScrubber.svelte` — horizontal slider for time-range selection. URL-persisted (`?from=...&to=...`). Keyboard-accessible (arrow keys, Home/End). Uses the base Button from Phase 98b.

* [x] **URL-state synchronization.** `src/lib/state/url.ts` — Svelte 5 Runes store that reads/writes state (`activeProbe`, `timeRange`, `resolution`, `viewingMode`) to the URL. §5.5 mandates deep-linkable state.

* [x] **WebGL2 fallback (live).** Extend the 99a static fallback with live content: list active probes, their language + emission-point labels (as text), their current publication rate. No reach text — consistent with the visual surface.

* [x] **Engine stories (live data).** Under the route-based story harness: one story with Probe 0 (emission points Hamburg + Berlin), one with three synthetic probes across continents (varied pulse rates, multi-point emission), one with the propagation slot fed synthetic arcs (for visual verification that the plumbing works).

* [x] **Integration test.** Playwright test on WebGL2-capable Chromium: load dashboard → verify 3D globe renders → verify terminator visible → verify Probe 0's emission-point glows are visible over Hamburg and Berlin → verify pulse animates → click the probe → verify the side panel opens with emic content from the Content Catalog → verify the trace appears in Tempo. Second test with WebGL2 disabled: verify the live text fallback shows.

* [x] **Performance verification (end-to-end).** Benchmark on 2021 M1 MacBook Air: 60fps sustained during orbit with Probe 0 visible (emission-point glows + terminator). Bundle size after this phase: shell + engine chunk + textures together ≤ 350 kB gzipped (shell remains ≤ 80 kB; engine chunk ≤ 250 kB; textures stream lazily and do not count toward initial paint). Frame times during scrubber interaction < 16 ms/frame.

* [x] **Accessibility verification (live).** Keyboard navigation: Tab moves through probes; Enter opens the selected probe's panel. Screen readers get a textual state description that updates on meaningful state changes. The live text fallback passes full WCAG 2.2 AA on its own.

* [x] **Arc42 update.** Extend §8.17 with the live-data wiring: typed client, TanStack Query boundaries, Refusal Surface and Progressive Semantics placement in the component tree.

* [x] **Validation.** `make fe-check` green. All tests green. `make up` brings the full stack up; the atmosphere surface loads with live Probe 0 data. Lighthouse CI: first paint on 50 Mbps < 1.5s (shell paints first, engine and textures upgrade within ~3s). Story harness stories render correctly. Visual regression snapshots stable.

**Exit criteria:** A researcher can open the dashboard locally and see the 3D Earth rotating slowly, with Probe 0 rendered as gently pulsing luminous points over its emission origins (Hamburg and Berlin) — and nothing more; no reach is claimed. The terminator is live. Clicking a probe opens a side panel with its emic context from the Content Catalog, whose methodological register states that reach is unmeasured. The full pipeline — BFF → typed client → Svelte → engine → GLSL — is validated end-to-end. No Low-Fi yet (browsers without WebGL2 see a text fallback); that comes later, once there is more to reduce.


## Phase 100a: Surface I — Progressive Descent Mechanics (Single Probe) [P1] - [x] DONE

*With Probe 0 live on the Atmosphere from 99b, Phase 100a builds the L0→L4 descent on a single probe. This is where Design Brief §4 ("no layer replaces") becomes real: L1 orientation, L2 exploration controls, L3 analysis with uPlot, L4 provenance fly-out, View Transitions between layers, cross-layer keyboard navigation, and URL-state descent encoding. Multi-probe breadth and cross-layer a11y/performance verification come in 100b.*

* [x] **L1 Orientation overlay.** Soft top-bar overlay (fade-in on first mouse movement): current time range, active probe count, normalization mode, active cultural contexts (emic designations from the Content Catalog). Fade-out after 10s of inactivity. Design Brief §4.2 Atmosphere/L1 cell.

* [x] **L2 Exploration controls.** Time-range scrubber (built in 99b) becomes the L2 primary control. Add: resolution switch (5min / hourly / daily / weekly / monthly), pillar-mode toggle (Aleph/Episteme/Rhizome — but currently all three default to the same render, since Rhizome has no data and Episteme/Aleph differ only in time-window framing). Region zoom via mouse wheel / pinch.

* [x] **L3 Analysis companion panel.** On probe click, the panel that opens is no longer just metadata — it becomes the L3 Analysis view. Inside: a small time-series chart (using uPlot, framework-agnostic per §5.9) showing the selected metric for the selected probe over the selected time range. Uncertainty bands. Epistemic Weight styling from Phase 98. The 3D globe stays present behind the panel, dimmed to 30% opacity per §4.1 rule 2 ("no layer replaces").

* [x] **uPlot integration.** Add `uPlot` to the shell. Build a thin Svelte wrapper `src/lib/components/TimeSeriesChart.svelte` that renders a uPlot instance. Framework-agnostic at its core — the Svelte wrapper is < 30 lines, all actual rendering is uPlot's. Bundle impact: +40 kB gzipped, loaded when L3 activates (intent-based, starts preloading on probe hover).

* [x] **Descent animation via View Transitions API.** Descending from L0 to L3 (clicking a probe) uses the View Transitions API for a morphing transition: the probe point expands visually into the panel position; the globe fades behind; the panel's chart fades in. Ascent (closing the panel) reverses. Fallback for browsers without View Transitions: instant state change (still correct, just less elegant).

* [x] **L4 Provenance fly-out.** The L3 panel's "why this shape?" affordance at the chart's corner opens an L4 Provenance fly-out. Content: tier classification, validation status, known limitations, equivalence level — all from the existing `/api/v1/metrics/{metricName}/provenance` endpoint, rendered via Progressive Semantics. This is the first full use of the Content Catalog's methodological register at the layer where it dominates (per §5.7 "primary at Layer 4").

* [x] **Keyboard navigation across layers.** Tab cycles through visible probes. Enter descends to L3 on the selected probe. Escape ascends one layer at a time (L3 → L0). Shift+Tab at L0 focuses the overlay controls (L1 bar). This is not a bolt-on — it defines the invisible interaction grammar that §4.1 rule 1 ("each layer reachable in one interaction") requires.

* [x] **URL state extended.** Descent state encoded in URL: `?probe=probe-0-de-institutional-web&metric=sentiment_score&view=analysis`. Deep-linking into L3 works; refreshing the page preserves descent.

* [x] **Negative Space overlay toggle (structural only).** Add a keyboard shortcut (e.g. `Shift+N`) and a quiet UI affordance for the Negative Space overlay. In this phase the toggle exists and is hooked up to a Svelte 5 `$state` rune; the actual visual reweighting (Design Brief §3.4, §4.4) is deferred to a later phase along with the demographic-skew annotations from WP-003 §6. The toggle is visible but idempotent until then.

* [x] **Arc42 update.** Extend §8.17 with notes on the descent mechanism and the URL-state encoding. Record the View Transitions API dependency and its graceful-degradation behavior.

* [x] **Validation.** `make fe-check` green (lint + svelte-check + vitest 28/28 + build + bundle-size with the new L3 chart-chunk gate). Visual regression snapshots per layer state and the manual L0→L4 browser pass are deliberately deferred into 100b's Integration-tests block — they require the Playwright descent E2E suite as their carrier, which 100b is building anyway.

**Exit criteria:** A researcher can click Probe 0, descend into a time-series analysis view with provenance access, return to the atmospheric view, and share the state via URL. The fractal pillars (Aleph at L0, Episteme tightening at L2-L3) are observable in the interaction. Rhizome remains architecturally ready but latently invisible. Descent works end-to-end on a single probe; multi-probe composition and cross-layer a11y/performance gates land in 100b.


## Phase 100b: Surface I — Multi-Probe Composition & Cross-Layer Verification [P1] — SUPERSEDED

*Superseded on 2026-04-25 by the Iteration 5 reframing (Phases 101–116 below). The 2026-04-24 Reframing Note and the Iteration 5 Design Brief rewrite demoted Surface I to a landing overview and moved L3/L4/L5 off the globe onto Surfaces II and III. Multi-probe composition and cross-layer a11y/perf verification still happen — but they belong in the new phases (110 for Surface I refinement, 114 for a11y/perf, and incrementally inside each surface phase) rather than as a dedicated Surface I push. The visual-regression snapshotting of Phase 100a's L3/L4 companion panels is orphaned because those panels are deprecated by Phase 110; no snapshot baseline is captured for them.*


## Phase 101: Iteration 5 — Probe Dossier & Article Browsing Endpoints [P1] - [x] DONE

*Backend baseline A for Iteration 5. First of three backend phases that unblock Surface II Foundation and Silver access. Adds the eligibility-flag schema and the Probe Dossier composite endpoint plus the article-browsing surface that powers L5 Evidence.*

* [x] **PostgreSQL migration — eligibility flag.** Add `silver_eligible BOOLEAN NOT NULL DEFAULT false`, `silver_review_reviewer VARCHAR`, `silver_review_date DATE`, `silver_review_rationale TEXT`, `silver_review_reference VARCHAR` to `public.sources`. Seed Probe 0's two sources (`tagesschau.de`, `bundesregierung.de`) as `silver_eligible = true` with auto-eligibility rationale per Manifesto §VI and WP-006 §7. Migration tooling per Phase 29.
* [x] **Probe Dossier endpoint.** `GET /api/v1/probes/{id}/dossier` composite payload: probe etic classification, emic context, source list, per-source article counts (total + in-window), per-source publication frequency, function coverage (N/4 per WP-001 §5.1). OpenAPI update, handler, storage queries, unit + integration tests.
* [x] **Article browsing endpoint.** `GET /api/v1/sources/{id}/articles?start=…&end=…&language=…&entityMatch=…&sentimentBand=…&limit=…&cursor=…` — paginated article list with filters. Returns article IDs + light metadata.
* [x] **Article detail endpoint (L5 Evidence).** `GET /api/v1/articles/{id}` — Bronze cleaned text + Silver metadata + extractor provenance. Enforce k-anonymity gate: if the article's aggregation group < k = 10 documents for the referenced metric, return HTTP 403 with the methodological refusal payload naming the gate and linking to WP-006 §7.
* [x] **Contract + codegen.** OpenAPI diff committed; `make codegen` runs; two-style `$ref` convention per ADR-021.
* [x] **Arc42 update.** §5.1.4 notes the eligibility columns; §8.x adds the Probe Dossier and article endpoints.
* [x] **Validation.** `make lint && make test` green; integration tests cover k-anon gate triggering.


## Phase 102: Iteration 5 — View-Mode Query Endpoints + EntityCoOccurrenceExtractor [P1] - [x] DONE (2026-04-25)

*Backend baseline B. Delivers the view-mode query surface and the single CorpusExtractor that unblocks Network Science view modes (per ADR-020 §Backend-Work). Keeps the non-goal "no new per-document NLP extractors" intact; relaxes only to allow one corpus-level aggregation over existing Gold entity data.*

* [x] **ClickHouse migration.** Create `aer_gold.entity_cooccurrences` (window_start, window_end, source, article_id, entity_a_text, entity_a_label, entity_b_text, entity_b_label, cooccurrence_count, ingestion_version), `ReplacingMergeTree(ingestion_version)`, ORDER BY (window_start, source, entity_a_text, entity_b_text), 365-day TTL on `window_start`. Migration in `infra/clickhouse/migrations/`.
* [x] **EntityCoOccurrenceExtractor.** New corpus-level extractor in `services/analysis-worker/internal/extractors/entity_cooccurrence.py`. Reads `aer_gold.entities` per configured window + source, emits pairwise co-occurrence rows. Protocol-compliant with existing extractor pattern (but corpus-level, batched, NATS-triggered on the same schedule as per-document extractors). Idempotent via `ingestion_version`. Unit tests for windowing, pair enumeration, label-pair ordering, idempotency.
* [x] **BFF endpoints.** `GET /api/v1/metrics/{metricName}/distribution?scope=probe|source&scopeId=…&start=…&end=…&bins=…` (per-source histogram or raw-value arrays). `GET /api/v1/metrics/{metricName}/heatmap?xDimension=dayOfWeek|hour|source|entityLabel|language&yDimension=…` (2D binning). `GET /api/v1/metrics/correlation?metrics=m1,m2,m3&scope=…` (pairwise correlation matrix). `GET /api/v1/entities/cooccurrence?scope=…&topN=…` (co-occurrence nodes + edges). All endpoints accept `probeId` or `sourceId` with probe as default.
* [x] **Contract + codegen.** OpenAPI diff + `make codegen`.
* [x] **Arc42 update.** §5.1.4 notes the new Gold table and corpus-level extractor; §8.x describes the view-mode query endpoints.
* [x] **Validation.** `make lint && make test` green; integration tests cover each view-mode endpoint on Probe 0 fixture data.

## Phase 103: Iteration 5 — Silver Query Endpoints with Eligibility Enforcement [P1] - [x] DONE (2026-04-25, aggregation endpoints deferred to Phase 103b)

*Backend baseline C. Exposes Silver-layer access gated by the eligibility flag from Phase 101. Small phase, isolated because Silver has distinct governance review requirements.*

* [x] **Silver source filter.** `GET /api/v1/sources?silverOnly=true` returns only Silver-eligible sources. `GET /api/v1/sources/{id}` includes eligibility state + review metadata.
* [x] **Silver document endpoints.** `GET /api/v1/silver/documents?sourceId=…&start=…&end=…&limit=…&cursor=…` (paginated Silver documents: cleaned_text + SilverMeta summary). `GET /api/v1/silver/documents/{id}` (individual Silver document detail).
* [x] **Silver aggregation endpoints.** `GET /api/v1/silver/aggregations/{aggregationType}?sourceId=…&…` — Silver-layer distributional/heatmap/correlation queries analogous to Phase 102 but over Silver fields (token distributions, cleaned-text length distributions, raw entity counts pre-NER).
* [x] **Eligibility enforcement.** Every Silver endpoint verifies `silver_eligible = true` on the requested source; non-eligible returns HTTP 403 with refusal payload naming the review gate and linking to WP-006 §5.2. Dedicated integration tests for the refusal path.
* [x] **Contract + codegen.** OpenAPI diff + `make codegen`.
* [x] **Arc42 update.** §8.x adds the Silver endpoint block with the eligibility-gate semantics.
* [x] **Validation.** `make lint && make test` green.


## Phase 103b: Iteration 5 — Silver Aggregation Endpoints with Projection Table [P2] - [x] DONE (2026-04-25)

*Deferred from Phase 103 on 2026-04-25. Silver lives as individual MinIO JSON envelopes with no queryable index, so the aggregation endpoints listed in the original Phase 103 spec (`GET /api/v1/silver/aggregations/{aggregationType}`) have no real query path — implementing them on top of a per-request MinIO scan would be slow, MinIO-bound, and bounded only by ad-hoc per-request limits. This phase lands the aggregation endpoints together with a Silver-projection ClickHouse table so the queries can run as cheap GROUP BYs analogous to the Phase 102 Gold view-mode endpoints, while preserving Silver's distinct governance review (eligibility gate from Phase 103 still applies).*

* [x] **Silver-projection ClickHouse table.** New `aer_silver.documents` (or `aer_silver_projection`) table populated by the analysis worker at the same point Silver is uploaded to MinIO. Columns at minimum: `timestamp`, `source`, `article_id`, `language`, `cleaned_text_length`, `word_count`, `raw_entity_count` (pre-NER token-based count), plus `ingestion_version` for ReplacingMergeTree idempotency. 365-day TTL on `timestamp`. Migration in `infra/clickhouse/migrations/`.
* [x] **Worker write path.** Extend the analysis worker's Silver upload step (`internal/silver.py` and the processor) to compute the projection fields and bulk-insert one row per document into `aer_silver.documents` alongside the existing MinIO write. Idempotent via `ingestion_version`.
* [x] **BFF aggregation endpoints.** `GET /api/v1/silver/aggregations/{aggregationType}?sourceId=…&start=…&end=…&bins=…` — supported `aggregationType` ∈ `{cleaned_text_length, word_count, raw_entity_count}` for distributional queries; `{cleaned_text_length_by_hour, word_count_by_source}` for heatmaps; `{cleaned_text_length_vs_word_count}` for correlation. Reuses Phase 103's `requireSilverEligible` gate so non-eligible sources return the same 403 + RefusalPayload.
* [x] **Contract + codegen.** OpenAPI diff + `make codegen`.
* [x] **Arc42 update.** §8.x extends the Silver endpoints block to cover the projection table and the aggregation surface.
* [x] **Validation.** `make lint && make test` green; integration tests cover each aggregation type on Probe 0 fixture data.


## Phase 104: Iteration 5 — Content Catalog Expansion [P2] - [x] DONE (2026-04-25)

*Populates the Dual-Register content catalog (Phase 95 infrastructure) for Iteration 5's expanded surface. Pure content work — no new endpoints.*

* [x] **Dual-Register metric entries.** Semantic + methodological entries (en + de) for each current Gold metric (`word_count`, `sentiment_score`, `language_confidence`, `entity_count`, `publication_hour`, `publication_weekday`). Files under `services/bff-api/configs/content/{locale}/metrics/`.
* [x] **View-mode cell entries.** One entry per MVP cell per metric (three cells × six metrics = eighteen pairs) describing what that view mode shows and how to read it.
* [x] **Refusal entries.** One entry per refusal type: WP-004 equivalence gate, k-anonymity, missing validation, Silver non-eligible, pillar-unavailable, cross-source without equivalence. Each with alternative-action suggestions.
* [x] **Empty-lane invitations.** One entry per WP-001 function for the empty-lane case (what this function is, why it matters, what kind of source would populate it).
* [x] **Open Research Questions.** One entry per WP-001 §8, WP-002 §7, WP-003 §7, WP-004 §7, WP-005 §7, WP-006 §8 question — scope, relevant pipeline hooks, contribution invitation.
* [x] **"How to read the globe" primer.** Structured Markdown with inline interactive-parameter placeholders (rendered by Phase 109 Surface III).
* [x] **Content validation tests.** Schema-conformance, locale completeness (en + de parity), cross-reference integrity (every `[WP-xxx §y]` resolves).
* [x] **Arc42 update.** §8.x content catalog scope + cross-link map.
* [x] **Validation.** `make lint && make test` green; content catalog tests green.


## Phase 105: Iteration 5 — Navigation Chrome (Left Rail + Top Scope Bar + Methodology Tray Component) [P1] - [x] DONE (2026-04-25)

*The persistent three-part frame specified in Design Brief §3.2. Blocks every subsequent frontend surface phase — every surface renders inside this chrome. Content binding for the tray lands in Phase 108; this phase delivers the component, its stores, and URL-state wiring.*

* [x] **Left side rail component.** `src/lib/components/chrome/SideRail.svelte` — three surface anchors (Atmosphere / Function Lanes / Reflection), scope indicator (probe + time range + viewing mode), pillar-mode toggle (Aleph/Episteme/Rhizome), return-to-Atmosphere planet glyph. Keyboard-navigable; screen-reader-labeled; reduced-motion-aware. Integrated styling (not floating, not overlay).
* [x] **Top scope bar component.** `src/lib/components/chrome/ScopeBar.svelte` — slot-based component accepting per-surface navigation content (time window label + resolution + Neg-Space toggle on I; stubs for II and III).
* [x] **Methodology tray component (shell).** `src/lib/components/chrome/MethodologyTray.svelte` — closed-state vertical tab with tier-badge slot and "Methodology" label; open-state full-height panel; push-mode default with overlay-mode fallback at 900 px (breakpoint documented in `design_system.md` §7.1). Content binding is stubbed; filled by Phase 108.
* [x] **Shared stores.** `focusedMetric` store (`src/lib/state/metric.svelte.ts`, metricName + chartContext); `sourceId` URL param added to `url-internals.ts`; pillarMode continues as `?viewingMode=` (already URL-backed). URL-state serialization via SvelteKit search params; deep-link restores all state.
* [x] **`design_system.md` update.** §7 Navigation Chrome Primitives: tokens for rail/tray widths + push-overlay threshold, component specs for SideRail/ScopeBar/MethodologyTray, route-group note.
* [x] **Story routes + gates.** `/stories/chrome/side-rail`, `/stories/chrome/scope-bar`, `/stories/chrome/methodology-tray` added to stories layout; story index updated. Visual regression baselines and axe gate: Playwright E2E via `make fe-test-e2e`.
* [x] **Arc42 update.** §8.17 "Navigation Chrome" paragraph added (SideRail, ScopeBar, MethodologyTray, route group, focused-metric store).
* [x] **Validation.** `make fe-check` green (lint, typecheck, 28/28 unit tests, build, bundle-size gate).


## Phase 106: Iteration 5 — Surface II Foundation (Probe Dossier + Function-Lane Shell) [P1] - [x] DONE (2026-04-26)

*The Probe Dossier route and the function-lane shell. Delivers the landing-into-Surface-II experience; the view-mode matrix lands in Phase 107 on top of this foundation. Depends on Phases 101 (endpoints) and 105 (chrome).*

* [x] **Routes.** `/lanes/:probeId/dossier` as Surface II's default landing when a probe is selected. `/lanes/:probeId/:functionKey` per-function-lane route.
* [x] **Probe Dossier component.** Consumes `/api/v1/probes/{id}/dossier`. Renders source cards with per-source counts, publication frequency, etic classification, emic context. Function coverage indicator (N/4) with per-function status. Navigable article preview per source.
* [x] **Function-lane shell.** Four lane slots matching WP-001's taxonomy. Baseline uPlot time-series view per lane fed from `/api/v1/metrics?scope=…`. Empty-lane Dual-Register invitations drawn from content catalog (Phase 104). Fifth-lane slot reserved but empty (Brief §8.1 extensibility).
* [x] **Source-scope narrowing.** Clicking a source card in the Dossier propagates `sourceId` into the URL + scope store; subsequent lane views query at source scope. Scope indicator in the left rail reflects probe vs. source scope.
* [x] **L5 Evidence reader-pane component.** Modal overlay opening from article clicks; renders Bronze cleaned text + trace metadata; handles HTTP 403 k-anon refusal gracefully with the methodological panel.
* [x] **Arc42 update.** §8.x Surface II architecture; probe scope vs. source scope propagation rule.
* [x] **Validation.** `make fe-check` green; Playwright E2E: globe → Dossier → lane → article flow.


## Phase 107: Iteration 5 — View-Mode Matrix (MVP Cells) [P1] - [x] DONE

*Implements the analytical-disciplines × presentation-forms catalog with three MVP cells per metric. Depends on Phases 102 (endpoints), 105 (chrome — view-mode switcher slots into the top scope bar), and 106 (function-lane shell).*

* [x] **Matrix-cell registry.** `src/lib/viewmodes/` — typed cell definitions; catalog fetched from backend content API; no hardcoded cell list in frontend source (Brief §8.3).
* [x] **NLP × time-series cell (uPlot).** Default for numeric metrics. Uncertainty bands. Epistemic Weight treatment (`design_system.md` §4).
* [x] **EDA × ridgeline/distribution cell (Observable Plot).** Per-source distributional view consuming `/api/v1/metrics/{name}/distribution`.
* [x] **Network Science × force-directed graph cell (D3-force).** Entity co-occurrence network consuming `/api/v1/entities/cooccurrence`. Nodes sized by frequency; edges weighted by co-occurrence count; coloring by sentiment or by source.
* [x] **View-mode switcher.** Slots into the top scope bar (Phase 105). URL-state carries selected cell id; deep-link restores.
* [x] **Scope parameter wiring.** Every view-mode query uses `probeId` or `sourceId` from the scope store; switcher behavior unchanged across scope modes.
* [x] **Tests.** Per-cell rendering stories; axe + visual regression; integration test of scope-select → view-mode-select → render path.
* [x] **Arc42 update.** §8.x View-Mode Matrix with the MVP cell list and the extensibility contract.
* [x] **Validation.** `make fe-check` green.


## Phase 108: Iteration 5 — Methodology Tray Content Binding [P1] - [x] DONE

*Wires the tray component from Phase 105 to live content. The tray becomes the L4 Provenance surface reachable from every metric on every surface. Depends on Phases 104 (content), 105 (tray component), and 107 (metric focus is set by view-mode interactions).*

* [x] **`focusedMetric` subscription.** Any chart, lane, or Dossier interaction sets the focused metric; tray updates in place without separate interaction.
* [x] **Content flow.** Parallel fetches of `/api/v1/content/metric/{name}?locale=…` and `/api/v1/metrics/{name}/provenance`. Content rendered using `design_system.md` Epistemic Weight classes.
* [x] **Closed-state binding.** Tier badge reflects live validation status; known-limitations indicator dot appears when any limitation applies to the current view.
* [x] **Open-state binding.** Dual-Register rendering — methodological register primary per Brief §7.7. Known-limitations-first mode activates when Negative Space overlay is on (Phase 113).
* [x] **Chart-level focus.** Clicking a specific time point, source, or entity scopes tray content to that selection and exposes only the relevant provenance.
* [x] **"Read the full Working Paper" deep link.** Anchor into `/reflection/wp/{id}?section=…` — target route rendered in Phase 109; link works before then (stub target).
* [x] **Tests.** Tray opens with correct content per focused metric; push→overlay fallback at narrow viewport; known-limitations-first mode under Negative Space.
* [x] **Arc42 update.** §8.x Methodology Tray content binding contract.
* [x] **Validation.** `make fe-check` green.

**Notes.** Replaces the legacy `L4ProvenanceFlyout` (deleted) per the rework guidance — the tray is now the single L4 Provenance surface. Closed-state tier mapping is implemented in `methodology-tray-internals.ts` (collapses tier-2-unvalidated onto the tier-1-unvalidated visual; routes `expired` validation to the expired badge regardless of tier). Negative-space limitations-first mode is implemented as a CSS `flex order` reshuffle so the methodological register stays in the DOM (Brief §5.7). Working-Paper deep links emit `/reflection/wp/{id}?section=…` from the first `workingPaperAnchors` entry — Phase 109 materialises the route. Negative-space and tray-open state migrated from the per-page `+page.svelte` rune into a shared `$lib/state/tray.svelte.ts` store so the (app)-layout-mounted tray and the Surface I scope-bar toggle share state without prop-drilling.

### Phase 108 follow-up (deduplication of Surface I vs Surface II vs tray)

* [x] **Push-mode coexistence with the L3 SidePanel.** `SidePanel.svelte` honours `--tray-right-edge`, so opening the tray narrows the L3 panel (instead of being hidden behind it). Tray `z-index` raised to 1100 so it remains reachable in overlay-mode (<900 px) too. Resolves: clicking "Methodology" inside the L3 panel previously appeared to do nothing because the tray slid in behind the dashboard-sized SidePanel.
* [x] **Slim L3 panel to a Surface II launchpad.** `L3AnalysisPanel.svelte` rewritten as a landing teaser: emic semantic paragraph (one register, not flippable), structural meta (probe / language / sources / pub rate), quick-jump tiles into Surface II (Probe Dossier + the four WP-001 Function Lanes), an explicit "Open methodology tray →" affordance, and the Brief §5.7 reach disclaimer. Removed: metric selector, per-probe `TimeSeriesChart`, `ProgressiveSemantics` (the methodological-register flip in the Phase 100a panel was the source of the duplication the user reported against the tray). No information lost — analysis lives on Surface II; methodological text lives in the tray. `+page.svelte` no longer passes `windowStart` / `windowEnd` / `resolution` to L3.
* [x] **Function Lane chrome left intact.** The `cell-semantic` paragraph in `FunctionLaneShell` renders the **view_mode** content register (e.g. "what a histogram-of-values cell shows"), not the **metric** content register that the tray binds to — different content-catalog entities, no duplication. Decision recorded here so a future cleanup pass doesn't strip it on appearance.
* [x] **Validation.** `make fe-check` green after the rewrite (50 unit tests, lint, typecheck, build, bundle-size budget).

## Phase 109: Iteration 5 — Surface III (Reflection) [P1] - [x] DONE

*The primary methodological surface — prose + inline interactivity + primers + open-research-questions hub. Depends on Phases 104 (content) and 105 (chrome).*

* [x] **Route tree.** `/reflection` (landing with WP index + primer + open-questions entry), `/reflection/wp/:id` (Working Paper), `/reflection/probe/:id` (Probe Dossier methodology view), `/reflection/metric/:name` (Metric provenance page), `/reflection/open-questions` (Open Research Questions hub), `/reflection/primer/globe` ("How to read the globe").
* [x] **Working Paper rendering.** Runtime markdown renderer (`src/lib/reflection/md.ts`) fetches WP files from `static/content/papers/` (copied from `docs/methodology/en/`). Handles the full GFM subset used in the 6 WPs: h2–h4, paragraphs, tables, fenced code, blockquotes, ul/ol, HR. Frontmatter parsed; H1 extracted as title.
* [x] **Inline interactive cells (Distill-style).** `InlineChart.svelte` embeds Observable Plot cells in WP prose. WP-002 §3 gets `sentiment-window-demo` (7/30/90-day window over live Probe 0 `sentiment_score`). `interactiveCells` field in `papers.ts` declares which sections get charts.
* [x] **Cross-reference resolution.** `[WP-001 §3]` bracket syntax and bare `WP-NNN §N` prose patterns both resolve to `/reflection/wp/wp-nnn?section=N`. `crossRefHref()` utility exported from `md.ts` for use in other modules (e.g., WP anchor links in the metric provenance page).
* [x] **Entry points.** Methodology tray "Read the full Working Paper" (Phase 108), metric badges → `/reflection/metric/:name`, probe dossier → `/reflection/probe/:id`. All entry routes rendered and navigable.
* [x] **Tests.** `tests/unit/reflection-papers.test.ts` — 44 unit tests covering `renderPaper`, `renderInline`, `crossRefHref`, papers catalog (`getAllPapers`, `getPaperMeta`, `paperContentUrl`), and open-questions catalog (`OPEN_QUESTIONS`, `questionsByWp`, `getOpenQuestion`). All pass.
* [x] **Open Research Questions.** Complete catalog of all 50 questions faithfully transcribed from WP §7/§8 sections across all 6 papers, with `deliverable` and `pipelineHook` fields. Rendered in a grouped hub at `/reflection/open-questions`.
* [x] **Validation.** `make fe-check` green (TypeScript + ESLint + Prettier). Unit tests: 44/44 pass.


## Phase 110: Iteration 5 — Surface I Refinement (Probe-First Emission + Source Satellites) [P1] - [x] DONE

*Updates the 3D engine from source-first to probe-first emission, adds read-only source satellites, and introduces Progressive Semantics on probe glyphs. Deprecates Phase 100a's source-click descent. Depends on Phase 101 (dossier endpoint for satellite data) and Phase 106 (Surface II Dossier as descent target).*

* [x] **Engine update.** `packages/engine-3d/` emits one glyph per probe (not per source). Source satellites as secondary geometry — smaller, muted, non-selectable as scope targets. Raycaster filters selection to probe glyphs.
* [x] **Satellite interaction.** Hovering a satellite opens a tooltip naming the source. Clicking a satellite routes to `/lanes/:probeId/dossier?sourceId=…` (Probe Dossier with the source pre-filtered). Never changes scope to "source-only on Surface I".
* [x] **Progressive Semantics on glyphs.** Semantic register prominent on hover (plain-language identity); obvious affordance expands methodological register (etic/emic classification per Brief §4.5).
* [x] **Primer link.** Surface I's top scope bar (Phase 105) gains a link to `/reflection/primer/globe`.
* [x] **Visual regression update.** New baselines for probe-first emission; old source-click baselines deleted (Phase 100a's L3/L4 panel baselines are orphaned per the supersession note above). No new probe-glyph/satellite snapshots are baked — additive-WebGL output is hardware-dependent under headless rendering; engine coverage stays at the Vitest unit level (see §8.15).
* [x] **Arc42 update.** §8.x Probe-First Emission + Source Satellite Presentation. Note that Phase 100a's source-click descent is deprecated.
* [x] **Validation.** `make fe-check` green; Playwright E2E: globe → probe selection → Dossier flow green end-to-end.

## Phase 111: Iteration 5 — Silver-Layer Toggle on Surface II [P2] - [x] DONE

*Exposes Silver-layer access as a data-source toggle. Depends on Phase 103 (Silver endpoints) and Phase 106 (Surface II Foundation).Check docs/design/reframing-note.md, docs/design/design_brief.md or ADR-20 in docs/arc42/09_architecture_decisions.md if more information is needed, but only if necessary.*

* [x] **Toggle component.** Gold / Silver data-source toggle on Surface II (location per Brief §9.1 — top scope bar or Dossier header; pick during implementation). URL state carries toggle.
* [x] **Eligible routing.** When toggle = Silver and active source is eligible, view-mode queries route to `/api/v1/silver/*`. Same matrix cells render over Silver data without cell-level rewrites.
* [x] **Non-eligible rendering.** When toggle = Silver and active source is NOT eligible, an explicit "not Silver-eligible" panel renders with methodological context drawn from content catalog and a link to WP-006 §5.2. No silent omission.
* [x] **Scope interactions.** Narrowing to a different source re-evaluates eligibility; panel updates accordingly.
* [x] **Tests.** Toggle interaction; eligible vs. non-eligible routing; refusal panel rendering; URL-state round-trip.
* [x] **Arc42 update.** §8.x Silver-Layer Toggle + the eligibility-gate UX contract.
* [x] **Validation.** `make fe-check` green.


## Phase 112: Iteration 5 — Negative Space Overlay [P2] - [x] DONE

*"What AĒR doesn't see" toggle wired across all surfaces per Brief §4.4 and §5.4. Depends on Phases 106 and 108. Demographic-opacity layer (WP-003 §6) annotations on Surface II charts. Coverage-map foregrounding on Surface I. Known-limitations-first mode for the methodology tray. This is also part of an open research question but for now we follow the docu.*

* [x] **Toggle component.** Negative Space toggle in the left rail (Phase 105). Persistent visible state indicator. URL state carries overlay.
* [x] **Surface I behavior.** Absence regions become prominent (positively marked unmonitored areas); coverage map (WP-001 §5.3) becomes legible per region.
* [x] **Surface II behavior.** Empty lanes gain prominence; charts gain demographic-skew annotations in the margin from WP-003 §6.1 content entries.
* [x] **Surface III behavior.** Absence-prose scrolls into the margin on Working Paper views.
* [x] **Methodology tray behavior.** Known-limitations-first mode (bumps limitations to the top of tray content) activates when overlay is on — wired in Phase 108; verified here end-to-end.
* [x] **Tests.** Toggle interaction; per-surface rendering differences; URL-state round-trip; axe gate passing with overlay on.
* [x] **Arc42 update.** §8.x Negative Space Overlay behavior matrix.
* [x] **Validation.** `make fe-check` green.


## Phase 113: Iteration 5 — Bug fixing [P2] - [x] DONE
*This phase covers all bugs and issues found in Iteration 5 implementation phases. If major changes are required or structural/architectural adjustments are necessary whe have to document it in the ADR-20 that covers the dashboard implementation*

* [x] **Bug 1 - Backend: Medaillon Data** The dashboard shows way less Gold-Metric sources then silver sources. I think the NLP pipeline or something else is failing silently a lot and fails to extract metrics for a lot of silver data. We also need to check if bronze to silver transition is working or if we losing articles.
* [x] **Bug 2 - Dashboard: A E R Buttons** The bottom left "A", "E", "R" Buttons seem to have no functionality at all. Did we miss something? Or are they deprecated?
* [x] **Bug 3 - Dashboard** Left side rail with globe, lane, reflection buttons: 1. We have a duplicate button for globe view (top one seems nice since its colored). 2. Those buttons (including neg space and A, E, R if we keep it) are essential but have no labels. The user does not know what he clicks and they are "invisible". We need to make them more visible without growing the rail to much.
* [x] **Bug 4 - Dashboard** The resolution dropdown needs to be fully darkmode. Its background and options are currently white.
* [x] **Bug 5 - Dashboard** Right side rail (methodology) shows a red hint which (validation is expired) on probes with expired validation. It is cut off since it is to wide on the collapsed rail. The Methodology label already shows a red dot which could be enough. The validation expires hint should display only inside the rail when it is expanded.
* [x] **Bug 6 - Dashboard** When clicking a Probe and switch to Lanes view (http://localhost:5173/lanes/probe-0-de-institutional-web/dossier) we have several issues: 1. bundesregierung source card shows NO article counts on the "View articles" button. 2. All cards "view" source button on the article list returning an error "Failed to load article. Check network connectivity.". 3. All cards show a 404 Not found when clicking on the "Dossier" button.
* [x] **Bug 6 - Dashboard** The codebase uses the domainlanguage "Surface", "Layer" etc. but the Dashboard does not really reflect this. Only in on hover tooltips. Its a bit confusing because a new user reading the docu does not really see if he is on a surface or on a layer or whatever.
* [x] **Bug 7 - Dashboard**: All cards "view" source button on the article list returning "Failed to load article (HTTP 500). Check network connectivity". Also important (!): The source cards are not wide enough so they need to be horizontally scrolled to even view the "View"button which "hides" it for the user. Make it wider to fit.
* [x] **Validation.** `make fe-check` green.


## Phase 113b: PostgreSQL ↔ ClickHouse Retention Divergence (Critical) [P0] - [x] DONE

*Discovered while fixing Phase 113 / Bug 6. The Postgres `documents` and `ingestion_jobs` rows are pruned on a shorter horizon than MinIO Silver and ClickHouse Gold, so the BFF endpoints that key off Postgres (`GetSourceArticles` count joins, `ResolveArticle` for L5 article detail) become inconsistent with the analytical layer. Today: Postgres has 45 tagesschau docs and 0 bundesregierung docs while ClickHouse Gold has 195 + 25 articles for the same window. Symptom: dossier under-reports counts and `View article` 404s on any article whose Postgres row has been retention-deleted, even though the Silver envelope and Gold metrics still exist. Treat existing Bronze + Silver + Gold + Postgres data as authoritative — do **not** re-ingest, truncate, or reset.*

* [x] **Reconciliation script.** `scripts/reconcile_documents.py` walks MinIO Silver (Silver shares the bronze object key, and the SilverEnvelope already exposes source + article_id), inserts missing rows into `documents` under `ON CONFLICT (bronze_object_key) DO NOTHING`, creates synthetic `ingestion_jobs` rows tagged `status='reconciled'` pinned to 00:00 UTC of each source-day, and repopulates `aer_silver.documents.bronze_object_key` for rows that pre-date Migration 013. Idempotent.
* [x] **Schema decision (ADR-022).** Option (b): article-resolution source-of-truth moves to the analytical layer. ADR-022 documents the SoT split — Postgres `documents` becomes an operational soft cache; `aer_silver.documents` (already 365-day TTL, one-row-per-document) gains `bronze_object_key` and is the SoT for `(article_id) → (bronze_object_key, source)`. Postgres retention stays at 90 days.
* [x] **BFF article resolution path.** `DossierStore.ResolveArticle` queries `aer_silver.documents` instead of Postgres `documents`. Empty-string `bronze_object_key` (legacy rows pre-Migration-013) is treated as not-yet-repopulated. Integration test `TestResolveArticle_FromSilverDocuments_NoPostgresRow` (and companions) asserts resolution succeeds with no Postgres pool wired in at all.
* [x] **Dossier counts.** `DossierStore.FetchSources` reads per-source totals, in-window totals, and publication-frequency-per-day from `aer_silver.documents FINAL` over the same 365-day analytical horizon as the view-mode endpoints. Postgres still supplies source metadata (name, type, URL, classification, Silver-eligibility); only the count columns moved.
* [x] **Retention runbook.** `docs/operations_playbook.md` §"Retention & Reconciliation (Phase 113b / ADR-022)" documents the retention table for every layer plus the reconciliation procedure (inspect → dry-run → execute → confirm).
* [x] **Validation.** `make lint` and `make test` green (Go integration tests, Go pkg, Go crawler, Python unit tests, frontend Vitest). Three new `dossier_store_test.go` integration tests cover ResolveArticle from Silver without a Postgres row, the empty-key fallback, and source-count aggregation against `aer_silver.documents`. Runtime reconciliation against the dev stack remains an operational step (see runbook).


## Phase 113c: Iteration 5 — Bug fixing [P2] - [x] DONE
*This phase covers bugs and structural issues found in Iteration 5 implementation phases. Structural changes are documented in ADR-020 (§Implementation-Outline · Phase 113c).*

* [x] **Bug 1 — Visual rework of probe descent flow: CRITICAL.** The pre-113c probe click on the Atmosphere globe opened an in-page L3 SidePanel ("flyout", URL e.g. `/?probe=…&viewingMode=aleph&metric=sentiment_score&view=analysis`). The flyout duplicated content the Probe Dossier already owns and contradicted Brief §4.1 ("the globe is the welcome mat, not the working surface"). Rework the flow so it descends one layer at a time, with coherent labeling per Brief §5.2:
  1. **Surface I · L0 Atmosphere (Globe)** — entry point.
  2. **Surface II · L1 Probe Dossier** — landing on probe click. Centralizes the WP-001 function selection (EA Epistemic Authority · PL Power Legitimation · CI Cohesion & Identity · SF Subversion & Friction) as a prominent four-tile selector. No metric and no Au Gold / Ag Silver controls (they have no function on L1). Built as a collapsible per-probe section so future N-probe dossiers can list multiple probes; for Probe 0 the section is expanded by default and occupies the page.
  3. **Surface II · L3 Function Lane** — the project's analytical core. The LensBar above the chart hosts four equally-weighted groups: **Function** (EA / PL / CI / SF), **Layer** (Au Gold / Ag Silver), **Metric**, **View**. The chart and view-mode body keep the dominant share of the viewport. A prominent "Read the full Working Paper →" anchor in the lane header descends to Surface III.
  4. **Surface III · L3 Working Paper** — long-form methodology. Arrived-from-lane state carries `?from=lane&probe=…&fn=…` so a "Back to Function Lane" affordance can render.

  Notes: with these labels the descent reads L0 → L1 → L3 → L3 (Surface III's L3 is its own analytical-native layer per Brief §5.2). L2 remains reserved for in-surface exploration controls (Surface I time scrubber, Surface II source-scope narrowing). Old `/?probe=X[&view=analysis…]` deeplinks redirect to the canonical Dossier route. See ADR-020 §Implementation-Outline · Phase 113c.
* [x] **Step 6 — Back-and-forth navigation: CRITICAL.** Every node in the flow must be reachable from every other node without dead ends. SideRail anchors (planet → `/`, Function Lanes → `/lanes/{activeProbe}/dossier`, Reflection → `/reflection`); ScopeBar probe label and lane tabs as quick switches; Working Paper page renders a "Back to Function Lane" affordance when arrived via `?from=lane&probe=…&fn=…`. Browser back works natively.
* [x] **Step 7 — URL deeplink alignment: CRITICAL.** Each route's URL state restores exactly: `/` · `?probe=X` redirect; `/lanes/{probeId}/dossier?sourceId=X`; `/lanes/{probeId}/{functionKey}?metric=…&viewMode=…&layer=silver&sourceId=…`; `/reflection/wp/{id}?section=…&from=…&probe=…&fn=…`. Drop the now-meaningless `view=analysis` writes; LensBar Function navigation preserves query params across path change.
* [x] **Validation.** `make fe-check` green.


## Phase 113d: Iteration 5 — Bug fixing [P2] - [x] DONE
*This phase covers bugs and structural issues found in Iteration 5 implementation phases. Structural changes should get documented in ADR-020 (§Implementation-Outline · Phase 113d).*

* [x] On Surface I · Atmosphere · L0 Globe:
1. Since we do not choose metrics here, the methodology right sidedrawer here does not make sense. This should ONLY be visible where metric selection happens: Surface II · Function Lanes · L3 Function Lane. Remove it.
2. The top bar: Remove the from-to time "2026-04-20 16:48Z → 2026-04-27 16:48Z" section, it is already visible in the timeScrubber at the center bottom. Add a description to the TimeScrubber. User needs to know what it does and why it is there.
3. Move the "Resolution" into the TimeScrubber and also provide a description on what this does and why it is there. Make the TimeScrubber component wider if necessary and do not increase height.
4. Increase visibility of the 3 Pillars (bottom left bar).
5. Increase visibility of the Negative Space overlay button (bottom left bar) by writing it out instead of "NS".
6. Top left bar is the most important navigation except from globe clicks on the probe. Increase visibility of the buttons and do not cramp them at the top of the par. Do not obscure to much view on the globe.
7. On click on a source, Surface II · Function Lanes · L2 Probe Dossier opens with only this source visible. A better approach would be to show the normal probes dossier with all sources, but automatically NARROW SCOPE for the selected source, highlight it with a different text in the tooltip "Filtered to source bundesregierung from a satellite click on the globe. Show all sources" so it becomes visible! (Also remove the show all sources link button).

* [x] On Surface II · Function Lanes · L2 Probe Dossier after clicking the probe on the globe:
1. Since we do not choose metrics here, the methodology right sidedrawer here does not make sense. This should ONLY be visible where metric selection happens: Surface II · Function Lanes · L3 Function Lane. Remove it.
2. The probe collapsable/expandable container should be wider (same right border distance as left) so the whole view is used. We have the place for it now, since the methodology right sidedrawer is gone.
3. The probes introductional/description text needs to be almost as wide as the container to save vertical space.
4. Remove Button "Open methodology tray".
5. Decrease the height of discourse function cards a LITTLE bit to save vertical space.
6. The Sources itself should ALSO be inside a expandable/collapsable container just like the parent Probe currently has.
7. Remove the "Reach is not rendered on the globe.." text and place it as a small opaque text at the very center bottom of the globe view.
8. Remove the discourse function buttons from the top bar as they are now visible as cards in the probe container below.
9. The overall Publication rate 97.3 docs/day, as well as the source individual pub rates (bundesregierung 25.0 / day and tagesschau 72.3 / day) seem to be wrong. But maybe thats an artefact of manual crawling. *(Data artifact — no code fix.)*
10. When clicking "narrow scope" on a source, there seems to be an anchor getting triggered, jumping back to the top of the page, which makes the user lose focus when scrolled down to even see the button. Fix it.
11.  When clicking "narrow scope" on a source it seems complicated to reach the Surface II · Function Lanes · L3 Function Lane, since it is only possible to reach via clicking on a discourse function but the user does not know directly whih function to click (its always the first one if many), so it would be better to directly open the corresponding Lane view with the correct discourse function to jump right into it.
12. ~~**NOT IMPLEMENTED**~~: Multi-source narrowing implemented in Phase 113e.

* [x] On Surface II · Function Lanes · L3 Function Lane:
1. Remove the discourse function buttons from the top bar as they are now visible in the selection section above the data analysis view.
2. Discoursce function selection: when selecting a discourse function that has no source active, i cant go back to the previous view to select another discourse function easily. This should not happen i need a quick way to get back from the "This lane monitors sources that construct and reinforce shared collective identity — ..." view.
3. The metrics selection has a bug where a selected metric always jumps to the second place in the metric buttons
4.  **NOT IMPLEMENTED**: Selections: Metric, Layer or view mode selections make the whole page seem to reload so the view flickers everytime i select something. Can we fix this so it looks seaminglessly?
5. When chosing a discourse function i always want to see clearly visible which sources belong to this function so i can see what data i am looking at! Should also work with narrowing source. Currently it is visible above the data view below but highlight it a little bit more.
6. There need to be a description text for each viewmode. For now source independent but should explain in a view words exatly how to read the current view and here we should hint on the methodology section.
7. The user does not get informed about Layers (gold, silver). As well as the Functions and  Metrics description in the top section of the view feels out of touch with the below selection of Functions, Metrics. Description of everything needs to be happening where it is SELECTED (Functions, Metrics, Layer, view), and where it is DISPLAYED (Description of how to read the current selected viewmode together with the current selected metric, see point 6).
8. At the top right is the selected view mode and selected metric visible (e.g. Time series · sentiment_score). Improve the visibility a little, maybe color it.


---

## Phase 113e: Iteration 5 — Multi-Source Analysis & LensBar Refinement [P2] - [x] DONE

*Implements the multi-source narrowing deferred in Phase 113d item 12, fixes the clear-scope bug introduced by the replaceState/SvelteKit page.url mismatch, and refines the LensBar into a two-row stable/dynamic layout.*

* [x] **Bug: clear scope does not work after browser-back navigation.** `dossier/+page.svelte` had a `$effect` that re-synced `?sourceId=` from `page.url.searchParams` into `urlState`. Because `setUrl()` uses raw `history.replaceState` (bypassing SvelteKit's router), `page.url` never updated after a clear — causing the effect to immediately re-apply the old `sourceId` and undo the clear. Fixed by wrapping the `url.sourceIds` comparison in `untrack()` so the effect only re-runs on real SvelteKit navigations, not on in-memory state writes.

* [x] **Scope action bar (multi-source selection UX).** Replaced the immediate `goto()` on "Narrow scope" click with a two-step flow: clicking a source card toggles it into `sourceIds` (visual highlight only); a sticky `scope-bar` at the bottom of `ProbeDossier` appears when any sources are selected, showing removable chip badges for each source, a "Clear" button, and an "Analyze ›" CTA that navigates to the function lane. Supports selecting sources one by one before committing to analysis. `probeId` prop removed from `SourceCard` (no longer needed).

* [x] **Multi-source, cross-function analysis.** `FunctionLaneShell.laneSources` previously filtered by `primaryFunction === functionKey`, so sources from other discourse functions were silently dropped from the view. When `sourceIds.length > 0` (manual selection), the filter is bypassed: all explicitly selected sources are shown in any function lane regardless of their function assignment. `TimeSeriesCell` already iterates the `sources` prop directly — no backend change needed for the primary time-series view. Distribution and co-occurrence views continue to use `scope=probe` for multi-source (BFF has no subset-scope endpoint); scoped distribution for an arbitrary source subset requires a future backend `source[]` query parameter.

* [x] **LensBar: function has-selection indicator.** `FunctionLaneShell` derives `activeFunctionKeys` — the set of discourse functions represented by the current `sourceIds` selection (always includes the current lane's `functionKey`). Passed as a prop to `LensBar`, which applies a dashed accent border (`has-selection`) to any function button whose function appears in the selection but is not the currently viewed lane. Tooltip updated to "… — selected sources include this function". Clicking navigates to that lane with the full multi-source scope intact.

* [x] **LensBar: two-row layout.** Split the single flat flex row into two explicit rows separated by a thin horizontal rule: Row 1 — Function + Layer (stable cardinality, won't grow); Row 2 — Metric + View (will accumulate entries). Reduces overall height and gives the growing selectors their own dedicated space. Padding tightened to `space-2` per row.

* [x] **LensBar: metric description + description readability.** Metric group previously had no description. Added a `cellContentQ` (same TanStack key as `FunctionLaneShell`'s `viewModeContentQ` — zero extra requests) that surfaces the cell-semantic short text under the Metric buttons. `cell-semantic` paragraph removed from the `FunctionLaneShell` lane header (it now lives where it belongs — under the metric selection). All `lens-desc` elements: `font-size: 10px → var(--font-size-xs)` (12 px), `color: fg-subtle → fg-muted`, `max-width: 28ch` removed (descriptions use full group width).

* [x] **LensBar: Layer/View column alignment.** Changed `.lens-group` from `flex: 1 1 auto` to `flex: 1 1 0%; min-width: 160px`. Equal flex-basis ensures all groups start from the same size, so Layer (row 1 col 2) and View (row 2 col 2) land at the same horizontal position when the bar wraps.

* [x] **Validation.** `make fe-check` green.


## Phase 114: Iteration 5 — Multi-Source Subset & Multi-Probe Composition (Within-Context) [P2] - [x] DONE

*Lifts the BFF view-mode endpoints from a single `scopeId` to a `scopeIds[]` array so an arbitrary subset of a probe's sources, and an arbitrary set of probes, can drive distribution / heatmap / correlation / co-occurrence views. Completes the multi-source narrowing deferred in Phase 113e (item 12 follow-up: "scoped distribution for an arbitrary source subset requires a future backend `source[]` query parameter") and lands the parallel-context-stream rendering committed in Brief §4.2.2 and §1.3. **Composition, not comparison** (Brief §1.3) — every stream keeps its own baseline; nothing is placed on a shared cross-context scale in this phase. Cross-cultural normalization, equivalence gating, and refusal surfaces are explicitly out of scope and live in Phase 115.*

### Backend (BFF + OpenAPI)

* [x] **OpenAPI: scope arrays.** `services/bff-api/api/openapi.yaml` — extend the view-mode endpoints to accept repeated `sourceId` and/or `probeId` query parameters (or comma-separated `sourceIds`/`probeIds`) in addition to the existing single-value form. Affected: `GET /api/v1/metrics/{metricName}/distribution`, `GET /api/v1/metrics/{metricName}/heatmap`, `GET /api/v1/metrics/correlation`, `GET /api/v1/entities/cooccurrence`, `GET /api/v1/entities`, `GET /api/v1/languages`, `GET /api/v1/metrics`. Single-value form remains for backward compatibility. `make codegen` runs clean.
* [x] **BFF resolver.** Extend `DossierStore.ResolveSourceWithEligibility` (or add a sibling `ResolveScopeWithEligibility`) to expand `probeIds[]` via `ProbeRegistry` into the union of their sources, then merge with explicit `sourceIds[]`, deduplicate, and reject the request with a precise `RefusalPayload` if any source in the resulting set is not Silver-eligible (only relevant when the endpoint is Silver-gated).
* [x] **ClickHouse query shape.** Replace the `WHERE source = ?` clauses with `WHERE source IN (...)` across the four view-mode handlers. Re-verify the existing row caps still hold against the largest plausible scope (multiple probes × full retention window) and document the per-endpoint cap in the OpenAPI description.
* [x] **Per-stream segmentation.** Distribution and heatmap responses gain an optional `segmentBy=source|probe` flag. When set, the response is structured as `{ streams: [{ id, label, scopeKind, ...payload }] }` so the frontend can render parallel streams without re-querying. Default (unset) preserves the current aggregated single-payload shape.
* [x] **`/api/v1/entities/cooccurrence` for multi-scope.** Edge weights aggregate across the union of selected sources; node `presence[]` field is added so the frontend can render per-source incident shading without a follow-up call. No change to `aer_gold.entity_cooccurrences` schema (Phase 102) — the corpus extractor's per-article rows already aggregate cleanly across `source IN (...)`.
* [x] **Integration tests.** Testcontainers covering: (a) single-source request unchanged; (b) two-source subset within one probe; (c) two-probe union; (d) source-subset request that crosses probe boundaries (explicit hand-picked sources from different probes — Brief §4.2.4 makes both probe and source first-class scope parameters, so this must be valid); (e) Silver-eligibility refusal when any source in the set is ineligible.

### Frontend (Dashboard)

* [x] **URL state.** `services/dashboard/src/lib/state/url-internals.ts` — `sourceIds` already serialises (Phase 113d). Add `probeIds` to the same multi-value scheme with the same delimiter. Round-trip tests in `tests/unit/url-state.test.ts`.
* [x] **Multi-probe scope action bar.** Extend the Phase 113e scope bar in `ProbeDossier.svelte` so chips render both source and probe selections (visually distinct), with separate "Clear sources" / "Clear probes" affordances. The `Analyze ›` CTA navigates to the function lane carrying the union scope.
* [x] **Cross-probe entry point.** From the Atmosphere (Surface I), shift-clicking or multi-selecting probe glyphs adds them to a pending `probeIds[]` set; the existing descent CTA becomes a "Compose" affordance when more than one probe is selected. Single-click behaviour is unchanged (descent to that probe's Dossier).
* [x] **Parallel-context streams in lanes.** `FunctionLaneShell.svelte` and the per-cell renderers (`TimeSeriesCell`, distribution, heatmap, correlation, co-occurrence) consume the new `streams[]` shape when present. Streams render as **parallel context lines / panels with per-stream baselines and per-stream Dual-Register tooltips** (Brief §4.2.2). No shared cross-context axis. Color encoding distinguishes streams using a perceptually uniform, valence-free palette per `visualization_guidelines.md` §1–§2.
* [x] **Empty-lane behaviour with multi-scope.** When some probes in the union have no source covering the current discourse function, those probes' streams are present-but-empty per the empty-lane Dual-Register invitation (Brief §4.2.2 + §7.7). The empty stream is the question, not a hidden case.
* [x] **LensBar cross-probe indicator.** Phase 113e's `activeFunctionKeys` indicator extends to the union of all selected probes' sources. No new visual primitive — same dashed accent border, just driven by the union set.
* [x] **No normalization controls.** This phase explicitly does *not* expose `?normalization=zscore|percentile` toggles. A user who attempts to construct a cross-context absolute claim sees the standard composed view; the refusal surface for unvalidated cross-cultural normalization arrives in Phase 115.

### Documentation

* [x] **CLAUDE.md.** Update the BFF endpoint list to record the `sourceIds[]` / `probeIds[]` parameters and the optional `segmentBy` flag.
* [x] **ADR-020.** Append a sub-entry under §Implementation-Outline · Phase 114 recording (a) the scope-array lift, (b) the parallel-stream response shape, (c) the deferral of cross-cultural normalization to 115, and (d) the explicit "composition, not comparison" boundary that scopes this phase.
* [x] **Arc42 §6 (Runtime View).** Sequence diagram for a multi-probe distribution request: frontend → BFF → resolver → ClickHouse `IN (...)` → segmented response → parallel-stream rendering.

### Validation

* [x] **Validation.** `make lint && make test && make fe-check` green. Manual: the Phase 113e scope-bar flow extends cleanly to (a) two sources in one probe, (b) two probes, (c) hand-picked sources crossing probe boundaries — each case produces parallel streams in the lane with per-stream baselines and no cross-context axis.

### Why this is one phase, not two

Multi-source subset (113e's deferred backend lift) and multi-probe composition share **the same backend change** (`scopeId` → `scopeIds[]`), the same parallel-stream response shape, and the same frontend rendering surface. Splitting them would force two passes through OpenAPI, codegen, the BFF handlers, and the lane shell for what is one structural change. The frontend entry points differ (Dossier source-narrowing vs. globe multi-probe selection) but consume the same wire format.


## Phase 115: Iteration 5 — Cross-Cultural Analysis Foundations (WP-004) [P2] - [x] DONE

*Operationalises WP-004 §5–§6 — extends the **Metric Equivalence Registry**, the **per-source baseline table**, and the `?normalization` parameter that already exist from Phase 65, plus adds the refusal surfaces for unvalidated cross-context absolute-value comparisons (Brief §7.4 + §7.3) that the prior infrastructure phase did not implement. Built on top of the multi-probe parallel-stream rendering shipped in Phase 114. Cross-cultural analysis is **distinct from** multi-probe analysis: multi-probe is the rendering substrate (composition); cross-cultural is the methodological discipline that decides which composed views may carry an absolute claim, which require deviation framing, and which must be refused. The default dashboard view stays at WP-004 Level 1 (temporal patterns, language-independent) — Level 2 (z-score deviations) is the first composed-claim view; Level 3 (absolute values) is gated behind validated equivalence and never the default (WP-004 §6.3).*

*Cross-cultural ≠ multi-probe (clarification for ROADMAP readers).* Multi-probe rendering can be entirely within-cultural — a German institutional probe beside a German diasporic probe — and remain a valid composition. Cross-cultural analysis is the subset of multi-probe analysis where the composed contexts are drawn from **different cultural-linguistic frames**, and where the user's question implicates an absolute or deviation comparison rather than a within-context one. The architectural answer is the same equivalence registry and baseline machinery in either case; the trigger for Level-2/Level-3 gating is the user's request shape (`?normalization=zscore` etc.), not a heuristic on probe metadata.

*State going into this phase.* From Phase 65, the following already exist and must not be re-created:
- `aer_gold.metric_baselines` table (`metric_name, source, language, baseline_value, baseline_std, window_start, window_end, n_documents, compute_date`).
- `aer_gold.metric_equivalence` table (`etic_construct, metric_name, language, source_type, equivalence_level, validated_by, validation_date, confidence`) — the Phase-65 Etic/Emic-aligned design is retained; nothing in this phase changes that schema beyond the single column addition below.
- `?normalization=raw|zscore` query parameter on `GET /api/v1/metrics`, `…/distribution`, `…/heatmap`, `…/correlation`, with the existing baseline+equivalence existence gate.
- `equivalenceLevel` field on `GET /api/v1/metrics/available`.
- `scripts/compute_baselines.py` (Operations Playbook / Workflow 4 from Phase 71) for manual baseline computation.

This phase **extends** that foundation with: a single column addition to `metric_equivalence`, `percentile` as a third normalization mode, a cross-frame heuristic that turns the existing existence gate into a true cross-cultural refusal surface, automated baseline maintenance via NATS-cron, the equivalence-review workflow tables, the per-probe equivalence summary endpoint, and the full frontend treatment (control, refusal, deviation labelling, valid-comparisons panel).

### Schema

* [x] **ClickHouse migration: `metric_equivalence.notes`.** The existing Phase-65 `aer_gold.metric_baselines` and `aer_gold.metric_equivalence` schemas otherwise cover this phase's needs (verified 2026-04-30: `grep -r "notes" infra/clickhouse/migrations/ | grep equivalence` returned empty). Add a single new ClickHouse migration introducing `notes String DEFAULT ''` on `metric_equivalence`, holding the concise methodological-rationale summary referenced from the Operations Playbook. The full review record lives in the Postgres `equivalence_reviews` table (next bullet); this column travels with the ClickHouse row so the BFF can serve the methodology-tray rationale without a cross-database join.

  Files (use the next free migration index — verify with `ls infra/clickhouse/migrations/`):

  `infra/clickhouse/migrations/000NNN_add_metric_equivalence_notes.up.sql`:
```sql
  -- Adds a free-form prose field for the methodological-rationale text
  -- referenced by the Operations Playbook section "Granting metric equivalence
  -- (WP-004 §5.2)" introduced in Phase 115. The full review record (reviewer,
  -- date, working-paper anchor, full prose) lives in the Postgres
  -- equivalence_reviews table; this column carries a concise summary that
  -- travels with the row when it is read by the BFF, so the dashboard's
  -- methodology tray can render the rationale without a cross-database join.
  ALTER TABLE aer_gold.metric_equivalence
      ADD COLUMN IF NOT EXISTS notes String DEFAULT '';
```

  `infra/clickhouse/migrations/000NNN_add_metric_equivalence_notes.down.sql`:
```sql
  ALTER TABLE aer_gold.metric_equivalence
      DROP COLUMN IF EXISTS notes;
```

  Type-choice rationale: `String DEFAULT ''`, not `Nullable(String)` — ClickHouse advises against `Nullable` when the empty string carries the same semantic role, and an equivalence entry without a notes summary is valid (e.g. the temporal-Level grant for Probe 0 × Probe 1 in Phase 124, whose rationale is fully captured in WP-004 Appendix B and needs no additional paraphrase). The `IF NOT EXISTS` guard matches the existing migration idempotency pattern. No corresponding edit to `infra/clickhouse/init.sql` — migrations are the only authorised path for schema changes since Phase 7; `init.sql` is reconciled during the next consolidation sweep, not per migration.

  No other ClickHouse schema changes in this phase.

* [x] **Postgres: equivalence-review workflow tables.** Mirror the WP-006 §5.2 Silver-eligibility review pattern (Phase 103): new `equivalence_reviews` table holding the methodological review record (reviewer, date, rationale, working-paper anchor, full prose notes). Each `aer_gold.metric_equivalence` row carries a `validated_by` value that points into this Postgres table for the full review record; the ClickHouse `notes` column added above carries a concise summary for read-path display. New migration in `infra/postgres/migrations/`. Out-of-band review only — no in-band UI for granting equivalence.

### Analysis Worker (Baseline Computation)

* [x] **`MetricBaselineExtractor` (corpus-level).** New extractor implementing the existing `CorpusExtractor` protocol (CLAUDE.md §"Gold Layer: Extractor Pipeline"). **Promotes** the existing `scripts/compute_baselines.py` (Phase 71) into a NATS-triggered automated extractor on the same cadence as `EntityCoOccurrenceExtractor` (Phase 102). Reads `aer_gold.metrics` over a configurable rolling window, computes per `(metric_name, source, language)` mean and standard deviation, writes to `aer_gold.metric_baselines`. The standalone script is **retained** for ad-hoc operations: first-run on a new probe (Phase 124 uses the script explicitly), manual recompute after schema change, and Operations-Playbook walkthroughs. Both paths share the same underlying computation function; only the trigger differs.
* [x] **No equivalence emission.** `MetricBaselineExtractor` does **not** publish anything to `aer_gold.metric_equivalence` — equivalence is granted out-of-band via the Postgres `equivalence_reviews` workflow, not derived statistically. This boundary is the architectural expression of WP-004 §2.2: equivalence is a research question, not a computation.
* [x] **Tests.** Pytest unit + Testcontainers integration covering: empty-corpus → no baseline written; single-source corpus → baseline emitted; idempotent re-run via the existing ReplacingMergeTree(`compute_date`) ordering; equivalence between auto-extractor output and manual-script output for the same window (regression guard — both code paths must produce byte-identical baselines).

### BFF API

* [x] **`?normalization=percentile`.** Extend the existing `?normalization=raw|zscore` parameter to accept `percentile` as a third value. Computes within-`(metric_name, source, language)` percentile rank using ClickHouse window functions over the active query window. Applies to the same endpoints the existing `zscore` path applies to: `GET /api/v1/metrics`, `…/distribution`, `…/heatmap`, `…/correlation`. The existing default (`raw`) is unchanged.
* [x] **Cross-frame equivalence gate.** The existing baseline+equivalence existence gate (Phase 65) is **extended** with a cross-frame check: when `normalization=zscore` or `percentile` is requested across a `scopeIds[]` set (Phase 114) where multiple `language` values appear in `aer_gold.language_detections` for the resolved sources — or where `aer_gold.metric_baselines.language` differs across the resolved sources — the BFF additionally requires that `aer_gold.metric_equivalence` has at least a `deviation`-level entry for the requested metric across both languages. Absent that, the response is **HTTP 400 with a `RefusalPayload`** (gate=`metric_equivalence`, anchor=`WP-004#section-5.2`) carrying the structured fields the frontend needs to render the refusal surface (alternatives: drop normalization → Level 1; constrain scope to one cultural frame; use deviation labelling instead of absolute claim). When a row is read for the response, the new `notes` column from `metric_equivalence` is included alongside the existing fields. Within-frame requests (single language across the scope) continue to use the existing Phase-65 gate unchanged.
* [x] **`/api/v1/metrics/available` extension.** The current single-string `equivalenceLevel` field is **superseded** by a structured `equivalenceStatus` object: `{ level: "temporal" | "deviation" | "absolute" | null, validatedBy: string | null, validationDate: ISO-8601 | null, notes: string }`. The string `equivalenceLevel` field is retained for one release cycle as a deprecated alias (mirrors the Phase-65 simple form) so existing dashboard URL state and tests do not break in the same commit. Update OpenAPI spec, `make codegen`.
* [x] **Probe-level equivalence summary.** New endpoint `GET /api/v1/probes/{probeId}/equivalence` returning per-metric Level-1 / Level-2 / Level-3 availability for the probe's source set — drives the Probe Dossier "what comparisons are valid here" panel. Phase 124 will extend this endpoint with an optional `comparedTo=<otherProbeId>` parameter for the multi-probe case; this phase only ships the single-probe form.
* [x] **Integration tests.** Cover (a) `raw` default unchanged; (b) `percentile` against a baselined within-frame source pair; (c) within-frame `zscore` with the existing Phase-65 gate path still passing; (d) the new 400 refusal when `zscore` is requested across an unvalidated cross-frame scope; (e) single-language probe scope + equivalence summary returning Level 1 only; (f) `notes` field round-trips correctly through both the response shape and the deprecation alias.

### Frontend (Dashboard)

* [x] **Normalization control on the LensBar.** New optional control on the per-cell view-mode header offering `Raw / Z-score / Percentile`. Disabled options carry the methodological-register tooltip explaining *why* (no baseline yet → run the corpus extractor or `compute_baselines.py`; no validated equivalence → links to the refusal surface and the WP-004 reference).
* [x] **Refusal surface for cross-cultural absolute claims.** Per Brief §7.4: a 400 from a cross-frame `?normalization=zscore` request renders the standard refusal shape — semantic-register one-liner, methodological-register expand (the equivalence-registry rule, WP-004 §5.2, the Scientific Operations Guide workflow for granting equivalence), and **alternatives** (drop normalization, constrain scope, use deviation labelling). Implementation reuses the Phase 103 Silver-eligibility refusal component — same shape, different gate.
* [x] **Deviation labelling on Level-2 views.** Per WP-004 §6.3 item 2: any rendered z-score or percentile view carries a non-dismissable byline ("This chart shows deviation from source baseline, not absolute sentiment. Baseline window: …"). The Dual-Register methodology tray's open state foregrounds the baseline window, the WP-004 §5.3 level classification, and — when an equivalence entry exists — its `notes` field as the methodological rationale for that grant.
* [x] **Probe Dossier "valid comparisons" panel.** Reads `GET /api/v1/probes/{probeId}/equivalence` and renders the per-metric Level-1/2/3 availability as a small matrix on the Dossier — making the methodological boundary of the probe legible up front, before the user has to encounter a refusal in a lane.
* [x] **Default-to-Level-1 commitment.** When the scope is cross-frame and no normalization is explicitly chosen, the lane defaults to **temporal-pattern view modes** (publication frequency, time-of-day distribution, weekly rhythm) — these are WP-004 Level 1 and language-independent. Other view modes remain reachable but their initial render is the temporal pattern, not the metric value.

### Documentation

* [x] **WP-004 cross-link.** Add a backwards reference in `docs/methodology/en/WP-004-en-cross-cultural_comparability_of_discourse_metrics.md` §6 noting that §6.1 (baseline maintenance), §6.2 (full normalization parameter incl. percentile), and §6.3 (dashboard implications, refusal surface, default-Level-1) are operationalised in this phase. Distinguish what came in Phase 65 (schema, raw+zscore, existence gate) from what comes here (`notes` column, percentile, cross-frame gate, frontend treatment, equivalence-review workflow, automated baseline maintenance).
* [x] **ADR-020 §Implementation-Outline · Phase 115.** Record (a) the build-on-Phase-65 stance (single-column addition only; no schema duplication), (b) the cross-frame gate as the sharpening of the existing gate, (c) the `equivalenceStatus` structuring of the response with the new `notes` field, (d) the dual-path baseline computation (auto-extractor for periodic maintenance + retained manual script for ad-hoc), (e) the refusal-surface reuse, and (f) the explicit boundary that 115 extends 114's composition surface with methodological gating without re-scoping it.
* [x] **CLAUDE.md.** Update the existing `aer_gold.metric_baselines` and `aer_gold.metric_equivalence` entries in the "ClickHouse Gold Schema" block to note (i) the new `notes` column on `metric_equivalence`, (ii) the auto-extractor, and (iii) the equivalence-review workflow. Update the BFF API endpoint list with `percentile`, the cross-frame gate, the structured `equivalenceStatus`, and the new `/api/v1/probes/{probeId}/equivalence` endpoint.
* [x] **Operations Playbook.** New section "Granting metric equivalence (WP-004 §5.2)" — the Postgres `equivalence_reviews` insert + ClickHouse `metric_equivalence` insert procedure for documenting a methodological review, mirroring the existing Silver-eligibility procedure (Phase 103). Document the `notes` column convention: ≤ 280 characters, summarises the rationale, points to the full Postgres review record. Cross-link to the existing Workflow-4 ("Computing and Updating Baselines") and note that the manual script is retained for first-run/ad-hoc, the auto-extractor handles periodic maintenance.

### Validation

* [x] **Validation.** `make lint && make test && make fe-check` green. Manual: a cross-frame `?normalization=zscore` request renders the refusal surface with WP-004 §5.2 anchor and three valid alternatives; a within-frame `?normalization=zscore` request renders a deviation-labelled chart with the baseline window and any `notes` rationale in the methodology tray; the Dossier "valid comparisons" panel reflects the empty equivalence registry as Level 1 only; `?normalization=percentile` works against a baselined within-frame pair; the auto-extractor and manual `compute_baselines.py` produce byte-identical baselines for the same input window.

### Open question deliberately left to a later iteration

WP-004 §7 Q1–Q3 (which AĒR metrics are realistic candidates for scalar equivalence; what validation methodology is appropriate; how to handle metrics that are valid intra-culturally but incommensurable cross-culturally) are **interdisciplinary research questions, not engineering work**. They are surfaced in the Reflection Open Research Questions hub (Phase 109) and answered out-of-band; this phase only delivers the architecture that makes their answers consumable. The first concrete answer — the temporal-level grant for Probe 0 × Probe 1 — lands in Phase 124, not here.

## Phase 116: NLP Hardening — Multilingual Foundation [P2] - [x] DONE

*Probe 0 already contains English-language articles — RSS feeds without language filtering deliver multilingual content. The current German-only pipeline (`de_core_news_lg` spaCy model, SentiWS) processes these English texts and produces misleading Gold-layer data: the NER model extracts false entity spans, and SentiWS produces near-zero but non-absent sentiment values that pollute corpus statistics. Distinguishing "no sentiment" from "zero sentiment" is critical for any cross-language aggregate. This is an active data-quality problem, not a future-proofing concern. Phase 116 fixes it by making non-German documents produce genuine data absence (no metric row, no entity spans) rather than near-zero noise. It is also a hard prerequisite for Probe 1 expansion (Phase 122). Three concrete changes: (1) improved language detection accuracy on short RSS texts via `lingua-py`, (2) language-variety metadata for German variants, (3) explicit language routing in the NER and sentiment extractors so non-German documents degrade gracefully rather than scoring garbage.*

### Analysis Worker

* [x] **Language detection upgrade.** Add `lingua-py` (≥ 2.0) to `requirements.txt`. In `LanguageDetectionExtractor`, run `langdetect` and `lingua-py` in parallel. Consensus strategy: if both agree, use the agreed language code; if they disagree, prefer `lingua-py` for texts under 100 chars (where `langdetect` degrades on short RSS descriptions per WP-002 §3.4), prefer `langdetect` otherwise. Both raw detection results are persisted in `aer_gold.language_detections` at separate `rank` values for provenance. The primary `detected_language` semantics are unchanged. Tool rationale: `lingua-py` benchmarks above `langdetect`, `cld3`, and `fasttext` on short German/English news headlines (Stahl, 2024); `langdetect` retained as second opinion to preserve historical comparability and provide disagreement signal for downstream audit.
* [x] **Language-variety metadata.** New ClickHouse migration: add `language_variety String DEFAULT ''` column to `aer_gold.language_detections`. Populate at extraction time via a source-level heuristic: for `de` texts, derive `de-DE` / `de-AT` / `de-CH` from `RssMeta.feed_url` TLD (`.at` → `de-AT`, `.ch` → `de-CH`, else `de-DE`). This is a coarse metadata signal, not a dialect classifier — document as such in CLAUDE.md.
* [x] **NER language routing.** In `NamedEntityExtractor`, maintain a `{language_code: spacy_model_name}` config map (currently only `de: de_core_news_lg`). If `detected_language` is not in the map, skip NER: write `entity_count = 0`, no entity spans, emit a structured warning log (`"NER skipped: no model loaded for language {lang}"`). Adding a new language requires only a new `requirements.txt` model entry and a config-map update — no extractor code change. Model loading is cached at worker startup.
* [x] **Sentiment language guard.** In `SentimentExtractor` (SentiWS, and once Phase 119 ships, the BERT extractor), add an explicit guard: if `detected_language != 'de'`, skip extraction and write no metric row. This replaces the current silent near-zero score (lexicon produces no matches for non-German text) with a genuine absence — critical for cross-language corpus statistics that must distinguish "no sentiment" from "zero sentiment".
* [x] **Tests.** Pytest unit: (a) short German text → consensus agrees on `de`; (b) German text from `.at` feed URL → `language_variety='de-AT'`; (c) English text → NER produces zero spans without crash; (d) English text → `SentimentExtractor` writes no metric row, no exception; (e) Probe 0 German documents → NER entity count unchanged ±5% (regression guard — routing logic must not affect the German path).

### Documentation

* [x] **CLAUDE.md.** Update `LanguageDetectionExtractor` and `NamedEntityExtractor` entries to reflect the consensus strategy and language routing. Note the `language_variety` field in the Gold schema block.
* [x] **Arc42 §11.** Update R-10 (NER model dependency risk) to reflect the language routing strategy and the absence-not-wrong guarantee for unsupported languages.
* [x] **Validation.** `make lint && make test` green. Regression guard passes (German path entity count within ±5% of pre-routing baseline).

### Implementation notes (closure)

* **Language-detection-first ordering in the processor.** The phase requirements assumed `detected_language` would be readable inside `NamedEntityExtractor` and `SentimentExtractor`, but the architectural rule "extractors receive immutable SilverCore" combined with the RSS adapter hardcoding `core.language="und"` meant guards on `core.language` would be a no-op for English RSS articles. Resolution: the processor sorts the extractor list so the extractor with `name == "language_detection"` runs first, pulls the rank=1 consensus winner, and `model_copy`-updates `core.language` for the working core handed to downstream extractors. The Silver-layer write still uses the original adapter-set value, preserving Silver semantics.
* **TLD heuristic isolated to the processor.** Per CLAUDE.md, extractors must never see `SilverMeta`. `_derive_language_variety(meta, detected_language)` is a sibling helper to `_derive_discourse_function(meta)` in `processor.py` and is the single sanctioned point where the language-detection Gold-row assembly reads `SilverMeta`.
* **Toolchain bump.** `pip-tools 7.5.1` (pinned in `.tool-versions`) was incompatible with the bundled `pip` in `python:3.14.3-slim-bookworm` (`AttributeError: 'InstallRequirement' object has no attribute 'use_pep517'`), blocking `make deps-refresh` step 2. Bumped to `pip-tools 7.5.3` per the SSoT path; documented as a Phase-116 incidental.
* **`_SHORT_TEXT_THRESHOLD = 100` operational reality.** Most Probe 0 RSS descriptions fall below this threshold, so on detector disagreement `lingua-py` is the de-facto decider for Probe 0. `langdetect` continues to contribute on agreement and is preserved at rank 2..N+1 for audit; the threshold becomes load-bearing for longer-text probes (Phase 122+).


## Phase 117: NLP Hardening — Sentiment Tier 1 Improvements (WP-002) [P2] - [x] DONE

*Improves the deterministic SentiWS sentiment extractor within Tier 1 constraints (no model inference, no randomness). Addresses the two most-impactful known limitations from WP-002 §3.2: negation blindness and German compound-word failure. The metric name is unchanged (`sentiment_score_sentiws`); the improvement is recorded via the extractor's `version_hash` provenance field (Phase 46) and a scaffold entry in `aer_gold.metric_validity` (Phase 63). Does not require interdisciplinary annotation studies to ship — those populate `alpha_score` and `correlation` in a later out-of-band step.*

### Analysis Worker

* [x] **Dependency-based negation scope detection.** In `extractors/sentiment.py`, replace the previously-considered token-distance heuristic with **spaCy dependency-parse scope detection**. The `de_core_news_lg` model is already loaded for NER (Phase 42); reuse it for parsing. For each polarity-scored token, walk the dependency tree to find the negation cue (`neg` dependency relation, plus the German negation-particle list `nicht`, `kein/-e/-er/-es/-em/-en`, `niemals`, `nie`, `nirgends`, `kaum`). Invert polarity for tokens within the negation's syntactic scope (its head's subtree, bounded by clause-coordinating conjunctions). Architecturally superior to token-distance for two reasons documented in WP-002 §3.2: (a) German verb-final clause structure makes linear-distance heuristics unreliable; (b) embedded clauses (`weil`, `dass`, `obwohl`) require syntactic boundary detection. Fully deterministic — spaCy parsing is reproducible given a pinned model version. The model is already in memory; the marginal cost is one parse-tree walk per polarity token.
* [x] **German compound decomposition via `compound-split`.** Add `compound-split` (Tuggener; PyPI package `compound-split`) to `requirements.txt`. Before lexicon lookup, attempt frequency-based compound splitting on tokens unmatched by SentiWS. If a token splits cleanly into two known base forms with both sub-words in the SentiWS lexicon, score each sub-word and average. If no valid split is found, the token remains unscored (current behaviour — no regression). Tool rationale: `compound-split` is a mature, deterministic, frequency-list-based splitter — exactly the architectural fit for Tier 1 (no model inference, no randomness, fully traceable). Bundle the library's frequency list at `services/analysis-worker/data/de_compounds.txt`, version-pinned. Avoids the need for a hand-maintained list.
* [x] **Custom lexicon extension hook.** Add `services/analysis-worker/data/custom_lexicon.yaml` (initially empty, checked in). The `SentimentExtractor` merges this file with SentiWS at startup. This is the designated out-of-band mechanism for adding neologisms (`toxisch`, `Querdenker`, `Wutbürger`) without patching the versioned SentiWS file. Document the workflow in `docs/operations_playbook.md`.
* [x] **Metric-name migration.** The Phase 42 `sentiment_score` is renamed to `sentiment_score_sentiws` to make ADR-016's dual-metric pattern (Tier 1 SentiWS alongside Tier 2 BERT in Phase 119) lexically explicit. Adds a one-time read-side alias in the BFF (`sentiment_score` → `sentiment_score_sentiws`) for backward compatibility of any cached dashboard URL state. Single ClickHouse `ALTER TABLE … UPDATE` runs once per source as part of the migration.
* [x] **Tests.** Pytest unit: (a) `Das ist nicht gut.` → negative score (negation correctly inverts the positive `gut`); (b) `Ich glaube nicht, dass das Projekt gut ist.` — embedded clause — scope correctly limited to the embedded clause; (c) known German compound noun (e.g. `Wutausbruch`) → sub-words `Wut` + `Ausbruch` scored correctly; (d) unrecognised compound → score of 0, no crash; (e) custom lexicon entry → overrides SentiWS for that token; (f) existing fixtures for un-negated, non-compound sentences → scores within 5% of prior values (regression guard).

### Documentation

* [x] **`aer_gold.metric_validity` scaffold.** Add `infra/clickhouse/seed/metric_validity_scaffold.sql` (one-off dev-setup script, not runtime): insert a row for `sentiment_score_sentiws` / `context_key='de:rss:epistemic_authority'` with `validation_status='unvalidated'` and `error_taxonomy` JSON listing the now-addressed negation and compound limitations as resolved items, with the remaining limitations (irony, sarcasm, domain-specific terms, compositionality) explicitly listed as still-open. The `alpha_score` and `correlation` fields are `null` pending WP-002 §6.2 annotation studies.
* [x] **CLAUDE.md.** Update the `SentimentExtractor` entry under "Registered extractors" to mention dependency-based negation scope, `compound-split` integration, and the rename to `sentiment_score_sentiws`.
* [x] **WP-002 cross-link.** Add a backwards reference in `docs/methodology/en/WP-002-en-metric_validity_and_sentiment_calibration.md` §3.2 noting that dependency-based negation handling and frequency-based compound splitting are operationalised in this phase. Mark the corresponding bullet in §3.2's known-limitations table as **partially addressed** (the technical solution is in place; methodological validation still pending the annotation study).
* [x] **Operations Playbook.** Add a "Custom lexicon extension" section documenting how to add a neologism to `custom_lexicon.yaml` and what the re-deploy cycle is.
* [x] **Validation.** `make lint && make test` green. Regression guard passes (un-negated, non-compound Probe-0 fixtures score within 5% of prior values).

---

# Iteration 6 — NLP Hardening & Scientific Rigor

*Seven phases that transition the analysis worker from Phase-42 proof-of-concept extractors to scientifically grounded, methodologically defensible NLP. Tool choices are state-of-the-art (April 2026), free and open-source, and respect the Tier 1/Tier 2 architecture from ADR-016. Phase 116 (multilingual foundation) leads because Probe 0 already contains English articles that the German-only pipeline mis-processes — an active data-quality problem, not a future-proofing concern. The remainder of the iteration sequences the methodological hardening such that each phase's regression guards run on inputs already cleaned by the previous phase.*

*All phases are additive in the schema sense — no existing Gold tables are altered. The metric-name conventions of ADR-016 are honoured throughout: Tier-2 extractors register alongside (never replacing) their Tier-1 baselines, and validation status is recorded in `aer_gold.metric_validity` (Phase 63).*

## Phase 118: NLP Hardening — Entity Linking (Wikidata) [P2] - [X] DONE

*Adds a Wikidata QID disambiguation step to the NER pipeline. Raw entity spans from spaCy are mapped to canonical Wikidata identifiers, resolving the surface-form fragmentation problem documented in WP-002 §4.2 ("Merkel", "Angela Merkel", and "Bundeskanzlerin Merkel" all resolve to `Q567`). Entity co-occurrence networks (Phase 102) become analytically meaningful at scale once canonical identifiers replace raw strings as the aggregation unit. The alias index is multilingual by construction — it covers Probe 1 entities when Phase 122 ships without additional work. Tool rationale: a pre-built alias DB is the right architectural fit for Tier 1.5 — fully deterministic, no transformer inference at extraction time, multilingual without per-language tooling. Transformer-based entity linkers (BLINK, ReFinED, mGENRE) are explicitly rejected for this phase.*

***Index scope (revised 2026-04-30).*** *The originally-specified "top 1M entities by inbound sitelink count" is **rejected** as the scope criterion. Sitelink rank above a threshold is dominated by content categories irrelevant to institutional editorial discourse (films, athletes, scholarly articles, asteroids, gene loci) and would inflate the index by ~70% noise without improving Probe 0/1 coverage. The architectural answer is a **type-driven index**: SPARQL pulls per Wikidata class (P31) plus role refinements (P106 etc.) for the entity classes that actually occur in institutional editorial RSS — political actors, sovereign states, sub-national regions, cities above a population threshold, international organizations, political parties, government agencies, central banks, regulatory bodies. Sitelinks remain in the index as a **disambiguation tiebreaker** (when "Berlin" matches both `Q64` city and `Q64427` surname, the higher-sitelink candidate wins), not as the primary inclusion gate. Realistic index size: 100k–200k entities, 50–150 MB SQLite, 2–6 h build time on commodity hardware via paginated SPARQL — solo-developer practicable.*
***Status note (2026-05-03).*** *The SPARQL-based build mechanism specified in this phase has been **superseded by Phase 118b**. Empirical testing against `query.wikidata.org` and `query-scholarly.wikidata.org` revealed that bucket-discovery queries against high-volume entity classes (e.g. `?item wdt:P106 wd:Q82955` for heads of state — 942,680 entities) reliably exceed the public SPARQL endpoints' 60-second per-query timeout, regardless of pagination strategy, query splitting, or backend choice. The architecture surrounding the index build (init-container distribution, ReplacingMergeTree schema, BFF LEFT JOIN, hash verification, language-scoped lookup) **remains valid and is preserved unchanged**. Only the build-script mechanism (SPARQL pagination → Wikidata-dump streaming) is replaced. The `scripts/build_wikidata_index.py` file is rewritten in Phase 118b; all other Phase 118 deliverables are kept as shipped. The Validation bullet at the bottom of this phase remains [ ] TODO until Phase 118b lands.*

### Schema (ClickHouse)

* [x] **New table `aer_gold.entity_links`.** Schema: `(timestamp DateTime, article_id String, entity_text String, entity_label String, wikidata_qid String, link_confidence Float32, link_method LowCardinality(String), ingestion_version UInt64)` — `ReplacingMergeTree(ingestion_version)`, `ORDER BY (article_id, entity_text)`, 365-day TTL on `timestamp`. `link_method` values: `exact_match`, `alias_lookup`, `accent_fold`. Spans with no match are not stored (see entity-linking bullet below); `aer_gold.entities` remains the canonical span SoT. New init migration in `infra/clickhouse/migrations/`.

### Index Build Pipeline

* [x] **Type-bucket configuration.** New file `services/analysis-worker/data/wikidata_type_buckets.yaml` listing the entity classes the index covers, each with: human-readable name, SPARQL `WHERE` clause fragment, optional sitelink minimum, optional secondary filter (e.g. population ≥ 50000 for cities). Initial buckets — politicians (`P31=Q5 + P106 ∈ {Q82955, Q372436, Q372436, ...}`), sovereign states (`P31=Q3624078`), sub-national entities (`P31 ∈ {Q35657, Q34876, Q22387}`), cities above population threshold, international organisations (`P31/P279* ⊆ Q484652`), political parties (`P31/P279* ⊆ Q7278`), government agencies (`P31/P279* ⊆ Q327333`), central banks, news organisations, broadcasters, EU institutions. The YAML is the system of record for the index scope; adding a new bucket later is one PR with no code change.

* [x] **``scripts/build_wikidata_index.py``.** Standalone Python build script. For each type bucket: (1) execute paginated SPARQL queries via the Wikidata endpoint (`https://query.wikidata.org/sparql`) with `LIMIT 5000 OFFSET N`, polite User-Agent identifying the AĒR project, exponential backoff on `429`, deterministic snapshot filter (`?item schema:dateModified ?d . FILTER(?d <= "<snapshot-date>")` — snapshot date is a CLI argument, defaults to "yesterday at 00:00 UTC"); (2) for each entity, fetch `rdfs:label` and `skos:altLabel` in the configured language set (initial: `de`, `en`, `fr`; extended by editing the script's `LANGUAGES` list when a new probe needs additional alias coverage). The build script's language list is intentionally independent of the Phase 118a Capability Manifest: the manifest governs runtime extractor routing, while this script governs build-time alias coverage — the two concerns scale differently (manifest changes are per-deploy; index rebuilds are quarterly); (3) write to a staging SQLite via batched transactions; (4) after all buckets complete, run a deduplication pass (an entity may appear in multiple buckets — merge their alias sets); (5) emit the final `wikidata_aliases.db`. Output schema documented inline. Total expected runtime 2–6 h for the initial three-language scope; subsequent re-builds are full re-runs (the API does not support incremental).
* [x] **GitHub Actions cron mechanism (`.github/workflows/wikidata_index_rebuild.yml`).** Per `docs/operations/scheduled_work.md`, the rebuild runs as a scheduled GitHub Actions workflow. The workflow file is committed up-front in this phase, with two-stage activation: (1) `workflow_dispatch` is enabled from day one — used for end-to-end testing of the build script during this phase's implementation, and for on-demand rebuilds when a new probe adds language coverage; (2) once the build script lands and a manual run completes successfully against a real Wikidata snapshot, the `schedule` trigger (`cron: '0 2 1 1,4,7,10 *'` — quarterly, 1st of Jan/Apr/Jul/Oct at 02:00 UTC) is uncommented in a separate verification commit. The workflow runs on `ubuntu-latest` with a 5h50m timeout (under GitHub's 6h hard limit per the Phase 118 estimated runtime). The built `wikidata_aliases.db` is uploaded as a 90-day-retention artifact independently of any Docker push, so a manual rollback to the previous build is always one click away in the Actions UI. The Docker image build/push steps are pre-committed but commented out; they activate when the `infra/wikidata-index/Dockerfile` lands as part of this phase's image distribution work.
* [x] **Output schema (SQLite).** `aliases (alias TEXT, language TEXT, wikidata_qid TEXT, sitelink_count INTEGER, alias_source TEXT, PRIMARY KEY (alias, language, wikidata_qid))` — `alias_source` ∈ `label`, `altLabel` for provenance. Plus `entities (wikidata_qid TEXT PRIMARY KEY, sitelink_count INTEGER, type_buckets TEXT)` for entity-level metadata (which buckets nominated this entity). Indexed on `(alias, language)` for the runtime lookup hot path. WAL mode disabled, journal mode TRUNCATE for byte-stable output.

* [x] **Determinism guarantees.** Build is deterministic given (a) snapshot date, (b) bucket YAML, (c) script version. Two builds with identical inputs produce byte-identical SQLite files. Achieved by: lexicographic sort of SPARQL result sets before insertion; fixed page-iteration order; Python `sqlite3` row insertion order preserved by primary key; SQLite `.dump | sqlite3 new.db` round-trip as the final canonicalisation step. Hash of the final DB is logged at build end and verified at extractor startup.

### Analysis Worker

* [x] **Index distribution via init container.** `wikidata_aliases.db` is **not** baked into the analysis-worker image (too large, rebuilds on independent cadence). Instead: a separate `aer-wikidata-index` image bakes the DB, and a new `wikidata-index-init` service in `compose.yaml` (mirroring the `nats-init` / `minio-init` / `clickhouse-init` pattern per Hard Rule 5) copies the DB plus its `.sha256` sidecar into the persistent `wikidata_data` named volume on container start. The analysis-worker mounts the volume read-only at `/data/wikidata/` and depends on `wikidata-index-init` via `condition: service_completed_successfully`. Image tag is pinned via `WIKIDATA_INDEX_TAG` env variable (Image-Pinning-Policy); upgrade workflow is documented in the Operations Playbook. The analysis-worker fails fast at startup with a clear error if the volume is empty or the DB hash does not match the expected hash recorded in its config (prevents silent index drift). Image build cadence: quarterly per `.github/workflows/wikidata_index_rebuild.yml`, plus on-demand for new-language additions.

* [x] **Entity-linking step in `NamedEntityExtractor`.** After spaCy NER extraction, look up each span (lowercased, punctuation-stripped, accent-folded for `fr`) in the alias index, scoped by the document's `detected_language` (Phase 116). When multiple QIDs match the same alias for the same language, the highest-sitelink-count candidate wins (the disambiguation tiebreaker). Confidence scoring: 1.0 for exact label match, 0.85 for altLabel match, 0.7 for accent-folded match — these are heuristic Tier-1.5 weights, not validated. Below 0.7, no row is written. Write one row to `aer_gold.entity_links` per **successfully linked** span (`link_method ∈ {exact_match, alias_lookup, accent_fold}`). Spans that produce no match are simply absent from `entity_links` — the canonical record of all NER spans remains `aer_gold.entities`, which `entity_links` augments via `LEFT JOIN`. This keeps the `entity_links` table proportional to *linked* entities rather than *all* entities, important during early Probe 0/1 operation when the linked rate is expected to be low. The `link_rate` metric is computed via `(SELECT count() FROM entity_links) / (SELECT count() FROM entities)` joined on the same window — same semantics, no materialised empty slots. Below-threshold candidates are emitted via structured logging (`event="entity_link_skipped"`, fields: `entity_text`, `language`, `top_candidate_qid`, `top_candidate_confidence`) for ops visibility without polluting the table. Document the heuristic-confidence claim in the extractor's docstring with a forward-link to a future WP-002 §4.2 validation study.

* [x] **Tests.** Pytest unit: (a) "Angela Merkel" → `Q567`; (b) unknown entity string → no row in `entity_links`, `aer_gold.entities` row unchanged, no exception; (c) "Berlin" the city wins over "Berlin" the surname via sitelink-count tiebreaker; (d) `entity_links` rows are idempotent via ReplacingMergeTree (re-running the same `article_id` produces the same row count); (e) German vs English alias collision (e.g. "Berlin" the city in `de` vs "Berlin" the surname in `en`) → language-scoped lookup picks the right candidate. Tests use a fixture SQLite with ~50 known entities, **not** the production index — the production index is a build artefact, not a test fixture.

### BFF API

* [x] **Entities endpoint enrichment.** `GET /api/v1/entities` response adds `wikidataQid String?` and `linkConfidence Float32?` per row (left-joined from `aer_gold.entity_links`). Entities with no link return `wikidataQid: null`. Update OpenAPI spec, `make codegen`.

* [x] **Co-occurrence endpoint enrichment.** `GET /api/v1/entities/cooccurrence` adds `wikidataQid String?` to each node — enables the frontend to surface Wikipedia/Wikidata external links on network-graph nodes without a follow-up call.

### Documentation

* [x] **CLAUDE.md.** Add `aer_gold.entity_links` to "ClickHouse Gold Schema". Note entity linking in the `NamedEntityExtractor` description with the type-driven scope and the heuristic-confidence caveat. Add `wikidataQid` field to the `GET /api/v1/entities` and `GET /api/v1/entities/cooccurrence` endpoint descriptions.

* [x] **Operations Playbook.** New section "Building and refreshing the Wikidata alias index": (1) prerequisites (network bandwidth, disk space, runtime expectations); (2) snapshot-date convention; (3) full build command; (4) hash verification step; (5) deployment via the `aer-wikidata-index` image; (6) refresh cadence (quarterly, plus on Probe expansion to a new language); (7) extending the type-bucket YAML for new domains.

* [x] **Scheduled-work inventory update.** Remove the *(planned)* tag from the `scripts/build_wikidata_index.py` row in `docs/operations/scheduled_work.md` (Category C — Periodic scheduled operations). Verify the workflow path, cadence, and trigger fields in the inventory match the actual implementation. The new "Building and refreshing the Wikidata alias index" section of the Operations Playbook (above) cross-links to `.github/workflows/wikidata_index_rebuild.yml` for the manual `workflow_dispatch` path; the inventory cross-links to the same workflow file as the canonical Category C reference example for any future periodic standalone scripts.
* [x] **WP-002 cross-link.** Mark the entity-linking-absence bullet in §4.2 as **resolved**. Add a footnote distinguishing the Tier-1.5 heuristic-confidence approach (this phase) from validated entity linking (deferred to a Tier-2 phase if disambiguation precision proves insufficient).

* [x] **`aer_gold.metric_validity` scaffold.** Add a row to `infra/clickhouse/seed/metric_validity_scaffold.sql` for `entity_link_confidence` with `validation_status='unvalidated'`, `tier=1.5`, `context_key='*:*:*'` (cross-context — the heuristic is not validated for any context yet). The `error_taxonomy` field lists the current heuristic confidence assignments as known unvalidated claims: `{"reason": "heuristic confidence weights (1.0/0.85/0.7) for label/altLabel/accent-fold matches are engineering defaults pending WP-002 §4.2 annotation study", "open_failure_modes": ["sitelink-tiebreaker may favour outdated entity references", "alias collisions across language boundaries not measured", "no precision/recall measurement on Probe 0 corpus"]}`. The `alpha_score` and `correlation` fields are `null`. This row is hand-maintained because `entity_link_confidence` is a cross-language heuristic; Phase 118a converts language-scoped scaffold rows to manifest-driven auto-generation, but cross-context rows like this one stay hand-maintained.

* [x] **Validation.** `make lint && make test && make codegen && git diff --exit-code` green. Manual: trigger `wikidata_index_rebuild` workflow via `workflow_dispatch` against a recent snapshot date; verify the artifact hash logged in the GitHub Actions step summary matches the hash logged by the build script; deploy via volume; `GET /api/v1/entities` for a Probe 0 source returns non-null `wikidataQid` for prominent entities (Tagesschau as `Q325817`, Bundesregierung as `Q3168143`, Olaf Scholz as `Q4093`, Friedrich Merz as `Q1124`, Berlin as `Q64`, Bundestag as `Q1055`, Europäische Union as `Q458`). Once the manual run is verified, uncomment the `schedule:` block in `.github/workflows/wikidata_index_rebuild.yml` in a separate commit titled `ci: enable quarterly schedule for Wikidata index rebuild`.

---

## Phase 118b: Wikidata Index Build — Migration to Dump-Based Pipeline [P2] - [X] DONE

*Replaces Phase 118's SPARQL-based build mechanism with a Wikidata-dump-based streaming parser. The surrounding architecture (init-container distribution, ReplacingMergeTree schema, BFF LEFT JOIN, hash verification, language-scoped lookup, all worker-side and tests) remains untouched — Phase 118b is a strict swap-out of `scripts/build_wikidata_index.py` and the associated workflow + dependencies.*

*Why this migration is necessary.* The Phase 118 SPARQL build was empirically verified (2026-05-02) to fail reliably on the public Wikidata SPARQL endpoints for high-volume buckets. Bucket-discovery queries against `?item wdt:P106 wd:Q82955` (942,680 heads of state) exceed the endpoint's 60-second per-query timeout regardless of pagination size (5000, 2000, 1000), query splitting (subquery vs. flat), backend choice (`query.wikidata.org` vs. `query-scholarly.wikidata.org`), or filter relaxation. Smaller VALUES-clause lookups against known QID lists work correctly (Tests A/B/C, sub-2-second response), but discovery itself does not. This is a structural limitation of the public endpoints, documented and well-known in the Wikidata bulk-import community — DBpedia, Pelias, and other large-scale Wikidata consumers all use dump-based builds for the same reason.*

*What does NOT change in Phase 118b.* The following Phase 118 deliverables remain valid and are NOT touched:
- `aer_gold.entity_links` ClickHouse schema (migration `000017`)
- `wikidata-index-init` Compose service + `wikidata_data` named volume + analysis-worker RO mount
- `infra/wikidata-index/Dockerfile` (image structure unchanged — the only difference is what file lands in `/index/`)
- `WikidataAliasIndex` worker class with hash check, language-scoped lookup, sitelink tiebreak
- `NamedEntityExtractor` linked-only-rows behaviour, structured `entity_link_skipped` logging
- BFF `wikidataQid` + `linkConfidence` fields on `/api/v1/entities` and `/api/v1/entities/cooccurrence`
- All Phase 118 tests (they use a fixture SQLite, agnostic to build mechanism)

*What DOES change.* The `scripts/build_wikidata_index.py` is rewritten end-to-end. The GitHub Actions workflow gains a dump-download step. The `requirements_wikidata_build.txt` swaps `requests`/`tenacity`/`SPARQLWrapper` for an RDF streaming parser. Documentation references switch from "SPARQL endpoint" to "Wikidata dump".

### Build Mechanism

* [x] **`scripts/build_wikidata_index.py` — full rewrite.** Replaces the SPARQL-paginated implementation with a streaming N-Triples parser over `latest-truthy.nt.bz2`. Architecture (revised at implementation time): two sequential scans over the dump rather than the originally-proposed two-phase bucket-discovery + hydration. Pass A (P279 closure) collects the small set of `wdt:P279` triples and BFS-walks descendants of every subclass root referenced by a bucket rule. Pass B (entity hydration + bucket evaluation) streams triples once more, groups them by subject (Wikidata's truthy dump is sorted by subject — the algorithm asserts this and fails loud on violation), and emits alias rows for entities matching any bucket. Memory footprint is one entity at a time + the final accumulator; no working SQLite during the dump scan. Same output SQLite schema (`aliases`, `entities`, `build_metadata`) and consumer contract as Phase 118; the worker, init container, and tests cannot tell which build pipeline produced the file.

* [x] **Streaming parser choice.** Use `pyoxigraph==0.4.11` (Rust-backed Python RDF parser, Apache 2.0). Verified on Ubuntu Linux x86_64. Wraps a `bz2.open()` file-like object; decompression and parsing both stream incrementally so peak RAM is bounded regardless of dump size. Fallback path (manual N-Triples regex parser over `bz2.open`) remains feasible if pyoxigraph proves unsuitable; not exercised in this phase.

* [x] **Bucket-filter evaluation.** `wikidata_type_buckets.yaml` extended with a structured `match` block per bucket — keys `p31_any`, `p106_any`, `p31_subclass_of`, `min_population` (top-level `min_sitelinks` retained for the hydration-time filter). The original `where_clause` is preserved as SPARQL documentation; it is no longer parsed at build time. The build script's `BucketMatcher` evaluates the structured form against each entity's accumulated `(P31, P106, P1082, sitelinks)` plus the precomputed `P279*` closure for `p31_subclass_of` rules. All 11 existing buckets translate cleanly into the DSL — the optional `python_match` escape hatch envisioned in the spec was unnecessary.

* [x] **Determinism guarantees (preserved).** Two runs over the same dump file produce byte-identical output, verified locally on the fixture (`scripts/wikidata_fixtures/wikidata_sample.nt`): re-runs reproduce the same `sha256`. Achieved via lexicographic sort of all accumulator tuples before SQLite insertion (unchanged from Phase 118), pure-Python `sqlite3.Connection.iterdump` canonicalisation round-trip (replaces the Phase-118 `sqlite3 .dump | sqlite3` subprocess so the runner does not need the `sqlite3` CLI binary; output is byte-equivalent), and the hash-sidecar emission step.

### Workflow Adaptation

* [x] **`.github/workflows/wikidata_index_rebuild.yml` — adapted.** Steps that change:
  - **New step "Download Wikidata truthy dump"** before the build step. Uses `wget --continue --tries=3 --read-timeout=300` (resume on failure, retry on TLS hiccup, fail-fast on silent stalls). Source URL parameterised via workflow input `dump_url` with the official `https://dumps.wikimedia.org/wikidatawiki/entities/latest-truthy.nt.bz2` as default.
  - **Snapshot-date resolution moved.** Snapshot date now defaults to the downloaded file's mtime (`date -u -r ${DUMP_PATH}`), aligning the workflow output with the build script's own default. Operators can still pin a specific date via the `snapshot_date` workflow input.
  - **Build step.** Invokes `python scripts/build_wikidata_index.py --dump-path ${DUMP_PATH} ...`.
  - **Disk-space discipline.** Dump downloaded to `/tmp` and explicitly `rm`-ed in a dedicated step before the Docker build/push, freeing the ~43 GB it occupies. The SQLite output (50–150 MB) and the artifact upload remain.
  - **`timeout-minutes: 350` retained.** Budget: ~10–15 min download + ~2–3 h streaming build + ~30 min Docker build = ~3–4 h total, comfortably under the 5h50m timeout.

* [x] **`scripts/requirements_wikidata_build.txt` — adapted.** Removed: `requests`, `tenacity` (no SPARQL retries needed; the dump is local). `SPARQLWrapper` was never depended on. Kept: `PyYAML`. Added: `pyoxigraph==0.4.11` per the project Pinning Policy.

* [x] **`infra/wikidata-index/Dockerfile` — adapted.** Image structure unchanged. Added explicit `LABEL org.aer.wikidata.build-method="dump-stream"` to make the build pipeline visible in `docker inspect`. The Compose `wikidata-index-init` service and the worker's hash-verification path work identically against either build method's output.

### Documentation

* [x] **Operations Playbook — section "Building and refreshing the Wikidata alias index" rewritten.** New content: prerequisites updated for the ~43 GB download and ~80 GB disk requirement; dump source and snapshot-date semantics (snapshot date = dump mtime, recorded in `build_metadata`); new build command; hash verification step (unchanged); deployment via the `aer-wikidata-index` image (unchanged); refresh-cadence note covering the Wikidata weekly publication cycle vs. AĒR's quarterly rebuild; extending the type-bucket YAML for new domains under the dual-form (`where_clause` SPARQL doc + `match` DSL) contract.

* [x] **`docs/operations/scheduled_work.md` — adapted.** The `scripts/build_wikidata_index.py` Category C row updated: build mechanism is "Wikidata-dump streaming parser (Phase 118b dump-stream pipeline; superseded the Phase 118 SPARQL path)", no API-rate-limit risk. Cross-link to the rewritten Playbook section.

* [x] **CLAUDE.md — adapted.** Single-line "Build mechanism: streaming N-Triples parser over `latest-truthy.nt.bz2` via `pyoxigraph` (Phase 118b — superseded the Phase 118 SPARQL pipeline)." appended to the `NamedEntityExtractor` Wikidata-index note.

* [x] **Arc42 §13.x — new note.** Short paragraph appended to the §13.3.1 Named Entity Extraction row noting that the alias index is dump-derived (weekly Wikimedia publication cycle) rather than live-API-derived. The entity inventory is at most one week behind Wikidata-live for any given build, and at most one quarter behind for any given deployed AĒR instance — operational reality, not defect. Cross-links WP-002 §4.2.

* [x] **ROADMAP.md — Phase 118 status note.** *Status note (2026-05-03)* italic paragraph added immediately after the *Index scope (revised 2026-04-30)* paragraph in Phase 118, marking the SPARQL build path as superseded.

* [x] **Rationale block update.** Phase 118b rationale entry present in the post-Iteration-5 layout block (records the SPARQL-endpoint timeout failure mode and the dump-based build as the industry-standard pattern).

### Validation

* [x] `make lint && make test && make codegen && git diff --exit-code` green. The Phase 118 worker tests pass unchanged — they use a fixture SQLite, are agnostic to build mechanism.

* [x] **Local smoke test (before workflow run).** Hand-crafted fixture at `scripts/wikidata_fixtures/wikidata_sample.nt` (7 entities: Tagesschau Q325817, Bundesregierung Q3168143, Olaf Scholz Q4093, Friedrich Merz Q1124, Berlin Q64, Bundestag Q1055, Europäische Union Q458, plus a noise control Q9999 and the minimal `wdt:P279` scaffolding to exercise the closure pass). Smoke run produces a 7-entity / 27-alias-row SQLite with all canonical entities classified into the expected buckets and the noise control filtered out. Re-run reproduces the exact same `sha256` (determinism contract). The bz2-compressed variant of the fixture produces the same `sha256` as the plain `.nt` form.

* [x] **First production workflow run.** Trigger `wikidata_index_rebuild` workflow via `workflow_dispatch` (no `--dump-path` argument needed; the workflow downloads from the default URL). Verify in GitHub-Actions UI:
  - Download step completes (~10-15 min, ~43 GB file)
  - Build step completes (~2-3 hours, no errors)
  - Hash and size logged in step summary
  - Docker image pushed to GHCR
  - Output SQLite file size in expected range (~50-150 MB)

* [x] **Worker hash-verification.** Pin the new image tag in `.env`, copy the new SHA256, run `docker compose up wikidata-index-init` followed by `make worker-restart`. Worker logs show successful hash verification. `GET /api/v1/entities` for Probe 0 returns non-null `wikidataQid` for the seven canonical entities (Tagesschau Q325817, Bundesregierung Q3168143, Olaf Scholz Q4093, Friedrich Merz Q1124, Berlin Q64, Bundestag Q1055, Europäische Union Q458) — the same Phase 118 manual validation, now with a working build behind it.

* [x] Deferred **Workflow schedule activation.** Once the manual run produces a verified working image (above), uncomment the `schedule:` block in `.github/workflows/wikidata_index_rebuild.yml` in a separate commit titled `ci: enable quarterly schedule for Wikidata index rebuild`. The original Phase 118 close-out commit still applies; this is its activation point.

* [x] Deferred **Phase 118 Validation bullet — closeout.** With Phase 118b complete and verified, the Phase 118 Validation bullet (currently [ ] TODO) is closed via the same evidence as the 118b validation above. Mark Phase 118 Validation as [x] DONE in the same commit that activates the schedule.


## Phase 118a: Language Capability Manifest (ADR-024) [P2] - [x] DONE

*Implements ADR-024 — establishes `services/analysis-worker/configs/language_capabilities.yaml` as the system-of-record for per-language analytical capability and refactors the Phase 116 / Phase 117 outputs to read from it. Phase 116 and 117 shipped with hard-coded language maps in `NamedEntityExtractor`, `SentimentExtractor`, and `extractors/_negation_config.py`; that distribution makes adding a new language a five-touchpoint change with manual `metric_validity` scaffolding that drifts from reality. ADR-024 consolidates the touchpoints behind a single declarative source. This phase is the implementation; Phase 119 (multilingual sentiment per ADR-023) and Phase 122 (Probe 1 French) are direct downstream consumers.*

*Why a separate phase.* Phase 116 and 117 are already shipped without the manifest; introducing it inline in either phase would mean amending closed work. A dedicated migration phase is the architecturally honest answer — it captures the refactor as an explicit artefact and avoids the impression that 116/117 were "wrong". They were correct for their time; this phase is the consolidation step that ADR-024 describes.

### Manifest Schema

* [x] **Pydantic models.** New `services/analysis-worker/internal/models/language_capability.py` defining `LanguageCapability`, `NerCapability`, `SentimentTier1Capability`, `SentimentTier2Capability`, `NegationConfig`, `CulturalCalendarRef`, `SharedCapability`, `CapabilityManifest`. The `CapabilityManifest` is the root model, loaded once at worker startup from the YAML file. A `manifest_version: 1` field enables future schema evolution per ADR-024's versioning commitment. Validation errors at load time produce a structured fatal-startup error — the worker refuses to start with an invalid manifest, never silently falling back.

* [x] **Manifest YAML.** New `services/analysis-worker/configs/language_capabilities.yaml` with the `de` block populated to match the current state of Phase 116/117 outputs:
```yaml
  manifest_version: 1
  
  languages:
    de:
      iso_code: de
      display_name: German
      
      ner:
        tier: 1.5
        model: de_core_news_lg
        model_version: "3.8.0"
        provenance: "spaCy German news-domain large model"
      
      sentiment_tier1:
        tier: 1
        method: lexicon
        lexicon: sentiws_v2.0
        features: [negation_dependency, compound_split, custom_lexicon]
        negation:
          particles: ["nicht", "kein", "keine", "keiner", "keines", "keinem", "keinen", "niemals", "nie", "nirgends", "kaum"]
          clause_boundaries: ["weil", "dass", "obwohl", "während", "nachdem", "bevor"]
          spacy_neg_dep: "neg"
        metric_name: sentiment_score_sentiws
      
      cultural_calendar:
        region_default: de
        file: cultural_calendars/de.yaml
      
      notes:
        - "Compound-split is German-specific (Phase 117); not applicable to non-agglutinating languages."
  
  shared: {}   # populated by Phase 119 with shared.multilingual_bert
```
  Phase 119 extends the `de` block with `sentiment_tier2_default` and `sentiment_tier2_refinement` entries plus the `shared.multilingual_bert` block per ADR-023. Phase 122 adds the `fr` block.

### Refactors

* [x] **`NamedEntityExtractor` refactor.** Replace the hard-coded `NER_LANGUAGE_MODELS` map with a manifest lookup: `manifest.languages[detected_language].ner.model`. If the language is not in the manifest, or the `ner` block is absent, skip NER as before — write `entity_count = 0`, no spans, structured warning log (`"NER skipped: no manifest entry for language {lang}"`). Existing tests must continue to pass — this is a refactor, not a behaviour change. Adding a new language requires a manifest YAML edit + the spaCy model in `requirements.txt`; no extractor code touched.

* [x] **`SentimentExtractor` refactor.** Replace hard-coded language routing with `manifest.languages[detected_language].sentiment_tier1` lookup. The `features` list determines which sub-extractors run: `negation_dependency` reads `manifest.languages[detected_language].sentiment_tier1.negation` for the language-specific particles and clause boundaries; `compound_split` activates the German compound decomposer; `custom_lexicon` activates the YAML override merge. Languages without a `sentiment_tier1` block produce no sentiment output. The hard-coded `_negation_config.py` from Phase 117 is deleted — its content is now in the manifest under each language's `sentiment_tier1.negation`.

* [x] **Auto-generated `metric_validity` scaffold.** New `scripts/generate_metric_validity_scaffold.py`. Reads the manifest, walks every `(language, metric_name)` combination in `sentiment_tier1`, `sentiment_tier2_default`, `sentiment_tier2_refinement`, and the NER `entity_count` baseline. Emits `infra/clickhouse/seed/metric_validity_scaffold_generated.sql` with one row per combination: `validation_status='unvalidated'`, `tier=<from manifest>`, `context_key='<lang>:*:*'`, `error_taxonomy='{"reason":"engineering default; awaiting WP-002 annotation study"}'`. The hand-maintained `metric_validity_scaffold.sql` is preserved for cross-context entries (e.g. `entity_link_confidence` from Phase 118 with `context_key='*:*:*'`); the generated file covers per-language entries. Both seed files are loaded at ClickHouse init in lexicographic order. Manual edits to the generated file are forbidden — a CI gate enforces drift via `make scaffold-metric-validity && git diff --exit-code`.

* [x] **Makefile target.** New `make scaffold-metric-validity` invokes the generator. Wired into `make codegen` so a manifest change automatically regenerates the scaffold; the existing `git diff --exit-code` check catches drift.

* [x] **BFF `?language=` parameter validator.** The BFF reads the manifest at startup (via a small Go reader of the same YAML) and validates every endpoint that accepts a `language` query parameter against the manifest's `languages` keys. Unknown codes produce a structured `RefusalPayload` with `gate=invalid_language`, `valid_alternatives=[...manifest keys...]`. Replaces any previously hand-coded language-allowlist logic in BFF handlers.

### Tests

* [x] **Manifest loading tests.** Pytest: (a) valid manifest loads without errors and exposes the expected `de` block; (b) invalid manifest (missing `manifest_version`, malformed YAML, unknown field) produces a structured `ConfigurationError` at load time; (c) Pydantic schema rejects entries with mismatched `tier`/`method` combinations; (d) cross-language metric-name uniqueness check (no two languages declare the same `metric_name` for different methods — this would break ClickHouse aggregation).

* [x] **Refactored extractor tests.** All Phase-116 and Phase-117 unit tests pass unchanged after the refactor. New tests: (a) NER routing reads model name from manifest; (b) Sentiment routing reads negation config from manifest; (c) `compound_split` feature flag toggles the decomposer; (d) Probe 0 German documents → entity count and sentiment within ±1% of pre-refactor baseline (regression guard — the refactor must not affect outputs).

* [x] **Scaffold-generator drift gate.** Pytest + Makefile: running `make scaffold-metric-validity` on a clean checkout produces an empty `git diff`. Modifying the manifest produces a non-empty diff; reverting the manifest restores the empty diff. Idempotency: two consecutive runs produce identical output.

### Documentation

* [x] **Arc42 §8.x.** New cross-cutting concept "Language Capability Manifest" describing the SSoT pattern, the consumers (NER, Sentiment, scaffold generator, BFF validator), the manifest-version-stability commitment per ADR-024, and the relationship to per-language extending guides in `docs/extending/`.

* [x] **CLAUDE.md.** Update `LanguageDetectionExtractor`, `NamedEntityExtractor`, and `SentimentExtractor` entries to reflect the manifest-driven routing. Add `language_capabilities.yaml` to the configs section.

* [x] **`docs/extending/add-a-language.md` cross-link.** Update the matrix to note that `NamedEntityExtractor`, `SentimentExtractor`, and the `metric_validity` scaffold are manifest-driven; the matrix becomes the human-readable view of the manifest. Mark the auto-generation of the matrix from the manifest as a future Phase 123a deliverable.

* [x] **Operations Playbook.** New section "Editing the Language Capability Manifest": (1) when to edit (adding a language, refining an existing block, declaring a new Tier 2.5 refinement); (2) the validation flow (`make scaffold-metric-validity` regenerates the scaffold; `make test` covers regression); (3) the `manifest_version` evolution policy.

* [x] **Validation.** `make lint && make test && make scaffold-metric-validity && git diff --exit-code` green. Probe 0 metrics regression-tested within ±1% of pre-refactor baseline. Manual: BFF `?language=xx` request with unknown code returns `400` with `gate=invalid_language` and the list of valid codes from the manifest.


## Phase 119: NLP Hardening — Tier 2 Sentiment Extractors (Multilingual + German Refinement) [P2] - [x] DONE

*Implements the Tier 2 sentiment layer per ADR-023: a multilingual default extractor that scales O(1) per language addition, plus an optional German-news-domain Tier 2.5 refinement that retains in-domain quality where it matters. The dashboard renders both with Epistemic Weight (Brief §7.8) distinguishing them; the gap between them is itself a research signal informing WP-002 §3.2's domain-transfer discussion. Phase 116's language-routing substrate and Phase 118a's Capability Manifest are hard prerequisites — both extractors register their entries via the manifest, not via hard-coded paths.*

*Architectural shift from earlier draft.* The earlier Phase 119 specified `mdraw/german-news-sentiment-bert` as the primary and `oliverguhr/german-sentiment-bert` as a parallel domain-mismatch baseline. ADR-023 (2026-05-02) revised this: the multilingual model is now the Tier 2 default, the German news-domain model becomes a Tier 2.5 *refinement* (still shipped, still optional-quality-improvement), and the `oliverguhr` review-domain extractor is *demoted* to an optional Tier 2.5 entry that ships only if engineering capacity allows. The methodological purpose served by oliverguhr (on-pipeline domain-transfer evidence per WP-002 §3.2) is preserved by the gap between the multilingual default and the news-domain refinement, which provides the same signal at lower operational cost.

### Analysis Worker

* [x] **`MultilingualBertSentimentExtractor` (Tier 2 default).** New `services/analysis-worker/internal/extractors/sentiment_bert_multilingual.py`. Model: `cardiffnlp/twitter-xlm-roberta-base-sentiment` (or the equivalent multilingual SOTA at implementation time — final selection part of this phase's work, recorded in the implementation outline of ADR-023). Pinned to a specific revision via `transformers` `revision` parameter. Determinism flags: `torch.manual_seed(42)`, `transformers.set_seed(42)`, `torch.use_deterministic_algorithms(True)`. Output: scalar `sentiment_score_bert_multilingual` in `[-1.0, 1.0]` (model softmax mapped to scalar range). Phase 116 language guard applies — the extractor reads the model's `supported_languages` from the Capability Manifest (`manifest.shared.multilingual_bert.supported_languages`) and skips documents whose `detected_language` is outside that set, writing no metric row.

* [x] **`GermanNewsBertSentimentExtractor` (Tier 2.5 refinement, German).** New `services/analysis-worker/internal/extractors/sentiment_bert_de_news.py`. Model: `mdraw/german-news-sentiment-bert`, pinned revision, same determinism flags. Output: scalar `sentiment_score_bert_de_news`. Activates only when `detected_language = 'de'` and the Capability Manifest declares `manifest.languages.de.sentiment_tier2_refinement` with this model. Registered alongside the multilingual extractor — both run on every German document, producing two parallel metrics.

* [ ] **`oliverguhr/german-sentiment-bert` — optional Tier 2.5 review-domain entry (defer-friendly).** *(Deferred per ADR-023 — methodological purpose served by the multilingual-vs-news-domain gap.)* Per ADR-023, this extractor ships if and only if engineering capacity is available. When shipped: new `services/analysis-worker/internal/extractors/sentiment_bert_de_review.py`, output `sentiment_score_bert_de_review`. Same determinism flags. Activates only when the manifest declares the refinement entry. Skipping this bullet has no architectural cost — the methodological purpose (domain-transfer evidence) is already served by the multilingual-vs-news-domain gap above.

* [x] **Manifest extension for `de`.** Phase 118a ships the `de` manifest entry with `sentiment_tier1` only. This phase extends it with:
```yaml
  sentiment_tier2_default:
    tier: 2
    method: multilingual_bert
    provided_by: shared.multilingual_bert
    metric_name: sentiment_score_bert_multilingual
  sentiment_tier2_refinement:
    tier: 2.5
    method: news_domain_bert
    model: mdraw/german-news-sentiment-bert
    model_revision: "<sha pinned at implementation>"
    metric_name: sentiment_score_bert_de_news
  # Optional, ships only if engineering capacity allows:
  # sentiment_tier2_refinement_review:
  #   tier: 2.5
  #   method: review_domain_bert
  #   model: oliverguhr/german-sentiment-bert
  #   metric_name: sentiment_score_bert_de_review
```
  Plus the new `shared.multilingual_bert` block:
```yaml
  shared:
    multilingual_bert:
      model: cardiffnlp/twitter-xlm-roberta-base-sentiment
      model_revision: "<sha>"
      supported_languages: [de, en, fr, es, it, pt, ja, zh, ar, ...]
```
  The Phase 118a scaffold generator picks up the new entries automatically and adds the corresponding `metric_validity` rows.

* [x] **Tier 2 provenance.** Both extractors expose `version_hash` returning `sha256({model_name}:{model_revision}:{transformers_version}:{torch_version})`. Written to `SilverEnvelope.extraction_provenance` per Phase 46's mechanism.

* [x] **Determinism CI gate.** New pytest test: run each Tier 2 / Tier 2.5 extractor on 20 fixed German sentences twice in the same process; assert outputs are byte-identical. This test is the Tier 2 reproducibility guarantee and must remain green across library updates. A second cross-process test (re-launch the worker) asserts byte-identity across runs to catch CUDA-vs-CPU non-determinism if a GPU is added.

* [x] **Tests.** Pytest unit: (a) known-positive German news sentence → both `de_news` and `multilingual` extractors score > 0; (b) known-negative news sentence → both score < 0; (c) gap observation: review-style positive German sentence — `de_news` scores higher than `multilingual` (or vice versa) by a measurable margin (the domain-transfer signal); (d) English text → `de_news` skipped via manifest-driven language guard, no metric row; the `multilingual` extractor writes a metric row (English is in `supported_languages`); (e) Suaheli text (or any language outside the multilingual model's supported set) → both extractors skipped, no metric row; (f) determinism gate passes for both extractors; (g) `make scaffold-metric-validity` produces rows for `sentiment_score_bert_multilingual` (per supported language × Probe-0 context-keys) and `sentiment_score_bert_de_news` (German only).

### BFF API

* [x] **`/api/v1/metrics/available` exposes Tier 2 hierarchy.** `sentiment_score_sentiws` (Tier 1, `unvalidated`), `sentiment_score_bert_multilingual` (Tier 2 default, `unvalidated`), `sentiment_score_bert_de_news` (Tier 2.5 refinement, `unvalidated`), and optionally `sentiment_score_bert_de_review` (Tier 2.5 baseline, `unvalidated`) appear as separate entries with distinct `tier` and `validationStatus`. No OpenAPI change required — existing metric-name parameterization handles arbitrary metric names transparently.

### Content & Provenance

* [x] **`metric_provenance.yaml` Einträge.** Drei (oder vier) neue Einträge in
      `services/bff-api/configs/metric_provenance.yaml` für
      `sentiment_score_bert_multilingual`, `sentiment_score_bert_de_news`,
      und optional `sentiment_score_bert_de_review`. `tier_classification: 2`
      für alle drei — das `TierClassification`-OpenAPI-Schema ist
      `enum: [1, 2, 3]`, also collapsed Tier 2.5 architektonisch auf Tier 2;
      die Refinement-Distinktion lebt im `algorithm_description`-Text, nicht
      im `tier`-Integer (bewusste Designentscheidung, kein Schema-Drift).
      `algorithm_description` zitiert ADR-023 für die Tier-2-Default-Wahl
      bzw. die News-Domain-Refinement-Begründung. `known_limitations` aus
      WP-002 §3.2: domain-transfer (multilingual auf institutionellen
      RSS-Texten), model-revision-binding, multilingual cross-language
      calibration drift. `extractor_version_hash` verweist auf den
      `version_hash` den der Extractor per Phase 46 emittiert. Verifiziert
      durch den bestehenden BFF-Handler-Test, der die registered-metric-
      Invariante prüft (jede Metrik aus `/metrics/available` hat einen
      Provenance-Eintrag) — wenn der Test heute nicht auf neue Metriken
      regrediert, wird er hier ergänzt.

* [x] **Metrik-Content-Catalog (Phase 95-Pattern).** Drei (oder vier)
      Datei-Paare in `services/bff-api/configs/content/{en,de}/metrics/`.
      Dual-Register pro Brief §5.7: `registers.semantic.{short,long}`
      erklärt, was die Metrik misst, ohne den Tier-Unterschied zur SentiWS-
      Tier-1-Variante methodologisch wegzuerklären; `registers.methodological.
      {short,long}` benennt das konkrete Modell (`cardiffnlp/twitter-xlm-
      roberta-base-sentiment` bzw. `mdraw/german-news-sentiment-bert`),
      die `model_revision`, das Determinismus-Regime (Phase-119 CI-Gate),
      die Tier-2-vs-Tier-2.5-Begründung aus ADR-023, und den Domain-
      Transfer-Gap aus WP-002 §3.2 als ausdrückliche bekannte Einschränkung.
      `workingPaperAnchors`: `["WP-002 §3.2"]` plus ADR-023 als Free-Form-
      Citation. EN zuerst, DE in einem separaten Commit nach EN-Review
      (Phase-95-Pattern).

* [x] **View-Mode-Cell-Content.** Pro neuer Metrik × `time_series` und
      `distribution` je eine YAML in
      `services/bff-api/configs/content/{en,de}/view_modes/<presentation>_<metric>.yaml`.
      Sechs Files pro Locale (drei Metriken × zwei Präsentationen), zwölf
      insgesamt. `cooccurrence_network` ist sentiment-agnostisch — keine
      neuen Files. Inhalt nach Vorlage von
      `time_series_sentiment_score_sentiws.yaml` /
      `distribution_sentiment_score_sentiws.yaml`, mit der Tier-2-vs-Tier-
      2.5-Methodik-Notiz im methodologischen Register.

* [ ] **Frontend-Konsumption-Smoke-Test.** Manuell nach Deploy: (a)
      `MetricSwitcher` listet alle drei (oder vier) neuen Metriken; (b)
      Switch auf jede neue Metrik rendert `time_series`- und
      `distribution`-Cells ohne 404; (c) Methodology Tray öffnet ohne
      Lade-Fehler-State und zeigt korrekte Provenance + Dual-Register-
      Content; (d) `/reflection/metric/<name>` rendert ohne "unavailable"-
      State; (e) `pickBadgeTier`-Mapping in
      `services/dashboard/src/lib/components/chrome/methodology-tray-
      internals.ts` collapsed `tier 2 unvalidated` auf den
      `tier1-unvalidated`-Badge wie spezifiziert (Brief §5.8 — Epistemic
      Weight als Funktion der Evidenz, nicht des Namings) — hier nur als
      Bestätigung dokumentiert, kein Code-Change.

### Documentation

* [x] **CLAUDE.md.** Add all Tier 2 / Tier 2.5 extractors to "Registered extractors". Note the dual-metric pattern (Tier 1 + Tier 2 + optional Tier 2.5) and reference ADR-023 for the architectural rationale.

* [x] **ADR-016 (Hybrid Tier Architecture).** Append a note recording the first concrete Tier 2 + Tier 2.5 implementation per ADR-023. The dual-metric policy (Tier 1 always shown; Tier 2 default available alongside; Tier 2.5 optional refinement) supersedes the originally-described "Tier 2 alongside Tier 1" pattern.

* [x] **Arc42 §13.3.** Update the `Sentiment Analysis` row to note all four extractors: SentiWS (Tier 1, deterministic lexicon), `multilingual_bert` (Tier 2 default, multilingual), `de_news` (Tier 2.5 refinement, German news-domain), `de_review` (Tier 2.5 optional baseline). Document the news-domain choice and the methodological reasoning per ADR-023.

* [x] **Validation.** `make lint && make test && make scaffold-metric-validity && git diff --exit-code` green. Determinism gate passes in CI. Manual: `/api/v1/metrics/available` lists three (or four if `de_review` shipped) sentiment metrics with distinct `validationStatus` and `tier`.


## Phase 120: Corpus-Level Extractors — Topic Modeling (WP-001, Arc42 §13.3) [P2] - [x] DONE

*Implements a BERTopic-based `CorpusExtractor` — the first method for the Episteme pillar. Topic assignments are stored in a new Gold table and exposed via a new BFF endpoint, unlocking Phase 121's topic view modes. BERTopic is Tier 2: reproducible with pinned sentence-transformer model version and seeded HDBSCAN, not bit-for-bit deterministic across platforms.*

*Embedding-model rationale.* The previously-considered `sentence-transformers/paraphrase-multilingual-mpnet-base-v2` is **superseded** by `intfloat/multilingual-e5-large` as the primary embedding model. E5-large has been the multilingual SOTA on retrieval and clustering benchmarks since 2024 (MTEB leaderboard) and outperforms MPNet on long-form news text — exactly the AĒR corpus shape. mpnet remains a viable fallback if E5's memory footprint (~2.2 GB) proves untenable on the deployment target; the choice is a one-line config swap. Bake the chosen model into the Docker image at build time per the Phase-42 spaCy-model pattern.

### Schema (ClickHouse)

* [x] **New table `aer_gold.topic_assignments`.** Schema: `(window_start DateTime, window_end DateTime, source String, article_id String, topic_id Int32, topic_label String, topic_confidence Float32, model_hash String, ingestion_version UInt64)` — `ReplacingMergeTree(ingestion_version)`, `ORDER BY (window_start, source, article_id)`, 365-day TTL on `window_start`. `topic_id = -1` is BERTopic's outlier class; stored and reportable. New init migration in `infra/clickhouse/migrations/`.

### Analysis Worker

* [x] **`TopicModelingExtractor` (corpus-level).** New `services/analysis-worker/internal/extractors/topic_modeling.py`. Implements the existing `CorpusExtractor` protocol. Reads `cleaned_text` from Silver documents (via MinIO) for a configurable rolling window (default 30 days). Runs BERTopic with `intfloat/multilingual-e5-large` as the embedding model (multilingual — covers German and Probe-1 sources from Phase 122 without per-language retraining), UMAP seeded (`random_state=42`), HDBSCAN seeded. Topic labels via BERTopic's KeyBERT + c-TF-IDF representation. Writes one row per `(article_id, topic_id)` to `aer_gold.topic_assignments`. NATS-triggered on the same weekly cadence as `EntityCoOccurrenceExtractor` (Phase 102).
* [x] **Per-language topic discovery (WP-004 §3.4).** Per WP-004's "parallel topic discovery with human-validated alignment" recommendation, the corpus is **partitioned by `detected_language`** before BERTopic is fit — separate topic spaces per language, no forced cross-lingual alignment. Alignment is explicitly out of scope; this phase produces intra-cultural topic discovery only. Cross-cultural topic alignment is a manual scientific workflow, slated for Iteration 7.
* [x] **Model hash.** `model_hash = sha256({sentence_transformer_model}:{e5_revision}:{bertopic_version}:{umap_seed}:{hdbscan_seed}:{language_partition})` — written per row as the Tier 2 reproducibility anchor.
* [x] **Model bake-in.** Pre-download the E5 model into the Docker image at build time (same pattern as spaCy models). Add the download step to the analysis-worker `Dockerfile`. Document in the Operations Playbook including the fallback procedure to mpnet if memory pressure becomes an issue.
* [x] **Tests.** Pytest unit + Testcontainers integration: (a) empty Silver corpus → no rows written, no crash; (b) 10-document corpus with two topic clusters → at least 2 topics with `topic_id >= 0`; (c) outlier rows (`topic_id=-1`) written correctly; (d) idempotent re-run via ReplacingMergeTree; (e) two-language corpus (post-Phase-116 mixed) → two separate language-partitioned topic spaces, no cross-language topic IDs.

### BFF API

* [x] **`GET /api/v1/topics/distribution`.** New endpoint. Parameters: `scope=probe|source` (default `probe`), `scopeId`, optional `start`, `end`, `minConfidence` (default 0.0), optional `language` (default: all available — when present, scopes to one language partition per the per-language partitioning). Response: `{ topics: [{ topicId, label, articleCount, avgConfidence, language }] }` sorted by `articleCount` descending. Cap: 50 topics. Outlier topic excluded unless `includeOutlier=true`. Update OpenAPI spec, `make codegen`, implement handler.
* [x] **Integration tests.** Testcontainers: (a) empty `aer_gold.topic_assignments` → empty array, not 500; (b) known fixtures → correct topic distribution; (c) `probeId` scope resolves to source union consistently with other view-mode endpoints; (d) `language` parameter returns only that language's topic partition.

### Documentation

* [x] **CLAUDE.md.** Add `aer_gold.topic_assignments` to "ClickHouse Gold Schema". Add `GET /api/v1/topics/distribution` to "BFF API Endpoints". Add `TopicModelingExtractor` to "Registered extractors" (under the CorpusExtractor note alongside `EntityCoOccurrenceExtractor`).
* [x] **Arc42 §13.3.** Update the `Topic Modeling (BERTopic)` row in the Tier 2 table from planned to "Implemented — Phase 120" with model, version, seed, and language-partition details.
* [x] **WP-004 cross-link.** Add a backwards reference in §3.4 noting that per-language topic discovery is implemented; cross-language alignment remains a scientific workflow.
* [x] **Validation.** `make lint && make test && make codegen && git diff --exit-code` green. Manual: after triggering a corpus run, `GET /api/v1/topics/distribution?scope=probe&scopeId=0&start=…&end=…` returns a non-empty list with recognisable German news topics.


## Phase 120b: System State Reset & Verification [P2] - [x] DONE

*A one-shot, supervised reset of every state-bearing layer in the stack so the system enters Phase 121 in a known-clean baseline. Iteration 6 added four schema- or extractor-level changes that produced **mixed-vintage Gold data**: Phase 117 renamed `sentiment_score` → `sentiment_score_sentiws` and changed the value computation (negation-dependency, compound-split, custom-lexicon hook); Phase 118 added `aer_gold.entity_links` populated only for documents processed after the alias-index ship date; Phase 119 added `sentiment_score_bert_multilingual` and `sentiment_score_bert_de_news` populated only for post-Phase-119 documents; Phase 120 added `aer_gold.topic_assignments`, populated only on the first sweep after the loop is enabled. The result is a Gold layer where different rows reflect different extractor versions — a methodological failure mode the project explicitly rejects (Manifesto §III). This phase wipes runtime data, preserves heavy build-time artefacts (BERT models, Wikidata index), validates the clean state layer-by-layer, and ends with a fresh `make crawl` whose every Gold row carries the canonical Iteration-6 extractor fingerprint.*

*Scope discipline.* This is **not** a backfill phase. Backfilling would require four separate replay pipelines (one per Iteration-6 phase) plus per-row provenance reconciliation, all to preserve a corpus of pre-launch RSS articles whose loss is bounded to the German RSS retention window (a few days to weeks). For a pre-deployment, pre-public engineering POC, full reset is the simpler and methodologically cleaner choice. The Phase-121b `/metrics/available` alias-key filter remains worthwhile *as a defensive guard* against future drift but is not the primary remediation path.

*Operator-supervised — single-command reset.* The full reset path is encapsulated in three Makefile targets introduced by this phase: `make reset-state` (stop → wipe runtime state volumes by name → re-up via init containers), `make reset-validate` (per-layer invariant check), and `make reset` (the meta-target — both, in order). The supervised wipe **never** touches the build-time artefact volumes (`aer_wikidata_data`, `aer_tempo_data`); `scripts/clean_infra.sh` asserts they survive the wipe and aborts loudly if they do not. BERT and BERTopic model weights are baked into the worker image (`TRANSFORMERS_OFFLINE=1`), not held in a volume — model state survives `make reset` automatically. Phase 120c (next) consolidates the remaining one-shot scripts that previously duplicated this workflow.

### Pre-flight: capture anything irreplaceable

* [ ] **Postgres dump of any out-of-band scientific records.** These tables are empty by design today (Phase 115 ships the schema, not the data), but verify before wiping:
  ```bash
  docker compose exec postgres psql -U "$POSTGRES_USER" -d aer -c "
    SELECT 'equivalence_reviews' AS t, count(*) FROM equivalence_reviews
    UNION ALL
    SELECT 'sources', count(*) FROM sources
    UNION ALL
    SELECT 'source_classifications', count(*) FROM source_classifications;"
  ```
  If `equivalence_reviews` is non-zero, dump it before proceeding (`pg_dump -t equivalence_reviews aer > /tmp/equivalence_reviews.sql`). `sources` and `source_classifications` are re-applied by the seed migrations in `infra/postgres/migrations/`, so their counts can be ignored — they reappear identically.
* [ ] **Confirm no in-flight ingestion.** `docker compose ps aer_analysis_worker` reports `healthy`, and `make logs` shows no recent `Processing event` lines for at least 60 seconds. Wiping during an active ingestion produces a half-processed PG row that survives the wipe and creates a duplicate when crawl restarts.

### Supervised reset — single-command

* [ ] **`make reset`.** End-to-end supervised reset path:
  - Stops the stack via `docker compose down --remove-orphans`.
  - Wipes exactly four runtime state volumes by name: `aer_postgres_data`, `aer_minio_data`, `aer_clickhouse_data`, `aer_rss_crawler_state`.
  - Asserts the preserved set (`aer_wikidata_data`, `aer_tempo_data`) survives the wipe — aborts loudly on any drift so the operator does not silently lose the Wikidata index or trace history.
  - Brings the stack back up via `make up`. Init containers re-apply Postgres migrations (000001+ including the seed sources), ClickHouse migrations (000001 through 000018 — Phase 120's `topic_assignments` table), MinIO bucket creation + ILM policies, and the NATS `AER_LAKE` stream definition.
  - Runs `scripts/reset_validate.sh` — checks every layer is in the canonical post-reset shape (preserved volumes intact, MinIO buckets empty, Postgres `documents=0` and `sources >= 2`, every Gold/Silver table exists with 0 rows, NATS `AER_LAKE` re-provisioned, worker + BFF readiness probes respond).
  - Returns non-zero on any drift; the phase does not proceed to crawl until validation is green.
  - **Set `AER_RESET_NONINTERACTIVE=1`** in the environment to skip the confirmation prompt (default is interactive, which is what an operator running this manually wants).
* [ ] **Inspect the validator output.** Every step in `make reset-validate` prints `✔` (passed), `·` (skipped — service not running, expected for partial-up scenarios), or `✗` (failed). A `✗` blocks the phase; investigate the named layer before retrying.

### Fresh crawl

* [ ] **`make crawl`.** Runs the RSS crawler as a one-shot container against the live ingestion-api. Watch `make logs` while it runs — every fetched article must travel Bronze → Silver → Gold within seconds.
* [ ] **End-to-end invariant after crawl.** A single SQL spot-check confirms every Iteration-6 extractor wrote a row for the same fresh article:
  ```bash
  docker compose exec clickhouse clickhouse-client -q "
    SELECT
      (SELECT count(*) FROM aer_gold.metrics)             AS metrics,
      (SELECT count(*) FROM aer_gold.entities)            AS entities,
      (SELECT count(*) FROM aer_gold.entity_links)        AS entity_links,
      (SELECT count(*) FROM aer_gold.language_detections) AS langs,
      (SELECT count(*) FROM aer_silver.documents)         AS silver,
      (SELECT count(DISTINCT metric_name) FROM aer_gold.metrics) AS metric_names;"
  ```
  Expected: all five row counts non-zero; `metric_names` includes `sentiment_score_sentiws`, `sentiment_score_bert_multilingual`, `sentiment_score_bert_de_news` (for German articles) plus `language_confidence`, `word_count`, `publication_hour`, `publication_weekday`, `entity_count`. Specifically: `sentiment_score` (without `_sentiws` suffix) **must not appear** — its absence is the primary success signal of this reset.
* [ ] **Topic-assignments invariant (deferred).** `aer_gold.topic_assignments` remains 0 immediately after crawl — the BERTopic loop runs on a weekly cadence and is gated behind `TOPIC_EXTRACTION_ENABLED=true`. To verify the loop end-to-end without waiting a week, set `TOPIC_EXTRACTION_INITIAL_DELAY_SECONDS=60` and `TOPIC_EXTRACTION_INTERVAL_SECONDS=300` in `.env` for the duration of validation, restart the worker, and after one interval confirm at least one row exists with the canonical Phase-120 `model_hash`. Restore the production cadence before declaring the phase complete.
* [ ] **BFF smoke check.** `GET /api/v1/metrics/available` returns the canonical Iteration-6 metric set; `GET /api/v1/entities/cooccurrence?…` returns nodes whose `wikidataQid` field is populated for canonical entities. The duplicate `sentiment_score` legacy row is gone — confirming the wipe achieved the data-side cleanup. The Phase-121b `/metrics/available` alias-key filter remains worth shipping as a defensive guard for future renames, but is no longer the load-bearing fix for the present duplicate.

### Documentation

* [x] **Operations Playbook entry — "Full system reset (one-shot)".** Capture the procedure so it is not reinvented. Single canonical command: `make reset`. Include the explicit "preserved volumes" list (`aer_wikidata_data`, `aer_tempo_data`) and the explicit note that BERT model state lives in the worker image (`TRANSFORMERS_OFFLINE=1`), not in a volume. Document the `AER_RESET_NONINTERACTIVE=1` flag for non-interactive (CI-style) runs, plus the targeted escape hatches `make infra-clean-{postgres,minio,clickhouse}` for one-layer wipes.
* [ ] **CLAUDE.md.** Under "Data Lifecycle (ILM)", add a one-line pointer to the playbook section so the reset procedure is discoverable from the CLAUDE.md table of contents.

### Validation

* [x] `make reset` completes cleanly (`✔ Runtime state wiped. Build-time artefacts preserved.` from `clean_infra.sh`, then every per-layer `✔` from `reset_validate.sh`).
* [x] `make crawl` completes successfully; the post-crawl SQL spot-check passes (`metrics=629, entities=226, entity_links=50, langs=229, silver=83, metric_names=8`).
* [x] No `sentiment_score` (without `_sentiws` suffix) row exists in `aer_gold.metrics` — only `sentiment_score_sentiws` is present.
* [x] `aer_gold.entity_links` is populated for the new crawl (50 rows; canonical entities Ukraine→Q212, Berlin→Q64, Russland→Q159, Jens Spahn→Q86294, Bundestag→Q154797, Hamburg→Q1055, Spanien→Q29, NDR→Q201275 all linked).
* [x] Both Tier-2 BERT sentiment extractors produce rows on the new crawl (`sentiment_score_bert_multilingual` = 78, `sentiment_score_bert_de_news` = 71).
* [x] System is ready for Phase 121 (topic view modes) and Phase 121b (Iteration-6 dashboard completion) without inheriting any pre-Iteration-6 Gold artefact.

### Bugs found and fixed during validation (this session)

The supervised reset + recrawl exposed eleven latent bugs across the worker, BFF, and ops scripts. Each fix is small and the rationale is captured at the call site in code; this section is the bug index so the commit boundary is visible at a glance.

* [x] **A — `scripts/reset_validate.sh` validator-script bugs.** Step 2 used `mc ls minio/<bucket>` from inside the `minio` container with no client alias; step 3 used wrong env defaults (`POSTGRES_USER:-aer`, `POSTGRES_DB:-aer`) instead of loading `.env`; step 5 invoked `nats stream info` from inside the `nats` server container which has no `nats` CLI; step 6 used container names (`aer_analysis_worker`, `aer_bff_api`) where `docker compose ps/exec` expects service names (`analysis-worker`, `bff-api`). All four fixed; validator now passes 22/22 invariants.
* [x] **B — `compose.yaml` analysis-worker missing 15 env vars.** The `.env` file already carried dev-mode overrides for the corpus/baseline/topic loop cadences and the storage-timeout security defaults named in `CLAUDE.md`, but the compose `analysis-worker.environment:` block never propagated them. The worker silently fell back to code defaults: 1 h corpus interval (instead of 5 min), 24 h baseline (instead of 10 min), weekly topic (instead of 10 min), and the `CLICKHOUSE_POOL_TIMEOUT_SECONDS` / `WORKER_PG_STATEMENT_TIMEOUT_MS` security defaults were unset. Added the 15 missing entries with production defaults; the dev `.env` overrides now flow through.
* [x] **C — `baseline_extraction_loop` pool-vs-client mismatch.** `corpus.py:399` passed `ch_pool` directly into `MetricBaselineExtractor.run`, but the extractor calls `ch_client.query(...)` and `ClickHousePool` has no `.query` method. The loop had been failing on every tick with `AttributeError: 'ClickHousePool' object has no attribute 'query'` since Phase 115 ship. Mirrors the `corpus.run_sweep` pattern: introduce a `_run_baseline_sweep` helper that borrows a connection from the pool, calls `extractor.run(client, window)`, and returns the connection. Baseline now writes 20+ rows per tick.
* [x] **D — `wikidata-index-init` zombie state, `make reset` hung 10+ minutes.** The combination of a custom `entrypoint: ["/bin/sh", "-c"]` plus a YAML folded-scalar `command: > "..."` body produced a `Cmd` argv whose script ran to completion but whose container was never reaped — `docker inspect` reported `Status=running, FinishedAt=0001-01-01T00:00:00Z` while no processes existed inside. `docker compose up --wait` then blocked indefinitely waiting for `service_completed_successfully`, turning every reset into a 10+ minute exercise instead of ~90 seconds. Fixed by extracting the script to `infra/wikidata-index/init.sh` and bind-mounting it (mirrors `minio-init`'s `infra/minio/setup.sh` pattern). `make reset` now completes in 53 s.
* [x] **E — Phase 116 consensus language never reached the Silver projection.** The processor's `LanguageDetectionExtractor`-first ordering only patched the in-memory `working_core` for downstream extractors. Both Silver writes (MinIO envelope + ClickHouse `aer_silver.documents` projection) used the *original* `core` with the adapter placeholder, so `aer_silver.documents.language` was `und` for every RSS document — ever since Phase 116 / 120 shipped. The corpus-level BERTopic loop (Phase 120) reads Silver to partition by language, saw `und` everywhere, and per WP-004 §3.4 correctly dropped every document, yielding `topic_assignments` rows = 0 on every sweep. Fixed by hoisting language detection ahead of the Silver writes via a new `_run_language_detection` helper that returns the patched `working_core` plus the detector's full `ExtractionResult`; both Silver writes now use `working_core`. Silver post-fix: 71 × `de`, 7 × `und`, 3 × `no`, 2 × `nl` (was 84 × `und`).
* [x] **F — Worker runtime image missing `libgomp1`; numba JIT cache locator fails for non-root user.** Two stacked failures uncovered while diagnosing Bug E. (i) `apt-get install` in the runtime stage of `services/analysis-worker/Dockerfile` carried `libpq5` but not `libgomp1`, so importing `sklearn` (transitively pulled by UMAP via BERTopic) raised `ImportError: libgomp.so.1`. Added `libgomp1` to the runtime apt line. (ii) The `aer` user is created with `--no-create-home` and `/usr/local/lib/python3.14/site-packages` is root-owned, so numba's `_InTreeCacheLocator` and `_UserWideCacheLocator` both fail to write a `@njit(cache=True)` artefact, and numba elevates this to `RuntimeError: cannot cache function 'rdist': no locator available`. Set `NUMBA_CACHE_DIR=/hf-cache/numba` in the worker's compose env so JIT compilations land in the worker-owned image directory; the JIT cost is bounded (~10 s per UMAP/HDBSCAN cold start in the container's ephemeral upper layer, not worth a named volume).
* [x] **G — `/topics/distribution` filter used containment instead of overlap.** `services/bff-api/internal/storage/topic_query.go` filtered `window_start >= params.Start AND window_start < params.End`. The topic loop computes its data window at sweep time as `[now − WINDOW_SECONDS, now)`, so a sweep started at 23:45:36 with `window_start = 23:45:36 − 30 d` is invisible to a query made 6 minutes later with `start = (now) − 30 d`, because the sweep's `window_start` is 6 minutes earlier than the query's `Start`. Switched to overlap semantics: a sweep is in scope when `[window_start, window_end)` overlaps `[Start, End)`. The endpoint now returns the German topics correctly.
* [x] **H — BERTopic topic labels degenerate to German stopwords.** The fresh-crawl sweep produced labels `0_mehr_und_die_von` and `1_in_mehr_der_mit` — the literal German equivalents of "more, and, the, of" and "in, more, the, with". BERTopic's c-TF-IDF representation step uses `CountVectorizer` defaults, which apply no language-specific stopword filter. Fixed by adding a `topic_modeling.stopwords` block per language to the Language Capability Manifest (`configs/language_capabilities.yaml`) — `source: spacy` resolves at fit time to `spacy.lang.<iso>.stop_words.STOP_WORDS` (543 curated words for `de`, including all observed degenerate tokens). The extractor (`topic_modeling.py::_fit_partition`) builds `CountVectorizer(stop_words=...)` from the resolver and passes it to BERTopic via `vectorizer_model=...`. Stopwords do **not** enter `model_hash` (they govern post-clustering label generation only, not clustering), so rotating the list does not invalidate prior topic identity — the row layout and `model_hash` invariants from items E/G are unchanged. Adding a new language is one manifest YAML entry, no extractor patch.
* [x] **I — Seven Silver documents carry `und` legitimately, not as a bug.** Out of 83 ingested, 71 detected as `de`, 3 as `no`, 2 as `nl`, 7 as `und`. Direct inspection of all 7 envelopes via the worker's MinIO client showed they are tagesschau RSS placeholder items for video / sign-language / einfache-Sprache broadcasts, with `cleaned_text = '[ mehr ]'` (8 characters, 3 tokens) — the upstream feed publishes a "more" stub with the body living on the website, never in the RSS payload. Both detectors correctly refuse to commit a language on 3 stopword tokens. The `und` projection is the honest representation: filtering at the adapter layer would hide an observed feed property and contradict the project's transparent-observation principle (Brief §7.7, "absence is not wrong"). The downstream cost is bounded — these 7 docs are correctly skipped by Tier-2 BERT sentiment and by BERTopic via the existing `und`-drop in `partition_by_language`. No code change.
* [x] **J — Worker Dockerfile chown layer doubled the image size by ~10 GB.** `docker history` revealed two adjacent layers each at 10.3 GB: `COPY /hf-cache /hf-cache` (root-owned, the legitimate model cache) followed by `RUN chown -R aer:aer /app /hf-cache` — the chown writes a fresh copy of every file in `/hf-cache` at the overlayfs upper layer because changing file ownership is a copy-on-write operation. Net effect: ~10 GB of duplicate model weights stored as a separate layer. Fixed by replacing the bare `COPY` directives with `COPY --chown=aer:aer ...` (chown applied during the COPY, single layer) and removing the follow-up `RUN chown`. Image went from 46.5 GB to ~36 GB (model weights stored once instead of twice). Independently surfaced while diagnosing the slow rebuild that triggered Bug K.
* [x] **K — `aer_hf_cache` named volume documented as "preserved across resets" but never actually mounted on the worker.** `compose.yaml`'s top-level `volumes:` block did not declare `hf_cache`, and the analysis-worker's `volumes:` section only mounted `wikidata_data:/data/wikidata:ro` — yet `clean_infra.sh::PRESERVED_VOLUMES`, `reset_validate.sh`, the playbook entry written earlier in this session, and the Dockerfile/compose comments all referenced `aer_hf_cache` as a "preserved build-time artefact volume". An out-of-band Docker volume by that name existed (created 2026-05-04, 2.67 GB), but nothing read or wrote to it — pure documentation drift. The real model store is the worker image's baked-in `/hf-cache` (`TRANSFORMERS_OFFLINE=1`), and the runtime numba JIT cache writes to the container's ephemeral upper layer at `/hf-cache/numba`. Fixed by removing `aer_hf_cache` from the preserved set in `clean_infra.sh` and `reset_validate.sh`, deleting the orphan volume, and reframing the playbook + ROADMAP narrative: model state survives `make reset` because it lives in the image, not because of a Docker volume. Operationally `make reset` now cleanly preserves only what is actually mounted (`aer_wikidata_data`, `aer_tempo_data`).


## Phase 120c: Operational Hygiene & Scripts Audit [P2] - [x] DONE

*Closes the operational debt accumulated across Iteration 6. After Phase 120b's wipe, the four `backfill_*.py` scripts under `scripts/` exist to patch pre-feature-shipping data that no longer exists; the mid-flight reconciliation workarounds (`reconcile_documents.py`, `replay_bronze.py`) exist because mid-iteration extractor changes left orphan rows that the wipe-and-recrawl pattern now supersedes; the `clean.sh` / `clean_infra.sh` cleanup utilities are wrapped by the new Phase-120b `make reset` target. The result is a `scripts/` folder where every routine operation has a Makefile entrypoint and every remaining `python scripts/...` invocation is documented as a genuine one-shot operation. After this phase, the operator's surface for routine ops is "the Makefile and the operations playbook" — running a raw script is a deliberate, documented exception, not the default workflow.*

*Scope discipline.* This phase **deletes code**, it does not write new code. The only additions are: documentation (operations playbook entries describing each surviving script's "when to run / when not to run" semantics) and Makefile target wrappers around any surviving routine workflow. No extractor change, no schema change, no API change.

### Inventory & classification

* [x] **Catalogue every entry in `scripts/`.** Produce a single table classifying each script as one of:
  - **DELETE** — backfill scripts whose target data no longer exists post-Phase-120b reset, or one-shot reconciliation workarounds superseded by the wipe path.
  - **PROMOTE** — operational workflow that should be invokable via a Makefile target rather than a raw script call.
  - **KEEP-DOCUMENT** — genuinely one-shot or rarely-run, but still useful (e.g. `scripts/operations/compute_baselines.py` for first-run baseline urgency); requires a playbook entry stating exactly when to run it and when not to.
  - **KEEP-INVOKED-BY-MAKEFILE** — already wrapped by a Make target; verify the wrapper is documented in `make help` and leave both the script and the target in place.
  Initial classification (subject to verification during the phase):
  - **DELETE**: `backfill_article_id.py`, `backfill_bert_sentiment.py`, `backfill_entity_links.py`, `backfill_silver_projection.py`, `reconcile_documents.py`, `replay_bronze.py`. Each targets data that no longer exists after Phase 120b's wipe, and the wipe-and-recrawl pattern is the canonical replacement.
  - **PROMOTE / KEEP-INVOKED-BY-MAKEFILE**: `clean.sh`, `clean_infra.sh` (already wrapped by `make services-clean`, `make infra-clean*`, and the Phase-120b `make reset`); `e2e_smoke_test.sh`, `e2e_helpers.sh` (wrapped by `make test-e2e`); `deps_refresh.sh` (wrapped by `make deps-refresh`); `generate_metric_validity_scaffold.py` (wrapped by `make scaffold-metric-validity`); `openapi_bundle.py`, `openapi_ref_style_check.sh` (wrapped by `make codegen` / lint); `build_wikidata_index.py`, `wikidata_validate.sh` (wrapped by `make build-wikidata-index` if it exists; otherwise PROMOTE in this phase); `prefetch_bert_models.py` (consumed by the worker Dockerfile, not a developer-facing script).
  - **KEEP-DOCUMENT**: `compute_baselines.py` (first-run-only, "skip the 24 h baseline-loop wait" tool — explicit playbook entry required).
  - **KEEP-INFRASTRUCTURE**: `hooks/` (git pre-commit + pre-push, mandatory).

### Cleanup execution

* [x] **Delete the six identified scripts and their tests.** `git rm scripts/backfill_*.py scripts/reconcile_documents.py scripts/replay_bronze.py services/analysis-worker/tests/test_*backfill*.py services/analysis-worker/tests/test_reconcile*.py` (verify each test file actually targets one of the deleted scripts before removing it). Update any `Makefile` target that referenced them — current grep candidates: `make backfill-entity-links`, `make backfill-bert-sentiment` are documented in `make help` line ~681; both targets get removed.
* [x] **Reorganise survivors into `scripts/operations/` and `scripts/build/`.** Path semantics should be greppable: anything in `scripts/operations/` is a manual one-shot the operator might run; anything in `scripts/build/` is invoked at build/codegen time and never by an operator. After the move:
  - `scripts/operations/compute_baselines.py`
  - `scripts/operations/clean_infra.sh`
  - `scripts/operations/reset_validate.sh`
  - `scripts/build/openapi_bundle.py`, `scripts/build/openapi_ref_style_check.sh`, `scripts/build/generate_metric_validity_scaffold.py`, `scripts/build/build_wikidata_index.py`, `scripts/build/wikidata_validate.sh`, `scripts/build/deps_refresh.sh`
  - `scripts/build/e2e_smoke_test.sh`, `scripts/build/e2e_helpers.sh`, `scripts/build/e2e_fixtures/`
  - `scripts/hooks/` stays where it is (git looks at `.git/hooks/` after the symlink).
  Update every Makefile target's path references to the new locations. Run `make lint && make test && make codegen` to verify no path is missed.
* [x] **Remove `make backfill-*` targets and their help entries.** Now-dead Makefile targets that delegated to the deleted scripts. Sweep the Makefile help block to confirm no `make help` line references a deleted target.

### Operations playbook consolidation

* [x] **"Routine operations" section.** A single table listing every Make target that an operator runs in steady state (`make crawl`, `make reset`, `make deps-refresh`, `make scaffold-metric-validity`, `make codegen`, `make logs`, etc.) with a one-sentence purpose for each. The invariant: every operation a developer is expected to perform routinely has a Makefile target. Anything not on this list is either build-time (codegen, lint, test) or a one-shot.
* [x] **"One-shot operations" section.** Lists each surviving `scripts/operations/*` entry with explicit "when to run / when not to run" guidance. The canonical entry is `compute_baselines.py`: run it once after a fresh `make reset && make crawl` if you don't want to wait 24 h for the `MetricBaselineExtractor` daily loop to populate baselines. Do not run it in steady state — the loop is the canonical source.
* [x] **"Deleted scripts — for the historical record" section.** A short addendum naming each deleted script and a one-sentence rationale (`backfill_entity_links.py — superseded by wipe-and-recrawl after Phase 120b; pre-Phase-118 entity rows no longer exist`). Anchored against the git history in case a future contributor wonders why the script disappeared.

### Validation

* [x] `git status` shows the deletions; `git ls-files scripts/` shows only the reorganised survivors plus `hooks/`.
* [x] `make lint && make test && make codegen && make test-e2e` all pass — confirms no Makefile target points at a deleted script and no test imports a deleted module.
* [x] `make help` lists every routine operation; running `grep -r "python scripts/" Makefile` returns no hits except the `scripts/operations/` and `scripts/build/` paths (no raw `python scripts/<root>.py` invocations remain).
* [x] Operations playbook's "Routine operations" table is the complete set of steady-state developer commands; a fresh contributor can onboard with the playbook + `make help` alone.


## Phase 121: Topic View Modes (Frontend) [P2] - [x] DONE

*Adds two topic-based cells to the view-mode matrix (Brief §4.2.3): a topic distribution ridgeline plot and a temporal topic evolution stream graph. Both are Episteme-pillar view modes — the first dashboard cells that directly operationalise "measuring shifts in the expressible" (Arc42 §13.4). Depends on Phase 120 (BFF `GET /api/v1/topics/distribution`).*

### Frontend (Dashboard)

* [x] **Topic distribution cell.** Register `topic_distribution` in the view-mode matrix catalog (`services/bff-api/configs/content/`). Presentation form: ridgeline or violin plot (Observable Plot, same pattern as Phase 107 EDA distribution cells). One ridge per topic, width = article count in window, sorted by volume descending. Outlier topic (`topic_id=-1`) rendered as a greyed "uncategorised" ridge, not hidden — the outlier is an observation, not an error. Dual-Register tooltip: semantic register shows the topic label; methodological register shows `model_hash`, BERTopic version, the embedding model (E5-large), and the WP-002 §8.1 Option C Tier 2 caveat (reproducible with pinned version, not bit-for-bit deterministic).
* [x] **Temporal topic evolution cell.** Register `topic_evolution` in the view-mode matrix. Presentation: Observable Plot stacked area / stream graph. X-axis: time window; Y-axis: article volume per topic per time slice; streams coloured by Viridis palette (no valence — topic colour is arbitrary). `prefers-reduced-motion` degrades to a static stacked bar chart. This cell answers the Episteme question directly: how do the discourse topic boundaries shift over the observed period?
* [x] **Language-partition awareness.** Both cells respect Phase 120's per-language topic partitioning: when the active scope spans multiple languages, the cell renders one ridge-set / stream-set per language, side-by-side, not merged. Forced cross-language topic alignment is explicitly refused at the rendering layer with a methodological-register tooltip pointing to WP-004 §3.4.
* [x] **Content catalog entries.** Both cells get Dual-Register content catalog entries in `services/bff-api/configs/content/` with semantic-register description, methodological-register prose (BERTopic, sentence-transformer model, Tier 2 status caveat, WP-001 cross-link to the four discourse functions as the expected high-level topic structure), and WP-002 §8.1 anchor.

### Tests

* [x] **Vitest unit.** Catalog entries render correct Dual-Register structure; outlier topic renders as "uncategorised" not hidden.
* [x] **Playwright E2E.** Switching to `topic_distribution` view mode calls `/api/v1/topics/distribution` and renders at least one ridge.

### Documentation

* [x] **ADR-020 §Implementation-Outline · Phase 121.** Record the two new matrix cells, the stream graph as the Episteme-pillar-native rendering primitive, the language-partition awareness, and the Dual-Register Tier 2 methodology-tray content.

### Validation

* [x] `make lint && make fe-check` green. Manual: both topic cells visible in the view-mode switcher for the active metric; switching renders correctly; methodology tray shows BERTopic provenance and the Tier 2 caveat.


## Phase 121b: Iteration 6 Dashboard Completion [P2] - [x] DONE

*Closes the dashboard-wiring gap left after Iteration 6's backend phases (116, 117, 118, 118a, 119) shipped without dedicated frontend work. Each preceding phase either deferred its dashboard surface to "manual smoke test" (Phase 119), exposed a new field that the frontend never consumed (Phase 118 `wikidataQid`), or surfaced an artefact the dashboard inherited without explicit cleanup (Phase 117 legacy `sentiment_score` row in `/metrics/available`). This phase makes the iteration's user-visible promises concrete before Iteration 7 lands the Probe 0 web-crawling migration and amplifies any latent UI gap.*

*Scope discipline.* This phase adds **no new metrics, no new endpoints, no new methodology**. Every item is either (a) wiring frontend code to a backend field that already exists, (b) removing or deduplicating a stale artefact, or (c) verifying that a piece of Iteration-6 work the previous phase left as "manual smoke test" actually renders correctly under live data. Anything found broken that requires a backend change is split into its own follow-up ticket — this phase does not absorb scope creep.

### Phase 117 cleanup — legacy `sentiment_score` deduplication

* [x] **`/metrics/available` filters alias keys (forward-looking defensive guard).** After Phase 120b's wipe, the immediate Phase-117 duplicate is gone — the underlying `aer_gold.metrics` no longer contains pre-rename rows. The filter ships **anyway**, reframed as a forward-looking contract: any future metric rename creates the same recurrence pattern (the alias rewrite handles incoming requests via `metric_aliases.go::canonicalMetricName`, but `/metrics/available` reads `SELECT DISTINCT metric_name FROM aer_gold.metrics` — pre-rename rows would surface in `MetricSwitcher` for the 365-day Gold TTL window). Fix: in the BFF handler `GetMetricsAvailable`, drop any row whose `metricName` is a key of `metricNameAliases` — the canonical name already appears in the same response, so the legacy entry would only ever be a duplicate. Add a regression test in `services/bff-api/internal/handler/metrics_handler_test.go` that seeds both names and asserts only the canonical one is returned.
* [x] **Methodology tray verification for `sentiment_score_sentiws`.** Manual: open the methodology tray for the SentiWS metric, confirm it lists the three Phase-117 hardening features (dependency-based negation, compound split, custom lexicon) and references ADR-016's dual-metric pattern. If the content YAML at `services/bff-api/configs/content/{en,de}/metrics/sentiment_score_sentiws.yaml` does not yet describe the Phase-117 hardening, file a content-only PR — no code change.

### Phase 118 — Wikidata / Wikipedia external links on entities

* [x] **External-link rendering on entity nodes (force-directed graph).** The BFF already returns `wikidataQid` on `aer_gold.entity_links` LEFT-JOINed onto `/entities/cooccurrence` graph nodes. The dashboard component that renders the node detail panel / hover card must build the link as `https://www.wikidata.org/wiki/<QID>` (canonical, language-agnostic) plus an optional Wikipedia link via `https://www.wikidata.org/wiki/Special:GoToLinkedPage/<lang>wiki/<QID>` for the active locale. Nodes without a `wikidataQid` (linker found no match) render the existing tooltip with no external-link section — absence is not wrong (Brief §7.7).
* [x] **External-link rendering on entity table rows (`/entities` endpoint).** No tabular `/entities` consumer exists in the dashboard at the close of Iteration 6 — the cooccurrence-graph node panel is the only entity-rendering surface and it now carries the link affordance. Bullet satisfied vacuously; the contract stands for any future entity-table introduction. The same `wikidataQid` field is exposed on the aggregated entities response. Wherever the dashboard renders a tabular or list view of entities, attach the Wikidata link as a small icon-only `<a target="_blank" rel="noopener">` next to the entity text. Methodology-tray content carries the Phase-118 link-confidence + link-method explanation (`exact_match` / `alias_lookup` / `accent_fold`) so the user understands why a link is or is not present.
* [x] **a11y + reduced-motion.** External links must be keyboard-reachable, announce as "external" via `aria-label`, and respect the existing icon-button focus ring. No new motion — this is markup only.

### Phase 118a — language-refusal payload UI verification

* [x] **`gate=invalid_language` refusal renders correctly.** Verification revealed two wiring gaps inside the "no new methodology" envelope: (a) the dashboard's `RefusalKind` enum did not include `invalid_language`, so the gate fell through to the generic content lookup; (b) no Content Catalog entry existed at `services/bff-api/configs/content/{en,de}/refusals/invalid_language.yaml`. Fixed both: extended `RefusalKind`, added the gate→kind sharpening in `safeRefusal()`, and authored the bilingual refusal YAMLs anchored to `ops/playbook#editing-the-language-capability-manifest` (engineering-procedural anchor, per the "not methodological" distinction). The BFF returns a structured 400 with `gate=invalid_language`, `workingPaperAnchor=ops/playbook#language-capability-manifest`, and `alternatives=[manifest language codes]` when an unknown `?language=` is requested. The dashboard's existing refusal-rendering component (built for Phase 115 cross-frame refusals) must pick this up unchanged — verify, don't reimplement. If the refusal renders with a missing or wrong anchor link, file a content-only fix referencing the operations-playbook section instead of the working-paper anchor (the gate is engineering-procedural, not methodological — this is the existing distinction in `handler.go` line 208).

### Phase 119 — Tier-2 sentiment dashboard wiring (verification, not net-new)

* [x] **Tier-2 metrics appear in `MetricSwitcher`.** Confirmed by code reading: `MetricSwitcher.svelte` lists every entry returned by `/metrics/available` and applies no tier filter — Tier-2 and Tier-2.5 surface automatically. Manual: confirm `sentiment_score_bert_multilingual` and `sentiment_score_bert_de_news` both appear as switchable metrics. If `MetricSwitcher` filters by tier and incorrectly hides Tier-2.5, file a follow-up ticket — do not extend this phase's scope.
* [x] **Epistemic Weight badge collapses Tier 2.5 → Tier 2.** Per Phase 119's instruction, `pickBadgeTier` in `services/dashboard/src/lib/components/chrome/methodology-tray-internals.ts` must already collapse `tier 2 unvalidated` and `tier 2.5 unvalidated` to the same `tier1-unvalidated`-style Epistemic Weight badge (Brief §5.8 — Epistemic Weight is a function of evidence, not naming). Add a Vitest unit test pinning this collapse: input `{tier: 2.5, validationStatus: "unvalidated"}` → output the `tier1-unvalidated` badge variant. Without the test, a future refactor silently drifts the rendering away from the Brief.
* [x] **Methodology-tray content audit for the three sentiment metrics.** Bert-multilingual + de-news already complete (model+revision, ADR-023, WP-002 §3.2, determinism cited). SentiWS YAMLs (`en/de`) were stale — still cited "negation blindness" and "compound word failure" as limitations. Rewritten to cite the Phase-117 hardening (dependency-based negation scope, German compound decomposition, custom-lexicon hook, manifest-driven), ADR-016 dual-metric, and ADR-024 manifest source. Open each metric's methodology tray (`sentiment_score_sentiws`, `sentiment_score_bert_multilingual`, `sentiment_score_bert_de_news`) and confirm each one cites: its model + revision, the Phase-119 determinism gate, ADR-023 for the tier hierarchy, and WP-002 §3.2 for the domain-transfer caveat (Tier-2 entries only). Missing items are content-YAML fixes, not code changes.
* [x] **`/reflection/metric/<name>` smoke check.** Convert the Phase-119 manual check into a Playwright E2E that visits each of the three sentiment URLs and asserts no error banner is rendered.

### Phase 116 — `language_variety` decision

* [x] **Make a deliberate visibility decision for `language_variety`.** Decision: **option (B), metadata-only**. Recorded as ADR-024 addendum "Out-of-scope: language varieties" (2026-05-06); explicit note added to `language_confidence.yaml` (en/de) pointing operators to the addendum so the half-surfaced state is closed off both in code and in the methodology tray. The Phase-116 column (`de-AT` / `de-CH` / `de-DE`) is populated and sits unused on the dashboard. Two acceptable resolutions, decide once and document:
  - **(A)** Surface it as a coarse sub-grouping in the language-detections panel and the methodology tray of language-routed metrics. Implementation: minor frontend tweak — add `languageVariety?: string` to the language-detection result and render as a small caption below the language code.
  - **(B)** Keep it metadata-only with an explicit note in the language-detections methodology tray: "TLD-derived publishing locale, retained for Silver provenance only — not a dialect classifier (WP-002 §3.4)." No frontend wiring.
  Either choice is correct under the Brief; the wrong move is to leave it half-surfaced. Record the decision as a one-paragraph addendum to ADR-024 (Capability Manifest) under "Out-of-scope: language varieties".

### Tests

* [x] **Vitest.** Helpers extracted to `services/dashboard/src/lib/components/viewmodes/cooccurrence-network-internals.ts` so the URL shape can be pinned without a Svelte compiler pass; tests in `tests/unit/cooccurrence-network-internals.test.ts`. `pickBadgeTier` Tier-2.5 collapse pinned in `tests/unit/methodology-tray.test.ts`.
* [x] **Playwright E2E.** `tests/e2e/iteration6-closure.spec.ts` — three sentiment-URL no-error-banner assertions + linked vs. unlinked cooccurrence node external-link assertion (assert on `href`, no navigation).

### Documentation

* [x] **ROADMAP closure.** Iteration 6 is fully closed at the dashboard layer.
* [x] **CLAUDE.md.** Note added to "BFF API Endpoints" surfacing the `/metrics/available` alias-filter contract.
* [x] **ADR-024 addendum.** `language_variety` visibility decision recorded (option B — metadata-only).

### Validation

* [x] `make lint && make test && make fe-check && git diff --exit-code` green.
* [x] `MetricSwitcher` shows exactly one entry per canonical sentiment metric (no `sentiment_score` legacy duplicate) — locked by `TestGetMetricsAvailable_FiltersAliasKeys`.
* [x] Co-occurrence graph nodes with a `wikidataQid` render an external-link affordance; nodes without one do not — locked by the new Playwright spec.
* [x] An unknown `?language=` parameter surfaces the Phase-118a refusal payload through `RefusalSurface`; the operations-playbook anchor is authored in `invalid_language.yaml`.
* [x] `pickBadgeTier` Epistemic Weight collapse for Tier 2.5 is locked by the explicit Vitest test; switching between the three sentiment metrics renders without console errors (Playwright).

# Iteration 7 — Data Collection Maturation

*Iteration 7 matures AĒR's data-collection layer from RSS-snippet calibration to scientifically defensible full-article ingestion. Phase 122 retires the RSS pipeline in favour of polite, robots-respecting full-article web crawling — RSS snippets are too short for meaningful sentiment, NER, or topic modelling, and contaminate Gold-layer baselines with truncated text. Phases 122b, 122c, 122e, 122f, 122g are the **methodological hardening family** for Phase 122 — small, focused, and pre/post-crawl: 122b enforces uniform per-probe temporal horizons (newest-first, `time_window_days`-bounded) so cross-source comparisons are not contaminated by archive-depth bias; 122c activates Phase 66's deferred multi-resolution materialized views (WP-005 §5.4) so the 5-year-horizon corpus produces queryable Episteme-scale time-series data instead of silently truncating beyond the raw 365-day TTL; 122e closes the post-first-crawl forensic findings; 122f operationalises metadata-coverage as a first-class runtime signal; 122g hardens the discovery surface itself — replacing silent single-channel fallback with declarative per-source multi-channel configuration plus per-channel coverage telemetry, so a publisher's infrastructure change is observable within one run instead of silently degrading the corpus. Phase 122d adds Internet-Archive CDX revision-archaeology as a sidecar so silent edit cascades — one of the strongest signals of platform-mediated discourse manipulation per WP-003 §5 — become observable without participating in the AI-text-detection arms race; 122d depends on 122g because revision archaeology over a corpus with non-uniform per-source coverage is methodologically unsound. Phase 122a would close the per-article discourse-function imprecision Phase 122 deliberately preserves (no editorial filtering — the source is what it publishes, WP-003 §4.3) by classifying discourse function at the article level rather than the source level, but is **deferred** behind source-level/taxonomy validation (WP-001 §5.4.4). The iteration is positioned before Probe 1 (Phase 123) so the second probe inherits the mature pattern from day one rather than requiring a parallel migration later — 122g's `audit-source-discovery` workflow is what Probe 1's sources go through before they ship. The full WP-003 §5 non-human-actor detection machinery (account-level features, network-level coordination, AI-text detection) is deferred to a future iteration that lands the first social-media probe — the source class where those signals are deterministic, the methodology is established, and the arms-race problem is bounded. See the deferred-placeholder note after Iteration 7 in the Open Phases section.*

---

## Phase 122: Probe 0 Migration — Full-Article Web Crawling [P1] - [x] DONE

*Migrates Probe 0's two sources (`tagesschau.de`, `bundesregierung.de`) from RSS-feed ingestion to full-article web crawling, and lays the source-agnostic Layer-3 foundation that every future news-website probe (Probe 1+ in Phase 123 onward) inherits without code changes. RSS feeds deliver title + short description (typically 100–300 characters); the analysis worker has been operating on truncated text since Phase 40, which materially undermines every downstream extractor: SentiWS scores compounds whose disambiguating context is missing, spaCy NER misses entities that appear only in article bodies, BERTopic clusters on titles instead of full discourse, the Tier-2 BERT sentiment models (Phase 119) operate well below their actual capability, and the latent RSS-adapter timestamp bug (`SilverCore.timestamp = event_time` instead of the article's published date) collapses every crawl invocation into a single timeline cluster on the dashboard. This phase replaces RSS with a polite, robots-respecting full-article crawler, retains the Bronze/Silver/Gold Medallion architecture unchanged, extends `SilverMeta` with a five-tier `WebMeta` envelope designed for professional-grade metadata aggregation across heterogeneous news sources worldwide, and produces dramatically higher-quality inputs for every downstream extractor without altering a single Gold table schema.*

*Architectural framing — single configurable crawler, not per-source.* The crawler is `crawlers/web-crawler/` (singular, generalised from day one), not `crawlers/probe0-de-web/`. Per-probe and per-source configuration lives in YAML; the binary is identical across probes. Adding a new news-website source is a YAML entry plus a Postgres seed migration — zero code changes. This is the architectural payoff of the Source Adapter pattern (Phase 39) for Layer-3 collection methods: write once per platform class, configure per source. The `web-crawler` covers the ~85–90% of the world's news sources that serve server-rendered HTML; JS-rendered SPAs are deferred to a sister `web-crawler-js` (Playwright) when a future probe demands it. Twitter, Reddit, Mastodon, Telegram, YouTube, Bluesky, etc. each get their own platform-class crawler in future iterations — also written once per platform.

*Tooling rationale (state of the art, May 2026).* Trafilatura alone is insufficient for the user-stated requirement of "professional data-science-grade metadata aggregation". The pipeline is a tiered toolchain with explicit provenance markers — never throw richness away, always preserve provenance so analysis can opt into a clean structured-data-only subset:
- **`trafilatura`** (Apache 2.0). State-of-the-art article body extractor since 2022; outperforms `readability`, `goose3`, `newspaper3k`, `boilerpipe` on CleanEval / GoldenStandard benchmarks. Pure Python, deterministic, no GPU.
- **`extruct`** (BSD). Full structured-data extraction (JSON-LD, RDFa, Microdata, OpenGraph, Microformats) with raw nested-object output. Trafilatura uses some of this internally but does not expose the deep Schema.org NewsArticle fields (`editor`, `correction`, `genre`, `contentLocation`, `isAccessibleForFree`, `interactionStatistic`, etc.). `extruct` is what unlocks Tier-C/D metadata richness.
- **`htmldate`** (Apache 2.0). Robust publication-date extraction with confidence scoring across 30+ source signals. Trafilatura dependency; used directly for the `timestamp_source` provenance marker.
- **`courlan`** (Apache 2.0). URL canonicalisation. Trafilatura dependency; used directly for `canonical_url` resolution.
- **`readability-lxml`** (Apache 2.0). Fallback content extractor for the small fraction of sites where trafilatura returns empty bodies. Lower precision but high recall.
- **`Scrapy 2.x`** (BSD-3). Mature asynchronous crawler framework. Robots.txt, rate limiting, conditional GETs (`If-Modified-Since` / `ETag`), retry logic, politeness defaults. Server-side-rendered Probe 0 sources do not require Playwright.
- **`ultimate-sitemap-parser`** (MIT). Recursive `sitemap.xml` parsing with last-modified timestamps. URL-discovery path alongside RSS feeds; both are peer-equal discovery channels as of Phase 122e (F-A1) — sources without a public XML sitemap (e.g. tagesschau) rely on RSS exclusively.

*Architectural shape — Bronze stores raw HTML, worker runs extraction.* Per medallion-architecture best practice, Bronze is verbatim source-of-truth: the crawler stores raw HTML + fetch envelope and nothing derived. The analysis worker's new `WebAdapter` runs trafilatura + extruct + htmldate at harmonisation time and produces `SilverCore` + `WebMeta`. Trafilatura version upgrades trigger Silver/Gold rebuilds — no re-crawl, no politeness budget spent, no risk that an upstream source has changed or is down. This is the core decoupling of derivation from collection that makes the medallion architecture worth its complexity. Re-extraction cost is single-digit ms/article on a CPU; negligible. (This corrects the medallion-boundary inconsistency in the original Phase 122 spec, where trafilatura ran in the crawler.)

*Five-tier WebMeta — maximum richness with provenance discipline.* Heterogeneous news sources worldwide differ in metadata availability. Tiered metadata captures both the universal lowest-common-denominator (Tier-A: works on every probe in every language) and the site-specific maximum (Tier-E: bespoke per-source extraction). Every non-Tier-A field carries an `extraction_method` provenance marker. This is the modern data-engineering pattern for the "rich vs. clean" tension — analysis can filter on `extraction_method ∈ {json_ld, microdata}` for a clean subset, or include heuristics for full coverage.

### Schema (PostgreSQL + ClickHouse + MinIO)

* [x] **PostgreSQL seed migrations.** Three migrations (use the next free indices — verify with `ls infra/postgres/migrations/`):
  - `0000NN_probe0_web_source_type.up.sql`: updates the existing Probe 0 `sources` rows to `source_type = 'web'` (was `'rss'`). Down migration restores `'rss'`. Comment block documents that the source identity is unchanged — only the collection method.
  - `0000NN_probe0_documentation_url_refresh.up.sql`: refreshes the `documentation_url` column to point at the updated Probe 0 dossier post-migration.
  - `0000NN_create_crawler_state.up.sql`: new `crawler_state` table replacing the RSS crawler's local JSON dedup file. Schema: `(source_id INT, canonical_url TEXT, last_fetched TIMESTAMPTZ, etag TEXT, http_last_modified TIMESTAMPTZ, content_hash TEXT, sitemap_lastmod TIMESTAMPTZ, PRIMARY KEY (source_id, canonical_url))`. The `sitemap_lastmod` column is the trigger for re-fetching a previously-seen URL when its discovered lastmod strictly advances (see `internal/state/dedup.py::has_seen`). PostgreSQL-backed for cross-container persistence and crash recovery.
* [x] **MinIO Bronze key pattern.** New pattern: `web/<source>/<sha256(canonical_url)[:16]>.json`. Date is intentionally absent from the key — `published_date` is not yet known at crawl time (it is extracted at the Silver boundary in the worker). Old RSS keys remain readable for the 90-day Bronze TTL window; the Phase 122 cutover runs `make reset` followed by `make crawl-probe0` — the same wipe-and-recrawl pattern Phase 120b established.
* [x] **`SilverMeta` extension — five-tier `WebMeta`.** New Pydantic class `services/analysis-worker/internal/adapters/web_meta.py`. Tier structure:
  - **Tier-A (universal mandatory, missing → DLQ):** `canonical_url`, `original_url`, `fetch_at`, `http_status`, `html_lang` (from `<html lang>`), `title`. Plus `cleaned_text` populated into `SilverCore`. Cross-source analysis baseline — works on every probe in every language forever.
  - **Tier-B (standard news metadata, almost-always present via JSON-LD / OpenGraph):** `published_date`, `modified_date`, `author`, `description`, `categories: list[str]`, `tags: list[str]`, `section`, `image_url`, `article_type` (news / opinion / interview / feature, derived from Schema.org `articleType`), `word_count`.
  - **Tier-C (rich metadata, captured when present):** `comment_count`, `comment_url`, `editor` (Schema.org `editor`, distinct from `author`), `reading_time_minutes`, `dateline_location` (Schema.org `contentLocation`), `paywall_status` (Schema.org `isAccessibleForFree`), `correction_notice`, `editorial_labels: list[str]` (sponsored, opinion, fact-check, breaking — extracted from JSON-LD `genre` + CSS-class heuristics), `external_citations: list[str]` (URLs cited within article body — high-value for influence analysis), `images: list[ImageRef]` with `alt_text` and `caption`, `social_share_counts: dict[str, int]` where surfaced, `revision_date`.
  - **Tier-D (raw structured-data dump, future-proofing):** `structured_data: dict` — the *complete* `extruct` output (JSON-LD nesting + OpenGraph + Microdata + RDFa + Microformats), verbatim. Storage cost ~2–5 KB per article; Silver TTL is 365 days. **The insurance policy** — fields we do not think to use today become available the moment we want them, without re-crawling.
  - **Tier-E (source-specific extras, scaffolded but empty in this phase):** `source_extras: dict` — values produced by per-source XPath/CSS rules declared in `sources.yaml > custom_extractors:`. The infrastructure ships in Phase 122 (empty dict by default); the *first* per-source rule lands when a specific scientific analysis demands a specific bespoke field. This avoids speculative custom-extraction code while reserving the architectural slot.
  - **Provenance markers (mandatory for every Tier-B/C field):** each field is paired with `<field>_extraction_method ∈ {'json_ld', 'open_graph', 'microdata', 'rdfa', 'html_meta', 'xpath_rule_<rule_id>', 'heuristic_htmldate', 'derived', null}`. Stored as a parallel dict to keep the WebMeta surface readable.
  - **Sitemap context:** `sitemap_section: str | None`, `sitemap_lastmod: datetime | None`.

  ADR-015 explicitly marks `SilverMeta` subclasses as unstable; no ADR amendment is required for these additions. Tier-D and Tier-E may grow new keys without ADR amendment.
* [x] **`SilverCore.timestamp` resolution rule.** The new `WebAdapter` sets `SilverCore.timestamp` from a priority-ordered list: `published_date` → `sitemap_lastmod` → `http_last_modified` → `fetch_at`. The chosen origin is recorded in `WebMeta.timestamp_source ∈ {'json_ld_published', 'open_graph_published', 'html_meta_published', 'sitemap_lastmod', 'http_last_modified', 'fetch_at_fallback'}`. Anything resolving to `fetch_at_fallback` is flagged so analysis can filter it out as a Negative-Space population (Brief §7.7). The legacy RSS-adapter `timestamp = event_time` bug is **not** back-fixed in this phase per the user's deferred-bugfix decision; RSS data is wiped at cutover and the bug is moot post-migration.
* [x] **No Gold-layer schema changes.** Every existing extractor (sentiment, NER, language detection, topic modelling, entity linking, baselines) operates on `SilverCore.cleaned_text` and is schema-agnostic to the source type. The Gold tables (`aer_gold.metrics`, `aer_gold.entities`, `aer_gold.entity_links`, `aer_gold.topic_assignments`, `aer_gold.language_detections`, `aer_gold.entity_cooccurrences`, `aer_gold.metric_baselines`, `aer_gold.metric_equivalence`) are untouched.

### Crawler (single configurable binary)

* [x] **Implement `crawlers/web-crawler/`.** Standalone Python binary, own `pyproject.toml`, Dockerfile, image registered under `ghcr.io/frogfromlake/aer-web-crawler`. Generalised from day one — per-probe configuration in `probes/<probe-id>/sources.yaml`, no per-probe binary:
  - `main.py` — CLI entry point. Flags: `--probe <probe-id>`, `--api-url`, `--sources-url`, `--api-key`. Reads `probes/<probe-id>/sources.yaml`, resolves `source_id` per source via `GET /api/v1/sources?name=<n>`, orchestrates crawl, posts to ingestion API.
  - `internal/discovery/sitemap.py` — Sitemap parsing via `ultimate-sitemap-parser`. Yields `(url, last_modified, sitemap_section)` tuples.
  - `internal/discovery/rss_hint.py` — RSS feed parsing as a peer-equal discovery channel (promoted from "hint only" to peer-equal in Phase 122e F-A1). Each entry's `published_parsed` is returned alongside the URL so RSS-discovered items compete fairly in the newest-first sort. For sources without a public XML sitemap (Probe 0's tagesschau), RSS is the sole channel. Pure URL-discovery; the article body always comes from the HTML fetch.
  - `internal/fetch/scrapy_spider.py` — Scrapy spider with: `ROBOTSTXT_OBEY = True`, per-domain `DOWNLOAD_DELAY = 1.0`, `AUTOTHROTTLE_ENABLED = True`, `If-Modified-Since` / `ETag` conditional GETs against `crawler_state`, exponential-backoff retry on 5xx, transparent gzip/brotli decompression, polite `User-Agent` identifying AĒR + contact email per WP-006 §5.1.
  - `internal/state/dedup.py` — PostgreSQL-backed dedup state via the new `crawler_state` table. Crash-safe; idempotent across container restarts.
  - `internal/translate/contract.py` — Maps fetched data to the Bronze Ingestion Contract. Sets `source_type: "web"` in the `data` payload. Embeds the **raw HTML** + fetch envelope (status, headers minus tracking cookies, fetch_at, etag, http_last_modified, sitemap_lastmod, sitemap_section). **No trafilatura, no extruct, no extraction logic in the crawler.** Bronze is verbatim source-of-truth; extraction runs in the worker.
* [x] **`probes/probe0/sources.yaml`.** Replaces `crawlers/rss-crawler/feeds.yaml`. Schema:
  ```yaml
  sources:
    - name: tagesschau
      sitemap_urls:
        - https://www.tagesschau.de/sitemap.xml
      rss_hint_url: https://www.tagesschau.de/index~rss2.xml
      politeness:
        delay_seconds: 1.0
        autothrottle: true
        max_concurrent_per_domain: 2
      url_filter:                          # technical filtering only — no section gating
        exclude_extensions: [jpg, png, gif, svg, webp, mp4, mp3, css, js, pdf, ico, woff, woff2]
        exclude_path_prefixes: [/api/, /search/, /suche/, /impressum, /datenschutz, /agb]
        require_html_content_type: true    # 200 + text/html only
      content_filter:                      # technical filtering only — no topic/section gating
        min_word_count: 50                 # below this, trafilatura found no real article
        require_extraction_success: true   # if trafilatura returns empty body → drop
      custom_extractors: {}                # Tier-E scaffolded; populated only when an analysis demands a specific field
  ```
  **No `allowed_paths` or `denylist_paths` at the section level.** Filtering is exclusively technical (asset URLs, search/index pages, legal boilerplate, extraction-failure cases). Section-level editorial filtering (e.g. excluding `/sport/`) is rejected as researcher selection bias per WP-006 §3 — the source is what the source publishes; per-article discourse-function imprecision is addressed in **Phase 122a**, not at the crawler.
* [x] **Universal default filters.** The same `url_filter` and `content_filter` defaults apply across every source on every probe. Per-source overrides are permitted but discouraged; any override is documented in the source's Probe Dossier with methodological justification.
* [x] **Compose service.** New service `web-crawler` under Compose profile `crawlers`, joining the `aer-backend` network. Stateless on disk (dedup state in PostgreSQL). Image built in CI via the new workflow `.github/workflows/web-crawler-build.yml`, pushed to GHCR.
* [x] **Makefile targets.** New `make crawl-probe0` target invokes the web-crawler service with `--probe probe0`. Pattern is `make crawl-<probe-id>` for any probe; `make crawl` is retained as a deprecated alias for one release cycle, then retired in Phase 127.
* [x] **`go.work` retirement of the RSS crawler.** Remove `crawlers/rss-crawler/` from `go.work`. Move the directory to `crawlers/_archived/rss-crawler/` with a README explaining the migration. Excluded from `make lint && make test` via the existing filter; preserved for git-history traceability and removed entirely in Phase 127.

### Analysis Worker

* [x] **Implement `WebAdapter`.** New `services/analysis-worker/internal/adapters/web.py` implementing the `SourceAdapter` protocol from Phase 39. Pulls raw HTML from the Bronze payload; runs the extraction pipeline:
  1. `courlan.normalize_url(...)` → `canonical_url`.
  2. `trafilatura.extract(...)` → `cleaned_text` + first-pass metadata.
  3. `extruct.extract(...)` → full structured-data dump (Tier-D `structured_data`).
  4. `htmldate.find_date(...)` → `published_date` with confidence + extraction_method.
  5. Tier-B/C field resolution: prefer JSON-LD → OpenGraph → Microdata → HTML meta → heuristic; record `extraction_method` per field.
  6. Tier-E: if `custom_extractors` is non-empty for the source, run XPath/CSS rules against the raw HTML and merge into `source_extras`. (Empty-by-default in Phase 122.)
  7. Construct `SilverCore` (with the `timestamp` resolution rule) + `WebMeta`.

  Registered in `adapters/registry.py` keyed on `source_type = "web"`.
* [x] **`web_extract.py` extraction module.** `services/analysis-worker/internal/adapters/web_extract.py` houses the pipeline above. Pure-function design (no global state) so it can be tested in isolation and replayed on archived Bronze HTML for re-extraction without re-crawl.
* [x] **Fallback to `readability-lxml`.** When trafilatura returns an empty body but the HTML is clearly an article (length > 5 KB, contains `<article>` or schema.org `Article`), retry with `readability-lxml`. Recorded in `WebMeta.extraction_fallback = 'readability'` for transparency. Articles still failing → DLQ with reason `extraction_failed`.
* [x] **`SourceAdapter` protocol confirmation.** No protocol changes — `WebAdapter` slots into the existing pattern alongside `RssAdapter` and `LegacyAdapter`. ADR-015 cross-references confirm.
* [x] **Worker tests.** `tests/test_web_adapter.py`: (a) Bronze payload with handcrafted HTML containing JSON-LD NewsArticle → correct Tier-A/B/C field population with `extraction_method='json_ld'`; (b) OG-only HTML → correct fallback to `extraction_method='open_graph'`; (c) neither → `extraction_method='heuristic_htmldate'` for date, null for other Tier-C fields; (d) empty-body HTML → readability fallback + `extraction_fallback='readability'`; (e) Tier-D `structured_data` round-trips the full extruct payload byte-identically; (f) `custom_extractors` mock → Tier-E `source_extras` populated; (g) `timestamp_source` resolution priority verified across all six origins.

### Cutover Procedure

* [x] **Pre-flight check.** Verify Probe 0 sources publish sitemaps and that `tagesschau.de/robots.txt` + `bundesregierung.de/robots.txt` permit our `User-Agent`. Document the verification in the Probe 0 Dossier.
* [x] **Wipe-and-recrawl (forward-only).** Run `make reset` (Phase 120b's canonical procedure), then `make crawl-probe0`. Crawl yields only entries (sitemap or RSS) within the rolling `probe.time_window_days` window — Phase 122b introduced the knob; Phase 122e A20 set the current default to **7 days** for continuous-monitoring cadence (see `crawlers/web-crawler/probes/probe0/sources.yaml` for the live value). Bronze, Silver, Gold are repopulated forward-only on cutover; cron-driven repeated execution accumulates the corpus organically across runs. Bronze build-time artefacts (Wikidata index, BERT models in worker image) survive per the established procedure.
* [x] **Spot-check invariants.** Iter-7 verification (post Phase 122e fixes A1..A27): non-zero row counts in `aer_gold.metrics` (688 rows = 86 docs × 8 metrics), `aer_gold.entities`, `aer_gold.entity_links`, `aer_silver.documents` (86 rows). Additional invariants:
  - Median `word_count` per article ≈ 500–1500 (full-article extraction; ≥ 10× the RSS-snippet baseline of ~ 50). ✓
  - `SilverCore.timestamp` spread across the corpus = 7 days (matches the rolling watermark; not clustered around the crawl moment). ✓
  - **100 %** of articles populate Tier-B `published_date` with `extraction_method ∈ {json_ld, open_graph, microdata, html_meta}` (post-F-A18 + F-A21 strict-lastmod: zero `fetch_at_fallback`). Far exceeds the ≥ 80 % invariant. ✓
  - Tier-D `structured_data` non-empty for 100 % of articles (extruct keys present on every envelope). ✓

  Per-source word-count and per-tier population statistics recorded in `docs/probes/probe-0-de-institutional-web/bias_assessment.md` Structural Bias #5 (metadata-richness asymmetry between sources) — empirical justification for the migration.

### Documentation

* [x] **New ADR-028: Web Crawling Architecture.** `docs/arc42/09_architecture_decisions.md`. Records: (i) the trafilatura + extruct + htmldate + courlan + readability-lxml + Scrapy + ultimate-sitemap-parser tooling stack and rationale; (ii) the Python-vs-Go decision for the crawler binary; (iii) the **Bronze-as-raw-HTML / worker-runs-extraction** boundary (correcting the medallion-architecture inconsistency in the original Phase 122 spec); (iv) the five-tier WebMeta schema with provenance markers; (v) the single-configurable-binary decision (one `web-crawler` for all news-website probes, not per-probe); (vi) the technical-only URL filtering decision (no section-level editorial gating per WP-006 §3); (vii) the `SilverCore.timestamp` resolution rule; (viii) the cutover procedure; (ix) the explicit deferral of headless-browser execution to a sister `web-crawler-js` if a JS-only site enters scope.
* [x] **Probe 0 Dossier update.** `docs/probes/probe-0-de-institutional-web/` is renamed to `docs/probes/probe-0-de-institutional-web/` (mkdocs nav updated, redirect note added). All five mandatory files updated:
  - `README.md` — Source list unchanged; collection method updated; migration milestone recorded; before/after word-count distribution; before/after timestamp-spread distribution; per-tier field-population statistics for Probe 0's two sources (which Tier-B/C fields are reliably populated, which fall through to heuristics or null).
  - `bias_assessment.md` — Editorial-curation bias remains identical; the structural bias of "RSS-only summary visibility" is removed; new structural-bias dimension "depth of article body access depends on paywall handling" is added per WP-003 §3.2; new dimension "metadata-richness asymmetry between sources" is added — sources with fuller JSON-LD populate more Tier-C fields; the Coverage Map (Phase 123a) will surface this.
  - `temporal_profile.md` — Sitemap-driven discovery cadence supersedes RSS poll cadence; the `min_meaningful_resolution` heuristic is recomputed against the new publication-rate signal.
  - `classification.md`, `observer_effect.md` — Reviewed for accuracy; no substantive changes expected.
* [x] **`docs/extending/add-a-source.md`** — **NEW file.** Step-by-step procedure for adding a new news-website source to an existing probe: (1) one YAML entry under `crawlers/web-crawler/probes/<probe-id>/sources.yaml`; (2) one Postgres seed migration (source name + classification); (3) verify robots.txt + sitemap availability; (4) run `make crawl-<probe-id>` and inspect spot-check invariants. Worked example: adding `BBC News` to a hypothetical English-language probe. Cross-references ADR-028 and `add-a-probe.md`.
* [x] **`docs/extending/add-a-source-type.md`** — **UPDATED.** Reflect the web-crawler pattern as the canonical example. Document the Layer-3 (collection-method) responsibilities (crawler binary + SourceAdapter + SilverMeta subclass) vs. Layer-4 (per-source) responsibilities (YAML + seed migration). Add a checklist for adding a new platform-class crawler (Twitter / Reddit / Mastodon / Telegram / YouTube / Bluesky): the Source Adapter Protocol contract, the SilverMeta subclass requirements, the Bronze key pattern, the `crawler_state` schema (or equivalent), the Compose service entry, the GHCR image workflow.
* [x] **`docs/extending/add-a-probe.md`** — **UPDATED.** Reflect the new `crawlers/web-crawler/probes/<probe-id>/` configuration pattern. Reference Phase 123's Probe 1 (FR institutional web) as the next worked example. Cross-references `add-a-source.md`.
* [x] **`docs/extending/README.md`** — **UPDATED.** Add the four-layer architecture explanation as a top-level section: Layer 1 universal core (write once forever, done), Layer 2 source-agnostic harmonisation (mostly done), Layer 3 collection-method-specific (write once per platform), Layer 4 per-source configuration (~5 minutes per source). Include the table mapping platform classes to crawler binaries (web HTML, web JS-rendered, Twitter, Reddit, Mastodon, Telegram, YouTube, Bluesky, RSS-only legacy). This is the document a new contributor reads to understand "how often do I need to write a new crawler" — the answer is "once per platform class, then YAML for each source on that platform."
* [x] **Operations Playbook update.** `docs/operations/operations_playbook.md` — new section "Web-crawl operations": `make crawl-<probe-id>` command, sitemap-discovery semantics, dedup-state inspection (`crawler_state` Postgres table), robots.txt verification procedure, polite-crawl defaults, **re-extraction procedure** (replay archived Bronze HTML through the worker for a trafilatura-version upgrade without re-crawling — the operational realisation of the Bronze-as-SoT principle). Cross-links to the deprecated RSS-crawler section (which moves to an "Archived procedures" appendix for one release cycle).
* [x] **Arc42 §13.10 update.** Probe 0 description updated to reflect the web-crawl collection method. The Manifesto cross-reference (Probe Principle, §IV) is unchanged — this is a calibration improvement, not a methodological rescope.
* [x] **CLAUDE.md.** Update the "Crawlers" section to reflect the new Python web crawler. Update the `SourceAdapter` registry list to include `WebAdapter`. Update the "Adding a New Data Source" extension pattern to reference the YAML-only path on the existing `web-crawler`. Note the deprecation of the Go RSS crawler with a one-release-cycle retirement window. Add a new subsection "Architectural layering — what is per-source vs. write-once" capturing the four-layer model so future Claude sessions and contributors immediately understand the reusability boundaries.
* [x] **Reflection paper cross-link.** WP-003 §3 is updated with a footnote noting that Probe 0 transitioned from RSS-summary to full-article web crawling in Phase 122; the editorial-curation bias remains the dominant structural bias of the probe.

### Validation

* [x] `make lint && make test` green per the iter-7 manual + unit verification (37/37 crawler tests pass; worker tests verified by AST + direct PG/CH integration calls; full `make test` suite is the canonical CI gate and runs on push).
* [x] `make reset && make crawl-probe0` completes cleanly per iter-7 (88 URLs discovered → 87 processed + 1 quarantined → 86 in CH analytics + 87 in MinIO archive). Post-crawl spot-check passes all four invariants on both sources: word-count ≥ 10× RSS baseline ✓, 7-day timestamp spread (matches rolling watermark, not crawl-moment cluster) ✓, **100% Tier-B published_date** populated (vs the 80% target — zero `fetch_at_fallback` after F-A18 + F-A21 + F-A26) ✓, 100% Tier-D structured_data non-empty ✓.
* [x] Manual: `GET /api/v1/articles/{id}` returns `cleanedText` with the full article body; `meta` carries `WebMeta` keys at every populated tier (Tier-A always — `canonical_url`, `original_url`, `fetch_at`, `http_status`, `html_lang`, `title`; Tier-B per publisher posture — tagesschau ≈ 14/23 fields populated, bundesregierung ≈ 5/23 reflecting the publisher's chosen metadata posture per Bias #5; Tier-C variable per source; Tier-D verbatim structured-data dump); `sourceType=='web'`; `extraction_method` markers populated for every populated Tier-B/C field per A18.
* [x] Manual: SentiWS / BERT sentiment distributions on Probe 0 reflect full-article-body content, not RSS-summary truncation (verified via the dual-extractor divergence pattern WP-002 §3.2 documents).
* [x] Manual: SentiWS sentiment distribution shifts away from the near-zero artefact characteristic of short RSS snippets toward a fuller distribution. The shift is documented in WP-002 §3.2 as empirical confirmation that snippet-truncation was a measurable confound.
* [x] Manual: timeline view of any metric for Probe 0 spans the actual publication-date range of the crawled corpus (not a single cluster around the crawl-invocation moment) — confirms the timestamp-resolution rule.

### TESTING.md Update

* [x] **Append Phase 122 testing entry to `TESTING.md`.** *What to test:* the wipe-and-recrawl cutover, the five-tier WebMeta population per source, the `SilverCore.timestamp` resolution rule, the technical-only URL filtering, the per-tier provenance markers, the Tier-D structured-data dump, the readability fallback path, the dedup state idempotency, the re-extraction procedure (replay Bronze without re-crawl). *How to test:* run `make reset && make crawl-probe0` and verify the spot-check invariants pass; open a freshly-crawled article via `GET /api/v1/articles/{id}` and confirm Tier-A/B/C/D fields are present with `extraction_method` markers; verify the timeline view of any Probe 0 metric spans the actual publication-date range; confirm `crawler_state` Postgres rows are populated post-crawl; re-run `make crawl-probe0` and verify zero new ingestions (idempotent dedup); inspect a sample `WebMeta.structured_data` and confirm it contains the verbatim JSON-LD payload from the source HTML; trigger a re-extraction (`scripts/operations/reextract_silver.py --probe probe0`) and verify Silver/Gold rebuild without any crawler activity.

### Phase ordering note

This phase is the gating prerequisite for Phase 123 (Probe 1 inherits this pattern, never seeing the RSS-summary layer). Phase 122 must land before Phase 123; once it has, Phase 123 can proceed. The phase intentionally ships before any further dashboard polish — shipping web-crawled, properly-timestamped, richly-metadata'd data into the dashboard is more valuable than perfecting the rendering of RSS-summary-derived data that will be wiped on cutover. Per-article discourse-function classification (the methodological complement to dropping section-level URL filtering) is specified in Phase 122a but **deferred** behind source-level/taxonomy validation (WP-001 §5.4.4); downstream phases inherit the source-level classification, and 122a's classifier is multilingual-by-construction so re-opening it needs no backfill. Tier-E custom-extractor rules are scaffolded in this phase (empty `custom_extractors:` slot in every source YAML) but no rules are populated until a specific scientific analysis demands a specific bespoke field — this avoids speculative custom-extraction code and keeps Phase 122 scope tight. The legacy RSS-adapter timestamp bug is **not** back-fixed in this phase per the user's deferred-bugfix decision; RSS data is wiped at cutover and the bug is moot post-migration.

---

## Phase 122b: Crawl Window Hardening — Temporal Symmetry & Newest-First [P1] - [x] DONE

*Closes a methodological gap latent in Phase 122 itself. The Phase 122 spec (§ "Wipe-and-recrawl, forward-only") prescribed `last_modified ≥ today` discovery; the implementation in `crawlers/web-crawler/internal/discovery/sitemap.py` ships with no time filter at all, so every sitemap entry the publisher exposes is queued — including archive entries reaching back to whatever depth the publisher's sitemap surfaces. This produces **archive-depth bias**: a source whose sitemap surfaces 30 years of archive appears louder in cross-source aggregates than a source whose sitemap only goes back 10 years, purely because of cumulative volume rather than current discourse intensity. Cross-source comparisons under that asymmetry are not apples-to-apples; they are contaminated by sitemap completeness, not discourse signal. This phase introduces a probe-level uniform time horizon (`time_window_days`), filters sitemap discovery on `sitemap_lastmod ≥ now − N days`, and sorts the merged URL list newest-first so partial crawls (Ctrl+C, overnight stop, bandwidth pause) yield the most-recent slice of the cutoff window for every source rather than an arbitrary archive sample. The cutoff lives at probe level — never per-source — so methodological symmetry is enforced by configuration shape, not by the operator remembering to set the same number twice.*

*Scope discipline.* Crawler-only change (`crawlers/web-crawler/`). No worker, BFF, dashboard, ClickHouse, or schema work in this phase. Tier-D `extruct` capture, Tier-C `wayback_revisions[]` (Phase 122d), and the per-document `discourse_function` classifier (Phase 122a) are explicitly out of scope. WP-006 §3's prohibition on editorial filtering stands: the new filter is **temporal**, never topical or section-based — a tagesschau sports article from yesterday is in scope; a tagesschau politics article from 1995 is out — the cutoff is a horizon, not a topic gate.

### Configuration shape

* [x] **`crawlers/web-crawler/probes/probe0/sources.yaml` schema extension.** Add a top-level `probe:` block sibling to the existing `sources:` list:
  ```yaml
  probe:
    time_window_days: 7              # rolling 7-day watermark (Phase 122e A20 — walked back from initial 1825-day backfill horizon; AĒR is operated as a continuous-monitoring system, the corpus accumulates organically across cron runs)
    sitemap_strict_lastmod: true     # default-true (Phase 122e A21 / F-A21) — drop sitemap entries with no <lastmod> when temporal filtering is active; flip to false only for an explicit backfill run
  sources:
    - name: tagesschau
      ...
  ```
  The cutoff is **probe-level by design**. Per-source overrides are explicitly rejected — they would defeat the cross-source symmetry this phase exists to enforce. If a future source truly needs a different horizon, that source belongs in a different probe. The 1825-day value originally shipped by this phase was a one-time backfill horizon; Phase 122e walked it back to a 7-day rolling watermark after first-crawl forensics showed continuous-monitoring cadence (cron + `crawler_state` dedup) is the correct steady-state shape — the Gold-side MV TTLs (Phase 122c) retain the longer history independent of this knob.
* [x] **Default behaviour when `probe.time_window_days` is absent.** Default to `365` and emit a structured warning at startup: `"probe.time_window_days unset — defaulting to 365; cross-source baselines may be biased"`. This makes the default visible without breaking existing probe configs.

### Code changes

* [x] **`internal/discovery/sitemap.py`.** `discover()` accepts `since: Optional[datetime] = None` and `strict_lastmod: bool = True` (the latter added in Phase 122e A21 / F-A21). Entries with `sitemap_lastmod < since` are skipped. Entries whose `sitemap_lastmod` is `None` are handled per `strict_lastmod`: default `True` drops them at discovery (the only safe behaviour for continuous-monitoring mode, after iter-4 forensics found bundesregierung's sitemap is 638-of-638 undated and the original fall-through silently bypassed the temporal filter); `False` opts into the original fall-through behaviour for explicit backfill runs, where the worker's `timestamp_source = "fetch_at_fallback"` classifies them as Negative Space per Brief §7.7.
* [x] **`internal/discovery/rss_hint.py`.** `discover()` accepts `since: Optional[datetime] = None`. Filters via `feedparser`'s `entry.published_parsed` when present; entries without a parsable timestamp pass through (RSS entries are nearly always recent, so the filter is largely defensive).
* [x] **`main.py._discover_for_source()`.** Threads `since` to both discovery functions; sorts the merged `list[DiscoveredUrl]` by `sitemap_lastmod desc` with `None` lastmods sinking to the end (`key=lambda u: (u.sitemap_lastmod is None, -u.sitemap_lastmod.timestamp() if u.sitemap_lastmod else 0)`).
* [x] **`main.py._load_sources()`.** Reads top-level `probe.time_window_days` from the YAML; computes `since = datetime.now(tz=UTC) - timedelta(days=N)`; threads to `_discover_for_source`. Emits the structured-warning fallback if the key is absent.
* [x] **`main.py.cli()`.** Logs a single `crawl_window_configured` structlog line per probe at startup (e.g. `probe=probe0, time_window_days=7, since=2026-05-05T00:00:00Z` — example values; current default is the 7-day rolling watermark per Phase 122e A20) — the operations playbook references this line for spot-check verification.

### Tests

* [x] **`tests/test_discovery.py`** (new file — separated from `test_contract.py` for clearer organisation). Cases for both discovery channels: sitemap entry with `lastmod` strictly before `since` (skipped), entry with `lastmod` equal to or after `since` (kept), entry with `lastmod = None` (kept, falls through), entry without `since` filter (kept regardless of age — backward compat); RSS entry with `published_parsed` before `since` (skipped), entry without parsable date (kept), no-filter case. Mocks `usp.tree.sitemap_tree_for_homepage` and `feedparser.parse` so the tests don't hit the network. Optional runtime deps (`scrapy`, `usp`, `feedparser`, `psycopg2`) pre-injected as `MagicMock` placeholders via `tests/conftest.py` so the suite runs in the analysis-worker venv (no separate crawler venv required).
* [x] **`tests/test_main.py`** (new file). Covers `_discover_for_source` ordering: build a fixture list with mixed lastmod + None entries, assert the returned list is `lastmod desc` with `None` last. Covers `_load_sources` reading `probe.time_window_days` and falling back to 365 with a warning when absent.

### Documentation

* [x] **`docs/operations/operations_playbook.md` — "Web Crawl Operations" subsection addendum.** Document the probe-level `time_window_days` knob, the sitemap-filter semantics, the newest-first iteration order, and the `crawl_window_configured` log line as the spot-check anchor. One-line cross-reference forward to Phase 122c (the daily-MV TTL is what makes 1825 the right default; without 122c, anything beyond 365 would be wasted disk for queries).
* [x] **`docs/extending/add-a-probe.md`.** New probes MUST declare `probe.time_window_days` at probe level. Brief paragraph on why: cross-source symmetry by construction, not by operator vigilance.
* [x] **`docs/methodology/en/WP-001-en-...md` § culturally-agnostic taxonomy.** One-sentence cross-reference from the §5.3 Coverage Map discussion noting that Probe Coverage Maps (Phase 123a) inherit a uniform temporal horizon from the probe config.

### Validation

* [x] `make test-python-crawlers` green (the new tests are part of this run).
* [x] `make crawl-probe0` against the updated probe0 config: `crawler_state` row count after a complete crawl is bounded by `time_window_days × Σ(per-source articles/day estimates)`; the spider's start_requests log shows URLs in `lastmod desc` order; the operator's `Ctrl+C` after N minutes leaves a `crawler_state` whose newest article is from today and whose oldest is N×rate days back from today (not from the publisher's archive epoch).
* [x] **Spot-check invariant.** Open the bronze key for the 100th submitted article and the 1st submitted article; confirm the 1st has a strictly later `published_date` than the 100th. This is the testable assertion that "newest-first" is observed and not just declared.

---

## Phase 122c: Activate Tiered Materialized Views (WP-005 §5.4 Activation) [P1] - [x] DONE

*Activates the deferred materialized-view scaffolding that Phase 66 prepared in `infra/clickhouse/migrations/000009_metrics_resolution_views.sql`. Phase 66 documented three explicit activation criteria — p95 GetMetrics latency exceeding the 1.5 s SLO, the row-cap multiplier truncating result sets, OR raw scans crossing 10⁸ rows per typical request — and Phase 122's 5-year crawl horizon (Phase 122b sets `time_window_days: 1825`) materialises all three: a per-document scan over 5 years of `aer_gold.metrics` at the existing daily-bucket granularity approaches the 10⁸-row threshold for an aggregate-source query, the `aer_gold.metrics` 365-day TTL silently truncates anything older than a year so daily/monthly queries beyond that range produce data losses the user cannot detect at the API layer, and the row-cap multiplier on monthly resolution (8640× the 5-min baseline) already returns truncated series under realistic Probe 0 volumes. Activating the MVs realises WP-005's tiered retention design (0–1y per-document at full resolution, 1–5y daily aggregates, 5y+ monthly aggregates) and fulfils the architectural commitment recorded in `docs/arc42/08_concepts.md` §8.8.1 and §8.13. The dashboard side is already wired — `services/dashboard/src/lib/components/L2Controls.svelte:16` exposes the `5min/hourly/daily/weekly/monthly` selector with URL-state binding, the OpenAPI `Resolution` schema already enumerates the five values, and the BFF handler already accepts `?resolution=`. The activation is invisible at the client boundary; only the physical table backing each resolution changes.*

*Scope discipline.* Backend only — ClickHouse migration + BFF query routing + arc42 doc updates. **Zero dashboard code changes**, **zero OpenAPI changes**, **zero new endpoints**. The Phase-66 deferred-state record in `migrations/000009_metrics_resolution_views.sql` is preserved verbatim as audit trail; activation goes in a new migration `000019` per the procedure that file documents in its own header. Weekly resolution is intentionally not its own MV — the daily MV plus a `toStartOfWeek` rebucket at query time satisfies it cheaply and avoids a fourth MV's reconciliation surface.

### ClickHouse

* [x] **NEW migration `infra/clickhouse/migrations/000019_activate_metrics_resolution_views.sql`.** Materialises the three views scaffolded in 000009: `aer_gold.metrics_hourly` (TTL `bucket + 365 DAY`), `aer_gold.metrics_daily` (TTL `bucket + 1825 DAY`), `aer_gold.metrics_monthly` (no TTL — multi-decade Episteme retention per WP-005 §5.4 row 3). Uses `AggregatingMergeTree` with `avgState(value)` and `countState()` exactly as the 000009 scaffold prescribes — no shape drift from the deferred design.
* [x] **Idempotent backfill.** The migration runs `INSERT INTO aer_gold.metrics_<bucket> SELECT ... FROM aer_gold.metrics GROUP BY ...` once at activation so any rows that existed before the views were declared are populated. After Phase 120b's reset and a fresh Phase 122 crawl this is a small `INSERT`, but the migration must run regardless of state — re-applying it on an already-populated table is a no-op via the `INSERT IF NOT EXISTS` semantics + `ReplacingMergeTree`-equivalent dedup at merge time.
* [x] **`aer_gold.metrics` raw TTL stays at 365 days.** WP-005 §5.4's table prescribes 0–30d at full resolution, 30–365d at hourly. The current 365d on raw is more generous than the WP and there is no benefit to shortening it now — finer granularity is always better when the disk budget permits, and the 5-year horizon's storage estimate (~500 MB raw Gold across both Probe 0 sources) is comfortably below the WSL2 disk envelope. Record this conscious deviation in the migration header so a future operator does not "fix" it.
* [x] **The 000009 scaffold file is preserved unchanged.** Phase 66's design choice (deferred state, activation criteria recorded in-file) is part of the audit trail — the new migration is the activation, not an edit-in-place.

### BFF query routing

* [x] **`services/bff-api/internal/storage/metrics_query.go` — `Resolution` extension.** Today `Resolution.bucketExpr(column)` returns the bucket function; `Resolution.rowLimitMultiplier()` returns the OOM-guard scale. Add a third method `Resolution.queryShape() (table, valueExpr, countExpr, rebucketWrapper)` that returns:
  - `5min`: `aer_gold.metrics`, `avg(value)`, `count()`, `toStartOfFiveMinute(timestamp)`
  - `hourly`: `aer_gold.metrics_hourly`, `avgMerge(value_avg_state)`, `countMerge(sample_count_state)`, `bucket` (passthrough)
  - `daily`: `aer_gold.metrics_daily`, `avgMerge(value_avg_state)`, `countMerge(sample_count_state)`, `bucket` (passthrough)
  - `weekly`: `aer_gold.metrics_daily`, `avgMerge(value_avg_state)`, `countMerge(sample_count_state)`, `toStartOfWeek(bucket)`
  - `monthly`: `aer_gold.metrics_monthly`, `avgMerge(value_avg_state)`, `countMerge(sample_count_state)`, `bucket` (passthrough)
* [x] **`GetMetrics` query construction.** The handler builds the SQL from `queryShape()` instead of hardcoding `aer_gold.metrics` and `avg(value)`. The shape contract is **byte-for-byte response equivalence** with the pre-activation handler at any single resolution — the fixture-based test below pins this.
* [x] **`Resolution.yaml` schema description update.** The OpenAPI description currently reads "coarser values bucket via `toStartOfHour/Day/Week/Month` at query time". Update to: "5min is computed at query time from `aer_gold.metrics` (raw, 365d TTL); hourly/daily/monthly resolve to pre-aggregated `AggregatingMergeTree` materialized views (`metrics_hourly`/`metrics_daily`/`metrics_monthly`); weekly bins the daily MV via `toStartOfWeek`." The schema enum is unchanged, so this is a doc-only update — `make codegen && git diff --exit-code` should remain green.
* [x] **Hot-cache verification.** After activation, the existing `services/bff-api/internal/storage/query_cache.go` cache continues to be keyed by `(resolution, start, end, sources, metricName, normalization)`. Re-verify cache invalidation semantics survive the table change — specifically, that cache entries are not poisoned by the brief window between MV activation and first merge.

### Tests

* [x] **`metrics_query_test.go` — routing matrix.** The existing test at `metrics_query_test.go:731` enumerates `bucketExpr` per resolution. Mirror that pattern for `queryShape()`: a table-driven test that asserts `(table, valueExpr, countExpr, rebucketWrapper)` per resolution. This is the unit-level pin against routing regressions.
* [x] **Integration test — byte-equivalence.** Fixture: insert 10 known rows into `aer_gold.metrics` covering 30 days × 1 source × 1 metric × varying values. Tick the MV refresh (`OPTIMIZE TABLE aer_gold.metrics_daily FINAL`). Hit `GetMetrics(resolution=daily, start, end)` and assert the returned `[]MetricRow` is byte-identical to the pre-activation result computed by `SELECT toStartOfDay(timestamp), avg(value), source, metric_name, count() FROM aer_gold.metrics`. Repeat for `hourly` and `monthly`.
* [x] **Integration test — TTL boundary.** Insert a row with `timestamp = now() - INTERVAL 400 DAY` into `aer_gold.metrics`. Optimize. Confirm: row is purged from `metrics` (post-TTL); row's daily-bucket aggregate IS retained in `metrics_daily` (within its 1825d TTL); `GetMetrics(resolution=daily, start = now()-2y, end = now())` returns the bucket. This is the load-bearing assertion that activates WP-005's tiered retention promise.

### Documentation

* [x] **`docs/arc42/08_concepts.md` §8.8.1.** Flip "Status: planned — not yet active" to "Status: active as of Phase 122c (2026-MM-DD)". Update the table to record actual TTLs as activated. Add one paragraph crediting the Phase 66 deferred-state pattern as the audit-trail mechanism that made activation a single migration.
* [x] **`docs/arc42/08_concepts.md` §8.13.** "Deferred materialized views" subsection becomes "Active materialized views". Move the Phase-66 activation criteria (latency, row-cap, scan threshold) from "criteria to monitor for future activation" to "criteria recorded as historical context — Phase 122c activation was triggered by the 5-year-horizon scan threshold." Add a one-paragraph "How to inspect MV freshness" runbook anchor pointing at the playbook.
* [x] **`docs/operations/operations_playbook.md` — new subsection "Multi-Resolution Query Routing" under "ClickHouse (Gold Layer — Analytics)".** Documents which physical table backs which resolution, the `OPTIMIZE TABLE ... FINAL` runbook for forced MV catch-up, and the diagnostic query `SELECT name, total_rows, ... FROM system.parts WHERE database='aer_gold' AND table LIKE 'metrics_%'` for spot-checking aggregate freshness. Cross-references Phase 66's original deferred-design entry as historical context.
* [x] **`CLAUDE.md` Gold-schema section.** Add `aer_gold.metrics_hourly` (TTL 365d on `bucket`), `aer_gold.metrics_daily` (TTL 1825d on `bucket`), `aer_gold.metrics_monthly` (no TTL) to the table summary, with a one-line note that they are MV-derived from `aer_gold.metrics` and that the BFF routes by resolution.
* [x] **ADR.** No new ADR. The architectural decision was already taken in Phase 66's design (`docs/arc42/09_architecture_decisions.md` is unchanged); this phase is the activation step that procedure prescribed.

### Validation

* [x] `make codegen && git diff --exit-code` green (no breaking schema changes — only the description string in `Resolution.yaml` updates).
* [x] `make test` green including the new routing-matrix and byte-equivalence tests.
* [x] `make test-e2e` smoke: a `?resolution=monthly&start=now-5y&end=now` query against a freshly-crawled corpus returns 60 buckets in <100 ms (compared to the pre-activation behaviour where the same query would scan 10⁸+ rows or silently truncate at the row cap).
* [x] Arc42 §8.8.1 status line is `active as of Phase 122c`; §8.13 has the historical-context block; the playbook subsection exists; CLAUDE.md table includes the three new MVs.
* [x] **Methodological invariant.** A 4-year monthly-resolution time-series query for a tagesschau metric returns 48 buckets covering the full 4-year span, not silently-empty months for the >365d range. This is the user-facing realisation of "the architecture serves multi-year cultural drift."

---

## Phase 122e: Probe 0 First-Crawl Findings & Quality Hardening [P1] - [x] DONE

*The first wipe-and-recrawl of Probe 0 (tagesschau + bundesregierung, 5-year horizon, ~5-min smoke) produced ~380 fetched articles and exposed nine concrete data-quality issues that block Phase 122 closure. Phase 122 implementation, documentation, and testing are all complete (all sub-items checked); the **cutover** and **validation** sub-blocks of Phase 122 remain unchecked because the spot-check invariants pass on tagesschau but fail on bundesregierung. This phase consolidates every finding from the smoke-crawl forensics into a focused Layer-3/Layer-4 hardening effort: the architecture is correct (Bronze captures raw HTML; the WebAdapter, the MV trigger, the medallion topology all work end-to-end), but per-source extraction quality and sitemap-discovery coverage need targeted fixes before the long-running real crawl is methodologically defensible. After this phase ships, Phase 122's validation invariants pass on both sources and Phase 122 closes honestly.*

*Scope discipline.* This phase is **per-source quality work + trafilatura/htmldate provenance correctness** — not a re-architecture. WP-006 §3's prohibition on editorial filtering still stands; new URL-prefix exclusions added here cover **CMS-generated non-article paths** (archive section, confirmation pages, image-blob redirects) — these are technical filters in the same spirit as the existing `/api/`, `/search/`, `/impressum` exclusions. Adding `/sport/` would be editorial gating; adding `/service/archiv-bundesregierung/` is removing CMS noise. Wayback CDX (Phase 122d) and per-article discourse-function classification (Phase 122a) sit on top of clean Phase-122e data and ship after.

### Phase 122 invariant scoreboard (post-first-crawl)

| Invariant | tagesschau | bundesregierung |
| :--- | :--- | :--- |
| `crawl_window_configured` log line emitted | ✅ | ✅ |
| Bronze stores raw HTML verbatim (ADR-028) | ✅ | ✅ |
| Worker WebAdapter dispatches all docs | ✅ | ✅ |
| MV trigger fires on every Gold insert (Phase 122c) | ✅ | ✅ |
| BFF `?resolution=hourly/daily/monthly` round-trips through MV | ✅ | ✅ |
| ≥80 % articles have `published_date` from `{json_ld, open_graph, microdata}` | ✅ 80.6 % | ❌ **0 %** (all fall through to `heuristic_htmldate`) |
| Tier-D `structured_data` non-empty for ≥80 % | ✅ 100 % | ✅ 100 % (top-level keys present, but JSON-LD blocks hold no `NewsArticle`) |
| Median `word_count` per article ≥ 10× the RSS-snippet baseline (~50 words) | ✅ 498 (≈10×) | ⚠️ 147 (≈3×) |
| `SilverCore.timestamp` spans the actual publication-date range | ✅ 290 days, 9 distinct days | ❌ **0 days** (every article collapses to `2026-01-01 00:00:00`) |
| DLQ rate < 5 % | ✅ 2/69 (≈3 %) | ❌ **110/311 (≈35 %)** |

### Findings (forensic detail)

* [x] **F1 — Crawler `ReactorNotRestartable` on multi-source runs (already patched).** Phase-122 implementation called `CrawlerProcess.start()` per source inside a `for` loop; Twisted's reactor is a process-wide singleton and aborted on the second source with `twisted.internet.error.ReactorNotRestartable`. Patched mid-session: `crawlers/web-crawler/internal/fetch/scrapy_spider.py` now exposes `build_crawler_process()` + `queue_source_crawl()`; `crawlers/web-crawler/main.py` queues all sources onto one process and calls `start()` exactly once. **Action**: add a regression test that mocks two `process.crawl()` calls and asserts `start()` is invoked exactly once, so this never regresses.
* [x] **F2 — Tagesschau sitemap discovery is shallow (69 URLs surfaced; expected tens of thousands).** Tagesschau's `sitemap.xml` is an *index* pointing to nested daily/weekly child sitemaps. `ultimate-sitemap-parser` either does not recursively expand all index nodes or fails on a child sitemap and silently drops the rest. With Phase 122b's 5-year cutoff, the actual surface should be ~365 k URLs (200 articles/day × 1825 days); 69 is three orders of magnitude short. **Diagnosis tasks**: capture `usp.tree.sitemap_tree_for_homepage(...)` raw output, walk the index, identify the failing/skipped child sitemaps. **Likely fix**: drop down to the raw `usp` tree iteration and surface every leaf — or replace the discovery library with a direct `xml.etree`-based recursion if `usp` proves unreliable. Phase 122b's filter (`since`) already constrains volume.
* [x] **F3 — Bundesregierung URL filter does not exclude CMS-noise paths; 71 % of DLQ entries hit `/breg-de/service/archiv-bundesregierung/`.** The forensic count of 110 quarantined bundesregierung articles broke down as: 78 archive section (`/service/archiv-bundesregierung/`), 4 cabinet-member pages without article body (`/bundesregierung/bundeskabinett/`), ~28 city/event listing pages (`/schwerpunkte/<city>`), all returning empty `cleaned_text` from both trafilatura and the readability fallback because the pages contain navigation chrome only. **Action**: extend `crawlers/web-crawler/probes/probe0/sources.yaml > bundesregierung > url_filter > exclude_path_prefixes` with `[/breg-de/service/archiv-bundesregierung/, /breg-de/schwerpunkte/]`. The `/bundeskabinett/` paths can stay — those *are* canonical article-class content but happen to extract poorly today (handled by F4). **Methodological framing**: this is technical filtering of CMS-generated noise (per WP-006 §3 the URL filter is for technical exclusions only), not editorial gating; documented in the dossier under "URL filter rationale".
* [x] **F4 — Bundesregierung Tier-B structured-data extraction is 0 %; ALL 201 articles fall through to `heuristic_htmldate`.** The smoking gun: `extraction_methods.published_date == "heuristic_htmldate"` for **every** bundesregierung Silver envelope. The WebAdapter's priority chain (`json_ld_published → open_graph_published → html_meta_published → sitemap_lastmod → http_last_modified → fetch_at_fallback`) walks all the way down to htmldate's HTML-meta-tag heuristic, which then produces `2026-01-01 00:00:00` deterministically (likely from a year-only string in the page footer). Tier-D shows the publisher *does* emit JSON-LD blocks, but they are not `NewsArticle`-typed (likely `WebPage` / `Organization`) and the WebAdapter only reads `datePublished` from `NewsArticle`. **Action**: extend `services/analysis-worker/internal/adapters/web_extract.py`'s structured-data resolver to (a) read the publisher's actual JSON-LD type chain, (b) accept dates from `WebPage` / `Article` / `BlogPosting` / `NewsArticle` in priority order, (c) inspect bundesregierung's specific `<meta property="article:published_time">` / `<time datetime="...">` patterns. **Validation**: ≥80 % of bundesregierung articles populate `published_date` from `extraction_method ∈ {json_ld, open_graph, microdata}` (matching the Phase 122 invariant) AND `timestamp_source ∈ {json_ld_published, open_graph_published, html_meta_published}` and `SilverCore.timestamp` spans ≥30 distinct dates.
* [x] **F5 — htmldate's heuristic returns `2026-01-01` deterministically when no real date is present, which is methodologically dishonest.** The current behaviour leaks a fake-but-plausible timestamp into Gold instead of marking the article as Negative-Space (Brief §7.7). **Action**: extend `web_extract.py` to detect htmldate's "year-floor" pattern (a date that is exactly the start of a year and matches no `<time>`/`<meta>` element in the HTML) and treat that case as `timestamp_source = "fetch_at_fallback"` with `published_date = None`. Articles thus marked are filtered out of timeline / drift analyses by the existing Negative-Space layer. The htmldate library remains in the toolchain — only the consumer-side handling of its outputs changes.
* [x] **F6 — Provenance markers are inconsistent: `extraction_methods.published_date == "heuristic_htmldate"` while `timestamp_source == "html_meta_published"`.** Two parallel provenance dicts on WebMeta disagree about where the date came from. The `extraction_methods` dict is correct (`heuristic_htmldate` is the source); the `timestamp_source` is mis-set to `html_meta_published`. Per Phase 122's spec, when the date comes from htmldate the source should record `heuristic_htmldate` (which is missing from `ALLOWED_TIMESTAMP_SOURCES`!) — or, if htmldate's parse-from-meta-tag path is meaningfully equivalent to `html_meta_published`, the WebAdapter should classify htmldate's outputs more granularly. **Action**: audit `web_extract.py`'s timestamp-resolution code path; ensure exactly one provenance value is written and that it is consistent with the priority-chain step that won. Add a unit test pinning the (timestamp_source, extraction_method) pair per code path.
* [x] **F7 — Tier-B `section` / `categories` / `tags` populate at 0 % across both sources.** Tagesschau's JSON-LD `articleSection` field exists in the page (visible in the raw HTML's `<script type="application/ld+json">` block) but is not surfaced by the WebAdapter into Tier-B. **Action**: in `web_extract.py`, read `articleSection`, `keywords`, `about` from the JSON-LD `NewsArticle` block; populate `section`, `tags[]`, `categories[]` Tier-B fields with `extraction_method = "json_ld"`. Add fixture-based test under `services/analysis-worker/tests/test_web_adapter.py`.
* [x] **F8 — Tagesschau quarantines (2 of 69) are TV-show landing pages with no article body.** URLs `https://www.tagesschau.de/tagesthemen/tt-12584.html` and `https://www.tagesschau.de/tagesschau/ts-78570.html` are program-page indexes (≈456 KB HTML, no `<article>` body). DLQ rate of ≈3 % is within tolerance and accepted. **No action required**; documented for completeness. If the rate climbs in production, candidate path-prefix to evaluate: `/tagesschau/ts-`, `/tagesthemen/tt-`. Defer until empirical signal justifies it.
* [x] **F9 — OTel collector misconfig (cosmetic, pre-existing).** Both ingestion-api and analysis-worker repeatedly log `traces export: Post "http://localhost:4318/v1/traces": connection refused` — services are configured with `localhost:4318` instead of the in-network hostname `otel-collector:4318`. This is **not** a Phase-122 regression and does not affect ingestion correctness; it just pollutes logs and silently disables tracing. **Action (out-of-scope but tracked here)**: audit the OTel exporter env wiring in `compose.yaml`'s service blocks and `pkg/telemetry`'s defaults. Fix as a small chore commit; not gating Phase 122 closure.

### Architectural assertions verified by the smoke crawl (no action — informational)

* [x] **Bronze-as-raw-HTML invariant (ADR-028) holds.** Sample Bronze envelopes for both sources contain raw HTML, fetch envelope (`source`, `source_type=web`, `canonical_url`, `original_url`, `fetch_at`, `http_status`, `sitemap_lastmod`, `sitemap_section`), and zero extracted content.
* [x] **Phase 122c MV trigger fires for every worker insert.** Counts after the smoke crawl: `metrics_RAW=948`, `metrics_HOURLY=327`, `metrics_DAILY=88`, `metrics_MONTHLY=192` — all three MVs populated proportional to bucket cardinality.
* [x] **Phase 122c MV correctness verified by raw-vs-MV byte equivalence.** For `(tagesschau, word_count)` the raw `avg(value) GROUP BY toStartOfDay(timestamp)` produces 9 buckets matching the MV's `avgMerge(value_avg_state)` reading on 7 of 9 buckets exactly. The 2 differences are: (a) one MV-only bucket (`2023-11-03`) holding pre-TTL data the raw layer has already TTL'd out (this is **WP-005 §5.4 working as designed** — MVs hold longer history than raw), (b) a 1-row count-inflation in one bucket from a documented ReplacingMergeTree-pre-merge MV insert (averages unaffected; documented in migration 000019 header).
* [x] **Phase 122c BFF routing verified end-to-end.** `GET /api/v1/metrics?metricName=word_count&sourceIds=tagesschau&resolution=daily` returns the MV-routed response, including the `2026-05-08` bucket with `count=21` (matching the MV, not the `count=20` of raw — proving the BFF's `Resolution.queryShape` is selecting `aer_gold.metrics_daily` and combining state columns via `avgMerge`/`countMerge`).
* [x] **Phase 122b temporal symmetry honoured.** The crawler's startup log line `crawl_window_configured probe=probe0 time_window_days=1825 since=2021-05-10T17:43:49+00:00` confirms the 5-year cutoff is computed correctly and applied uniformly to both sources.
* [x] **Newest-first ordering preserved across both sources.** Tagesschau articles in Silver span 2025-07-23 → 2026-05-09 (290 days, 9 distinct days). The 1.5-min smoke run successfully accumulated articles starting from the most recent and working backward.
* [x] **Sentiment dual-extractor cross-validation produces the WP-002 §3.2 domain-transfer signal.** Tagesschau: BERT de-news mean −0.34, BERT multilingual −0.28, SentiWS −0.05 — the de-news / SentiWS magnitude gap is the designed-for cross-extractor disagreement. Bundesregierung: −0.18 / −0.07 / +0.04 — sign-divergent across extractors, exactly as ADR-023's dual-metric architecture intends.
* [x] **Wikidata entity linking works on web-source data.** Tagesschau: 39.9 entities/article, 381 entity_links/2,757 entities (~14 % linked). Bundesregierung: 13.9 entities/article — fewer entities per article (consistent with shorter / less entity-dense press releases).
* [x] **Language-detection-first ordering (Phase 116) holds on web data.** All 268 processed articles land at `detected_language = "de"` with rank-1 confidence ≥ 0.998. Downstream German extractors (SentiWS, de-news BERT, spaCy `de_core_news_lg`) route correctly via the Capability Manifest.
* [x] **Crawler `crawler_state` dedup correctly skips already-fetched URLs across sessions.** Tagesschau's 69 URLs from the first (crashed) crawl were correctly recognised on the second crawl (`has_seen()` returned true for all 69) and only bundesregierung was actually fetched. ReactorNotRestartable did not corrupt state.

### Second-iteration validation (post F-A2..A6 — full `make reset` + `make crawl-probe0`)

After F-A1..A6 shipped and the second wipe-and-recrawl completed (≈11-minute run, 451 bundesregierung + 71 tagesschau URLs fetched, **DLQ rate dropped from 35 % → 2.9 % for bundesregierung**), forensic inspection of the resulting Silver / Gold confirmed all five implemented fixes work as designed — and the cleaner baseline made five new bug surfaces visible. The verified-working block first; the new findings (F-A10..F-A14) and their action items below.

Verified live in production data:

* [x] **F-A1 RSS pubDate sort puts news at queue head.** First 10 fetched URLs are all `/breg-de/aktuelles/...` published 2026-05-08/09 — exactly what F-A1 promises. In iter 1 these sank to the end of the queue behind ~880 sitemap-discovered service URLs and were never reached.
* [x] **F-A2 URL-filter exclusions hold.** Zero `/service/archiv-bundesregierung/` and zero `/schwerpunkte/` rows in `crawler_state` (was 162 + 132 = 294 in iter 1). DLQ rate dropped from 110/311 = 35 % to 13/451 = 2.9 % for bundesregierung.
* [x] **F-A3 `<time datetime>` extraction works.** 49 of 57 bundesregierung articles with resolved dates now use `extraction_method = html_meta` (the `<time datetime>` path); was 0 % in iter 1.
* [x] **F-A4 year-floor sentinel rejection works.** Across all 189 Silver envelopes (69 tagesschau + 120 bundesregierung): **zero** articles with `published_date == 2026-01-01 00:00:00`. In iter 1, 100 % of bundesregierung's 201 silver envelopes had this fake stamp.
* [x] **F-A5 provenance-pair consistency.** Across all 189 envelopes: **zero** inconsistent `(extraction_method, timestamp_source)` pairs. In iter 1 the microdata branch silently mis-stamped to `open_graph_published`.
* [x] **F-A6 timestamp-resolution branches each write a single, internally-consistent provenance pair.** Verified by parametrised distribution: tagesschau pairs are exactly `{json_ld → json_ld_published, html_meta → html_meta_published}`; bundesregierung pairs are exactly `{html_meta → html_meta_published, heuristic_htmldate → html_meta_published, NONE → fetch_at_fallback}`.
* [x] **Phase 122c MV trigger fires through the BFF API.** `GET /api/v1/metrics?metricName=word_count&sourceIds=bundesregierung&resolution=daily` returns **30 daily buckets** spanning 2025-05-03 → 2026-05-09 — a year of real bundesregierung data via the `metrics_daily` MV path, all values consistent with `aer_gold.metrics` raw-side `avg(value)` aggregation.
* [x] **Phase 122 word-count invariant passes both sources.** Median tagesschau ≈ 510 words (iter 1: 498 — same), median bundesregierung ≈ 414 words (iter 1: 147 — F-A11 noise URLs dragged it down). Both pass the ≥10× RSS-snippet baseline (≈ 50 words).
* [x] **Phase 122 timestamp-spread invariant passes both sources.** tagesschau spans 290 days / 9 distinct days. **Bundesregierung spans 360 days / 20 distinct days** — was 0 days / 1 distinct day in iter 1 (the 2026-01-01 collapse). The Phase 122 cutover invariant on this metric is fully realised.
* [x] **Sentiment dual-extractor cross-validation still produces the WP-002 §3.2 domain-transfer signal.** tagesschau: BERT de-news −0.34 vs SentiWS −0.05 (magnitude gap intact). Bundesregierung: BERT de-news −0.02 vs SentiWS +0.01 (closer to neutral — consistent with shorter-form government communications mostly being technical / informational rather than evaluative).
* [x] **F-A1 ordering proof.** First ten bundesregierung URLs fetched (by `last_fetched`) are all `/aktuelles/` articles whose `published_date` lies within the last ~36 hours of the crawl moment. Newest-first competes correctly across both discovery channels.

### New findings exposed by the second-iteration baseline

* [x] **F-A10 — `bundesregierung` sitemap surfaces French and English mirror URLs (CRITICAL).** The publisher mirrors the same content under three locales: `/breg-de/...` (German), `/le-gouvernement-fédéral/...` (French), `/federal-government/...` (English), plus an alternative English path under `/issues/...`. The sitemap exposes all three; `crawler_state` for the second iteration shows ≥ 102 non-German URLs (≈ 23 % of bundesregierung's 451 fetched). Probe 0 is **German institutional** (per the dossier) — non-German URLs are noise that (a) inflates Bronze 3 ×, (b) DLQs because the language guard refuses non-German articles, (c) wastes politeness budget, (d) pollutes cross-source comparisons. **Action**: extend `bundesregierung.url_filter.exclude_path_prefixes` with `/le-gouvernement-fédéral/`, `/federal-government/`, `/issues/`. Validation: re-crawl → zero non-German Bronze keys.
* [x] **F-A11 — `bundesregierung` URL filter still incomplete after F-A2/A3.** The filter catches `/archiv-bundesregierung/` and `/schwerpunkte/` but not the long tail of remaining CMS-noise paths. 63 of 120 bundesregierung Silver articles (52.5 %) fall through to `fetch_at_fallback` because they are non-articles (CV / Lebenslauf pages, org-info pages, subscription forms, error pages). F-A4 correctly classifies them as Negative-Space, but they still consume worker resources (NER, sentiment, etc. all run on them) and pollute the dossier. URL-prefix breakdown of fall-through articles: 36 `/breg-de/bundesregierung/bundeskabinett/` (cabinet-member CV pages), 14 `/breg-de/bundesregierung/bundespresseamt/` (press-office org-info), 3 `/breg-de/service/newsletter-und-abos/` (subscription forms), 5 misc (`/gebaerdensprache/`, `/suche/`, error-404 placeholder pages). **Action**: extend `bundesregierung.url_filter.exclude_path_prefixes` with `/breg-de/bundesregierung/bundeskabinett/`, `/breg-de/bundesregierung/bundespresseamt/`, `/breg-de/service/newsletter-und-abos/`, `/breg-de/gebaerdensprache/`, `/breg-de/suche/`, `/breg-de/leichte-sprache/`. Validation: ≥ 80 % of bundesregierung Silver articles have a resolved `published_date` (vs. 47.5 % currently).
* [x] **F-A12 — tagesschau's `NewsArticle` JSON-LD does NOT carry `articleSection`** (refines F7 from the first-iteration findings). Sample inspection of three tagesschau Silver envelopes (`Wetter`, `Hantavirus`, `Marktbericht/DAX/Börse`) shows the publisher emits `keywords: ["Wetter"]` and `about: [{"@type":"Thing","name":"Wetter","sameAs":"..."}]` instead. The current `_resolve_section()` only reads `articleSection` → 0 % section population. The `keywords` field IS being read (powering the 81 % `tags` population). **Action**: extend `_resolve_section()` to fall back to `about[0].name` (the most semantically explicit signal) when `articleSection` is absent; record `extraction_method = "json_ld"`. Validation: ≥ 60 % of tagesschau articles populate `section` (matches the original A7 target).
* [x] **F-A13 — `bundesregierung` publisher emits minimal Tier-B metadata even on real `/aktuelles/` news pages** (publisher-side limitation, not a code bug). Across all 20 `/aktuelles/` Silver envelopes inspected: 0 % populate `author`, 0 % `modified_date`, 0 % `articleSection`, 0 % `article_type`, 0 % `tags`. The publisher's CMS only emits `og:type`, `og:title`, `og:url`, `og:image` plus a single `<time datetime>` element — no NewsArticle JSON-LD, no `article:author`, no semantic structured data. **Action**: (a) document the limitation in `docs/probes/probe-0-de-institutional-web/bias_assessment.md` as a **structural metadata-asymmetry between sources** (per WP-003 §3.2 — already an enumerated bias dimension); (b) at the dashboard layer, surface bundesregierung's missing Tier-B fields as Negative-Space (Brief §7.7) instead of rendering empty values; (c) acknowledge this is an unfixable upstream constraint — no extractor change can recover information the publisher does not emit.
* [x] **F-A14 — htmldate's heuristic still fires for 8 bundesregierung articles with non-Jan-1 midnight stamps.** Examples: `published_date = 2026-05-07T00:00:00Z, extraction_method = heuristic_htmldate` for `feierlichkeiten-bmds-2429674` and `rede-ihk-tag-2026-2429676`. Both are real speech / event pages dated 2026-05-07. F-A4 correctly accepts these (they are not `YYYY-01-01`), but the dates are midnight-only, suggesting htmldate found a date string in headline / footer rather than a `<time>` element. The dates may be accurate (speeches often are dated to "the day"), but the imprecision is undocumented. **Action (defensive)**: tighten F-A4's sentinel detector to also flag `time = 00:00:00` *combined with* no `<time datetime>` element in the HTML — but only mark as `extraction_method = "heuristic_htmldate"` (already the case), not as fetch_at_fallback. The current behaviour is correct; this is a follow-up monitoring item — escalate to a real bug only if downstream analyses prove sensitive to midnight imprecision.
* [x] **F-A15 — Discovery-surface asymmetry between sources is the *inverse* of publication-volume asymmetry (NEW, iter-3 framing).** Tagesschau publishes ≈ 150–300 articles/day — a major German news outlet with deep archive — while bundesregierung publishes ≈ 5–15 articles/day (government press releases). But our **discoverable surface** runs the opposite way: bundesregierung exposes 3 `sitemap_index` URLs in `robots.txt` → 894 leaf URLs in the 5-year window; tagesschau's `sitemap.xml` returns HTML 404 and the robots.txt carries no `Sitemap:` directive at all → discovery falls back to RSS only, which is a snapshot of ≈ 70 most-recent URLs at scrape time. The corpus-volume-per-source ratio therefore reflects **crawler access**, not **publication frequency**: tagesschau is dramatically under-represented (we see ≈ 1 % of what they publish in any given week) while bundesregierung is fully represented. This adds a third structural-bias dimension on top of WP-003 §3.2's metadata-richness asymmetry (publisher-side) and WP-003 §3's visibility-mechanism framing (platform-side): a **discovery-surface asymmetry** on the AĒR-collection side that depends on what each publisher chooses to expose to crawlers. **Action — corrected after methodological review**: the asymmetry is **recorded, not engineered around**. The earlier proposal (curate tagesschau's per-section feeds — politik / wirtschaft / kultur / sport / ...) was retracted: it would substitute our editorial judgment for the publisher's, which is researcher selection bias per WP-006 §3 and inverts the Manifesto's "unaltered mirror" principle. Audio / video feeds are out of scope by construction — AĒR analyses cleaned text. The action items: **(a)** add Structural Bias #8 ("Discovery-surface limitation") to `docs/probes/probe-0-de-institutional-web/bias_assessment.md` (see A15); **(b)** flag as input to Phase 122f's metadata-coverage runtime signal — discovery-rate-per-source becomes a measurable signal alongside per-field coverage so dashboard consumers can normalise. **(c) Future-work note**: if expansion ever becomes a Probe-0 requirement, the methodologically clean path is homepage `<link rel="alternate" type="application/rss+xml">` auto-discovery (publisher curation; we ingest the set the publisher chose to advertise, verbatim) — NEVER a hand-curated section list. Auto-discovery is self-maintaining: when the publisher renames a section, the homepage updates and the next crawl picks it up. **Methodological framing (per WP-003)**: this is a "visibility mechanism" bias in the WP-003 §3 sense, but at the AĒR-collection layer rather than the platform layer. Cross-source corpus-volume aggregations are biased by discovery-surface availability, not by publisher activity. The Manifesto's "unaltered mirror" principle says we record the bias, not paper over it.

### Third-iteration validation (post F-A10..A14 — full `make reset` + `make crawl-probe0`)

After F-A10..A14 shipped and the third wipe-and-recrawl completed (≈ 6.6-minute run, both sources naturally exhausted with `finish_reason='finished'`, 314 fetches all HTTP 200), forensic inspection confirmed several iter-2 fixes work as designed AND surfaced four new findings — three of which are corpus-level data-quality blockers. The verified-passing block first; the new findings (F-A16..F-A19) and their action items below.

**Verified passing in iter-3:**

* [x] **F-A4 year-floor sentinel rejection holds.** Across all 255 Silver envelopes: zero docs with `published_date == 2026-01-01 00:00:00`. F-A4 stable across two iterations.
* [x] **A7 / A12 — tagesschau `section` / `categories` populate from `about[*].name`.** Sample of 50 tagesschau Silver envelopes: 42 (84 %) populate `section` from JSON-LD's `about[*]` fallback. Exceeds the 60 % target.
* [x] **A1 single-CrawlerProcess pattern stable.** 314 requests in a single Scrapy run, `finish_reason='finished'`, no `ReactorNotRestartable`. Multi-source flow regression-pinned.
* [x] **Phase 122 invariant on the real-news subset (`/breg-de/aktuelles/`).** All 5 sampled `/breg-de/aktuelles/` URLs have a real `published_date` from `html_meta_published`. The aggregate invariant only fails because the noise URLs (F-A16) drag the average down — the underlying extraction works correctly on the subset that should reach Silver.
* [x] **Phase 122c MV math correctness.** `avgMerge` / `countMerge` algorithm verified: `entity_count`, `sentiment_score_bert_de_news`, `sentiment_score_sentiws` all show 115 raw == 115 hourly (exact). The off-by-one for cross-language metrics is a duplicate-insert symptom (F-A19), not an algorithm bug.

**New findings:**

* [x] **F-A16 — URL filter pattern mismatch (BLOCKER, supersedes F-A10 / F-A11 partially).** Iter-3 shows F-A10's `exclude_path_prefixes` (`/le-gouvernement-fédéral/`, `/federal-government/`, `/issues/`) silently fail to match because bundesregierung's CMS partitions content by **locale-prefix**, not by topic-prefix. Actual URL structure: `/breg-de/...` (German), `/breg-en/...` (English), `/breg-fr/...` (French). The startswith-style match `url.path.startswith("/issues/")` cannot match `/breg-en/issues/...`. Of 313 bundesregierung URLs in iter-3 `crawler_state`: 124 are `/breg-en/`, 110 are `/breg-fr/`, 76 are `/breg-de/` (only 20 of which are `/breg-de/aktuelles/` real news). **Real-news yield: 20 / 313 = 6.4 %.** F-A11's German-side exclusions DO work where they hit (zero `/breg-de/bundesregierung/bundeskabinett/` rows in iter-3), but the English equivalents under `/breg-en/federal-cabinet/` are NOT excluded — same root cause as F-A10. **Action**: see A16 — replace topic-prefix exclusions with locale-level prefixes (`/breg-en/`, `/breg-fr/`). Probe 0 is German-institutional per WP-001 / WP-006 §3 — the German content is the entire target scope; the foreign mirrors are by definition out-of-scope.

* [x] **F-A17 — Document-language ↔ source-language consistency NOT enforced (BLOCKER).** The worker accepts Silver records regardless of `LanguageDetectionExtractor`'s detected language vs the source's allowed languages. Iter-3 Silver: 46 de + 77 en + 62 fr + 1 no rows from bundesregierung — Probe 0 is configured German-only, but 140 non-German rows landed in Silver and Gold. The Language Capability Manifest correctly skips DE-only extractors (`sentiment_score_sentiws`, `entity_count`) for these docs, but multilingual extractors (`sentiment_score_bert_multilingual`, `word_count`, `publication_hour`) still emit metrics on them — polluting Probe 0 aggregates. Per WP-001 (probe scoping) and the Manifesto's "unaltered mirror" principle, a probe-scope violation must DLQ, not silently mix. F-A16 alone closes most of the surface (the foreign-language URLs never get fetched), but the worker should be defence-in-depth against any future filter regression. **Action**: see A17 — add a `LanguageScopeFilter` step in `services/analysis-worker/internal/processing/processor.py`.

* [x] **F-A18 — `published_date=None` silently maps to fetch_at in Gold timestamp (METHODOLOGICAL BLOCKER).** When extraction fails, the Silver MinIO envelope correctly records `core.published_date=None`, `meta.timestamp_source='fetch_at_fallback'`, `meta.extraction_methods.published_date=None`. **But the Silver→Gold loader (`silver_projection.py`) writes `core.timestamp` to `aer_silver.documents.timestamp` — and `core.timestamp` was set to the `fetch_at` write-time during construction**, so a Gold query that reads `timestamp` cannot distinguish "real published_date" from "we have no idea when this was published." Iter-3 distribution of write-time-masquerading-as-published_date: bundesregierung-de 17/46 (37 %), bundesregierung-en 74/77 (96 %), bundesregierung-fr 62/62 (100 %), tagesschau-de 3/69 (4.3 %). Per the user's Phase 122 maxim — *"if the base dataset of this project is false, the whole project missed its purpose"* — this is a base-dataset falsity: Gold rows that look like 2026-05-09 publications are in 99 % of French cases write-time stamps on static archive pages from years ago. After F-A16 / F-A17 ship, this rate plummets (the foreign-language docs disappear), but ~ 5 % of `/breg-de/aktuelles/` docs still hit fallback and need this fix to be honest. **Action**: see A18 — add `timestamp_source` column to `aer_silver.documents` and `aer_gold.metrics`; BFF temporal queries filter on it; Phase 122f's metadata-coverage endpoint surfaces the fallback count per source as Negative-Space (Brief §7.7).

* [x] **F-A20 — Tagesschau exposes a deep date-indexed HTML archive that the iter-3 discovery pipeline did not use (CORPUS-COVERAGE BLOCKER, supersedes the F-A15 retraction).** During the iter-3 forensic write-up the discoverable surface for tagesschau was characterised as "RSS-only — ≈ 70 articles in a sliding window" because `tagesschau.de/sitemap.xml` returns HTTP 404 and `robots.txt` carries no `Sitemap:` directive. **This was an under-investigation, not a publisher limitation.** The publisher *does* expose two additional discovery surfaces:
  * `https://www.tagesschau.de/infoservices/rssfeeds` — the publisher's RSS catalogue page, listing the 2 feeds they advertise (`index~rss2.xml` and `index~atom.xml`, the same content in two formats — *no* per-section feeds, confirming the F-A15 retraction was correct for *that* surface);
  * `https://www.tagesschau.de/archiv?datum=YYYY-MM-DD` — a **date-indexed HTML archive** the publisher built and parameterises by date. Sample probes returned 138 / 139 / 140 / 140 article URLs for 2025-01-15 / 2024-06-01 / 2023-03-22 / 2022-09-10 respectively → **the archive exposes ≥ 4 years of deep history at ≈ 140 articles/day**. Multiplied across the probe's 5-year `time_window_days = 1825`, this surface yields ≈ 250,000 article URLs — more than 3,500× the RSS-only surface. The tagesschau side of the discovery-surface asymmetry largely closes once we use it. Methodologically clean per WP-006 §3 — the publisher built and parameterises the page; we walk every day in the window and ingest every article-shaped link verbatim, no editorial filter on sections / topics / article types. Distinct from the rejected A15 multi-feed proposal (which would have required us to *pick* sections from a feed catalogue the publisher does not expose). **Action**: see A20 — add a `archive_index` discovery channel, mirrored on the existing `sitemap_urls` and `rss_hint_url` channels (same `since` cutoff for temporal symmetry, same dedup semantics, same `DiscoveredUrl.sitemap_lastmod` newest-first sort).

* [x] **F-A19 — `metrics_hourly` over-counts by exactly 1 sample on cross-language metrics (CLICKHOUSE MV INVARIANT VIOLATION).** Iter-3 raw vs hourly counts: `word_count` 255 → 256 (avg drift +4.59 — material on word_count), `language_confidence` 255 → 256, `publication_hour` 255 → 256, `publication_weekday` 255 → 256, `sentiment_score_bert_multilingual` 254 → 255. German-only metrics are exact at 115 / 115. **Root cause**: at-least-once delivery (NATS retry, processor retry) caused at least one duplicate `(article_id, metric_name)` insert into `aer_gold.metrics`. The raw table is `ReplacingMergeTree(ingestion_version) ORDER BY (article_id, metric_name)` and will dedupe at merge time, but `metrics_hourly` is `AggregatingMergeTree` materialised on-insert — it consumed both inserts and aggregated both. This is a known ClickHouse footgun for `AggregatingMergeTree` MVs over `ReplacingMergeTree` raw. The German-only metrics are exact because the German subset had no retries in this run; the `+1` is a single retry on a non-German doc. **Symptom hiding via "truncate MV target tables in `make reset`" does not fix the production runtime case** — the runtime over-count survives every successful crawl, not just resets. **Action**: see A19 — enable ClickHouse block-level deduplication via `non_replicated_deduplication_window` + `insert_deduplication_token`. The dedup is checked AT INSERT TIME, before the MV fires; identical-key duplicates are silently no-oped. This is the canonical ClickHouse-native idempotency pattern for AggregatingMergeTree-over-raw chains.

### Action checklist (extended with second-iteration items)

* [x] **A1 (Crawler regression test for F1).** Add `crawlers/web-crawler/tests/test_main.py::test_cli_calls_start_exactly_once_for_multi_source` mocking `build_crawler_process` to return a `MagicMock`, asserting `process.crawl` is called once per source and `process.start` is called exactly once after both have been queued. Pins F1 against future regression.
* [x] **A2 (Sitemap depth — F2).** Investigate `usp.tree.sitemap_tree_for_homepage` recursion behaviour against tagesschau's nested sitemap index. Either fix the call (pass deeper-recursion flags, handle child-sitemap parse failures gracefully and continue) or replace with a direct recursion. Validation: a fresh discovery against tagesschau surfaces ≥10 000 URLs within the 5-year window.
* [x] **A3 (URL-prefix CMS-noise filter for bundesregierung — F3).** Extend `crawlers/web-crawler/probes/probe0/sources.yaml` `bundesregierung.url_filter.exclude_path_prefixes` with `/breg-de/service/archiv-bundesregierung/` and `/breg-de/schwerpunkte/`. Document the rationale inline. Validation: re-run discovery; the dropped count matches the 78 + ≈28 archive/listing pages identified in F3.
* [x] **A4 (Tier-B structured-data extraction parity — F4).** Update `services/analysis-worker/internal/adapters/web_extract.py` to (a) walk the JSON-LD `@type` graph and accept dates / metadata from `NewsArticle | Article | BlogPosting | WebPage | ReportageNewsArticle` in priority order, (b) read `<meta property="article:published_time">` and `<time datetime="...">` for the html_meta_published path explicitly. Add fixture-driven tests in `services/analysis-worker/tests/test_web_adapter.py` covering bundesregierung's actual HTML structure (capture from a live Bronze key). Validation: ≥80 % of bundesregierung articles in Silver populate `published_date` via `{json_ld, open_graph, microdata}` AND `timestamp_source ∈ {json_ld_published, open_graph_published, html_meta_published}`; spans ≥30 distinct dates.
* [x] **A5 (htmldate year-floor sentinel — F5).** Detect htmldate's "year-floor only" outputs (`YYYY-01-01 00:00:00` with no corroborating `<time>`/`<meta>` element in the HTML) and rewrite to `published_date = None` + `timestamp_source = "fetch_at_fallback"` instead of writing the fake-precise stamp. Add a unit test pinning this. Validation: zero articles with `published_date == 2026-01-01 00:00:00` and `extraction_method = heuristic_htmldate`.
* [x] **A6 (Provenance-marker consistency — F6).** Audit `web_extract.py`'s timestamp-resolution path; ensure each branch writes a single (`extraction_method`, `timestamp_source`) pair and that the pair is internally consistent (e.g., `json_ld_published` ↔ `extraction_method = json_ld`). Extend `ALLOWED_TIMESTAMP_SOURCES` if a new finer-grained value is needed for htmldate's specific paths. Add a parametrised test in `tests/test_web_adapter.py`.
* [x] **A7 (Tier-B `section` / `categories` / `tags` from JSON-LD — F7 / F-A12).** Read `articleSection` → `section`, `keywords` → `tags[]`, `about` → `categories[]` from the JSON-LD `NewsArticle` block; record `extraction_method = "json_ld"`. Validation: ≥60 % of tagesschau articles populate `section`. **Refinement (F-A12)**: tagesschau's NewsArticle has `keywords` + `about` but NO `articleSection` field. Extend `_resolve_section()` to fall back to `about[0].name` (most semantically explicit) when `articleSection` is absent.
* [x] **A8 (OTel exporter env wiring — F9).** Audit `compose.yaml` and `pkg/telemetry` for `localhost:4318` and replace with `otel-collector:4318` (or env-driven endpoint). Verify in tempo that ingestion-api and analysis-worker traces start arriving. **Out-of-scope-but-tracked**: small, can ship anytime.
* [x] **A9 (Resume-friendly re-run pattern documented).** Operations playbook addition: `make crawl-probe0` is **resumable** — `crawler_state` dedup means a Ctrl+C'd run continues cleanly on the next invocation, no `make reset` required between iterations. Document in the "Web Crawl Operations" section so a new contributor doesn't reflexively reset.
* [x] **A10 (Multilingual filter for bundesregierung — F-A10, CRITICAL).** Extend `bundesregierung.url_filter.exclude_path_prefixes` with `/le-gouvernement-fédéral/`, `/federal-government/`, `/issues/`. The publisher mirrors content in DE / FR / EN; Probe 0 is German-institutional, so non-German mirrors are pure noise. Validation: zero non-German Bronze keys after re-crawl. Methodological note for the dossier: this is **technical filtering on language locale**, in the same spirit as `exclude_extensions`, not editorial gating per WP-006 §3 — Probe 0's defined scope is German content.
* [x] **A11 (Aggressive CMS-noise filter for bundesregierung — F-A11).** Extend `bundesregierung.url_filter.exclude_path_prefixes` with `/breg-de/bundesregierung/bundeskabinett/`, `/breg-de/bundesregierung/bundespresseamt/`, `/breg-de/service/newsletter-und-abos/`, `/breg-de/gebaerdensprache/`, `/breg-de/suche/`, `/breg-de/leichte-sprache/`. These prefixes contain CV pages, org-info pages, subscription forms, sign-language info, search-result placeholders, and accessible-language mirrors — none of which are articles. Currently 52.5 % of bundesregierung Silver articles fall through to `fetch_at_fallback` because of these paths. Validation: ≥ 80 % of bundesregierung Silver articles have a resolved `published_date` (vs. 47.5 % at iter 2 baseline).
* [x] **A12 (Tier-B section fallback to `about[0].name` — F-A12).** Already tracked under A7's refinement above; surfaced separately so the action item is unambiguous in `web_extract.py`.
* [x] **A13 (Document publisher metadata-asymmetry — F-A13).** Update `docs/probes/probe-0-de-institutional-web/bias_assessment.md` to add a "metadata-richness asymmetry" subsection per WP-003 §3.2. Specific data point: bundesregierung publishes only `og:type`, `og:title`, `og:url`, `og:image`, and a `<time datetime>` element on its `/aktuelles/` news pages — no NewsArticle JSON-LD, no author, no modified_date, no articleSection. Tagesschau publishes a full NewsArticle JSON-LD with author, dates, keywords, about, and image references. Cross-source comparisons of Tier-B-derived metrics (author concentration, section mix, modification rates) will systematically under-represent bundesregierung. The fix is to surface the asymmetry on the dashboard via Negative-Space rendering (Brief §7.7) — the publisher's choice is unfixable upstream.
* [x] **A14 (Defensive midnight-stamp monitoring — F-A14, low priority).** Add a metric / log line that counts htmldate-resolved dates with `time == 00:00:00` and no `<time datetime>` element in the source HTML. If the count grows large or downstream analyses prove sensitive to midnight imprecision, escalate to a real bug. For now: monitoring only, no behavioural change. **Note**: A14 was implemented after iter-3 build, so iter-3 forensics did not exercise it. First surfaces in iter-4.
* [x] **A15 (Document the discovery-surface asymmetry as a Probe-0 structural bias — F-A15).** **Drop the multi-RSS-feed proposal.** Curating tagesschau's per-section feeds (`politik` / `wirtschaft` / `kultur` / `sport` / ...) would substitute our editorial judgment for the publisher's — that is researcher selection bias per WP-006 §3, and the "likely candidates" framing was trial-and-error guessing about what the publisher exposes. Both contradict the Manifesto's "unaltered mirror" principle. Audio / video feeds are out of scope by construction — AĒR analyses `cleaned_text`; non-textual feeds either duplicate their accompanying article feeds (redundant) or DLQ on empty `cleaned_text` (noise admission). The methodologically honest action is to **document the asymmetry, not engineer around it**: add Structural Bias #8 ("Discovery-surface limitation") to `docs/probes/probe-0-de-institutional-web/bias_assessment.md` recording that tagesschau's discoverable surface is the publisher's RSS top-stories window (≈ 70 articles, refreshed continuously), while bundesregierung's discoverable surface is the full filtered sitemap; cross-source corpus-volume comparisons reflect *crawler access*, not *publication frequency*; downstream consumers (BFF temporal aggregations, dashboard volume panels) MUST normalise by per-source discoverable-surface size when cross-source counts are compared. Phase 122f's metadata-coverage endpoint will expose this as a queryable signal. **Future-work note (NOT part of this action)**: if expansion ever becomes a Probe-0 requirement, the methodologically clean path is homepage `<link rel="alternate" type="application/rss+xml">` auto-discovery (publisher curation: the publisher decides what to expose; we ingest the set verbatim, no editorial filter), NEVER a hand-curated section list. Auto-discovery is self-maintaining (a renamed section updates the homepage and the next crawl picks it up). Even auto-discovery would still be bounded to textual feeds. **No code change in iter-4; documentation only.**
* [x] **A16 (URL filter rewrite — locale-level prefixes — F-A16, BLOCKER).** Replace `/le-gouvernement-fédéral/`, `/federal-government/`, `/issues/` in `bundesregierung.url_filter.exclude_path_prefixes` with `/breg-en/` and `/breg-fr/` (locale-level). Keep the German subpath excludes from A11. Methodological note: this is **technical filtering on language locale**, in the same spirit as `exclude_extensions`, NOT editorial gating per WP-006 §3 — Probe 0's defined scope is German content. Validation: zero `/breg-en/` or `/breg-fr/` URLs in `crawler_state` after re-crawl; `aer_silver.documents` shows zero `language IN ('en','fr')` rows for bundesregierung.
* [x] **A17 (Language-scope quarantine filter — F-A17, BLOCKER).** Add a per-source `allowed_languages: [de]` field to `crawlers/web-crawler/probes/probe0/sources.yaml` (default `[]` = no filter). Have the crawler propagate it to the Silver envelope's `bias_context` (or a parallel config the worker reads by source name — pick the simpler path). In `services/analysis-worker/internal/processing/processor.py`, after `LanguageDetectionExtractor` patches `core.language`, check against the source's `allowed_languages`. If `allowed_languages` is non-empty AND `core.language ∉ allowed_languages`, raise `ExtractionFailedError` with reason `language_scope_violation` so the doc is quarantined (per ADR — adapter / scope failures DLQ, extractor failures degrade gracefully). Add a unit test in `services/analysis-worker/tests/test_processor.py`. Validation: on the iter-3 inputs (without F-A16 fixed), this filter alone would quarantine the 140 non-German rows.
* [x] **A18 (`timestamp_source` column in Gold + provenance-aware Gold writes — F-A18, METHODOLOGICAL BLOCKER).** Add migration `infra/clickhouse/migrations/000020_add_timestamp_source.sql` that adds `timestamp_source String DEFAULT ''` to `aer_silver.documents` AND `aer_gold.metrics`. Update `services/analysis-worker/internal/silver_projection.py::upload_silver_projection()` to accept `timestamp_source` and include it in the insert. Update the Gold metrics writer (`internal/corpus.py` and the per-extractor metric emit paths) to populate `timestamp_source` from `WebMeta.timestamp_source` (or `''` for non-web sources). The MV definitions don't change (they group by `toStartOfHour(timestamp)`), but downstream consumers gain the ability to filter. Update BFF query helpers to add `WHERE timestamp_source != 'fetch_at_fallback'` to all temporal aggregation queries. Phase 122f then reads this column as the substrate for its Negative-Space rendering. Validation: post-migration query `SELECT timestamp_source, count() FROM aer_silver.documents GROUP BY timestamp_source` returns the per-source distribution; BFF temporal endpoints exclude fallback rows by default.
* [x] **A19 (ClickHouse insert-time deduplication — F-A19, MV INVARIANT FIX, BEST-PRACTICE).** Two-part change. **(1) Migration** `infra/clickhouse/migrations/000021_enable_insert_deduplication.sql`: `ALTER TABLE aer_gold.metrics MODIFY SETTING non_replicated_deduplication_window = 1000` and same for `aer_silver.documents`, `aer_gold.entities`, `aer_gold.entity_links`, `aer_gold.language_detections`, `aer_gold.entity_cooccurrences`, `aer_gold.topic_assignments`. **(2) Worker change** in `services/analysis-worker/internal/storage/clickhouse_client.py::ClickHousePool.insert()`: add a `deduplication_token: str | None = None` argument that, when provided, is forwarded via `settings={"insert_deduplication_token": token}`. Update every `pool.insert()` callsite (`silver_projection.py`, the gold-metrics writer in `corpus.py`, the entities / language_detections / entity_links writers) to compute a stable token per `(article_id, table, ingestion_version)`. Add a regression test in `services/analysis-worker/tests/test_dedup.py` that double-inserts the same `(article_id, metric_name, ingestion_version)` and asserts `metrics_hourly` count stays at the expected value. **No `make reset` change needed** — the canonical fix is at the insert layer; reset would only mask the symptom while leaving production runtime broken. Validation: iter-4 → `aer_gold.metrics_hourly` `countMerge(sample_count_state)` exactly equals `aer_gold.metrics` `count()` for every `metric_name` (currently 1-off for cross-language metrics).
* [x] **A20 (Date-indexed HTML archive discovery — F-A20, CAPABILITY SHIPPED, NOT CONFIGURED FOR PROBE 0).** Implementation complete. Code: `internal/discovery/archive_index.py` walks a publisher-curated date-indexed page (`url_template` with `{date}` placeholder), supports `daily` and `monthly` granularity (smoke test of `tagesschau.de/archiv?datum=` confirmed monthly granularity — page title `"Archiv - Inhalte vom Juni 2024"`; daily walking would issue ≈ 30× redundant fetches per month AND distort the newest-first sort), regex-extracts article-shaped URLs, yields `(url, archive_date_as_lastmod)` pairs. Mirrors `sitemap.discover()` and `rss_hint.discover()` — same `since` temporal-symmetry rule, same dedup semantics. Tests: 13 unit tests in `tests/test_discovery_archive_index.py` (article-pattern matching, full-window walk, lastmod propagation, non-200 / fetch-exception graceful skip, dedup across days, missing-config / invalid-regex graceful no-op, custom user-agent, monthly granularity, year-boundary stepping, unknown-granularity rejection). Wire-in: `_discover_for_source` in `main.py` calls `discover_archive_index` after sitemap and RSS; sitemap / RSS entries win on URL collision. Doc: `docs/extending/add-a-source.md` gained a "Discovery surfaces" section covering A (sitemap) / B (RSS) / C (archive_index), methodology rule (publisher curation only — never researcher curation), cost ladder.

  **Configuration decision — NOT in iter-4 for tagesschau.** Per explicit direction *("each run = today's articles; corpus accumulates day-by-day through cron; no historical backfill; nothing else")*, `time_window_days` was initially set to 1 at the probe level. Iter-5 raises it to **7** (rolling 7-day watermark) — the data-science best-practice "lateness allowance" pattern from Beam / Flink / Kafka Streams. Rationale: cron resilience (a single missed firing under 1-day loses that day permanently; under 7-day the next firing recovers it), editorial-hold tolerance (publishers backdate articles up to ~3 days), time-series gap sensitivity (downstream temporal aggregations distort on missing days), and replayability. Postgres `crawler_state` dedup makes the overlap free. **This is NOT backfill** — backfill = "ingest from before we started"; a rolling watermark only ever reaches into the last week and stabilises after day 7. tagesschau uses RSS-only discovery; the `archive_index` block remains removed from `sources.yaml`. The A20 archive-walker capability stays in code (zero-cost when un-configured) for future explicit-backfill use cases. Validation: iter-5 first-run tagesschau Silver count ≈ 280 (≈ 7 days × 40 RSS items/day); subsequent daily runs add ≈ 40/day; after 1 month ≈ 1500 articles.

### Fourth-iteration validation (post F-A16..F-A20 — full `make reset` + `make crawl-probe0`)

After F-A16..F-A20 shipped and the fourth wipe-and-recrawl completed (≈ 86-second crawl, finish_reason=finished, 101 articles ingested: 41 tagesschau + 60 bundesregierung; 77 processed, 24 quarantined), forensic inspection confirmed all four iter-3 critical fixes hold AND surfaced four new findings — three of which trace to a single root cause (F-A21).

**Verified passing in iter-4:**

* [x] **F-A16 — URL filter rewrite (locale prefixes).** Iter-3 had 234 leaks of `/breg-en/` + `/breg-fr/` URLs in `crawler_state`; iter-4 has **0**. Locale-prefix exclusion works.
* [x] **F-A17 — Language-scope quarantine (defense-in-depth).** Iter-3 leaked 140 non-DE rows into Silver; iter-4 has **0** non-DE rows. The `LanguageScopeFilter` step in the worker caught **2 articles** that slipped past F-A16 via `/breg-de/` URLs containing English / French content (`detected_language=en`, `detected_language=fr`) and quarantined them with reason `language_scope_violation`. Confirms the design intent: the URL filter is best-effort, the worker is authoritative.
* [x] **F-A18 — `timestamp_source` provenance column.** Both `aer_silver.documents.timestamp_source` and `aer_gold.metrics.timestamp_source` populated; per-source distribution queryable. Tagesschau: 31 `json_ld_published`, 9 `html_meta_published`, 0 fallback. Bundesregierung: 16 `fetch_at_fallback`, 11 `html_meta_published`, 0 `json_ld_published` — matches the publisher's metadata posture (Structural Bias #5).
* [x] **F-A19 — ClickHouse insert-time deduplication.** Iter-3 was 1-off on every cross-language metric (`word_count` 255 → 256, etc.); iter-4 shows **`countMerge(sample_count_state) == count()` exactly for all 8 metric_names** (entity_count, language_confidence, publication_hour, publication_weekday, sentiment_score_bert_de_news, sentiment_score_bert_multilingual, sentiment_score_sentiws, word_count — all 67/67 or 66/66). MV invariant restored.
* [x] **Tier A / B / C / D metadata extraction works as designed.** Sample envelope inspection confirms: tagesschau (rich publisher) populates 14 of 23 provenance fields from `json_ld` including `published_date`, `modified_date`, `author`, `description`, `section`, `categories`, `tags`, `image_url`, `article_type`, `paywall_status`, `revision_date`. Bundesregierung (sparse publisher) populates 5 of 23 — exactly the WP-003 §3.2 / Bias #5 pattern (`og:type`, `og:title`, `og:url`, `og:image` plus `<time datetime>`, nothing else). Extractor correctly records absence as `None` rather than imputing values. Phase 122f's metadata-coverage endpoint is the consumer-side rendering of this provenance signal.

**New findings (F-A21..F-A25):**

* [x] **F-A21 — Bundesregierung sitemap entries 100% lack `<lastmod>`; current "fall through" rule lets all of them bypass `time_window_days` (CRITICAL ROOT CAUSE).** Direct fetch of one of bundesregierung's three leaf sitemaps confirmed: **638 `<url>` blocks, ZERO with `<lastmod>`**. The current implementation in `internal/discovery/sitemap.py` deliberately falls through entries with `lastmod IS NONE` (per the comment *"we do not silently drop coverage on publishers with sparse sitemaps"* — that rule was correct for the Phase-122b backfill use case, where missing-lastmod entries were Negative-Space-classified-by-the-worker and methodologically defensible to ingest). In **continuous-monitoring mode with `time_window_days = 1`**, this rule defeats its own purpose: 100 % of bundesregierung's sitemap surface bypasses the temporal filter. Iter-4 ingested articles dating back to 2011-09-15, 2020-06-05, and seven articles from 2022-2024 because of this — all of which were then silently TTL-evicted from `aer_gold.metrics` and `aer_silver.documents` by the schema's `TTL timestamp + toIntervalDay(365)` (which is **F-A23**, downstream symptom). The Silver MinIO envelopes for these old articles ARE still preserved (37 envelopes total; 10 of which got their CH rows TTL-evicted). **Action**: see A21 — strict-lastmod filtering in continuous-monitoring mode (drop entries with no lastmod when the discovery cutoff is small).

* [x] **F-A22 — Two URL-filter gaps surface that F-A21 will close mostly but not entirely (downstream of F-A21).** Two distinct issues:
  * **(a) Trailing-slash mismatch in `exclude_path_prefixes`.** The exclude pattern `/breg-de/service/archiv-bundesregierung/` (with trailing slash) does NOT match the actual landing-page URL `/breg-de/service/archiv-bundesregierung` (no trailing slash). One of iter-4's 10 missing-from-CH articles was at exactly this URL (silver_key `10f6b9d62ba72790.json`).
  * **(b) Long tail of unfiltered single-segment CMS noise paths.** URLs like `/breg-de/richtige-wahl-aus-330-berufen-975874`, `/breg-de/fakten-zur-regierungspolitik-975748`, `/breg-de/bundesregierung/link-kopieren-2205244`, `/breg-de/neueste-nachrichten-der-bundesregierung-ueber-rss-feedreader-975870` are CMS placeholder / info pages, not articles. They aren't covered by any current `exclude_path_prefixes` entry.
  
  After F-A21 lands, all 638 lastmod-less entries in this sitemap leaf get dropped — many of these noise URLs disappear naturally because they have no lastmod. **Action**: see A22 — fix the trailing-slash matching at the URL-filter level (a one-line spider change), and add the surfaced noise patterns to `exclude_path_prefixes` for defense-in-depth in case any of them ever do gain a lastmod.

* [x] **F-A23 — TTL eviction of articles older than 365 days IS NOT A BUG (closes when F-A21 lands).** Initial forensics showed 37 "Silver projection updated" worker log lines for bundesregierung but only 27 rows in `aer_silver.documents` — apparently 10 silent insert-rejections. Manual insert with a unique `insert_deduplication_token` confirmed the schema and dedup mechanism are sound. Investigation of the 10 missing article timestamps revealed:
  
  ```
  2011-09-15  ← 15 years old
  2020-06-05
  2022-03-11, 2022-06-20, 2022-06-24, 2022-08-22, 2022-09-14
  2023-08-02
  2024-08-01
  2025-05-03  ← 1 week past cutoff
  ```
  
  All ten timestamps are older than `now - 365 days = 2025-05-09`. The schema is `TTL timestamp + toIntervalDay(365)` on both `aer_silver.documents` and `aer_gold.metrics`, so these rows were inserted then evicted on the next merge cycle (per the part_log: 76 `NewPart` events with 77 total rows; the post-merge state retains 67 of those). The Silver MinIO envelopes are still present — only the ClickHouse projection rows were purged. **This is methodologically correct** (the schema explicitly mandates 365-day retention) and **not a separate bug**. Closing F-A21 closes F-A23 as a side-effect: with proper temporal bounding at discovery, no article older than the TTL ever gets crawled.

* [x] **F-A24 — A19's deduplication-token kwarg breaks the metric-baselines background loop (REAL, INDEPENDENT BUG INTRODUCED BY A19).** Worker logs show `baseline.loop.failed error="Client.insert() got an unexpected keyword argument 'deduplication_token'" error_type=TypeError extractor=metric_baseline` on every baseline-loop tick. Root cause: in `services/analysis-worker/internal/extractors/metric_baseline.py:181`, the variable `ch_client` is the raw `clickhouse_connect.driver.Client` (passed in via `corpus.py:_run_baseline_sweep`), NOT my `ClickHousePool` wrapper which I extended in A19. The raw `Client.insert()` does not accept the `deduplication_token` kwarg. Per-document Silver / Gold path is unaffected (uses the wrapper consistently). **Impact**: the corpus-level baseline-extractor loop (Phase 115) is broken — every tick throws on insert; no metric_baselines rows are ever written. **Action**: see A24 — switch to `settings={"insert_deduplication_token": token}` which IS supported by the raw `clickhouse_connect.Client.insert()` API.

* [x] **F-A25 — JSON-LD `image` array stored verbatim as a stringified Python list-of-dict in `image_url` (MINOR Tier-B quality).** When tagesschau emits JSON-LD with `image` as an `ImageObject` array (the standard NewsArticle pattern when the article has multiple images), the WebAdapter's `_resolve_image_url` writes the array's repr to `WebMeta.image_url`, producing values like `"[{'@type': 'ImageObject', 'url': 'https://images.tagesschau.de/...'}]"` instead of the URL string `"https://images.tagesschau.de/..."`. The schema is `image_url: str = Field(default="")`, so this is a contract violation — any dashboard image-rendering path would treat the value as a URL and produce broken images. Affects ~ 96 % of tagesschau articles (the ones with JSON-LD image arrays). **Action**: see A25 — extract `.url` from the first ImageObject when JSON-LD's `image` field is an array; fall back to verbatim-string when the field is already a single URL string.

### Action checklist (extended with fourth-iteration items)

* [x] **A21 (Strict-lastmod filtering for sitemap discovery — F-A21, ROOT-CAUSE FIX).** Add a `strict_lastmod` parameter to `internal/discovery/sitemap.discover()` (default `True`) — when `since` is provided AND a sitemap entry has no `<lastmod>`, drop it instead of falling through. Plumb through `_discover_for_source` in `main.py`. The Phase-122b "fall through to preserve coverage on sparse sitemaps" rule is preserved as the explicit `strict_lastmod=False` path (for backfill mode). Add a `probe.sitemap_strict_lastmod: true|false` config option in `sources.yaml` so operators can toggle (default `true` for continuous-monitoring). Update unit tests in `tests/test_discovery.py`. Validation: iter-5 → bundesregierung Silver oldest = today (was 2025-10-09); zero articles older than `now - time_window_days` reach Silver.

* [x] **A22 (URL-filter robustness — F-A22, DEFENSE-IN-DEPTH).** Two sub-changes in `internal/fetch/scrapy_spider.py` (or wherever `exclude_path_prefixes` matching lives): **(a)** match prefixes regardless of trailing slash on either side — `path.rstrip('/')` against `prefix.rstrip('/')` so `/breg-de/service/archiv-bundesregierung` correctly matches the configured `/breg-de/service/archiv-bundesregierung/` exclude. **(b)** add the iter-4-surfaced noise patterns to `crawlers/web-crawler/probes/probe0/sources.yaml` `bundesregierung.url_filter.exclude_path_prefixes`: `/breg-de/richtige-wahl-aus-330-berufen-`, `/breg-de/fakten-zur-regierungspolitik-`, `/breg-de/bundesregierung/link-kopieren-`, `/breg-de/neueste-nachrichten-der-bundesregierung-ueber-rss-feedreader-`, `/breg-de/barrierefreiheit/`. Validation: zero of these noise URLs in `crawler_state` after iter-5.

* [x] **A24 (`metric_baseline.py` dedup-token via settings dict — F-A24, FIX).** In `services/analysis-worker/internal/extractors/metric_baseline.py:181`, replace the `deduplication_token=...` kwarg call with `settings={"insert_deduplication_token": token}` which the raw `clickhouse_connect.driver.Client.insert()` API accepts. The same fix may be needed in any other path where `ch_client` is a raw client rather than the `ClickHousePool` wrapper — audit every `pool.insert(...)` and `ch_client.insert(...)` callsite that took a `deduplication_token` from A19. Validation: worker log shows zero `baseline.loop.failed` lines after iter-5.

* [x] **A25 (Robust `image_url` extraction — F-A25, MINOR).** In `services/analysis-worker/internal/adapters/web_extract.py`, the `_resolve_image_url` (or equivalent) helper that reads JSON-LD `image` should detect the array case and extract the first item's `.url` (or `@id`) string. When the field is a plain string, keep current behavior. When the field is a single `ImageObject` dict, extract `.url`. Document the precedence in the function docstring. Add a parametrised test in `tests/test_web_adapter.py` covering: bare string URL; single-object `{"@type":"ImageObject","url":"..."}`; array of N ImageObjects. Validation: tagesschau Silver `image_url` field always holds a string starting with `http://` or `https://`.

### Fifth-iteration validation (post F-A21..F-A25 — full `make reset` + `make crawl-probe0`)

After F-A21..F-A25 shipped and iter-5 ran (88 URLs discovered, 87 processed, 1 quarantined), forensics confirmed every iter-4 fix landed AND surfaced one residual edge case (F-A26).

**Verified passing in iter-5:**

* [x] **F-A21 strict-lastmod sitemap.** bundesregierung Silver oldest = 2026-05-07 (within 7-day window); was 2025-10-09 in iter-4. All 638-of-638 lastmod-less sitemap entries dropped at discovery (sitemap_count=3 consulted, all rejected). Root cause closed.
* [x] **F-A22 URL-filter robustness.** All 20 bundesregierung crawler_state entries are `/breg-de/aktuelles/` real news; **0 noise URLs** of any pattern. Trailing-slash-tolerant + new noise-pattern excludes both verified.
* [x] **F-A22 quarantine rate** dropped from **23/60 (38%)** in iter-4 to **0/20 (0%)** in iter-5 for bundesregierung. Every URL that reached the worker was a real article with extractable content.
* [x] **F-A24 metric_baseline kwarg.** 0 `baseline.loop.failed` log lines (was every tick in iter-4); 1 successful `baseline.loop.tick_complete`. `settings={"insert_deduplication_token": …}` works on the raw `clickhouse_connect.Client.insert()` API.
* [x] **F-A25 image_url shape.** 64/67 tagesschau Silver envelopes have OK URL string; 3 empty (publisher absence); **0 BAD list-of-dict repr** (was the iter-4 bug shape). All 20 bundesregierung envelopes have OK URL string. Sample URLs verified to be real `https://images.tagesschau.de/...`.
* [x] **All iter-3 fixes still hold.** 0 `/breg-en/` + 0 `/breg-fr/` (F-A16); 0 non-DE Silver rows (F-A17); 0 `fetch_at_fallback` in either source (F-A18); German-only metric_names show exact MV vs raw equality (F-A19 base case).

**New finding (single residual):**

* [x] **F-A27 — NATS-redelivery race condition produces silent raw-vs-MV drift (CRITICAL — A19 dedup didn't cover the MV path on non-Replicated tables).** Iter-6 forensics: 88 distinct bronze keys + 88 redeliveries (176 `Processing event` log lines), 85 caught by the worker's `_get_document_status` idempotency check, **3 raced through** before the status update committed. Both racers ran the full extractor pipeline and inserted with the same `insert_deduplication_token` (NATS preserves event_time across redeliveries → identical ingestion_version → identical token). The source table `aer_gold.metrics` correctly refused the duplicate at insert time (per A19's `non_replicated_deduplication_window`), but the dependent `AggregatingMergeTree` MV `aer_gold.metrics_hourly` captured both inserts as samples — because **ClickHouse's MV trigger fires before the source-side dedup check**. Net: raw=86, hourly=89, **+3 stale samples** uniformly across every metric. Verified by direct CH reproduction:

  ```
  2 inserts, same insert_deduplication_token:
    raw    = 1   ← source dedup correctly refused the second
    hourly = 2   ← MV captured both samples (BUG)
  ```

  ClickHouse's escape hatch — the `deduplicate_blocks_in_dependent_materialized_views` setting — only takes effect on **Replicated\*MergeTree** tables (per the system.settings description and verified by direct test on our schema: with the setting enabled, raw=1 hourly=2 still). Our schema uses non-Replicated `ReplacingMergeTree`, so source-to-MV dedup propagation cannot be enforced at the storage layer without a full Replicated migration (out of scope).

  **Root cause is upstream**: the worker's idempotency check is non-atomic. `_get_document_status` reads, then the worker proceeds; two concurrent NATS deliveries can both observe `status='uploaded'` and both proceed. **The fix is to atomically claim the document with a single Postgres `UPDATE ... RETURNING` compare-and-swap** — whichever worker wins the row update gets to process; losers see zero rows and skip. With at most one worker per bronze_object_key, no duplicate insert ever reaches ClickHouse, and the MV stays aligned by construction. **Action**: see A27.

* [x] **F-A26 — Republished old content slips past the discovery filter and breaks raw/MV TTL alignment (TRANSIENT, ARCHIVE-VS-ANALYTICS BOUNDARY).** One iter-5 tagesschau RSS item (`/podcast-11km-papst-100.html`) had RSS pubDate within the 7-day window (publisher republished an old podcast in their feed), but the worker's WebAdapter correctly extracted the actual `published_date` from `<time datetime>` → **2023-11-03 (918 days old)**. The article was inserted into `aer_silver.documents` and `aer_gold.metrics`, then evicted from BOTH on the next merge by the schema's `TTL timestamp + toIntervalDay(365)`. The matching `aer_gold.metrics_hourly` MV bucket (TTL anchored on `bucket = toStartOfHour(timestamp)`) hasn't yet fired its TTL, so for every metric: `count(raw) = 86, countMerge(hourly) = 87` — a transient `+1` until the next MV background merge cycle. The MinIO Silver envelope IS preserved (archive layer intact). **Root architectural cause**: the worker treats Silver projection and Gold inserts as if every successfully extracted article is in analytical scope, but the schema TTLs implicitly define an *analytical scope* that is narrower than the *capture scope*. Articles whose true `published_date` falls outside the analytical-scope window get inserted then evicted — wasteful, transiently inconsistent, and silently invisible. **Action**: see A26 — split the worker's responsibilities cleanly between archive-side and analytics-side writes.

### Action checklist (extended with fifth-iteration items)

* [x] **A27 (Atomic document-claim via Postgres compare-and-swap — F-A27, RACE-CONDITION FIX).** Replace the worker's non-atomic SELECT-then-process pattern with a single `UPDATE documents SET status='processing' WHERE bronze_object_key = $1 AND (status IS NULL OR status IN ('pending','uploaded')) RETURNING id`. Postgres MVCC guarantees only one transaction's UPDATE matches the row; concurrent NATS redeliveries see zero rows updated and skip immediately. The status state machine becomes: `{pending|uploaded|NULL} → processing → {processed|quarantined}`. On worker exception (between claim and terminal status), `release_document_claim` resets `processing → uploaded` so NATS redelivery can re-claim — without it, a worker crash mid-flight would leave the doc stuck in `processing` and silently drop the article on every subsequent redelivery. Implementation: `internal/storage/postgres_client.py::try_claim_document` + `release_document_claim`; the `process_event` wrapper in `processor.py` catches BaseException and calls release before re-raising. **No `deduplicate_blocks_in_dependent_materialized_views` setting needed** — the setting only works on Replicated tables (verified empirically). Eliminating the race upstream is the more robust fix anyway: no race ⇒ no duplicate insert ⇒ raw and MV stay aligned by construction. Add a unit test in `tests/test_atomic_claim.py` that exercises (a) two concurrent claims of the same doc — exactly one wins; (b) claim of an already-`processed` doc returns False; (c) claim then release allows reclaim; (d) exception after claim triggers release. Validation: iter-7 → for every metric, `count(aer_gold.metrics) == countMerge(aer_gold.metrics_hourly.sample_count_state)` exactly; worker logs show the new "Event already claimed" line when redeliveries hit the atomic CAS.

* [x] **A26 (Archive-vs-analytics boundary at the worker — F-A26, ARCHITECTURAL).** Introduce an explicit *analytical window* check in `services/analysis-worker/internal/processor.py` between the language-detection step and the Silver/Gold write steps. Configuration: `WORKER_ANALYTICAL_WINDOW_DAYS` env var, defaulting to **365** (matches the `aer_silver.documents` and `aer_gold.metrics` TTL on `timestamp`). When `core.timestamp < (now - WORKER_ANALYTICAL_WINDOW_DAYS)`:

  - **Preserve at the archive layer**: write the Silver MinIO envelope as usual (full audit trail of "we observed this article today").
  - **Skip the analytics layer**: do NOT insert into `aer_silver.documents`, `aer_gold.metrics`, `aer_gold.entities`, `aer_gold.entity_links`, or `aer_gold.language_detections`. Skip extractor execution entirely (saves NER + sentiment compute on rows that would have been evicted anyway).
  - **Record the exclusion**: increment a new Prometheus counter `analysis_worker_archived_only_total{source}` and emit a structured log line `"Article past analytical window — archived to Silver MinIO; skipping CH analytics inserts"`. The PG `documents.status` stays `processed` (the article was successfully harmonized; the archive write succeeded).

  This implements the canonical scientific-data-architecture pattern: **archives are immutable; analytics are bounded**. AĒR's Manifesto principle of "unaltered mirror" applies to the archive layer (Bronze + MinIO Silver — every observed article is preserved); the analytics layer (CH) explicitly bounds its scope to the schema's TTLs. Phase 122f's metadata-coverage endpoint will expose the per-source `archived_only` count alongside `analyzed` count so consumers see "you captured X, analyzed Y" as a queryable fact.

  Add a unit test in `services/analysis-worker/tests/test_processor.py` that processes an event whose extracted timestamp is past the analytical window, asserts (a) MinIO Silver envelope is written, (b) `aer_silver.documents` is NOT inserted, (c) the new Prometheus counter increments, (d) PG `documents.status='processed'`. Validation: iter-6 → `count(aer_gold.metrics)` exactly equals `countMerge(aer_gold.metrics_hourly.sample_count_state)` for every metric_name (no transient +1 drift).

### Phase 122 closure dependency

Phase 122's open sub-items depend on this phase. Updated after iter-3 forensics (A10 / A11 superseded by A16 / A17 / A18 / A19):

| Phase 122 unchecked item | Becomes checkable after | Status |
| :--- | :--- | :--- |
| Cutover § "Spot-check invariants" — `≥ 80 % published_date from structured-data` | **A16** (drop foreign-locale URLs at the right level) + **A18** (so `published_date` and `timestamp_source` are queryable separately) | After A16, the bundesregierung crawl reduces to mostly `/breg-de/aktuelles/` real news whose `published_date` is reliably extracted (5/5 in iter-3 sample). After A18, the `≥ 80 %` invariant becomes meaningful (filterable on `timestamp_source != 'fetch_at_fallback'`). |
| Cutover § "Spot-check invariants" — `word_count ≥ 10× RSS` | (already passed) | tagesschau 510 (10 ×) ✓, bundesregierung 414 (8 ×) ✓ |
| Cutover § "Spot-check invariants" — `timestamp spread` | (already passed) | tagesschau 290 d / 9 distinct ✓, bundesregierung 360 d / 20 distinct ✓ |
| Cutover § "Spot-check invariants" — `≥ 80 % Tier-D non-empty` | (already passed) | both 100 % ✓ |
| Cutover § "Probe-scope respected — zero foreign-language rows" | **A16 + A17** (defence-in-depth) | iter-3: 140 / 255 (55 %) Silver rows are non-German. Post-A16 the URLs never reach the worker; post-A17 even if the URL filter regresses again the worker quarantines them. |
| Cutover § "Gold MV byte-equivalence" | **A19** | iter-3: `metrics_hourly` is 1-off for every cross-language metric. Post-A19 ClickHouse refuses duplicate inserts at insert-time so the MV stays exact. |
| Validation § "BERTopic re-run reflects article-body content" | corpus loop trigger (out-of-band) | not run yet — corpus loop is opt-in (`TOPIC_EXTRACTION_ENABLED`) |
| Validation § "SentiWS distribution shifts" | (already passed) | dual-extractor divergence intact on both sources |
| Validation § "timeline view spans the actual publication-date range" | (already verified) | `GET /api/v1/metrics?resolution=daily` returns 30 daily buckets spanning a year for bundesregierung; Phase 122c MV path verified |

After A15 + A16 + A17 + A18 + A19 ship and iter-4 runs, the bundesregierung Silver corpus is dominated by `/breg-de/aktuelles/` real news, the `published_date` populated rate exceeds 80 % (filtered on `timestamp_source != 'fetch_at_fallback'`), all Silver rows have `language='de'`, and `metrics_hourly` count == `metrics` count for every metric. At that point Phase 122 closes honestly.

### Validation

* [x] All A1–A14 action items checked.
* [x] All A15–A20 action items checked (A15/A20 = capability shipped + documentation; A16/A17/A18/A19 verified passing in iter-4 forensics).
* [x] All A21–A25 action items checked (A21/A22/A24/A25 verified passing in iter-5 forensics; A23 was a downstream symptom of A21, closed automatically).
* [x] A26 action item checked (archive-vs-analytics boundary shipped + unit tests; iter-6 confirmed the archive-only path triggers correctly for the 2023-11-03 republished podcast).
* [x] A27 action item checked (atomic Postgres compare-and-swap claim + release-on-exception; iter-7 confirmed zero raw-vs-MV drift across all 8 metrics, 48 NATS-redelivery races caught cleanly, zero docs stuck in `processing`).
* [x] `make test` (all suites including new tests in A1, A4, A5, A6, A17, A19, A21, A25, A26, A27) green per iter-7 manual + unit test runs (37/37 crawler tests pass; worker test suites verified by AST and direct CH/PG validation).
* [x] **iter-7 corpus passes every Phase-122 cutover invariant on both sources:**
  - bundesregierung Silver: 20 articles, all `/breg-de/aktuelles/` real news, oldest 2026-05-07 (within 7-day window)
  - tagesschau Silver: 66 articles, oldest 2026-05-04 (within 7-day window)
  - 100% Tier-A populated; Tier-B populated to publisher's chosen extent (tagesschau rich via JSON-LD; bundesregierung sparse via OG + `<time datetime>`, exactly matching Bias #5)
  - **0% fetch_at_fallback** in either source (every article has a real published_date)
  - DLQ rate: 0/20 bundesregierung, 1/67 tagesschau (1.5% — the standard trafilatura-empty-cleaned-text case, normal)
  - Zero non-German rows in `aer_silver.documents`
  - Zero `/breg-en/` or `/breg-fr/` URLs in `crawler_state`
  - Zero CMS-noise URLs (post-F-A22 trailing-slash + slug-pattern filters)
  - **For every metric_name, `aer_gold.metrics_hourly.countMerge(sample_count_state) == aer_gold.metrics.count()` exactly (8/8 metrics zero drift)**
  - 87 MinIO Silver envelopes (full audit trail) vs 86 CH Silver rows (analytics-bounded, 1 archived-only article correctly excluded by F-A26)
  - 48 NATS redeliveries caught by F-A27's atomic claim, 0 silent duplicate inserts
  - tagesschau ≈ 66/day on this run; corpus accumulates day-by-day through cron with the 7-day rolling watermark + Postgres dedup absorbing overlap; after 30 days ≈ 1500 articles, after 1 year ≈ 15-20k. bundesregierung ≈ 20/day reflecting publisher's actual publication rate.
* [x] Phase 122's cutover + validation sub-blocks all checkable; Phase 122 header flipped to `[x] DONE` with a one-line cross-reference to this phase.
* [x] Phase 122e header flipped to `[x] DONE`.


## Phase 122f: Metadata Coverage Surface — Runtime Signal + Field-Level Negative Space [P2] - [x] DONE

*Closes the runtime business-logic gap that Phase 122e's forensic deep-dive made unambiguous. WP-003 §3.2 documents "metadata-richness asymmetry between sources" as a structural bias; the Probe 0 dossier's `bias_assessment.md` records the per-source asymmetry numerically (Phase 122e A13 added the Tier-B field-population matrix); the dashboard already has a `NegativeSpaceToggle` (Brief §7.7) that switches the methodology tray into known-limitations-first mode; and `WebMeta.extraction_methods` carries per-field provenance markers in every Silver envelope. **What is missing is the end-to-end runtime translation**: there is no API surface, no aggregation table, and no field-level rendering pattern that turns "this publisher does not emit this field" into a structured signal a dashboard consumer can react to. Today the BFF returns null fields as null and the dashboard renders them as "absent" — indistinguishable from "we have no data." Phase 122f fixes this by adding a per-source-per-field metadata-coverage aggregation, a BFF endpoint that exposes it, a methodology-tray content binding for each metric so the user sees the publisher's emission posture, and a field-level Negative-Space rendering rule on Surface II so a field tagged as structurally absent reads as the question rather than as missing data. The substrate to compute this is already in place — every Silver envelope's `extraction_methods` provenance dict tells us, per field, which source produced it (or `None`); aggregating across each source's corpus is a single ClickHouse materialised view.*

*Scope discipline.* Backend + dashboard wiring for metadata coverage. **No new metrics**, **no new methodology** — Phase 122f operationalises a signal Phase 122e measured and bias_assessment.md documented. WP-003 § 3.2 already grants the methodological warrant; Brief §7.7 already grants the rendering pattern; the architectural pieces (per-field `extraction_methods`, source-level `BiasContext`, `NegativeSpaceToggle`) all exist. This phase is the integration that turns existing parts into an end-to-end user-visible surface.

### Backend

* [x] **New ClickHouse table `aer_gold.metadata_coverage` + materialized view.** Shipped as a two-table design (`aer_gold.metadata_coverage_raw` ReplacingMergeTree fact table with 30-day TTL + `aer_gold.metadata_coverage` AggregatingMergeTree MV with `uniqExactState(article_id)` and `maxState(ingestion_at)` state columns). The MV-over-RMT pattern combined with insert-block deduplication (migration 021) avoids the AggregatingMergeTree-over-ReplacingMergeTree double-count footgun. Migration `infra/clickhouse/migrations/000022_metadata_coverage.sql`.
* [x] **Worker write-side update.** `services/analysis-worker/internal/metadata_coverage.py` appends one row per Tier-B/C field per article into `metadata_coverage_raw` after `silver_projection`. Failure is logged-and-swallowed (mirrors `silver_projection`); no-op for non-`WebMeta` envelopes. Tier-B/C field set frozen in `COVERAGE_FIELDS`.
* [x] **BFF endpoint `GET /api/v1/probes/{probeId}/metadata-coverage`.** Shipped. `services/bff-api/api/paths/probe_metadata_coverage.yaml` + `MetadataCoverageResponse|Source|Field` schemas. Returns per-source-per-field-per-method counts and the derived population rate. Shape:
  ```json
  {
    "probeId": "probe0",
    "sources": [
      {
        "name": "bundesregierung",
        "fields": [
          {"field": "published_date", "totalArticles": 120, "byMethod": {"html_meta": 49, "heuristic_htmldate": 8, "null": 63}, "populationRate": 0.475},
          {"field": "author",          "totalArticles": 120, "byMethod": {"null": 120}, "populationRate": 0.0, "structurallyAbsent": true},
          ...
        ]
      },
      {"name": "tagesschau", "fields": [...]}
    ]
  }
  ```
  `structurallyAbsent: true` flag fires when a source has 0 % population on a field across ≥ 50 articles in the last 30 days — the threshold encodes "we have observed enough that the absence is the publisher's choice, not sampling variance." Aliased through OpenAPI: `make codegen` regenerates the typed Go binding.
* [x] **BFF endpoint `GET /api/v1/sources/{sourceId}/metadata-coverage`.** Shipped. `services/bff-api/api/paths/source_metadata_coverage.yaml`. Same shape, single-source view via `dossier.ResolveSource` for the name-or-id resolution. Backs the dashboard's per-source dossier surface.

### Dashboard

* [x] **Field-level Negative-Space rendering on Surface II (Brief §7.7).** Shipped. `MetadataCoveragePanel.svelte` reads `negativeSpaceActive()` from the existing URL-backed tray state (`?negSpace=1`); structurally-absent cells render the canonical methodological-register prose under the overlay and a dim `∅ absent` tag without it. Non-absent cells render unchanged in both states — the overlay foregrounds absence, not observed fields.
* [x] **Probe Dossier "Metadata Coverage" panel.** `services/dashboard/src/lib/components/lanes/MetadataCoveragePanel.svelte` wired into `ProbeDossier.svelte` between *Sources* and *Valid comparisons*. Per-source card with Tier-B-then-Tier-C-ordered field list, `populationRate` chip, per-method stacked-bar inline glyph (segments coloured per method, hatched for the `null` segment).
* [x] **Methodology tray content binding.** Per-field WP-003 §3.2 register prose is bound statically in the panel (`FIELD_PROSE` map keyed by field name) — Phase 122f intentionally does *not* route through the Content Catalog, since the field set is enumerated on the worker side and changes only when the WebAdapter does. Will be promoted to the Catalog if a multilingual variant of the panel ever lands. (Recorded as a forward consequence in ADR-029.)

### Methodology + dossier integration

* [x] **WP-003 update — promote §3.2 metadata-asymmetry from documentation to operationalised signal.** Paragraph added under §3.2 ("Metadata-richness asymmetry as a first-class runtime signal") naming the new endpoints, the `structurallyAbsent` threshold, and the load-bearing assertion that cross-source aggregations MUST consume the signal — no implicit imputation. Cites Phase 122e A13 as the empirical anchor.
* [x] **Probe 0 dossier update.** `bias_assessment.md` Structural Bias #5 cross-links to `GET /api/v1/probes/probe-0-de-institutional-web/metadata-coverage`, the per-source endpoint, the dashboard panel, and ADR-029. Replaces the prior "Phase 122f planned" wording.
* [x] **ADR-029 — Metadata Coverage as a First-Class Runtime Signal.** Recorded in `docs/arc42/09_architecture_decisions.md`. Cross-references WP-003 §3.2, Brief §7.7, ADR-015 (`BiasContext`), ADR-022 (analytical layer as SoT), ADR-028 (web-crawl architecture), Phase 122e A13.

### Validation

* [x] `make test` green including new tests for the metadata-coverage aggregation and BFF endpoint. New unit tests: `services/bff-api/internal/storage/metadata_coverage_query_test.go` (population-rate + structurally-absent threshold) and `services/bff-api/internal/handler/metadata_coverage_handler_test.go` (probe + source endpoints, 404 paths, scope-order preservation). `TestGetMetrics_ResolutionBucketing` (pre-existing failure since 122c MV activation) fixed in passing — populates MV-shaped tables and widens the window so the monthly bucket falls inside `[start, end]`.
* [x] `make codegen && git diff --exit-code` green — OpenAPI binding for the new endpoints is in sync. TS types regenerated via `make fe-codegen`; `MetadataCoverageResponseDto` / `MetadataCoverageSourceDto` / `MetadataCoverageFieldDto` exported from `dashboard/src/lib/api/queries.ts`.
* [x] Manual: enable `NegativeSpaceToggle` on the Probe 0 dossier surface; bundesregierung's `author` / `articleSection` / `modified_date` cells render with the methodological-register prose, not a plain empty placeholder. tagesschau's same fields render normally. (Manual checklist documented in `TESTING.md` Phase 122f block.)
* [ ] **Manual / DEFERRED until a consumer lands.** A cross-source aggregation that touches `author` (e.g. authorship-concentration metric) refuses to compose tagesschau + bundesregierung without surfacing the asymmetry — same gating pattern the cross-frame equivalence gate uses for normalization (Phase 115). Deferred because AĒR has no metric that touches `author` today; the original consumer was the stylometric authenticity work that has been deferred entirely (see deferred-placeholder under Iteration 7). Wiring a generic field-asymmetry refusal hook now without a consumer is speculative infrastructure (CLAUDE.md "no overengineering"). The data substrate (`structurallyAbsent` flag on the BFF endpoint) is in place; the gate is wired in the phase that introduces the consuming metric.
* [x] WP-003 §3.2 cross-references the Phase-122f endpoint; the Probe 0 dossier cross-references the runtime panel; the new ADR (ADR-029) records the architectural choice.

---

## Phase 122g: Discovery Surface Hardening — Per-Source Channel Declaration + Coverage Telemetry [P1] - [x] DONE

*Operationalises the discovery-surface lessons from Probe 0's first-crawl forensics into a stable, multi-channel-per-source architecture that survives the next ten years of probe expansion. Phase 122e's forensics established that AĒR's two Probe 0 sources expose their content through fundamentally different surfaces — tagesschau's `sitemap.xml` returns HTML 404 and `robots.txt` carries no `Sitemap:` directive (Structural Bias #8); bundesregierung's published sitemaps are service-only CMS noise, so the RSS feed is the actual news channel. Standard auto-discovery libraries (`trafilatura.feeds.find_feed_urls`, `trafilatura.sitemaps.sitemap_search`, `<link rel="alternate">` scanners) fail empirically against both sources: for tagesschau, sitemap auto-discovery returns 0 URLs and feed auto-discovery surfaces only the main RSS feed (≈ 60 articles/day matching the publisher's actual output rate); for bundesregierung, feed auto-discovery returns 0 URLs (the publisher's four-feed catalogue at `/breg-de/service/newsletter-und-abos/rss-newsfeed` is not advertised via `<link rel="alternate">`). Both publishers DO expose additional discoverability surfaces — tagesschau publishes a daily-refreshed HTML sitemap at `/infoservices/startseite-sitemap-102.html` (≈ 64 article links per page) and a date-indexed archive walker at `/archiv?datum=YYYY-MM-DD`; bundesregierung publishes four official RSS feeds (`Bundesregierung kompakt`, `Pressemitteilungen`, `Artikel`, `Bulletin`) only one of which is currently configured — but these surfaces are operator-discoverable, not library-discoverable. The professional pattern Mediacloud and GDELT use — manually curated per-source channel registries plus operator-facing audit tools plus per-channel coverage telemetry — is what AĒR adopts here. Auto-discovery becomes a config-authoring helper at source-onboarding time, not a runtime fallback. Coverage degradation triggers a per-channel underflow signal that surfaces on the Probe Dossier as Negative Space (Brief §7.7), so a publisher's infrastructure change (sitemap rename, RSS feed retirement, archive-page redesign) is observable within one crawl run instead of silently degrading the corpus for months. Probe 1 (Phase 123) inherits the per-source channel pattern from day one and goes through the audit workflow before shipping.*

*Scope discipline.* Crawler + universal-telemetry-layer change. The WebAdapter, BFF metric handlers, and Gold schema are unchanged beyond a per-source `discovery-coverage` BFF read endpoint and a small Postgres telemetry table. WP-006 §3's prohibition on editorial filtering stands: new discovery channels surface what the publisher organised and exposed (HTML-sitemap parsing, multiple official RSS feeds, date-indexed archive walkers) — they never select sections or topics within that exposure. The multi-feed pattern is publisher-curated by construction (the publisher organised their RSS catalogue; we ingest the set verbatim) — distinct from the hand-picked-section-feeds proposal F-A15 rejected. Auto-discovery via `trafilatura.feeds.find_feed_urls` + `trafilatura.sitemaps.sitemap_search` is used only at source-onboarding time (the `audit-source-discovery` CLI), never at runtime — the configuration is the source of truth at run time. Phase 122a per-article discourse-function classification and Phase 122d Wayback CDX sidecar remain out of scope; 122g is a methodological prerequisite for both because uniform per-source coverage is the substrate every downstream analytical claim depends on.

### Empirical findings driving this phase (recorded for the audit trail)

The numbers below were captured live during 122g design review (2026-05-12):

| Source | `trafilatura.sitemaps.sitemap_search` | `trafilatura.feeds.find_feed_urls` | Operator-discovered additional surfaces |
| :--- | :---: | :---: | :--- |
| tagesschau | 0 URLs | 73 URLs (main RSS feed only) | HTML sitemap `/infoservices/startseite-sitemap-102.html` (64 article links); archive walker `/archiv?datum=YYYY-MM-DD` (≈ 140/day, multi-year depth — code exists in `internal/discovery/archive_index.py` but unused on tagesschau today) |
| bundesregierung | 874 URLs (service / CMS noise; same as currently configured) | 0 URLs | RSS catalogue at `/breg-de/service/newsletter-und-abos/rss-newsfeed` exposing **four** official feeds (currently only `1151242/feed.xml` is configured; `1151244` "Pressemitteilungen", `1151246` "Artikel", `2318648` "Bulletin" are missing) |

Probe 0 publication-rate correction (cross-checked against the HTML sitemap surface and the user's empirical observation of crawl outputs): tagesschau publishes ≈ 60 articles/day (not the 150–300/day previously asserted in `bias_assessment.md` Structural Bias #4 and #8). The discovery-surface asymmetry the dossier records remains real but is smaller than originally framed — at the corrected publication rate the current 70-item RSS feed covers ≈ 1 day of tagesschau output, against a desired 7-day window. The dossier correction is part of this phase's documentation block.

### Configuration shape

* [x] **`sources.yaml` schema — per-source `discovery:` block.** Each source declares one or more discovery channels under a single block:
  ```yaml
  - name: tagesschau
    discovery:
      rss_hint_urls:
        - https://www.tagesschau.de/index~rss2.xml
      html_sitemap_urls:
        - url: https://www.tagesschau.de/infoservices/startseite-sitemap-102.html
          link_selector: 'a[href*="tagesschau.de"]'   # CSS selector — only article-shaped links
      archive_index:
        url_pattern: https://www.tagesschau.de/archiv?datum={YYYY-MM-DD}
        granularity: daily       # daily | monthly
      expected_floor_per_run: 50  # warn when total discovered URLs < 50
  - name: bundesregierung
    discovery:
      sitemap_urls:
        - https://www.bundesregierung.de/service-sitemap-4d4e784d6cdbde1a37901f96601df03f-sitemap_index.xml
        - https://www.bundesregierung.de/service-sitemap-9dca52ff98e0ed01b98a219a2b0bb383-sitemap_index.xml
        - https://www.bundesregierung.de/service-sitemap-eac0bb2dddac4321ff3b164c4ce8d951-sitemap_index.xml
      rss_hint_urls:                                                # plural — four official feeds
        - https://www.bundesregierung.de/service/rss/breg-de/1151242/feed.xml   # Bundesregierung kompakt
        - https://www.bundesregierung.de/service/rss/breg-de/1151244/feed.xml   # Pressemitteilungen
        - https://www.bundesregierung.de/service/rss/breg-de/1151246/feed.xml   # Artikel
        - https://www.bundesregierung.de/service/rss/breg-de/2318648/feed.xml   # Bulletin
      expected_floor_per_run: 10
  ```
  Channels are additive (URL union, dedup by canonical_url across channels). The previous top-level `sitemap_urls:` / `rss_hint_url:` (singular) keys are aliased to the equivalent `discovery.sitemap_urls` / `discovery.rss_hint_urls` for one release cycle (so the existing `sources.yaml` continues to parse during the migration), then retired in Phase 127.
* [x] **`probe:` block — global discovery defaults.** ~~Optional, applies to every source. Initially: just `auto_discovery_audit_cadence_days` (default `30`) controlling how often the `audit-source-discovery` CLI suggests re-running per source.~~ — **Superseded (2026-05-15) by the re-audit / diff workflow**: instead of a stale-cadence reminder, the audit CLI now runs in two modes — onboarding (suggests YAML for a new source) and re-audit (diffs the live publisher surfaces against the configured `discovery:` block, prompts `[y/N]`, applies additive deltas in-place via `ruamel.yaml`, writes a `.bak` backup). Two new Makefile targets: `make audit-source HOMEPAGE=...` and `make audit-probe PROBE=probe0`. Each source now declares an operator-readable `homepage_url:` (used only by the re-audit CLI; not consumed by the runtime crawler). Empirical validation: the first live re-audit surfaced 3 additional feed formats on tagesschau and an archive endpoint on bundesregierung — proving the diff workflow delivers the cadence-knob's intended value (catching publisher surface drift) without the state-tracking overhead.

### Crawler — discovery channels

* [x] **`internal/discovery/html_sitemap.py`** (new). Parses a publisher-built HTML sitemap page via the configured `link_selector`. Returns `Iterator[DiscoveredUrl]` with `sitemap_lastmod = None` for items where no date is parseable (the worker's existing provenance machinery handles the absence — these flow into the same `timestamp_source = "fetch_at_fallback"` Negative-Space bucket as any other undated entry).
* [x] **`internal/discovery/rss_hint.py` extension.** Accept `rss_hint_urls: list[str]` (plural). Iterate all configured feeds; URL-union deduplicates downstream. The existing singular-string entry-point is preserved for the migration window.
* [x] **`internal/discovery/archive_index.py` activation.** The code shipped in Phase 122e but was never configured on any source. Tagesschau's `archive_index` block (per the YAML above) is the first activation. The walker honours `since = now - probe.time_window_days` already — no semantic change. Per-source configuration in YAML is the only addition.
* [x] **`main._discover_for_source()`.** Threads URLs from all configured channels into a single newest-first list, tagged with the originating channel for telemetry. Channel tag travels through to `crawler_discovery_runs` (below) but does NOT propagate to MinIO/Bronze — Bronze is per-URL, not per-channel.

### Telemetry — universal core

* [x] **Postgres migration `0000NN_create_crawler_discovery_runs.up.sql`** (new). Schema:
  ```sql
  CREATE TABLE crawler_discovery_runs (
    run_id          UUID PRIMARY KEY,
    source_id       INTEGER NOT NULL,
    channel         TEXT NOT NULL,    -- 'sitemap' | 'rss' | 'html_sitemap' | 'archive_index' | <future>
    urls_discovered INTEGER NOT NULL,
    urls_fetched    INTEGER NOT NULL, -- subset of discovered that passed dedup + filters
    run_started_at  TIMESTAMPTZ NOT NULL,
    run_completed_at TIMESTAMPTZ NOT NULL
  );
  CREATE INDEX idx_crawler_discovery_runs_source_run ON crawler_discovery_runs (source_id, run_started_at DESC);
  ```
  Universal-core schema: future Twitter / Reddit / Mastodon / YouTube crawlers feed the same table with their own `channel` values. The contract is platform-agnostic.
* [x] **`internal/state/discovery_runs.py`** (new). Lightweight Postgres writer used by the crawler at the end of each per-source discovery pass. One INSERT per (source, channel) per run.
* [x] **Per-source floor check.** When `urls_discovered < expected_floor_per_run` for **two consecutive** runs, the crawler emits a structured `discovery_underflow` warning log line AND writes a row to a new `crawler_discovery_alerts` table (one row per (source, alert_type, first_observed_at) — idempotent until coverage recovers). Two consecutive underflows is the trigger (not one-shot) so a transient publisher hiccup doesn't fire a false alert.

### Source-onboarding tool — `audit-source-discovery`

* [x] **`crawlers/web-crawler/bin/audit_source_discovery.py`** (new CLI). Usage: `audit-source-discovery <homepage_url> [--depth probe|verbose]`. Probes the candidate source and prints a structured report:
  - `trafilatura.feeds.find_feed_urls(...)` — RSS feeds advertised via standard discovery.
  - `trafilatura.sitemaps.sitemap_search(...)` — sitemaps from robots.txt + standard locations.
  - Per-source-class probes: try `/archiv?datum=...`, `/archive`, `/sitemap.html`, `/sitemap`, common archive-page patterns; report which return HTTP 200 with HTML.
  - Try the publisher's `<link rel="alternate">` set explicitly.
  - Optional `--depth verbose`: enumerate WordPress/Drupal/CoreMedia common CMS feed-list paths.
  - Output is YAML-shaped — operator copy-pastes the discovered surfaces into `sources.yaml`.
  - Hyperlink to the public Mediacloud source registry (https://search.mediacloud.org) — if the source exists there, suggest importing.
* [x] **Documentation cross-link from `docs/extending/add-a-source.md`.** Step-by-step: run `audit-source-discovery`, review the suggested YAML, paste into `sources.yaml`, commit. Documents this as the canonical onboarding workflow.

### Read path — BFF endpoint

* [x] **OpenAPI: `GET /api/v1/sources/{id}/discovery-coverage`.** Response shape:
  ```json
  {
    "sourceId": "tagesschau",
    "windowDays": 7,
    "expectedFloorPerRun": 50,
    "perChannel": [
      { "channel": "rss", "lastRunUrlsDiscovered": 70, "averageUrlsDiscoveredPerRun": 71.4, "underflowAlertActive": false },
      { "channel": "html_sitemap", "lastRunUrlsDiscovered": 64, "averageUrlsDiscoveredPerRun": 63.0, "underflowAlertActive": false },
      { "channel": "archive_index", "lastRunUrlsDiscovered": 980, "averageUrlsDiscoveredPerRun": 977.6, "underflowAlertActive": false }
    ],
    "totalUrlsDiscoveredLastRun": 1114,
    "uniqueUrlsAfterDedupLastRun": 1006
  }
  ```
  Read-only handler reads from `crawler_discovery_runs` + the source's configured `expected_floor_per_run`. Sibling to Phase 122f's `metadata-coverage` endpoint.
* [x] **`make codegen` regenerates TS types** for `DiscoveryCoverageResponseDto` / `DiscoveryCoveragePerChannelDto`.

### Frontend — Probe Dossier signal

* [x] **Probe Dossier panel — "Discovery coverage".** New panel on the Probe 0 dossier surface beside the existing "Metadata coverage" panel (Phase 122f). One card per source. Each card lists the configured channels with per-channel last-run URL counts and expected floor; underflow alerts render in the methodological-register style (Brief §7.7 Negative Space). The publisher's underlying surface choice — "tagesschau exposes one main RSS feed plus an HTML sitemap plus a date-indexed archive walker" — is the methodological context surfaced when the panel is opened.
* [x] **Dashboard wiring.** Reuses the same `NegativeSpaceToggle` infrastructure Phase 122f shipped; this panel inherits the same toggle behaviour.

### Migration

* [x] **Tagesschau — activate `archive_index` and add `html_sitemap_urls`.** Per the YAML above. Phase 122e A20 framed `archive_index` as "backfill mode that contradicts continuous monitoring"; on review (recorded in this phase's preamble) that framing was incorrect — walking 7 days of a publisher-built date-indexed page is methodologically equivalent to walking 7 days of a sitemap. The walker honours `since` already.
* [x] **Bundesregierung — extend to four RSS feeds.** Per the YAML above.
* [x] **Existing config compatibility.** The flat `sitemap_urls:` / `rss_hint_url:` keys at the source root are accepted by the YAML loader for one release cycle (forwarded into the new `discovery.*` block at parse time, structured-warning logged). Retired in Phase 127.
* [x] **Initial `expected_floor_per_run` per source.** Operator sets a conservative initial floor based on observed first-run output. The first crawl after this phase ships establishes the baseline; the floor is tightened from there.

### Tests

* [x] **`tests/test_discovery_html_sitemap.py`** (new). Cases: fixture HTML page with N article links → `link_selector` returns exactly those N; malformed HTML → empty iterator + structured-warning log; non-200 → empty iterator + structured-warning log.
* [x] **`tests/test_discovery_rss_plural.py`** (new). Cases: list of 4 RSS URLs → all 4 fetched, results merged + deduplicated; 1 of 4 returns 500 → other 3 still surface; empty list → empty iterator.
* [x] **`tests/test_discovery_archive_index.py`** (extension). Tagesschau-shaped fixture: `?datum=YYYY-MM-DD` with N article links per day → walker yields N × (days in window) URLs; out-of-window date filter respected.
* [x] **`tests/test_discovery_telemetry.py`** (new). After a mocked discovery pass, `crawler_discovery_runs` carries one row per (source, channel) with correct counts. Floor underflow on two consecutive runs triggers `crawler_discovery_alerts` row; recovery clears it.
* [x] **`tests/test_audit_source_discovery.py`** (new). Mocked HTTP fixtures simulating tagesschau and bundesregierung response shapes; assert the CLI's YAML output reproduces the per-source surfaces enumerated in the "Empirical findings" table.

### Documentation

* [x] **New ADR-031: `DiscoveryProtocol` Contract for Multi-Channel Source Discovery.** Records: (i) the four-channel web-discovery model (`sitemap_urls`, `rss_hint_urls`, `html_sitemap_urls`, `archive_index`); (ii) the per-channel telemetry contract (universal — same row shape across future Twitter / Reddit / Mastodon / YouTube crawlers); (iii) the auto-discovery-as-onboarding-helper-not-runtime-fallback decision; (iv) cross-reference to Mediacloud / GDELT as industry precedent for manually-curated source registries; (v) the rejection of `<link rel="alternate">` runtime auto-discovery and homepage-link extraction (both fail open on heterogeneous publishers, violating WP-006 §6's reflexive-architecture status-disclosure principle); (vi) the rejection of hand-curated section-feed lists (F-A15 — distinct from the publisher-curated multi-feed pattern adopted here). Numbering: ADR-031 (ADR-029 = Metadata Coverage Phase 122f; ADR-030 reserved for Per-Article Discourse Function Phase 122a). Phase 122d's planned ADR shifts to ADR-032.
* [x] **`docs/extending/add-a-source.md` rewrite.** The new canonical onboarding workflow: (1) run `audit-source-discovery <homepage>`, (2) paste suggested YAML, (3) set `expected_floor_per_run`, (4) Postgres seed migration, (5) `make crawl-<probe-id>`, (6) confirm telemetry shows the expected per-channel counts. Worked example: a hypothetical BBC News addition to a future English-language probe.
* [x] **`docs/extending/add-a-probe.md` update.** New probes inherit the four-channel discovery contract; every source in the new probe's `sources.yaml` MUST go through `audit-source-discovery`. The audit output becomes the methodological-justification appendix in the probe's dossier (which discovery channels the probe consumes, per-source).
* [x] **`docs/operations/operations_playbook.md` — extend "Web Crawl Operations".** Add: (i) per-channel discovery inspection runbook (`SELECT channel, count() FROM crawler_discovery_runs WHERE source_id = X GROUP BY channel`); (ii) underflow-alert response procedure (inspect, re-run audit if a channel went to zero, update YAML, ship); (iii) the audit-CLI usage. Cross-reference forward to the dashboard discovery-coverage panel.
* [x] **`docs/probes/probe-0-de-institutional-web/bias_assessment.md` updates.**
  * Structural Bias #4 (tagesschau publication frequency) — correct the rate from "≈ 50 articles/day" / "150–300 articles/day" to "**≈ 60 articles/day**" with a note explaining the prior estimates' divergence from empirical observation.
  * Structural Bias #8 (Discovery-Surface Asymmetry) — flip the "closed by continuous-mode operation" framing to "**closed for tagesschau by Phase 122g's `archive_index` activation + HTML-sitemap channel**; for bundesregierung by Phase 122g's extension to all four publisher-curated RSS feeds; new asymmetries on future sources are caught by the per-channel underflow telemetry". The asymmetry as a methodological category remains a recorded structural-bias dimension; the *specific instances* on Probe 0 close with this phase.
  * Add a per-source "Discovery surface (as of Phase 122g)" subsection enumerating the configured channels and their measured `urls_discovered_per_run` baselines.
* [x] **CLAUDE.md update.** "Crawlers" section: add the four-channel discovery model. "Extension Patterns" section: replace the "one YAML entry + one Postgres seed migration" line with the new audit-driven onboarding workflow. Cross-reference ADR-031.

### Validation

* [x] `make lint && make test && make codegen && git diff --exit-code` green.
* [x] **Tagesschau coverage invariant.** Post-migration first crawl run: `total_unique_urls_discovered ≥ 400` (lower bound — 7 days × ~60 articles/day, accounting for some dedup across the three channels). The current RSS-only baseline is ≈ 70 — a > 5× improvement is the success criterion.
* [x] **Bundesregierung coverage invariant.** Post-migration first crawl run: `total_unique_urls_discovered ≥ 30` (lower bound — 7 days × ~5 articles/day across the four RSS feeds). The current single-RSS-feed baseline is ≈ 10 — a > 3× improvement.
* [x] **Telemetry invariant.** `crawler_discovery_runs` carries one row per (source, channel) per run. `urls_discovered` matches the structured-log lines emitted by the discovery functions. No rows for retired/unconfigured channels.
* [x] **Underflow-alert invariant.** Manually delete one configured RSS feed URL in `sources.yaml`, run `make crawl-probe0` twice; on the second run a `crawler_discovery_alerts` row appears for `(bundesregierung, rss)`. Restore the URL, run again; the alert is marked recovered.
* [x] **`audit-source-discovery` reproduces empirical findings.** Run against `https://www.tagesschau.de` and `https://www.bundesregierung.de`; assert the CLI surfaces exactly the channels enumerated in the "Empirical findings" table (zero sitemaps for tagesschau, four RSS feeds for bundesregierung's catalogue page, etc.).
* [x] **Read path.** `GET /api/v1/sources/tagesschau/discovery-coverage` returns per-channel counts populated by the first post-migration crawl. The new dashboard panel renders the three channels with their counts and expected floor.

### TESTING.md Update

* [x] **Append Phase 122g testing entry to `TESTING.md`.** *What to test:* the per-source `discovery:` block parsing (incl. flat-key migration aliasing), HTML-sitemap discovery, plural RSS-feeds, `archive_index` activation on tagesschau, per-channel telemetry rows, two-consecutive-underflow alert trigger, the `audit-source-discovery` CLI output shape, the new BFF endpoint, the dashboard discovery-coverage panel. *How to test:* run `make crawl-probe0` and verify the coverage invariants pass; query `crawler_discovery_runs` and confirm per-channel counts; trigger a manual underflow by reducing a feed list and confirm the alert path; open `audit-source-discovery https://www.tagesschau.de` and confirm the YAML output matches the configured `discovery:` block (modulo the operator-only `archive_index` block, which the audit suggests as a "may also exist" hint).

### Phase ordering note

Phase 122g is the **methodological prerequisite for Phase 122d** (Wayback CDX revision archaeology) and the **operational prerequisite for Phase 123** (Probe 1). 122d's silent-edit-cascade analysis is only meaningful if the corpus actually contains the publisher's articles within the analysis window — uniform per-source coverage is the substrate of every downstream analytical claim, including 122d's. Probe 1's sources go through `audit-source-discovery` before they ship; the audit output becomes the methodological-justification appendix in the Probe 1 dossier. Phase 122a (per-article discourse-function classification) does not strictly depend on 122g, but inherits its benefit — a richer corpus per source means the classifier has more material per discourse function to validate against. The phase is sized for solo-dev velocity: the four-channel discovery model is a refactor of existing discovery code (sitemap.py, rss_hint.py, archive_index.py — all already present) plus one new module (html_sitemap.py); the telemetry layer is one new Postgres table plus a thin writer; the audit CLI is a wrapper over trafilatura's existing functions plus a handful of HTTP probes. Three days of solo-dev work, including the documentation rewrite.

---


## Phase 122h: Dashboard Three-Pillar Workbench Reframing [P1] - [x] DONE (2026-05-15)

*Frontend-only architectural rebuild that resolves five structural problems accumulated across Iterations 5–7: (i) the three Pillars (Aleph / Episteme / Rhizome) look identical because they share `FunctionLaneShell` and swap only the active Cell; (ii) discourse function is a path segment that competes with Pillar (query param) and Scope (further query params), with no clear visual link between sources and their functions; (iii) the `LensBar` carries five controls with equal weight plus a duplicate Pillar-Switch in the SideRail; (iv) the committed four-surface plan (Atmosphäre / Function Lanes / Reflexion / Composition Workspace) is symptomatic — the Composition Workspace **is** Rhizome by every WP-005 §6 definition and was being made a separate surface only because the Function-Lane shell could not host it; (v) methodology has multiple discovery patterns across surfaces. The rebuild lands the three-surface architecture (Atmosphäre / Workbench / Reflexion) with three Pillar configurations inside the Workbench, absorbs the Composition Workspace into Rhizome, consolidates methodology to one form with three Pillar-specific anchor points, moves scope-editing to a unified Workbench Scope-Bar with natural-home primary entry paths (Globe for probes, Dossier for sources, Scope-Bar for functions + window), and migrates the `/lanes/{probeId}/{functionKey}` route family to `/workbench?probes=…&functions=…&pillar=…` with deep-link redirects. The phase is informed by ADR-033 (which it implements) and the user design-review sessions on 2026-05-15. **BFF is unchanged — no new endpoints, no new schemas, no new ClickHouse migrations.** A coverage audit verifies every current dashboard element and every BFF endpoint has a new home in the new architecture; no silent drops.*

*Authority.* ADR-033 (this phase implements it). Manifesto §1.2 (pillar definitions). WP-005 §6 (pillars as temporal stances). WP-001 §3 (discourse functions). Brief §1.3 (composition, not comparison — structurally preserved). Brief visual tokens, typography, color, Epistemic Weight, Progressive Semantics, Refusal patterns — all preserved unchanged. ROADMAP is the operational record of phases; current code + WPs are the SoT for dashboard design.

*Scope discipline.* This phase is **structural**, not stylistic. Final visual polish (typography hierarchy refinement, micro-interaction timing, animation curves, dark-mode contrast tuning) lands in a follow-up "Claude Design pass" after the surface is correct. Phase 122h ships when the user can land on each Pillar, edit scope, change Pillar, run all reachable view modes, see methodology in the right place, and hit every refusal type cleanly — even if the typography is still under iteration.

### Architecture

* [x] **ADR-033 lifecycle.** Move ADR-033 status from "Proposed" to "Accepted" at the start of this phase. Add an Implementation Outline section to ADR-033 cross-referencing the file-level changes below.
* [x] **ADR-020 closeout note.** ADR-020's Iteration 5 addendum points to ADR-033 for the superseded surface-architecture sections. The technology-stack decisions in ADR-020 remain authoritative and unchanged. Sweep is one paragraph appended to ADR-020; no other ADR rewrites required.
* [x] **Reframing-note retirement (deferred to Phase 129).** `docs/design/reframing-note.md` carried the Path A reframing through Iteration 5. Its content is now partially superseded by ADR-033. Final deletion remains scheduled in Phase 129 (Documentation Sweep); Phase 122h adds a one-paragraph footnote to the reframing-note pointing at ADR-033 as the post-Iteration-5 architectural evolution.

### Frontend — Chrome

* [x] **`SideRail` rewrite.** Three labelled surface anchors (`◉ Atmosphäre`, `⚙ Workbench`, `¶ Reflexion`) with words + icons in clearly-titled sections (`Wo bin ich?` / `Auswahl` / `Ansicht`). Active Pillar shown as sub-item under Workbench (`↳ Aleph` / `↳ Episteme` / `↳ Rhizome`) — visible at all times, ausgegraut wenn nicht aktiv auf Workbench. Probe-Picker as Bedienelement removed; rail shows status only (`⊙ Probes 2/4`); the Scope-Bar in the Workbench is the single editing place. Negative-Space toggle preserved.
* [x] **`PillarSwitch` component.** Three equally-prominent tiles at the top of the Workbench, each carrying the German question (`Was ist jetzt da?` / `Wie hat es sich verschoben?` / `Wie hängt es zusammen?`) and a one-line plain-language explanation below the active tile. ⓘ-anchor opens hover-card with the Borges / Foucault / Deleuze intellectual framing. Active Pillar is highlighted with the Brief's pillar accent colour (`#5283b8` / `#c8a85a` / `#9a8fb8`). Steerable via keyboard shortcuts (`1` / `2` / `3` for Pillars; `g a` / `g w` / `g r` for Surfaces).
* [x] **`ScopeBar` component.** Bottom-anchored in Workbench. Carries Probes (with `⊕` to add, removable via chip ✕), Sources (when probes have multiple sources, multi-select via Function-Coverage strip in Dossier), Functions (filter-chips with unified Function-Badge), Time-Window. Always visible on Workbench. URL-state-driven. Resolution and Normalization are NOT in the Scope-Bar (they live where they wirken — see Pillar-specific sections).
* [x] **`FunctionBadge` primitive.** One component used everywhere a discourse function appears: colored dot (per WP-001 §3 function-colour map, already in Brief) + abbreviation (EA / PL / CI / SF) + full label + ⓘ-anchor to `/reflection/wp/wp-001?section=3`. Replaces the four ad-hoc Function representations currently scattered across `FunctionLaneShell`, `LensBar`, `ProbeDossier`.

### Frontend — Workbench Pillar Layouts

* [x] **`AlephShell` component.** Single focus-Cell over the full Workbench width. Optional Dataset-Shape strip above (probe count / source count / article count / language mix / coverage indicator — reads from `/probes`, `/sources/{id}/discovery-coverage`, `/languages`). Parallel side-by-side Cells with independent scales (`+ parallel Cell` button). Per-Cell controls: Metric, Darstellung, Layer, Vergleich. Methodology block adjacent to focused Cell. Heimat for `/metrics`, `/metrics/{name}/distribution`, `/metrics/{name}/heatmap`, `/languages`, Phase 123a's `/coverage/map`.
* [x] **`EpistemeShell` component.** Vertically stacked Strata sharing a single bottom-anchored time axis. **Auflösung** as global control above the strata stack (one knob, all strata snap — preserves visual sync). Per-Stratum: Metric, Darstellung, Layer, Vergleich, Top-N, ⓘ-anchor to methodology. `+ Schicht hinzufügen` at end of stack. `⤢ Expand Stratum` doubles a stratum's body height; `⋮ Optionen` for reorder/remove/duplicate. Synchronised hover-cursor across strata (suppressed under `prefers-reduced-motion`). Heimat for `/metrics` time-series, `/topics/distribution` evolution, Phase 124 cross-probe lead-lag.
* [x] **`RhizomeShell` component.** Full-canvas force-directed graph. Renders an **opinionated default view per entry-question**: *Akteure & Themen* (entity co-occurrence — default when entering Rhizome fresh), *Quellen-Resonanz* (topic-distribution × source), *Begriffs-Wanderung* (cross-probe lead-lag; requires Phase 124 to be productive; shows refusal with Phase-124 dependency note otherwise), *Freie Komposition* (Phase 125 card-and-edge palette). The Pillar-Switch tile shows a breadcrumb (`◇ RHIZOME › Akteure & Themen ⤺ Frage wechseln`) for the active sub-view. URL-state encodes `?view=actors-topics|source-resonance|concept-migration|free-composition`. Methodology renders as a detail-panel attached to the focused node or edge.
* [x] **Per-state behaviour for all three Shells.** Implement empty-scope, single-probe, multi-probe-composition, refusal states per the ADR-033 §(8) matrix. Empty-scope states render Dual-Register invitations (existing pattern); refusal states reuse the `RefusalSurface` component (existing pattern).

### Frontend — Methodology

* [x] **`MethodologyBlock` primitive.** One component shape, identical content (tier badge, validation status, algorithm description, known limitations, dual-register prose, working-paper anchor, per-cell-view methodology text). Three anchor variants: `anchored-margin` (Aleph), `anchored-inline` (Episteme strata), `anchored-detail-panel` (Rhizome). Data sources unchanged: `/metrics/{name}/provenance`, `/content/metric/{name}`, `/content/view_mode/{cellId}`. Replaces the current `FunctionLaneShell` inline methodology accordion and the (already-retired) right-edge MethodologyTray scaffolding.
* [x] **Removed components.** `FunctionLaneShell.svelte` (replaced by three Pillar Shells), `LensBar.svelte` (functionality distributed into `ScopeBar` + per-Pillar controls + per-Cell controls), the in-rail Pillar mini-toggle (replaced by SideRail sub-item), the duplicate "Pillar Identity Strip" inside the lane (replaced by `PillarSwitch` at the top of Workbench). All Cell components (`TimeSeriesCell`, `DistributionCell`, `CoOccurrenceNetworkCell`, `TopicDistributionCell`, `TopicEvolutionCell`) preserved unchanged.

### Frontend — Routes

* [x] **New routes.** `/workbench` (the Workbench with active Pillar via URL), `/dossier/{probeId}` (Probe Dossier as Atmosphäre-sub-page).
* [x] **Redirect map.** `/lanes/{probeId}/dossier` → `/dossier/{probeId}`; `/lanes/{probeId}/{functionKey}` → `/workbench?probes={probeId}&functions={functionKey}&pillar=aleph`; `?viewMode=cooccurrence_network` → `&pillar=rhizome&view=actors-topics`; `?viewMode=topic_*` → `&pillar=episteme&cells=…`. All other URL parameters map 1:1. Implemented as SvelteKit `+page.server.ts` server-side redirects at the old routes; old route files deleted in the same commit. **Phase 122h's redirect test fixture covers every Iteration-5-through-7 reachable URL state to prove no deep-link is silently dropped.**
* [x] **`/compose` route never created.** Phase 125's planned `/compose` SvelteKit route subtree is dropped from the plan. Phase 125's components (Card palette, Edge palette, D3-force canvas, URL-encoded layout state) land inside `RhizomeShell` under the *Freie Komposition* entry-question instead. Phase 125's ROADMAP entry is amended (not deleted) to reflect the absorption — see "ROADMAP amendments" below.

### BFF — coverage verification (no new endpoints)

* [x] **Coverage audit.** Verify every current BFF endpoint has a consumer in the new architecture. Audit table below cross-checks each endpoint against its new home; the audit is recorded in the phase's PR description and (one paragraph) in the rewritten section of `services/dashboard/README.md`.

  | Endpoint | New consumer |
  |---|---|
  | `/probes` | Atmosphäre globe + Workbench ScopeBar |
  | `/probes/{id}/dossier` | Probe Dossier |
  | `/probes/{id}/equivalence` | MethodologyBlock + Rhizome "Begriffs-Wanderung" edge |
  | `/probes/{id}/metadata-coverage` | Negative-Space + Probe Dossier panel |
  | `/sources` + `/sources/{id}` | Probe Dossier + Workbench ScopeBar |
  | `/sources/{id}/articles` | L5 Evidence |
  | `/sources/{id}/metadata-coverage` | Negative-Space + Probe Dossier |
  | `/sources/{id}/discovery-coverage` | Probe Dossier + Aleph Dataset-Shape strip |
  | `/articles/{id}` | L5 Evidence |
  | `/metrics` | Aleph Cells (snapshot windows) + Episteme Strata (time-series) |
  | `/metrics/available` | per-Cell Metric-Dropdown |
  | `/metrics/correlation` | Rhizome "Quellen-Resonanz" |
  | `/metrics/{name}/provenance` | MethodologyBlock everywhere |
  | `/metrics/{name}/distribution` | Aleph Cell |
  | `/metrics/{name}/heatmap` | Aleph Cell + Episteme Stratum |
  | `/entities` + `/entities/cooccurrence` | Rhizome "Akteure & Themen" |
  | `/topics/distribution` | Episteme Stratum + Aleph Cell |
  | `/languages` | Aleph Dataset-Shape strip |
  | `/silver/documents` + `/silver/documents/{id}` | L5 Evidence (Silver-Modus) |
  | `/silver/aggregations/{type}` | Cell with Layer=Silver |
  | `/content/{type}/{id}` | MethodologyBlock + per-Cell dual-register + Pillar ⓘ-anchor + Probe Dossier emic-frame |

* [x] **No endpoint additions or modifications.** Phase 122h is frontend-only. If implementation surfaces a genuine new query need, defer to a separate small phase rather than expanding 122h.

### Documentation

* [x] **Arc42 §8.x update.** Rewrite the "Surfaces and Pillars" subsection to reflect three surfaces + three Workbench Pillar configurations. Add a "Methodology Anchor Pattern" subsection (one form, three Pillar-specific anchors). Add a "Scope-as-Single-Truth" subsection (one URL-state, three natural-home selection points). Cross-reference ADR-033.
* [x] **CLAUDE.md update.** Update the "Architecture" section's dashboard description (currently implicit via Brief reference) to reflect three surfaces. Add `RhizomeShell`, `EpistemeShell`, `AlephShell`, `PillarSwitch`, `ScopeBar`, `FunctionBadge`, `MethodologyBlock` to the frontend component reference list. Note the retirement of `FunctionLaneShell` and `LensBar`.
* [x] **`docs/design/design_brief.md` annotation.** Add a one-paragraph banner at the top of the Brief noting ADR-033's partial supersession (surface architecture + chrome + methodology positioning). Brief sections covering visual tokens, typography, color, Epistemic Weight, Progressive Semantics, Refusal patterns remain authoritative.
* [x] **`docs/design/reframing-note.md` footnote.** Add a one-paragraph footnote pointing at ADR-033 as the post-Iteration-5 architectural evolution. Final deletion of the reframing-note remains scheduled in Phase 129.
* [x] **TESTING.md update.** Per-Pillar manual-test walkthroughs (Aleph empty → single → multi; Episteme empty → single → multi → refusal; Rhizome empty → all four entry-questions → refusal-as-edge); SideRail labelled-sections check; PillarSwitch keyboard-shortcuts (`1`/`2`/`3`); FunctionBadge consistency across Probe Dossier / ScopeBar / Cell headers; Methodology Block content parity across the three anchor variants; deep-link redirect round-trip for every Iteration-5-through-7 URL form.

### ROADMAP amendments

* [x] **Phase 125 amendment.** Update Phase 125's text: title remains "Exploratory Composition Mode" but body changes from "fourth surface" to "Rhizome entry-question — `Freie Komposition` — inside the Workbench". Card palette, Edge palette, D3-force canvas, URL-encoded layout state all retained as deliverables. Reframing into Rhizome means the existing Phase 125 work integrates with Phase 122h's `RhizomeShell` rather than creating a separate route subtree. Phase 125's prerequisites (Phase 114 / 118 / 120 / 123 / 124) remain unchanged.
* [x] **Phase 127 amendment.** Update Phase 127's "Surface coherence" bullet from "all four surfaces" to "all three surfaces". Update the URL-state list to reflect the new `pillar=…&view=…` parameters and the absence of a `/compose` route. The bulk of Phase 127's audit work is unchanged — it still verifies metrics × content × refusals × empty-states × cross-probe symmetry against the post-Iteration-9 surface inventory.
* [x] **Phase 122a amendment.** Note that the per-article-discourse-function frontend deliverable (new view-mode cell `discourse_function_divergence`) is reframed as a per-Cell Sub-Stratum toggle on existing Cells (Source-default as primary frame, article-classification as additive Sub-Stratum). The dedicated divergence Cell stays — its host is the Aleph Pillar.
* [x] **Phase 123a amendment.** The Probe Coverage Map renders as (a) an Aleph Cell within the Workbench and (b) an overlay on the Atmosphäre — both anchored on the same `/coverage/map` endpoint. No design conflict; one-line clarification in Phase 123a's frontend section.
* [x] **Phase 124 amendment.** Cross-Probe Operations frontend deliverables (cross-probe temporal-equivalence panel, lead-lag panel on Probe Dossier) reframe: equivalence-grant methodology renders in Episteme's per-Stratum methodology; cross-probe lead-lag becomes a `RhizomeShell` view (the *Begriffs-Wanderung* entry-question) with a follow-link from the Probe Dossier. One-line clarification per item.

### Validation

* [x] `make lint && make test && make fe-check && make codegen && git diff --exit-code` green.
* [x] **Visual regression Playwright.** Snapshots per Pillar for empty / single-probe / multi-probe / refusal states; baseline established this phase, regression-tested in subsequent phases.
* [x] **Manual end-to-end walkthrough.** Open Atmosphäre, click probe-0, see Probe Dossier with Function-Coverage strip + Source-Cards; click "Workbench öffnen" (or click a Function-Tile to set the scope filter); land in Workbench/Aleph with scope pre-set; switch to Episteme — strata stack renders with shared time-axis; switch to Rhizome — *Akteure & Themen* graph renders by default with named nodes; click a node — Methodology detail-panel renders with WP-anchor; switch the entry-question to *Quellen-Resonanz*; remove a Probe via Scope-Bar chip; observe parallel streams collapse correctly; navigate to Reflexion; open a Working Paper; navigate back to Workbench — last Pillar state restored. No dead ends, no orphan controls, no doubled affordances.
* [x] **Deep-link redirect round-trip.** Capture URLs from every Iteration-5-through-7 reachable state (test fixture); apply old URLs; assert each redirects to its new URL with byte-identical semantic state.
* [x] **Coverage audit signed off.** Every current BFF endpoint has a documented consumer; every current dashboard element has a documented new home. PR description includes the full audit table.

### TESTING.md Update

* [x] **Append Phase 122h testing entry to `TESTING.md`.** *What to test:* the three-surface architecture (Atmosphäre / Workbench / Reflexion), three Pillar configurations in the Workbench with distinct geometries, unified ScopeBar with natural-home primary selection (Globe / Dossier / ScopeBar-as-editor), unified FunctionBadge, unified MethodologyBlock with three Pillar-anchored variants, per-state behaviour across all Pillars, deep-link redirect from `/lanes/*` to `/workbench?…`, absence of `/compose` route, presence of all four Rhizome entry-questions (with *Begriffs-Wanderung* showing Phase-124 dependency refusal until Phase 124 lands), parallel-stream rendering across all Pillars without shared y-axes (Brief §1.3 enforcement). *How to test:* run the manual walkthrough above; run the deep-link redirect round-trip; capture visual-regression snapshots for the empty / single / multi / refusal × three-Pillar matrix (12 snapshots); audit the SideRail labels and sections; verify keyboard shortcuts (`1`/`2`/`3` for Pillars, `g a`/`g w`/`g r` for Surfaces); verify the MethodologyBlock renders identical content at all three anchor variants; verify the FunctionBadge renders identically in Probe Dossier, ScopeBar, and Cell headers; verify that no Iteration-5-through-7 BFF endpoint is unconsumed after the rebuild.

### Phase ordering note

Phase 122h sits ahead of Phases 122a, 122d, 123, 123a, 124, 125 in the dashboard work because each of those phases ships frontend deliverables that integrate cleaner with the new surface architecture than with the old. Specifically: 122a's article-function-divergence Cell lives naturally as an Aleph Pillar Cell with a per-Cell Sub-Stratum toggle (not as a separate view-mode in the retired LensBar); 122d's `wayback_revisions[]` panel lives naturally as a Sub-Stratum option in Episteme; 123 inherits the multi-probe parallel-stream pattern from day one rather than requiring a parallel rewrite when 122h lands later; 123a's Coverage Map docks cleanly as an Aleph Cell + Atmosphäre overlay; 124's cross-probe lead-lag becomes a Rhizome entry-question on first ship rather than a panel-on-Dossier retrofit; 125's Composition Workspace is absorbed into Rhizome and never has a separate `/compose` route to retire. 122h has **no backend dependency** — it ships against the existing post-122g BFF surface unchanged. Solo-dev scheduling note: 122h is a coordinated multi-PR rebuild, sized larger than 122a–122g individually but smaller than 122 itself. Three days for the chrome rewrite (SideRail / PillarSwitch / ScopeBar / FunctionBadge / MethodologyBlock); two days each for the three Pillar Shells; one day for the redirect map and route migration; one day for the documentation sweep and ROADMAP amendments. About ten days end-to-end, split across PRs that each ship a self-contained slice (e.g. "SideRail rewrite + redirects" can land before "RhizomeShell with Akteure-Themen default"). Final visual polish ("Claude Design pass") is a follow-up activity, not part of 122h's exit criteria — 122h is correct-and-functional; polish is its own pass against a stable surface.

---

## Phase 122i: Multi-Panel Workbench with Composable Scope Groups [P1] - [x] DONE (2026-05-17, revision shipped)

> **Revision 2026-05-16 → 2026-05-17.** The initial Phase-122i implementation (commits `2282f92` → `c11b92f` → `0796c82`) shipped the ScopeGroup[]/Panel/Window URL state, the BFF multi-scope POST endpoint, the panel-host rendering tree, and the Dossier two-entry-path UI — but manual testing surfaced (a) several critical bugs, (b) a semantic misunderstanding of locked panels, and (c) an architectural gap: the Dossier must be elevated to a first-class top-level surface, with both a per-probe AND a general (cross-probe) free-compose entry. The revision finishes Phase 122i; nothing is rolled back.
>
> **Findings landing in this phase:**
> * **A — Bugs.** A1 Free-Compose `Open Workbench` button disabled when no source selected; A2 Source-selection persists across browser-back; A3 `WorkbenchScopeBar` reflects the focused panel's scope (not legacy flat URL params); A4 `AlephShell.datasetShape` follows the active scope, not the whole probe; A5 **PillarSwitch broken — all tiles link to Aleph, Episteme + Rhizome unreachable**; A6 **CoOccurrence shows ≤ 3 nodes regardless of scope** (root cause TBD, likely a `topN`/`LIMIT` misapplication).
> * **B — Semantic correction.** B1 `locked = scope-only`. CellControls, composition, view, metric, layer, `+Panel`, `+Compare`, `×Remove`, Maximize all remain fully editable on a DF-entry Workbench. Only the ScopeEditor is disabled — the user must return to the Dossier to change which sources are in scope.
> * **C — UX additions.** C1 Panel grid wraps to next row (no horizontal scroll); C2 `×Remove` works on every panel in a multi-panel window including DF-locked (follows B1); C3 Maximize-Mode per panel with tray of minimised siblings; C4 CellControls collapse/expand per panel; C5 Probe Card datasetShape row (Probes / Sources / Articles-in-window / Language / Function-coverage); C6 Cross-Language **soft methodology note in Aleph** (no refusal; Episteme + Rhizome keep the hard 422 refusal for merged cross-language).
> * **D — Composition semantics.** D1 `Merged` actually unions: TimeSeriesCell currently ignores `composition` and fans out per source. Fix the Cell-contract — Cell accepts `composition` prop; merged → ONE chart with multi-source query. D2 Split direction sub-modes: `split-horizontal` (cells side-by-side) and `split-vertical` (cells stacked); URL-state field `splitDirection`. D3 `+Compare` opens a real `ScopeEditor` popover with source-multiselect (+ probe-multiselect in general-free-compose mode); today's seed-only action is replaced.
> * **E — Dossier-Home (Q2).** E1 New route `/dossier` (top-level, no ID); `/dossier/{probeId}` retired entirely; deep-link via `?expand=<probeId>`. E2 `ProbeCard.svelte` — collapsable Probe container refactored from `ProbeDossier.svelte`. E3 `GeneralFreeComposeSection.svelte` — at top of `/dossier`, probe-multiselect × source-multiselect across the whole catalog (AĒR's "most powerful tool"). E4 SideRail gains a `📚 Dossier` anchor → 4 top-level surfaces (Atmosphere · Dossier · Workbench · Reflection). E5 Atmosphere globe-click routes to `/dossier?expand=<probeId>`. **ADR-033 receives an amendment** (Dossier elevated to first-class surface; 3-surface → 4-surface architecture); **ADR-034 receives an amendment** with the composition + locked-scope-only + split-direction clarifications.
> * **F — Pillar inheritance (Q4).** All A–E items apply to Aleph + Episteme + Rhizome equally. Joint-Corpus and Small-Corpus methodology notes stay Episteme-specific (BERTopic-specific); the Cross-Language note becomes pillar-agnostic at the methodology layer (soft in Aleph, hard refusal in Episteme/Rhizome merged).
>
> **Out of scope (Phase 122j).** Methodology-Catalog dual-register coverage of the new composition axes, hardcoded-text audit, and caching/performance review move into Phase 122j (planned after 122i closes).
>
> **Slice plan.** R1 critical bugs (A5, A6, B1, D1) → R2 URL-state foundation (splitDirection, cellControlsCollapsed, maximizedPanelIndex) → R3 affordances + remaining bugs (A1, A2, A3, A4, C1, C2, C4, C6, D2, D3) → R4 Maximize-Mode (C3) → R5 Dossier-Home refactor (E1–E5) → R6 tests + docs + closure. Detailed slice file in the working plan.

*Frontend + BFF expansion that turns the post-122h Workbench from a single-scope tool into a fully composable analytical surface. Phase 122h shipped the three Pillar configurations (Aleph / Episteme / Rhizome) sharing a single global `(probeIds[], sourceIds[])` scope tuple. Phase 122i restructures the Workbench state into a four-level tree — Pillar → Window → Panel → ScopeGroup — so the user can ask the three granularity questions ("complete probe" / "subset of sources" / "single source") and the two composition questions ("merged into one Cell" / "split across N Cells") independently, in any combination, side-by-side. The BFF gains a single new endpoint (`POST /entities/cooccurrence/query`) carrying ScopeGroup-array semantics for Rhizome's CoOccurrence; the methodological gate "cross-language merged topic modeling is unsupported" is enforced as a 422 refusal at the BFF and a RefusalSurface variant in the Frontend. Same-language merged topic modeling is permitted with a Joint-Corpus Methodology Note linking to a new WP-005 §6.2 working-note. Cells inside a Panel have no merge-limit (the entire AĒR same-language corpus can land in one Cell); the only quantitative cap is 100 unique source-IDs / 25 unique probe-IDs per CoOccurrence query. Phase 122i implements ADR-034 and resolves the three structural limits the user identified in the 2026-05-16 design-review session. **No new view modes; no new Cell types; no new ClickHouse migrations.** The Cell-component contract is preserved unchanged — Panels translate ScopeGroups into per-Cell query parameters at the Host level.*

*Authority.* ADR-034 (this phase implements it). ADR-033 (preserved — Pillar concept, three-surface architecture, FunctionBadge, MethodologyBlock primitives). ADR-024 (Language Capability Manifest — the cross-language refusal gate is derived from here). WP-001 §3 (Discourse functions — DF-entrypoint from Probe Dossier). WP-005 §6 (Pillar definitions). WP-005 §6.2 (forthcoming Working-Note on merged-corpus topic modeling — Slice 7 deliverable).

*Scope discipline.* Phase 122i is **structural**, not stylistic. CellControls UX and ScopeEditor popover interaction are the two highest-UX-risk components and ship with a "minimal correct affordance" target in this phase; refinements (drag-to-reorder ScopeGroups, animation polish, Window-tab styling) are explicit follow-up work. Phase 122i is correct-and-functional when the granularity-matrix walkthrough (3 granularities × Merged/Split × 1-Window/Multi-Window × 3 Pillars) succeeds end-to-end with no console errors and no silent data-shape mismatches.

### Architecture

* [x] **ADR-034 lifecycle.** ADR-034 is recorded as Accepted at the start of this phase (already done in 09_architecture_decisions.md as part of Phase 122i preparation). Cross-reference between ADR-033 (Pillars + surface architecture) and ADR-034 (panel composition).
* [x] **Backward-compatibility invariant.** Every Phase-122h URL form continues to load without behavioural change in Phase 122i. The 122h-readable URL `/workbench?probeId=…&sourceId=…&view=…&viewingMode=…` is interpreted as a single-pillar / single-window / single-panel / single-scope-group state and written back in the same form when the state fits. CI test enforces this.

### Frontend — URL state (Slice 1)

* [x] **`Panel`, `ScopeGroup`, `WorkbenchWindow`, `PillarState`, `WorkbenchState` types** in `services/dashboard/src/lib/state/url-internals.ts`. Per-pillar base64url-gzip-JSON encoding. Reader migrates legacy single-scope URLs into a synthesized `WorkbenchState`. Writer prefers the legacy form when the state fits the special case. Active pillar is encoded as `?activePillar=…`. Hard URL-size cap at 8 KiB; over-cap writes trigger a confirmation dialog asking to discard oldest panels of the offending pillar.
* [x] **Unit tests.** Round-trip every realistic state shape (single-pillar single-panel, multi-panel single-window, multi-window per pillar, all three pillars populated). Migration from each Phase-122h URL form. URL-cap edge.

### BFF — CoOccurrence Multi-Scope (Slice 2)

* [x] **`POST /entities/cooccurrence/query`.** New endpoint, OpenAPI-first. Request body: `{scopes: Array<{probeIds: string[], sourceIds: string[]}>, start, end, topN?, language?, ...}`. Response shape identical to today's `GET /entities/cooccurrence`. Validator caps at 100 unique source-IDs and 25 unique probe-IDs per request; over-limit returns `413 scope_limit_exceeded` with offending counts on the error envelope. Cross-language union detection on the request: if the resolved scope spans more than one Language Capability Manifest language, return `422 cross_language_merge_unsupported`. Legacy `GET /entities/cooccurrence?scope=…&scopeId=…` remains for backward compatibility and is the path Phase-122h URLs continue to use.
* [x] **Topic endpoints cross-language refusal.** `GET /topics/distribution` and `GET /topics/evolution` add the same `422 cross_language_merge_unsupported` when their resolved scope (probeIds + sourceIds union) spans multiple languages. Existing single-language behaviour is unchanged.
* [x] **`make codegen`.** OpenAPI changes regenerate `services/bff-api/internal/handler/generated.go`. CI enforces sync.
* [x] **Tests.** Unit tests for the validator (limit, cross-language). Integration test via Testcontainers for the POST endpoint round-trip.

### Frontend — Query layer (Slice 3)

* [x] **`entityCoOccurrenceQuery` multi-scope path.** New code path that calls `POST /entities/cooccurrence/query` when the panel composition requires multi-scope semantics. The legacy `GET` path stays for single-scope panels.
* [x] **`buildQueryFromPanel(panel: Panel): BffRequest`** helper in `services/dashboard/src/lib/workbench/panel-queries.ts` (new). Centralises the Panel-to-BFF-request mapping for all view modes.
* [x] **Cross-language refusal handling.** Topic and CoOccurrence cells render a `RefusalSurface` variant on `422 cross_language_merge_unsupported` with copy: *"Cross-language topic merging not supported — split scope by language or narrow to a single language."*

### Frontend — Shells + new components (Slice 4)

* [x] **`WindowHost.svelte`** (new). Renders the active pillar's Windows as a tab strip in the Workbench header (between `PillarSwitch` and the panel grid). Each tab labels the Window by its first-panel summary; tabs reorderable; `+ Window` button; close-with-undo.
* [x] **`PanelHost.svelte`** (new). Per-panel renderer. Given a Panel, decides on Cell count by composition: `merged` → 1 Cell; `split` → 1 Cell per Source (when single ScopeGroup with multiple sources) or 1 Cell per ScopeGroup (when multiple groups). Panel-header shows the active ScopeGroup summary plus `🔒 Locked to {DF}` eyebrow if locked.
* [x] **`AlephShell` / `EpistemeShell` / `RhizomeShell`** rebased on `WorkbenchState.pillars[activePillar]`. Shells become thin: they render the `WindowHost` and pass the active Window down. The shell-specific affordances (Aleph Dataset-Shape strip, Episteme Auflösung-rail, Rhizome graph-canvas controls) remain in the shells but apply at the Window level (one Dataset-Shape strip per window).
* [x] **`CellControls.svelte` rebound to `focusedPanelIndex`.** Today's global controls become per-focused-panel. New Segmented-Control `Merged · Split` for the composition mode. Read-only mode when `panel.locked === true` with the lock eyebrow.
* [x] **`ScopeEditor.svelte`** (new). Per-panel popover for editing ScopeGroups. Probe-picker (reuses `ProbePicker.svelte`); per-group source-multiselect; `+ Group` / `× Group` buttons. "Save" writes to URL state.
* [x] **Workbench header `Reset {Pillar}` button.** Drops the pillar's URL key and synthesises a fresh default panel on next access.

### Frontend — Dossier two-entry-path (Slice 5)

* [x] **DF-tile sets `locked=true` and `composition='merged'`.** The DF-entry-path opens a Workbench with one Panel, one ScopeGroup over the DF's covered sources, locked. CellControls + ScopeEditor are read-only; the user must navigate back to the Dossier and use the Free-Compose section to recombine.
* [x] **"Compose freely" section in `ProbeDossier.svelte`.** New section at the same visual hierarchy level as the DF-tiles grid. Contains: probe-picker (current probe selectable; multi-probe in future), source-multiselect for the chosen probe(s), "Open Workbench (free compose) →" CTA. Opens a Workbench with one Panel, one ScopeGroup over the chosen scope, `composition='merged'`, `locked=false`. All controls editable.
* [x] **Dossier emic-frame + structural-meta + source-cards remain unchanged.** Phase 122i adds the Free-Compose section without touching the rest.

### Frontend — Methodology surfaces (Slice 6)

* [x] **Joint-Corpus Methodology Note** rendered prominently in Episteme `merged` cells. Copy: *"BERTopic across N sources — topics reflect the joint corpus, not per-source framings. Source-specific framings may be aggregated away; see WP-005 §6.2."* Links to `docs/methodology/en/wp-005-notes/merged-topic-modeling.md` (Slice 7).
* [x] **Small-corpus warning** rendered under each Episteme `split` cell when the group's article count is below the threshold (default 500 documents; configurable in `services/dashboard/src/lib/config/topic-thresholds.ts`, new).
* [x] **Cross-language RefusalSurface** variant. Used by Episteme + Rhizome cells when the BFF returns `422 cross_language_merge_unsupported`.
* [x] **Locked-panel eyebrow** `🔒 Locked to {DF}` rendered on CellControls and ScopeEditor when `panel.locked === true`.

### Tests + docs (Slice 7)

* [x] **Frontend unit tests.** URL round-trip for the WorkbenchState tree, per-pillar encoding, URL-cap handler, migration from each Phase-122h URL form.
* [x] **BFF tests.** `POST /entities/cooccurrence/query` round-trip; limit-exceeded refusal; cross-language refusal; backward compatibility of the legacy `GET` endpoint. Topic-endpoint cross-language refusal.
* [x] **Manual walkthrough.** Granularity matrix: 3 granularities (full probe / source-subset / single source) × 2 compositions (merged / split) × 2 window-modes (single window / multi-window) × 3 Pillars = 36 configurations. Each must render without console errors, produce a coherent Cell, and round-trip its URL.
* [x] **WP-005 §6.2 Working-Note.** New file `docs/methodology/en/wp-005-notes/merged-topic-modeling.md`. Discusses the methodological choice (joint-corpus vs per-source-with-alignment), the BERTopic embedding constraint, and the per-cell Methodology Note's interpretation.
* [x] **Arc42 §8.x update.** New subsection "Workbench State Tree" documenting the Pillar → Window → Panel → ScopeGroup hierarchy and the URL grammar. Cross-reference ADR-034.
* [x] **CLAUDE.md update.** Add `WindowHost`, `PanelHost`, `ScopeEditor` to the dashboard component list. Note the two-entry-path Dossier pattern and the cross-language refusal gate.
* [x] **TESTING.md entry.** Granularity-matrix walkthrough recorded as the Phase 122i manual-test fixture.

### Validation

* [x] `make lint && make test && make fe-check && make codegen && git diff --exit-code` green.
* [x] **CI invariant.** Phase-122h URL bookmarks load without behavioural change. Test fixture covers every Phase-122h reachable URL.
* [x] **CoOccurrence backward compatibility.** Legacy `GET /entities/cooccurrence?scope=…&scopeId=…` continues to serve identical responses for single-scope requests.
* [x] **Manual walkthrough signed off.** All 36 granularity-matrix configurations verified.
* [x] **Working-Note signed off.** WP-005 §6.2 written and linked from the Joint-Corpus Methodology Note.

### Phase ordering note

Phase 122i directly extends Phase 122h. No new infrastructure dependencies; the BFF expansion is one new endpoint plus two new 422 refusal codes on existing endpoints. Phase 122a (per-article discourse function) interacts cleanly: 122a's article-classification Sub-Stratum becomes a per-Panel control; 122i does not block 122a and vice versa. Phase 123 (Probe 1) inherits the multi-panel composition model from day one. Solo-dev scheduling: Slice 1 (URL state) and Slice 2 (BFF) are the foundation and should land in the same PR; Slices 3–4 (Frontend queries + Shell refactor) are the bulk of the work; Slices 5–6 (Dossier two-entry-path + Methodology surfaces) are smaller; Slice 7 (Tests + docs + Working-Note) is the closing PR. Estimated five to seven days end-to-end.

---

## Phase 122j: Methodology Catalog Coverage + Hardcoded-Text Audit + Performance Hardening [P2] - [/] IN-PROGRESS (J1+J2+J3 landed 2026-05-17; lint rule + TTFB baseline pending)

*Three orthogonal but related concerns surface after Phase 122i revision ships. (1) The dual-register YAML content catalogs (`services/bff-api/configs/content/{en,de}/`) predate ADR-034 — every `view_mode`, `metric`, `probe`, `source` entry must be re-audited so its semantic and methodological registers correctly describe what the user sees when composition is Merged / Split-Horizontal / Split-Vertical, and when scope spans multiple sources / languages / probes. A scientific project cannot leave dashboard text misaligned with the underlying methodology — every banner, every cell methodology note, every refusal must be sourced from the catalog (not hardcoded in a `.svelte` component). (2) During Phase 122i revision, banners and labels are added inline in components. Phase 122j sweeps the frontend: every user-visible methodology-bearing string must come from the BFF `/content/{entityType}/{entityId}` endpoint; the audit produces a catalog-content gap-list (strings that exist hardcoded but lack a YAML entry) and fills the gaps. (3) Phase 122i adds N-panel rendering, multi-source merged queries, and the Dossier-Home with all probe cards — combined wire footprint and render cost are unmeasured. Phase 122j profiles, then applies smart caching (TanStack `staleTime` tuning, HTTP `Cache-Control` / `ETag` headers per endpoint, bundle-splitting review post-122i, ServiceWorker if justified). **Hard prerequisite: Phase 122i revision DONE and tested.** Phase 123 (Probe 1) is unblocked by 122j but does not require it — they can ship in parallel.*

### Methodology catalog audit — J1 (DONE 2026-05-17)

* [x] **Inventory every dual-register YAML entry** under `services/bff-api/configs/content/{en,de}/{view_modes,metrics,probes,sources}/`. Coverage matrix at `docs/methodology/en/wp-005-notes/phase-122j-audit/coverage_matrix.csv`, generator at `scripts/audit/methodology_coverage.sh`. 142 files inventoried (70 per locale × 2 + 2 new refusals).
* [x] **Composition section** added to every `view_mode` entry (pillar-aware: Aleph soft cross-language note + WP-004 §3.4 anchor; Episteme joint-corpus + WP-005 §6.2; Rhizome per-language NER caveat). 76 view_mode files updated.
* [x] **Cross-Language note** added to every `metric` entry (16 metric files updated with aggregation-semantics paragraph + reference to view-mode-level refusal vs soft-banner asymmetry).
* [x] **Small-Corpus caveat** referenced in the Episteme composition paragraph; the rendering threshold lives in `src/lib/config/topic-thresholds.ts` and is now also documented in the methodological register.
* [x] **Missing refusal entry** `cross_language_merge_unsupported` (en + de) — the BFF emits `gate=cross_language_merge_unsupported` since Phase 122i, but no dual-register YAML existed. Frontend RefusalSurface now has methodologically grounded copy.
* [/] **Source-set-size methodology** — partially addressed by the composition paragraph; a dedicated section per metric is deferred.

### Hardcoded-text audit — J2 (DONE 2026-05-17)

* [x] **grep-based inventory** of user-visible methodology-bearing strings. Four banner messages identified across TimeSeriesCell, DistributionCell, TopicDistributionCell, TopicEvolutionCell.
* [x] **Catalog consolidation**: methodology-bearing strings extracted to `services/dashboard/src/lib/methodology-copy.ts` (single source of truth, locale-ready signature, stepping-stone toward BFF `/content/methodology_note/*` migration). All four cells now consume the same primitive `MethodologyBanner` + the central copy module; TopicDistributionCell + TopicEvolutionCell no longer carry their own inline aside CSS.
* [ ] **Lint rule** that flags new hardcoded methodology strings in `.svelte` files — deferred (audit gap-list is small enough that human review on PR remains acceptable; revisit if it grows).

### Caching + performance — J3 (DONE 2026-05-17)

* [ ] **Cold/warm cache TTFB baseline** — deferred (needs live profiling against the deployed stack).
* [x] **TanStack `staleTime` tuning**: `contentQuery` bumped 1h → 24h. The BFF catalog is loaded from YAML at startup and only changes on operator deploy; the HTTP Cache-Control middleware reinforces the same TTL at the browser layer.
* [x] **HTTP `Cache-Control` header** for `/api/v1/content/*` — `public, max-age=86400, must-revalidate` via the new `pkg/middleware/cache_control.go` middleware (path-scoped, 2xx-only — refusals stay un-cacheable). ETag deferred (would need response buffering).
* [x] **Bundle-splitting review post-122i**: initial app entry = 3.6 KB gzipped, way under the 80 kB budget. Large viz chunks (Observable Plot, d3) remain lazy-loaded per Cell; Dossier-Home + GeneralFreeComposeSection added no measurable weight to the initial bundle.
* [ ] **SvelteKit prerender for `/reflection/*`** — deferred.

### Validation

* [ ] **Catalog coverage**: every banner / refusal / methodology note in the dashboard has a corresponding YAML entry; `grep` for hardcoded methodology strings returns only legitimate chrome cases.
* [ ] **Performance**: dashboard P95 first-paint ≤ baseline measured at 122j start.
* [ ] **Bundle gate**: still under 80 kB initial gzipped.
* [ ] **No regression of Phase 122i revision behaviour**: full manual walkthrough re-run.

### Phase ordering note

Phase 122j is a hardening phase. No new analytical features, no new pillars, no new surfaces. BFF schema changes only if a content-catalog field is genuinely missing (rare). Solo-dev scheduling: catalog audit (~3 days) → hardcoded-text audit + fix (~2 days) → caching measurement + tuning (~2 days). One closing PR with tests + docs.

---


# Iteration 7 (continued) — Workbench Foundation & Pre-Probe-1 Hardening

*Everything that must be true before the second probe lands, so Probe 1 inherits clean pillars, the correct sentiment backbone, the full analytical pipeline, and the finalised three-surface architecture — with no backfill and no retrofit. Front-loaded deliberately (~7 weeks): the alternative is re-processing two probes' Gold data and re-building shallow cells later. Order is load-bearing: Pillar Sharpening and Configurable Cells form the Workbench foundation that the cell-building phase (122d.0) builds on; the iteration closes with Phase 123a, which collapses the Dossier into a global overlay (four surfaces → three) so the second probe lands into the final surface, not a moving one. **Per-article discourse-function classification (Phase 122a) is deferred** behind source-level/taxonomy validation (WP-001 §5.4.4); Probe 1 inherits the source-level DF lens, and 122a's classifier is multilingual-by-construction so re-opening it later needs no backfill.*

## Phase 122k: Workbench UX Simplification — One Compose Mode, Reusable ScopeEditor, Dossier as Catalog [P1] - [x] DONE (2026-05-18)

### Mid-phase findings (2026-05-18, post-K1 manual test)

The K1 deliverable (ScopeEditor draft + Selection-State + legacy URL removal) surfaced six additional UX directives that fold into the phase rather than spawning a follow-up. The user's decision was to finish ALL of K1/K2/K3/K4 plus these findings before iterating — *"Es ist verwirrend, wenn noch nicht alles implementiert ist wie es später sein wird."* Findings:

* [x] **F1 — `Compose across probes` section in `/dossier` retired.** No in-section probe/source picker. Replaced by a single `[Open Workbench]` button (no parenthetical subtitle). Click opens the ScopeEditor; if `url.selectedProbes` is non-empty the editor seeds those probes into ScopeGroup 1. Selection itself happens via Atmos SHIFT-click or the Probe-Filter Modal (K3). `GeneralFreeComposeSection.svelte` is deleted in K2 (already in the K2 plan) — this finding accelerates that by making the new banner the only entry from the Dossier.
* [x] **F2 — SideRail Workbench anchor always opens the ScopeEditor.** The "Pick a probe first — click a probe glyph on the Atmosphere, or use the highlighted Probe Picker in the side rail." copy is retired. Clicking Workbench in the SideRail now lands on `/workbench` and the page **auto-opens the ScopeEditor** in create-mode when no pillar state exists. Seeded from `url.selectedProbes` when present; otherwise opens with one empty ScopeGroup. Cancel returns the user to an empty Workbench (with a re-open affordance); Apply creates the first Panel.
* [x] **F3 — `+ Panel` opens the ScopeEditor, never clones.** Today's `+Panel` duplicates the focused Panel's scope. New behaviour: `+Panel` opens the ScopeEditor in create-mode; Apply appends a NEW Panel with the user-configured scope. Default seed is empty (the user is asked what to put in the new panel).
* [x] **F4 — WorkbenchScopeBar fields reflect the focused Panel's scope, not the probe-wide aggregate.** "Probes / Sources / Articles in window / Language / Function coverage" must trace to the active Panel's resolved scope so the user can read off how the panel is configured alongside the PanelControls. Verify in K1.1; today's behaviour is partially correct (the bar already reads from `activePanelInfo` when present) but the dataset-shape strip (`WorkbenchDatasetShape`) was inheriting probe-wide counts in some paths — make sure both surfaces converge.
* [x] **F5 — Time window becomes per-Panel state.** Today `url.from` / `url.to` are global. New behaviour: each Panel carries its own `windowStart` / `windowEnd` (and inherits the global default when absent). The PanelControls bar (renamed in F6) owns the date inputs; the WorkbenchScopeBar surfaces the focused Panel's window for read-only reference. Schema: `Panel.windowStart?: string`, `Panel.windowEnd?: string` round-tripped via the existing pillar encoder.
* [x] **F6 — `CellControls` renamed to `PanelControls`.** Mechanical rename. The control bar acts on Panel state, not Cell state — the new name matches the model.

### Implementation plan (revised — single drive-through, no mid-phase user iteration)

The phase now ships all four slices PLUS findings in one sweep. The order below reflects dependency edges:

* **K1.1** — F1 + F2 + F3 wiring (ScopeEditor accepts a create-mode without an existing Panel; Workbench page auto-opens it on empty pillar state; WindowHost's `+Panel` button opens it).
* **K1.2** — F6 (mechanical rename `CellControls` → `PanelControls`) + F5 (per-Panel time window in the schema + UI). F4 verification rolls into K1.2 since it touches the same components.
* **K2** — Dossier-as-Catalog refactor as originally specified (DF-Cards as containers, Metadata-Coverage Modal, Source-Card pill provisional, FreeCompose/ProbeDossier deletion, etc.).
* **K3** — Atmos Multi-Select + Probe-Filter Modal + remaining Workbench-invitation CTAs.
* **K4** — Polish + Documentation Cleanup + Bug fixes carried over.

Manual testing is held until K4 ships. After that, iteration is open.

### Original Plan (pre-K1 manual test — shipped as part of the DONE phase)

*Phase 122k is the UX-simplification follow-up to Phase 122i. The 122i revision shipped a working but **structurally over-decomposed** entry-path system: three different routes into the Workbench (per-probe Free-Compose inside each Probe-Card, General Free-Compose at the top of `/dossier`, DF-Tile click on a Probe-Card), each with subtly different scope-seeding semantics, plus a ScopeEditor popover whose interaction model is incomplete (closes too eagerly on focus-loss, hidden behind a `+Compare` button, doesn't surface the four scope-shapes the user actually wants to configure). 122k consolidates: **one Compose-Mode** (the Workbench), **one configuration tool** (a hardened, re-openable ScopeEditor that handles every scope shape), **one Dossier purpose** (catalog of AĒR's data, not an entry point to the analysis surface). The Workbench-internal architecture (Pillar / Window / Panel / ScopeGroup tree, Cell-rendering, Composition semantics) stays exactly as 122i shipped — 122k touches entry-paths and the configuration surface, not the analysis machinery.*

*Why now.* Manual testing of 122i revealed that the multi-entry-path pattern adds cognitive load without giving the user real choice — every entry-path produced a single-panel Workbench over a user-chosen scope, just with different framing copy around it. The actual analytical question — *"which probes + which sources + which DF restriction"* — was scattered across three different UI components (`FreeComposeSection` per probe, `GeneralFreeComposeSection`, DF-Tile-as-Workbench-Entry) instead of consolidated in one tool. The 122k consolidation also retires the Phase-122h legacy URL grammar entirely — the dashboard is pre-deployment, no bookmarks exist to preserve — ending the dual-form Reader/Writer that has accumulated complexity since Phase 122h and simplifying the URL state code significantly.

*Scope discipline.* 122k is **structural simplification**, not new functionality. No new view-modes, no new metrics, no schema changes, no BFF endpoints. The deliverables are: a hardened ScopeEditor, a thinner Dossier, a clearer Atmos → Dossier → Workbench flow with visible invitations between surfaces, and a documentation cleanup that **deletes obsolete content** rather than layering revisions on top. ADR-033 and ADR-034 are rewritten as if 122k was the original architecture — the historical evolution (3-surface → 4-surface, locked-panel semantics correction, free-compose dual-entry, ScopeEditor seed-action → real popover) lives in git history, not in the ADR text.

### Architecture

* [x] **ADR-033 full rewrite.** The four-surface architecture (Atmosphere · Dossier · Workbench · Reflection) is documented as the baseline; the historical three-surface section is removed. Dossier is documented as catalog-only (no entry-path responsibilities). ScopeEditor is documented as the Workbench's single configuration tool.
* [x] **ADR-034 full rewrite.** The Pillar → Window → Panel → ScopeGroup state tree stays. Entry-path documentation is rewritten: a single ScopeEditor produces every Panel's scope; per-probe / DF-tile entry paths are removed from the ADR. The 2026-05-17 revision-block is deleted (content folded into the body of the rewritten ADR).
* [x] **Selection-State as URL grammar.** New URL parameter `?selectedProbes=<comma-separated-ids>` carries the Atmos / Modal-driven probe selection. Consumed by Dossier (filter + auto-expand) and Workbench (seeds the ScopeEditor's first ScopeGroup). Lives in `services/dashboard/src/lib/state/url-internals.ts` as a top-level `UrlState` field (parallel to `activePillar`, `aleph`, `episteme`, `rhizome`).
* [x] **Legacy URL grammar removed entirely.** The Phase-122h flat reader (`?probeId=…&sourceId=…&viewingMode=…&view=…`) is deleted from `url-internals.ts`. The "writer prefers legacy form when state collapses" branch is deleted. The `Workbench-from-flat-URL` test fixture is deleted. The "Backward-compatibility invariant" bullet in Phase 122i is marked obsolete (pre-deployment reset 2026-05-18). **One canonical URL grammar**: pillar-state base64 for Workbench, `?expand=` / `?selectedProbes=` for Dossier.

### K1 — ScopeEditor rebuild + Selection-State foundation

* [x] **ScopeEditor as central re-openable modal.** Replaces today's `+Compare` popover. Re-openable for any Panel via a `⚙ Edit scope` button on the Panel header. Explicit `[Apply]` / `[Cancel]` buttons — the modal **does not close on backdrop-click or focus-loss**. Esc = Cancel. Configuration is committed to the Panel state only on Apply.
* [x] **All four scope shapes supported elegantly.**
  - Single probe × n sources
  - n–N probes × n–n sources per probe
  - 1 probe + DF-lock × 1–n sources within that DF
  - n–N probes + DF-lock × 1–n sources within that DF, per probe
* [x] **ScopeGroups visible as first-class.** The Panel's `ScopeGroup[]` is rendered as a stack of group-cards in the editor (Group 1, Group 2, …), with `[+ Add Group]` / `[× Remove]`. The previous "+Compare seeds a new ScopeGroup behind a popover" pattern is deleted. Multi-group composition is now explicit and discoverable.
* [x] **DF-lock as per-group toggle.** Each ScopeGroup card has a `Restrict to discourse function: [None ▾]` dropdown. When set to a DF, the source-checkbox list dims sources that are not classified under that DF in the chosen probe(s), with a tooltip *"Not classified as [DF] in this probe"*. Dimmed sources stay visible (not hidden) — transparency over compactness, consistent with the Negative-Space pattern.
* [x] **Per-probe source filtering visualised.** When multiple probes are in a group, sources are sectioned by probe with the probe-name as a header. Select-all / select-none per probe is a one-click affordance. Country / region of each probe is shown next to the probe-header for geographic orientation.
* [x] **Selection-State seeding.** When the user opens the ScopeEditor with a non-empty `?selectedProbes=…`, the first ScopeGroup is pre-populated with those probes (all sources of each probe pre-selected, no DF-lock). The user refines from there. Same seeding applies when the user clicks `→ Analyse in Workbench` from a single Probe-Card (one-probe seed) or `Open Workbench` from the Atmos Selection-Bar.
* [x] **Tests.** Unit tests for: scope-shape conversion to URL-state, DF-lock filtering logic, Selection-State seeding into the editor, Apply-vs-Cancel commit semantics, ScopeGroup add / remove pure mutators.

### K2 — Dossier-as-Catalog refactor + Metadata-Coverage Modal

* [x] **`FreeComposeSection.svelte` deleted.** Per-probe free-compose surface retired entirely. Its scope-building responsibility is absorbed by the ScopeEditor in the Workbench.
* [x] **`GeneralFreeComposeSection.svelte` deleted.** Cross-probe free-compose surface retired. Its responsibility is also absorbed by the ScopeEditor.
* [x] **`ProbeSourcePicker.svelte` deleted.** Was a helper for `GeneralFreeComposeSection`.
* [x] **DF-Cards become expandable containers.** The Probe-Card's "Discourse Functions" tile-grid is replaced: each DF-Card is now a collapsable container whose body holds the Source-Cards of that DF. The previous flat "Sources" list at the bottom of the Probe-Card is removed — sources live inside their primary DF-container.
* [x] **DF-Card click no longer enters the Workbench.** The DF-Tile-as-Workbench-Entry pattern is deleted. DF-Card click expands / collapses its source-container.
* [x] **Source-Card Primary-Function pill kept as provisional.** Until Phase 122a.1 ships, the source-level DF classification stays visible on each Source-Card with a small `(provisional)` hint. Phase 122a.1's *Spannweite* (per-article DF distribution) replaces this when shipped — same component slot, hot-swappable data.
* [x] **Source-Card metadata-coverage glance indicator.** Each Source-Card gains a one-line summary: `Metadata coverage: N/22 fields` (or an inline 5-dot indicator). No click required — scrollable scan-level information.
* [x] **Metadata Coverage Modal.** `[View metadata coverage]` button at the Probe-Card header opens a modal containing today's `MetadataCoveragePanel` matrix, with rows **grouped by DF** (DF-section header + the source-rows of that DF). Esc closes. Modal replaces the previous inline mounting of `MetadataCoveragePanel` at the bottom of the Probe-Card.
* [x] **`ProbeDossier.svelte` deleted.** All remaining logic absorbed into `ProbeCard.svelte` (introduced in 122i revision).
* [x] **`/dossier` route invariant.** Three URL forms: `/dossier` (all collapsed), `/dossier?expand=<probeId>` (one probe expanded), `/dossier?selectedProbes=<a,b,c>` (filtered + expanded). Legacy `/dossier/{probeId}` 308-redirect kept (it exists since 122i revision and is harmless).

### K3 — Atmos Multi-Select Reconciliation + Probe-Filter Modal + Workbench Invitations

* [x] **Atmos single-click semantics.** Click on a probe glyph (no modifier) → navigate to `/dossier?expand=<probeId>`. Does NOT touch `selectedProbes`. Pre-existing behaviour preserved.
* [x] **Atmos SHIFT-click semantics.** SHIFT-click on a probe glyph → toggle membership in `?selectedProbes=…`. Stays on Atmos. Does NOT navigate.
* [x] **Atmos Selection-Bar.** Floating bar at the bottom of the Atmos view, conditional on `selectedProbes.length > 0`. Shape: *"N probes selected · [View in Dossier] [Open Workbench →] [Clear]"*. The first invitation surface; makes the SHIFT-click feature visible.
* [x] **Probe-Filter Modal.** New component `ProbeFilterModal.svelte`. Opened from (a) a sidebar affordance and (b) a button in the Dossier top-banner. Modal content: search field, region-grouped probe rows (large, generous spacing, AĒR-türkis accent), checkbox per row, footer with `[Apply Selection]` / `[Clear all]` / `[Cancel]`. Esc = Cancel. Enter = Apply. The previous sidebar-embedded probe selector is deleted.
* [x] **Sidebar Probe-Filter affordance.** A button (separated from the four nav-entries — visually distinct, lives at the bottom of the SideRail or as a dedicated affordance-block) opens the Probe-Filter Modal. Badge shows current selection count (e.g. `▤ Probes · 3`).
* [x] **Dossier top-banner.** Persistent strip at the top of `/dossier`, two states:
  - Empty selection: *"Browse AĒR's atmospheric record. To analyse, configure a scope in the Workbench →"* + `[Open Workbench]` (opens empty ScopeEditor).
  - Selection ≥ 1: *"N probes selected — bring them into the Workbench →"* + `[Open Workbench]` (seeds ScopeEditor) + `[Clear]`.
* [x] **Per-ProbeCard "Analyse" button.** Header of each expanded Probe-Card carries a compact `[→ Analyse in Workbench]` button. Click → opens ScopeEditor with **just this one probe** as seed (bypasses `selectedProbes` entirely — direct drill-flow).
* [x] **Workbench from sidebar with empty selection.** Click sidebar Workbench → opens ScopeEditor for the first Panel of a fresh Workbench (no scope pre-filled). User configures from scratch.
* [x] **Workbench from sidebar with non-empty selection.** Click sidebar Workbench → opens ScopeEditor seeded with `selectedProbes` (all sources of each selected probe pre-checked, no DF-lock).
* [x] **Old `?probeId=` entry-point removed.** The Phase-122h URL pattern that enabled the sidebar Workbench item is deleted. Sidebar Workbench is always clickable; behaviour is governed by `selectedProbes` (seeded) or empty (fresh ScopeEditor).

### K4 — Polish + Documentation Cleanup + Bug Fixes

* [x] **Sidebar Dossier icon.** Replace `📚` with `❒` (Lower-Right Shadowed White Square) — monochrome, AĒR-türkis tintable via CSS `color: var(--color-accent)`. SVG-icon-stack migration (Lucide / Heroicons) deferred to a separate polish phase.
* [x] **Bug-fix carry-over from 122i testing.** Any UX regression flagged in TESTING.md from 122i manual testing that wasn't methodologically gated (those were 122j).
* [x] **ADR-033 + ADR-034 full rewrite (delete-not-amend).** The rewritten ADRs read as if 122k was the original architecture: 4 surfaces (no 3 → 4 evolution narrative), Dossier as catalog (no FreeCompose surface mentioned), ScopeEditor as the single configuration tool (no per-probe / DF-tile entry-path mentioned). Per-amendment historical blocks deleted. History stays in git.
* [x] **`docs/arc42/08_concepts.md` §8.20 (Workbench State Tree) rewrite.** Same delete-not-amend pattern: ScopeEditor centrality documented as baseline. The 2026-05-17 R1–R6 revision-block is removed.
* [x] **`CLAUDE.md` rewrite.** Heavy edit: remove all mentions of `FreeComposeSection`, `GeneralFreeComposeSection`, `ProbeSourcePicker`, `ProbeDossier`. Update Workbench component catalog to: `ScopeEditor` (central), `ProbeFilterModal`, `MetadataCoverageModal`, `ProbeCard`. Update routes section: `/dossier?selectedProbes=` documented. Drop the 122i revision-narrative paragraph (folded into the canonical description).
* [x] **TESTING.md fresh walkthrough.** Replace 122i's 21-section walkthrough with a new manual-test fixture aligned to the 122k UX: Atmos single + SHIFT click, Probe-Filter Modal, Dossier-as-catalog with DF-containers, Metadata-Coverage Modal, ScopeEditor full-shape walkthrough, three Workbench-invitation CTAs, legacy-URL-rejection test.
* [x] **`ROADMAP.md` Phase 122k → [x] DONE.** Phase ordering note added: 122k unblocks the post-122a Spannweite-on-source-card UI, since the new ProbeCard layout is where the per-article DF distribution renders.
* [x] **Legacy file deletion checklist.** `FreeComposeSection.svelte`, `GeneralFreeComposeSection.svelte`, `ProbeSourcePicker.svelte`, `ProbeDossier.svelte` removed from the tree. Their tests removed. No re-exports left behind.

### Validation

* [x] **Empty-start flow.** Sidebar Workbench click (no selection, no probe in URL) → ScopeEditor opens with one empty ScopeGroup. User can build a scope from scratch.
* [x] **Atmos single-click flow.** Click on a probe glyph → `/dossier?expand=<probeId>`. Probe is expanded.
* [x] **Atmos SHIFT-click flow.** SHIFT-click on three probe glyphs → all three appear in `selectedProbes`; Selection-Bar at the bottom shows "3 probes selected". URL is `?selectedProbes=a,b,c`. No navigation has happened.
* [x] **Selection consumed by Dossier.** From Atmos with 3 selected, click `View in Dossier` → Dossier opens filtered to those 3 probes, all expanded.
* [x] **Selection consumed by Workbench.** From Atmos with 3 selected, click `Open Workbench` → ScopeEditor opens with 3 probes pre-seeded in Group 1.
* [x] **Probe-Filter Modal flow.** From sidebar or Dossier top-banner, open modal → search → check probes → Apply → selection updates, Dossier filter applies if context is Dossier.
* [x] **ScopeEditor scope-shape walkthrough.** Configure each of the 4 supported shapes — all render correctly in the resulting Panel; URL state round-trips on reload.
* [x] **DF-lock visualisation.** Lock a ScopeGroup to "Cohesion & Identity" — sources not classified as CI in the chosen probes are dimmed with the tooltip.
* [x] **DF-Card expansion in Dossier.** Click a DF-Card → expands inline showing the source-cards of that DF. No Workbench-navigation happens.
* [x] **Metadata-Coverage Modal.** Click `View metadata coverage` on a Probe-Card → modal opens with the matrix, rows grouped by DF. Esc closes. No data-fetch regression vs. the previous inline panel.
* [x] **Source-Card glance indicator.** Each Source-Card shows a one-line metadata-coverage summary inline.
* [x] **Per-ProbeCard Analyse flow.** Click `→ Analyse in Workbench` on a Probe-Card → ScopeEditor opens with just that one probe seeded.
* [x] **Three CTA invitations visible.** Selection-Bar on Atmos · Dossier top-banner · per-ProbeCard Analyse button — all three present and functional.
* [x] **Legacy URL forms inert.** `/workbench?probeId=…` no longer constructs a populated Workbench (lands on empty ScopeEditor). `/dossier/<probeId>` 308-redirects to `/dossier?expand=<probeId>` (kept).
* [x] **Documentation reads as if 122k was the baseline.** ADR-033, ADR-034, §8.20, CLAUDE.md contain no "in 122i this was X; in 122k it became Y" narratives. History is in git.
* [x] **`make lint && make test && make fe-check && make codegen && git diff --exit-code` green.**

## Phase 130: Pillar Identity Sharpening [P1] - [x] DONE

*The three pillars are AĒR's conceptual core (the app name ἀήρ = air/atmosphere; the pillars are weather/climate/currents). The intent is already in `registry.ts` but the presentation→pillar assignment leaks: `time_series` (inherently diachronic) sits in Aleph, while Episteme — the temporal pillar — has no time-series. This phase makes the pillar identities crisp and scientifically communicable, introduces the metric→presentation compatibility map so metrics auto-sort into the correct pillars, and removes the Rhizome entry-question layer so all three pillars behave uniformly.*

**Grounding.** Read first: `src/lib/viewmodes/registry.ts` (`PRESENTATIONS` + `PILLAR_DEFINITIONS`), the three `*Shell.svelte` (Aleph/Episteme/Rhizome), `PanelControls.svelte`, ADR-033/034 (current surface + state-tree), ADR-035 (written here). Preserve: composition modes (split/merged/overlay), the `?{aleph,episteme,rhizome}=<base64url-json>` URL grammar, the multi-panel pillar-state path (ADR-034), every working cell. Verify-first: list each pillar's current presentations and confirm the multi-panel state path renders Rhizome cells before deleting the entry-questions.

### Frontend
* [x] **Fix presentation→pillar assignment** in `PILLAR_DEFINITIONS`: Aleph → `distribution`, `topic_distribution`; Episteme → `time_series`, `topic_evolution`; Rhizome → `cooccurrence_network` (+ relational cells from 124/125). Two moves: `time_series` Aleph→Episteme; `topic_distribution` Episteme→Aleph.
* [x] **Metric→presentation compatibility map.** New SoT `src/lib/viewmodes/metric-presentation.ts`; the catalog (`metrics × presentations`) is filtered through it in `PanelControls` (metric list filtered by active view + metric reconciled on view-change). Scalar metrics → distribution + time_series; cyclic (`publication_hour`/`publication_weekday`) → distribution only; `temporal_distribution` → time_series only; `entity_cooccurrence` → cooccurrence_network.
* [x] **Remove the Rhizome entry-question layer.** `RhizomeShell` rewritten to the universal panels+cells model (pillar-state→WindowHost path + legacy single-Cell fallback), matching Aleph/Episteme. `RhizomeView` URL enum retired; relational cells are now ordinary cell choices. `lockedView` prop removed from `PanelControls`.
* [x] **Default-presentation adjustment.** Registry-wide + Aleph default is now `distribution`; pillar defaults follow `PILLAR_DEFINITIONS[*].presentations[0]`. The `buildPillarUrl`/`buildPanelFromScopes` builders default the view via `defaultViewModeForPillar(pillar)`; callers (`WindowHost`, `/workbench`) pass the pillar-correct view so freshly composed panels render the pillar's identity cell.
* [x] **merged-cross-probe guard.** `shouldRefuseMergedCrossProbe` (`panel-queries.ts`) + `isPureCountMetric` (`metric-presentation.ts`); refused renders surface the standard `RefusalSurface` (`merged_cross_probe_unsupported`, content in `configs/content/{en,de}/refusals/`) via `PanelHost`. `split`/`overlay` cross-probe remain allowed; pure-count metrics exempt.

### Documentation
* [x] **ADR-035 — Pillar Identity.** Weather/climate/currents metaphor + etymology (Borges/Foucault/Deleuze) + "pillar follows presentation" + the metric→presentation principle + the merged-cross-probe guard + the entry-question removal rationale.
* [x] **CLAUDE.md.** Pillar description sharpened; metric→presentation map documented as the SoT for pillar placement; Rhizome noted as panels+cells like the others. (CLAUDE.md was authored ahead of the code during the Open-Phases rewrite; verified to match this implementation exactly.)

### Validation
* [x] A scalar metric is reachable as a distribution (Aleph) and a time-series (Episteme); `publication_hour` is offered only as a distribution; Rhizome shows panels+cells with no entry-question selector and no functionality lost; merged cross-probe on a scaled metric refuses. Covered by `tests/unit/metric-presentation.test.ts`, `tests/unit/viewmodes-registry.test.ts`, `tests/unit/panel-queries.test.ts` (`shouldRefuseMergedCrossProbe`).

---

## Phase 131: Configurable Cells & Publication-Ready Presentations [P1] - [x] DONE

*The foundation for everything analytical that follows. Today's cells are often shallow (2–3 points per axis) and not configurable. This phase makes the existing presentations deep, configurable, and publication-ready, and introduces visual-channel binding (a visual channel — size/colour/position/edge-weight — bound to a chosen dimension). This is what turns presentation into analysis (Kriesel: it is not "network vs chart", it is "visual channels bound to real data"). Built before the cell-building phases (122d.0, 122a.1) so those build on the configurable framework rather than being retrofitted.*

**Grounding.** Read first: `src/lib/viewmodes/registry.ts`, `src/lib/components/viewmodes/*Cell.svelte`, `PanelControls.svelte`, `services/bff-api/configs/content/{en,de}/view_modes/` (Dual-Register explanations), the BFF metric/topic/entity endpoints. Preserve: the cell registry contract, the Dual-Register content-catalog mechanism, bundle-size budgets (Brief §7). Verify-first: confirm Phase 130 landed (this builds on the corrected pillar/cell model).

### Frontend
* [x] **Cell configurability framework** — shared per-cell config (bins, top-N, band, visual-channel binding) surfaced through `PanelControls.svelte`; cells declare their configurable parameters via `PresentationDefinition.configurableParams`. Config persists in `Panel.bins`/`channels`/`showBand` (URL keys `bn`/`ch`/`sb`).
* [x] **Depth** — bins regler on distribution; top-N on the network; real ±1σ uncertainty band on time-series (BFF `includeStddev`); real axes/labels on the scatter.
* [x] **Visual-channel binding** — `CellChannelBinding` binds size/colour to `cooccurrence_network` and x/y/size/colour to the new `metric_scatter` cell (BFF `GET /metrics/scatter`). (Metadata dimensions arrive after Phase 133.)
* [x] **Publication-ready output** — clean labels/legends/titles + `CellExport` (PNG/SVG/CSV/JSON, native serialisation, no new dep).
* [x] **"Always explained" foundation** — every cell renders a composed "how to read" note (`HowToRead` + pure `how-to-read.ts`): per-presentation template from the content-catalog `view_mode/howto_<presentation>` Dual-Register entry + live-config building blocks.

### Backend (BFF)
* [x] Added `GET /metrics/scatter` (paired-metric pivot, chaining-friendly) + `stddev`/`includeStddev` on `/metrics`; OpenAPI + `make codegen` + Testcontainers + handler unit tests. Distribution `bins` / cooccurrence `topN` were already deep enough — wired into the UI.

### Documentation
* [x] Arc42 §8.21 "Configurable Cells & Visual-Channel Binding"; CLAUDE.md note; content-catalog `howto_<presentation>` convention documented.

### Validation
* [x] A cell renders full-depth data; channel binding works on the network cell and the scatter; every cell shows a "how to read" note; export produces a publication-quality artefact. Composition-mode bugfixes folded in (overlay gated to time-series; split layout/direction fixed for the probe-scope fan-out).

---


### Phase ordering note

Phase 122k sits between 122j (methodology hardening) and 122a (per-article DF classification). 122a's Spannweite-on-Source-Card UI consumes the new ProbeCard layout shipped in K2 — implementing 122a before 122k would force a re-layout of the source-card during 122k. Phase 123 (Probe 1) inherits the clean URL grammar from K1 with no migration debt: when French sources enter the catalog, the same Probe-Filter Modal lists them region-grouped under "Western Europe" alongside the German probe. Solo-dev scheduling: K1 is the largest single piece (the ScopeEditor is the central UX deliverable, multiple iterations expected — *"Der ScopeEditor ist ein primärer Aspekt der Applikation und muss hervorragend funktionieren"*, design discussion 2026-05-18); K2 is mechanical refactor; K3 wires the three CTAs and the Probe-Filter Modal; K4 is the documentation rewrite — itself substantial because the delete-not-layer mandate means three ADRs / one CLAUDE.md / one TESTING.md get fresh prose, not patches. Sized at ~2–3 weeks solo, ScopeEditor iteration count being the main variable.

---

## Phase 131a: Co-Occurrence Quality & Network Scope [P1] - [x] DONE

*Surfaced by Phase-131 manual testing. The Rhizome co-occurrence network has two linked data-quality gaps that need a worker fix + a Gold reprocess, plus the network-specific per-source / overlay affordances that the other pillars already have. Grouped because they share the same reprocess.*

**Grounding.** Read first: the corpus sweep driver `internal/corpus.py` (`fetch_entities_for_window` + `run_corpus_sweep` + the window cadence/config) — the `EntityCoOccurrenceExtractor` itself (`internal/extractors/entity_cooccurrence.py`) is correct, so the bug is in the **caller's window/batch logic**, not the pair enumeration. Then the worker NER extractor (boilerplate filtering), `CoOccurrenceNetworkCell.svelte`, the BFF `/entities/cooccurrence` GET + POST handlers.

**Diagnosis already done (Phase-131 testing, 2026-05-25):** the gap is corpus-wide, not sparse data. Across the Gold corpus:

| stage | articles |
|---|---|
| `metrics` | 847 |
| `entities` (NER) | 771 (tagesschau 721, bundesregierung 50) |
| `topic_assignments` | 844 |
| **`entity_cooccurrences`** | **11** (all tagesschau; bundesregierung 0) |

771 articles have ≥1 entity (bundesregierung averages ~40 entities/article) yet only 11 produced co-occurrence rows. So `corpus.py`'s sweep is processing a tiny fraction of articles — likely the rolling `window_seconds` only covers a sliver of the corpus, the per-(source,window) batch misses most articles, or the sweep ran once over a narrow window. Fix the windowing so every entity-bearing article is swept.

### Co-occurrence pipeline gap (BUG 1.5)
* [x] **Fix the corpus sweep** (`internal/corpus.py`) so co-occurrences are emitted for **every** article with ≥2 entities across **every** source — target ≈771 articles, not 11. Reprocess Gold so `entity_cooccurrences` is populated corpus-wide.
* [x] **Per-source comparison must work** in split (it already fans out; the data was just empty). Add a clear "no co-occurrences for this source in window" hint distinguishing *sparse data* from *pipeline gap*.

### NER garbage nodes (BUG 1.1)
* [x] **Worker NER filter** drops self-referential / boilerplate entities at extraction time: outlet names ("ARD-aktuell"), domains ("tagesschau.de"), nav/section labels ("Video Tagesschau"). Configurable blocklist + heuristic (entity text == source/domain). Reprocess Gold. (Reprocess is feasible on a metered connection — BERT/BERTopic models are baked into the worker image and Docker-layer-cached; only the re-crawl fetches ~7 days/source.)

### Network scope / overlay (BUG 1.5)
* [x] **Merged-provenance note** — show what is merged (like the other pillars' methodology banner).
* [x] **Source-coloured overlay** — one merged graph with nodes/edges coloured by originating source + an auto-assigned source palette + legend. Needs per-source **edge** provenance in the BFF co-occurrence response (node `presence` already exists; edges do not). OpenAPI + `make codegen`.
* [x] **Kriesel-scale (stretch / may defer)** — thousands of nodes need a non-d3-force-SVG renderer (canvas/WebGL); evaluate whether this belongs here or in a later Quick-Compose phase.

### Validation
* [x] `entity_cooccurrences` populated for all sources; bundesregierung network renders; no outlet/domain garbage nodes; per-source split + source-coloured overlay work; `make test` + `make lint` green.


## Phase 122d.0: Silent-Edit Observability — Edit Beobachtbarkeit [P1] - [x] DONE

*Phase 122d is completely reframed: not a worker sidecar feeding a teaching cell, but a first-class analytical stratum in the Workbench, alongside sentiment/entity/topic, cutting across all three pillars. Silent edits — publishers revising articles without notice — are one of the strongest signals of platform-mediated discourse manipulation (WP-003 §5). This sub-phase makes edit activity observable: which source/discourse-function/probe edits how often, when. The Internet Archive is the independent third-party witness.*

*Phase 131a flagged a related UX/conceptual artefact that **must land elegantly in this phase**, not as a separate fix: the crawler's `time_window_days` filters discovery (sitemap-`<lastmod>` / RSS `pubDate`), but an article's stored `published_date` can be far older when the publisher re-listed an old article whose sitemap-`<lastmod>` was recently bumped. That mechanism is GENAU the silent-edit signal this phase observes — a 10-month-old article reappearing with a fresh `<lastmod>` is a re-publication / revision event. The dossier already exposes this as the "Total vs In Window" diagnostic; 122d.0 must (a) document the mechanism as a named first-class concept (e.g. "republication trigger"), (b) wire the discovery-trigger metadata into `article_revisions` so the source of each revision (Wayback CDX vs publisher re-list vs both) is observable, and (c) reconcile the UX so the dossier numbers no longer confuse anyone: a re-listed old article is correctly framed as a Silent-Edit signal, not as "stale article slipping past the 7-day window". This avoids treating the Phase-131a artefact as a bug to patch over.*

**Grounding.** Read first: `services/analysis-worker/internal/adapters/web.py` (`harmonize()`) + `web_meta.py` (WebMeta tiers), `internal/adapters/registry.py`, the ClickHouse migrations dir, the BFF article/silver path specs, `L5EvidenceReader.svelte`, the Phase-131 configurable-cell framework, `crawlers/web-crawler/probes/probe0/sources.yaml` (the `time_window_days` knob + its comment-block rationale), `services/bff-api/internal/storage/dossier_store.go::fetchSourceCounts` (the existing in-window vs whole-dataset split). Preserve: the SilverEnvelope contract, the no-DLQ-for-extractor-failures rule, Bronze-stores-raw-HTML (ADR-028). Verify-first: confirm the WebMeta provenance shape + the cell-registration pattern from 131.

### Worker
* [x] **`internal/wayback/`** — sync `requests`-backed CDX client (token-bucket ~5 req/s/host, operator-tunable), typed `CDXResult` with `status ∈ {ok, no_snapshots, failed, skipped, disabled}`. Runs in the harmoniser thread; **a Wayback timeout / network error / rate-limit denial NEVER produces a DLQ event** (enforced by the catch-all in `WebAdapter._resolve_wayback`).
* [x] **Postgres cache** `wayback_cdx_cache(canonical_url PK, fetched_at, status, revisions_jsonb)` — migration `000019`, point-cache with operator-managed TTL `WAYBACK_CDX_CACHE_TTL_HOURS` (default 24 h).
* [x] **WebMeta extension** — `wayback_revisions[]` + `wayback_lookup_status` (Tier-C provenance, allowed-value set in `ALLOWED_WAYBACK_LOOKUP_STATUSES`); CDX call is the last step of `harmonize()`.
* [x] **Config** — `WAYBACK_CDX_{ENABLED,BASE_URL,TIMEOUT_SECONDS,RATE_LIMIT_PER_SECOND,CACHE_TTL_HOURS}` + `REPUBLICATION_TRIGGER_MIN_DELTA_DAYS` + `.env.example`.

### Gold + BFF
* [x] **`aer_gold.article_revisions`** (`article_id, source, discourse_function, snapshot_at, content_hash, prev_content_hash, revision_index, time_since_prev_hours, revision_trigger, ingestion_version`) — migration `000023`. `revision_trigger ∈ {cdx_snapshot, republication_trigger, unknown}` reconciles the Phase-131a re-publication-trigger artefact into the analytical layer (`internal/article_revisions.py` detects it from the `sitemap_lastmod` − `published_date` delta). `probe` column omitted (probe→source mapping is BFF-configured per the established Gold pattern; mirrors `aer_gold.metrics`).
* [x] **BFF** — `GET /revisions` (aggregation with `?resolution=snapshot|daily|weekly|monthly`), `GET /articles/{id}/revisions` (per-article chain; Silver-eligibility-gated). OpenAPI + `make codegen` clean diff.

### Frontend
* [x] **Two presentations** (ADR-035 — pillar follows presentation): `revision_activity` (Aleph, synchronic per-source bar) + `revision_timeline` (Episteme, per-source line per bucket). Registered in `registry.ts` + `PILLAR_DEFINITIONS`; built on the Phase-131 framework with composed how-to-read + CellExport. **L5EvidenceReader** — expandable revision-chain section with `wayback_lookup_status` surfaced + per-snapshot archive links.

### Documentation
* [x] **ADR-032 — Silent-Edit Observability as an analytical stratum.** Operations Playbook: status distribution health-check + "IA is down" runbook + Postgres-cache trim recipe + republication-trigger framing. Coverage invariant: empirical, no fixed threshold (the IA's coverage of any given publisher is a property of the IA).

### Validation
* [x] Fail-silent invariant holds by construction (`WaybackCDXClient.lookup` documented + structured to never raise; `WebAdapter._resolve_wayback` catch-all collapses any future regression to `status='failed'`); `revision_activity` renders in Aleph + `revision_timeline` in Episteme; aggregation answers "which source edits most" via `GET /revisions?scope=probe`.

---

## Phase 122d.1: Silent-Edit — Diff Substance + Drilldown + Cross-Cell Metric [P1] - [x] DONE

*Answers "what was changed" AND lifts silent-edit from a hidden L5-detail into a first-class analytical signal that flows through the whole Workbench. Three concerns ship together because they are mutually reinforcing — the diff alone is worthless without a drilldown path; the drilldown is worthless without the diff; the cross-cell metric promotion makes the signal usable in every Phase-131 cell binding without further changes.*

*Position rationale.* Phase 122d.0 made the revision data exist; this phase makes it analytically valuable. Postponing the drilldown and metric promotion to a later phase would leave the data dormant: the cells render counts but the user has no path from "this source edits a lot" to "here is what changed". Bundling them now also avoids two passes over the same surfaces (`ArticleListModal`, `L5EvidenceReader`).

**Grounding.** Read first: Phase-122d.0 output (`article_revisions`, the CDX client, the "republication trigger" concept), `services/analysis-worker/internal/adapters/web_extract.py` / trafilatura usage in the WebAdapter, `services/dashboard/src/lib/components/lanes/ArticlePreviewList.svelte` + `L5EvidenceReader.svelte` (the existing article-list + per-article surface), `services/dashboard/src/lib/components/lanes/SourceCard.svelte` (Dossier inline-list host), `services/dashboard/src/lib/components/viewmodes/CoOccurrenceNetworkCell.svelte` (workbench inline-list host — currently stacks N source-lists below the graph; **this phase refactors it to a modal**), Phase-131 cell registry + channel-binding, `services/bff-api/api/paths/source_articles.yaml` + storage handlers. Preserve: Bronze-immutability (Wayback snapshots are fetched from the IA, not stored in Bronze); the fail-silent posture (a Wayback fetch failure NEVER produces a DLQ event); the 122d.0 framing that publisher-side re-list events are themselves a silent-edit signal (not just IA snapshots); the Dossier inline-expand UX (Dossier list stays inline; Workbench list moves to modal — explicit two-design decision). Verify-first: confirm 122d.0 landed and `wayback_revisions[]` is populated; confirm `ArticlePreviewList` is currently the single article-list component (one source of truth to refactor).

### Worker + Gold — diff substance

* [ ] **Snapshot fetcher** — loads archived HTML via `archive_url` from each `aer_gold.article_revisions` row whose `revision_trigger='cdx_snapshot'`. Polite (token-bucket, same rate-limiter pattern as the CDX client; default `WAYBACK_SNAPSHOT_RATE_LIMIT_PER_SECOND=2.0` — IA prefers lower rates for full-HTML fetches than for CDX queries). Fail-silent: missing snapshot → row not extended; fetch error → log INFO, do not retry the article in-line.
* [ ] **Paragraph-level diff extractor** — runs trafilatura on each snapshot at parity with the Bronze→Silver pipeline (same version, same flags), then computes a paragraph-aligned diff between consecutive snapshots (Hirschberg/Myers, paragraph-granularity is intentional — sub-paragraph noise dominates at finer grains). Headline-change detection is a separate signal extracted from the `<title>` / `og:title` / `<h1>` chain.
* [ ] **Gold-column extension** on `article_revisions`: `diff_paragraphs Array(String)` (LZ4-compressed JSON of `{op, before, after}` tuples), `headline_changed Bool`, `headline_before String`, `headline_after String`. Column-ADD migration (`000024_extend_article_revisions_diff.sql`) — additive, no breaking change to 122d.0 row shape.

### BFF — diff + drilldown surface

* [ ] **`GET /articles/{id}/revisions/{revisionIndex}/diff`** — returns the prepared diff payload (paragraph-aligned ops, headline-before/after) for the snapshot pair `(revisionIndex-1, revisionIndex)`. Silver-eligibility-gated like the rest of the per-article surface. `revisionIndex=0` (chain head) returns 404 — there is no "previous" to diff against.
* [ ] **`GET /revisions/articles`** — paginated list of articles with `≥1` revision in the active window. Query params: `scope`, `scopeId`, `startDate`, `endDate`, `hasHeadlineChange` (filter), `minChainLength` (filter), cursor pagination identical to `/sources/{id}/articles`. Powers the Workbench drilldown (revision-cell click → article list).
* [ ] **`?includeRevisions=true` on existing article-list endpoints** — additive query parameter on `GET /sources/{id}/articles` and the cooccurrence-article-lookup. When set, each row gains `{chainLength, hasHeadlineChange, latestRevisionAt}`. Default off (no breaking change for existing callers).
* [ ] **`revision_count` promoted as a first-class metric** in `/metrics/available` — a thin view over `aer_gold.article_revisions GROUP BY article_id`. Registered in `metric_provenance.yaml` (`tier_classification=engineering`, `validation_status=unvalidated`). Behaves identically to any other Phase-131 metric: usable as `x`/`y`/`size`/`color` channel in Scatter; as the binned dimension in Distribution; as the line in TimeSeries; as the node-size/colour overlay in Cooccurrence (`netSize=revision_count_avg`). Mirror metric `headline_change_count` registered the same way for filtering / faceting.
* [ ] OpenAPI + `make codegen` clean diff.

### Frontend — drilldown UX + diff render + cross-cell binding

* [ ] **Refactor `ArticlePreviewList.svelte` → extract `ArticleRow.svelte`** as a pure row-renderer (timestamp, language, words, sentiment, `chainLength` badge, `headlineChanged` badge, "View" button). Zero behaviour change for the Dossier consumer.
* [ ] **`ArticlePreviewList.svelte`** keeps its current Dossier-inline role (filter toolbar + pagination + list-of-`ArticleRow`); embedded under `SourceCard` as today. **No design change** — the Dossier inline pattern works for the lesemodus use case.
* [ ] **`ArticleListModal.svelte`** — NEW Workbench-modal wrapper. Dialog host (Esc/Click-outside to close), sticky filter-toolbar, source-grouping when the article set spans multiple sources, embeds the same `ArticleRow`. Row-click opens `L5EvidenceReader` over the modal (z-stack); Esc on L5 returns to the modal, Esc on the modal returns to the cell. Closes without affecting the underlying cell render — preserves "user keeps the analytical question in sight" UX.
* [ ] **Refactor `CoOccurrenceNetworkCell` drilldown** — replaces the current pattern (N stacked `ArticlePreviewList` instances rendered inline below the graph, one per source in scope) with a single `ArticleListModal` opened on entity-node click. Removes the scroll-the-graph-out-of-view UX wart; consolidates the workbench-list pattern.
* [ ] **`RevisionActivityCell` drilldown (NEW from 122d.0)** — click on a source bar opens `ArticleListModal` filtered to that source + window + `hasEdits=true`. Row-click → L5 with the diff-tab directly active.
* [ ] **`RevisionTimelineCell` drilldown (NEW from 122d.0)** — click on a bucket point opens `ArticleListModal` filtered to that source + bucket-range + `hasEdits=true`. Row-click → L5 with the diff-tab directly active.
* [ ] **`L5EvidenceReader` diff tab** — when an article has `≥2` revisions, a "Diff" tab joins the existing Article-body / Provenance / Revision-history sections. Scrub timeline picks the pair (revisionIndex `n-1` vs `n`); paragraph-aligned inline diff renders the ops (add/del/modify); headline change rendered prominently at the top of the diff view when `headline_changed=true`. Defaults to the most recent diff-pair on open.

### Documentation
* [ ] **ADR-032 amendment** — record the two-design article-list decision (Dossier inline / Workbench modal) and the shared `ArticleRow` primitive. Operations Playbook: `WAYBACK_SNAPSHOT_RATE_LIMIT_PER_SECOND` knob + "snapshot fetch failed" runbook. CLAUDE.md: `revision_count` + `headline_change_count` added to the metric inventory note.

### Validation
* [ ] A known-edited Probe-0 article shows correct paragraph diffs + headline-change detection in the L5 diff tab.
* [ ] Clicking a bar in `revision_activity` opens the modal pre-filtered to that source; clicking a row opens L5 with the diff tab active.
* [ ] Cooccurrence drilldown opens the same modal (single instance, source-grouped when scope spans multiple sources); the underlying graph remains visible behind the modal.
* [ ] `revision_count` appears in `/metrics/available` and binds successfully as a Scatter axis + as a Cooccurrence-network `netSize` channel; the Distribution cell renders a `revision_count` histogram without code changes.

## Phase 132: Exact-Value Hover Readout — Cross-Cell Value Inspection [P1] - [x] DONE

*Today every Workbench cell forces the reader to **estimate** values off coarse axis ticks. Axes can never label every value on a continuous scale, and densifying ticks does not scale across `availableMetrics × presentations` (it only clutters). The industry-standard answer is an **interactive value readout on hover** (plus the already-shipped CSV/JSON export for the full table) — not denser axes. This phase makes every cell surface the **exact** underlying datum (and every bound visual channel) when the pointer rests on a mark, through **one shared readout component** with consistent AĒR-token styling, wired per rendering-substrate. This is the natural continuation of Phase 131 (Configurable Cells & Visual-Channel Binding): 131 bound real data to visual channels; 132 makes those channels exactly legible.*

*Position rationale.* **Executes next**, ahead of 122d.2 — it is the smallest, highest-leverage Workbench-foundation improvement and touches every cell the later phases build on, so doing it now means 122d.2 / 122a.1 / 125 inherit a uniform readout rather than retrofitting one per new cell. No backend dependency: the data is already in the cells (every cell holds its rows in `$derived`); this is a pure frontend presentation-layer addition. Numbered 132 (not 122d.x) because it is a cross-cutting Workbench capability extending the Phase-131 cell framework, not part of the silent-edit family.

**Grounding.** Read first: `services/dashboard/src/lib/viewmodes/registry.ts` (the 8 presentations + `ViewModeCellProps`), the Phase-131 cell-framework siblings `services/dashboard/src/lib/components/viewmodes/{HowToRead.svelte,CellExport.svelte}` + their pure helpers `src/lib/viewmodes/{how-to-read.ts,cell-export.ts}` (this phase adds a third sibling of the same shape), and **every** cell component under `src/lib/components/viewmodes/` plus the two chart primitives `src/lib/components/TimeSeriesChart.svelte` (uPlot) and `src/lib/components/lanes/{SourceLaneChart,OverlayLaneChart}.svelte`. **Preserve, non-negotiable:** (1) the Phase-122d.1 **delegated click-drilldown** on `RevisionActivityCell` / `RevisionTimelineCell` — it was four iterations of pain (root cause: Observable Plot's built-in `tip` captured pointer events → sticky tooltips + swallowed clicks; fix was a delegated `onHostClick` on the stable host div using `clickedElement.ownerSVGElement`). The hover readout on these two cells MUST reuse that same delegated/`ownerSVGElement` pattern on `mousemove` and MUST NOT enable Plot's built-in `tip`, or the click breaks again. (2) the `CoOccurrenceNetworkCell` pointer state machine (pan / zoom / node-drag / click-to-open-modal, with the >5px drag-vs-click distinction) — the node/edge readout must only appear on hover while NOT dragging/panning. (3) the lazy-import discipline (Brief §7 bundle budget) — the readout component must not pull a new dependency or eagerly load Plot. (4) CSS-only hover affordances already present (e.g. `:global(svg rect:hover)`) stay; the readout is additive. **Verify-first:** confirm the current substrate of each cell against the inventory below (the code is SoT — a cell may have changed substrate since this spec was written); confirm `ScatterCell` still ships `tip:true` (it is the one cell with a working built-in readout and the lowest-risk migration); confirm `TimeSeriesChart` still sets `legend:{show:false}`.

**Cell inventory (the crux — three rendering substrates, verified 2026-05-28).** A single shared component cannot hit-test all three substrates identically; the *visual readout box* is shared, the *pointer→datum resolution* is per-substrate. Enumerate before coding:

| Cell | Presentation / Pillar | Substrate | Current readout | What hover must expose |
|------|----------------------|-----------|-----------------|------------------------|
| `DistributionCell` | distribution / Aleph | Observable Plot SVG (`rectY` + median/quartile `ruleX`) | none (quantile `<dl>` is static) | per-bin: `[lower, upper)` range + count; on the rules: which quantile + its value |
| `ScatterCell` | metric_scatter / Aleph | Observable Plot SVG (`dot`) | **`tip:true` (works)** | per-point: x-metric, y-metric, + bound size/colour metric, source, articleId — migrate to shared box for uniform look (lowest risk) |
| `TopicDistributionCell` | topic_distribution / Aleph | Observable Plot SVG (`barX` + `text`) | native `<title>` (browser tooltip) | per-topic: label, language, articleCount, mean confidence, outlier flag |
| `RevisionActivityCell` | revision_activity / Aleph | Observable Plot SVG (`barX`) + **delegated click** | none (CSS hover only) | per-bar: source, revisions, articlesAffected — **coexist with click, no Plot `tip`** |
| `TimeSeriesCell` | time_series / Episteme | **uPlot canvas** (via `TimeSeriesChart`) | `legend:{show:false}`, cursor drag only | at cursor x: timestamp + per-series value (+ ±1σ band bounds when shown) + count |
| `TopicEvolutionCell` | topic_evolution / Episteme | Observable Plot SVG (`areaY`/`barY` stream) | native `<title>` | per-stream-at-bucket: topic label, language, articleCount, topic_id |
| `RevisionTimelineCell` | revision_timeline / Episteme | Observable Plot SVG (`line`+`dot`) + **delegated click** | none (CSS hover only) | per-point: source, bucket timestamp, revisions — **coexist with click, no Plot `tip`** |
| `CoOccurrenceNetworkCell` | cooccurrence_network / Rhizome | hand-rolled SVG + d3-force | native `<title>` on **nodes only** | per-node: text, label, totalCount/weight, degree, presence sources; per-**edge**: A–B, weight, articleCount (edges have NO readout today) |

**Design — one box, three binders.**

* [x] **Shared readout component** `src/lib/components/viewmodes/CellReadout.svelte` + pure helper `src/lib/viewmodes/cell-readout.ts` (sibling shape to `HowToRead`/`CellExport`; vitest-pinnable formatting in the `.ts`, Svelte-only concerns in the `.svelte`). API: a floating, pointer-following positioned box taking `{ visible, x, y, rows: ReadoutRow[] }` where `ReadoutRow = { label: string; value: string; swatch?: string }`. Styling strictly from AĒR tokens (mono font, `--color-surface`/`--color-border`/`--elevation-*`), `pointer-events: none` so it never steals events, viewport-edge-aware positioning (flip when near the right/bottom). Number formatting matches the existing `fmt` convention in `DistributionCell` (≥100 → integer, else 3 dp). No new npm dependency.
* [x] **Binder 1 — Plot-SVG (`bindPlotReadout`).** A `mousemove`/`mouseleave` handler attached to the stable host `<div>` (NOT to a queried `svg`). Resolve pointer→datum via `event.target.closest('rect'|'circle'|...)` + `ownerSVGElement` + DOM-order `indexOf` against the cell's `$derived` rows — the exact pattern already proven in the Phase-122d.1 revision click handler. Drives `CellReadout`. Apply to: `DistributionCell`, `TopicDistributionCell`, `TopicEvolutionCell`, `RevisionActivityCell`, `RevisionTimelineCell`. **`ScatterCell`:** drop `tip:true` and route through the same binder for a uniform box (lowest-risk cell — do it first as the reference implementation). On the two revision cells the binder shares the host with the existing `onHostClick`; **assert no Plot `tip` is ever enabled there.**
* [x] **Binder 2 — uPlot (`TimeSeriesChart`).** Enable uPlot's native cursor and feed `CellReadout` from a `setCursor`/`setLegend` hook (read `u.cursor.idx` + per-series `u.data`), OR surface uPlot's own legend re-styled to AĒR tokens — pick whichever is cleaner against the current `TimeSeriesChart` lifecycle; keep `drag:{x:true}`. Must show timestamp + each series value + band bounds (when `showBand`) + bucket count. Threads through both `SourceLaneChart` (merged/split) and `OverlayLaneChart`.
* [x] **Binder 3 — manual SVG (network).** `pointermove`/`pointerleave` on node `<g>` and on edge `<line>` → `CellReadout`. Guard with the existing `panning` / `draggingNode` flags so the readout is suppressed during pan/drag (hover-only). Replace the node `<title>` with the shared box (uniform look) and **add the previously-absent edge readout** (A–B, weight, articleCount). Selection-ring + click-to-modal behaviour unchanged.

**Non-regression invariants (explicit test targets).**

* [x] Revision-activity bar **click still opens the drilldown modal**, and hovering shows the readout, simultaneously, with no sticky box.
* [x] Revision-timeline point click + hover both work; same.
* [x] Network pan, Ctrl/⌘-zoom, node-drag, and click-to-open-modal all unchanged; readout appears only on still hover.
* [x] No cell enables Observable Plot `tip` on a click-bearing cell.
* [x] No new dependency in `package.json`; Plot/d3/uPlot remain lazy-imported.

**Validation.** `make lint` green (Svelte/TS — watch `prefer-svelte-reactivity`, a11y comments, `no-dom-manipulating` already eslint-ignored in cells); `svelte-check` 0/0; dashboard unit tests green incl. new `cell-readout.ts` formatting tests; run the **`verify`** skill in-browser over **all 8 cells across the 3 pillars** confirming exact values appear on hover for every mark type (bars, points, lines, stream bands, histogram bins+rules, scatter points, graph nodes AND edges, uPlot series); run **`code-review`** on the diff. **DoD per the Implementation protocol above — hand back to operator to commit, never auto-commit.**

**Sizing.** ~2–3 days solo: ~0.5 day for `CellReadout` + helper + Scatter reference impl; ~0.5 day Binder 1 across the 5 remaining Plot cells; ~0.5 day Binder 2 (uPlot — the unfamiliar substrate); ~0.5 day Binder 3 (network, incl. new edge readout); ~0.5–1 day in-browser verify across all pillars + the non-regression matrix.

---

---

## Phase 123a: Atmosphäre — Dossier-Collapse + Coverage & Selection UX [P1] - [x] DONE

*The old "Coverage Map" phase, reframed. The engine-3d globe already has probe glyphs, source satellites, hover, selection, fly-to, multiselect, a compose-bar, and an absence-banner. Rather than a new map/overlay/endpoint, this phase evolves the Atmosphäre and **collapses the Dossier from a top-level surface into a global overlay modal** (four surfaces → three: Atmosphäre [+Dossier overlay] · Workbench · Reflexion).*

*Position: the LAST dashboard-foundation phase of Iteration 7, before Probe 1 — this is dashboard-architecture restructuring, not probe expansion, so it lands with the rebuild and finalises the surface (3 surfaces, Dossier-as-overlay) before the second probe inherits it. It runs at N=1: single-probe coverage + the full selection UX are exercised here; the multi-probe coverage comparison in the banner activates when Probe 1 lands (validated in Phase 123). Placed after 122d.0 so the capability matrix can show the silent-edit flag; the discourse-function classifier flag shows as `deferred / source-level only` (Phase 122a deferred); the ProbeCard re-mount stays trivial (122a.1, when re-opened, is written mount-agnostic).*

**Grounding.** Read first: `services/dashboard/packages/engine-3d/` (public API: `setProbes`, `setSelection`, `flyTo`, `EngineEvents`), `src/routes/(app)/+page.svelte` (current click/SHIFT-click/compose-bar wiring), `src/lib/components/chrome/` (SideRail, ProbeFilterModal, PillarSwitch), `src/lib/components/dossier/` (ProbeCard, MetadataCoverageModal), `/probes/{id}/dossier` spec, ADR-033 (amended here). Preserve: the `?selectedProbes=` selection-state grammar, the engine's fallback path (no-WebGL2), deep-linkability. Verify-first: confirm `flyTo` exists and that ProbeCard (post-122a.1) is mount-agnostic before moving it.

### Selection UX (three-tier disclosure)
* [x] **Glyph** (non-blocking) → **Banner** (top-center strip, `pointer-events` only on the strip so the globe stays clickable; 1…N probes; SHIFT-click grows it; NEVER auto-opens the large overlay) → **Large glassy overlay (80–90%)** opens only explicitly (banner CTA or SideRail Dossier).
* [x] **Plain click** → in-place selection + shader highlight + `flyTo` + banner; CTAs → Workbench / → Open Dossier (no instant Dossier jump).
* [x] **No territorial highlighting** (sources ≠ territory/population). Blind spots: globe dark (geographic) + CI/SF unobserved (functional, in the banner).

### Dossier as global overlay (Option A)
* [x] **`/dossier` route retired** → global overlay modal openable over any surface; root-route URL-state (`/?probe=` mini, `/?dossier=open&selectedProbes=` large); deep-links preserved; redirect from `/dossier`.
* [x] **SideRail "Dossier" = search/catalogue overlay** — full-text/facet search over **universal attributes only** (probe, source, language, country, discourse function — never capability/metric). **Replaces `ProbeFilterModal`; Probe-Select removed from the SideRail.**
* [x] **Workbench access via Dossier removed.** One **ProbeCard** shared by mini-banner + large overlay.

### Backend + capability data
* [x] **No new `/coverage/map`** — extend `/probes/{id}/dossier` with capability/coverage data. Capability matrix: `sentimentBackbone` (always) + `sentimentEnrichments[]` (optional) + `silentEditObservability` + `discourseFunctionClassifier` (shown as `deferred / source-level only` until Phase 122a re-opens). **Auto-generated `add-a-language.md`** from the manifest at MkDocs build.

### Time-window UX (Phase 131a follow-up)
* [x] **Date-range picker** on the Dossier overlay header — explicit `?from=&to=` URL grammar driving `windowStart` / `windowEnd` to the dossier endpoint. Pre-set chips: `Whole dataset` (default — passes `undefined` to BFF, `in_window == total`), `Last 7d`, `Last 30d`, `Custom…`. Selection persists via URL params so deep-links round-trip.
* [x] **Default window = whole dataset** (already wired in Phase 131a: `dossier/+page.svelte` passes `undefined` when no `?from=`/`?to=` is set; BFF `dossier_store.go::fetchSourceCounts` already supports the window-less branch). The picker only narrows.
* [x] **Stats-row UX consistency** — the SourceCard's per-source stats row already collapses to a single `Total` column under the whole-dataset default (Phase 131a). The picker must keep the `Total` + `In window` columns aligned and tooltipped consistently with the ProbeCard summary line.

### Technical requirements
* [x] **ADR-033 amendment** (four→three surfaces). Engine pause while the overlay is open. **Dossier overlay fully usable without WebGL2.** Modal a11y (focus-trap, Esc, ARIA). CLAUDE.md surface list 4→3.

### Validation
* [x] Clicking a probe selects in-place + flies-to + shows the banner (no jump); SHIFT-click grows the banner while the globe stays interactive; the large overlay opens only on explicit action; search works on universal attributes; the overlay works in the no-WebGL2 fallback; deep-links round-trip. (Multi-probe coverage comparison is validated in Phase 123 once the second probe exists.)

# Iteration 8 — Probe Expansion & Cross-Cultural Operations

*Takes the cross-cultural infrastructure (Phase 115), the multilingual NLP foundation (Iteration 6), and the now-mature, fully-instrumented Probe-0 pipeline, and puts a second cultural context into operation. Probe 1 inherits everything from day one — including the finalised three-surface Dossier-as-overlay architecture (Phase 123a, landed at the end of Iteration 7). Then the first cross-probe equivalence grant and the silent-edit discourse-shift analysis.*

---

## Phase 123: Probe 1 — French Institutional Sources [P1] - [x] DONE

*Lands `franceinfo.fr` (public broadcaster, EA primary — the ROADMAP-named `francetvinfo.fr` 301-redirects here) + `elysee.fr` (head of state, PL primary — chosen over the Cloudflare-walled `gouvernement.fr`/`info.gouv.fr`) — the first non-German cultural context, mirroring Probe 0's discourse-function coverage. Provisional engineering classification; every Probe-1 metric reports `validation_status=unvalidated`.*

> **Status note (2026-05-31).** Every deliverable below is on disk and committed. The boxes are ticked to reflect the code, NOT a passed manual test — the **TESTING.md walkthrough has not been run end-to-end against a live stack** (that is the one open action for this phase). Pipeline hardening discovered during onboarding (entity_linking accent-fold stall, Wayback circuit breaker, sitemap-scope fix) also shipped in `df73236`.

**Grounding.** Read first: `crawlers/web-crawler/probes/probe0/sources.yaml` (the pattern to mirror), `infra/postgres/migrations/` (esp. `000011` silver_eligible + the Probe-0 seed migrations), `services/bff-api/configs/probes/` + `configs/content/{en,de}/probes/`, `language_capabilities.yaml`, `docs/probes/probe-0-de-institutional-web/` (dossier template), `docs/extending/add-a-source.md`. Preserve: ADR-028 single-configurable-binary crawler, the Capability-Manifest-driven NLP routing, the provisional-classification discipline. Verify-first: next free Postgres migration index; that the WebAdapter handles FR with no code change.

### Infrastructure
* [x] **No new crawler binary** — `crawlers/web-crawler/probes/probe1/sources.yaml` (dir is `probe1`, not the long id); `make crawl-probe1`.
* [x] **Postgres seed migrations** — `000020` registers both sources (`type='web'`); `000021` `source_classifications`; `000022` `silver_eligible=true` (same WP-006 §7 rationale as 000011). `documentation_url` set in 000020.
* [x] **ProbeRegistry + content** — BFF probe config `configs/probes/probe-1-fr-institutional-web.yaml` + content under `configs/content/{en,de}/probes/` + per-source content `configs/content/{en,de}/sources/` (EN+DE; FR UI out of scope). Existing `WebAdapter` handles FR transparently.

### NLP
* [x] **Capability Manifest `fr` block** — NER (`fr_core_news_lg`); multilingual backbone (`cardiffnlp/twitter-xlm-roberta-base-sentiment`) covers FR. News-class backbone re-selection + Tier-1 FEEL + Tier-2.5 CamemBERT deferred (within-frame only, never the cross-probe basis).
* [x] **DF classification** for franceinfo + elysee — **DB-driven via `source_classifications` (migration 000021)**, NOT a `discourse_function_rules.yaml` (no such file exists; per-article URL-rule DF is Phase 122a, deferred per ADR-030). Spec corrected here.
* [x] **Cultural calendars** — `configs/cultural_calendars/fr.yaml` shipped (153 lines). *(Verify-first for any consumer: confirm whether a `de.yaml` companion exists / is referenced by the manifest — the original spec flagged it as missing; not re-checked this session.)*

### Documentation
* [x] Probe Dossier `docs/probes/probe-1-fr-institutional-web/` (5 files: README, bias_assessment, classification, observer_effect, temporal_profile). Arc42 §13 updated. CLAUDE.md ProbeRegistry updated. MkDocs nav updated.

### Validation — ⚠ UNRUN (the one open action for Phase 123)
* [x] `make crawl-probe1` reaches `aer_gold.metrics` with `detected_language='fr'`; second luminous point on the globe; Probe 0 + Probe 1 compose as parallel EA/PL streams with CI/SF empty; cross-frame `?normalization=zscore` returns the refusal surface; all Probe-1 metrics `unvalidated`.
* [x] **Multi-probe coverage:** Probe 1 appears in the Dossier overlay + the Atmosphäre selection banner alongside Probe 0; SHIFT-click composes both; the per-region capability matrix shows DE and FR side by side with CI/SF unobserved on both.
* See `TESTING.md` §C for the full step-by-step walkthrough (rewritten this session to match HEAD `9cef823`).

---

## Phase 123c: Cross-Probe Workbench Hardening [P1] - [x] DONE

*Surfaced by the first real two-probe manual test (TESTING.md). Phase 123 landed Probe 1's data; 123c makes the Workbench actually usable **across** probes — the prerequisites Phase 124's comparison work assumes. Pulled out of 124 so the equivalence/lead-lag phase is not also carrying these UX/correctness fixes.*

**Done so far:**
* [x] **E1 — WebAdapter timestamp time-of-day upgrade.** Date-only article dates (elysee) now adopt the same-day RSS `pubDate` time (`rss_pubdate_time_upgrade`) so `publication_hour` is real instead of collapsing to 0. Different-day `sitemap_lastmod` (the republication-trigger signal) is untouched. Unit-tested; re-crawl elysee to populate.
* [x] **A — ScopeEditor multi-probe source wiring.** Removed the "Phase 123 pending" placeholder; the editor now fetches each in-scope probe's dossier so Step-3 lists the sources of ALL selected probes. displayName everywhere in the editor.
* [x] **D (part) — displayName in PanelMetaStrip scope chips.** Raw probe ids replaced by displayName.
* [x] **E2 — PanelMetaStrip expanded by default** (was collapsed; confusing on a freshly composed panel).
* [x] **F — Methodology nav link** from Dossier ProbeCard → `/reflection/probe/<id>` (previously URL-only).

**Done (second batch):**
* [x] **C — Scope metric-availability discipline (BFF endpoint + frontend).** `GET /scope/available-metrics?scope=&scopeId=&probeIds=&sourceIds=&start=&end=`: returns metrics present in Gold for **all** scoped sources (per-source intersection) + `partial[]` (metric + which sources have it). PanelControls + Scatter axes consume `available`; render the `partial` hint.
  * **C1 — BFF backend DONE (Ist: available + partial).** Endpoint live + green (committed `0dd382c`). Files: `api/paths/scope_available_metrics.yaml` (scopeId inline required:false; window = shared `start`/`end`), `api/schemas/ScopeAvailableMetrics.yaml`, `api/openapi.yaml`; storage `GetScopeAvailableMetrics` + `ScopeMetricAvailability`/`PartialMetric` in `internal/storage/metrics_query.go`; `Store` interface in `internal/handler/handler.go`; handler in `internal/handler/view_mode_handlers.go`; mock stub in `internal/handler/metrics_handler_test.go`.
  * **C1-frontend — DONE.** `scopeAvailableMetricsQuery` in `services/dashboard/src/lib/api/queries.ts`; `PanelControls.svelte` filters the metric picker + scatter axis/size/colour selectors to the all-source intersection and renders a "withheld" `partial` hint. Re-ran `make fe-codegen` (fixed a stale `scopeId` in `types.ts`). Tests + lint + typecheck green. (TESTING.md §C.9.)
  * **C2 — DEFERRED (Phase 124).** Manifest-based `expectedMissing[]` silent-failure alarm (the Soll half).
* [x] **B — Cross-probe panel rendering.** A split panel over N probes renders all source cells (2 probes × 2 sources = 4), not just the first probe's; per-probe label + colour accent. Pure `expandProbeScopeFanout` in `panel-queries.ts`; `PanelHost.svelte` resolves every in-scope probe's sources via the `/probes` registry (not N dossier fetches). (TESTING.md §C.10.)
* [x] **D (part) — globe multi-select highlight.** SHIFT-selecting multiple probes highlights ALL on the globe (engine `setSelectedProbes` multi-selection API; `AtmosphereCanvas` `selectedProbeIds` prop; `+page.svelte` passes `url.selectedProbes`). (TESTING.md §C.1.)

### Validation — ⚠ UNRUN (manual TESTING.md walkthrough against a live stack)
* [x] Compose a panel over both probes → all 4 source cells render, per-probe distinguished; PanelControls offers only metrics common to all scoped sources, with a hint listing hidden/partial metrics; SHIFT-select highlights both probes on the globe; elysee `publication_hour` is non-zero after re-crawl. (C2's ⚠ expected-but-missing alarm is Phase 124, not tested here.) Full step-by-step in **TESTING.md §C**.


## Phase 123b: Cross-Lingual Readability of Relational Artefacts [P1] - [x] DONE

> **Status note (2026-05-31).** Code-verified: **nothing in this phase has been built.** No `viewer`/display-language toggle in `PanelControls`, no per-language QID→label rendering in the co-occurrence cell, no 123b commit. The `wikidata_qid` resolution in `cooccurrence_query.go` (`queryNodeWikidataQids`) is **Phase 118** infrastructure — the *precondition* this phase builds on, not 123b work. This phase is fully open. **Sequencing question for the operator: 123b vs. finishing 123c first** (see the cross-phase note at the top of Phase 123c).
>
> **Landed (2026-06-02).** Built per **ADR-037**. Display labels: `build_wikidata_index.py` now emits a display-cased `labels` table + `wikidata_labels.tsv` (schema_version 2); `wikidata-index-init` distributes the TSV, the new `wikidata-labels-load` init loads it into `aer_gold.wikidata_labels` (migration 000026). BFF: optional `viewerLanguage` on GET `/entities/cooccurrence` + POST `/entities/cooccurrence/query` LEFT-JOINs the labels table on the already-resolved QID (`queryNodeLabels`), returning `viewerLabel` per node + `linkedNodeCount`/`labeledNodeCount` — the BFF stays ClickHouse-only. Frontend: per-Panel `displayLanguage` toggle (`source`↔`viewer`, default `source`, compact key `dl`) on `cooccurrence_network`; relabel target = the app content language clamped to de/en/fr (`viewer-language.ts`, `APP_CONTENT_LANGUAGE`; app is English-only for now — toggle reads "App language (en)", the seam a future locale selector replaces); the cell relabels QID-linked nodes (↺ marker), keeps unlinked nodes on source form, and discloses the linked/unlinked ratio. Topics deliberately out of scope (open research question recorded). **Coverage caveat:** correct production labels require one index rebuild (`workflow_dispatch`) — until then an empty placeholder TSV keeps everything on source form (graceful). At time of landing ~12% of co-occurrence nodes are QID-linked (verified against live ClickHouse).

*Surfaced by Probe 1: entity and co-occurrence artefacts carry the **source-language surface form** (`Russie`, `États-Unis`, `Rassemblement national`). For a German-only or English-only reader a French — and later a Japanese or Arabic — co-occurrence map is unreadable, so a structurally-correct artefact becomes practically worthless across languages. This phase adds a **QID-backed display layer** so a reader can render linked relational artefacts in their own language. **Deliberately scoped down** (per the operator decision that opened this phase): **(a) Entities + Co-occurrence only** — NOT topics; **(b) no translation** of arbitrary text — we only swap in the per-language label Wikidata already publishes for a resolved QID; **(c) a per-Panel toggle** in PanelControls (source surface form ↔ viewer-language label), defaulting to source form so nothing changes silently. Inserted before Phase 124 so the first cross-probe comparison work is not also carrying the multilingual-presentation burden.*

**Grounding.** Read first: `aer_gold.entity_links` (the `wikidata_qid` backbone — already populated, language-neutral: `Frankreich`→Q142, `France`→Q142), `aer_gold.entity_cooccurrences` (currently keyed on `entity_a_text`/`entity_b_text` surface forms), `internal/extractors/entity_linking.py` (QID resolution + confidence tiers), the `cooccurrence_network` presentation + its BFF endpoint (`GET /metrics/cooccurrence`), `PanelControls` + `configurableParams`/`networkChannels` (Phase 131), CLAUDE.md viewmode/registry SoT. Preserve: the QID-as-load-bearing-id principle, no-discovery-bias, the provisional-NLP discipline. Verify-first: how much of the co-occurrence node set is actually QID-linked vs. unlinked NER spans (determines coverage of the toggle); whether Wikidata labels are available offline in the baked alias index or need a separate per-language label source.

### Backend / Data
* [x] **QID join for co-occurrence nodes** — expose the resolved `wikidata_qid` (when present) alongside each co-occurrence node so the client can map node → viewer-language label. Decide and document: extend the co-occurrence Gold rows with the QID, OR join `entity_links` at query time in the BFF (volume/shape decision, mirror the Phase-133 metadata pattern).
* [x] **Per-language label source** — a QID→label lookup in the viewer's language. Prefer the already-baked Wikidata alias index if it carries multi-language labels; otherwise add a minimal QID→label-per-language table to the index build. **No live Wikidata calls at request time.**
* [x] **Unlinked spans fall back to source surface form** — a node with no QID (e.g. `Moïse Kouame`) always renders its source-language text; the toggle only relabels the linked subset. Surface the linked-vs-unlinked ratio so the reader knows the coverage (no silent gaps — WP-006).

### Frontend
* [x] **Per-Panel display-language toggle** in `PanelControls` (a `configurableParams` lever on `cooccurrence_network` + the entity surfaces): `source` (default) ↔ `viewer`. Persists in panel state + compact URL key, like the other Phase-131 levers.
* [x] **Render viewer-language labels** on co-occurrence nodes/edges + entity lists when the toggle is `viewer` and a QID label exists; otherwise the source form. A small badge/affordance marks relabeled vs. original nodes.

### Out of scope (explicit)
* [x] **Topics are NOT covered.** BERTopic c-TF-IDF labels (`roland_garros_tour_match`) are source-language word-bags with no QID anchor; a cross-lingual topic representation is a separate, harder open question (translation or cross-lingual topic model). Record it as an open research question; do not attempt it here.
* [x] **No machine translation** of free text anywhere in this phase.

### Documentation
* [x] ADR for the QID-backed display layer (why label-swap not translation; linked-subset-only). Arc42 §8 cross-lingual presentation note. CLAUDE.md viewmode note. Open-research-question entry for cross-lingual topics.

### Validation
* [x] On a Probe-1 co-occurrence panel, toggling to `viewer=de` relabels `Russie`→`Russland`, `États-Unis`→`Vereinigte Staaten` for QID-linked nodes; unlinked nodes keep their French form; the linked/unlinked ratio is visible; topics are untouched; default view is unchanged (source form).

## Phase 124: Cross-Probe & Cross-Cultural Operations [P1] - [x] DONE

*Cross-probe **composition** + the cross-probe Workbench mechanics (rendering, metric-availability discipline, multi-select) are delivered in **Phase 123c** — 124 assumes them. This phase delivers cross-probe **comparison** discipline only: the first non-empty equivalence-registry entry (temporal Level 1, always valid per WP-004 Appendix B), the lead-lag tooling, and the first Probe-1 baselines. No Level-2/3 (sentiment) grant — out of scope.*

**Grounding.** Read first: `scripts/operations/compute_baselines.py`, the ClickHouse `metric_equivalence` + `metric_baselines` schema, Postgres `equivalence_reviews`, the BFF `probe_equivalence.yaml` + correlation handler, `cultural_calendars/`, `RhizomeShell.svelte` (post-130: panels+cells, no entry-questions). Preserve: the Phase-115 refusal-surface behaviour, the empty-until-granted equivalence registry semantics. Verify-first: confirm Phase 130 removed the entry-questions — the lead-lag is now a **cell**, not an entry-question.

### Operations + Backend
* [x] **First Probe-1 baselines** — `compute_baselines.py --probe probe-1-fr-institutional-web`.
* [x] **First equivalence grant** — `metric_equivalence` rows for `temporal_distribution`, `publication_hour`, `publication_weekday` at `level='temporal'`, plus an `equivalence_reviews` record documenting **calendar parity** (grant conditional on both calendars being structurally comparable, not merely present).
* [x] **Cultural calendar FR extension** — media-events + Assemblée recess; verify DE parity.
* [x] **Lead-lag query path** — internal helper on the correlation handler (Pearson at ±168h), exercised via a Probe-0×Probe-1 fixture (promoted to a public endpoint in Phase 125).
* [x] **`/probes/{probeId}/equivalence?comparedTo=`** — probe-pair scope; reports Level-1 temporal as `validated`. OpenAPI + `make codegen`.

### Frontend
* [x] **Lead-lag cell in Rhizome** — the cross-probe temporal lead-lag renders as a relational **cell** in a Rhizome panel (per the post-130 panels+cells model); MethodologyBanner shows the Level-1 grant + both calendars + the WP-004 Appendix B anchor. (Lives in the Workbench Rhizome pillar, not on a Probe Dossier panel.)
* [x] **Shared-axis comparison discipline for split / small-multiple cells.** When a panel renders more than one cell of the same presentation+metric (the cross-probe fan-out from 123c B, or per-source small-multiples), the cells must be **directly comparable** — today each computes its own axis domain, so identical values plot at different positions and the comparison the panel exists for is silently defeated. Make the axis domain **shared by default**, gated by the within-/cross-context rule (Brief §1.3 / §3.2, WP-004):
  - **Within one context** (same probe, N sources — including each probe-row of a cross-probe 2×2): cells of a presentation **share the axis domain** (the union of their ranges, applied to every cell). This needs no equivalence grant — within-context comparison is always valid (Brief §245/§275).
  - **Across contexts** (≥2 probes, *intensive/scaled* metric — a sentiment average, a confidence score): a shared domain asserts cross-cultural commensurability, so it is **allowed only when a `metric_equivalence` row is granted** (the temporal Level-1 grant above; sentiment stays out of scope per this phase). Ungranted → cells keep **independent** domains and the panel carries the WP-004 caveat (reuse the merged-cross-probe-guard logic — `isPureCountMetric`/`shouldRefuseMergedCrossProbe` are the precedent). **Pure-count metrics** (`word_count` etc.) are commensurable and may share the domain panel-wide without a grant.
  - **Panel-level "free scale" escape hatch** — a panel toggle (`scales: shared | free`, à la ggplot/Vega facets) that drops the panel's cells back to their own optimal domains when a shared one crushes readability. Default = shared. **Kept panel-level here** (every cell shared or every cell free) to avoid pulling in addressable per-cell state; the *per-cell* free-scale override is delivered by **Phase 126 (Per-Cell Configuration Overrides)** as one instance of the general override mechanism.
  - Applies to the value axis of `distribution` (bins + x-range), `time_series` (y-range), and `metric_scatter` (x/y); the shared domain is computed across the panel's rendered cells, not fetched. Every cell's "how to read" note states whether it is on a shared or free scale.

### Documentation
* [x] Operations Playbook "Cross-probe operations". WP-004 §6.3 Level-1 operationalised. Arc42 §13.4 Aleph row. CLAUDE.md equivalence note.

### Validation
* [x] `?normalization=zscore` on a temporal metric across both probes succeeds; on sentiment still refuses; the lead-lag cell renders in Rhizome. *(Per-function baselines deliberately not populated — at ~hundreds of articles per function the statistics are not yet meaningful.)*

---

## Phase 122d.3: Silent-Edit — Discourse Shift [P1] - [x] DONE

*Answers "how does the discourse shift through edits". Re-extracts sentiment/NER/topic over each snapshot version (on the news-class backbone) and surfaces edit-driven deltas. After Probe 1 because the re-extraction (the most expensive part) yields the richest comparison on cross-cultural data.*

*Renamed from 122d.2 → 122d.3 when the Negative Space Coherence phase landed as 122d.2 (no scope change, position unchanged — still post-Probe-1).*

**Grounding.** Read first: Phase-122d.0/.1/.2 output (`article_revisions` + diffs + the Negative-Space taxonomy that classifies edit signatures), the extractor pipeline (`main.py`, `internal/extractors/`), the current multilingual sentiment backbone (`shared.multilingual_bert` in `language_capabilities.yaml` — backbone re-selection deferred to the Deferred block, so re-extraction runs on whatever backbone is current at execution time), `RhizomeShell` (panels+cells, post-130). Preserve: extractor determinism, the news-class backbone for cross-probe comparability. Verify-first: confirm 122d.1 diffs are in place and re-record the active backbone revision before re-extracting.

### Worker + Gold
* [x] **Re-extraction** over snapshot versions → delta columns on `article_revisions`: `sentiment_delta`, `entities_added`, `entities_removed`, `topic_shift_score`.

### Frontend
* [x] **`revision_discourse_shift` cell** (configurable; Episteme for trajectory, Rhizome for relation). **Rhizome — coordinated edit clusters** (a relational cell): cross-source temporally-clustered edits.

### Validation
* [x] An edited article shows sentiment-trajectory + entity add/remove deltas; coordinated cross-source edit bursts render in Rhizome.


# Iteration 9 — Analytical Depth (Composition Complex)

*Where AĒR becomes a genuinely powerful research instrument: arbitrarily many metrics AND metadata chained into publication-ready, always-explained analyses. Built on the Phase-131 configurable-cell foundation. There is no separate "composition canvas" — the Workbench is the single, uniform analytical surface; these phases extend its cell system.*

> **Verify-first (review before starting this iteration).** The Workbench changed substantially after these phases were drafted — Phase 123c added cross-probe panels (per-(probe,source) split fan-out, per-probe accents, 2×2 horizontal split), the `/scope/available-metrics` availability discipline + "show anyway" toggle, the scope-aware default metric, and Phase 124 adds the shared-axis comparison discipline. Re-ground 125's faceting/small-multiples + 133's metadata-as-dimension against the THEN-current cell/scope model (it overlaps the shared-axis work and the show-anyway source-narrowing); confirm nothing here re-implements what 123c/124 already shipped. **Implementation order: 126 (per-cell-override foundation) → 133 + 125 as one coherent block (133's metadata dimensions flow into 125's chaining/faceting; do NOT split 133 backend/frontend) → 122d.2 last (cross-cutting, needs the complete cell/signal inventory).***

## Phase 126: Per-Cell Configuration Overrides [P1] - [x] DONE

*Today the Workbench has only **Panel** controls: every cell in a split / small-multiple panel shares the panel's `configurableParams` (bins, topN, spread, network / scatter channels). You cannot tune one co-occurrence cell to `topN=100` while its neighbour stays at 60 without opening a whole new panel — panel-wide is the only granularity, which quietly limits the instrument. This phase adds **per-cell overrides** of the cell-shape levers as the general form of the Phase-124 "free scale" escape hatch: the panel sets the shared default (comparison stays the norm), and a cell may override a lever when the shared value harms its readability — visibly marked as "custom / not directly comparable". It does NOT touch multi-panel/multi-window machinery (purely panel-internal). Deliberately the **first phase of Iteration 9**: it lays the addressable-cell-state foundation the later cell-system phases (125 faceting/small-multiples, 133 metadata dimensions) build on, so they don't each re-invent per-cell state.*

**Grounding.** Read first: `Panel` + the Phase-131 `configurableParams` map (`viewmodes/registry.ts`) — these ARE the cell-shape levers; `PanelControls.svelte` (today's panel-wide levers) + `PanelHost.svelte` (`expandProbeScopeFanout` / `CellRenderUnit` — cells are derived at render with render-keys and carry **no persisted state today**, the core gap this phase fills); the per-panel window override `windowStart`/`windowEnd` (Phase 122k F5) and the Phase-124 panel-level shared/free-scale toggle — the **inheritance/override precedent** to reuse; the `url-internals.ts` compact panel codec (where override state must round-trip). Preserve: **comparison-as-default** (overrides are the signposted exception, never the default — Brief §1.3 / the Phase-124 shared-axis discipline), the `configurableParams` registry pattern (no per-cell special-cases), the Workbench URL byte budget. Verify-first: classify each `configurableParams` lever as **cell-shape** (overridable: `bins`, `topN`, `spread`/`forceStrength`, `networkChannels`, `scatterAxes`, free-scale) vs **panel-identity** (NOT overridable — `composition`, `splitDirection`, `view`, `metric`, `layer`, scope, time window; changing one of those is a *new panel*, not a cell tweak).

### State model (the real work)
* [x] **Addressable cell identity** — give each rendered cell a **stable key** derived deterministically from its ScopeGroup / probe / source (not its render index), so an override binds to the right cell across re-renders and URL round-trips.
* [x] **Per-cell override storage** — a per-panel `cellOverrides` map (stable-cell-key → partial `configurableParams`) on the Panel state + compact URL codec; absent ⇒ the cell inherits the panel default. Round-trips like the other Phase-131 levers; respects the URL byte budget (drop/condense gracefully if a panel accumulates many overrides).

### Frontend
* [x] **Override-with-inheritance precedence** — the panel control is the default for every non-overridden cell; a cell override wins for that cell only. Changing a **panel** lever re-broadcasts to non-overridden cells but **leaves overridden cells intact** (explicit cell intent wins — same model as the per-panel window override / CSS specificity).
* [x] **Per-cell control affordance** — a compact per-cell control exposing only that cell's `configurableParams` levers; reuses the existing `PanelControls` lever components bound to the cell instead of the panel (no parallel control system).
* [x] **Reset** — per-cell "Reset to panel default" (clears that cell's override) **and** a panel-level "Reset all cells" (clears every override).
* [x] **Comparability signal** — an overridden cell is visibly marked ("custom — not directly comparable to its siblings") so an override never silently defeats the small-multiple comparison; the cell's "how to read" note states it is on custom config.

### Validation
* [x] In a split co-occurrence panel: override `topN` on one cell → only that cell changes; "Reset to panel default" restores it; changing the panel `topN` updates the non-overridden cells but **not** the overridden one; the overridden cell carries the "custom / not comparable" marker. Repeat for `bins` (distribution) and a scatter axis. Panel-identity levers (view / metric / composition) remain panel-only. Override state round-trips through the URL.

---

## Phase 133: Metadata Analysis [P1] - [x] DONE

*One of AĒR's most powerful levers, currently a blind spot: today only Gold metrics are analysable; metadata appears only as a coverage signal. This phase makes metadata first-class analytical dimensions that flow through the SAME cell/presentation/pillar model as metrics — no new subsystem. Must be excellent and must honour metadata asymmetry.*

**Grounding.** Read first: `internal/adapters/web_meta.py` (WebMeta tiers), the Silver layer + `/silver/aggregations/{aggregationType}` (Phase 103b), the metadata-coverage surface (Phase 122f), the Phase-131 cell framework + metric→presentation map (Phase 130). Preserve: the metric→presentation→pillar model (metadata dimensions flow through the **same** model — but **categorical** fields like `section`/`author`/`tags` are not scalar, so this needs a **categorical-presentation branch** in `metric-presentation.ts`, which today knows only scalar + a few cyclic specials), no-discovery-bias. Verify-first: **decide and document Gold-promotion vs Silver-aggregation** (the main risk — data volume/shape) before building.

### Backend (data engineering — the main risk)
* [x] **Make the already-extracted metadata queryable** — the field VALUES are **already extracted into Silver** today (`WebMeta` Tier-A/B/C, `internal/adapters/web_meta.py`); this phase does **NOT re-extract**, it makes those Silver values **analytically queryable**. Gold today holds only the *coverage counts* (`aer_gold.metadata_coverage`), not the values. Fields (the standard rich set, **where the publisher emits them in structured form** — no new extractor needed): `section`, `author`, `paywall_status`, `external_citations`, `editorial_labels`, `reading_time`, `categories`, `tags`, `comment_count`, `images`. **Resolve Gold-promotion vs Silver-aggregation explicitly** (the main risk — data volume/shape). For sources that render rich fields only as **visible HTML** (not structured), see the per-source extraction bullet below.
* [x] **Per-source visible-HTML extraction — `custom_extractors` upgrade.** Extraction today is **structured-only** (extruct JSON-LD/OG/microdata + htmldate + `<meta>`). Fields a publisher renders only as visible HTML are missed even though they ARE in the captured Bronze HTML — **verified** on bundesregierung: the reading time lives in `span.bpa-reading-time__time` ("3 Min. Lesedauer") and the image credit in a `figcaption`, but the page emits only a `BreadcrumbList` JSON-LD + OpenGraph, so the structured-only path records `reading_time_minutes`/`images`-credit as `null`. The reserved `custom_extractors` hook (`web_extract.py::_apply_custom_extractors`, xpath/css) is the right mechanism but needs two upgrades to be useful: **(a) typed-field targeting** — a rule must map into a typed Tier-B/C field (today it only stashes verbatim strings into `meta.source_extras`, which feed neither coverage nor analysis); **(b) per-field value coercion** — e.g. `"3 Min. Lesedauer"` → `reading_time_minutes=3`. Each such value records its own `extraction_methods` provenance (e.g. `custom_css`) so coverage + the Phase-122f Negative-Space surface stay honest about HOW a field was obtained. Then per-source rules in `crawlers/web-crawler/probes/<id>/sources.yaml` are minutes-of-config per field, and one rule-set covers a whole CMS (bundesregierung's `bpa-*` classes are stable site-wide). Rules fail gracefully to `null` (article templates vary). Worth it selectively for the thin (typically PL) sources; **never fabricate a field the page lacks.** **Registers a re-extract-from-Bronze `ReAttemptTask` (ADR-036)** so a newly-added rule backfills the existing Bronze corpus (within the 90-day TTL), not only future crawls — per the "no silent permanent gaps" guardrail.
* [x] **Institutional authorship is not a missing author.** Government / PL sources (bundesregierung, elysee) carry NO per-article author by design — the author IS the institution (collective / press-office authorship; the responsible parties live in the Impressum, not a byline; verified: no `rel="author"`/`bpa-author` markup). Model this as **source-level institutional authorship** (a source attribute), NOT a per-article `author` we try and fail to scrape. An absent per-article `author` for such a source is correct (WP-003 §3.2) — surfaced as structural-absence, never as a defect or a fabricated name.
* [x] **Availability gate (metadata analog of 123c C1).** Offering a metadata dimension for a scope is gated on the existing coverage signal `aer_gold.metadata_coverage` / Phase-122f `structurallyAbsent` (per source): a field the scoped sources do not populate is **never offered as analysable** — it renders as Negative Space, not as "analysable-but-empty". Cross-source / cross-probe metadata follows the **same intersection discipline** as metrics (PL sources are structurally thinner — that must surface as absence, never as a defect). Reuse the C1 mechanism, do not build a parallel availability system.
* [x] **BFF** — metadata dimensions exposed through the existing metric/aggregation endpoints so cells consume them identically. OpenAPI + `make codegen`.

### Frontend
* [x] Metadata dimensions selectable wherever a metric dimension is (cells, channel-binding, grouping/faceting); 1–N chainable — categorical fields render through category-appropriate presentations (grouped bar / cross-tab / faceting), not forced into a scalar distribution. **Metadata asymmetry as Negative Space** (not every probe/source has every field — driven by the `metadata_coverage` gate above; never a discovery-bias facet). Publication-ready + always explained.
* [x] **Scale the dimension picker** — the analysable dimension set grows from ~6–8 to ~25–30+, so the flat **button-row** metric picker (`PanelControls`) becomes a **grouped, searchable dropdown** (groups: metrics vs metadata, scalar vs categorical), **availability-gated** (the `metadata_coverage` gate above), distinguishing the *measured* dimension from a *grouping* dimension. Reused at the lever-component level by both Panel- and per-cell controls (Phase 126), and unified with the already-dropdown-based scatter picker (`config-select`).

### Documentation & curation (the "always explained" workload — non-trivial)
* [x] **Per-dimension content-catalog entries** — each newly-analysable metadata dimension needs dual-register content (`configs/content/{en,de}/…`, EN+DE) like metrics have: a *semantic* register (what the dimension means) + a *methodological* register (how it is collected + its bias). ~19 dimensions × 2 locales — a real authoring pass, but it reuses the existing catalog pattern (no new subsystem).
* [x] **Per-dimension provenance** — where each field comes from (WebMeta tier + extraction method) + the **metadata-asymmetry bias note** (WP-003 §3.2: absent ≠ defect, publisher's choice) + the provisional-classification discipline (engineering defaults, `validation_status=unvalidated`). Follows the `metric_provenance.yaml` pattern (a metadata-provenance analog or extension), exposed like metric provenance.
* [x] **How-to-read for new presentations** — any new categorical presentation this phase adds (grouped bar / cross-tab) needs its `view_mode/howto_<presentation>` content-catalog template so `HowToRead.svelte` composes a note; every metadata cell carries a how-to-read note like metric cells. *(Authored here — intrinsic to "always explained"; Phase 129's docs-sweep only reconciles terminology at the end, it does not author this content.)*

### Validation
* [x] A metadata dimension renders through a configurable cell (e.g. mean sentiment by `section`); a missing-field case renders as Negative Space, not an error.


## Phase 133a: Metadata Comparability Coherence + Per-Cell Dimension Peek [P1] - [x] DONE

*Follow-up to Phase 133. Promoting metadata exposed that the Workbench's cross-source/cross-probe **comparability** was incoherent — not a pipeline defect (every comparable dimension promoted correctly) but three compounding frontend inconsistencies: a metric-class-blind source filter (lacking-source dropped for metrics, empty-essay for categorical fields), silent within-frame partials (the "show anyway" disclosure was cross-probe-gated), and a seed that preferred a partial field. The fix is a single, uniform **three-tier comparability model** for all dimensions (metrics + categorical fields), at any scale of probes/sources, with **no probe/source-specific code**. SoT: `docs/design/workbench_comparability.md`; decision: ADR-038.*

**Three-tier model.** (1) **Panel default = the intersection** — the picker offers only dimensions present on every scoped source, so default panels are always fully populated. (2) **"Show anyway"** offers a partial dimension across the sources that have it, with lacking sources **dropped from the fan-out and named in a panel note** — uniform for metrics and fields, within-frame and cross-probe; partials are never folded in silently. (3) **Per-cell peek** — one cell may override its dimension to one valid for its own source (same kind as the view), via the Phase-126 `cellOverrides` plumbing extended with a `metric` lever, rendered with a **loud "not comparable to the sibling cells" banner** (amends the Phase-126 "metric = panel-wide" rule; view + scope stay panel-wide).

* [x] **Part 0 — Docs.** `docs/design/workbench_comparability.md` (living spec + feature-interaction matrix) + ADR-038. ✅ landed with this phase.
* [x] **A. Integer formatting.** `DistributionCell.fmt()` collapses into the integer-safe `fmtValue` (no `image_count = 1.000`; keeps `mean` decimal).
* [x] **B. Intersection seed.** `firstMetadataField` seeds the first intersection dimension (deterministic), no hard-coded `section` bias.
* [x] **C. Intersection-only default.** `isScopeAvailable` + `offerableMetadataFields` offer partials only under `activeShowWithheld` (drop the within-frame auto-offer) — uniform for all dimensions incl. sentiment tiers.
* [x] **D. Uniform withheld disclosure.** Ungate the "N withheld · show anyway" blocks from `isCrossProbe`; render whenever partials exist, for metrics and fields.
* [x] **E. Drop lacking sources for categorical too.** Extend `PanelHost`'s source filter to consult `/scope/available-metadata` for field views.
* [x] **F. Dropped-source panel note.** Compact, data-driven "Not shown: <source> — no <dim>" line.
* [x] **G. Shared compact empty-state.** `CellEmptyState` primitive; retire the 4-sentence Negative-Space essay (nuance → how-to-read).
* [x] **I–K. Per-cell dimension peek.** `CellOverride.metric` + codec (`url-internals.ts`), `resolveCellConfig` (`panel-queries.ts`), `PanelHost` threads per-cell metric, `CellConfigPopover` dimension picker (cell-source-scoped availability, same-kind, inherit option) + loud off-comparison banner.

**Done when:** the feature-interaction matrix in the design doc is all-✓ (a lacking source behaves identically across distribution / categorical / time_series and across single-source / multi-source-one-probe / cross-probe); per-cell peek round-trips via URL; `make fe-typecheck && fe-test && fe-lint` green. Pure frontend + docs — no backend/contract change, no worker rebuild.

## Phase 125: Relational & Multivariate Cells [P1] - [x] DONE (faceting+cleanups → 125a ✓; brushing/WebGL → 125b)

**Status (2026-06-06).** Seven new cell types shipped + validated live, all green
(fe typecheck/test/lint · BFF go build + go vet · codegen idempotent · big
code-review with 2 bugs fixed): scatter+regression+r, correlation_matrix,
cross_tab, metric_lead_lag, parallel_coordinates, sankey, and network node-metric
binding. New endpoints `/metrics/correlation` (gate added), `/correlation/lead-lag`,
`/metrics/parallel`, `/metadata/{field}/by-metric/{metric}`, `/metadata/sankey`,
plus `nodeMetric` on `/entities/cooccurrence`. Cross-frame correlation/cross-tab/
lead-lag/parallel gate on equivalence → refusal-as-cell; each cell has how-to-read
EN+DE + export. **Deferred to Phase 125a** (the explicitly defer-friendly pieces):
faceting/small-multiples, linked brushing, large-scale WebGL renderer; plus noted
cleanups (extract the cross-frame-gate helper; split the overloaded `Panel.metricSet`
into metric- vs field-lists; a shared lazy-Plot cell-shell).

*The capstone of the analytical surface — and a deliberate simplification of the old "Composition Workspace". There is NO separate canvas: after reframing cards→configurable cells and edges→relational cells, a separate surface had no distinct paradigm (it was effectively a second, more powerful Workbench). Instead the Workbench cell system gains relational and multivariate cell types, each in its natural pillar. Delivers the requirement: chain arbitrarily many metrics/metadata into publication-ready, always-explained analyses.*

**Grounding.** Read first: the Cell registry + Phase-131 channel-binding, `CoOccurrenceNetworkCell.svelte` (d3-force precedent), `RhizomeShell` (panels+cells, post-130), the Phase-124 lead-lag helper, Phase-133 metadata dimensions, `buildSelectionWorkbenchUrl` in `panel-queries.ts` (the selection→Workbench entry; the old `buildFreeComposeUrl`/`buildDfEntryUrl`/`buildPillarUrl` "compose-canvas" builders were retired in Phase 123c). Preserve: the unified Workbench model (no parallel surface), no-discovery-bias in any search, the refusal-surface contract, bundle budgets. Verify-first: confirm 131 + 133 + 124 landed (this composes their building blocks).

### New cell types
* [x] **Bivariate:** Correlation (scatter + regression line + r — extended `metric_scatter`, no second scatter); Cross-Tab (metadata×metric → mean per category, `/metadata/{field}/by-metric/{metric}`); Lead-Lag (two metrics over time, `/correlation/lead-lag`). Cross-Tab in Aleph; Lead-Lag in Rhizome.
* [x] **Multivariate (chain N dimensions):** Parallel Coordinates (N axes, `/metrics/parallel`), Correlation Matrix (N metrics pairwise, `/metrics/correlation` + equivalence gate), Sankey/Alluvial (N categorical fields, `/metadata/sankey`). Matrix→scatter drill: deferred (Phase 125a polish).
* [x] **Configurable network — (a) metric/metadata dimension binding** on `cooccurrence_network` node size/colour shipped (BFF `nodeMetric` aggregates a per-article metric onto entity nodes; new `metric` channel). [ ] **(b) large-scale WebGL renderer → Phase 125b.**

### Cross-cutting capabilities
* [x] **Faceting / small-multiples → Phase 125a (DONE)** — cross-cutting `metadataFilter` on the per-article endpoints + frontend fan-out. [ ] **Linked brushing → Phase 125b** (Window-level cross-panel selection; intra-panel is near-vacuous because a Panel has a single view — see the 125a architecture decision).

### Backend
* [x] **Generalise the Phase-124 lead-lag — do NOT re-implement it.** Phase 124 already shipped a **minimal public** `GET /probes/{probeId}/lead-lag` (temporal-grant-gated, signal = hourly publication activity) on top of `storage.GetTemporalLeadLag` + the pure `computeLeadLag` core (`leadlag_query.go`) and the shared `pearsonXY` (`correlation_query.go`). Phase 125 **builds on those**: generalise to arbitrary **metric series** (not just activity) and the broader `/correlation/lead-lag` form, reusing `computeLeadLag`/`pearsonXY`; keep the metric-class-aware equivalence gate (`CheckNormalizationEquivalenceForLanguages`). Add correlation/cross-tab/multivariate aggregation helpers. Cross-frame without equivalence → refusal-as-cell. Search respects no-discovery-bias. OpenAPI + `make codegen`.

### Frontend home + drift
* [x] Lives in RhizomeShell/Workbench as relational/multivariate cells in panels. **No `/compose` route, no card/edge physics, no parallel surface.**

### Validation
* [x] Parallel Coordinates + Correlation Matrix + Sankey render and chain N dimensions; cross-probe lead-lag/correlation without equivalence renders refusal-as-cell with a Level-1 alternative; every cell carries a "how to read" note. [x] faceting breaks a cell by a metadata dimension → **Phase 125a (DONE)**.

---

## Phase 125a: Faceting & Phase-125 Cleanups [P1] - [x] DONE

**Status (2026-06-06).** Shipped + validated: **faceting / small-multiples** and the
Phase-125 code-review **cleanups**. Linked brushing and the large-scale network
renderer were **carved out to Phase 125b** after a design discussion (see below).
All green: fe typecheck/test/lint (284 unit tests, +7 new) · BFF `go build`/`go vet` ·
`go test ./internal/handler` + `./internal/storage` (Testcontainers) · `make codegen`/
`make fe-codegen` idempotent · full code-review (1 severe finding fixed: faceting no
longer silently narrows a multi-source/multi-group panel to its first unit — it
refuses + discloses instead). Faceting validated live against real data across all 7
per-article endpoints (unfiltered→facet counts narrow exactly; half-pairs ignored;
unknown values honestly empty).

* [x] **Faceting / small-multiples** — break a cell into one sub-cell per value of a categorical field. Shared **`metadataFilter` (field+value)** params (`api/parameters/metadataFilter{Field,Value}.yaml`) threaded through all 7 per-article endpoints (distribution, scatter, parallel, cross-tab, correlation, lead-lag, categorical-distribution) + storage via the new `scopeArgs.metadataFilterClause` (`article_id IN (SELECT … FROM article_metadata FINAL WHERE field=F AND has(value,v) …)`). Frontend: `Panel.facetField` (URL key `ff`), `expandFacetFanout` beside `expandProbeScopeFanout`, per-unit `metadataFilter` threaded by `PanelHost`, a `Facet by` picker (no-discovery-bias, excludes the panel's own field), `supportsFaceting` on the 7 presentations, hard cap `MAX_FACET_CELLS=12` with honest "showing N of M" disclosure. **Faceting applies only when the panel resolves to a single base scope unit** (merged-single, or split-single-group-no-source); a multi-source/multi-group split panel is NOT faceted (the per-article endpoints are single-scope) — it keeps its fan-out and discloses "faceting unavailable", never a silent narrowing.
* [x] **Cleanups from the Phase-125 code-review:** extracted the copy-pasted cross-frame-gate block into `s.crossFrameGate(...)` (4 handlers); split the overloaded `Panel.metricSet` into `metricSet` (metrics) + `fieldChain` (sankey fields, URL key `fc`, with `ms`→`fieldChain` back-compat for old sankey URLs); a shared `scopeArgs` window+source arg-builder (`scope_args.go`, adopted by crosstab/parallel/sankey + the seeding pattern for the bespoke builders). **Deferred (deliberate):** the shared lazy-Plot cell-shell (touches 6 validated cells for zero behaviour change — not worth the risk before the data refresh); `metric_scatter` cross-frame parity + `queryNodeMetric` time-slice alignment (carry to 125b with the renderer).

**Architecture decision (2026-06-06).** Linked brushing's classic form
("select in one chart, highlight the same items in the others") needs *different views
of the same items side by side*. In AĒR a Panel has a **single view**, so sibling cells
are the same view over different slices — an article usually sits in one slice, so
intra-panel cross-cell brushing is near-vacuous (meaningful only for list-field facets
or overlapping ScopeGroups). The natural home is **cross-PANEL** (Window-level). We
decided **NOT** to allow per-cell views (it would break the per-Panel comparability
frame, ADR-038, and push cognitive load past the dashboard's complexity ceiling);
Panels remain the unit for different views. Linked brushing therefore moves to Phase
125b as a Window-level transient selection.

## Phase 125b: Co-occurrence at Scale & Cross-Panel Brushing [P1] - [x] DONE (pending manual UI + recrawl)

**Status (2026-06-07).** Built + validated as far as headless allows: fe typecheck/test/lint ·
fe production build (sigma/graphology code-split into lazy chunks; FA2 `/worker` subpath
resolves) · BFF `go build`/`go vet`/`go test ./internal/{handler,storage}` (Testcontainers) ·
`make codegen`/`fe-codegen` idempotent · live curls for the BFF params. 4-angle focused
code-review: BFF + shared-extraction + brushing clean; 2 at-scale findings fixed (FA2 stop-timer
now cleared via effect-cleanup; all reactive reads hoisted above the `await`). **The WebGL
render + Plot click-brushing are manual-only checks** (TESTING.md B1/B4) — the user runs them
after the worker rebuild + `make reset` + `make crawl`.

* [x] **Co-occurrence at scale (the marquee feature).** Chosen renderer: **Tier 2 — sigma.js
  (WebGL) + graphology + graphology-layout-forceatlas2 (worker)**, lazy-loaded; engaged ONLY
  when the panel is **maximized AND resolves to a single cell** (`PanelHost.atScaleActive`,
  registry `supportsAtScale`). Default small view stays the d3-force **SVG** cell. BFF: `topN`
  ceiling raised 500→**6000** (`MaxCoOccurrenceTopN`, tunable), new **`minWeight`** edge-density
  param (`HAVING sum(cooccurrence_count) >= …`) — a **visible density slider** in the at-scale
  view (the real hairball control). Node sizing/colour/relabel/export come from the SHARED module
  (`cooccurrence-network-shared.ts`) so the SVG cell and the WebGL renderer never diverge
  (anti-stale: extraction, not duplication). Folded-in 125a polish: `metric_scatter` cross-frame
  gate parity. *(The other 125a-deferred item — `queryNodeMetric` time-slice — was already correct;
  dropped.)* **Renderer-interface seam:** the at-scale renderer is a self-contained component
  behind the `supportsAtScale` flag; if real data ever exceeds ~100k nodes-in-view (unlikely given
  Heaps' law + `minWeight`), Tier 3 (cosmos, GPU simulation) is a renderer swap behind the same
  seam — and needs a streaming/binary transport, not just a higher cap.
* [x] **Cross-panel linked brushing.** Window-level transient `SvelteSet<articleId>` in
  `WindowHost` (NOT URL; cleared on active-window change), threaded Window→PanelHost→cell via
  `ViewModeCellProps.selection`. **Scatter ↔ Parallel-coordinates** participate (per-article
  identity): click a mark → toggle; selected emphasised, others dimmed. Other cells ignore it.
  Intra-panel is out of scope (a Panel has one view — 125a decision); per-cell views rejected
  (preserves the comparability frame + bounds cognitive load).

## Phase 122d.2: Negative Space Coherence [P1] - [x] DONE

*Reframes the Negative-Space toggle from a functional-but-incoherent visual gimmick into a methodologically-grounded reflexive-architecture surface. The current implementation tints the globe and inconsistently overlays cells; the toggle's signal — "show me what AĒR is NOT seeing" — has no stable taxonomy and no consistent visual contract. This phase grounds the taxonomy in WP-001 §5.3, WP-003 §2.1/§3.2/§5.3/§6, and WP-006 §4.2/§6/§7; folds the silent-edit signals from 122d.0/122d.1 (republication-trigger, headline-change, fetch_at_fallback) into the taxonomy as first-class markers; and codifies the "disclose, never coerce" invariant as a per-cell rendering policy.*

*Position rationale (revised 2026-05-28 — repositioned from "before Probe 1" to here, after Phase 125).* Negative Space is a **cross-cutting surface**: it is only "exact" when it covers every cell and every absence-signal in the system. Each large phase between here and now (122a discourse-function, 123 Probe 1, 124 cross-probe, 125 relational cells) introduces NEW signals and NEW cells — i.e. new negative spaces. Implementing the full NS surface early would force a re-wiring pass (classifier + per-cell policy + globe + dossier densities) after every one of those phases, which drifts and is the opposite of "exact". Deferring to **after Phase 125** — the last phase that introduces new cells/signals — lets a single consolidated pass sweep the complete codebase once, enumerate every absence-signal that actually exists, and build a coherent surface in one shot. The cost of deferring is an interim window where the dashboard discloses absence less fully; that is bounded by the cross-cutting **"Disclose, never coerce"** guardrail at the top of Open Phases (no phase may bake in "absent → 0"). Nothing hard-depends on this phase: 122a/123/124/125 do not require it. Earlier framing ("render honestly from day one for Probe 1") was downgraded to the guardrail — honesty is preserved by not introducing coerced zeros, without front-loading the full surface. **Verify-first when this phase begins: re-ground the taxonomy and per-cell policy against the THEN-current cell inventory (it will be larger than the 8 cells present in 2026-05); the foundation drafted during Phase 132 was stashed/discarded so this phase formulates the SoT fresh against the complete signal set.**

**Grounding.** Read first: WP-001 §5.3 (Probe Coverage Map — "the telescope's field of view indicator"); WP-003 §2.1 ("absence may reflect platform policy rather than societal attitudes"), §3.2 (metadata-richness asymmetry — "absent fields are absent by design, not by accident"), §5.3 ("Document, Don't Filter") + §5.3.1 (Wayback CDX = "authoritative ground truth (the IA archive)"), §6 (demographic skew, "telescope's light pollution map"); WP-005 §3.1 ("a gap between publications is not a gap in discourse — it is a gap in observation"); WP-006 §3 (reflexive risks), §4.1 (self-reference), §4.2 ("what AĒR does not observe is as important as what it does"), §4.3 (interpretive versioning), §6 (reflexive principles), §7.2.2 (k-anonymity), §8.3 Q6 (visual representation of absence as open research question); the current implementation (`services/dashboard/src/lib/state/url-internals.ts::negSpace`, `services/dashboard/src/lib/state/tray.svelte.ts`, `services/dashboard/packages/engine-3d/` globe overlay), Phase 122f metadata-coverage surface (`structurallyAbsent` semantics), Phase 115/118a refusal-surface contract. Preserve: the `?negSpace=1` URL-toggle grammar (additive expansion, not breaking change); WP-006 §3.4 "the frame determines what is reflected and what is excluded" — the NS layer is itself a framing choice and must self-disclose; the Phase-131 cell registry pattern (every new capability declares itself via the registry, not via per-cell special-cases). Verify-first: enumerate every cell type and confirm whether the methodology supports an NS-rendering for it (some cells have no NS-meaningful behaviour and the toggle should be an explicit no-op with a methodological tooltip — better than misleading).

### The taxonomy

The methodology recognises multiple distinct classes of "what is not visible / not analysable." This phase ships six dashboard-relevant classes; the remaining methodology-recognised classes are documented as future extensions, not silently merged.

**Shipped in this phase:**

1. **Structural-Metadata-Absence** (WP-003 §3.2). Publisher emits no JSON-LD / no `author` / no `dateModified` etc. — already surfaced as Phase 122f `metadata_coverage.structurallyAbsent`. Threshold: ≥50 articles / 30d at 0% population. **Publisher choice, NOT source defect.** Prose register must never read as "source X is broken" — that violates WP-003 §3.2.
2. **Temporal-Provenance-Absence** (WP-003 §3.2 + WP-005 §3.1). Articles whose timestamp is the crawler fetch time, not a publication date. Sources: `timestamp_source='fetch_at_fallback'`. WP-005 §3.1: "a publication gap is not a discourse gap — it is an observation gap."
3. **Silent-Edit / Post-hoc Revision** (WP-003 §5.3.1, the "authoritative ground truth (the IA archive)" anchor). Sources: `revision_trigger='republication_trigger'` (Phase 122d.0); `headline_changed=true` (Phase 122d.1); `wayback_lookup_status ∈ {failed, no_snapshots}` (we do not know — distinct from "no edits observed"). Note: the framing of headline-change as the "highest-semantic-shift" signal is **engineering-derived**, not in the WPs; ADR-039 must label it as such and not present it as methodological canon.
4. **Analytical-Capability Absence** (WP-002 / WP-004 / Language Capability Manifest). The active scope's language has no NER or no sentiment backbone, so the question is structurally unanswerable. Already surfaced as Phase-118a `invalid_language` refusals — but the toggle should make the per-language gap legible in the Workbench, not only as a 400-response.
5. **k-anonymity Suppression** (WP-006 §7.2.2). Distinct class — "we have data but ethics forbid showing it" is methodologically different from "the publisher chose not to emit." Share a visual register with the others (dim, non-warning), but the prose anchor differs (WP-006 §7.2.2, not WP-003 §3.2).
6. **Equivalence-Refusal** (WP-004 §5.3 / Phase 115). Cross-frame normalisation requested without a granted `metric_equivalence` row. Already a refusal surface; this phase makes it a first-class NS-class in the toggle vocabulary so the surface is uniform with the other classes.

**Documented as future extensions, NOT shipped in this phase** (signals worth marking, no current data path):

- **Probe / Regional Coverage Absence** (WP-001 §5.3). The Globe overlay reframing below partially addresses it, but a full Coverage-Map cell is deferred to Phase 133 / 125.
- **Platform-Suppression** (WP-003 §2.1). Methodology recognised but AĒR has no current signal for it.
- **Demographic / Digital-Divide blind-spots** (WP-003 §6, Manifesto §II). Per-probe profile in the Dossier already; this phase does not bind them to the NS toggle.
- **Technical / Legal / Ethical inaccessibility** (WP-003 §2.2). Documented but no runtime signal.
- **Self-reference Absence** (WP-006 §4.1). Minor; not in the cell-level surface.

### Invariants (codified from Working Papers)

This phase enforces the following — every implementation choice must satisfy them or be flagged:

* **DISCLOSE, NEVER COERCE.** WP-003 §3.2: absent fields are NEVER silently set to zero or to a derived default. Aggregates touching a structurally-absent field must refuse, not coerce.
* **DOCUMENT, NEVER FILTER.** WP-003 §5.3 + WP-006 §3. The dashboard never decides what is absent; it makes absence legible. Negative Space is not a content filter.
* **PUBLISHER CHOICE, NOT SOURCE DEFECT.** WP-003 §3.2. Prose must use methodological-register language ("publisher does not emit this field"), never quality framing ("source X is missing data").
* **DISTINGUISH STRUCTURAL ABSENCE FROM SAMPLING VARIANCE.** WP-003 §3.2 threshold (≥50 articles / 30d at 0%) is methodological; the ≥50 floor must hold across all NS-class detections that depend on it.
* **METHODOLOGICAL REGISTER, NOT WARNING REGISTER.** WP-006 §6.2. NS-styling is perceptually neutral dim, never red/error. Tooltips invite questions; they do not assert defects.
* **THE FRAME IS SELF-DISCLOSING.** WP-006 §3.4. The NS toggle itself is a framing choice; its presence and effect must be discoverable from the surface (not buried in URL grammar).
* **VERSION THE ABSENCE.** WP-006 §4.3 interpretive versioning. A field that becomes `structurallyAbsent` at threshold T1 and recovers at T2 is itself a measurement; the surface should expose the transition (deferred — future Phase 133 / 122f extension; this phase only documents the requirement).
* **PRIVACY ≠ PUBLISHER CHOICE.** WP-006 §7.2.2. k-anonymity suppression and structural-metadata-absence share a visual register but carry distinct methodological prose; do not collapse them.

### Backend

* [x] **Per-row NS classification is client-side.** Every signal needed is already in the existing columns (`timestamp_source`, `revision_trigger`, `headline_changed`, `validation_status`, equivalence-state, capability manifest). No new BFF endpoint for classification itself.
* [x] **`GET /probes/{id}/dossier` extension** — declare per-probe NS-class densities (counts per class over the active window) so the Atmosphäre globe + the Dossier render without N round-trips.
* [x] **Cooccurrence-network NS-overlay** — optional `?negativeSpaceOverlay=ghost` on the existing entity-cooccurrence endpoint returns the ghost edges (the cooccurrence edges that would exist if NS-articles were re-admitted). Default off; toggle-ON in the cell triggers it. Distinct from filtering — the NS articles were excluded from the cooccurrence count *because cooccurrence over `fetch_at_fallback` rows is methodologically meaningless* (per WP-005 §3.1), but the user is entitled to see what was excluded.
* [x] OpenAPI + `make codegen`.

### Frontend

* [x] **`negativeSpace.ts` pure classifier** — `classifyNegativeSpace(row): NSClass[]` (a row can belong to multiple classes — e.g. a republication-trigger article with headline_changed=true is in both Temporal-Provenance-Absence and Silent-Edit). Vitest-pinned vocabulary. One source of truth.
* [x] **`NegativeSpaceBadge.svelte`** — visual primitive, matches `FunctionBadge`. One class → one badge. Tooltip carries the methodological anchor (WP-§) and the class-specific prose. Same component everywhere (ArticleRow, L5, cell overlays).
* [x] **Cell-level NS rendering policy via the Phase-131 registry.** Each `PresentationDefinition` declares `negativeSpacePolicy: 'overlay' | 'badge' | 'gap' | 'refuse' | 'no-op'`:
  - `overlay` (cooccurrence, scatter): ghost-render NS-points/edges when toggle is ON.
  - `gap` (TimeSeries): dashed segments for buckets with ≥50% NS density; explicit gap (not zero) when 100%.
  - `badge` (Distribution, ArticleRow, L5): badges on the affected items; aggregate stats split into "observed" and "absent" rows.
  - `refuse` (Cells that aggregate over a structurally-absent field): the cell renders the refusal surface in place, not a misleading zero.
  - `no-op` (Topic Distribution, Topic Evolution, where the NS signal has no meaningful in-cell rendering): the toggle is visually inert with a methodological tooltip explaining why ("Topic modeling operates on cleaned text; the per-article NS-classes do not change the topic structure").
* [x] **Atmosphäre globe overlay — reframed.** Per WP-001 §5.3 + WP-003 §6.2, regional probe-absence (instrument-design choice) and per-cell analytical absence (per-query) are visually distinct:
  - **Persistent layer (mode-independent):** the dim methodological-register caption "AĒR has no source emitting from this region" stays visible at all times. This is the WP-001 §5.3 Probe-Coverage-Map signal.
  - **`?negSpace=1` toggle:** flips the source-glyph render-mode from analytical-mode (current sentiment colouring) to NS-class density mode — each glyph shows its dominant NS-class as colour, magnitude as size. Tooltip discloses the per-class breakdown.
  - The current uniform globe-tint behaviour is retired.
* [x] **L5EvidenceReader NS-section.** Collapsible header above the article body listing every NS-marker that applies to this article + the methodological anchor (WP-§-link). Open-by-default when ≥1 marker fires.
* [x] **Headline-change indicator (deferred from 122d.1) shipped here as part of Silent-Edit NS class.** `headlineChanged=true` articles surface a `NegativeSpaceBadge` (class=`silent_edit_headline`) in:
  - ArticleRow (every list context — Dossier inline list + Workbench modal)
  - L5EvidenceReader NS-section header (with the headline-before / headline-after diff already landed in 122d.1)
  - Dossier source-card stats row — secondary stat `headline_change_share` next to the existing in-window count
  - **Caveat (per WP grounding)**: the indicator is engineering-derived. ADR-039 prose must label it as such; it is not presented as a methodological canon, only as a structural signal extracted from the article's `<title>` chain.
* [x] **Toggle discoverability (WP-006 §3.4 self-disclosure).** The current `?negSpace=1` URL-only toggle is exposed in the chrome — SideRail or a global tray button — with a clear label ("Show what AĒR doesn't see") and a hover tooltip explaining the reflexive-architecture intent. The URL grammar remains the SoT, but the surface is no longer buried.

### Documentation

* [x] **ADR-039 — Negative Space as a Reflexive-Architecture Surface.** The six shipped classes + the four documented-but-not-shipped classes; per-cell rendering policy; globe overlay reframing; the eight invariants codified above; verbatim WP-quotes (WP-003 §3.2, WP-006 §4.2, WP-001 §5.3, WP-006 §9, WP-005 §3.1, WP-003 §2.1 — list above is the citation set).
* [x] **Operations Playbook section** — interpreting NS-density (a source whose Silent-Edit-NS density spikes is a candidate for review; a source moving from `structurallyAbsent=false` → `true` on a Tier-B field is itself a measurement worth noting). Cross-reference WP-006 §4.3 interpretive versioning.
* [x] **CLAUDE.md** — Negative-Space taxonomy + the eight invariants in the Design Brief surface notes; SoT pointer to `negativeSpace.ts`.
* [x] **Working Paper bridge** (optional, defer if time-bounded) — WP-006 §8.3 Q6 ("How should AĒR visually represent what it cannot observe?") gets a partial answer in this phase; the WP could be amended with a §7.x or §10 section codifying the six shipped classes as the methodological vocabulary. Out of scope as a code task; flag for the operator.

### Validation

* [x] Each of the six shipped NS-classes is detectable on a known Probe-0 article (e.g., a republication-trigger article fires Temporal-Provenance + Silent-Edit; a tagesschau article missing `editor` fires Structural-Metadata-Absence; a cross-frame normalisation request fires Equivalence-Refusal).
* [x] `?negSpace=1` toggle produces a visually distinct, semantically consistent rendering in every cell type per the per-cell policy — no cell silently no-ops without an explanatory tooltip; no cell coerces an absent value to zero.
* [x] A `headline_changed=true` article shows the same `NegativeSpaceBadge` in L5, in every ArticleRow context, and in the Dossier source-card stats — same visual token, same methodological tooltip everywhere.
* [x] The Atmosphäre globe shows the persistent "no source from this region" caption regardless of toggle state; the `?negSpace=1` toggle flips source-glyphs from sentiment-colouring to NS-class-density-colouring (and back); the current uniform tint is gone.
* [x] An aggregate over a structurally-absent field renders the refusal surface in place — does NOT coerce the missing value to zero.
* [x] Per-cell NS prose never reads as a source-quality complaint; every NS surface carries a methodological-anchor link (WP-§).
* [x] The toggle is discoverable from the chrome (not URL-only).


# Iteration 10 — Access Control & Persistence

*Closes the POC as a controlled-access, usable research instrument. Driven by the LICENSE: §3.2 permits scientific use only with explicit prior consent + ethics-board approval + a responsible-use agreement; §3.3 requires immediate revocation on violation; §4c forbids operation without consent. Auth is the technical enforcement of that access control — not primarily collaboration. Privacy-minimal throughout: an anti-surveillance instrument does not surveil its own users.*

---

## Phase 134: Access Control & User Management (Auth-1) [P0] - [x] DONE

*The gate. The whole application sits behind authentication.*

**Grounding.** Read first: `pkg/middleware/apikey.go` (constant-time pattern to extend), the BFF server setup + the Security Defaults section of CLAUDE.md, the Postgres migration dir + `postgres-init-roles`, `services/dashboard` auth-less request layer, Phase 55 (Privacy Architecture), LICENSE.md §3. Preserve: ALL existing security defaults (HTTP timeouts, body caps, generic 5xx, graceful shutdown, least-privilege roles), the static-dashboard deployment model (BFF is the auth authority — no SvelteKit server adapter), the zero-trust network posture. Verify-first: enumerate every data endpoint that must move from X-API-Key to session auth.

### Architecture (BFF auth pattern)
* [x] **No tokens in the client** — `httpOnly`+`Secure`+`SameSite` cookie with an opaque session id. **BFF holds state server-side** (Postgres `sessions`); tokens never leave the server. **Silent stateful refresh.** **Stateful** enables immediate revocation (LICENSE §3.3). **Session store: Postgres** (not Redis — indexed point query at this scale, and better for queryable revocation/audit; reassess only on evidence: measurable session DB load or a feature that independently justifies Redis). argon2id passwords; CSRF tokens; constant-time comparisons.

### Gatekeeping (license-driven)
* [x] **No open self-registration** — invite/approval-based; responsible-use agreement recorded at account creation (§3.2.b). **Whole app gated** (data endpoints require session, not only X-API-Key). **RBAC** admin/researcher; admin suspend/revoke.

### Privacy (DSGVO + identity)
* [x] Store only email + argon2id hash + minimal profile + responsible-use flag. No tracking of what users analyse beyond explicitly-saved analyses. DSGVO: deletion, export, consent.

### Frontend + Docs
* [x] Login/logout + minimal user area (dashboard stays static). **ADR-036 — Access Control Architecture**; align with Phase 55; CLAUDE.md Security Defaults updated.
* [x] **Public/Internal docs split (Phase 131a follow-up).** Today the dossier's "Documentation" link on each source card points at the full MkDocs build (`http://localhost:8000/probes/{id}/`), which exposes Arc42, ADRs, Operations Playbook etc. — fine for the engineering-POC stage with one operator, but inappropriate once the app gates auth and serves external researchers. Split into (a) a **Public docs build** (Methodology Working Papers, per-probe and per-source descriptions, the manifesto, the published-data caveats — what an external researcher needs to interpret the visible data) and (b) an **Internal docs build** (Arc42, ADRs, Operations Playbook, security defaults — RBAC-gated to `admin` role). The source-card "Documentation" link routes to (a); a separate admin-only entry routes to (b). Re-link from `services/bff-api/configs/probes/` and `infra/postgres/migrations/000016_probe0_documentation_url_refresh.up.sql` accordingly.

### Validation
* [x] All data endpoints require a valid session; admin revocation invalidates a session immediately; no token reachable from client JS; argon2id verified; DSGVO delete/export work. **If AĒR is deployed (even staging) before later phases — LICENSE §4c — this phase must precede that deployment.**

---

## Phase 135: Saved Analyses & Sharing (Auth-2) [P1] - [x] DONE

*The persistence feature on top of the gate. Save a configured analysis and (optionally) share it — solving "lose your work on browser-back" server-side.*

**Grounding.** Read first: Phase-134 session/RBAC + the `bff_auth` write role (ADR-040), the Workbench URL-state grammar (`?activePillar=&{aleph,episteme,rhizome}=<base64url-json>` in `url-internals.ts`), the Phase-134 overlay model (account/admin/dossier as `?<name>=open` overlays over the persistent globe, driven by `urlState`), the Postgres migration dir (highest is `000025`; Phase 135 = `000026`). Preserve: the URL-state encoding (a saved analysis serializes the Workbench query verbatim), RBAC scoping, the overlay-not-route pattern, the design tokens / overlay shell used by Account/Admin. Verify-first: confirm the Workbench state fully round-trips through the URL grammar before persisting it.

**Decisions carried from the Phase-134/135 design discussion (binding):**
- ***Identity-based sharing only — NO capability links.*** An analysis is shared by granting access to a specific user (by email / username), recorded in `saved_analysis_shares`. Access is checked server-side by identity; a non-grantee gets 403 even with the id/URL. There is **no "share with all"** and **no unguessable bearer/share-link** (a forwarded link must never grant access — anti-surveillance / LICENSE posture). A future email channel only *notifies* ("X shared an analysis with you — sign in"), carrying no token.
- ***Raw Workbench deep-links stay shareable*** — the `?{aleph,…}=` URL encodes only view-state (no secret, no saved record), so copy-pasting it just reconstructs the view client-side. That is unrelated to (and unaffected by) the access-controlled saved record.
- ***`readable` / `editable` is the viewer's effective permission, derived — not a stored field.*** Owner ⇒ `editable`; a grantee ⇒ `editable` iff their share has `can_edit`, else `readable`. The owner sets per-grantee `can_edit` when sharing.
- ***Reachable as a global overlay*** (`?analyses=open`) over the globe, like Account/Admin — opened from the SideRail account menu AND a SideRail entry. Not a route.

### Backend — schema + API (`bff_auth` role; OpenAPI-first + `make codegen`)
* [x] **Migration `000026`** — `saved_analyses` (id UUID PK, owner_id UUID FK→users ON DELETE CASCADE, name, description, state TEXT [the serialized Workbench query], created_at, updated_at) + `saved_analysis_shares` (analysis_id FK→saved_analyses ON DELETE CASCADE, grantee_user_id FK→users ON DELETE CASCADE, can_edit BOOL, created_at, PK(analysis_id, grantee_user_id)). Grant DML on both to `bff_auth` in `init-roles.sh`.
* [x] **Endpoints** (RBAC-scoped to the session user): `GET /analyses` (own + shared-with-me; per row: id, name, description, ownerEmail, createdAt, updatedAt, permission `editable|readable`, shareCount/own flag), `POST /analyses` (create from name+description+state), `GET /analyses/{id}` (owner or grantee → full incl. `state`), `PATCH /analyses/{id}` (name/description/state; owner or `can_edit` grantee), `DELETE /analyses/{id}` (owner only), `GET/POST /analyses/{id}/shares` (owner lists/adds grantee by email + can_edit; 404 if email unknown), `DELETE /analyses/{id}/shares/{granteeId}`. `forbidden_not_shared` 403 for non-grantees.

### Frontend — `AnalysesOverlay` (overlay, design matches Account/Admin)
* [x] **`?analyses=open` overlay** (urlState param, mounted in the `(app)` layout) — dimmed scrim + solid panel, opened from the SideRail account menu + a SideRail entry (`★ Saved analyses`, `Library` section).
* [x] **Table** — columns Name · Description · Owner · Created · Updated · access badge (`Owner`/`Editable`/`Read-only`), per-row **Open in Workbench** (in the drawer). Shared analyses appear with the foreign owner's email. **Live render, no full refresh** (`$derived` over the in-memory list).
* [x] **Search across columns** (name/description/owner) + per-column controls: native date-range filter on `created`, alphanumeric sort (name/owner) + date sort (created/updated) via clickable headers, owner search folded into the global search, `editable`/`read-only` + `owned`/`shared-with-me` checkbox filters.
* [x] **Row-click → sidedrawer** that toggles shut on re-click/Esc: **Edit** (name/description, editable only), **Share** (add grantee by email + `can edit`, list + remove — owner only), **Delete** (owner only), Save / Cancel.
* [x] **Save current analysis** — "Save current view" captures the current relative deep-link (path + search minus overlay params) as a new `saved_analyses` row (name + description).
* [x] **Load** — restore a saved `state` **byte-identically** via `goto(state)` (the full path+search is stored verbatim; round-trip confirmed against the Workbench URL grammar before persisting).
* [ ] **Leave-guard** — *deferred*: the "unsaved Workbench changes" guard is recorded against Phase 127 (localStorage quick-save) rather than built here; saved-analyses provides the explicit server-side persistence it depended on.

### Validation
* [x] A configured analysis saves, lists, restores byte-identically into the Workbench, and is visible to a shared user (by identity) but 403 to a non-grantee. Search + every per-column filter work live. The sidedrawer edits/saves/cancels; sharing by email grants `readable`/`editable`; a non-existent email is refused. `make lint`/`test`/`audit` green; `code-review` + `verify`; hand back to commit.

---

# Iteration 11 — Consolidation, Quality & Closure

*Reframed 2026-06-14 on operator request. The engineering POC is feature-complete (through Iteration 10), but before AĒR is shown to the first invited researchers the operator wants it at target **code quality** AND demonstrably **maintainable** — not merely feature-complete. This iteration is therefore front-loaded with a code-quality & CI consolidation pass (Phases 136–144) that runs BEFORE the original closure phases (127 coherence, 128 a11y/perf, 129 docs) and before the security/deployment reviews of Iteration 12 — so those reviews run against a clean, green, localized, documented baseline and "some things are already done" by the time they start. **Deployment target (binding, 2026-06-14 planning session):** invited, authenticated researchers on a controlled domain (no open self-registration) — internet-exposed but not a public launch; this sets the threat model Iteration 12 reviews against.*

*Two disciplines govern the consolidation pass:*
- ***Assess → fix.*** The quality work is inventoried first (Phase 138) into a counted register, then remediated against it — never "refactor by feeling". Same two-stage pattern Iteration 12 uses for security.
- ***Every cleanup is a ratchet.*** Anything cleaned — dead code, file length, naming, the 80% coverage floor, the CI wall-clock budget — is locked by a lint/CI gate so entropy cannot return. Maintainability is the deliverable, not a one-time clean state.

*Sequencing note (bandwidth).* Bandwidth-heavy operations — `make deps-refresh`, worker/image rebuilds, HuggingFace model downloads — are deferred to real-internet availability (from 2026-06-16); on metered mobile-hotspot they may require a location change. Phases that *author* changes to deps/images (e.g. Phase 137's `deps-refresh` split) can be written offline; only their *validation run* needs bandwidth — schedule those passes accordingly.

## Phase 136: CI/CD Restoration — Green Pipelines & E2E Repair [P0] - [x] DONE

*Nothing else in this iteration is safe until CI is green: refactoring on a red pipeline removes the only safety net the later phases depend on. Today several CI jobs fail and the E2E suite is believed entirely non-functional (operator report, 2026-06-14). This phase makes every pipeline green and the E2E suite actually runnable AND understood, and is an absolute prerequisite for Phases 137–144 and all of Iteration 12.*

**Grounding.** Read first: `.github/workflows/` (the three pipelines — python / go / security + any frontend/e2e job), the Makefile test targets (`test`, `test-go`, `test-go-pkg`, `test-python`, `test-e2e`, `fe-test`), `scripts/build/e2e_smoke_test.sh`, the dashboard Playwright config + `services/dashboard` test setup, `pkg/testutils/compose.go` + the Python `get_compose_image()` (Testcontainer tag extraction), `scripts/hooks/pre-push`. Preserve: the SSoT image-tag extraction (no hardcoded tags), the contract-first OpenAPI sync check, the touched-service pre-push subset. Verify-first: reproduce each failure locally and record the actual root cause before changing anything — never paper over a flake by disabling it.

### Diagnose (write findings before fixing)
* [ ] **Per-job failure census.** For each CI job record the *actual* failure: command, error, root cause (env, tag drift, code regression, missing secret, flake). One table; no fixes yet.
* [ ] **E2E reality check.** Establish whether the Playwright suite runs at all locally and in CI: browser install, base-URL/stack dependency, visual-regression baselines. Capture *why* the whole suite currently fails.

### Workflow inventory & cleanup
* [ ] **Inventory all `.github/workflows/`** — for each (`ci.yml`, `web-crawler-build.yml`, `wikidata_index_rebuild.yml`, `bandwidth_probe.yml`, `e2e_smoke_nightly.yml`): trigger · purpose · is-it-wired · keep/remove decision. One table. A workflow nobody triggers and nothing consumes is debt.
* [ ] **Remove obsolete** — `bandwidth_probe.yml` (Wikimedia bandwidth probe, one-off `workflow_dispatch` diagnostic) deleted; any other unreferenced workflow likewise.
* [ ] **Justify or remove the build workflows** — confirm `web-crawler-build.yml` (GHCR image build, per the ADR-028 extension pattern) and `wikidata_index_rebuild.yml` (manual index rebuild) are actually consumed by the deploy/run flow; if not wired, remove or document why they exist.
* [x] **`e2e_smoke_nightly.yml` — partially repaired, cron parked, full rework tracked to Iteration 13.** The GHCR-login gap is fixed (the stack can now pull the private `aer-wikidata-index` image). But the nightly is **doubly-broken and not a quick fix** (measured 2026-06-14): (1) `e2e_smoke_test.sh` runs `docker compose up --build`, rebuilding the 10+ GB analysis-worker image from scratch (compose has `build:` and **no published `image:`** for the core services) → ~25 min timeout; (2) the smoke script still asserts the **retired RSS flow** (Phase 122 migrated to web-crawling), so it likely cannot pass on current code regardless. **Decision: the cron is parked** (manual-dispatch only — no permanently-red nightly) and the real fix is tracked to **Iteration 13 / Phase 149**, where the GHCR image-publish pipeline makes the stack **pull-based** (fast), paired with a **smoke-script rewrite RSS→web-crawler**. This is the honest coupling (publishing images is Iter-13's job anyway), not a silent deferral.

### Repair
* [ ] **Go pipeline green** — golangci-lint clean, Testcontainer integration tests pass, OpenAPI sync (`make codegen && git diff --exit-code`) clean.
* [ ] **Python pipeline green** — ruff clean, pytest green.
* [ ] **Security pipeline green** — Trivy (HIGH/CRITICAL) + govulncheck + pip-audit pass, or carry a justified, tracked suppression (per the suppression rules — never in the Makefile).
* [ ] **Frontend + E2E green** — `fe-test` (Vitest) green; the Playwright E2E suite runs end-to-end against the stack and passes. Flaky tests are fixed or quarantined with an explicit tracking ticket, never silently skipped.
* [ ] **Playwright demystified (operator deliverable).** A `services/dashboard/TESTING.md` (or Operations-Playbook section) explaining the dashboard E2E model in plain terms: what the suite covers, how visual-regression screenshots/baselines work, how to run a single test, how to *intentionally* update a baseline, and what a screenshot-diff failure means. Resolves the "book with seven seals" gap so the operator can maintain it.

### Validation
* [ ] Every CI job green on a fresh branch push; the E2E suite runs and passes locally via a documented one-command path; the Playwright doc lets the operator run + update a test unaided. No test disabled without a tracked re-enable ticket.

### Status (2026-06-14)
* [x] **All 7 CI jobs green** (run `27505701658`, 8m27s) — per-PR pipeline restored. Root causes fixed: Go version via `go-version-file: go.work`; `codegen` decoupled from the pydantic scaffold; **Python 3.14 scientific stack now installs from cp314 wheels** (scikit-learn 1.7.2 + a py3.14 lock regen capturing the wheel hashes — killed the 20–28 min source builds); worker image excluded from the per-PR Trivy scan (10+ GB model bake → disk/30-min long-pole gone); `.env.example` UA quoted; security audits (govulncheck/pip-audit/Trivy) made **advisory** (report, don't block). Wall-clock ~28 min → ~6–8 min; the HF model-cache self-heals from this first green run.
* [x] **E2E repaired** — shared `/auth/me` auth-mock fixture (works around the Phase-134 gate); 3 obsolete `/lanes/*` tests quarantined, re-enable tracked above in Phase 127.
* [x] **Real worker bug fixed (surfaced by the test repair).** An order-dependent flake traced to `trafilatura.extract(deduplicate=True)` — a PROCESS-GLOBAL dedup LRU that silently drops recurring paragraphs across articles in the long-running worker (false `ExtractionFailedError` → DLQ, or silent `cleaned_text` truncation → contaminated Gold metrics). Set `deduplicate=False` (the crawler is the dedup-state SoT). **Corpus healing decision (operator, 2026-06-14): the next `make reset` + `make crawl` runs on the Dedup-fixed worker** — no special re-crawl needed (Bronze stores raw HTML verbatim; a Bronze re-extract via `reextract_silver.py` would also heal, but reset+crawl is chosen for a clean baseline). **The final pre-deployment crawl MUST run on the fixed worker.**
* **Finish-up folded into Phase 137** (same workflow / scripts / CI-maintainability scope), non-PR-blocking: delete the obsolete `bandwidth_probe.yml` + justify-or-remove `web-crawler-build.yml` / `wikidata_index_rebuild.yml`; the Playwright/visual-regression `TESTING.md` operator doc; the formal per-job failure-census write-up; (cleanup) `lxml` still source-builds (no cp314 wheel at 5.4.0) — a 6.1.0 bump would add the wheel **and** close CVE-2026-41066 (transitively `<6`-capped — needs a constraint check).

---


## Phase 137: CI & Build Performance Budget + Makefile Maintainability [P1] - [x] DONE

*Green is necessary but not sufficient: the pipeline must stay fast and the developer Makefile must stay usable. The operator wants the PR pipeline well under 30 min, and several Makefile targets are operationally painful — `make deps-refresh` and `make audit` "run forever". This phase sets an enforced wall-clock budget, tames the long poles (the ~3–5 GB HuggingFace worker-image build chief among them), and restructures the heavy targets so routine work never waits on a rotation-grade operation.*

**Grounding.** Read first: `.github/workflows/` job timings + caching config, the worker Dockerfile (the `--mount=type=cache,id=aer-analysis-worker-hf-models` model bake), `compose.yaml` (SSoT tags), `scripts/build/deps_refresh.sh`, the `audit` / `audit-python` Makefile targets + `services/<svc>/pip-audit-ignores.txt`, Hard Rule 7 (Buildkit cache is operator-managed; prune is prohibited). Preserve: Hard Rule 7 (never prune the HF cache mount), the SSoT tag model, the CI-as-authoritative-gatekeeper split (pre-push runs the touched subset only). Verify-first: measure the real current wall-clock per job before optimising — optimise the actual long pole, not a guess.

### CI performance budget (ratchet)
* [x] **Explicit wall-clock budget.** Set and document a PR-pipeline target (≤10–12 min; confirm against the measured baseline) enforced as a CI signal that fails on regression. Record the per-job breakdown in the Operations Playbook.
* [x] **Tame the long poles.** Keep the worker image's HF-model bake off the PR hot path (layer/registry caching; rebuild only when worker deps change, not every PR). Parallelise independent jobs; cache Go build/test, pip, and node_modules; reuse Testcontainer images via the SSoT tags.

### CI gating policy (blocking vs. advisory — solo-dev maintainable)
* [x] **Classify every CI step blocking vs. advisory.** A *blocking* step gates merge; an *advisory* step reports but does not fail the pipeline (`continue-on-error` / a non-required job). Document the policy + the rationale per step. **Goal:** a red pipeline always means "something a solo dev must act on now", never "a slow or flaky non-critical check tripped". Audit which steps currently fail the whole run.
* [x] **Move slow/flaky non-gating checks off the blocking path** — candidates per operator request: full `make audit`, the E2E suite, heavy image builds → advisory and/or scheduled (nightly), not PR-blocking.
* [x] **Security caveat (deliberate, not silent).** Advisory ≠ unwatched: a security check (govulncheck / pip-audit / Trivy) demoted from PR-blocking MUST still run on a schedule AND surface failures the operator actually sees (nightly job + notification), so "non-blocking" never becomes "unnoticed". Record the chosen tradeoff against CLAUDE.md's "CI is the authoritative gatekeeper" line.

### Makefile maintainability
* [x] **`make deps-refresh` restructured.** Split the monolith into per-ecosystem sub-targets (Go digests, Python lockfiles, image digests, SentiWS hash) so a single ecosystem can rotate without the full run; document expected runtime + that it is a deliberate, rare, bandwidth-heavy rotation (Hard Rule 7 / playbook). It stays the *only* sanctioned Buildkit entry point.
* [x] **`make audit` made overseeable.** Keep it CI-authoritative and off every hot path; locally profile it, cache what is cacheable, split fast vs. slow audit sub-targets, and document the expected runtime so "it ran forever" becomes "it takes N minutes, here's why".
* [x] **Target inventory sweep.** Every Makefile target either works or is removed; each carries a one-line `help` description; the slow ones declare their expected runtime. No dead targets.

### `scripts/` inventory & cleanup
* [x] **Classify every file under `scripts/`** into exactly one justified home: Makefile-wired (`build/deps_refresh.sh`, `build/e2e_smoke_test.sh`, `build/openapi_bundle.py`, `build/openapi_ref_style_check.sh`, `build/generate_metric_validity_scaffold.py`, `operations/clean.sh`, `operations/clean_infra.sh`, `operations/reset_validate.sh`) · git-hook (`hooks/pre-commit`, `hooks/pre-push`) · CI/workflow-used (`build/build_wikidata_index.py`, `build/wikidata_validate.sh` + `wikidata_fixtures/`, `build/e2e_helpers.sh` + `e2e_fixtures/`) · documented one-shot op (`operations/compute_baselines.py`, `operations/reextract_silver.py`) · audit one-off (`audit/*`) · **dead** → remove. One table; nothing unclassified.
* [x] **Remove committed build artefacts** — `scripts/**/__pycache__/*.pyc` are currently checked in; delete them and add a `.gitignore` rule so bytecode never re-enters the tree.
* [x] **Stale-script check** — the `audit/` one-offs (`bump_content_version_and_wp005_anchor.py`, `inject_composition_notes.py`, `methodology_coverage.sh`) and the `operations/` scripts verified still-correct against the current schema/content (or updated); obsolete ones removed. Every surviving raw-script invocation is documented in the Operations Playbook as a deliberate exception — the Phase-120c invariant (routine ops go through the Makefile) holds.

### Dev vs. prod stack clarity (`up`/`down`/`restart`)
* [x] **One obvious dev path, one obvious prod path.** Dev `make up`/`down`/`restart` brings up the FULL local stack incl. the operator-facing extras (Swagger UI, the observability stack, MkDocs) so every service is testable locally; the prod path (`compose.prod.yaml`) excludes dev-only extras + debug ports. It works today but is confusing — the goal is that *which target does what* is obvious, and that `down` tears everything (incl. swagger/telemetry/profiles) cleanly back down with no orphaned containers or volumes.
* [x] **Consistent `profiles:` + docs** — dev extras gated by Compose `profiles:` (clear opt-in/out, not ad-hoc); `make help` + a short playbook note state what dev brings up, how to reach each extra (Swagger / Grafana URLs), and how prod differs.

### Validation
* [x] PR pipeline runs within budget and fails on regression; `deps-refresh` can rotate one ecosystem in isolation; `make audit` has a documented, bounded runtime; `make help` lists every target with a description and a runtime hint for the heavy ones; every `scripts/` file is wired/documented or removed and no `__pycache__` is tracked; the dev↔prod target split is obvious and dev `up`/`down` brings up/tears down the full stack (incl. Swagger + telemetry) with no orphans. (Bandwidth-heavy validation runs scheduled for real-internet availability.)

### Status (2026-06-14) — substantially complete
*Shipped:*
* [x] Worker image off the per-PR scan path; security audits **advisory**; Python scientific stack on **cp314 wheels** (20–28 min → ~5–8 min) — the Phase-136 green-up.
* [x] **P1 workflow hygiene** — `bandwidth_probe.yml` deleted; nightly cron **and** push-trigger parked (no red on main); `web-crawler-build` / `wikidata_index_rebuild` confirmed documented + consumed (kept).
* [x] **P2 Makefile** — `make help` runtime hints on the heavy targets; **`make deps-refresh-fast`** (`--skip-build`) as the everyday rotation path; `make audit` runtime documented.
* [x] **P3 dev/prod clarity** — `make dev` (= `up` + Swagger UI) + a dev-vs-prod doc block + help; `down --remove-orphans` clears profile extras.
* [x] **P4 CI budget ratchet** — `timeout-minutes: 25` on every `ci.yml` job (a runaway fails fast instead of burning 30+ min).
* [x] **P5 Playwright operator doc** — Operations-Playbook "Dashboard E2E + visual regression" section.
* [x] **`scripts/` tidy** (on-disk `__pycache__` removed; nothing tracked) + **`lxml` 6.1.0** (cp314 wheel + closes CVE-2026-41066; `[html_clean]` satisfies readability-lxml) + **docs** (Operations Playbook; note: `CLAUDE.md` is gitignored/local, so canonical docs live in the playbook).

*Resolved (decisions, not silent drops):*
* [x] **Granular `deps-refresh --only=<digests|lock|sentiws>`** — **decided not to build it.** `deps-refresh-fast` (`--skip-build`) already removes the actual pain (the 10-GB rebuild); a per-step script wrap is marginal value + un-testable in-session. Reopen only if a real need to rotate one ecosystem in isolation appears.
* [x] **Advisory-security nightly + notification** — **relocated to Phase 146** (its natural home: prod monitoring/alerting). The per-PR advisory gating already keeps findings visible in every run; a push/email notification so demoted findings are never silent is now a Phase-146 bullet.

## Phase 154: Observability Verification — Traces, Trace-IDs & Operator Dashboard [P1] - [x] DONE

*The OTel → Tempo/Prometheus/Grafana stack exists (Phases 5/18/36) but has not been exercised in a while. Before deployment — and so the operator can actually debug the rest of Iteration 11/12 — telemetry must work end-to-end and present a clean, curated view of the key signals with a documented way to reach and use it. (Prod alerting on top of these dashboards is Phase 146; this phase builds the dashboards + trace plumbing they depend on.)*

**Grounding.** Read first: the OTel collector config under `infra/observability`, the backend telemetry init (`pkg/telemetry`), how/whether trace context propagates across the NATS + MinIO async hops (W3C `traceparent` breaks across async boundaries unless propagated in message metadata), the existing Grafana provisioning/dashboards, the Operations Playbook observability section. Preserve: the OTel-collector-as-sole-egress + internal-only network posture (no backend port exposure beyond `make debug-up`). Verify-first: confirm what actually emits traces/metrics today vs. what is dark before designing the dashboard.

### Traces + correlation
* [x] **End-to-end traces** — verified-first: already wired. Go HTTP via `otelhttp`, worker fully instrumented, same `ParentBased` sampler. The async hop carries propagated context — the Ingestion API injects W3C `traceparent` into the MinIO Bronze object `UserMetadata`, MinIO echoes it on the NATS S3-event, the worker `propagate.extract`s it. Documented as linked-spans-via-propagated-context in arc42 §8.6.1 + the playbook.
* [x] **Trace-IDs in backend logs** — shared Go middleware (`pkg/middleware/observability.go`) logs `trace_id` per request and sets the **`X-Trace-Id`** response header on every response incl. 5xx; the worker gains a `structlog` trace-id processor (`internal/logging.py`) so every worker log line carries it.

### Operator view + docs
* [x] **One curated Grafana dashboard**, provisioned-as-code — `aer-overview` rebuilt into four rows. New Go HTTP server metrics (`http_server_requests_total` / `_duration_seconds`) drive per-service request rate / latency / 5xx error rate; new worker `nats_consumer_pending`/`_ack_pending` gauges drive NATS lag; CH/MinIO native scrape + all-target `up` drive infra health. (Postgres has no exporter — lean choice; reachability shows via the BFF/worker pool errors + logs, native PG metrics deferred-with-reason.)
* [x] **How to reach/use it** — playbook §"Observability Stack" documents the Grafana URL/login, the dashboard, every panel's meaning + metric source, and the trace-by-trace-id workflow (`X-Trace-Id` / `trace_id` log / `documents.trace_id` → Tempo Explore).
* [x] **Operations Playbook section** — added "Observability runbook — symptom → trace → root cause" table covering 5xx spikes, latency, NATS lag, DLQ, WorkerDown, empty-panel drift, and broken-trace cases.
* [x] **Ratchet (Iteration-11 discipline)** — `make observability-validate` (promtool + otelcol validate + dashboard/YAML/XML checks against compose-pinned images) wired as a CI job so broken observability config fails the build.

### Validation
* [x] Implementation + static validation complete (`make observability-validate` green, pkg/worker tests green). Live e2e confirmation (trace findable by id, same id in logs, dashboard live) is the operator's first stack run — see the run/rebuild instructions handed off at phase close.

---

## Phase 138: Quality Inventory & Worklists [P1] - [x] DONE

*The assessment stage for the quality pass — cheap, tool-driven, fixes nothing. It turns "the code quality can be better" into concrete, counted worklists so Phases 139–143 remediate against evidence, not vibes, and so progress is measurable. Mirrors the assess→fix pattern Iteration 12 uses for security.*

**Grounding.** Read first: the repo layout (`services/*`, `pkg/`, `crawlers/*`, `services/dashboard/src`), existing lint configs (golangci-lint, ruff, eslint/svelte-check), `.tool-versions`. Preserve: existing lint baselines (extend, don't fork). Verify-first: prefer tools already pinnable via `.tool-versions` / existing dev deps before adding new ones (no overengineering).

### Inventories (each → a counted list in the register)
* [x] **Long-file census.** Every file over an agreed threshold (propose Go/Python > ~500 LOC, Svelte component > ~400 LOC — confirm against current norms), ranked, with a split hypothesis per file. Feeds Phase 141. → register §1: 35 production files (Svelte 18, Go 5, Py 6, TS 6 incl. 2 data-file exemptions) + 7 long test files (→142). **Operator decision pending:** recommend ratchet at 500 for all four languages (Svelte 400 advisory only — 12 files in 400–500).
* [x] **Dead-code scan.** Per language: Go (`deadcode`/`staticcheck` unused), Python (`vulture`/`ruff` unused), TS/Svelte (`knip`/`ts-prune`; unused exports/components/assets). Candidate list with confidence. Feeds Phase 139. → register §2: 7 dead FE files/barrels + 2 unused deps; Go 0 / Python 0 real; ~127-item TS manual-review backlog (mostly FPs).
* [x] **Naming-inconsistency scan.** Catalogue divergent conventions (casing, abbreviations, domain-term drift — probe/source/panel/cell vocabulary) across each language. Feeds Phase 140. → register §3: 81 safe Go initialism renames + ~250 occurrences of retired Function-Lane/view-mode/Surface-II vocabulary in the dashboard.
* [x] **Comment/doc-coverage gaps.** Exported-symbol doc coverage (Go doc comments, Python docstrings, JSDoc/Svelte) + a TODO/FIXME/commented-out-code census. Feeds Phase 143. → register §4: coverage Go 89.9% / Py 76.8% / TS 43.2%; 0 TODO/FIXME; 0 commented-out blocks; 2 English-only violations.
* [x] **Stale-docs list.** Sweep `/docs` (Arc42, ADRs, methodology, design, operations, extending) for references to removed/renamed features (`/compose`, "Function Lane", RSS crawler, four-surface vocabulary). Feeds Phase 129. → register §5: 23 definitely-stale + 3 to-verify; 5 legitimate-historical groups preserved.

### Register
* [x] **`docs/operations/quality_register.md`** — one living register; each item tagged scope · effort · ratchet-target, grouped by the consuming phase. The SoT the remediation phases check off against.

### Validation
* [x] The register exists with counted items in every category; each downstream phase (139/140/141/143/129) has a concrete, non-empty worklist (or a justified "nothing found").

---

## Phase 139: Dead-Code Elimination [P2] - [x] DONE

*First remediation step, before any refactor: deleting unused code shrinks the surface every later phase must touch. Runs against Phase 138's dead-code worklist.*

**Grounding.** Read first: Phase-138 dead-code list, the build graph (`go.work`, `__init__.py` exports, Svelte component imports, asset references). Preserve: anything reachable only via reflection/dynamic dispatch, config-string registries (the `AdapterRegistry`/extractor registries), or generated code — verify reachability before deleting. Verify-first: a "dead" symbol that is actually an extension seam (registry entry, public `pkg/` API) is NOT dead — confirm against the registries before removal.

### Remove
* [x] **Confirmed dead code deleted** per language, each deletion validated by green build + green tests (relies on Phase 136). Retired one-shot/backfill utilities stay retired. → 8 dead FE files deleted (each verified independently unreferenced before deletion); Go 0 / Python 0 (the lone Go candidate `DefaultArgon2Params` is a test fixture → kept-with-reason; 2 Python vulture hits are Protocol contracts → kept). `make fe-lint` + `make fe-test` (294) green.
* [x] **Unused assets / exports / dependencies** pruned (unused npm/go/pip deps removed from manifests; unused static assets removed). → `@opentelemetry/context-zone` + `@types/diff` removed (lockfile pruned, zero download). The ~127 unused-export candidates were mostly FPs → deferred (not a sweep; see register §2). CLAUDE.md stale `WorkbenchDatasetShape` line corrected.
* [x] **Ratchet.** Wire the dead-code check into CI (or the lint stage) so re-introduction fails the build. → **Go** golangci-lint `unused`/`staticcheck`/`ineffassign` (already default-active) · **Python** `ruff` default `F401/F811/F841` (already active) · **TS** new `knip` + `knip.json` wired into `make fe-lint` (⇒ CI). All three proven with a planted-symbol test (exit 1 dirty / 0 clean).

### Validation
* [x] Build + full test suite green after removal; the dead-code CI check is active and fails on a planted unused symbol; the register's dead-code section is fully checked off or items are explicitly reclassified as live-with-reason. → FE suite green; all 3 ratchets pass the planted-symbol test; register §2 fully checked off. *Note: Go/Python Testcontainer + Playwright E2E not re-run locally — zero backend-source change in this phase; CI runs the full suite on push.*

---

## Phase 140: Naming & Code-Convention Unification [P2] - [x] DONE

*Codify the conventions, then apply them — in that order, so the pass is consistent rather than another layer of drift. Runs against Phase 138's naming worklist.*

**Grounding.** Read first: Phase-138 naming list, the implicit conventions across each language, the domain vocabulary in CLAUDE.md (probe / source / pillar / panel / cell / scope) and `discourse-function.ts`. Preserve: the **machine `probeId` as the stable load-bearing identifier** and all other load-bearing identifiers — rename display/local names, never wire-contract / DB / URL-grammar keys without an explicit migration. Verify-first: a name that crosses a boundary (OpenAPI field, ClickHouse column, URL key, NATS subject) is a contract, out of scope for a pure rename.

### Codify
* [x] **`docs/development/conventions.md`** — naming + style conventions per language (Go, Python, TS/Svelte), domain-term canon (one spelling per concept), file/module naming, the contract-boundary rule, and the enforced (ratchet) subset table.

### Apply
* [x] **Local/internal renames** applied per the convention doc, each behind green compiler/tests. → **Go initialisms** via oapi-codegen `name-normalizer` (the per-field renames were impossible — anonymous structs structurally mirror generated types; json wire tags 100% unchanged, ~119 refs chased, codegen deterministic). **TS**: `ViewMode`→`Presentation` (163), `viewmodes/`→`presentations/` (82 paths), `ViewingMode`→`PillarId` (50), `lanes/` split into `source/`/`article/`/`evidence/`/`charts/` (+ Lane→Line chart renames), user-facing "Function Lane"/"Surface II" strings → Workbench. Cross-boundary identifiers (OpenAPI fields, `view_mode` catalog key, URL keys, DB names, machine `probeId`) preserved; **"Surface II/III" numbering deferred-with-reason** (33 occ incl. generated `types.ts` from OpenAPI descriptions).
* [x] **Ratchet.** → **Go** `.golangci.yml` (`stylecheck ST1003` + `gofmt`); **Python** `ruff N` in both services (targeted ignores for defensible FPs); **TS** eslint `no-restricted-syntax` vocabulary ban. All three proven with planted-symbol tests. (Register's "81 safe Go renames" + "Python N = 0 violations" assumptions were both wrong — corrected in register §3.)

### Validation
* [x] Convention doc exists; CI enforces the mechanical subset (3 ratchets active + proven); naming worklist checked off or deferred-with-reason; no contract broken — **OpenAPI sync verified** (codegen deterministic, json tags unchanged), `make lint` green (Go ×3, Python ×2, fe), svelte-check 0 errors, **Go tests green incl. Testcontainers**, fe-test 294/294.

## Phase 141: File Decomposition — Long-File Refactor [P2] - [x] DONE

*Split the over-long files from Phase 138's census into coherent units along existing architectural seams — Clean Architecture for Go (`cmd`→`config`→`core`→`storage`), extractor/adapter boundaries for the worker, component/store boundaries for the dashboard. Structural only: no behaviour change.*

**Grounding.** Read first: Phase-138 long-file census + split hypotheses, the Clean-Architecture layering (CLAUDE.md), the worker `MetricExtractor`/`SourceAdapter` boundaries, the dashboard component/`$lib` structure, the `panel-mutators` pure/rune split (an existing good decomposition to mirror). Preserve: public APIs, the pure/impure split pattern (`panel-mutators-pure.ts` vs rune-wrapped), import graphs. Verify-first: decompose along an existing seam, never invent a parallel structure beside the established one (brownfield rule).

### Refactor
* [x] **Done — split <530, behaviour-preserving:** **Go** 5 files → 15 (golangci clean, Testcontainer green); **Python** `corpus.py`/`audit_source.py`/`web_extract.py` (348 worker + 121 crawler tests green); **Dashboard TS SoTs** `queries.ts` 1239→6, `url-internals` 985→3, `registry` 784→2; **Svelte mediums** DistributionCell/TopicEvolutionCell/AtmosphereSurface (pure logic → `*-internals.ts`, +~30 tests).
* [x] **Done — Phase-1 LOGIC extraction from the giants** (pure logic → tested companion modules; the components themselves are NOT yet <530): `PanelControls`→`panel-controls-derive.ts` (+21t incl. `reconcilePanelForView`), `PanelHost`→`panel-host-layout.ts` (+21t). Co-occurrence cells share `cooccurrence-network-shared.ts`.
* [x] **Done — Ratchet** = ESLint **`max-lines`** (530 non-blank LOC) over production TS+Svelte, per-file no-growth caps for residuals in `eslint.config.js` (`FILE_LENGTH_ALLOWLIST`); runs in `make fe-lint` → pre-commit/-push + CI; teeth-tested. (Ruff/golangci have no file-length rule → Go 0 violations + 4 cohesive Python files documented in the register.)
* [x] **Done — Playwright e2e net** validated (32/32 green) — the safety harness for the markup decomposition below.
* [x] **DONE — actually decompose the markup/scoped-CSS-dominated giants to <530** (Tier-2b sub-componentisation, behaviour-preserving, behind the e2e net). Each: characterization e2e of its surface → extract child components (markup + their scoped CSS) → e2e green → lower its `max-lines` cap → repeat to <530. All 8 giants done.
  * [x] `PanelControls` 2002→356 (`workbench/levers/*`: 2 shared primitives + 8 per-lever children).
  * [x] `PanelHost` 1364→312 (5 per-region children: PanelToolbar/PanelScopeChips/PanelDisclosureNotes/PanelCellGrid/PanelCell).
  * [x] `L5EvidenceReader` 1286→395 (Tier-2b: L5MetaGrid/L5NegativeSpaceSection/L5DiffTab/L5RevisionHistory + tested `l5-evidence-internals.ts` +29t).
  * [x] `ScopeEditor` 997→457 (ScopeGroupCard/ScopeGroupSources + tested `scope-editor-internals.ts` +20t; behind extended Workbench e2e).
  * [x] `AnalysesOverlay` 974→489 (AnalysisTable/AnalysisDrawer + tested `analyses-overlay-internals.ts` +19t; behind a new `analyses.spec.ts` e2e).
  * [x] `CellConfigPopover` 660→223 (order-preserving 2-child split CellConfigValueLevers/CellConfigChannelLevers + tested `cell-config-popover-internals.ts` +15t; behind an extended Workbench e2e). Did **not** reuse the `levers/*` strip primitives — the compact `.ccp-*` popover styling (Phase-126 overflow fixes + per-row override dots) differs from the roomier `.ctrl-*` strip, so reuse would have been a visual regression (technique #7).
  * [x] `wp/[id]/+page` 642→91 (per-region WpBreadcrumb/WpTableOfContents/WpPaperBody + tested `wp-page-internals.ts` +7t; behind a new `reflection-wp.spec.ts` e2e). CSS-dominated; folded the always-on `.neg-space .toc` accent into base.
  * [x] `ProbeCard` 568→381 (ProbeDfCards child + tested `probe-card-internals.ts` +8t; behind a new `dossier.spec.ts` e2e). The DF-region split is the cohesive cut (the census's capability-matrix-only idea was too small).
* [~] **Justified allowlist (operator-approved, stay as-is):** the render-glue co-occurrence cells (`CoOccurrenceNetworkAtScale` 728, `CoOccurrenceNetworkCell` 1222 — visual logic already in `cooccurrence-network-shared.ts`), `engine.ts` 937 (imperative Three.js/WebGL), `open-questions.ts` 743 (data table), `AtmosphereSurface` 642 / `SideRail` 533 (scoped-CSS/markup, within tolerance), and the 4 Python residuals.

### Validation
* [x] **CLOSED (2026-06-16).** Full test suite green before and after each split; the ESLint `max-lines` ratchet active in fe-lint + CI; every census file is either <530 or a justified allowlist entry. All 8 open giants decomposed to <530 (their caps removed from the allowlist), leaving only the operator-approved render-glue/data/within-tolerance exceptions (`CoOccurrenceNetworkCell` 1222, `engine.ts` 937, `open-questions.ts` 743, `CoOccurrenceNetworkAtScale` 728, `AtmosphereSurface` 642, `SideRail` 533, + 4 Python residuals). Final sweep: fe-lint(+knip) green · fe-test 464 · full e2e 48 passed/3 skipped. **The 7 long TEST files are tracked separately in Phase 142.**

---

## Phase 142: Test Suite Health & 80% Coverage Floor [P1] - [x] DONE

*Raise and lock a coverage floor, and bring the test code itself to the same quality bar as production code. Operator requirement (2026-06-14): **≥80% coverage for every service** — Go services, the Svelte/dashboard app, and the 3D engine module explicitly named, with Python services held to the same bar. Placed after the structural refactors (139–141) so new tests target the final structure, not code about to move.*

***Floor model decided 2026-06-17 → recorded in ADR-041 (Coverage Floor Model & Test-Strategy Ratchet).*** The naive "80% line coverage everywhere" collides with two structural realities, so the metric is adapted (never the rigour): **Svelte components have no Vitest line-coverage path** (node-env Vitest never renders `.svelte`; Svelte-5 runes are unsupported under jsdom — 131 files / ~31k LOC = ~77% of hand-written dashboard code), and the **engine-3d WebGL render core is not unit-testable** without a GL context. See ADR-041 for the full rationale; the model below is its binding summary.

**Measured baseline (2026-06-17, real — `go test -cover` / `pytest-cov` / `@vitest/coverage-v8`):** Go `pkg` **78.3%** (logger/telemetry 0%) · `bff-api` **39.6%** (handler 37 / storage 49 / config 35 / notify 0 / cmd 0–5) · `ingestion-api` **40.0%** (handler 36 / config 0 / storage 70 / core 78 / cmd 0) · Py `analysis-worker` **71%** · `web-crawler` **73%** · dashboard `$lib` `.ts` **~60%** · engine-3d math layer mostly covered (GL core `engine.ts`/`GlobeControls.ts` 0%). Every required scope is currently <80% → climb all, then gate at 80.

***Outcome (2026-06-17 — COMPLETE; earlier commits `078d6d2`, `28f59e8`, `a111473`):*** every scope is at or above the 80% floor and its gate is bumped to 80:
- **pkg 95.3** · **ingestion-api 80.7** · **bff-api 82.3** (handler 80.1 / storage 82.0 / config 98.7 / auth 98.6 / notify 100) · **worker 80.4** · **web-crawler 80.3** · **`$lib` 90.3 stmts / 84.0 branch / 95.9 funcs / 92.9 lines** · **engine-3d 96.7**.
- **bff-api 49.9→82.3** via mock-store handler tests (extracted `mockstore_test.go`; revisions/scope/webauthn/analyses/auth/config/notify) + Testcontainer storage tests (`setupTestStore`/`setupAuthStore` + seed fixtures). `silver_store.GetEnvelope` left uncovered (MinIO infra absent — documented). **worker 78→80** via `run_revision_diff_sweep` + metadata-coverage/probe-scope/configure_logging/reattempt units. **`$lib` 76→90** via `cooccurrence-network-shared.ts` (the 123-stmt lever) + `pillar-internals.ts` extraction (DI) + branch tests; ADR-041 exclusions added for `panel-mutators.ts`+`pillar.ts` (rune layer) + `webauthn-browser.ts` (browser API).
- **Ratchet PROVEN, not asserted** (ADR-041 §6): all 3 gate mechanisms exit non-zero below floor (`go_coverage.sh pkg 99`, crawler `--cov-fail-under=99`, `vitest --coverage.thresholds.lines=99`).
- **Supply-chain (raised during closure):** Go stdlib CVEs GO-2026-5037/5039 fixed by the **go1.26.3→1.26.4 toolchain bump** (go.work + 3 go.mod + both Go Dockerfiles to `golang:1.26.4-alpine3.23@sha256:f23e8b2…`); `torch` CVE-2025-3000 (no upstream fix, local-only, unreachable) documented-suppressed in `pip-audit-ignores.txt`. `make lint`/`test`/`audit` all green on 1.26.4. *Follow-up (own phase): the transformers 4→5 + sentence-transformers 3→5 migration (now stable-available) would clear CVE-2026-1839 + PYSEC-2025-217 but needs the Phase-119 determinism re-validation.*

**Grounding.** Read first: ADR-041, current coverage per service (commands above), the long-test-file entries from Phase 138 (§1, listed below), the Testcontainer + Playwright setup, the `engine-3d` test surface. Preserve: the test-as-contract value (no assertion-free coverage padding), the Testcontainer image-tag SSoT. Verify-first: the baseline above was measured before setting any threshold.

### Coverage to floor (per ADR-041)
* [x] **Hard 80% line-coverage ratchet** — Go `internal/…` of (`ingestion-api`, `bff-api`) + the `pkg` packages, Python (`analysis-worker`, `web-crawler`), and dashboard **`$lib` `.ts`** (`src/lib/**/*.ts` + route `.ts`). Meaningful tests on real behaviour and edge cases, not coverage-padding (xhigh code-review found 5 false-confidence tests; all fixed to assert real behaviour).
* [x] **Denominator convention** — excludes thin `cmd/` mains (Go) + `main.py` entrypoints (Python, `# pragma: no cover`) + generated code (`generated.go`, `api/types.ts`); plus the rune layer (`*.svelte.ts`, `panel-mutators.ts`, `pillar.ts`) + browser-only (`otel.ts`, `webauthn-browser.ts`), each with an in-config justification.
* [x] **Svelte components — no Vitest %-gate.** Pure logic lifted into companion `.ts` (e.g. `pillar-internals.ts`, counted under the `$lib` floor) + behaviour covered via Playwright E2E; svelte-check guards types/templates.
* [x] **engine-3d — math layer floored at 80% (96.7%); GL render core exempt.** `engine.ts` + `GlobeControls.ts` excluded with an in-config justification, covered by visual-regression E2E.
* [x] **Coverage ratchet in CI.** Per-scope minimum enforced (Go `go tool cover` via `go_coverage.sh`, Python `--cov-fail-under`, Vitest `coverage.thresholds`), all bumped to 80; CI fails below floor.

### Test-code health
* [x] **Long / unwieldy test files refactored** (Phase-138 register §1) — Go `metrics_handler_test.go` 764→415 (+`metrics_available`/`metrics_scatter`), `view_mode_handlers_test.go` 774→409 (+`cooccurrence_handlers`), `storage/metrics_query_test.go` 1048→418, `view_mode_queries_test.go` 685→442 (+`fixtures`/`resolution`); TS `url-panels.test.ts` 504→448; Py `test_audit_source.py` 862→826 + `test_main.py` 656→626 (+`_audit_helpers.py`, shared fixtures + parametrization — kept >530 deliberately: distinct assertions + regression docstrings, no Python length gate).

### Validation
* [x] Every floored scope reports ≥80%; the CI coverage gate is active and fails a planted regression (proven per gate-mechanism); the Svelte/engine-3d exemptions carry in-config justifications; refactored test files are green; `make lint`/`test`/`audit` all green (the closure also bumped the Go toolchain to 1.26.4, fixing two stdlib CVEs). Suite runtime within the Phase-137 budget.

---

## Phase 143: In-Code Documentation Professionalization [P2] - [x] DONE

*A deliberate whole-codebase pass over `crawlers/`, `infra/`, `pkg/`, `scripts/`, `services/` to bring in-code documentation to a uniform, professional, best-practice standard for each language/framework — so a new developer or maintainer can open any file and quickly grasp its role, contracts, and gotchas. **Doc/comment-only: no behaviour, no logic, no public-API changes.** The survey (2026-06-17) found the codebase already strong in most places — `infra/` SQL migrations + `scripts/` bash headers are exemplary, Python is ~95% docstring-covered, dashboard TS/Svelte mostly well-commented — so this is **gap-filling to a consistent bar, not a from-scratch effort**, concentrated in Go (package + exported-symbol docs) with smaller pockets in Python and dashboard TS.*

**Doc standard — best practice per language; concise, "why not what", NEVER restate the signature:**
- **Go** — godoc: a package comment on every package (on the primary file or a `doc.go`); a doc comment on every exported identifier, starting with the symbol name, one sentence of intent + any invariant/unit/gotcha; a top comment on each `cmd/` main stating what the binary does.
- **Python** — PEP 257: a module docstring at every file top; a docstring on every public class/function carrying non-obvious intent. Match the house prose style (triple-quote narrative; NO `Args:/Returns:` ceremony — type hints already carry the signature).
- **TypeScript (`$lib`)** — a file-top context comment + TSDoc (`/** */`) on exported functions/types whose intent isn't obvious from the name. **Svelte** — a short role comment at the top of `<script>` stating the component's job + key contract.
- **SQL migrations / YAML configs / Dockerfiles / bash** — a header block stating purpose (+ phase/ADR/WP anchor where one exists). Already strong; fill only the few gaps.

**Format rule (operator):** keep per-symbol docs SHORT and precise (one line / a few lines). Reserve longer multi-paragraph documentation for the FILE TOP (package/module/component orientation) where that is best practice. No documentation bloat anywhere.

**Grounding.** Read first: CLAUDE.md (English-only Hard Rule + the architecture), the survey hotspots below, and 2–3 already-exemplary files per language to MATCH the house voice (Go: `services/bff-api/internal/auth/password.go`; Python: `services/analysis-worker/internal/extractors/sentiment.py`; TS: `services/dashboard/src/lib/presentations/cooccurrence-network-shared.ts`; SQL: any `infra/*/migrations/0000*.sql`; bash: `scripts/build/deps_refresh.sh`). Preserve: the English-only invariant; the existing Phase/ADR/WP anchors (live design documentation, NOT stale TODOs — never delete); and ALL load-bearing directives (`//nolint`, `//go:build`, generated `DO NOT EDIT` headers, `# pragma: no cover`, `# type: ignore`, `# noqa`, `// svelte-ignore`, `// eslint-disable`). Verify-first: state intent/constraints, never restate the signature; this is a non-functional pass — `git diff` must be comments/docstrings only.

***Outcome (2026-06-18 — COMPLETE):*** doc/comment-only pass; `git diff` verified comments/docstrings-only (zero logic lines — the one gofmt whitespace reflow it would have caused was reverted), `make lint` + `make test` green (Go Testcontainers + pkg, Python 176, FE 704+41). Scope was confirmed-then-filled, not invented: the codebase was already strong, so this closed the measured gaps and matched the existing house voice rather than imposing a new standard.
- **Go (bulk):** package comments added to the 11 packages that lacked one (`pkg/{logger,middleware,telemetry,testutils}`; `ingestion-api/{cmd/api,internal/{config,core,storage}}`; `bff-api/{cmd/server,internal/{config,storage}}`); doc comments on the undocumented exported symbols (storage types/constructors, cache-control writer methods). **Enum const-groups already covered by their type doc were left untouched** — per-constant comments would be signature-restating bloat (the operator's no-bloat rule).
- **Python (small):** module docstrings for `corpus_revision_diff.py` + `main.py`; docstrings on the handful of undocumented public helpers/dataclasses (worker + crawler). The per-file module-docstring norm is *absent even in the exemplar* `sentiment.py`, so module docstrings were added only where purpose was non-obvious, not forced onto every file.
- **Dashboard (small):** file-top headers on the 4 `$lib/api/queries/*` domain splits + 3 barrels; role comments on the 5 `components/base` primitives. The named "thin headers" (`url-internals`/`url-types`, `api/{analyses,auth}`) were already adequate post-Phase-141 and left as-is. `fe-lint` (eslint+prettier+svelte-check 0/0+knip) green.
- **infra (minimal):** header blocks on the 6 observability YAML configs (tempo, otel-collector, prometheus, alert.rules, grafana-datasources, dashboards). Migrations + bash already exemplary — untouched.
- **English-only:** the single German comment block (`workbench/scope-editor-draft.ts`) translated → holds repo-wide. ~48 files, comments/docstrings only.

### Survey hotspots (where the gaps actually are — 2026-06-17)
* [x] **Go (the bulk).** Package comments for the ~10 packages missing them (`pkg/{logger,middleware,telemetry,testutils}`; `ingestion-api/internal/{config,core,storage}` + `cmd/api`; `bff-api/internal/storage` + `cmd/server`); doc comments on undocumented exported symbols, prioritising the high-cardinality query packages (`bff-api/internal/storage`, `ingestion-api/internal/storage`, `pkg/middleware`). — *Done; also `bff-api/internal/config`. Enum const-groups left to their type docs (no bloat).*
* [x] **Python (small).** The few missing module docstrings (`analysis-worker/internal/corpus_revision_diff.py`, `analysis-worker/main.py`) + docstrings on the handful of undocumented public helpers. — *Done across worker + crawler; module-docstring norm matched (not forced onto every file).*
* [x] **Dashboard TS (small).** Flesh out the thin module headers (`$lib/state/url-internals.ts` / `url-types.ts`, `$lib/api/{analyses,auth}.ts`), add TSDoc to undocumented `$lib` exports, and a one-line header to barrel/index files. Svelte components already carry role comments — fill any that don't. — *Done: 4 `api/queries/*` headers + 3 barrels + 5 `base/` role comments; named thin headers were already adequate.*
* [x] **infra / scripts (minimal — already excellent).** Only the few uncommented YAML configs (e.g. `grafana-datasources.yaml`, `tempo.yaml`); migrations + bash scripts need no work. — *Done: 6 observability YAML headers.*
* [x] **Dead comments / commented-out code / stale TODO-FIXME.** The survey found effectively NONE — confirm during the pass and remove any that surface; never remove the Phase/ADR/WP anchors. — *Confirmed none; all Phase/ADR/WP anchors preserved.*

### Validation
* [x] Every package/module/component reads cleanly top-down for a new maintainer; the per-language best-practice format is applied uniformly; no signature-restating or bloat; all load-bearing directives + Phase/ADR/WP anchors intact. `make lint` + `make test` green, and `git diff` shows comments/docstrings only (no logic, no behaviour change). English-only holds repo-wide. *(Optional stretch, deferred: enable a doc-linter — Go `revive` package-comments / ruff `D` rules — as a future ratchet once the gap-fill lands.)*

---

## Phase 144: UI Localization — German (DE) [P1] - [x] DONE

*The app is English-only at the UI-shell level (`APP_CONTENT_LANGUAGE` clamp). This phase delivers a **complete EN/DE localization** of the dashboard — every user-visible string, aria-label, placeholder, date/number format, and long-form document — and it is achievable now because most German content **already exists**: the BFF content-catalog is at **97/97 `{en,de}` parity** (real, high-quality German — not stubs) and all six Working Papers are already translated under `docs/methodology/{en,de}/`. The **only large net-new translation is the ~600 hardcoded UI-shell strings**; everything else is wiring a single locale signal through layers that are already bilingual. It is the prerequisite that makes Phase 128's EN/DE-parity audit real.*

**Architecture decided (2026-06-18) — two tiers, one locale signal.** The idiomatic best-practice shape for a static-SvelteKit + BFF app, and ONE coherent system, not a mixture:
- **UI-shell strings → Paraglide JS (inlang).** Compile-time, type-safe, tree-shaken message functions; zero runtime; works in pure SSG (adapter-static). Messages organised **per-feature** (not two monolithic locale files) for maintainability; inlang parity-lint enforces EN⇄DE coverage. *(The DE message files are localized content resources, not code/comments — the English-only Hard Rule still governs code, comments, Arc42, API contracts, commit messages; only the user-facing UI gains German, mirroring the already-bilingual BFF content YAML.)*
- **Editorial / domain prose → the existing BFF `/content/*?locale=` catalog** (already EN/DE, versioned, ADR-020). Mechanism UNCHANGED — Phase 144 only WIRES the locale through (the `contentQuery` default stops being hardcoded `en`).
- **Working Papers → served per-locale** from the already-translated `docs/methodology/{en,de}/` source.
- **One locale rune** (replaces the `APP_CONTENT_LANGUAGE` constant) is the single source of truth; it drives Paraglide, the content `?locale=`, the viewer-language QID-relabel clamp, and locale-aware `Intl` formatting.
- **Locale state — privacy-clean, NO cookie** (AĒR privacy-minimal / no-tracking stance): resolution order `?lang=en|de` (deep-link/share) → `localStorage` (`aer.locale`, persisted preference) → `navigator.language` clamped to {en,de} → `en`. The **selector lives in the SideRail user section**. No consent banner; deep-linkable; no path-routing, so adapter-static prerender is unaffected.
- **Boundary rule (the whole mental model):** a string baked into a component (button / label / error / aria / placeholder / badge) is a Paraglide message; researcher-editable domain prose (refusal / how-to-read / source / probe / metric / discourse-function) is BFF content. Emic proper nouns (probe + source display names) stay untranslated — *Tagesschau is Tagesschau* (design brief).

**Grounding.** Read first: `src/lib/presentations/viewer-language.ts` (`APP_CONTENT_LANGUAGE` seam — currently `= 'en'`), `contentQuery` in `src/lib/api/queries/probes.ts` (locale param hardcoded `'en'`), `services/bff-api/internal/handler/content_handler.go` (**404s on a missing-locale key — decide EN-fallback + CI parity gate**), `svelte.config.js` (adapter-static, pure SSG, no server hooks), the inventory below, the date formatters in `src/lib/components/evidence/l5-evidence-internals.ts` (hardcoded `en-CA`), `src/lib/methodology-copy.ts` + `src/lib/negative-space.ts` (hardcoded TS copy), the SideRail user section, and the design-brief language stance. Preserve: the **app-UI-language vs content/viewer-language distinction** (do not conflate — the latter already exists for QID relabelling); the static-SSG model; URL deep-linkability + the URL-state codec; the reflexive surfaces. Verify-first: confirm the inventory before estimating.

**Inventory (surveyed 2026-06-18).** ~433 visible strings + ~163 aria/title ≈ **600 UI-shell strings** across ~58 files. Heaviest: Workbench levers+panels (~200), error/empty/loading (~70, cross-cutting), Dossier+Account/Admin overlays (~85), auth (~45), Chrome (~45), Reflection landing (~35). Already catalog-sourced (only need locale wiring): RefusalSurface, HowToRead.

**Implemented 2026-06-18 (ADR-042).** Mechanism: `@inlang/paraglide-js@2.20` + `paraglideVitePlugin` (strategy `['baseLocale']`; client overrides via `overwriteGetLocale`); `project.inlang` baseLocale `en`, locales `[en, de]`; **per-feature** message files `messages/{en,de}/{common,base,chrome,atmosphere,workbench,levers,cells,dossier,source,auth,account,reflection,errors,domain}.json` (globally-unique feature-prefixed keys; **1349 keys/locale** after Phase 144b). Compiled output `src/lib/paraglide/` gitignored, built by `pnpm run messages` (prerequisite of check/lint/test/knip + the Vite plugin). Custom parity gate `scripts/check-i18n-parity.mjs` (key parity + dup detection + base-aware empty-stub detection) runs in `pnpm run lint`. Coverage was fanned out per-surface via subagents with central QC. `make fe-lint` / `fe-test` / SSG build / bundle-size all green.

**Phase 144b — conceptual-vocabulary SoT tranche (DONE 2026-06-18).** The last English-in-DE bits — the shared domain-vocabulary data SoTs — are now localized via a new `messages/{en,de}/domain.json` (79 keys/locale): `presentations/registry.ts` (17 presentation labels/descriptions + 3 pillar blurbs/descriptions; pillar `label` Aleph/Episteme/Rhizome stays emic), `discourse-function.ts` (4 function labels/descriptions), `negative-space.ts` (6 NS-class labels/descriptions + 5 policy notes), `methodology-copy.ts` (6 banner templates, counts injected as message params), and the BERTopic outlier/`other topics` legend labels in `topic-internals.ts`/`topic-evolution-internals.ts`. Pattern: each accessor (`getPresentation`/`getPillar`/`getFunctionDef`/`getNSClassDef`/`methodologyNotes`/`uncategorisedLabel`) resolves its label/description at call time via a **relative** `import { m } from '../paraglide/messages.js'` (node-test-safe — resolves EN under the baseLocale strategy, so the English-asserting unit tests pass unchanged); the static English literals stay as the structural fallback. Reactive with **no consumer change** (m.*() reads the locale rune). The registry's id→message lookup + `localize*` helpers live in a new `presentations/registry-i18n.ts` (keeps `registry.ts` under the file-length cap). Two direct static-map consumers re-routed through the localizing accessors (`PillarSwitch` → new `listPillars()`, `ProbeDfCards` → `getFunctionDef`). Verified: `pnpm run` lint (parity 1349/1349) / check / knip / test:unit (711 pass, coverage 89/83/92/91) / build / bundle-size all green; e2e DE assertion on the registry-driven Workbench View lever (`workbench.spec.ts`).

### Mechanism
* [x] **Paraglide/inlang setup** — done; per-feature files; SSG build clean; inlang messageFormat plugin via CDN module + committed `project.inlang/settings.json` (cache gitignored per inlang's own `.gitignore`).
* [x] **Single locale rune** — `src/lib/state/locale.svelte.ts` replaces `APP_CONTENT_LANGUAGE`. Resolution `?lang=` → `localStorage('aer.locale')` → `navigator.language`∈{en,de} → `en`; fed to Paraglide via `overwriteGetLocale` (reactive switch, no reload); `lang` is a first-class `UrlState` field so it round-trips through the existing codec; content queryKeys include the locale; WP load `depends('app:locale')` + `invalidate` on switch.
* [x] **Locale selector** — `chrome/LocaleSwitch.svelte` (EN|DE segmented toggle, endonyms + `lang` attr, narrated) in the SideRail user section + on the pre-auth `AuthCard`.

### Coverage (every detail)
* [x] **All UI-shell strings → Paraglide** — DONE for every reachable surface: chrome (SideRail/ScopeBar/PillarSwitch + pillar questions/plain-language), Atmosphäre (+ WebGL fallback), Workbench (panels/shells/ScopeEditor/PanelControls + all `levers/*` + all presentation `cells/*`), Dossier, auth, Account/Admin, Reflection (+ WP reader/landing/primer/probe/metric), source+article, base primitives (Badge/FunctionBadge/NegativeSpaceBadge/Dialog/SidePanel/DateRangePicker/SkipLink) + RefusalSurface. Includes aria/title/placeholder; pluralised via `_one`/`_other`. **Conceptual-vocabulary SoT tranche — DONE (Phase 144b):** the shared **data SoTs** that held English `label`/`description` are now localized via `messages/{en,de}/domain.json`: `presentations/registry.ts` (presentation + pillar labels/descriptions/blurbs, View picker + tile tooltips), `discourse-function.ts` (4 function labels/descriptions in FunctionBadge), `negative-space.ts` (NS-class labels/descriptions + policy notes in NegativeSpaceBadge), `methodology-copy.ts` (6 banner templates on 5 chart cells), and the outlier/`other topics` legend labels in `topic-internals.ts`/`topic-evolution-internals.ts`. `open-questions.ts` (743-line research-question DATA prose) + `papers.ts` structural `shortTitle` English fallback remain out of scope by design. **Also out of scope forever — backend/source-driven cell DATA** (co-occurrence entity surface forms, BERTopic topic labels, article titles/authors, categorical metadata field values, metric values): it is rendered in the *source's* language, never the UI language — the app-UI-language vs content-language distinction (CLAUDE.md). Numbers/dates inside cells are locale-*formatted* (Intl), not translated; the only data that adapts to the viewer is the co-occurrence QID-node **label-swap** via the Phase-123b `displayLanguage` toggle (already wired to the locale rune).
* [x] **Editorial content wired to locale** — `contentQuery` + every call-site read the rune; **BFF EN-fallback** (`content_handler.go`: missing non-base locale serves EN + WARN log) + **bidirectional CI parity gate** (`TestContentCatalogLocaleParity`, en⇄de).
* [x] **Working Papers per-locale** — `scripts/sync-papers.mjs` (committed output + `--check` drift gate in lint) syncs `docs/methodology/{en,de}/` → `static/content/papers/{en,de}/wp-NNN.md`; WP reader fetches by locale. Per-paper titles/abstracts/status localised via `reflection/paper-display.ts` (messages); `papers.ts` keeps an English structural fallback.
* [x] **Locale-aware formatting** — `$lib/localization/format.ts` (rune-aware) + pure `format-core.ts` (node-tested); `en-CA`/`de-DE` Intl; replaced the hardcoded `en-CA` date + `toLocaleString` sites across the converted surfaces.
* [x] **Emic proper nouns stay untranslated** — probe/source display names, pillar names (Aleph/Episteme/Rhizome), metric machine ids verified not routed through messages; surface names localise (Atmosphäre/Reflexion; Workbench unchanged).

### Validation
* [x] Every converted surface renders DE+EN via the SideRail selector; the **parity gate** (`check-i18n-parity.mjs`, **1349/1349** after Phase 144b) + Paraglide compile fail on any en-only/de-only or stub message; `?lang=` round-trips + deep-links; date/number follow locale; WP bodies + editorial prose appear in DE; emic nouns untranslated. `make fe-lint` / `fe-test` / SSG build / bundle-size green; Go BFF content config+handler tests green. Playwright `tests/e2e/locale.spec.ts` (DE render + live switch + `?lang=de` deep-link) + a registry-driven SoT assertion in `tests/e2e/workbench.spec.ts` (`?lang=de` → View lever "Darstellung"/"Verteilung"); both run in CI. **DoD status:** Phase 144b closed the conceptual-vocabulary SoT tranche, and **Phase 144c (below) closed the last two data surfaces** — the `/reflection/open-questions` 50-question prose catalog AND the `how-to-read.ts` cell-note building blocks (surfaced by 144c's closing sweep). The parity gate now stands at **1426/1426** and the DoD "no hardcoded English in any user-visible surface" is **literally met**.

---

## Phase 144c: Open-Questions Research-Prose Localization (DE) [P1] - [x] DONE

*Closed the last English-in-DE surfaces after Phase 144/144b, making the literal DoD — "no hardcoded English in any user-visible surface" — fully true. Two data surfaces were localized: (1) the planned `/reflection/open-questions` Open Research Questions hub (50 questions / 250 prose fields, rendered straight from `src/lib/reflection/open-questions.ts`), and (2) — surfaced by the closing sweep (task f) — the `how-to-read.ts` cell-note **building blocks** (~55 config-derived strings rendered under the German template line in 19 cells; the per-presentation template line itself already came from the localized BFF `view_modes/howto_*` catalog, but `cross_probe_lead_lag` has no catalog entry so its English fallback also leaked). Operator chose (2026-06-18) to fold the how-to-read finding into 144c rather than defer it.*

**Implementation (DONE 2026-06-18, pending operator commit):**
- **Open-questions** — per-locale static catalog (operator-locked mechanism): `open-questions.de.ts` holds `OPEN_QUESTIONS_DE_PROSE` (50 entries keyed by `id`, prose-only); `open-questions.ts` keeps the locale-independent structure + a `QuestionProse` type + `localize()`/locale-aware `questionsByWp`/`getOpenQuestion` resolving via paraglide's `getLocale()` (relative import → node-safe, reactive, no reload). Page uses `$derived(questionsByWp())`. DE transcribed from the approved `docs/methodology/de/WP-00X-de-*.md` §7/§8. Parity gate = `tests/unit/open-questions-parity.test.ts` (every EN id has a populated DE entry, no orphans, accessors overlay under `de`). e2e `tests/e2e/open-questions.spec.ts` (`?lang=de` German catalog + EN default).
- **How-to-read** — the boundary-correct tier is Paraglide (UI microcopy with interpolation, not researcher-editable content): 77 new `cells_howto_*` keys/locale in `messages/{en,de}/cells.json` (EN reproduces the current strings verbatim so the substring-based `how-to-read.test.ts` stays green in node-env; `_one`/`_other` plural pairs; `{param}` interpolation incl. the strength/direction words + channel-label maps). `how-to-read.ts` now calls `m.*()`; the built-in fallback templates are localized too (so `cross_probe_lead_lag` is German). DE assertions added to `how-to-read.test.ts`.
- Parity gate now **1426/1426** (1349 + 77). All gates green: `pnpm` check / test:unit (721, cov 89/83/91/91) / lint / knip / build / bundle-size; e2e open-questions + locale + workbench (13) pass.*

**This is content work, not UI-vocabulary work** — it belongs in the data/content tier (NOT Paraglide `messages/`). The questions are faithful transcriptions of WP §7/§8, and **those sections are already translated** in `docs/methodology/de/WP-00X-de-*.md` — so the German is a *transcription from approved translations*, not a fresh translation. Budget the labour as scholarly transcription + QC of ~250 prose fields, not engineering.

**Grounding (read first):** `src/lib/reflection/open-questions.ts` (the catalog + `OpenQuestion` interface + `questionsByWp`), `src/routes/(app)/reflection/open-questions/+page.svelte` (renders `q.shortLabel`/`q.question`/`q.deliverable`/`q.pipelineHook`/`q.disciplinaryScope` directly; shell strings already Paraglide), the 6 German WP files under `docs/methodology/de/`, `src/lib/state/locale.svelte.ts` (the `locale()` rune), ADR-042 + the Phase-144 two-tier model in this ROADMAP. Note the BFF content catalog already has an `open_research_question` entityType with 8 per-WP-**section** files (`configs/content/{en,de}/open_research_questions/*.yaml`) — that is a *different granularity* (section-level reflexive notes), not the 50 per-question entries; do not conflate.

**Mechanism — decide first (operator/next-session call):**
- **Recommended — per-locale static catalog.** Keep the page pure SSG (no BFF round-trip / loading-error states on a static page; mirrors how WP bodies use a per-locale static source). Restructure `open-questions.ts` so each prose field is locale-keyed (`{ en, de }`) or split into `open-questions.de.ts`, selected via `locale()` (reactive, no consumer change — same pattern as the 144b SoTs). Keep the stable `id`/`sourceWp`/`subsection` etc. as locale-independent structure.
- **Alternative — BFF content catalog.** Add 50 per-question `open_research_question` entries `{en,de}` and rewire the page to `contentQuery(...)`. More "boundary-rule-pure" (researcher-editable prose → BFF content) and the entityType + plumbing exist, but it adds a network dependency + empty/error states to a today-static page and touches the internet-facing service. Heavier; only if the operator wants this prose researcher-editable at runtime.

**Tasks:**
* [x] Lock the mechanism (per-locale static catalog — operator-confirmed).
* [x] Transcribe all 50 questions' DE prose from `docs/methodology/de/` WP §7/§8 (faithful to the approved translations; WP-anchor numbers + emic terms + `Q1/Q2…` structure preserved).
* [x] Wire the page to resolve by the locale rune (`$derived(questionsByWp())`); reactive switch (no reload) + `?lang=de` deep-link verified by e2e.
* [x] Parity check `tests/unit/open-questions-parity.test.ts` — every EN question has a populated DE counterpart with identical `id`, no orphans, accessors overlay under `de` (node-safe Vitest gate; the messages-only `check-i18n-parity.mjs` doesn't cover TS catalogs).
* [x] e2e `tests/e2e/open-questions.spec.ts`: `?lang=de` renders a known German question string (authenticated fixture) + EN default.
* [x] Verify green: `pnpm run` lint / check / knip / test:unit / build / bundle-size.
* [x] Swept for other English-only TS data catalogs: `papers.ts` / `registry*` / `negative-space.ts` / `discourse-function.ts` are 144b structural fallbacks (rendered via `m.*()`), NOT raw. **One genuine finding — `how-to-read.ts` building blocks** — was localized in this phase (see above). Phase 144 header "one remaining gap" note flipped to closed; DoD literally met.

---

## Closure block (Phases 127–129 + 152 README) — run last, against the consolidated surface

*The three original Iteration-11 closure phases keep their scope but move **after** the consolidation pass (136–144): coherence, accessibility/performance, and documentation are verified against the cleaned, localized, green-CI surface — so the audit is not invalidated by a later refactor. They are also the bridge into Iteration 12: a coherent, documented, accessible app is the baseline the security/deployment reviews assume.*

## Phase 127: Dashboard Coherence & Progressive Descent [P2] - [x] DONE

*Holistic verification that every piece composes into one coherent dashboard. Run as a continuous lightweight checklist after each major complex (Pillar Sharpening, 123a, A/B/125, Auth) plus a final pass — avoids drift accumulation.*

**Grounding.** Read first: every surface route, the SideRail/chrome, the RefusalSurface component, the Negative-Space toggle, the URL-state grammar across surfaces. Preserve: the three-surface invariant (post-123a), the single RefusalSurface shape, deep-linkability. Verify-first: this phase is itself the reconciliation — treat each prior phase's output as the thing under audit.

### Routing & state`
* [x] **Back/forward navigation across all surfaces via unified AER designed buttons (arrows or named buttons)** ✅ DONE — global AĒR back/forward arrows (`←`/`→`) added to the SideRail under the surface anchors (`SideRail.svelte` `.rail-history`), visible on every authenticated surface (Reflection included). They defer to the browser's own history stack (`history.back()`/`history.forward()`): because the app mutates history directly via `history.pushState` (`url.svelte` `pushUrl`/`setUrl`), a reliable enabled/disabled state is NOT derivable from SvelteKit's nav hooks (the pushState entries desync any in-session index), so — like the browser's native toolbar — the arrows are always available and no-op at either end. Localized EN/DE (`chrome_nav_back`/`chrome_nav_forward` + titles). **Auth-page fix:** all post-auth transitions (the `(app)` layout bounce → `/login`, login submit, already-signed-in skip, accept-invite) now navigate with `replaceState`, so `/login` is never left in the back-stack — the back-arrow no longer flashes the login screen then bounces to `/`. Verified via Playwright.
* [x] **Back/forward state preservation + leave-guard** ✅ DONE — every reachable Workbench state is URL-encoded (deep-links round-trip), so browser back/forward already preserve charts/panels/config. The unsaved-work gap (leaving via SideRail / browser-back with a *changed* analysis) is closed by a **confirm modal** (`WorkbenchLeaveGuard.svelte`, via the `Dialog` primitive): a `beforeNavigate` guard in `workbench/+page.svelte` compares the current deep-link against a clean baseline (`src/lib/workbench/dirty.svelte.ts`) ONLY at navigation time (no standing effect — perf-neutral) and, when dirty, cancels the nav and offers three exits: **leave without saving** · **save as a new analysis** (name → `createAnalysis`) · **update the loaded saved analysis** (`updateAnalysis`, shown only when one was opened). The baseline resets to clean on entry / saved-analysis load (page effect keyed on `savedAnalysis`) and on every successful Save (`AnalysesOverlay` create/update). A clean or unconfigured Workbench leaves silently. *(An earlier localStorage quick-save + restore-card approach was reverted — it degraded load performance and was heavier than the operator wanted; the operator-specified design is this navigation-away confirm modal only, no browser-close guard.)* Localized EN/DE (`workbench_leave_*`). All five paths verified via Playwright.
* [x] **Re-enable the Phase-136 quarantined E2E tests (MUST).** ✅ DONE — all 3 rewritten against the three-surface grammar + current cell DOM and un-skipped (no `QUARANTINED`/`test.skip` markers remain; full e2e 56 passed / 0 skipped): `atmosphere.spec.ts` → probe→Dossier descent via the Atmosphäre overlay (`?dossier=open&selectedProbes=`); `topic-views.spec.ts` → `topic_distribution` as an Aleph Workbench cell (`?activePillar=aleph&aleph=<base64url-json>`) firing `/topics/distribution` + rendering a `.plot-host` ridge; `iteration6-closure.spec.ts` → `cooccurrence_network` as a Rhizome cell firing `/entities/cooccurrence`, node→Wikidata external links (`svg.graph g.node`, `[data-testid="entity-external-links"]`). Seeds generated via the app's real `encodePillarState`; d3-force node clicks made robust with `toPass`.

### Coherence audit (three surfaces)
*Spot-verified via a logged-in live walkthrough (Playwright, real session via `make create-admin`): all three surfaces + the Workbench create-mode ScopeEditor + a built Aleph distribution cell render coherently; the negative-space/refusal disclosures are present and on-brand (globe "Coverage disclosure" no-geo-claim card; per-cell `WITHHELD` "1 metric not present on every scoped source" + `NO SIGNAL` "constant across this scope, dropped not silently filtered — ADR-039"; `FunctionBadge`). Exhaustive per-refusal-type and full cross-probe-symmetry enumeration remains a continuous checklist, but no incoherence surfaced in the walkthrough.*
* [x] Pillar-identity coherence; metric+metadata inventory coherence (all in the picker, EN+DE explanations); "always explained" coherence; configurable-cell coherence.
* [x] Refusal consistency (cross-frame equivalence, refusal-as-cell, merged-cross-probe guard, k-anonymity, Silver-eligibility, invalid_language) — one `RefusalSurface` shape with anchor + alternatives.
* [x] Negative-Space / empty-state audit (tier asymmetry, metadata asymmetry, coverage gaps, CI/SF, silent-edit `no_snapshots`).
* [x] Cross-probe symmetry; Dossier-overlay coherence; Auth-surface coherence.

### Validation
* [x] Deep-link round-trip + the new SideRail back/forward + the Workbench leave-guard (all five paths) + the auth-page back-flash fix verified via a live Playwright walkthrough. The dedicated **keyboard-only / reduced-motion E2E** is folded into **Phase 128** (Accessibility + Performance Verification), which owns the axe/keyboard/reduced-motion gate across the whole surface.

### Known finding (resolved)
* [x] **ScopeEditor modal tucked under the SideRail at narrow viewports.** ✅ FIXED — the create-mode `ScopeEditor` backdrop centred in the full viewport (`inset: 0`) below the rail (`z-index 50` < `450`), so for any window narrower than ~1712px the panel's left edge slid under the rail (clipped `SCOPEEDITOR` eyebrow + heading; the **leftmost probe checkbox was rail-intercepted / un-clickable**). Invisible on wide monitors (≥~1712px), which is why it went unnoticed. Fix: the backdrop now reserves the rail gutter — `inset: 0 0 0 var(--rail-width)`, matching `.workbench-main` — so `place-items: center` centres within the content area and the panel's `100%` resolves to that area's width. Verified via Playwright at 1366px: panel left edge x=200 (≥184), both probe checkboxes clickable, header un-clipped. Layout-only, no behaviour change.

---

## Phase 128: Accessibility + Performance Verification [P1] - [x] DONE

*Full WCAG 2.2 AA + Lighthouse CI + High-Fi hardware performance across the complete post-Iteration-10 surface and both content languages (DE+FR alongside EN/DE UI).*

*Runs after Phase 144 — the EN/DE UI-parity check below is now backed by a real localized UI (Phase 144), not aspirational; the Lighthouse/bundle budgets compose with the Phase-137 CI performance budget.*

**Grounding.** Read first: existing Lighthouse/bundle-budget config + the a11y E2E setup, Brief §10 (performance budgets), the engine-3d reduced-motion path. Preserve: existing bundle budgets (shell), the reduced-motion contract. Verify-first: Phase 127 substantially in place (this audits the coherent surface).

* [x] **Axe audit** over every reachable state (three surfaces × Dossier overlay × configurable cells × Composition cells incl. D3-force × auth surfaces × overlays × single/multi-probe). Zero AA violations. — `tests/e2e/a11y-app.spec.ts` (+ existing `a11y.spec.ts`); fixed 4 AA violation classes: Plot `aria-prohibited-attr` (`plot-a11y.ts`), pillar/DF text contrast, panel-host `nested-interactive`, control-strip `target-size` (2.5.8).
* [x] **Modal a11y** (Dossier overlay focus-trap/Esc/ARIA — already in place); keyboard-operable cell config (fixed `CellConfigPopover` focus-on-open + restore); auth surfaces narrated (`AuthNotice` `role="alert"`). Behaviour pinned by e2e.
* [x] **Lighthouse + bundle budgets** — `@lhci/cli` + `lighthouserc.cjs` (`make fe-lighthouse`, CI step); bundle gate extended with a per-other-lazy-chunk cap so a new feature chunk (e.g. relational network) fails CI on regression.
* [~] **Performance** — Kriesel network force budget (auto-stop + Settle cap, already present); **engine pause while overlay open** + tab-hidden pause (`engine.setActive` + Page Visibility, wired in `AtmosphereSurface`). **60 fps / <16 ms frame on M1-class hardware = operator manual pass** (procedure filed in the Operations Playbook).
* [~] **Reduced-motion** (fly-to→instant via `engine.reducedMotion`; network→static fallback) ✅. **EN/DE parity** enforced by the Phase-144 i18n-parity gate ✅. **Screen-reader pass = operator manual pass** (checklist filed in the Operations Playbook).
* [x] Arc42 Accessibility & Performance Envelope (§10.3); CI gate spec + bundle-budget table; Operations Playbook manual-pass checklist.

### Validation
* [~] All CI gates green (axe / Lighthouse / bundle / lint / unit / e2e). **Hardware-test + screen-reader logs = operator-pending** (procedures documented in the Operations Playbook → *Manual accessibility + hardware pass*).

## Phase 129: Documentation Sweep + Terminology Reconciliation [P-Docs] - [x] DONE

*Final consolidation against post-Iteration-10 reality. **Absorbs the Phase-138 stale-docs worklist** — the operator's "go over all `/docs` files and check they are correct" is this phase, now evidence-driven from the inventory rather than ad-hoc.*

*Done 2026-06-20. The `quality_register.md` §5 stale-docs list was the worklist (all checked off there). Both mkdocs sites build `--strict`. Spec-vs-reality corrections surfaced: `add-a-language.md` is hand-authored, not auto-generated (the "cutover" expectation was itself stale); the terminology question is a science↔code **swap**, not a one-word rename. README RSS items deferred to Phase 152 (operator decision — full rewrite there).*

**Grounding.** Read first: all ADRs in `docs/arc42/09_architecture_decisions.md`, CLAUDE.md, the extending guides, the Operations Playbook, `mkdocs.yml`. Preserve: the MkDocs strict-build constraint (cross-references resolve), the English-only documentation rule. Verify-first: which ADRs actually landed vs. planned before writing the coherence pass.

* [x] **Arc42 sweep** — three surfaces / pillar identity / Workbench reconciled across ch03/05/12/13 + §8.17; **access-control layer** (ADR-040) added as §8.7.7 (it was missing from arc42 — a real correctness gap); metadata-comparability (ADR-038) pointer in §8.32; all cross-references resolve.
* [x] **ADR coherence** — verbatim **ADR-023 duplicate removed**; ADR-numbering convention documented (insertion-order, ADR-032 after ADR-035); ADR-025/026 confirmed still-correctly-Deferred (triggers N=10 / N≥5, current N=2); **`docs/design/reframing-note.md` deleted** (+ denav + design_brief provenance row).
* [x] **CLAUDE.md sweep** — reviewed; already current (web-crawler, Workbench, ADR-040); only the now-deleted `_archived/rss-crawler/` references updated. *(CLAUDE.md is gitignored — not in the committed diff.)*
* [x] **Working-Paper cross-link audit** (clean — no retirement drift, §-refs resolve post-renumber); **Operations Playbook** end-to-end (crawl/volume/network/e2e/TOC tables + archived section); **extending guides** reviewed; `add-a-language.md` confirmed hand-authored + current (not auto-generated — spec drift noted).
* [x] **RSS-crawler retirement** — `crawlers/_archived/rss-crawler/` (whole `_archived/`) deleted; references repointed to git history.
* [x] **Terminology reconciliation** — `docs/development/terminology-reconciliation-proposal.md` written (Path A reconcile / Path B status-quo / **Path C labels-only** via the existing `displayName`/`shortName` layer, recommended-to-evaluate). Framed against the invited-researcher audience (ADR-040). Nothing renamed; the decision stays open in the doc. *(Committed directly as the review artifact — operator decision 2026-06-20: no separate PR for a solo dev.)*
* [x] **08_concepts duplicate numbering** — the second `8.12`–`8.22` block renumbered monotonically to **§8.25–8.34** (inbound refs verified safe; one internal ref §8.13→§8.26 fixed); stale Surface-II/Function-Lane sections given supersession banners.

### Validation
* [x] `mkdocs build --strict` passes (both public + internal sites); all cross-references resolve (one pre-existing `add-a-source` anchor fixed; a couple of unrelated pre-existing anchor INFOs remain, non-fatal); terminology artifact committed (no PR per operator); ROADMAP marked DONE in place per the closure-block convention.

---

## Phase 152: Public README & Project Entry Point [P-Docs] - [x] DONE

*The README is the front door for external people who discover the project — researchers, collaborators, the curious. This phase produces a current, well-presented README as the entry point. Supplemented with the real deployment/install section after Iteration 13 (Phase 149).*

*Done 2026-06-20. **Drift correction (operator, 2026-06-20):** the original note pegged this phase to a "post-design (Phase 151) surface" — there is **no Phase 151**; the dashboard surface is already final, so screenshots were captured from the current live surface. The README is a concise dual-audience front-door (researchers + developers) with deep developer reference linked out to the existing docs rather than embedded (DRY).*

**Grounding.** Read first: the current `README.md`, CLAUDE.md (project identity + architecture), the Arc42 manifesto + scientific-foundations chapter, the provisional-metrics + Negative-Space sections (the honesty surfaces), the Working Papers (for the POC caveats), the design brief (for visual consistency of any diagram). Preserve: the English-only rule, the manifesto framing (atmospheric sensor, not surveillance instrument), the provisional/POC honesty. Verify-first: every claim in the README is checked against shipped reality — no aspirational features.

* [x] **Rewrite the README** — manifesto framing, the medallion architecture, the three surfaces + Pillars, how to run it locally (Makefile), where the docs live. Concise, dual-audience (**For Researchers** / **For Developers**), well-structured; verified against shipped reality (three surfaces, auth-gate, Probe 0 + Probe 1, provisional metrics).
* [x] **Architecture diagram** — a committed **Mermaid** flowchart of the medallion data flow (web-crawler → Ingestion → Bronze → NATS → Worker → Silver/Gold → BFF → Dashboard) + Postgres metadata/auth. Renders natively on GitHub; source-controlled so it stays maintainable.
* [x] **Dashboard screenshots** — captured from the **current live surface** (no Phase 151): `docs/assets/readme/{atmosphere,workbench-aleph,workbench-episteme,reflection}.png` — Atmosphère globe, Workbench Aleph (distribution histogram, 583 articles), Workbench Episteme (time series), Reflexion (WP-001). Reproducible capture recipe in `docs/assets/readme/README.md`.
* [x] **POC honesty banner (non-negotiable).** Up-front GitHub `[!IMPORTANT]` banner: engineering proof-of-concept, **provisional** metrics, scientific validity **limited / not yet peer-validated**.
* [x] **Deployment placeholder** — a "Deployment" section stub pointing to Iteration 13 (Phase 149) for the real install / run-in-prod instructions.

### Validation
* [x] README renders cleanly on the git host (standard markdown + Mermaid + GitHub admonition); every feature claim checked against shipped reality; the Mermaid diagram + four current screenshots are in place; the POC / limited-validity statement is prominent at the top; all doc/file links verified to resolve.


---

## Phase 153: Transactional Email for Invites & Password Reset [P0] - [x] ENGINEERING-COMPLETE (pending operator: Brevo account + live end-to-end validation)

*Engineering done 2026-06-20 (pending operator commit). Provider-agnostic SMTP relay (`net/smtp` + STARTTLS, no new Go dependency) behind the existing `notify.LinkSender` seam; default provider **Brevo** (EU/DSGVO). Bilingual EN+DE plain-text templates (no tracking pixels). Optional all-or-nothing SMTP config (empty `SMTP_HOST` → LogSender fallback, keeps `make create-admin` break-glass; partial group → boot error). Graceful failure: send errors logged at ERROR, admin responses carry `delivered` (OpenAPI `AdminActionLink`) and the UI shows the one-time link for manual delivery when not delivered; forgot-password stays 202 (no enumeration). Decision recorded in **ADR-043** + Operations Playbook §"Transactional Email". The Validation box stays open until the operator creates the Brevo account, sets the secrets, and runs the live end-to-end test (a relay is required and cannot be unit-tested).*

*Pulled forward (execution order) ahead of the Phase 145–147 reviews: 153 introduces the only new internet-reachable auth surface in this iteration (the email-delivery path), a new external provider dependency, and a new secret — building it first means Stage A scrutinises the **complete** auth / secret / dependency surface (incl. the now-live `forgot-password` mail-bombing/abuse vector) instead of a half-wired flow, and 146's secret inventory + notification-channel decision cover it from the start. The phase number is unchanged per this file's "numbers are insertion-order ids, not execution order" convention.*

*The deployment target is **invited** researchers, but auth today is bound to `make create-admin` and has **no real email channel** — invite / accept-invite / forgot-password / reset-password flows exist as endpoints (ADR-040) but cannot actually reach a human. Without email delivery the invite-only model is inoperable, so this is a launch prerequisite, not a nice-to-have. The phase first determines the best free, low-dependency, stable solution, then wires it.*

**Grounding.** Read first: the BFF auth flows (`internal/auth/`, the `/auth/*` endpoints — accept-invite, forgot/reset-password), ADR-040, the boot-time secret validator + `.env.example`, the privacy-minimal posture (Phase 55 — only email + hash stored), the EU-residency requirement. Preserve: the no-token-in-link sharing posture (a notification email carries no bearer access — ADR-040), the secret-validation discipline, DSGVO minimalism. Verify-first: confirm exactly which flows need outbound email and what each message must (and must not) contain.

### Assess (free · stable · low-dependency · EU/DSGVO)
* [x] **Evaluate transactional-email options** against the constraints — must work permanently, be as free as possible, add no fragile dependency, and be DSGVO-appropriate (the provider processes user email addresses → EU residency / DPA matters). Compare hosted free-tier transactional-email providers vs. an SMTP relay; **explicitly reject self-hosted SMTP** as the default (deliverability / spam / maintenance burden is hostile to a solo dev). Record the decision (short ADR or playbook note) with the free-tier limits + the failure/abuse story. → **Decision: provider-agnostic SMTP relay (stdlib `net/smtp`), default Brevo (EU, 300 mails/day free, DPA). ADR-043 + Playbook §"Transactional Email" (free-tier limits + abuse/outage story).**

### Implement
* [x] **Email delivery wired** for invite, accept-invite, and forgot/reset-password — minimal templates, English (+ DE per Phase 144), no tracking pixels (anti-surveillance posture). Provider credentials go through the boot-time secret validator + `.env.example` (REPLACE-ME placeholder). → `notify.SMTPSender` (STARTTLS) + bilingual `templates.go`; `SMTP_*` config with grouped boot validation; wired in `cmd/server/main.go`. (accept-invite needs no outbound mail — it consumes the invite link.)
* [x] **Graceful failure** — a send failure is logged + surfaced to the admin, never a silent drop of an invite; the `make create-admin` path stays as the break-glass admin bootstrap (documented fallback). → send errors logged at ERROR; admin `delivered` flag + UI manual-delivery note; LogSender fallback when SMTP unset.

### Validation
* [x] Unit/integration-verifiable parts: grouped config validation (host-set-but-credential-missing → boot error; complete group → `EmailEnabled`), SMTP message framing (bilingual body, CRLF, RFC 2047 subject, header-injection guard), `delivered` true/false/fallback paths. lint + Go suites + `svelte-check` + i18n-parity green.
* [x] **Live end-to-end (operator) — deferred to deployment (Phase 149).** An admin invites a researcher by email; the invitee receives a working accept-invite mail and activates (consent captured); forgot/reset-password delivers; a provider outage degrades gracefully with operator visibility; credentials validated at boot. *Requires a live Brevo account + a sending domain + secrets — cannot be unit-tested.* **No spend / no action needed until deployment:** until then the stack runs on the `LogSender` fallback (empty `SMTP_HOST`) — invites/resets still work, the admin delivers the one-time link manually. The Brevo account + sender-domain authentication + `SMTP_*` secrets are wired in **Phase 149** (the email sender lives on the deployment domain decided in Phase 148), so this box is ticked there, not here.


# Iteration 12 — Production Readiness & Deployment Hardening

*The deployment gate for the first invited researchers. **Target (binding, 2026-06-14):** invited, authenticated researchers on a controlled domain — internet-exposed, no open self-registration. Runs AFTER Iteration 11: the reviews assume a clean, green, localized, documented baseline, so "some findings are already closed" by the time they start. Two stages, the professional assess → triage → remediate pattern: **Stage A** produces a single findings register (the reviews fix nothing); a **triage gate** classifies every finding BLOCKER / SHOULD / LATER against the invite-researcher Definition-of-Done; **Stage B** remediation phases are derived from the register and numbered post-triage (their content is not knowable until the audit runs — this is deliberate, not under-specified). Ends with a Go/No-Go launch checklist.*

*Threat model (Stage-A input).* Internet-exposed surface (bots scan everything) + a small set of authenticated, semi-trusted researchers behind invite-only auth (Phase 134/135). STRIDE the BFF / Traefik / auth surface; assume no open registration; assume the operator is the only admin initially. Code-quality refactoring is NOT a deployment gate here (it was Iteration 11) — Stage B closes security/ops BLOCKERs only; CLEAN/DRY refinements surfaced in review go to the standing backlog (LATER).

---

## Phase 145: Security Review, Threat Model & Pentest [P0] - [x] STAGE-A REVIEW DONE 2026-06-21 (live-repro confirm-subset pending; remediation = Stage B)

*Stage A. The internet-facing surface under adversarial review. Produces findings, fixes nothing.*

> **Privacy of findings (load-bearing).** This repo is **public** and `docs/` is the MkDocs source. The detailed findings register — severities, exploit scenarios, `file:line` — is therefore held **privately and gitignored** at `.security/phase145_security_findings_register.md`, **never committed**. Only the *method* and *triage counts* live in this public ROADMAP; the Stage-B remediation phases (below) are likewise worded to describe fixes generically, not to publish the unfixed weakness. A committed unredacted register would be an attack roadmap for the very deploy this iteration is gating.

**Method (executed).** Adversarial multi-agent sweep: **10 independent finders**, one per trust-boundary / OWASP class, blind to each other → **refute-by-default verification** of every finding (a skeptic tries to disprove each; only code-grounded findings survive) → synthesis into a triaged register. 41 raw findings → 4 refuted → **29 after cross-domain dedup**. This is a structured *self-review* and is recorded as **necessary-but-not-sufficient**: a live external pentest remains advised before scaling beyond the first vetted invite cohort (the solo-operator budget makes the self-review the launch gate, not a substitute for eventual independent testing).

**Grounding.** Read first: the BFF auth middleware (`internal/auth/`), the ingestion-api `pkg/middleware/apikey.go`, the Traefik config (TLS, headers, sole-ingress posture), the Security Defaults section of CLAUDE.md, Phase 75 (BFF security hardening), ADR-040 (session model). Preserve: every existing security default — this is a review, not a rewrite. Verify-first: enumerate the actual internet-reachable surface (Traefik routes, BFF endpoints, MkDocs) before testing.

### Trust-boundary map (the review spine)
Six boundaries enumerated and STRIDE'd (detail in the private register): **Internet→Traefik**, **Traefik→BFF** (incl. the X-Forwarded-For trust question), **Internet→MkDocs**, **Browser→BFF**, **Crawler→Ingestion**, **BFF→Postgres/ClickHouse**.

### Review coverage (done)
* [x] **Threat model (STRIDE)** of the auth / session / BFF / Traefik surface, per trust boundary — documented (private register).
* [x] **Auth/session pentest** — session fixation/rotation, CSRF (`Sec-Fetch-Site` + SameSite), cookie flags (`__Host-`/httpOnly/Secure), invite/accept/reset flows, RBAC boundary (researcher→admin escalation), brute-force/throttle. **Result: the application-layer auth model is confirmed sound;** the operator's "deep-link / URL-swap → suddenly logged-in" concern is **refuted by code** (the client route-gate is advisory, the BFF is authoritative, no gated data ships in the static bundle, `safeRedirect` holds).
* [x] **Web surface** — OWASP-class checks on every BFF endpoint (authz on every data route, input validation, body caps, error leakage, injection on the ClickHouse/Postgres query paths), security headers (HSTS, CSP, frame/`nosniff`), TLS config. **SQL-injection refuted** (all values bound; interpolated fragments allowlisted).
* [x] **Secrets posture** — boot-time validation coverage; no secret reachable from client JS or logs; `.env.example` completeness; **GitHub Actions secrets audit** (the operator's open question).
* [x] **Findings → private register** with severity (CVSS-ish) + repro + fix sketch. No fixes here.

### Triage gate (against the invite-researcher Definition-of-Done)
- **BLOCKER ×2 · SHOULD ×16 · LATER ×11** (detail private).
- **The two BLOCKERs are ingress / deployment-posture defects (the prod compose overlay), not application-logic flaws** — they gate any internet-facing deploy and are closed in Stage B before launch.
- Confirmed-SAFE controls (advisory client gate, `safeRedirect`, CSRF fallback, no SQLi) are recorded explicitly so the invariants are auditable, not assumed.

### Validation
* [x] Threat model documented; the internet-reachable surface enumerated and reviewed; findings register populated + triaged BLOCKER/SHOULD/LATER (private).
* [x] **Confirm-real live-repro subset** run against `make debug-up` (2026-06-21) — runtime-dependent severities exercised (timing/enumeration, throttle-lockout, gate-anchoring); triage locked + one new finding surfaced. Results in the private register §7. The post-fix "verify-fixed" checks belong to each Stage-B phase's own DoD.

**→ Remediation:** tracked in **Stage B → Security-remediation phases (SR-1..SR-5)** below — *145-derived, provisional* until 146/147 findings merge in at the triage gate.

---

## Phase 146: Infrastructure & Deployment Readiness Review [P0] - [x] STAGE-A REVIEW DONE 2026-06-21 (verdict NOT-GO until Stage B; remediation = SR-6/SR-7)

*Stage A. Everything a POC never needed but a deployment does — prod compose, secrets, backups, monitoring-in-anger, recovery.*

> **Result (private register §"Phase 146", SEC-031..064 — 6 BLOCKERs).** Method: 8-auditor readiness sweep (one per infra dimension), seeded with the verify-first inventory. **Verdict: NOT-GO for an unattended single-box invite-launch as configured today — because this review surfaced a BUILD, not just config edits.** The readiness layers an operator assumes exist are *absent*, not misconfigured: off-box backups + a *tested* restore, alert routing that actually reaches a human, a production deployment runbook, and a working first-admin-bootstrap path on a prod box. Several BLOCKERs are config-only/quick; the heavy items are the backup mechanism + the alert-routing/host-signal collection. **9 operator decisions (D1–D9)** were surfaced (backup approach/retention/key-custody, alert channel, observability-footprint + launch host RAM, crawl scheduler, first-admin shape, ACME staging, and the durable `env_file`-vs-allowlist root-cause). Remediation lands in **SR-6 + SR-7** (Stage B below); the absent build is the launch gate.

**Grounding.** Read first: `compose.yaml` + `compose.prod.yaml`, `.env.example`, the infra init containers (`minio-init`, `nats-init`, `clickhouse-init`, `postgres-init-roles`), the observability stack (OTel→Tempo/Prometheus/Grafana), the Operations Playbook, the `make reset`/backup story, the GitHub secrets set. Preserve: IaC-only provisioning (Hard Rule 5), the SSoT tag model, zero-trust network posture. Verify-first: inventory what `compose.prod.yaml` actually changes vs. dev before reviewing.

* [ ] **Prod compose review + `compose.yaml`↔`compose.prod.yaml` drift.** First reconcile the two files: confirm `compose.prod.yaml` is still current against `compose.yaml` (the SSoT for image tags) — no stale/missing services, tags, env, or overrides; the prod overlay only *adds/overrides* deliberately, never silently diverges. Then review the prod config itself — TLS/Traefik prod config, resource limits, restart policies, log handling, no debug ports exposed, secret-injection model.
* [ ] **Secrets management** — full required-secret inventory across services; GitHub Actions + deployment secrets reconciled with the boot-time validators and `.env.example`; rotation story.
* [ ] **Backup & restore** — Postgres (auth + metadata), ClickHouse Gold, MinIO buckets: documented, **tested** restore (not just "we take backups"). Recovery runbook.
* [ ] **Monitoring/alerting in anger** — which signals page the operator (service down, DLQ growth, disk, cert expiry); dashboards + alert rules; the observability stack wired for prod.
* [ ] **Advisory-security notification (relocated from Phase 137).** govulncheck / pip-audit / Trivy were made *advisory* (report, don't block) in Phase 136 — visible in every CI run but easy to miss on a green check. Add a scheduled run + a notification (push/email) so a demoted HIGH/CRITICAL finding is never silent. Needs a notification-channel decision.
* [ ] **Deployment runbook** — first deploy, upgrade, rollback, the `make create-admin` bootstrap, the public/internal docs split (Phase 134).
* [ ] **Findings → register**, triaged.

### Validation
* [ ] A restore from backup succeeds in a clean environment; the secret inventory is complete; the deployment runbook is followed end-to-end on a staging target; findings registered + triaged.

---

## Phase 147: Per-Service Code & Robustness Review [P1] - [x] STAGE-A REVIEW DONE 2026-06-21 (code SOLID, zero BLOCKERs; remediation = SR-8)

*Stage A. Each service under `/services/` (+ `pkg/`, the crawler) reviewed for robustness, error handling, resource safety, and CLEAN-architecture adherence — beyond the mechanical quality pass of Iteration 11 (dead code, naming, file length). This is the senior-eyes review of each service's correctness and failure behaviour.*

> **Result (private register §"Phase 147", SEC-065..098 — ZERO BLOCKERs).** Method: 9 per-service reviewers (senior-eyes) + refute-by-default verification, classifying real robustness defects vs CLEAN/DRY refinements (the latter → backlog, never a gate). **Verdict: the per-service code is SOLID with bounded, well-understood defects; nothing launch-blocking.** ~9 genuine defects (rest are `should` hardening + `later` refinements). The two worth an operator's attention — failure mode is *silent, unalarmed analytical data loss* under specific failure windows — are recorded as the highest-priority `should`s and recommended before sustained production crawling. Code robustness does **not** change the Phase-146 NOT-GO (that is owned by the absent infra build). Remediation lands in **SR-8** (Stage B below).

**Grounding.** Read first: each service's `cmd`→`config`→`core`→`storage` layering, the worker extractor/adapter pipeline + graceful-degradation contract, the BFF query paths, the ingestion-api body/timeout caps, the crawler politeness/state model. Preserve: the load-bearing constraints in CLAUDE.md (SilverEnvelope contract, language-detection-first ordering, extractors must not mutate core, graceful-degradation vs DLQ rules). Verify-first: review against the documented constraints — flag any drift as a finding.

* [ ] **ingestion-api** — body caps, timeouts, error paths, auth, MinIO/Postgres failure handling.
* [ ] **bff-api** — query robustness, authz coverage, input validation, ClickHouse/Postgres pool + timeout handling, generic-5xx discipline.
* [ ] **analysis-worker** — extractor graceful-degradation, DLQ-boundary correctness, pool timeouts, idempotency (ReplacingMergeTree dedup), re-attempt-framework health.
* [ ] **web-crawler** — politeness defaults, conditional-GET/state correctness, discovery-channel telemetry, failure isolation.
* [ ] **pkg/ + dashboard** — shared-code soundness; dashboard error/empty/loading states, request-layer resilience, no secret leakage.
* [ ] **Findings → register**, triaged (CLEAN/DRY refinements that are not blockers → LATER).

### Validation
* [ ] Every service has a recorded review with findings triaged; load-bearing-constraint drift is fixed-now (if BLOCKER) or registered.

---

## Stage B — Remediation (derived) + Go/No-Go [ ] TODO — Stage A complete + triage gate passed 2026-06-21

> **Progress 2026-06-21 (~62/100 findings closed + committed).** **Track A — topology-independent, DONE:** SR-2 ✅, SR-3 ✅ (SEC-014 → 149), SR-4 ✅ (SEC-030 → 149), SR-5 ✅ (SEC-010 compose-half → 149), SR-8 ✅, **plus the disk-fill class** SEC-055 + the two NEW capacity-audit findings SEC-099 (ClickHouse system-logs) / SEC-100 (NATS stream) ✅ live-verified. A handful of LATER items are accept-with-rationale (SEC-007/012). **Track B — deployment-coupled, REMAINS = SR-1 / SR-6 / SR-7, executed + verified in Phase 149** (all 8 BLOCKERs live there). **Phase 148 decisions/ADR-044 are DONE** (D1–D9 locked). **Stage B + Go/No-Go close at the end of Phase 149.** Itemised SoT: private `.security/REMEDIATION_PLAN.md` + `PHASE148_DECISIONS.md`.

*Derived from the complete Stage-A register (Phases 145 + 146 + 147, all reviewed; one shared private register, one triage gate). Eight remediation phases (SR-1..SR-8), grouped by area. The phase text describes fixes **generically** (the unfixed weakness is not published in this public repo); the concrete findings + repro live in the private register (`.security/…`). **Whole-register tally: ~96 findings, 8 BLOCKERs** (SEC-001/002 ingress + SEC-031..036 deploy-build; Phase 147 added zero). The launch gate is the Phase-146 absent BUILD (backups/restore/runbook/first-admin) + the two ingress BLOCKERs.*

**Remediation ambition (operator directive, 2026-06-21): eliminate as many attack vectors as possible — not the launch-minimum.** Stage B closes **every BLOCKER and every SHOULD**, and **pulls LATER hardening forward wherever the fix is cheap and low-risk** (e.g. the boot-guards, the loopback-binding of debug ports, the defense-in-depth role checks). Only genuinely deferred-by-design items (latent / forward-looking, with no present exploit path) stay on the standing backlog, each with a recorded rationale rather than silent omission. Where findings share a root cause, fix them as one change (e.g. the ingress-posture fixes land together in the prod overlay); apply the register's sequencing notes (a prerequisite fix precedes the control that depends on it).

### Security & Readiness remediation phases (SR-1..SR-8)

*The full derived set across all three Stage-A reviews: **SR-1..5** close the 145 security surface, **SR-6..7** the 146 infra/deploy build, **SR-8** the 147 per-service robustness defects. Scope is the **target posture**; per-finding detail lives in the private register. Findings are referenced by opaque ID (SEC-xxx) for closure-tracking only — the IDs reveal nothing without the register. **Several SR-6/SR-7 items are blocked on operator decisions D1–D9** (register §"Phase 146" §3) — resolve those before building. Each phase's DoD is "the listed findings' private-register `Pass =` / live-repro checks succeed."*

**SR-1 — Ingress & Deployment-Posture Hardening [P0 — launch gate]. → executed in Phase 149.** Bring the production compose overlay + Traefik into line with the documented zero-trust posture: only Traefik (80/443) public; admin + observability consoles reachable only via an authenticated/allow-listed path (or not at all); no backend service host-published in prod; client IPs normalised at the trusted proxy hop; TLS options pinned; debug forwarders loopback-bound. *Closes:* SEC-001, SEC-002 (BLOCKERs) + SEC-018, SEC-003 (SHOULD) + SEC-023, SEC-024 (cheap LATER, pulled forward). One coherent overlay/Traefik change. **Launch is not authorised until SEC-001 + SEC-002 pass their live-repro (connection-refused / forward-auth).** Also delivers the proxy-trusted-IP precondition that SR-3 depends on.

**SR-2 — BFF Auth & Session Boundary Hardening [P1]. [x] DONE 2026-06-21** (SEC-005 folded in: self-service `/auth/sessions` list + log-out-everywhere shipped). Tighten existing auth controls (no new surface): anchor route auth-exemption to exact resolved routes, not unanchored path suffixes; make the login throttle unable to deny a holder of correct credentials (evaluate creds before any hard-block / key on IP+account with a step-up rather than a victim-refusal); revoke a user's live sessions on admin-initiated reset; add the production secure-cookie boot guard; replace substring-based admin gating with a prefix/sub-router; correct the stale gateway-auth comments (+ a CI guard). *Closes:* SEC-013, SEC-020, SEC-008, SEC-004, SEC-011, SEC-025, SEC-026. *Decide at phase start:* fold in SEC-005 (self-service session list + revoke-all) or send it to backlog.

**SR-3 — BFF DoS & Resource-Safety Hardening [P1] (after SR-1). [x] DONE 2026-06-21** (SEC-014 per-client rate-limit deferred to SR-1/Phase 149 — needs the proxy-trusted-IP fix first; SEC-012 accept-with-rationale, closed by SEC-072's fan-out cap). Bound every request and query: per-route request-body caps (413, not buffered-then-500); length + per-user row caps on saved analyses; a per-client (post-auth) rate limiter replacing the single global bucket; a fan-out cap on the multi-query report path. *Closes:* SEC-015, SEC-016, SEC-014, SEC-012. **Depends on SR-1** (the per-IP keying is only trustworthy once the proxy normalises client IPs).

**SR-4 — Email & Token-Flow Hardening [P0 — required by the iteration DoD]. [x] DONE 2026-06-21** (app-half: async forgot-password, reset throttle, SMTP conn-deadline, prior-token invalidation, base-URL guard, fragment-borne tokens. SEC-030 SMTP-into-container forwarding → Phase 149 — oracle already closed so enabling email there is safe). Make the email-driven invite/reset flow both *work* and *not leak*: wire the SMTP config group into the BFF container (today it is not plumbed in, so transactional email is inert — the onboarding DoD cannot be met without this); move delivery off the request path with constant work on both branches so the response carries no account-existence signal; bound the whole SMTP conversation with a deadline (not just the dial); add a per-account cooldown + single outstanding token; require the public base URL when email is enabled; keep raw tokens out of server-side request logs. **SEC-030 + SEC-019 are fixed together** so enabling email does not simultaneously open the timing oracle. *Closes:* SEC-030, SEC-019, SEC-022, SEC-006, SEC-027, SEC-009.

**SR-5 — Error-Leakage & Security Headers [P2]. [x] DONE 2026-06-21** (SEC-017 generic error handlers, SEC-021 CSP via SvelteKit kit.csp build-hashed, SEC-010 CORS boot-guard app-half; SEC-010 compose `CORS_ALLOWED_ORIGINS` wiring → Phase 149). Generic request/response error handlers (no raw binding/dependency text to the client); wire the CORS allow-list through to the container + a boot guard against an effective wildcard outside development; add the Content-Security-Policy (+ Trusted-Types) the docs already assume. *Closes:* SEC-017, SEC-010, SEC-021.

**SR-6 — Backup/Restore & Monitoring [P0 — launch gate]. → executed in Phase 149** (the disk-fill subset is already DONE + live-verified, pulled forward: SEC-055 container-log rotation across all services + the two capacity-audit findings SEC-099 ClickHouse system-log disable/TTL [system DB 1.27 GiB→1.02 MiB] + SEC-100 NATS stream max_age/max_bytes). Build the recover-from-loss layer that does not exist today: scheduled off-box (EU, GDPR-processor) backups of the irreplaceable stores (auth/consent + crawl cursors, the object-storage corpus, the analytical store incl. the indefinite-retention rollups captured as partitions, not replay), encrypted + retention-pruned, with a **restore tested in a clean environment** and a recovery runbook. In the same change, make monitoring *reach a human*: an alert route/contact point, host-level signal collection (disk-fill, memory/OOM, cert-expiry, per-service liveness), backup-success + crawl-staleness integrity alerts, and a persisted/retained metrics store. *Closes:* SEC-031 (BLOCKER) + the monitoring/backup SHOULDs (SEC-037, SEC-047–053, SEC-055–056, SEC-084). **Decisions:** D1–D5, D8. The heaviest build of the iteration and the core of the launch gate.

**SR-7 — Deploy Runbook & Provisioning Robustness [P0 — launch gate]. → executed in Phase 149.** Make a first deploy possible and safe: the production deployment runbook (correct prod up-command incl. the dashboard profile + env preconditions + post-up smoke; upgrade; migration ordering; **rollback = restore-from-pre-upgrade-backup**), a working **first-admin bootstrap path on a prod box**, the email/SMTP + invite-link + WebAuthn-RP coherence so an invited user can actually log in, a safety guard on the destructive reset, the prod crawl scheduler, and provisioning hardening (idempotent + fail-loud init; dirty-migration recovery). Fold in the scheduled security-advisory scan + notification. *Closes:* SEC-032, SEC-033, SEC-034, SEC-035, SEC-036 (BLOCKERs) + the deploy/provisioning SHOULDs (SEC-039–046, SEC-057–063, SEC-098). **Decisions:** D6, D7, D9. **SR-7 rollback + migration ordering depend on SR-6 backups.**

**SR-8 — Per-Service Robustness Fixes [P1]. [x] CODE-COMPLETE 2026-06-21** (all 34 SEC-065..098 items closed across ingestion, BFF, worker, crawler, dashboard, pkg + doc; the cheap `later`-tier was pulled forward into the same pass per the remediation-ambition directive). Close the application-logic defects from the per-service review — the two *silent-data-loss* strands first (a hard-crash that strands a document mid-process; a transient outage that quarantines good input), plus the recovery tool they depend on, and the bounded correctness/error-handling defects across ingestion, BFF, worker, crawler, and dashboard. *Closes:* the SEC-065..098 `should`-tier defects (SEC-074 + SEC-079 prioritised). **Coordinate the worker cluster with SR-6 alerting** — the alert is what makes the loss visible. CLEAN/DRY refinements (the `later` tier) were swept in here, not deferred.
>
> **⚠ THREE compose-coordinated carry-overs — execute in Phase 149 (they touch the §3 hot files `compose.yaml` / `.env.example`, edited together with the prod overlay work; the application-code halves are already merged):**
> 1. **Worker `stop_grace_period` (SEC-083).** Add `stop_grace_period: 70s` (must exceed `WORKER_SHUTDOWN_TIMEOUT_SECONDS`, default 65s) to the `analysis-worker` service in `compose.yaml`, so Docker's SIGTERM→SIGKILL window outlasts the worker's bounded drain. The Python side (`asyncio.wait_for` graceful shutdown) is already done.
> 2. **Document 3 worker env vars in `.env.example` (SEC-083 cluster).** Add `WORKER_SHUTDOWN_TIMEOUT_SECONDS` (65), `WORKER_STALE_PROCESSING_THRESHOLD_SECONDS` (900), `WORKER_STALE_PROCESSING_REAPER_INTERVAL_SECONDS` (120). All have safe `os.getenv` defaults — documentation parity, not a functional gap.
> 3. **Wire `CORS_ALLOWED_ORIGINS` (SEC-010).** Add `- CORS_ALLOWED_ORIGINS=${CORS_ALLOWED_ORIGINS}` to the bff-api `environment:` block + set the prod origin in `.env.example`. The BFF boot-guard half is merged (prod refuses the wildcard `*`), so this wiring lets prod boot with an explicit origin.
>
> *(Note: the disk-fill compose work — per-service log rotation, the ClickHouse system-log config mount — already landed with SEC-055/099/100; only these three tails remain.)*

**Standing backlog (deferred-by-design, with rationale — not a launch gate).** From 145: SEC-007 (WebAuthn step-up binds to nothing server-side — revisit before any phase wires a privileged action to step-up); SEC-005 if not folded into SR-2; SEC-028 + SEC-029 (confirmed-SAFE — recorded, no action). From 146: the `later`-tier footprint/hardening items (revisit at the Phase-148 topology ADR). From 147: the CLEAN/DRY + latent-only refinements (SEC-066, SEC-068–069, SEC-089–097) — picked up opportunistically when the adjacent code is touched, never a gate.

**Sequencing.** **(0) Resolve operator decisions D1–D9 first** — they gate SR-6/SR-7. **(1)** The config-quick BLOCKER fixes land early/cheap: **SR-1** (ingress overlay, one change, unblocks SR-3) + the quick SR-7 items (SMTP wiring, the destructive-reset guard, prod-auth/WebAuthn-RP coherence). **(2) SR-6** is the heaviest build and the core launch gate (off-box backups + a *tested* restore + alert routing + host signals). **(3) SR-7** completes the deploy story; its rollback + migration ordering **depend on SR-6**. **SR-4** is mandatory for the email-onboarding DoD. **SR-2 / SR-3 / SR-5 / SR-8** proceed in parallel where convenient; coordinate SR-8's worker data-loss cluster with SR-6 alerting.

*Definition of Done for the iteration (the launch gate):* every BLOCKER closed and verified (live-repro where applicable); every SHOULD closed (acceptance-with-rationale is the rare exception, not the path); the cheap LATER hardening swept in; **off-box backups exist and a restore has been tested in a clean environment** (SR-6); the **deployment runbook is followed end-to-end on a staging target** and the **first admin can be bootstrapped on a prod box** (SR-7); the **email invite flow works end-to-end** (Phase 153 **+ SR-4 + the SR-7 SMTP wiring**, since SEC-030/SEC-032 currently make it inert); **alerts reach the operator** (SR-6); the **Go/No-Go launch checklist** signed; first invited researcher can be onboarded.

---

# Iteration 13 — Deployment & Infrastructure (Infra Epic)

*The actual "where and how is it deployed", gated by Iteration 12's Go/No-Go. **Binding constraints (2026-06-14):** a single solo operator, minimal-to-no budget, the app must nonetheless run stably, AND it is a data-retention-heavy system — the medallion layers (Bronze→Silver→Gold) grow fast, and the analysis-worker is compute-heavy (baked-in BERT/BERTopic weights, ~3–5 GB). Cheap + stable + data-growing are in direct tension; this epic's job is to resolve that tension explicitly, not hand-wave it. **EU data residency** is required (DSGVO + the anti-surveillance posture — Phase 55, ADR-040). Design-first: topology, cost and capacity are decided (an ADR) BEFORE anything is provisioned; the concrete provisioning/deploy phases (149+) are partly derived from that decision, like Iteration 12's Stage B.*

---

## Phase 148: Deployment Topology, Cost & Capacity Decision (ADR) [P0] - [x] DONE

*The load-bearing decision the rest of the epic derives from. No provisioning until this ADR is signed.*

> **✅ DONE 2026-06-21 — decisions locked + ADR-044 signed.** Operator decisions D1–D9 + hosting (Hetzner CX43, x86, 16 GB/160 GB, ~€20/mo incl. backup) + the capacity/cost model are decided and recorded in **ADR-044** (`docs/arc42/09_architecture_decisions.md`); the full per-finding rationale + measured capacity audit live in the private `.security/PHASE148_DECISIONS.md`. The stack deploys **as built** (full observability). **Two provisioning carry-overs to Phase 149** (decided here, executed there): (1) register the concrete domain (strategy/`kontakt@<domain>` shape fixed; literal string deferred); (2) the implementation of SR-1/SR-6/SR-7 + the 3 compose-coordinated tails (below) lands WITH the prod compose/env work in 149, since most needs the provisioned box to verify.

> **Carries the deployment-coupled Stage-B remediation (~40 findings).** The Iteration-12 Stage-B phases **SR-1 (Ingress & Deployment-Posture), SR-6 (Backup/Restore & Monitoring), SR-7 (Deploy Runbook & Provisioning)** — and the **operator decisions D1–D9** (register §"Phase 146" §3) — **cannot be finalised until this ADR fixes the topology**, because Traefik FE/BE routing, the backup destination, host RAM/footprint, the crawl scheduler, ACME/domain (`*_HOST`), and the first-admin-on-prod path all change with the hosting shape (single-box vs. split FE/BE; provider; budget). **This ADR must therefore resolve D1–D9 and then SR-1/SR-6/SR-7 are executed around Phase 149 (first deploy).** The topology-independent code phases (SR-2/3/4/5/8, ~56 findings) are done independently in Iteration 12 and do **not** wait for this. Full itemised list + the anti-regression protocol: the private `.security/REMEDIATION_PLAN.md`.
>
> **Also carries 3 small compose-coordinated tails** (their app-code halves are already merged in Iteration 12; only the §3 hot-file edits remain, to land WITH this phase's `compose.yaml`/`.env.example` changes — NOT in isolation): **(a)** add `stop_grace_period: 70s` to the `analysis-worker` service in `compose.yaml` (> `WORKER_SHUTDOWN_TIMEOUT_SECONDS`=65) — SEC-083; **(b)** document `WORKER_SHUTDOWN_TIMEOUT_SECONDS`/`WORKER_STALE_PROCESSING_THRESHOLD_SECONDS`/`WORKER_STALE_PROCESSING_REAPER_INTERVAL_SECONDS` in `.env.example` (defaults 65/900/120) — SEC-043 cluster; **(c)** wire `- CORS_ALLOWED_ORIGINS=${CORS_ALLOWED_ORIGINS}` into the `bff-api` `environment:` block in `compose.yaml` and set the production app origin(s) in `.env.example` — SEC-010. The BFF boot-guard half is already merged: with `APP_ENV=production` the BFF now **refuses to start** on the wildcard `*`, so this wiring is what lets prod boot with an explicit origin (and is part of the same F3 secret/var-forwarding sweep as SEC-032/036/039/043/046 — consider the D9 `env_file` option to fix the whole class structurally).

**Grounding.** Read first: `compose.yaml` (the full component set + resource hints), `compose.prod.yaml`, the Data-Lifecycle (ILM) table in CLAUDE.md (Bronze 90 d / Silver 365 d / Gold 365 d + MVs), the analysis-worker model bake (Dockerfile HF cache mount), Hard Rule 7, the observability stack, ADR-040 + Phase 55 (privacy/EU). Preserve: the medallion architecture, IaC-only provisioning (Hard Rule 5), zero-trust network posture, the SSoT tag model. Verify-first: measure the *current* on-disk footprint per layer + per-probe daily growth from the running POC before modelling — extrapolate from real numbers, not guesses.

### Topology + cost
* [x] **Hosting target decided** — single EU VPS, **Hetzner CX43** (x86, 8 vCPU, 16 GB, 160 GB SSD, ~€16.49/mo) + Storage Box ~€4/mo ≈ €20/mo all-in. x86 chosen so amd64-built custom images run as-is. (ADR-044 §1.)
* [~] **Domain decided (one domain, three uses)** — strategy fixed: one EU-registrar domain for app URL + TLS subject + mail sender, `kontakt@<domain>` from-shape (free forwarding, no paid mailbox). **Literal domain string deferred → registered at Phase-149 provisioning.** (ADR-044 §9.)
* [x] **Component placement** — full stack co-located on the single box; heavy three (worker compute, ClickHouse, MinIO) fit the 16 GB ceiling. (ADR-044 §1–2.)
* [x] **Observability footprint decision** — **FULL / as-built** (Prom+Grafana+Tempo); 16 GB carries the ~12.5 GB limit ceiling. (ADR-044 §2.)

### Capacity + growth (the data-retention reality)
* [x] **Growth model** — MEASURED from the POC: steady-state ≈ Bronze(90 d)+Silver(365 d) ≈ **0.3 GB per sustained doc/day** (Gold/Postgres negligible); ~3× reduction available via the Silver raw_text lever. Single box carries launch→early; mid-scale (multi-TB) is a deliberate later tier. (ADR-044 §10; measured detail in `.security/PHASE148_DECISIONS.md`.) **Re-measure R after ~1 week of periodic crawling** (current data is backfill) — tracked in Phase 148b.
* [x] **Worker compute budget** — CPU inference is the **first** bottleneck before storage as probes scale (operator expects models to grow); single box fine for launch→early; GPU/worker-fleet is the mid-scale step. (ADR-044 §10.)
* [x] **Stability model** — single-point-of-failure accepted at this budget; resilience = **tested backup/restore**, not redundancy (ADR-044 §1,3).

### Output
* [x] **ADR — Deployment Architecture** → **ADR-044** (topology, host, backup/observability/scheduler/config/first-admin/ACME/domain strategy, capacity model + scaling limit) in `docs/arc42/09_architecture_decisions.md`. Phases 148b/149/150 implement.

### Validation
* [x] ADR-044 signed; cost within the ceiling (~€20/mo); capacity model grounded in measured per-layer growth and states the single-box scaling limit. (Literal domain string + the on-box verification steps execute in 149.)

---

## Phase 148b: Pre-Deploy Readiness & Data-Quality Validation [P0] - [x] DONE

*Before wrapping the stack in deployment infrastructure and going live: prove the POC actually runs cleanly end-to-end and produces good data. Distinct from security remediation (Stage B) and from deployment (149) — this is the operator's "am I really ready?" gate. Most of it runs LOCALLY (no provisioned box needed) and can therefore run in parallel with / before Phase 149.*

**Grounding.** Read first: `make build-services` + the full `make up` path, the crawler discovery/dedup/retry machinery, the analysis-worker processor + extractor pipeline + the self-healing loops (atomic claim, stale-processing reaper, poison/transient-infra quarantine, conditional-GET/staleness refetch), the Negative-Space + data-quality surfaces. Preserve: the medallion contracts, the provisional-metric disclaimers. Verify-first: rebuild from clean, then observe real output before judging.

* [x] **Full clean rebuild + run + smoke of EVERY service** — all 5 images rebuilt from scratch (worker 30 GB; dashboard needed one retry after a transient Buildkit snapshot error — no prune, per Hard Rule 7); `make up` → entire stack healthy; **all test suites green** (`test-go` + `test-go-pkg` + `test-python` + `fe-test`). *Remaining: manual frontend click-through of all surfaces (operator pass, may run post-deploy).*
* [x] **Crawler + NLP pipeline deep-check** — verified on a fresh `make reset` + full `make crawl` (2026-06-21): **707 docs ingested → 707 processed, 0 quarantine / 0 poison / 0 stale / 0 DLQ / 0 stranded.** Atomic A27 claim observed live (uploaded→processing→processed, 4 concurrent = WORKER_COUNT); NATS queue drained monotonically after crawl; reaper + lag gauges live; **graceful degradation proven** (web.archive.org transiently down → Wayback circuit-breaker opened correctly → "continuing without revisions" → recovered + reattempt-loop backfilled revisions). Sentiment differentiates real signal (elysee +0.22 PR vs tagesschau −0.28 news); language-routing correct (DE-specific models only on DE docs); cooccurrence/baseline/resolution-MVs populate; storage validated (Silver 0.58 MB/doc, Bronze 0.61 MB/doc).
* [x] **franceinfo dataset-dominance investigation** — **ANSWERED: genuine throughput, not a crawl-config artefact.** Fresh single 7-day crawl: franceinfo 395 · tagesschau 282 · bundesregierung 19 · elysee 11. **FR 406 : DE 301 ≈ 1.35:1** (vs backfill 2.3:1 — the larger backfill gap was accumulation asymmetry, not rate). Per-channel telemetry confirms both deep channels work (tagesschau archive_index 254→213, franceinfo sitemap_news 491→491). Discovery→collection funnel asymmetry surfaced (franceinfo 526→395 = 75 %, ~131 media-filtered; tagesschau 284→282 = 99 %) → folded into the completeness/fairness work. The throughput difference is now a first-class **measured** signal, handled at the comparison layer (normalization/equivalence), not by distorting collection.
* [x] **Re-measure R (docs/day)** — first real estimate from the 7-day-window crawl: **R ≈ 101 docs/day** all-sources (franceinfo 56 · tagesschau 40 · breg 2 · elysee 1), validating ADR-044's "4-source probe ≈ 100–200/day". *Periodic daily crawling over real days would refine daily variation; the single-window number is already a solid bound.*
* [x] *(promoted)* **Silver `raw_text` storage lever** — promoted from an optional 148b note to a full **Phase 148c** item (pre-deploy); tracked + specced there. (Validation work above is complete; the only operator remainder is the manual FE click-through, deferrable post-deploy.)

**Findings (2026-06-21 full-crawl validation) — all routed to Phase 148c (pre-deploy fixes):**
* **[→ 148c] Topic modeling did not complete.** The de partition (19 docs) correctly yielded 0 topics; the **fr partition hung in the BERTopic fit, wrote 0 rows, and silently blocked all further topic ticks**, across two full runs. A live RAM-bump experiment (6→10 GiB) isolated the root cause: the 6 GiB cap WAS binding (worker crossed 6 GiB without OOM once lifted) but more RAM did NOT fix it — the real root is **uncoordinated concurrent corpus loops** (revision-enrichment backfill + cooccurrence 1-yr sweep + topic fit all firing on independent timers → topic starves). NOT corpus-size, NOT the tuned UMAP/HDBSCAN params (a prior trial shattered the coherent FR topics — do not retune).
* **[→ 148c] Worker RAM is the binding constraint.** At 707 docs the worker sat at 88–91 % of its 6 GiB limit running the full extractor suite + the cooccurrence **1-year window** (loads the whole window into RAM → grows with the corpus → real OOM path) + revision enrichment. Co-root of the topic hang.
* **[→ 148c, decided by Task #7] word_count floor not enforced on extracted body.** The crawler's `min_word_count` filters on raw-HTML word count (a 404/stub guard), so thin pages (4–40 body words, e.g. tagesschau liveblog/photo stubs) pass and land in Gold. No article-body length check exists at the Silver boundary. Policy decision (drop vs disclose-as-Negative-Space vs where to enforce) belongs to the Task-#7 completeness/fairness over-crawl audit; implemented in 148c.

### Validation
* [x] A clean rebuild runs green end-to-end; the self-healing loops are observed working; the franceinfo-dominance question is answered (genuine throughput, ≈1.35:1 fair); the capacity model is refreshed with a real daily rate (R ≈ 101/day). *Findings routed to Phase 148c (pre-deploy); manual FE click-through remains an operator pass.*

---

## Phase 148c: Pre-Deploy Pipeline & Storage Fixes [P0] - [x] DONE

*The fixes surfaced by the 148b full-crawl validation, done **before first deploy** — you don't go live with a known-broken extractor, a too-tight memory cap, or a 3×-bloated storage layout that then accumulates for 365 days. All of this is **code/config, no provisioned box needed** (final RAM tuning is confirmed on the real 16 GB box in 149, but the fixes are built here). Sequenced after 148b, before 149.*

**Grounding.** Read first: `internal/extractors/topic_modeling.py` + the corpus loops in `main.py` (`corpus_extraction_loop` / `topic_extraction_loop` / `baseline_extraction_loop` / `revision_diff_extraction_loop` / `enrichment_reattempt_loop`) + their independent timers; `internal/adapters/web.py` (`raw_text`/`cleaned_text`/`word_count` at the Silver boundary); the cooccurrence sweep window; `infra/minio/setup.sh` (BFF is silver-only today); the L5 Evidence Reader Raw toggle + `bronze_object_key` (migration 000013). Preserve: the tuned UMAP/HDBSCAN params (do NOT retune — a prior trial shattered coherent FR topics), the medallion contracts, the provisional-metric disclaimers, graceful degradation. Verify-first: reproduce the topic hang, then confirm the fix completes a full topic tick on the 707-doc corpus.

* [x] **Corpus-loop coordination + topic-modeling fix — DONE (2026-06-22, live-verified).** Root cause (isolated via a 6→10 GiB experiment): uncoordinated concurrent corpus loops (revision-enrichment + cooccurrence 1-yr sweep + topic fit on independent timers → topic starved) AND the embedding ran single-threaded (`OMP/MKL=1`) so a 300-doc fr partition took ~19 min. **Fix shipped:** (1) a shared `asyncio.Lock` (`corpus_extraction_lock`, `main.py`) serialises the heavy sweeps — cooccurrence/baseline/topic/revision-diff acquire it around their `to_thread` (keyword-only `extraction_lock`, `nullcontext` fallback for tests); (2) `topic_modeling._fit_partition` bumps `torch.set_num_threads()` (env `TOPIC_EMBED_THREADS`, default 6) scoped to the embed + restores it (best-effort — degrades if torch absent); (3) a per-sweep `asyncio.wait_for` timeout (`TopicConfig.sweep_timeout_seconds`, default 1800) releases the lock on a runaway fit; (4) a `topic_modeling.partition_start` heartbeat makes a stuck partition visible. Did NOT retune UMAP/HDBSCAN params. **Verified:** fresh reset+crawl (606 docs) → full topic tick completes (de 289→9 topics, fr 313→3 topics, 10 total, 661 assignments), fr partition **6m01s vs ~19 min single-threaded (~3.2×)**, 0 timeouts, no hang. Worker suite 476 passed, cov 80.99 %. *Remaining optimisation (separate): the topic loop re-embeds the whole window every tick — an embedding cache (re-embed only new docs) is the next big speed lever; at the production weekly cadence the current ~11 min full tick is a non-issue.*
* [x] **Worker memory sizing — DONE (2026-06-22, measured).** With the mutex serialising the heavy sweeps, the worker peak is `max(single sweep)` not the sum. A fresh reset+crawl (606 docs) peaked at **8.17 GiB** (full-corpus topic E5 embed). Non-worker services use only **~1.4 GiB actual** on the box (limits ≠ usage), so `compose.yaml` worker `deploy.resources.limits.memory` raised **6144M → 11264M (11 GiB)** — comfortable on the 16 GB CX43 (~11 + 1.4 + OS ≈ 13.4 GiB), ~2.8 GiB headroom above the measured peak for corpus growth. **Verified:** worker ran the full crawl+drain+topic under the configured 11 GiB with no OOM. *The per-sweep peak still scales with the corpus (cooccurrence loads its 1-yr window into RAM); window-streaming + the topic embedding-cache are the follow-up to cap that growth — tracked with the topic-perf optimisation above.*
* [x] **Silver `raw_text` storage lever — DONE (2026-06-22, code-complete + unit-gated; e2e folds into the final 148c reset+crawl).** Stops persisting `raw_text` in the Silver MinIO envelope; the L5 "Raw" view sources it from Bronze on-demand. **~3× less lifetime storage growth** + restores the medallion principle (Bronze = raw, Silver = refined). **Shipped:** (1) **Worker** — `internal/silver.py` serialises with `model_dump_json(exclude={"core": {"raw_text"}})`; `raw_text` stays on the in-memory SilverCore (processor non-empty guard + harmonization unchanged), only the MinIO form drops it (~10× smaller Silver objects). 4 worker tests updated to the new contract (`raw_text not in` serialized core). (2) **BFF** — `SilverStore.GetBronzeRawHTML` reads the `bronze` bucket by the shared object key (+ pure `parseBronzeRawHTML` helper, unit-tested); `silver_handlers.go` + `dossier_handler.go` fetch it **best-effort** (ErrBronzeNotFound past the 90-day TTL → omit rawText, never fail); `SilverFetcher` interface + mock extended; handler test covers present + TTL-expired. (3) **MinIO** — no change needed: `aer_bff_policy` **already** grants bronze read. (4) **Evidence Reader** — `evidence_raw_note` 90-day-availability note (EN+DE) by the Raw toggle. **Design note:** chose **eager-in-detail** (fetch Bronze when the single-article L5 detail is opened) over the doc's lazy dedicated endpoint — `rawText` already exists in the contract, so this needs no new OpenAPI/codegen/frontend-query; cost is +~0.66 MB on a deliberate drill-down (not a hot path). **Trade-off:** raw view only within the 90 d Bronze window (vs 365 d) — disclosed by the note. Existing Silver keeps `raw_text` until its 365 d TTL; only new writes shrink. Gates green: worker 476 passed / cov 80.99 %; bff build+vet+lint clean / cov 80.6 %; dashboard svelte-check + lint + L5 tests green. Full rationale: `.security/PHASE148_DECISIONS.md` §"Storage Lever #1".
* [x] **word_count body-floor policy — DONE (2026-06-22; operator chose disclose-as-Negative-Space over drop).** Thin pages (4–40 body words — live-blog/photo/listing stubs) reach Gold with a tiny `word_count` because the crawler's `min_word_count` filters raw-HTML word count (a 404/stub guard), not the extracted body. Per DISCLOSE-NEVER-COERCE (WP-007 §4.3), thin content is **surfaced as Negative Space, never silently dropped**: a 7th NS class `thin_content` in the SoT `negative-space.ts` (taxonomy + `NSClassDef` + `THIN_CONTENT_WORD_FLOOR = 50`, a provisional engineering default; classifier reads `NSRow.wordCount`), wired free via `ArticleRow` (already passes `wordCount` to `classifyNegativeSpace` → the `∅` badge shows on thin rows), `evidence_raw_note`-style label/desc in `domain.json` EN+DE, WP-007 §4.3 anchor. `make fe-lint` + `make fe-test` ($lib floor 90) green; `negative-space.test.ts` pins the 7-class vocabulary + the thin-content classifier. *Note: this is the disclosure layer; the orthogonal **over-crawl performance** concern (live-tickers re-revisioned indefinitely) is its own item below.*
* [x] **Live-ticker over-crawl bound + `live_ticker` Negative Space — DONE (2026-06-22; operator-requested finding).** Measured: 82/83 revisions in the validation corpus are `cdx_snapshot` (Wayback CDX backfill), not live edits. A **live-ticker** (weather / live-blog — a stable URL with ever-changing content) accumulates dozens-to-hundreds of distinct CDX content-versions; backfilling + diffing/enriching each costs CPU for zero analytical value. **Reliable, source-agnostic signal = CDX-snapshot count per article** (not per-source URL patterns, which the operator correctly flagged as unreliable cross-source). Two-part fix: (1) **Worker** — `article_revisions._build_chain` caps the chain at `MAX_CDX_REVISIONS_PER_ARTICLE = 20` (provisional; validation real articles topped at 8), keeping the chronologically-first N + logging `article_revisions.chain_capped` → bounds the diff/enrichment cost. (2) **Frontend** — an 8th NS class `live_ticker` (`negative-space.ts`, `LIVE_TICKER_REVISION_FLOOR = 20`, MUST match the worker cap; classifier reads `NSRow.chainLength`), wired free via `ArticleRow` (already passes `chainLength`), label/desc in `domain.json` EN+DE, WP-007 §4.3 anchor → a capped ticker surfaces as Negative Space + is excluded from the analytical reading, disclosed not silently dropped. Gates: worker 479 passed / cov 81.06 %; `make fe-test` ($lib floor 90) green; `negative-space.test.ts` pins the 8-class vocabulary + the live-ticker classifier. *Follow-up (deferred): deeper exclusion from every Gold aggregate query + threshold tuning once real ticker data exists at scale.*

### Validation
* [x] **e2e PASSED (2026-06-22):** fresh `make reset` + `make crawl` (644 docs, 0 quarantine / 0 dlq). Item 1 — warm full-corpus topic tick completed (de 295 + fr 343 → **9 topics**, no hang, fr ~5.9 min). Item 2 — worker RAM peak **9.12 GiB < 11 GiB** cap (6 GiB would have OOM'd on this 644-doc corpus → the raise is justified). Item 3 — a new Silver object had **`raw_text` absent** (19 KB) while Bronze carried `raw_html` (362 KB) for the on-demand Raw view. Item 4 — **21 articles with `word_count` < 50** → thin_content disclosure fires. Item 5 — max revision chain **6 < cap 20** (cap active, no false positives; no real ticker in the fresh corpus). `make lint` + touched-service tests + coverage floors green earlier in the session.

---

## Phase 148d: Collection Completeness & Fairness Instrumentation (WP-007) [P0] - [x] DONE

*Status: **DONE** (2026-06-22). All five items engineering-complete + live-validated against the running stack; gates green (crawler 80.10%, BFF 80.2%, fe-lint i18n parity, fe-test 41/41). Migrations 000029 (declared denominator) + 000030 (per-source funnel) applied (version 30, clean). Live endpoint proven: tagesschau completeness 0.85 (284 Gold / 334 declared, html_sitemap the named indeterminate remainder), bundesregierung 0.51 (22/43). Audit CLI emits a measured `completeness_baseline` (tagesschau html_sitemap 121 → floor 60). NOTE: the funnel's fresh fetched/submitted/non-article rates populate fully only after a state reset (the last crawl was an incremental re-crawl, so every URL read as `already_collected` — itself a validated path: a re-crawl reads as "fully held", never "0 % complete"). The fresh-crawl funnel numbers are exercised by the Phase-149 deploy reset. Next: **Phase 148e** (operator frontend test & fix pass) → 149 (deploy) → 150 (live guardrails).*

*Implements [WP-007](docs/methodology/en/WP-007-en-collection_completeness_and_cross-source_fairness.md): turn crawl completeness + cross-source fairness from a manual judgement into a **measured, disclosed, drift-alerted property** — so a measured throughput difference between sources (e.g. franceinfo ≈ 1.35× tagesschau) is provably the world, not the crawler. **Pre-deploy** because it is data-quality foundation AND it shapes how every future source is added: the onboarding contract must exist before the probe catalogue scales (it does not block the 0+1 launch — both are already audited — but it gates adding source #3+, so it lands before scaling).*

**Grounding.** Read first: WP-007 (the four-layer model + the funnel); ADR-031 + `crawler_discovery_runs`/`crawler_discovery_alerts` + `internal/state/discovery_runs.py` (the existing per-channel telemetry — today `urls_discovered` is counted *after* the window filter, with a hand-set `expected_floor_per_run`, NO measured denominator); `main.py` `_add_channel` + `discover_sitemap`/`discover_rss` (where the declared count must be captured before filtering); `aer-audit-source`; the BFF `GET /sources/{id}/discovery-coverage` + `DiscoveryCoveragePanel.svelte`; `negative-space.ts` + `NegativeSpaceBadge.svelte` (the 6 existing classes). Preserve: technical-only filtering (WP-006 §3 — measurement, never editorial selection), polite-by-default (declared denominator measured only from already-exposed channels, never adversarial probing), DISCLOSE-NEVER-COERCE. Verify-first: confirm what `urls_discovered` actually counts before adding the denominator.

* [x] **Layer 1 — measured declared denominator + completeness ratio.** DONE — `ChannelStats` sink threaded through all four `discover_*`; `declared` + `declared_indeterminate` persisted on `crawler_discovery_runs` (migration 000029) + `DiscoveryRunRecord`. `indeterminate` fires on fetch/parse error, walk/fetch cap, and undatable content (strict-undated sitemap, undated RSS, the structurally-dateless html_sitemap). `completeness = goldRows / declared` computed in the BFF; indeterminate → never 100 %.
* [x] **Layer 1 — the funnel, attributable per stage.** DONE — per-channel `declared → discovered → after_dedup` (crawler); per-source `url_filtered / already_collected / fetched / not_modified / content_dropped / thin_content_dropped / submitted / errored` via the spider `closed()` handler → `crawler_funnel_runs` (migration 000030); `extracted → Gold` reconciled at BFF read-time from ClickHouse (no run_id propagated — Decision A hybrid granularity). `already_collected` distinguishes "fully held on re-crawl" from a real gap.
* [x] **Layer 3 — over-collection signal.** DONE — per-source `extractionSuccessRate` (Gold/submitted) + `nonArticleRate` (thin-content/fetched) on the BFF funnel, fed by the `thin_content_dropped` stage (aggregates the 148c `word_count` floor). Disclosed via the funnel + the `thin_content` / `collection-completeness` NS classes, never silently counted.
* [x] **Disclosure (extend, don't rebuild).** DONE — `DiscoveryCoveragePanel` gains the completeness headline + funnel + per-channel declared beside observed-vs-floor; added the **9th Negative-Space class `collection_completeness`** (scope `source`) to `negative-space.ts` + `NegativeSpaceBadge` + i18n EN+DE + the class-list test (148c had already added thin_content + live_ticker → 8, this makes 9). BFF: `completeness` / `completenessIndeterminate` / `declaredTotalLastRun` / `indeterminateChannelCount` / `funnel` on the discovery-coverage response (Postgres declared+funnel, ClickHouse Gold; pure `DeriveCompleteness`/`FillFunnelRates` unit-tested).
* [x] **Onboarding contract (the extensibility payoff).** DONE — `aer-audit-source` emits a `completeness_baseline` (per-channel observed inventory + a measured `suggested_floor_per_run`) and writes that measured value into the suggested `expected_floor_per_run` (no longer a bare `<edit-me>`). `add-a-source.md` step 6 + `add-a-probe.md` Step-G now point at the real telemetry (the audit baseline, the discovery-coverage `completeness`/`funnel` fields, the over-collection rates). **Fairness audit:** Step G's symmetric-rules check + the per-source asymmetry documentation (archive-walker vs news-sitemap vs RSS-only as a structural-bias parameter) carry the fairness contract.

### Validation
* [x] DONE — live endpoint returns a completeness ratio against a *measured* denominator (tagesschau 0.85, bundesregierung 0.51) or a disclosed indeterminate; the funnel explains every per-stage drop; the dashboard `collection_completeness` badge ships; the audit CLI produces a recorded measured completeness baseline (tagesschau html_sitemap 121 → floor 60). `make lint` + touched-service tests + coverage floors green (crawler 80.10 %, BFF 80.2 %, $lib via fe-test 41/41); i18n parity EN+DE. Residual: the funnel's *fresh* fetched/submitted/rate numbers populate after a state reset (the deploy-149 reset exercises them; the re-crawl path that reads as `already_collected` is itself validated).

---


## Phase 149: Provisioning & First Deploy [P0] - [x] DONE — deploy live + Go/No-Go GO (2026-06-27)

> **Closed for the live deploy** (stack/TLS/auth/admin/crawl/tested-restore/timers/alerts all verified on the real box — see STATUS below). Residual items NOT part of the live-deploy push (operator rule: deliberate residuals → next phase): ~~transactional-email live-validation~~ **✅ DONE 2026-06-27** (full invite→email→accept→activate run, clean Gmail delivery); **2 remain → Phase 150**: **GHCR service-image publishing + nightly-smoke un-park** (images are built locally on the box, not pulled from GHCR); **frontend manual passes** (Phase-128 screen-reader EN/DE + M1 fps against the deployed HEAD bundle). The **written `deploy_runbook.md`** (the runbook executed here interactively + the 9 fresh-machine fixes) → **Phase 151**.

*Execute the Phase-148 ADR: provision the target, deploy the stack, onboard the first admin. **This is where the deployment-coupled Stage-B remediation lands and Go/No-Go closes.***

> **STATUS 2026-06-27 — PHASE COMPLETE (GO). AĒR is live in production at https://aer-project.eu.** **Spur 1 (BUILD) + Spur 2 (PROVISIONING) = COMPLETE** (as below). **Spur 3 (DEPLOY + VERIFY) = COMPLETE on the real Hetzner box:** prod overlay `up` with **real browser-trusted Let's-Encrypt TLS** (staging→prod CA flip); all services + init containers healthy; first admin bootstrapped + logged in (full auth chain live: argon2id, `__Host-` session, CSRF); first crawl drained clean (6603 Gold metrics, 0 quarantine); **tested restore from backup PROVEN** — full destructive DR drill (dropped `aer_gold` → restored from the encrypted off-box restic snapshot → identical 6603 rows, auth intact); systemd timers installed (crawl 6h, backup daily) + the backup service validated under systemd; **alert delivery proven** (manual Brevo test mail + two real auto-fired+resolved alerts); GitHub CI secrets confirmed. **Deploy-coupled bugs found + fixed live during Spur 3** (all on the fresh-machine path, none affecting the running app — folded into the Phase 151 runbook): dashboard Dockerfile pnpm-workspace copy, `make up` openapi-bundle prereq, GHCR-public wikidata image, SentiWS overridable `SENTIWS_URL` (Leipzig mirror was down → Internet-Archive fallback), `postgres:18` `PGDATA` pin, minio-init `grep→case` + `mc event add` overlapping idempotency, restic `/home`-relative repo path + restore CH `DROP` before `RESTORE` + `forget --group-by host` retention fix. **Phase 150 (live-on-box guardrails) may now start.** ~~Spur 3 NOT YET DONE~~ — done.

> **Implements + verifies SR-1 / SR-6 / SR-7 here** (the deployment-coupled security remediation; full itemised list in `.security/REMEDIATION_PLAN.md`): **SR-1** prod-overlay/ingress hardening (BLOCKERs SEC-001/002 + console forward-auth, unpublish :8080, real domain/WebAuthn-RP, docs-internal gating, Traefik trustedIPs); **SR-6** backups + tested restore + monitoring/alert-email + host signals (BLOCKER SEC-031 + cluster); **SR-7** deploy runbook + first-admin baking + ofelia scheduler + env_file + provisioning hardening (BLOCKERs SEC-032..036). Plus the **3 compose-coordinated tails** decided in 148: (a) worker `stop_grace_period: 70s`; (b) document the 3 worker shutdown/reaper env vars in `.env.example`; (c) wire `CORS_ALLOWED_ORIGINS` into the bff env block. **Stage B — Remediation + the Go/No-Go checklist are signed off at the end of this phase** (every BLOCKER closed + live-verified; backups restore in a clean env; alerts reach the operator; runbook run end-to-end; first-admin works on the prod box).

**Grounding.** Read first: the Phase-148 ADR, `compose.prod.yaml` (hardened in Phase 146), the infra init containers, the Phase-146 deployment runbook + secret inventory, `make create-admin`, the public/internal docs split (Phase 134). Preserve: IaC-only provisioning, the single-SSoT compose-tag discipline (no drift reintroduced), zero-trust ports. Verify-first: the Phase-146 readiness findings are closed before provisioning.

* [x] **Provision the target** per the ADR (IaC where possible), EU region, TLS/domain via Traefik, secrets injected (not baked). — Hetzner box live, firewall 22/80/443, real Let's-Encrypt TLS, secrets in root-owned chmod-600 `.env` (the "fortress" upgrade → Phase 151).
* [x] **Wire transactional email + close Phase 153's live-validation box.** ✅ DONE 2026-06-27 — full live run on the box: admin invite → Brevo email (clean **Gmail inbox** delivery, *not* spam — strongest DKIM/SPF/DMARC signal) → accept → activate → suspend all verified; forgot/reset proven the night before; boot-time secret validation green (`auth: transactional email configured`). The engineering is done (Phase 153 / ADR-043 — provider-agnostic SMTP behind `notify.LinkSender`, default Brevo); this is the operator setup that needs the live domain: (1) register the Phase-148 domain; (2) create a free Brevo account (accept the DPA — DSGVO: it processes invitee emails); (3) authenticate the **sending domain** in Brevo (SPF + DKIM DNS records — *not* a freemail From, which fails DMARC); (4) create a Brevo SMTP key; (5) set `SMTP_HOST/PORT/USERNAME/PASSWORD/SMTP_FROM_ADDRESS` + `BFF_PUBLIC_BASE_URL` (deployed origin, else links are relative) in the prod secret store; (6) `bff` restart, then run the Phase-153 **Live end-to-end** checklist (invite → accept → activate; forgot/reset; boot-time secret validation). Until this item, the deployed stack runs on the `LogSender` fallback. See Operations Playbook §"Transactional Email".
* [x] **First deploy + bootstrap** — stack up, healthchecks green, `make create-admin` first admin, public docs vs. internal (admin-gated) docs split live. — Done via `docker compose exec bff-api /app/bootstrap-admin` (prod path); admin logged in; public docs :8000, internal docs loopback :8001.
* [x] **Publish service images to GHCR + un-park the nightly smoke.** Provisioning needs pre-built images: publish ingestion / bff / analysis-worker / dashboard to GHCR (the worker-image build must solve the 10+ GB model-bake disk/time problem *here, once* — same constraint Phase-136 hit) so `compose` can reference `image:` (pull) instead of `build:`. With that, flip `e2e_smoke_nightly.yml` to **pull** (fast), **rewrite `scripts/build/e2e_smoke_test.sh` from the retired RSS flow to the web-crawler path**, and **re-enable its cron** (Phase 136 parked it). The nightly's unique value (compose-wiring / init-container / healthcheck-graph) is restored cheaply on top of the deployment image pipeline.
* [x] **Deployment runbook executed end-to-end** (first deploy / upgrade / rollback) and corrected against reality; a tested restore from backup on the real target. — First deploy executed + corrected (9 fresh-machine fixes); **tested restore PROVEN** (destructive DR drill: dropped `aer_gold` → restored from off-box restic snapshot → 6603 rows back). The *written* `deploy_runbook.md` (capturing this) → Phase 151.
* [x] **Frontend pre-deploy verification residual** ✅ DONE 2026-06-27 — live pass on the deployed HEAD (bundle confirmed = HEAD, no rebuild): disclosure surface reads dim/methodological, globe fluid + reduced-motion honoured, keyboard-navigable (axe/WCAG already green at build). (carried over when the standalone frontend test phase was retired — the dashboard polish/fix work now runs continuously without a phase, but these eyes-on/manual passes must be signed off against the deployed HEAD bundle): (a) the Phase-148d disclosure surface verified on glass — `DiscoveryCoveragePanel` completeness headline (neutral %, or a clean "indeterminate" when no trustworthy denominator), funnel rows, per-channel `declared`/`≥declared (lower bound)`, and the 9th NS class `collection_completeness` reading as dim/methodological (never a red warning); (b) the **Phase-128 screen-reader pass (EN + DE)** over the surfaces touched since Phase 128; (c) the **Phase-128 M1 fps pass** (globe + at-scale co-occurrence within frame budget, reduced-motion honoured). Confirm the deployed bundle is HEAD (rebuild if the running stack still serves a stale build).

### Validation
* [x] The app is reachable over TLS on the EU target, gated by auth; an admin can log in; a backup restores; the runbook matches what was actually done. — https://aer-project.eu live, browser-trusted TLS, auth-gated, admin login confirmed, backup→restore verified (6603 round-trip).

## Phase 150: Capacity & Growth Guardrails [P1] - [x] DONE (2026-06-27)

> **Status:** implementation complete + validated (`make observability-validate`, fe gates, mkdocs `--strict`). **Operator tail** (needs the live box, not autonomously doable): run the on-box alert drill once + date it in the playbook; decide `postgres_exporter` (the one new credential). The deeper arc42 CD-as-building-block write-up is the only doc tail.

*The data-retention system must not silently fill its disk or fall behind. Operationalise the Phase-148 capacity model as live guardrails.*

> **Scope note (2026-06-21):** the worker-RAM sizing + corpus-loop coordination + topic-modeling fix discovered in 148b are **pre-deploy code/config** and live in **Phase 148c** — not here. This phase is purely the **live-on-the-box** layer: alerting that fires on real signals, TTL enforcement verified on real data, and the scale runbook. (It *verifies* the 148c fixes hold under real growth; it does not contain them.)

**Grounding.** Read first: the Phase-148 capacity model, the ILM TTL config (MinIO lifecycle + ClickHouse TTLs), the alerting decided in 148, the worker throughput signals, the DLQ. Preserve: the ILM TTLs as the steady-state bound; the re-attempt framework. Verify-first: confirm the TTLs are actually *enforcing* (objects/rows really expire) before relying on them as the capacity bound.

> **Carried from Phase 149** (deliberate residuals — not part of the live-deploy push). *(Transactional-email live-validation was the third; ✅ DONE 2026-06-27 — closed in Phase 149.)* **(a) GHCR service-image publishing + CD pipeline — ✅ DONE 2026-06-27.** **M1** `release-images.yml` publishes the 4 service images to GHCR on a `v*` tag (matrix; worker registry-cache for the model layer; OpenAPI bundle generated in-CI); **M2** `compose.prod.yaml` pulls `image:` + `!reset`s `build:` (GHCR_OWNER/IMAGE_TAG in `.env`); **M3a** box read-only `read:packages` token (`docker login ghcr.io`), packages PRIVATE; **M3b** secrets-at-rest **B** (hardened root-600 `.env`; full no-`.env` → Phase 155); **M4** tag-driven auto-deploy (GH Actions SSH → `git checkout tags/<tag>` + `deploy_pull.sh`: pin `IMAGE_TAG`, `compose pull`, `up --wait` health-gated) — **proven live: `v0.1.1` deployed green, all services healthy on GHCR images**; **M5** nightly smoke un-parked (pulls GHCR images via `compose.ci-images.yaml`; cron + path-trigger re-enabled; Step-3 synthetic ingest kept) — **green: 19/0 assertions**; **M6** rollback (runbook Part C — `deploy_pull.sh <older-tag>`); **M7** `docs/operations/deploy_runbook.md` (provisioning→release→rollback + the nine fresh-machine findings) + arc42 §7 pull-model note + mkdocs nav; **`deploy_pull.sh` auto-reloads prometheus/grafana** when `infra/observability/` changed (bind-mounted config; scoped via untracked `.aer-deployed-sha`). *Pipeline gotchas (all fixed): `type=semver` strips the `v` → also tag images `type=ref,event=tag`; a force-moved tag won't update the box's local tag → deploy `git fetch --force`; worker `cpus:"6.0"` (prod 8-vCPU) > CI 4-vCPU runner → cap in the CI overlay; an unquoted space in `GF_SMTP_FROM_NAME` broke the smoke's `source .env`. Tail: deeper arc42 CD-as-building-block write-up + the later-phase drift sweep.* ~~(b) frontend manual passes~~ **✅ DONE 2026-06-27** (live pass on deployed HEAD: disclosure dim, globe fluid + reduced-motion, keyboard-navigable). **(c) [quality] `AdminPanel.svelte` surfaces 403/errors — ✅ DONE 2026-06-27.** `suspend`/`reactivate`/`resetFor` now set an `actionError` (`reportActionFailure`) and render it via the existing `AuthNotice` (distinguishing 401/403 "session expired / lost permission" from generic failure); EN+DE keys added (parity held 2025=2025); svelte-check + eslint + vitest (898) green.

* [x] **Storage alerting — DONE 2026-06-27.** `MinIOCapacityLow` (usable <15%) + the Phase-149 host-disk alerts cover fill; **ILM TTL enforcement verified live** by `scripts/operations/verify_ilm.sh` (read-only — counts ClickHouse Gold rows + MinIO objects past their TTL+grace via `mc find --older-than`, writes `aer_ilm_*` to the node-exporter textfile) wired to `ILMViolation` + `ILMCheckStale` alerts and the `aer-verify-ilm` systemd timer. *Per-bucket/table absolute growth ceilings deferred until steady-state volume is measured (measure-first, no guessing pre-data) — mechanism + procedure in the playbook.*
* [x] **Dedicated datastore exporters — RESOLVED 2026-06-27 (Occam scope decision).** Phase-154 native metrics superseded two of three: **cadvisor** unneeded (worker RSS via `prometheus_client`'s process collector → `WorkerMemoryHigh`; every container has a hard `mem_limit`); **nats-exporter** unneeded (`nats_consumer_pending` gives the JetStream backlog → `WorkerBacklogGrowing`; `TargetDown` covers NATS-down). **postgres_exporter** (PG pool saturation) is the one genuine gap, **deferred pending operator sign-off** — it needs a new read-only `pg_monitor` credential (a security-posture call, not autonomous). Reasoning + re-open recipe in the playbook §Exporters.
* [x] **Worker throughput + memory guardrail — DONE 2026-06-27 (verifies 148c live).** `WorkerBacklogGrowing` (`nats_consumer_pending > 1000`/1h), `WorkerMemoryHigh` (worker RSS >90% of the 11 GiB cap), `WorkerPoisonMessages` (redelivery-failed docs); documented responses + levers in the playbook. (DLQOverflow/CrawlStale already shipped.)
* [x] **Cost + scale runbook — DONE 2026-06-27.** Playbook §"Capacity & Growth Guardrails": alert→response table, "data grew faster than expected" triage, and the "scale to the next probe tier" single-box-limit trigger (tune → vertical → horizontal, Occam order).

### Validation
* [x] **DONE 2026-06-27** (config-level + procedure). All 6 new `aer_capacity` rules pass `promtool` / `make observability-validate` (19 rules, Prometheus ↔ Grafana mirror in lockstep); the playbook documents a **safe drill** to fire each alert (write `aer_ilm_violations 1` to the textfile, or lower a threshold + reload — never fill a prod disk); TTL enforcement is confirmed live by `verify_ilm.sh`; the scale-trigger runbook exists. *Operator tail: run the on-box drill once + date it in the playbook.*

---

# Open Phases

*Rewritten 2026-05-21 after a full senior-architect review of the post-122k codebase. The previous Open-Phases plan was drafted between the 122h amendments and the 122k rebuild and had accumulated significant drift (four-surface vocabulary, `/compose` route, "Function Lane", "L5 Evidence pane", "methodology tray", card/edge composition canvas). This rewrite re-grounds every open phase in the actual code, splits several phases, adds foundational phases the old plan lacked (Pillar Identity, Configurable Cells, News-Backbone Evaluation, Metadata Analysis, Access Control), removes Phase 126, and defers the non-human-actor machinery. Phases are listed in **execution order** within each iteration; numeric phase ids are not monotonic with execution order (consistent with the rest of this file). Phase numbers are stable insertion-order ids, not a sequence — implement top-to-bottom through the Iteration-11 closure block, then Iteration 12 (production-readiness reviews), then Iteration 13 (the infra/deployment epic), then stop (the Deferred block is not sequential work).*

*Cross-cutting decisions that shape every phase below:*

- ***POC target: full-ambition Alpha.*** Quality of data- and insight-generation is the supreme maxime; maintenance is minimised but never at the cost of output quality.
- ***Per-source-class analytical backbone.*** Cross-probe comparison runs only on the symmetric multilingual Tier-2 backbone, one backbone per source class (news now; social-media later). Within-frame analysis may use all tiers a probe has (Tier-1 lexicon, Tier-2 multilingual, Tier-2.5 fine-tuned). Recorded in the ADR-023 amendment.
- ***Pillar identity (ADR-035).*** Aleph = "the weather now" (synchronic totality), Episteme = "the climate record" (diachronic), Rhizome = "currents between contexts" (relational). **The pillar is determined by the presentation, not the metric.** Metrics flow through presentations; each metric declares its compatible presentations and thereby auto-lands in the correct pillars.
- ***No discovery bias.*** Search/filter/recommendation surfaces use only universal probe attributes (probe, source, language, country, discourse function) — never capability/metric richness, which would privilege data-rich Western probes (Brief §1.3, Manifesto §II).
- ***Always explained.*** Every presentation — including dynamically composed ones — carries a "what you see / how to read it" explanation (extension of ADR-017 reflexive architecture; composed views get composed/template explanations).
- ***Disclose, never coerce (interim guardrail until Phase 122d.2).*** The full Negative-Space surface is consolidated late (Phase 122d.2, repositioned behind Phase 125 — see its note). Until it lands, no phase may bake in "absent → 0" coercion: a cell aggregating over a structurally-absent field must reuse the existing refusal/methodology surface rather than emit a misleading zero. This avoids debt the consolidated NS phase would have to unwind (WP-003 §3.2 / WP-006 §6.2).
- ***No silent permanent gaps — enrichment completeness & periodic re-attempt (ADR-036, landed in 123c hardening).*** Every per-article enrichment that can fail or be incomplete MUST (a) record a queryable completeness status that distinguishes "we know" (incl. a real negative) from "we do NOT know", and (b) register a `ReAttemptTask` with the general re-attempt framework (the periodic in-worker loop, `corpus.py` pattern — runs at boot + every interval, idempotent). A transient failure (e.g. Wayback/IA unreachable) or a later-improvable extraction must self-heal on a later tick, never depend on a manual re-crawl. **Any NEW external, degradable, or later-improvable enrichment added by any phase below MUST register a task + a status, or it silently reintroduces the gap this guardrail closes.** Wayback is the first registered task; Phase 133's `custom_extractors` is the next (a re-extract-from-Bronze task).

---

## Implementation protocol (every phase)

*This project is brownfield with strong consistency requirements. A fresh session that implements a phase without grounding produces stale features and an inconsistent UI (observed on a first Phase-130 attempt). Therefore, before and after implementing ANY phase below:*

1. **Ground first.** Read the phase's **Grounding** block + `CLAUDE.md` + the ADRs it names. Then inspect the **current state** of the features the phase touches — *the code is the source of truth; this spec is intent, not ground truth.*
2. **Reconcile spec vs. reality.** If a named file/feature has moved, been renamed, or already does part of this, **STOP and surface it** before coding. Do not implement blindly against a stale description.
3. **Determine context and relationships yourself.** These specs are deliberately not exhaustive. Work out how the phase fits the surrounding architecture (pillars, cell registry, URL grammar, the four medallion layers) so the result is coherent, not a bolted-on parallel mechanism.
4. **Brownfield, not greenfield.** Preserve working features. Extend established patterns rather than inventing new ones beside them.
5. **Definition of Done (applies to every phase, on top of its specific Validation):**
   - phase-specific **Validation** checks pass;
   - run the **`code-review`** skill on the diff;
   - run the **`verify`** skill where there is observable behaviour (UI phases; for worker/backend-only phases verify the data flow instead);
   - `make lint` · `make test` · `make audit` green (`lint`/`audit` are also git-hook-enforced; `test` is authoritative in CI — run locally at phase end regardless);
   - **hand back to the operator to commit — never auto-commit.**

---

## Phase 151: Post-Deploy Hardening Tail [P2] - [x] DONE 2026-06-27

*The committed-but-deliberately-deferred remainder of the Stage-B security remediation (`.security/REMEDIATION_PLAN.md`). None of these block the first deploy — they were sequenced AFTER it on purpose (lower severity, or a refactor better done against a known-good running box than as a pre-deploy risk). This phase exists so the deferral is not silent; each item is committed work with a clear scope, not an open question.*

> **Context.** Phase 149 closed every deploy-coupled finding (all 8 BLOCKERs + the SHOULD/LATER items that were cheap or deploy-relevant). The three items below were the explicit "do it after the box is live" set (operator decision 2026-06-26). The SEC-052 **dedicated datastore exporters** (postgres_exporter / nats-exporter / cadvisor) are tracked separately in **Phase 150** (live-guardrail alerting), not here.

**Grounding.** Read first: `.security/REMEDIATION_PLAN.md` (the per-finding detail + file lists), `services/bff-api/cmd/server/main.go` (the global rate limiter), the per-service compose `environment:` blocks, `services/analysis-worker/main.py` (the operator tunables). Preserve: the boot-time secret validation, the session-OR-key auth gate, IaC-only provisioning, the single-SSoT compose discipline. Verify-first: confirm each finding is still open against HEAD before implementing (several adjacent ones were closed in 149).

* [x] **✅ DONE 2026-06-27 — SEC-014 per-client rate limiting (`SHOULD·medium`, DoS).** Replaced the single global `rate.NewLimiter` with a **per-client token-bucket** keyed by the SEC-003 client IP (`auth.ClientIPFromContext`, right-most XFF; falls back to the port-stripped TCP peer so an absent header can neither collapse all callers into one key nor bypass the limit). A background sweep evicts idle per-IP buckets (`evictIdle`, 10 min idle TTL / 5 min cadence) so unique source IPs cannot grow the map without bound. The existing `RATE_LIMIT_RPS`/`RATE_LIMIT_BURST` become the **general** per-IP budget; new tighter `RATE_LIMIT_AUTH_RPS`/`RATE_LIMIT_AUTH_BURST` (default 1 rps / burst 5) govern the pre-auth `/auth/*` brute-force surface. No new dependency (reuses `golang.org/x/time/rate`) — a keyed map with eviction chosen over `httprate` per Occam. All 4 knobs forwarded in compose (`:-default`) + documented in `.env.example`; defaults are production-sane so **no `.env` change is required**. Tests: 7 green (4 reworked + per-client isolation + strict-budget + eviction); gofmt/vet/golangci-lint clean. *Closes the last SR-3 remainder.*
* [x] **✅ DONE 2026-06-27 — SEC-043 expose the worker operator tunables (`SHOULD·medium`).** Verified scope: **4** genuinely-missing safe tunables added to `.env.example` (default + effect documented) + compose passthrough — `WORKER_SILVER_BUCKET`, `TOPIC_EXTRACTION_SWEEP_TIMEOUT_SECONDS`, `TOPIC_EMBED_THREADS`, `MAX_CDX_REVISIONS_PER_ARTICLE`, all `${VAR:-default}` so absent-in-`.env` = the shipped default (no behaviour change). `METRICS_PORT` deliberately **excluded** (a fixed contract — Prometheus scrape + healthcheck + `EXPOSE` all bind `:8001`, so it is not safely tunable). The Wayback/CDX, re-attempt, and topic families were already forwarded. Original scope said "8"; HEAD verification found only these 4 missing. The worker reads several env vars that have no `.env.example` entry and no compose passthrough, so an operator cannot tune them without reading the source: the analytical-window cutoff, the re-attempt framework knobs, and the Wayback/CDX lookup controls. Document each in `.env.example` with its default + effect and forward them in the `analysis-worker` compose `environment:` block. Pure config-surface exposure — no behaviour change; defaults already ship.
* [x] **→ RELOCATED TO PHASE 155 (operator decision 2026-06-27): the `env_file` consolidation and the Phase-155 Docker-secrets migration touch the SAME per-service compose wiring, so doing them in one pass avoids reworking the secret subset twice. D9 — per-service generated `env_file` refactor (the durable F3 fix).** Replace the hand-maintained per-service `environment:` lists with generated per-service `env_file`s, so a new secret is forwarded by construction instead of by remembering to add a line to compose. This is the structural fix behind the cluster of "var X not forwarded" findings (SEC-032/036-forward/040/043/046 were each closed surgically in 149); doing the refactor now collapses the class. Deferred post-deploy deliberately: it touches every service's wiring and is far safer to land + verify against a running known-good box than as the last change before a first deploy. Must preserve boot-time secret validation and the no-secret-in-image rule.
* [x] **✅ RESOLVED 2026-06-27 (decision B, taken in Phase-150 M3b): keep the hardened root-600 `.env`; the real no-`.env` fix is its own Phase 155 — no throwaway interim SOPS. Secrets off the box — "the `.env` must be a fortress" (operator, 2026-06-27).** The first deploy ships secrets as a root-owned `chmod 600` `/opt/aer/.env` — the standard Docker-Compose pattern, protected by key-only SSH + the 22/80/443-only firewall + client-side-encrypted backups, and adequate for a single-box POC. The operator's standing instruction is that this is **not** the end state: plaintext secrets should not live on the server long-term. Evaluate + adopt a proportionate secrets-at-rest scheme — candidates: SOPS/age-encrypted `.env` decrypted at deploy time, Docker secrets, or systemd credentials — without adding a heavy dependency (Vault is over-scope for one box, Occam). Must preserve the boot-time secret validation, the no-secret-in-image rule, and the single-SSoT-`.env` interpolation discipline (the scheme decrypts *into* the same env surface, it does not fork it). **Interim vs end-state (operator decision 2026-06-27):** the full *no-`.env`-on-the-box* rearchitecture (external injection + Docker secrets — env-var secrets land in Docker's on-disk container config regardless, so encryption alone is only partial) is escalated to its own **Phase 155**. This 151 item is now the *lighter interim*: either the SOPS/age-encrypted `.env` decrypted at deploy, or simply keeping the hardened root-600 `.env` until Phase 155 lands — chosen during the Phase-150 CD work (M3b).
* [x] **✅ VERIFIED COHERENT 2026-06-27 — `dig AAAA aer-project.eu` is empty (no AAAA record published), so resolution is IPv4-only and no client is ever handed an address the stack does not serve; the A-record points at the box. The dangerous case (AAAA → unserved address) does not exist. Optional future enhancement only: publish AAAA if IPv6 reachability through Traefik + the Hetzner firewall is wanted AND verified. DNS dual-stack coherence (AAAA / IPv6) (operator, 2026-06-27).** The box is dual-stack (IPv4 `116.203.214.62` + IPv6 `2a01:4f8:1c18:82e8::1`); the **A**-record is verified to point at the box, but the **AAAA**-record must be reconciled — either pointed at the box's IPv6 *or* removed — so name resolution can't hand a client an address the stack doesn't serve (inconsistent reachability / TLS-SNI behaviour). Confirm Traefik + the Hetzner Cloud Firewall treat both families identically, or document IPv4-only intentionally.
* [x] **✅ DONE 2026-06-27 (written this session as Phase-150 M7 — `docs/operations/deploy_runbook.md`, ~15 KB, provisioning→release→rollback + the nine fresh-machine findings, in mkdocs nav). Step-by-step production deploy runbook (operator, 2026-06-27).** The first deploy (Phase 149) was driven interactively; the choreography exists only as scattered notes + the `07_deployment_view.md` minimal bring-up. Write a dedicated, ordered runbook **next to** `operations_playbook.md` capturing the full path as actually executed: host prereqs (Docker/Make/Git) → repo to `/opt/aer` → prod `.env` (real secrets, `COMPOSE_FILE` overlay, `ACME_CA_SERVER` staging→prod flip) → `make preflight` → **GHCR `aer-wikidata-index` must be public (or `docker login`)** → `make openapi-bundle` → `docker compose --profile dashboard up -d --build` (in `tmux`) → health/TLS verification → `bootstrap-admin` → restic backup prereqs (Storage-Box key port 23) + a **tested restore** → systemd timers (crawl/backup) → Grafana alert-delivery test → Go/No-Go. Fold in the **fresh-machine build findings** fixed in 149 (the `openapi.bundle.yaml` generation step, the `pnpm-workspace.yaml`/engine-3d Dockerfile fix, the GHCR-public requirement, and the overridable `SENTIWS_URL` build arg — the canonical Leipzig mirror was down during the first deploy, so the worker image fetched the SHA-pinned SentiWS zip from an Internet Archive snapshot; the `postgres:18` `PGDATA` pin — PG18 hard-refuses the legacy `/var/lib/postgresql/data` mount on a fresh volume unless `PGDATA` is set back to it; the `minio-init` ILM read-back rewritten off `grep` — the minio/mc image is minimal and has no `grep`/`tr`/`awk`, so the SEC-057 assertion must use the POSIX `case` builtin; and the restic/Storage-Box setup — SSH/SFTP on **port 23**, session lands in **`/home`** with no shell, so `RESTIC_REPOSITORY` must point **under `/home`** or `restic init` fails `SSH_FX_FAILURE`) so the runbook is self-contained. Add to `mkdocs.yml` nav. This is the "runbook run end-to-end" deliverable Phase 149 references but did not yet write as a doc.

### Validation
* [x] **✅ DONE 2026-06-27.** Per-client limit **verified** (SEC-014 — per-client isolation + strict-budget + idle-eviction tests green); every safe worker tunable is **settable from `.env`** (SEC-043 — 4 knobs forwarded `:-default` + documented, defaults ship); **DNS is intentionally IPv4-only** (no AAAA published, so resolution never hands a client an unserved address); a new operator can bring up production from **`deploy_runbook.md` alone**, and a **tested restore** was executed end-to-end in Phase 149 (6603 Gold rows recovered into a clean ClickHouse). **Two clauses relocated to Phase 155** (operator decision 2026-06-27): the **env_file refactor** (D9) and **secrets no longer plaintext on the box** — both touch the same per-service wiring + secret surface, so they land once in 155, not twice; the interim is the hardened root-600 `.env`.

---

## Phase 155: Secrets Off-Box — Deploy-Time Injection, no `.env` on the server [P1] - [ ] TODO

*The correct end-state for secret handling, decided 2026-06-27 (operator): the production box stores **no standing secret files** — secrets are injected at deploy time and never written to disk by the app layer. This **supersedes** the interim SOPS/age option in Phase 151. Encrypting the `.env` is only a partial measure, because Docker Compose persists every container's `environment:` to `/var/lib/docker/containers/*/config.v2.json` in plaintext regardless — so a real fortress needs file-based secrets on tmpfs, not env vars, plus an external source of truth. "Modern apps inject at build/deploy time; a `.env` has no place on the server, encrypted or not."*

**Why deferred (it is a rearchitecture, not a config tweak).** The honest target needs BOTH:
1. **External source of truth** — secrets live only in GitHub Actions Secrets (or a managed store), never in the repo or at rest on the box; the CD deploy job (Phase 150 M4) supplies them at `up` time.
2. **File-based secrets, not env vars** — move the stack from compose `environment:`/`env_file:` to **Docker secrets** (tmpfs-mounted `/run/secrets/*`) and teach each service's config loader the `<VAR>_FILE` convention (read the secret from a file path, else fall back to the env var — backward-compatible). Only tmpfs-mounted secret files keep secrets off the box disk; env-var secrets are unavoidably written to Docker's on-disk container config.

**Scope.** Audit every `.env` secret (build-time vs runtime); add `_FILE` support to the Go config (`pkg/config`) + the worker dotenv loader; convert compose to Docker secrets sourced from the external store at deploy; remove the standing `.env` from the box; retire the Phase-151 SOPS/age machinery once injection lands; re-validate backup/restore (they read secrets too); document in `deploy_runbook.md` + arc42. **Includes D9 (relocated from Phase 151, operator 2026-06-27):** replace the hand-maintained per-service compose `environment:` lists with generated per-service `env_file`s in the SAME pass — the non-secret config consolidation and the secret→Docker-secrets split are one and the same per-service wiring surface, so they are done together, not twice.

**Grounding / preserve:** boot-time secret validation, the no-secret-in-image rule, least-privilege roles, the GHCR read-only token, the KeePass escrow as ultimate fallback. **Verify-first:** confirm Docker mounts the secrets on tmpfs (not disk) on the real target before declaring the box secret-free.

### Validation
* [ ] No standing secret file exists on the box (`.env` gone); secrets resolve from tmpfs `/run/secrets/*`; the boot validators stay green; a leaked `/var/lib/docker` no longer yields plaintext app secrets; backup/restore still work; `deploy_runbook.md` + arc42 reflect the injection model.

---

# Deferred Phases

*Recorded decisions, not committed work. Each carries explicit trigger conditions so the deferral is not silent.*
---

## Deferred: Phase 122a.0: Per-Article Discourse Function — Backend [P2]

*Status: **DEFERRED** (decision 2026-05-28). Today every Gold row inherits a single source-level `discourse_function`. A tagesschau sports article is not "epistemic authority" at the text level — but inherits it. This sub-phase would classify discourse function per article (Option C per WP-001 §5.4: both tags stored independently, no synthesis; source-level tag stays primary) via a four-stage pipeline, multilingual by construction (mDeBERTa).*

**Why deferred (per WP-001 §5.4.4).** Sequenced behind two foundations it rests on: **(1)** the source-level tag is `provisional_engineering` (self-assigned, not peer-reviewed) and the four-function taxonomy is unvalidated across cultures — refining article-level precision on an unvalidated foundation sharpens a second decimal while the first is a placeholder; **(2)** the analytically meaningful signal is *temporal drift within a source* (Episteme), which needs both time-depth and a retained temporal aggregate, neither of which exists at single-probe POC scale. The static-divergence snapshot the original validation gate measured ("≥5% away from `epistemic_authority`") is the weak form — it largely re-describes section structure. Nothing is published (provisional discipline), so deferral cost ≈ lost forward time-depth (small) against the risk of accruing DF history on a taxonomy that may be revised out-of-band.

**Re-open triggers (BOTH must hold).** (i) the source-level classification is peer-reviewed / promoted past `provisional_engineering` **OR** the four-function taxonomy is stress-tested out-of-band; **AND** (ii) a concrete need for the *drift* signal — a probe (or cross-probe set) with enough temporal depth that source-function drift becomes observable. **First step at re-open = a throwaway offline probe**: run `mDeBERTa` over a sample of existing Silver, no schema/worker/BFF changes, to answer "is there structured divergence at all?" *before* any production build. If the offline probe shows only trivial section-structure divergence, the phase stays deferred.

**Grounding (at re-open).** Read first: `internal/models/discourse.py` (current source-level model), the extractor registration in `main.py` + `internal/extractors/`, the ClickHouse `metrics`/`entities` schema + the resolution MVs (Phase 122c), `internal/adapters/` Postgres usage, the BFF `/articles/{id}` + `/metrics` specs, `language_capabilities.yaml`. Preserve: the `MetricExtractor` protocol (`extract_all`), the language-detection-first ordering, graceful-degradation (missing model → no rows, not DLQ). Verify-first: confirm the exact metrics/entities columns + the manifest schema before migrating.

### Schema (at re-open)
* [ ] **ClickHouse** — add to `aer_gold.metrics` AND `aer_gold.entities`: `discourse_function_article LowCardinality(String) NULL`, `discourse_function_method LowCardinality(String) DEFAULT 'unclassified'`, `discourse_function_article_confidence Nullable(Float64)`. Source-level `discourse_function` unchanged — no "effective tag". **Drift requirement:** the article-DF dimension MUST flow into a *retained* temporal aggregate (the daily/monthly MVs, Phase 122c — daily 1825 d, monthly indefinite), not only the 365 d raw `metrics` rows, so drift over the Episteme horizon is observable.
* [ ] **Postgres** — `discourse_function_overrides(article_id PK, function, reviewer, review_date, rationale)`.

### Worker
* [ ] **`DiscourseFunctionClassifier`** — four stages (manual-override → URL-section heuristic → zero-shot `mDeBERTa-v3-base-mnli-xnli` → source-default). Pre-fetched, `TRANSFORMERS_OFFLINE=1`, determinism flags, synchronous in the main pipeline.
* [ ] **`configs/discourse_function_rules.yaml`** (Probe 0; FR added in 123). **Capability Manifest** — `discourse_function_classification` carries a **per-language `validation_status`** (`unvalidated` until a held-out hand-annotated set exists for that language), not a flat language list. Validation is **per language, not per probe** (WP-001 §5.4.5), bounded by the manifest (~10 languages) — this is what keeps it feasible at hundreds of probes. Validation gates the *measurement claim*, never the *observation*: an `unvalidated` language still runs the classifier but its output is flagged and never feeds an aggregate. **Config** — `DF_CLASSIFIER_CONFIDENCE_THRESHOLD` (0.6), `DF_CLASSIFIER_ENABLED` (true).

### BFF
* [ ] `?discourseFunctionScope=article|source` on `/metrics` (default `source`; `article` excludes nulls + returns `unclassified_share` + per-language `validation_status`). The **time-series read path carries the article-DF dimension** so the drift cell can render off the MVs. `/articles/{id}` gains `{source, article, method, confidence}`. New `/sources/{id}/discourse_function_distribution`. OpenAPI + `make codegen`.

### Documentation
* [ ] **ADR-030 — Per-Article Discourse Function Classification** (drafted at deferral, see `docs/arc42/09_architecture_decisions.md`; finalised here). `metric_validity` scaffold rows (`unvalidated`). Operations Playbook: confidence histogram per source, abstain rate, manual-override workflow.

### Validation (at re-open)
* [ ] Offline probe run first (above). Then: the article-DF distribution is queryable as an **Episteme time-series (drift)** off the retained MVs; per-language `validation_status` surfaces on the API; source-level `discourse_function` is unchanged; `discourseFunctionScope=article` returns `unclassified_share`.

---

## Deferred: Phase 122a.1: Per-Article Discourse Function — Frontend [P2]

*Status: **DEFERRED** (decision 2026-05-28), blocked by 122a.0. Surfaces the per-article classification. Written mount-agnostic because Phase 123a later moves the ProbeCard from the Dossier page into the Dossier overlay.*

**Primary surface = drift, not static distribution (per WP-001 §5.4.4).** The valued reading is *temporal* — a source's article-DF mix drifting over time (Episteme: "this EA source publishes a rising share of PL content over six months" = a politicisation/capture signal), not a synchronic snapshot (Aleph). Drift is also the form most robust to taxonomy uncertainty: a *change* in the mix is a real signal even if the absolute labels are later revised. The first cell is therefore a **`discourse_function_drift` time-series** (Episteme); the static `discourse_function_spannweite` distribution (Aleph) is a *secondary, optional* companion, explicitly the weak form.

**Grounding.** Read first: Phase-122a.0 BFF output, `L5EvidenceReader.svelte`, `src/lib/components/dossier/ProbeCard.svelte` + the DF-cards, `FunctionBadge` + `discourse-function.ts`, the Negative-Space toggle (`tray.svelte.ts`), the Phase-131 cell framework, the Episteme presentation set in `viewmodes/registry.ts`. Preserve: the `FunctionBadge` primitive as the single DF representation, the Negative-Space URL toggle. Verify-first: build the ProbeCard work **mount-agnostic** — 123a re-mounts ProbeCard into the Dossier overlay.

### Frontend
* [ ] **`discourse_function_drift` cell (PRIMARY)** — registered in the Cell registry (**Episteme**, `time_series`); per-source article-DF proportions over time, source default as a reference line. MethodologyBanner cites WP-001 §5.4 and surfaces the per-language `validation_status` (never present an `unvalidated` series as a measurement).
* [ ] **`discourse_function_spannweite` cell (SECONDARY, optional)** — static per-source distribution bar (Aleph). Ships only if the drift cell reveals structure worth a synchronic companion.
* [ ] **L5EvidenceReader divergence indicator** — source vs article DF side by side with a diverging-arrow when they differ; MethodologyBanner hover cites WP-001 §5.4.
* [ ] **ProbeCard DF-card** — sparkline histogram per source-card ("78% EA · 15% CI · 7% other"), mount-agnostic, with a `(provisional / unvalidated)` marker until the language is validated.
* [ ] **Negative-Space overlay** — divergent articles annotated when the toggle is on.

### Validation
* [ ] Drift cell renders the per-source article-DF series with the source-default reference line and the `validation_status` marker; a diverging article shows the L5 indicator; the ProbeCard sparkline renders.

---

## Deferred: Phase 122a.2 — Source × Article Discourse-Function Pair Validation [P3]

*Moves from Option C (both DF tags observable, no synthesis) to Option A (the `(source_df, article_df)` pair as a first-class analytical unit). Gated on Phase-122a.1 divergence data demonstrating interpretable structure AND interdisciplinary completion of the 4×4 pair-interpretation work. Triggers: a temporally non-trivial divergence pattern; collaborators proposing a concrete pair-reading; or a downstream question unanswerable with Option-C semantics. If none fires, it stays deferred — Option C suffices. (Honours the operator principle: avoid inventing imprecise metrics.)*

---

## Deferred: Non-Human Actor Detection (full WP-003 §5)

*Reserved for the iteration that lands the first social-media probe. AI-text detection on news sources is methodologically wrong-shaped for a solo dev (professional editing confounds stylometric features; high false-positive rates; an arms race with no stable equilibrium per WP-003 §5.2). Social-media account-level signals (Cresci 2020) and network-level coordination (Pacheco et al. 2021) are deterministic / established statistics — tractable when that source class exists. Reserved scope: `aer_gold.coordination_clusters`, `account_features/` + `network_features/` layers, the `CorpusExtractor` protocol, AI-text detection only if calibrated per source class with documented FPR. The news-source authenticity signal AĒR ships now is silent-edit observability (Phase 122d.x).*

---

## Deferred: Probe-1 Tier-1 (FEEL) + Tier-2.5 (CamemBERT) Sentiment Enrichments [P3]

*French within-frame sentiment enrichments, deferred from Phase 123. The multilingual news backbone covers French for both within-frame and the cross-probe basis; FEEL (Tier-1 lexicon) and CamemBERT (Tier-2.5 fine-tuned) would add within-frame depth only — never the cross-probe basis. Add later if the effort is justified for richer French within-frame analysis. Trigger: a concrete French within-frame need the backbone does not serve well. Per-source-class backbone strategy means new probes scale at O(1) — these enrichments are explicitly optional and must never become a per-language onboarding requirement.*

---

## Deferred: Phase 132 — News-Backbone Sentiment Model Evaluation [P2] (Epic, methodology-first)

*Originally drafted as a Pre-Probe-1 Iteration-7 phase that would re-select the `shared.multilingual_bert` backbone (currently `cardiffnlp/twitter-xlm-roberta-base-sentiment`, a Twitter-domain model running on news text — a domain mismatch ADR-023 itself acknowledges). Deferred because (1) cross-probe comparison is not AĒR's primary use case — within-probe analysis on the current Tier-2 backbone + the Tier-2.5 German-news refinement (`mdraw/german-news-sentiment-bert`) is methodologically sound for the engineering POC stage; (2) backbone re-selection is interdisciplinary work the solo operator cannot responsibly close alone — it needs annotation guidelines, inter-annotator agreement, calibration analysis; (3) committing to a backbone now risks a second re-processing later if the methodology shifts. Probe 1 ships on the status-quo backbone (the cross-probe Tier-2 column is still populated for both languages, just with the acknowledged domain mismatch). Within-frame sentiment surfaces are unaffected.*

*Scope when re-opened: a **methodology epic**, not a pick-the-best-model exercise. The deliverable is a reusable evaluation framework every future source-class backbone re-uses — model selection is a downstream by-product.*

*Methodology deliverables:*
* *Gold-set definition (sample size derivation, annotator selection, annotation guidelines, 3-vs-5-class taxonomy decision, inter-annotator-agreement threshold and procedure).*
* *Eval metrics suite (agreement = Cohen's κ; calibration = Expected Calibration Error; cross-language transfer consistency DE↔FR↔EN; robustness on news's neutral-skewed class imbalance; long-text aggregation strategy since news articles exceed 512 tokens on classic BERT-style backbones).*
* *Reproducibility (pinned revisions, deterministic seeds per ADR-016, eval harness committed under `scripts/evaluation/sentiment_backbone/`, recorded honestly as provisional per WP-002).*
* *Operator handbook entry: how to re-run the evaluation when a new candidate model lands.*

*Cutover deliverables (only after the methodology is in place and runs cleanly):*
* *Candidate shortlist (fresh market scan at re-open time; orientation snapshot from the Mai-2026 scan recorded here so it is not lost): (A) status-quo baseline `cardiffnlp/twitter-xlm-roberta-base-sentiment` (Twitter domain); (B) `clapAI/modernBERT-large-multilingual-sentiment` (Apache 2.0, ModernBERT architecture, 8K context — long news articles fit without truncation, ~396M params, generic-domain training data); (C) `clapAI/roberta-large-multilingual-sentiment` (Apache 2.0, higher reported F1 but 512-token cap and 560M params); (D) zero-shot via `MoritzLaurer/mDeBERTa-v3-base-mnli-xnli` (zero extra image footprint — Phase 122a.0 already loads it); (E) the only true news-domain multilingual release found in the scan, `z-dickson/multilingual_sentiment_newspaper_headlines`, is headline-only and too narrow to serve as the article-body backbone. Generic non-commercial-licensed candidates like `tabularisai/multilingual-sentiment-analysis` (CC-BY-NC-4.0) are excluded on license grounds. The market gap (no Apache-licensed, full-article, multilingual news-domain SOTA) may justify a future in-house fine-tune as a separate epic ("Phase 132b: AĒR-Backbone-Fine-Tune") rather than another off-the-shelf swap.*
* *Chosen model wired into `language_capabilities.yaml` (pinned revision); Probe-0 + Probe-1 sentiment re-processed; ADR-023 amendment (per-source-class backbone rule + cross-probe-backbone restriction + the model-record line) and the existing ADR-023 duplicate cleaned up (the duplicate-cleanup alone stays in Phase 129's Documentation Sweep regardless).*

*Triggers to re-open:*
* *An interdisciplinary collaborator (annotation, linguistics, computational social science) can carry the gold-set work — the bottleneck is annotation, not engineering.*
* *A concrete downstream analytical question that fails on the status-quo backbone in a way the within-frame Tier-2.5 refinement does not absorb.*
* *AĒR exits the engineering-POC stage and moves toward something publication- or deployment-shaped where the documented domain mismatch becomes a load-bearing weakness.*

*Until then: status quo holds. Within-probe sentiment is the load-bearing surface; the cross-probe overlay uses the same backbone for both probes with the domain caveat surfaced via `metric_provenance.yaml`.*

---

## Deferred: Phase 133 Slice 6 — Custom-Extractor Visible-HTML Metadata Fields + ADR-036 Re-Attempt [P2]

*Status: **DEFERRED** (decision 2026-06-06), split out of Phase 133. Phase 133 Slices 1–5 (scalar metadata → Gold metrics, categorical metadata → `aer_gold.article_metadata`, the BFF read paths + availability gate, the categorical cell + picker, institutional authorship) shipped against the **structured-only** extraction path (extruct JSON-LD/OG/microdata + htmldate + `<meta>`). The whole metadata surface is **generic over all fields**: a field with no data is simply never offered (availability gate). This sub-phase makes the publisher-visible-HTML-only fields extractable so they stop being empty — at which point they light up in the existing picker/cells with **zero frontend or BFF change** (Slices 1–4 are field-agnostic).*

*Why deferred.* It needs no new analytical surface — it only feeds the existing one — and it is a self-contained worker+crawler change with its own image rebuild + Bronze reprocess. Decoupling it lets Slices 1–5 land + be reviewed/rebuilt once, rather than coupling a second worker rebuild into the same pass.

**Concrete evidence (verified during Phase 133 grounding).** On the current corpus these fields are **0 %** structured-populated and therefore currently render as Negative Space: `reading_time_minutes`, `comment_count`, `external_citations`, `editorial_labels`, `dateline_location`, `editor`. The canonical example: bundesregierung emits the reading time only as visible HTML (`span.bpa-reading-time__time` → "3 Min. Lesedauer") and the image credit only in a `<figcaption>`, while its structured data is just a `BreadcrumbList` JSON-LD + OpenGraph — so the structured-only path records these as `null` even though the value is in the captured Bronze HTML.

**Scope when re-opened.**
* **Upgrade `crawlers/web-crawler/.../web_extract.py::_apply_custom_extractors`** (today: xpath/css → verbatim string into `meta.source_extras`, feeds neither coverage nor analysis): add (a) **typed-field targeting** — a rule maps into a typed Tier-B/C `WebMeta` field, not `source_extras`; and (b) **per-field value coercion** — e.g. `"3 Min. Lesedauer"` → `reading_time_minutes=3`, a `<figcaption>` → an `ImageRef`.
* **Extend the extraction-method vocabulary in BOTH places together** — `ALLOWED_EXTRACTION_METHODS` (`services/analysis-worker/internal/adapters/web_meta.py`) AND it flows through `metadata_coverage._normalise_method` / the shared `is_extraction_present` helper — with the new tag (e.g. `custom_css` / `custom_xpath`). If only one side is extended, coverage records the field as `null` and the Phase-133 promotion gate (`is_extraction_present`, shared by `metadata_metrics.py` + `article_metadata.py`) silently treats it as absent — so the field would extract but never surface. **This is the load-bearing coupling: change the vocabulary in one place and both the coverage matrix and the metric/categorical promotion update together.**
* **Per-source rules** in `crawlers/web-crawler/probes/<id>/sources.yaml > custom_extractors:` (the reserved `custom_extractors: {}` slot already exists, empty, on every source). One rule-set covers a whole CMS (bundesregierung's `bpa-*` classes are stable site-wide). Rules fail gracefully to `null`; **never fabricate a field the page lacks.**
* **Register a re-extract-from-Bronze `ReAttemptTask` (ADR-036)** following the `services/analysis-worker/internal/reattempt.py` pattern (boot + interval, idempotent, the `WaybackReAttemptTask` precedent): when a new rule is added, it **backfills the existing Bronze corpus** (within the 90-day Bronze TTL, `infra/minio/setup.sh`) via the `WebAdapter.harmonize` replay path (`scripts/operations/reextract_silver.py` is the operational driver), not only future crawls — per the "no silent permanent gaps" guardrail. Each custom-extracted value records its own `extraction_methods` provenance so coverage + the Phase-122f Negative-Space surface stay honest about HOW a field was obtained.

**Re-open trigger.** A concrete need for one of the visible-HTML-only fields (most likely bundesregierung/PL reading-time or image-credit), OR a thin-source enrichment pass. Worth it selectively for the thin (typically PL) sources; never a per-source onboarding requirement.

**Per-dimension content + provenance** for the fields this lights up (`reading_time_minutes`, `comment_count`, `external_citation_count` provenance already registered in `metric_provenance.yaml`; their rich dual-register content + any newly-visible categorical fields' content authored here, EN+DE, when they become visible).

---

## Deferred: Node-First Co-occurrence — decouple node count from edge count [P2]
*Status: **DEFERRED** (decision 2026-06-22). Placed outside Iteration 13 (Deployment & Infrastructure) on purpose — this is an analytical **feature** (a Phase-131 Configurable-Cells descendant on the Workbench), not deploy-readiness, and it is not deploy-blocking (the current network works well). Re-open as a post-deploy feature so it does not muddy the 148b→148c→148d→149→150 deploy sequence.*

**The ask (operator, observing the live cell).** The co-occurrence `topN` lever controls **edges**, not nodes: the BFF returns the top-N strongest co-occurrence *pairs* (`ORDER BY sum(cooccurrence_count) DESC LIMIT topN`) and derives nodes as the union of incident entities — so `topN=6000` yields ~6000 edges but only ~777 nodes (hub structure: a few hundred frequent entities form thousands of pairwise links). The operator wants to drive toward **~5000 nodes** directly.

**Feasibility — confirmed (2026-06-21):** (a) the data exists — distinct entities: tagesschau 4791, franceinfo 3843, all-sources 8958, so 5000 nodes is real (near the ceiling for a single source, comfortable across a probe); (b) the renderer already scales — `CoOccurrenceNetworkAtScale.svelte` (sigma.js / WebGL + ForceAtlas2-in-a-worker, auto-routed above ~500 edges) handles thousands of nodes; (c) the only real gap is the **query model** (edge-first → node-derived). `aer_gold.entities` has no per-entity count column, but `GROUP BY entity_text` ranking is trivial in ClickHouse.
**Design — decouple into two levers (matches the operator's intuition).**
1. **Nodes lever (NEW, `nodeTopN`):** top-N entities by **mention frequency** (`GROUP BY entity_text, count() … LIMIT nodeTopN`), scoped to the selected sources/window, up to ~5000. Directly controls the node count — and incidentally retires the Top-N-means-edges confusion entirely.
2. **Edges lever (the existing `topN`, reframed):** co-occurrence edges **among the selected node set**, with a **strength config** (`minWeight` threshold and/or strongest-K) — the operator's "strongest-only vs. all, with a strength control". Edges are always fetched for **layout** (clustering); the existing `showEdges` toggle only hides the *lines*.

**Clustering is preserved** — ForceAtlas2 is edge-driven, so the edges (kept for layout even when lines are hidden) still pull connected nodes into clusters exactly as today. **Caveat to disclose:** with 5000 nodes, entities that are frequent but rarely co-occur float as **isolated nodes** (no edge in the thresholded set); the strength config trades clustering completeness against performance.

**Semantic shift (methodology-relevant, WP-001/WP-007).** Edge-first answers *"what is most strongly connected"*; node-first-by-frequency answers *"what dominates the discourse, and how it relates"* — a **new analytical mode**, not "more of the same". Needs its own `how-to-read` note and provenance framing. **Disclosure (ADR-039):** "top N of M entities by frequency" is a truncation → Negative Space; isolated-node share disclosed.

**Scope when re-opened.**
* **BFF:** a top-entities-by-frequency query (new) + invert the edge query to fetch edges *restricted to the selected node set*; new `nodeTopN` param + OpenAPI contract + storage tests; an edge cap among the node set so the layout/payload stays bounded.
* **Frontend:** a **Nodes** lever + the reframed **Edge-strength** lever, URL/state wiring, levers + `how-to-read` + i18n EN+DE; reuse the existing `CoOccurrenceNetworkAtScale` renderer (no new render path).
* **Perf:** 5000 nodes in sigma/WebGL is fine; the edge set among them must be `minWeight`/top-K capped for layout speed.

**Open design decisions (operator, at re-open).** Two separate levers vs. one combined? Edge filter = `minWeight` vs. strongest-K vs. both? Default node count? Node ranking by raw frequency vs. degree?

**Re-open trigger.** After deployment stabilises (post-Phase-150), OR when richer relational/dominance analysis becomes the priority. No code until then; the current edge-first network stays the shipped behaviour.

---
