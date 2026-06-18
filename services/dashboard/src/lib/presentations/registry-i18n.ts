// Locale-resolution layer for the presentation/pillar registry (Phase 144b /
// ADR-042). Extracted from registry.ts so the registry stays under the file-
// length cap; this module owns the id→message lookups and the two `localize*`
// helpers the registry accessors apply.
//
// The English literals in `registry-data.ts` (presentation label/description)
// and in `PILLAR_DEFINITIONS` (pillar blurb/description) remain the structural
// fallback; these helpers swap in the active-UI-locale strings. Reactive —
// `m.*()` reads the locale rune per call, so a View picker / cell header / pillar
// tooltip re-renders on a language switch with no consumer change. The relative
// `m` import keeps the module node-safe (Vitest does not resolve `$lib` at
// runtime); node tests fall back to the base locale 'en'. The pillar `label`
// (Aleph / Episteme / Rhizome) is an EMIC proper noun and is never translated.
import type { Presentation, PillarId } from '$lib/state/url-internals';
import { m } from '../paraglide/messages.js';
import type { PresentationDefinition, PillarDefinition } from './registry';

const PRESENTATION_MESSAGES: Record<Presentation, { label: () => string; desc: () => string }> = {
  time_series: {
    label: () => m.domain_presentation_time_series_label(),
    desc: () => m.domain_presentation_time_series_desc()
  },
  distribution: {
    label: () => m.domain_presentation_distribution_label(),
    desc: () => m.domain_presentation_distribution_desc()
  },
  cooccurrence_network: {
    label: () => m.domain_presentation_cooccurrence_network_label(),
    desc: () => m.domain_presentation_cooccurrence_network_desc()
  },
  metric_scatter: {
    label: () => m.domain_presentation_metric_scatter_label(),
    desc: () => m.domain_presentation_metric_scatter_desc()
  },
  topic_distribution: {
    label: () => m.domain_presentation_topic_distribution_label(),
    desc: () => m.domain_presentation_topic_distribution_desc()
  },
  topic_evolution: {
    label: () => m.domain_presentation_topic_evolution_label(),
    desc: () => m.domain_presentation_topic_evolution_desc()
  },
  revision_activity: {
    label: () => m.domain_presentation_revision_activity_label(),
    desc: () => m.domain_presentation_revision_activity_desc()
  },
  revision_timeline: {
    label: () => m.domain_presentation_revision_timeline_label(),
    desc: () => m.domain_presentation_revision_timeline_desc()
  },
  revision_discourse_shift: {
    label: () => m.domain_presentation_revision_discourse_shift_label(),
    desc: () => m.domain_presentation_revision_discourse_shift_desc()
  },
  revision_edit_clusters: {
    label: () => m.domain_presentation_revision_edit_clusters_label(),
    desc: () => m.domain_presentation_revision_edit_clusters_desc()
  },
  cross_probe_lead_lag: {
    label: () => m.domain_presentation_cross_probe_lead_lag_label(),
    desc: () => m.domain_presentation_cross_probe_lead_lag_desc()
  },
  categorical_distribution: {
    label: () => m.domain_presentation_categorical_distribution_label(),
    desc: () => m.domain_presentation_categorical_distribution_desc()
  },
  correlation_matrix: {
    label: () => m.domain_presentation_correlation_matrix_label(),
    desc: () => m.domain_presentation_correlation_matrix_desc()
  },
  cross_tab: {
    label: () => m.domain_presentation_cross_tab_label(),
    desc: () => m.domain_presentation_cross_tab_desc()
  },
  metric_lead_lag: {
    label: () => m.domain_presentation_metric_lead_lag_label(),
    desc: () => m.domain_presentation_metric_lead_lag_desc()
  },
  parallel_coordinates: {
    label: () => m.domain_presentation_parallel_coordinates_label(),
    desc: () => m.domain_presentation_parallel_coordinates_desc()
  },
  sankey: {
    label: () => m.domain_presentation_sankey_label(),
    desc: () => m.domain_presentation_sankey_desc()
  }
};

const PILLAR_MESSAGES: Record<PillarId, { blurb: () => string; desc: () => string }> = {
  aleph: { blurb: () => m.domain_pillar_aleph_blurb(), desc: () => m.domain_pillar_aleph_desc() },
  episteme: {
    blurb: () => m.domain_pillar_episteme_blurb(),
    desc: () => m.domain_pillar_episteme_desc()
  },
  rhizome: {
    blurb: () => m.domain_pillar_rhizome_blurb(),
    desc: () => m.domain_pillar_rhizome_desc()
  }
};

/** A copy of `def` with `label`/`description` resolved for the active UI locale
 *  (id and every structural flag are locale-independent). */
export function localizePresentation(def: PresentationDefinition): PresentationDefinition {
  const msg = PRESENTATION_MESSAGES[def.id];
  return msg ? { ...def, label: msg.label(), description: msg.desc() } : def;
}

/** A copy of `def` with `blurb`/`description` resolved for the active UI locale
 *  (the emic `label` and every other field are locale-independent). */
export function localizePillar(def: PillarDefinition): PillarDefinition {
  const msg = PILLAR_MESSAGES[def.id];
  return msg ? { ...def, blurb: msg.blurb(), description: msg.desc() } : def;
}
