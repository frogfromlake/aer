import { describe, it, expect } from 'vitest';

import {
  clampTheme,
  themeAttr,
  DEFAULT_THEME,
  THEMES,
  type Theme
} from '../../src/lib/state/theme-internals';

// Color Themes feature — pin the pure resolution helpers that both the rune
// shell (theme.svelte.ts) and the anti-FOUC bootstrap (static/theme-init.js)
// rely on. The browser-only rune module is covered by E2E (ADR-041).

describe('THEMES / DEFAULT_THEME', () => {
  it('offers the five agreed choices', () => {
    expect(THEMES).toEqual(['system', 'dark', 'light', 'contrast-dark', 'contrast-light']);
  });
  it('defaults to dark (Design Brief §5.5)', () => {
    expect(DEFAULT_THEME).toBe('dark');
  });
});

describe('clampTheme', () => {
  it('passes through every known theme', () => {
    for (const t of THEMES) expect(clampTheme(t)).toBe(t);
  });
  it('rejects unknown / empty / null values', () => {
    expect(clampTheme('solar')).toBeNull();
    expect(clampTheme('')).toBeNull();
    expect(clampTheme(null)).toBeNull();
    expect(clampTheme(undefined)).toBeNull();
  });
});

describe('themeAttr', () => {
  it('maps system to null so the data-theme attribute is removed', () => {
    expect(themeAttr('system')).toBeNull();
  });
  it('maps every non-system theme to its identical data-theme value', () => {
    const nonSystem: Theme[] = ['dark', 'light', 'contrast-dark', 'contrast-light'];
    for (const t of nonSystem) expect(themeAttr(t)).toBe(t);
  });
});
