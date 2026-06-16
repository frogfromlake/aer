import { describe, expect, it } from 'vitest';

import {
  DISCOURSE_FUNCTIONS,
  sourceMatchesDf,
  toggleProbeInGroup,
  toggleSourceInGroup,
  selectAllSourcesInGroup,
  clearSourcesInGroup,
  pruneSourcesToLock,
  resolvePanelLock,
  type DossierSource,
  type SourcesForProbe
} from '../../src/lib/workbench/scope-editor-internals';
import type { ScopeGroup } from '../../src/lib/state/url-internals';
import type { DiscourseFunction } from '../../src/lib/discourse-function';

// Phase 141 — pure cores of the ScopeEditor draft mutations, lifted out of
// the component during its decomposition. The component just owns the
// `$state` and delegates to these; covering them here is the markup-net's
// companion (the e2e pins the rendered cards, these pin the edit semantics).

// A tiny structural source factory — the resolver only reads `name` +
// `primaryFunction`, so a partial cast keeps the fixtures legible.
function src(name: string, primaryFunction: DiscourseFunction | null): DossierSource {
  return { name, primaryFunction } as unknown as DossierSource;
}

// Two probes, each with two sources of differing primary functions.
const SOURCES: Record<string, DossierSource[]> = {
  'probe-a': [src('a-ea', 'epistemic_authority'), src('a-pl', 'power_legitimation')],
  'probe-b': [src('b-ea', 'epistemic_authority'), src('b-ci', 'cohesion_identity')]
};
const sourcesForProbe: SourcesForProbe = (p) => SOURCES[p] ?? [];

describe('DISCOURSE_FUNCTIONS', () => {
  it('lists the four discourse functions with unique ids', () => {
    expect(DISCOURSE_FUNCTIONS).toHaveLength(4);
    const ids = DISCOURSE_FUNCTIONS.map((d) => d.id);
    expect(new Set(ids).size).toBe(4);
    expect(ids).toContain('epistemic_authority');
    expect(ids).toContain('subversion_friction');
  });
});

describe('sourceMatchesDf', () => {
  it('matches everything when the lock is null', () => {
    expect(sourceMatchesDf(src('x', 'power_legitimation'), null)).toBe(true);
    expect(sourceMatchesDf(src('x', null), null)).toBe(true);
  });

  it('matches on PRIMARY function only', () => {
    expect(sourceMatchesDf(src('x', 'epistemic_authority'), 'epistemic_authority')).toBe(true);
    expect(sourceMatchesDf(src('x', 'power_legitimation'), 'epistemic_authority')).toBe(false);
    expect(sourceMatchesDf(src('x', null), 'epistemic_authority')).toBe(false);
  });
});

describe('toggleProbeInGroup', () => {
  it('adds a probe not yet in the group', () => {
    const scopes: ScopeGroup[] = [{ probeIds: ['probe-a'], sourceIds: [] }];
    const next = toggleProbeInGroup(scopes, 0, 'probe-b', sourcesForProbe);
    expect(next[0]?.probeIds).toEqual(['probe-a', 'probe-b']);
  });

  it('removes a probe already in the group and prunes its orphaned sources', () => {
    const scopes: ScopeGroup[] = [
      { probeIds: ['probe-a', 'probe-b'], sourceIds: ['a-ea', 'b-ea'] }
    ];
    const next = toggleProbeInGroup(scopes, 0, 'probe-a', sourcesForProbe);
    expect(next[0]?.probeIds).toEqual(['probe-b']);
    // a-ea belonged to the removed probe → dropped; b-ea survives.
    expect(next[0]?.sourceIds).toEqual(['b-ea']);
  });

  it('does not mutate the input array', () => {
    const scopes: ScopeGroup[] = [{ probeIds: ['probe-a'], sourceIds: [] }];
    const next = toggleProbeInGroup(scopes, 0, 'probe-b', sourcesForProbe);
    expect(scopes[0]?.probeIds).toEqual(['probe-a']);
    expect(next).not.toBe(scopes);
  });

  it('returns the input unchanged for an out-of-range group index', () => {
    const scopes: ScopeGroup[] = [{ probeIds: ['probe-a'], sourceIds: [] }];
    expect(toggleProbeInGroup(scopes, 5, 'probe-b', sourcesForProbe)).toBe(scopes);
  });

  it('only touches the addressed group', () => {
    const scopes: ScopeGroup[] = [
      { probeIds: ['probe-a'], sourceIds: [] },
      { probeIds: ['probe-b'], sourceIds: [] }
    ];
    const next = toggleProbeInGroup(scopes, 0, 'probe-b', sourcesForProbe);
    expect(next[1]).toBe(scopes[1]);
  });
});

