// Exact-value hover readout — Phase 132 (pure pieces).
//
// Every Workbench cell surfaces the EXACT underlying datum (and every bound
// visual channel) when the pointer rests on a mark, through one shared
// `CellReadout` box. Axes can never label every value on a continuous scale,
// so the readout — not denser ticks — is how a reader reads precise values.
//
// This module holds the substrate-independent, vitest-pinnable pieces:
//   • number / timestamp formatting (consistent across every cell);
//   • viewport-edge clamp of the pointer-anchored box;
//   • the Observable-Plot pointer→data-index resolver (the same
//     `ownerSVGElement` + DOM-order `indexOf` pattern proven by the
//     Phase-122d.1 revision click handler — safe ONLY for marks rendered
//     in input data order: no `sort`, no `facet`, no `stack`).
// The Svelte-only concerns (positioning element, rendering) live in
// `CellReadout.svelte`.

/** One line in the readout box. `swatch` paints a small colour chip before
 *  the label (used for per-series / per-source rows). */
export interface ReadoutRow {
  label: string;
  value: string;
  swatch?: string;
}

/** The complete readout state a cell hands to `CellReadout`. `x`/`y` are
 *  viewport (clientX/clientY) coordinates; the box follows the pointer. */
export interface ReadoutState {
  visible: boolean;
  x: number;
  y: number;
  title?: string;
  rows: ReadoutRow[];
  /** Optional affordance footer — e.g. "Click to see articles" on clickable
   *  marks. Rendered muted/italic below the value rows. */
  hint?: string;
}

/** The hidden/empty state — assign this to dismiss the readout. */
export const HIDDEN_READOUT: ReadoutState = Object.freeze({
  visible: false,
  x: 0,
  y: 0,
  rows: []
});

/** Format a numeric value for the readout. Integers render exact (counts
 *  stay clean: `5`, not `5.000`); large magnitudes drop the fraction;
 *  otherwise 3 dp — matching the DistributionCell quantile convention so the
 *  readout and the static summary agree to the digit. */
export function fmtValue(n: number | null | undefined): string {
  if (n === null || n === undefined || !Number.isFinite(n)) return '—';
  if (Number.isInteger(n)) return String(n);
  return Math.abs(n) >= 100 ? n.toFixed(0) : n.toFixed(3);
}

/** Format a seconds-epoch timestamp as a compact `YYYY-MM-DD HH:mm` UTC
 *  label (the dashboard's canonical time grain for hover readouts). */
export function fmtTimestamp(secondsEpoch: number): string {
  if (!Number.isFinite(secondsEpoch)) return '—';
  const iso = new Date(secondsEpoch * 1000).toISOString();
  return `${iso.slice(0, 10)} ${iso.slice(11, 16)}`;
}

/** Clamp a pointer-anchored box inside the viewport. The box is normally
 *  placed below-right of the pointer; when that would overflow the right or
 *  bottom edge it flips to the opposite side, never leaving a `margin` gutter.
 *  Pure + testable — no DOM access. */
export function clampReadoutPosition(
  clientX: number,
  clientY: number,
  boxW: number,
  boxH: number,
  viewportW: number,
  viewportH: number,
  offset = 14,
  margin = 8
): { left: number; top: number } {
  let left = clientX + offset;
  let top = clientY + offset;
  if (left + boxW + margin > viewportW) {
    left = Math.max(margin, clientX - offset - boxW);
  }
  if (top + boxH + margin > viewportH) {
    top = Math.max(margin, clientY - offset - boxH);
  }
  return { left, top };
}

/** Map a pointer event over an Observable Plot SVG to the data index of the
 *  hovered mark, using the clicked element's `ownerSVGElement` + DOM-order
 *  `indexOf` (the Phase-122d.1 pattern). Returns `null` when the pointer is
 *  not over a `selector` mark.
 *
 *  SAFE ONLY for marks rendered in input data order — `Plot.barX`/`rectY`/
 *  `dot`/`line` with NO `sort`, `facet`, or `stack`. Sorted/faceted/stacked
 *  marks (the BERTopic cells) reorder the DOM and must use Plot's own
 *  data-bound `tip` instead. */
export function markIndexFromEvent(target: EventTarget | null, selector: string): number | null {
  const el = target instanceof Element ? target.closest(selector) : null;
  if (!el) return null;
  const svg = (el as SVGElement).ownerSVGElement;
  if (!svg) return null;
  const idx = Array.from(svg.querySelectorAll(selector)).indexOf(el);
  return idx >= 0 ? idx : null;
}
