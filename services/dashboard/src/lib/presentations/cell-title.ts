// Phase 148e — typed cell-title model + composer.
//
// Before this, every one of the 17 presentation cells built its `<h3>` heading
// ad hoc, so the subject wrapper (`<code>` vs `<span class="primary">` vs plain),
// the slot order, and the separators (— · × → ∪ vs) all diverged. This module is
// the single source of title grammar: a declarative `CellTitleSpec` each cell
// fills, and `composeCellTitle()` — a pure, testable normaliser — that orders the
// qualifier tail, drops empties, and applies defaults. `CellTitleBar.svelte`
// renders the normalised result with the two-tier grammar (presentation eyebrow
// + strong subject line + scope pill + muted qualifier tail).
//
// Colour budget: exactly two viridis anchors — the presentation eyebrow and the
// scope pill — everything else is neutral fg / muted (Design Brief, Phase 148e).

/** Operator joining a metric/field pair on the subject line. */
export type TitleOp = '×' | '→';
/** Relation joining a probe/source pair on the scope pill. */
export type ScopeRelation = '∪' | 'vs';

/** The strong subject line under the eyebrow. */
export type TitleSubject =
  | { kind: 'none' } // relational cells: the presentation IS the subject
  | { kind: 'single'; label: string } // a metric OR a field
  | { kind: 'pair'; left: string; op: TitleOp; right: string }
  | { kind: 'topics'; label: string };

/** The scope pill(s) — a display label resolved via `resolveScopeLabel`. */
export type TitleScope =
  | { kind: 'none' }
  | { kind: 'single'; label: string }
  | { kind: 'pair'; left: string; right: string; relation: ScopeRelation };

/** A muted tail item (resolution, r=, Top N, ≥2 sources, layer/tier badge). */
export interface TitleQualifier {
  label: string;
  /** Visual tone — `muted` plain text (default), `tier`/`layer` a small chip. */
  tone?: 'muted' | 'tier' | 'layer';
  /** Optional tooltip. */
  title?: string;
}

/** What a cell declares; `composeCellTitle` normalises it for the bar. */
export interface CellTitleSpec {
  /** Localised presentation label (the eyebrow). */
  presentation: string;
  subject: TitleSubject;
  /** Dimmed sub-slot after the subject (e.g. "BERT Multilingual"). */
  model?: string | null;
  scope: TitleScope;
  qualifiers?: ReadonlyArray<TitleQualifier | null | undefined>;
  /** Stable seed for the aria-labelledby id. */
  idSeed: string;
}

/** The normalised title the bar renders. */
export interface CellTitle {
  presentation: string;
  subject: TitleSubject;
  model: string | null;
  scope: TitleScope;
  qualifiers: TitleQualifier[];
  idSeed: string;
}

// Fixed tail order so qualifiers never reshuffle between cells: plain muted
// items keep their declared order, badge-toned items (tier/layer) sort last so
// they read as a trailing chip group.
function qualifierRank(q: TitleQualifier): number {
  return q.tone === 'tier' || q.tone === 'layer' ? 1 : 0;
}

/**
 * Normalise a cell's declarative spec: strip empty qualifiers, apply the fixed
 * tail order, and default `model` to null. Pure — unit-tested.
 */
export function composeCellTitle(spec: CellTitleSpec): CellTitle {
  const qualifiers = (spec.qualifiers ?? [])
    .filter((q): q is TitleQualifier => !!q && q.label.trim().length > 0)
    .map((q) => ({ ...q, tone: q.tone ?? ('muted' as const) }));
  // Stable sort by rank (Array.prototype.sort is stable in modern engines).
  qualifiers.sort((a, b) => qualifierRank(a) - qualifierRank(b));

  const model = spec.model && spec.model.trim().length > 0 ? spec.model : null;

  return {
    presentation: spec.presentation,
    subject: spec.subject,
    model,
    scope: spec.scope,
    qualifiers,
    idSeed: spec.idSeed
  };
}
