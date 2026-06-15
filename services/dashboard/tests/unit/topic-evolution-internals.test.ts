import { describe, it, expect } from 'vitest';
import {
  computeBuckets,
  seriesId,
  buildSeriesMeta,
  selectTopSeriesIds,
  collectLanguages,
  buildPlotRows,
  buildLegendEntries,
  type Bucket,
  type SeriesMetaEntry
} from '../../src/lib/presentations/topic-evolution-internals';
import type { TopicDistributionResponseDto } from '../../src/lib/api/queries';

const entry = (topicId: number, label: string, articleCount: number, language = 'de') => ({
  topicId,
  label,
  articleCount,
  language
});
const payload = (topics: ReturnType<typeof entry>[]) =>
  ({ topics }) as unknown as TopicDistributionResponseDto;

const meta = (
  over: Partial<SeriesMetaEntry> & Pick<SeriesMetaEntry, 'topicId'>
): SeriesMetaEntry => ({
  language: 'de',
  label: 'x',
  isOutlier: false,
  totalArticles: 0,
  ...over
});

describe('computeBuckets', () => {
  const NOW = new Date('2026-06-15T00:00:00.000Z').getTime();

  it('slices a bounded window into ~1 bucket per 12h with exact ISO boundaries', () => {
    // 5 days = 10 × 12h → 10 buckets.
    const out = computeBuckets('2026-01-01T00:00:00.000Z', '2026-01-06T00:00:00.000Z', NOW);
    expect(out).toHaveLength(10);
    expect(out[0]!.start).toBe('2026-01-01T00:00:00.000Z');
    expect(out[9]!.end).toBe('2026-01-06T00:00:00.000Z');
    // First midpoint = +6h.
    expect(out[0]!.midpoint).toBe('2026-01-01T06:00:00.000Z');
  });

  it('clamps to MAX_BUCKETS (14) for very wide windows', () => {
    const out = computeBuckets('2025-01-01T00:00:00.000Z', '2026-01-01T00:00:00.000Z', NOW);
    expect(out).toHaveLength(14);
  });

  it('clamps to MIN_BUCKETS (3) for a 12h window (desired=1)', () => {
    expect(
      computeBuckets('2026-01-01T00:00:00.000Z', '2026-01-01T12:00:00.000Z', NOW)
    ).toHaveLength(3);
  });

  it('falls back to TARGET_BUCKETS (10) when desired rounds to 0 (sub-6h window)', () => {
    expect(
      computeBuckets('2026-01-01T00:00:00.000Z', '2026-01-01T01:00:00.000Z', NOW)
    ).toHaveLength(10);
  });

  it('falls back to the last 30 days when unbounded, ending at now', () => {
    const out = computeBuckets(undefined, undefined, NOW);
    expect(out).toHaveLength(14); // 30d → desired 60 → clamped 14
    expect(out[out.length - 1]!.end).toBe(new Date(NOW).toISOString());
  });

  it('returns [] for an inverted or zero-span window', () => {
    expect(computeBuckets('2026-01-06T00:00:00.000Z', '2026-01-01T00:00:00.000Z', NOW)).toEqual([]);
    expect(computeBuckets('2026-01-01T00:00:00.000Z', '2026-01-01T00:00:00.000Z', NOW)).toEqual([]);
  });
});

describe('seriesId', () => {
  it('keys by language::topicId', () => {
    expect(seriesId({ topicId: 7, language: 'fr', label: 'L', isOutlier: false })).toBe('fr::7');
  });
});

describe('buildSeriesMeta', () => {
  it('sums article counts across payloads, relabels outliers, defaults blank language to und', () => {
    const m = buildSeriesMeta([
      payload([entry(1, 'Climate', 10), entry(2, 'Economy', 5), entry(-1, '', 3)]),
      payload([entry(1, 'Climate', 4), entry(3, 'Sport', 2, '')])
    ]);
    expect(m['de::1']!.totalArticles).toBe(14);
    expect(m['de::-1']!.label).toBe('uncategorised');
    expect(m['de::-1']!.isOutlier).toBe(true);
    expect(m['und::3']!.totalArticles).toBe(2); // blank language → und
  });
});

