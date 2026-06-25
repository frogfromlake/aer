// URL-state types + enum consts — extracted from url-internals.ts (Phase 141).
// Leaf module (no imports); url-internals re-exports for the stable import path.
// Pure URL (de)serialisation helpers backing `url.svelte.ts`. Kept rune-
// free in their own module so vitest can import them without a Svelte
// compiler pass. The runes-based store lives in `url.svelte.ts` and
// re-exports these for component-side use.

export type Resolution = '5min' | 'hourly' | 'daily' | 'weekly' | 'monthly';
export type PillarId = 'aleph' | 'episteme' | 'rhizome';

// Phase 144 — UI locale (app-UI-language). The single set of localized UI
// languages; mirrors `project.inlang/settings.json` `locales`. Distinct from
// the viewer/content LabelLanguage (de/en/fr) used for QID relabelling — the
// UI shell ships en/de only. Carried in the URL as `?lang=` for deep-link /
// share; the resolved value (with localStorage + navigator fallback) is the
// `locale` rune in `state/locale.svelte.ts`, the single source of truth fed to
// Paraglide via `overwriteGetLocale`.
export type Locale = 'en' | 'de';
export const LOCALES: readonly Locale[] = ['en', 'de'];
export const DEFAULT_LOCALE: Locale = 'en';

/** Clamp a raw BCP-47 string ("de-DE", "DE", "de_AT") to a supported UI
 *  locale, or null. Lower-cases and takes the primary subtag. Pure — shared
 *  by the URL parser and the locale rune's localStorage/navigator fallback. */
export function clampLocale(raw: string | null | undefined): Locale | null {
  if (!raw) return null;
  const primary = raw.toLowerCase().split(/[-_]/)[0] ?? '';
  return (LOCALES as readonly string[]).includes(primary) ? (primary as Locale) : null;
}
// Phase 130 / ADR-035 — the Rhizome entry-question enum (`RhizomeView`:
// actors-topics / source-resonance / concept-migration / free-composition)
// was removed. Rhizome now uses the universal panels+cells model like Aleph
// and Episteme; its relational cells are ordinary `Presentation` choices.
// Presentation-form axis of the View-Mode Matrix (Brief §4.2.3 /
// reframing-note §3.2). MVP cells in Phase 107: time_series,
// distribution, cooccurrence_network. The catalog is extensible —
// new presentations are added here and registered in $lib/presentations/.
export type Presentation =
  | 'time_series'
  | 'distribution'
  | 'cooccurrence_network'
  | 'topic_distribution'
  | 'topic_evolution'
  // Phase 131 — paired-metric scatter (Aleph, synchronic). Visual channels
  // (x / y position, point size, point colour) are each bound to a chosen
  // metric dimension via `Panel.channels`, so the single-metric picker is
  // hidden for this presentation (registry `usesMetric: false`).
  | 'metric_scatter'
  // Phase 122d.0 — Silent-Edit Observability (ADR-032). Two presentations
  // because the same underlying signal answers two different questions:
  //   `revision_activity` (Aleph, snapshot) — "which source edits most
  //                                            right now"
  //   `revision_timeline` (Episteme, over-time) — "how edit activity
  //                                                drifts week-to-week"
  // The pillar-follows-presentation rule (ADR-035) admits the two cells
  // into Aleph and Episteme respectively without breaking the strict 1-1
  // pillar→presentation mapping.
  | 'revision_activity'
  | 'revision_timeline'
  // Phase 122d.3 — Silent-Edit Discourse Shift (Episteme, over-time). The
  // discourse-level reading of edits: how a source's edits move sentiment
  // and meaning across the window. Metric-less (usesMetric:false); the
  // signal is the re-extraction delta, not a chosen Gold metric.
  | 'revision_discourse_shift'
  // Phase 122d.3 — coordinated cross-source edit clusters (Rhizome,
  // relational). Cross-source temporally-clustered silent edits on shared
  // entities — the relational counterpart of `revision_discourse_shift`,
  // split into its own pillar per the strict 1-1 mapping (ADR-035).
  | 'revision_edit_clusters'
  // Phase 124 — cross-probe temporal lead-lag (Rhizome, relational). The
  // lagged cross-correlation of two probes' hourly publication activity;
  // metric-less (usesMetric:false) and inherently a probe-pair artefact.
  | 'cross_probe_lead_lag'
  // Phase 133 — categorical metadata distribution (Aleph, synchronic). Article
  // count per category value of a categorical metadata FIELD (section / author /
  // tags / …). Field-driven, not metric-driven (usesMetric:false,
  // usesMetadataField:true); the chosen field is carried in `Panel.metric`.
  | 'categorical_distribution'
  // Phase 125 — pairwise Pearson correlation matrix over an N-metric set
  // (Aleph, synchronic). Channel-driven (usesMetric:false); the metric set is
  // carried in `Panel.metricSet`. Drill a cell → scatter for that pair.
  | 'correlation_matrix'
  // Phase 125 — cross-tab of a categorical metadata FIELD × a numeric METRIC
  // (Aleph, synchronic): the metric's mean per category value. The field rides
  // in `Panel.metric` (usesMetadataField:true); the numeric metric binds to
  // `Panel.channels.x`.
  | 'cross_tab'
  // Phase 125 — generalised metric lead-lag (Rhizome, relational): the lagged
  // cross-correlation of two metrics' hourly series over one scope. The two
  // metrics bind to `Panel.channels.x` / `.y` (channel-driven, usesMetric:false).
  | 'metric_lead_lag'
  // Phase 125 — parallel coordinates (Aleph, multivariate): one polyline per
  // article across N metric axes (`Panel.metricSet`). Channel-driven
  // (usesMetric:false).
  | 'parallel_coordinates'
  // Phase 125 — Sankey/alluvial (Rhizome, relational): article flows across an
  // ordered chain of categorical metadata fields. The field chain is carried in
  // `Panel.metricSet` (reused as a generic ordered string list).
  | 'sankey';
