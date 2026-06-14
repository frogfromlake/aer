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
  | 'networkChannels'
  | 'scatterAxes'
  | 'forceStrength'
  // Co-occurrence redesign — show/hide the edge (connection) lines. A nodes-only
  // view is far more readable for a dense map; relational structure still reads
  // from clustering + node placement.
  | 'showEdges'
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

const PRESENTATIONS: readonly PresentationDefinition[] = [
  {
    id: 'time_series',
    label: 'Time series',
    discipline: 'nlp',
    description: 'Per-source time series with uncertainty bands.',
    layout: 'per-source',
    usesMetric: true,
    usesResolution: true,
    usesNormalization: true,
    configurableParams: ['band', 'scales'],
    // Overlay (N per-source viridis lines on one canvas) is implemented only
    // here — the per-scope cells render one artefact and ignore it.
    supportsOverlay: true,
    negativeSpacePolicy: 'gap',
    loadComponent: async () =>
      (await import('$lib/components/presentations/TimeSeriesCell.svelte')).default
  },
  {
    id: 'distribution',
    label: 'Distribution',
    discipline: 'eda',
    description: 'Histogram + quantile summary across the scope.',
    layout: 'per-scope',
    usesMetric: true,
    usesResolution: false,
    configurableParams: ['bins', 'scales'],
    supportsFaceting: true,
    negativeSpacePolicy: 'badge',
    loadComponent: async () =>
      (await import('$lib/components/presentations/DistributionCell.svelte')).default
  },
  {
    id: 'cooccurrence_network',
    label: 'Co-occurrence network',
    discipline: 'network_science',
    description: 'Force-directed entity co-occurrence graph.',
    layout: 'per-scope',
    // Operates on entity pairs from `/entities/cooccurrence` — the active
    // metric is not consumed.
    usesMetric: false,
    usesResolution: false,
    configurableParams: [
      'topN',
      'networkChannels',
      'forceStrength',
      'settleTime',
      'showEdges',
      'displayLanguage'
    ],
    supportsAtScale: true,
    negativeSpacePolicy: 'overlay',
    loadComponent: async () =>
      (await import('$lib/components/presentations/CoOccurrenceNetworkCell.svelte')).default
  },
  // Phase 131 — Aleph-pillar paired-metric scatter. Synchronic (no time
  // axis): each point is one article positioned by two metrics, with optional
  // size/colour channels bound to further metrics. The single-metric picker
  // is hidden (usesMetric:false) — the x/y/size/colour pickers under
  // `scatterAxes` drive the cell instead.
  {
    id: 'metric_scatter',
    label: 'Scatter',
    discipline: 'metadata_mining',
    description: 'Per-article scatter — position, size, and colour each bound to a metric.',
    layout: 'per-scope',
    usesMetric: false,
    usesResolution: false,
    configurableParams: ['scatterAxes', 'scales'],
    supportsFaceting: true,
    negativeSpacePolicy: 'badge',
    loadComponent: async () =>
      (await import('$lib/components/presentations/ScatterCell.svelte')).default
  },
  // Phase 121 — Episteme-pillar topic view modes.
  // Both cells are metric-agnostic in terms of the active `metric` URL
  // parameter (BERTopic operates on the cleaned text, not on a Gold
  // metric). The lane shell still composes a content-catalog id of the
  // form `<presentation>_<metricName>` so each (cell × metric) pair has
  // its own Dual-Register entry; we ship one entry per first-class
  // metric (sentiment_score_sentiws, word_count, …) per language.
  {
    id: 'topic_distribution',
    label: 'Topic distribution',
    discipline: 'episteme',
    description: 'Per-language BERTopic ridgeline — what is being talked about, by volume.',
    layout: 'per-scope',
    usesMetric: false,
    usesResolution: false,
    loadComponent: async () =>
      (await import('$lib/components/presentations/TopicDistributionCell.svelte')).default
  },
  {
    id: 'topic_evolution',
    label: 'Topic evolution',
    discipline: 'episteme',
    description: 'Stream graph of topic volume over time — how the expressible shifts.',
    layout: 'per-scope',
    usesMetric: false,
    usesResolution: false,
    loadComponent: async () =>
      (await import('$lib/components/presentations/TopicEvolutionCell.svelte')).default
  },
  // Phase 122d.0 — Silent-Edit Observability (ADR-032). Two presentations
  // surface the same underlying `aer_gold.article_revisions` signal at
  // different time-grains:
  //
  //   `revision_activity` is the synchronic Aleph cell — "which source
  //   edits most right now". The BFF collapses the window to a single
  //   bucket and the cell renders one bar per source. The metric picker
  //   is hidden (`usesMetric: false`) because edits-per-source is the
  //   *only* quantity this cell answers.
  //
  //   `revision_timeline` is the diachronic Episteme cell — "how the
  //   edit-frequency curve moves over time". The BFF buckets the window
  //   on a calendar grain (daily / weekly / monthly) chosen by the
  //   shared resolution control.
  //
  // Neither cell exposes a normalization / overlay / band — all per-cell
  // configurable parameters from Phase 131 are inapplicable; the cell
  // surface is therefore declared as `configurableParams: []`.
  {
    id: 'revision_activity',
    label: 'Revision activity',
    discipline: 'metadata_mining',
    description:
      'Silent-edit activity per source for the current window (Wayback CDX + sitemap-lastmod). Synchronic snapshot.',
    layout: 'per-scope',
    usesMetric: false,
    usesResolution: false,
    configurableParams: [],
    loadComponent: async () =>
      (await import('$lib/components/presentations/RevisionActivityCell.svelte')).default
  },
  {
    id: 'revision_timeline',
    label: 'Revision timeline',
    discipline: 'episteme',
    description:
      'Silent-edit activity over time — how often each source revises, by daily / weekly / monthly bucket.',
    layout: 'per-scope',
    usesMetric: false,
    usesResolution: true,
    configurableParams: [],
    loadComponent: async () =>
      (await import('$lib/components/presentations/RevisionTimelineCell.svelte')).default
  },
  // Phase 122d.3 — Silent-Edit Discourse Shift (Episteme). Goes one level
  // deeper than `revision_timeline`: not HOW OFTEN a source edits, but what
  // the edits DO to the discourse — the mean sentiment delta + semantic
  // (topic) shift per source over the window, re-extracted from each
  // snapshot version. Metric-less; the resolution control buckets the
  // trajectory. The provisional delta backbones are disclosed in the
  // how-to-read note.
  {
    id: 'revision_discourse_shift',
    label: 'Discourse shift',
    discipline: 'episteme',
    description:
      'How silent edits move the discourse — mean sentiment delta and semantic (topic) shift per source over time.',
    layout: 'per-scope',
    usesMetric: false,
    usesResolution: true,
    configurableParams: [],
    loadComponent: async () =>
      (await import('$lib/components/presentations/RevisionDiscourseShiftCell.svelte')).default
  },
  // Phase 122d.3 — Rhizome coordinated-edit clusters. The relational reading
  // of silent edits: cross-source temporal coincidences on the same entity
  // (≥2 sources silently editing the same name in the same time bucket).
  // Metric-less; a disclosed coincidence, never a causal claim (WP-003 §5).
  {
    id: 'revision_edit_clusters',
    label: 'Edit clusters',
    discipline: 'network_science',
    description:
      'Coordinated cross-source silent edits — which entities ≥2 sources quietly changed in the same time bucket.',
    layout: 'per-scope',
    usesMetric: false,
    usesResolution: true,
    configurableParams: [],
    loadComponent: async () =>
      (await import('$lib/components/presentations/RevisionEditClustersCell.svelte')).default
  },
  // Phase 124 — Rhizome cross-probe temporal lead-lag. A relational artefact
  // over a probe PAIR: the lagged cross-correlation of the two probes' hourly
  // publication activity. Metric-less (the signal is publication activity, not
  // a chosen Gold metric), so the single-metric picker is hidden. Gated
  // server-side on the temporal Level-1 equivalence grant; an ungranted pair
  // renders the refusal surface.
  {
    id: 'cross_probe_lead_lag',
    label: 'Lead-lag',
    discipline: 'network_science',
    description:
      'Cross-probe temporal lead-lag — does one culture lead the other in publication rhythm?',
    layout: 'per-scope',
    usesMetric: false,
    usesResolution: false,
    configurableParams: [],
    loadComponent: async () =>
      (await import('$lib/components/presentations/LeadLagCell.svelte')).default
  },
  // Phase 133 — categorical metadata distribution (Aleph, synchronic). One bar
  // per category value of a categorical metadata field (section / author / tags
  // / …), ranked by distinct-article count. Field-driven: the single-metric
  // picker is hidden (usesMetric:false) and PanelControls offers categorical
  // fields from `/scope/available-metadata` instead (usesMetadataField:true).
  // The chosen field rides in `Panel.metric`. `topN` caps the visible bars.
  {
    id: 'categorical_distribution',
    label: 'Category distribution',
    discipline: 'metadata_mining',
    description:
      'Article count per category of a metadata field (section, author, tags, …) — what kinds of articles the scope is made of.',
    layout: 'per-scope',
    usesMetric: false,
    usesMetadataField: true,
    usesResolution: false,
    configurableParams: ['topN'],
    supportsFaceting: true,
    negativeSpacePolicy: 'badge',
    loadComponent: async () =>
      (await import('$lib/components/presentations/CategoricalDistributionCell.svelte')).default
  },
  // Phase 125 — pairwise Pearson correlation matrix (Aleph, synchronic). An
  // N×N heatmap over a chosen metric set (`Panel.metricSet`); the single-metric
  // picker is hidden (usesMetric:false) and the `metricSet` lever drives it.
  // Drill a cell → scatter for that metric pair.
  {
    id: 'correlation_matrix',
    label: 'Correlation matrix',
    discipline: 'metadata_mining',
    description:
      'Pairwise Pearson correlation across a chosen set of metrics — which move together, which are independent.',
    layout: 'per-scope',
    usesMetric: false,
    usesResolution: false,
    configurableParams: ['metricSet'],
    supportsFaceting: true,
    negativeSpacePolicy: 'badge',
    loadComponent: async () =>
      (await import('$lib/components/presentations/CorrelationMatrixCell.svelte')).default
  },
  // Phase 125 — cross-tab (Aleph, synchronic): a categorical metadata field ×
  // a numeric metric → the metric's mean per category value. Field-driven
  // (usesMetadataField:true → Group-by picker); the numeric metric binds via
  // the `crossMetric` lever (Panel.channels.x).
  {
    id: 'cross_tab',
    label: 'Cross-tab',
    discipline: 'metadata_mining',
    description:
      'A metric broken down by a metadata category — e.g. mean sentiment per section. Bars ranked by article count, coloured by the metric.',
    layout: 'per-scope',
    usesMetric: false,
    usesMetadataField: true,
    usesResolution: false,
    configurableParams: ['crossMetric', 'topN'],
    supportsFaceting: true,
    negativeSpacePolicy: 'badge',
    loadComponent: async () =>
      (await import('$lib/components/presentations/CrossTabCell.svelte')).default
  },
  // Phase 125 — generalised metric lead-lag (Rhizome, relational): does xMetric
  // lead yMetric over the scope? Channel-driven (usesMetric:false); the two
  // metrics bind via the `leadLagAxes` lever (Panel.channels.x/.y).
  {
    id: 'metric_lead_lag',
    label: 'Lead-lag (metrics)',
    discipline: 'metadata_mining',
    description:
      'Lagged cross-correlation of two metrics over time — does one consistently lead the other?',
    layout: 'per-scope',
    usesMetric: false,
    usesResolution: false,
    configurableParams: ['leadLagAxes'],
    supportsFaceting: true,
    negativeSpacePolicy: 'badge',
    loadComponent: async () =>
      (await import('$lib/components/presentations/MetricLeadLagCell.svelte')).default
  },
  // Phase 125 — parallel coordinates (Aleph, multivariate): one polyline per
  // article across N metric axes. Channel-driven (usesMetric:false); the axis
  // set is the `metricSet` lever (Panel.metricSet, shared with the matrix).
  {
    id: 'parallel_coordinates',
    label: 'Parallel coordinates',
    discipline: 'metadata_mining',
    description:
      'One line per article across several metric axes — read clusters and crossing patterns across many dimensions at once.',
    layout: 'per-scope',
    usesMetric: false,
    usesResolution: false,
    configurableParams: ['metricSet'],
    supportsFaceting: true,
    negativeSpacePolicy: 'badge',
    loadComponent: async () =>
      (await import('$lib/components/presentations/ParallelCoordinatesCell.svelte')).default
  },
  // Phase 125 — Sankey/alluvial (Rhizome, relational): article flows across an
  // ordered chain of categorical metadata fields. Channel-driven
  // (usesMetric:false); the field chain is the `sankeyFields` lever
  // (Panel.fieldChain — Phase 125a split it out of metricSet). Lazy-loads d3-sankey.
  {
    id: 'sankey',
    label: 'Sankey',
    discipline: 'metadata_mining',
    description:
      'Article flows between categories across an ordered chain of metadata fields — where the corpus splits and merges.',
    layout: 'per-scope',
    usesMetric: false,
    usesResolution: false,
    configurableParams: ['sankeyFields'],
    negativeSpacePolicy: 'badge',
    loadComponent: async () =>
      (await import('$lib/components/presentations/SankeyCell.svelte')).default
  }
];

