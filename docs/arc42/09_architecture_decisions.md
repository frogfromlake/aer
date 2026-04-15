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
