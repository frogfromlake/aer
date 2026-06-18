import { describe, it, expect } from 'vitest';
import {
  pickViewerLabelLanguage,
  SUPPORTED_LABEL_LANGUAGES
} from '../../src/lib/presentations/viewer-language';

describe('pickViewerLabelLanguage', () => {
  it('maps a region-tagged locale to its primary supported subtag', () => {
    expect(pickViewerLabelLanguage('de-DE')).toBe('de');
    expect(pickViewerLabelLanguage('fr-CA')).toBe('fr');
    expect(pickViewerLabelLanguage('en-GB')).toBe('en');
  });

  it('is case- and separator-insensitive', () => {
    expect(pickViewerLabelLanguage('DE')).toBe('de');
    expect(pickViewerLabelLanguage('de_AT')).toBe('de');
  });

  it('falls back to English for an unsupported or empty locale', () => {
    // ja is not a baked label language → English default rather than a silent
    // relabel to nothing.
    expect(pickViewerLabelLanguage('ja-JP')).toBe('en');
    expect(pickViewerLabelLanguage('')).toBe('en');
    expect(pickViewerLabelLanguage(undefined)).toBe('en');
    expect(pickViewerLabelLanguage(null)).toBe('en');
  });

  it('only ever returns a supported label language', () => {
    for (const raw of ['de-DE', 'zz', '', 'fr', 'en-US-x-foo']) {
      expect(SUPPORTED_LABEL_LANGUAGES).toContain(pickViewerLabelLanguage(raw));
    }
  });

  it('is the identity on the UI locales (en/de are both in the label set)', () => {
    // Phase 144 — components feed `locale()` (en|de) through this clamp; both
    // are baked label languages, so the relabel target equals the UI locale.
    expect(pickViewerLabelLanguage('en')).toBe('en');
    expect(pickViewerLabelLanguage('de')).toBe('de');
  });
});
