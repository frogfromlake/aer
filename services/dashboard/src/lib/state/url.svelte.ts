// URL-backed shared state for the Atmosphere (Design Brief §5.5 —
// "deep-linkable state"). This module is the single write-path for the
// five URL-governed parameters:
//
//   ?from=<ISO>         — start of the current time window
//   ?to=<ISO>           — end of the current time window
//   ?probe=<probeId>    — currently-selected probe (opens the side panel)
//   ?resolution=<r>     — temporal aggregation resolution
//   ?viewingMode=<m>    — Atmosphere viewing mode
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
  type Resolution,
  type UrlState,
  type ViewingMode
} from './url-internals';

export type { Resolution, UrlState, ViewingMode };

// Inline browser check (rather than `$app/environment`) so this module
// is importable under plain vitest without a SvelteKit resolver.
const browser = typeof window !== 'undefined';

// A single module-scoped store: the URL is a singleton per tab, so a
// second instance would only desync. Components subscribe via the
// `urlState` rune object below; direct mutation goes through `setUrl`.
let internalState = $state<UrlState>({ ...EMPTY_URL_STATE });

let hydrated = false;

function hydrateOnce(): void {
  if (hydrated || !browser) return;
  internalState = readFromSearch(window.location.search);
  window.addEventListener('popstate', () => {
    internalState = readFromSearch(window.location.search);
  });
  hydrated = true;
}

/**
 * Reactive snapshot of the URL-backed state. Reads return the current
 * value; assign through `setUrl`/`patchUrl` to update both state and URL.
 */
export function urlState(): UrlState {
  hydrateOnce();
  return internalState;
}

/**
 * Replace the URL-backed state. Writes via `history.replaceState` so
 * rapid scrubber drags do not flood the history stack.
 */
export function setUrl(next: Partial<UrlState>): void {
  hydrateOnce();
  const merged: UrlState = { ...internalState, ...next };
  internalState = merged;
  if (!browser) return;
  const search = writeToSearch(merged);
  const url = `${window.location.pathname}${search}${window.location.hash}`;
  window.history.replaceState(window.history.state, '', url);
}
