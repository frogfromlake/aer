// Phase 148e — shared scope-label resolver.
//
// Every presentation cell renders a "scope" in its title: a probe id, a source
// name, or a pair of those. Before this util, each cell rendered the raw
// `scopeId` verbatim — which leaks a machine probe id (`probe-0-de-institutional-
// web`) into the heading for every probe-scoped cell. Only LeadLagCell resolved
// it, via an inline `labelFor()`. This lifts that pattern into one pure function
// so the whole Workbench resolves scope labels identically.
//
// Resolution rule (mirrors the BFF Probe schema fallback): a probe id maps to
// `shortName ?? displayName ?? id`. A source name (or any id with no probe
// match) is returned verbatim — source names are already human-readable in the
// institutional-web probes, and the per-source emic designation lives in the
// SourceCard, not the cell title (Phase 148e decision 5b).

export interface ProbeLabelLike {
  probeId: string;
  displayName?: string | null;
  shortName?: string | null;
}

/**
 * Resolve a single scope id to its display label. Probe ids resolve through the
 * probe registry's `shortName ?? displayName ?? id` fallback; anything else
 * (source names, unknown ids) returns verbatim.
 */
export function resolveScopeLabel(scopeId: string, probes: ReadonlyArray<ProbeLabelLike>): string {
  const p = probes.find((x) => x.probeId === scopeId);
  if (p) return p.shortName ?? p.displayName ?? scopeId;
  return scopeId;
}
