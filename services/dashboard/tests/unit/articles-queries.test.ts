import { describe, expect, it } from 'vitest';

import {
  articleDetailQuery,
  metadataDistributionQuery,
  metricsAvailableQuery,
  scopeAvailableMetadataQuery,
  sourceArticlesQuery,
  type FetchContext
} from '../../src/lib/api/queries';

type FetchFn = typeof globalThis.fetch;

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

const SCOPE = { scope: 'source' as const, scopeId: 's1', start: '2026-01-01', end: '2026-02-01' };

describe('sourceArticlesQuery', () => {
  it('forwards the article-list filters + pagination', async () => {
    const { ctx, calls } = capture({ articles: [], nextCursor: null });
    const outcome = await sourceArticlesQuery(ctx, 'tagesschau', {
      start: '2026-01-01',
      language: 'de',
      entityMatch: 'Merkel',
      sentimentBand: 'negative',
      limit: 20,
      cursor: 'c1',
      includeRevisions: true
    }).queryFn();
    expect(outcome.kind).toBe('success');
    const url = calls[0]!.url;
    expect(url).toContain('/sources/tagesschau/articles?');
    expect(url).toContain('entityMatch=Merkel');
    expect(url).toContain('sentimentBand=negative');
    expect(url).toContain('includeRevisions=true');
  });
});

describe('articleDetailQuery', () => {
  it('adds metricName when provided and omits otherwise', async () => {
    const { ctx, calls } = capture({ articleId: 'a1' });
    await articleDetailQuery(ctx, 'a1', 'sentiment').queryFn();
    expect(calls[0]!.url).toContain('/articles/a1?metricName=sentiment');

    const bare = capture({ articleId: 'a1' });
    await articleDetailQuery(bare.ctx, 'a1').queryFn();
    expect(bare.calls[0]!.url).toMatch(/\/articles\/a1$/);
  });
});

describe('metricsAvailableQuery', () => {
  it('builds the available-metrics path with the window', async () => {
    const { ctx, calls } = capture([]);
    await metricsAvailableQuery(ctx, { startDate: '2026-01-01', endDate: '2026-02-01' }).queryFn();
    expect(calls[0]!.url).toContain('/metrics/available?');
    expect(calls[0]!.url).toContain('startDate=2026-01-01');
  });
});

describe('metadataDistributionQuery', () => {
  it('encodes the field + forwards topN', async () => {
    const { ctx, calls } = capture({ values: [] });
    await metadataDistributionQuery(ctx, 'section', { ...SCOPE, topN: 15 }).queryFn();
    expect(calls[0]!.url).toContain('/metadata/section/distribution?');
    expect(calls[0]!.url).toContain('topN=15');
  });
});

describe('scopeAvailableMetadataQuery', () => {
  it('joins probe/source ids into the available-metadata path', async () => {
    const { ctx, calls } = capture({ fields: [] });
    await scopeAvailableMetadataQuery(ctx, {
      probeIds: ['p1', 'p2'],
      sourceIds: ['s1'],
      start: '2026-01-01'
    }).queryFn();
    const url = calls[0]!.url;
    expect(url).toContain('/scope/available-metadata?');
    expect(url).toContain('probeIds=p1%2Cp2');
    expect(url).toContain('sourceIds=s1');
  });
});
