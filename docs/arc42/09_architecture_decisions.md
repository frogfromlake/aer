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
3. **Crawlers run as one-shot containers, never as long-running services.** Each crawler has its own Dockerfile and is declared in `compose.yaml` behind the `crawlers` profile, so `make up` never starts one. Invocation is explicit and ad-hoc — `make crawl`, host cron, systemd timer, or any external scheduler wrapping `docker compose run --rm <crawler>`. Containers join the internal `aer-backend` network so they can reach `ingestion-api:8081` without exposing host ports; this is purely an ops placement and does not change the HTTP-only coupling contract above.

### Consequences
* **Positive:** Adding a new data source requires only writing a new standalone crawler — zero changes to any AĒR microservice. Crawlers can be written in any language, have independent release cadences, and be scheduled per deployment target. Dev and prod invocations are identical (`docker compose run` in both), so there's no "dev-only shortcut" to maintain. The Ingestion API remains small and stable.
* **Negative:** There is no centralized orchestration or monitoring of crawler execution within AĒR itself — Compose is the packaging/networking story, not a scheduler. Crawler failures are invisible to the system unless the crawlers implement their own logging/alerting, or the host scheduler captures exit codes.

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

---

## ADR-014: Database Migration Strategy

| Property | Value |
| :--- | :--- |
| **Date** | 2026-04-03 |
| **Status** | Accepted |
| **Resolves** | D-3 (No Database Migration Tooling), D-5 (Hardcoded Dummy Source) |

### Context

Database schemas were initialized via `init.sql` scripts mounted into Docker's `/docker-entrypoint-initdb.d/` directory. These scripts execute only on first container creation (empty volume). Any schema change required either manually altering the running database or wiping the volume entirely — an unacceptable risk for production data. Additionally, a hardcoded dummy source (`source_id=1`) in the PostgreSQL init script created an implicit coupling between the Wikipedia crawler and the database seeding order.

### Decision

We adopt a **two-tier migration strategy** tailored to each database engine:

**PostgreSQL — `golang-migrate` (startup runner)**

1. Versioned SQL migration files live in `infra/postgres/migrations/` following the `NNNNNN_description.up.sql` / `.down.sql` naming convention.
2. The `ingestion-api` runs `golang-migrate` on startup, before the HTTP server begins accepting traffic. This ensures the schema is always current without requiring a separate migration container.
3. `golang-migrate` uses PostgreSQL advisory locks, making concurrent startups safe if the service is later scaled horizontally.
4. The original `infra/postgres/init.sql` is reduced to a no-op stub. It remains mounted for Docker convention compatibility but performs no schema operations.

**ClickHouse — Shell-based versioned scripts (init container)**

1. ClickHouse lacks a native migration framework. Versioned SQL files live in `infra/clickhouse/migrations/` with the naming convention `NNNNNN_description.sql`.
2. A `clickhouse-init` container (using the same ClickHouse image) runs `infra/clickhouse/migrate.sh` on startup. The script maintains a `aer_gold.schema_migrations` tracking table and skips already-applied versions.
3. Services depending on ClickHouse (e.g., `analysis-worker`) declare `clickhouse-init` as a dependency with `condition: service_completed_successfully`.

**Source Registry — Dynamic resolution replaces hardcoded IDs**

1. The hardcoded dummy source insert is replaced by a proper seed migration (`000002_seed_wikipedia_source.up.sql`) that inserts a `wikipedia` source with its actual API URL.
2. A new `GET /api/v1/sources?name=<name>` endpoint on the ingestion-api allows crawlers to resolve their `source_id` dynamically by name instead of assuming `source_id=1`.
3. The Wikipedia crawler defaults to dynamic resolution (`-source-id=0`) but retains the explicit flag for backward compatibility.

### Alternatives Considered

| Alternative | Reason for Rejection |
|-------------|---------------------|
| **Flyway / Liquibase** | JVM-based — introduces a ~200MB runtime dependency for a task achievable with a 5MB Go binary. Violates Occam's Razor. |
| **Atlas (ariga.io)** | Declarative schema diffing is powerful but adds complexity for sequential migrations. `golang-migrate` is simpler and more widely adopted. |
| **Embed migrations in Go binary** | Would couple migration files to the Go build. Keeping them in `infra/` maintains the IaC convention and allows the files to be used independently (e.g., by CI or manual psql sessions). |
| **Dedicated migration init container** | Adds container coordination overhead. Running migrations in-process is simpler and provides clearer startup error reporting. |

### Consequences

* **Positive:** Schema evolution is now possible without volume wipes. All changes are versioned, auditable, and reversible. Crawlers are decoupled from database seeding order. The migration contract (`CREATE IF NOT EXISTS`, `ALTER ... IF NOT EXISTS`) enables no-downtime schema changes.
* **Negative:** The ingestion-api startup time increases slightly (~50-100ms) due to migration version checks. The ClickHouse migration runner is a custom shell script rather than a battle-tested framework — it must be kept simple and idempotent.

---

## ADR-015: Evolvable Silver Contract

| Property | Value |
| :--- | :--- |
| **Date** | 2026-04-05 |
| **Status** | Accepted |
| **Supersedes** | Extends ADR-002 (Silver Contract) |

### Context

The current `SilverRecord` is a flat, monolithic Pydantic model hardwired to one data shape. Every field — `title`, `raw_text`, `word_count`, `source`, `status`, `metric_value`, `timestamp` — lives in a single model shared by all sources. This creates three structural problems:

1. **Schema Rigidity:** Adding a new data source (e.g., RSS feeds, forum threads, social media posts) requires modifying the shared `SilverRecord`, risking regressions across the entire pipeline. Fields specific to one source pollute the contract for all others.
2. **Provenance Destruction:** The current processor overwrites `raw_text` with cleaned text during harmonization. The original text is lost at the Silver level, violating the principle that each Medallion layer should preserve the fidelity of the layer below.
3. **Layer Violation:** `metric_value` and `status` are embedded in the Silver model, but metric extraction belongs in the Gold layer and processing status belongs in PostgreSQL. Their presence in Silver conflates harmonization with analytics and lifecycle management.

AĒR is a research instrument, not a product. The Silver schema, Gold metrics, and analysis pipeline will undergo radical changes as interdisciplinary collaboration matures (Chapter 13, §13.6 Open Research Questions). The architecture must treat schema evolution as the normal case, not the exception.

### Decision

We split the Silver layer into two tiers and introduce a Source Adapter pattern:

**1. `SilverCore` — Universal Minimum Contract**

