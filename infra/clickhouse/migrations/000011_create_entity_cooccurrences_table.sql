-- Migration 011: Create aer_gold.entity_cooccurrences for Network Science view modes.
-- Phase 102: Iteration 5 — View-Mode Query Endpoints + EntityCoOccurrenceExtractor.
--
-- Backs the GET /api/v1/entities/cooccurrence endpoint (Network Science x
-- force-directed graph). Rows are emitted per-article by the
-- EntityCoOccurrenceExtractor (the first CorpusExtractor implementation,
-- ADR-020). The BFF aggregates by summing cooccurrence_count across articles
-- in a window — standard document-coupling network construction.
--
-- ingestion_version = nanoseconds at corpus-sweep start. Re-runs over the
-- same window emit identical (window_start, source, entity_a_text,
-- entity_b_text) tuples with a higher ingestion_version, and
-- ReplacingMergeTree collapses to the latest row on merge / FINAL.
--
-- entity_a_text is lexicographically <= entity_b_text per row, so an
-- unordered pair (A, B) has exactly one canonical representation.

CREATE TABLE IF NOT EXISTS aer_gold.entity_cooccurrences (
    window_start        DateTime,
    window_end          DateTime,
    source              String,
    article_id          String,
    entity_a_text       String,
    entity_a_label      String,
    entity_b_text       String,
    entity_b_label      String,
    cooccurrence_count  UInt32,
    ingestion_version   UInt64 DEFAULT 0
)
ENGINE = ReplacingMergeTree(ingestion_version)
ORDER BY (window_start, source, article_id, entity_a_text, entity_b_text)
TTL window_start + INTERVAL 365 DAY;
