import { describe, expect, it } from 'vitest';

import {
  cellContentId,
  DEFAULT_METRIC_NAME,
  getPresentation,
  listPresentations
} from '../../src/lib/viewmodes';

describe('view-mode registry', () => {
  it('exposes the Phase 107 MVP plus Phase 121 Episteme presentations', () => {
    const ids = listPresentations().map((p) => p.id);
    expect(ids).toEqual([
      'time_series',
      'distribution',
      'cooccurrence_network',
      'topic_distribution',
      'topic_evolution'
    ]);
  });

  it('pairs each presentation with a discipline (Phase 121 introduces episteme as the second per-discipline pairing)', () => {
    const disciplines = listPresentations().map((p) => p.discipline);
    // Phase 121: topic_distribution + topic_evolution share the
    // 'episteme' discipline (the pillar, not the presentation), so
    // discipline-uniqueness across the matrix no longer holds. The
    // structural invariant is that every discipline maps to ≥ 1
    // presentation form.
    expect(new Set(disciplines).has('episteme')).toBe(true);
  });

  it('returns the time-series default when the URL state carries no view-mode', () => {
    expect(getPresentation(null).id).toBe('time_series');
  });

  it('round-trips a known id', () => {
    expect(getPresentation('distribution').id).toBe('distribution');
    expect(getPresentation('cooccurrence_network').id).toBe('cooccurrence_network');
    expect(getPresentation('topic_distribution').id).toBe('topic_distribution');
    expect(getPresentation('topic_evolution').id).toBe('topic_evolution');
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
    expect(layouts['topic_distribution']).toBe('per-scope');
    expect(layouts['topic_evolution']).toBe('per-scope');
  });

  it('classes the Phase 121 topic cells under the Episteme discipline', () => {
    const presentations = listPresentations();
    const dist = presentations.find((p) => p.id === 'topic_distribution');
    const evo = presentations.find((p) => p.id === 'topic_evolution');
    expect(dist?.discipline).toBe('episteme');
    expect(evo?.discipline).toBe('episteme');
  });
});
