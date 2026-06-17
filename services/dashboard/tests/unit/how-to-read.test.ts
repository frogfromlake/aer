import { describe, expect, it } from 'vitest';

import { composeHowToRead } from '../../src/lib/presentations/how-to-read';

// Phase 131 — the "how to read" note is composed from a per-presentation
// template plus config-derived building blocks.

describe('composeHowToRead', () => {
  it('uses the built-in fallback template when none is supplied', () => {
    const lines = composeHowToRead('distribution', { bins: 30 });
    expect(lines[0]).toContain('full distribution');
    expect(lines.some((l) => l.includes('30'))).toBe(true);
  });

  it('prefers the catalog template over the built-in fallback', () => {
    const lines = composeHowToRead('distribution', { bins: 30 }, 'CATALOG TEMPLATE LINE');
    expect(lines[0]).toBe('CATALOG TEMPLATE LINE');
  });

  it('reflects the active bin count for distribution', () => {
    const lines = composeHowToRead('distribution', { bins: 80 });
    expect(lines.some((l) => l.includes('80 bars'))).toBe(true);
  });

  it('describes the band state for time series', () => {
    const shown = composeHowToRead('time_series', { showBand: true });
    expect(shown.some((l) => l.includes('±1 standard deviation'))).toBe(true);
    const hidden = composeHowToRead('time_series', { showBand: false });
    expect(hidden.some((l) => l.toLowerCase().includes('hidden'))).toBe(true);
  });

  it('describes scatter position/size/colour channel bindings', () => {
    const lines = composeHowToRead('metric_scatter', {
      x: 'word_count',
      y: 'sentiment_score_sentiws',
      size: 'entity_count',
      color: 'language_confidence',
      renderedCount: 12
    });
    expect(lines.some((l) => l.includes('word_count'))).toBe(true);
    expect(lines.some((l) => l.includes('Bigger dots = higher entity_count'))).toBe(true);
    expect(lines.some((l) => l.includes('language_confidence'))).toBe(true);
    expect(lines.some((l) => l.includes('12 article'))).toBe(true);
  });

  it('describes the network top-N and bound visual channels', () => {
    const lines = composeHowToRead('cooccurrence_network', {
      topN: 80,
      netSize: 'degree',
      netColor: 'presence'
    });
    expect(lines.some((l) => l.includes('80 most-frequent pairs'))).toBe(true);
    expect(lines.some((l) => l.includes('different entities it connects to'))).toBe(true);
    expect(lines.some((l) => l.includes('how many sources mention it'))).toBe(true);
  });

  it('never returns empty strings', () => {
    const lines = composeHowToRead('topic_distribution', {});
    expect(lines.length).toBeGreaterThan(0);
    expect(lines.every((l) => l.length > 0)).toBe(true);
  });

  it('describes the scatter regression r with strength + direction', () => {
    const strong = composeHowToRead('metric_scatter', { x: 'a', y: 'b', r: 0.85 });
    expect(strong.some((l) => l.includes('strong positive correlation'))).toBe(true);
    const weakNeg = composeHowToRead('metric_scatter', { x: 'a', y: 'b', r: -0.25 });
    expect(weakNeg.some((l) => l.includes('weak negative correlation'))).toBe(true);
    const moderate = composeHowToRead('metric_scatter', { x: 'a', y: 'b', r: 0.5 });
    expect(moderate.some((l) => l.includes('moderate positive correlation'))).toBe(true);
    const flat = composeHowToRead('metric_scatter', { x: 'a', y: 'b', r: 0 });
    expect(flat.some((l) => l.includes('negligible flat correlation'))).toBe(true);
  });

  it('cooccurrence viewer-language relabel discloses the labeled-node count + singular/plural', () => {
    const many = composeHowToRead('cooccurrence_network', {
      topN: 50,
      netSize: 'total_count',
      netColor: 'label',
      displayLanguage: 'viewer',
      viewerLanguage: 'en',
      labeledNodeCount: 3
    });
    expect(many.some((l) => l.includes('3 Wikidata-linked dots are shown'))).toBe(true);
    const one = composeHowToRead('cooccurrence_network', {
      topN: 50,
      netSize: 'total_count',
      netColor: 'label',
      displayLanguage: 'viewer',
      viewerLanguage: 'en',
      labeledNodeCount: 1
    });
    expect(one.some((l) => l.includes('1 Wikidata-linked dot is shown'))).toBe(true);
  });

  it('cooccurrence source-language disclosure names the linked-node count when relabel is off', () => {
    const lines = composeHowToRead('cooccurrence_network', {
      topN: 50,
      netSize: 'metric',
      netColor: 'metric',
      displayLanguage: 'source',
      linkedNodeCount: 7
    });
    expect(lines.some((l) => l.includes('7 dots are Wikidata-linked'))).toBe(true);
    // metric size/colour channel labels also surface.
    expect(lines.some((l) => l.includes('mean of the chosen metric'))).toBe(true);
  });

  it('covers the uniform + source_overlay network colour channel labels', () => {
    const uniform = composeHowToRead('cooccurrence_network', {
      topN: 1,
      netSize: 'total_count',
      netColor: 'uniform'
    });
    expect(uniform.some((l) => l.includes('all dots share one colour'))).toBe(true);
    const overlay = composeHowToRead('cooccurrence_network', {
      topN: 1,
      netSize: 'total_count',
      netColor: 'source_overlay'
    });
    expect(overlay.some((l) => l.includes('which source it came from'))).toBe(true);
  });

  it('composes the correlation_matrix note with the rendered metric count (plural/singular)', () => {
    const multi = composeHowToRead('correlation_matrix', { renderedCount: 4 });
    expect(multi.some((l) => l.includes('4 metrics in the matrix'))).toBe(true);
    const single = composeHowToRead('correlation_matrix', { renderedCount: 1 });
    expect(single.some((l) => l.includes('1 metric in the matrix'))).toBe(true);
  });

  it('composes the cross_tab note with the rendered category count', () => {
    const lines = composeHowToRead('cross_tab', { renderedCount: 3, distinctValues: 10 });
    expect(lines.some((l) => l.includes('3 categories shown of 10'))).toBe(true);
    const oneOfOne = composeHowToRead('cross_tab', { renderedCount: 1, distinctValues: 1 });
    expect(oneOfOne.some((l) => l.includes('1 category shown'))).toBe(true);
  });

  it('composes the metric_lead_lag note from the x/y metrics + bucket count', () => {
    const lines = composeHowToRead('metric_lead_lag', {
      x: 'word_count',
      y: 'sentiment_score',
      renderedCount: 42
    });
    expect(lines.some((l) => l.includes('word_count') && l.includes('sentiment_score'))).toBe(true);
    expect(lines.some((l) => l.includes('42 overlapping hourly buckets'))).toBe(true);
  });

  it('composes parallel_coordinates + sankey + cross_probe_lead_lag notes', () => {
    expect(
      composeHowToRead('parallel_coordinates', { renderedCount: 1 }).some((l) =>
        l.includes('1 article drawn')
      )
    ).toBe(true);
    expect(composeHowToRead('sankey', {}).some((l) => l.includes('Bands are the top flows'))).toBe(
      true
    );
    const crossProbe = composeHowToRead('cross_probe_lead_lag', { renderedCount: 1 });
    expect(crossProbe.some((l) => l.includes('1 overlapping hour'))).toBe(true);
  });

  it('composes the categorical_distribution top-N-of-M note', () => {
    const lines = composeHowToRead('categorical_distribution', {
      renderedCount: 5,
      distinctValues: 12
    });
    expect(lines.some((l) => l.includes('top 5 of 12 distinct values'))).toBe(true);
  });

  it('appends the shared/free axis-scale disclosure for value-axis presentations', () => {
    const shared = composeHowToRead('distribution', { scales: 'shared' });
    expect(shared.some((l) => l.includes('shared across this panel'))).toBe(true);
    const free = composeHowToRead('time_series', { scales: 'free' });
    expect(free.some((l) => l.includes('independent (free)'))).toBe(true);
    // A non-value-axis presentation gets no scale note even when scales is set.
    const network = composeHowToRead('cooccurrence_network', {
      topN: 1,
      netSize: 'total_count',
      netColor: 'label',
      scales: 'shared'
    });
    expect(network.every((l) => !l.includes('shared across this panel'))).toBe(true);
  });

  it('appends the per-cell override disclosure when configOverridden is set', () => {
    const lines = composeHowToRead('distribution', { configOverridden: true });
    expect(lines.some((l) => l.includes('custom configuration that differs'))).toBe(true);
  });

  it('blank catalog template falls through to the built-in fallback', () => {
    const lines = composeHowToRead('distribution', {}, '   ');
    expect(lines[0]).toContain('full distribution');
  });
});
