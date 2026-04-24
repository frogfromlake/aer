import { describe, expect, it } from 'vitest';

import { readFromSearch, writeToSearch } from '../../src/lib/state/url-internals';

describe('readFromSearch', () => {
  it('returns all-null for an empty search string', () => {
    expect(readFromSearch('')).toEqual({
      from: null,
      to: null,
      probe: null,
      emissionPoint: null,
      resolution: null,
      viewingMode: null,
      metric: null,
      view: null
    });
  });

  it('parses ISO dates and normalises them to UTC ISO form', () => {
    const s = '?from=2026-04-01T00:00Z&to=2026-04-22T00:00:00Z';
    const state = readFromSearch(s);
    expect(state.from).toBe('2026-04-01T00:00:00.000Z');
    expect(state.to).toBe('2026-04-22T00:00:00.000Z');
  });

  it('drops invalid dates rather than surfacing NaN', () => {
    const state = readFromSearch('?from=not-a-date');
    expect(state.from).toBeNull();
  });

  it('validates resolution and viewingMode against their enums', () => {
    const good = readFromSearch('?resolution=hourly&viewingMode=episteme');
    expect(good.resolution).toBe('hourly');
    expect(good.viewingMode).toBe('episteme');

    const bad = readFromSearch('?resolution=yearly&viewingMode=ghosts');
    expect(bad.resolution).toBeNull();
    expect(bad.viewingMode).toBeNull();
  });

  it('passes probe through unmodified', () => {
    const state = readFromSearch('?probe=probe-0-de-institutional-rss');
    expect(state.probe).toBe('probe-0-de-institutional-rss');
  });

  it('validates view against the descent-layer enum', () => {
    expect(readFromSearch('?view=analysis').view).toBe('analysis');
    expect(readFromSearch('?view=atmosphere').view).toBe('atmosphere');
    expect(readFromSearch('?view=sideways').view).toBeNull();
  });

  it('accepts identifier-shaped metric names and rejects garbage', () => {
    expect(readFromSearch('?metric=sentiment_score').metric).toBe('sentiment_score');
    expect(readFromSearch('?metric=word_count').metric).toBe('word_count');
    expect(readFromSearch('?metric=has%20space').metric).toBeNull();
    expect(readFromSearch('?metric=drop%3Btable').metric).toBeNull();
    expect(readFromSearch(`?metric=${'a'.repeat(200)}`).metric).toBeNull();
  });

  it('parses ep as a non-negative integer and rejects garbage', () => {
    expect(readFromSearch('?probe=p&ep=0').emissionPoint).toBe(0);
    expect(readFromSearch('?probe=p&ep=2').emissionPoint).toBe(2);
    expect(readFromSearch('?probe=p&ep=-1').emissionPoint).toBeNull();
    expect(readFromSearch('?probe=p&ep=1.5').emissionPoint).toBeNull();
    expect(readFromSearch('?probe=p&ep=abc').emissionPoint).toBeNull();
  });
});

describe('writeToSearch', () => {
  it('omits null fields entirely', () => {
    expect(
      writeToSearch({
        from: null,
        to: null,
        probe: null,
        emissionPoint: null,
        resolution: null,
        viewingMode: null,
        metric: null,
        view: null
      })
    ).toBe('');
  });

  it('emits only populated fields', () => {
    const qs = writeToSearch({
      from: '2026-04-01T00:00:00.000Z',
      to: '2026-04-22T00:00:00.000Z',
      probe: 'probe-0',
      emissionPoint: null,
      resolution: 'hourly',
      viewingMode: null,
      metric: null,
      view: null
    });
    expect(qs).toContain('from=2026-04-01');
    expect(qs).toContain('to=2026-04-22');
    expect(qs).toContain('probe=probe-0');
    expect(qs).toContain('resolution=hourly');
    expect(qs).not.toContain('viewingMode');
    expect(qs).not.toContain('ep=');
  });

  it('emits ep alongside probe and drops it without a probe', () => {
    const withProbe = writeToSearch({
      from: null,
      to: null,
      probe: 'probe-0',
      emissionPoint: 1,
      resolution: null,
      viewingMode: null,
      metric: null,
      view: null
    });
    expect(withProbe).toContain('probe=probe-0');
    expect(withProbe).toContain('ep=1');

    const orphan = writeToSearch({
      from: null,
      to: null,
      probe: null,
      emissionPoint: 1,
      resolution: null,
      viewingMode: null,
      metric: null,
      view: null
    });
    expect(orphan).not.toContain('ep=');
  });

  it('round-trips through readFromSearch', () => {
    const original = {
      from: '2026-04-01T00:00:00.000Z',
      to: '2026-04-22T00:00:00.000Z',
      probe: 'probe-0-de-institutional-rss',
      emissionPoint: 1,
      resolution: 'daily' as const,
      viewingMode: 'aleph' as const,
      metric: 'sentiment_score',
      view: 'analysis' as const
    };
    const qs = writeToSearch(original);
    expect(readFromSearch(qs)).toEqual(original);
  });

  it('drops metric when not in the analysis view', () => {
    const qs = writeToSearch({
      from: null,
      to: null,
      probe: 'probe-0',
      emissionPoint: null,
      resolution: null,
      viewingMode: null,
      metric: 'sentiment_score',
      view: null
    });
    expect(qs).not.toContain('metric=');
  });

  it('omits view when it is the default atmosphere layer', () => {
    const qs = writeToSearch({
      from: null,
      to: null,
      probe: null,
      emissionPoint: null,
      resolution: null,
      viewingMode: null,
      metric: null,
      view: 'atmosphere'
    });
    expect(qs).not.toContain('view=');
  });
});
