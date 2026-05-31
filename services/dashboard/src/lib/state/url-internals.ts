// Pure URL (de)serialisation helpers backing `url.svelte.ts`. Kept rune-
// free in their own module so vitest can import them without a Svelte
// compiler pass. The runes-based store lives in `url.svelte.ts` and
// re-exports these for component-side use.

export type Resolution = '5min' | 'hourly' | 'daily' | 'weekly' | 'monthly';
export type ViewingMode = 'aleph' | 'episteme' | 'rhizome';
// Phase 130 / ADR-035 — the Rhizome entry-question enum (`RhizomeView`:
// actors-topics / source-resonance / concept-migration / free-composition)
// was removed. Rhizome now uses the universal panels+cells model like Aleph
// and Episteme; its relational cells are ordinary `ViewMode` choices.
// Presentation-form axis of the View-Mode Matrix (Brief §4.2.3 /
// reframing-note §3.2). MVP cells in Phase 107: time_series,
// distribution, cooccurrence_network. The catalog is extensible —
// new presentations are added here and registered in $lib/viewmodes/.
export type ViewMode =
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
  | 'revision_timeline';
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

// Phase 122i revision (D2). Split direction governs how a Panel arranges
// its small-multiples when composition='split'. Horizontal = cells
// side-by-side (default); vertical = stacked. Ignored when composition
// is 'merged'.
export type SplitDirection = 'horizontal' | 'vertical';

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
export type NetworkSizeChannel = 'total_count' | 'degree';
// Phase 131a — `source_overlay` colours nodes & edges by their originating
// source from the BFF per-edge `presence` field. Available whenever the
// scope covers multiple sources; the cell auto-promotes it as the default
// for merged scopes.
export type NetworkColorChannel = 'label' | 'presence' | 'uniform' | 'source_overlay';

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
  // Phase 131 (BUG1.7) — co-occurrence force-layout spread (0..100). Higher =
  // stronger node repulsion = more spread-out graph (less single-cluster
  // crowding). Layout-only, not a metric. Default 50.
  forceStrength?: number;
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
}

export interface WorkbenchWindow {
  panels: Panel[]; // 1..MAX_PANELS_PER_WINDOW
  focusedPanelIndex: number;
  // Phase 122i revision (C3). When set, the window renders only the
  // maximised panel at full canvas; the other panels live in a minimised
  // tray for swap. Out-of-bounds values are treated as "no maximize"
  // by the WindowHost render path. Absent / null = no maximize.
  maximizedPanelIndex?: number | null;
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
  // Negative Space overlay (Phase 112). `null` and `false` are equivalent;
  // only `true` is serialised as `negSpace=1`. When active, all three
  // surfaces shift into "what AĒR doesn't see" mode per Design Brief §4.4.
  negSpace: boolean | null;
  // Normalization mode (Phase 115). `null` and `raw` are equivalent.
  normalization: Normalization | null;
  // Phase 122i / ADR-034 — Multi-Panel Workbench state.
  //
  // Single canonical URL grammar (Phase 122k pre-deployment reset):
  // `?activePillar=…&aleph=<base64url-json>&episteme=…&rhizome=…`.
  // The Phase-122h legacy flat form (`?probeId=&sourceId=&view=&metric=
  // &viewingMode=&layer=`) has been retired entirely — no bookmarks exist
  // to preserve. All per-Panel state lives inside the pillar payload.
  activePillar: ViewingMode | null;
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
}

// SSoT default lookback used when ?from/?to are absent. Both the page
// (for the L1 Window readout + activity query) and the TimeScrubber
// (for thumb positions) read this so a reset converges on one range.
export const DEFAULT_LOOKBACK_MS = 7 * 24 * 60 * 60 * 1000;

export const EMPTY_URL_STATE: UrlState = {
  from: null,
  to: null,
  resolution: null,
  negSpace: null,
  normalization: null,
  activePillar: null,
  pillars: null,
  selectedProbes: [],
  dossier: null
};

