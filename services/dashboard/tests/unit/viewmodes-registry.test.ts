import { describe, expect, it } from 'vitest';

import {
  cellContentId,
  DEFAULT_METRIC_NAME,
  defaultViewModeForPillar,
  getPillar,
  getPresentation,
  listPresentations,
  pillarForViewMode,
  PILLAR_DEFINITIONS,
  presentationsForPillar,
  resolvePresentation
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

  it('returns the distribution default when the URL state carries no view-mode (Phase 130)', () => {
    expect(getPresentation(null).id).toBe('distribution');
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
      'cooccurrence_network_sentiment_score_sentiws'
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

describe('pillar mapping', () => {
  it('defines all three pillars in display order: Aleph, Episteme, Rhizome', () => {
    expect(PILLAR_DEFINITIONS.map((p) => p.id)).toEqual(['aleph', 'episteme', 'rhizome']);
  });

  it('uses strict 1-to-1 presentation mapping with no overlap', () => {
    const seen = new Set<string>();
    for (const p of PILLAR_DEFINITIONS) {
      for (const v of p.presentations) {
        expect(seen.has(v)).toBe(false);
        seen.add(v);
      }
    }
    // Phase 130 / ADR-035 — pillar follows presentation.
    // Aleph (synchronic): distribution + topic_distribution
    expect(getPillar('aleph').presentations).toEqual(['distribution', 'topic_distribution']);
    // Episteme (diachronic): time_series + topic_evolution
    expect(getPillar('episteme').presentations).toEqual(['time_series', 'topic_evolution']);
    // Rhizome (relational): cooccurrence_network
    expect(getPillar('rhizome').presentations).toEqual(['cooccurrence_network']);
  });

  it('falls back to Aleph for unknown/null pillar', () => {
    expect(getPillar(null).id).toBe('aleph');
  });

  it('returns the right presentations per pillar', () => {
    const alephIds = presentationsForPillar('aleph').map((p) => p.id);
    expect(alephIds).toEqual(['distribution', 'topic_distribution']);

    const epistemIds = presentationsForPillar('episteme').map((p) => p.id);
    expect(epistemIds).toEqual(['time_series', 'topic_evolution']);

    const rhizIds = presentationsForPillar('rhizome').map((p) => p.id);
    expect(rhizIds).toEqual(['cooccurrence_network']);

    // null → Aleph default
    expect(presentationsForPillar(null).map((p) => p.id)).toEqual([
      'distribution',
      'topic_distribution'
    ]);
  });

  it('returns the first presentation as the pillar default', () => {
    expect(defaultViewModeForPillar('aleph')).toBe('distribution');
    expect(defaultViewModeForPillar('episteme')).toBe('time_series');
    expect(defaultViewModeForPillar('rhizome')).toBe('cooccurrence_network');
    expect(defaultViewModeForPillar(null)).toBe('distribution');
  });

  it('reverse-maps every presentation back to exactly one pillar', () => {
    expect(pillarForViewMode('time_series')).toBe('episteme');
    expect(pillarForViewMode('distribution')).toBe('aleph');
    expect(pillarForViewMode('topic_distribution')).toBe('aleph');
    expect(pillarForViewMode('topic_evolution')).toBe('episteme');
    expect(pillarForViewMode('cooccurrence_network')).toBe('rhizome');
  });

  it('resolvePresentation respects the active pillar', () => {
    // viewMode belongs to active pillar → return it
    expect(resolvePresentation('topic_evolution', 'episteme').id).toBe('topic_evolution');
    expect(resolvePresentation('time_series', 'episteme').id).toBe('time_series');
    expect(resolvePresentation('topic_distribution', 'aleph').id).toBe('topic_distribution');
    // viewMode does NOT belong to active pillar → fall back to pillar default
    expect(resolvePresentation('cooccurrence_network', 'aleph').id).toBe('distribution');
    expect(resolvePresentation('time_series', 'aleph').id).toBe('distribution');
    // null viewMode → pillar default
    expect(resolvePresentation(null, 'rhizome').id).toBe('cooccurrence_network');
    expect(resolvePresentation(null, null).id).toBe('distribution');
  });

  it('each pillar carries a glyph, label, abbr, blurb, description and color', () => {
    for (const p of PILLAR_DEFINITIONS) {
      expect(p.glyph).toBeTruthy();
      expect(p.label).toBeTruthy();
      expect(p.abbr).toBeTruthy();
      expect(p.blurb).toBeTruthy();
      expect(p.description).toBeTruthy();
      expect(p.color).toMatch(/^#[0-9a-fA-F]{6}$/);
    }
  });
});
