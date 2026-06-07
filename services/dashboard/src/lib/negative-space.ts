// Negative-Space single source of truth (Phase 122d.2, ADR-039).
//
// "Negative Space" is what AĒR does NOT see / cannot analyse. The methodology
// (WP-001/003/004/005/006) recognises several distinct classes; this module is
// the ONE place that names them, carries their methodological prose + WP anchor,
// and classifies a row. Consumers display via `NegativeSpaceBadge`
// (`$lib/components/base/NegativeSpaceBadge.svelte`) — they never re-implement
// the vocabulary or the register.
//
// INVARIANTS (codified from the Working Papers — see ADR-039):
//   • DISCLOSE, NEVER COERCE — absence is made legible, never set to 0.
//   • PUBLISHER CHOICE, NOT SOURCE DEFECT — prose is methodological, never
//     "source X is broken" (WP-003 §3.2).
//   • METHODOLOGICAL REGISTER, NOT WARNING — colours are perceptually neutral
//     dim, never red/error (WP-006 §6.2).

export const NS_CLASSES = [
  'structural_metadata_absence',
  'temporal_provenance_absence',
  'silent_edit',
  'analytical_capability_absence',
  'k_anonymity_suppression',
  'equivalence_refusal'
] as const;

export type NSClass = (typeof NS_CLASSES)[number];

export interface NSClassDef {
  /** URL-stable key. */
  key: NSClass;
  /** Short abbreviation for the badge. */
  abbr: string;
  /** Full human-readable label. */
  label: string;
  /** Perceptually-neutral dim accent (NEVER red/warning — WP-006 §6.2). */
  color: string;
  /** Methodological-register one-liner (publisher-choice, never "broken"). */
  description: string;
  /** Reflection-route anchor to the governing Working-Paper section. */
  wpAnchor: string;
  /** Granularity at which this class is detected: per-article, or
   *  source-/scope-level (surfaced by source-cards / refusals / scope notes,
   *  not the per-row classifier). */
  scope: 'article' | 'source' | 'query';
}

export const NS_CLASS_DEFINITIONS: Record<NSClass, NSClassDef> = {
  structural_metadata_absence: {
    key: 'structural_metadata_absence',
    abbr: 'SMA',
    label: 'Structural Metadata Absence',
    color: '#8a8f98',
    description:
      'The publisher does not emit this metadata field — absent by design, a publisher choice (WP-003 §3.2).',
    wpAnchor: '/reflection/wp/wp-003?section=3.2',
    scope: 'source'
  },
  temporal_provenance_absence: {
    key: 'temporal_provenance_absence',
    abbr: 'TPA',
    label: 'Temporal Provenance Absence',
    color: '#9a8f7a',
    description:
      'The timestamp is the crawler fetch time, not a real publication date — an observation gap, not a discourse gap (WP-005 §3.1).',
    wpAnchor: '/reflection/wp/wp-005?section=3.1',
    scope: 'article'
  },
  silent_edit: {
    key: 'silent_edit',
    abbr: 'SE',
    label: 'Silent Edit',
    color: '#8f9a8a',
    description:
      'The article was edited after publication (headline change / republication / archive gap) — the visible text may not be the original (WP-003 §5.3.1).',
    wpAnchor: '/reflection/wp/wp-003?section=5.3',
    scope: 'article'
  },
  analytical_capability_absence: {
    key: 'analytical_capability_absence',
    abbr: 'ACA',
    label: 'Analytical Capability Absence',
    color: '#7a8a9a',
    description:
      "The active scope's language has no NER / sentiment backbone, so this question is structurally unanswerable (WP-002 / WP-004).",
    wpAnchor: '/reflection/wp/wp-004',
    scope: 'query'
  },
  k_anonymity_suppression: {
    key: 'k_anonymity_suppression',
    abbr: 'KA',
    label: 'k-Anonymity Suppression',
    color: '#9a8a9a',
    description:
      'Data exists but is withheld to protect k-anonymity — an ethical choice, distinct from publisher omission (WP-006 §7.2.2).',
    wpAnchor: '/reflection/wp/wp-006?section=7.2.2',
    scope: 'query'
  },
  equivalence_refusal: {
    key: 'equivalence_refusal',
    abbr: 'ER',
    label: 'Equivalence Refusal',
    color: '#8a9a9a',
    description:
      'Cross-frame normalisation was requested without a granted metric equivalence — refused rather than coerced (WP-004 §5.3).',
    wpAnchor: '/reflection/wp/wp-004?section=5.3',
    scope: 'query'
  }
};

