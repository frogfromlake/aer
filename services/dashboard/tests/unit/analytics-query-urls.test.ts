import { describe, expect, it } from 'vitest';

import {
  correlationLeadLagQuery,
  correlationMatrixQuery,
  crossTabQuery,
  entityCoOccurrenceQuery,
  entityCoOccurrenceQueryMulti,
  metricScatterQuery,
  parallelCoordsQuery,
  sankeyQuery,
  silverAggregationQuery,
  topicDistributionQuery,
  type FetchContext
} from '../../src/lib/api/queries';

// Phase 142 — branch coverage for the analytics query factories. Each factory
// has a column of `if (params.X) qs.set(...)` optional-param branches; the
// existing queries.test.ts exercises the happy path with minimal params, so
// these tests drive the OPTIONAL branches (all set, then absent) by capturing
// the request URL/body through a mock fetch and asserting the wire shape.

interface Captured {
  url: string;
  init?: RequestInit | undefined;
}

function capture(): { ctx: FetchContext; last: () => Captured } {
  let last: Captured = { url: '' };
  const fetchFn = (async (url: RequestInfo | URL, init?: RequestInit) => {
    last = { url: String(url), init };
    return new Response('{}', { status: 200, headers: { 'Content-Type': 'application/json' } });
  }) as typeof fetch;
  return { ctx: { baseUrl: '/api/v1', fetch: fetchFn }, last: () => last };
}

async function run(opt: { queryFn: () => Promise<unknown> }, c: ReturnType<typeof capture>) {
  await opt.queryFn();
  return new URL(c.last().url, 'http://x');
}

const baseParams = { scope: 'probe' as const, scopeId: 'probe-0' };
const windowed = { ...baseParams, start: '2026-01-01T00:00:00Z', end: '2026-02-01T00:00:00Z' };
const facet = { metadataFilter: { field: 'section', value: 'Politik' } };

describe('metricScatterQuery URL shape', () => {
  it('sets every optional channel + the metadata facet when present', async () => {
    const c = capture();
    const u = await run(
      metricScatterQuery(c.ctx, {
        ...windowed,
        ...facet,
        xMetric: 'sentiment_score',
        yMetric: 'word_count',
        sizeMetric: 'entity_count',
        colorMetric: 'language_confidence',
        maxPoints: 5000
      }),
      c
    );
    expect(u.pathname).toBe('/api/v1/metrics/scatter');
    expect(u.searchParams.get('start')).toBe('2026-01-01T00:00:00Z');
    expect(u.searchParams.get('sizeMetric')).toBe('entity_count');
    expect(u.searchParams.get('colorMetric')).toBe('language_confidence');
    expect(u.searchParams.get('maxPoints')).toBe('5000');
    expect(u.searchParams.get('metadataFilterField')).toBe('section');
  });

  it('omits every optional when absent', async () => {
    const c = capture();
    const u = await run(
      metricScatterQuery(c.ctx, { ...baseParams, xMetric: 'a', yMetric: 'b' }),
      c
    );
    expect(u.searchParams.has('start')).toBe(false);
    expect(u.searchParams.has('sizeMetric')).toBe(false);
    expect(u.searchParams.has('maxPoints')).toBe(false);
    expect(u.searchParams.has('metadataFilterField')).toBe(false);
  });
});

describe('correlationMatrixQuery / parallelCoordsQuery', () => {
  it('joins the metric set and carries the window + maxPoints', async () => {
    const c = capture();
    const u = await run(
      correlationMatrixQuery(c.ctx, { ...windowed, metrics: ['a', 'b', 'c'] }),
      c
    );
    expect(u.pathname).toBe('/api/v1/metrics/correlation');
    expect(u.searchParams.get('metrics')).toBe('a,b,c');
    expect(u.searchParams.get('end')).toBe('2026-02-01T00:00:00Z');

    const c2 = capture();
    const u2 = await run(
      parallelCoordsQuery(c2.ctx, { ...windowed, ...facet, metrics: ['x', 'y'], maxPoints: 2000 }),
      c2
    );
    expect(u2.pathname).toBe('/api/v1/metrics/parallel');
    expect(u2.searchParams.get('maxPoints')).toBe('2000');
    expect(u2.searchParams.get('metadataFilterValue')).toBe('Politik');
  });
});

describe('crossTabQuery', () => {
  it('URL-encodes the field/metric path segments and carries topN', async () => {
    const c = capture();
    const u = await run(
      crossTabQuery(c.ctx, 'sub section', 'sentiment/raw', { ...windowed, topN: 25 }),
      c
    );
    expect(u.pathname).toBe('/api/v1/metadata/sub%20section/by-metric/sentiment%2Fraw');
    expect(u.searchParams.get('topN')).toBe('25');
  });

  it('omits topN when absent', async () => {
    const c = capture();
    const u = await run(crossTabQuery(c.ctx, 'section', 'wc', baseParams), c);
    expect(u.searchParams.has('topN')).toBe(false);
  });
});

describe('correlationLeadLagQuery', () => {
  it('carries both metrics + maxLagHours', async () => {
    const c = capture();
    const u = await run(
      correlationLeadLagQuery(c.ctx, { ...windowed, xMetric: 'a', yMetric: 'b', maxLagHours: 72 }),
      c
    );
    expect(u.pathname).toBe('/api/v1/correlation/lead-lag');
    expect(u.searchParams.get('maxLagHours')).toBe('72');
  });
});

describe('sankeyQuery', () => {
  it('joins the field chain + topN', async () => {
    const c = capture();
    const u = await run(sankeyQuery(c.ctx, ['author', 'section'], { ...windowed, topN: 8 }), c);
    expect(u.pathname).toBe('/api/v1/metadata/sankey');
    expect(u.searchParams.get('fields')).toBe('author,section');
    expect(u.searchParams.get('topN')).toBe('8');
  });
});

