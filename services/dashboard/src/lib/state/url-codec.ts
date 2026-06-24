// URL compact-serialisation codec + validation guards + base64url — extracted (Phase 141).
import type {
  CellChannelBinding,
  CellOverride,
  CompactCellOverride,
  CompactChannelBinding,
  CompactPanel,
  CompactPillarState,
  CompactScopeGroup,
  CompactWindow,
  Panel,
  PillarState,
  WorkbenchWindow
} from './url-types';
import {
  MAX_PANELS_PER_WINDOW,
  MAX_WINDOWS_PER_PILLAR,
  METRIC_NAME_RE,
  NORMALIZATIONS,
  RESOLUTIONS,
  VIEW_MODES
} from './url-types';

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
  if (p.maxNodes !== undefined) c.mn = p.maxNodes;
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
  if (p.scales === 'free') c.sc = 1;
  // Phase 122k F5 — per-panel window.
  if (p.windowStart !== undefined) c.ws = p.windowStart;
  if (p.windowEnd !== undefined) c.we = p.windowEnd;
  // Phase 125 — N-metric set for multivariate cells.
  if (p.metricSet !== undefined && p.metricSet.length > 0) c.ms = [...p.metricSet];
  // Phase 125a — categorical field chain for the sankey cell.
  if (p.fieldChain !== undefined && p.fieldChain.length > 0) c.fc = [...p.fieldChain];
  // Phase 125a — faceting field for small-multiples.
  if (p.facetField !== undefined && p.facetField.length > 0) c.ff = p.facetField;
  // Phase 131 per-cell config. bins/channels omitted when unset; showBand
  // omitted unless explicitly disabled (default = shown).
  if (p.bins !== undefined) c.bn = p.bins;
  if (p.channels !== undefined) {
    const cb = compactChannels(p.channels);
    if (cb) c.ch = cb;
  }
  if (p.showBand === false) c.sb = 0;
  if (p.showLabels === false) c.sl = 0;
  if (p.showEdges === true) c.se = 1;
  if (p.forceStrength !== undefined) c.fs = p.forceStrength;
  if (p.settleSeconds !== undefined) c.st = p.settleSeconds;
  if (p.displayLanguage === 'viewer') c.dl = 1;
  // Phase 126 — per-cell overrides. Each empty override is dropped; the whole
  // `co` map is omitted when no cell carries one.
  if (p.cellOverrides !== undefined) {
    const co: Record<string, CompactCellOverride> = {};
    for (const [k, ov] of Object.entries(p.cellOverrides)) {
      const cov = compactCellOverride(ov);
      if (cov) co[k] = cov;
    }
    if (Object.keys(co).length > 0) c.co = co;
  }
  return c;
}

// Phase 126 — channel + cell-override (de)compaction, shared by the panel-level
// channels field and the per-cell overrides.
function compactChannels(ch: CellChannelBinding): CompactChannelBinding | null {
  const cb: CompactChannelBinding = {};
  if (ch.x !== undefined) cb.x = ch.x;
  if (ch.y !== undefined) cb.y = ch.y;
  if (ch.size !== undefined) cb.sz = ch.size;
  if (ch.color !== undefined) cb.co = ch.color;
  if (ch.netSize !== undefined) cb.ns = ch.netSize;
  if (ch.netColor !== undefined) cb.nc = ch.netColor;
  if (ch.netMetric !== undefined) cb.nm = ch.netMetric;
  if (ch.netColorMetric !== undefined) cb.ncm = ch.netColorMetric;
  return Object.keys(cb).length > 0 ? cb : null;
}