// Data layer toggle (Phase 111). `gold` is the default (omitted from URL);
// `silver` routes Surface II queries to /api/v1/silver/* and enforces the
// WP-006 §5.2 eligibility gate. Only meaningful when a probe is selected.
export type DataLayer = 'gold' | 'silver';
// Normalization mode (Phase 115). `null` and `raw` are equivalent;
// `zscore` and `percentile` are URL-addressable so a deviation-labelled
// chart deep-links cleanly. The cross-frame equivalence gate is enforced
// server-side.
export type Normalization = 'raw' | 'zscore' | 'percentile';

// Phase 122i / ADR-034 — Multi-Panel Workbench state.
//
// The Workbench is a four-level tree:
//   pillar  → window  → panel  → scopeGroup
//
// A `ScopeGroup` is a slice of the corpus addressed by probe-ids + an
// optional source-id narrowing. A `Panel` is one analytical unit (view ×
// metric × layer × …) over 1..M ScopeGroups; the `composition` flag
// decides whether the ScopeGroups feed one merged Cell or one Cell each
// (split = small-multiples). A `WorkbenchWindow` holds 1..8 panels
// arranged side-by-side (4 at typical viewport widths; 5..8 horizontal-
// scroll). A `PillarState` holds 1..4 windows. Each Pillar persists its
// own state in the URL so a pillar-switch is non-destructive.
export type Composition = 'merged' | 'split' | 'overlay';

/**
 * Phase 148e / 149 — the composition a freshly-created or pillar-seeded Panel
 * opens with. `split` (per-scope small-multiples) is the honest default
 * everywhere, merge an explicit opt-in — EXCEPT the POOLED-RELATIONAL Rhizome
 * views, which only carry signal with all sources/probes in ONE cell:
 * `cooccurrence_network` (single graph; split refuses), `revision_edit_clusters`
 * (a cluster needs ≥2 sources in a cell → split's single-source cells are empty
 * by construction), `cross_probe_lead_lag` (needs a probe pair in a cell → split
 * shows "needs two probes"). Opening these split greets the user with a refusal /
 * empty / notice they never asked for, so they open `merged`.
 */
const MERGED_DEFAULT_VIEWS: ReadonlySet<Presentation> = new Set<Presentation>([
  'cooccurrence_network',
  'revision_edit_clusters',
  'cross_probe_lead_lag'
]);

