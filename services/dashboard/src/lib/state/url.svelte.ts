// URL-backed shared state for the Atmosphere (Design Brief §5.5 —
// "deep-linkable state"). This module is the single write-path for the
// URL-governed parameters. The full set is the `UrlState` interface in
// `url-internals.ts`; the load-bearing ones:
//
//   ?from=<ISO> / ?to=<ISO>   — current time window
//   ?probe=<probeId>          — Dossier mini-overlay, single probe (Phase 123a)
//   ?dossier=open             — Dossier large search/catalogue overlay (Phase 123a)
//   ?selectedProbes=a,b,c     — cross-surface probe-selection cart
//   ?resolution=<r>           — temporal aggregation resolution
//   ?activePillar= & ?aleph=/?episteme=/?rhizome=  — Workbench pillar state
//   ?normalization=<m>                              — global overlay
//
// Two constraints:
//   1. Reads must be cheap and reactive (stories and the time scrubber
//      both read the same state), so we expose a single `$state`-backed
//      object and refresh it on `popstate`.
//   2. Writes must be idempotent and not stack history entries — users
//      expect Back to undo the *descent*, not every scrubber nudge. We
//      use `history.replaceState` exclusively.
//
// URL writes bypass SvelteKit's router on purpose: `replaceState` and
// `pushState` from `$app/navigation` would trigger a re-navigation,
// which for a long-lived WebGL page is wasteful and in practice resets
// scroll. Plain `history.replaceState` keeps the canvas alive.
//
// Pure (de)serialisation lives in `./url-internals` so vitest can import
// it without a Svelte compiler pass.

import {
  EMPTY_URL_STATE,
  readFromSearch,
  writeToSearch,
  type DataLayer,
  type Resolution,
  type UrlState,
  type PillarId,
  type Presentation
} from './url-internals';

export type { DataLayer, Resolution, UrlState, PillarId, Presentation };

// Inline browser check (rather than `$app/environment`) so this module
// is importable under plain vitest without a SvelteKit resolver.
const browser = typeof window !== 'undefined';

// A single module-scoped store: the URL is a singleton per tab, so a
// second instance would only desync. Components subscribe via the
// `urlState` rune object below; direct mutation goes through `setUrl`.
let internalState = $state<UrlState>({ ...EMPTY_URL_STATE });

// Hydrate eagerly at module-load time (not inside urlState()) so that
// the mutation of `internalState` never occurs inside a $derived context,
// which Svelte 5 forbids (state_unsafe_mutation).
if (browser) {
  internalState = readFromSearch(window.location.search);
  window.addEventListener('popstate', () => {
    internalState = readFromSearch(window.location.search);
  });
}

/**
 * Re-hydrate the URL state from the current `window.location.search`.
 * Called from the (app) root layout's `afterNavigate` hook so SvelteKit
 * SPA navigations (Function-tile clicks, ProbePicker switches, redirects)
 * propagate into the rune store. Without this, components reading
 * `urlState()` after navigation would see stale pre-navigation values.
 */
export function rehydrateUrlState(): void {
  if (!browser) return;
  internalState = readFromSearch(window.location.search);
}

/**
 * Reactive snapshot of the URL-backed state. Reads return the current
 * value; assign through `setUrl`/`patchUrl` to update both state and URL.
 */
export function urlState(): UrlState {
  return internalState;
}

/**
 * Replace the URL-backed state. Writes via `history.replaceState` so
 * rapid scrubber drags do not flood the history stack.
 */
export function setUrl(next: Partial<UrlState>): void {
  const merged: UrlState = { ...internalState, ...next };
  internalState = merged;
  if (!browser) return;
  const search = writeToSearch(merged);
  const url = `${window.location.pathname}${search}${window.location.hash}`;
  window.history.replaceState(window.history.state, '', url);
}

/**
 * Phase 122k §11 finding — `pushUrl` writes via `history.pushState` so
 * the navigation is undoable via the browser back-button. Used by the
 * ScopeEditor's Apply commit: after creating a Panel, back-nav should
 * restore the pre-Apply state (no pillars), which causes the Workbench
 * page to auto-open the ScopeEditor with the resumed draft.
 */
export function pushUrl(next: Partial<UrlState>): void {
  const merged: UrlState = { ...internalState, ...next };
  internalState = merged;
  if (!browser) return;
  const search = writeToSearch(merged);
  const url = `${window.location.pathname}${search}${window.location.hash}`;
  window.history.pushState(window.history.state, '', url);
}

// Phase 135 — global overlays (account / admin / analyses / dossier) are
// MUTUALLY EXCLUSIVE: only one window may be open at a time. Opening one always
// clears the others and pushes a history entry, so browser back/forward steps
// through the window stack. Closing/toggling-shut replaces (no new entry).
export type OverlayName = 'account' | 'admin' | 'analyses' | 'dossier';
const OVERLAYS_CLEARED = {
  account: null,
  admin: null,
  analyses: null,
  dossier: null
} satisfies Pick<UrlState, OverlayName>;

/** True when `which` is the currently-open overlay. */
export function isOverlayOpen(which: OverlayName): boolean {
  return internalState[which] != null;
}

/** Open exactly `which` (clearing any other overlay); pushes a history entry. */
export function openOverlay(which: OverlayName, value: string = 'open'): void {
  pushUrl({ ...OVERLAYS_CLEARED, [which]: value });
}

/** Re-click behaviour: open `which` if closed, close it if already open. */
export function toggleOverlay(which: OverlayName, value: string = 'open'): void {
  if (internalState[which] != null) setUrl({ [which]: null });
  else openOverlay(which, value);
}
