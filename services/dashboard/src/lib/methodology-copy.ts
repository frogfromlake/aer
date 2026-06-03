// methodology-copy.ts — Phase 122j J2.
//
// Centralised source of truth for soft methodology-note banner copy. The
// banners are shown above Cell content when a methodological caveat
// applies but does not warrant a refusal (joint corpus, small corpus,
// cross-source comparability). Before Phase 122j the copy was inlined
// across four Cell components; the four payloads have now been
// consolidated here so the next iteration can promote them to the BFF
// `/content/methodology_note/{id}` endpoint without touching the
// rendering sites.
//
// Each note is a pure function `(ctx) => { headline, body, anchorHref,
// anchorLabel }`. Dynamic counts are injected via the `ctx` argument so
// the static portion stays auditable and the dynamic portion stays at
// render time (Svelte 5 reactivity).
//
// The full methodological reasoning is anchored in the working papers:
//   WP-004 §3.4   — cross-frame comparability (Aleph)
//   WP-005 §6.2   — joint-corpus + small-corpus (Episteme)
// The catalog entries under `services/bff-api/configs/content/{en,de}/`
// carry the dual-register methodological+semantic copy for the
// underlying view-modes; the banners here are the user-facing surface.
// English-only per CLAUDE.md "All code, comments, documentation, API
// contracts, and commit messages: English only".

export interface MethodologyNote {
  headline: string;
  body: string;
  anchorHref: string;
  anchorLabel: string;
}

const WP_004_34: Pick<MethodologyNote, 'anchorHref' | 'anchorLabel'> = {
  anchorHref: '/reflection/wp/wp-004?section=3.4',
  anchorLabel: 'WP-004 §3.4'
};

const WP_005_62: Pick<MethodologyNote, 'anchorHref' | 'anchorLabel'> = {
  anchorHref: '/reflection/wp/wp-005?section=6.2',
  anchorLabel: 'WP-005 §6.2'
};

// Phase 124 — temporal Level-1 equivalence grant (cross-probe lead-lag).
const WP_004_APPB: Pick<MethodologyNote, 'anchorHref' | 'anchorLabel'> = {
  anchorHref: '/reflection/wp/wp-004?section=appendix-b',
  anchorLabel: 'WP-004 Appendix B'
};

export const methodologyNotes = {
  // Aleph — merged time-series over multiple sources.
  alephMergedTimeSeries: (sourceCount: number): MethodologyNote => ({
    headline: `Merged across ${sourceCount} sources`,
    body: 'the time series reflects the joint corpus, not per-source framings. Interpret cross-source comparability cautiously, especially when sources span multiple cultural-linguistic frames.',
    ...WP_004_34
  }),

  // Aleph — merged distribution over multiple sources.
  alephMergedDistribution: (sourceCount: number): MethodologyNote => ({
    headline: `Distribution across ${sourceCount} merged sources`,
    body: 'the histogram aggregates the joint corpus, not per-source distributions. Per-source comparability may be affected by source heterogeneity, especially across cultural-linguistic frames.',
    ...WP_004_34
  }),

  // Episteme — BERTopic joint-corpus over multiple sources (distribution).
  epistemeJointCorpusDistribution: (sourceCount: number): MethodologyNote => ({
    headline: `BERTopic across ${sourceCount} sources`,
    body: 'topics reflect the joint corpus, not per-source framings. Source-specific framings may be aggregated away.',
    ...WP_005_62
  }),

  // Episteme — BERTopic joint-corpus over multiple sources (evolution stream).
  epistemeJointCorpusEvolution: (sourceCount: number): MethodologyNote => ({
    headline: `BERTopic stream across ${sourceCount} sources`,
    body: 'streams aggregate the joint corpus, not per-source framings. Source-specific framings may be aggregated away.',
    ...WP_005_62
  }),

  // Episteme — small-corpus warning (article count below stability threshold).
  epistemeSmallCorpus: (articleCount: number, threshold: number): MethodologyNote => ({
    headline: `Small corpus (${articleCount} articles, < ${threshold})`,
    body: 'BERTopic rarely converges on a coherent topic set below this threshold. Interpret topics cautiously.',
    ...WP_005_62
  }),

  // Rhizome — cross-probe temporal lead-lag (Phase 124). The grant level
  // comes from the BFF response; the note states why this comparison is
  // admissible and where its boundary lies.
  rhizomeLeadLagGrant: (level: string): MethodologyNote => ({
    headline: `Temporal ${level === 'temporal' ? 'Level-1' : level} grant`,
    body: 'publication timing is measured on clock/calendar time — a culture-independent axis — so comparing the two probes’ rhythm is valid given verified DE/FR calendar parity. This is a when-comparison only; it asserts nothing about how much or how positive.',
    ...WP_004_APPB
  })
} as const;

export type MethodologyNoteId = keyof typeof methodologyNotes;
