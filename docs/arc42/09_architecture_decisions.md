# 9. Architecture Decisions

This chapter records all significant architectural decisions using the Architecture Decision Record (ADR) format. Each ADR captures the context, the decision, and its consequences. ADRs are immutable once accepted — if a decision is superseded, the original ADR is marked as such and a new one is created.

> **Note:** ADR-001 was an informal, undocumented decision made during project inception (selection of the Monorepo structure). It is retroactively captured in Chapter 2 (Architecture Constraints) and not repeated here.

## ADR-002: Data Governance, Resiliency, and The Silver Contract

**Date:** 2026-03-28  
**Status:** Accepted

### Context
The AĒR system relies on unstructured, potentially chaotic data collected from external sources into the `bronze` Data Lake layer. The Python `analysis-worker` is responsible for processing this data asynchronously via NATS events. Two major risks were identified:
1. **Malformed Data:** Unexpected schema changes or missing critical fields in the raw data can cause the Python worker to throw unhandled exceptions, crashing the service and blocking the NATS message queue.
2. **Duplicate Events:** NATS JetStream guarantees "at-least-once" delivery. Network partitions or duplicate ingestion jobs could lead to the exact same raw data being processed twice, resulting in distorted, duplicated metrics in the ClickHouse `gold` layer.

### Decision
To guarantee deterministic execution and scientific integrity, we implemented the following resilience patterns:
1. **The Silver Contract (Pydantic):** Before data is promoted from `bronze` to `silver`, the Python worker maps the raw data and strictly validates it against a predefined `SilverRecord` Pydantic model. 
2. **Dead Letter Queue (DLQ):** If the validation fails (e.g., missing mandatory fields), the worker catches the `ValidationError`, gracefully aborts processing, and routes the raw JSON into a dedicated `bronze-quarantine` bucket. The worker does not crash.
3. **Storage-Level Idempotency:** Before processing a NATS event, the worker checks if the `object_key` already exists in the `silver` or `bronze-quarantine` buckets. If it does, the event is acknowledged but skipped, guaranteeing exactly-once processing semantics for the downstream analytics database.

### Consequences
* **Positive:** The processing pipeline is highly robust. Corrupt data is isolated for manual inspection without halting the system. Metric duplication is structurally prevented.
* **Negative:** Checking MinIO for existing object keys adds a slight latency overhead (one HTTP HEAD request per event) before processing. Given the system's asynchronous nature, this tradeoff is acceptable.

---

## ADR-003: The Metadata Index and Progressive Disclosure

**Date:** 2026-03-28  
**Status:** Accepted

### Context
A core UI/UX goal of the AĒR dashboard is "Progressive Disclosure". When a sociologist analyzes an aggregated time-series metric (e.g., a spike in a specific keyword in the Gold layer), they must be able to click on that data point and drill down to the exact original raw document (Bronze layer) that caused it. Because ClickHouse (Gold) is an OLAP database optimized for aggregations, it is highly inefficient for storing and querying deep relational metadata and full file paths.

### Decision
We introduce a dedicated Metadata Index using **PostgreSQL**. 
1. The Go `ingestion-api` writes a record to PostgreSQL detailing the `source_id`, `job_id`, the exact `bronze_object_key` (MinIO path), and the OpenTelemetry `trace_id` before saving the file to the Data Lake.
2. The frontend will use the aggregate data from ClickHouse for the high-level "weather map" visualizations.
3. For drill-downs, the frontend will query the PostgreSQL database to resolve the exact file origins and trace executions.

### Consequences
* **Positive:** Clear separation of concerns. ClickHouse remains extremely fast and lean because it only holds numerical time-series data. PostgreSQL securely handles relational mapping and audit trails.
* **Negative:** The `ingestion-api` has to manage a distributed transaction span (writing to MinIO and PostgreSQL sequentially). If MinIO succeeds but PostgreSQL fails, there is an unindexed file in the data lake (an acceptable edge case handled by eventual consistency sweeps later).

---

## ADR-004: Contract-First Backend-for-Frontend (BFF)

**Date:** 2026-03-28  
**Status:** Accepted

