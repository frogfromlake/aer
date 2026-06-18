// UI-locale rune (Phase 144 / ADR-042) — the single source of truth for the
// app-UI language (en | de). It feeds three consumers from ONE value:
//   1. Paraglide message functions, via `overwriteGetLocale` (below). Because
//      Paraglide resolves the locale per `m.*()` call and `current` is a
//      Svelte 5 `$state`, reading it inside a component's reactive scope makes
//      a language switch re-render WITHOUT a page reload.
//   2. The BFF editorial-content `?locale=` (the `contentQuery` call-sites read
//      `locale()` into their TanStack queryKey, so content refetches on switch).
//   3. `Intl` date/number formatting ($lib/localization/format.ts).
//
// Resolution order (privacy-clean — NO cookie, AĒR no-tracking stance):
//   ?lang=en|de  (deep-link / share, round-tripped via the URL state)
//     → localStorage 'aer.locale'  (persisted preference)
//       → navigator.language ∈ {en,de}
//         → 'en'
//
// The selector (chrome/LocaleSwitch.svelte) calls `setLocale`, which updates
// the rune + localStorage + the `?lang=` URL param (through the one URL
// write-path so it is preserved across other mutations). No path-routing, so
// the static (adapter-static) prerender is unaffected: prerendered HTML is
// English and the client re-renders to the resolved locale on hydration.

import { browser } from '$app/environment';
import { invalidate } from '$app/navigation';
import { overwriteGetLocale } from '$lib/paraglide/runtime';
import { clampLocale, DEFAULT_LOCALE, type Locale } from './url-internals';
import { setUrl, urlState } from './url.svelte';

export type { Locale };
export { LOCALES } from './url-internals';

const STORAGE_KEY = 'aer.locale';

function resolveInitial(): Locale {
  if (!browser) return DEFAULT_LOCALE;
  // `urlState()` is already hydrated at this point (url.svelte.ts hydrates at
  // module load, which runs before this module's body via import ordering).
  const fromUrl = urlState().lang;
  if (fromUrl) return fromUrl;
  let stored: string | null = null;
  try {
    stored = localStorage.getItem(STORAGE_KEY);
  } catch {
    // Private-mode / disabled storage — fall through to navigator.
  }
  const fromStorage = clampLocale(stored);
  if (fromStorage) return fromStorage;
  const fromNav = clampLocale(navigator.language);
  if (fromNav) return fromNav;
  return DEFAULT_LOCALE;
}

// The reactive SoT. Initialised once on the client; stays at the SSR/prerender
// default ('en') on the server so prerendered output is deterministic.
let current = $state<Locale>(DEFAULT_LOCALE);

if (browser) {
  current = resolveInitial();
  // Back/forward to a `?lang=`-bearing URL re-applies that locale. A URL
  // WITHOUT `?lang=` keeps the current preference (localStorage), per the
  // resolution order — so popstate never silently drops to English.
  window.addEventListener('popstate', () => {
    const fromUrl = clampLocale(new URLSearchParams(window.location.search).get('lang'));
    if (fromUrl && fromUrl !== current) current = fromUrl;
  });
}

// Feed Paraglide. Recomputed per `m.*()` call (Paraglide does not cache), so
// the `current` read is tracked by Svelte and a switch re-renders messages.
overwriteGetLocale(() => current);

/** The active UI locale. Reactive — read inside a component / `$derived`. */
export function locale(): Locale {
  return current;
}

/**
 * Switch the UI locale. Updates the rune (→ Paraglide re-render + content
 * refetch via queryKeys), persists to localStorage, and pins `?lang=` in the
 * URL (via the single URL write-path, so it survives later mutations). No
 * reload — all surface state is URL-backed and stays put.
 */
export function setLocale(next: Locale): void {
  if (next === current && urlState().lang === next) return;
  current = next;
  if (!browser) return;
  try {
    localStorage.setItem(STORAGE_KEY, next);
  } catch {
    // Storage unavailable — the `?lang=` URL param still carries the choice.
  }
  setUrl({ lang: next });
  // Re-run loads that fetch locale-specific data outside TanStack (the
  // Working-Paper markdown load `depends('app:locale')`). A no-op on pages
  // without that dependency. Content queries refetch via their queryKeys.
  void invalidate('app:locale');
}
