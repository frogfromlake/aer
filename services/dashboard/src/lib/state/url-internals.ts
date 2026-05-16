// Pure URL (de)serialisation helpers backing `url.svelte.ts`. Kept rune-
// free in their own module so vitest can import them without a Svelte
// compiler pass. The runes-based store lives in `url.svelte.ts` and
// re-exports these for component-side use.

export type Resolution = '5min' | 'hourly' | 'daily' | 'weekly' | 'monthly';
export type ViewingMode = 'aleph' | 'episteme' | 'rhizome';
// Rhizome entry-question (Phase 122h / ADR-033 §2 Rhizome paragraph). The
// Rhizome Pillar renders one of four opinionated default views; the URL
// encodes which view is active so deep-links restore the exact entry.
// Replaces the retired Surface I L3 ViewLayer (`atmosphere`/`analysis`) —
// the L3 companion panel was retired in Phase 124b; the `view` URL key is
// repurposed for the Rhizome sub-view.
export type RhizomeView =
  | 'actors-topics'
  | 'source-resonance'
  | 'concept-migration'
  | 'free-composition';
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
export type Composition = 'merged' | 'split';

export interface ScopeGroup {
  // 1..K probe ids. Multi-probe entries are valid even though the
  // production corpus is single-probe today — the Cell-host unions them
  // when querying.
  probeIds: string[];
  // 0..L source ids; empty = "all sources of probeIds".
  sourceIds: string[];
}

export interface Panel {
  scopes: ScopeGroup[]; // 1..M scope-groups
  composition: Composition;
  view: ViewMode;
  metric: string;
  layer: DataLayer;
  resolution?: Resolution;
  normalization?: Normalization;
  topN?: number;
  // Set true when the Panel was opened from a discourse-function tile in
  // the Probe Dossier. Locked panels render CellControls + ScopeEditor
  // read-only; the user must go back to the Dossier to recombine.
  locked?: boolean;
  lockedReason?: 'df_entry';
  lockedFunction?: string;
}

export interface WorkbenchWindow {
  panels: Panel[]; // 1..MAX_PANELS_PER_WINDOW
  focusedPanelIndex: number;
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
  viewingMode: ViewingMode | null;
  // Metric the L3 Analysis view is locked onto. A free-form string so
  // new gold metrics land without a schema bump; the L3 panel falls
  // back to a sensible default when this is null.
  metric: string | null;
  // Rhizome sub-view (Phase 122h). `null` = "render Rhizome's default
  // (Akteure & Themen) when the Pillar is Rhizome; otherwise ignored".
  view: RhizomeView | null;
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
  // Phase 122i / ADR-034 — Multi-Panel Workbench state.
  //
  // When a richer-than-flat state is in scope (multi-panel, multi-window,
  // multi-scope-group, or a `locked` panel), the writer emits
  // `?activePillar=…&aleph=<base64url-json>&…` and DROPS the legacy flat
  // params. When the state reduces to a single pillar / single window /
  // single panel / single scope-group / not-locked, the writer prefers
  // the legacy `?probeId=&sourceId=&view=&metric=&viewingMode=&layer=`
  // form so Phase-122h bookmarks remain byte-stable.
  activePillar: ViewingMode | null;
  pillars: WorkbenchPillarsState | null;
}

// SSoT default lookback used when ?from/?to are absent. Both the page
// (for the L1 Window readout + activity query) and the TimeScrubber
// (for thumb positions) read this so a reset converges on one range.
export const DEFAULT_LOOKBACK_MS = 7 * 24 * 60 * 60 * 1000;

export const EMPTY_URL_STATE: UrlState = {
  from: null,
  to: null,
  resolution: null,
  viewingMode: null,
  metric: null,
  view: null,
  sourceIds: [],
  probeIds: [],
  viewMode: null,
  layer: null,
  negSpace: null,
  normalization: null,
  activePillar: null,
  pillars: null
};

const RESOLUTIONS: readonly Resolution[] = ['5min', 'hourly', 'daily', 'weekly', 'monthly'];
const VIEWING_MODES: readonly ViewingMode[] = ['aleph', 'episteme', 'rhizome'];
const RHIZOME_VIEWS: readonly RhizomeView[] = [
  'actors-topics',
  'source-resonance',
  'concept-migration',
  'free-composition'
];
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

