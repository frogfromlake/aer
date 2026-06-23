import { describe, expect, it } from 'vitest';

import {
  cellContentId,
  hasCellMethodologyContent,
  DEFAULT_METRIC_NAME,
  defaultPresentationForPillar,
  getPillar,
  getPresentation,
  listPresentations,
  pillarForPresentation,
  PILLAR_DEFINITIONS,
  presentationsForPillar,
  resolvePresentation
} from '../../src/lib/presentations';

describe('view-mode registry', () => {
  it('exposes the Phase 107 MVP, Phase 121 Episteme, Phase 131 scatter, and Phase 122d.0 revision presentations', () => {
    const ids = listPresentations().map((p) => p.id);
    expect(ids).toEqual([
      'time_series',
      'distribution',
      'cooccurrence_network',
      'metric_scatter',
      'topic_distribution',
      'topic_evolution',
      'revision_activity',
      'revision_timeline',
      'revision_discourse_shift',
      'revision_edit_clusters',
      'cross_probe_lead_lag',
      'categorical_distribution',
      'correlation_matrix',
      'cross_tab',
      'metric_lead_lag',
      'parallel_coordinates',
      'sankey'
    ]);
  });

  // Phase 131 — the scatter presentation lives in Aleph (synchronic) and
  // declares its configurable visual-channel binding.
  it('places metric_scatter in the Aleph pillar', () => {
    expect(presentationsForPillar('aleph').map((p) => p.id)).toContain('metric_scatter');
    expect(pillarForPresentation('metric_scatter')).toBe('aleph');
  });

  it('declares per-cell configurable params (Phase 131)', () => {
    const byId = Object.fromEntries(listPresentations().map((p) => [p.id, p]));
    expect(byId['distribution']?.configurableParams).toContain('bins');
    expect(byId['time_series']?.configurableParams).toContain('band');
    expect(byId['cooccurrence_network']?.configurableParams).toContain('topN');
    expect(byId['cooccurrence_network']?.configurableParams).toContain('networkChannels');
    expect(byId['metric_scatter']?.configurableParams).toContain('scatterAxes');
    // The scatter cell ignores the single-metric picker (channels drive it).
    expect(byId['metric_scatter']?.usesMetric).toBe(false);
  });

  it('marks overlay as supported only on the time-series cell (Phase 131 bugfix)', () => {
    const byId = Object.fromEntries(listPresentations().map((p) => [p.id, p]));
    expect(byId['time_series']?.supportsOverlay).toBe(true);
    expect(byId['distribution']?.supportsOverlay ?? false).toBe(false);
    expect(byId['topic_distribution']?.supportsOverlay ?? false).toBe(false);
    expect(byId['cooccurrence_network']?.supportsOverlay ?? false).toBe(false);
    expect(byId['metric_scatter']?.supportsOverlay ?? false).toBe(false);
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

  it('flags exactly the view_modes that ship per-metric methodology content', () => {
    // True for the metric-bearing views (a `<view>_<metric>.yaml` exists)…
    for (const v of [
      'distribution',
      'time_series',
      'topic_distribution',
      'topic_evolution',
      'cooccurrence_network'
    ] as const) {
      expect(hasCellMethodologyContent(v)).toBe(true);
    }
    // …false for channel-driven / metric-agnostic views (no such entry → would
    // 404 if fetched). metric_scatter is the reported regression.
    for (const v of [
      'metric_scatter',
      'categorical_distribution',
      'correlation_matrix',
      'cross_tab',
      'parallel_coordinates',
      'metric_lead_lag',
      'sankey',
      'cross_probe_lead_lag',
      'revision_activity',
      'revision_timeline',
      'revision_discourse_shift',
      'revision_edit_clusters'
    ] as const) {
      expect(hasCellMethodologyContent(v)).toBe(false);
    }
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
    // Aleph (synchronic): distribution + topic_distribution + metric_scatter (Phase 131)
    //                   + revision_activity (Phase 122d.0 / ADR-032 — silent-edit snapshot)
    expect(getPillar('aleph').presentations).toEqual([
      'distribution',
      'topic_distribution',
      'metric_scatter',
      'categorical_distribution',
      'correlation_matrix',
      'cross_tab',
      'parallel_coordinates',
      'revision_activity'
    ]);
    // Episteme (diachronic): time_series + topic_evolution
    //                      + revision_timeline (Phase 122d.0 — silent-edit over time)
    //                      + revision_discourse_shift (Phase 122d.3 — discourse trajectory)
    expect(getPillar('episteme').presentations).toEqual([
      'time_series',
      'topic_evolution',
      'revision_timeline',
      'revision_discourse_shift'
    ]);
    // Rhizome (relational): cooccurrence_network + cross_probe_lead_lag
    //                     + revision_edit_clusters (Phase 122d.3 — coordinated edits)
    expect(getPillar('rhizome').presentations).toEqual([
      'cooccurrence_network',
      'cross_probe_lead_lag',
      'metric_lead_lag',
      'sankey',
      'revision_edit_clusters'
    ]);
  });

  it('falls back to Aleph for unknown/null pillar', () => {
    expect(getPillar(null).id).toBe('aleph');
  });

  it('returns the right presentations per pillar', () => {
    const alephIds = presentationsForPillar('aleph').map((p) => p.id);
    expect(alephIds).toEqual([
      'distribution',
      'topic_distribution',
      'metric_scatter',
      'categorical_distribution',
      'correlation_matrix',
      'cross_tab',
      'parallel_coordinates',
      'revision_activity'
    ]);

    const epistemIds = presentationsForPillar('episteme').map((p) => p.id);
    expect(epistemIds).toEqual([
      'time_series',
      'topic_evolution',
      'revision_timeline',
      'revision_discourse_shift'
    ]);

    const rhizIds = presentationsForPillar('rhizome').map((p) => p.id);
    expect(rhizIds).toEqual([
      'cooccurrence_network',
      'cross_probe_lead_lag',
      'metric_lead_lag',
      'sankey',
      'revision_edit_clusters'
    ]);

    // null → Aleph default
    expect(presentationsForPillar(null).map((p) => p.id)).toEqual([
      'distribution',
      'topic_distribution',
      'metric_scatter',
      'categorical_distribution',
      'correlation_matrix',
      'cross_tab',
      'parallel_coordinates',
      'revision_activity'
    ]);
  });

  it('returns the first presentation as the pillar default', () => {
    expect(defaultPresentationForPillar('aleph')).toBe('distribution');
    expect(defaultPresentationForPillar('episteme')).toBe('time_series');
    expect(defaultPresentationForPillar('rhizome')).toBe('cooccurrence_network');
    expect(defaultPresentationForPillar(null)).toBe('distribution');
  });

  it('reverse-maps every presentation back to exactly one pillar', () => {
    expect(pillarForPresentation('time_series')).toBe('episteme');
    expect(pillarForPresentation('distribution')).toBe('aleph');
    expect(pillarForPresentation('topic_distribution')).toBe('aleph');
    expect(pillarForPresentation('topic_evolution')).toBe('episteme');
    expect(pillarForPresentation('cooccurrence_network')).toBe('rhizome');
    // Phase 122d.0 / ADR-032 — silent-edit observability.
    expect(pillarForPresentation('revision_activity')).toBe('aleph');
    expect(pillarForPresentation('revision_timeline')).toBe('episteme');
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