### Context
To power the AĒR UI dashboard (e.g., the "weather map" visualizations), the frontend requires fast, reliable access to the aggregated sociological metrics stored in the ClickHouse `gold` layer. API drift (where documentation and implementation go out of sync) and type mismatches between frontend and backend are common sources of bugs.

### Decision
We implemented a dedicated Backend-for-Frontend (BFF) service in Go, strictly following a **Contract-First** API design:
1.  **Modular OpenAPI 3.0:** The API is defined entirely in modular YAML files (`openapi.yaml`, `paths/`, `schemas/`, etc.) before any business logic is written.
2.  **Code Generation:** We use `oapi-codegen` with the `strict-server` configuration to automatically generate the HTTP routing boilerplate (via `chi`) and strictly typed Go structs. 
3.  **Direct OLAP Access:** The BFF connects directly to ClickHouse using the native `clickhouse-go` driver to serve analytical queries with maximum performance.

### Consequences
* **Positive:** The API documentation is the single source of truth and is guaranteed to match the implementation. Type safety prevents runtime JSON marshaling errors. The modular OpenAPI structure ensures long-term maintainability as the API grows.
* **Negative:** Developers must learn the `oapi-codegen` workflow and remember to run `make codegen` whenever the API contract is modified.

---

## ADR-005: Hybrid Testing Strategy (Mocks vs. Testcontainers)

**Date:** 2026-03-28  
**Status:** Accepted

### Context
To ensure the long-term stability of the AĒR pipeline, an automated testing strategy is required. The system consists of stateful, IO-heavy Go adapters (Ingestion, BFF) and stateless, logic-heavy Python workers (Analysis). Using a single testing paradigm (e.g., only mocking or only end-to-end testing) across all languages leads to fragile or extremely slow CI pipelines.

### Decision
We adopt a hybrid testing strategy tailored to the responsibilities of each layer:
1. **Python (Analysis Worker):** We use **Unit Tests with Mocks** (`pytest` and `unittest.mock`). Since the worker's sole responsibility is deterministic data transformation and contract validation (Pydantic), mocking MinIO and ClickHouse ensures tests run in milliseconds and focus purely on business logic (e.g., DLQ routing, idempotency).
2. **Go (Ingestion & BFF API):** We use **Integration Tests with Testcontainers** (`testcontainers-go`). Since the Go services act as glue code between the outside world and our databases (Postgres, MinIO, ClickHouse), mocking the databases would render the tests useless. Testcontainers spin up real, ephemeral Docker containers to validate SQL schemas, queries, and S3 uploads.

### Consequences
* **Positive:** High test reliability. Python logic is tested fast and in isolation. Go storage adapters are tested against real database engines, preventing schema drift bugs.
* **Negative:** The Go integration tests will take slightly longer to execute in the CI pipeline because they require pulling and starting Docker images.

---

## ADR-006: Graceful Degradation & Exponential Backoff

**Date:** 2026-03-28  
**Status:** Accepted

### Context
In a distributed microservice architecture, startup sequences and transient network failures are unpredictable. If a Go service (e.g., Ingestion API, BFF API) boots faster than its required infrastructure (PostgreSQL, MinIO, ClickHouse), or if a database temporarily drops connections, the service traditionally crashes (`os.Exit(1)`). This leads to cascading failures and requires external orchestrators to constantly restart containers.

### Decision
We implement a **Context-Aware Exponential Backoff Strategy** using `github.com/cenkalti/backoff/v5` for all infrastructure connection attempts.
1. Database and Object Storage adapters must wrap their initial connection and ping requests in a generic retry loop.
2. The loop uses an exponential backoff algorithm (e.g., waiting 1s, 2s, 4s...) up to a defined maximum elapsed time (e.g., 30 seconds).
3. The retry mechanism must be bound to the application's global `context.Context`. If the system receives a shutdown signal (`SIGINT`/`SIGTERM`) during the backoff period, the retry loop is immediately aborted.

### Consequences
* **Positive:** The system becomes self-healing. Services can be started in any order. Temporary network partitions do not require manual intervention. The use of `v5` generics allows returning initialized connections directly.
* **Negative:** Service startup might be intentionally delayed if infrastructure is down, meaning immediate failure feedback is suppressed in favor of resilience.

---

## ADR-007: Data Lifecycle & Retention Strategy (Graceful Degradation)

**Date:** 2026-03-28  
**Status:** Accepted

