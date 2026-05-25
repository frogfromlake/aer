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
    'Each point is one article, positioned by two metrics — read the cloud for correlation, clusters, and outliers.'
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
      if (facts.renderedCount !== undefined) {
        out.push(
          `${facts.renderedCount} article${facts.renderedCount === 1 ? '' : 's'} had both metrics and could be plotted.`
        );
      }
      break;
    case 'topic_distribution':
    case 'topic_evolution':
      // No extra config levers yet; the template line stands alone.
      break;
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
    case 'total_count':
    default:
      return 'how often it is mentioned alongside others (more = bigger)';
  }
}

function networkColorLabel(v: string): string {
  switch (v) {
    case 'presence':
      return 'how many sources mention it (one colour per count)';
    case 'uniform':
      return 'nothing — all dots share one colour';
    case 'label':
    default:
      return 'what kind of thing it is — person, place, or organisation';
  }
}
