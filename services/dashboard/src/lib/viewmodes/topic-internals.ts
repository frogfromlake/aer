// Pure helpers shared between the Phase 121 topic cells. Kept rune-free
// in their own module so vitest can import them without a Svelte
// compiler pass; the cell components consume them directly.

import type { TopicDistributionEntryDto } from '$lib/api/queries';

/** Outlier topic id from BERTopic (the "no coherent cluster" class). */
export const OUTLIER_TOPIC_ID = -1;

/** Label that replaces the BFF outlier representation in the dashboard.
 *  Per Phase 121 the outlier is rendered as a greyed "uncategorised"
 *  ridge rather than hidden — an outlier is an observation, not an
 *  error (Brief §7.7 / absence-as-data). */
export const UNCATEGORISED_LABEL = 'uncategorised';

export interface NormalisedTopic {
  topicId: number;
  /** Display label — outlier ids are relabelled to UNCATEGORISED_LABEL. */
  label: string;
  /** Falls back to `'und'` when the BFF reports an empty language code. */
  language: string;
  articleCount: number;
  avgConfidence: number;
  isOutlier: boolean;
  /** 0 = highest-volume topic in its language partition. Outliers are
   *  always pinned to the bottom of their partition (highest ridge index). */
  ridge: number;
}

/** Normalise a flat list of TopicDistributionEntry rows from the BFF
 *  into the dashboard's display shape: outliers relabelled, language
 *  fallback applied, sorted within partitions, and ridge index assigned.
 *  Pure — safe to call inside `$derived.by` or vitest. */
export function normaliseTopics(entries: readonly TopicDistributionEntryDto[]): NormalisedTopic[] {
  const byLang = new Map<string, NormalisedTopic[]>();
  for (const t of entries) {
    const isOutlier = t.topicId === OUTLIER_TOPIC_ID;
    const language = t.language || 'und';
    const norm: NormalisedTopic = {
      topicId: t.topicId,
      label: isOutlier ? UNCATEGORISED_LABEL : t.label,
      language,
      articleCount: t.articleCount,
      avgConfidence: t.avgConfidence,
      isOutlier,
      ridge: 0
    };
    const arr = byLang.get(language) ?? [];
    arr.push(norm);
    byLang.set(language, arr);
  }
  const out: NormalisedTopic[] = [];
  for (const arr of byLang.values()) {
    arr.sort((a, b) => {
      // Outlier always last within its partition.
      if (a.isOutlier !== b.isOutlier) return a.isOutlier ? 1 : -1;
      return b.articleCount - a.articleCount;
    });
    arr.forEach((r, idx) => (r.ridge = idx));
    out.push(...arr);
  }
  return out;
}

/** Distinct language partitions present in a normalised result, in
 *  first-encounter order. Used by both topic cells to decide whether to
 *  facet the plot (multi-language ⇒ one stack per language partition,
 *  no cross-language alignment per WP-004 §3.4). */
export function languagesOf(rows: readonly NormalisedTopic[]): string[] {
  const seen: string[] = [];
  for (const r of rows) {
    if (!seen.includes(r.language)) seen.push(r.language);
  }
  return seen;
}
