import { describe, expect, it } from 'vitest';

import {
  pickBadgeTier,
  workingPaperHref
} from '../../src/lib/components/chrome/methodology-tray-internals';
import type { components } from '../../src/lib/api/types';

type MetricProvenance = components['schemas']['MetricProvenance'];

function prov(p: Partial<MetricProvenance>): MetricProvenance {
  return {
    metricName: 'sentiment_score',
    tierClassification: 1,
    algorithmDescription: '',
    knownLimitations: [],
    validationStatus: 'unvalidated',
    extractorVersionHash: 'v1',
    ...p
  } as MetricProvenance;
}

describe('pickBadgeTier', () => {
  it('falls back to tier1-unvalidated when provenance is null', () => {
    expect(pickBadgeTier(null)).toBe('tier1-unvalidated');
  });

  it('maps expired validation to the expired badge regardless of tier', () => {
    expect(pickBadgeTier(prov({ tierClassification: 2, validationStatus: 'expired' }))).toBe(
      'expired'
    );
  });

  it('maps tier 1 unvalidated to tier1-unvalidated', () => {
    expect(pickBadgeTier(prov({ tierClassification: 1, validationStatus: 'unvalidated' }))).toBe(
      'tier1-unvalidated'
    );
  });

  it('maps tier 1 validated to tier1-validated', () => {
    expect(pickBadgeTier(prov({ tierClassification: 1, validationStatus: 'validated' }))).toBe(
      'tier1-validated'
    );
  });

  it('maps tier 2 validated to tier2-validated', () => {
    expect(pickBadgeTier(prov({ tierClassification: 2, validationStatus: 'validated' }))).toBe(
      'tier2-validated'
    );
  });

  it('collapses tier 2 unvalidated onto the tier1-unvalidated visual', () => {
    expect(pickBadgeTier(prov({ tierClassification: 2, validationStatus: 'unvalidated' }))).toBe(
      'tier1-unvalidated'
    );
  });

  it('maps tier 3 to tier3 regardless of validation', () => {
    expect(pickBadgeTier(prov({ tierClassification: 3, validationStatus: 'validated' }))).toBe(
      'tier3'
    );
  });
});

describe('workingPaperHref', () => {
  it('returns null on empty input', () => {
    expect(workingPaperHref(null)).toBe(null);
    expect(workingPaperHref(undefined)).toBe(null);
    expect(workingPaperHref('')).toBe(null);
  });

  it('parses "WP-002 §3" into a sectioned deep link', () => {
    expect(workingPaperHref('WP-002 §3')).toBe('/reflection/wp/wp-002?section=3');
  });

  it('encodes multi-character sections', () => {
    expect(workingPaperHref('WP-006 §5.2')).toBe('/reflection/wp/wp-006?section=5.2');
  });

  it('drops the query string when no section is given', () => {
    expect(workingPaperHref('WP-001')).toBe('/reflection/wp/wp-001');
  });

  it('returns null on a non-WP anchor (e.g. a free-form citation)', () => {
    expect(workingPaperHref('Smith 2020')).toBe(null);
  });
});