export function defaultCompositionForView(view: Presentation): Composition {
  return MERGED_DEFAULT_VIEWS.has(view) ? 'merged' : 'split';
}

// Phase 148e/148g — co-occurrence opening lever values for a freshly-created /
// pillar-seeded panel (beyond composition). Labels now default OFF (a 10k-node map
// is unreadable with labels) and the settle cap is left UNSET so it auto-scales
// with the node count (`autoSettleSeconds`) — pinning a short settle starved big
// maps. The only seeded value is the label-density filter (top 10% by prominence),
// which takes effect WHEN the reader turns labels on; the lever still lets them
// adjust it.
export const DEFAULT_COOC_LABEL_TOP_PERCENT = 10;

/** Phase 148e — the initial per-cell lever values a panel of `view` opens with
 *  (spread into the new Panel by both create-mode and the pillar-switch seed).
 *  Empty for every non-co-occurrence presentation. */
export function initialLeversForView(view: Presentation): Pick<Panel, 'labelTopPercent'> {
  if (view === 'cooccurrence_network') {
    return { labelTopPercent: DEFAULT_COOC_LABEL_TOP_PERCENT };
  }
  return {};
}

// Phase 122i revision (D2). Split direction governs how a Panel arranges
// its small-multiples when composition='split'. Horizontal = cells
// side-by-side (default); vertical = stacked. Ignored when composition
// is 'merged'.
export type SplitDirection = 'horizontal' | 'vertical';

// Phase 124 — shared-axis comparison discipline. When a panel renders more
// than one cell of a value-axis presentation (distribution / time_series /
// metric_scatter), 'shared' (the default) puts every cell on the union axis
// domain so identical values plot at identical positions; 'free' drops each
// cell back to its own optimal domain (the ggplot/Vega facet escape hatch).
export type ScaleMode = 'shared' | 'free';

export interface ScopeGroup {
  // 1..K probe ids. Multi-probe entries are valid even though the
  // production corpus is single-probe today — the Cell-host unions them
  // when querying.
  probeIds: string[];
  // 0..L source ids; empty = "all sources of probeIds".
  sourceIds: string[];
}

// Phase 131 — visual-channel binding. Each visual channel (position / size /
// colour) of a configurable cell can be bound to a chosen dimension. For the
// scatter cell the channels bind to *metric* names; for the co-occurrence
// network they bind to a graph dimension (node weight vs. degree; colour by
// entity label, cross-source presence, or uniform).
// Phase 125 — `metric` sizes/colours nodes by the per-article metric named in
// `CellChannelBinding.netMetric`, aggregated (mean) over the articles where the
// entity appears (BFF `nodeMetric`). 'total_count'/'degree' stay graph-intrinsic.
export type NetworkSizeChannel = 'total_count' | 'degree' | 'metric';
// Phase 131a — `source_overlay` colours nodes & edges by their originating
// source from the BFF per-edge `presence` field. Available whenever the
// scope covers multiple sources; the cell auto-promotes it as the default
// for merged scopes.
export type NetworkColorChannel =
  | 'label'
  | 'presence'
  | 'uniform'
  | 'source_overlay'
  // Phase 148g — colour nodes by their owning PROBE (the cross-probe merge fill;
  // border keeps per-source provenance). An actor bridging probes reads grey.
  | 'probe_overlay'
  // Phase 125 — colour nodes by `CellChannelBinding.netMetric` (mean per-article
  // metric over the mentioning articles).
  | 'metric'
  // Co-occurrence redesign — colour nodes by detected COMMUNITY (Louvain) so each
  // theme-cluster gets its own colour (the "David Kriesel" topic-map effect).
  // Structural (from the edge topology), not user data. Default colour mode.
  | 'community';

