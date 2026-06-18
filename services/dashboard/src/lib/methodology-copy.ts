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
// Phase 144b / ADR-042 — the headline + body strings are localized via Paraglide
// (`messages/{en,de}/domain.json`, `domain_methnote_*`); the dynamic counts ride
// in as message parameters. `m.*()` reads the UI-locale rune per call, so the
// banner re-renders on a language switch with no call-site change. The relative
// import keeps the module node-safe (Vitest does not resolve `$lib` at runtime).
// The WP anchors stay code-side here (locale-independent).
//
// The full methodological reasoning is anchored in the working papers:
//   WP-004 §3.4   — cross-frame comparability (Aleph)
//   WP-005 §6.2   — joint-corpus + small-corpus (Episteme)
// The catalog entries under `services/bff-api/configs/content/{en,de}/`
// carry the dual-register methodological+semantic copy for the
// underlying view-modes; the banners here are the user-facing surface.
import { m } from './paraglide/messages.js';

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
    headline: m.domain_methnote_aleph_merged_ts_headline({ count: sourceCount }),
    body: m.domain_methnote_aleph_merged_ts_body(),
    ...WP_004_34
  }),

  // Aleph — merged distribution over multiple sources.
  alephMergedDistribution: (sourceCount: number): MethodologyNote => ({
    headline: m.domain_methnote_aleph_merged_dist_headline({ count: sourceCount }),
    body: m.domain_methnote_aleph_merged_dist_body(),
    ...WP_004_34
  }),

  // Episteme — BERTopic joint-corpus over multiple sources (distribution).
  epistemeJointCorpusDistribution: (sourceCount: number): MethodologyNote => ({
    headline: m.domain_methnote_episteme_jc_dist_headline({ count: sourceCount }),
    body: m.domain_methnote_episteme_jc_dist_body(),
    ...WP_005_62
  }),

  // Episteme — BERTopic joint-corpus over multiple sources (evolution stream).
  epistemeJointCorpusEvolution: (sourceCount: number): MethodologyNote => ({
    headline: m.domain_methnote_episteme_jc_evo_headline({ count: sourceCount }),
    body: m.domain_methnote_episteme_jc_evo_body(),
    ...WP_005_62
  }),

  // Episteme — small-corpus warning (article count below stability threshold).
  epistemeSmallCorpus: (articleCount: number, threshold: number): MethodologyNote => ({
    headline: m.domain_methnote_episteme_small_corpus_headline({
      count: articleCount,
      threshold
    }),
    body: m.domain_methnote_episteme_small_corpus_body(),
    ...WP_005_62
  }),

  // Rhizome — cross-probe temporal lead-lag (Phase 124). The grant level
  // comes from the BFF response; the note states why this comparison is
  // admissible and where its boundary lies. The displayed level is computed
  // here (temporal → "Level-1") and injected as a message parameter.
  rhizomeLeadLagGrant: (level: string): MethodologyNote => ({
    headline: m.domain_methnote_rhizome_leadlag_headline({
      level: level === 'temporal' ? 'Level-1' : level
    }),
    body: m.domain_methnote_rhizome_leadlag_body(),
    ...WP_004_APPB
  })
} as const;

export type MethodologyNoteId = keyof typeof methodologyNotes;
