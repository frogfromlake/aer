// Pillar pure internals (Phase 142 — extracted from pillar.ts).
//
// `pillar.ts` couples the public `pickPillar` to the Svelte-5 rune state layer
// (`$lib/state/url.svelte`), so it cannot load under the node-env unit runner.
// The genuinely pure logic — the cross-pillar seed transform and the two
// plain-language copy maps — lives here, behind type-only `$lib` imports (which
// the bundler erases) so it IS unit-testable in isolation. `pillar.ts` imports
// these back and re-exports the copy maps, so behaviour is unchanged.
//
// The one VALUE dependency the seed needs — `defaultPresentationForPillar` —
// is injected as a parameter rather than imported, so this module needs no
// `$lib` value import (the node runner cannot resolve the `$lib` alias).

import type {
  Panel,
  PillarState,
  ScopeGroup,
  PillarId,
  Presentation,
  WorkbenchPillarsState
} from '$lib/state/url-internals';

/** Resolver for a pillar's default presentation — injected so this module stays
 *  free of the `$lib/presentations` value import. */
export type DefaultPresentationResolver = (id: PillarId | null) => Presentation;

/**
 * Build a default PillarState for `targetPillar` by cloning the focused panel of
 * `currentPillar` and reassigning its view to the new pillar's default
 * presentation. Returns `null` when the source pillar has no focused panel to
 * clone (then the caller leaves the target pillar unseeded and the shell renders
 * an empty-scope prompt).
 *
 * Pure: never mutates `pillars`; the cloned scopes are deep copies so the seeded
 * panel cannot alias the source panel's arrays.
 */
export function seedPillarFromCurrent(
  pillars: WorkbenchPillarsState,
  currentPillar: PillarId,
  targetPillar: PillarId,
  defaultPresentationForPillar: DefaultPresentationResolver
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
    // Phase 123a — split is the scientifically-honest default: per-scope
    // small-multiples instead of pooling probes/sources onto one shared
    // axis (which would imply a cross-frame commensurability the methodology
    // refuses for scaled/intensive metrics). Merged stays an explicit opt-in.
    composition: 'split',
    view: defaultPresentationForPillar(targetPillar),
    metric: sourcePanel.metric,
    layer: sourcePanel.layer
  };
  return {
    windows: [{ panels: [seededPanel], focusedPanelIndex: 0 }],
    activeWindowIndex: 0
  };
}

/**
 * Plain-language Pillar questions — the visible identity of each tile in
 * PillarSwitch. Teaches the user what each Pillar asks in one sentence;
 * the Borges / Foucault / Deleuze framing lives in
 * `PillarDefinition.description` and on the ⓘ hover-card.
 */
export const PILLAR_QUESTIONS: Record<PillarId, string> = {
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
export const PILLAR_PLAIN_LANGUAGE: Record<PillarId, string> = {
  aleph:
    'A synchronic view of the active window — distributions, levels (e.g. mean sentiment), language mix, and per-source comparisons. No time axis.',
  episteme:
    'A diachronic view — how metrics, topics, and framings move across the window. Time is the main axis.',
  rhizome:
    'A relational view — who is mentioned with whom, which sources share topics, how concepts migrate.'
};
