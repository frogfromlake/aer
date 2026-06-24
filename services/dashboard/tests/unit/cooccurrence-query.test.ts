import { describe, it, expect } from 'vitest';
import {
  isMultiProbeMerge,
  scopeLanguages,
  isCrossLanguageMerge,
  coOccurrenceQueryForScope,
  effectiveEdgeCap,
  autoSettleSeconds,
  COOCCURRENCE_EDGE_DENSITY,
  COOCCURRENCE_EDGE_MAX
} from '../../src/lib/presentations/cooccurrence-query';
import type { FetchContext } from '../../src/lib/api/queries';

const ctx = {} as FetchContext;
const langs: Record<string, string> = {
  'probe-0-de': 'de',
  'probe-1-fr': 'fr',
  'probe-2-de': 'de'
};
const languageOf = (id: string): string | undefined => langs[id];

describe('isMultiProbeMerge', () => {
  it('is false for zero or one probe', () => {
    expect(isMultiProbeMerge(undefined)).toBe(false);
    expect(isMultiProbeMerge([])).toBe(false);
    expect(isMultiProbeMerge(['probe-0-de'])).toBe(false);
  });
  it('is true for more than one probe', () => {
    expect(isMultiProbeMerge(['probe-0-de', 'probe-1-fr'])).toBe(true);
  });
});

describe('scopeLanguages', () => {
  it('dedupes and sorts distinct probe languages', () => {
    expect(scopeLanguages(['probe-0-de', 'probe-2-de'], languageOf)).toEqual(['de']);
    expect(scopeLanguages(['probe-1-fr', 'probe-0-de'], languageOf)).toEqual(['de', 'fr']);
  });
  it('skips probes with no known language', () => {
    expect(scopeLanguages(['probe-0-de', 'unknown'], languageOf)).toEqual(['de']);
  });
});

describe('isCrossLanguageMerge', () => {
  it('is false for a single-language multi-probe merge', () => {
    expect(isCrossLanguageMerge(['probe-0-de', 'probe-2-de'], languageOf)).toBe(false);
  });
  it('is true only when >1 probe spans >1 language', () => {
    expect(isCrossLanguageMerge(['probe-0-de', 'probe-1-fr'], languageOf)).toBe(true);
    // single probe is never a cross-language merge, even if others exist
    expect(isCrossLanguageMerge(['probe-1-fr'], languageOf)).toBe(false);
  });
});

describe('effectiveEdgeCap (node↔edge coupling)', () => {
  it('auto-scales edges with the node count (density baseline)', () => {
    expect(effectiveEdgeCap(200)).toBe(200 * COOCCURRENCE_EDGE_DENSITY);
    expect(effectiveEdgeCap(400)).toBe(400 * COOCCURRENCE_EDGE_DENSITY);
  });
  it('honours an explicit density (edge lever pins a value)', () => {
    expect(effectiveEdgeCap(400, 800)).toBe(800);
  });
  it('clamps to the BFF edge ceiling', () => {
    expect(effectiveEdgeCap(10000)).toBe(COOCCURRENCE_EDGE_MAX);
    expect(effectiveEdgeCap(200, 999999)).toBe(COOCCURRENCE_EDGE_MAX);
  });
  it('never returns below the minimum', () => {
    expect(effectiveEdgeCap(0)).toBeGreaterThanOrEqual(5);
  });
});

describe('autoSettleSeconds', () => {
  it('scales ~1s per 100 nodes, clamped to [12, 120]', () => {
    expect(autoSettleSeconds(200)).toBe(12); // 2 → clamped up
    expect(autoSettleSeconds(5000)).toBe(50);
    expect(autoSettleSeconds(10000)).toBe(100);
    expect(autoSettleSeconds(20000)).toBe(120); // clamped down
  });
});

describe('coOccurrenceQueryForScope routing', () => {
  const base = {
    scope: 'probe' as const,
    scopeId: 'probe-0-de',
    start: undefined,
    end: undefined,
    topN: 60
  };
  it('routes a single probe to the GET query', () => {
    const q = coOccurrenceQueryForScope(ctx, { ...base, probeIds: ['probe-0-de'] });
    expect(q.queryKey[1]).toBe('entity-cooccurrence');
  });
  it('routes a multi-probe merge to the multi-scope POST query', () => {
    const q = coOccurrenceQueryForScope(ctx, {
      ...base,
      probeIds: ['probe-0-de', 'probe-1-fr'],
      sourceIds: [],
      allowCrossLanguage: true
    });
    expect(q.queryKey[1]).toBe('entity-cooccurrence-multi');
  });
});
