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
export type ViewMode =
  | 'time_series'
  | 'distribution'
  | 'cooccurrence_network'
  | 'topic_distribution'
  | 'topic_evolution';
// Data layer toggle (Phase 111). `gold` is the default (omitted from URL);
// `silver` routes Surface II queries to /api/v1/silver/* and enforces the
// WP-006 §5.2 eligibility gate. Only meaningful when a probe is selected.
export type DataLayer = 'gold' | 'silver';
// Normalization mode (Phase 115). `null` and `raw` are equivalent;
// `zscore` and `percentile` are URL-addressable so a deviation-labelled
// chart deep-links cleanly. The cross-frame equivalence gate is enforced
// server-side.
export type Normalization = 'raw' | 'zscore' | 'percentile';

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
  // the user clicks source cards. Supports multi-source selection (Phase
  // 113d). Empty array = no scope narrowing. Serialised as comma-separated
  // `sourceId` query parameter.
  sourceIds: string[];
  // Multi-probe composition set (Phase 114). Populated by shift+click on
  // the globe or the Compose CTA in the Probe Dossier. When non-empty,
  // Function Lane cells query the BFF with all probes unioned. Serialised
  // as comma-separated `probeId` query parameter.
  probeIds: string[];
  // View-Mode Matrix selection (Phase 107). Only meaningful inside
  // Surface II's Function Lanes; consumers treat `null` as the default
  // presentation (`time_series`).
  viewMode: ViewMode | null;
  // Silver-layer toggle (Phase 111). `null` and `gold` are equivalent;
  // only `silver` is emitted in the URL.
  layer: DataLayer | null;
  // Negative Space overlay (Phase 112). `null` and `false` are equivalent;
  // only `true` is serialised as `negSpace=1`. When active, all three
  // surfaces shift into "what AĒR doesn't see" mode per Design Brief §4.4.
  negSpace: boolean | null;
  // Normalization mode (Phase 115). `null` and `raw` are equivalent.
  normalization: Normalization | null;
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
  sourceIds: [],
  probeIds: [],
  viewMode: null,
  layer: null,
  negSpace: null,
  normalization: null
};

const RESOLUTIONS: readonly Resolution[] = ['5min', 'hourly', 'daily', 'weekly', 'monthly'];
const VIEWING_MODES: readonly ViewingMode[] = ['aleph', 'episteme', 'rhizome'];
const VIEW_LAYERS: readonly ViewLayer[] = ['atmosphere', 'analysis'];
const VIEW_MODES: readonly ViewMode[] = [
  'time_series',
  'distribution',
  'cooccurrence_network',
  'topic_distribution',
  'topic_evolution'
];
const DATA_LAYERS: readonly DataLayer[] = ['gold', 'silver'];
const NORMALIZATIONS: readonly Normalization[] = ['raw', 'zscore', 'percentile'];
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
    sourceIds: parseSourceIds(p.get('sourceId')),
    probeIds: parseSourceIds(p.get('probeId')),
    viewMode: parseEnum(p.get('viewMode'), VIEW_MODES),
    layer: parseEnum(p.get('layer'), DATA_LAYERS),
    negSpace: p.get('negSpace') === '1' ? true : null,
    normalization: parseEnum(p.get('normalization'), NORMALIZATIONS)
  };
}

function parseMetric(v: string | null): string | null {
  if (v === null) return null;
  return METRIC_NAME_RE.test(v) ? v : null;
}

function parseSourceIds(v: string | null): string[] {
  if (!v) return [];
  return v
    .split(',')
    .map((s) => s.trim())
    .filter((s) => s.length > 0);
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
  // metric, viewMode, layer, and sourceIds are Surface II concepts; they
  // are meaningful on /lanes/* routes regardless of whether a ?probe= param
  // is present (probe is a path param on Surface II, not a query param).
  if (state.metric) p.set('metric', state.metric);
  if (state.view && state.view !== 'atmosphere') p.set('view', state.view);
  if (state.sourceIds.length > 0) p.set('sourceId', state.sourceIds.join(','));
  if (state.probeIds.length > 0) p.set('probeId', state.probeIds.join(','));
  if (state.viewMode) p.set('viewMode', state.viewMode);
  if (state.layer === 'silver') p.set('layer', 'silver');
  // `negSpace=1` when the Negative Space overlay is active. Not scoped to
  // a probe — the overlay applies globally across all surfaces.
  if (state.negSpace === true) p.set('negSpace', '1');
  // Normalization is omitted when raw (the default) so the URL stays
  // clean for the Level-1 view.
  if (state.normalization && state.normalization !== 'raw') {
    p.set('normalization', state.normalization);
  }
  const qs = p.toString();
  return qs.length === 0 ? '' : `?${qs}`;
}
