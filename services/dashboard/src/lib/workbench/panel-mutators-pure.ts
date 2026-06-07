// Phase 122i / ADR-034 — Workbench-state mutators (pure helpers).
//
// Each helper takes the current `WorkbenchPillarsState` plus the
// location of the change and returns the next state. Pure helpers are
// the tested surface; the rune-backed wrappers in `panel-mutators.ts`
// just thread them through `urlState()` / `setUrl()` for runtime use.
//
// Hard caps mirror the URL-state constants: `MAX_PANELS_PER_WINDOW`,
// `MAX_WINDOWS_PER_PILLAR`. Over-cap operations return the input by
// reference so callers can branch on referential equality.

import {
  MAX_PANELS_PER_WINDOW,
  MAX_WINDOWS_PER_PILLAR,
  type CellChannelBinding,
  type CellOverride,
  type CellOverridePatch,
  type Panel,
  type PillarState,
  type ScopeGroup,
  type ViewingMode,
  type WorkbenchPillarsState,
  type WorkbenchWindow
} from '../state/url-internals';

export interface PanelPath {
  pillar: ViewingMode;
  windowIndex: number;
  panelIndex: number;
}

export interface WindowPath {
  pillar: ViewingMode;
  windowIndex: number;
}

function pillarsOrEmpty(state: WorkbenchPillarsState | null): WorkbenchPillarsState {
  return state ?? { aleph: null, episteme: null, rhizome: null };
}

function setPillar(
  state: WorkbenchPillarsState | null,
  pillar: ViewingMode,
  next: PillarState | null
): WorkbenchPillarsState {
  const base = pillarsOrEmpty(state);
  return { ...base, [pillar]: next };
}

function setWindow(pillar: PillarState, windowIndex: number, next: WorkbenchWindow): PillarState {
  const windows = pillar.windows.slice();
  windows[windowIndex] = next;
  return { ...pillar, windows };
}

function setPanel(window: WorkbenchWindow, panelIndex: number, next: Panel): WorkbenchWindow {
  const panels = window.panels.slice();
  panels[panelIndex] = next;
  return { ...window, panels };
}

function clonePanel(p: Panel): Panel {
  const clone: Panel = {
    ...p,
    scopes: p.scopes.map((g) => ({
      probeIds: [...g.probeIds],
      sourceIds: [...g.sourceIds]
    }))
  };
  // Phase 126 — deep-copy the per-cell override map (and its nested channel
  // objects) so a cloned panel never shares mutable override state with its
  // source. The Phase-126 mutators all rebuild immutably, so this is defensive,
  // but it keeps clonePanel's deep-copy contract honest for future editors.
  if (p.cellOverrides) {
    const co: Record<string, CellOverride> = {};
    for (const [k, ov] of Object.entries(p.cellOverrides)) {
      co[k] = ov.channels ? { ...ov, channels: { ...ov.channels } } : { ...ov };
    }
    clone.cellOverrides = co;
  }
  return clone;
}

// ---------------------------------------------------------------------------
// Pure mutators
// ---------------------------------------------------------------------------

export function updatePanelPure(
  pillars: WorkbenchPillarsState | null,
  path: PanelPath,
  update: (prev: Panel) => Panel
): WorkbenchPillarsState | null {
  const base = pillarsOrEmpty(pillars);
  const pillar = base[path.pillar];
  if (!pillar) return pillars;
  const win = pillar.windows[path.windowIndex];
  if (!win) return pillars;
  const panel = win.panels[path.panelIndex];
  if (!panel) return pillars;
  // Phase 122k — the Phase-122i B1 lock-protection ("locked panels reject
  // scope updates from the mutator") is retired. In K1 the only path that
  // edits scope is the ScopeEditor, which exposes the DF-lock toggle
  // directly. The mutator therefore commits the update verbatim; the lock
  // semantic moves entirely into the UI layer (the editor's DF-lock
  // dropdown governs lockedFunction; PanelHost's Apply handler writes
  // both scope and lock atomically).
  const nextPanel = update(panel);
  const nextWin = setPanel(win, path.panelIndex, nextPanel);
  return setPillar(base, path.pillar, setWindow(pillar, path.windowIndex, nextWin));
}

export function addPanelPure(
  pillars: WorkbenchPillarsState | null,
  pillar: ViewingMode,
  template?: Panel
): WorkbenchPillarsState | null {
  const base = pillarsOrEmpty(pillars);
  const pillarState = base[pillar];
  if (!pillarState) return pillars;
  const winIndex = pillarState.activeWindowIndex;
  const win = pillarState.windows[winIndex] ?? pillarState.windows[0];
  if (!win) return pillars;
  if (win.panels.length >= MAX_PANELS_PER_WINDOW) return pillars;
  const newPanel = template ? clonePanel(template) : clonePanel(win.panels[win.focusedPanelIndex]!);
  const nextWin: WorkbenchWindow = {
    panels: [...win.panels, newPanel],
    focusedPanelIndex: win.panels.length
  };
  return setPillar(base, pillar, setWindow(pillarState, winIndex, nextWin));
}

