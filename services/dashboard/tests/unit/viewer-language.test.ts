import { describe, it, expect } from 'vitest';
import {
  pickViewerLabelLanguage,
  viewerLabelLanguage,
  APP_CONTENT_LANGUAGE,
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
});

describe('viewerLabelLanguage (relabel target = app content language)', () => {
  it('returns the app content language, not the browser locale', () => {
    // The dashboard is English-only today; the relabel must follow the app
    // language so an English UI never relabels entities into a browser locale.
    expect(viewerLabelLanguage()).toBe(APP_CONTENT_LANGUAGE);
    expect(SUPPORTED_LABEL_LANGUAGES).toContain(viewerLabelLanguage());
  });
});
