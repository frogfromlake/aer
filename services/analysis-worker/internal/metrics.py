from prometheus_client import Counter, Gauge, Histogram

events_processed_total = Counter(
    "events_processed_total",
    "Total number of events successfully processed through the pipeline.",
)

events_quarantined_total = Counter(
    "events_quarantined_total",
    "Total number of events moved to the dead letter queue (quarantine).",
)

# Phase 122e A26 / F-A26 — articles preserved in the archive layer
# (Silver MinIO envelope) but excluded from the analytics layer because
# their extracted `published_date` falls outside `WORKER_ANALYTICAL_WINDOW_DAYS`.
# Surfaces the archive-vs-analytics scope difference as a queryable signal
# (Phase 122f will read this alongside per-field metadata coverage).
analysis_worker_archived_only_total = Counter(
    "analysis_worker_archived_only_total",
    (
        "Articles preserved in the Silver MinIO archive but excluded from "
        "ClickHouse analytics inserts because their published_date is older "
        "than WORKER_ANALYTICAL_WINDOW_DAYS. "
        "These articles WERE observed and harmonized successfully — they "
        "are recorded in MinIO Silver as an audit-trail entry — but the "
        "analytics layer treats them as out of scope."
    ),
    ["source"],
)

event_processing_duration_seconds = Histogram(
    "event_processing_duration_seconds",
    "End-to-end processing duration per event in seconds.",
    buckets=(0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0),
)

dlq_size = Gauge(
    "dlq_size",
    "Current number of objects accumulated in the bronze-quarantine bucket.",
)

analysis_worker_poison_messages_total = Counter(
    "analysis_worker_poison_messages_total",
    "Messages that exhausted NATS redeliveries and were routed to the poison-pill DLQ.",
    ["reason"],
)

corpus_extraction_runs_total = Counter(
    "corpus_extraction_runs_total",
    "Number of corpus-extraction sweeps started, by extractor and outcome.",
    ["extractor", "outcome"],
)

corpus_extraction_duration_seconds = Histogram(
    "corpus_extraction_duration_seconds",
    "Duration of a single corpus-extraction sweep in seconds.",
    ["extractor"],
    buckets=(0.5, 1.0, 2.5, 5.0, 10.0, 30.0, 60.0, 300.0),
)

corpus_extraction_rows_written_total = Counter(
    "corpus_extraction_rows_written_total",
    "Total rows written by corpus extractors, by extractor and target table.",
    ["extractor", "table"],
)
