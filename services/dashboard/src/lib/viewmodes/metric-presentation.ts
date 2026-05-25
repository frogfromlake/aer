// Metric → presentation compatibility map — Phase 130 / ADR-035.
//
// The View-Mode Matrix is `availableMetrics × presentations` (see
// `registry.ts`). Not every cell of that product is meaningful: a cyclic
// metric like `publication_hour` is a distribution over a bounded domain
// (0..23), not a series along the corpus time-axis; `temporal_distribution`
// is *only* a series. Rendering the nonsensical pairings (`publication_hour`
// as a time-series, `temporal_distribution` as a distribution) would
// produce charts that *look* analysable but are not.
//
// This module is the single source of truth for which presentations a
// metric supports. The catalog is filtered through it (PanelControls hides
// the metrics that the active presentation cannot render), and the pillar
// each metric is reachable in follows automatically from
// `PILLAR_DEFINITIONS` (the presentation, not the metric, decides the
// pillar — ADR-035).
//
// Adding a metric: scalar metrics need no entry (they fall through to the
// default). Only metrics with a non-scalar shape (cyclic, series-only,
// pair-only) need an explicit entry.

import type { ViewMode } from '$lib/state/url-internals';

/** Presentations a scalar metric supports: a value-per-article that can be
 *  binned into a distribution (Aleph) or averaged along time (Episteme). */
const SCALAR_PRESENTATIONS: readonly ViewMode[] = ['distribution', 'time_series'];

/** Explicit per-metric overrides. Anything not listed here is treated as a
 *  scalar metric and gets `SCALAR_PRESENTATIONS`. */
const METRIC_PRESENTATION_OVERRIDES: Readonly<Record<string, readonly ViewMode[]>> = {
  // Cyclic metrics live on a bounded, repeating domain (hour-of-day,
  // day-of-week). They are distributions over that domain — never a series
  // along the corpus time-axis.
  publication_hour: ['distribution'],
  publication_weekday: ['distribution'],
  // A pre-binned temporal histogram is, by construction, a time-series and
  // nothing else.
  temporal_distribution: ['time_series'],
  // Entity co-occurrence is pair-shaped: it renders only as the relational
  // network cell (Rhizome). It never appears in the scalar metric picker
  // because `cooccurrence_network` declares `usesMetric: false`.
  entity_cooccurrence: ['cooccurrence_network']
};

/** The presentations the given metric sensibly supports, in registry order
 *  preference (distribution before time_series for scalars). */
export function presentationsForMetric(metricName: string): readonly ViewMode[] {
  return METRIC_PRESENTATION_OVERRIDES[metricName] ?? SCALAR_PRESENTATIONS;
}

/** Whether the (metric, presentation) pairing is meaningful. */
export function metricSupportsPresentation(metricName: string, view: ViewMode): boolean {
  return presentationsForMetric(metricName).includes(view);
}

// ---------------------------------------------------------------------------
// Pure-count classification — merged-cross-probe guard (Brief §1.3).
//
// A `merged` composition that pools more than one probe onto a single
// shared axis turns co-presence into comparison: "context A has more X than
// context B". For an *intensive* / *scaled* metric (a sentiment average, a
// confidence score) that shared axis is a ranking and is refused. For a
// *pure count* / *extensive* metric (article counts, word counts) the
// merged total is a legitimate sum over the unioned corpus, not a ranking,
// so the merge is permitted.
// ---------------------------------------------------------------------------

/** Metrics whose values are extensive counts — summable across contexts
 *  without implying a comparison. Everything else is intensive/scaled. */
const PURE_COUNT_METRICS: ReadonlySet<string> = new Set([
  'word_count',
  'entity_count',
  'publication_hour',
  'publication_weekday',
  'temporal_distribution'
]);

/** Whether the metric is a pure (extensive) count — the only class for
 *  which a merged cross-probe pooling is permitted (Brief §1.3). */
export function isPureCountMetric(metricName: string): boolean {
  return PURE_COUNT_METRICS.has(metricName);
}
