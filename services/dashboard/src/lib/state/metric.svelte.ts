// Focused-metric store (Phase 105). Tracks which metric is currently in
// focus for the methodology tray (§3.3 — "the tray's content follows
// whatever metric or finding is currently in focus"). Chart interactions
// set this via setFocusedMetric; the tray subscribes and updates in place.
//
// Intentionally NOT URL-backed: the focused metric is transient UI state.
// Deep-linking into a methodology view uses the ?metric= parameter (already
// in url-internals.ts), which is scoped to the analysis view; the tray
// binding is a live interaction, not a persistent URL state.

export interface FocusedMetric {
  metricName: string;
  /** Optional opaque context string, e.g. probe + time bucket, for scoping
   *  tray content to a specific point or selection (Phase 108). */
  chartContext?: string;
}

const browser = typeof window !== 'undefined';

let internal = $state<FocusedMetric | null>(null);

export function focusedMetric(): FocusedMetric | null {
  return internal;
}

export function setFocusedMetric(next: FocusedMetric | null): void {
  if (!browser) return;
  internal = next;
}
