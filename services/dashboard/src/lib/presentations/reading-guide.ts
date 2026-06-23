// Reading Guide composer — Phase 148f.
//
// The single brain behind the Workbench "How to read" surface. It explains a
// cell's *live* configuration COMPOSITIONALLY: rather than authoring every
// (presentation × composition × metric × …) combination (a combinatorial
// explosion), it emits one grounded fragment per active dimension and groups
// them into the SIX epistemic questions a scientist asks — the structure of a
// paper's methods section:
//
//   what · measure · sample · encoding · compare · limits
//
// Each `ReadingNote` is tagged with its question, its render LEVEL (panel-shared
// vs per-cell delta), an optional visual `channel` (so the UI can tint a note to
// match the chart's encoding), and an optional Working-Paper/ADR anchor.
//
// Backward compatibility: the legacy `composeHowToRead(presentation, facts,
// templateBase) → string[]` (consumed by every cell's CellExport payload) is
// preserved here as a thin wrapper over `buildCellNotes` — so the export output
// is byte-identical and `how-to-read.test.ts` passes unchanged. `how-to-read.ts`
// re-exports it. The richer `composeReadingGuide` is the new on-screen path.
//
// Pure + node-test-safe: all user strings are Paraglide messages imported via the
// RELATIVE path (vitest does not resolve `$lib`; it falls back to base locale
// 'en'). All label resolution that needs the reactive registry (metricLabel,
// fieldLabel, …) happens in the Svelte caller and is passed in pre-resolved.

import { m } from '../paraglide/messages.js';
import type { Presentation } from '$lib/state/url-internals';
import type { PanelSubjectKind } from './metric-presentation';

export type ReadingQuestion = 'what' | 'measure' | 'sample' | 'encoding' | 'compare' | 'limits';
export type ReadingLevel = 'panel' | 'cell';
export type ReadingChannel = 'x' | 'y' | 'size' | 'color' | 'scope' | 'value';

export interface ReadingNote {
  question: ReadingQuestion;
  level: ReadingLevel;
  /** Short chip token (a value or localized label). */
  dimension?: string;
  /** The grounded explanation sentence. */
  text: string;
  /** Optional WP/ADR deep-link. */
  anchor?: { href: string; label: string };
  /** The visual channel this note describes, so the UI can tint it to match. */
  channel?: ReadingChannel;
}

export interface ReadingGuide {
  notes: ReadingNote[];
}

/** The live configuration facts a cell knows about itself, used to compose the
 *  ENCODING / LIMITS / COMPARE building blocks. All optional — only the ones
 *  relevant to the active presentation are read. Unchanged from the Phase-131
 *  contract so every existing caller keeps working. */
export interface HowToReadFacts {
  bins?: number | undefined;
  topN?: number | undefined;
  showBand?: boolean | undefined;
  /** Scatter / network visual-channel bindings. */
  x?: string | undefined;
  y?: string | undefined;
  size?: string | undefined;
  color?: string | undefined;
  netSize?: string | undefined;
  netColor?: string | undefined;
  /** Number of nodes / points / sources actually rendered, when known. */
  renderedCount?: number | undefined;
  /** Phase 133 — total distinct categories for a categorical_distribution
   *  field; with `renderedCount` (bars shown) it composes "top N of M". */
  distinctValues?: number | undefined;
  /** Phase 124 — multi-cell axis-scale state for value-axis presentations. */
  scales?: 'shared' | 'free' | undefined;
  /** Phase 123b — cross-lingual relabel state for the co-occurrence cell. */
  displayLanguage?: 'source' | 'viewer' | undefined;
  viewerLanguage?: string | undefined;
  linkedNodeCount?: number | undefined;
  labeledNodeCount?: number | undefined;
  /** Phase 125 (A1) — Pearson r for the scatter regression line. */
  r?: number | undefined;
  /** Phase 126 — per-cell override differs from the panel default. */
  configOverridden?: boolean | undefined;
}

/** The full input for the rich on-screen guide: the cell facts PLUS the panel-
 *  level dimensions and the pre-resolved labels the panel guide needs. */
