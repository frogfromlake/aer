import { describe, it, expect } from 'vitest';
import {
  buildProbeMarkers,
  computeWindow,
  computeActivity,
  resolveFlyTo,
  buildFlatProbes
} from '../../src/lib/components/atmosphere/atmosphere-surface-internals';
import { DEFAULT_LOOKBACK_MS } from '../../src/lib/state/url-internals';
import type { MetricsResponseDto, ProbeDto } from '../../src/lib/api/queries';

const probe = (over: Partial<ProbeDto> & Pick<ProbeDto, 'probeId'>): ProbeDto =>
  ({
    language: 'de',
    shortName: 'Short',
    displayName: 'Display Name',
    sources: [],
    emissionPoints: [],
    ...over
  }) as unknown as ProbeDto;

const ep = (latitude: number, longitude: number, label = 'pt') => ({ latitude, longitude, label });

describe('buildProbeMarkers', () => {
  it('maps probeId→id, shortName→label, and aligns sourceName positionally', () => {
    const markers = buildProbeMarkers([
      probe({
        probeId: 'p1',
        language: 'fr',
        shortName: 'FR-Inst',
        sources: ['s1', 's2'],
        emissionPoints: [ep(1, 2, 'a'), ep(3, 4, 'b'), ep(5, 6, 'c')]
      })
    ]);
    expect(markers[0]).toMatchObject({ id: 'p1', label: 'FR-Inst', language: 'fr' });
    expect(markers[0]!.emissionPoints[0]).toEqual({
      latitude: 1,
      longitude: 2,
      label: 'a',
      sourceName: 's1'
    });
    // Third emission point has no aligned source → no sourceName key (no satellite).
    expect(markers[0]!.emissionPoints[2]).toEqual({ latitude: 5, longitude: 6, label: 'c' });
    expect('sourceName' in markers[0]!.emissionPoints[2]!).toBe(false);
  });
});

describe('computeWindow', () => {
  const NOW = new Date('2026-06-15T12:00:00.000Z').getTime();

  it('passes through explicit bounds and computes the hour span', () => {
    const w = computeWindow(
      '2026-01-01T00:00:00.000Z',
      '2026-01-02T00:00:00.000Z',
      NOW,
      DEFAULT_LOOKBACK_MS
    );
    expect(w).toEqual({
      start: '2026-01-01T00:00:00.000Z',
      end: '2026-01-02T00:00:00.000Z',
      hours: 24
    });
  });

  it('falls back to the default lookback for a missing/invalid from, and now for a missing to', () => {
    const w = computeWindow(null, null, NOW, DEFAULT_LOOKBACK_MS);
    expect(w.start).toBe(new Date(NOW - DEFAULT_LOOKBACK_MS).toISOString());
    expect(w.end).toBe(new Date(NOW).toISOString());
    // An unparseable from is treated the same as missing.
    expect(computeWindow('not-a-date', null, NOW, DEFAULT_LOOKBACK_MS).start).toBe(
      new Date(NOW - DEFAULT_LOOKBACK_MS).toISOString()
    );
  });

  it('floors the hour span to at least 1 (no divide-by-zero downstream)', () => {
    const w = computeWindow(
      '2026-01-01T00:00:00.000Z',
      '2026-01-01T00:00:00.000Z',
      NOW,
      DEFAULT_LOOKBACK_MS
    );
    expect(w.hours).toBe(1);
  });
});

describe('computeActivity', () => {
  const rows = [
    { source: 's1', count: 10 },
    { source: 's1', count: 5 },
    { source: 's2', count: 3 },
    { source: 's4' } // missing count → treated as 0
  ] as unknown as MetricsResponseDto['data'];

  it('sums per-source counts then per-probe, divided by the window hours', () => {
    const out = computeActivity(
      rows,
      [probe({ probeId: 'p1', sources: ['s1', 's2'] }), probe({ probeId: 'p2', sources: ['s3'] })],
      5
    );
    expect(out).toEqual([
      { probeId: 'p1', documentsPerHour: 18 / 5 }, // (10+5)+3 = 18
      { probeId: 'p2', documentsPerHour: 0 } // s3 absent → 0
    ]);
  });
});

describe('resolveFlyTo', () => {
  const dtos = [probe({ probeId: 'p1', emissionPoints: [ep(48.85, 2.35), ep(0, 0)] })];

  it('returns the first emission point of the active probe', () => {
    expect(resolveFlyTo(dtos, 'p1')).toEqual({ latitude: 48.85, longitude: 2.35 });
  });

  it('returns null when there is no active probe, no match, or no emission points', () => {
    expect(resolveFlyTo(dtos, null)).toBeNull();
    expect(resolveFlyTo(dtos, 'pX')).toBeNull();
    expect(resolveFlyTo([probe({ probeId: 'p2', emissionPoints: [] })], 'p2')).toBeNull();
  });
});

describe('buildFlatProbes', () => {
  it('projects probeId/displayName/language', () => {
    expect(
      buildFlatProbes([probe({ probeId: 'p1', displayName: 'France Info', language: 'fr' })])
    ).toEqual([{ probeId: 'p1', displayName: 'France Info', language: 'fr' }]);
  });
});
