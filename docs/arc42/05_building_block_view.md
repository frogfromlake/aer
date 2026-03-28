# 5. Building Block View

Since AĒR is in an early development phase, only the highest abstraction level (Level 1) is documented here. The system follows a strict pipeline architecture (data flow from left to right).

## 5.1 Whitebox AĒR Overall System (Level 1)

The system is divided into logical main building blocks communicating via defined databases and message brokers. There are no direct, synchronous HTTP dependencies between data collection, processing, and storage.

### 5.1.1 Ingestion API (Go)
* **Responsibility:** Acts as the entry point for raw data. Fetches unstructured data from external sources.
* **Implementation:** A Go microservice that writes raw documents to MinIO (Bronze Layer) and simultaneously logs the operation, source, and OpenTelemetry `trace_id` into the PostgreSQL Metadata Index.

### 5.1.2 Analysis Worker (Python)
* **Responsibility:** Deterministic data harmonization and metric calculation.
* **Implementation:** Subscribes to NATS JetStream. Validates raw data against the "Silver Contract" (Pydantic). Calculates sociological metrics (e.g., N-grams) and stores them in ClickHouse (Gold Layer). Routes malformed data to a Dead Letter Queue (DLQ).

### 5.1.3 BFF API / Serving Layer (Go)
* **Responsibility:** Provides a contract-first REST API to the frontend.
* **Implementation:** Decouples the frontend from direct database queries. Fetches aggregated time-series data from the ClickHouse `gold` layer using generated OpenAPI interfaces (`oapi-codegen`).

### 5.1.4 Storage & Event Core
* **MinIO (Object Storage):** Holds raw (`bronze`), cleaned (`silver`), and quarantined (`bronze-quarantine`) data. Acts as the event publisher.
* **PostgreSQL (Metadata Index):** The relational memory. Links MinIO paths to execution traces to enable "Progressive Disclosure".
* **ClickHouse (OLAP):** The high-performance analytical database storing calculated metrics.
* **NATS JetStream:** The central message broker ensuring decoupled, at-least-once delivery between Go and Python.