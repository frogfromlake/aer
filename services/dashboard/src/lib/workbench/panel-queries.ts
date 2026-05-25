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
import { getPresentation, isPureCountMetric } from '../viewmodes';

// ---------------------------------------------------------------------------
// Cell-render strategy
//
// `merged-single`   = 1 Cell, single ScopeGroup (the most common Aleph
//                     case). Sources within the group either render as
//                     a merged chart or as per-source small-multiples
//                     depending on `composition`.
// `merged-multi`    = N Cells, one per ScopeGroup, with the panel's
//                     composition='merged' propagated to each Cell so
//                     the sources INSIDE a group are unioned. The
//                     groups themselves remain side-by-side — only the
//                     within-group source set is merged (Phase 122k).
// `split`           = N Cells, one per ScopeGroup or per source within a
//                     single ScopeGroup. Each Cell uses the existing
//                     per-scope endpoints; no multi-scope payload.
// ---------------------------------------------------------------------------

export type CellRenderStrategy = 'merged-single' | 'merged-multi' | 'overlay' | 'split';

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
  // Phase 122k §11 finding — when the panel splits across multiple
  // ScopeGroups, each unit carries its ScopeGroup index so the host can
  // render the per-group visual accent (number badge + tint). Absent for
  // merged renders and for split-by-source units (single-group case).
  groupIndex?: number;
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
    // Phase 122k §11 finding — merged composition with multiple groups
    // produces ONE Cell PER group. Within each Cell, the group's sources
    // merge into a single chart (composition='merged' is forwarded to
    // the Cell). The ScopeGroups themselves remain side-by-side as
    // group-cells; only sources INSIDE a group are unioned.
    return {
      strategy: 'merged-multi',
      units: groups.map((g, i) => ({ ...scopeGroupToUnit(`mg${i}`, g), groupIndex: i }))
    };
  }

  // Phase 122k §14c finding 2 — `overlay` composition. The Cell receives
  // the union of sources but renders them as N separate viridis-coloured
  // lines on a shared canvas (per-source queries, one plot). Multi-group
  // overlay produces one Cell per group (like merged-multi), each
  // overlaying its group's sources.
  if (panel.composition === 'overlay') {
    if (groups.length === 1) {
      const g = groups[0]!;
      return {
        strategy: 'overlay',
        units: [scopeGroupToUnit('o1', g)]
      };
    }
    return {
      strategy: 'overlay',
      units: groups.map((g, i) => ({ ...scopeGroupToUnit(`og${i}`, g), groupIndex: i }))
    };
  }

  // split composition
  if (groups.length > 1) {
    // One Cell per ScopeGroup. Each unit carries its source group's
    // index so the PanelHost can visually distinguish the groups.
    return {
      strategy: 'split',
      units: groups.map((g, i) => ({ ...scopeGroupToUnit(`g${i}`, g), groupIndex: i }))
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

/**
 * Phase 130 / ADR-035 — merged-cross-probe guard (Brief §1.3).
 *
 * A `merged` composition pools the sources of a ScopeGroup onto one shared
 * axis. When that single Cell pools more than one probe (a ScopeGroup with
 * >1 probeId), an *intensive* metric (a sentiment average, a confidence
 * score) on that shared axis reads as a cross-context ranking — the exact
 * comparison framing Brief §1.3 forbids. Pure-count metrics are exempt
 * (a merged total is a legitimate sum, not a ranking), and metric-less
 * presentations (topic / co-occurrence) are governed by the separate
 * cross-language gate, not this one. `split` / `overlay` keep each probe on
 * its own axis and are never refused.
 *
 * The function is pure so it is unit-testable; the PanelHost renders the
 * standard RefusalSurface (`merged_cross_probe_unsupported`) when it
 * returns true. Callers that already hold a `selectCellRender(panel)` result
 * pass it in to avoid recomputing the render plan.
 */
export function shouldRefuseMergedCrossProbe(panel: Panel, cellRender?: CellRender): boolean {
  if (panel.composition !== 'merged') return false;
  // Metric-less presentations (topic_*, cooccurrence_network) do not pool a
  // scalar metric onto a shared axis — the §1.3 ranking concern does not
  // apply, and cross-language merges are caught by their own gate.
  if (getPresentation(panel.view).usesMetric === false) return false;
  if (isPureCountMetric(panel.metric)) return false;
  // Refuse only when a single rendered Cell actually pools >1 probe.
  const render = cellRender ?? selectCellRender(panel);
  return render.units.some((u) => u.probeIds.length > 1);
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
import { defaultViewModeForPillar } from '../viewmodes';

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

/**
 * Phase 122k — Build a default Panel from a list of ScopeGroups. Used by
 * the ScopeEditor in create-mode (workbench-page F2, +Panel F3) to turn
 * the editor's draft into a Panel that the addPanel mutator can append.
 */
export function buildPanelFromScopes(
  scopes: readonly ScopeGroupType[],
  opts: {
    view?: ViewModeType;
    metric?: string;
    layer?: DataLayer;
    lockedFunction?: string | null | undefined;
  } = {}
): PanelType {
  const panel: PanelType = {
    scopes: scopes.map((g) => ({ probeIds: [...g.probeIds], sourceIds: [...g.sourceIds] })),
    // Phase 122k §11 finding — Split is the more informative default
    // (per-source small-multiples reveal heterogeneity); merge collapses
    // multi-source scopes into a single aggregate, which makes sense
    // when the user explicitly asks for the union but not as a default.
    composition: 'split',
    // Phase 130 — callers pass the pillar-correct default view
    // (`defaultViewModeForPillar`); the literal fallback is the registry-
    // wide default `distribution`, never the diachronic `time_series`.
    view: opts.view ?? 'distribution',
    metric: opts.metric ?? 'sentiment_score_sentiws',
    layer: opts.layer ?? 'gold'
  };
  if (opts.lockedFunction) {
    panel.locked = true;
    panel.lockedReason = 'df_entry';
    panel.lockedFunction = opts.lockedFunction;
  }
  return panel;
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
    // Phase 130 — the default view follows the pillar (Aleph→distribution,
    // Episteme→time_series, Rhizome→cooccurrence_network) so a freshly
    // composed Workbench renders the pillar's identity cell, not a leaked
    // time-series.
    view: opts.view ?? defaultViewModeForPillar(opts.pillar),
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
