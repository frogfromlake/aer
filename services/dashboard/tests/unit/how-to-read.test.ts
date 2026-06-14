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
});
