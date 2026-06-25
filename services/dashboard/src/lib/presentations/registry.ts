// View-Mode Matrix registry — Phase 107.
//
// The matrix has two axes (Design Brief §4.2.3):
//   - Analytical disciplines (NLP, EDA, Network Science, …)
//   - Presentation forms (time-series, distribution, force-directed graph, …)
//
// A "cell" is a (discipline × presentation × metric) triple — concretely
// identified by `<presentation>_<metricName>` so the BFF content catalog
// can carry one Dual-Register entry per cell (see Arc42 §8.13).
//
// The registry below names the structural presentation forms; the cell
// catalog itself is dynamic — it is `availableMetrics × presentations`.
// The set of available metrics comes from `/api/v1/metrics/available`,
// not a hardcoded list. Brief §8.3 forbids hardcoding cells in the
// frontend; this file holds the rendering primitives only.
//
// Adding a new presentation form (e.g. `heatmap`, `correlation_matrix`):
//   1. Add a case to `Presentation` in `$lib/state/url-internals.ts`.
//   2. Add a `PresentationDefinition` entry below.
//   3. Implement the cell component under `$lib/components/presentations/`.
//   No FunctionLaneShell change is required if the cell is rendered via
//   the registry's `loadComponent` indirection.

import type { Component } from 'svelte';
import type { FetchContext } from '$lib/api/queries';
import type { Presentation, PillarId } from '$lib/state/url-internals';

export type AnalyticalDiscipline =
  | 'nlp'
  | 'eda'
  | 'network_science'
  | 'metadata_mining'
  | 'clustering'
  | 'episteme';

// Phase 131 — the per-cell configuration levers a presentation exposes.
// PanelControls renders the matching control for each entry a presentation
// declares in `configurableParams`, so the config surface stays in lockstep
// with what the cell can actually consume (no misleading no-op knobs).
//   bins             — distribution histogram bin count.
//   band             — time-series ±1σ uncertainty band toggle.
//   topN             — co-occurrence network top-edge cap.
//   networkChannels  — co-occurrence node size/colour channel binding.
//   scatterAxes      — scatter x/y/size/colour metric-dimension binding.
//   displayLanguage  — co-occurrence cross-lingual relabel (source ↔ viewer,
//                      Phase 123b): swap QID-linked node labels to the viewer's
//                      language; unlinked nodes keep their source surface form.
export type CellParamKind =
  | 'bins'
  | 'band'
  | 'topN'
  // Phase 148g — co-occurrence node-first breadth (top-N entities by weight).
  | 'maxNodes'
  | 'networkChannels'
  | 'scatterAxes'
  | 'forceStrength'
  // Co-occurrence redesign — show/hide the edge (connection) lines. A nodes-only
  // view is far more readable for a dense map; relational structure still reads
  // from clustering + node placement.
  | 'showEdges'
  // Phase 148g — co-occurrence node labels on/off (default on; uniform distance).
  | 'showLabels'
  // Phase 148g — co-occurrence label-density filter (top N% by size/colour).
  | 'labelFilter'
  // Co-occurrence redesign — large-scale layout settle time (seconds): how long
  // ForceAtlas2 runs before freezing. Lets the user give a big map more time to
  // relax into clusters.
  | 'settleTime'
  | 'displayLanguage'
  // Phase 124 — axis-scale mode (shared vs free) for multi-cell value-axis
  // panels. PanelControls renders the toggle for the presentations that carry
  // a comparable value axis (distribution, time_series, metric_scatter).
  | 'scales'
  // Phase 125 — N-metric set picker for multivariate cells (correlation_matrix,
  // parallel_coordinates). A multi-select of scope-available metrics persisted
  // in `Panel.metricSet`.
  | 'metricSet'
  // Phase 125 — a single numeric-metric picker for the cross-tab cell (the
  // metric aggregated per category value). Persisted in `Panel.channels.x`.
  | 'crossMetric'
  // Phase 125 — two numeric-metric pickers (x leads y) for the metric lead-lag
  // cell. Persisted in `Panel.channels.x` / `.y`.
  | 'leadLagAxes'
  // Phase 125 — an ordered multi-select of categorical FIELDS for the Sankey
  // cell (the alluvial chain). Persisted in `Panel.fieldChain` (Phase 125a
  // split it out of the overloaded `metricSet`).
  | 'sankeyFields';

