-- Migration 010: Convert Gold fact tables to ReplacingMergeTree for idempotent writes.
-- Phase 74: Worker Idempotency — Dedup Gate (R-14).
--
-- The processor writes sequentially into three ClickHouse tables before setting
-- document_status = 'processed' in PostgreSQL. A NATS redelivery after a partial
-- success would double-insert rows. Plain MergeTree does not deduplicate.
-- ReplacingMergeTree(ingestion_version) keeps only the row with the highest
-- ingestion_version per ORDER BY tuple after a merge (or OPTIMIZE ... FINAL).
--
-- ingestion_version is a monotone UInt64 built from the deterministic MinIO
-- event timestamp (Unix nanoseconds). Redelivered events share the same event
-- time → identical version → deterministic collapse.
--
-- Strategy (per table): RENAME old → *_old, CREATE new ReplacingMergeTree,
-- INSERT INTO new SELECT from *_old with ingestion_version = 0 for legacy
-- rows, then DROP *_old. Nullable(String) article_id keys require
-- allow_nullable_key = 1 on the new tables.

-- ---------- aer_gold.metrics ----------
RENAME TABLE aer_gold.metrics TO aer_gold.metrics_old;

CREATE TABLE aer_gold.metrics
(
    timestamp           DateTime,
    value               Float64,
    source              String DEFAULT '',
    metric_name         String DEFAULT '',
    article_id          Nullable(String),
    discourse_function  String DEFAULT '',
    ingestion_version   UInt64 DEFAULT 0
)
ENGINE = ReplacingMergeTree(ingestion_version)
ORDER BY (article_id, metric_name)
TTL timestamp + INTERVAL 365 DAY
SETTINGS allow_nullable_key = 1;

INSERT INTO aer_gold.metrics
    (timestamp, value, source, metric_name, article_id, discourse_function, ingestion_version)
SELECT
    timestamp, value, source, metric_name, article_id, discourse_function, 0
FROM aer_gold.metrics_old;

DROP TABLE aer_gold.metrics_old;

-- ---------- aer_gold.entities ----------
RENAME TABLE aer_gold.entities TO aer_gold.entities_old;

CREATE TABLE aer_gold.entities
(
    timestamp           DateTime,
    source              String,
    article_id          Nullable(String),
    entity_text         String,
    entity_label        String,
    start_char          UInt32,
    end_char            UInt32,
    discourse_function  String DEFAULT '',
    ingestion_version   UInt64 DEFAULT 0
)
ENGINE = ReplacingMergeTree(ingestion_version)
ORDER BY (article_id, entity_label, start_char, end_char)
TTL timestamp + INTERVAL 365 DAY
SETTINGS allow_nullable_key = 1;

INSERT INTO aer_gold.entities
    (timestamp, source, article_id, entity_text, entity_label, start_char, end_char, discourse_function, ingestion_version)
SELECT
    timestamp, source, article_id, entity_text, entity_label, start_char, end_char, discourse_function, 0
FROM aer_gold.entities_old;

DROP TABLE aer_gold.entities_old;

-- ---------- aer_gold.language_detections ----------
RENAME TABLE aer_gold.language_detections TO aer_gold.language_detections_old;

CREATE TABLE aer_gold.language_detections
(
    timestamp           DateTime,
    source              String,
    article_id          Nullable(String),
    detected_language   String,
    confidence          Float64,
    rank                UInt8,
    ingestion_version   UInt64 DEFAULT 0
)
ENGINE = ReplacingMergeTree(ingestion_version)
ORDER BY (article_id, rank)
TTL timestamp + INTERVAL 365 DAY
SETTINGS allow_nullable_key = 1;

INSERT INTO aer_gold.language_detections
    (timestamp, source, article_id, detected_language, confidence, rank, ingestion_version)
SELECT
    timestamp, source, article_id, detected_language, confidence, rank, 0
FROM aer_gold.language_detections_old;

DROP TABLE aer_gold.language_detections_old;
