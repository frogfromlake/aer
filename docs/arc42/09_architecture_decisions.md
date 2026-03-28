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