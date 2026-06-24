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
// `shortName ?? displayName ?? id`. A source name resolves through the injected
// `sourceLabel` resolver to its WP-001 emic designation ("Tagesschau", "Élysée"),
// falling back to the raw name. Phase 148g overrides the earlier 148e decision
// (raw source names in titles) — the operator wants the display name everywhere.
// The resolver is injected (not imported) so this module stays pure + testable;
// the default is identity, preserving the verbatim behaviour for callers that
// pass none.

export interface ProbeLabelLike {
  probeId: string;
  displayName?: string | null;
  shortName?: string | null;
}

/**
 * Resolve a single scope id to its display label. Probe ids resolve through the
 * probe registry's `shortName ?? displayName ?? id` fallback; anything else
 * (a source name) resolves through `sourceLabel` (default: verbatim).
 */
export function resolveScopeLabel(
  scopeId: string,
  probes: ReadonlyArray<ProbeLabelLike>,
  sourceLabel: (name: string) => string = (s) => s
): string {
  const p = probes.find((x) => x.probeId === scopeId);
  if (p) return p.shortName ?? p.displayName ?? scopeId;
  return sourceLabel(scopeId);
}
