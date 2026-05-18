// Pillar state helpers (Phase 122h / ADR-033 §2 — Phase 122k cleanup).
//
// `pickPillar` switches the active Pillar in the URL state and seeds the
// target pillar's PillarState by cloning the current pillar's focused
// panel when the target is empty. Single URL grammar (Phase 122k): only
// `activePillar` is written; the legacy `viewingMode` URL key is gone.

import { defaultViewModeForPillar, getPillar, type PillarDefinition } from '$lib/viewmodes';
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
 * Switch the active Pillar. Idempotent — no-op when the requested Pillar
 * is already active. When the target pillar has no PillarState yet, the
 * current pillar's focused panel is cloned (with its view rebased to the
 * target pillar's default) so the user keeps continuity across switches.
 */
export function pickPillar(id: ViewingMode): void {
  const url = urlState();
  const currentPillar: ViewingMode = url.activePillar ?? 'aleph';
  if (id === currentPillar) return;

  const updates: Partial<UrlState> = { activePillar: id };
  if (url.pillars) {
    const targetState = url.pillars[id];
    if (!targetState) {
      const seeded = seedPillarFromCurrent(url.pillars, currentPillar, id);
      if (seeded) {
        updates.pillars = { ...url.pillars, [id]: seeded };
      }
    }
  }
  setUrl(updates);
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
  return getPillar(urlState().activePillar);
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