describe('toggleSourceInGroup', () => {
  it('adds and removes a source', () => {
    const scopes: ScopeGroup[] = [{ probeIds: ['probe-a'], sourceIds: [] }];
    const added = toggleSourceInGroup(scopes, 0, 'a-ea');
    expect(added[0]?.sourceIds).toEqual(['a-ea']);
    const removed = toggleSourceInGroup(added, 0, 'a-ea');
    expect(removed[0]?.sourceIds).toEqual([]);
  });

  it('returns input unchanged for a bad index', () => {
    const scopes: ScopeGroup[] = [{ probeIds: ['probe-a'], sourceIds: [] }];
    expect(toggleSourceInGroup(scopes, 3, 'a-ea')).toBe(scopes);
  });
});

describe('selectAllSourcesInGroup', () => {
  it('selects all of a probe sources when unlocked', () => {
    const scopes: ScopeGroup[] = [{ probeIds: ['probe-a'], sourceIds: [] }];
    const next = selectAllSourcesInGroup(scopes, 0, 'probe-a', null, sourcesForProbe);
    expect(next[0]?.sourceIds.sort()).toEqual(['a-ea', 'a-pl']);
  });

  it('selects only DF-matching sources when locked', () => {
    const scopes: ScopeGroup[] = [{ probeIds: ['probe-a'], sourceIds: [] }];
    const next = selectAllSourcesInGroup(
      scopes,
      0,
      'probe-a',
      'epistemic_authority',
      sourcesForProbe
    );
    expect(next[0]?.sourceIds).toEqual(['a-ea']);
  });

  it('preserves other probes already-selected sources', () => {
    const scopes: ScopeGroup[] = [{ probeIds: ['probe-a', 'probe-b'], sourceIds: ['b-ci'] }];
    const next = selectAllSourcesInGroup(scopes, 0, 'probe-a', null, sourcesForProbe);
    expect(next[0]?.sourceIds).toContain('b-ci');
    expect(next[0]?.sourceIds).toContain('a-ea');
    expect(next[0]?.sourceIds).toContain('a-pl');
  });
});

describe('clearSourcesInGroup', () => {
  it('empties the source list, keeping the probes', () => {
    const scopes: ScopeGroup[] = [{ probeIds: ['probe-a'], sourceIds: ['a-ea', 'a-pl'] }];
    const next = clearSourcesInGroup(scopes, 0);
    expect(next[0]?.sourceIds).toEqual([]);
    expect(next[0]?.probeIds).toEqual(['probe-a']);
  });
});

describe('pruneSourcesToLock', () => {
  it('drops sources that no longer match the new lock', () => {
    const scopes: ScopeGroup[] = [{ probeIds: ['probe-a'], sourceIds: ['a-ea', 'a-pl'] }];
    const next = pruneSourcesToLock(scopes, 0, 'epistemic_authority', sourcesForProbe);
    expect(next[0]?.sourceIds).toEqual(['a-ea']);
  });

  it('returns the SAME array reference when nothing is pruned', () => {
    const scopes: ScopeGroup[] = [{ probeIds: ['probe-a'], sourceIds: ['a-ea'] }];
    const next = pruneSourcesToLock(scopes, 0, 'epistemic_authority', sourcesForProbe);
    expect(next).toBe(scopes);
  });
});

describe('resolvePanelLock', () => {
  it('surfaces the shared lock when all groups agree', () => {
    expect(resolvePanelLock(['epistemic_authority', 'epistemic_authority'])).toBe(
      'epistemic_authority'
    );
  });

  it('returns null when all groups are unlocked', () => {
    expect(resolvePanelLock([null, null])).toBeNull();
  });

  it('returns null on a mix of locks', () => {
    expect(resolvePanelLock(['epistemic_authority', 'power_legitimation'])).toBeNull();
    expect(resolvePanelLock(['epistemic_authority', null])).toBeNull();
  });

  it('handles the single-group case', () => {
    expect(resolvePanelLock(['power_legitimation'])).toBe('power_legitimation');
    expect(resolvePanelLock([null])).toBeNull();
  });
});
