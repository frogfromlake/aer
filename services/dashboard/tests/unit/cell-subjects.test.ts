import { describe, expect, it } from 'vitest';

import { cellSubjects, getPresentation } from '../../src/lib/presentations';
import type { Panel, Presentation } from '../../src/lib/state/url-internals';

// Minimal Panel factory — only the fields cellSubjects reads matter; the rest
// satisfy the type.
function panel(p: Partial<Panel> & { view: Presentation }): Panel {
  return {
    scopes: [{ probeIds: ['probe-0'], sourceIds: [] }],
    composition: 'merged',
    metric: '',
    layer: 'gold',
    ...p
  } as Panel;
}

function subjectsFor(view: Presentation, p: Partial<Panel>) {
  return cellSubjects(getPresentation(view), panel({ view, ...p }));
}

describe('cellSubjects (Phase 148g)', () => {
  it('single-metric views bind Panel.metric as the primary subject', () => {
    expect(subjectsFor('distribution', { metric: 'word_count' })).toEqual([
      { name: 'word_count', roles: ['primary'] }
    ]);
    expect(subjectsFor('time_series', { metric: 'sentiment_score_sentiws' })).toEqual([
      { name: 'sentiment_score_sentiws', roles: ['primary'] }
    ]);
  });

  it('scatter binds x / y / size / colour channels', () => {
    const s = subjectsFor('metric_scatter', {
      channels: { x: 'sentiment_score_sentiws', y: 'word_count', size: 'entity_count' }
    });
    expect(s).toEqual([
      { name: 'sentiment_score_sentiws', roles: ['x'] },
      { name: 'word_count', roles: ['y'] },
      { name: 'entity_count', roles: ['size'] }
    ]);
  });

  it('dedupes a metric bound to more than one channel, merging roles', () => {
    const s = subjectsFor('metric_scatter', {
      channels: { x: 'word_count', y: 'entity_count', color: 'word_count' }
    });
    expect(s).toContainEqual({ name: 'word_count', roles: ['x', 'color'] });
    expect(s).toContainEqual({ name: 'entity_count', roles: ['y'] });
  });

  it('correlation_matrix / parallel_coordinates bind the metric set', () => {
    expect(
      subjectsFor('correlation_matrix', { metricSet: ['word_count', 'entity_count'] })
    ).toEqual([
      { name: 'word_count', roles: ['set'] },
      { name: 'entity_count', roles: ['set'] }
    ]);
  });

  it('cross_tab binds the group-by field and the aggregated metric', () => {
    expect(
      subjectsFor('cross_tab', { metric: 'section', channels: { x: 'sentiment_score_sentiws' } })
    ).toEqual([
      { name: 'section', roles: ['groupBy'] },
      { name: 'sentiment_score_sentiws', roles: ['aggregated'] }
    ]);
  });

  it('metric_lead_lag binds leading / lagging metrics', () => {
    expect(
      subjectsFor('metric_lead_lag', {
        channels: { x: 'sentiment_score_sentiws', y: 'word_count' }
      })
    ).toEqual([
      { name: 'sentiment_score_sentiws', roles: ['leading'] },
      { name: 'word_count', roles: ['lagging'] }
    ]);
  });

  it('cooccurrence node metrics only count when their channel is bound to metric', () => {
    expect(
      subjectsFor('cooccurrence_network', {
        channels: { netSize: 'metric', netMetric: 'word_count', netColor: 'community' }
      })
    ).toEqual([{ name: 'word_count', roles: ['nodeSize'] }]);
    // colour falls back to the size metric when its own colour metric is unset
    expect(
      subjectsFor('cooccurrence_network', {
        channels: { netSize: 'metric', netColor: 'metric', netMetric: 'word_count' }
      })
    ).toEqual([{ name: 'word_count', roles: ['nodeSize', 'nodeColor'] }]);
  });

  it('categorical_distribution binds Panel.metric as a category field', () => {
    expect(subjectsFor('categorical_distribution', { metric: 'author' })).toEqual([
      { name: 'author', roles: ['field'] }
    ]);
  });

  it('sankey binds the field chain', () => {
    expect(subjectsFor('sankey', { fieldChain: ['section', 'author'] })).toEqual([
      { name: 'section', roles: ['chain'] },
      { name: 'author', roles: ['chain'] }
    ]);
  });

  it('adds the facet field as a subject on a faceting view', () => {
    const s = subjectsFor('distribution', { metric: 'word_count', facetField: 'section' });
    expect(s).toEqual([
      { name: 'word_count', roles: ['primary'] },
      { name: 'section', roles: ['facet'] }
    ]);
  });

  it('returns no per-subject methodology for metric-agnostic views', () => {
    expect(subjectsFor('revision_activity', { metric: 'whatever' })).toEqual([]);
    expect(subjectsFor('topic_distribution', { metric: 'stale_metric' })).toEqual([]);
  });
});
