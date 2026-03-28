# 9. Architecture Decisions

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