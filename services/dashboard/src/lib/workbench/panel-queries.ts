// Phase 122i / ADR-034 — Multi-Panel Workbench: Panel → BFF query mapper.
//
// The Pillar Shells take a `Panel` (Pillar→Window→Panel→ScopeGroup tree
// node from the URL state) and ask this module two questions:
//
//   1. Which `Cell` shape do I need to render? (selectCellRender)
//   2. How do I translate the Panel's ScopeGroups into a concrete BFF
//      request descriptor? (buildQueryFromPanel)
//
// Cell components themselves keep their Phase-107 `(scope, scopeId,
// sources[])` contract (see `viewmodes/registry.ts:72-93`). The Panel
// host translates ScopeGroups into per-Cell tuples for the `split`-
// composition path, or into a unioned multi-scope POST descriptor for
// the `merged`-composition path that needs the new CoOccurrence
// endpoint.

import type { Panel, ScopeGroup, ViewMode } from '$lib/state/url-internals';

// ---------------------------------------------------------------------------
// Cell-render strategy
//
// `merged-single`   = 1 Cell, single ScopeGroup, existing GET endpoints
//                     (legacy Aleph behaviour as a special case).
// `merged-multi`    = 1 Cell, union of N ScopeGroups, multi-scope path
//                     (CoOccurrence → POST /entities/cooccurrence/query,
//                      Topics → existing GET with unioned probeIds/sourceIds).
// `split`           = N Cells, one per ScopeGroup or per source within a
//                     single ScopeGroup. Each Cell uses the existing
//                     per-scope endpoints; no multi-scope payload.
// ---------------------------------------------------------------------------

export type CellRenderStrategy = 'merged-single' | 'merged-multi' | 'split';

export interface CellRenderUnit {
  // Stable React-key-style identifier so the Panel host can `{#each}` over
  // multiple Cells without re-mounting.
  key: string;
  // The `(scope, scopeId)` tuple to hand to a Cell that still consumes the
  // Phase-107 contract. `probeIds`/`sourceIds` carry multi-target lists
  // for endpoints that already support them (Topics, TimeSeries, etc.).
  scope: 'probe' | 'source';
  scopeId: string;
  probeIds: readonly string[];
  sourceIds: readonly string[];
}

export interface CellRender {
  strategy: CellRenderStrategy;
  units: readonly CellRenderUnit[];
}

// Views that operate on entity pairs and ignore the active metric. Their
// merged-multi-scope path needs the POST endpoint; their split path
// renders one Cell per ScopeGroup with the legacy GET endpoint.
const COOCCURRENCE_VIEWS: ReadonlySet<ViewMode> = new Set(['cooccurrence_network']);

/**
 * selectCellRender chooses how many Cells to render for the Panel and how
 * each Cell receives its scope. The function is pure and stateless: hosts
 * call it on every Panel update, memoising via the returned `key` values.
 */
export function selectCellRender(panel: Panel): CellRender {
  const groups = panel.scopes;
  if (groups.length === 0) {
    return { strategy: 'split', units: [] };
  }

  if (panel.composition === 'merged') {
    if (groups.length === 1) {
      const g = groups[0]!;
      return {
        strategy: 'merged-single',
        units: [scopeGroupToUnit('m1', g)]
      };
    }
    // Multi-scope merged. The Panel host issues exactly one query that
    // unions the groups. We still return one CellRenderUnit so the host
    // can drive a single Cell; the union is encoded in the unit's
    // probeIds/sourceIds for endpoints that consume CSV lists, and the
    // CoOccurrence host translates the same data into a POST descriptor.
    return {
      strategy: 'merged-multi',
      units: [scopeGroupsToUnionUnit('mN', groups)]
    };
  }

  // split composition
  if (groups.length > 1) {
    // One Cell per ScopeGroup.
    return {
      strategy: 'split',
      units: groups.map((g, i) => scopeGroupToUnit(`g${i}`, g))
    };
  }

  // Single ScopeGroup, split by source. When the group has zero source
  // narrowing, the host has no way to enumerate the probe's sources at
  // this layer (it doesn't have the Dossier); it renders the merged
  // single-Cell shape instead and the Shell-level component fans out per
  // source when it gets that data. We surface that case as `merged-single`
  // here so the Shell can apply its own per-source iteration as needed.
  const g = groups[0]!;
  if (g.sourceIds.length === 0) {
    return {
      strategy: 'merged-single',
      units: [scopeGroupToUnit('m1', g)]
    };
  }
  return {
    strategy: 'split',
    units: g.sourceIds.map((source, i) => ({
      key: `s${i}`,
      scope: 'source' as const,
      scopeId: source,
      probeIds: [],
      sourceIds: [source]
    }))
  };
}

function scopeGroupToUnit(key: string, g: ScopeGroup): CellRenderUnit {
  // Resolution choice:
  //   - probe scope when the group has at least one probe and no source
  //     narrowing OR multiple probes with explicit sources;
  //   - source scope when the group has exactly one source (single-target
  //     query path on the existing GET endpoints).
  const isSingleSource = g.probeIds.length === 1 && g.sourceIds.length === 1;
  if (isSingleSource) {
    return {
      key,
      scope: 'source',
      scopeId: g.sourceIds[0]!,
      probeIds: [],
      sourceIds: [g.sourceIds[0]!]
    };
  }
  return {
    key,
    scope: 'probe',
    scopeId: g.probeIds[0] ?? '',
    probeIds: g.probeIds,
    sourceIds: g.sourceIds
  };
}