const RESOLUTIONS: readonly Resolution[] = ['5min', 'hourly', 'daily', 'weekly', 'monthly'];
const VIEWING_MODES: readonly ViewingMode[] = ['aleph', 'episteme', 'rhizome'];
const VIEW_MODES: readonly ViewMode[] = [
  'time_series',
  'distribution',
  'cooccurrence_network',
  'topic_distribution',
  'topic_evolution',
  'metric_scatter',
  'revision_activity',
  'revision_timeline'
];
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
  // Phase 122k — single canonical grammar. Pillar-state base64url is the
  // only form for Workbench state; `?selectedProbes=…` carries the
  // Atmos/Modal selection consumed by Dossier and Workbench.
  const alephRaw = p.get('aleph');
  const epistemeRaw = p.get('episteme');
  const rhizomeRaw = p.get('rhizome');
  const aleph = alephRaw !== null ? decodePillarState(alephRaw) : null;
  const episteme = epistemeRaw !== null ? decodePillarState(epistemeRaw) : null;
  const rhizome = rhizomeRaw !== null ? decodePillarState(rhizomeRaw) : null;
  // The pillars wrapper is populated whenever ANY pillar key is present in
  // the URL — even if decoding failed (the per-pillar slot is then null).
  // This preserves the diagnostic "pillar URL with malformed payload"
  // case so the dashboard can render a refusal surface rather than
  // silently falling back to an empty Workbench.
  const hasPillars = alephRaw !== null || epistemeRaw !== null || rhizomeRaw !== null;
  return {
    from: parseIso(p.get('from')),
    to: parseIso(p.get('to')),
    resolution: parseEnum(p.get('resolution'), RESOLUTIONS),
    negSpace: p.get('negSpace') === '1' ? true : null,
    normalization: parseEnum(p.get('normalization'), NORMALIZATIONS),
    activePillar: parseEnum(p.get('activePillar'), VIEWING_MODES),
    pillars: hasPillars ? { aleph, episteme, rhizome } : null,
    selectedProbes: parseIdList(p.get('selectedProbes')),
    dossier: p.get('dossier') === 'open' ? 'open' : null
  };
}

