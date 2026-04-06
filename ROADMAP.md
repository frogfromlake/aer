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

### Open Phases

---

## Code Review: Phasen 39–43

## Phase 44: Extractor Pipeline Hardening — Protocol Correctness & DRY (Findings 1, 2, 5)
*The NER extractor uses fragile `id()`-based caching and an implicit `extract_entities()` method that is not part of the `MetricExtractor` protocol. The processor calls it via `hasattr()` — ad-hoc polymorphism that bypasses the protocol system. Additionally, the processor duplicates the quarantine routing block three times. This phase makes the extractor contract explicit and the processor DRY.*

* [ ] **Introduce `EntityExtractor` Protocol.** Create a second protocol in `extractors/base.py`: `EntityExtractor(MetricExtractor)` with `def extract_entities(self, core: SilverCore, article_id: str | None) -> list[GoldEntity]`. The `NamedEntityExtractor` implements `EntityExtractor`. The processor checks `isinstance(extractor, EntityExtractor)` instead of `hasattr()`. This makes the contract explicit and type-checkable.
* [ ] **Replace `id()`-Based Doc Caching with Single-Pass Extraction.** Refactor `NamedEntityExtractor` to process the spaCy doc once in a unified method (e.g., `_extract_all()`) called by both `extract()` and `extract_entities()`. The method returns `(list[GoldMetric], list[GoldEntity])`. The processor calls the unified path for `EntityExtractor` instances. Remove `self._last_doc` and `self._last_core_id` — no mutable instance-level cache. Document in `extractors/base.py` that extractors must be stateless between documents.
* [ ] **Extract Quarantine Helper in Processor.** Refactor the three identical quarantine blocks in `processor.py` into a single `_quarantine(self, obj_key, raw_content, reason, span)` method. Each call site passes only the reason string. Reduces ~30 lines of duplication to ~3 call sites.
* [ ] **Update Tests.** Add test for `isinstance(NamedEntityExtractor, EntityExtractor)`. Add test that an extractor with a non-callable `extract_entities` attribute does not crash the processor. Verify quarantine helper produces identical span attributes and metric increments.
* [ ] **Update Arc42 Documentation.** Chapter 8 (§8.10): document the `EntityExtractor` sub-protocol. Chapter 5 (§5.1.2): note stateless extractor requirement.

---

## Phase 45: Language Detection — Persist Detected Language (Finding 3)
*The `LanguageDetectionExtractor` stores `language_confidence` but discards the detected language code itself. A confidence score without the corresponding classification is analytically useless — one cannot answer "what percentage of documents are German?" from the Gold layer alone.*

* [ ] **Add `detected_language` String Metric.** The Gold metrics table stores `float64` values — a language code is not a metric. Two options: (a) encode the ISO 639-1 language code as a new column in a dedicated ClickHouse table (`aer_gold.language_detections`), or (b) store the language code as a `metric_name = "detected_language"` with the value being a hash/enum mapping. Option (a) is architecturally cleaner — evaluate and decide via an inline ADR comment in the ROADMAP, not a full ADR.
* [ ] **Create ClickHouse Migration 004.** `aer_gold.language_detections` table: `(timestamp DateTime, source String, article_id Nullable(String), detected_language String, confidence Float64)`. MergeTree, ordered by `(timestamp, source)`, 365-day TTL.
* [ ] **Extend `LanguageDetectionExtractor`.** Return both the `GoldMetric` (confidence) and the new structured output. The processor must be extended to handle this new output type — reuse the `EntityExtractor` pattern from Phase 44 or introduce a generic `StructuredOutputExtractor` protocol.
* [ ] **Add BFF Endpoint `GET /api/v1/languages`.** Returns aggregated language distribution per source: `SELECT detected_language, count() as count, avg(confidence) as avg_confidence ... GROUP BY detected_language ORDER BY count DESC`. Add to OpenAPI spec, codegen, handler, storage, and tests.
* [ ] **Update E2E Smoke Test.** Assert that `GET /api/v1/languages` returns at least one entry with `detected_language = "de"`.
* [ ] **Update Arc42 Documentation.** Chapter 5 (§5.1.3: new BFF endpoint). Chapter 5 (§5.1.4: new ClickHouse table). Chapter 13 (§13.3.1: update language detection status).

