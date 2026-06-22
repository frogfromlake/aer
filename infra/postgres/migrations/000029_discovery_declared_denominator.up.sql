-- Phase 148d (WP-007) — measured declared denominator for completeness.
--
-- ADR-031's discovery telemetry (migration 000018) records, per
-- (source, channel) per run, how many URLs AĒR surfaced (`urls_discovered`,
-- after the temporal-window filter) and how many were unique
-- (`urls_after_dedup`). It had NO measured denominator — only a hand-set
-- `expected_floor_per_run` heuristic — so it could not express a
-- completeness *ratio* (`completeness = collected / declared`), only gross
-- underflow.
--
-- WP-007 §4.1 turns completeness from a guess into a measurement: the
-- publisher-declared, in-window inventory counted at the channel's parse
-- boundary, BEFORE AĒR's filters. `declared` is that denominator.
--
-- `declared_indeterminate` is the honest companion: TRUE when the declared
-- count is only a LOWER BOUND and therefore cannot be trusted as the full
-- in-window inventory — a fetch/parse error swallowed entries, a walk/fetch
-- cap truncated the channel, or the channel surfaced advertised-but-undatable
-- content. When TRUE the BFF reports completeness as *indeterminate*
-- (Negative Space), never a clean ratio and never 100 % (WP-007 §3, §5). This
-- is the structural guarantee that AĒR never asserts a completeness certainty
-- it cannot measure.
--
-- Nullable + no default backfill: rows written before this migration carry
-- NULL `declared` (denominator was never measured for them) and the BFF
-- treats NULL as indeterminate, distinct from a measured 0.
BEGIN;

ALTER TABLE crawler_discovery_runs
    ADD COLUMN declared INTEGER CHECK (declared IS NULL OR declared >= 0),
    ADD COLUMN declared_indeterminate BOOLEAN NOT NULL DEFAULT FALSE;

COMMENT ON COLUMN crawler_discovery_runs.declared IS
    'WP-007 §4.1 completeness denominator: publisher-declared in-window item '
    'count measured at the channel parse boundary, before AĒR filters. NULL = '
    'never measured (pre-148d rows) → treated as indeterminate by the BFF.';

COMMENT ON COLUMN crawler_discovery_runs.declared_indeterminate IS
    'WP-007 §3/§5: TRUE when `declared` is only a lower bound (fetch/parse '
    'error, walk/fetch cap, or undatable content) → completeness reported as '
    'indeterminate, never a clean ratio.';

COMMIT;
