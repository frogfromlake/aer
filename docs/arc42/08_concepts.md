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