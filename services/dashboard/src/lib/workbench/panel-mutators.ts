// Phase 122i / ADR-034 — Workbench state mutators.
//
// The Workbench-state tree (Pillar → Window → Panel → ScopeGroup) lives
// in the URL via `urlState()` / `setUrl()`. UI components mutate the
// tree through the helpers in this module so the editing surface stays
// declarative: "patch this panel" rather than "rebuild the entire URL
// state object".
//
// Every mutator is pure — it takes the current `WorkbenchPillarsState`
// (plus the location of the change) and returns a new state. The
// rune-store mutation happens in `applyPanel*()` which composes
// `setUrl({ activePillar, pillars: …new state… })`.
//
// Hard caps mirror the URL-state constants: `MAX_PANELS_PER_WINDOW`,
// `MAX_WINDOWS_PER_PILLAR`. Over-cap operations are no-ops and return
// the same object so callers can branch on referential equality.

import { setUrl, urlState } from '$lib/state/url.svelte';
import {
  EMPTY_URL_STATE,
  MAX_PANELS_PER_WINDOW,
  MAX_WINDOWS_PER_PILLAR,
  type Panel,
  type PillarState,
  type ScopeGroup,
  type ViewingMode,
  type WorkbenchPillarsState,
  type WorkbenchWindow
} from '$lib/state/url-internals';

// ---------------------------------------------------------------------------
// Path types: every mutator addresses the slice of state it touches.
// ---------------------------------------------------------------------------

export interface PanelPath {
  pillar: ViewingMode;
  windowIndex: number;
  panelIndex: number;
}

export interface WindowPath {
  pillar: ViewingMode;
  windowIndex: number;
}

// ---------------------------------------------------------------------------
// Pure tree mutators
// ---------------------------------------------------------------------------

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
// Rune-store entry points
// ---------------------------------------------------------------------------

/** Read the active pillar's PillarState, or `null` if none in scope. */
export function activePillarState(): { pillar: ViewingMode | null; state: PillarState | null } {
  const url = urlState();
  const pillar = url.activePillar;
  if (!pillar) return { pillar: null, state: null };
  const state = url.pillars?.[pillar] ?? null;
  return { pillar, state };
}

/** Replace a Panel at `path` with the result of `update(prev)`. No-op when
 *  the path does not resolve to an existing panel. */
export function updatePanel(path: PanelPath, update: (prev: Panel) => Panel): void {
  const url = urlState();
  const pillars = pillarsOrEmpty(url.pillars);
  const pillar = pillars[path.pillar];
  if (!pillar) return;
  const win = pillar.windows[path.windowIndex];
  if (!win) return;
  const panel = win.panels[path.panelIndex];
  if (!panel) return;
  if (panel.locked) return; // Locked panels are read-only.
  const nextPanel = update(panel);
  const nextWin = setPanel(win, path.panelIndex, nextPanel);
  const nextPillar = setWindow(pillar, path.windowIndex, nextWin);
  const nextPillars = setPillar(pillars, path.pillar, nextPillar);
  setUrl({ pillars: nextPillars });
}

/** Append a new Panel (cloned from `template`) to the focused window of
 *  `pillar`. No-op when at the `MAX_PANELS_PER_WINDOW` cap. */
export function addPanel(pillar: ViewingMode, template?: Panel): void {
  const url = urlState();
  const pillars = pillarsOrEmpty(url.pillars);
  const pillarState = pillars[pillar];
  if (!pillarState) return;
  const win = pillarState.windows[pillarState.activeWindowIndex] ?? pillarState.windows[0];
  if (!win) return;
  if (win.panels.length >= MAX_PANELS_PER_WINDOW) return;
  const winIndex = pillarState.activeWindowIndex;
  const newPanel = template ? clonePanel(template) : clonePanel(win.panels[win.focusedPanelIndex]!);
  // The new panel becomes the focused one so subsequent CellControls
  // edits target it without an extra click.
  const nextWin: WorkbenchWindow = {
    panels: [...win.panels, newPanel],
    focusedPanelIndex: win.panels.length
  };
  const nextPillar = setWindow(pillarState, winIndex, nextWin);
  setUrl({ pillars: setPillar(pillars, pillar, nextPillar) });
}

/** Remove the panel at `path`. If the window would become empty, the
 *  window is dropped instead; if the pillar's last window is dropped,
 *  the pillar slot is cleared (back to legacy-flat-URL territory). */