---

## Phase 46: Sentiment Provenance & Metric Hygiene (Findings 4, 8)
*The `lexicon_version` metric stores a truncated hash as a float — it is neither human-readable nor useful as a time-series value. Provenance metadata (which lexicon version produced which score) does not belong in the metrics table. This phase moves provenance to the correct layer and cleans up the metric schema.*

* [ ] **Remove `lexicon_version` Metric from `SentimentExtractor`.** The sentiment extractor should produce only `sentiment_score`. Lexicon version provenance belongs in the Silver envelope (as part of `SilverMeta`) or as a structured log, not as a ClickHouse time-series metric.
* [ ] **Add Lexicon Version to Silver Envelope.** Extend `SilverMeta` or introduce a new `extraction_provenance` field in `SilverEnvelope` that records which extractor versions, model versions, and lexicon hashes were used during processing. This is a metadata concern, not an analytical one. The exact schema is deferred to this phase — keep it minimal (a `dict[str, str]` mapping extractor name to version hash).
* [ ] **Update E2E Smoke Test.** Remove `lexicon_version` from the expected metrics list in `EXPECTED_METRICS`. Add `lexicon_version` absence assertion. Verify `sentiment_score` is still present.
* [ ] **Update Arc42 Documentation.** Chapter 8 (§8.10): document the provenance pattern. Chapter 13 (§13.3.1): remove `lexicon_version` metric reference.

---

## Phase 47: BFF API Consistency & Input Validation (Findings 6, 7, 10)
*The BFF API has inconsistent date parameter handling (`/metrics` silently defaults, `/entities` rejects), LIMIT is validated in the wrong layer, and ClickHouse queries use string interpolation for integer parameters.*

* [ ] **Unify Date Parameter Handling.** Make `startDate` and `endDate` required for all data endpoints (`/metrics`, `/entities`, `/metrics/available`). Remove the silent 24-hour fallback from `GetMetrics`. Update the OpenAPI spec to mark both parameters as `required: true`. Regenerate stubs via `make codegen`. This is a breaking API change — document it in the changelog and bump the API version comment in the spec.
* [ ] **Move `limit` Validation to Handler Layer.** In `GetEntities`, validate `limit` in the handler: `limit < 1 || limit > 1000 → 400 Bad Request`. Remove the silent correction in `clickhouse.go`. The storage layer should trust its inputs (defense in depth remains as a panic guard, not as business logic).
* [ ] **Parameterize LIMIT in ClickHouse Queries.** Replace `fmt.Sprintf(..., limit)` and `fmt.Sprintf(..., s.rowLimit)` with proper query parameter binding (`$N`). Verify that the ClickHouse Go driver supports parameterized LIMIT clauses. If not, document the limitation as an inline comment and add an explicit `if limit < 0 { limit = 100 }` guard before interpolation.
* [ ] **Update Handler Unit Tests.** Add test: `GetMetrics` without `startDate` or `endDate` returns 400. Add test: `GetEntities` with `limit=0` or `limit=5000` returns 400. Update existing tests that relied on the silent defaults.
* [ ] **Update Arc42 Documentation.** Chapter 5 (§5.1.3): document breaking change in date parameter semantics. Chapter 10: update quality scenarios for input validation.

---

## Phase 48: Temporal Extractor Defensive Guards & Extractor Robustness (Finding 9)
*The `TemporalDistributionExtractor` assumes UTC timestamps without validation. While adapters currently set UTC correctly, the extractor should be self-defending — a non-UTC timestamp would silently produce wrong hour/weekday metrics without any indication of error.*

