// Pure URL (de)serialisation helpers backing `url.svelte.ts`. Kept rune-
// free in their own module so vitest can import them without a Svelte
// compiler pass. The runes-based store lives in `url.svelte.ts` and
// re-exports these for component-side use.

export type Resolution = '5min' | 'hourly' | 'daily' | 'weekly' | 'monthly';
export type ViewingMode = 'aleph' | 'episteme' | 'rhizome';
// Descent layer on the Atmosphere surface. Only `atmosphere` (L0/L1/L2)
// and `analysis` (L3 panel open) are URL-addressable; L4 is a transient
// fly-out inside L3 and intentionally not encoded (closing the browser
// tab and returning should land at L3, not inside the provenance
// overlay — the overlay is a disclosure, not a descent).
export type ViewLayer = 'atmosphere' | 'analysis';
// Presentation-form axis of the View-Mode Matrix (Brief §4.2.3 /
// reframing-note §3.2). MVP cells in Phase 107: time_series,
// distribution, cooccurrence_network. The catalog is extensible —
// new presentations are added here and registered in $lib/viewmodes/.
export type ViewMode = 'time_series' | 'distribution' | 'cooccurrence_network';

export interface UrlState {
  from: string | null;
  to: string | null;
  probe: string | null;
  // Zero-based index into the selected probe's emissionPoints array.
  // A probe like probe-0-de-institutional-rss bundles multiple publishers
  // at distinct origins (Tagesschau/Hamburg, Bundesregierung/Berlin), so
  // probeId alone does not deep-link a click — we also encode which point.
  emissionPoint: number | null;
  resolution: Resolution | null;
  viewingMode: ViewingMode | null;
  // Metric the L3 Analysis view is locked onto. A free-form string so
  // new gold metrics land without a schema bump; the L3 panel falls
  // back to a sensible default when this is null.
  metric: string | null;
  // Current descent layer. `null` is treated as `atmosphere` by consumers.
  view: ViewLayer | null;
  // Source-scope narrowing: set by the Probe Dossier (Phase 106) when
  // the user clicks a source card. Propagates into view-mode queries.
  // Only meaningful when `probe` is also set; dropped otherwise in writeToSearch.
  sourceId: string | null;
  // View-Mode Matrix selection (Phase 107). Only meaningful inside
  // Surface II's Function Lanes; consumers treat `null` as the default
  // presentation (`time_series`).
  viewMode: ViewMode | null;
}

// SSoT default lookback used when ?from/?to are absent. Both the page
// (for the L1 Window readout + activity query) and the TimeScrubber
// (for thumb positions) read this so a reset converges on one range.
export const DEFAULT_LOOKBACK_MS = 7 * 24 * 60 * 60 * 1000;

export const EMPTY_URL_STATE: UrlState = {
  from: null,
  to: null,
  probe: null,
  emissionPoint: null,
  resolution: null,
  viewingMode: null,
  metric: null,
  view: null,
  sourceId: null,
  viewMode: null
};

const RESOLUTIONS: readonly Resolution[] = ['5min', 'hourly', 'daily', 'weekly', 'monthly'];
const VIEWING_MODES: readonly ViewingMode[] = ['aleph', 'episteme', 'rhizome'];
const VIEW_LAYERS: readonly ViewLayer[] = ['atmosphere', 'analysis'];
const VIEW_MODES: readonly ViewMode[] = ['time_series', 'distribution', 'cooccurrence_network'];
// A metric name must be short, ascii, and identifier-shaped to avoid
// smuggling structure into the URL. The BFF's `metric_name` is already
// snake-case ascii, so this matches the wire contract exactly.
const METRIC_NAME_RE = /^[a-z0-9_]{1,64}$/i;

function parseIso(v: string | null): string | null {
  if (v === null) return null;
  const d = new Date(v);
  return Number.isNaN(d.getTime()) ? null : d.toISOString();
}

function parseEnum<T extends string>(v: string | null, allowed: readonly T[]): T | null {
  if (v === null) return null;
  return (allowed as readonly string[]).includes(v) ? (v as T) : null;
}

function parseNonNegativeInt(v: string | null): number | null {
  if (v === null) return null;
  if (!/^\d+$/.test(v)) return null;
  const n = Number.parseInt(v, 10);
  return Number.isFinite(n) ? n : null;
}

export function readFromSearch(search: string): UrlState {
  const p = new URLSearchParams(search);
  return {
    from: parseIso(p.get('from')),
    to: parseIso(p.get('to')),
    probe: p.get('probe'),
    emissionPoint: parseNonNegativeInt(p.get('ep')),
    resolution: parseEnum(p.get('resolution'), RESOLUTIONS),
    viewingMode: parseEnum(p.get('viewingMode'), VIEWING_MODES),
    metric: parseMetric(p.get('metric')),
    view: parseEnum(p.get('view'), VIEW_LAYERS),
    sourceId: p.get('sourceId'),
    viewMode: parseEnum(p.get('viewMode'), VIEW_MODES)
  };
}

function parseMetric(v: string | null): string | null {
  if (v === null) return null;
  return METRIC_NAME_RE.test(v) ? v : null;
}

export function writeToSearch(state: UrlState): string {
  const p = new URLSearchParams();
  if (state.from) p.set('from', state.from);
  if (state.to) p.set('to', state.to);
  if (state.probe) p.set('probe', state.probe);
  // `ep` is only meaningful when a probe is selected; drop it otherwise
  // so reload-after-close does not restore a phantom emission point.
  if (state.probe && state.emissionPoint !== null && state.emissionPoint >= 0) {
    p.set('ep', String(state.emissionPoint));
  }
  if (state.resolution) p.set('resolution', state.resolution);
  if (state.viewingMode) p.set('viewingMode', state.viewingMode);
  // `metric` is only meaningful inside the analysis view — a stray
  // metric param on the bare atmosphere would be a non-restorable state.
  if (state.metric && state.view === 'analysis') p.set('metric', state.metric);
  if (state.view && state.view !== 'atmosphere') p.set('view', state.view);
  // sourceId is only meaningful when a probe is selected.
  if (state.probe && state.sourceId) p.set('sourceId', state.sourceId);
  // viewMode is only meaningful when a probe is selected (Surface II
  // Function Lanes). Drop it on the bare Atmosphere so reload converges
  // on the default presentation.
  if (state.probe && state.viewMode) p.set('viewMode', state.viewMode);
  const qs = p.toString();
  return qs.length === 0 ? '' : `?${qs}`;
}