export function removePanel(path: PanelPath): void {
  const url = urlState();
  const pillars = pillarsOrEmpty(url.pillars);
  const pillar = pillars[path.pillar];
  if (!pillar) return;
  const win = pillar.windows[path.windowIndex];
  if (!win) return;
  if (win.panels.length === 0) return;
  if (win.panels.length === 1) {
    // Drop the window. If it was the last, clear the pillar slot.
    if (pillar.windows.length === 1) {
      setUrl({ pillars: setPillar(pillars, path.pillar, null) });
      return;
    }
    const windows = pillar.windows.slice();
    windows.splice(path.windowIndex, 1);
    const activeIdx = Math.min(pillar.activeWindowIndex, windows.length - 1);
    setUrl({
      pillars: setPillar(pillars, path.pillar, {
        windows,
        activeWindowIndex: activeIdx
      })
    });
    return;
  }
  const panels = win.panels.slice();
  panels.splice(path.panelIndex, 1);
  const focusedIdx = Math.min(win.focusedPanelIndex, panels.length - 1);
  const nextWin: WorkbenchWindow = { panels, focusedPanelIndex: focusedIdx };
  const nextPillar = setWindow(pillar, path.windowIndex, nextWin);
  setUrl({ pillars: setPillar(pillars, path.pillar, nextPillar) });
}

/** Focus a Panel at `path`. Used by Click → CellControls binding. */
export function focusPanel(path: PanelPath): void {
  const url = urlState();
  const pillars = pillarsOrEmpty(url.pillars);
  const pillar = pillars[path.pillar];
  if (!pillar) return;
  const win = pillar.windows[path.windowIndex];
  if (!win) return;
  if (win.focusedPanelIndex === path.panelIndex && pillar.activeWindowIndex === path.windowIndex)
    return;
  const nextWin: WorkbenchWindow = { ...win, focusedPanelIndex: path.panelIndex };
  const nextPillar: PillarState = {
    ...setWindow(pillar, path.windowIndex, nextWin),
    activeWindowIndex: path.windowIndex
  };
  setUrl({ pillars: setPillar(pillars, path.pillar, nextPillar) });
}

/** Append a new ScopeGroup to the focused panel of `pillar`. Used by
 *  the "+ Compare" affordance in the Panel header. */
export function addScopeGroupToFocused(pillar: ViewingMode, template?: ScopeGroup): void {
  const url = urlState();
  const pillars = pillarsOrEmpty(url.pillars);
  const pillarState = pillars[pillar];
  if (!pillarState) return;
  const winIndex = pillarState.activeWindowIndex;
  const win = pillarState.windows[winIndex];
  if (!win) return;
  const panelIndex = win.focusedPanelIndex;
  const panel = win.panels[panelIndex];
  if (!panel || panel.locked) return;
  const newGroup: ScopeGroup = template
    ? { probeIds: [...template.probeIds], sourceIds: [...template.sourceIds] }
    : // Default new group: inherit the first ScopeGroup's probes and an
      // empty sourceIds list so the user picks the comparison source.
      { probeIds: [...(panel.scopes[0]?.probeIds ?? [])], sourceIds: [] };
  const nextPanel: Panel = { ...panel, scopes: [...panel.scopes, newGroup] };
  const nextWin = setPanel(win, panelIndex, nextPanel);
  setUrl({ pillars: setPillar(pillars, pillar, setWindow(pillarState, winIndex, nextWin)) });
}

/** Remove a ScopeGroup at `groupIndex` from the panel at `path`. */
export function removeScopeGroup(path: PanelPath, groupIndex: number): void {
  updatePanel(path, (panel) => {
    if (panel.scopes.length <= 1) return panel;
    const scopes = panel.scopes.slice();
    scopes.splice(groupIndex, 1);
    return { ...panel, scopes };
  });
}

/** Reserved for future Window-Tab UI. Caps at `MAX_WINDOWS_PER_PILLAR`. */
export function addWindow(pillar: ViewingMode, template?: WorkbenchWindow): void {
  const url = urlState();
  const pillars = pillarsOrEmpty(url.pillars);
  const pillarState = pillars[pillar];
  if (!pillarState) return;
  if (pillarState.windows.length >= MAX_WINDOWS_PER_PILLAR) return;
  const win: WorkbenchWindow = template ?? {
    panels: [clonePanel(pillarState.windows[pillarState.activeWindowIndex]!.panels[0]!)],
    focusedPanelIndex: 0
  };
  const nextPillar: PillarState = {
    windows: [...pillarState.windows, win],
    activeWindowIndex: pillarState.windows.length
  };
  setUrl({ pillars: setPillar(pillars, pillar, nextPillar) });
}

/** Initialise an empty Workbench at `pillar` with a single panel built
 *  from `template`. Used when the user adds the first panel without
 *  going through a Dossier entry path (e.g. the future ScopeBar `+`
 *  affordance). */
export function ensurePillar(pillar: ViewingMode, template: Panel): void {
  const url = urlState();
  if (url.pillars?.[pillar]) return;
  const pillarState: PillarState = {
    windows: [{ panels: [clonePanel(template)], focusedPanelIndex: 0 }],
    activeWindowIndex: 0
  };
  const pillars = pillarsOrEmpty(url.pillars);
  setUrl({
    activePillar: pillar,
    pillars: setPillar(pillars, pillar, pillarState)
  });
}

/** Re-export the empty defaults so callers can construct templates
 *  without re-importing from url-internals. */
export { EMPTY_URL_STATE };
