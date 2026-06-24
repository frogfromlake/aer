// Cell subjects — Phase 148g.
//
// A cell's "subjects" are the metrics and metadata fields it ACTUALLY binds,
// across every binding slot a presentation can use — not just the single
// `Panel.metric` slot. The single-metric views (distribution, time_series)
// bind one metric there; the multivariate / channel-driven views bind their
// real subjects elsewhere:
//   metric_scatter        → channels.x / .y / .size / .color (metric OR field)
//   correlation_matrix    → metricSet[]
//   parallel_coordinates  → metricSet[]
//   cross_tab             → Panel.metric (group-by field) + channels.x (metric)
//   metric_lead_lag       → channels.x (leads) / channels.y (lags)
//   cooccurrence_network  → channels.netMetric / .netColorMetric
//   categorical_distribution → Panel.metric (a categorical field)
//   sankey                → fieldChain[]
//   + any faceting view   → facetField (a categorical field)
//
// The methodology surface (MeasureDetail) enumerates these so every bound
// metric and field gets its own methodology block — "every presentation ×
// every metric × every field", not just the two single-metric views.
//
// This module is PURE and KIND-AGNOSTIC: it returns each subject's machine
// name + the role(s) it plays in the cell. Whether a name is a real metric or
// a categorical field is resolved by the consumer (it depends on the reactive
// `/metrics/available` registry, which is not pure) — see SubjectMethodology.

import type { Presentation } from '$lib/state/url-internals';
import type { Panel } from '$lib/state/url-internals';
import type { PresentationDefinition } from './registry';

/** The role a subject plays in a cell. Drives the per-block role label. */
export type SubjectRole =
  | 'primary' // the single bound metric of distribution / time_series
  | 'x'
  | 'y'
  | 'size'
  | 'color'
  | 'set' // correlation-matrix / parallel-coordinates set member
  | 'groupBy' // cross-tab group-by field
  | 'aggregated' // cross-tab aggregated metric
  | 'leading' // metric lead-lag — the leading metric
  | 'lagging' // metric lead-lag — the lagging metric
  | 'nodeSize' // co-occurrence node-size metric
  | 'nodeColor' // co-occurrence node-colour metric
  | 'field' // categorical-distribution category field
  | 'chain' // sankey field-chain member
  | 'facet'; // small-multiples facet field

/** One bound subject: a machine name + the role(s) it plays. A name bound to
 *  more than one channel (e.g. a scatter metric on both x and colour) appears
 *  once with both roles. */
export interface CellSubject {
  name: string;
  roles: SubjectRole[];
}

/** Push a (name, role) pair, skipping empties and merging roles for a name
 *  already seen so each subject renders exactly one methodology block. */
function add(into: CellSubject[], name: string | undefined | null, role: SubjectRole): void {
  if (!name) return;
  const existing = into.find((s) => s.name === name);
  if (existing) {
    if (!existing.roles.includes(role)) existing.roles.push(role);
    return;
  }
  into.push({ name, roles: [role] });
}

/** The metrics + fields a cell actually binds, in reading order, deduped by
 *  name. Empty for views with no per-subject methodology (revision_*,
 *  cross_probe_lead_lag) — those rely on the per-view howto alone. */
export function cellSubjects(presentation: PresentationDefinition, panel: Panel): CellSubject[] {
  const out: CellSubject[] = [];
  const ch = panel.channels;
  const view: Presentation = presentation.id;

  switch (view) {
    case 'distribution':
    case 'time_series':
      // Single-metric views — the subject is the panel metric.
      add(out, panel.metric, 'primary');
      break;
    case 'metric_scatter':
      add(out, ch?.x, 'x');
      add(out, ch?.y, 'y');
      add(out, ch?.size, 'size');
      add(out, ch?.color, 'color');
      break;
    case 'correlation_matrix':
    case 'parallel_coordinates':
      for (const m of panel.metricSet ?? []) add(out, m, 'set');
      break;
    case 'cross_tab':
      // Group-by categorical field (Panel.metric) + the aggregated metric
      // (channels.x — the crossMetric lever).
      add(out, panel.metric, 'groupBy');
      add(out, ch?.x, 'aggregated');
      break;
    case 'metric_lead_lag':
      add(out, ch?.x, 'leading');
      add(out, ch?.y, 'lagging');
      break;
    case 'cooccurrence_network':
      // Node size / colour only carry a metric when their channel is bound to
      // 'metric'. Colour falls back to the size metric when its own is unset.
      if (ch?.netSize === 'metric') add(out, ch?.netMetric, 'nodeSize');
      if (ch?.netColor === 'metric') add(out, ch?.netColorMetric ?? ch?.netMetric, 'nodeColor');
      break;
    case 'categorical_distribution':
      // The category field rides in Panel.metric (usesMetadataField).
      add(out, panel.metric, 'field');
      break;
    case 'sankey':
      for (const f of panel.fieldChain ?? []) add(out, f, 'chain');
      break;
    default:
      // revision_activity / revision_timeline / revision_discourse_shift /
      // revision_edit_clusters / cross_probe_lead_lag / topic_* — no
      // per-subject methodology; the per-view howto carries the method.
      break;
  }

  // Any faceting view adds its facet field as a subject (the metadata field
  // the small-multiples split is cut on).
  if (presentation.supportsFaceting && panel.facetField) add(out, panel.facetField, 'facet');

  return out;
}
