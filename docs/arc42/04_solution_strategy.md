# 4. Solution Strategy

To achieve the goals defined in Chapter 1—specifically scientific integrity through transparency (Ockham's Razor) and strict modularity—AĒR utilizes a polyglot microservice architecture.

The fundamental strategy relies on the strict separation of data collection, data storage, and data analysis.

## 4.1 Technology Decisions

| Technology | Domain | Justification (Why?) |
| :--- | :--- | :--- |
| **Go (Golang)** | Ingestion Layer & BFF | Go offers outstanding concurrency (Goroutines). It is ideal for acting as a "vacuum" to execute thousands of asynchronous HTTP requests (APIs, Scraping) efficiently and in parallel. |
| **Python** | Analysis & Processing Layer | Python possesses the most robust ecosystem for deterministic data science and linguistics (e.g., spaCy, NLTK). It is deliberately chosen to implement transparent statistical models instead of opaque black-box LLMs. |
| **ClickHouse** | Analytics Database | A column-oriented database (OLAP) providing extreme read performance for aggregated time-series data—mandatory for a high-performance "weather map" dashboard. |
| **MinIO (S3)** | Object Storage | Highly scalable storage for unstructured, raw JSON/Text dumps (The Data Lake). |
| **NATS (JetStream)** | Event Broker | An ultra-lightweight, high-performance messaging system. Replaces synchronous polling to enable real-time, asynchronous triggering between Go ingestion and Python analysis workers while ensuring data persistence. |
| **PostgreSQL** | Relational Database | Storage of system metadata, user management, and configuration states for the data lake. |
| **Docker** | Containerization | Isolation of all services. Ensures that local development (via WSL2/Ubuntu) and production environments are identical. |

## 4.2 Architecture Patterns (The Data Pipeline)

AĒR adapts a simplified version of the "Medallion Architecture" (Bronze, Silver, Gold) to ensure raw data is never tampered with:

1. **Ingestion (Go):** The Go microservices act fast and simple. They request external sources and store the data in its absolute raw format (Bronze Layer in MinIO). *No content modification occurs here.*
2. **Harmonization (Python):** MinIO automatically pushes an event to the NATS Message Broker whenever new raw data is ingested. A Python worker, acting as a NATS Consumer, instantly picks up the raw data, removes HTML boilerplate, standardizes timestamps, and structures the metadata. The data schema is unified (Silver Layer).
3. **Deterministic Analysis (Python):** Python services subscribe to the harmonized data. Here, sociological models are applied (e.g., framing identification, N-gram counting, deterministic sentiment analysis). Extracted metrics are stored in ClickHouse (Gold Layer).
4. **Backend-for-Frontend (Go):** The frontend communicates exclusively with a Go API that retrieves pre-calculated aggregations from ClickHouse with extreme performance.

## 4.3 Cross-cutting Solutions

* **Contract-First Design:** Communication between Go and Python services is strictly defined in advance via OpenAPI/Swagger specifications.
* **Progressive Disclosure (UI/UX):** The dashboard displays aggregated trends (Gold Layer). If an analyst clicks to drill down, the system must transparently trace the path back to the unaltered raw data string (Bronze Layer).