/** All presentation-form definitions, in display order. */
export function listPresentations(): readonly PresentationDefinition[] {
  return PRESENTATIONS;
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
  return found ?? DEFAULT_PRESENTATION;
}

/** Compose the cell id used by the content catalog (see
 *  `services/bff-api/configs/content/{en,de}/view_modes/`). */
export function cellContentId(presentation: Presentation, metricName: string): string {
  return `${presentation}_${metricName}`;
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

/** Lookup pillar definition by id; null/unknown returns the Aleph default. */
export function getPillar(id: PillarId | null): PillarDefinition {
  return PILLAR_DEFINITIONS.find((p) => p.id === id) ?? DEFAULT_PILLAR;
}

/** Presentations available for the active pillar, in display order. */
export function presentationsForPillar(id: PillarId | null): PresentationDefinition[] {
  const pillar = getPillar(id);
  const out: PresentationDefinition[] = [];
  for (const presId of pillar.presentations) {
    const p = PRESENTATIONS.find((x) => x.id === presId);
    if (p) out.push(p);
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
    if (found) return found;
  }
  const defaultId = pillarDef.presentations[0] ?? DEFAULT_PRESENTATION.id;
  return PRESENTATIONS.find((p) => p.id === defaultId) ?? DEFAULT_PRESENTATION;
}