export interface ReadingGuideInput extends HowToReadFacts {
  presentation: Presentation;
  subjectKind: PanelSubjectKind;
  /** Localised presentation name (e.g. "Distribution") for the concrete WHAT note. */
  presentationLabel?: string | undefined;
  /** Pre-resolved subject labels (resolved by the Svelte caller). */
  metricLabel?: string | undefined;
  fieldLabel?: string | undefined;
  fieldDescription?: string | null | undefined;
  /** Panel-level config dimensions. */
  composition?: 'merged' | 'split' | 'overlay' | undefined;
  normalization?: 'raw' | 'zscore' | 'percentile' | undefined;
  resolution?: string | undefined;
  windowLabel?: string | undefined;
  facetFieldLabel?: string | undefined;
  probeCount?: number | undefined;
  sourceCount?: number | undefined;
}

/** Localized built-in template lines — the fallback when the content catalog
 *  has no `howto_<presentation>` entry. The catalog entry, when present,
 *  supersedes these via `templateBase`. */
const FALLBACK_TEMPLATES: Record<Presentation, () => string> = {
  distribution: () => m.cells_howto_tmpl_distribution(),
  time_series: () => m.cells_howto_tmpl_time_series(),
  cooccurrence_network: () => m.cells_howto_tmpl_cooccurrence_network(),
  topic_distribution: () => m.cells_howto_tmpl_topic_distribution(),
  topic_evolution: () => m.cells_howto_tmpl_topic_evolution(),
  metric_scatter: () => m.cells_howto_tmpl_metric_scatter(),
  revision_activity: () => m.cells_howto_tmpl_revision_activity(),
  revision_timeline: () => m.cells_howto_tmpl_revision_timeline(),
  revision_discourse_shift: () => m.cells_howto_tmpl_revision_discourse_shift(),
  cross_probe_lead_lag: () => m.cells_howto_tmpl_cross_probe_lead_lag(),
  revision_edit_clusters: () => m.cells_howto_tmpl_revision_edit_clusters(),
  categorical_distribution: () => m.cells_howto_tmpl_categorical_distribution(),
  correlation_matrix: () => m.cells_howto_tmpl_correlation_matrix(),
  cross_tab: () => m.cells_howto_tmpl_cross_tab(),
  metric_lead_lag: () => m.cells_howto_tmpl_metric_lead_lag(),
  parallel_coordinates: () => m.cells_howto_tmpl_parallel_coordinates(),
  sankey: () => m.cells_howto_tmpl_sankey()
};

function networkSizeLabel(v: string): string {
  switch (v) {
    case 'degree':
      return m.cells_howto_net_size_degree();
    case 'metric':
      return m.cells_howto_net_size_metric();
    case 'total_count':
    default:
      return m.cells_howto_net_size_total();
  }
}

function networkColorLabel(v: string): string {
  switch (v) {
    case 'presence':
      return m.cells_howto_net_color_presence();
    case 'source_overlay':
      return m.cells_howto_net_color_overlay();
    case 'uniform':
      return m.cells_howto_net_color_uniform();
    case 'metric':
      return m.cells_howto_net_color_metric();
    case 'label':
    default:
      return m.cells_howto_net_color_label();
  }
}

/** The presentation-derived building blocks — ENCODING / LIMITS / per-cell
 *  COMPARE. Tagged by question, by visual channel (where relevant), and by LEVEL:
 *  config/static notes (shared across the panel's cells) are `level:'panel'`;
 *  runtime-data notes (this cell's r / counts / override) are `level:'cell'`.
 *  Extracted verbatim (in text + push order) from the original `composeHowToRead`
 *  switch so the legacy export path stays byte-identical:
 *  `composeHowToRead = [base, ...buildCellNotes(...).text]`. */
