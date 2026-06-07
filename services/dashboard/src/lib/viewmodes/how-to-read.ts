// "How to read this" composer — Phase 131.
//
// Every configurable cell renders a short "how to read" note. The note is
// COMPOSED, not hardcoded: a per-presentation template line (sourced from the
// content-catalog Dual-Register `view_mode/howto_<presentation>` entry when
// available, else a built-in fallback) followed by config-derived building
// blocks that reflect the cell's *live* configuration (bin count, top-N,
// bound visual channels, band visibility).
//
// This module is pure so vitest can pin the composition without a Svelte
// render or a network round-trip; the cell component fetches the catalog
// template and passes it (plus the live facts) in.

import type { ViewMode } from '$lib/state/url-internals';

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

/** Built-in per-presentation template lines — the fallback when the content
 *  catalog has no `howto_<presentation>` entry yet. Kept terse; the catalog
 *  entry, when present, supersedes these. */
const FALLBACK_TEMPLATES: Record<ViewMode, string> = {
  distribution:
    'This is the full distribution of the metric across the scope — the shape (not just the average) of how values spread.',
  time_series:
    'This traces the metric over time. Each point is a bucket mean; read the slope, not single points.',
  cooccurrence_network:
    'Nodes are entities; an edge means two entities were named in the same article. Read clusters as recurring framings.',
  topic_distribution:
    'Each ridge is a topic; its area is how much of the corpus discusses it right now (a synchronic snapshot).',
  topic_evolution:
    'Each stream is a topic; its width over time shows how its share of the discourse rises and falls.',
  metric_scatter:
    'Each point is one article, positioned by two metrics — read the cloud for correlation, clusters, and outliers.',
  revision_activity:
    'One bar per source — how many silent edits we observed in the active window. Wayback CDX captures third-party-witnessed edits; sitemap-lastmod jumps capture publisher-side re-listings (republication trigger).',
  revision_timeline:
    'Edit activity over time. Each point is a per-source bucket count: rising = the source is editing more often; falling = it has settled.',
  revision_discourse_shift:
    "Each point is one source's mean sentiment change for that bucket (later minus earlier), re-extracted from each snapshot version. Above zero = edits read more positively; hover for the semantic shift and entity churn. Provisional measures.",
  cross_probe_lead_lag:
    'Each point is one time-shift (lag) between the two probes; the height is how strongly their hourly publication rhythms line up at that shift. The tallest point is the lead-lag: a shift to the right means the compared probe follows the reference.',
  revision_edit_clusters:
    'Each row is an entity that two or more sources silently edited in the same time bucket — a cross-source coincidence on the same name. A disclosed coincidence, not a causal claim; widen the bucket or raise the source threshold to tighten it.',
  categorical_distribution:
    'Each bar is one category value of the chosen metadata field; its height is how many articles in the scope carry that value. Read the shape, not single bars — which categories dominate the corpus.',
  correlation_matrix:
    'Each cell is the Pearson correlation of two metrics over the scope; warm = they rise together, cool = one falls as the other rises, pale = unrelated. The diagonal is always 1.',
  cross_tab:
    'Each bar is one category of the metadata field; its length/colour is the mean of the chosen metric among the articles in that category. Read which categories run high or low on the metric.',
  metric_lead_lag:
    'Each point is one time-shift (lag) between two metrics; the height is how strongly they line up at that shift. The tallest point is the lead-lag: a shift right means the second metric follows the first.',
  parallel_coordinates:
    'Each line is one article threaded across several metric axes (each axis scaled to its own min–max). Lines that move together cluster; lines that cross show trade-offs between metrics.',
  sankey:
    'Each column is a metadata field; each band is a flow of articles from one category to the next. Thicker bands carry more articles — read where the corpus concentrates and splits.'
};

/** Compose the "how to read" note as an ordered list of sentences: the
 *  template line first, then config-derived building blocks. `templateBase`
 *  (from the content catalog) overrides the built-in template when supplied. */