export function removePanelPure(
  pillars: WorkbenchPillarsState | null,
  path: PanelPath
): WorkbenchPillarsState | null {
  const base = pillarsOrEmpty(pillars);
  const pillar = base[path.pillar];
  if (!pillar) return pillars;
  const win = pillar.windows[path.windowIndex];
  if (!win) return pillars;
  if (win.panels.length === 0) return pillars;
  if (win.panels.length === 1) {
    if (pillar.windows.length === 1) return setPillar(base, path.pillar, null);
    const windows = pillar.windows.slice();
    windows.splice(path.windowIndex, 1);
    const activeIdx = Math.min(pillar.activeWindowIndex, windows.length - 1);
    return setPillar(base, path.pillar, { windows, activeWindowIndex: activeIdx });
  }
  const panels = win.panels.slice();
  panels.splice(path.panelIndex, 1);
  const focusedIdx = Math.min(win.focusedPanelIndex, panels.length - 1);
  const nextWin: WorkbenchWindow = { panels, focusedPanelIndex: focusedIdx };
  return setPillar(base, path.pillar, setWindow(pillar, path.windowIndex, nextWin));
}

export function focusPanelPure(
  pillars: WorkbenchPillarsState | null,
  path: PanelPath
): WorkbenchPillarsState | null {
  const base = pillarsOrEmpty(pillars);
  const pillar = base[path.pillar];
  if (!pillar) return pillars;
  const win = pillar.windows[path.windowIndex];
  if (!win) return pillars;
  if (win.focusedPanelIndex === path.panelIndex && pillar.activeWindowIndex === path.windowIndex)
    return pillars;
  const nextWin: WorkbenchWindow = { ...win, focusedPanelIndex: path.panelIndex };
  const nextPillar: PillarState = {
    ...setWindow(pillar, path.windowIndex, nextWin),
    activeWindowIndex: path.windowIndex
  };
  return setPillar(base, path.pillar, nextPillar);
}

export function addScopeGroupPure(
  pillars: WorkbenchPillarsState | null,
  pillar: ViewingMode,
  template?: ScopeGroup
): WorkbenchPillarsState | null {
  const base = pillarsOrEmpty(pillars);
  const pillarState = base[pillar];
  if (!pillarState) return pillars;
  const winIndex = pillarState.activeWindowIndex;
  const win = pillarState.windows[winIndex];
  if (!win) return pillars;
  const panelIndex = win.focusedPanelIndex;
  const panel = win.panels[panelIndex];
  if (!panel || panel.locked) return pillars;
  const newGroup: ScopeGroup = template
    ? { probeIds: [...template.probeIds], sourceIds: [...template.sourceIds] }
    : { probeIds: [...(panel.scopes[0]?.probeIds ?? [])], sourceIds: [] };
  const nextPanel: Panel = { ...panel, scopes: [...panel.scopes, newGroup] };
  const nextWin = setPanel(win, panelIndex, nextPanel);
  return setPillar(base, pillar, setWindow(pillarState, winIndex, nextWin));
}

// Phase 122i revision (C3) — Maximize-Mode mutators.

/**
 * Set or clear the maximized-panel pointer on a window. Pass `null` (or
 * an out-of-bounds index) to clear. Setting the same index twice is a
 * no-op (returns the input by reference so callers can detect idempotent
 * toggles via referential equality).
 */
export function setMaximizedPanelPure(
  pillars: WorkbenchPillarsState | null,
  pillar: ViewingMode,
  windowIndex: number,
  panelIndex: number | null
): WorkbenchPillarsState | null {
  const base = pillarsOrEmpty(pillars);
  const pillarState = base[pillar];
  if (!pillarState) return pillars;
  const win = pillarState.windows[windowIndex];
  if (!win) return pillars;
  const next: number | null =
    panelIndex !== null &&
    Number.isInteger(panelIndex) &&
    panelIndex >= 0 &&
    panelIndex < win.panels.length
      ? panelIndex
      : null;
  const current = win.maximizedPanelIndex ?? null;
  if (current === next) return pillars;
  const nextWin: WorkbenchWindow =
    next === null
      ? { ...win, maximizedPanelIndex: null }
      : { ...win, maximizedPanelIndex: next, focusedPanelIndex: next };
  return setPillar(base, pillar, setWindow(pillarState, windowIndex, nextWin));
}

/** Toggle: if the panel is currently maximized, un-maximize; otherwise
 *  maximize it. */
export function toggleMaximizedPanelPure(
  pillars: WorkbenchPillarsState | null,
  pillar: ViewingMode,
  windowIndex: number,
  panelIndex: number
): WorkbenchPillarsState | null {
  const base = pillarsOrEmpty(pillars);
  const pillarState = base[pillar];
  if (!pillarState) return pillars;
  const win = pillarState.windows[windowIndex];
  if (!win) return pillars;
  const isCurrentlyMaximized = win.maximizedPanelIndex === panelIndex;
  return setMaximizedPanelPure(
    pillars,
    pillar,
    windowIndex,
    isCurrentlyMaximized ? null : panelIndex
  );
}

