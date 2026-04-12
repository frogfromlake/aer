# Probe 0 — Temporal Profile (WP-005)

This document records the temporal characteristics of the Probe 0 sources and the resulting `min_meaningful_resolution` heuristics that the BFF API surfaces via `GET /api/v1/metrics/available`.

> **Status: provisional engineering heuristic.** WP-005 §3.3 calls for a measured signal-to-noise study to derive the minimum meaningful resolution per source. For Probe 0 the values were computed by simple division of publication rates rather than a measured study. They are advisory only — the BFF does not enforce them.

---

## Publication Rates

| Source | Approx. articles / day | Articles / hour (avg) | `min_meaningful_resolution` |
| :--- | :--- | :--- | :--- |
| `tagesschau.de` | ≈ 50 | ≈ 2.1 | `hourly` |
| `bundesregierung.de` | ≈ 5 | ≈ 0.2 | `daily` |

These values are reflected in the BFF static config map at `services/bff-api/internal/config/min_resolution.go` (Phase 66) and surfaced per metric via the `minMeaningfulResolution` field on `GET /api/v1/metrics/available`.

### Derivation

The heuristic divides the average publication rate by the bucket width and selects the coarsest resolution at which an average bucket still contains ≥ 1 document:

- **`hourly`** is the finest resolution at which tagesschau.de averages roughly two documents per bucket. Sub-hour buckets (`5min`) for tagesschau.de are mostly empty and noise-dominated.
- **`daily`** is the finest resolution at which bundesregierung.de averages roughly five documents per bucket. Hourly buckets for bundesregierung.de average less than one document and are dominated by the all-or-nothing publication schedule of the Federal Press Office.

This is a deliberately conservative heuristic. A measured study (WP-005 §3.3) would replace these single thresholds with per-metric values that account for the variance of each metric, not just the publication rate of the source.

## Editorial Publishing Schedule

Both sources publish primarily during German business hours and on weekdays. This editorial schedule shows up directly in the `publication_hour` and `publication_weekday` metrics produced by `TemporalDistributionExtractor` and must not be confused with discourse rhythms in the underlying population.

- **tagesschau.de** publishes throughout the day with concentrated bursts around the morning, noon, evening, and late-night news cycles.
- **bundesregierung.de** publishes mostly during regular government business hours; weekend publications are rare and typically reflect crisis communication.

A holiday-driven volume drop (e.g., on a federal holiday listed in `configs/cultural_calendars/de.yaml`) is **not** a discourse shift — it is the absence of publication activity. The cultural calendar exists precisely to disambiguate these two cases during manual interpretation.

## Cultural Calendar

The German cultural calendar lives at `configs/cultural_calendars/de.yaml`. It enumerates federal public holidays (recurring), federal election dates (one-off), religious observances, and recurring major media events. It is consumed as a static lookup by analysts when interpreting temporal patterns; it is not yet wired into the query layer.

See:

- `configs/cultural_calendars/de.yaml` — the calendar file itself.
- [Scientific Operations Guide → **Workflow 6: Updating the Cultural Calendar**](../../scientific_operations_guide.md#workflow-6-updating-the-cultural-calendar).
- [Operations Playbook → **Cultural Calendar Files**](../../operations_playbook.md#cultural-calendar-files).

## Tiered Retention (Planned, Not Yet Active)

WP-005 §5.4 proposes a tiered retention strategy in which raw 5-minute samples are kept short-term and progressively coarser pre-aggregated tables retain progressively longer history. The target tiers are documented in **Arc42 §8.8 (addendum)**.

> **Status: planned — not yet active.** Probe 0 currently runs on the flat 365-day TTL on `aer_gold.metrics`. Activation depends on the deferred materialized views in migration `000009`. Until then, the temporal profile of Probe 0 is bounded by that flat retention.

## Cross-References

- WP-005 — *Temporal Granularity of Discourse Shifts*
- Arc42 §8.13 — Multi-Resolution Temporal Framework
- Arc42 §8.8 (addendum) — Tiered Retention status
- `services/bff-api/internal/config/min_resolution.go` — `minMeaningfulResolution` config map
- `configs/cultural_calendars/de.yaml` — German cultural calendar
- [Scientific Operations Guide → **Workflow 6: Updating the Cultural Calendar**](../../scientific_operations_guide.md#workflow-6-updating-the-cultural-calendar)
