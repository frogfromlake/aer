// Phase 148g — co-occurrence query routing + cross-language detection, shared by
// the SVG cell (CoOccurrenceNetworkCell) and the at-scale renderer
// (CoOccurrenceNetworkAtScale). Kept out of the renderers so both stay under the
// file-length ratchet and the routing decision is unit-tested in one place.
//
// Routing rule: a scope that unions MORE THAN ONE probe merges through the
// multi-scope POST (`/entities/cooccurrence/query`, which the BFF unions across
// probes). A single probe — even one spanning several sources — keeps the richer
// GET path (which also carries the node-metric size/colour + Negative-Space
// channels the POST does not), so the common single-probe case is unchanged from
// before Phase 148g; only the genuine cross-probe merge (the previously-broken
// "only the first probe" case) takes the new path.
// Relative import (NOT `$lib`) so this module loads under node-env Vitest — the
// vitest config carries no `$lib` alias (project convention for node-tested .ts).
import {
  entityCoOccurrenceQuery,
  entityCoOccurrenceQueryMulti,
  type CoOccurrenceGraphDto,
  type FetchContext,
  type QueryOptions
} from '../api/queries';

export interface CoOccurrenceScopeRequest {
  scope: 'probe' | 'source';
  scopeId: string;
  probeIds?: readonly string[] | undefined;
  sourceIds?: readonly string[] | undefined;
  start: string | undefined;
  end: string | undefined;
  topN: number;
  maxNodes?: number | undefined;
  viewerLanguage?: string | undefined;
  nodeMetric?: string | undefined;
  nodeColorMetric?: string | undefined;
  minWeight?: number | undefined;
  negativeSpaceOverlay?: 'ghost' | undefined;
  allowCrossLanguage?: boolean | undefined;
}

/** True when the scope unions more than one probe → the multi-scope POST. */
export function isMultiProbeMerge(probeIds?: readonly string[] | undefined): boolean {
  return (probeIds?.length ?? 0) > 1;
}

// Phase 148g — node↔edge coupling. The node count is the primary control
// (breadth); the edge cap is a DENSITY that scales WITH the node count so the
// graph never fragments into a satellite cloud. COOCCURRENCE_EDGE_DENSITY is the
// baseline "edges per node" (avg degree); the BFF clamps edges to [5, 6000].
export const COOCCURRENCE_EDGE_DENSITY = 5;
export const COOCCURRENCE_EDGE_MAX = 6000;
export const COOCCURRENCE_EDGE_MIN = 5;
export const COOCCURRENCE_DEFAULT_MAXNODES = 200;

// Phase 148g — the settle cap and the node count MUST scale together: a 10k-node
// FA2 layout needs far longer to relax than a 200-node one, so a flat default
// starves the big maps (the operator saw 10k stuck at 20 s). MAX 240 s (≈1 s per
// 40 entities → 10k ≈ 4 min); the auto-stop still ends earlier on convergence, so
// this is only the upper bound.
export const SETTLE_SECONDS_MAX = 240;
export const SETTLE_SECONDS_MIN = 12;

/** Node-count-scaled default for the layout settle cap (seconds): ~1 s per 40
 *  entities, clamped to [12, 240]. Used by BOTH the settle lever (so its displayed
 *  default is truthful) and the at-scale renderer (so the layout uses the same
 *  value) — they read the panel's maxNodes, so they always agree. */
export function autoSettleSeconds(maxNodes: number): number {
  const scaled = Math.round((maxNodes > 0 ? maxNodes : 200) / 40);
  return Math.min(SETTLE_SECONDS_MAX, Math.max(SETTLE_SECONDS_MIN, scaled));
}

/** Effective edge cap for a node-first graph: an explicit user density when set,
 *  otherwise the auto baseline (nodes × density), always clamped to the BFF range.
 *  This is what keeps "more nodes → more edges" coherent: the node lever recomputes
 *  this, while the edge lever pins an explicit value (see the ratio-preserving
 *  setter in ConfigValueLevers). */
export function effectiveEdgeCap(maxNodes: number, explicitTopN?: number | undefined): number {
  if (explicitTopN !== undefined && explicitTopN > 0) {
    return Math.min(COOCCURRENCE_EDGE_MAX, Math.max(COOCCURRENCE_EDGE_MIN, explicitTopN));
  }
  const nodes = maxNodes > 0 ? maxNodes : COOCCURRENCE_DEFAULT_MAXNODES;
  const auto = Math.round(nodes * COOCCURRENCE_EDGE_DENSITY);
  return Math.min(COOCCURRENCE_EDGE_MAX, Math.max(COOCCURRENCE_EDGE_MIN, auto));
}

/** Distinct probe languages within the scope (deduped, sorted). */
export function scopeLanguages(
  probeIds: readonly string[] | undefined,
  languageOf: (probeId: string) => string | undefined
): string[] {
  const seen = new Set<string>();
  for (const id of probeIds ?? []) {
    const l = languageOf(id);
    if (l) seen.add(l);
  }
  return [...seen].sort();
}

/** A cross-language merge (>1 probe spanning >1 manifest language) needs the
 *  user's explicit confirmation before the BFF will union it (Phase 148g). */
export function isCrossLanguageMerge(
  probeIds: readonly string[] | undefined,
  languageOf: (probeId: string) => string | undefined
): boolean {
  return isMultiProbeMerge(probeIds) && scopeLanguages(probeIds, languageOf).length > 1;
}

/** Build the co-occurrence query for a scope, routing single-probe → GET and
 *  multi-probe merge → multi-scope POST (see file header). */
export function coOccurrenceQueryForScope(
  ctx: FetchContext,
  p: CoOccurrenceScopeRequest
): QueryOptions<CoOccurrenceGraphDto> {
  if (isMultiProbeMerge(p.probeIds)) {
    return entityCoOccurrenceQueryMulti(ctx, {
      scopes: [{ probeIds: [...(p.probeIds ?? [])], sourceIds: [...(p.sourceIds ?? [])] }],
      start: p.start,
      end: p.end,
      topN: p.topN,
      maxNodes: p.maxNodes,
      viewerLanguage: p.viewerLanguage,
      allowCrossLanguage: p.allowCrossLanguage
    });
  }
  // The GET param type uses bare optionals (`?: string`), so under
  // exactOptionalPropertyTypes we must OMIT (not pass `undefined`) the unset ones.
  return entityCoOccurrenceQuery(ctx, {
    scope: p.scope,
    scopeId: p.scopeId,
    start: p.start,
    end: p.end,
    topN: p.topN,
    ...(p.maxNodes !== undefined ? { maxNodes: p.maxNodes } : {}),
    ...(p.viewerLanguage ? { viewerLanguage: p.viewerLanguage } : {}),
    ...(p.nodeMetric ? { nodeMetric: p.nodeMetric } : {}),
    ...(p.nodeColorMetric ? { nodeColorMetric: p.nodeColorMetric } : {}),
    ...(p.minWeight !== undefined ? { minWeight: p.minWeight } : {}),
    ...(p.negativeSpaceOverlay ? { negativeSpaceOverlay: p.negativeSpaceOverlay } : {})
  });
}
