import { describe, expect, it } from 'vitest';

import {
  correlationLeadLagQuery,
  correlationMatrixQuery,
  crossTabQuery,
  entityCoOccurrenceQueryMulti,
  metricScatterQuery,
  parallelCoordsQuery,
  sankeyQuery,
  topicDistributionQuery,
  type FetchContext
} from '../../src/lib/api/queries';

type FetchFn = typeof globalThis.fetch;

// Capturing fetch — records the requested URL + init so tests can assert the
// query functions build the right path/params, and returns a 200 JSON body so
// the queryFn resolves to a `success` outcome.
function capture(body: unknown = {}) {
  const calls: { url: string; init: RequestInit | undefined }[] = [];
  const fetch = (async (input: RequestInfo | URL, init?: RequestInit) => {
    calls.push({ url: String(input), init });
    return new Response(JSON.stringify(body), {
      status: 200,
      headers: { 'Content-Type': 'application/json' }
    });
  }) as unknown as FetchFn;
  return { calls, ctx: { baseUrl: '/api/v1', fetch } as FetchContext };
}

const SCOPE = { scope: 'probe' as const, scopeId: 'p1', start: '2026-01-01', end: '2026-02-01' };

describe('metricScatterQuery', () => {
  it('builds the scatter path with x/y + optional channels', async () => {
    const { ctx, calls } = capture({ points: [] });
    const outcome = await metricScatterQuery(ctx, {
      ...SCOPE,
      xMetric: 'sentiment',
      yMetric: 'word_count',
      sizeMetric: 'entity_count',
      colorMetric: 'lexical_diversity',
      maxPoints: 500
    }).queryFn();
    expect(outcome.kind).toBe('success');
    const url = calls[0]!.url;
    expect(url).toContain('/metrics/scatter?');
    expect(url).toContain('xMetric=sentiment');
    expect(url).toContain('yMetric=word_count');
    expect(url).toContain('sizeMetric=entity_count');
    expect(url).toContain('maxPoints=500');
  });
});

describe('correlationMatrixQuery', () => {
  it('joins the metric set into the correlation path', async () => {
    const { ctx, calls } = capture({ metrics: [], matrix: [] });
    await correlationMatrixQuery(ctx, { ...SCOPE, metrics: ['a', 'b', 'c'] }).queryFn();
    expect(calls[0]!.url).toContain('/metrics/correlation?');
    expect(calls[0]!.url).toContain('metrics=a%2Cb%2Cc');
  });
});

describe('crossTabQuery', () => {
  it('encodes field + metric into the path and forwards topN', async () => {
    const { ctx, calls } = capture({ rows: [] });
    await crossTabQuery(ctx, 'country', 'sentiment', { ...SCOPE, topN: 10 }).queryFn();
    expect(calls[0]!.url).toContain('/metadata/country/by-metric/sentiment?');
    expect(calls[0]!.url).toContain('topN=10');
  });
});

describe('correlationLeadLagQuery', () => {
  it('builds the lead-lag path with both metrics + maxLagHours', async () => {
    const { ctx, calls } = capture({ lags: [] });
    await correlationLeadLagQuery(ctx, {
      ...SCOPE,
      xMetric: 'a',
      yMetric: 'b',
      maxLagHours: 48
    }).queryFn();
    expect(calls[0]!.url).toContain('/correlation/lead-lag?');
    expect(calls[0]!.url).toContain('maxLagHours=48');
  });
});

describe('parallelCoordsQuery', () => {
  it('builds the parallel path with metrics + maxPoints', async () => {
    const { ctx, calls } = capture({ axes: [], rows: [] });
    await parallelCoordsQuery(ctx, { ...SCOPE, metrics: ['x', 'y'], maxPoints: 200 }).queryFn();
    expect(calls[0]!.url).toContain('/metrics/parallel?');
    expect(calls[0]!.url).toContain('metrics=x%2Cy');
    expect(calls[0]!.url).toContain('maxPoints=200');
  });
});

describe('sankeyQuery', () => {
  it('joins the field chain into the sankey path', async () => {
    const { ctx, calls } = capture({ nodes: [], links: [] });
    await sankeyQuery(ctx, ['country', 'language'], { ...SCOPE, topN: 5 }).queryFn();
    expect(calls[0]!.url).toContain('/metadata/sankey?');
    expect(calls[0]!.url).toContain('fields=country%2Clanguage');
  });
});

describe('entityCoOccurrenceQueryMulti', () => {
  it('POSTs the scope groups as a JSON body', async () => {
    const { ctx, calls } = capture({ nodes: [], edges: [] });
    await entityCoOccurrenceQueryMulti(ctx, {
      scopes: [{ probeIds: ['p1'], sourceIds: ['s1'] }],
      start: '2026-01-01',
      end: '2026-02-01',
      topN: 30,
      viewerLanguage: 'en'
    }).queryFn();
    expect(calls[0]!.url).toContain('/entities/cooccurrence/query');
    expect(calls[0]!.init?.method).toBe('POST');
    const body = JSON.parse(String(calls[0]!.init?.body));
    expect(body.scopes[0].probeIds).toEqual(['p1']);
    expect(body.topN).toBe(30);
    expect(body.viewerLanguage).toBe('en');
  });
});

describe('topicDistributionQuery', () => {
  it('forwards probe/source ids + confidence + outlier flags', async () => {
    const { ctx, calls } = capture({ topics: [] });
    await topicDistributionQuery(ctx, {
      scope: 'probe',
      scopeId: 'p1',
      probeIds: ['p1', 'p2'],
      sourceIds: ['s1'],
      language: 'de',
      minConfidence: 0.5,
      includeOutlier: true
    }).queryFn();
    const url = calls[0]!.url;
    expect(url).toContain('/topics/distribution?');
    expect(url).toContain('probeIds=p1%2Cp2');
    expect(url).toContain('minConfidence=0.5');
    expect(url).toContain('includeOutlier=true');
  });
});
