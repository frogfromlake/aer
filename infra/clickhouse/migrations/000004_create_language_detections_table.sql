-- Migration 004: Create aer_gold.language_detections table.
-- Phase 45: Persist Detected Language — stores ISO 639-1 language codes
-- alongside confidence scores from langdetect.
--
-- Each row represents one language candidate for a processed document.
-- rank=1 is the most likely language. Storing ranked candidates preserves
-- the full output of detect_langs() for downstream analysis.

CREATE TABLE IF NOT EXISTS aer_gold.language_detections (
    timestamp DateTime,
    source String,
    article_id Nullable(String),
    detected_language String,
    confidence Float64,
    rank UInt8
) ENGINE = MergeTree()
ORDER BY (timestamp, source)
TTL timestamp + INTERVAL 365 DAY;