/** One presentation-form axis entry. The matrix-cell id is composed at
 *  call-sites as `<id>_<metricName>` to match content-catalog yaml keys. */
export interface PresentationDefinition {
  /** URL-stable key. Mirrors `Presentation` in `url-internals.ts`. */
  id: Presentation;
  /** Short human label for the switcher. */
  label: string;
  /** Discipline this presentation defaults to (matrix row). The same
   *  presentation can serve multiple disciplines — this is the canonical
   *  pairing used by the MVP cells. */
  discipline: AnalyticalDiscipline;
  /** One-line description for the switcher tooltip. */
  description: string;
  /** Whether this presentation renders one chart per source (per-source
   *  small-multiples) or one chart per scope (a single aggregate view). */
  layout: 'per-source' | 'per-scope';
  /** Lazy loader for the cell component. Each presentation lives in its
   *  own chunk so heavy libraries (Observable Plot, d3-force) only land
   *  on demand — keeps the shell bundle gate green (Brief §7). */
  loadComponent: () => Promise<Component<PresentationCellProps>>;
  /** Does this presentation's cell consume the active `metric` prop?
   *  BERTopic cells (`topic_*`) operate on cleaned text; `cooccurrence_*`
   *  on entity pairs. For those, the metric selector is misleading — the
   *  Cell ignores it. PanelControls hides the Metric row when this is
   *  false. Defaults to `true`. */
  usesMetric?: boolean;
  /** Phase 133 — this presentation consumes a categorical metadata FIELD
   *  (section / author / tags / …), carried in `Panel.metric`, instead of a
   *  Gold metric. PanelControls draws its dimension picker from
   *  `/scope/available-metadata` rather than the metric list (and keeps the
   *  single-metric picker hidden via `usesMetric: false`). Defaults to false. */
  usesMetadataField?: boolean;
  /** Does this presentation's cell consume the active `resolution` prop?
   *  Only time-series-shaped cells (per-source small-multiples on a time
   *  axis) honour resolution today. PanelControls and EpistemeShell hide
   *  the Resolution control when no active view honours it. Defaults to
   *  `false`. */
  usesResolution?: boolean;
  /** Does this presentation's cell consume the `normalization` prop (the
   *  Compare lever: raw / deviation / percentile)? Only the time-series cell
   *  threads it into its query today; everywhere else the control was a no-op,
   *  so PanelControls hides the Compare row unless this is true. Defaults to
   *  `false`. */
  usesNormalization?: boolean;
  /** Phase 131 — the per-cell configuration levers this presentation's cell
   *  consumes. PanelControls renders exactly these (and nothing else) so the
   *  config surface never offers a knob the cell ignores. Absent / empty =
   *  no configurable parameters. */
  configurableParams?: readonly CellParamKind[];
  /** Phase 131 (bugfix) — whether the `overlay` composition is meaningful for
   *  this presentation. Overlay (N per-source lines on one shared canvas) is
   *  only implemented for the time-series cell; every other (per-scope) cell
   *  renders one artefact regardless, making overlay indistinguishable from
   *  merged. PanelControls hides the Overlay button when this is false.
   *  Defaults to false. */
  supportsOverlay?: boolean;
  /** Phase 125a — whether this presentation supports faceting / small-multiples
   *  (breaking the cell into one sub-cell per categorical value via a
   *  `metadataFilter`). True only for the per-article presentations whose cells
   *  thread `metadataFilter` into their query (distribution, categorical
   *  distribution, scatter, cross-tab, correlation matrix, metric lead-lag,
   *  parallel coordinates). PanelControls offers the facet picker only when
   *  true; PanelHost ignores `facetField` otherwise. Defaults to false. */
  supportsFaceting?: boolean;
  /** Phase 151 — whether this presentation has a Silver-layer (per-document)
   *  query path. True ONLY for `distribution` (DistributionCell routes to the
   *  Silver aggregation endpoint); every other cell renders a "not available on
   *  Silver" notice. PanelControls offers the Au-Gold / Ag-Silver layer lever
   *  ONLY when true, and PanelHost/PanelCellGrid coerces the effective layer to
   *  gold otherwise — so the toggle never appears where it has no effect.
   *  Defaults to false. */
  supportsSilver?: boolean;
  /** Phase 125b + co-occurrence redesign — whether this presentation has a
   *  large-scale (WebGL) renderer variant. True only for `cooccurrence_network`
   *  (the sigma.js at-scale map). PanelHost auto-routes to
   *  CoOccurrenceNetworkAtScale on a SINGLE cell once the Top-N lever crosses
   *  ~500 edges (no Maximize dependency); below that, the default SVG cell.
   *  Defaults to false. */
  supportsAtScale?: boolean;
  /** Phase 122d.2 / ADR-039 — how this presentation treats the Negative-Space
   *  toggle, declared so the per-cell contract is uniform (no silent no-op).
   *    overlay → ghost-render the excluded NS-artefacts (cooccurrence ghost edges)
   *    gap     → break continuity where NS-density is high (time-series)
   *    badge   → mark/annotate affected items (per-article cells)
   *    refuse  → render the refusal surface (aggregate over a structurally-absent
   *              dimension — runtime condition, not a static default)
   *    no-op   → the toggle is inert for this view, WITH an explanatory note
   *  Defaults to 'no-op'. PanelHost surfaces a self-disclosing per-panel note
   *  from this policy when the toggle is on; the actual data-bearing renderings
   *  live in the cell (cooccurrence ghost), the article list (∅ badges) and the
   *  globe/dossier. */
  negativeSpacePolicy?: 'overlay' | 'badge' | 'gap' | 'refuse' | 'no-op';
}

