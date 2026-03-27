# 8. Cross-cutting Concepts

## 8.1 Development Roadmap (Phases)

AĒR is developed iteratively from "bottom to top" (data source to UI). Each phase must be fully functional before the next begins.

* **Phase 1: Foundation & Ingestion (Focus: Go & Infrastructure)**
    * Setup of the Docker infrastructure (MinIO, PostgreSQL).
    * Development of the first Go-based ingestion service (e.g., a generic REST API client).
    * *Milestone:* Raw data flows automatically into the Data Lake.

* **Phase 2: Processing & Analysis (Focus: Python)**
    * Setup of the Python environment for deterministic text analysis.
    * Implementation of the harmonization step (Bronze -> Silver).
    * Implementation of initial simple sociological/linguistic metrics (counting, keyword tracking).
    * *Milestone:* Analyzed metrics are successfully stored in ClickHouse.

* **Phase 3: Backend-for-Frontend (Focus: Go API)**
    * Development of the API layer reading from ClickHouse and serving clients efficiently.
    * Definition of OpenAPI/Swagger specifications.
    * *Milestone:* A responsive API delivers aggregated JSON responses.

* **Phase 4: Presentation / Dashboard (Focus: Frontend)**
    * Development of the UI (e.g., Vue.js, Svelte, or React).
    * Connection to the BFF and implementation of the "weather map" concept with Progressive Disclosure.

## 8.2 Testing Strategy

To guarantee the scientific reliability of the data, AĒR follows a rigorous testing strategy:

1. **Unit Testing (Logic):** Especially the harmonization and analysis algorithms in Python must be covered by strict unit tests. When a sentence enters the analysis module, the output must be 100% deterministic and verifiable.
2. **Integration Testing (Databases):** The interaction of Go services with databases (MinIO/PostgreSQL) is tested automatically in isolated test containers (e.g., using `Testcontainers`).
3. **Contract Testing (APIs):** Before the Frontend and BFF communicate, strict adherence to the OpenAPI contracts is enforced and tested.

## 8.3 Shared Foundations (DRY Principle)

To maintain a clean and scalable codebase across multiple Go microservices, AĒR utilizes a **Go Workspace** (`go.work`). 

* **The `pkg/` Module:** A central, local Go module residing at the root level. It encapsulates cross-cutting concerns that are identical across all Go services, such as:
    * **Structured Logging:** A centralized `log/slog` wrapper. It enforces consistent, machine-readable JSON logging in production and staging environments, while providing human-readable, ANSI-colored output for local development.
    * **Configuration Management:** Powered by `spf13/viper`. It unmarshals configurations into strongly-typed structs, providing flexible multi-stage support via environment variables (`APP_ENV`) and a graceful fallback to local `.env` files.
* **Benefits:** Microservices (e.g., `ingestion-api`, `bff-api`) import this local package. This enforces the DRY (Don't Repeat Yourself) principle and guarantees that changes to infrastructure adapters propagate instantly across the entire system.

## 8.4 Clean Architecture (Microservice Structure)

To ensure high maintainability, testability, and adherence to the Separation of Concerns principle, all Go microservices within AĒR (e.g., `ingestion-api`, `bff-api`) strictly follow a **Clean Architecture** directory structure:

* **`cmd/api/main.go`**: The entry point. It contains zero business logic. Its sole responsibility is to load the configuration, initialize external dependencies (logger, databases), perform Dependency Injection (DI), and start the service.
* **`internal/config/`**: Handles the mapping of environment variables (`.env`) to service-specific configuration structs using `viper`.
* **`internal/storage/`**: Contains infrastructure adapters (e.g., `postgres.go`, `minio.go`). These adapters abstract the complexities of external database drivers away from the core logic.
* **`internal/core/`**: Contains the pure business logic and orchestration. It defines the interfaces it needs and relies entirely on injected dependencies, making it highly unit-testable.

## 8.5 Infrastructure as Code (IaC) and Provisioning

AĒR strictly separates application logic from infrastructure provisioning. Microservices (Go/Python) must never attempt to create infrastructure components (e.g., databases, tables, or S3 buckets) upon startup. They must assume the required infrastructure is already present.

* **Local Environment:** Infrastructure components are orchestrated via `docker compose`. Dedicated initialization containers (e.g., `minio-init` using the `minio/mc` image) act as one-off jobs to provision required layers (like the `bronze` and `silver` buckets) immediately after the foundational services start.
* **Production Environment:** Provisioning will be managed by robust IaC tools (e.g., Terraform, OpenTofu) or Kubernetes Init Containers. This ensures immutability, auditability, and prevents race conditions across horizontally scaled microservices.

## 8.6 Observability and Distributed Tracing (Day-2 Operations)

AĒR utilizes a modern observability stack to ensure the complex, asynchronous data pipeline (Ingestion -> NATS -> Python -> ClickHouse) remains fully transparent and debuggable.

* **Standardization:** All microservices (Go and Python) are instrumented using the **OpenTelemetry (OTel)** standard. Data is sent to a central `otel-collector`.
* **Context Propagation:** When the `ingestion-api` triggers a process, a unique `Trace-ID` is generated. This ID is injected into the NATS message headers and extracted by the Python `analysis-worker`. This creates a unified, end-to-end trace spanning multiple independent services.
* **Visualization:** Traces are exported via the OTLP protocol to **Grafana Tempo**, allowing developers to identify bottlenecks and verify deterministic execution paths instantly. System metrics are aggregated via **Prometheus** and visualized alongside traces in **Grafana**.