`SilverCore` defines the absolute minimum a document must have for *any* NLP pipeline to operate. These fields are instrumentally motivated (the pipeline needs them), not scientifically motivated (they don't represent analytical conclusions):

| Field | Type | Purpose |
| :--- | :--- | :--- |
| `document_id` | `str` | Deterministic SHA-256 hash of `source + bronze_object_key`. Enables idempotency checks without MinIO HEAD requests. |
| `source` | `str` | Origin identifier (e.g., `"wikipedia"`, `"tagesschau"`). |
| `source_type` | `str` | Discriminator for `SilverMeta` lookup (e.g., `"rss"`, `"forum"`, `"legacy"`). |
| `raw_text` | `str` | Original text as received from Bronze, unmodified. |
| `cleaned_text` | `str` | Whitespace-normalized text for downstream NLP processing. |
| `language` | `str` | ISO 639-1 code. Default `"und"` (undetermined). |
| `timestamp` | `datetime` | Deterministic event timestamp from MinIO event metadata. |
| `url` | `str` | Source URL for provenance and Progressive Disclosure. Default empty string. |
| `schema_version` | `int` | Starts at `2`. Version `1` represents the legacy `SilverRecord` format. |
| `word_count` | `int` | Word count of `cleaned_text`. Retained in Silver as a basic document descriptor. |

Fields removed from Silver: `metric_value` (belongs in Gold), `status` (belongs in PostgreSQL).

**2. `SilverMeta` — Source-Specific Context**

`SilverMeta` is a discriminated union that preserves source-specific richness without polluting the core. Each source type defines its own Pydantic model (e.g., `RssMeta` with `feed_url`, `author`, `categories`). The meta envelope is explicitly marked as **unstable** — source adapters may add, rename, or restructure meta fields without a formal ADR. Only `SilverCore` changes require an ADR.

**3. Source Adapter Protocol**

A `SourceAdapter` protocol defines a single method: `harmonize(raw: dict, event_time: datetime) -> tuple[SilverCore, SilverMeta | None]`. Each data source provides an adapter that maps its raw Bronze data to the universal `SilverCore` plus an optional source-specific `SilverMeta`. The processor looks up the adapter via a registry keyed by `source_type`. Unknown source types are routed to the DLQ.

A `LegacyAdapter` provides backward compatibility for existing Bronze objects (Wikipedia-era) that lack a `source_type` field. It maps old-format documents to `SilverCore` with `source_type = "legacy"` and `schema_version = 1`.

**4. Schema Evolution Strategy**

- New fields are added as `Optional` with defaults.
- Removed fields are deprecated (kept with a deprecation marker) for one release cycle, then dropped.
- The Silver bucket is append-only — existing objects are never re-processed to match a new schema version.
- The `schema_version` field enables the worker to handle multiple schema generations simultaneously.

**Both `SilverCore` and `SilverMeta` are provisional.** They represent the current best understanding of what the pipeline needs. As interdisciplinary research (Chapter 13) produces new requirements — new metadata fields, new normalization steps, new language-specific processing — the schema will evolve. The architecture supports this without pipeline-wide regressions.

### Consequences

* **Positive:** Adding a new data source requires only a new adapter and (optionally) a new `SilverMeta` model — zero changes to the processor, validator, or existing tests. Schema evolution is routine. Provenance is preserved (`raw_text` vs. `cleaned_text`). Layer responsibilities are clean (Silver = harmonization, Gold = analytics, PostgreSQL = lifecycle).
* **Negative:** The adapter registry adds one lookup per event (O(1) dict access — negligible). The `SilverCore` model is slightly larger than the old `SilverRecord` due to the addition of `document_id`, `cleaned_text`, `language`, `source_type`, and `schema_version`. The `LegacyAdapter` must be maintained until all pre-Phase 39 Bronze objects have expired from the 90-day ILM window.

---

## ADR-016: Hybrid Tier Architecture for Metric Validation (Option C)

| Property | Value |
| :--- | :--- |
| **Date** | 2026-04-11 |
| **Status** | Accepted |
| **Relates to** | WP-002 (Metric Validity and Sentiment Calibration) |

### Context

WP-002 §5 evaluates three architectural options for presenting metrics with varying levels of scientific validation:

- **Option A (Strict Gating):** Only display metrics that have passed the full five-step validation protocol. This blocks all current functionality until validation studies are complete.
- **Option B (Flat Display):** Display all metrics equally regardless of validation status. This risks misinterpretation of provisional results as validated measurements.
- **Option C (Hybrid Tier Architecture):** Classify metrics into tiers based on validation status and present them through Progressive Disclosure, ensuring transparency without blocking unvalidated work.

AĒR's Phase 42 extractors are engineering proof-of-concept implementations with known limitations (see the per-metric `known_limitations` field in `services/bff-api/configs/metric_provenance.yaml`, served by `GET /api/v1/metrics/{metricName}/provenance`; the underlying methodological discussion lives in WP-002 §3). Interdisciplinary validation requires external collaborators who are not yet engaged (Chapter 13, §13.5). Blocking metric display until validation is complete would render the system non-functional for an indeterminate period. Displaying all metrics without distinction would violate AĒR's commitment to methodological transparency (Manifesto §II).

### Decision

We adopt **Option C — Hybrid Tier Architecture** with the following structure:

**Tier 1 — Immutable Baseline.** All currently implemented metrics are classified as Tier 1. They are always displayed in the dashboard and are never hidden or replaced by higher-tier metrics. Tier 1 metrics include both deterministic metrics (word count, temporal distribution) and provisional NLP metrics (sentiment, NER, language detection). The "immutable" property means that once a metric is published as Tier 1, it remains visible — a Tier 2 or Tier 3 metric of the same phenomenon does not suppress the Tier 1 score.

**Tier 2 — Validated Enrichments.** Future metrics that have passed the five-step validation protocol (WP-002 §4) and are reproducible with a fixed seed. Available via Progressive Disclosure alongside (not instead of) the Tier 1 baseline. Examples: validated sentiment calibration, topic models with established inter-annotator agreement.

**Tier 3 — LLM-Augmented Enrichments.** Non-deterministic metrics produced by Large Language Models. Explicitly flagged as non-deterministic in the Gold schema. Available only through Progressive Disclosure and never displayed as primary metrics.

**Infrastructure:**

- The `aer_gold.metric_validity` ClickHouse table stores validation metadata per metric. Initially empty.
- The BFF API `GET /api/v1/metrics/available` endpoint exposes `validation_status` (`unvalidated`, `validated`, `expired`) per metric, derived from the validity table.
- The dashboard (future) will never hide the Tier 1 score behind a Tier 2/3 score — Progressive Disclosure adds information, it does not replace it.

### Consequences

* **Positive:** The system is immediately usable with current provisional metrics. Methodological transparency is maintained — consumers can distinguish validated from unvalidated metrics. The architecture supports incremental validation without requiring a system-wide freeze. The dashboard principle (never hide Tier 1 behind Tier 2/3) prevents the common pitfall of sophisticated models silently replacing simpler, more auditable baselines.
* **Negative:** Consumers must understand the tier system to correctly interpret results. The validation table is initially empty, meaning all current metrics report `unvalidated` — which is honest but may reduce perceived system maturity. The tier classification decision for each future metric requires interdisciplinary agreement, adding process overhead.

---

## ADR-017: Reflexive Architecture Principles

**Date:** 2026-04-12
**Status:** Accepted

### Context

WP-006 ("The Reflexive Architecture") argues that an observatory of discourse cannot be methodologically neutral: the choice of metric, visualization, visibility, and governance all encode normative commitments. The working paper proposes five design principles that any such system must satisfy if it is to remain reflexive about its own role in shaping what it claims to observe:

1. **Methodological Transparency** — every metric must expose its algorithmic provenance, tier classification, known limitations, and validation status at the point of consumption, not buried in documentation.
2. **Non-Prescriptive Visualization** — visual encodings must not smuggle in normative judgments (e.g., red/green for "bad/good"), must always show uncertainty alongside point estimates, and must offer multiple visualization modes to avoid privileging one interpretive frame.
3. **Reflexive Documentation** — each data source must link to its own bias profile and classification rationale so consumers can audit the instrument, not just its outputs.
4. **Governed Openness** — access to raw and aggregated data must follow a documented governance model, not an ad-hoc permission matrix.
5. **Interpretive Humility** — system surfaces must refuse to answer questions the underlying data cannot support, even when a numeric answer is technically derivable.

AĒR is currently a headless pipeline with a BFF API and no dashboard. Principles 2, 4, and 5 are primarily frontend and policy concerns; principles 1 and 3 have immediate consequences for the backend contract.

### Decision

We adopt the five WP-006 principles as an architectural commitment. Their status is split by phase boundary:

**Implemented in Phase 67:**

- **Principle 1 (Methodological Transparency)** is implemented via `GET /api/v1/metrics/{metricName}/provenance`, which returns tier classification, algorithm description, known limitations, validation status, extractor version hash, and cultural context notes. Static fields are sourced from `services/bff-api/configs/metric_provenance.yaml`; dynamic fields are resolved at query time against `aer_gold.metric_validity` and `aer_gold.metric_equivalence`.
- **Principle 3 (Reflexive Documentation)** is implemented via the PostgreSQL `sources.documentation_url` column (migration 000007) and a corresponding `GET /api/v1/sources` endpoint that surfaces the URL. Phase 70 (migration 000008) repointed Probe 0 from a single bias-profile file to the probe dossier directory `docs/probes/probe-0-de-institutional-rss/` — see Arc42 §8.15 for the Probe Dossier Pattern.

**Deferred to the dashboard phase:**

- **Principle 2 (Non-Prescriptive Visualization)** is captured as requirements in `docs/design/visualization_guidelines.md`. These constrain future frontend work (viridis color scale, no red/green encoding, no normative labels, uncertainty alongside point estimates, multiple visualization modes).
- **Principle 4 (Governed Openness)** is deferred until a governance model is drafted alongside the first public deployment plan. Until then, access is controlled by the existing static API key (ADR-011), and the constraint is documented, not enforced.
- **Principle 5 (Interpretive Humility)** is partially prefigured by the validation-gate pattern from ADR-016 (the BFF returns HTTP 400 when z-score normalization is requested without a registered equivalence entry). Full implementation — including query-time refusal for insufficiently-supported questions — is deferred until the dashboard introduces interpretive surfaces.

### Consequences

* **Positive:** The backend contract now exposes the methodological provenance required for scientifically-literate consumption of metrics, and source-level bias documentation is addressable by URL rather than being hidden in Git. The split between implemented and deferred principles is explicit — future frontend work has a concrete spec in `docs/design/visualization_guidelines.md` rather than a vague commitment.
* **Negative:** The static provenance config (`metric_provenance.yaml`) must be kept in sync with the extractor registry by hand; the handler test verifies that every registered metric has an entry, but additions to the extractor registry still require a deliberate documentation step. Three of the five principles remain commitments without enforcement — in principle, a future frontend could violate them without the backend noticing.

---

## ADR-018: Constant-Time API-Key Comparison

**Date:** 2026-04-13
**Status:** Accepted

### Context

The BFF API and the Ingestion API both authenticate callers with a static API key (ADR-011) compared against an expected value held in process memory. Prior to Phase 75 the comparison used `==` on byte slices, whose runtime cost depends on how many leading bytes match before the first mismatch. Over a sufficient number of probes an attacker can measure this difference and reconstruct the key byte by byte — a classical timing side channel against equality comparison on secrets.

The guarantee required here is narrow: the comparison function itself must not leak information about the candidate through its observable runtime. Defences like rate limiting (R-1 resolution) reduce how fast an attacker can probe, but they do not remove the underlying signal — they only make it slower to exploit.

### Decision

The shared API-key middleware in `pkg/middleware/apikey.go` compares the candidate against the expected key via `crypto/subtle.ConstantTimeCompare`. `subtle.ConstantTimeCompare` runs in time dependent only on the length of its inputs (which are of fixed length here), not on where the first differing byte sits. Both the BFF API and the Ingestion API share this middleware through the workspace-linked `pkg/` module, so the guarantee holds across both internet-facing entry points without duplicating the comparison logic.

The unauthorized response path was also tightened in the same patch: the middleware now sets `Content-Type: application/json` before writing the 401 body, rather than using `http.Error` (which forces `text/plain`). This is unrelated to the timing guarantee and is documented here only because it lives in the same function.

**Non-goals.** This ADR does not address key rotation, key revocation, per-caller keys, or a rate-limiting strategy. Those concerns are tracked separately (ADR-011 for the single-key model, R-2 for plaintext `.env` storage, the existing BFF rate limiter for probe velocity).

### Consequences

* **Positive:** The equality check on the auth secret is free of the dominant timing signal that makes naive `==` comparisons exploitable. A sanity test (`TestAPIKeyAuth_UsesConstantTimeCompare` in `pkg/middleware/apikey_test.go`) verifies that a wrong key produces the same 401 outcome regardless of how many leading bytes match, and that `subtle.ConstantTimeCompare` is the function under test. Because the middleware is shared, both services inherit the fix from the same source file.
* **Negative:** `subtle.ConstantTimeCompare` returns `int` rather than `bool` and demands exactly-length inputs — a minor ergonomic cost at every call site. The constant-time guarantee only applies to the comparison; timing signals arising from surrounding work (header parsing, bearer-token extraction, response serialization) are not addressed and are considered acceptable under the current threat model.

---

## ADR-019: IaC-Only NATS JetStream Stream Provisioning

**Date:** 2026-04-13
**Status:** Accepted

### Context

Hard Rule 5 of the project charter forbids services from creating infrastructure at startup — buckets, tables, streams, and schemas must be provisioned by dedicated init containers and migration scripts. Before Phase 73, the analysis worker violated this rule for its NATS JetStream consumer: on startup it called `js.add_stream(...)` to idempotently declare the `AER_LAKE` stream if it did not already exist. The call was implemented as a safety net — "the worker must not crash against a missing stream in a fresh environment" — but in practice it meant the worker held implicit write authority over stream configuration, and the real stream definition lived partly in Python code, partly in whatever NATS container had been started first.

This is the same class of drift as a service that auto-creates its own Postgres tables. It produces three concrete failure modes: (1) the stream config in code and the config in production can silently diverge, since no migration ever runs against an existing stream; (2) a misconfigured worker with the wrong retention or subject filter can "fix" the stream for every other consumer; (3) environment bootstrap order stops being deterministic — whichever service happens to come up first defines the stream.

### Decision

NATS JetStream streams are provisioned exclusively by the `nats-init` init container from versioned JSON stream definitions under `infra/nats/streams/`. The analysis worker's `js.add_stream(...)` call was removed in Phase 73, and the worker now only *subscribes* to the stream — it assumes the stream exists and fails fast if it does not, exactly as it would against a missing PostgreSQL table or a missing MinIO bucket. The sole authoritative definition of the `AER_LAKE` stream is `infra/nats/streams/AER_LAKE.json`, which records subjects, retention, storage, replica count, and deduplication window.

This mirrors how `minio-init` and `clickhouse-init` already work for their respective infrastructures (ADR-014 for ClickHouse migrations). With the worker demoted to a pure consumer, Hard Rule 5 is uniformly enforced across all three storage backends.

**Non-goals.** This ADR does not introduce a migration framework for stream schema changes — changes today are applied by editing the JSON file and restarting `nats-init`. A proper migration story is only worth building once the first backwards-incompatible stream change lands, which has not happened yet.

**Single-node defaults.** `infra/nats/streams/AER_LAKE.json` ships with `num_replicas: 1` and `max_age: 0` (unbounded retention). These are correct for a single-node development or production deployment — JetStream cannot replicate beyond the number of cluster nodes, and unbounded retention is acceptable when bronze-layer ILM (MinIO) is the authoritative data lifecycle policy. **A multi-node deployment must raise `num_replicas` to an odd quorum-safe value (typically 3) and set a bounded `max_age` aligned with the bronze bucket's 90-day TTL** so that NATS does not become an implicit second system of record. Both values live in the versioned stream JSON and are applied idempotently by `nats-init` on every bring-up; no code change is required to flip a single-node dev stack into a clustered production stack.

### Consequences

* **Positive:** The stream definition is now a single, versioned, diff-able file in the repository, owned by infrastructure rather than by any service. A misconfigured worker can no longer corrupt stream configuration for other consumers. Bootstrap order is deterministic: `nats-init` runs, the stream exists, services that depend on it start. The Hard Rule 5 surface is uniform — three storage backends, three init containers, zero services with implicit provisioning authority. See also CLAUDE.md Hard Rule 5 and §8.4 (Infrastructure as Code) in Chapter 8.
* **Negative:** Fresh development environments now require `nats-init` to have run before the worker can start; a developer who bypasses the normal compose bring-up and starts the worker directly against an empty NATS instance will see a hard failure instead of silent self-repair. This is the intended trade-off — fail loudly in the wrong place rather than quietly mask a missing provisioning step.

---

## ADR-020: Frontend Technology Stack for the AĒR Dashboard

**Status:** Accepted
**Date:** 2026-04-17
**Supersedes:** None
**Superseded by:** None
**Related ADRs:** ADR-003 (Progressive Disclosure), ADR-008 (Zero-Trust Networking), ADR-011 (Static API Key), ADR-013, ADR-014 (Database Migration Strategy), ADR-015 (Evolvable Silver Contract), ADR-016 (Hybrid Tier Architecture), ADR-017 (Reflexive Architecture), ADR-018 (Constant-Time API-Key Comparison), ADR-019 (IaC-only NATS Stream Provisioning)
**Related WPs:** WP-001, WP-002, WP-003, WP-004, WP-005, WP-006
**Authority:** This ADR is written *against* the Design Brief (`docs/design/design_brief.md`). Every decision below must demonstrate compliance with the brief's §3 (Navigation is First-Class), §4 (Three Surfaces), §5 (Five Layers), §6 (Terminology — Path B), §7.6 (Fidelity Modes), §7.7 (Progressive Semantics), §7.8 (Epistemic Weight), §7.9 (Visualization Stack Separation), §8 (Extensibility), §9 (Silver-Layer Access), and §10 (Performance).

**Iteration 5 addendum (2026-04-25):** The Design Brief was substantially rewritten following the 2026-04-24 Reframing Note. The technology choices below remain validated by the reframing — if anything the reframing leans harder on the non-globe visualization domains. This ADR has been updated in three places: (a) the Compliance check audits the Iteration 5 brief; (b) the Implementation Outline reorders so Surface II and Surface III precede further Surface I deepening; (c) a new Backend Work section enumerates the endpoint, schema, and query work the reframing implies.

---

### Context

The AĒR backend has reached Phase 93 with a stable Bronze → Silver → Gold pipeline, a BFF API with validation gates (ADR-016) and security hardening (Phase 75), full OpenTelemetry observability (Phase 86), supply-chain hardening with SBOM and image signing (Phase 84), and six Working Papers defining the methodological scope. The frontend phase now begins.

The Design Brief specifies **what** the dashboard must be: three surfaces, five layers of progressive descent, fractal cultural granularity, dual-register communication, epistemic weight visualisation, and a visualisation stack separated across four domains. The brief deliberately defers **how** these requirements are technically realised — that is this ADR.

Four clusters of technical constraint emerge from the brief:

**Cluster A — 3D atmosphere with a low-fidelity twin.** Surface I requires a WebGL2-based rotating globe with custom shaders (terminator, probe glow, absence fields, atmospheric scattering). The same informational surface must render in a 2D fallback mode on ten-year-old hardware without scientific loss of depth.

**Cluster B — Progressive Semantics with both registers in the DOM.** Every data point carries semantic and methodological registers simultaneously. Transitions between registers are local micro-interactions, not page navigations. Content is sourced from a BFF-served content catalog, not hardcoded.

**Cluster C — Performance on two hardware classes.** High-fidelity targets a 2021 M1 MacBook Air at 60fps; low-fidelity targets a 2015 ThinkPad on 5 Mbps. First paint budgets: <1.5s high-fi, <3s low-fi. The total initial bundle budget is approximately 180 kB gzipped.

**Cluster D — Contract-first development experience.** The frontend must consume `services/bff-api/api/openapi.yaml` via generated TypeScript types, with CI drift detection, matching the Go backend's `oapi-codegen` pattern (Phase 9).

Zero-Trust (ADR-008) forbids a Node runtime exposed in production for the dashboard; the frontend deploys as static assets behind Traefik. The static-API-key problem (ADR-011, hardened via constant-time comparison in ADR-018) is handled at the ingress layer — the frontend never holds credentials in-browser.

Three additional project-wide invariants shape the stack:

* **Observability parity (Phase 86).** The frontend must emit OpenTelemetry traces that propagate into the existing collector stack (OTel Collector → Tempo). A user session becomes a trace span that links into BFF and worker traces.
* **Supply-chain hardening (Phase 84).** The frontend build must produce SBOMs and signed container images on par with the Go services. No transient build dependencies without lockfile pinning.
* **Bilingual documentation (ongoing).** All user-facing strings, Dual-Register content, and error messages exist in EN and DE, sourced from the new content catalog, not hardcoded.

---

### Decision

We adopt the following frontend technology stack, organised in five layers.

#### Layer 1 — UI Framework: SvelteKit (static adapter) with Svelte 5 Runes

**SvelteKit** in static-output mode is the chrome framework. **Svelte 5 Runes** (`$state`, `$derived`, `$effect`) are the reactivity primitives.

Rationale:

* **Bundle footprint.** Svelte compiles framework code away. The shell (SvelteKit router, Svelte runtime, TanStack Query Svelte adapter, initial UI components) compiles to approximately 40–60 kB gzipped. This leaves approximately 120 kB of budget headroom within the 180 kB initial target of the Design Brief §7. React 19 + TanStack Router + a state library + TanStack Query baseline at approximately 75 kB gzipped before the first application component — leaving the budget under continuous stress.
* **Single reactivity model.** Svelte 5 Runes express the dashboard's state topology (URL state + server state + user interactions + 3D engine events) in one mental model. Alternative frameworks require two or three separate libraries (Jotai + TanStack Query + URL hooks) with different mental models — an accidental cost for a single-developer project.
* **View Transitions API as a first-class citizen.** SvelteKit's `onNavigate` hook integrates the View Transitions API for surface transitions without custom animation code. This directly serves the Design Brief's requirement for seamless surface changes without visible re-assembly.
* **Build-time accessibility linting.** Svelte's compiler reports accessibility violations (missing alt text, wrong ARIA roles, keyboard traps) at build time. WCAG 2.2 AA conformance (Design Brief §5.6, §9) is a baseline assumption; compiler-enforced checks make that baseline enforceable in a single-developer setting.
* **Static output compatibility.** `@sveltejs/adapter-static` produces pure HTML/JS/CSS deployable behind Traefik with no runtime. ADR-008 Zero-Trust compatibility is maintained without special provisions.
* **2026 maturity.** Svelte 5 has been stable since Q4 2024. Production adopters include The New York Times, Apple Music, 1Password, and parts of Spotify's stack. The bus factor is acceptable: the core team is employed by Vercel.

Rejected alternatives:

* **React 19 + Vite + TanStack Router.** The largest ecosystem, best-in-class TypeScript-driven routing, perfect community bus factor. Rejected because the baseline bundle size pressures the low-fidelity budget, and because the multi-library state management pattern (Jotai + TanStack Query + separate URL state) introduces accidental complexity for a single developer. React remains the correct choice for teams with existing React expertise or enterprise-ecosystem dependencies — neither applies to AĒR.
* **Solid.js + SolidStart.** Technically elegant, smallest runtime, signals model is the Solid pioneer. Rejected because the surrounding ecosystem (forms, accessibility primitives, static-output adopters) is thinner than Svelte's. A five-year solo maintenance horizon requires a community large enough to absorb library deprecation, CVE patches, and unfamiliar errors. Svelte is on the right side of that line; Solid is borderline.
* **Astro + Islands architecture.** Compelling for prose-first Surface III but rejected because Surfaces I and II require cross-surface state coordination (active probe, time range, pillar mode) that the islands pattern systematically fragments. The impedance mismatch outweighs the partial fit.
* **Qwik.** Resumability optimises for cold-start time after network transit. AĒR is an analyst tool with long sessions — cold-start optimisation is not a dominant concern. Ecosystem adoption is thinner than the other candidates.

#### Layer 2 — 3D Atmosphere Engine: vanilla three.js with custom GLSL shaders

> The globe, terminator, probe glow, probe reach auras, absence fields, atmospheric halo, and a reserved slot for Rhizome propagation arcs are rendered by a **vanilla three.js** engine module. The module is framework-agnostic and receives data through a narrow imperative API.

Rationale:

* **Shader control.** Terminator rendering (day/night boundary) is a fragment-shader problem: a per-pixel dot-product between the sun-direction vector and the surface normal. Probe glow requires a bloom-and-scatter shader with a bounded pulse driven by activity density. Probe reach auras require a translucent volumetric field sampled from per-probe geometry (not a hardcoded circle). Absence fields require custom blending. Rayleigh/Mie atmospheric scattering follows well-documented shader patterns. The reserved propagation slot uses a curve-along-great-circle shader that is wired into the engine from day one but consumes no fragment cycles while its data input is empty. Declarative framework wrappers (react-three-fiber, Threlte, Solid-Three) add abstraction overhead that impairs shader-level control at the 60fps target.
* **Framework-agnostic module.** Per Design Brief §5.9, the engine module receives data and produces WebGL output. It does not know the UI framework. This decouples the engine's quality from the framework choice and allows future framework migration without rewriting the engine.
* **Tree-shakeability.** three.js is large (~650 kB minified) but modular. Only the classes actually imported enter the bundle — for AĒR's expected scope, approximately 80 kB gzipped when the full engine is built. The engine module lazy-loads after the shell: first paint renders the low-fidelity 2D atmosphere, and the WebGL upgrade streams in without blocking.
* **Vector-first landmass rendering.** The globe surface is built from Natural Earth 1:50m land polygons triangulated at build time, not from a satellite-imagery texture. This keeps landmasses crisp under the country-level zoom that later phases will require, removes a raster asset stream-in path from the first paint, and aligns with Design Brief §3.1 ("restrained silhouettes ... no satellite imagery"). Country borders are an optional, lazy-loaded line mesh (default off) so the atmospheric register dominates by default.
* **Maturity and community size.** three.js has been maintained since 2010 and is the default choice for non-game WebGL work. The bus factor is excellent; shader documentation is abundant.

Rejected alternatives:

* **deck.gl alone.** Strong for geospatial point rendering but weaker for the custom shader effects (terminator physics, atmospheric halo, absence fields) the Design Brief demands at Layer 0. deck.gl's strengths belong to Layer 3 geo-analytics (see Layer 3 below), not to the atmospheric surface.
* **regl (functional WebGL wrapper).** Minimal and elegant, but reinvents primitives three.js already provides (camera controls, loaders, picking). The bundle saving is modest (~30 kB); the engineering cost is not.
* **react-three-fiber / Threlte.** Excellent developer experience for common 3D patterns, but adds abstraction between the code and the shader pipeline. Design Brief §5.9 explicitly excludes framework-coupled 3D wrappers for AĒR.

#### Layer 3 — Visualisation Domains (per Design Brief §5.9)

Each domain is served by a framework-agnostic rendering module:

| Domain | Library | Bundle (gzipped, approx.) | Load trigger |
|---|---|---|---|
| 3D Atmosphere | three.js (vanilla) + custom GLSL | ~80 kB engine + ~20 kB shaders | Surface I, after shell |
| 2D Geo-Analytics | MapLibre GL JS + deck.gl | ~180 kB MapLibre + ~90 kB deck.gl | Surface I/II, Layer 3, on intent |
| Scientific Charts | Observable Plot + uPlot + D3 (utility) | ~90 kB Plot + ~40 kB uPlot + ~30 kB used D3 modules | Surface II/III, Layer 3, on intent |
| Relational Networks | D3-force (2D) + three.js (3D, reused) | ~20 kB D3-force; three.js already loaded | Surface II, Layer 3, Rhizome pillar mode |

Rationale:

* **MapLibre GL JS + deck.gl** is the open-source state-of-the-art stack for interactive geospatial data visualisation in 2026. Uber, CARTO, Foursquare, and the open-source GIS community use this combination. Non-commercial licensing matches AĒR's self-hosted deployment model.
* **Observable Plot** provides 2026's state-of-the-art declarative grammar for scientific diagrams (violin plots, ridgeline plots, box plots, heatmaps, streamgraphs, small multiples). It is built by Mike Bostock (creator of D3) and designed for scientific communication. Tree-shakeable.
* **uPlot** is unmatched for dense time series (>1k points). Design Brief §7 requires handling the BFF's downsampled row ceiling (Phase 13 + Phase 78) at Layer 3 without jank.
* **D3** is used as a utility library for custom visualisations Plot cannot express — Sankey diagrams, chord diagrams, force-directed layouts, Rhizome propagation graphs. Only the D3 modules actually imported enter the bundle (~30 kB typical).
* **D3-force + three.js** for relational networks: 2D force-directed graphs use D3-force's deterministic layout; 3D network rendering (Rhizome propagation in spatial context) reuses the already-loaded three.js engine from Layer 2.

Rejected alternatives:

* **Plotly.js, Apache ECharts, Highcharts.** Comprehensive chart libraries but monolithic (300–500 kB gzipped). Violates §7 and §5.9 (framework-agnostic but not modular).
* **Recharts, Nivo, Chakra-UI charts, Mantine charts.** React-specific. Explicitly excluded by §5.9.
* **Mapbox GL JS.** Technically capable but its 2020 licence change ties deployment to a commercial service. Violates the open-source deployment model.
* **Cytoscape.js for networks.** Capable but larger than D3-force for the network sizes AĒR will render; adds a second graph paradigm without clear gain.

#### Layer 4 — Cross-Cutting Infrastructure

**Build tool.** Vite 6 (via SvelteKit). Dev server with HMR; production build with Rollup. Rollup's bundle analyser is part of CI output for budget enforcement.

**TypeScript.** Strict mode (`strict: true`, `noUncheckedIndexedAccess: true`, `exactOptionalPropertyTypes: true`). No `any` without a `// TODO(aer-NNN):` comment referencing a tracked issue. No `@ts-ignore` — only `@ts-expect-error` with justification.

**API client generation.** `openapi-typescript` generates types from `services/bff-api/api/openapi.yaml`. A `make codegen-ts` target regenerates types; CI runs `make codegen-ts` then `git diff --exit-code`, mirroring the Go backend's `oapi-codegen` pattern from Phase 9. Drift in generated types fails the build.

**Content catalog consumption.** The frontend consumes Dual-Register content (§5.7) via new BFF endpoints: `GET /api/v1/content/{entityType}/{entityId}?locale={de|en}`. Response includes both registers (short and long variants), content version, and review metadata. Backend implementation is a separate phase (see Implementation Plan below); frontend implementation uses a local JSON fallback until the backend endpoint ships, such that the switch is a one-line change in the content layer.

**Observability (Phase 86 parity).** The frontend emits OpenTelemetry traces via `@opentelemetry/sdk-trace-web` and `@opentelemetry/instrumentation-fetch`. Trace context (`traceparent` header) is propagated into BFF requests; a user session is a root span that links to BFF and worker spans in Grafana Tempo. Resource attributes include `service.name=aer-dashboard`, `service.version` (from Git tag), and `deployment.environment`. Metrics — Web Vitals (LCP, INP, CLS), JS errors, route-change timings — emit to the existing Prometheus pipeline via the OTel Collector.

**Supply chain (Phase 84 parity).** The production container image (Nginx serving static assets) is built reproducibly with pinned base image digests, scanned with Trivy in CI, signed with Cosign, and an SBOM is generated with Syft. `pnpm` enforces a strict lockfile; `pnpm audit --audit-level high` in CI.

**Testing.** Vitest for unit tests (co-located with source). Playwright for end-to-end tests across Chromium, Firefox, and WebKit. Visual-regression snapshots for the atmosphere surface, Layer 0–3 transitions, and at least one chart per visualisation domain. Per §5.9 consequence 4, each visualisation module has a Storybook-style isolated test environment (via **Histoire**, Svelte-native, lighter than Storybook) that runs without the framework shell.

**Package manager.** pnpm with strict lockfile. Workspace structure: single package initially; split into workspaces (`apps/dashboard`, `packages/engine-3d`, `packages/viz-charts`) if and when framework-agnostic modules grow large enough to warrant separate versioning.

**Linting and formatting.** ESLint with the official Svelte plugin and `@typescript-eslint`. Prettier with Svelte plugin. A single `make fe-check` target runs lint + typecheck + test + bundle-size gate, matching the Go backend's `make lint` and `make test` ergonomics.

#### Layer 5 — Architecture Patterns

The frontend is organised around five conceptual modules, consistent with the Design Brief's separation of chrome and visualisation:

1. **Shell.** Routes, layouts, navigation, theming, global state, OTel instrumentation. SvelteKit + Svelte 5.
2. **Data Layer.** API client (from openapi-typescript), TanStack Query cache, URL state synchronisation, refusal handling, trace context propagation. Framework-bound but thin.
3. **Surface Modules.** Three top-level surface implementations (Atmosphere, Function Lanes, Reflection). Each is a SvelteKit route subtree with its own code-split boundary.
4. **Visualisation Modules.** Framework-agnostic rendering modules per §5.9 domain. Imported by surface modules via thin adapter components. Each has its own test harness.
5. **Content Layer.** Dual-Register content fetching, caching, and locale resolution. Thin wrapper over the new BFF content endpoints with graceful fallback during the phase before the backend lands.

Cross-module communication is explicit: shell → surface → visualisation is a one-way data flow with events propagated upward. No shared mutable state between visualisation modules and the shell beyond what the adapter exposes.

---

### Consequences

#### Positive consequences

* **Bundle budget is comfortable, not tight.** Initial shell at ~60 kB leaves approximately 120 kB of headroom for progressive enhancement of Surface I before code-splitting thresholds are hit.
* **Framework choice is reversible.** The decision to keep visualisation modules framework-agnostic (§5.9) means a future migration from Svelte to something newer costs only the chrome rewrite — perhaps 30% of the codebase — not the hard engineering in the 3D engine, the charts, and the maps.
* **Accessibility is enforceable.** Svelte's compile-time A11Y linter catches the common failures before they ship. This is significant for a single-developer project where manual accessibility review is unreliable.
* **Development experience matches the backend.** Contract-first with generated types, CI drift detection, and a `make fe-check` target parallel to `make lint` / `make test` creates symmetry between the Go backend and TypeScript frontend.
* **Observability is end-to-end.** With OTel Web SDK emitting into the Phase 86 collector stack, a single trace can follow a user click through BFF queries, ClickHouse, and back — the same dashboard used to debug worker issues works for the frontend.
* **Performance budgets are achievable by construction.** Every budget in Design Brief §7 maps to a concrete measurement: initial bundle, surface-level chunk, visualisation-module chunk. Each can be benchmarked in isolation.

#### Negative consequences and accepted risks

* **Svelte ecosystem smaller than React.** Specific libraries (complex date pickers, enterprise form systems) may be unavailable in Svelte-native form. Mitigation: the visualisation-stack separation (§5.9) ensures that scientific and data-viz libraries are framework-agnostic; only chrome-layer libraries are framework-bound, and the chrome-layer requirements are modest. Accepted risk.
* **Svelte 5 Runes are recent (stable since late 2024).** Some tutorials still reference the older store-based Svelte 3/4 API. Mitigation: the official documentation is excellent and current; LLM-based coding assistants are Runes-aware as of 2026. Accepted risk.
* **three.js without a declarative wrapper means more imperative engine code.** The engine module will be larger in line count than an r3f equivalent. Mitigation: this cost is a direct consequence of §5.9 and is accepted as part of the framework-agnosticism commitment. The engine module is tested in isolation per §5.9 consequence 4.
* **MapLibre GL JS is a heavy dependency (~180 kB gzipped).** It loads only when geo-analytics views activate, but when it loads, it dominates that chunk. Mitigation: intent-based preload (hover on "geo-analytics" affordance begins the download before the user clicks). Accepted cost.
* **The content catalog requires backend work before the frontend can fully consume it.** The new BFF endpoints must complete before Dual-Register content flows end-to-end. Mitigation: until that phase completes, the frontend uses placeholder content from a local JSON file; switching to the BFF endpoint is a one-line change in the content layer. Deferred risk.
* **Low-fidelity 2D atmosphere must be specified and implemented separately.** The 2D fallback is not automatic — it is a deliberately constructed equirectangular map with terminator and probe markers rendered via Canvas 2D or SVG. This is additional work, but §5.6 requires it regardless of framework choice. Accepted cost.
* **Frontend OTel emission adds bundle weight (~15 kB gzipped).** The Web SDK is not free. Mitigation: lazy-load the OTel instrumentation after first paint so it does not block initial rendering. Accepted cost — observability parity with backend is an architectural invariant.

#### Neutral consequences

* The dashboard deploys as a static bundle behind Traefik. No change to ADR-008 Zero-Trust posture.
* The dashboard is a pure BFF consumer. No new backend responsibilities except the content catalog endpoints.
* The dashboard honours existing API contracts (ADR-016 normalisation gate; ADR-017 validation status propagation). Refusal handling is a frontend responsibility; the API semantics are unchanged.
* The single-container production image follows the Phase 84 supply-chain pattern: pinned base image, Trivy-scanned, Cosign-signed, SBOM-attached.

---

### Compliance check against the Design Brief (Iteration 5)

The ADR is valid only if the selected stack satisfies every requirement in the Design Brief. This section audits that compliance explicitly against the Iteration 5 rewrite.

| Brief requirement | ADR compliance |
|---|---|
| §3 — Navigation is first-class (left rail + top scope bar + right methodology tray) | SvelteKit nested layouts render the left rail and top scope bar as persistent chrome in the root layout. The methodology tray is a layout-level component with push-mode by default (CSS grid resize on tray open) and overlay-mode fallback at narrow viewports. Scope state is URL-backed and carried across surface transitions. No surface or layer can render without the chrome present. |
| §3.3 — Methodology tray always reachable, binds to active metric | The tray is a framework-bound component (SvelteKit) that subscribes to a `focusedMetric` store. Any chart/lane/probe click updates the store; the tray content updates in place. Tier badges read live from `/api/v1/metrics/{metricName}/provenance`. The Reflection anchor is a SvelteKit `<a href>` to the Surface III route scoped to the metric. |
| §4 — Three surfaces with redistributed priority (I overview, II primary scientific, III primary methodological) | Three SvelteKit route subtrees (`/atmosphere`, `/lanes`, `/reflection`) with shared layout. Code-splitting at the surface boundary. Surface I disappears during descent by route transition (not overlay); the planet glyph in the left rail returns to `/atmosphere` preserving scope via URL state. |
| §4.1 — Probe-first emission on the globe with source satellites | The 3D engine (`packages/engine-3d/`) emits one glyph per probe from `/api/v1/probes`. Source satellites are rendered as secondary geometry from the probe's dossier data; raycaster filters select only probe glyphs for scope changes. Satellite clicks open the Probe Dossier with the source pre-filtered via URL param. |
| §4.2 — Probe Dossier as Surface II landing; function lanes; view-mode matrix | Surface II's default landing route is `/lanes/:probeId/dossier`. Function lanes are a `/lanes/:probeId/:functionKey` subtree. The view-mode matrix is driven by a backend-served catalog (see Backend Work §) consumed through a typed matrix-cell registry; the frontend has no hardcoded cell list. |
| §4.2.4 — probeId and sourceId both first-class scope parameters | All view-mode endpoints accept either `probeId` or `sourceId`; the frontend's URL state carries both. The scope indicator in the left rail shows which is active. |
| §4.3 — Reflection as primary methodological surface | Surface III is a SvelteKit route with MDX-style prose rendering + inline interactive Observable Plot cells. Working Papers are authored as MD files under `services/dashboard/content/papers/` and rendered client-side; the Open Research Questions hub is a dedicated route `/reflection/open-questions`. The "How to read the globe" primer is `/reflection/primer/globe`. |
| §4.5 — Probe concept introduced in context | Probe glyph components implement Progressive Semantics natively: semantic register is the default label, methodological register expands on hover/click. The primer route is linked from the top scope bar of Surface I. First-visit overlay is a Phase-gated enhancement. |
| §5 — Redistributed Surface × Layer matrix | Route structure mirrors the matrix. Layers are not separate routes; they are states within a surface route governed by URL params (e.g. `?layer=3&metric=sentiment_score`). The methodology tray (L4) is a component, not a route; it is open/closed per URL param. L5 Evidence is a modal-overlay component reachable from Surfaces II and III. |
| §5.5 — Fractal cultural granularity, probe-defined | Granularity is read from `/api/v1/probes/{id}/dossier` at runtime. Frontend has no hardcoded cultural hierarchy. k-anonymity gate at L5 honoured via BFF-returned error on under-k queries. |
| §6 — Terminology (Path B) | Engineering usage preserved. API surface uses `probeId`/`sourceId`; PostgreSQL schema unchanged. Brief's §6 documents the WP-001 drift; no code change required. Reconciliation deferred to a post-Iteration-5 ADR. |
| §7.6 — High-Fi / Low-Fi modes with shared data layer | Shared TanStack Query cache. 3D engine (high-fi) and 2D Canvas renderer (low-fi) consume the same normalised data models. Auto-detection at startup; user override persisted in URL. |
| §7.7 — Progressive Semantics with both registers in DOM | Content layer prefetches both registers on metric hover. ARIA-Expanded pattern for register transitions. Both registers present in DOM; CSS and ARIA control prominence. |
| §7.8 — Epistemic Weight from live validation status | Rendering modules receive `validationStatus` from `/api/v1/metrics/available` and apply Weight treatments from a shared style token map. No frontend constant table. |
| §7.9 — Four visualisation domains, framework-agnostic | All four domain libraries selected are framework-agnostic. Chrome (SvelteKit) wraps but does not penetrate them. Relational Networks domain now has a concrete role articulated in the brief (entity co-occurrence on Surface II, cross-source propagation under Rhizome, exploratory composition reserved). |
| §8 — Extensibility by construction | Metric-agnostic rendering; view-mode matrix catalog-driven; no hardcoded discourse-function, pillar, metric, tier, resolution, or view-mode enums anywhere. |
| §9 — Silver-Layer Access with eligibility flag | New Silver query endpoints on the BFF, gated by a `silver_eligible` column on the `sources` table with review metadata. Surface II exposes a data-source toggle; non-eligible sources render an explicit "not Silver-eligible" state with methodological context. See Backend Work below. |
| §10 — Performance budgets | Shell ~60 kB; per-surface chunks within budget; three.js engine lazy-loaded; progressive enhancement from low-fi to high-fi. All four frame budgets and initial-load budgets achievable. Bundle-size gate enforced in CI. |
| §11 — API key handling | Static bundle behind Traefik; key injected server-side (ADR-018 constant-time compare enforces correct handling at the BFF); no browser credentials. |
| `visualization_guidelines.md` | Colour palettes (viridis), uncertainty display, refusal honouring — all handled at the visualisation-module level and enforced by code review, not framework choice. |
| Arc42 §8.14 Reflexive Architecture (ADR-017) | Surface III renders methodological-transparency content; refusal surfaces realise Principle 5 (Interpretive Humility); the always-visible methodology tray (§3.3) operationalizes the reflexive commitment at the interaction level. |
| Phase 84 Supply-Chain Hardening | Frontend image built with pinned base digest, Trivy-scanned, Cosign-signed, SBOM-attached. |
| Phase 86 Observability Wiring | Frontend emits OTel traces into the existing collector → Tempo pipeline. Trace context propagates to BFF. |

---

### Backend Work Implied by Iteration 5

The Iteration 5 brief pressures the BFF (and adjacent backend layers) in several specific ways beyond what earlier iterations anticipated. This section enumerates the scope deltas. The BFF is the correct layer for most of this — it is the query-shaping and authorization surface between the dashboard and Gold/Silver storage. A few items touch PostgreSQL (schema migrations), ClickHouse (query endpoints / views), or the analysis worker (one corpus-level aggregation).

#### Probe Dossier and article browsing

New or expanded endpoints to support Surface II's Probe Dossier (§4.2.1) and L5 Evidence (§5):

- `GET /api/v1/probes/{id}/dossier` — composite payload: probe classification, emic context, source list with per-source article counts and publication frequency, function coverage indicator. Replaces or subsumes the smaller per-resource endpoints from earlier ADR drafts.
- `GET /api/v1/sources/{id}/articles?start=…&end=…&language=…&entityMatch=…&sentimentBand=…&limit=…&cursor=…` — paginated article list per source with filters. Returns article IDs with light metadata for listing.
- `GET /api/v1/articles/{id}` — individual article detail. Reads cleaned text from Silver (or raw from Bronze if the source is Silver-eligible) and provenance metadata from ClickHouse Gold. Subject to a k-anonymity gate: the BFF verifies the article is part of an aggregation of at least *k* = 10 documents for the relevant metric before returning; otherwise HTTP 403 with a methodological refusal payload.

#### Silver-Layer access (§9 of the brief)

New schema, new endpoints, new governance:

- **Schema migration.** Add to `public.sources`:
  - `silver_eligible` (BOOLEAN NOT NULL DEFAULT false) — the flag itself.
  - `silver_review_reviewer` (VARCHAR) — reviewer identifier.
  - `silver_review_date` (DATE) — when the review was completed.
  - `silver_review_rationale` (TEXT) — free-text rationale.
  - `silver_review_reference` (VARCHAR) — link to a review document under `docs/governance/` or similar.

  Seed migration: Probe 0's two sources (`tagesschau.de` RSS, `bundesregierung.de` RSS) are seeded as `silver_eligible = true` with `silver_review_rationale = 'Probe 0 — institutional public data, government/public-broadcaster RSS, no re-identification risk per Manifesto §VI and WP-006 §7. Auto-eligible.'`. All other sources default to `false`.
- **Read-only role.** The existing `bff_readonly` Postgres role gains `SELECT` on the new columns; no write access. Review mutations happen via a separate administrative path (out of scope for Iteration 5 — the seed covers Probe 0; later sources are flagged via a one-off migration per review).
- **Silver eligibility surface.** `GET /api/v1/sources?silverOnly=true` — filter returning only Silver-eligible sources. `GET /api/v1/sources/{id}` includes the eligibility state and review metadata in its response.
- **Silver query endpoints.** Analogous to the Gold `/metrics`, `/entities`, `/languages` endpoints but scoped to Silver data:
  - `GET /api/v1/silver/documents?sourceId=…&start=…&end=…&limit=…` — paginated document listing for a Silver-eligible source.
  - `GET /api/v1/silver/documents/{id}` — individual Silver document (cleaned text + SilverMeta).
  - `GET /api/v1/silver/aggregations/{aggregationType}?sourceId=…&…` — aggregate queries analogous to `/metrics/distribution`, `/metrics/heatmap`, etc., but computed on Silver fields (token distributions, cleaned-text length distributions, entity raw counts pre-NER, etc.).

  All Silver endpoints verify the source's `silver_eligible = true` before returning data; non-eligible sources return HTTP 403 with the methodological refusal payload naming the review gate.

#### View-mode queries (§4.2.3 of the brief)

Each view mode is a cell in discipline × presentation. Most map to queries the BFF can compose from existing Gold data; a few require new aggregations:

- `GET /api/v1/metrics/{metricName}/distribution?scope=probe|source&scopeId=…&start=…&end=…&bins=…` — per-source/per-probe distributional data (histogram or raw-value arrays) for ridgeline/violin/density plots.
- `GET /api/v1/metrics/{metricName}/heatmap?scope=…&scopeId=…&xDimension=dayOfWeek&yDimension=hour&start=…&end=…` — 2D binning for heatmap view modes. Dimensions are enumerable: `dayOfWeek`, `hour`, `source`, `entityLabel`, `language`.
- `GET /api/v1/metrics/correlation?metrics=m1,m2,m3&scope=…&scopeId=…&start=…&end=…` — pairwise correlation matrix for metadata-mining × correlation-matrix views.
- `GET /api/v1/entities/cooccurrence?scope=…&scopeId=…&start=…&end=…&topN=…` — entity co-occurrence pair counts for Network Science × force-directed-graph views. **See the CorpusExtractor section below for the data backing this endpoint.**

**Scope parameter convention.** All view-mode endpoints accept either `probeId` or `sourceId` as the scope parameter (with probe scope as the default when ambiguous), preserving source-specific analysis as a first-class mode per Brief §4.2.4.

#### Content catalog expansion

The reframing increases the volume of Dual-Register content substantially, but does not add a new endpoint beyond what earlier ADR drafts anticipated. The existing plan for `GET /api/v1/content/{entityType}/{entityId}?locale=...` stands; the content surface under `services/bff-api/configs/content/` grows to cover:

- A Dual-Register pair (semantic / methodological) per metric, per view-mode cell, per refusal type, per probe, per discourse function, per function-lane empty state.
- Open-research-question entries for each WP §8 / §7 entry, with cross-links to the Working Paper anchor.
- The "How to read the globe" primer as a structured Markdown document with inline interactive parameters.

#### CorpusExtractor — entity co-occurrence (the one scope relaxation)

The Brief's view-mode catalog includes Network Science views that require aggregate pair-level data (entity co-occurrence). Computing this at BFF query time over the full `aer_gold.entities` table does not scale for even moderately sized probes. The clean architectural answer is the `CorpusExtractor` protocol already reserved in `CLAUDE.md` (analysis worker, "for future TF-IDF, LDA, co-occurrence").

Iteration 5 implements **one CorpusExtractor**: `EntityCoOccurrenceExtractor`. It reads `aer_gold.entities` in a configured time window per source, computes pairwise co-occurrence counts per document-window, and writes a new Gold table:

```sql
CREATE TABLE aer_gold.entity_cooccurrences (
    window_start      DateTime,
    window_end        DateTime,
    source            String,
    article_id        String,
    entity_a_text     String,
    entity_a_label    String,
    entity_b_text     String,
    entity_b_label    String,
    cooccurrence_count UInt32,
    ingestion_version  UInt64
) ENGINE = ReplacingMergeTree(ingestion_version)
ORDER BY (window_start, source, entity_a_text, entity_b_text)
TTL window_start + INTERVAL 365 DAY;
```

The extractor runs on the same NATS-triggered schedule as the per-document extractors but emits corpus-level rows. The BFF's `GET /api/v1/entities/cooccurrence` endpoint queries this table.

**Explicit non-scope:** topic modelling, article clustering, affect-space clustering, themescape rendering, and diachronic embedding drift all remain deferred beyond Iteration 5. The view-mode catalog's MVP stays within what per-document extractors plus this single corpus-level aggregation can support.

#### What Iteration 5 does *not* add to the backend

For clarity on what stays out:

- No changes to Bronze ingestion or to the Silver harmonisation path (ADR-015 contract unchanged).
- No changes to per-document extractors already in the pipeline (sentiment, entities, language, temporal distribution, word count).
- No changes to NATS JetStream, MinIO provisioning, or the Medallion architecture.
- No changes to authentication (Traefik + static API key per ADR-011/ADR-018).
- No real-time streaming requirements; the dashboard remains pull-based.
- No per-user state, saved dashboards, or personal annotations.
- No topic modelling, article clustering, or diachronic embedding infrastructure.

---

### Implementation Outline (Iteration 5 reordered)

Phased implementation — including exact phase numbers, ordering, scope, and splits — is scoped in `ROADMAP.md` under `Open Phases`. That file is the single source of truth; as surfaces have matured, phases have been split and renumbered. Duplicating phase numbers here produced drift, so this section is deliberately number-free and captures only the *architectural increments* the ADR commits to — in the rough sequence in which they will be built. ROADMAP.md binds each increment to a concrete phase.

**Iteration 5 reordering note (2026-04-25):** The reframing moves Surfaces II and III ahead of further Surface I deepening. Phase 100a's Surface I descent (L0–L4 on the globe) is committed as a reference point; its source-click behaviour is deprecated (Brief §4.1 probe-first emission), and its L3/L4 companion panels are replaced by transitions to Surfaces II and III with the always-visible methodology tray. Phase 100b is deferred pending Iteration 5 shipping. The increments below reflect the reordering.

The increments land in approximately this order. Early increments unblock the chrome; middle increments deliver Surface II and Surface III; later increments return to Surface I for the atmosphere refinements implied by the reframing (probe-first emission, source satellites) and add the cross-cutting overlays.

**Backend Baseline for Iteration 5.**
PostgreSQL migration adding `silver_eligible` and review-metadata columns to `public.sources`, with Probe 0 seeded as auto-eligible. New BFF endpoints landing together so the frontend can consume them: Probe Dossier composite endpoint, per-source article listing and detail, Silver query endpoints gated on eligibility, view-mode query endpoints (`/metrics/{name}/distribution`, `/metrics/{name}/heatmap`, `/metrics/correlation`, `/entities/cooccurrence`). `EntityCoOccurrenceExtractor` implemented in the analysis worker and wired into the NATS-triggered pipeline, writing to `aer_gold.entity_cooccurrences`. OpenAPI contract updated; `make codegen` runs across BFF and ingestion APIs; integration tests cover the new endpoints. This increment can land before frontend chrome work but must land before Surface II is fed real data.

**Content Catalog (Backend).**
YAML structure under `services/bff-api/configs/content/` with `en/` and `de/` locale subdirectories. Content entries for the Phase 42 metrics, each view-mode cell, the Probe 0 dossier, the four discourse functions (WP-001), the refusal types, and the "How to read the globe" primer. BFF endpoint `GET /api/v1/content/{entityType}/{entityId}?locale=...`. OpenAPI updated; integration tests against the new endpoint.

**Frontend Scaffolding.**
SvelteKit static project setup under `services/dashboard/` (already in place from Phase 97). TypeScript strict mode. `openapi-typescript` integration with `make codegen-ts` target. CI workflow for lint + typecheck + test + bundle-size gate. OTel Web SDK wired into the Phase 86 collector. Supply-chain automation (Trivy + Cosign + Syft) for the production image on par with Phase 84. The existing scaffolding carries over; extensions land as subsequent increments require.

**Navigation Chrome (Iteration 5 new).**
Left side rail (three surface anchors, scope indicator, pillar toggle, return-to-Atmosphere planet glyph) + top scope bar (surface-local navigation) + right methodology tray (closed/open states, push-mode default, push→overlay fallback at narrow viewports). Implemented as root-layout SvelteKit components; shared `focusedMetric` / scope stores. `design_system.md` gains a Navigation Chrome primitive family for the rail, scope bar, and tray. Accessibility: keyboard-navigable, screen-reader-labeled, reduced-motion-aware. This chrome is the frame in which every subsequent surface work renders.

**Surface II Foundation (Function Lanes + Probe Dossier).**
The Probe Dossier route as Surface II's default landing when a probe is selected. Source cards with per-source counts, publication frequency, etic classification, emic context. Function-lane routes with baseline time-series views driven by uPlot, plus empty-lane Dual-Register invitations. Source-scope narrowing from the Dossier propagating through URL state to view-mode queries. This is the single largest increment of Iteration 5; it delivers the dashboard's primary scientific surface.

**View-Mode Matrix (Iteration 5 new).**
The analytical-disciplines × presentation-forms matrix implemented as a backend-served catalog consumed by a typed matrix-cell registry on the frontend. Initial cells: time-series (NLP × time-series), ridgeline (EDA × distributional), heatmap (EDA × heatmap), correlation matrix (metadata mining × correlation-matrix), entity co-occurrence graph (Network Science × force-directed graph). Three MVP cells exposed per metric, chosen from structurally different cells; additional cells register as backend catalog grows. Rendering uses Observable Plot for static scientific charts and D3-force for the Network Science cell.

**Methodology Tray Content Binding.**
The tray subscribes to the `focusedMetric` store set by any chart/lane/Dossier interaction. Content flows from `/api/v1/content/...` and `/api/v1/metrics/{metricName}/provenance` in parallel. Dual-Register rendering (semantic summary primary on non-L4 surfaces; methodological register primary inside the tray). "Read the full Working Paper" anchor deep-links into Surface III.

**Surface III (Reflection).**
Long-form prose rendering (MDX-style with inline interactive Observable Plot cells). Working Paper routes (`/reflection/wp/{id}`). Probe Dossier methodology view (`/reflection/probe/{id}`). Metric provenance pages (`/reflection/metric/{metricName}`). Open Research Questions hub (`/reflection/open-questions`). "How to read the globe" primer (`/reflection/primer/globe`). Linked from every relevant affordance across Surfaces I and II. Code-split from Surfaces I and II.

**Silver-Layer Toggle (Iteration 5 new).**
Data-source toggle on Surface II (Gold default; Silver when the active source is eligible). Queries routed to Silver endpoints when the toggle is active. Non-eligible sources render an explicit "not Silver-eligible" state with the review-gate explanation drawn from the content catalog. View modes unchanged — the matrix operates identically over Silver, only the data source shifts.

**Surface I Refinement (probe-first emission + source satellites + Progressive Semantics on glyphs).**
Update the 3D engine and the Surface I shell to emit one glyph per probe (not per source), with source satellites as read-only presentation geometry. Raycaster filters selection to probes only; satellite clicks open the Probe Dossier with the source pre-filtered. Probe glyphs implement Progressive Semantics (semantic label primary, methodological expansion on obvious affordance). Link the "How to read the globe" primer from Surface I's top scope bar. This increment deprecates Phase 100a's source-click descent.

**Progressive Descent Infrastructure.**
Layer transitions via View Transitions API. Keyboard navigation for all five layers. URL-state persistence across descent and ascent on all three surfaces. L5 Evidence reader-pane overlay reachable from Surface II (chart click) and Surface III (inline citations). k-anonymity gate enforced at the BFF for article-detail requests.

**Geo-Analytics (Layer 3 on Surface II).**
MapLibre + deck.gl integration for intra-probe geo views when a probe's emic structure has geographic substance. Intent-based preload.

**Relational Networks beyond entity co-occurrence.**
The entity-co-occurrence cell ships with Surface II foundation. Cross-source propagation under Rhizome pillar mode is implemented when a second probe is live. Exploratory composition mode remains reserved for a later iteration.

**Negative Space Overlay.**
"What AĒR doesn't see" toggle. Demographic-opacity layer (WP-003 §6) annotations on Surface II charts. Coverage-map foregrounding on Surface I. Known-limitations-first mode for the methodology tray.

**Accessibility Audit and Performance Verification.**
Full WCAG 2.2 AA audit. Lighthouse CI on every PR. Hardware-class performance testing on both target devices (M1 Air, 2015 ThinkPad). Low-Fi mode verification (2D equirectangular map replacing the 3D globe while preserving probe-first emission and source satellites).

**Documentation Sweep.**
Arc42 chapters updated with §8.x for the frontend architecture (navigation chrome, surfaces, layers, view-mode matrix, Silver-layer toggle). `docs/design/reframing-note.md` deleted once Iteration 5 is shipping (its content has been merged into `design_brief.md` by Step 2). MkDocs navigation updated. Post-Iteration-5 terminology-reconciliation ADR drafted.

**Phase 113c — Probe-Click Descent and Layer-Label Reconciliation.**
Two structural corrections to Iteration 5 that fall out of the iteration-5 bug review:

1. *Probe click descends directly to Surface II L1 Probe Dossier.* The pre-113c flow opened an in-page L3 Analysis SidePanel ("flyout") on Surface I when a probe glyph was clicked. The flyout duplicated information the Probe Dossier already owns and contradicted the Brief §4.1 commitment that the globe is the welcome mat, not the working surface. Phase 113c retires the flyout: a probe click (and a satellite click) navigates to `/lanes/{probeId}/dossier`, optionally with `?sourceId=…` for satellite descent. The framing copy that used to live in the flyout (emic semantic paragraph, structural meta, methodology-tray entry, reach disclaimer per Brief §5.7) moves into the Dossier where it belongs. Old `?probe=X[&view=analysis…]` deeplinks redirect to the canonical Dossier route.

2. *Canonical layer-label reconciliation across ROADMAP, ADR, and UI chrome.* Brief §5.2 is the SoT: every surface owns L0–L2 chrome natively; L3 Analysis is native on Surfaces II and III; L4 Provenance is the methodology tray (everywhere); L5 Evidence is the reader-pane overlay (Surfaces II/III). The user-path layer labels on the primary descent now read coherently:

   - **Surface I · L0 Atmosphere (Globe)** — entry point
   - **Surface II · L1 Probe Dossier** — landing on probe selection
   - **Surface II · L3 Function Lane** — primary analytical surface (LensBar: Function · Layer · Metric · View)
   - **Surface III · L3 Working Paper** — long-form methodology

   L2 stays reserved for in-surface exploration controls (Surface I time scrubber, Surface II source-scope narrowing, Surface III section anchors). The ROADMAP entry for Phase 113c is updated to use these labels; in-code comments and component headers are aligned. No UI route renumbering is required — the change is a labeling reconciliation, not a routing change.

Phase 113c also promotes the four-function coverage indicator on the Probe Dossier into the central interaction of L1 (a function-tile selector that descends to the lane), and adds Function and Layer groups to the Function Lane LensBar so EA/PL/CI/SF and Au/Ag share visual weight with Metric and View.

**Phase 113d — Structural Bug Fixes (2026-04-27).**

1. *Methodology tray scoped to Surface II L3 only.* `MethodologyTray.svelte` now reads `page.url.pathname` via a `$derived` rune to detect whether a Function Lane is active (`/lanes/{probeId}/{functionKey}` where `functionKey ≠ dossier`). The `<aside>` is conditionally rendered only on that route; the `--tray-right-edge` CSS variable resets to `0px` otherwise. This removes the sidedrawer from Surface I and the Probe Dossier, where metric selection (and thus methodology context) does not occur.

2. *Surface I ScopeBar and TimeScrubber rationalisation.* The from/to date label was removed from the ScopeBar — it duplicates information already visible in the TimeScrubber. Resolution selection was moved into `TimeScrubber.svelte` as a new `.resolution-row` section (native `<select>`, Svelte prop interface `resolution` / `onResolutionChange`). Both sections carry descriptive captions so the user understands function and purpose without supplementary documentation.

3. *URL serialisation gap on Surface II fixed.* `writeToSearch` previously gated `metric`, `viewMode`, `layer`, and `sourceId` serialisation on `state.probe !== null`. On Surface II routes `probe` is a path segment, not a query param, so these fields were silently dropped on every URL write. The gates are removed; these four fields now serialise unconditionally when set. The `sourceId` param now supports comma-separated multi-source narrowing (`sourceIds: string[]` in `UrlState`).

4. *Source-scope narrowing redesigned.* Satellite click on Surface I no longer filters the Dossier to a single source; instead it calls `setUrl({ sourceIds: [name] })` before navigating to `/lanes/{probeId}/dossier`. The Dossier always shows all sources but displays a scope-note banner and a "× Clear scope" control when `url.sourceIds` is non-empty. `SourceCard.svelte` renders a "Narrow scope" / "Remove from scope" toggle that updates `sourceIds` and, on addition, navigates directly to the source's `primaryFunction` lane — so the user lands in the correct lane immediately. Multi-source narrowing is supported: selecting a second source adds to the array rather than replacing it.

5. *Function Lane LensBar metric-ordering bug fixed.* `activeMetric` was inserted at position 2 in the iteration array (`[DEFAULT_METRIC_NAME, activeMetric, ...fromApi]`), causing it to always appear second regardless of its canonical position. The fix iterates `[DEFAULT_METRIC_NAME, ...fromApi]` only and appends `activeMetric` at the end solely if not already present (i.e., while the API is loading).

6. *View-mode component flicker eliminated.* The `$effect` in `FunctionLaneShell.svelte` previously set `CellComponent = null` before loading the next component module, causing an unmount/remount cycle on every selection. Removing that null assignment keeps the previous component mounted (and visible) until the new module resolves.

7. *Methodology expandable in Function Lane replaces the sidedrawer.* The right-side `MethodologyTray` for Surface II L3 is replaced by an inline expandable section below the cell (default expanded). This surfaces methodology context directly in the data view without requiring the user to discover the hidden sidedrawer. The expandable hosts a link to the WP for further reading.

8. *LensBar descriptions at selection points.* Each LensBar group (Function, Layer, Metric, View) now shows a `.lens-desc` caption below its button row describing the active selection. Function and Layer descriptions are static; View description reflects the active presentation's description string from `viewmodes.ts`.

Each increment is independent and deployable on its own. No increment leaves the dashboard in a non-functional state.

**Phase 114 — Multi-Source Subset & Multi-Probe Composition (Within-Context) (2026-04-30).**

Lifts every view-mode endpoint from a single `scopeId` to a composable scope union and introduces the parallel-stream response shape. Four architectural decisions are recorded here:

1. *Scope-array lift.* `GET /api/v1/metrics/{metricName}/distribution`, `.../heatmap`, `.../correlation`, `/api/v1/entities/cooccurrence`, `/api/v1/entities`, `/api/v1/languages`, and `/api/v1/metrics` all gain `sourceIds` and `probeIds` query parameters (comma-separated). The existing `scope=probe|source` + `scopeId` form is retained for backward compatibility; the new parameters are additive. At the BFF, the scope resolver expands `probeIds` via `ProbeRegistry`, unions them with explicit `sourceIds`, deduplicates, and rejects the request with a structured `RefusalPayload` if any source in the set is Silver-ineligible (on gated endpoints). The ClickHouse `WHERE source = ?` clause becomes `WHERE source IN (?)` across all four view-mode handlers; existing per-endpoint row caps hold against the largest plausible scope.

2. *Parallel-stream response shape.* Distribution and heatmap responses gain an optional `segmentBy=source|probe` flag. When set, the response body contains a top-level `streams` array in addition to the aggregate fields; each `DistributionStream` / `HeatmapStream` element carries `id`, `label`, `scopeKind`, and the full histogram/quantile payload scoped to the individual segment. The co-occurrence endpoint instead gains a per-node `presence[]` field enumerating which sources each node appears in. This design avoids a second round-trip for parallel-stream rendering on the frontend. The aggregate payload is always present, preserving the non-segmented consumer contract.

3. *Composition, not comparison.* Every stream in a multi-probe response carries its own independent baseline. No shared cross-context scale exists in this phase. The `segmentBy` flag is intentionally unavailable alongside normalization modes — a user composing two probes sees raw aggregated streams from each, never an absolute cross-probe comparison. This constraint is structural, not advisory: the normalization gate (equivalence check, `RefusalPayload` with `gate=metric_equivalence`) lives in Phase 115.

4. *Deferral of cross-cultural normalization.* Phase 114 deliberately ships zero normalization controls. The cross-frame equivalence gate (`aer_gold.metric_equivalence` checks, `percentile` mode, deviation-labelling UI, valid-comparisons panel, `RefusalPayload`) is fully out of scope and lives in Phase 115. The architecture separates the rendering substrate (multi-probe parallel-stream, this phase) from the methodological discipline that decides which composed views may carry an absolute or deviation claim (Phase 115).

**Phase 115 — Iteration 5 — Cross-Cultural Analysis Foundations (WP-004) (2026-04-30 / 2026-05-01).**

Operationalises WP-004 §5–§6 as a tightly-scoped extension of the Phase-65 schema and gate. Six architectural decisions are recorded here:

1. *Build on Phase 65; do not duplicate.* The `aer_gold.metric_baselines` and `aer_gold.metric_equivalence` schemas remain unchanged apart from a single new column: `metric_equivalence.notes String DEFAULT ''` (ClickHouse migration `000014_add_metric_equivalence_notes.sql`). The full review record (reviewer, date, working-paper anchor, full prose) lives in the new Postgres `equivalence_reviews` table (Phase 115 migration `000014_equivalence_reviews`). The ClickHouse `notes` column travels with the row so the BFF read path can serve the methodology-tray rationale without a cross-database join.

2. *Cross-frame gate as a sharpening of the existing gate.* The Phase-65 existence-gate (refuse `?normalization=zscore` when no `metric_equivalence` row exists at deviation-or-absolute level) is extended with a multi-language coverage check. When the resolved `sourceIds`/`probeIds` scope spans more than one detected language, the BFF additionally requires that `metric_equivalence` covers every language in scope; absent that, the response is HTTP 400 with a structured `RefusalPayload` (`gate=metric_equivalence`, `workingPaperAnchor=WP-004#section-5.2`, three concrete `alternatives` — drop normalization, constrain scope, use deviation labelling). Within-frame requests continue to use the unchanged Phase-65 path. The `gate=metric_equivalence` value joins the existing `RefusalPayload.gate` enum (`k_anonymity`, `silver_eligibility`, `equivalence`).

3. *Structured `equivalenceStatus` on `/metrics/available`.* The bare-string `equivalenceLevel` field is superseded by a structured `equivalenceStatus` object (`level`, `validatedBy`, `validationDate`, `notes`). The deprecated alias is retained for one release cycle so existing dashboard URL state and tests do not break in the same commit. New clients consume `equivalenceStatus`; the alias is removed in a future cleanup phase.

4. *Dual-path baseline computation.* The `MetricBaselineExtractor` (Phase 115) is the second `CorpusExtractor` in the analysis worker, running on the same NATS-cron pattern as `EntityCoOccurrenceExtractor` (Phase 102). Default cadence is daily, default window is 90 days. The standalone `scripts/compute_baselines.py` is retained for first-run on a new probe (Phase 123 uses it explicitly), manual recompute after a schema change, and Operations-Playbook walkthroughs. Both paths share the canonical computation function `internal.extractors.metric_baseline.compute_baseline_rows` so they produce byte-identical baselines for the same input window — the regression guard documented by the Phase-115 unit test in `services/analysis-worker/tests/test_metric_baseline_extractor.py`.

5. *Refusal-surface reuse.* The cross-cultural refusal renders through the same `RefusalSurface` component that handles the Phase-103 Silver-eligibility gate. The frontend recognises the structured `gate=metric_equivalence` field on a 400 body and sharpens the refusal kind to `cross_frame_equivalence_missing`, which keys into the new `cross_frame_equivalence_missing.yaml` Content Catalog entries (en + de) — same Dual-Register shape as the existing refusal entries.

6. *Composition surface vs. methodological gating.* Phase 115 extends Phase 114's composition surface with methodological gating; it does not re-scope the surface. Multi-probe rendering (Phase 114) remains the substrate; cross-cultural analysis is the subset where the user's question implicates an absolute or deviation comparison rather than a within-context one. The architectural answer is the same equivalence registry and baseline machinery in either case; the trigger for Level-2/Level-3 gating is the user's request shape (`?normalization=zscore` etc.), not a heuristic on probe metadata.

---

### References

* `docs/design/design_brief.md` — the authoritative brief this ADR answers to.
* `docs/design/visualization_guidelines.md` — visualisation constraints, encompassed by the brief.
* `docs/arc42/09_architecture_decisions.md` — ADR-003, ADR-008, ADR-011, ADR-013, ADR-014, ADR-015, ADR-016, ADR-017, ADR-018, ADR-019.
* `docs/methodology/en/WP-002-en-metric_validity_and_sentiment_calibration.md` — validation status semantics consumed by Epistemic Weight.
* `docs/methodology/en/WP-004-en-cross-cultural_comparability_of_discourse_metrics.md` — equivalence-gate semantics honoured by refusal surfaces.
* `docs/methodology/en/WP-006-en-observer_effect_reflexivity_and_the_ethics_of_discourse_measurement.md` — reflexive architecture principles realised in Surface III.
* `services/bff-api/api/openapi.yaml` — the contract the frontend consumes.
* Roadmap Phase 75 — BFF Security Hardening.
* Roadmap Phase 84 — Supply Chain & Container Hardening.
* Roadmap Phase 86 — Observability Wiring.
* Roadmap Phase 93 — most recent completed phase at time of writing.

---

### Decision Record

* **Proposed:** 2026-04-17 by Fabian Quist (senior architect) in collaborative design session with AĒR.
* **Ratified:** 2026-04-20 by the implementing engineer (Fabian Quist).
* **Iteration 5 update:** 2026-04-25. Following the 2026-04-24 Reframing Note, the Design Brief was rewritten to demote Surface I (the globe) to a landing overview, elevate Surface II (Function Lanes) and Surface III (Reflection) as the primary scientific and methodological surfaces, add Navigation as a first-class concern (§3), introduce the view-mode matrix (§4.2.3), commit Silver-layer access with an eligibility-flag gate (§9), resolve probe-first globe emission with source-satellite presentation (§4.1, §8.4), and record the WP-001 terminology drift with a deferred reconciliation (§6, Path B). The ADR's Authority, Compliance-check, Backend-Work, and Implementation-Outline sections have been updated accordingly; the technology stack decisions are unchanged.
* **Phase 113d update:** 2026-04-27. Implementation-Outline updated with Phase 113d structural fixes: methodology tray scoped to L3 only, URL serialisation gap fixed (multi-source `sourceIds`), satellite/scope-narrowing redesign, LensBar ordering fix, view-mode flicker fix, inline methodology expandable, and LensBar selection descriptions.
* **Phase 114 update:** 2026-04-30. Implementation-Outline updated with Phase 114: scope-array lift (`scopeId` → composable `sourceIds`/`probeIds` union), parallel-stream response shape (`segmentBy` flag + `streams[]` array on distribution and heatmap, `presence[]` on co-occurrence nodes), composition-not-comparison constraint, and deferral of cross-cultural normalization gating to Phase 115.
* **Phase 115 update:** 2026-05-01. Implementation-Outline updated with Phase 115: single-column `notes` extension on `aer_gold.metric_equivalence`, Postgres `equivalence_reviews` workflow table, cross-frame equivalence gate (HTTP 400 with structured `RefusalPayload`), `?normalization=percentile` as a third Level-2 view, structured `equivalenceStatus` on `/metrics/available` (with deprecation alias), `GET /api/v1/probes/{probeId}/equivalence`, dual-path baseline computation (auto extractor + retained CLI script with byte-identical regression guarantee), and full frontend treatment (LensBar normalization control, deviation byline, Probe Dossier "valid comparisons" panel, refusal-surface reuse).
* **Review date:** 2027-04 (12-month review) or on any major Svelte / three.js / ecosystem regression.

---

## ADR-021: Contract-First for All HTTP Services

**Status:** Accepted (Phase 96, 2026-04-19)

**Context.**
Hard Rule 4 ("Contract-first API") originated with the BFF, where an OpenAPI spec drives `oapi-codegen`. The ingestion API predates the rule and had no spec. A code review ahead of the Phase 97 frontend work surfaced three concrete gaps: (1) the crawler → ingestion boundary had no machine-readable contract, so crawlers and the ingestion handler could drift silently; (2) there was no browsable API reference for either service; (3) the modular BFF spec introduced in Phase 95 mixed two `$ref` styles, and a naïve "normalize to one style" effort is technically impossible without regressing generated Go types (see Consequences).

**Decision.**
Every AĒR HTTP service ships a contract-first OpenAPI 3.0 specification under `services/<service>/api/`. The spec is the SSoT for the service's HTTP surface. Codegen runs against the modular tree; a custom bundler produces a single-file artifact for Swagger UI. A principled two-style `$ref` convention is adopted and enforced by a CI lint gate.

**The two-style `$ref` convention.**

1. **Path-level refs to top-level components** (`api/paths/*.yaml`, pointing at `components/schemas/X`) MAY use `#/components/schemas/X`. This is the only form under which `kin-openapi` (oapi-codegen's loader) emits a **named** Go type — it inlines the path-item into the root document before resolving the ref. Switching such a ref to `../schemas/X.yaml` collapses the schema to an anonymous inline type, materially changing the generated API surface.
2. **All other refs** — refs inside schemas, parameters, responses, or any external file — MUST use external-file refs (`../schemas/X.yaml`). JSON Reference's `#` means "current document", so `#/components/...` inside an external file does not resolve against the root; strict bundlers (`redocly bundle`, `swagger-cli bundle`) correctly reject it.

Rule 2 is enforced by `scripts/openapi_ref_style_check.sh` (run via `make openapi-lint`). Rule 1 is enforced implicitly by `make codegen && git diff --exit-code`: any regression from a named type to an inline type shows up as a drift diff.

**Tooling.**

* `scripts/openapi_bundle.py` produces `services/<service>/api/openapi.bundle.yaml` by emulating `kin-openapi`'s path-item flattening, then resolving external-file refs. Off-the-shelf bundlers were evaluated and rejected because both (`redocly`, `swagger-cli`) reject the Rule-1 form this repo relies on for named Go types.
* A `swagger-ui` container is gated behind the `dev` compose profile, bound to `127.0.0.1:8089`, and mounts both bundle files with a multi-spec dropdown.
* `make codegen` regenerates both services from the modular source directly — the bundle is for human and frontend consumption only.

**Ingestion API scope.**
The ingestion API initially shipped a types-only codegen target (`internal/apicontract/generated.go`). Resolved in Phase 96c: the handler was migrated to `strict-server` (matching the BFF pattern), the `apicontract` package was removed, and the generated output now lives in `internal/handler/generated.go`. Both HTTP services now use `strict-server`, and the CI drift check covers both generated files byte-for-byte.

**Alternatives considered.**

* **BFF-only contract-first (status quo).** Rejected: leaves the crawler → ingestion boundary undocumented and unable to drift-check.
* **Normalize all refs to external-file form.** Rejected: attempted and reverted. Removing path-level `#/components/...` refs from the four BFF path files collapses four named Go types into anonymous inline types, breaking the BFF API's generated surface. The "inconsistency" observed in the Phase 95 spec is in fact the intended behavior of `kin-openapi` — what was missing was a written convention and a lint gate.
* **Normalize all refs to `#/components/...` form.** Rejected: fundamentally impossible because `#` means "current document" in JSON Reference; the style is only resolvable at path level via `kin-openapi`'s special-case flattening.
* **Use `redocly bundle` for the bundled artifact.** Rejected: `redocly` correctly refuses path-level `#/components/...` refs in external files. Routes away from either (a) a custom bundler or (b) abandoning named Go types; we chose (a).
* **Use `strict-server` for ingestion in Phase 96.** Deferred on scope grounds — the contract-first gap was closable without also rewriting the existing handler. The migration was completed in Phase 96c.

**Consequences.**

* New contributors must be aware of the two-style convention. §8.19 and the lint gate are the enforcement points; a violation of Rule 2 fails `make lint`.
* `scripts/openapi_bundle.py` is now a small first-party tool that must be maintained. Its surface is intentionally small (~100 lines) and has no external dependency beyond PyYAML (already present via the analysis worker).
* Both HTTP services (ingestion-api and bff-api) now use `strict-server`. Contract drift between the OpenAPI spec and handler behavior is structurally prevented by the `StrictServerInterface` compile-time check and the `make codegen && git diff --exit-code` CI gate covering both generated files.
* Swagger UI is available but gated behind a compose profile so it never accidentally ships to production.

**References.**

* `services/bff-api/api/openapi.yaml`, `services/ingestion-api/api/openapi.yaml`
* `scripts/openapi_bundle.py`, `scripts/openapi_ref_style_check.sh`
* Arc42 §8.19 — API Contract Layout & Tooling.
* Roadmap Phase 96 — OpenAPI Contract Consolidation.

### Decision Record

* **Proposed:** 2026-04-19 during Phase 96 implementation.
* **Ratified:** 2026-04-19 by the implementing engineer; confirmed 2026-04-20 by the author (Fabian Quist). Extended by Phase 96c (2026-04-20): ingestion `strict-server` convergence resolves the last open consequence.
* **Review date:** 2027-04 or on any oapi-codegen / kin-openapi major version bump that changes path-item flattening semantics.

---

## ADR-022: Article Resolution Source-of-Truth Lives in the Analytical Layer

**Status:** Accepted (Phase 113b, 2026-04-26)

**Context.**
The Medallion architecture spreads document state across two retention horizons. PostgreSQL holds *operational* metadata — `sources`, `ingestion_jobs`, `documents` (the bronze object key, idempotency key, lifecycle status) — pruned at 90 days to match Bronze ILM (Phase 52, `infra/postgres/migrations/000004_add_retention_indexes.up.sql`). MinIO Silver and ClickHouse Gold hold the *analytical* truth — derived metrics, NER, language detections, the SilverEnvelope — retained for 365 days. Phase 113 surfaced the consequence: between days 90 and 365, ClickHouse Gold has rows whose Postgres `documents` row has been deleted. Two concrete BFF symptoms followed. (1) `DossierStore.FetchSources` joins `documents → ingestion_jobs → sources` for `articlesTotal` / `articlesInWindow`, so the dossier under-reports counts (observed: Postgres 45 tagesschau / 0 bundesregierung versus Gold 195 / 25 in the same window). (2) `DossierStore.ResolveArticle` keys article-detail lookups on `documents.article_id`, so "View article" 404s on any article whose Postgres row has been retention-deleted, even though the Silver envelope and Gold metrics are still present.

The mismatch is structural, not transient. Aligning Postgres retention to 365 days hides today's incident but rebinds operational and analytical horizons; the next time the analytical horizon shifts (e.g. extending Gold to 730 days for longitudinal work) the same divergence reappears.

**Decision.**
Article resolution and per-source article counts move to the analytical layer.

* `aer_silver.documents` (already one-row-per-document, 365-day TTL) gains a `bronze_object_key String` column. The analysis worker writes it at the same point Silver is uploaded to MinIO.
* `DossierStore.ResolveArticle` queries `aer_silver.documents` for `(article_id) → (bronze_object_key, source)`. PostgreSQL `documents.article_id` is no longer consulted.
* `DossierStore.FetchSources` reads per-source counts from ClickHouse (`aer_silver.documents` for the count; `aer_gold.metrics` was considered but produces multiple rows per article). Postgres still supplies the human-facing source metadata (name, type, URL, classification, Silver-eligibility); only the count columns move.

PostgreSQL `documents` and `ingestion_jobs` retain their current 90-day retention and become an *operational soft cache* — load-bearing during ingestion (idempotency keys, job status, lifecycle flips), not load-bearing for read-path resolution. Their schema is unchanged.

**Consequences.**

* The dossier-count divergence becomes definitionally impossible: counts are read from the same store that owns the analytical horizon. The 90/365 split can persist, or move independently in either direction, without re-introducing the bug.
* `View article` succeeds for any article whose SilverEnvelope still exists in MinIO, regardless of Postgres state. This is the correct semantics — Silver is the canonical record.
* The historical drift (Postgres rows deleted before this ADR landed) is a one-shot data backfill, not a permanent obligation. `scripts/reconcile_documents.py` walks MinIO Bronze and re-creates the missing operational rows once; after this ADR, recurring reconciliation is unnecessary.
* `aer_silver.documents.bronze_object_key` is a new column on an existing ReplacingMergeTree table. Existing rows backfill to the empty string; the worker repopulates on the next reprocess of any redelivered event. The reconciliation script also writes it for historical articles whose Silver envelope is recoverable from MinIO.
* The k-anonymity gate and provenance lookup are unchanged — they already key on `(source, article_id, timestamp)` against ClickHouse Gold.
* The BFF read-only Postgres role keeps its current grants; nothing new was needed.

**Alternatives considered.**

* **Align Postgres retention to 365 days.** Rejected. Hides the current symptom by paying analytical-horizon storage in the operational store. Re-creates the same class of bug the next time horizons diverge.
* **Maintain a recurring reconciliation cron.** Rejected. Treats a structural mismatch as an operational chore. The cron eventually drifts out of sync with whichever store changes retention first.
* **Add `bronze_object_key` to every `aer_gold.metrics` row.** Rejected. Duplicative — the metrics table writes 5–10 rows per article. `aer_silver.documents` is already one row per article and is the natural home for the article→object-key index.
* **Add a dedicated `aer_gold.article_index` table.** Rejected on Ockham's-Razor grounds. `aer_silver.documents` already exists, already has the right cardinality, already has the right TTL.

**References.**

* Roadmap Phase 113b — PostgreSQL ↔ ClickHouse Retention Divergence.
* `infra/clickhouse/migrations/000013_add_bronze_object_key_to_silver_documents.sql` — schema migration.
* `infra/postgres/migrations/000004_add_retention_indexes.up.sql` — Postgres retention policy (unchanged by this ADR).
* `scripts/reconcile_documents.py` — one-shot historical backfill.
* `services/bff-api/internal/storage/dossier_store.go` — `ResolveArticle`, `FetchSources` rewritten.
* `docs/operations_playbook.md` §Retention.

### Decision Record

* **Proposed:** 2026-04-26 while fixing Phase 113b.
* **Ratified:** 2026-04-26 by the implementing engineer.
* **Review date:** when either retention horizon changes, or when `aer_silver.documents` is replaced by a richer Silver-projection schema.
`
## ADR-023: Multilingual Sentiment Strategy

| Property | Value |
| :--- | :--- |
| **Date** | 2026-05-02 |
| **Status** | Accepted |
| **Relates to** | WP-002 (Metric Validity), ADR-016 (Hybrid Tier Architecture), Phase 119, Phase 122 |

### Context

Phase 119 specifies `mdraw/german-news-sentiment-bert` as the primary German Tier 2 sentiment extractor, with `oliverguhr/german-sentiment-bert` as a domain-mismatch baseline. The pattern is German-specific by construction: a pinned model revision, a per-model determinism CI gate, a per-model Docker image footprint contribution.

Phase 122 sketches `cmarkea/distilcamembert-base-sentiment` for French following the same pattern. At N=2 languages this is acceptable. At N=50 (the project's stated long-term coverage trajectory — see `docs/extending/scalability-roadmap.md`) maintaining 50 separate per-language Tier 2 sentiment extractors is operationally untenable: 50 model revisions to track, 50 determinism gates to run in CI, an exploding image size, 50 memory footprints competing for analysis-worker resources.

The naïve alternative — a single multilingual sentiment model covering all languages — has its own cost: WP-002 §3.2 documents domain transfer as a primary failure mode of sentiment models. A model trained on 100-language Twitter data is structurally less calibrated to e.g. German institutional editorial RSS than a German-news-fine-tuned model on the same texts.

The decision is which of the two costs the architecture should default to, and how to allow the other to coexist where it adds value.

### Decision

**Multilingual model as the Tier 2 default, per-language models as optional Tier 2.5 refinements where available.** Both run in parallel under the ADR-016 dual-metric pattern. The dashboard renders both with Epistemic Weight (Brief §7.8) distinguishing them; the gap between them is itself a research signal that informs WP-002 §3.2's domain-transfer discussion.

Concretely:

1. **Tier 2 default extractor.** A single `MultilingualBertSentimentExtractor` running `cardiffnlp/twitter-xlm-roberta-base-sentiment` (or the equivalent multilingual SOTA at the time of implementation; final selection part of Phase 119's implementation work). Produces `metric_name = "sentiment_score_bert_multilingual"`. Covers all languages the model handles. Single Docker-image footprint contribution. Single determinism CI gate. The Phase 116 language guard ensures the extractor only runs on documents whose `detected_language` is in the model's supported set; for unsupported languages, the metric is genuinely absent (no row written), not zero.

2. **Tier 2.5 per-language refinements.** Where a high-quality per-language news-domain model exists on Hugging Face Hub (e.g. `mdraw/german-news-sentiment-bert` for German, `cmarkea/distilcamembert-base-sentiment` for French), it ships as an additional extractor producing `metric_name = "sentiment_score_bert_<lang>_<domain>"` (e.g. `sentiment_score_bert_de_news`). Optional and language-bound.

3. **Both metrics ship in parallel.** A German document produces both `sentiment_score_bert_multilingual` and `sentiment_score_bert_de_news`. The dashboard surfaces both. Epistemic Weight reflects each metric's `validation_status` independently per ADR-016. The two are NEVER averaged into a composite.

4. **Tier 1 SentiWS pattern unchanged.** SentiWS-class deterministic lexicon extractors remain the Tier 1 baseline per Phase 117. The dual-metric pattern of ADR-016 (Tier 1 + Tier 2 in parallel) is preserved; this ADR refines the Tier 2 layer into Tier 2 (multilingual) + optional Tier 2.5 (per-language).

5. **Phase 119 rescope.** Phase 119 implements (1) the multilingual Tier 2 extractor and (2) the German Tier 2.5 refinement (`mdraw/german-news-sentiment-bert`). The `oliverguhr/german-sentiment-bert` review-domain baseline originally specified in Phase 119 is **demoted** from a primary deliverable to an optional Tier 2.5 refinement (`sentiment_score_bert_de_review`); it ships if and only if the engineering capacity is available, with the same defer-friendly pattern as other language-specific extractors. The methodological motivation for the review-domain baseline (on-pipeline domain-transfer evidence) is preserved by the multilingual-vs-per-language gap, which provides the same signal at lower operational cost.

### Consequences

**Positive.**
- Sentiment coverage scales with O(1) extractors per language addition: a new probe in a language the multilingual model supports gets sentiment for free.
- Per-language quality is preserved where a strong native model exists.
- The methodological caution of WP-002 §3.2 (domain transfer is a real failure mode) is honoured by surfacing both metrics, not by hiding the multilingual model's weaknesses.
- The Phase 122 NLP section becomes simpler: French ships with the multilingual default automatically; the Tier 2.5 CamemBERT integration is an optional refinement, not a blocker.

**Negative.**
- The dashboard surfaces more metrics for sentiment-rich probes. The Epistemic Weight system (Brief §7.8) is responsible for keeping the cognitive load manageable.
- The multilingual model's per-language calibration is unknown until annotation studies run for each context. The `aer_gold.metric_validity` scaffold ships with `validation_status='unvalidated'` for the multilingual extractor at every context-key, mirroring the Tier 1/2 pattern.
- A multilingual model upgrade (new revision, new model entirely) is a single coordinated change that affects all languages simultaneously. This is intentional: the determinism gate catches drift, but every language's sentiment baseline shifts together. Per-language models do not have this property.

### Implementation Outline

Phase 119 ships:
- `MultilingualBertSentimentExtractor` (primary, all-language).
- `GermanNewsBertSentimentExtractor` (Tier 2.5 refinement for German).
- Determinism CI gates for both.
- `metric_validity` scaffold rows for both at each Probe-0 context-key.
- `metric_provenance.yaml` entries for both.

Phase 122 (French) adds:
- Optional `FrenchNewsBertSentimentExtractor` (Tier 2.5 refinement for French) following the same pattern as the German Tier 2.5 extractor.
- The multilingual default automatically covers French via the language guard — no extractor work.

Future probes:
- Tier 2 default coverage requires no work — the multilingual extractor handles it.
- Tier 2.5 refinements are optional, language-by-language, defer-friendly.

### References

- WP-002 §3.2 (domain transfer as primary failure mode)
- ADR-016 (Hybrid Tier Architecture, dual-metric pattern)
- Brief §7.8 (Epistemic Weight)
- Phase 119, Phase 122
- `docs/extending/add-a-language.md`
- `docs/extending/scalability-roadmap.md`

### Decision Record

- **Proposed:** 2026-05-02 by Fabian Quist (senior architect) with AĒR.
- **Ratified:** 2026-05-02 by the implementing engineer.
- **Review date:** 2027-05 (12-month review) or on a major shift in multilingual sentiment SOTA.

---

## ADR-023: Multilingual Sentiment Strategy

| Property | Value |
| :--- | :--- |
| **Date** | 2026-05-02 |
| **Status** | Accepted |
| **Relates to** | WP-002 (Metric Validity), ADR-016 (Hybrid Tier Architecture), Phase 119, Phase 122 |

### Context

Phase 119 specifies `mdraw/german-news-sentiment-bert` as the primary German Tier 2 sentiment extractor, with `oliverguhr/german-sentiment-bert` as a domain-mismatch baseline. The pattern is German-specific by construction: a pinned model revision, a per-model determinism CI gate, a per-model Docker image footprint contribution.

Phase 122 sketches `cmarkea/distilcamembert-base-sentiment` for French following the same pattern. At N=2 languages this is acceptable. At N=50 (the project's stated long-term coverage trajectory — see `docs/extending/scalability-roadmap.md`) maintaining 50 separate per-language Tier 2 sentiment extractors is operationally untenable: 50 model revisions to track, 50 determinism gates to run in CI, an exploding image size, 50 memory footprints competing for analysis-worker resources.

The naïve alternative — a single multilingual sentiment model covering all languages — has its own cost: WP-002 §3.2 documents domain transfer as a primary failure mode of sentiment models. A model trained on 100-language Twitter data is structurally less calibrated to e.g. German institutional editorial RSS than a German-news-fine-tuned model on the same texts.

The decision is which of the two costs the architecture should default to, and how to allow the other to coexist where it adds value.

### Decision

**Multilingual model as the Tier 2 default, per-language models as optional Tier 2.5 refinements where available.** Both run in parallel under the ADR-016 dual-metric pattern. The dashboard renders both with Epistemic Weight (Brief §7.8) distinguishing them; the gap between them is itself a research signal that informs WP-002 §3.2's domain-transfer discussion.

Concretely:

1. **Tier 2 default extractor.** A single `MultilingualBertSentimentExtractor` running `cardiffnlp/twitter-xlm-roberta-base-sentiment` (or the equivalent multilingual SOTA at the time of implementation; final selection part of Phase 119's implementation work). Produces `metric_name = "sentiment_score_bert_multilingual"`. Covers all languages the model handles. Single Docker-image footprint contribution. Single determinism CI gate. The Phase 116 language guard ensures the extractor only runs on documents whose `detected_language` is in the model's supported set; for unsupported languages, the metric is genuinely absent (no row written), not zero.

2. **Tier 2.5 per-language refinements.** Where a high-quality per-language news-domain model exists on Hugging Face Hub (e.g. `mdraw/german-news-sentiment-bert` for German, `cmarkea/distilcamembert-base-sentiment` for French), it ships as an additional extractor producing `metric_name = "sentiment_score_bert_<lang>_<domain>"` (e.g. `sentiment_score_bert_de_news`). Optional and language-bound.

3. **Both metrics ship in parallel.** A German document produces both `sentiment_score_bert_multilingual` and `sentiment_score_bert_de_news`. The dashboard surfaces both. Epistemic Weight reflects each metric's `validation_status` independently per ADR-016. The two are NEVER averaged into a composite.

4. **Tier 1 SentiWS pattern unchanged.** SentiWS-class deterministic lexicon extractors remain the Tier 1 baseline per Phase 117. The dual-metric pattern of ADR-016 (Tier 1 + Tier 2 in parallel) is preserved; this ADR refines the Tier 2 layer into Tier 2 (multilingual) + optional Tier 2.5 (per-language).

5. **Phase 119 rescope.** Phase 119 implements (1) the multilingual Tier 2 extractor and (2) the German Tier 2.5 refinement (`mdraw/german-news-sentiment-bert`). The `oliverguhr/german-sentiment-bert` review-domain baseline originally specified in Phase 119 is **demoted** from a primary deliverable to an optional Tier 2.5 refinement (`sentiment_score_bert_de_review`); it ships if and only if the engineering capacity is available, with the same defer-friendly pattern as other language-specific extractors. The methodological motivation for the review-domain baseline (on-pipeline domain-transfer evidence) is preserved by the multilingual-vs-per-language gap, which provides the same signal at lower operational cost.

### Consequences

**Positive.**
- Sentiment coverage scales with O(1) extractors per language addition: a new probe in a language the multilingual model supports gets sentiment for free.
- Per-language quality is preserved where a strong native model exists.
- The methodological caution of WP-002 §3.2 (domain transfer is a real failure mode) is honoured by surfacing both metrics, not by hiding the multilingual model's weaknesses.
- The Phase 122 NLP section becomes simpler: French ships with the multilingual default automatically; the Tier 2.5 CamemBERT integration is an optional refinement, not a blocker.

**Negative.**
- The dashboard surfaces more metrics for sentiment-rich probes. The Epistemic Weight system (Brief §7.8) is responsible for keeping the cognitive load manageable.
- The multilingual model's per-language calibration is unknown until annotation studies run for each context. The `aer_gold.metric_validity` scaffold ships with `validation_status='unvalidated'` for the multilingual extractor at every context-key, mirroring the Tier 1/2 pattern.
- A multilingual model upgrade (new revision, new model entirely) is a single coordinated change that affects all languages simultaneously. This is intentional: the determinism gate catches drift, but every language's sentiment baseline shifts together. Per-language models do not have this property.

### Implementation Outline

Phase 119 ships:
- `MultilingualBertSentimentExtractor` (primary, all-language).
- `GermanNewsBertSentimentExtractor` (Tier 2.5 refinement for German).
- Determinism CI gates for both.
- `metric_validity` scaffold rows for both at each Probe-0 context-key.
- `metric_provenance.yaml` entries for both.

Phase 122 (French) adds:
- Optional `FrenchNewsBertSentimentExtractor` (Tier 2.5 refinement for French) following the same pattern as the German Tier 2.5 extractor.
- The multilingual default automatically covers French via the language guard — no extractor work.

Future probes:
- Tier 2 default coverage requires no work — the multilingual extractor handles it.
- Tier 2.5 refinements are optional, language-by-language, defer-friendly.

### References

- WP-002 §3.2 (domain transfer as primary failure mode)
- ADR-016 (Hybrid Tier Architecture, dual-metric pattern)
- Brief §7.8 (Epistemic Weight)
- Phase 119, Phase 122
- `docs/extending/add-a-language.md`
- `docs/extending/scalability-roadmap.md`

### Decision Record

- **Proposed:** 2026-05-02 by Fabian Quist (senior architect) with AĒR.
- **Ratified:** 2026-05-02 by the implementing engineer.
- **Review date:** 2027-05 (12-month review) or on a major shift in multilingual sentiment SOTA.

---

## ADR-024: Language Capability Manifest

| Property | Value |
| :--- | :--- |
| **Date** | 2026-05-02 |
| **Status** | Accepted |
| **Relates to** | Phase 116, Phase 122, ADR-023, WP-001 §5.3 |

### Context

Per-language analytical capability is currently distributed across at least five locations:

1. `services/analysis-worker/requirements.txt` — which spaCy models are installed.
2. The hard-coded NER language map in `NamedEntityExtractor`.
3. The hard-coded sentiment language map in `SentimentExtractor`.
4. `extractors/_negation_config.py` — per-language negation configuration (Phase 117).
5. `services/analysis-worker/configs/cultural_calendars/<region>.yaml` — per-region cultural calendars.

There is no single place that answers the question: *"For language X, which capabilities does AĒR have?"* This produces three concrete problems:

- **Manual scaffolding drift.** When a new language is added, the engineer must remember to add a row to the `aer_gold.metric_validity` scaffold for each `(metric_name, context_key)` pair. This is easy to forget; the gap is invisible until a consumer queries the validation status.
- **Probe Coverage Map blocked.** WP-001 §5.3 mandates a Probe Coverage Map showing for each region which discourse functions and which analytical instruments are available. Without a manifest, the map has no data source other than walking the five locations above.
- **`docs/extending/add-a-language.md` is hand-maintained.** The matrix is the canonical reference for language additions, but it cannot be verified against running code — a documentation/code drift risk grows with every language addition.

The decision is whether to consolidate these five touchpoints behind a single declarative source, or accept the distributed status quo as the price of layered architecture.

### Decision

**A single YAML manifest at `services/analysis-worker/configs/language_capabilities.yaml` is the system of record for per-language analytical capability.** Existing code is refactored to read the manifest; the hard-coded language maps become derived views.

The manifest schema:

```yaml
# services/analysis-worker/configs/language_capabilities.yaml
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
      metric_name: sentiment_score_sentiws
    
    sentiment_tier2_default:
      tier: 2
      method: multilingual_bert
      provided_by: shared.multilingual_bert  # references the multilingual default
      metric_name: sentiment_score_bert_multilingual
    
    sentiment_tier2_refinement:
      tier: 2.5
      method: news_domain_bert
      model: mdraw/german-news-sentiment-bert
      model_revision: "<sha>"
      metric_name: sentiment_score_bert_de_news
    
    cultural_calendar:
      region_default: de
      file: cultural_calendars/de.yaml
    
    notes:
      - "Compound-split is German-specific (Phase 117); not applicable to non-agglutinating languages."

  fr:
    iso_code: fr
    display_name: French
    
    ner:
      tier: 1.5
      model: fr_core_news_lg
      model_version: "3.8.0"
    
    sentiment_tier1:
      tier: 1
      method: lexicon
      lexicon: feel_v1.0
      features: [negation_dependency, custom_lexicon]
      metric_name: sentiment_score_feel
    
    sentiment_tier2_default:
      tier: 2
      method: multilingual_bert
      provided_by: shared.multilingual_bert
      metric_name: sentiment_score_bert_multilingual
    
    # No tier2_refinement yet — sentiment_tier2_refinement key omitted.
    
    cultural_calendar:
      region_default: fr
      file: cultural_calendars/fr.yaml

shared:
  multilingual_bert:
    model: cardiffnlp/twitter-xlm-roberta-base-sentiment
    model_revision: "<sha>"
    supported_languages: [de, en, fr, es, it, pt, ja, zh, ar, ...]
```

### What the manifest drives

1. **NER and Sentiment language routing.** The hard-coded language maps in `NamedEntityExtractor` and `SentimentExtractor` are removed; both extractors read the manifest at startup.

2. **Auto-generated `aer_gold.metric_validity` scaffold.** A new build-time script `scripts/generate_metric_validity_scaffold.py` reads the manifest and emits the scaffold SQL. Every `(metric_name, language)` combination gets a row with `validation_status='unvalidated'`, `tier=<tier>`, `error_taxonomy='{"reason":"engineering default; awaiting WP-002 annotation study"}'`. Manual edits to the scaffold are forbidden — the manifest is the source of truth.

3. **Probe Coverage Map data source.** When implemented (separate ROADMAP phase), the BFF endpoint `GET /api/v1/coverage/map` reads the manifest plus `source_classifications` to produce per-probe coverage information.

4. **Auto-generated `add-a-language.md` matrix.** The touchpoint matrix in `docs/extending/add-a-language.md` is regenerated from the manifest at MkDocs build time via a custom MkDocs macro plugin (or a pre-build script). The hand-maintained version becomes a build artefact.

5. **Single-source `language` parameter validation.** BFF endpoints accepting a `?language=` query parameter validate against the manifest's language list, rejecting unknown codes with a structured `RefusalPayload`.

### Consequences

**Positive.**
- Adding a new language becomes a single-file change with computable downstream effects.
- The Probe Coverage Map becomes implementable.
- Documentation/code drift is structurally prevented.
- The `metric_validity` scaffold is always honest — every `(metric_name, language)` combination has a row, no silent gaps.

**Negative.**
- Refactoring the existing extractors to read the manifest is implementation work that must happen before the second language ships in production.
- A schema change to the manifest is now a coordinated change affecting multiple downstream consumers (the scaffold script, the `add-a-language.md` generator, the BFF). Versioning the manifest schema explicitly (a `manifest_version: 1` field) mitigates this.
- The manifest is a YAML file, not a typed config object. A Pydantic schema in `services/analysis-worker/internal/models/language_capability.py` is the architectural answer to that drawback.

### Implementation Outline

This ADR is ratified before implementation; the implementation ships as a single phase, sized appropriately for solo work:

1. Pydantic schema for the manifest (`LanguageCapability`, `SharedCapability`).
2. Manifest YAML with `de` filled in to match Probe 0's current state.
3. Refactor `NamedEntityExtractor` and `SentimentExtractor` to read the manifest. Existing tests must continue to pass — this is a refactor, not a behaviour change.
4. `scripts/generate_metric_validity_scaffold.py` and `make scaffold-metric-validity` Makefile target. The generated SQL is committed (not gitignored) so reviewers see drift.
5. `metric_validity` scaffold migrated from hand-maintained to generated; CI gates the drift.
6. `language` parameter validator in the BFF.
7. The auto-generation of `add-a-language.md` is **deferred** to the Probe Coverage Map phase; for now the doc remains hand-maintained but checked against the manifest by a `scripts/check_language_doc_drift.py` script in CI.

Estimated effort: 1 working day for the refactor + scaffold-generator. The fr-language manifest entry is added by Phase 122 as part of its existing scope.

### References

- WP-001 §5.3 (Probe Coverage Map mandate)
- ADR-023 (Multilingual Sentiment Strategy)
- Phase 122 (consumer of the manifest)
- `docs/extending/add-a-language.md`
- `docs/extending/scalability-roadmap.md`

### Decision Record

- **Proposed:** 2026-05-02 by Fabian Quist with AĒR.
- **Ratified:** 2026-05-02 by the implementing engineer.
- **Review date:** 2027-05 or when language count exceeds 10.

---

## ADR-025: Cultural Calendar Composition

| Property | Value |
| :--- | :--- |
| **Date** | 2026-05-02 (deferred sketch) |
| **Status** | **Deferred** — ratification when probe count reaches N=10 or duplication friction is felt |
| **Relates to** | WP-004 §6.3, WP-005 §4.3, Phase 123 |

### Context

The current cultural calendar mechanism (Phase 115 / Workflow 6) is one YAML file per region: `configs/cultural_calendars/de.yaml`, `fr.yaml`, etc. Each file lists holidays, election dates, religious calendars, and recurring media events. The structure is linear: at N regions, there are N files, each authored independently.

At N=2–5 this is acceptable. At N≈30 — and AĒR's stated trajectory targets that range — religious and cultural categories duplicate across files: Ramadan, Eid al-Fitr, Eid al-Adha appear in every probe with a Muslim-majority constituency; Christmas, Easter, Pentecost in every probe with a Christian heritage; Lunar New Year across East Asian probes.

Duplication has two costs: data drift (two files disagree on Ramadan 2027 dates) and authoring friction (a new probe author re-types known holidays).

### Decision (sketched, not yet ratified)

**YAML inheritance via an `extends:` directive.** Shared bases live in `configs/cultural_calendars/_shared/` and are extended by per-region files:

```yaml
# _shared/christian-western.yaml
events:
  - name: "Christmas"
    date: "12-25"
    category: religious_holiday
  - name: "Easter Sunday"
    date_rule: "easter_western"
    category: religious_holiday
  # ...

# fr.yaml
extends: _shared/christian-western.yaml
events:
  - name: "Bastille Day"
    date: "07-14"
    category: national_holiday
  - name: "Toussaint"
    date: "11-01"
    category: religious_holiday
  # ...
```

Multiple `extends:` entries are allowed for layered cultural traditions (e.g. a probe in a country with both Christian and Muslim significant populations).

### Why deferred

Premature implementation is forbidden by Occam's Razor. The mechanism is not currently needed (N=2 at the close of Phase 122). Implementing it now adds a YAML inheritance engine to maintain *before* it pays off.

The ADR exists in this sketched form so the path is documented and consensual; ratification and implementation happen at the trigger condition.

### Trigger conditions for ratification

Any of:
- Probe count reaches N=10.
- Three or more probes share a religious calendar that contains five or more identical events.
- A bug is filed where two regional calendars disagree on a shared event's date or category.

### Implementation Outline (when triggered)

1. YAML loader extension in the analysis worker reads `extends:` directive recursively, with cycle detection.
2. Effective calendar = base ∪ per-region (per-region overrides base on name conflict).
3. New `_shared/` subdirectory.
4. Existing region files migrated incrementally — `de.yaml` and `fr.yaml` converted to inherit from `_shared/christian-western.yaml`.
5. CI test verifying no infinite-recursion `extends:` chains.

### References

- WP-005 §4.3 (cultural calendar foundation)
- Phase 123 (current single-file pattern)
- `docs/extending/scalability-roadmap.md` § Cultural Calendar Composition

### Decision Record

- **Sketched:** 2026-05-02 by Fabian Quist with AĒR.
- **Status:** Deferred. Path documented; implementation pending trigger.

---

## ADR-026: Multilingual NER Fallback

| Property | Value |
| :--- | :--- |
| **Date** | 2026-05-02 (deferred sketch) |
| **Status** | **Deferred** — ratification when N≥5 probes are affected by NER coverage gaps |
| **Relates to** | Phase 116, ADR-016, ADR-023, ADR-024 |

### Context

spaCy provides trained NER models for ~25 languages: German, English, French, Spanish, Italian, Portuguese, Dutch, Danish, Swedish, Norwegian, Finnish, Polish, Russian, Ukrainian, Japanese, Chinese, Korean, Greek, Romanian, Lithuanian, and a handful of others. Many languages relevant to AĒR's stated coverage trajectory have no off-the-shelf spaCy NER model: Suaheli, Tagalog, Bengali, Khmer, Amharic, Tigrinya, Hausa, Yoruba, etc.

The Phase 116 absence-not-wrong guarantee handles this gracefully today: no model → `entity_count = 0`, no entity spans, structured warning log. The `aer_gold.metric_validity` scaffold (when ADR-024 implements it) records the gap honestly.

But: a probe without NER has no entity co-occurrence network, no Wikidata QID linking, no relational analysis at all. The probe operates at WP-004 Level 1 only. At N=1 such probe this is acceptable. At N=5 affected probes, half of AĒR's Episteme-pillar capability is dark for those probes — and the project's stated mission is global coverage, which means non-spaCy-supported languages will be in the majority eventually.

### Decision (sketched, not yet ratified)

**A multilingual transformer-based NER fallback as an optional Tier 2 path, language-routed by ADR-024's manifest.** The architectural shape mirrors ADR-023's multilingual sentiment decision:

1. **Tier 1.5 spaCy NER stays as the primary** for languages with a trained model. Phase 116's existing language router selects the spaCy model. No change.

2. **Tier 2 multilingual NER fallback** for languages without a spaCy model. Candidate models: `Babelscape/wikineural-multilingual-ner` (covers 9 languages), `Davlan/distilbert-base-multilingual-cased-ner-hrl` (10 languages), or the SOTA at the time of implementation. Output: separate metric `entity_count_bert_multilingual` and entity spans written to a separate Gold table column or with a `link_method` value distinguishing them. The two NER paths NEVER merge their outputs; the dashboard surfaces them with Epistemic Weight per Brief §7.8.

3. **ADR-024's manifest declares the path per language.** A language entry's `ner` block specifies either `tier: 1.5` (spaCy) or `tier: 2` (multilingual transformer) or both.

### Why deferred

Two reasons. **First, trigger condition not met.** N=2 at end of Phase 122; both languages have spaCy NER. Implementing now is premature.

**Second, the multilingual NER landscape is unstable.** The named candidate models (WikiNeural, distilBERT-multilingual-NER) are 2022-vintage; better alternatives almost certainly exist by the time the trigger fires. Ratifying a specific model choice in 2026 for a 2028 implementation would be locking in a stale decision.

### Trigger conditions for ratification

Any of:
- N≥5 probes operate in languages without spaCy NER coverage.
- A probe in a politically significant context has no NER and the Coverage Map (when implemented) makes the gap acutely visible.
- The Wikidata entity-linking layer (Phase 118) is observed producing significantly degraded results for non-spaCy probes (because there are no entity spans to link).

### Implementation Outline (when triggered)

1. Re-survey the multilingual NER landscape. Select a model based on language coverage, license, deployment footprint.
2. Pinned-revision integration following the Phase 119 dual-extractor pattern (determinism flags, version hash, separate metric name).
3. ADR-024 manifest extended with the per-language `tier: 2` NER declaration.
4. New `MultilingualBertNamedEntityExtractor` registered alongside the existing `NamedEntityExtractor`. Routing decided by manifest, not by code.
5. Wikidata entity linking (Phase 118) extended to consume entity spans from both NER paths transparently.

### References

- Phase 116 (absence-not-wrong guarantee)
- ADR-023 (parallel multilingual+per-language pattern for sentiment)
- ADR-024 (Language Capability Manifest)
- `docs/extending/scalability-roadmap.md` § NER model availability

### Decision Record

- **Sketched:** 2026-05-02 by Fabian Quist with AĒR.
- **Status:** Deferred. Path documented; implementation pending trigger.

---
## ADR-027: Wikidata Entity Linking is Disambiguation, not Discovery

**Date:** 2026-05-04
**Status:** Accepted
**Related ADRs:** ADR-016 (Hybrid Tier Architecture), ADR-020 (Frontend Stack)
**Related Phases:** 102 (`EntityCoOccurrenceExtractor`), 118 (Wikidata alias index), 118b (dump-stream build pipeline)

### Context

Phase 118 introduces a Wikidata alias index that maps NER surface forms to canonical QIDs. During the Phase-118b post-mortem (2026-05) — triggered by three semantic bugs in the first 157k-entity production build — a deeper architectural question surfaced that had never been written down: **is the index meant to be a *Disambiguation* layer (resolve fragmented surface forms when possible; canonical store stays string-keyed) or a *Discovery* layer (the index defines what entities the system can perceive)?**

The two interpretations have very different consequences. Discovery makes Coverage a hard requirement: every news brand, every regional politician, every culturally-relevant entity in every probe's language must be in the index, or the system is "blind" to it. Disambiguation makes Coverage optional: unlinked entities flow through the pipeline as string-keyed nodes; linking adds a canonical-id sidecar where it can.

The implicit Coverage anxiety — *"if we're missing Tagesschau, the system can't see news brands; if we're missing French politicians, Probe 2 is broken"* — would force the YAML bucket curation to scale to N≈30 probes across all language and domain frames. That is not a maintainable trajectory and was the visible source of the Phase-118b post-mortem's open question.

This ADR resolves that question by reading what the codebase *actually does*, not what either interpretation might prefer.

### Decision

**Phase 118 is Disambiguation. The Wikidata alias index is a metadata sidecar over `aer_gold.entities`, not the canonical entity registry.**

Specifically:

1. **All NER spans land unconditionally in `aer_gold.entities`** (`NamedEntityExtractor.extract_all` in `services/analysis-worker/internal/extractors/entities.py`). This table is the canonical record of every entity surface form the system has ever observed.
2. **`aer_gold.entity_links` rows exist only for spans that the alias index successfully resolves above the 0.7 confidence threshold** — the table is proportional to *linked* entities, not to *all* entities (ROADMAP Phase 118 line: *"important during early Probe 0/1 operation when the linked rate is expected to be low"*).
3. **The Phase-102 `EntityCoOccurrenceExtractor` operates on `(entity_text, entity_label)` tuples** and has no QID input. Co-occurrence networks are built from surface forms; QIDs never enter the storage pipeline.
4. **The BFF read path uses `LEFT JOIN`, never `INNER JOIN`** — `services/bff-api/internal/storage/entities_query.go` and `cooccurrence_query.go`. Unlinked entities surface with `wikidataQid = null`. The cooccurrence-handler comment is explicit: *"entity linking is a metadata layer over the canonical `aer_gold.entities` data, not a load-bearing dependency."*
5. **The frontend types declare `wikidataQid?: string | null`**; consumers handle the null branch (currently the View-Mode cells render string-keyed nodes whether or not a QID is attached).

Coverage of the alias index is therefore **scoped, not exhaustive**. WP-002 §4.2's footnote codifies this directly: *"the type-bucket scope is curated for institutional editorial discourse and may under-cover entities salient outside that frame."*

### Consequences

* **Positive:** The architecture absorbs a low link rate gracefully. Unlinked entities (Tagesschau as `Q703907`/`Q15416 television program`; Der Spiegel as `Q131478`/`Q41298 magazine`; locally-prominent journalists; long-tail social-media figures) appear as string-keyed co-occurrence nodes — analytically usable, just not collapsed into a canonical QID. Bucket curation has a defined ceiling: high-precision political/institutional entities for the institutional-discourse domain, nothing more. Adding a new probe in a new language does not require a synchronous bucket-curation push for that language's news landscape — the system continues to function on string-keyed nodes while the curation catches up (or never does, if the value is judged insufficient).
* **Positive:** Phase-118 maintenance burden is bounded. The quarterly rebuild is automation-driven; YAML changes are PR-driven; misses do not block analytics. The implicit "coverage debt" that Discovery would impose disappears.
* **Negative:** Disambiguation precision is the only dimension that matters, and it is currently Tier-1.5-heuristic (sitelink-count tiebreaker, no annotation-validated weights). A wrong link (e.g. "Merkel" → an unrelated `Q…` named Merkel because of bad sitelink ranking) is a *worse* outcome than no link, because it produces a false canonical clustering in the co-occurrence view. This is the Tier-2 work documented in WP-002 §4.2 footnote¹ and `aer_gold.metric_validity` (`entity_link_confidence` row, `validation_status='unvalidated'`).
* **Negative:** Operators reading dashboards must understand that "entities not in the index" are not "entities the system missed" — they are "entities not yet collapsed to canonical IDs." This is a documentation burden, addressed by the Operations Playbook section "Reading the entity-linking confidence column" (cross-link to be added when the playbook lands the section).

**Non-goals.** This ADR does not commit to the current YAML bucket scope as the long-term architecture. A future Phase-119/120 may migrate to a Wikipedia-sitelink-based scope that eliminates per-domain YAML curation; that migration is in scope of a separate ADR if it ships, and is consistent with the Disambiguation framing — Wikipedia-scope is one mechanism for choosing *which* surface forms get a canonical ID, not a redefinition of what the index *is*.

### What this rules out

* The Wikidata index is **not** a "registry of relevant entities for AĒR." Operators must not interpret missing entities as out-of-scope-for-analysis.
* Future code must **not** introduce paths that drop unlinked entities. INNER JOIN against `entity_links` is forbidden in the BFF storage layer; lint-equivalent: any handler that depends on `wikidata_qid != ''` for correctness has misunderstood the architecture.
* Bucket curation is **not** a multi-language, multi-domain coverage commitment. Buckets exist to make the precision of the institutional-political-discourse linking high; their absence in other domains is acceptable per the design.

### Decision Record

- **Drafted:** 2026-05-04 by Fabian Quist with AĒR, after the Phase-118b post-mortem revealed the Disambiguation-vs-Discovery question had never been written down explicitly.
- **Status:** Accepted. The codebase already implements Disambiguation consistently across worker, BFF, and frontend; this ADR codifies the existing implementation rather than directing a change.

---