/** Common props passed to every cell. The cell decides which subset
 *  it needs (e.g. distribution / cooccurrence ignore `sources`). */
export interface PresentationCellProps {
  ctx: FetchContext;
  scopeProbeId: string;
  /** Resolved scope: probe id or single source name. */
  scopeId: string;
  scope: 'probe' | 'source';
  /** RFC 3339 window bounds, or `undefined` for the whole dataset (no time
   *  filter — time-limiting is an optional feature, not the default). Cells
   *  that render a time axis must derive their domain from the returned data
   *  when these are absent. */
  windowStart: string | undefined;
  windowEnd: string | undefined;
  metricName: string;
  /** Concrete sources within the active scope — used by per-source cells
   *  (time-series renders one panel per source). */
  sources: ReadonlyArray<{ name: string; emicDesignation: string | null | undefined }>;
  /** Phase 111 — Silver-layer toggle. When `silver`, cells route queries
   *  to /api/v1/silver/* where a matching aggregation exists. Cells that
   *  have no Silver equivalent render a "not available" notice.
   *  Defaults to `gold` when absent. */
  dataLayer?: 'gold' | 'silver';
  /** Phase 114 — Multi-probe composition set. When non-empty, cells
   *  include all probes in BFF queries via the `probeIds` parameter so
   *  the backend unions their sources. Absent = single-probe scope. */
  probeIds?: string[];
  /** Phase 148g — the merged unit's explicit source allowlist (per ScopeGroup),
   *  threaded to the co-occurrence cell so a cross-probe merge unions exactly the
   *  scoped sources via the multi-scope POST. Empty/undefined = all sources of the
   *  scope's probes. */
  sourceIds?: readonly string[] | undefined;
  /** Phase 122i revision (D1). Cells that historically fanned out per
   *  source (`TimeSeriesCell`) need to know whether the panel intends
   *  `merged` (one chart over the unioned scope) or `split` (one chart
   *  per source). Per-scope cells (DistributionCell, Topic*, CoOccurrence*)
   *  ignore this prop — they always query the unioned scope and render
   *  one artefact. Absent = legacy fan-out behaviour. */
  composition?: 'merged' | 'split' | 'overlay';
  // Phase 131 — per-cell configuration, threaded from `Panel` by PanelHost.
  // Each cell reads only the levers it declares in `configurableParams`. The
  // explicit `| undefined` keeps these assignable under the project's
  // `exactOptionalPropertyTypes` when PanelHost forwards an unset Panel field.
  /** Distribution histogram bin count (default 30 when absent). */
  bins?: number | undefined;
  /** Co-occurrence network top-edge cap (default 60 when absent). */
  topN?: number | undefined;
  /** Phase 148g — co-occurrence node-first breadth: the top-N most-frequent
   *  entities (by weight) to show, edges drawn among them. 0/undefined keeps the
   *  legacy edge-first behaviour (nodes are a by-product of the top-N edges). */
  maxNodes?: number | undefined;
  /** Visual-channel binding — scatter axes/size/colour, network size/colour. */
  channels?: import('$lib/state/url-internals').CellChannelBinding | undefined;
  /** Phase 125 — the N-metric set for multivariate cells (correlation_matrix,
   *  parallel_coordinates). Threaded from `Panel.metricSet`. */
  metricSet?: readonly string[] | undefined;
  /** Phase 125a — the ordered categorical field chain for the sankey cell.
   *  Threaded from `Panel.fieldChain` (split out of the overloaded metricSet). */
  fieldChain?: readonly string[] | undefined;
  /** Phase 125a — faceting / small-multiples restriction. When set, this cell is
   *  one facet sub-cell: its per-article query is restricted to articles whose
   *  `field` carries `value`. Injected per render-unit by PanelHost; absent on a
   *  normal (non-faceted) cell. */
  metadataFilter?: { field: string; value: string } | undefined;
  /** Phase 125b — cross-panel linked brushing. A Window-level transient set of
   *  selected article ids, shared across the panels of a Window. Per-article
   *  cells (scatter, parallel coordinates) emphasise selected articles and toggle
   *  on click; aggregating cells ignore it. NOT URL-persisted (transient). */
  selection?:
    | {
        ids: ReadonlySet<string>;
        toggle: (articleId: string) => void;
        clear: () => void;
      }
    | undefined;
  /** Time-series ±1σ uncertainty band; undefined = shown. */
  showBand?: boolean | undefined;
  /** Co-occurrence — show/hide edge (connection) lines; undefined = shown.
   *  Network cells only. */
  showEdges?: boolean | undefined;
  /** Phase 148g — co-occurrence node labels on/off; undefined = on. All labels
   *  render at the same distance (no LOD mix). Network cell only. */
  showLabels?: boolean | undefined;
  /** Phase 148g — label-density filter: only the top N% of nodes (ranked by
   *  `labelRankBy`) are labelled; undefined/100 = all. Network cell only. */
  labelTopPercent?: number | undefined;
  labelRankBy?: 'size' | 'colour' | undefined;
  /** Time-series temporal bucketing (Episteme Resolution lever). Per-panel;
   *  the time-series cell previously read the global URL resolution, ignoring
   *  the panel control. */
  resolution?: import('$lib/state/url-internals').Resolution | undefined;
  /** Normalization mode (Compare lever): raw | zscore | percentile. Consumed
   *  by the time-series cell only. */
  normalization?: import('$lib/state/url-internals').Normalization | undefined;
  /** Co-occurrence force-layout spread (0..100; default 50). Higher = stronger
   *  repulsion = more spread-out graph. Network cell only. */
  forceStrength?: number | undefined;
  /** Co-occurrence large-scale (WebGL) layout settle time in seconds; how long
   *  ForceAtlas2 runs before freezing. Default 12. At-scale renderer only. */
  settleSeconds?: number | undefined;
  /** Phase 123b — co-occurrence cross-lingual relabel. 'viewer' relabels
   *  QID-linked nodes to the reader's language; 'source' / undefined keeps the
   *  source surface form. Network cell only. */
  displayLanguage?: 'source' | 'viewer' | undefined;
  // Phase 124 — shared-axis coordination across the cells of one panel.
  /** Stable per-rendered-cell key, assigned by PanelHost. Identifies this
   *  cell when it reports its extent so the host can union extents per axis. */
  cellKey?: string | undefined;
  /** Report this cell's own data extent on an axis (`value` = the metric axis,
   *  whatever its orientation; `x` = scatter's x-metric axis). Pass `null`
   *  when the cell has no data. PanelHost unions reported extents into
   *  `sharedDomains`. No-op when absent (legacy / non-coordinated render). */
  reportExtent?:
    | ((axis: 'value' | 'x', extent: readonly [number, number] | null) => void)
    | undefined;
  /** The union axis domains for the panel, supplied only in 'shared' scale
   *  mode. A value-axis cell applies `value` to its metric axis; the scatter
   *  cell additionally applies `x`. Absent = free scale (the cell auto-scales
   *  to its own data). */
  sharedDomains?:
    | { readonly value?: readonly [number, number]; readonly x?: readonly [number, number] }
    | undefined;
  // Phase 126 — true when this rendered cell carries a per-cell override that
  // differs from the panel default. Cells that compose a "how to read" note
  // pass it into the `configOverridden` fact so the note discloses the cell is
  // on a custom (not-directly-comparable) configuration.
  configOverridden?: boolean | undefined;
  /** Phase 124 — the panel's effective axis-scale state for this cell, used
   *  only for the "how to read" disclosure: 'shared' (on the union axis),
   *  'free' (multi-cell but independent — by user choice or the cross-context
   *  guard), or undefined (single-cell / not a value-axis panel — no note). */
  axisScaleState?: 'shared' | 'free' | undefined;
}