export function buildCellNotes(presentation: Presentation, facts: HowToReadFacts): ReadingNote[] {
  const out: ReadingNote[] = [];
  // `enc`/`lim` default to level:'panel' — config/static notes that are SHARED
  // across every cell of a panel (bins, channels, "each bar is a bin"), so the
  // panel Reading Guide shows them once. Pass level:'cell' for RUNTIME-DATA notes
  // that genuinely differ cell-to-cell (this cell's Pearson r, its rendered/
  // truncation counts), which the per-cell delta carries. The push ORDER is
  // unchanged from the original switch, so the legacy `composeHowToRead` export
  // (which flattens all notes regardless of level) stays byte-identical.
  const enc = (text: string, level: ReadingLevel = 'panel', channel?: ReadingChannel): void => {
    out.push(
      channel
        ? { question: 'encoding', level, text, channel }
        : { question: 'encoding', level, text }
    );
  };
  const lim = (text: string, level: ReadingLevel = 'panel'): void => {
    out.push({ question: 'limits', level, text });
  };

  switch (presentation) {
    case 'distribution':
      enc(m.cells_howto_bl_dist_bars());
      if (facts.bins !== undefined) enc(m.cells_howto_bl_dist_bins({ bins: facts.bins }));
      enc(m.cells_howto_bl_dist_median());
      break;
    case 'time_series':
      enc(m.cells_howto_bl_ts_slope());
      enc(
        facts.showBand === false
          ? m.cells_howto_bl_ts_band_hidden()
          : m.cells_howto_bl_ts_band_shown()
      );
      break;
    case 'cooccurrence_network':
      enc(m.cells_howto_bl_net_dots());
      if (facts.topN !== undefined) enc(m.cells_howto_bl_net_topn({ topN: facts.topN }));
      if (facts.netSize)
        enc(m.cells_howto_bl_net_size({ label: networkSizeLabel(facts.netSize) }), 'panel', 'size');
      if (facts.netColor)
        enc(
          m.cells_howto_bl_net_color({ label: networkColorLabel(facts.netColor) }),
          'panel',
          'color'
        );
      enc(m.cells_howto_bl_net_spread());
      if (facts.displayLanguage === 'viewer' && facts.viewerLanguage) {
        const labeled = facts.labeledNodeCount ?? 0;
        enc(
          labeled === 1
            ? m.cells_howto_bl_net_relabel_one({ labeled, viewerLanguage: facts.viewerLanguage })
            : m.cells_howto_bl_net_relabel_other({ labeled, viewerLanguage: facts.viewerLanguage }),
          'cell'
        );
      } else if ((facts.linkedNodeCount ?? 0) > 0) {
        const linked = facts.linkedNodeCount ?? 0;
        enc(
          linked === 1
            ? m.cells_howto_bl_net_srclang_one({ linked })
            : m.cells_howto_bl_net_srclang_other({ linked }),
          'cell'
        );
      }
      break;
    case 'metric_scatter':
      enc(m.cells_howto_bl_scatter_cloud());
      if (facts.x && facts.y)
        enc(m.cells_howto_bl_scatter_xy({ x: facts.x, y: facts.y }), 'panel', 'x');
      if (facts.size) enc(m.cells_howto_bl_scatter_size({ size: facts.size }), 'panel', 'size');
      if (facts.color)
        enc(m.cells_howto_bl_scatter_color({ color: facts.color }), 'panel', 'color');
      if (facts.r !== undefined) {
        const mag = Math.abs(facts.r);
        const strength =
          mag >= 0.7
            ? m.cells_howto_scatter_strength_strong()
            : mag >= 0.4
              ? m.cells_howto_scatter_strength_moderate()
              : mag >= 0.2
                ? m.cells_howto_scatter_strength_weak()
                : m.cells_howto_scatter_strength_negligible();
        const dir =
          facts.r > 0
            ? m.cells_howto_scatter_dir_positive()
            : facts.r < 0
              ? m.cells_howto_scatter_dir_negative()
              : m.cells_howto_scatter_dir_flat();
        enc(m.cells_howto_bl_scatter_r({ r: facts.r.toFixed(2), strength, dir }), 'cell');
      }
      if (facts.renderedCount !== undefined) {
        const count = facts.renderedCount;
        enc(
          count === 1
            ? m.cells_howto_bl_scatter_plotted_one({ count })
            : m.cells_howto_bl_scatter_plotted_other({ count }),
          'cell'
        );
      }
      break;
    case 'correlation_matrix':
      enc(m.cells_howto_bl_corr_offdiag());
      if (facts.renderedCount !== undefined) {
        const count = facts.renderedCount;
        enc(
          count === 1
            ? m.cells_howto_bl_corr_count_one({ count })
            : m.cells_howto_bl_corr_count_other({ count }),
          'cell'
        );
      }
      break;
    case 'cross_tab':
      enc(m.cells_howto_bl_ct_bars());
      if (facts.renderedCount !== undefined) {
        const count = facts.renderedCount;
        const of =
          facts.distinctValues !== undefined && facts.distinctValues > count
            ? m.cells_howto_bl_ct_of({ total: facts.distinctValues })
            : '';
        // "top N of M" is a runtime truncation disclosure → cell-level LIMITS.
        lim(
          count === 1
            ? m.cells_howto_bl_ct_count_one({ count, of })
            : m.cells_howto_bl_ct_count_other({ count, of }),
          'cell'
        );
      }
      break;
    case 'metric_lead_lag':
      if (facts.x && facts.y) enc(m.cells_howto_bl_mll_peak({ x: facts.x, y: facts.y }));
      if (facts.renderedCount !== undefined) {
        const count = facts.renderedCount;
        enc(
          count === 1
            ? m.cells_howto_bl_mll_count_one({ count })
            : m.cells_howto_bl_mll_count_other({ count }),
          'cell'
        );
      }
      break;
    case 'parallel_coordinates':
      enc(m.cells_howto_bl_pc_axes());
      if (facts.renderedCount !== undefined) {
        const count = facts.renderedCount;
        enc(
          count === 1
            ? m.cells_howto_bl_pc_count_one({ count })
            : m.cells_howto_bl_pc_count_other({ count }),
          'cell'
        );
      }
      break;
    case 'sankey':
      enc(m.cells_howto_bl_sankey_bands());
      break;
    case 'cross_probe_lead_lag':
      enc(m.cells_howto_bl_cpll_peak());
      if (facts.renderedCount !== undefined) {
        const count = facts.renderedCount;
        enc(
          count === 1
            ? m.cells_howto_bl_cpll_count_one({ count })
            : m.cells_howto_bl_cpll_count_other({ count }),
          'cell'
        );
      }
      break;
    case 'categorical_distribution':
      enc(m.cells_howto_bl_cat_bars());
      if (
        facts.renderedCount !== undefined &&
        facts.distinctValues !== undefined &&
        facts.distinctValues > facts.renderedCount
      ) {
        // top-N truncation → cell-level LIMITS (runtime count).
        lim(
          m.cells_howto_bl_cat_topn({ count: facts.renderedCount, total: facts.distinctValues }),
          'cell'
        );
      }
      lim(m.cells_howto_bl_cat_empty());
      break;
    case 'topic_distribution':
    case 'topic_evolution':
    case 'revision_activity':
    case 'revision_timeline':
    case 'revision_discourse_shift':
    case 'revision_edit_clusters':
      // No extra config levers yet; the template line stands alone.
      break;
  }

  // Phase 124 — shared/free axis disclosure for value-axis presentations rendered
  // as >1 cell. The axis MODE is a panel-wide setting (identical for every cell),
  // so it is a PANEL-level COMPARE note shown once in the panel guide, never
  // duplicated under each cell (Phase 148f).
  if (
    facts.scales &&
    (presentation === 'distribution' ||
      presentation === 'time_series' ||
      presentation === 'metric_scatter')
  ) {
    out.push({
      question: 'compare',
      level: 'panel',
      text:
        facts.scales === 'shared' ? m.cells_howto_bl_scale_shared() : m.cells_howto_bl_scale_free()
    });
  }

  // Phase 126 — per-cell override disclosure → cell-level LIMITS (this cell is
  // not comparable to its siblings).
  if (facts.configOverridden) lim(m.cells_howto_bl_override(), 'cell');

  return out.filter((n) => n.text.length > 0);
}