export function readFromSearch(search: string): UrlState {
  const p = new URLSearchParams(search);
  // Phase 122i: when any pillar key is present, the URL is in
  // multi-panel form. Legacy flat params (probeId/sourceId/view/…) are
  // intentionally ignored to keep the read deterministic.
  const alephRaw = p.get('aleph');
  const epistemeRaw = p.get('episteme');
  const rhizomeRaw = p.get('rhizome');
  const hasPillarState = alephRaw !== null || epistemeRaw !== null || rhizomeRaw !== null;
  if (hasPillarState) {
    const aleph = alephRaw !== null ? decodePillarState(alephRaw) : null;
    const episteme = epistemeRaw !== null ? decodePillarState(epistemeRaw) : null;
    const rhizome = rhizomeRaw !== null ? decodePillarState(rhizomeRaw) : null;
    const activePillar = parseEnum(p.get('activePillar'), VIEWING_MODES);
    return {
      from: parseIso(p.get('from')),
      to: parseIso(p.get('to')),
      resolution: parseEnum(p.get('resolution'), RESOLUTIONS),
      viewingMode: null,
      metric: null,
      view: null,
      sourceIds: [],
      probeIds: [],
      viewMode: null,
      layer: null,
      negSpace: p.get('negSpace') === '1' ? true : null,
      normalization: parseEnum(p.get('normalization'), NORMALIZATIONS),
      activePillar,
      pillars: { aleph, episteme, rhizome }
    };
  }
  return {
    from: parseIso(p.get('from')),
    to: parseIso(p.get('to')),
    resolution: parseEnum(p.get('resolution'), RESOLUTIONS),
    viewingMode: parseEnum(p.get('viewingMode'), VIEWING_MODES),
    metric: parseMetric(p.get('metric')),
    view: parseEnum(p.get('view'), RHIZOME_VIEWS),
    sourceIds: parseSourceIds(p.get('sourceId')),
    probeIds: parseSourceIds(p.get('probeId')),
    viewMode: parseEnum(p.get('viewMode'), VIEW_MODES),
    layer: parseEnum(p.get('layer'), DATA_LAYERS),
    negSpace: p.get('negSpace') === '1' ? true : null,
    normalization: parseEnum(p.get('normalization'), NORMALIZATIONS),
    activePillar: null,
    pillars: null
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
  if (state.resolution) p.set('resolution', state.resolution);
  // Phase 122i: when `state.pillars` is non-null, the writer emits the
  // multi-panel form (`?activePillar=…&aleph=…&episteme=…&rhizome=…`)
  // and drops the legacy flat Surface II params. The reader honours the
  // pillar form as authoritative when present (see `readFromSearch`).
  if (state.pillars) {
    if (state.activePillar) p.set('activePillar', state.activePillar);
    if (state.pillars.aleph) p.set('aleph', encodePillarState(state.pillars.aleph));
    if (state.pillars.episteme) p.set('episteme', encodePillarState(state.pillars.episteme));
    if (state.pillars.rhizome) p.set('rhizome', encodePillarState(state.pillars.rhizome));
  } else {
    if (state.viewingMode) p.set('viewingMode', state.viewingMode);
    // metric, viewMode, layer, and sourceIds are Surface II concepts; they
    // are meaningful on /workbench routes regardless of whether a ?probe= param
    // is present.
    if (state.metric) p.set('metric', state.metric);
    if (state.view) p.set('view', state.view);
    if (state.sourceIds.length > 0) p.set('sourceId', state.sourceIds.join(','));
    if (state.probeIds.length > 0) p.set('probeId', state.probeIds.join(','));
    if (state.viewMode) p.set('viewMode', state.viewMode);
    if (state.layer === 'silver') p.set('layer', 'silver');
  }
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

// ---------------------------------------------------------------------------
// Phase 122i / ADR-034 — PillarState encoder / decoder.
//
// Encoded shape uses short keys so the URL stays under the 8 KiB cap even
// for the realistic worst-case of 4 windows × 4 panels × multiple scope
// groups. Base64url (RFC 4648 §5) keeps the payload URL-safe; no gzip is
// applied because typical states stay well under 2 KiB and gzip would
// force asynchronous CompressionStream usage in what is otherwise a
// synchronous read/write path. Hand-rolled type guards validate every
// nested field on decode; malformed input returns `null` and the
// rune-store renders a refusal surface.
//
// Short-key map:
//   w  → windows[]              p   → panels[]            s  → scopes[]
//   pi → probeIds[]             si  → sourceIds[]
//   c  → composition ("m"|"s")  v   → view (full string)  m  → metric
//   l  → layer ("g"|"s")        r   → resolution          n  → normalization
//   tN → topN                   L   → locked (1)          lr → lockedReason
//   lf → lockedFunction         fi  → focusedPanelIndex
//   aw → activeWindowIndex
//
// `c` and `l` get a one-letter alphabet because they're the only
// well-bounded enums where one-letter keys aid compression; `v`/`r`/`n`
// keep their canonical strings because their alphabets carry semantic
// value that aids URL debuggability.
// ---------------------------------------------------------------------------

interface CompactScopeGroup {
  pi: string[];
  si: string[];
}

interface CompactPanel {
  s: CompactScopeGroup[];
  c: 'm' | 's';
  v: ViewMode;
  m: string;
  l: 'g' | 's';
  r?: Resolution;
  n?: Normalization;
  tN?: number;
  L?: 1;
  lr?: 'df_entry';
  lf?: string;
}

interface CompactWindow {
  p: CompactPanel[];
  fi: number;
}

interface CompactPillarState {
  w: CompactWindow[];
  aw: number;
}

function compactPillarState(s: PillarState): CompactPillarState {
  return {
    w: s.windows.map((win) => ({
      p: win.panels.map(compactPanel),
      fi: win.focusedPanelIndex
    })),
    aw: s.activeWindowIndex
  };
}

function compactPanel(p: Panel): CompactPanel {
  const c: CompactPanel = {
    s: p.scopes.map((g) => ({ pi: g.probeIds, si: g.sourceIds })),
    c: p.composition === 'merged' ? 'm' : 's',
    v: p.view,
    m: p.metric,
    l: p.layer === 'silver' ? 's' : 'g'
  };
  if (p.resolution !== undefined) c.r = p.resolution;
  if (p.normalization !== undefined) c.n = p.normalization;
  if (p.topN !== undefined) c.tN = p.topN;
  if (p.locked === true) c.L = 1;
  if (p.lockedReason !== undefined) c.lr = p.lockedReason;
  if (p.lockedFunction !== undefined) c.lf = p.lockedFunction;
  return c;
}

function expandPillarState(c: CompactPillarState): PillarState {
  return {
    windows: c.w.map((w) => ({
      panels: w.p.map(expandPanel),
      focusedPanelIndex: w.fi
    })),
    activeWindowIndex: c.aw
  };
}

function expandPanel(c: CompactPanel): Panel {
  const p: Panel = {
    scopes: c.s.map((g) => ({ probeIds: g.pi, sourceIds: g.si })),
    composition: c.c === 'm' ? 'merged' : 'split',
    view: c.v,
    metric: c.m,
    layer: c.l === 's' ? 'silver' : 'gold'
  };
  if (c.r !== undefined) p.resolution = c.r;
  if (c.n !== undefined) p.normalization = c.n;
  if (c.tN !== undefined) p.topN = c.tN;
  if (c.L === 1) p.locked = true;
  if (c.lr !== undefined) p.lockedReason = c.lr;
  if (c.lf !== undefined) p.lockedFunction = c.lf;
  return p;
}

export function encodePillarState(state: PillarState): string {
  const compact = compactPillarState(state);
  const json = JSON.stringify(compact);
  return base64UrlEncode(json);
}

export function decodePillarState(encoded: string): PillarState | null {
  const json = base64UrlDecode(encoded);
  if (json === null) return null;
  let raw: unknown;
  try {
    raw = JSON.parse(json);
  } catch {
    return null;
  }
  if (!isCompactPillarState(raw)) return null;
  return expandPillarState(raw);
}

// ---------------------------------------------------------------------------
// Hand-rolled type guards. No Zod dependency — the bundle budget is tight
// and these guards live in a single file. Validation is strict: any
// unexpected field type yields `null`, which the rune store surfaces as a
// refusal "Shared link could not be read — please reconfigure".
// ---------------------------------------------------------------------------

function isCompactPillarState(v: unknown): v is CompactPillarState {
  if (!isRecord(v)) return false;
  if (!Array.isArray(v.w) || v.w.length === 0 || v.w.length > MAX_WINDOWS_PER_PILLAR) return false;
  if (typeof v.aw !== 'number' || !Number.isInteger(v.aw) || v.aw < 0 || v.aw >= v.w.length)
    return false;
  return v.w.every(isCompactWindow);
}

function isCompactWindow(v: unknown): v is CompactWindow {
  if (!isRecord(v)) return false;
  if (!Array.isArray(v.p) || v.p.length === 0 || v.p.length > MAX_PANELS_PER_WINDOW) return false;
  if (typeof v.fi !== 'number' || !Number.isInteger(v.fi) || v.fi < 0 || v.fi >= v.p.length)
    return false;
  return v.p.every(isCompactPanel);
}

function isCompactPanel(v: unknown): v is CompactPanel {
  if (!isRecord(v)) return false;
  if (!Array.isArray(v.s) || v.s.length === 0) return false;
  if (!v.s.every(isCompactScopeGroup)) return false;
  if (v.c !== 'm' && v.c !== 's') return false;
  if (typeof v.v !== 'string' || !(VIEW_MODES as readonly string[]).includes(v.v)) return false;
  if (typeof v.m !== 'string' || !METRIC_NAME_RE.test(v.m)) return false;
  if (v.l !== 'g' && v.l !== 's') return false;
  if (v.r !== undefined && !(RESOLUTIONS as readonly string[]).includes(v.r as string))
    return false;
  if (v.n !== undefined && !(NORMALIZATIONS as readonly string[]).includes(v.n as string))
    return false;
  if (v.tN !== undefined && (typeof v.tN !== 'number' || !Number.isFinite(v.tN))) return false;
  if (v.L !== undefined && v.L !== 1) return false;
  if (v.lr !== undefined && v.lr !== 'df_entry') return false;
  if (v.lf !== undefined && typeof v.lf !== 'string') return false;
  return true;
}

function isCompactScopeGroup(v: unknown): v is CompactScopeGroup {
  if (!isRecord(v)) return false;
  if (!Array.isArray(v.pi) || !v.pi.every((x) => typeof x === 'string')) return false;
  if (!Array.isArray(v.si) || !v.si.every((x) => typeof x === 'string')) return false;
  return true;
}

function isRecord(v: unknown): v is Record<string, unknown> {
  return typeof v === 'object' && v !== null && !Array.isArray(v);
}

// ---------------------------------------------------------------------------
// Base64url codec. Browser-native btoa/atob handle ASCII; for safety with
// any future non-ASCII metric labels we round-trip via TextEncoder so the
// codec is UTF-8-clean.
// ---------------------------------------------------------------------------

function base64UrlEncode(s: string): string {
  // Node test environments and modern browsers both expose `btoa`. The
  // intermediate Latin-1 conversion is the standard pattern for encoding
  // UTF-8 strings as base64.
  const bytes = new TextEncoder().encode(s);
  let bin = '';
  for (let i = 0; i < bytes.length; i++) bin += String.fromCharCode(bytes[i]!);
  return btoa(bin).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
}

function base64UrlDecode(s: string): string | null {
  try {
    const padded = s.replace(/-/g, '+').replace(/_/g, '/') + '==='.slice((s.length + 3) % 4);
    const bin = atob(padded);
    const bytes = new Uint8Array(bin.length);
    for (let i = 0; i < bin.length; i++) bytes[i] = bin.charCodeAt(i);
    return new TextDecoder('utf-8', { fatal: true }).decode(bytes);
  } catch {
    return null;
  }
}
