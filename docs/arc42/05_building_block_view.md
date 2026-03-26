# 5. Building Block View

Since AĒR is in an early development phase, only the highest abstraction level (Level 1) is documented here. The system follows a strict pipeline architecture (data flow from left to right).

## 5.1 Whitebox AĒR Overall System (Level 1)

The system is divided into four logical main building blocks (layers) communicating via defined databases/queues. There are no direct, synchronous dependencies between data collection and data analysis.

| Building Block (Layer) | Primary Tech | Responsibility (Blackbox Description) |
| :--- | :--- | :--- |
| **1. Ingestion Layer** | Go | A fleet of independent, lightweight microservices (Crawlers/API Clients). Each service is responsible for exactly one external data source. They collect unstructured data and store it in raw format (Bronze). |
| **2. Storage Layer** | S3 (MinIO) / Postgres / ClickHouse | The central nervous system for data. Unstructured raw data lands in MinIO (Object Storage) alongside PostgreSQL (Metadata index). Analytical, calculated metrics (time-series) are stored efficiently in ClickHouse (Gold). |
| **3. Analysis Layer** | Python | Deterministic worker services. They fetch raw data, clean it (Silver), and apply linguistic/sociological models (e.g., N-gram counting, keyword tracking). Results are written to the analytical database. |
| **4. Serving Layer (BFF)** | Go | The Backend-for-Frontend. A performant API (REST or GraphQL) serving frontend read requests. It aggregates data from ClickHouse and delivers ready-to-use "weather map" metrics to the UI. |

## 5.2 Key Interfaces

* **Ingestion -> Storage:** Append-only. Ingestion services may only add data, never alter or delete it.
* **Storage -> Analysis:** Asynchronous. Python services read raw data via polling or event triggers, process it in memory, and write the results back as new records.
* **BFF -> Frontend:** Strict OpenAPI contract. The frontend is unaware of the data processing complexity; it merely requests ready-made sociological metrics for specific timeframes.