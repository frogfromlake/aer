// Phase 141 — ScopeEditor pure internals.
//
// The non-trivial, previously-inline draft mutation cores of the
// ScopeEditor, lifted out of the component so they can be unit-tested and
// shared with the per-card child (ScopeGroupCard). Every function here is
// pure: it takes the current `ScopeGroup[]` (and, where source membership
// matters, a `sourcesForProbe` resolver) and returns the next array — never
// mutating its input. The reactive `$state` ownership stays in the component.
//
// The draft-PERSISTENCE one-shot (sessionStorage) lives separately in
// `scope-editor-draft.ts`; this module is purely the in-editor edit logic.

import type { ScopeGroup } from '$lib/state/url-internals';
import type { ProbeDossierDto } from '$lib/api/queries';
import type { DiscourseFunction } from '$lib/discourse-function';

/** A single source row as it arrives on the probe dossier. */
export type DossierSource = ProbeDossierDto['sources'][number];

/** Resolves the cached sources for a probe (`[]` while its dossier loads). */
export type SourcesForProbe = (probeId: string) => DossierSource[];

/** DF metadata table — id, human label, and the chip/accent colour. The
 *  single source of truth for the four discourse functions the editor
 *  surfaces (mirrors the worker/BFF DF taxonomy). */
export const DISCOURSE_FUNCTIONS: ReadonlyArray<{
  id: DiscourseFunction;
  label: string;
  color: string;
}> = [
  { id: 'epistemic_authority', label: 'Epistemic Authority', color: '#7dc7e5' },
  { id: 'power_legitimation', label: 'Power Legitimation', color: '#e8a25c' },
  { id: 'cohesion_identity', label: 'Cohesion & Identity', color: '#a3c984' },
  { id: 'subversion_friction', label: 'Subversion & Friction', color: '#d97a7a' }
];

/** Whether a source matches a DF lock.
 *
 *  Phase 122k §11 finding — DF-lock matches on PRIMARY only. The secondary-
 *  function tag is a softer signal (Bundesregierung's secondary EA exists
 *  because policy reads as authoritative, but the source's structural role is
 *  PL). Locking on EA must NOT auto-include PL-primary sources just because
 *  they carry a secondary EA. `null` (no lock) matches everything. */
export function sourceMatchesDf(source: DossierSource, df: DiscourseFunction | null): boolean {
  if (df === null) return true;
  return source.primaryFunction === df;
}

/** Toggle a probe's membership in a group. Source-IDs orphaned by a probe
 *  deselection are dropped automatically: any source whose owning probe is no
 *  longer in the group is filtered out. */
export function toggleProbeInGroup(
  scopes: ScopeGroup[],
  groupIndex: number,
  probeId: string,
  sourcesForProbe: SourcesForProbe
): ScopeGroup[] {
  const group = scopes[groupIndex];
  if (!group) return scopes;
  const probeIds = group.probeIds.includes(probeId)
    ? group.probeIds.filter((p) => p !== probeId)
    : [...group.probeIds, probeId];
  const remainingProbes = new Set(probeIds);
  const liveSourceIds = group.sourceIds.filter((sid) => {
    for (const pid of remainingProbes) {
      if (sourcesForProbe(pid).some((s) => s.name === sid)) return true;
    }
    return false;
  });
  return scopes.map((g, i) => (i === groupIndex ? { probeIds, sourceIds: liveSourceIds } : g));
}

/** Toggle a single source's membership in a group. */
export function toggleSourceInGroup(
  scopes: ScopeGroup[],
  groupIndex: number,
  sourceName: string
): ScopeGroup[] {
  const group = scopes[groupIndex];
  if (!group) return scopes;
  const sourceIds = group.sourceIds.includes(sourceName)
    ? group.sourceIds.filter((s) => s !== sourceName)
    : [...group.sourceIds, sourceName];
  return scopes.map((g, i) =>
    i === groupIndex ? { probeIds: [...group.probeIds], sourceIds } : g
  );
}

/** Select all of one probe's DF-matching sources in a group, preserving the
 *  selection of other probes' sources. */
export function selectAllSourcesInGroup(
  scopes: ScopeGroup[],
  groupIndex: number,
  probeId: string,
  lock: DiscourseFunction | null,
  sourcesForProbe: SourcesForProbe
): ScopeGroup[] {
  const group = scopes[groupIndex];
  if (!group) return scopes;
  const matching = sourcesForProbe(probeId)
    .filter((s) => sourceMatchesDf(s, lock))
    .map((s) => s.name);
  const otherProbeSources = group.sourceIds.filter(
    (sid) => !sourcesForProbe(probeId).some((s) => s.name === sid)
  );
  return scopes.map((g, i) =>
    i === groupIndex
      ? { probeIds: [...group.probeIds], sourceIds: [...otherProbeSources, ...matching] }
      : g
  );
}

/** Clear all sources in a group (whole-probe scope). */
export function clearSourcesInGroup(scopes: ScopeGroup[], groupIndex: number): ScopeGroup[] {
  const group = scopes[groupIndex];
  if (!group) return scopes;
  return scopes.map((g, i) =>
    i === groupIndex ? { probeIds: [...group.probeIds], sourceIds: [] } : g
  );
}

/** Prune a group's sources to those still matching a new DF lock. Returns the
 *  input array unchanged when nothing is pruned (so a no-op lock change does
 *  not churn the draft state). */
export function pruneSourcesToLock(
  scopes: ScopeGroup[],
  groupIndex: number,
  df: DiscourseFunction,
  sourcesForProbe: SourcesForProbe
): ScopeGroup[] {
  const group = scopes[groupIndex];
  if (!group) return scopes;
  const filtered = group.sourceIds.filter((name) => {
    for (const pid of group.probeIds) {
      const src = sourcesForProbe(pid).find((s) => s.name === name);
      if (src && sourceMatchesDf(src, df)) return true;
    }
    return false;
  });
  if (filtered.length === group.sourceIds.length) return scopes;
  return scopes.map((g, i) =>
    i === groupIndex ? { probeIds: [...group.probeIds], sourceIds: filtered } : g
  );
}

/** Resolve the single panel-level lock from the per-group locks.
 *
 *  Today the Panel carries ONE `lockedFunction`: when every group shares the
 *  same restriction, that lock surfaces on the panel; a mix leaves the panel
 *  unlocked. (A future phase lifts the lock into per-ScopeGroup schema.) */
export function resolvePanelLock(
  perGroupLock: (DiscourseFunction | null)[]
): DiscourseFunction | null {
  const uniqueLocks = new Set(perGroupLock);
  return uniqueLocks.size === 1
    ? ((Array.from(uniqueLocks)[0] as DiscourseFunction | null | undefined) ?? null)
    : null;
}
