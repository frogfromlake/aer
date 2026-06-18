// "How to read this" composer — Phase 131 (localized Phase 144c).
//
// Every configurable cell renders a short "how to read" note. The note is
// COMPOSED, not hardcoded: a per-presentation template line (sourced from the
// content-catalog Dual-Register `view_modes/howto_<presentation>` entry when
// available, else the localized built-in fallback below) followed by
// config-derived building blocks that reflect the cell's *live* configuration
// (bin count, top-N, bound visual channels, band visibility).
//
// All user-visible strings are Paraglide messages (`cells_howto_*`), so the
// note follows the UI locale; `m.*()` reads the locale rune per call, making it
// reactive to a language switch. The module stays pure (vitest pins the
// composition without a Svelte render) by importing `m` from the RELATIVE
// paraglide path — node-env vitest does not resolve `$lib` and falls back to the
// base locale 'en', so the English assertions hold unchanged.

import { m } from '../paraglide/messages.js';
import type { Presentation } from '$lib/state/url-internals';

/** The live configuration facts a cell knows about itself, used to compose
 *  the config-derived building blocks. All optional — only the ones relevant
 *  to the active presentation are read. */
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
  /** Phase 124 — multi-cell axis-scale state for value-axis presentations:
   *  'shared' (this cell is on the panel's union axis, directly comparable),
   *  'free' (independent axis). Absent for single-cell panels — no note. */
  scales?: 'shared' | 'free' | undefined;
  /** Phase 123b — cross-lingual relabel state for the co-occurrence cell. */
  displayLanguage?: 'source' | 'viewer' | undefined;
  viewerLanguage?: string | undefined;
  linkedNodeCount?: number | undefined;
  labeledNodeCount?: number | undefined;
  /** Phase 125 (A1) — Pearson r for the scatter regression line (correlation
   *  strength/direction of the x↔y relationship). Absent when undefined. */
  r?: number | undefined;
  /** Phase 126 — this cell is on a per-cell override that differs from the
   *  panel default, so it is not directly comparable to its sibling cells. */
  configOverridden?: boolean | undefined;
}

/** Localized built-in template lines — the fallback when the content catalog
 *  has no `howto_<presentation>` entry (e.g. `cross_probe_lead_lag`). The
 *  catalog entry, when present, supersedes these via `templateBase`. */
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