export function composeHowToRead(
  presentation: ViewMode,
  facts: HowToReadFacts,
  templateBase?: string | null
): string[] {
  const out: string[] = [];
  const base =
    templateBase && templateBase.trim().length > 0
      ? templateBase.trim()
      : FALLBACK_TEMPLATES[presentation];
  if (base) out.push(base);

  switch (presentation) {
    case 'distribution':
      out.push(
        'Taller bars = more articles fall at that value. The hump is the typical article; long tails are the unusual ones.'
      );
      if (facts.bins !== undefined) {
        out.push(`Split into ${facts.bins} bars (the Bins slider) — more bars show finer detail.`);
      }
      out.push(
        'The solid line is the middle (median); the dashed lines mark the typical range (25th–75th percentile).'
      );
      break;
    case 'time_series':
      if (facts.showBand === false) {
        out.push('Read the slope, not single dots: rising = the metric is trending up over time.');
        out.push(
          'The uncertainty band is hidden — turn it on under Band to see how noisy each point is.'
        );
      } else {
        out.push('Read the slope, not single dots: rising = the metric is trending up over time.');
        out.push(
          'The shaded band is the spread within each time-bucket (±1 standard deviation): a wide band means that point hides a lot of variation, so trust it less.'
        );
      }
      break;
    case 'cooccurrence_network':
      out.push(
        'Each dot is an entity (a person, place, or organisation); a line means the two were named in the same article. Tight clumps are recurring storylines.'
      );
      if (facts.topN !== undefined) {
        out.push(`Showing the ${facts.topN} most-frequent pairs (the Top N slider).`);
      }
      out.push(channelLine('Dot size', facts.netSize, networkSizeLabel));
      out.push(channelLine('Dot colour', facts.netColor, networkColorLabel));
      out.push('Crowded into one blob? Raise the Spread slider to push the dots apart.');
      // Phase 123b — cross-lingual relabel state.
      if (facts.displayLanguage === 'viewer' && facts.viewerLanguage) {
        const labeled = facts.labeledNodeCount ?? 0;
        out.push(
          `Labels: ${labeled} Wikidata-linked ${labeled === 1 ? 'dot is' : 'dots are'} shown in the app language (${facts.viewerLanguage}); ↺ marks those whose label differs from the source form. The rest keep their source-language form. Unlinked entities are never translated.`
        );
      } else if ((facts.linkedNodeCount ?? 0) > 0) {
        out.push(
          `Labels are in each entity's source language. ${facts.linkedNodeCount} ${facts.linkedNodeCount === 1 ? 'dot is' : 'dots are'} Wikidata-linked — switch Labels to the app language to relabel that subset.`
        );
      }
      break;
    case 'metric_scatter':
      out.push(
        'Each dot is one article. If the cloud tilts up-right, the two metrics rise together; a shapeless cloud means they are unrelated.'
      );
      if (facts.x && facts.y) {
        out.push(`Left–right = ${facts.x}; up–down = ${facts.y}.`);
      }
      if (facts.size) out.push(`Bigger dots = higher ${facts.size}.`);
      if (facts.color) out.push(`Dot colour = ${facts.color} (brighter = higher).`);
      if (facts.r !== undefined) {
        const mag = Math.abs(facts.r);
        const strength =
          mag >= 0.7 ? 'strong' : mag >= 0.4 ? 'moderate' : mag >= 0.2 ? 'weak' : 'negligible';
        const dir = facts.r > 0 ? 'positive' : facts.r < 0 ? 'negative' : 'flat';
        out.push(
          `The amber line is the linear fit; r = ${facts.r.toFixed(2)} (${strength} ${dir} correlation). Correlation is not causation, and r only captures a straight-line relationship.`
        );
      }
      if (facts.renderedCount !== undefined) {
        out.push(
          `${facts.renderedCount} article${facts.renderedCount === 1 ? '' : 's'} had both metrics and could be plotted.`
        );
      }
      break;
    case 'correlation_matrix':
      out.push(
        'Read off-diagonal cells: a strong warm/cool cell flags two metrics worth inspecting as a scatter (switch this panel to the Scatter view and pick that pair). Correlation is not causation, and it only captures straight-line relationships.'
      );
      if (facts.renderedCount !== undefined) {
        out.push(
          `${facts.renderedCount} metric${facts.renderedCount === 1 ? '' : 's'} in the matrix (the Metric set lever). Cells with too few overlapping articles are drawn dashed (honest empty), never as a zero.`
        );
      }
      break;
    case 'cross_tab':
      out.push(
        'Bars are ranked by article count; the colour encodes the mean metric value (so a short bar with few articles is a thin sample — read it with care). The count and spread (±σ) are in the hover.'
      );
      if (facts.renderedCount !== undefined) {
        out.push(
          `${facts.renderedCount} categor${facts.renderedCount === 1 ? 'y' : 'ies'} shown${facts.distinctValues !== undefined && facts.distinctValues > facts.renderedCount ? ` of ${facts.distinctValues}` : ''} (the Top N lever). Categories where no article carries the metric are absent, never zero.`
        );
      }
      break;
    case 'metric_lead_lag':
      if (facts.x && facts.y) {
        out.push(
          `A peak at lag 0 means ${facts.x} and ${facts.y} move in step; a peak to the right means ${facts.y} follows ${facts.x}, to the left means it leads. Correlation is not causation.`
        );
      }
      if (facts.renderedCount !== undefined) {
        out.push(
          `${facts.renderedCount} overlapping hourly bucket${facts.renderedCount === 1 ? '' : 's'} at lag 0 (a sample-size proxy — a thin overlap is a noisy curve).`
        );
      }
      break;
    case 'parallel_coordinates':
      out.push(
        'Each axis is independently scaled to its own range, so a line high on one axis and low on the next is an article that scores high on the first metric and low on the second. Only articles carrying every metric are drawn.'
      );
      if (facts.renderedCount !== undefined) {
        out.push(
          `${facts.renderedCount} article${facts.renderedCount === 1 ? '' : 's'} drawn (those with all chosen metrics).`
        );
      }
      break;
    case 'sankey':
      out.push(
        'Bands are the top flows between each pair of fields (a long tail is dropped, not summed into a misleading band). A list-valued field can place one article in several bands, so band widths are article-flow weights, not a strict partition.'
      );
      break;
    case 'cross_probe_lead_lag':
      out.push(
        'A peak at lag 0 means the two cultures publish in step; a peak left or right means one consistently runs ahead. This is a Level-1 (temporal) comparison only — it reads when discourse happens, never how much or how positive.'
      );
      if (facts.renderedCount !== undefined) {
        out.push(
          `${facts.renderedCount} overlapping hour${facts.renderedCount === 1 ? '' : 's'} fed the correlation at lag 0 — a small overlap makes the peak noisy, so read it cautiously.`
        );
      }
      break;
    case 'categorical_distribution':
      out.push(
        'Taller bars = more articles carry that category value. Bars are ranked by article count; the Top N slider sets how many you see and the total distinct-value count is always disclosed, so any long tail is stated in words rather than folded into a misleading bar.'
      );
      if (
        facts.renderedCount !== undefined &&
        facts.distinctValues !== undefined &&
        facts.distinctValues > facts.renderedCount
      ) {
        out.push(
          `Showing the top ${facts.renderedCount} of ${facts.distinctValues} distinct values (the Top N slider).`
        );
      }
      out.push(
        'An empty chart has two possible causes — the field is structurally absent (Negative Space, a publisher choice) or no captured article in the window carries it; the per-source metadata coverage panel distinguishes them. Either way it is never coerced to a zero bar.'
      );
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
      facts.scales === 'shared'
        ? 'Scale: shared across this panel’s cells — identical values sit at identical positions, so you can compare cells directly.'
        : 'Scale: independent (free) — each cell is scaled to its own data, so read shapes within a cell, not positions across cells.'
    );
  }

  // Phase 126 — per-cell override disclosure. When this cell's configuration
  // differs from the panel default it is not directly comparable to its
  // siblings; say so explicitly (comparison-as-default, Brief §1.3).
  if (facts.configOverridden) {
    out.push(
      'This cell is on a custom configuration that differs from the panel default — read it on its own terms, not directly against its sibling cells.'
    );
  }
  return out.filter((s) => s.length > 0);
}

function channelLine(
  prefix: string,
  value: string | undefined,
  label: (v: string) => string
): string {
  if (!value) return '';
  return `${prefix} encodes ${label(value)}.`;
}

function networkSizeLabel(v: string): string {
  switch (v) {
    case 'degree':
      return 'how many different entities it connects to (more links = bigger)';
    case 'metric':
      return 'the mean of the chosen metric over the articles mentioning it (higher = bigger; grey = no such article)';
    case 'total_count':
    default:
      return 'how often it is mentioned alongside others (more = bigger)';
  }
}

function networkColorLabel(v: string): string {
  switch (v) {
    case 'presence':
      return 'how many sources mention it (one colour per count)';
    case 'source_overlay':
      return 'which source it came from — one colour per source, grey when shared (Phase 131a)';
    case 'uniform':
      return 'nothing — all dots share one colour';
    case 'metric':
      return 'the mean of the chosen metric over its articles (blue→amber low→high; grey = no such article)';
    case 'label':
    default:
      return 'what kind of thing it is — person, place, or organisation';
  }
}
