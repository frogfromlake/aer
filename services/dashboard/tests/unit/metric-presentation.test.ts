import { describe, expect, it } from 'vitest';

import {
  getPresentation,
  isPureCountMetric,
  metricSupportsPresentation,
  panelSubjectKind,
  presentationsForMetric
} from '../../src/lib/presentations';
import type { Presentation } from '../../src/lib/state/url-internals';

describe('metric → presentation compatibility map (Phase 130 / ADR-035)', () => {
  it('treats an unknown / scalar metric as distribution + time_series', () => {
    expect(presentationsForMetric('sentiment_score_sentiws')).toEqual([
      'distribution',
      'time_series'
    ]);
    // unknown metric falls through to the scalar default
    expect(presentationsForMetric('some_future_scalar')).toEqual(['distribution', 'time_series']);
  });

  it('restricts cyclic metrics to distribution only', () => {
    expect(presentationsForMetric('publication_hour')).toEqual(['distribution']);
    expect(presentationsForMetric('publication_weekday')).toEqual(['distribution']);
  });

  it('restricts temporal_distribution to time_series only', () => {
    expect(presentationsForMetric('temporal_distribution')).toEqual(['time_series']);
  });

  it('maps entity_cooccurrence to the relational network cell', () => {
    expect(presentationsForMetric('entity_cooccurrence')).toEqual(['cooccurrence_network']);
  });

  it('metricSupportsPresentation reflects the map', () => {
    // a scalar is reachable as a distribution (Aleph) AND a time-series (Episteme)
    expect(metricSupportsPresentation('sentiment_score_sentiws', 'distribution')).toBe(true);
    expect(metricSupportsPresentation('sentiment_score_sentiws', 'time_series')).toBe(true);
    // publication_hour is offered ONLY as a distribution
    expect(metricSupportsPresentation('publication_hour', 'distribution')).toBe(true);
    expect(metricSupportsPresentation('publication_hour', 'time_series')).toBe(false);
    // temporal_distribution is never a distribution
    expect(metricSupportsPresentation('temporal_distribution', 'distribution')).toBe(false);
    expect(metricSupportsPresentation('temporal_distribution', 'time_series')).toBe(true);
  });
});

describe('pure-count classification (Brief §1.3 merged-cross-probe guard)', () => {
  it('classes extensive counts as pure counts', () => {
    expect(isPureCountMetric('word_count')).toBe(true);
    expect(isPureCountMetric('entity_count')).toBe(true);
    expect(isPureCountMetric('publication_hour')).toBe(true);
    expect(isPureCountMetric('publication_weekday')).toBe(true);
    expect(isPureCountMetric('temporal_distribution')).toBe(true);
  });

  it('classes intensive / scaled metrics as non-pure-count', () => {
    expect(isPureCountMetric('sentiment_score_sentiws')).toBe(false);
    expect(isPureCountMetric('sentiment_score_bert_multilingual')).toBe(false);
    expect(isPureCountMetric('language_confidence')).toBe(false);
    // unknown metrics default to non-pure-count (conservative: refuse merge)
    expect(isPureCountMetric('some_future_metric')).toBe(false);
  });
});

describe('panelSubjectKind (Phase 148f) — what Panel.metric denotes per view', () => {
  const kindOf = (id: Presentation) => panelSubjectKind(getPresentation(id));

  it("classifies metric-driven views as 'metric'", () => {
    expect(kindOf('distribution')).toBe('metric');
    expect(kindOf('time_series')).toBe('metric');
  });

  it("classifies field-grouped views as 'field' (no metric provenance to fetch)", () => {
    expect(kindOf('categorical_distribution')).toBe('field');
    expect(kindOf('cross_tab')).toBe('field');
  });

  it("classifies channel-driven / multivariate / metric-agnostic views as 'none'", () => {
    for (const id of [
      'metric_scatter',
      'cooccurrence_network',
      'topic_distribution',
      'topic_evolution',
      'correlation_matrix',
      'parallel_coordinates',
      'sankey',
      'metric_lead_lag',
      'cross_probe_lead_lag',
      'revision_activity',
      'revision_timeline',
      'revision_discourse_shift',
      'revision_edit_clusters'
    ] as const) {
      expect(kindOf(id)).toBe('none');
    }
  });
});