/** Compose the "how to read" note as an ordered list of sentences: the
 *  template line first, then config-derived building blocks. `templateBase`
 *  (from the content catalog) overrides the built-in template when supplied. */
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

  switch (presentation) {
    case 'distribution':
      out.push(m.cells_howto_bl_dist_bars());
      if (facts.bins !== undefined) out.push(m.cells_howto_bl_dist_bins({ bins: facts.bins }));
      out.push(m.cells_howto_bl_dist_median());
      break;
    case 'time_series':
      out.push(m.cells_howto_bl_ts_slope());
      out.push(
        facts.showBand === false
          ? m.cells_howto_bl_ts_band_hidden()
          : m.cells_howto_bl_ts_band_shown()
      );
      break;
    case 'cooccurrence_network':
      out.push(m.cells_howto_bl_net_dots());
      if (facts.topN !== undefined) out.push(m.cells_howto_bl_net_topn({ topN: facts.topN }));
      if (facts.netSize)
        out.push(m.cells_howto_bl_net_size({ label: networkSizeLabel(facts.netSize) }));
      if (facts.netColor)
        out.push(m.cells_howto_bl_net_color({ label: networkColorLabel(facts.netColor) }));
      out.push(m.cells_howto_bl_net_spread());
      // Phase 123b — cross-lingual relabel state.
      if (facts.displayLanguage === 'viewer' && facts.viewerLanguage) {
        const labeled = facts.labeledNodeCount ?? 0;
        out.push(
          labeled === 1
            ? m.cells_howto_bl_net_relabel_one({ labeled, viewerLanguage: facts.viewerLanguage })
            : m.cells_howto_bl_net_relabel_other({ labeled, viewerLanguage: facts.viewerLanguage })
        );
      } else if ((facts.linkedNodeCount ?? 0) > 0) {
        const linked = facts.linkedNodeCount ?? 0;
        out.push(
          linked === 1
            ? m.cells_howto_bl_net_srclang_one({ linked })
            : m.cells_howto_bl_net_srclang_other({ linked })
        );
      }
      break;
    case 'metric_scatter':
      out.push(m.cells_howto_bl_scatter_cloud());
      if (facts.x && facts.y) out.push(m.cells_howto_bl_scatter_xy({ x: facts.x, y: facts.y }));
      if (facts.size) out.push(m.cells_howto_bl_scatter_size({ size: facts.size }));
      if (facts.color) out.push(m.cells_howto_bl_scatter_color({ color: facts.color }));
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
        out.push(m.cells_howto_bl_scatter_r({ r: facts.r.toFixed(2), strength, dir }));
      }
      if (facts.renderedCount !== undefined) {
        const count = facts.renderedCount;
        out.push(
          count === 1
            ? m.cells_howto_bl_scatter_plotted_one({ count })
            : m.cells_howto_bl_scatter_plotted_other({ count })
        );
      }
      break;
    case 'correlation_matrix':
      out.push(m.cells_howto_bl_corr_offdiag());
      if (facts.renderedCount !== undefined) {
        const count = facts.renderedCount;
        out.push(
          count === 1
            ? m.cells_howto_bl_corr_count_one({ count })
            : m.cells_howto_bl_corr_count_other({ count })
        );
      }
      break;
    case 'cross_tab':
      out.push(m.cells_howto_bl_ct_bars());
      if (facts.renderedCount !== undefined) {
        const count = facts.renderedCount;
        const of =
          facts.distinctValues !== undefined && facts.distinctValues > count
            ? m.cells_howto_bl_ct_of({ total: facts.distinctValues })
            : '';
        out.push(
          count === 1
            ? m.cells_howto_bl_ct_count_one({ count, of })
            : m.cells_howto_bl_ct_count_other({ count, of })
        );
      }
      break;
    case 'metric_lead_lag':
      if (facts.x && facts.y) out.push(m.cells_howto_bl_mll_peak({ x: facts.x, y: facts.y }));
      if (facts.renderedCount !== undefined) {
        const count = facts.renderedCount;
        out.push(
          count === 1
            ? m.cells_howto_bl_mll_count_one({ count })
            : m.cells_howto_bl_mll_count_other({ count })
        );
      }
      break;
    case 'parallel_coordinates':
      out.push(m.cells_howto_bl_pc_axes());
      if (facts.renderedCount !== undefined) {
        const count = facts.renderedCount;
        out.push(
          count === 1
            ? m.cells_howto_bl_pc_count_one({ count })
            : m.cells_howto_bl_pc_count_other({ count })
        );
      }
      break;
    case 'sankey':
      out.push(m.cells_howto_bl_sankey_bands());
      break;
    case 'cross_probe_lead_lag':
      out.push(m.cells_howto_bl_cpll_peak());
      if (facts.renderedCount !== undefined) {
        const count = facts.renderedCount;
        out.push(
          count === 1
            ? m.cells_howto_bl_cpll_count_one({ count })
            : m.cells_howto_bl_cpll_count_other({ count })
        );
      }
      break;
    case 'categorical_distribution':
      out.push(m.cells_howto_bl_cat_bars());
      if (
        facts.renderedCount !== undefined &&
        facts.distinctValues !== undefined &&
        facts.distinctValues > facts.renderedCount
      ) {
        out.push(
          m.cells_howto_bl_cat_topn({ count: facts.renderedCount, total: facts.distinctValues })
        );
      }
      out.push(m.cells_howto_bl_cat_empty());
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

  // Phase 124 — shared-axis disclosure for value-axis presentations rendered
  // as >1 cell. Absent `scales` (single-cell or non-value-axis) → no line.
  if (
    facts.scales &&
    (presentation === 'distribution' ||
      presentation === 'time_series' ||
      presentation === 'metric_scatter')
  ) {
    out.push(
      facts.scales === 'shared' ? m.cells_howto_bl_scale_shared() : m.cells_howto_bl_scale_free()
    );
  }

  // Phase 126 — per-cell override disclosure. When this cell's configuration
  // differs from the panel default it is not directly comparable to its
  // siblings; say so explicitly (comparison-as-default, Brief §1.3).
  if (facts.configOverridden) out.push(m.cells_howto_bl_override());

  return out.filter((s) => s.length > 0);
}

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
