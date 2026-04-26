import { describe, expect, it } from 'vitest';

import {
  cellContentId,
  DEFAULT_METRIC_NAME,
  getPresentation,
  listPresentations
} from '../../src/lib/viewmodes';

describe('view-mode registry', () => {
  it('exposes the three Phase 107 MVP presentations', () => {
    const ids = listPresentations().map((p) => p.id);
    expect(ids).toEqual(['time_series', 'distribution', 'cooccurrence_network']);
  });

  it('pairs each presentation with a structurally distinct discipline', () => {
    const disciplines = listPresentations().map((p) => p.discipline);
    expect(new Set(disciplines).size).toBe(disciplines.length);
  });

  it('returns the time-series default when the URL state carries no view-mode', () => {
    expect(getPresentation(null).id).toBe('time_series');
  });

  it('round-trips a known id', () => {
    expect(getPresentation('distribution').id).toBe('distribution');
    expect(getPresentation('cooccurrence_network').id).toBe('cooccurrence_network');
  });

  it('composes content-catalog cell ids matching the BFF yaml convention', () => {
    expect(cellContentId('time_series', 'sentiment_score')).toBe('time_series_sentiment_score');
    expect(cellContentId('distribution', 'word_count')).toBe('distribution_word_count');
    expect(cellContentId('cooccurrence_network', DEFAULT_METRIC_NAME)).toBe(
      'cooccurrence_network_sentiment_score'
    );
  });

  it('marks per-source vs per-scope layout for downstream rendering', () => {
    const layouts = Object.fromEntries(listPresentations().map((p) => [p.id, p.layout]));
    expect(layouts['time_series']).toBe('per-source');
    expect(layouts['distribution']).toBe('per-scope');
    expect(layouts['cooccurrence_network']).toBe('per-scope');
  });
});