function parseIdList(v: string | null): string[] {
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
  // Phase 122k — single canonical Workbench grammar.
  if (state.pillars) {
    if (state.activePillar) p.set('activePillar', state.activePillar);
    if (state.pillars.aleph) p.set('aleph', encodePillarState(state.pillars.aleph));
    if (state.pillars.episteme) p.set('episteme', encodePillarState(state.pillars.episteme));
    if (state.pillars.rhizome) p.set('rhizome', encodePillarState(state.pillars.rhizome));
  } else if (state.activePillar) {
    // ActivePillar without a pillar payload is meaningful on the Dossier
    // and Atmos surfaces (it lets the user pre-select which pillar a
    // future Workbench will open into) — emit it so a reload restores it.
    p.set('activePillar', state.activePillar);
  }
  // `negSpace=1` when the Negative Space overlay is active. Not scoped to
  // a probe — the overlay applies globally across all surfaces.
  if (state.negSpace === true) p.set('negSpace', '1');
  // Normalization is omitted when raw (the default) so the URL stays
  // clean for the Level-1 view.
  if (state.normalization && state.normalization !== 'raw') {
    p.set('normalization', state.normalization);
  }
  // Phase 122k — probe selection set.
  if (state.selectedProbes.length > 0) {
    p.set('selectedProbes', state.selectedProbes.join(','));
  }
  // Phase 123a — Dossier overlay state.
  if (state.dossier === 'open') p.set('dossier', 'open');
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

interface CompactChannelBinding {
  x?: string;
  y?: string;
  sz?: string;
  co?: string;
  ns?: NetworkSizeChannel;
  nc?: NetworkColorChannel;
}

interface CompactPanel {
  s: CompactScopeGroup[];
  c: 'm' | 's' | 'o';
  v: ViewMode;
  m: string;
  l: 'g' | 's';
  r?: Resolution;
  n?: Normalization;
  tN?: number;
  L?: 1;
  lr?: 'df_entry';
  lf?: string;
  // Phase 131 per-cell config short keys.
  bn?: number; // bins (distribution)
  ch?: CompactChannelBinding; // visual-channel binding
  sb?: 0; // showBand=false (default true → omitted)
  fs?: number; // forceStrength (network spread)
  // Phase 122i revision short keys.
  sd?: 'h' | 'v'; // splitDirection (D2)
  cc?: 1; // cellControlsCollapsed (C4)
  sw?: 1; // showWithheld — offer partial (some-source) metrics anyway (Issue 6)
  // Phase 122k F5 — per-panel time window. ISO date strings; absent when
  // the panel inherits the global default. Encoded verbatim so URL-state
  // debugging is straightforward.
  ws?: string;
  we?: string;
}

interface CompactWindow {
  p: CompactPanel[];
  fi: number;
  // Phase 122i revision (C3). maximizedPanelIndex — absent / undefined =
  // no maximize. Encoded as a numeric index when set; out-of-bounds
  // values are rejected by the type guard.
  mp?: number;
  // Phase 122k §14 finding 6 — panels-per-row override.
  ppr?: number;
}

interface CompactPillarState {
  w: CompactWindow[];
  aw: number;
}

function compactPillarState(s: PillarState): CompactPillarState {
  return {
    w: s.windows.map((win) => {
      const cw: CompactWindow = {
        p: win.panels.map(compactPanel),
        fi: win.focusedPanelIndex
      };
      // Phase 122i revision (C3). Only emit `mp` when it's a valid
      // in-bounds index; null / undefined / out-of-bounds → omitted.
      if (
        win.maximizedPanelIndex !== undefined &&
        win.maximizedPanelIndex !== null &&
        Number.isInteger(win.maximizedPanelIndex) &&
        win.maximizedPanelIndex >= 0 &&
        win.maximizedPanelIndex < win.panels.length
      ) {
        cw.mp = win.maximizedPanelIndex;
      }
      if (
        win.panelsPerRow !== undefined &&
        Number.isInteger(win.panelsPerRow) &&
        win.panelsPerRow >= 1 &&
        win.panelsPerRow <= MAX_PANELS_PER_WINDOW
      ) {
        cw.ppr = win.panelsPerRow;
      }
      return cw;
    }),
    aw: s.activeWindowIndex
  };
}

function compactPanel(p: Panel): CompactPanel {
  const c: CompactPanel = {
    s: p.scopes.map((g) => ({ pi: g.probeIds, si: g.sourceIds })),
    c: p.composition === 'merged' ? 'm' : p.composition === 'overlay' ? 'o' : 's',
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
  // Phase 122i revision short keys. Default values omitted: horizontal
  // split direction is the default (writer leaves it implicit); the
  // collapsed flag only matters when true.
  if (p.splitDirection === 'vertical') c.sd = 'v';
  else if (p.splitDirection === 'horizontal') c.sd = 'h';
  if (p.cellControlsCollapsed === true) c.cc = 1;
  if (p.showWithheld === true) c.sw = 1;
  // Phase 122k F5 — per-panel window.
  if (p.windowStart !== undefined) c.ws = p.windowStart;
  if (p.windowEnd !== undefined) c.we = p.windowEnd;
  // Phase 131 per-cell config. bins/channels omitted when unset; showBand
  // omitted unless explicitly disabled (default = shown).
  if (p.bins !== undefined) c.bn = p.bins;
  if (p.channels !== undefined) {
    const cb: CompactChannelBinding = {};
    if (p.channels.x !== undefined) cb.x = p.channels.x;
    if (p.channels.y !== undefined) cb.y = p.channels.y;
    if (p.channels.size !== undefined) cb.sz = p.channels.size;
    if (p.channels.color !== undefined) cb.co = p.channels.color;
    if (p.channels.netSize !== undefined) cb.ns = p.channels.netSize;
    if (p.channels.netColor !== undefined) cb.nc = p.channels.netColor;
    if (Object.keys(cb).length > 0) c.ch = cb;
  }
  if (p.showBand === false) c.sb = 0;
  if (p.forceStrength !== undefined) c.fs = p.forceStrength;
  return c;
}

function expandPillarState(c: CompactPillarState): PillarState {
  return {
    windows: c.w.map((w) => {
      const win: WorkbenchWindow = {
        panels: w.p.map(expandPanel),
        focusedPanelIndex: w.fi
      };
      if (w.mp !== undefined) win.maximizedPanelIndex = w.mp;
      if (w.ppr !== undefined) win.panelsPerRow = w.ppr;
      return win;
    }),
    activeWindowIndex: c.aw
  };
}

function expandPanel(c: CompactPanel): Panel {
  const p: Panel = {
    scopes: c.s.map((g) => ({ probeIds: g.pi, sourceIds: g.si })),
    composition: c.c === 'm' ? 'merged' : c.c === 'o' ? 'overlay' : 'split',
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
  if (c.sd === 'v') p.splitDirection = 'vertical';
  else if (c.sd === 'h') p.splitDirection = 'horizontal';
  if (c.cc === 1) p.cellControlsCollapsed = true;
  if (c.sw === 1) p.showWithheld = true;
  // Phase 122k F5 — per-panel window.
  if (typeof c.ws === 'string') p.windowStart = c.ws;
  if (typeof c.we === 'string') p.windowEnd = c.we;
  // Phase 131 per-cell config.
  if (typeof c.bn === 'number') p.bins = c.bn;
  if (c.ch !== undefined && typeof c.ch === 'object') {
    const cb: CellChannelBinding = {};
    if (typeof c.ch.x === 'string') cb.x = c.ch.x;
    if (typeof c.ch.y === 'string') cb.y = c.ch.y;
    if (typeof c.ch.sz === 'string') cb.size = c.ch.sz;
    if (typeof c.ch.co === 'string') cb.color = c.ch.co;
    if (c.ch.ns === 'total_count' || c.ch.ns === 'degree') cb.netSize = c.ch.ns;
    if (
      c.ch.nc === 'label' ||
      c.ch.nc === 'presence' ||
      c.ch.nc === 'uniform' ||
      c.ch.nc === 'source_overlay'
    )
      cb.netColor = c.ch.nc;
    if (Object.keys(cb).length > 0) p.channels = cb;
  }
  if (c.sb === 0) p.showBand = false;
  if (typeof c.fs === 'number') p.forceStrength = c.fs;
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
  // Phase 122i revision (C3). Optional `mp`; when present must be a
  // valid panel index. Out-of-bounds → reject (the URL is malformed,
  // not just "no maximize").
  if (v.mp !== undefined) {
    if (typeof v.mp !== 'number' || !Number.isInteger(v.mp) || v.mp < 0 || v.mp >= v.p.length)
      return false;
  }
  if (v.ppr !== undefined) {
    if (
      typeof v.ppr !== 'number' ||
      !Number.isInteger(v.ppr) ||
      v.ppr < 1 ||
      v.ppr > MAX_PANELS_PER_WINDOW
    )
      return false;
  }
  return v.p.every(isCompactPanel);
}

function isCompactPanel(v: unknown): v is CompactPanel {
  if (!isRecord(v)) return false;
  if (!Array.isArray(v.s) || v.s.length === 0) return false;
  if (!v.s.every(isCompactScopeGroup)) return false;
  if (v.c !== 'm' && v.c !== 's' && v.c !== 'o') return false;
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
  // Phase 122i revision short keys.
  if (v.sd !== undefined && v.sd !== 'h' && v.sd !== 'v') return false;
  if (v.cc !== undefined && v.cc !== 1) return false;
  if (v.sw !== undefined && v.sw !== 1) return false;
  // Phase 122k F5 — per-panel window keys.
  if (v.ws !== undefined && typeof v.ws !== 'string') return false;
  if (v.we !== undefined && typeof v.we !== 'string') return false;
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
