// UI-theme primitives (pure, node-testable) — the static counterpart to the
// rune shell in `theme.svelte.ts`. Kept framework-free so the resolution logic
// is unit-tested under node-env Vitest (the rune module itself is browser-only,
// like `url-internals.ts` is to `locale.svelte.ts`).
//
// The colour theme is a personal *device* preference: persisted to localStorage
// ('aer.theme') but — unlike the UI locale — NOT mirrored to the URL, because a
// theme is never a shareable view parameter.

/** The selectable colour-theme choices. `system` follows the OS. */
export const THEMES = ['system', 'dark', 'light', 'contrast-dark', 'contrast-light'] as const;
export type Theme = (typeof THEMES)[number];

/** First-visit default (Design Brief §5.5 — dark is the primary theme). */
export const DEFAULT_THEME: Theme = 'dark';

/** localStorage key holding the persisted choice. */
export const THEME_STORAGE_KEY = 'aer.theme';

/** Narrow an untrusted string to a known Theme, or null if unrecognised. */
export function clampTheme(value: string | null | undefined): Theme | null {
  return value && (THEMES as readonly string[]).includes(value) ? (value as Theme) : null;
}

/**
 * The `data-theme` attribute value an active choice maps to, or `null` for
 * `system` — meaning the attribute should be REMOVED so the `prefers-color-scheme`
 * fallback in tokens.css governs. The non-system choices map 1:1 to the
 * `:root[data-theme='…']` selectors.
 */
export function themeAttr(theme: Theme): string | null {
  return theme === 'system' ? null : theme;
}