describe('selectTopSeriesIds', () => {
  it('always keeps the outlier and the top-K named topics by volume', () => {
    const m: Record<string, SeriesMetaEntry> = {};
    for (let i = 1; i <= 9; i++) m[`de::${i}`] = meta({ topicId: i, totalArticles: i });
    m['de::-1'] = meta({ topicId: -1, totalArticles: 1, isOutlier: true, label: 'uncategorised' });
    const top = selectTopSeriesIds(m);
    expect(top['de::-1']).toBe(true); // outlier kept regardless of its low volume
    expect(top['de::9']).toBe(true); // highest named kept
    expect(top['de::1']).toBeUndefined(); // 9th-ranked named dropped (TOP_K=8)
    expect(Object.keys(top)).toHaveLength(9); // 8 named + outlier
  });
});

describe('collectLanguages', () => {
  it('lists distinct languages in first-seen order', () => {
    const m = buildSeriesMeta([
      payload([entry(1, 'A', 1, 'de'), entry(2, 'B', 1, 'fr'), entry(3, 'C', 1, 'de')])
    ]);
    expect(collectLanguages(m)).toEqual(['de', 'fr']);
  });
});

describe('buildPlotRows', () => {
  const top = { 'de::1': true as const, 'de::-1': true as const };
  const seriesMeta: Record<string, SeriesMetaEntry> = {
    'de::1': meta({ topicId: 1, label: 'Climate', totalArticles: 10 }),
    'de::-1': meta({ topicId: -1, label: 'uncategorised', isOutlier: true, totalArticles: 3 })
  };
  const b = (midpoint: string): Bucket => ({ start: '', end: '', midpoint });
  const aligned = [
    {
      bucket: b('2026-01-01T06:00:00.000Z'),
      payload: payload([entry(1, 'Climate', 6), entry(2, 'Economy', 4), entry(-1, '', 1)])
    },
    { bucket: b('2026-01-01T18:00:00.000Z'), payload: payload([entry(1, 'Climate', 2)]) }
  ];
  const rows = buildPlotRows(aligned, top, seriesMeta);

  it('emits real counts for top-K series with the bucket midpoint as a Date', () => {
    const r = rows.find((x) => x.series === 'de::1' && x.articleCount === 6)!;
    expect(r.bucket.toISOString()).toBe('2026-01-01T06:00:00.000Z');
    expect(r.topicId).toBe(1);
  });

  it('folds non-top-K topics into a per-language "other" stream', () => {
    const other = rows.find((x) => x.series === 'de::other')!;
    expect(other).toMatchObject({ articleCount: 4, topicId: -2, label: 'other topics' });
  });

  it('backfills explicit zeros for top-K series absent from a bucket', () => {
    const zero = rows.filter((x) => x.series === 'de::-1' && x.articleCount === 0);
    expect(zero).toHaveLength(1); // outlier absent in bucket 2 → one zero row
    expect(zero[0]!.bucket.toISOString()).toBe('2026-01-01T18:00:00.000Z');
  });
});

describe('buildLegendEntries', () => {
  it('orders named (Viridis) → other (slate) → outlier (grey) and matches the chart palette', () => {
    const m: Record<string, SeriesMetaEntry> = {
      'de::1': meta({ topicId: 1, label: 'Climate', totalArticles: 10 }),
      'de::2': meta({ topicId: 2, label: 'Economy', totalArticles: 5 }),
      'de::-1': meta({ topicId: -1, label: 'uncategorised', isOutlier: true, totalArticles: 3 })
    };
    const top = { 'de::1': true as const, 'de::2': true as const, 'de::-1': true as const };
    const out = buildLegendEntries(m, top, ['de']);
    expect(out.map((e) => e.id)).toEqual(['de::1', 'de::2', 'de::other', 'de::-1']);
    expect(out[0]!.colour).toBe('#440154'); // VIRIDIS[0]
    expect(out[1]!.colour).toBe('#482677'); // VIRIDIS[1]
    expect(out[2]!.colour).toBe('#5b6677'); // OTHER_COLOUR
    expect(out[3]!).toMatchObject({ colour: '#888888', isOutlier: true, label: 'uncategorised' });
  });
});
