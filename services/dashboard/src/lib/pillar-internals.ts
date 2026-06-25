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
// Phase 148e — the one VALUE dependency, imported RELATIVELY (not via the `$lib`
// alias the node unit-runner cannot resolve). It is a pure leaf helper, so it
// adds no rune/Svelte coupling to this node-testable module.
import { defaultCompositionForView, initialLeversForView } from './state/url-types';

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
  const targetView = defaultPresentationForPillar(targetPillar);
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
    // Phase 148e — EXCEPT the co-occurrence network (the Rhizome default): one
    // relational graph, so a multi-source/-probe scope opens `merged` rather
    // than splitting into a refusal the moment the user switches to Rhizome.
    composition: defaultCompositionForView(targetView),
    view: targetView,
    metric: sourcePanel.metric,
    layer: sourcePanel.layer,
    // Phase 148e — chart-first, like buildPanelFromScopes: a pillar-switch seed
    // opens with its control strip collapsed so the cell shows without scrolling.
    cellControlsCollapsed: true,
    // Phase 148e — view-specific opening lever values (co-occurrence: sparse
    // labels + short settle so the seeded Rhizome graph is legible on open).
    ...initialLeversForView(targetView)
  };
  return {
    windows: [{ panels: [seededPanel], focusedPanelIndex: 0 }],
    activeWindowIndex: 0
  };
}

// Phase 144 — the plain-language Pillar questions + stance copy moved to
// Paraglide chrome messages (chrome_pillar_question_* / chrome_pillar_plain_*),
// consumed directly by PillarSwitch.svelte. They were UI display strings, not
// pure logic, so they now live with the other localized UI copy.
