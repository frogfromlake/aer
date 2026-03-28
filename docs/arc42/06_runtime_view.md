# 6. Runtime View

This view describes the deterministic path of data through the AĒR system. To preserve scientific integrity, these processes are strictly sequential and observable via OpenTelemetry.

## 6.1 Data Ingestion & Processing Flow (Happy Path)

1. **Ingestion (Go):** The `ingestion-api` generates or collects a raw dataset. It creates an ingestion job in PostgreSQL, uploads the JSON to MinIO (`bronze` bucket), and logs the `trace_id` and file path in Postgres.
2. **Event Trigger (MinIO -> NATS):** MinIO detects the new object and automatically publishes a notification to NATS JetStream (`aer.lake.bronze`).
3. **Harmonization (Python):** The `analysis-worker` receives the event, fetches the raw file, and validates it strictly against the Silver Contract. If valid, it uploads the cleaned data to the `silver` bucket.
4. **Analytics (Python -> ClickHouse):** The worker extracts numerical metrics from the data and inserts them as time-series points into the `aer_gold.metrics` table in ClickHouse.

## 6.2 Error Handling & Resilience (Dead Letter Queue Flow)

To ensure the pipeline never crashes on unpredictable web data:

1. **Malformed Data Ingested:** The `ingestion-api` uploads a corrupt or non-compliant document.
2. **Validation Failure:** The `analysis-worker` attempts to map the data to the Silver Schema (Pydantic) and fails.
3. **Quarantine:** Instead of crashing, the worker catches the validation error, aborts the pipeline for this specific event, and routes the raw file to the `bronze-quarantine` bucket (Dead Letter Queue) for manual inspection.

## 6.3 Data Serving & Progressive Disclosure

This sequence describes how the frontend retrieves data and traces it back to the source.

1. **Dashboard Load:** The UI requests time-series metrics from the `BFF API` (e.g., `GET /api/v1/metrics`).
2. **OLAP Query:** The BFF fetches the aggregated data points from ClickHouse and returns them as strictly typed JSON.
3. **Drill-Down:** A sociologist clicks on a specific anomaly in the UI chart.
4. **Trace Resolution:** The UI queries the PostgreSQL Metadata Index using the timestamp/source to resolve the `trace_id` and the exact `bronze_object_key`.
5. **Raw Data Access:** The original unstructured document can be retrieved from MinIO for qualitative review, guaranteeing absolute transparency.