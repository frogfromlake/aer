import { describe, expect, it } from 'vitest';

import {
  EMPTY_URL_STATE,
  readFromSearch,
  writeToSearch,
  type UrlState
} from '../../src/lib/state/url-internals';

// Test helper: builds a complete UrlState from EMPTY_URL_STATE + overrides.
// Keeps tests resilient against future UrlState extensions (Phase 122i
// added activePillar + pillars; future phases may add more).
function state(overrides: Partial<UrlState> = {}): UrlState {
  return { ...EMPTY_URL_STATE, ...overrides };
}

describe('readFromSearch', () => {
  it('returns all-null for an empty search string', () => {
    expect(readFromSearch('')).toEqual(state());
  });

  it('parses ISO dates and normalises them to UTC ISO form', () => {
    const s = '?from=2026-04-01T00:00Z&to=2026-04-22T00:00:00Z';
    const parsed = readFromSearch(s);
    expect(parsed.from).toBe('2026-04-01T00:00:00.000Z');
    expect(parsed.to).toBe('2026-04-22T00:00:00.000Z');
  });

  it('drops invalid dates rather than surfacing NaN', () => {
    expect(readFromSearch('?from=not-a-date').from).toBeNull();
  });

  it('validates resolution and viewingMode against their enums', () => {
    const good = readFromSearch('?resolution=hourly&viewingMode=episteme');
    expect(good.resolution).toBe('hourly');
    expect(good.viewingMode).toBe('episteme');

    const bad = readFromSearch('?resolution=yearly&viewingMode=ghosts');
    expect(bad.resolution).toBeNull();
    expect(bad.viewingMode).toBeNull();
  });

  it('validates view against the Rhizome sub-view enum (Phase 122h)', () => {
    expect(readFromSearch('?view=actors-topics').view).toBe('actors-topics');
    expect(readFromSearch('?view=source-resonance').view).toBe('source-resonance');
    expect(readFromSearch('?view=concept-migration').view).toBe('concept-migration');
    expect(readFromSearch('?view=free-composition').view).toBe('free-composition');
    expect(readFromSearch('?view=analysis').view).toBeNull();
    expect(readFromSearch('?view=sideways').view).toBeNull();
  });

  it('accepts identifier-shaped metric names and rejects garbage', () => {
    expect(readFromSearch('?metric=sentiment_score').metric).toBe('sentiment_score');
    expect(readFromSearch('?metric=word_count').metric).toBe('word_count');
    expect(readFromSearch('?metric=has%20space').metric).toBeNull();
    expect(readFromSearch('?metric=drop%3Btable').metric).toBeNull();
    expect(readFromSearch(`?metric=${'a'.repeat(200)}`).metric).toBeNull();
  });

  it('validates viewMode against its enum', () => {
    expect(readFromSearch('?viewMode=time_series').viewMode).toBe('time_series');
    expect(readFromSearch('?viewMode=distribution').viewMode).toBe('distribution');
    expect(readFromSearch('?viewMode=cooccurrence_network').viewMode).toBe('cooccurrence_network');
    expect(readFromSearch('?viewMode=scatter').viewMode).toBeNull();
  });

  it('parses layer=silver and layer=gold correctly', () => {
    expect(readFromSearch('?layer=silver').layer).toBe('silver');
    expect(readFromSearch('?layer=gold').layer).toBe('gold');
    expect(readFromSearch('?layer=bronze').layer).toBeNull();
    expect(readFromSearch('').layer).toBeNull();
  });

  it('parses negSpace=1 as true and absent/other values as null', () => {
    expect(readFromSearch('?negSpace=1').negSpace).toBe(true);
    expect(readFromSearch('?negSpace=0').negSpace).toBeNull();
    expect(readFromSearch('?negSpace=true').negSpace).toBeNull();
    expect(readFromSearch('').negSpace).toBeNull();
  });

  it('parses comma-separated probeId param into probeIds array', () => {
    expect(readFromSearch('?probeId=probe-0-de-institutional-web').probeIds).toEqual([
      'probe-0-de-institutional-web'
    ]);
    expect(
      readFromSearch('?probeId=probe-0-de-institutional-web,probe-1-de-public-rss').probeIds
    ).toEqual(['probe-0-de-institutional-web', 'probe-1-de-public-rss']);
    expect(readFromSearch('').probeIds).toEqual([]);
  });
});

