// UI-theme rune (Color Themes feature) — the single source of truth for the
// active colour theme (system | dark | light | contrast-dark | contrast-light).
// Mirrors `locale.svelte.ts`: a Svelte 5 `$state` rune persisted to localStorage
// and applied to `<html data-theme>`. Reading `theme()` inside a component's
// reactive scope makes a switch re-render WITHOUT a page reload.
//
// Resolution order (privacy-clean — NO cookie, AĒR no-tracking stance):
//   localStorage 'aer.theme'  →  'dark' (DEFAULT_THEME)
//
// FOUC: the static prerender ships `<html data-theme="dark">` and the
// render-blocking `static/theme-init.js` re-applies any non-dark stored choice
// BEFORE first paint (CSP `script-src 'self'` forbids an inline app.html
// script). This module then re-applies on hydration and on every switch, so the
// init script and the rune never disagree.

import { browser } from '$app/environment';
import {
  clampTheme,
  DEFAULT_THEME,
  themeAttr,
  THEME_STORAGE_KEY,
  THEMES,
  type Theme
} from './theme-internals';

export { THEMES, DEFAULT_THEME };
export type { Theme };

function resolveInitial(): Theme {
  if (!browser) return DEFAULT_THEME;
  let stored: string | null = null;
  try {
    stored = localStorage.getItem(THEME_STORAGE_KEY);
  } catch {
    // Private-mode / disabled storage — fall through to the default.
  }
  return clampTheme(stored) ?? DEFAULT_THEME;
}

/** Reflect a choice onto `<html>`. `system` removes the attribute so the
 *  `prefers-color-scheme` fallback in tokens.css governs. */
function applyTheme(t: Theme): void {
  if (!browser) return;
  const attr = themeAttr(t);
  if (attr === null) document.documentElement.removeAttribute('data-theme');
  else document.documentElement.setAttribute('data-theme', attr);
}

// Reactive SoT. Stays at DEFAULT_THEME on the server so prerendered output is
// deterministic; resolved + applied on the client at module load.
let current = $state<Theme>(DEFAULT_THEME);

if (browser) {
  current = resolveInitial();
  applyTheme(current);
}

/** The active theme choice. Reactive — read inside a component / `$derived`. */
export function theme(): Theme {
  return current;
}

/** Switch the theme: update the rune (→ live re-render), persist, apply to DOM. */
export function setTheme(next: Theme): void {
  if (next === current) return;
  current = next;
  if (!browser) return;
  try {
    localStorage.setItem(THEME_STORAGE_KEY, next);
  } catch {
    // Storage unavailable — the choice still holds for this session via the rune.
  }
  applyTheme(next);
}
