// Task B — UI-friendly display labels for metric and categorical-field machine
// names, used everywhere a name is shown to the user (pickers, cell headers,
// disclosure notes, reflection pages). Machine names stay load-bearing for ids,
// anchors, deep-links and query state — ONLY visible text is relabelled.
//
// Two sources, by nature of the data:
//   • metricLabel(name) — metrics are an extensible set (new extractors add
//     them), so their localized label is served by the BFF on /metrics/available
//     (from the per-locale content catalogue) and registered here. A reactive
//     module-level $state map lets any deep cell read the label without threading
//     it through props. Falls back to humanizeMachineName for metrics that have
//     no catalogue entry yet (so a new extractor never renders blank).
//   • fieldLabel(name) — categorical metadata fields are a FIXED set, so their
//     localized label lives in Paraglide (like their Task-C descriptions), with
//     the same humanize fallback.
import * as m from '../paraglide/messages.js';
import { humanizeMachineName } from '../labels-core';

export { humanizeMachineName };

// Reactive registry of localized metric labels, seeded from /metrics/available.
const metricLabels = $state<Record<string, string>>({});

// Register (or refresh on locale change) the labels carried by an
// /metrics/available response. Safe to call repeatedly; only non-empty labels
// overwrite, so a locale that lacks a label keeps the previous render readable.
export function registerMetricLabels(
  dtos: ReadonlyArray<{ metricName: string; displayLabel?: string | null }>
): void {
  for (const d of dtos) {
    if (d.displayLabel) metricLabels[d.metricName] = d.displayLabel;
  }
}

// The display label for a metric machine name. Reactive: re-renders when the
// registry updates (e.g. after a locale switch re-fetches the labels).
export function metricLabel(name: string): string {
  if (!name) return '';
  return metricLabels[name] ?? humanizeMachineName(name);
}

// Localized labels for the fixed categorical-field set (same fields as the
// Task-C metadata catalogue). Paraglide reads the UI-locale rune per call.
const FIELD_LABELS: Record<string, () => string> = {
  published_date: m.metadata_field_label_published_date,
  modified_date: m.metadata_field_label_modified_date,
  author: m.metadata_field_label_author,
  description: m.metadata_field_label_description,
  categories: m.metadata_field_label_categories,
  tags: m.metadata_field_label_tags,
  section: m.metadata_field_label_section,
  image_url: m.metadata_field_label_image_url,
  article_type: m.metadata_field_label_article_type,
  word_count: m.metadata_field_label_word_count,
  comment_count: m.metadata_field_label_comment_count,
  comment_url: m.metadata_field_label_comment_url,
  editor: m.metadata_field_label_editor,
  reading_time_minutes: m.metadata_field_label_reading_time_minutes,
  dateline_location: m.metadata_field_label_dateline_location,
  paywall_status: m.metadata_field_label_paywall_status,
  correction_notice: m.metadata_field_label_correction_notice,
  editorial_labels: m.metadata_field_label_editorial_labels,
  external_citations: m.metadata_field_label_external_citations,
  images: m.metadata_field_label_images,
  social_share_counts: m.metadata_field_label_social_share_counts,
  revision_date: m.metadata_field_label_revision_date
};

// The display label for a categorical-field machine name. Curated where known,
// humanize fallback otherwise (so a future field still reads cleanly).
export function fieldLabel(name: string): string {
  const fn = FIELD_LABELS[name];
  return fn ? fn() : humanizeMachineName(name);
}

// One name → label for a dimension that may be either a metric or a field. Used
// by mixed pickers (the dimension dropdown lists metrics AND fields). Prefers a
// registered metric label, then a curated field label, then humanize.
export function dimensionLabel(name: string): string {
  if (!name) return '';
  if (metricLabels[name]) return metricLabels[name];
  const fn = FIELD_LABELS[name];
  return fn ? fn() : humanizeMachineName(name);
}