import { PRESENTATIONS } from './registry-data';
// Locale-resolution helpers (Phase 144b / ADR-042) — swap the English fallback
// label/description/blurb in the static tables for the active-UI-locale strings.
// Extracted to registry-i18n.ts to keep this file under the length cap.
import { localizePresentation, localizePillar } from './registry-i18n';

export function listPresentations(): readonly PresentationDefinition[] {
  return PRESENTATIONS.map(localizePresentation);
}

// Phase 130 — the registry-wide default is `distribution` (the default
// pillar Aleph's default presentation). Before Phase 130 this was
// `time_series`, which leaked a diachronic cell as the synchronic-pillar
// default; the fix moves `time_series` into Episteme and makes Aleph
// default to `distribution`.
const DEFAULT_PRESENTATION: PresentationDefinition =
  PRESENTATIONS.find((p) => p.id === 'distribution') ?? PRESENTATIONS[0]!;

/** Lookup by URL-stable id; returns the distribution default when the
 *  caller passed `null` (e.g. URL state was empty). */
export function getPresentation(id: Presentation | null): PresentationDefinition {
  const found = PRESENTATIONS.find((p) => p.id === id);
  return localizePresentation(found ?? DEFAULT_PRESENTATION);
}

/** Compose the cell id used by the content catalog (see
 *  `services/bff-api/configs/content/{en,de}/view_modes/`). */
