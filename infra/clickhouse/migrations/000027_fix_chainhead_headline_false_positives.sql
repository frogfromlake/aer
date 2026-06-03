-- Migration 027 — correct chain-head headline false positives.
-- Phase 122d.1 bugfix (precursor to Phase 122d.3 Silent-Edit work).
--
-- Bug
-- ---
-- The Phase-122d.1 chain-head diff (revision_index = 0) compares the
-- current Silver body — wrapped by `_silver_text_to_html`, which carries
-- NO `<title>` element — against the oldest Wayback snapshot. The diff
-- writer asserted `headline_changed` with the rule
-- `prev_headline != curr_headline and bool(curr_headline)`, which fires
-- whenever `prev_headline` is empty. Since the Silver wrapper ALWAYS
-- extracts an empty headline, EVERY chain-head pair was stamped with a
-- spurious "− (empty) + <title>" headline change (109/109 chain-head
-- rows on the first non-German probe at the time of writing).
--
-- The writer is fixed in `article_revisions_diff.compute_diff` (a
-- headline change now requires a real title on BOTH sides). That fix
-- only governs FUTURE diffs; rows already written are terminal under the
-- diff sweep's `length(diff_paragraphs)=0` re-processing gate, so this
-- migration corrects the persisted Gold rows directly.
--
-- Safety
-- ------
-- `headline_changed = true AND headline_before = ''` uniquely identifies
-- the bug: every GENUINE headline change carries a non-empty
-- `headline_before` (a change cannot be asserted without a known
-- baseline — the same invariant the writer now enforces). The update is
-- idempotent: after it runs, no row matches the predicate, so a re-apply
-- (or a no-op on a freshly-reset, empty table) changes nothing. We also
-- clear `headline_after` to match `compute_diff`'s contract (both title
-- fields are empty when `headline_changed=false`).
--
-- `mutations_sync = 2` makes the mutation block until applied on all
-- replicas so the migration runner's success implies the data is fixed.

ALTER TABLE aer_gold.article_revisions
    UPDATE
        headline_changed = false,
        headline_before = '',
        headline_after = ''
    WHERE headline_changed = true
      AND headline_before = ''
    SETTINGS mutations_sync = 2;