function expandChannels(c: CompactChannelBinding): CellChannelBinding | null {
  const cb: CellChannelBinding = {};
  if (typeof c.x === 'string') cb.x = c.x;
  if (typeof c.y === 'string') cb.y = c.y;
  if (typeof c.sz === 'string') cb.size = c.sz;
  if (typeof c.co === 'string') cb.color = c.co;
  if (c.ns === 'total_count' || c.ns === 'degree' || c.ns === 'metric') cb.netSize = c.ns;
  if (
    c.nc === 'label' ||
    c.nc === 'presence' ||
    c.nc === 'uniform' ||
    c.nc === 'source_overlay' ||
    c.nc === 'metric' ||
    c.nc === 'community'
  )
    cb.netColor = c.nc;
  if (typeof c.nm === 'string' && c.nm.length > 0) cb.netMetric = c.nm;
  if (typeof c.ncm === 'string' && c.ncm.length > 0) cb.netColorMetric = c.ncm;
  return Object.keys(cb).length > 0 ? cb : null;
}

function compactCellOverride(ov: CellOverride): CompactCellOverride | null {
  const c: CompactCellOverride = {};
  if (ov.bins !== undefined) c.bn = ov.bins;
  if (ov.topN !== undefined) c.tN = ov.topN;
  if (ov.maxNodes !== undefined) c.mn = ov.maxNodes;
  if (ov.forceStrength !== undefined) c.fs = ov.forceStrength;
  if (ov.showBand !== undefined) c.sb = ov.showBand ? 1 : 0;
  if (ov.showLabels !== undefined) c.sl = ov.showLabels ? 1 : 0;
  if (ov.showEdges !== undefined) c.se = ov.showEdges ? 1 : 0;
  if (ov.scales !== undefined) c.sc = ov.scales === 'free' ? 1 : 0;
  if (ov.displayLanguage !== undefined) c.dl = ov.displayLanguage === 'viewer' ? 1 : 0;
  if (ov.channels !== undefined) {
    const cb = compactChannels(ov.channels);
    if (cb) c.ch = cb;
  }
  if (ov.metric !== undefined) c.mc = ov.metric;
  return Object.keys(c).length > 0 ? c : null;
}