describe('writeToSearch', () => {
  it('omits null fields entirely', () => {
    expect(writeToSearch(state())).toBe('');
  });

  it('emits only populated fields', () => {
    const qs = writeToSearch(
      state({
        from: '2026-04-01T00:00:00.000Z',
        to: '2026-04-22T00:00:00.000Z',
        resolution: 'hourly'
      })
    );
    expect(qs).toContain('from=2026-04-01');
    expect(qs).toContain('to=2026-04-22');
    expect(qs).toContain('resolution=hourly');
    expect(qs).not.toContain('viewingMode');
    expect(qs).not.toContain('layer=');
    expect(qs).not.toContain('negSpace=');
  });

  it('round-trips through readFromSearch', () => {
    const original = state({
      from: '2026-04-01T00:00:00.000Z',
      to: '2026-04-22T00:00:00.000Z',
      resolution: 'daily',
      viewingMode: 'rhizome',
      metric: 'sentiment_score',
      view: 'actors-topics',
      viewMode: 'distribution'
    });
    expect(readFromSearch(writeToSearch(original))).toEqual(original);
  });

  it('emits metric when set', () => {
    expect(writeToSearch(state({ metric: 'sentiment_score' }))).toContain('metric=sentiment_score');
  });

  it('emits view when a Rhizome sub-view is set (Phase 122h)', () => {
    expect(writeToSearch(state({ viewingMode: 'rhizome', view: 'source-resonance' }))).toContain(
      'view=source-resonance'
    );
  });

  it('emits viewMode when set', () => {
    expect(writeToSearch(state({ viewMode: 'distribution' }))).toContain('viewMode=distribution');
  });

  it('emits layer=silver when set, omits for gold', () => {
    expect(writeToSearch(state({ layer: 'silver' }))).toContain('layer=silver');
    expect(writeToSearch(state({ layer: 'gold' }))).not.toContain('layer=');
  });

  it('round-trips layer=silver through readFromSearch', () => {
    const original = state({ sourceIds: ['tagesschau'], layer: 'silver' });
    expect(readFromSearch(writeToSearch(original))).toEqual(original);
  });

  it('emits negSpace=1 when true, omits when null', () => {
    expect(writeToSearch(state({ negSpace: true }))).toContain('negSpace=1');
    expect(writeToSearch(state())).not.toContain('negSpace=');
  });

  it('round-trips negSpace=true through readFromSearch', () => {
    const original = state({ negSpace: true });
    expect(readFromSearch(writeToSearch(original))).toEqual(original);
  });

  it('normalization roundtrips zscore and percentile, omits raw (Phase 115)', () => {
    expect(readFromSearch(writeToSearch(state({ normalization: 'zscore' }))).normalization).toBe(
      'zscore'
    );
    expect(
      readFromSearch(writeToSearch(state({ normalization: 'percentile' }))).normalization
    ).toBe('percentile');
    expect(writeToSearch(state({ normalization: 'raw' }))).not.toContain('normalization');
  });

  it('emits probeId param for multi-probe composition and omits when empty', () => {
    expect(
      writeToSearch(state({ probeIds: ['probe-0-de-institutional-web', 'probe-1-de-public-rss'] }))
    ).toContain('probeId=probe-0-de-institutional-web%2Cprobe-1-de-public-rss');
    expect(writeToSearch(state())).not.toContain('probeId=');
  });

  it('round-trips probeIds through readFromSearch', () => {
    const original = state({
      probeIds: ['probe-0-de-institutional-web', 'probe-1-de-public-rss']
    });
    expect(readFromSearch(writeToSearch(original))).toEqual(original);
  });
});
