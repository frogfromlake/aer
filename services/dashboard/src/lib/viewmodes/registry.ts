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
import type { ViewMode } from '$lib/state/url-internals';

export type AnalyticalDiscipline =
  | 'nlp'
  | 'eda'
  | 'network_science'
  | 'metadata_mining'
  | 'clustering';

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
}

const PRESENTATIONS: readonly PresentationDefinition[] = [
  {
    id: 'time_series',
    label: 'Time series',
    discipline: 'nlp',
    description: 'Per-source time series with uncertainty bands.',
    layout: 'per-source',
    loadComponent: async () =>
      (await import('$lib/components/viewmodes/TimeSeriesCell.svelte')).default
  },
  {
    id: 'distribution',
    label: 'Distribution',
    discipline: 'eda',
    description: 'Histogram + quantile summary across the scope.',
    layout: 'per-scope',
    loadComponent: async () =>
      (await import('$lib/components/viewmodes/DistributionCell.svelte')).default
  },
  {
    id: 'cooccurrence_network',
    label: 'Co-occurrence network',
    discipline: 'network_science',
    description: 'Force-directed entity co-occurrence graph.',
    layout: 'per-scope',
    loadComponent: async () =>
      (await import('$lib/components/viewmodes/CoOccurrenceNetworkCell.svelte')).default
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
export const DEFAULT_METRIC_NAME = 'sentiment_score';