### Context
As AĒR scales and continuously ingests raw web data (Bronze layer) and generates time-series metrics (Gold layer), storage costs and database memory usage will grow indefinitely. Unbounded data growth leads to system degradation and potential out-of-memory (OOM) crashes in ClickHouse.

### Decision
We implement automated, infrastructure-level Data Lifecycle Management (DLM):
1. **MinIO ILM (Information Lifecycle Management):** Raw unstructured data in the `bronze` bucket is automatically deleted after 90 days. Quarantined data (`bronze-quarantine`) is purged after 30 days. The `silver` layer (cleaned, structured data) serves as the persistent training/re-evaluation baseline and has no expiry.
2. **ClickHouse TTL (Time-To-Live):** Analytical time-series data in the `aer_gold.metrics` table is automatically dropped after 365 days using ClickHouse's native `TTL` feature on the `MergeTree` engine. Table schemas are managed via immutable IaC scripts (`init.sql`), not application code.

### Consequences
* **Positive:** Predictable storage costs. Protection against storage-related crashes. Zero application-level cron jobs required.
* **Negative:** Raw Bronze data is permanently lost after 90 days, meaning we cannot retroactively re-parse the original HTML/JSON for those specific records if a bug is found in the parser later (unless we re-crawl).

---

## ADR-008: Docker Network Segmentation

**Date:** 2026-04-02  
**Status:** Accepted

### Context
The initial AĒR deployment used a single flat Docker bridge network (`aer-network`) for all containers. As the system matured and TLS termination via Traefik was introduced (see ADR-012), a security review identified that any container on the network could reach any other container — including databases. If the BFF API or Traefik were compromised, an attacker would have direct network access to PostgreSQL, ClickHouse, MinIO, and NATS.

### Decision
We split the flat `aer-network` into two isolated Docker bridge networks:
1. **`aer-frontend`:** Contains only internet-facing services — Traefik (reverse proxy/TLS termination). Services that need to be reachable from the internet are also attached to this network.
2. **`aer-backend`:** Contains all databases (PostgreSQL, ClickHouse, MinIO), the message broker (NATS), the analysis worker, init containers, the observability stack (OTel Collector, Tempo, Prometheus), and the documentation server.

Only two services bridge both networks: the `bff-api` (must be routable from Traefik and must reach ClickHouse) and `grafana` (must serve dashboards externally and must reach Tempo/Prometheus internally). All other services are exclusively on `aer-backend`.

### Consequences
* **Positive:** Databases are unreachable from the internet-facing network. A compromised Traefik container cannot directly access PostgreSQL or MinIO. The blast radius of a frontend compromise is significantly reduced.
* **Negative:** Services that need to bridge both networks must be explicitly configured with two `networks:` entries. Debugging network issues requires awareness of which network a container belongs to.

---

## ADR-009: Hard-Pinning Policy & Compose as SSoT for Image Versions

**Date:** 2026-04-02  
**Status:** Accepted

### Context
Docker's `latest` tag and rolling minor-version tags (e.g., `postgres:16`) are convenient but non-deterministic — they resolve to different binaries at different points in time. This breaks reproducibility: two developers running `docker compose pull` on different days could end up with different database versions, causing "works on my machine" bugs. Additionally, Testcontainers in both Go and Python must use the same image versions as the compose stack to ensure test fidelity.

### Decision
We adopt a strict hard-pinning policy with `compose.yaml` as the Single Source of Truth:
1. **All infrastructure images** in `compose.yaml` must use immutable, patch-level tags (e.g., `postgres:18.3-alpine3.23`, `nats:2.12.6-alpine3.22`). The use of `latest`, `rc`, `alpha`, or major/minor-only tags is prohibited.
2. **Testcontainers** (both Go and Python) must dynamically parse their image tags from `compose.yaml` at test time via dedicated SSoT parsers (`pkg/testutils/compose.go` and the Python `get_compose_image()` function). No image tag may be hardcoded in test files.
3. **Upgrades** are performed manually and deliberately. Before upgrading, the changelog/release notes of the image are reviewed, the full stack is validated locally via `make up`, and the pinned version is committed to Git — enabling rollback via `git revert`.

