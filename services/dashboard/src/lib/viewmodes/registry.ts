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
//   1. Add a case to `ViewMode` in `$lib/state/url-internals.ts`.
//   2. Add a `PresentationDefinition` entry below.
//   3. Implement the cell component under `$lib/components/viewmodes/`.
//   No FunctionLaneShell change is required if the cell is rendered via
//   the registry's `loadComponent` indirection.

import type { Component } from 'svelte';
import type { FetchContext } from '$lib/api/queries';
import type { ViewMode, ViewingMode } from '$lib/state/url-internals';

export type AnalyticalDiscipline =
  | 'nlp'
  | 'eda'
  | 'network_science'
  | 'metadata_mining'
  | 'clustering'
  | 'episteme';

/** One presentation-form axis entry. The matrix-cell id is composed at
 *  call-sites as `<id>_<metricName>` to match content-catalog yaml keys. */
export interface PresentationDefinition {
  /** URL-stable key. Mirrors `ViewMode` in `url-internals.ts`. */
  id: ViewMode;
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
  loadComponent: () => Promise<Component<ViewModeCellProps>>;
  /** Does this presentation's cell consume the active `metric` prop?
   *  BERTopic cells (`topic_*`) operate on cleaned text; `cooccurrence_*`
   *  on entity pairs. For those, the metric selector is misleading — the
   *  Cell ignores it. PanelControls hides the Metric row when this is
   *  false. Defaults to `true`. */
  usesMetric?: boolean;
  /** Does this presentation's cell consume the active `resolution` prop?
   *  Only time-series-shaped cells (per-source small-multiples on a time
   *  axis) honour resolution today. PanelControls and EpistemeShell hide
   *  the Resolution control when no active view honours it. Defaults to
   *  `false`. */
  usesResolution?: boolean;
}

/** Common props passed to every cell. The cell decides which subset
 *  it needs (e.g. distribution / cooccurrence ignore `sources`). */
export interface ViewModeCellProps {
  ctx: FetchContext;
  scopeProbeId: string;
  /** Resolved scope: probe id or single source name. */
  scopeId: string;
  scope: 'probe' | 'source';
  windowStart: string;
  windowEnd: string;
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
    loadComponent: async () =>
      (await import('$lib/components/viewmodes/TimeSeriesCell.svelte')).default
  },
  {
    id: 'distribution',
    label: 'Distribution',
    discipline: 'eda',
    description: 'Histogram + quantile summary across the scope.',
    layout: 'per-scope',
    usesMetric: true,
    usesResolution: false,
    loadComponent: async () =>
      (await import('$lib/components/viewmodes/DistributionCell.svelte')).default
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
    loadComponent: async () =>
      (await import('$lib/components/viewmodes/CoOccurrenceNetworkCell.svelte')).default
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
      (await import('$lib/components/viewmodes/TopicDistributionCell.svelte')).default
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
      (await import('$lib/components/viewmodes/TopicEvolutionCell.svelte')).default
  }
];

/** All presentation-form definitions, in display order. */
export function listPresentations(): readonly PresentationDefinition[] {
  return PRESENTATIONS;
}

const DEFAULT_PRESENTATION: PresentationDefinition = PRESENTATIONS[0]!;

/** Lookup by URL-stable id; returns the time-series default when the
 *  caller passed `null` (e.g. URL state was empty). */
export function getPresentation(id: ViewMode | null): PresentationDefinition {
  const found = PRESENTATIONS.find((p) => p.id === id);
  return found ?? DEFAULT_PRESENTATION;
}

/** Compose the cell id used by the content catalog (see
 *  `services/bff-api/configs/content/{en,de}/view_modes/`). */
export function cellContentId(presentation: ViewMode, metricName: string): string {
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

// -------------------------------------------------------------------------
// Pillar mapping — Aleph / Episteme / Rhizome × Presentation
//
// The three pillars are the project's conceptual frame (Brief §3.1, WP-005
// §6). Each pillar bundles a curated subset of presentations so the user
// sees one coherent set of analytical lenses per pillar, rather than the
// full matrix at once. The Pillar switcher in the SideRail and the per-lane
// identity strip on Surface II L3 both consume this mapping.
//
// Mapping (strict 1-to-1, no overlap):
//   Aleph    = synchronic totality  → time_series, distribution
//   Episteme = diachronic register  → topic_distribution, topic_evolution
//   Rhizome  = relational currents  → cooccurrence_network
// -------------------------------------------------------------------------

export interface PillarDefinition {
  id: ViewingMode;
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
  presentations: ViewMode[];
}

export const PILLAR_DEFINITIONS: readonly PillarDefinition[] = [
  {
    id: 'aleph',
    label: 'Aleph',
    abbr: 'A',
    glyph: '◉',
    blurb: 'Synchronic totality — "the weather now"',
    description:
      'Every observed probe in scope, no temporal aggregation beyond the active window. Snapshot-oriented analyses: sentiment levels, lexical density, distributional shape.',
    color: '#5283b8',
    presentations: ['time_series', 'distribution']
  },
  {
    id: 'episteme',
    label: 'Episteme',
    abbr: 'E',
    glyph: '◐',
    blurb: 'Diachronic knowledge register — "the climate record"',
    description:
      'How the expressible shifts over time. Topic models, drift, the long-term shape of what can be said within the discursive formation.',
    color: '#c8a85a',
    presentations: ['topic_distribution', 'topic_evolution']
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
    presentations: ['cooccurrence_network']
  }
];

const DEFAULT_PILLAR: PillarDefinition = PILLAR_DEFINITIONS[0]!;

/** Lookup pillar definition by id; null/unknown returns the Aleph default. */
export function getPillar(id: ViewingMode | null): PillarDefinition {
  return PILLAR_DEFINITIONS.find((p) => p.id === id) ?? DEFAULT_PILLAR;
}

/** Presentations available for the active pillar, in display order. */
export function presentationsForPillar(id: ViewingMode | null): PresentationDefinition[] {
  const pillar = getPillar(id);
  const out: PresentationDefinition[] = [];
  for (const presId of pillar.presentations) {
    const p = PRESENTATIONS.find((x) => x.id === presId);
    if (p) out.push(p);
  }
  return out;
}

/** Default viewMode for the given pillar — the first presentation in its set. */
export function defaultViewModeForPillar(id: ViewingMode | null): ViewMode {
  const pillar = getPillar(id);
  return pillar.presentations[0] ?? DEFAULT_PRESENTATION.id;
}

/** Reverse lookup: which pillar owns the given viewMode? Returns the pillar
 *  id, or null when the viewMode is not registered under any pillar (which
 *  should be impossible under the strict 1-1 mapping but keeps callers
 *  defensive against future presentation additions). */
export function pillarForViewMode(viewMode: ViewMode): ViewingMode | null {
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
  viewMode: ViewMode | null,
  pillar: ViewingMode | null
): PresentationDefinition {
  const pillarDef = getPillar(pillar);
  if (viewMode !== null && pillarDef.presentations.includes(viewMode)) {
    const found = PRESENTATIONS.find((p) => p.id === viewMode);
    if (found) return found;
  }
  const defaultId = pillarDef.presentations[0] ?? DEFAULT_PRESENTATION.id;
  return PRESENTATIONS.find((p) => p.id === defaultId) ?? DEFAULT_PRESENTATION;
}
