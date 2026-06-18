// Pillar state helpers (Phase 122h / ADR-033 §2 — Phase 122k cleanup).
//
// `pickPillar` switches the active Pillar in the URL state and seeds the
// target pillar's PillarState by cloning the current pillar's focused
// panel when the target is empty. Single URL grammar (Phase 122k): only
// `activePillar` is written; the legacy `viewingMode` URL key is gone.

import { defaultPresentationForPillar, getPillar, type PillarDefinition } from '$lib/presentations';
import { setUrl, urlState } from '$lib/state/url.svelte';
import type { UrlState, PillarId } from '$lib/state/url-internals';
import { seedPillarFromCurrent } from './pillar-internals';

// Phase 142 — the pure cross-pillar seed transform lives in
// `pillar-internals.ts` (node-unit-testable). Phase 144 — the plain-language
// pillar question/stance copy moved to Paraglide chrome messages (consumed by
// PillarSwitch.svelte), so it is no longer re-exported here.

/**
 * Switch the active Pillar. Idempotent — no-op when the requested Pillar
 * is already active. When the target pillar has no PillarState yet, the
 * current pillar's focused panel is cloned (with its view rebased to the
 * target pillar's default) so the user keeps continuity across switches.
 */
export function pickPillar(id: PillarId): void {
  const url = urlState();
  const currentPillar: PillarId = url.activePillar ?? 'aleph';
  if (id === currentPillar) return;

  const updates: Partial<UrlState> = { activePillar: id };
  if (url.pillars) {
    const targetState = url.pillars[id];
    if (!targetState) {
      const seeded = seedPillarFromCurrent(
        url.pillars,
        currentPillar,
        id,
        defaultPresentationForPillar
      );
      if (seeded) {
        updates.pillars = { ...url.pillars, [id]: seeded };
      }
    }
  }
  setUrl(updates);
}

/** Current Pillar definition derived from URL state. Defaults to Aleph. */
export function activePillarDefinition(): PillarDefinition {
  return getPillar(urlState().activePillar);
}
