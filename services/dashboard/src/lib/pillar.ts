// Pillar state helpers (Phase 122h / ADR-033 §2).
//
// `pickPillar` reconciles the active Pillar with the URL viewMode so a
// deep-linked Cell-view that lives under one Pillar (e.g. topic_evolution
// under Episteme) is preserved when the user switches Pillars only if the
// new Pillar's presentation set includes it. Otherwise the pillar's
// default viewMode is selected.
//
// Extracted from the legacy `SideRail.pickPillar()` + `FunctionLaneShell.switchPillar()`
// duplicate implementations so the PillarSwitch (Slice 4) and any future
// programmatic pillar-switch (e.g. keyboard shortcuts) share one source of
// truth.

import {
  defaultViewModeForPillar,
  getPillar,
  pillarForViewMode,
  type PillarDefinition
} from '$lib/viewmodes';
import { setUrl, urlState } from '$lib/state/url.svelte';
import type {
  Panel,
  PillarState,
  ScopeGroup,
  UrlState,
  ViewingMode,
  WorkbenchPillarsState
} from '$lib/state/url-internals';

/**
 * Switch the active Pillar and reconcile `viewMode` so the active Cell
 * is always a member of the new Pillar's presentation set. Idempotent —
 * no-op when the requested Pillar is already active.
 *
 * Phase 122i revision (A5 fix). The Phase-122h implementation wrote only
 * to `url.viewingMode`. In a pillar-state URL (`?activePillar=&aleph=…`),
 * the reader forces `viewingMode` to `null` (legacy params are dropped
 * when any pillar key is present), AND the writer ignores `viewingMode`
 * when `pillars` is non-null. The result: clicking Episteme/Rhizome
 * tiles in a pillar-state URL was a complete no-op. This fix writes both
 * fields, and additionally seeds the target pillar with a clone of the
 * current pillar's focused-panel scope so the user keeps continuity
 * across pillar-switches.
 */
export function pickPillar(id: ViewingMode): void {
  const url = urlState();
  const currentPillar: ViewingMode = url.activePillar ?? url.viewingMode ?? 'aleph';
  if (id === currentPillar) return;

  // Pillar-state form: flip activePillar + seed target pillar from the
  // current pillar's focused panel if it has no state yet. Also keep
  // legacy `viewingMode` in sync so PillarSwitch.activeId and any
  // remaining Phase-122h legacy readers agree.
  if (url.pillars) {
    const updates: Partial<UrlState> = { activePillar: id, viewingMode: id };
    const targetState = url.pillars[id];
    if (!targetState) {
      const seeded = seedPillarFromCurrent(url.pillars, currentPillar, id);
      if (seeded) {
        updates.pillars = { ...url.pillars, [id]: seeded };
      }
    }
    setUrl(updates);
    return;
  }

  // Legacy flat-URL form: write `viewingMode` (and `viewMode` if the
  // current viewMode doesn't belong to the new pillar). Also update
  // `activePillar` defensively — if the URL later migrates to pillar
  // form, the active-pillar pointer is already correct.
  const currentViewMode = url.viewMode;
  const currentBelongs = currentViewMode !== null && pillarForViewMode(currentViewMode) === id;
  if (currentBelongs) {
    setUrl({ viewingMode: id, activePillar: id });
  } else {
    setUrl({
      viewingMode: id,
      activePillar: id,
      viewMode: defaultViewModeForPillar(id)
    });
  }
}

/**
 * Build a default PillarState for `targetPillar` by cloning the focused
 * panel of `currentPillar` and reassigning its view to the new pillar's
 * default presentation. Returns `null` when the source pillar has no
 * focused panel to clone (then the caller leaves the target pillar
 * unseeded and the shell renders an empty-scope prompt).
 */
function seedPillarFromCurrent(
  pillars: WorkbenchPillarsState,
  currentPillar: ViewingMode,
  targetPillar: ViewingMode
): PillarState | null {
  const source = pillars[currentPillar];
  if (!source) return null;
  const sourceWindow = source.windows[source.activeWindowIndex] ?? source.windows[0];
  if (!sourceWindow) return null;
  const sourcePanel = sourceWindow.panels[sourceWindow.focusedPanelIndex] ?? sourceWindow.panels[0];
  if (!sourcePanel) return null;
  const seededPanel: Panel = {
    scopes: sourcePanel.scopes.map(
      (g): ScopeGroup => ({
        probeIds: [...g.probeIds],
        sourceIds: [...g.sourceIds]
      })
    ),
    composition: 'merged',
    view: defaultViewModeForPillar(targetPillar),
    metric: sourcePanel.metric,
    layer: sourcePanel.layer
  };
  return {
    windows: [{ panels: [seededPanel], focusedPanelIndex: 0 }],
    activeWindowIndex: 0
  };
}

/** Current Pillar definition derived from URL state. Defaults to Aleph. */
export function activePillarDefinition(): PillarDefinition {
  return getPillar(urlState().viewingMode);
}

/**
 * Plain-language Pillar questions — the visible identity of each tile in
 * PillarSwitch. Teaches the user what each Pillar asks in one sentence;
 * the Borges / Foucault / Deleuze framing lives in
 * `PillarDefinition.description` and on the ⓘ hover-card.
 */
export const PILLAR_QUESTIONS: Record<ViewingMode, string> = {
  aleph: 'What does the dataset look like right now?',
  episteme: 'How has the discourse shifted over time?',
  rhizome: 'How is the discourse connected?'
};

/**
 * Plain-language one-line "what this Pillar does to the dataset" — shown
 * below the active Pillar tile so the user understands the stance without
 * the WP framing. Examples reference real metrics so the user maps the
 * abstract stance to the concrete dashboard immediately.
 */
export const PILLAR_PLAIN_LANGUAGE: Record<ViewingMode, string> = {
  aleph:
    'A synchronic view of the active window — distributions, levels (e.g. mean sentiment), language mix, and per-source comparisons. No time axis.',
  episteme:
    'A diachronic view — how metrics, topics, and framings move across the window. Time is the main axis.',
  rhizome:
    'A relational view — who is mentioned with whom, which sources share topics, how concepts migrate.'
};
