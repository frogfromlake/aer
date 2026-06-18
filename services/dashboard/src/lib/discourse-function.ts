// Discourse-function single source of truth (Phase 122h, ADR-033 §4).
//
// The four WP-001 §3 discourse functions are referenced from four UI
// locations (ProbeDossier source cards,
// per-Cell filter strips, SourceCard headers). Each location used to
// declare its own FUNCTION_* constant with inconsistent shapes — ProbeDossier
// carried color+description, SourceCard had only label+abbr. This module
// is the unified definition; all consumers import from here.
//
// New consumers MUST go through `FunctionBadge` in
// `$lib/components/base/FunctionBadge.svelte` rather than reading
// `FUNCTION_DEFINITIONS` directly for display — the Badge owns the visual
// grammar (Brief §5.7 Progressive Semantics, ADR-033 §4).

// Relative import (NOT `$lib/...`): this SoT is imported by node-env Vitest,
// which does not resolve the `$lib` alias for runtime imports (Phase 144 /
// ADR-042). `m.*()` reads the UI-locale rune per call, so the accessors below
// resolve label/description reactively; in node tests (no overwriteGetLocale)
// they fall back to the base locale 'en', keeping the English-pinning tests green.
import { m } from './paraglide/messages.js';

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

// Locale-resolved label/description per function (Phase 144b / ADR-042). The
// English literals in FUNCTION_DEFINITIONS above are the structural fallback;
// every display accessor returns these localized values. Reactive: m.*() reads
// the UI-locale rune, so a FunctionBadge re-renders on a language switch with no
// consumer change. The WP-001 §3 anchor stays inside both locales' descriptions.
const FUNCTION_LABELS: Record<DiscourseFunction, () => string> = {
  epistemic_authority: () => m.domain_function_epistemic_authority_label(),
  power_legitimation: () => m.domain_function_power_legitimation_label(),
  cohesion_identity: () => m.domain_function_cohesion_identity_label(),
  subversion_friction: () => m.domain_function_subversion_friction_label()
};
const FUNCTION_DESCRIPTIONS: Record<DiscourseFunction, () => string> = {
  epistemic_authority: () => m.domain_function_epistemic_authority_desc(),
  power_legitimation: () => m.domain_function_power_legitimation_desc(),
  cohesion_identity: () => m.domain_function_cohesion_identity_desc(),
  subversion_friction: () => m.domain_function_subversion_friction_desc()
};

/** A copy of `def` with `label`/`description` resolved for the active UI locale
 *  (key/abbr/color are locale-independent). */
function localizeFunction(def: FunctionDef): FunctionDef {
  return {
    ...def,
    label: FUNCTION_LABELS[def.key](),
    description: FUNCTION_DESCRIPTIONS[def.key]()
  };
}

/** Type-safe, locale-resolved lookup with the unknown-key escape valve callers
 *  occasionally need. */
export function getFunctionDef(key: string | null | undefined): FunctionDef | null {
  if (!key) return null;
  const def = FUNCTION_DEFINITIONS[key as DiscourseFunction];
  return def ? localizeFunction(def) : null;
}

/** Ordered list for chip rows, switchers, coverage strips. */
export const FUNCTION_DEFINITIONS_ORDERED: readonly FunctionDef[] = DISCOURSE_FUNCTIONS.map(
  (key) => FUNCTION_DEFINITIONS[key]
);

/** Canonical anchor used by every ⓘ-affordance on the Badge and elsewhere. */
export const FUNCTION_INFO_HREF = '/reflection/wp/wp-001?section=3';