function scopeGroupsToUnionUnit(key: string, groups: readonly ScopeGroup[]): CellRenderUnit {
  const probeSeen = new Set<string>();
  const sourceSeen = new Set<string>();
  const probeIds: string[] = [];
  const sourceIds: string[] = [];
  for (const g of groups) {
    for (const p of g.probeIds) {
      if (!probeSeen.has(p)) {
        probeSeen.add(p);
        probeIds.push(p);
      }
    }
    for (const s of g.sourceIds) {
      if (!sourceSeen.has(s)) {
        sourceSeen.add(s);
        sourceIds.push(s);
      }
    }
  }
  return {
    key,
    scope: 'probe',
    scopeId: probeIds[0] ?? '',
    probeIds,
    sourceIds
  };
}

// ---------------------------------------------------------------------------
// CoOccurrence-specific: panel → POST request descriptor.
//
// Returned only when the Panel renders CoOccurrence with merged composition
// AND has multiple ScopeGroups OR a multi-scope-group with explicit
// sourceIds across multiple probes (i.e. a shape the legacy GET endpoint
// cannot express). Other CoOccurrence cases use the legacy GET path.
// ---------------------------------------------------------------------------

export interface CoOccurrencePostDescriptor {
  scopes: ReadonlyArray<{
    probeIds: readonly string[];
    sourceIds: readonly string[];
  }>;
  topN: number | undefined;
}

export function coOccurrencePostDescriptorForPanel(
  panel: Panel
): CoOccurrencePostDescriptor | null {
  if (!COOCCURRENCE_VIEWS.has(panel.view)) return null;
  if (panel.composition !== 'merged') return null;
  if (panel.scopes.length < 2) return null;
  return {
    scopes: panel.scopes.map((g) => ({
      probeIds: g.probeIds,
      sourceIds: g.sourceIds
    })),
    topN: panel.topN
  };
}

// ---------------------------------------------------------------------------
// Phase 122i / ADR-034 — Workbench-URL builders for the two Probe-Dossier
// entry paths (Slice 5).
//
// The Dossier surfaces two equally-prominent entry paths into the
// Workbench:
//   1. DF-tile (locked, deterministic) — `buildDfEntryUrl()` produces a
//      single-pillar single-window single-panel URL with
//      composition='merged', locked=true, lockedReason='df_entry'. The
//      Workbench renders this in read-only mode; the user must navigate
//      back to the Dossier to recombine scope.
//   2. Free-Compose — `buildFreeComposeUrl()` produces a single-pillar
//      single-window single-panel URL with composition='merged',
//      locked=false. All controls editable; +Panel / +Compare adds
//      richer state from there.
//
// Both helpers emit the new `?activePillar=&aleph=…` form because the
// `locked` flag (for DF-tile) and the explicit Pillar choice (for
// Free-Compose) require the richer state. Phase-122h legacy URLs are
// untouched.
// ---------------------------------------------------------------------------

import {
  EMPTY_URL_STATE,
  writeToSearch,
  type DataLayer,
  type PillarState,
  type Panel as PanelType,
  type ScopeGroup as ScopeGroupType,
  type ViewMode as ViewModeType,
  type ViewingMode
} from '../state/url-internals';

export interface BuildEntryUrlParams {
  pillar: ViewingMode;
  probeIds: readonly string[];
  sourceIds: readonly string[];
  view?: ViewModeType;
  metric?: string;
  layer?: DataLayer;
}

/**
 * Build the DF-entry URL: locked single-panel state over the DF-covered
 * source set. The Workbench treats `locked=true` as read-only.
 */
export function buildDfEntryUrl(params: BuildEntryUrlParams & { lockedFunction: string }): string {
  return buildPillarUrl({
    ...params,
    locked: true,
    lockedReason: 'df_entry',
    lockedFunction: params.lockedFunction
  });
}

/**
 * Build the Free-Compose-entry URL: an editable single-panel state. The
 * user can mutate composition, scope, view, metric, etc. from there.
 */
export function buildFreeComposeUrl(params: BuildEntryUrlParams): string {
  return buildPillarUrl(params);
}

interface BuildPillarUrlOpts extends BuildEntryUrlParams {
  locked?: boolean;
  lockedReason?: 'df_entry';
  lockedFunction?: string;
}

function buildPillarUrl(opts: BuildPillarUrlOpts): string {
  const scopeGroup: ScopeGroupType = {
    probeIds: [...opts.probeIds],
    sourceIds: [...opts.sourceIds]
  };
  const panel: PanelType = {
    scopes: [scopeGroup],
    composition: 'merged',
    view: opts.view ?? 'time_series',
    metric: opts.metric ?? 'sentiment_score_sentiws',
    layer: opts.layer ?? 'gold'
  };
  if (opts.locked) {
    panel.locked = true;
    if (opts.lockedReason) panel.lockedReason = opts.lockedReason;
    if (opts.lockedFunction) panel.lockedFunction = opts.lockedFunction;
  }
  const pillarState: PillarState = {
    windows: [{ panels: [panel], focusedPanelIndex: 0 }],
    activeWindowIndex: 0
  };
  return writeToSearch({
    ...EMPTY_URL_STATE,
    activePillar: opts.pillar,
    pillars: {
      aleph: opts.pillar === 'aleph' ? pillarState : null,
      episteme: opts.pillar === 'episteme' ? pillarState : null,
      rhizome: opts.pillar === 'rhizome' ? pillarState : null
    }
  });
}