describe('entityCoOccurrenceQuery (GET) optional channels', () => {
  it('sets every optional channel including a distinct colour metric + ghost overlay', async () => {
    const c = capture();
    const u = await run(
      entityCoOccurrenceQuery(c.ctx, {
        ...windowed,
        topN: 200,
        viewerLanguage: 'en',
        nodeMetric: 'sentiment_score',
        nodeColorMetric: 'word_count',
        minWeight: 3,
        negativeSpaceOverlay: 'ghost'
      }),
      c
    );
    expect(u.pathname).toBe('/api/v1/entities/cooccurrence');
    expect(u.searchParams.get('topN')).toBe('200');
    expect(u.searchParams.get('viewerLanguage')).toBe('en');
    expect(u.searchParams.get('nodeMetric')).toBe('sentiment_score');
    expect(u.searchParams.get('nodeColorMetric')).toBe('word_count');
    expect(u.searchParams.get('minWeight')).toBe('3');
    expect(u.searchParams.get('negativeSpaceOverlay')).toBe('ghost');
  });

  it('omits nodeColorMetric when it equals the size metric, and minWeight when 0', async () => {
    const c = capture();
    const u = await run(
      entityCoOccurrenceQuery(c.ctx, {
        ...baseParams,
        nodeMetric: 'sentiment_score',
        nodeColorMetric: 'sentiment_score', // same → omitted
        minWeight: 0 // → omitted
      }),
      c
    );
    expect(u.searchParams.has('nodeColorMetric')).toBe(false);
    expect(u.searchParams.has('minWeight')).toBe(false);
  });
});

describe('entityCoOccurrenceQueryMulti (POST)', () => {
  it('POSTs the union scope groups + window + topN + viewerLanguage', async () => {
    const c = capture();
    await entityCoOccurrenceQueryMulti(c.ctx, {
      scopes: [{ probeIds: ['probe-0'], sourceIds: ['tagesschau'] }],
      start: '2026-01-01T00:00:00Z',
      end: '2026-02-01T00:00:00Z',
      topN: 100,
      viewerLanguage: 'en'
    }).queryFn();
    const init = c.last().init!;
    expect(c.last().url).toBe('/api/v1/entities/cooccurrence/query');
    expect(init.method).toBe('POST');
    const body = JSON.parse(init.body as string);
    expect(body.scopes).toEqual([{ probeIds: ['probe-0'], sourceIds: ['tagesschau'] }]);
    expect(body.windowStart).toBe('2026-01-01T00:00:00Z');
    expect(body.topN).toBe(100);
    expect(body.viewerLanguage).toBe('en');
  });

  it('omits topN + viewerLanguage from the body when absent', async () => {
    const c = capture();
    await entityCoOccurrenceQueryMulti(c.ctx, {
      scopes: [{ probeIds: ['p'], sourceIds: [] }]
    }).queryFn();
    const body = JSON.parse(c.last().init!.body as string);
    expect('topN' in body).toBe(false);
    expect('viewerLanguage' in body).toBe(false);
  });
});

describe('topicDistributionQuery', () => {
  it('sets every optional incl. probe/source id lists, language, minConfidence, includeOutlier', async () => {
    const c = capture();
    const u = await run(
      topicDistributionQuery(c.ctx, {
        scope: 'probe',
        scopeId: 'probe-0',
        probeIds: ['probe-0', 'probe-1'],
        sourceIds: ['tagesschau'],
        start: '2026-01-01T00:00:00Z',
        end: '2026-02-01T00:00:00Z',
        language: 'de',
        minConfidence: 0.3,
        includeOutlier: true
      }),
      c
    );
    expect(u.pathname).toBe('/api/v1/topics/distribution');
    expect(u.searchParams.get('probeIds')).toBe('probe-0,probe-1');
    expect(u.searchParams.get('sourceIds')).toBe('tagesschau');
    expect(u.searchParams.get('language')).toBe('de');
    expect(u.searchParams.get('minConfidence')).toBe('0.3');
    expect(u.searchParams.get('includeOutlier')).toBe('true');
  });

  it('omits the id lists when empty and includeOutlier when false', async () => {
    const c = capture();
    const u = await run(
      topicDistributionQuery(c.ctx, {
        scopeId: 'probe-0',
        probeIds: [],
        sourceIds: [],
        minConfidence: 0, // 0 is defined → still set
        includeOutlier: false
      }),
      c
    );
    expect(u.searchParams.has('probeIds')).toBe(false);
    expect(u.searchParams.has('sourceIds')).toBe(false);
    expect(u.searchParams.get('minConfidence')).toBe('0');
    expect(u.searchParams.has('includeOutlier')).toBe(false);
  });
});

describe('silverAggregationQuery', () => {
  it('encodes the aggregation type and carries bins + window', async () => {
    const c = capture();
    const u = await run(
      silverAggregationQuery(c.ctx, 'word_count_by_source', {
        sourceId: 'tagesschau',
        start: '2026-01-01T00:00:00Z',
        end: '2026-02-01T00:00:00Z',
        bins: 40
      }),
      c
    );
    expect(u.pathname).toBe('/api/v1/silver/aggregations/word_count_by_source');
    expect(u.searchParams.get('sourceId')).toBe('tagesschau');
    expect(u.searchParams.get('bins')).toBe('40');
  });

  it('omits bins + window when absent', async () => {
    const c = capture();
    const u = await run(silverAggregationQuery(c.ctx, 'word_count', { sourceId: 's' }), c);
    expect(u.searchParams.has('bins')).toBe(false);
    expect(u.searchParams.has('start')).toBe(false);
  });
});