function expandCellOverride(c: CompactCellOverride): CellOverride {
  const ov: CellOverride = {};
  if (typeof c.bn === 'number') ov.bins = c.bn;
  if (typeof c.tN === 'number') ov.topN = c.tN;
  if (typeof c.mn === 'number') ov.maxNodes = c.mn;
  if (typeof c.fs === 'number') ov.forceStrength = c.fs;
  if (c.sb === 0 || c.sb === 1) ov.showBand = c.sb === 1;
  if (c.sl === 0 || c.sl === 1) ov.showLabels = c.sl === 1;
  if (c.se === 0 || c.se === 1) ov.showEdges = c.se === 1;
  if (c.sc === 0 || c.sc === 1) ov.scales = c.sc === 1 ? 'free' : 'shared';
  if (c.dl === 0 || c.dl === 1) ov.displayLanguage = c.dl === 1 ? 'viewer' : 'source';
  if (c.ch !== undefined && typeof c.ch === 'object') {
    const cb = expandChannels(c.ch);
    if (cb) ov.channels = cb;
  }
  if (typeof c.mc === 'string' && c.mc.length > 0) ov.metric = c.mc;
  return ov;
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
  if (c.mn !== undefined) p.maxNodes = c.mn;
  if (c.L === 1) p.locked = true;
  if (c.lr !== undefined) p.lockedReason = c.lr;
  if (c.lf !== undefined) p.lockedFunction = c.lf;
  if (c.sd === 'v') p.splitDirection = 'vertical';
  else if (c.sd === 'h') p.splitDirection = 'horizontal';
  if (c.cc === 1) p.cellControlsCollapsed = true;
  if (c.sw === 1) p.showWithheld = true;
  if (c.sc === 1) p.scales = 'free';
  // Phase 122k F5 — per-panel window.
  if (typeof c.ws === 'string') p.windowStart = c.ws;
  if (typeof c.we === 'string') p.windowEnd = c.we;
  // Phase 125a — categorical field chain for the sankey cell.
  if (Array.isArray(c.fc) && c.fc.length > 0)
    p.fieldChain = c.fc.filter((m) => typeof m === 'string');
  // Phase 125a — faceting field for small-multiples.
  if (typeof c.ff === 'string' && c.ff.length > 0) p.facetField = c.ff;
  // Phase 125 — N-metric set for multivariate cells. Pre-125a sankey URLs
  // stored the field chain in `ms`; route that legacy form into `fieldChain`.
  if (Array.isArray(c.ms) && c.ms.length > 0) {
    const list = c.ms.filter((m) => typeof m === 'string');
    if (c.v === 'sankey') {
      if (p.fieldChain === undefined) p.fieldChain = list;
    } else {
      p.metricSet = list;
    }
  }
  // Phase 131 per-cell config.
  if (typeof c.bn === 'number') p.bins = c.bn;
  if (c.ch !== undefined && typeof c.ch === 'object') {
    const cb = expandChannels(c.ch);
    if (cb) p.channels = cb;
  }
  if (c.sb === 0) p.showBand = false;
  if (c.sl === 0) p.showLabels = false;
  if (c.se === 1) p.showEdges = true;
  if (typeof c.fs === 'number') p.forceStrength = c.fs;
  if (typeof c.st === 'number') p.settleSeconds = c.st;
  if (c.dl === 1) p.displayLanguage = 'viewer';
  // Phase 126 — per-cell overrides. Empty overrides are dropped so a decoded
  // panel never carries a no-op entry.
  if (c.co !== undefined && typeof c.co === 'object') {
    const co: Record<string, CellOverride> = {};
    for (const [k, cov] of Object.entries(c.co)) {
      const ov = expandCellOverride(cov);
      if (Object.keys(ov).length > 0) co[k] = ov;
    }
    if (Object.keys(co).length > 0) p.cellOverrides = co;
  }
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
  if (v.sc !== undefined && v.sc !== 1) return false;
  // Phase 122k F5 — per-panel window keys.
  if (v.ws !== undefined && typeof v.ws !== 'string') return false;
  if (v.we !== undefined && typeof v.we !== 'string') return false;
  // Phase 125 — N-metric set.
  if (v.ms !== undefined && (!Array.isArray(v.ms) || !v.ms.every((m) => typeof m === 'string')))
    return false;
  // Phase 125a — categorical field chain (sankey).
  if (v.fc !== undefined && (!Array.isArray(v.fc) || !v.fc.every((m) => typeof m === 'string')))
    return false;
  // Phase 125a — faceting field.
  if (v.ff !== undefined && typeof v.ff !== 'string') return false;
  // Phase 126 — per-cell overrides. A record of cellKey → CompactCellOverride.
  if (v.co !== undefined) {
    if (!isRecord(v.co)) return false;
    for (const cov of Object.values(v.co)) {
      if (!isCompactCellOverride(cov)) return false;
    }
  }
  return true;
}

function isCompactCellOverride(v: unknown): v is CompactCellOverride {
  if (!isRecord(v)) return false;
  if (v.bn !== undefined && (typeof v.bn !== 'number' || !Number.isFinite(v.bn))) return false;
  if (v.tN !== undefined && (typeof v.tN !== 'number' || !Number.isFinite(v.tN))) return false;
  if (v.fs !== undefined && (typeof v.fs !== 'number' || !Number.isFinite(v.fs))) return false;
  if (v.sb !== undefined && v.sb !== 0 && v.sb !== 1) return false;
  if (v.sl !== undefined && v.sl !== 0 && v.sl !== 1) return false;
  if (v.sc !== undefined && v.sc !== 0 && v.sc !== 1) return false;
  if (v.dl !== undefined && v.dl !== 0 && v.dl !== 1) return false;
  // `ch` is expanded defensively (each field type-checked in expandChannels),
  // mirroring the panel-level channel handling — only its container shape is
  // validated here.
  if (v.ch !== undefined && !isRecord(v.ch)) return false;
  if (v.mc !== undefined && typeof v.mc !== 'string') return false;
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
