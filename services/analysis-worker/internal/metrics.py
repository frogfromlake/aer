from prometheus_client import Counter, Gauge, Histogram

events_processed_total = Counter(
    "events_processed_total",
    "Total number of events successfully processed through the pipeline.",
)

events_quarantined_total = Counter(
    "events_quarantined_total",
    "Total number of events moved to the dead letter queue (quarantine).",
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
