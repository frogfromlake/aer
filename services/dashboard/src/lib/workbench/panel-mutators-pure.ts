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
  return {
    ...p,
    scopes: p.scopes.map((g) => ({
      probeIds: [...g.probeIds],
      sourceIds: [...g.sourceIds]
    }))
  };
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
  let nextPanel = update(panel);
  // Phase 122i revision (B1): `locked` is scope-only. Updates to view /
  // metric / layer / composition / resolution / normalization / topN /
  // cellControlsCollapsed all apply; mutations to `scopes` are silently
  // discarded so a stale call site can never escape the scope-lock.
  if (panel.locked) {
    nextPanel = { ...nextPanel, scopes: panel.scopes };
  }
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