### Consequences
* **Positive:** Fully reproducible environments. Tests always run against the exact same database versions as development and production. Upgrade decisions are explicit and auditable via Git history.
* **Negative:** Slightly higher maintenance overhead — version bumps require manual intervention instead of automatic pulls. Two images currently violate this policy (`prom/prometheus:v3`, `grafana/grafana:12.4`) and are tracked as technical debt (see Chapter 11, D-1).

---

## ADR-010: External Crawler Architecture ("Dumb Pipes, Smart Endpoints")

**Date:** 2026-04-02  
**Status:** Accepted

### Context
AĒR's long-term vision is to ingest data from hundreds of diverse sources — Wikipedia, news APIs, social media feeds, RSS aggregators, government databases. The initial approach of embedding data-fetching logic directly into the `ingestion-api` microservice would create a monolithic, tightly coupled service that grows linearly with every new data source. Each source has unique authentication, pagination, rate limiting, and data format requirements.

### Decision
We adopt a **"Dumb Pipes, Smart Endpoints"** architecture where crawlers are strictly external to the AĒR system boundary:
1. **Crawlers** are standalone programs (typically Go binaries) that live under `crawlers/` in the monorepo. Each crawler is a self-contained executable responsible for fetching data from one specific external source, transforming it into the generic AĒR Ingestion Contract format, and submitting it via `HTTP POST` to the Ingestion API (`POST /api/v1/ingest`).
2. **The Ingestion API** is source-agnostic. It accepts a `source_id` and an array of opaque JSON documents. It stores them verbatim in MinIO (Bronze layer) without inspecting the content. The crawlers are the adapters that translate source-specific formats into the generic contract.
3. **Crawlers are not orchestrated by `compose.yaml`.** They run ad-hoc (manually, via cron, or via external schedulers) and communicate with AĒR exclusively over HTTP.

### Consequences
* **Positive:** Adding a new data source requires only writing a new standalone crawler — zero changes to any AĒR microservice. Crawlers can be written in any language. They can run on different machines, on different schedules, and can be independently tested. The Ingestion API remains small and stable.
* **Negative:** There is no centralized orchestration or monitoring of crawler execution within AĒR itself. Crawler failures are invisible to the system unless the crawlers implement their own logging/alerting.

---

## ADR-011: BFF API Authentication via Static API Key

**Date:** 2026-04-02  
**Status:** Accepted

### Context
The BFF API is the only application service exposed to the internet via Traefik. Without authentication, any client with network access could read aggregated metrics from ClickHouse or probe internal system state via health endpoints. A full OAuth2/JWT setup was considered but deemed overengineered for the current single-operator deployment model.

### Decision
We implement a lightweight API-key authentication middleware on the BFF API:
1. All routes except `/healthz` and `/readyz` require a valid API key.
2. The key is accepted via the `X-API-Key` request header or the `Authorization: Bearer <key>` header.
3. The key value is configured via the `BFF_API_KEY` environment variable (sourced from `.env`).
4. Requests with missing or invalid keys receive a `401 Unauthorized` JSON response.

Health and readiness probes remain unauthenticated to support Docker healthchecks and future Kubernetes liveness/readiness probes.

### Consequences
* **Positive:** Minimal implementation complexity (a single `chi` middleware function). Blocks unauthorized access without introducing external dependencies (no OAuth provider, no JWT library, no session store). Easy to rotate by changing the `.env` value and restarting.
* **Negative:** A static API key provides authentication but not authorization — there is no concept of user roles, scopes, or per-user rate limiting. Sufficient for the current single-operator model but must be replaced with a proper identity solution (e.g., OAuth2, OIDC) if multi-user access is introduced.

---

## ADR-012: TLS Termination via Traefik Reverse Proxy

**Date:** 2026-04-02  
**Status:** Accepted

### Context
When deploying AĒR on a VPS for production use, the BFF API must be accessible over HTTPS. Implementing TLS directly in the Go `bff-api` binary would require certificate management logic, renewal automation, and key file handling — concerns that do not belong in application code. Additionally, other services (e.g., Grafana) may need TLS in the future.