export function cellContentId(presentation: Presentation, metricName: string): string {
  return `${presentation}_${metricName}`;
}

// The view_modes that ship a per-metric methodology entry
// (`<presentation>_<metric>.yaml`) in the content catalogue. The remaining
// presentations are channel-driven (scatter, lead-lag, sankey, parallel-
// coordinates, cross-tab) or metric-agnostic (revision_*), so `cellContentId`
// would resolve to an entry that does not exist — CellMethodology gates its
// per-metric view-mode content fetch on this set so it never fires a 404 for a
// cell that has no per-metric methodology. SoT for the entries:
// services/bff-api/configs/content/{en,de}/view_modes/.
const VIEW_MODES_WITH_PER_METRIC_CONTENT: ReadonlySet<Presentation> = new Set([
  'distribution',
  'time_series',
  'topic_distribution',
  'topic_evolution',
  'cooccurrence_network'
]);

/** Whether a `view_mode/<presentation>_<metric>` methodology entry exists in the
 *  content catalogue for this presentation (so the per-metric fetch is worth
 *  making). False for channel-driven / metric-agnostic views. */
export function hasCellMethodologyContent(presentation: Presentation): boolean {
  return VIEW_MODES_WITH_PER_METRIC_CONTENT.has(presentation);
}

/** Default metric the lane uses when a presentation requires a metric
 *  but none is otherwise specified. Matches the Phase 106 baseline so
 *  switching presentations on a lane with no explicit metric still
 *  yields a meaningful render. */
