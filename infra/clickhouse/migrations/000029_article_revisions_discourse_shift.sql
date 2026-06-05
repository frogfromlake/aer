-- Migration 029 — Silent-Edit Discourse Shift: re-extraction deltas.
-- Phase 122d.3.
--
-- Phase 122d.0 (migration 000023) made the EXISTENCE of revisions
-- queryable; Phase 122d.1 (migration 000024) made the SUBSTANCE (the
-- paragraph diff + headline change) queryable. This migration makes the
-- DISCOURSE SHIFT across an edit queryable — what the silent re-write did
-- to sentiment, to the named-entity set, and to the article's semantic
-- vector.
--
-- The values are produced by the Phase-122d.1 revision-diff sweep
-- (`corpus.run_revision_diff_sweep`), which already holds both snapshot
-- HTMLs in memory per pair — re-extraction piggybacks on it with no
-- second Wayback fetch. The deltas are computed strictly
-- later-in-time minus earlier-in-time (see the sign-convention note in
-- the sweep; the chain-head pair compares the current article body
-- against the newest snapshot, so "later" there is the current article).
--
-- Additive only — no ORDER BY change, no engine change, no breaking
-- semantics. The Phase-122d.0/.1 row shape is fully preserved; rows
-- written before this migration ran (and identical re-archivals, and
-- rows not yet re-extracted) read the defaults with `deltas_computed=false`.
--
-- Provisional-classification discipline (every Probe metric is
-- `validation_status=unvalidated`): the deltas are engineering defaults,
-- not validated measurements. The backbones are the ones pinned in
-- `services/analysis-worker/configs/language_capabilities.yaml`:
-- * `sentiment_delta` — `cardiffnlp/twitter-xlm-roberta-base-sentiment`
--   (the multilingual news-class backbone, chosen for cross-probe
--   comparability), scalar in [-1, 1]; the delta lies in [-2, 2].
-- * `topic_shift_score` — cosine distance of `intfloat/multilingual-e5-large`
--   embeddings of the two snapshot texts. Honest name: it measures the
--   article's SEMANTIC shift, NOT a topic-label switch (BERTopic is a
--   corpus-level fit with no per-snapshot label). Range [0, 2], typically
--   [0, 1] for normalised embeddings.
-- * `entities_added` / `entities_removed` — set difference of NER surface
--   spans (`{de,fr,en}_core_news_lg`) between the two snapshots.
--
-- Column typing — sentinel + flag, NOT Nullable
-- ---------------------------------------------
-- `sentiment_delta`/`topic_shift_score` default 0, but 0 is a legitimate
-- "no change" value, so `deltas_computed Bool` disambiguates "0 because
-- unchanged" from "0 because not (yet) computed". BFF aggregates filter
-- `WHERE deltas_computed` so identical re-archivals and pending rows
-- never pollute trajectory averages. This matches the table's existing
-- all-`DEFAULT` style (no Nullable columns) and keeps `avg()`/`countIf()`
-- clean.
--
-- TTL anchors
-- -----------
-- Inherited from migration 000023 — the new columns share the row's
-- `snapshot_at + INTERVAL 365 DAY` TTL.

ALTER TABLE aer_gold.article_revisions
    ADD COLUMN IF NOT EXISTS sentiment_delta Float64
        DEFAULT 0;

ALTER TABLE aer_gold.article_revisions
    ADD COLUMN IF NOT EXISTS entities_added Array(String)
        DEFAULT [];

ALTER TABLE aer_gold.article_revisions
    ADD COLUMN IF NOT EXISTS entities_removed Array(String)
        DEFAULT [];

ALTER TABLE aer_gold.article_revisions
    ADD COLUMN IF NOT EXISTS topic_shift_score Float64
        DEFAULT 0;

ALTER TABLE aer_gold.article_revisions
    ADD COLUMN IF NOT EXISTS deltas_computed Bool
        DEFAULT false;