* [ ] **Add UTC Assertion in `TemporalDistributionExtractor`.** Before extracting `hour` and `weekday`, assert `core.timestamp.tzinfo is not None` and that the UTC offset is zero. If the timestamp is naive or non-UTC, log a warning and return an empty list (consistent with other extractors' graceful degradation). Do not raise an exception — extractors must not crash the pipeline.
* [ ] **Add UTC Assertion in `SilverCore` Pydantic Validator.** Add a Pydantic `field_validator` on `timestamp` that ensures the value is timezone-aware. Naive datetimes should be rejected at the Silver contract level, not at individual extractors. This is the architecturally correct fix — the extractor guard is defense-in-depth.
* [ ] **Update Tests.** Add test: `TemporalDistributionExtractor` with naive datetime returns empty list. Add test: `SilverCore` with naive timestamp raises `ValidationError`. Ensure all existing test fixtures use `tzinfo=timezone.utc` (they already do — verify no regressions).
* [ ] **Update Arc42 Documentation.** Chapter 5 (§5.1.2): document UTC enforcement at the Silver contract level.

---

## Phase 49: BFF Query Performance — Available Metrics Caching (Finding 11)
*`GET /api/v1/metrics/available` executes `SELECT DISTINCT metric_name` on every call — a full table scan on a growing table. With only a handful of distinct metric names that change infrequently (only when new extractors are deployed), this is wasteful. This phase adds a minimal in-process cache.*

* [ ] **Implement TTL Cache for `GetAvailableMetrics`.** Add a simple in-process cache in `clickhouse.go`: a `sync.RWMutex`-protected struct holding `([]string, time.Time)`. Cache TTL: 60 seconds (configurable via `BFF_METRICS_CACHE_TTL_SECONDS`, default `60`). On cache miss or expiry, execute the query and refresh. The cache is invalidated on TTL expiry only — no event-driven invalidation needed at this scale.
* [ ] **Add Cache TTL to Config.** Add `BFF_METRICS_CACHE_TTL_SECONDS` to `.env.example`, `compose.yaml`, and `services/bff-api/internal/config/config.go`.
* [ ] **Update Tests.** Add test: two consecutive calls within TTL result in only one ClickHouse query. Add test: call after TTL expiry triggers a fresh query. Verify thread safety under concurrent access.
* [ ] **Update Arc42 Documentation.** Chapter 8 (§8.4 or new §8.11): document the caching strategy and its rationale (Occam's Razor — no Redis, no distributed cache, just in-process TTL).

---

### Architectural Notes for Future Phases (Not Scheduled)

The following concerns are deliberately **not** addressed in Phases 39–43 but are architecturally anticipated. They are listed here to ensure the current design does not preclude them.

**Corpus-Level Extraction (TF-IDF, LDA, Co-occurrence Networks):** Requires a batch processing mechanism. The `CorpusExtractor` protocol (Phase 41) defines the interface but no scheduler. Options: a NATS-triggered cron job that periodically reads from the Silver bucket, or a dedicated batch worker container. Decision deferred until Tier 2 methods are scientifically validated (Chapter 13, §13.3.2).

**Multi-Language Support:** The current probe is German-only. Adding a second language requires: a language-specific spaCy model, a language-specific sentiment lexicon, language detection at the adapter level, and language-aware tokenization. The `SilverCore.language` field (Phase 39) and the extractor pipeline (Phase 41) are designed to support this — extractors can dispatch to language-specific logic based on `core.language`.

**Silver Schema Migration Tooling:** Phase 39 introduces `schema_version` but does not implement automatic migration of existing Silver objects. If a schema change requires reprocessing historical data, a one-off migration script (reading from Silver, re-harmonizing, writing back) will be needed. This is a data engineering task, not an architectural one.

**Gold Schema Evolution Beyond Metrics + Entities:** Future analytical outputs (topic distributions, co-occurrence graphs, narrative frames) may not fit the `(timestamp, value, source, metric_name)` shape of `aer_gold.metrics`. New ClickHouse tables will be needed. The BFF API's `GET /api/v1/metrics/available` pattern (Phase 43) is designed to be extensible — a `GET /api/v1/data-types/available` meta-endpoint could discover all available analytical outputs across tables.

**Scientific Probe Selection:** The RSS probe (Phase 40) is an engineering decision. The first *scientifically motivated* probe selection requires answering Open Research Questions 1–3 from Chapter 13 (§13.6). This is a research milestone, not an engineering phase, and will be documented as a separate research deliverable.