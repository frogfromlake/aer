import { describe, expect, it } from 'vitest';

import { methodologyNotes } from '../../src/lib/methodology-copy';

// Phase 122j J2 / 142 — the soft methodology-note banner copy is the SoT for
// the joint-corpus / small-corpus / cross-frame caveat banners. These tests pin
// the dynamic-count injection + the working-paper anchors so a copy refactor
// cannot silently drop the methodological attribution.

describe('alephMergedTimeSeries', () => {
  it('injects the source count and anchors WP-004 §3.4', () => {
    const n = methodologyNotes.alephMergedTimeSeries(3);
    expect(n.headline).toBe('Merged across 3 sources');
    expect(n.body).toContain('joint corpus');
    expect(n.anchorHref).toBe('/reflection/wp/wp-004?section=3.4');
    expect(n.anchorLabel).toBe('WP-004 §3.4');
  });
});

describe('alephMergedDistribution', () => {
  it('injects the count and anchors WP-004 §3.4', () => {
    const n = methodologyNotes.alephMergedDistribution(2);
    expect(n.headline).toBe('Distribution across 2 merged sources');
    expect(n.anchorLabel).toBe('WP-004 §3.4');
  });
});

describe('episteme joint-corpus notes', () => {
  it('distribution variant injects the count and anchors WP-005 §6.2', () => {
    const n = methodologyNotes.epistemeJointCorpusDistribution(4);
    expect(n.headline).toBe('BERTopic across 4 sources');
    expect(n.anchorHref).toBe('/reflection/wp/wp-005?section=6.2');
  });

  it('evolution variant injects the count and anchors WP-005 §6.2', () => {
    const n = methodologyNotes.epistemeJointCorpusEvolution(4);
    expect(n.headline).toBe('BERTopic stream across 4 sources');
    expect(n.anchorLabel).toBe('WP-005 §6.2');
  });
});

describe('epistemeSmallCorpus', () => {
  it('names both the actual count and the threshold', () => {
    const n = methodologyNotes.epistemeSmallCorpus(120, 500);
    expect(n.headline).toBe('Small corpus (120 articles, < 500)');
    expect(n.body).toContain('rarely converges');
  });
});

describe('rhizomeLeadLagGrant', () => {
  it('renders "Level-1" for the temporal grant level', () => {
    const n = methodologyNotes.rhizomeLeadLagGrant('temporal');
    expect(n.headline).toBe('Temporal Level-1 grant');
    expect(n.anchorLabel).toBe('WP-004 Appendix B');
  });

  it('passes a non-temporal level through verbatim', () => {
    const n = methodologyNotes.rhizomeLeadLagGrant('deviation');
    expect(n.headline).toBe('Temporal deviation grant');
  });
});