export interface CellChannelBinding {
  // Scatter — metric names bound to the position + optional size/colour
  // channels. `x` / `y` are required for a render; `size` / `color` optional.
  x?: string;
  y?: string;
  size?: string;
  color?: string;
  // Co-occurrence network — node-size + node-colour channel selectors.
  netSize?: NetworkSizeChannel;
  netColor?: NetworkColorChannel;
  // Phase 125 — the per-article metric name aggregated onto nodes when netSize
  // or netColor is 'metric' (drives BFF `nodeMetric`). This is the SIZE-channel
  // metric when size and colour bind to different metrics.
  netMetric?: string;
  // Phase 125 / ISSUE 7 — the COLOUR-channel metric, letting colour bind to a
  // different metric than size (drives BFF `nodeColorMetric`). When unset and
  // netColor === 'metric', the colour channel falls back to `netMetric`.
  netColorMetric?: string;
}

// Phase 126 — per-cell configuration override. A split / small-multiple panel
// shares one set of cell-shape levers (the panel default); a single cell may
// override a lever when the shared value harms its readability. A CellOverride
// is a PARTIAL of the panel's cell-shape levers only — never the panel-identity
// ones (view / metric / composition / splitDirection / scope / window / layer /
// resolution / normalization stay panel-wide; changing one of those is a NEW
// panel, not a cell tweak). A lever absent from the override inherits the panel
// default; a lever present wins for that cell only (CSS-specificity model). The
// map lives on `Panel.cellOverrides`, keyed by the stable per-cell key
// (`cellOverrideKey` in panel-queries.ts).
export interface CellOverride {
  bins?: number;
  topN?: number;
  maxNodes?: number;
  forceStrength?: number;
  showBand?: boolean;
  showEdges?: boolean;
  showLabels?: boolean;
  labelTopPercent?: number;
  labelRankBy?: 'size' | 'colour';
  // Phase 148g — provenance-border mode (per-node ring(s) for source/probe).
  provenanceBorder?: 'none' | 'source' | 'probe' | 'both';
  scales?: ScaleMode;
  displayLanguage?: 'source' | 'viewer';
  channels?: CellChannelBinding;
  // ADR-038 — per-cell DIMENSION peek. A metric name (metric views) or a
  // categorical field name (field views), same kind as the panel view, valid for
  // THIS cell's own source. Breaks comparability by design → rendered with a loud
  // banner. Amends the Phase-126 "metric = panel-wide" rule (view + scope stay
  // panel-wide; only the dimension is per-cell-overridable).
  metric?: string;
}

// Phase 126 — a CellOverride PATCH. Identical shape, but every lever may be set
// to `undefined` to CLEAR it (revert the cell to the panel default). Under the
// project's `exactOptionalPropertyTypes`, an absent key and an explicit-`undefined`
// key differ; the patch type makes "clear this lever" expressible, which the
// plain CellOverride (number, not number|undefined) cannot.
export type CellChannelPatch = {
  [K in keyof CellChannelBinding]?: CellChannelBinding[K] | undefined;
};
export type CellOverridePatch = {
  bins?: number | undefined;
  topN?: number | undefined;
  maxNodes?: number | undefined;
  forceStrength?: number | undefined;
  showBand?: boolean | undefined;
  showEdges?: boolean | undefined;
  showLabels?: boolean | undefined;
  labelTopPercent?: number | undefined;
  labelRankBy?: 'size' | 'colour' | undefined;
  provenanceBorder?: 'none' | 'source' | 'probe' | 'both' | undefined;
  scales?: ScaleMode | undefined;
  displayLanguage?: 'source' | 'viewer' | undefined;
  channels?: CellChannelPatch | undefined;
  metric?: string | undefined;
};

