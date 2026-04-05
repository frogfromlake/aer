-- Migration 003: Create aer_gold.entities table for Named Entity Recognition output.
-- Phase 42: Provisional Tier 1 Metrics — NER Extractor (spaCy de_core_news_lg).
--
-- Stores raw entity spans extracted from SilverCore.cleaned_text.
-- Entity linking (resolving spans to canonical identifiers) is NOT implemented.
-- The entity_label values follow spaCy's German NER taxonomy: PER, ORG, LOC, MISC.

CREATE TABLE IF NOT EXISTS aer_gold.entities (
    timestamp DateTime,
    source String,
    article_id Nullable(String),
    entity_text String,
    entity_label String,
    start_char UInt32,
    end_char UInt32
) ENGINE = MergeTree()
ORDER BY (timestamp, source)
TTL timestamp + INTERVAL 365 DAY;
