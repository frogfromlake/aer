// Phase 122i / ADR-034 — Workbench-state mutators (rune-store entry points).
//
// Thin wrappers around the pure helpers in `./panel-mutators-pure.ts`
// that thread `urlState()` / `setUrl()` for runtime use. The pure
// helpers are the testable surface (see `panel-mutators.test.ts`); this
// file only adapts them to the rune store so UI components have a
// declarative API.

import { setUrl, urlState } from '$lib/state/url.svelte';
import {
  EMPTY_URL_STATE,
  type CellOverridePatch,
  type Panel,
  type PillarState,
  type ScopeGroup,
  type PillarId,
  type WorkbenchWindow
} from '$lib/state/url-internals';
import {
  addPanelPure,
  addScopeGroupPure,
  addWindowPure,
  applyCellOverride,
  clearCellOverrides,
  focusPanelPure,
  removeCellOverride,
  removePanelPure,
  updatePanelPure,
  type PanelPath
} from './panel-mutators-pure';

export type { PanelPath, WindowPath } from './panel-mutators-pure';
export { EMPTY_URL_STATE };

/** Read the active pillar's PillarState, or `null` if none in scope. */
export function activePillarState(): { pillar: PillarId | null; state: PillarState | null } {
  const url = urlState();
  const pillar = url.activePillar;
  if (!pillar) return { pillar: null, state: null };
  const state = url.pillars?.[pillar] ?? null;
  return { pillar, state };
}

/** Replace a Panel at `path` with the result of `update(prev)`. */
export function updatePanel(path: PanelPath, update: (prev: Panel) => Panel): void {
  const next = updatePanelPure(urlState().pillars, path, update);
  if (next) setUrl({ pillars: next });
}

/** Append a new Panel (cloned from `template`) to the focused window of `pillar`. */
export function addPanel(pillar: PillarId, template?: Panel): void {
  const next = addPanelPure(urlState().pillars, pillar, template);
  if (next) setUrl({ pillars: next });
}

/** Remove the panel at `path`. */
export function removePanel(path: PanelPath): void {
  const next = removePanelPure(urlState().pillars, path);
  if (next) setUrl({ pillars: next });
}

/** Focus a Panel at `path`. */
export function focusPanel(path: PanelPath): void {
  const current = urlState().pillars;
  const next = focusPanelPure(current, path);
  // focusPanelPure returns the SAME reference when the panel is already
  // focused. Without this guard, clicking anywhere inside an already-focused
  // panel still calls setUrl → a fresh internalState object → a global
  // re-render every click (felt as a "panel refresh", and reset in-cell
  // interactions: d3 selection/drag, the how-to-read collapse). Only write
  // when focus actually moved.
  if (next && next !== current) setUrl({ pillars: next });
}

/** Append a new ScopeGroup to the focused panel of `pillar`. */
export function addScopeGroupToFocused(pillar: PillarId, template?: ScopeGroup): void {
  const next = addScopeGroupPure(urlState().pillars, pillar, template);
  if (next) setUrl({ pillars: next });
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

/** Phase 126 — set/merge a per-cell override (a partial of the cell-shape
 *  levers) on the panel at `path`. A lever set to `undefined` reverts it to the
 *  panel default. */
export function setCellOverride(path: PanelPath, cellKey: string, patch: CellOverridePatch): void {
  updatePanel(path, (p) => applyCellOverride(p, cellKey, patch));
}

/** Phase 126 — clear one cell's override ("Reset to panel default"). */
export function resetCellOverride(path: PanelPath, cellKey: string): void {
  updatePanel(path, (p) => removeCellOverride(p, cellKey));
}

/** Phase 126 — clear every per-cell override on the panel ("Reset all cells"). */
export function resetAllCellOverrides(path: PanelPath): void {
  updatePanel(path, (p) => clearCellOverrides(p));
}

/** Reserved for future Window-Tab UI. */
export function addWindow(pillar: PillarId, template?: WorkbenchWindow): void {
  const next = addWindowPure(urlState().pillars, pillar, template);
  if (next) setUrl({ pillars: next });
}

/** Phase 122k §14 finding 6 — set or clear the panels-per-row override
 *  on a window. Passing `null` clears (auto-fill heuristic resumes). */
export function setPanelsPerRow(
  pillar: PillarId,
  windowIndex: number,
  panelsPerRow: number | null
): void {
  const pillars = urlState().pillars;
  if (!pillars) return;
  const pillarState = pillars[pillar];
  if (!pillarState) return;
  const win = pillarState.windows[windowIndex];
  if (!win) return;
  const nextWin: WorkbenchWindow = { ...win };
  if (panelsPerRow === null) delete nextWin.panelsPerRow;
  else nextWin.panelsPerRow = panelsPerRow;
  const nextWindows = pillarState.windows.map((w, i) => (i === windowIndex ? nextWin : w));
  setUrl({
    pillars: {
      ...pillars,
      [pillar]: { ...pillarState, windows: nextWindows }
    }
  });
}

/** Initialise an empty Workbench at `pillar` with a single panel built
 *  from `template`. Used when the user adds the first panel without
 *  going through a Dossier entry path. */
export function ensurePillar(pillar: PillarId, template: Panel): void {
  const url = urlState();
  if (url.pillars?.[pillar]) return;
  const pillarState: PillarState = {
    windows: [{ panels: [template], focusedPanelIndex: 0 }],
    activeWindowIndex: 0
  };
  const base = url.pillars ?? { aleph: null, episteme: null, rhizome: null };
  setUrl({
    activePillar: pillar,
    pillars: { ...base, [pillar]: pillarState }
  });
}