export interface Panel {
  scopes: ScopeGroup[]; // 1..M scope-groups
  composition: Composition;
  view: Presentation;
  metric: string;
  layer: DataLayer;
  // Phase 149 — optional human label for the panel (e.g. "FR vs DE sentiment").
  // Shown in the panel header next to the presentation title; editable inline.
  // Rides in the saved-analysis state (it lives in the pillar payload). Encoded
  // as `pn` in the compact payload; absent/empty when unset.
  label?: string;
  resolution?: Resolution;
  normalization?: Normalization;
  topN?: number;
  // Phase 148g — co-occurrence node-FIRST breadth cap (top-N entities by weight;
  // edges among them). 0/undefined = edge-first (nodes are a by-product of topN
  // edges). Up to 10000 for the at-scale fine-grained map.
  maxNodes?: number;
  // Phase 131 — per-cell configuration. Each cell declares which of these it
  // consumes (registry `configurableParams`); PanelControls surfaces only
  // the relevant levers. All optional so a Panel without explicit config
  // renders at the cell's published defaults.
  //   bins      — distribution histogram bin count (default 30).
  //   channels  — visual-channel binding (scatter axes/size/colour; network
  //               node size/colour).
  //   showBand  — time-series ±1σ uncertainty band; undefined = shown.
  bins?: number;
  channels?: CellChannelBinding;
  showBand?: boolean;
  // Co-occurrence redesign — show/hide the edge (connection) lines. undefined =
  // shown (default true). A nodes-only view is far more readable for a dense
  // map; the relational structure still reads from clustering + node placement.
  showEdges?: boolean;
  // Phase 148g — co-occurrence node labels on/off (default ON). All labels render
  // at the same zoom distance (no LOD mix) so the map is never half-labelled.
  showLabels?: boolean;
  // Phase 148g — label density filter: only the top labelTopPercent % of nodes
  // (ranked by labelRankBy: 'size' or 'colour') are labelled. 100/undefined =
  // all. Lets a dense 10k-node map show labels for just the most prominent nodes.
  labelTopPercent?: number;
  labelRankBy?: 'size' | 'colour';
  // Phase 148g — co-occurrence provenance BORDER. Orthogonal to the node FILL
  // (`channels.netColor`): a coloured ring (source), a second ring (probe), or
  // both, so a reader sees WHO published a node alongside its size + metric
  // colour. 'none'/undefined = no ring. Encoded as `pv` in the compact payload.
  provenanceBorder?: 'none' | 'source' | 'probe' | 'both';
  // Phase 131 (BUG1.7) — co-occurrence force-layout spread (0..100). Higher =
  // stronger node repulsion = more spread-out graph (less single-cluster
  // crowding). Layout-only, not a metric. Default 50.
  forceStrength?: number;
  // Co-occurrence redesign — large-scale (WebGL) layout SETTLE time in seconds:
  // how long the ForceAtlas2 worker runs before freezing. Higher = more time to
  // relax into clusters. Layout-only. Default 12, lever range 3..60.
  settleSeconds?: number;
  // Phase 123b — co-occurrence cross-lingual relabel. 'source' (default) keeps
  // each node on its source-language surface form; 'viewer' swaps QID-linked
  // nodes to the viewer-language Wikidata label (unlinked nodes stay on source
  // form). undefined = 'source' so nothing relabels silently.
  displayLanguage?: 'source' | 'viewer';
  // Phase 122i revision (B1). When `locked` is true the Panel's scope is
  // frozen (the ScopeEditor refuses scope mutations); everything else —
  // view, metric, layer, composition, splitDirection, cellControlsCollapsed
  // — remains fully editable. Set when the Panel was opened from a
  // discourse-function tile in the Probe Dossier.
  locked?: boolean;
  lockedReason?: 'df_entry';
  lockedFunction?: string;
  // Phase 122i revision (D2). Direction of split-composition small-
  // multiples within the panel. Absent / undefined = horizontal default.
  splitDirection?: SplitDirection;
  // Phase 122i revision (C4). When true, the focused panel renders its
  // PanelControls collapsed (header-only with an expand toggle). Persists
  // in the URL so a deep-link survives. Per-panel; only meaningful on
  // the focused panel of the active window.
  cellControlsCollapsed?: boolean;
  // Phase 123c (Issue 6) — "show anyway". When true, the metric picker also
  // offers metrics present for only SOME scoped sources (normally withheld),
  // and the panel renders cells only for the sources that actually carry the
  // chosen metric (PanelHost drops the data-less ones). Default false/absent
  // = the strict cross-source-intersection behaviour.
  showWithheld?: boolean;
  // Phase 122k F5 — per-Panel time window. When set, overrides the global
  // `url.from` / `url.to` for THIS panel only. ISO-date strings; when
  // absent the panel inherits the global default (current behaviour).
  // Encoded as `ws` / `we` in the compact pillar payload.
  windowStart?: string;
  windowEnd?: string;
  // Phase 124 — per-panel axis-scale mode for multi-cell value-axis panels.
  // 'shared' (default, omitted) puts every cell on the union domain;
  // 'free' restores per-cell optimal domains. Encoded as `sc` in the compact
  // payload (emitted only when 'free').
  scales?: ScaleMode;
  // Phase 125 — the N-metric set for multivariate cells (correlation_matrix,
  // parallel_coordinates). Ordered list of metric names; the cell picks a
  // sensible scope-available default when absent. Encoded as `ms` in the
  // compact payload (emitted only when set).
  metricSet?: string[];
  // Phase 125a — the ordered chain of categorical metadata FIELDS for the
  // sankey/alluvial cell. Split out of the overloaded `metricSet` (which held
  // metric names AND field names) so the two never collide. Encoded as `fc` in
  // the compact payload (emitted only when set). Pre-125a sankey URLs stored
  // the chain in `ms`; expandPanel maps that legacy form into `fieldChain`.
  fieldChain?: string[];
  // Phase 125a — faceting / small-multiples. When set, a split panel breaks the
  // cell into one sub-cell per distinct value of this categorical metadata
  // field, each restricted to articles carrying that value (a `metadataFilter`
  // on the per-article query). Empty/absent ⇒ no faceting. Encoded as `ff`.
  facetField?: string;
  // Phase 126 — per-cell overrides of the cell-shape levers above. Keyed by the
  // stable cell key (`cellOverrideKey`, panel-queries.ts). Absent ⇒ every cell
  // inherits the panel defaults; an entry overrides only the levers it names,
  // for that cell only (comparison-as-default — the override is the signposted
  // exception, Brief §1.3). Encoded as `co` in the compact payload.
  cellOverrides?: Record<string, CellOverride>;
}

