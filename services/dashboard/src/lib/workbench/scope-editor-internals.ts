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
import { m } from '../paraglide/messages.js';

/** A single source row as it arrives on the probe dossier. */
export type DossierSource = ProbeDossierDto['sources'][number];

/** Resolves the cached sources for a probe (`[]` while its dossier loads). */
export type SourcesForProbe = (probeId: string) => DossierSource[];

/** DF metadata table — id, human label, and the chip/accent colour. The
 *  single source of truth for the four discourse functions the editor
 *  surfaces (mirrors the worker/BFF DF taxonomy). `label` is a getter so the
 *  rendered text stays locale-reactive (resolved against the active locale at
 *  each render) and reuses the canonical `domain_function_*_label` catalog —
 *  never frozen at module load. */
export const DISCOURSE_FUNCTIONS: ReadonlyArray<{
  id: DiscourseFunction;
  label: () => string;
  color: string;
}> = [
  {
    id: 'epistemic_authority',
    label: () => m.domain_function_epistemic_authority_label(),
    color: '#7dc7e5'
  },
  {
    id: 'power_legitimation',
    label: () => m.domain_function_power_legitimation_label(),
    color: '#e8a25c'
  },
  {
    id: 'cohesion_identity',
    label: () => m.domain_function_cohesion_identity_label(),
    color: '#a3c984'
  },
  {
    id: 'subversion_friction',
    label: () => m.domain_function_subversion_friction_label(),
    color: '#d97a7a'
  }
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

/** Clear ONLY the given probe's sources from a group, leaving every other
 *  probe's selected sources intact — the per-probe mirror of
 *  selectAllSourcesInGroup (the "Clear all" button is scoped to its own probe
 *  section, not the whole group). Returns the input unchanged on a no-op. */
export function clearSourcesForProbeInGroup(
  scopes: ScopeGroup[],
  groupIndex: number,
  probeId: string,
  sourcesForProbe: SourcesForProbe
): ScopeGroup[] {
  const group = scopes[groupIndex];
  if (!group) return scopes;
  const probeSourceNames = new Set(sourcesForProbe(probeId).map((s) => s.name));
  const remaining = group.sourceIds.filter((sid) => !probeSourceNames.has(sid));
  if (remaining.length === group.sourceIds.length) return scopes; // no-op
  return scopes.map((g, i) =>
    i === groupIndex ? { probeIds: [...group.probeIds], sourceIds: remaining } : g
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

/** Materialise the per-probe "whole-probe" intent inside a multi-probe group
 *  before the scope is committed.
 *
 *  A ScopeGroup carries a FLAT `sourceIds` list shared by all its probes, but
 *  every consumer resolves a non-empty list as "exactly these sources" for the
 *  whole group. So a group like `{probeIds:[X,Y], sourceIds:[x1]}` — the user
 *  picked one source of X and left Y untouched — wrongly renders as just `x1`,
 *  dropping all of Y. The editor's intent is per-probe: a probe with NO selected
 *  source means "all of that probe's sources".
 *
 *  This makes that intent explicit at commit time: for any group that has SOME
 *  narrowing (`sourceIds` non-empty), every in-scope probe that contributes no
 *  source of its own has ALL its (DF-lock-matching) sources appended. A group
 *  with no narrowing (empty `sourceIds` = whole group) is left untouched, so the
 *  common "all sources" case still round-trips as an empty list (and stays live
 *  to a probe's future sources). Per-group lock is respected, mirroring
 *  `selectAllSourcesInGroup`. Pure; unit-tested. */
export function materializeWholeProbeSources(
  scopes: ScopeGroup[],
  perGroupLock: (DiscourseFunction | null)[],
  sourcesForProbe: SourcesForProbe
): ScopeGroup[] {
  return scopes.map((group, i) => {
    if (group.sourceIds.length === 0) return group; // whole group — leave live
    const lock = perGroupLock[i] ?? null;
    const sourceIds = [...group.sourceIds];
    for (const probeId of group.probeIds) {
      const probeSources = sourcesForProbe(probeId).filter((s) => sourceMatchesDf(s, lock));
      if (probeSources.length === 0) continue; // sources not loaded — cannot materialise
      const hasSelection = probeSources.some((s) => sourceIds.includes(s.name));
      if (hasSelection) continue; // this probe already narrowed — keep its selection
      for (const s of probeSources) if (!sourceIds.includes(s.name)) sourceIds.push(s.name);
    }
    if (sourceIds.length === group.sourceIds.length) return group;
    return { probeIds: [...group.probeIds], sourceIds };
  });
}
