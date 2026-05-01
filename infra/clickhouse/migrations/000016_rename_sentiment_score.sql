-- Migration 016: Phase 117 — rename `sentiment_score` → `sentiment_score_sentiws`.
--
-- ADR-016's dual-metric pattern (Tier 1 deterministic SentiWS alongside the
-- forthcoming Tier 2 BERT extractors in Phase 119) is made lexically explicit
-- by suffixing the lexicon family. The rename is applied once across all Gold
-- tables that store metric names verbatim. The BFF carries a one-cycle
-- read-side alias (`sentiment_score` → `sentiment_score_sentiws`) so cached
-- dashboard URL state continues to resolve.
--
-- ClickHouse forbids `ALTER … UPDATE` on columns that are part of the
-- `ORDER BY` key. `metric_name` is in the sort key of all three target
-- tables, so the rewrite is expressed as INSERT-SELECT (new key) followed
-- by `ALTER … DELETE` (old key). Both steps are mutations: they return
-- immediately and run asynchronously.
--
-- Idempotency: the INSERT-SELECT filters on the source name, the DELETE
-- removes the old name. Re-running is a no-op once both have completed
-- because no rows remain with `metric_name='sentiment_score'`. The
-- ReplacingMergeTree dedup key already includes `metric_name`, so the
-- inserted rows are distinct from any existing `sentiment_score_sentiws`
-- rows produced by the post-rebuild worker.

INSERT INTO aer_gold.metrics
    (timestamp, value, source, metric_name, article_id, discourse_function, ingestion_version)
SELECT
    timestamp, value, source, 'sentiment_score_sentiws',
    article_id, discourse_function, ingestion_version
FROM aer_gold.metrics
WHERE metric_name = 'sentiment_score';

ALTER TABLE aer_gold.metrics
    DELETE WHERE metric_name = 'sentiment_score';

INSERT INTO aer_gold.metric_baselines
    (metric_name, source, language, baseline_value, baseline_std,
     window_start, window_end, n_documents, compute_date)
SELECT
    'sentiment_score_sentiws', source, language, baseline_value, baseline_std,
    window_start, window_end, n_documents, compute_date
FROM aer_gold.metric_baselines
WHERE metric_name = 'sentiment_score';

ALTER TABLE aer_gold.metric_baselines
    DELETE WHERE metric_name = 'sentiment_score';

INSERT INTO aer_gold.metric_equivalence
    (etic_construct, metric_name, language, source_type, equivalence_level,
     validated_by, validation_date, confidence, notes)
SELECT
    etic_construct, 'sentiment_score_sentiws', language, source_type, equivalence_level,
    validated_by, validation_date, confidence, notes
FROM aer_gold.metric_equivalence
WHERE metric_name = 'sentiment_score';

ALTER TABLE aer_gold.metric_equivalence
    DELETE WHERE metric_name = 'sentiment_score';