export interface WorkbenchWindow {
  panels: Panel[]; // 1..MAX_PANELS_PER_WINDOW
  focusedPanelIndex: number;
  // Phase 149 — the former `maximizedPanelIndex` (Maximize-Mode) was removed when
  // Zen mode replaced it. Zen is transient view state owned by WindowHost, never
  // URL-persisted, so the Window shape carries no zoom pointer.
  // Phase 122k §14 finding 6 — configurable panels-per-row. When set,
  // the panel raster uses `repeat(N, 1fr)` so N panels share each row.
  // Absent / undefined = auto-fill with the previous `minmax(28rem, 1fr)`
  // heuristic. Valid range: 1..8 (capped by MAX_PANELS_PER_WINDOW).
  panelsPerRow?: number;
}

export interface PillarState {
  windows: WorkbenchWindow[]; // 1..MAX_WINDOWS_PER_PILLAR
  activeWindowIndex: number;
}

export interface WorkbenchPillarsState {
  aleph: PillarState | null;
  episteme: PillarState | null;
  rhizome: PillarState | null;
}

export const MAX_PANELS_PER_WINDOW = 8;
export const MAX_WINDOWS_PER_PILLAR = 4;
// Total URL byte budget for the Workbench state. When a write would push
// `?activePillar=…&aleph=…&episteme=…&rhizome=…` past this, the rune
// store sets the `pendingUrlOverflow` flag and the Workbench renders a
// confirm dialog asking the user which pillar's oldest window to drop.
export const WORKBENCH_URL_CAP_BYTES = 8192;

