import { afterEach, describe, expect, it, vi } from 'vitest';

import {
  appendMetadataFilter,
  fetchJson,
  setUnauthenticatedHandler,
  type FetchContext
} from '../../src/lib/api/queries';

// Phase 142 — direct coverage of the low-level fetchJson wrapper: the 401
// handler hook, the per-gate refusalKind sharpening, the structured
// alternatives/workingPaperAnchor passthrough, and the metadata-filter helper.
// (The query factories in queries.test.ts cover the happy/refusal/error paths
// at one level up; this file pins the branch logic the factories cannot reach.)

type FetchFn = typeof globalThis.fetch;

function jsonResponse(status: number, body?: unknown, statusText?: string): Response {
  const init: ResponseInit = { status, headers: { 'Content-Type': 'application/json' } };
  if (statusText !== undefined) init.statusText = statusText;
  return new Response(body === undefined ? null : JSON.stringify(body), init);
}

function ctx(fetchFn: FetchFn): FetchContext {
  return { baseUrl: '/api/v1/', fetch: fetchFn };
}

afterEach(() => {
  setUnauthenticatedHandler(() => {});
  vi.unstubAllGlobals();
});

describe('fetchJson — request shaping', () => {
  it('strips a trailing slash from baseUrl and merges Accept + custom headers', async () => {
    let captured: { url: string; init: RequestInit } | null = null;
    const spy: FetchFn = (async (url: RequestInfo | URL, init?: RequestInit) => {
      captured = { url: String(url), init: init ?? {} };
      return jsonResponse(200, { ok: true });
    }) as FetchFn;
    const out = await fetchJson(ctx(spy), '/probes', 'unspecified', {
      headers: { 'X-Test': '1' }
    });
    expect(out.kind).toBe('success');
    expect(captured!.url).toBe('/api/v1/probes'); // no double slash
    const headers = captured!.init.headers as Record<string, string>;
    expect(headers.Accept).toBe('application/json');
    expect(headers['X-Test']).toBe('1');
  });

  it('falls back to the global fetch when no override is supplied', async () => {
    const g = vi.fn(async () => jsonResponse(200, { ok: true }));
    vi.stubGlobal('fetch', g);
    const out = await fetchJson({ baseUrl: '/api/v1' }, '/probes', 'unspecified');
    expect(out.kind).toBe('success');
    expect(g).toHaveBeenCalledOnce();
  });
});

describe('fetchJson — 401 handling', () => {
  it('invokes the unauthenticated handler and throws a 401 network-error', async () => {
    const onUnauth = vi.fn();
    setUnauthenticatedHandler(onUnauth);
    const spy: FetchFn = (async () => jsonResponse(401, { message: 'expired' })) as FetchFn;
    await expect(fetchJson(ctx(spy), '/probes', 'unspecified')).rejects.toMatchObject({
      kind: 'network-error',
      httpStatus: 401,
      message: 'unauthenticated'
    });
    expect(onUnauth).toHaveBeenCalledOnce();
  });

  it('does not throw if no handler is registered (null guard)', async () => {
    setUnauthenticatedHandler(undefined as unknown as () => void);
    // Re-set to null via a fresh import is not possible; instead register a noop
    // then clear by registering an empty fn — the optional-chain still no-ops.
    const spy: FetchFn = (async () => jsonResponse(401)) as FetchFn;
    await expect(fetchJson(ctx(spy), '/x', 'unspecified')).rejects.toMatchObject({
      httpStatus: 401
    });
  });
});