// Phase 117 renamed `sentiment_score` → `sentiment_score_sentiws` to make
// ADR-016's dual-metric pattern lexically explicit. The default the
// dashboard prepends to the metric picker matches the canonical name so
// the legacy alias never surfaces in the UI (the BFF's alias filter would
// drop it from `/metrics/available`, but the dashboard would re-introduce
// it via this default).
export const DEFAULT_METRIC_NAME = 'sentiment_score_sentiws';

// Phase 123c (Issue 4) — the default metric for a CROSS-PROBE scope. The
// `DEFAULT_METRIC_NAME` (SentiWS) is German-only, so it yields empty cells on
// a panel that also holds a French source. A panel spanning more than one
// probe defaults instead to the multilingual sentiment backbone — the one
// sentiment metric every probe carries (the cross-probe comparison basis per
// the backbone strategy / ADR-016). Single-probe panels keep
// `DEFAULT_METRIC_NAME`.
export const CROSS_PROBE_DEFAULT_METRIC = 'sentiment_score_bert_multilingual';

// -------------------------------------------------------------------------
// Pillar mapping — Aleph / Episteme / Rhizome × Presentation
//
// The three pillars are the project's conceptual frame (Brief §3.1, WP-005
// §6). Each pillar bundles a curated subset of presentations so the user
// sees one coherent set of analytical lenses per pillar, rather than the
// full matrix at once. The Pillar switcher in the SideRail and the per-lane
// identity strip on Surface II L3 both consume this mapping.
//
// Mapping (strict 1-to-1, no overlap) — Phase 130 / ADR-035. The pillar
// is determined by the PRESENTATION, not the metric: a presentation is
// inherently synchronic (no time axis), diachronic (time is the axis), or
// relational, and lands in exactly one pillar accordingly. Metrics then
// flow through whichever presentations they support (see
// `metric-presentation.ts`).
//   Aleph    = synchronic totality ("the weather now")   → distribution, topic_distribution, metric_scatter
//   Episteme = diachronic register ("the climate record")→ time_series, topic_evolution
//   Rhizome  = relational currents ("currents between")  → cooccurrence_network
//
// Phase 130 corrected two leaks: `time_series` (inherently diachronic) sat
// in Aleph and moved to Episteme; `topic_distribution` (a synchronic
// snapshot of what is being talked about, by volume) sat in Episteme and
// moved to Aleph.
// -------------------------------------------------------------------------

export interface PillarDefinition {
  id: PillarId;
  label: string;
  abbr: string;
  glyph: string;
  /** One-line headline — shown next to the active pillar identity strip. */
  blurb: string;
  /** Two-to-three-sentence description shown when the user lands on a lane
   *  for the first time, or expands the identity strip. */
  description: string;
  /** Accent colour driving the identity strip border + glyph. */
  color: string;
  /** Ordered list of presentations available within this pillar. The first
   *  entry is the default viewMode when the pillar is freshly selected. */
  presentations: Presentation[];
}