export interface UrlState {
  from: string | null;
  to: string | null;
  resolution: Resolution | null;
  // Phase 144 — UI locale deep-link (`?lang=en|de`). Persisted here so the
  // existing write-path (`writeToSearch` rebuilds the query from scratch)
  // preserves it across every other URL mutation. The resolved locale rune
  // (state/locale.svelte.ts) writes it via `setUrl({ lang })`; a shared link
  // with `?lang=` seeds the rune on load. `null` = not pinned in the URL
  // (the rune then falls back to localStorage → navigator → en).
  lang: Locale | null;
  // Normalization mode (Phase 115). `null` and `raw` are equivalent.
  normalization: Normalization | null;
  // Phase 122i / ADR-034 — Multi-Panel Workbench state.
  //
  // Single canonical URL grammar (Phase 122k pre-deployment reset):
  // `?activePillar=…&aleph=<base64url-json>&episteme=…&rhizome=…`.
  // The Phase-122h legacy flat form (`?probeId=&sourceId=&view=&metric=
  // &viewingMode=&layer=`) has been retired entirely — no bookmarks exist
  // to preserve. All per-Panel state lives inside the pillar payload.
  activePillar: PillarId | null;
  pillars: WorkbenchPillarsState | null;
  // Phase 122k — Probe Selection State. Populated by Atmos SHIFT-click
  // on probe glyphs and by the Probe-Filter Modal. Consumed by:
  //   - Dossier: filters the catalog to these probes, auto-expanded
  //   - Workbench: seeds the ScopeEditor's first ScopeGroup when the
  //     user opens the Workbench with a non-empty selection
  // Serialised as `?selectedProbes=a,b,c`. Empty array = no selection.
  selectedProbes: string[];
  // Phase 123a — Dossier-as-overlay. The Dossier is no longer a top-level
  // route; it opens as a global search/catalogue overlay over any surface
  // via `?dossier=open` (round-trips for deep-linking). Probe focus is
  // carried by `?selectedProbes=`, not a separate param.
  dossier: 'open' | null;
  // Phase 134 / ADR-040 — account + admin as global overlays over the globe
  // (same model as the Dossier), so the globe never remounts on open/close.
  account: 'open' | null;
  admin: 'open' | null;
  // Phase 149 — "About AĒR" intro panel as a global overlay (same model): the
  // re-openable home of the welcome content (what / purpose / for whom / state /
  // future). Driven by `?about=open` so it deep-links and round-trips.
  about: 'open' | null;
  // Phase 135 — saved-analyses overlay. `save` opens it directly in the
  // save-current-view flow (a discoverable affordance from the Workbench).
  analyses: 'open' | 'save' | null;
  // Phase 135 — the saved analysis the current Workbench view was opened from
  // (set by "Open in Workbench"). A first-class field so it survives panel
  // mutations (writeToSearch rebuilds the query) and lets "Save" offer an
  // in-place update. Never part of a saved deep-link (stripped on capture).
  savedAnalysis: string | null;
}

// SSoT default lookback used when ?from/?to are absent. Both the page
// (for the L1 Window readout + activity query) and the TimeScrubber
// (for thumb positions) read this so a reset converges on one range.
export const DEFAULT_LOOKBACK_MS = 7 * 24 * 60 * 60 * 1000;

export const EMPTY_URL_STATE: UrlState = {
  from: null,
  to: null,
  resolution: null,
  lang: null,
  normalization: null,
  activePillar: null,
  pillars: null,
  selectedProbes: [],
  dossier: null,
  account: null,
  admin: null,
  about: null,
  analyses: null,
  savedAnalysis: null
};

export const RESOLUTIONS: readonly Resolution[] = ['5min', 'hourly', 'daily', 'weekly', 'monthly'];
export const VIEWING_MODES: readonly PillarId[] = ['aleph', 'episteme', 'rhizome'];
export const VIEW_MODES: readonly Presentation[] = [
  'time_series',
  'distribution',
  'cooccurrence_network',
  'topic_distribution',
  'topic_evolution',
  'metric_scatter',
  'revision_activity',
  'revision_timeline',
  'revision_discourse_shift',
  'revision_edit_clusters',
  'cross_probe_lead_lag',
  'categorical_distribution',
  'correlation_matrix',
  'cross_tab',
  'metric_lead_lag',
  'parallel_coordinates',
  'sankey'
];
export const NORMALIZATIONS: readonly Normalization[] = ['raw', 'zscore', 'percentile'];
// A metric name must be short, ascii, and identifier-shaped to avoid
// smuggling structure into the URL. The BFF's `metric_name` is already
// snake-case ascii, so this matches the wire contract exactly.
export const METRIC_NAME_RE = /^[a-z0-9_]{1,64}$/i;

// --- Compact (URL-serialised) shapes ---
export interface CompactScopeGroup {
  pi: string[];
  si: string[];
}

export interface CompactChannelBinding {
  x?: string;
  y?: string;
  sz?: string;
  co?: string;
  ns?: NetworkSizeChannel;
  nc?: NetworkColorChannel;
  nm?: string; // netMetric (Phase 125 node-metric name — size channel)
  ncm?: string; // netColorMetric (Phase 125 / ISSUE 7 — colour channel metric)
}