describe('fetchJson — refusal classification', () => {
  it('sharpens to cross_frame_equivalence_missing on the metric_equivalence gate', async () => {
    const spy: FetchFn = (async () =>
      jsonResponse(400, { message: 'no grant', gate: 'metric_equivalence' })) as FetchFn;
    const out = await fetchJson(ctx(spy), '/metrics', 'normalization_equivalence_missing');
    expect(out.kind).toBe('refusal');
    if (out.kind === 'refusal') {
      expect(out.refusalKind).toBe('cross_frame_equivalence_missing');
      expect(out.message).toBe('no grant');
      expect(out.httpStatus).toBe(400);
    }
  });

  it('routes invalid_language / cross_language_merge / scope_limit gates to their kinds', async () => {
    const cases: Array<[string, string]> = [
      ['invalid_language', 'invalid_language'],
      ['cross_language_merge_unsupported', 'cross_language_merge_unsupported'],
      ['scope_limit_exceeded', 'scope_limit_exceeded']
    ];
    for (const [gate, expected] of cases) {
      const spy: FetchFn = (async () => jsonResponse(422, { gate })) as FetchFn;
      const out = await fetchJson(ctx(spy), '/x', 'unspecified');
      expect(out.kind).toBe('refusal');
      if (out.kind === 'refusal') expect(out.refusalKind).toBe(expected);
    }
  });

  it('passes structured alternatives + workingPaperAnchor through onto the refusal', async () => {
    const spy: FetchFn = (async () =>
      jsonResponse(403, {
        message: 'k-anon',
        alternatives: ['widen the window', 42, 'add a source'],
        workingPaperAnchor: 'WP-006#7'
      })) as FetchFn;
    const out = await fetchJson(ctx(spy), '/x', 'k_anonymity_threshold_not_met');
    expect(out.kind).toBe('refusal');
    if (out.kind === 'refusal') {
      // non-string alternatives are filtered out.
      expect(out.alternatives).toEqual(['widen the window', 'add a source']);
      expect(out.workingPaperAnchor).toBe('WP-006#7');
    }
  });

  it('keeps the expected kind and a status-line message when the refusal body is opaque', async () => {
    const spy: FetchFn = (async () =>
      new Response('not json', { status: 400, statusText: 'Bad Request' })) as FetchFn;
    const out = await fetchJson(ctx(spy), '/x', 'validation_missing');
    expect(out.kind).toBe('refusal');
    if (out.kind === 'refusal') {
      expect(out.refusalKind).toBe('validation_missing');
      expect(out.message).toBe('400 Bad Request');
      expect(out.alternatives).toBeUndefined();
    }
  });
});

describe('fetchJson — error + transport paths', () => {
  it('throws a network-error with the BFF message on a 5xx', async () => {
    const spy: FetchFn = (async () =>
      jsonResponse(503, { message: 'down' }, 'Service Unavailable')) as FetchFn;
    await expect(fetchJson(ctx(spy), '/x', 'unspecified')).rejects.toMatchObject({
      kind: 'network-error',
      httpStatus: 503,
      message: 'down'
    });
  });

  it('falls back to the status line when the 5xx body has no message', async () => {
    const spy: FetchFn = (async () =>
      new Response('boom', { status: 500, statusText: 'Server Error' })) as FetchFn;
    await expect(fetchJson(ctx(spy), '/x', 'unspecified')).rejects.toMatchObject({
      message: '500 Server Error'
    });
  });

  it('maps a transport failure to a network-error with the error message', async () => {
    const spy: FetchFn = (async () => {
      throw new Error('offline');
    }) as FetchFn;
    await expect(fetchJson(ctx(spy), '/x', 'unspecified')).rejects.toMatchObject({
      kind: 'network-error',
      message: 'offline'
    });
  });

  it('maps a non-Error throw to a generic transport message', async () => {
    const spy: FetchFn = (async () => {
      throw 'weird';
    }) as FetchFn;
    await expect(fetchJson(ctx(spy), '/x', 'unspecified')).rejects.toMatchObject({
      message: 'network request failed'
    });
  });
});

describe('appendMetadataFilter', () => {
  it('writes both halves when field and value are present', () => {
    const qs = new URLSearchParams();
    appendMetadataFilter(qs, { field: 'section', value: 'Politik' });
    expect(qs.get('metadataFilterField')).toBe('section');
    expect(qs.get('metadataFilterValue')).toBe('Politik');
  });

  it('writes nothing when the filter is undefined or a half is blank', () => {
    const a = new URLSearchParams();
    appendMetadataFilter(a, undefined);
    expect(a.toString()).toBe('');
    const b = new URLSearchParams();
    appendMetadataFilter(b, { field: 'section', value: '' });
    expect(b.toString()).toBe('');
    const c = new URLSearchParams();
    appendMetadataFilter(c, { field: '', value: 'x' });
    expect(c.toString()).toBe('');
  });
});
