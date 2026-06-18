import { describe, expect, it } from 'vitest';
import {
  intlLocale,
  localizedDate,
  localizedDateTime,
  localizedNumber
} from '../../src/lib/localization/format-core';
import { clampLocale } from '../../src/lib/state/url-internals';

// Phase 144 — pure formatting core + the locale clamp. The rune-aware wrappers
// (format.ts) and the rune itself (locale.svelte.ts) are E2E-covered (ADR-041).

describe('intlLocale', () => {
  it('maps the UI locale to a BCP-47 tag', () => {
    expect(intlLocale('en')).toBe('en-CA');
    expect(intlLocale('de')).toBe('de-DE');
  });
});

describe('localizedNumber', () => {
  it('groups thousands per locale', () => {
    expect(localizedNumber(1234567, 'en')).toBe('1,234,567');
    expect(localizedNumber(1234567, 'de')).toBe('1.234.567');
  });
  it('falls back to a plain string when Intl throws', () => {
    // A bogus option forces a RangeError inside toLocaleString.
    expect(localizedNumber(42, 'en', { style: 'currency' } as Intl.NumberFormatOptions)).toBe('42');
  });
});

describe('localizedDate', () => {
  it('renders ISO-like in EN and DD.MM.YYYY in DE', () => {
    const iso = '2026-03-09T10:00:00Z';
    expect(localizedDate(iso, 'en')).toBe('2026-03-09');
    // de-DE day/month order with the year; exact separators are Intl's.
    expect(localizedDate(iso, 'de')).toContain('2026');
    expect(localizedDate(iso, 'de')).not.toBe(localizedDate(iso, 'en'));
  });
  it('returns the raw input on an unparseable date', () => {
    expect(localizedDate('not-a-date', 'en')).toBe('not-a-date');
  });
});

describe('localizedDateTime', () => {
  it('produces a non-empty localized string for a valid timestamp', () => {
    const iso = '2026-03-09T10:00:00Z';
    expect(localizedDateTime(iso, 'en').length).toBeGreaterThan(0);
    expect(localizedDateTime(iso, 'de').length).toBeGreaterThan(0);
  });
  it('returns the raw input on an unparseable timestamp', () => {
    expect(localizedDateTime('nope', 'de')).toBe('nope');
  });
});

describe('clampLocale', () => {
  it('accepts supported primary subtags case-insensitively', () => {
    expect(clampLocale('de')).toBe('de');
    expect(clampLocale('DE')).toBe('de');
    expect(clampLocale('de-DE')).toBe('de');
    expect(clampLocale('de_AT')).toBe('de');
    expect(clampLocale('en-GB')).toBe('en');
  });
  it('rejects unsupported or empty input', () => {
    expect(clampLocale('fr')).toBeNull();
    expect(clampLocale('')).toBeNull();
    expect(clampLocale(null)).toBeNull();
    expect(clampLocale(undefined)).toBeNull();
  });
});
