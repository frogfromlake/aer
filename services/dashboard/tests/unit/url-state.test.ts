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
      view: null,
      sourceIds: [],
      probeIds: [],
      viewMode: null,
      layer: null,
      negSpace: null,
      normalization: null
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
    expect(readFromSearch('?probeId=probe-0-de-institutional-rss').probeIds).toEqual([
      'probe-0-de-institutional-rss'
    ]);
    expect(
      readFromSearch('?probeId=probe-0-de-institutional-rss,probe-1-de-public-rss').probeIds
    ).toEqual(['probe-0-de-institutional-rss', 'probe-1-de-public-rss']);
    expect(readFromSearch('').probeIds).toEqual([]);
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
        view: null,
        sourceIds: [],
        probeIds: [],
        viewMode: null,
        layer: null,
        negSpace: null,
        normalization: null
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
      view: null,
      sourceIds: [],
      probeIds: [],
      viewMode: null,
      layer: null,
      negSpace: null,
      normalization: null
    });
    expect(qs).toContain('from=2026-04-01');
    expect(qs).toContain('to=2026-04-22');
    expect(qs).toContain('probe=probe-0');
    expect(qs).toContain('resolution=hourly');
    expect(qs).not.toContain('viewingMode');
    expect(qs).not.toContain('ep=');
    expect(qs).not.toContain('layer=');
    expect(qs).not.toContain('negSpace=');
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
      view: null,
      sourceIds: [],
      probeIds: [],
      viewMode: null,
      layer: null,
      negSpace: null,
      normalization: null
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
      view: null,
      sourceIds: [],
      probeIds: [],
      viewMode: null,
      layer: null,
      negSpace: null,
      normalization: null
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
      view: 'analysis' as const,
      sourceIds: [],
      probeIds: [],
      viewMode: 'distribution' as const,
      layer: null,
      negSpace: null,
      normalization: null
    };
    const qs = writeToSearch(original);
    expect(readFromSearch(qs)).toEqual(original);
  });

  it('emits metric regardless of view (no analysis-view gate)', () => {
    const qs = writeToSearch({
      from: null,
      to: null,
      probe: 'probe-0',
      emissionPoint: null,
      resolution: null,
      viewingMode: null,
      metric: 'sentiment_score',
      view: null,
      sourceIds: [],
      probeIds: [],
      viewMode: null,
      layer: null,
      negSpace: null,
      normalization: null
    });
    expect(qs).toContain('metric=sentiment_score');
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
      view: 'atmosphere',
      sourceIds: [],
      probeIds: [],
      viewMode: null,
      layer: null,
      negSpace: null,
      normalization: null
    });
    expect(qs).not.toContain('view=');
  });

  it('emits viewMode regardless of probe (no probe gate)', () => {
    const qs = writeToSearch({
      from: null,
      to: null,
      probe: null,
      emissionPoint: null,
      resolution: null,
      viewingMode: null,
      metric: null,
      view: null,
      sourceIds: [],
      probeIds: [],
      viewMode: 'distribution',
      layer: null,
      negSpace: null,
      normalization: null
    });
    expect(qs).toContain('viewMode=distribution');
  });

  it('emits layer=silver when set, omits for gold — no probe gate', () => {
    const withSilver = writeToSearch({
      from: null,
      to: null,
      probe: 'probe-0',
      emissionPoint: null,
      resolution: null,
      viewingMode: null,
      metric: null,
      view: null,
      sourceIds: [],
      probeIds: [],
      viewMode: null,
      layer: 'silver',
      negSpace: null,
      normalization: null
    });
    expect(withSilver).toContain('layer=silver');

    const withGold = writeToSearch({
      from: null,
      to: null,
      probe: 'probe-0',
      emissionPoint: null,
      resolution: null,
      viewingMode: null,
      metric: null,
      view: null,
      sourceIds: [],
      probeIds: [],
      viewMode: null,
      layer: 'gold',
      negSpace: null,
      normalization: null
    });
    expect(withGold).not.toContain('layer=');

    const noProbe = writeToSearch({
      from: null,
      to: null,
      probe: null,
      emissionPoint: null,
      resolution: null,
      viewingMode: null,
      metric: null,
      view: null,
      sourceIds: [],
      probeIds: [],
      viewMode: null,
      layer: 'silver',
      negSpace: null,
      normalization: null
    });
    expect(noProbe).toContain('layer=silver');
  });

  it('round-trips layer=silver through readFromSearch', () => {
    const original = {
      from: null,
      to: null,
      probe: 'probe-0-de-institutional-rss',
      emissionPoint: null,
      resolution: null,
      viewingMode: null,
      metric: null,
      view: null,
      sourceIds: ['tagesschau'],
      probeIds: [],
      viewMode: null,
      layer: 'silver' as const,
      negSpace: null,
      normalization: null
    };
    const qs = writeToSearch(original);
    expect(readFromSearch(qs)).toEqual(original);
  });

  it('emits negSpace=1 when true, omits when null', () => {
    const withNegSpace = writeToSearch({
      from: null,
      to: null,
      probe: null,
      emissionPoint: null,
      resolution: null,
      viewingMode: null,
      metric: null,
      view: null,
      sourceIds: [],
      probeIds: [],
      viewMode: null,
      layer: null,
      negSpace: true,
      normalization: null
    });
    expect(withNegSpace).toContain('negSpace=1');

    const withoutNegSpace = writeToSearch({
      from: null,
      to: null,
      probe: null,
      emissionPoint: null,
      resolution: null,
      viewingMode: null,
      metric: null,
      view: null,
      sourceIds: [],
      probeIds: [],
      viewMode: null,
      layer: null,
      negSpace: null,
      normalization: null
    });
    expect(withoutNegSpace).not.toContain('negSpace=');
  });

  it('round-trips negSpace=true through readFromSearch', () => {
    const original = {
      from: null,
      to: null,
      probe: 'probe-0-de-institutional-rss',
      emissionPoint: null,
      resolution: null,
      viewingMode: null,
      metric: null,
      view: null,
      sourceIds: [],
      probeIds: [],
      viewMode: null,
      layer: null,
      negSpace: true as const,
      normalization: null
    };
    const qs = writeToSearch(original);
    expect(readFromSearch(qs)).toEqual(original);
  });

  it('normalization roundtrips zscore and percentile, omits raw (Phase 115)', () => {
    const zRound = readFromSearch(
      writeToSearch({
        from: null,
        to: null,
        probe: null,
        emissionPoint: null,
        resolution: null,
        viewingMode: null,
        metric: null,
        view: null,
        sourceIds: [],
        probeIds: [],
        viewMode: null,
        layer: null,
        negSpace: null,
        normalization: 'zscore'
      })
    );
    expect(zRound.normalization).toBe('zscore');

    const pRound = readFromSearch(
      writeToSearch({
        from: null,
        to: null,
        probe: null,
        emissionPoint: null,
        resolution: null,
        viewingMode: null,
        metric: null,
        view: null,
        sourceIds: [],
        probeIds: [],
        viewMode: null,
        layer: null,
        negSpace: null,
        normalization: 'percentile'
      })
    );
    expect(pRound.normalization).toBe('percentile');

    const rawRound = writeToSearch({
      from: null,
      to: null,
      probe: null,
      emissionPoint: null,
      resolution: null,
      viewingMode: null,
      metric: null,
      view: null,
      sourceIds: [],
      probeIds: [],
      viewMode: null,
      layer: null,
      negSpace: null,
      normalization: 'raw'
    });
    expect(rawRound).not.toContain('normalization');
  });

  it('negSpace is not scoped to a probe — emits independently', () => {
    const qs = writeToSearch({
      from: null,
      to: null,
      probe: null,
      emissionPoint: null,
      resolution: null,
      viewingMode: null,
      metric: null,
      view: null,
      sourceIds: [],
      probeIds: [],
      viewMode: null,
      layer: null,
      negSpace: true,
      normalization: null
    });
    expect(qs).toContain('negSpace=1');
    expect(qs).not.toContain('probe=');
  });

  it('emits probeId param for multi-probe composition and omits when empty', () => {
    const withProbes = writeToSearch({
      from: null,
      to: null,
      probe: null,
      emissionPoint: null,
      resolution: null,
      viewingMode: null,
      metric: null,
      view: null,
      sourceIds: [],
      probeIds: ['probe-0-de-institutional-rss', 'probe-1-de-public-rss'],
      viewMode: null,
      layer: null,
      negSpace: null,
      normalization: null
    });
    expect(withProbes).toContain('probeId=probe-0-de-institutional-rss%2Cprobe-1-de-public-rss');

    const noProbes = writeToSearch({
      from: null,
      to: null,
      probe: null,
      emissionPoint: null,
      resolution: null,
      viewingMode: null,
      metric: null,
      view: null,
      sourceIds: [],
      probeIds: [],
      viewMode: null,
      layer: null,
      negSpace: null,
      normalization: null
    });
    expect(noProbes).not.toContain('probeId=');
  });

  it('round-trips probeIds through readFromSearch', () => {
    const original = {
      from: null,
      to: null,
      probe: null,
      emissionPoint: null,
      resolution: null,
      viewingMode: null,
      metric: null,
      view: null,
      sourceIds: [],
      probeIds: ['probe-0-de-institutional-rss', 'probe-1-de-public-rss'],
      viewMode: null,
      layer: null,
      negSpace: null,
      normalization: null
    };
    const qs = writeToSearch(original);
    expect(readFromSearch(qs)).toEqual(original);
  });
});
