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

import type { Presentation } from '$lib/state/url-internals';

/** Presentations a scalar metric supports: a value-per-article that can be
 *  binned into a distribution (Aleph) or averaged along time (Episteme). */
const SCALAR_PRESENTATIONS: readonly Presentation[] = ['distribution', 'time_series'];

/** Explicit per-metric overrides. Anything not listed here is treated as a
 *  scalar metric and gets `SCALAR_PRESENTATIONS`. */
const METRIC_PRESENTATION_OVERRIDES: Readonly<Record<string, readonly Presentation[]>> = {
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
export function presentationsForMetric(metricName: string): readonly Presentation[] {
  return METRIC_PRESENTATION_OVERRIDES[metricName] ?? SCALAR_PRESENTATIONS;
}

/** Whether the (metric, presentation) pairing is meaningful. */
export function metricSupportsPresentation(metricName: string, view: Presentation): boolean {
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
  'temporal_distribution',
  // Phase 133 — scalar-metadata EVENT counts are extensive (summable across
  // contexts), so a merged cross-probe pooling is a legitimate total, not a
  // ranking. Deliberately NOT listed: `paywall_status` (per-article 0/1 whose
  // merged aggregate is a PROPORTION — intensive) and `reading_time_minutes`
  // (a per-article DURATION — closer to an intensive per-article quantity than
  // an event count; summed reading-minutes across probes is not a clean total,
  // so the merged-cross-probe guard conservatively refuses it).
  'image_count',
  'external_citation_count',
  'comment_count'
]);

/** Whether the metric is a pure (extensive) count — the only class for
 *  which a merged cross-probe pooling is permitted (Brief §1.3). */
export function isPureCountMetric(metricName: string): boolean {
  return PURE_COUNT_METRICS.has(metricName);
}

// ---------------------------------------------------------------------------
// Integer-valued classification — display formatting (Phase 133 A).
//
// Some metrics carry an integer per article (counts, cyclic ordinals); the
// rest are continuous (sentiment averages, confidence scores). For an
// integer metric the histogram's bin EDGES are pure binning artefacts —
// 30 equal divisions of [min, max] land on fractional boundaries like
// `34.133` that (a) are meaningless for an integer quantity and (b) a
// reader using "." as a thousands separator misreads as "34133". Rounding
// those edges to integers in the readout / axis kills both problems; the
// `count` (article tally) is unaffected. Continuous metrics keep decimals.
// ---------------------------------------------------------------------------

/** Metrics whose per-article value is an integer (counts + cyclic ordinals).
 *  Their histogram bin edges and axis ticks are rendered as integers.
 *  `raw_entity_count` is the Silver-layer alias of `entity_count`. */
const INTEGER_VALUED_METRICS: ReadonlySet<string> = new Set([
  'revision_count',
  'word_count',
  'entity_count',
  'raw_entity_count',
  'publication_hour',
  'publication_weekday',
  // Phase 133 — scalar metadata are integer-valued (counts, minutes) or a 0/1
  // flag (paywall_status), so their histogram bin edges + axis ticks render as
  // integers.
  'image_count',
  'external_citation_count',
  'comment_count',
  'reading_time_minutes',
  'paywall_status'
]);

/** Whether the metric's underlying values are integers — drives integer
 *  bin-edge / tick formatting in the distribution cell. */
export function isIntegerMetric(metricName: string): boolean {
  return INTEGER_VALUED_METRICS.has(metricName);
}

// ---------------------------------------------------------------------------
// Metadata-derived metrics — picker grouping (Phase 133, Issue 4).
//
// These ride the normal metric rails but are publisher-declared structural
// metadata (image count, paywall flag, …), not the analytical NLP/structural
// measures (sentiment, entity_count, word_count). They carry less analytical
// weight, so the picker surfaces them under a separate "Metadata" group rather
// than mixed in with sentiment et al. Mirror of the worker's
// SCALAR_METADATA_METRIC_NAMES (services/analysis-worker/internal/metadata_metrics.py).
// ---------------------------------------------------------------------------

/** Scalar metadata metrics promoted to Gold by the processor (Phase 133). */
const SCALAR_METADATA_METRIC_NAMES: ReadonlySet<string> = new Set([
  'image_count',
  'paywall_status',
  'external_citation_count',
  'comment_count',
  'reading_time_minutes'
]);

/** Whether the metric is a promoted scalar-metadata dimension (vs an analytical
 *  NLP/structural measure) — drives the picker's "Metadata" grouping. */
export function isMetadataMetric(metricName: string): boolean {
  return SCALAR_METADATA_METRIC_NAMES.has(metricName);
}

// ---------------------------------------------------------------------------
// Panel subject kind (Phase 148f) — what `Panel.metric` actually denotes.
//
// `Panel.metric` is overloaded: for a metric-driven view it is a metric; for a
// field-driven view (`usesMetadataField`) it holds a categorical FIELD name
// (e.g. `author`); for every channel-driven / multivariate / metric-agnostic
// view it is left UNRECONCILED by the view-switch logic (panel-controls-derive)
// and may be a stale value from a previously-active view. So `Panel.metric` is
// untrusted: classification is driven by the PRESENTATION flags, never by the
// string. Consumers (CellMethodology fetch-gating, the reading guide's MEASURE
// note) use this to decide whether to fetch metric provenance/content, field
// content, or neither — which is why field-driven views no longer 404 on
// `/metrics/{field}/provenance`.
// ---------------------------------------------------------------------------
export type PanelSubjectKind = 'metric' | 'field' | 'none';

/** Classify what the panel's subject (`Panel.metric`) denotes for a given
 *  presentation. `'metric'` only when the view reliably binds a real metric
 *  (`usesMetric`); `'field'` for field-grouped views (`usesMetadataField`);
 *  `'none'` otherwise (channel-driven / multivariate / topic / revision /
 *  co-occurrence — their real subject lives in channels/fieldChain/topics, not
 *  in `Panel.metric`, so no single-metric methodology is fetched). */
export function panelSubjectKind(pres: {
  usesMetric?: boolean;
  usesMetadataField?: boolean;
}): PanelSubjectKind {
  if (pres.usesMetadataField) return 'field';
  if (pres.usesMetric) return 'metric';
  return 'none';
}
