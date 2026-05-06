import { describe, expect, it } from 'vitest';

import {
  normaliseTopics,
  languagesOf,
  OUTLIER_TOPIC_ID,
  UNCATEGORISED_LABEL
} from '../../src/lib/viewmodes/topic-internals';
import type { TopicDistributionEntryDto } from '../../src/lib/api/queries';

const topic = (
  id: number,
  label: string,
  language: string,
  articleCount: number,
  avgConfidence = 0.7
): TopicDistributionEntryDto => ({
  topicId: id,
  label,
  language,
  articleCount,
  avgConfidence
});

describe('topic normalisation (Phase 121)', () => {
  it('relabels the BERTopic outlier (topicId=-1) as "uncategorised", does not hide it', () => {
    const out = normaliseTopics([
      topic(0, 'energy', 'de', 12),
      topic(OUTLIER_TOPIC_ID, '-1_misc_words', 'de', 5)
    ]);
    // Outlier survives normalisation — visibility is the renderer's job.
    expect(out.find((r) => r.isOutlier)).toBeDefined();
    expect(out.find((r) => r.isOutlier)?.label).toBe(UNCATEGORISED_LABEL);
    // No row was dropped.
    expect(out.length).toBe(2);
  });

  it('pins the outlier to the bottom of its language partition (highest ridge index)', () => {
    const out = normaliseTopics([
      topic(OUTLIER_TOPIC_ID, '-1', 'de', 50),
      topic(1, 'climate', 'de', 30),
      topic(2, 'inflation', 'de', 40)
    ]);
    const outlier = out.find((r) => r.isOutlier);
    const named = out.filter((r) => !r.isOutlier);
    // Outlier ridge > every named ridge in the same partition.
    for (const n of named) {
      expect(outlier!.ridge).toBeGreaterThan(n.ridge);
    }
  });

  it('sorts named topics by article count descending within a language partition', () => {
    const out = normaliseTopics([
      topic(1, 'a', 'de', 10),
      topic(2, 'b', 'de', 30),
      topic(3, 'c', 'de', 20)
    ]);
    const labels = out.filter((r) => !r.isOutlier).map((r) => r.label);
    expect(labels).toEqual(['b', 'c', 'a']);
  });

  it('keeps language partitions independent — topic ids never share a ridge index across languages', () => {
    const out = normaliseTopics([
      topic(0, 'climate', 'de', 100),
      topic(0, 'climat', 'fr', 80),
      topic(1, 'inflation', 'de', 60)
    ]);
    const de = out.filter((r) => r.language === 'de').map((r) => r.label);
    const fr = out.filter((r) => r.language === 'fr').map((r) => r.label);
    expect(de).toEqual(['climate', 'inflation']);
    expect(fr).toEqual(['climat']);
    // Ridge index 0 exists in both partitions — independent y-scales.
    expect(out.find((r) => r.language === 'de' && r.ridge === 0)?.label).toBe('climate');
    expect(out.find((r) => r.language === 'fr' && r.ridge === 0)?.label).toBe('climat');
  });

  it('falls back to "und" when the BFF reports an empty language code', () => {
    const out = normaliseTopics([topic(0, 'a', '', 5)]);
    expect(out[0]!.language).toBe('und');
  });

  it('languagesOf preserves first-encounter order for stable facet rendering', () => {
    const out = normaliseTopics([
      topic(0, 'a', 'de', 5),
      topic(0, 'a', 'fr', 5),
      topic(1, 'b', 'de', 3)
    ]);
    expect(languagesOf(out)).toEqual(['de', 'fr']);
  });
});
