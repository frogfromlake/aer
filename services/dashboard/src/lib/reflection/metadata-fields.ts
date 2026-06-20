// Task C — static tier catalogue + pure status classifier for the Reflection
// "metadata fields" surface. Mirrors the worker's tier split
// (analysis-worker/internal/metadata_coverage.py TIER_B_FIELDS / TIER_C_FIELDS);
// keep in order-sync with it. The tier + the per-field prose are curation, so
// they live client-side; the live fill measurements come from GET /metadata-fields.
// Pure (no Paraglide import) so node unit tests can exercise the classifier.

import type { MetadataFieldStatDto } from '$lib/api/queries';

export type FieldTier = 'B' | 'C';

// Tier-B: common publisher metadata most news sites declare in structured form.
export const TIER_B_FIELDS: readonly string[] = [
  'published_date',
  'modified_date',
  'author',
  'description',
  'categories',
  'tags',
  'section',
  'image_url',
  'article_type',
  'word_count'
];

// Tier-C: specialised / rarely-declared fields (many need a custom heuristic
// extractor beyond generic structured-metadata harvesting).
export const TIER_C_FIELDS: readonly string[] = [
  'comment_count',
  'comment_url',
  'editor',
  'reading_time_minutes',
  'dateline_location',
  'paywall_status',
  'correction_notice',
  'editorial_labels',
  'external_citations',
  'images',
  'social_share_counts',
  'revision_date'
];

export const ALL_METADATA_FIELDS: readonly string[] = [...TIER_B_FIELDS, ...TIER_C_FIELDS];

export function tierOf(field: string): FieldTier {
  return TIER_C_FIELDS.includes(field) ? 'C' : 'B';
}

// The corpus-wide status of a field, derived purely from its live stat.
//   unobserved — never seen in any source's coverage (no column at all)
//   absent     — observed but populated on 0 articles (structural absence everywhere)
//   constant   — populated but a single distinct value across the corpus (no signal)
//   partial    — populated on some but not most observed articles
//   populated  — populated on (nearly) every observed article
export type FieldStatus = 'unobserved' | 'absent' | 'constant' | 'partial' | 'populated';

// A field counts as fully "populated" at/above this corpus fill rate. Below it
// (but above zero) reads as "partial". A disclosed display threshold, not a gate.
export const POPULATED_THRESHOLD = 0.95;

export function classifyFieldStatus(stat: MetadataFieldStatDto | undefined): FieldStatus {
  if (!stat || stat.totalArticles === 0) return 'unobserved';
  if (stat.populatedArticles === 0) return 'absent';
  if (stat.constant) return 'constant';
  if (stat.populationRate >= POPULATED_THRESHOLD) return 'populated';
  return 'partial';
}

export function populationPct(stat: MetadataFieldStatDto | undefined): number {
  if (!stat) return 0;
  return Math.round(stat.populationRate * 100);
}
