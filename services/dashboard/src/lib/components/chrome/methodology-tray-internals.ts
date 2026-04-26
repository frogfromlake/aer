// Pure helpers backing MethodologyTray.svelte (Phase 108).
// Kept rune-free in their own module so vitest can import them under
// the default `node` environment without a Svelte compiler pass.

import type { components } from '$lib/api/types';

type MetricProvenance = components['schemas']['MetricProvenance'];

export type BadgeTier =
  | 'tier1-unvalidated'
  | 'tier1-validated'
  | 'tier2-validated'
  | 'tier3'
  | 'expired'
  | 'refused';

/**
 * Map (tierClassification, validationStatus) → Badge tier.
 *
 * The BFF's enum is more granular than the badge palette: the badge
 * collapses tier-2-unvalidated and tier-1-unvalidated into the same
 * visual treatment because both report the same epistemic status to
 * the reader (Brief §5.8 — Epistemic Weight is a function of evidence,
 * not naming).
 */
export function pickBadgeTier(p: MetricProvenance | null): BadgeTier {
  if (!p) return 'tier1-unvalidated';
  if (p.validationStatus === 'expired') return 'expired';
  if (p.tierClassification === 3) return 'tier3';
  if (p.tierClassification === 2) {
    return p.validationStatus === 'validated' ? 'tier2-validated' : 'tier1-unvalidated';
  }
  return p.validationStatus === 'validated' ? 'tier1-validated' : 'tier1-unvalidated';
}

/**
 * Parse a Working Paper anchor like `"WP-002 §3"` into a deep link
 * targeting `/reflection/wp/wp-002?section=3`. Returns null when the
 * input is empty or doesn't match the canonical shape.
 *
 * Phase 109 materialises the target route; Phase 108 only needs to
 * emit the URL — anchors that pre-date the section grammar
 * (e.g. `"WP-001"` with no §) drop the query string but still resolve
 * to the WP page.
 */
export function workingPaperHref(anchor: string | null | undefined): string | null {
  if (!anchor) return null;
  const m = anchor.match(/^(WP-\d+)\s*§?\s*(.*)$/i);
  if (!m) return null;
  const id = m[1]!.toLowerCase();
  const section = (m[2] ?? '').trim();
  const qs = section ? `?section=${encodeURIComponent(section)}` : '';
  return `/reflection/wp/${id}${qs}`;
}