export const PILLAR_DEFINITIONS: readonly PillarDefinition[] = [
  {
    id: 'aleph',
    label: 'Aleph',
    abbr: 'A',
    glyph: '◉',
    blurb: 'Synchronic totality — "the weather now"',
    description:
      'Every observed probe in scope, no time axis beyond the active window. Snapshot-oriented analyses: sentiment levels, lexical density, distributional shape, paired-metric structure, what is being talked about right now (by volume), and which sources are currently editing most (silent-edit activity).',
    color: '#5283b8',
    presentations: [
      'distribution',
      'topic_distribution',
      'metric_scatter',
      'categorical_distribution',
      'correlation_matrix',
      'cross_tab',
      'parallel_coordinates',
      'revision_activity'
    ]
  },
  {
    id: 'episteme',
    label: 'Episteme',
    abbr: 'E',
    glyph: '◐',
    blurb: 'Diachronic knowledge register — "the climate record"',
    description:
      'How the expressible shifts over time. Time is the axis: metric time-series, topic evolution, drift, and silent-edit activity over time — the long-term shape of what can be said within the discursive formation.',
    color: '#c8a85a',
    presentations: [
      'time_series',
      'topic_evolution',
      'revision_timeline',
      'revision_discourse_shift'
    ]
  },
  {
    id: 'rhizome',
    label: 'Rhizome',
    abbr: 'R',
    glyph: '◇',
    blurb: 'Relational propagation — "currents between contexts"',
    description:
      'How frames move. Entity co-occurrence, lead-lag, cross-probe diffusion — the relational substrate of the discourse.',
    color: '#9a8fb8',
    presentations: [
      'cooccurrence_network',
      'cross_probe_lead_lag',
      'metric_lead_lag',
      'sankey',
      'revision_edit_clusters'
    ]
  }
];

const DEFAULT_PILLAR: PillarDefinition = PILLAR_DEFINITIONS[0]!;

/** Lookup pillar definition by id; null/unknown returns the Aleph default.
 *  Blurb + description are resolved for the active UI locale. */
export function getPillar(id: PillarId | null): PillarDefinition {
  return localizePillar(PILLAR_DEFINITIONS.find((p) => p.id === id) ?? DEFAULT_PILLAR);
}

/** All pillar definitions in display order, blurb + description localized for
 *  the active UI locale. The PillarSwitch iterates this rather than the raw
 *  `PILLAR_DEFINITIONS` so its tooltips localize without a consumer change. */
export function listPillars(): readonly PillarDefinition[] {
  return PILLAR_DEFINITIONS.map(localizePillar);
}

/** Presentations available for the active pillar, in display order. */
export function presentationsForPillar(id: PillarId | null): PresentationDefinition[] {
  const pillar = getPillar(id);
  const out: PresentationDefinition[] = [];
  for (const presId of pillar.presentations) {
    const p = PRESENTATIONS.find((x) => x.id === presId);
    if (p) out.push(localizePresentation(p));
  }
  return out;
}

/** Default viewMode for the given pillar — the first presentation in its set. */
export function defaultPresentationForPillar(id: PillarId | null): Presentation {
  const pillar = getPillar(id);
  return pillar.presentations[0] ?? DEFAULT_PRESENTATION.id;
}

/** Reverse lookup: which pillar owns the given viewMode? Returns the pillar
 *  id, or null when the viewMode is not registered under any pillar (which
 *  should be impossible under the strict 1-1 mapping but keeps callers
 *  defensive against future presentation additions). */
export function pillarForPresentation(viewMode: Presentation): PillarId | null {
  const def = PILLAR_DEFINITIONS.find((p) => p.presentations.includes(viewMode));
  return def?.id ?? null;
}

/** Resolve the active presentation given URL state (viewMode, pillar).
 *
 * Rules:
 *   - If the URL viewMode is non-null AND belongs to the active pillar, use it.
 *   - Otherwise fall back to the pillar's default presentation.
 *
 * Without this resolution, a viewMode-less URL on the Episteme pillar would
 * render `time_series` (the registry-wide default), which is an Aleph cell
 * — visually breaking the pillar identity. */
export function resolvePresentation(
  viewMode: Presentation | null,
  pillar: PillarId | null
): PresentationDefinition {
  const pillarDef = getPillar(pillar);
  if (viewMode !== null && pillarDef.presentations.includes(viewMode)) {
    const found = PRESENTATIONS.find((p) => p.id === viewMode);
    if (found) return localizePresentation(found);
  }
  const defaultId = pillarDef.presentations[0] ?? DEFAULT_PRESENTATION.id;
  return localizePresentation(
    PRESENTATIONS.find((p) => p.id === defaultId) ?? DEFAULT_PRESENTATION
  );
}
