// Pure helpers for ProbeCard (Phase 141 decomposition). The grouping of sources
// under their primary discourse function, the coverage set, and the two
// structural-meta scalars live here so they are unit-testable in isolation; the
// component (and its ProbeDfCards child) keeps only the markup + reactive state.
import type { DiscourseFunction } from '$lib/discourse-function';
import type { ProbeDossierSourceDto } from '$lib/api/queries';

// The four canonical discourse-function keys, in taxonomy order — kept local so
// this pure module needs no VALUE import from $lib (the node-only unit runner
// cannot resolve the `$lib` alias; the type import above is erased). Mirrors
// DISCOURSE_FUNCTIONS in $lib/discourse-function.
const FUNCTION_KEYS: readonly DiscourseFunction[] = [
  'epistemic_authority',
  'power_legitimation',
  'cohesion_identity',
  'subversion_friction'
];

export type SourcesByFunction = Record<DiscourseFunction, ProbeDossierSourceDto[]>;

// Group sources by primary discourse function for the DF-Card container pattern.
// Sources without a primaryFunction (or with an unknown one) are excluded here —
// they surface via `unclassifiedSources`.
export function groupSourcesByFunction(
  sources: readonly ProbeDossierSourceDto[]
): SourcesByFunction {
  const out: SourcesByFunction = {
    epistemic_authority: [],
    power_legitimation: [],
    cohesion_identity: [],
    subversion_friction: []
  };
  for (const s of sources) {
    const fn = s.primaryFunction as DiscourseFunction | null | undefined;
    if (fn && fn in out) out[fn].push(s);
  }
  return out;
}

// Sources with no primary discourse function — rendered in their own container
// after the four canonical functions (Negative Space: a classified-but-unmapped
// source is disclosed, not dropped).
export function unclassifiedSources(
  sources: readonly ProbeDossierSourceDto[]
): ProbeDossierSourceDto[] {
  return sources.filter((s) => !s.primaryFunction);
}

// The set of functions that have at least one source — drives the per-card
// covered/uncovered state.
export function coveredFunctions(grouped: SourcesByFunction): Set<DiscourseFunction> {
  return new Set(FUNCTION_KEYS.filter((fn) => grouped[fn] && grouped[fn].length > 0));
}

// Total publication rate across the probe's sources (docs/day), or null when no
// source reports a rate (so the UI renders an em-dash rather than a false 0).
export function publicationRatePerDay(sources: readonly ProbeDossierSourceDto[]): number | null {
  const total = sources.reduce((sum, s) => sum + (s.publicationFrequencyPerDay ?? 0), 0);
  return total > 0 ? total : null;
}

// Function-coverage percentage (0 when the total is 0, avoiding a divide-by-zero).
export function coveragePercent(functionCoverage: { covered: number; total: number }): number {
  return functionCoverage.total > 0
    ? Math.round((functionCoverage.covered / functionCoverage.total) * 100)
    : 0;
}
