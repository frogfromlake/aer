// Presentation-definition data table — extracted from registry.ts (Phase 141).
// Pure data; registry.ts owns the types + accessors. `import type` is
// circular-safe (erased at runtime).
import type { PresentationDefinition } from './registry';

export const PRESENTATIONS: readonly PresentationDefinition[] = [
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
    // The only presentation with a Silver-layer (per-document) query path.
    supportsSilver: true,
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
      'maxNodes',
      'networkChannels',
      'forceStrength',
      'settleTime',
      'showLabels',
      'showEdges',
      'displayLanguage'
    ],
    supportsAtScale: true,
    negativeSpacePolicy: 'overlay',
    // Phase 148g — the WebGL renderer is the SINGLE co-occurrence renderer; it is
    // always selected via `supportsAtScale` (the old d3-force SVG cell is retired).
    // `loadComponent` points here too so the default-renderer slot resolves to the
    // same component (it is never actually rendered for co-occurrence).
    loadComponent: async () =>
      (await import('$lib/components/presentations/CoOccurrenceNetworkAtScale.svelte')).default
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