// ── Per-cell rendering policy (ADR-039) ───────────────────────────────────────
//
// How a Workbench cell treats the Negative-Space toggle. The per-article
// data-bearing renderings live in the article list (∅ badges), the source
// dossier, the co-occurrence ghost overlay and the globe; aggregate cells whose
// DTOs carry no per-article NS marker SELF-DISCLOSE via these notes rather than
// faking data (DISCLOSE-NEVER-COERCE). Every note carries a WP anchor.

export type NSPolicy = 'overlay' | 'badge' | 'gap' | 'refuse' | 'no-op';

export const NS_POLICY_NOTES: Record<NSPolicy, string> = {
  overlay:
    'Negative Space: when the toggle is on, the artefacts excluded as methodologically meaningless are ghost-rendered here (dim) — shown for disclosure, not re-admitted to the counts (WP-005 §3.1).',
  gap: 'Negative Space: intervals dominated by unobserved articles break the line rather than read as zero — an observation gap is not a discourse gap (WP-005 §3.1).',
  badge:
    'Negative Space: per-article markers (∅) appear in the article list and the source dossier; this aggregate view never coerces an absent value to zero (WP-003 §3.2).',
  refuse:
    'Negative Space: an aggregate over a structurally-absent field is refused here rather than coerced to a zero (WP-003 §3.2).',
  'no-op':
    'Negative Space has no per-article rendering for this view — the NS-classes do not change its structure. See the article list (∅) and the source dossier (WP-006 §4.2).'
};

export function nsPolicyNote(policy: string | null | undefined): string {
  return NS_POLICY_NOTES[(policy as NSPolicy) ?? 'no-op'] ?? NS_POLICY_NOTES['no-op'];
}

/** Type-safe lookup with an unknown-key escape valve (mirrors getFunctionDef). */
export function getNSClassDef(key: string | null | undefined): NSClassDef | null {
  if (!key) return null;
  return NS_CLASS_DEFINITIONS[key as NSClass] ?? null;
}

/** Ordered list for legends / breakdown rows. */
export const NS_CLASS_DEFINITIONS_ORDERED: readonly NSClassDef[] = NS_CLASSES.map(
  (key) => NS_CLASS_DEFINITIONS[key]
);

/** The per-article inputs the row classifier reads. A structural subset so the
 *  article-list DTO (`ArticleListItem`) and the revision DTO both fit. */
export interface NSRow {
  timestampSource?: string | null | undefined;
  hasHeadlineChange?: boolean | null | undefined;
}

/**
 * classifyNegativeSpace returns the PER-ARTICLE NS-classes a row belongs to (a
 * row can belong to several). It covers the two article-granular classes:
 *   • Temporal-Provenance-Absence — `timestampSource === 'fetch_at_fallback'`
 *     (the methodologically-honest "no real date" marker; empty/other = a real
 *     or legacy date, NOT this class).
 *   • Silent-Edit — `hasHeadlineChange === true` (the per-row signal available
 *     in the list DTO; finer silent-edit signals — republication trigger,
 *     Wayback lookup failure — surface in the L5 reader's NS-section from the
 *     per-revision query, not the list row).
 *
 * The source-/query-level classes (structural-metadata, analytical-capability,
 * k-anonymity, equivalence-refusal) are NOT row-derivable; they are surfaced by
 * their own mechanisms (metadata-coverage, capability gate, refusal surfaces).
 */
export function classifyNegativeSpace(row: NSRow): NSClass[] {
  const out: NSClass[] = [];
  if (row.timestampSource === 'fetch_at_fallback') out.push('temporal_provenance_absence');
  if (row.hasHeadlineChange === true) out.push('silent_edit');
  return out;
}
