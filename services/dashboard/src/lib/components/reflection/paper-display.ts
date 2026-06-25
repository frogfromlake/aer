// Localized display strings for the Working-Paper catalog (Phase 144).
//
// `papers.ts` keeps the STRUCTURAL metadata (id, dates, dependency graph,
// interactive cells) plus English fallback labels used by node tests. The
// user-visible title / abstract / status are localized here, keyed by paper id,
// so a language switch relabels the landing + open-questions catalog without
// touching the structural data file or its tests. WP numbers/anchors stay emic.
import { m } from '$lib/paraglide/messages.js';

/** Localized short title for a paper id (e.g. 'wp-001'); falls back to id. */
export function paperShortTitle(id: string): string {
  switch (id.toLowerCase()) {
    case 'wp-001':
      return m.reflection_paper_wp_001_short_title();
    case 'wp-002':
      return m.reflection_paper_wp_002_short_title();
    case 'wp-003':
      return m.reflection_paper_wp_003_short_title();
    case 'wp-004':
      return m.reflection_paper_wp_004_short_title();
    case 'wp-005':
      return m.reflection_paper_wp_005_short_title();
    case 'wp-006':
      return m.reflection_paper_wp_006_short_title();
    case 'wp-007':
      return m.reflection_paper_wp_007_short_title();
    default:
      return id.toUpperCase();
  }
}

/** Localized 1–2 sentence abstract for a paper id; falls back to ''. */
export function paperAbstract(id: string): string {
  switch (id.toLowerCase()) {
    case 'wp-001':
      return m.reflection_paper_wp_001_abstract();
    case 'wp-002':
      return m.reflection_paper_wp_002_abstract();
    case 'wp-003':
      return m.reflection_paper_wp_003_abstract();
    case 'wp-004':
      return m.reflection_paper_wp_004_abstract();
    case 'wp-005':
      return m.reflection_paper_wp_005_abstract();
    case 'wp-006':
      return m.reflection_paper_wp_006_abstract();
    case 'wp-007':
      return m.reflection_paper_wp_007_abstract();
    default:
      return '';
  }
}

/** Localized review status for a paper id (wp-001 is the only v2 draft). */
export function paperStatus(id: string): string {
  return id.toLowerCase() === 'wp-001'
    ? m.reflection_paper_status_draft_v2()
    : m.reflection_paper_status_draft();
}
