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
import type { ViewingMode } from '$lib/state/url-internals';

/**
 * Switch the active Pillar and reconcile `viewMode` so the active Cell
 * is always a member of the new Pillar's presentation set. Idempotent —
 * no-op when the requested Pillar is already active.
 */
export function pickPillar(id: ViewingMode): void {
  const url = urlState();
  if (id === (url.viewingMode ?? 'aleph')) return;
  const currentViewMode = url.viewMode;
  const currentBelongs = currentViewMode !== null && pillarForViewMode(currentViewMode) === id;
  if (currentBelongs) {
    setUrl({ viewingMode: id });
  } else {
    setUrl({ viewingMode: id, viewMode: defaultViewModeForPillar(id) });
  }
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