// Phase 126 — compact per-cell override. Mirrors CompactPanel's cell-shape
// short keys, but `sb` / `sc` / `dl` are presence-encoded (0 = the default
// value, 1 = the non-default value) so an override can pin either side.
export interface CompactCellOverride {
  bn?: number; // bins
  tN?: number; // topN
  mn?: number; // maxNodes (Phase 148g)
  fs?: number; // forceStrength
  sb?: 0 | 1; // showBand (0 = false, 1 = true)
  se?: 0 | 1; // showEdges (0 = false, 1 = true)
  sl?: 0 | 1; // showLabels (0 = false, 1 = true) — Phase 148g
  lp?: number; // labelTopPercent (Phase 148g)
  lk?: 0 | 1; // labelRankBy (0 = size, 1 = colour) — Phase 148g
  pv?: 0 | 1 | 2 | 3; // provenanceBorder (0 none, 1 source, 2 probe, 3 both) — Phase 148g
  sc?: 0 | 1; // scales (0 = shared, 1 = free)
  dl?: 0 | 1; // displayLanguage (0 = source, 1 = viewer)
  ch?: CompactChannelBinding; // visual-channel binding
  mc?: string; // metric/field dimension (ADR-038 per-cell peek)
}

export interface CompactPanel {
  s: CompactScopeGroup[];
  c: 'm' | 's' | 'o';
  v: Presentation;
  m: string;
  l: 'g' | 's';
  pn?: string; // label (Phase 149 — human panel caption)
  r?: Resolution;
  n?: Normalization;
  tN?: number;
  mn?: number; // maxNodes (Phase 148g)
  L?: 1;
  lr?: 'df_entry';
  lf?: string;
  // Phase 131 per-cell config short keys.
  bn?: number; // bins (distribution)
  ch?: CompactChannelBinding; // visual-channel binding
  sb?: 0; // showBand=false (default true → omitted)
  se?: 1; // showEdges=true (default hidden → omitted)
  sl?: 1; // showLabels=true (default false → omitted) — Phase 148g
  lp?: number; // labelTopPercent (default 100 → omitted) — Phase 148g
  lk?: 0 | 1; // labelRankBy (0 = size, 1 = colour; default size → omitted) — Phase 148g
  pv?: 1 | 2 | 3; // provenanceBorder (1 source, 2 probe, 3 both; default none → omitted) — Phase 148g
  fs?: number; // forceStrength (network spread)
  st?: number; // settleSeconds (large-scale FA2 settle time)
  dl?: 1; // displayLanguage='viewer' (default 'source' → omitted)
  // Phase 122i revision short keys.
  sd?: 'h' | 'v'; // splitDirection (D2)
  cc?: 1; // cellControlsCollapsed (C4)
  sw?: 1; // showWithheld — offer partial (some-source) metrics anyway (Issue 6)
  sc?: 1; // scales='free' (Phase 124; default 'shared' → omitted)
  // Phase 122k F5 — per-panel time window. ISO date strings; absent when
  // the panel inherits the global default. Encoded verbatim so URL-state
  // debugging is straightforward.
  ws?: string;
  we?: string;
  // Phase 125 — N-metric set for multivariate cells (correlation_matrix,
  // parallel_coordinates). Absent when unset.
  ms?: string[];
  // Phase 125a — categorical field chain for the sankey cell. Absent when unset.
  fc?: string[];
  // Phase 125a — faceting field for small-multiples. Absent when unset.
  ff?: string;
  // Phase 126 — per-cell overrides, keyed by stable cell key.
  co?: Record<string, CompactCellOverride>;
}

export interface CompactWindow {
  p: CompactPanel[];
  fi: number;
  // Phase 149 — the `mp` (maximizedPanelIndex) key was retired with Maximize-Mode
  // (Zen mode is transient, not URL-encoded). A legacy `mp` in an old URL is
  // simply ignored by the decoder.
  // Phase 122k §14 finding 6 — panels-per-row override.
  ppr?: number;
}

export interface CompactPillarState {
  w: CompactWindow[];
  aw: number;
}
