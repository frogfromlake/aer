import { describe, expect, it } from 'vitest';

import {
  contentQuery,
  entityCoOccurrenceQuery,
  metricDistributionQuery,
  metricsQuery,
  probesQuery,
  type FetchContext
} from '../../src/lib/api/queries';

type FetchFn = typeof globalThis.fetch;

function mockFetch(response: { status: number; body?: unknown; throws?: unknown }): FetchFn {
  return (async () => {
    if (response.throws) throw response.throws;
    return new Response(response.body === undefined ? null : JSON.stringify(response.body), {
      status: response.status,
      headers: { 'Content-Type': 'application/json' }
    });
  }) as unknown as FetchFn;
}

function ctxWith(fetchFn: FetchFn): FetchContext {
  return { baseUrl: '/api/v1', fetch: fetchFn };
}

describe('probesQuery', () => {
  it('returns success with typed data on 200', async () => {
    const probes = [
      {
        probeId: 'p',
        language: 'de',
        emissionPoints: [{ latitude: 0, longitude: 0, label: 'x' }],
        sources: ['s']
      }
    ];
    const q = probesQuery(ctxWith(mockFetch({ status: 200, body: probes })));
    const outcome = await q.queryFn();
    expect(outcome.kind).toBe('success');
    if (outcome.kind === 'success') {
      expect(outcome.data).toEqual(probes);
    }
  });

  it('surfaces a 400 as an unspecified refusal with the BFF message', async () => {
    const q = probesQuery(ctxWith(mockFetch({ status: 400, body: { message: 'bad request' } })));
    const outcome = await q.queryFn();
    expect(outcome.kind).toBe('refusal');
    if (outcome.kind === 'refusal') {
      expect(outcome.refusalKind).toBe('unspecified');
      expect(outcome.message).toBe('bad request');
      expect(outcome.httpStatus).toBe(400);
    }
  });

  it('throws a network-error outcome on 5xx so TanStack Query retries', async () => {
    const q = probesQuery(ctxWith(mockFetch({ status: 503, body: { message: 'down' } })));
    await expect(q.queryFn()).rejects.toMatchObject({
      kind: 'network-error',
      httpStatus: 503
    });
  });

  it('throws a network-error outcome on transport failure', async () => {
    const q = probesQuery(ctxWith(mockFetch({ status: 0, throws: new Error('offline') })));
    await expect(q.queryFn()).rejects.toMatchObject({
      kind: 'network-error',
      message: 'offline'
    });
  });
});

describe('metricsQuery', () => {
  it('encodes query params and serialises raw-mode refusal as validation_missing', async () => {
    let capturedUrl = '';
    const spy: FetchFn = (async (url: RequestInfo | URL) => {
      capturedUrl = typeof url === 'string' ? url : url.toString();
      return new Response(JSON.stringify({ message: 'no validation' }), { status: 400 });
    }) as unknown as FetchFn;

    const q = metricsQuery(ctxWith(spy), {
      startDate: '2026-04-01',
      endDate: '2026-04-22',
      source: 'tagesschau',
      metricName: 'sentiment_score',
      normalization: 'raw',
      resolution: 'hourly'
    });
    const outcome = await q.queryFn();

    expect(capturedUrl).toContain('/metrics?');
    expect(capturedUrl).toContain('startDate=2026-04-01');
    expect(capturedUrl).toContain('source=tagesschau');
    expect(capturedUrl).toContain('normalization=raw');
    expect(capturedUrl).toContain('resolution=hourly');
    expect(outcome.kind).toBe('refusal');
    if (outcome.kind === 'refusal') {
      expect(outcome.refusalKind).toBe('validation_missing');
    }
  });

  it('classifies a zscore-mode refusal as normalization_equivalence_missing', async () => {
    const q = metricsQuery(ctxWith(mockFetch({ status: 400, body: { message: 'no baseline' } })), {
      startDate: '2026-04-01',
      endDate: '2026-04-22',
      normalization: 'zscore'
    });
    const outcome = await q.queryFn();
    expect(outcome.kind).toBe('refusal');
    if (outcome.kind === 'refusal') {
      expect(outcome.refusalKind).toBe('normalization_equivalence_missing');
    }
  });
});

describe('contentQuery', () => {
  it('url-encodes the entity id and includes locale', async () => {
    let capturedUrl = '';
    const spy: FetchFn = (async (url: RequestInfo | URL) => {
      capturedUrl = typeof url === 'string' ? url : url.toString();
      return new Response(
        JSON.stringify({
          entityId: 'probe-0',
          entityType: 'probe',
          locale: 'de',
          registers: {
            semantic: { short: 's', long: 'l' },
            methodological: { short: 's', long: 'l' }
          },
          contentVersion: 'v1',
          lastReviewedBy: 'me',
          lastReviewedDate: '2026-04-22'
        }),
        { status: 200 }
      );
    }) as unknown as typeof fetch;

    const q = contentQuery(ctxWith(spy), 'probe', 'probe 0/weird', 'de');
    const outcome = await q.queryFn();

    expect(capturedUrl).toContain('/content/probe/probe%200%2Fweird');
    expect(capturedUrl).toContain('locale=de');
    expect(outcome.kind).toBe('success');
  });
});

describe('view-mode queries (Phase 107)', () => {
  it('metricDistributionQuery encodes scope, scopeId, and the time window', async () => {
    let capturedUrl = '';
    const spy: FetchFn = (async (url: RequestInfo | URL) => {
      capturedUrl = typeof url === 'string' ? url : url.toString();
      return new Response('{}', { status: 200 });
    }) as unknown as FetchFn;

    const q = metricDistributionQuery(ctxWith(spy), 'sentiment_score', {
      scope: 'source',
      scopeId: 'tagesschau',
      start: '2026-04-01T00:00:00.000Z',
      end: '2026-04-22T00:00:00.000Z',
      bins: 40
    });
    await q.queryFn();
    expect(capturedUrl).toContain('/metrics/sentiment_score/distribution?');
    expect(capturedUrl).toContain('scope=source');
    expect(capturedUrl).toContain('scopeId=tagesschau');
    expect(capturedUrl).toContain('bins=40');
  });

  it('entityCoOccurrenceQuery defaults to probe scope when called with one', async () => {
    let capturedUrl = '';
    const spy: FetchFn = (async (url: RequestInfo | URL) => {
      capturedUrl = typeof url === 'string' ? url : url.toString();
      return new Response('{}', { status: 200 });
    }) as unknown as FetchFn;

    const q = entityCoOccurrenceQuery(ctxWith(spy), {
      scope: 'probe',
      scopeId: 'probe-0-de-institutional-rss',
      start: '2026-04-01T00:00:00.000Z',
      end: '2026-04-22T00:00:00.000Z',
      topN: 25
    });
    await q.queryFn();
    expect(capturedUrl).toContain('/entities/cooccurrence?');
    expect(capturedUrl).toContain('scope=probe');
    expect(capturedUrl).toContain('topN=25');
  });
});
