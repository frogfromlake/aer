// Discourse-function single source of truth (Phase 122h, ADR-033 §4).
//
// The four WP-001 §3 discourse functions are referenced from four UI
// locations (ProbeDossier source cards, WorkbenchScopeBar function chips,
// per-Cell filter strips, SourceCard headers). Each location used to
// declare its own FUNCTION_* constant with inconsistent shapes — ProbeDossier
// carried color+description, SourceCard had only label+abbr. This module
// is the unified definition; all consumers import from here.
//
// New consumers MUST go through `FunctionBadge` in
// `$lib/components/base/FunctionBadge.svelte` rather than reading
// `FUNCTION_DEFINITIONS` directly for display — the Badge owns the visual
// grammar (Brief §5.7 Progressive Semantics, ADR-033 §4).

export const DISCOURSE_FUNCTIONS = [
  'epistemic_authority',
  'power_legitimation',
  'cohesion_identity',
  'subversion_friction'
] as const;

export type DiscourseFunction = (typeof DISCOURSE_FUNCTIONS)[number];

export interface FunctionDef {
  /** URL-stable key, matches the Postgres `sources.primary_function` enum. */
  key: DiscourseFunction;
  /** Two-letter abbreviation (EA / PL / CI / SF). */
  abbr: string;
  /** Full human-readable label. */
  label: string;
  /** Accent colour for badges, chips, identity strips. */
  color: string;
  /** One-line description anchored on WP-001 §3. */
  description: string;
}

export const FUNCTION_DEFINITIONS: Record<DiscourseFunction, FunctionDef> = {
  epistemic_authority: {
    key: 'epistemic_authority',
    abbr: 'EA',
    label: 'Epistemic Authority',
    color: '#5283b8',
    description: 'Sources that produce and legitimate knowledge claims (WP-001 §3).'
  },
  power_legitimation: {
    key: 'power_legitimation',
    abbr: 'PL',
    label: 'Power Legitimation',
    color: '#7ec4a0',
    description: 'Sources that frame, justify, or contest political power (WP-001 §3).'
  },
  cohesion_identity: {
    key: 'cohesion_identity',
    abbr: 'CI',
    label: 'Cohesion & Identity',
    color: '#c8a85a',
    description: 'Sources that articulate collective identity and social cohesion (WP-001 §3).'
  },
  subversion_friction: {
    key: 'subversion_friction',
    abbr: 'SF',
    label: 'Subversion & Friction',
    color: '#9a8fb8',
    description: 'Sources that challenge dominant frames or introduce friction (WP-001 §3).'
  }
};

/** Type-safe lookup with the unknown-key escape valve callers occasionally need. */
export function getFunctionDef(key: string | null | undefined): FunctionDef | null {
  if (!key) return null;
  return FUNCTION_DEFINITIONS[key as DiscourseFunction] ?? null;
}

/** Ordered list for chip rows, switchers, coverage strips. */
export const FUNCTION_DEFINITIONS_ORDERED: readonly FunctionDef[] = DISCOURSE_FUNCTIONS.map(
  (key) => FUNCTION_DEFINITIONS[key]
);

/** Canonical anchor used by every ⓘ-affordance on the Badge and elsewhere. */
export const FUNCTION_INFO_HREF = '/reflection/wp/wp-001?section=3';
