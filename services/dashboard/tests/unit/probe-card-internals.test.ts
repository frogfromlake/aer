import { describe, expect, it } from 'vitest';

import {
  groupSourcesByFunction,
  unclassifiedSources,
  coveredFunctions,
  publicationRatePerDay,
  coveragePercent
} from '../../src/lib/components/dossier/probe-card-internals';
import type { ProbeDossierSourceDto } from '../../src/lib/api/queries';

function src(over: Partial<ProbeDossierSourceDto>): ProbeDossierSourceDto {
  return {
    name: 'x',
    primaryFunction: null,
    publicationFrequencyPerDay: 0,
    ...over
  } as ProbeDossierSourceDto;
}

describe('groupSourcesByFunction', () => {
  it('buckets sources under their primary function', () => {
    const grouped = groupSourcesByFunction([
      src({ name: 'a', primaryFunction: 'epistemic_authority' }),
      src({ name: 'b', primaryFunction: 'power_legitimation' }),
      src({ name: 'c', primaryFunction: 'epistemic_authority' })
    ]);
    expect(grouped.epistemic_authority.map((s) => s.name)).toEqual(['a', 'c']);
    expect(grouped.power_legitimation.map((s) => s.name)).toEqual(['b']);
    expect(grouped.cohesion_identity).toEqual([]);
    expect(grouped.subversion_friction).toEqual([]);
  });

  it('excludes sources with no / unknown primary function', () => {
    const grouped = groupSourcesByFunction([
      src({ name: 'none', primaryFunction: null }),
      src({ name: 'weird', primaryFunction: 'not_a_function' as never })
    ]);
    expect(grouped.epistemic_authority).toEqual([]);
    expect(grouped.power_legitimation).toEqual([]);
  });
});

describe('unclassifiedSources', () => {
  it('returns only sources without a primary function', () => {
    const out = unclassifiedSources([
      src({ name: 'classified', primaryFunction: 'cohesion_identity' }),
      src({ name: 'u1', primaryFunction: null }),
      src({ name: 'u2', primaryFunction: undefined as never })
    ]);
    expect(out.map((s) => s.name)).toEqual(['u1', 'u2']);
  });
});

describe('coveredFunctions', () => {
  it('is the set of functions with at least one source', () => {
    const grouped = groupSourcesByFunction([
      src({ name: 'a', primaryFunction: 'epistemic_authority' }),
      src({ name: 'b', primaryFunction: 'subversion_friction' })
    ]);
    const covered = coveredFunctions(grouped);
    expect(covered.has('epistemic_authority')).toBe(true);
    expect(covered.has('subversion_friction')).toBe(true);
    expect(covered.has('power_legitimation')).toBe(false);
    expect(covered.size).toBe(2);
  });
});

describe('publicationRatePerDay', () => {
  it('sums the per-source rates', () => {
    expect(
      publicationRatePerDay([
        src({ publicationFrequencyPerDay: 5 }),
        src({ publicationFrequencyPerDay: 1.5 })
      ])
    ).toBe(6.5);
  });

  it('returns null when no source reports a rate', () => {
    expect(publicationRatePerDay([src({ publicationFrequencyPerDay: 0 })])).toBeNull();
    expect(publicationRatePerDay([])).toBeNull();
  });
});

describe('coveragePercent', () => {
  it('rounds covered/total to a percentage', () => {
    expect(coveragePercent({ covered: 2, total: 4 })).toBe(50);
    expect(coveragePercent({ covered: 1, total: 3 })).toBe(33);
  });

  it('returns 0 when total is 0 (no divide-by-zero)', () => {
    expect(coveragePercent({ covered: 0, total: 0 })).toBe(0);
  });
});