/** LEGACY export-path composer — preserved byte-for-byte. The template/fallback
 *  line first, then the cell building blocks' text in order. Consumed by every
 *  cell's CellExport `exportPayload.howToRead`. Re-exported from `how-to-read.ts`. */
export function composeHowToRead(
  presentation: Presentation,
  facts: HowToReadFacts,
  templateBase?: string | null
): string[] {
  const out: string[] = [];
  const base =
    templateBase && templateBase.trim().length > 0
      ? templateBase.trim()
      : FALLBACK_TEMPLATES[presentation]();
  if (base) out.push(base);
  for (const n of buildCellNotes(presentation, facts)) out.push(n.text);
  return out.filter((s) => s.length > 0);
}

/** The PANEL-level (shared) building blocks: WHAT / MEASURE / SAMPLE / COMPARE.
 *  These describe the whole panel and render once, not per cell. */
function buildPanelNotes(input: ReadingGuideInput, templateBase?: string | null): ReadingNote[] {
  const out: ReadingNote[] = [];

  // WHAT — the concrete chosen view, then its methodological template, then the
  // composition (so the reader sees exactly what they picked, not vague prose).
  if (input.presentationLabel) {
    out.push({
      question: 'what',
      level: 'panel',
      dimension: input.presentationLabel,
      text: m.rg_what_presentation({ presentation: input.presentationLabel })
    });
  }
  const base =
    templateBase && templateBase.trim().length > 0
      ? templateBase.trim()
      : FALLBACK_TEMPLATES[input.presentation]();
  if (base) out.push({ question: 'what', level: 'panel', text: base });
  if (input.composition) {
    const text =
      input.composition === 'merged'
        ? m.rg_what_merged()
        : input.composition === 'overlay'
          ? m.rg_what_overlay()
          : m.rg_what_split();
    out.push({ question: 'what', level: 'panel', dimension: input.composition, text });
  }

  // MEASURE — the metric, the field, or the view's intrinsic subject.
  if (input.subjectKind === 'metric' && input.metricLabel) {
    out.push({
      question: 'measure',
      level: 'panel',
      dimension: input.metricLabel,
      text: m.rg_measure_metric({ label: input.metricLabel }),
      channel: 'value'
    });
  } else if (input.subjectKind === 'field') {
    out.push({
      question: 'measure',
      level: 'panel',
      dimension: input.fieldLabel ?? '',
      text: input.fieldDescription ?? m.rg_measure_field_generic()
    });
  } else {
    // 'none' — the subject lives in the presentation, not Panel.metric.
    out.push({ question: 'measure', level: 'panel', text: intrinsicSubject(input.presentation) });
  }

  // SAMPLE — scope, window, faceting.
  if (input.sourceCount !== undefined || input.probeCount !== undefined) {
    out.push({
      question: 'sample',
      level: 'panel',
      channel: 'scope',
      text: m.rg_sample_scope({
        sources: input.sourceCount ?? 0,
        probes: input.probeCount ?? 0
      })
    });
  }
  out.push({
    question: 'sample',
    level: 'panel',
    text: input.windowLabel
      ? m.rg_sample_window({ window: input.windowLabel })
      : m.rg_sample_window_full()
  });
  if (input.facetFieldLabel) {
    out.push({
      question: 'sample',
      level: 'panel',
      dimension: input.facetFieldLabel,
      text: m.rg_sample_facet({ field: input.facetFieldLabel })
    });
  }

  // ENCODING — resolution is a panel-level temporal bucketing (Episteme).
  if (input.resolution) {
    out.push({
      question: 'encoding',
      level: 'panel',
      dimension: input.resolution,
      text: m.rg_encoding_resolution({ resolution: input.resolution })
    });
  }

  // COMPARE — normalization mode (equivalence-gated for z-score/percentile).
  if (input.normalization && input.normalization !== 'raw') {
    out.push({
      question: 'compare',
      level: 'panel',
      dimension: input.normalization,
      text: input.normalization === 'zscore' ? m.rg_compare_zscore() : m.rg_compare_percentile(),
      anchor: { href: '/reflection/wp/wp-004?section=5.3', label: 'WP-004 §5.3' }
    });
  } else {
    out.push({ question: 'compare', level: 'panel', dimension: 'raw', text: m.rg_compare_raw() });
  }

  return out.filter((n) => n.text.length > 0);
}

/** The intrinsic subject of a metric-agnostic ('none') view. */
function intrinsicSubject(presentation: Presentation): string {
  switch (presentation) {
    case 'topic_distribution':
    case 'topic_evolution':
      return m.rg_measure_topics();
    case 'cooccurrence_network':
      return m.rg_measure_entities();
    case 'revision_activity':
    case 'revision_timeline':
    case 'revision_discourse_shift':
    case 'revision_edit_clusters':
      return m.rg_measure_revisions();
    default:
      // scatter / correlation / parallel-coords / metric-lead-lag / cross-probe.
      return m.rg_measure_multi();
  }
}

/** Compose the full reading guide (panel notes + cell notes), grouped by the six
 *  questions at render time. Pure + node-testable. */
export function composeReadingGuide(
  input: ReadingGuideInput,
  templateBase?: string | null
): ReadingGuide {
  const notes = [
    ...buildPanelNotes(input, templateBase),
    ...buildCellNotes(input.presentation, input)
  ];
  return { notes };
}

/** Flatten a guide to ordered text lines, optionally filtered by level. */
export function readingGuideLines(guide: ReadingGuide, level?: ReadingLevel): string[] {
  return guide.notes.filter((n) => !level || n.level === level).map((n) => n.text);
}
