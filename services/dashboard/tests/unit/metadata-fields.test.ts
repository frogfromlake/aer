import { describe, it, expect } from 'vitest';
import {
  TIER_B_FIELDS,
  TIER_C_FIELDS,
  ALL_METADATA_FIELDS,
  tierOf,
  classifyFieldStatus,
  populationPct
} from '../../src/lib/reflection/metadata-fields';
import type { MetadataFieldStatDto } from '../../src/lib/api/queries';

const stat = (over: Partial<MetadataFieldStatDto> = {}): MetadataFieldStatDto => ({
  field: 'x',
  totalArticles: 100,
  populatedArticles: 100,
  populationRate: 1,
  sourcesObserved: 2,
  sourcesPopulated: 2,
  distinctValues: 5,
  constant: false,
  ...over
});

describe('metadata-fields catalogue', () => {
  it('mirrors the worker tier split: 10 Tier-B + 12 Tier-C = 22', () => {
    expect(TIER_B_FIELDS).toHaveLength(10);
    expect(TIER_C_FIELDS).toHaveLength(12);
    expect(ALL_METADATA_FIELDS).toHaveLength(22);
  });

  it('tierOf classifies known fields', () => {
    expect(tierOf('article_type')).toBe('B');
    expect(tierOf('paywall_status')).toBe('C');
  });
});

describe('classifyFieldStatus', () => {
  it('unobserved when there is no stat or nothing was observed', () => {
    expect(classifyFieldStatus(undefined)).toBe('unobserved');
    expect(classifyFieldStatus(stat({ totalArticles: 0 }))).toBe('unobserved');
  });

  it('absent when observed but never populated (structural absence everywhere)', () => {
    expect(classifyFieldStatus(stat({ populatedArticles: 0, populationRate: 0 }))).toBe('absent');
  });

  it('constant takes precedence over a high fill rate', () => {
    expect(classifyFieldStatus(stat({ constant: true, distinctValues: 1 }))).toBe('constant');
  });

  it('populated at/above threshold, partial below', () => {
    expect(classifyFieldStatus(stat({ populationRate: 1 }))).toBe('populated');
    expect(classifyFieldStatus(stat({ populationRate: 0.96 }))).toBe('populated');
    expect(classifyFieldStatus(stat({ populationRate: 0.6 }))).toBe('partial');
  });

  it('populationPct rounds the 0..1 rate', () => {
    expect(populationPct(stat({ populationRate: 0.667 }))).toBe(67);
    expect(populationPct(undefined)).toBe(0);
  });
});