// ---------------------------------------------------------------------------
// Phase 126 — per-cell override helpers (pure, Panel → Panel).
//
// These operate on a single Panel so they compose cleanly with
// `updatePanel(path, fn)` (the rune wrappers live in panel-mutators.ts) and are
// trivially vitest-pinned. A `patch` field set to `undefined` CLEARS that lever
// (reverts the cell to the panel default); a field absent from the patch is left
// untouched. The whole `cellOverrides` map and any emptied override are dropped
// so a panel never carries a no-op entry.
// ---------------------------------------------------------------------------

const CHANNEL_KEYS = ['x', 'y', 'size', 'color', 'netSize', 'netColor'] as const;

function setOrDelete<K extends keyof CellOverride>(
  o: CellOverride,
  key: K,
  value: CellOverride[K] | undefined
): void {
  if (value === undefined) delete o[key];
  else o[key] = value;
}

/** Merge `patch` over the cell's existing override (override wins per-lever).
 *  Channels merge one level deep: a cell can re-point a single scatter axis
 *  while inheriting the rest. A channel set to `undefined` reverts THAT channel
 *  to the panel default (per-cell channels re-point or inherit; they do not
 *  force-unbind a channel the panel binds). */
export function applyCellOverride(panel: Panel, cellKey: string, patch: CellOverridePatch): Panel {
  const existing: CellOverride = panel.cellOverrides?.[cellKey] ?? {};
  const next: CellOverride = { ...existing };

  if ('bins' in patch) setOrDelete(next, 'bins', patch.bins);
  if ('topN' in patch) setOrDelete(next, 'topN', patch.topN);
  if ('forceStrength' in patch) setOrDelete(next, 'forceStrength', patch.forceStrength);
  if ('showBand' in patch) setOrDelete(next, 'showBand', patch.showBand);
  if ('showEdges' in patch) setOrDelete(next, 'showEdges', patch.showEdges);
  if ('scales' in patch) setOrDelete(next, 'scales', patch.scales);
  if ('displayLanguage' in patch) setOrDelete(next, 'displayLanguage', patch.displayLanguage);
  if ('metric' in patch) setOrDelete(next, 'metric', patch.metric); // ADR-038 per-cell peek
  if ('channels' in patch) {
    const ch: CellChannelBinding = { ...(existing.channels ?? {}) };
    const pch = patch.channels ?? {};
    for (const k of CHANNEL_KEYS) {
      if (!(k in pch)) continue;
      const val = pch[k];
      if (val === undefined) delete ch[k];
      else (ch as Record<string, unknown>)[k] = val;
    }
    if (Object.keys(ch).length > 0) next.channels = ch;
    else delete next.channels;
  }

  const overrides: Record<string, CellOverride> = { ...(panel.cellOverrides ?? {}) };
  if (Object.keys(next).length > 0) overrides[cellKey] = next;
  else delete overrides[cellKey];

  const out: Panel = { ...panel };
  if (Object.keys(overrides).length > 0) out.cellOverrides = overrides;
  else delete out.cellOverrides;
  return out;
}

/** Drop one cell's override entirely (its "Reset to panel default"). */
export function removeCellOverride(panel: Panel, cellKey: string): Panel {
  if (!panel.cellOverrides || !(cellKey in panel.cellOverrides)) return panel;
  const overrides = { ...panel.cellOverrides };
  delete overrides[cellKey];
  const out: Panel = { ...panel };
  if (Object.keys(overrides).length > 0) out.cellOverrides = overrides;
  else delete out.cellOverrides;
  return out;
}

/** Drop every per-cell override on the panel (the panel-level "Reset all"). */
export function clearCellOverrides(panel: Panel): Panel {
  if (!panel.cellOverrides) return panel;
  const out: Panel = { ...panel };
  delete out.cellOverrides;
  return out;
}

export function addWindowPure(
  pillars: WorkbenchPillarsState | null,
  pillar: ViewingMode,
  template?: WorkbenchWindow
): WorkbenchPillarsState | null {
  const base = pillarsOrEmpty(pillars);
  const pillarState = base[pillar];
  if (!pillarState) return pillars;
  if (pillarState.windows.length >= MAX_WINDOWS_PER_PILLAR) return pillars;
  const win: WorkbenchWindow = template ?? {
    panels: [clonePanel(pillarState.windows[pillarState.activeWindowIndex]!.panels[0]!)],
    focusedPanelIndex: 0
  };
  const nextPillar: PillarState = {
    windows: [...pillarState.windows, win],
    activeWindowIndex: pillarState.windows.length
  };
  return setPillar(base, pillar, nextPillar);
}