### Decision
We introduce Traefik as a dedicated reverse proxy for TLS termination:
1. **Traefik** (`traefik:v3.6.12`) runs on the `aer-frontend` network and listens on ports `80` (HTTP, redirects to HTTPS) and `443` (HTTPS).
2. **Automatic TLS** is provided via ACME/Let's Encrypt using the TLS challenge (`tlschallenge`). Certificates are persisted in a Docker volume (`traefik-certs`).
3. **Service discovery** uses Docker labels. Only the `bff-api` is explicitly exposed (`traefik.enable=true`) with the routing rule `PathPrefix(/api)`. All other services have `exposedbydefault=false`.
4. **Application services remain HTTP-only internally.** TLS is terminated at Traefik — internal container-to-container communication on `aer-backend` uses plain HTTP.

### Consequences
* **Positive:** Zero TLS code in application services. Automatic certificate renewal. Centralized routing and TLS configuration. Adding TLS to additional services (e.g., Grafana) requires only Docker labels — no code changes.
* **Negative:** Traefik becomes a single point of entry and therefore a single point of failure for all external traffic. Internal traffic is unencrypted (acceptable within a Docker bridge network on a single host, but would need mTLS in a multi-host deployment).

---

## ADR-013: Network Zero-Trust & Port Hardening

**Date:** 2026-04-02  
**Status:** Accepted  
**Supersedes:** Extends ADR-008 (Docker Network Segmentation)

### Context
ADR-008 introduced network segmentation (`aer-frontend` / `aer-backend`), isolating databases from internet-facing containers at the Docker network level. However, all backend services still exposed their ports directly to the host via `ports:` directives in `compose.yaml` — PostgreSQL (5432), ClickHouse (8123, 9002), NATS (4222, 8222), MinIO (9000, 9001), OTel Collector (4317, 4318), and the Ingestion API (8081). Additionally, Grafana (3000) and the MinIO Console (9001) were accessible directly without TLS.

This undermined the segmentation: any process on the host machine — or any attacker with host access — could bypass Traefik entirely and connect to databases, the message broker, or internal APIs directly over `localhost`. On a shared VPS, this represents a significant attack surface for lateral movement.

### Decision
We adopt a **Zero-Trust network posture** where Traefik is the sole ingress point for all external traffic:

1. **Remove all host port bindings from backend services.** PostgreSQL, ClickHouse, NATS, MinIO (API + Console), OTel Collector, Tempo, and the Ingestion API no longer expose ports to the host. They communicate exclusively over the `aer-backend` Docker bridge network.

2. **Route UI services through Traefik.** Grafana and the MinIO Console are now served through Traefik with TLS termination and Host-based routing (`GRAFANA_HOST`, `MINIO_CONSOLE_HOST` environment variables). Their direct `ports:` bindings have been removed. MinIO is added to the `aer-frontend` network so Traefik can reach it.

3. **Introduce a `debug` Compose profile for developer access.** A single `debug-ports` service (based on `alpine/socat`) acts as a TCP proxy that re-exposes all removed ports to `localhost` — but only when explicitly opted in via `docker compose --profile debug up` (or `make debug-up`). The default `docker compose up` / `make up` does not expose any backend ports.

4. **Traefik remains the only service with host-bound ports** (80/443). The BFF API retains its direct port (8080) for local development but is also routed through Traefik for production TLS. The documentation server (MkDocs, 8000) keeps its host port as a dev-only convenience.

### Threat Model Addressed
| Threat | Mitigation |
|--------|------------|
| Lateral movement from compromised host process to database | No host ports — `localhost` connections to PostgreSQL/ClickHouse are refused |
| Accidental exposure of databases on a public VPS | Default compose profile binds zero backend ports |
| Unencrypted access to Grafana/MinIO Console | Both routed through Traefik with automatic TLS |
| Developer friction from locked-down ports | `make debug-up` provides explicit, reversible opt-in |

### Consequences
* **Positive:** The default deployment posture is production-hardened. Databases and internal APIs are unreachable from outside the Docker network. UI services benefit from automatic TLS. The attack surface on a shared VPS is reduced to Traefik ports 80/443.
* **Negative:** Developers must run `make debug-up` (or `docker compose --profile debug up`) for direct database access during debugging. The `debug-ports` socat proxy adds a minor TCP forwarding hop (~0.1ms latency). Host-based Traefik routing for Grafana and MinIO Console requires DNS configuration (subdomain records pointing to the VPS).